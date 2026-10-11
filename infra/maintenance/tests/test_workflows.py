import unittest
from pathlib import Path

ROOT = Path('/repository')


class WorkflowTests(unittest.TestCase):
    def workflows(self):
        paths = [ROOT / '.github/workflows' / ('maintenance-' + name + '.yml') for name in ('scan', 'apply', 'test')]
        for path in paths:
            self.assertTrue(path.exists(), 'maintenance workflow missing')
        import yaml
        return [yaml.safe_load(path.read_text()) for path in paths]

    def test_jobs_have_fixed_checkout_and_no_public_artifacts(self):
        for workflow in self.workflows():
            for job in workflow['jobs'].values():
                for step in job['steps']:
                    if 'uses' in step:
                        self.assertEqual(step['uses'], 'actions/checkout@fbc6f3992d24b796d5a048ff273f7fcc4a7b6c09')
                        self.assertFalse(step['with']['persist-credentials'])
                    if 'run' in step:
                        self.assertNotIn('${{ inputs.', step['run'])
                        self.assertNotIn('${{ secrets.', step['run'])

    def test_native_gate_does_not_hold_deployment_lock_and_execute_has_no_signer_writer(self):
        scan, apply, _ = self.workflows()
        self.assertNotIn('concurrency', apply)
        approve = apply['jobs']['approve']
        execute = apply['jobs']['execute']
        self.assertEqual(approve['environment'], 'production-maintenance')
        self.assertNotIn('concurrency', approve)
        self.assertEqual(execute['environment'], 'maintenance-executor')
        self.assertEqual(execute['concurrency'], {'group': 'production-deploy', 'cancel-in-progress': False})
        text = str(execute)
        self.assertNotIn('MAINTENANCE_APPROVAL_PRIVATE_KEY', text)
        self.assertNotIn('MAINTENANCE_OPS_WRITE_KEY', text)
        self.assertIn('MAINTENANCE_OPS_READ_KEY', text)
        self.assertIn('MAINTENANCE_APPLY_SSH_KEY', text)
        for job in apply['jobs'].values():
            self.assertIn("github.ref == 'refs/heads/main'", job['if'])
        self.assertEqual(scan['permissions']['actions'], 'write')
        self.assertEqual(scan['jobs']['scan']['environment'], 'maintenance-scan')
        events = scan.get('on', scan.get(True))
        self.assertEqual(events['schedule'][0]['cron'], '0 0 * * *')
        self.assertFalse(events['workflow_dispatch']['inputs']['force_request']['default'])
        self.assertFalse(scan['concurrency']['cancel-in-progress'])
        for workflow in (scan, apply):
            for job in workflow['jobs'].values():
                checkout = next(step for step in job['steps'] if 'uses' in step)
                self.assertEqual(checkout['with']['ref'], '${{ vars.MAINTENANCE_EXECUTOR_SHA }}')

    def test_ansible_only_runs_local_fixed_cli(self):
        for name, operation in [('scan', 'scan'), ('apply', 'execute')]:
            path = ROOT / 'infra/maintenance/playbooks' / (name + '.yml')
            self.assertTrue(path.exists(), 'controller-local playbook missing')
            import yaml
            play = yaml.safe_load(path.read_text())[0]
            self.assertEqual(play['hosts'], 'localhost')
            self.assertEqual(play['connection'], 'local')
            self.assertFalse(play['gather_facts'])
            task = play['tasks'][0]
            self.assertTrue(task['no_log'])
            self.assertEqual(task['ansible.builtin.command']['argv'], ['python', '-B', '-m', 'maintenance.cli', operation])

    def test_always_audit_has_only_writer_and_no_execution_credentials_or_lock(self):
        _, apply, _ = self.workflows()
        self.assertIn('audit', apply['jobs'], 'durable audit workflow job missing')
        audit = apply['jobs']['audit']
        self.assertEqual(audit['needs'], ['prepare', 'approve', 'execute'])
        self.assertIn('always()', audit['if'])
        self.assertIn("github.ref == 'refs/heads/main'", audit['if'])
        self.assertEqual(audit['environment'], 'maintenance-audit')
        self.assertNotIn('concurrency', audit)
        text = str(audit)
        self.assertIn('MAINTENANCE_OPS_WRITE_KEY', text)
        for name in ('MAINTENANCE_OPS_READ_KEY', 'MAINTENANCE_APPLY_SSH_KEY', 'MAINTENANCE_SCAN_SSH_KEY', 'MAINTENANCE_APPROVAL_PRIVATE_KEY', 'MAINTENANCE_HOSTS_JSON', 'GH_TOKEN'):
            self.assertNotIn(name, text)
        self.assertIn('needs.execute.result', text)
        self.assertIn('needs.approve.outputs.approval_commit', text)


class ReceiptTests(unittest.TestCase):
    def setUp(self):
        from test_host import HostTests
        self.fixture = HostTests()
        self.fixture.setUp()
        self.addCleanup(self.fixture.tmp.cleanup)

    def test_report_returns_private_root_receipt_without_mutation(self):
        fixture = self.fixture
        request = fixture.request()
        envelope = request['envelope']
        fixture.agent.run('apply', request)
        before = list(fixture.runner.calls)
        response = fixture.agent.run('report', {})
        self.assertIn('receipts', response, 'complete private root receipt missing')
        self.assertEqual(response['receipts'][0]['plan'], envelope['plan'])
        self.assertEqual(fixture.runner.calls, before)
        self.assertEqual(response['receipt_total'], 1)

    def test_report_rejects_symlink_and_nonprivate_receipts(self):
        import uuid
        from maintenance.plan import MaintenanceError
        fixture = self.fixture
        fixture.agent.run('report', {})
        path = fixture.agent.state / 'plans' / (str(uuid.uuid4()) + '.json')
        outside = fixture.root / 'outside.json'
        outside.write_text('{"status":"failed"}')
        path.symlink_to(outside)
        with self.assertRaises(MaintenanceError):
            fixture.agent.run('report', {})
        path.unlink()
        path.write_text('{"status":"failed"}')
        path.chmod(0o644)
        with self.assertRaises(MaintenanceError):
            fixture.agent.run('report', {})

    def test_report_bounds_receipts_and_preserves_total(self):
        import uuid
        fixture = self.fixture
        fixture.agent.run('report', {})
        for _ in range(23):
            fixture.agent._write(fixture.agent.state / 'plans' / (str(uuid.uuid4()) + '.json'), {'status': 'failed', 'diagnostic': 'synthetic'})
        response = fixture.agent.run('report', {})
        self.assertEqual(len(response['receipts']), 20)
        self.assertEqual(response['receipt_total'], 23)
        self.assertEqual(response['receipt_truncated'], 3)
        self.assertTrue(response['receipt_history_complete'])
        self.assertEqual(response['unresolved_execution_count'], 23)

    def test_report_aggregate_detects_hidden_failure_and_invalid_diagnosis(self):
        import uuid
        fixture = self.fixture
        fixture.agent.run('report', {})
        oldest = fixture.agent.state / 'plans' / (str(uuid.uuid4()) + '.json')
        fixture.agent._write(oldest, {'status': 'failed', 'started_at': 900, 'operator_diagnosed_at': True})
        for _ in range(21):
            fixture.agent._write(fixture.agent.state / 'plans' / (str(uuid.uuid4()) + '.json'), {'status': 'verified'})
        response = fixture.agent.run('report', {})
        self.assertTrue(response['receipt_history_complete'])
        self.assertEqual(response['unresolved_execution_count'], 1)
        self.assertTrue(all(record['status'] == 'verified' for record in response['receipts']))
        fixture.agent._write(oldest, {'status': 'failed', 'started_at': 900, 'operator_diagnosed_at': 950})
        self.assertEqual(fixture.agent.run('report', {})['unresolved_execution_count'], 0)

    def test_report_history_bounds_yield_unknown_not_zero_unresolved(self):
        import uuid
        from unittest.mock import patch
        from maintenance import host
        fixture = self.fixture
        fixture.agent.run('report', {})
        fixture.agent._write(fixture.agent.state / 'plans' / (str(uuid.uuid4()) + '.json'), {'status': 'verified', 'diagnostic': 'large'})
        with patch.object(host, 'MAX_RECEIPT_HISTORY_BYTES', 1, create=True):
            result = fixture.agent.run('report', {})
        self.assertFalse(result['receipt_history_complete'])
        self.assertIsNone(result['unresolved_execution_count'])


if __name__ == '__main__':
    unittest.main()
