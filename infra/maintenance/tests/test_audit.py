import copy
import json
import unittest
import tempfile
from pathlib import Path
from unittest.mock import Mock, patch

from maintenance import cli
from maintenance.plan import MaintenanceError, build_plan, canonical_json, plan_sha256
from test_controller import ready


class AuditTests(unittest.TestCase):
    def setUp(self):
        self.assertTrue(hasattr(cli, 'audit'), 'durable failure audit missing')
        self.executor = 'a' * 40
        self.plan = build_plan(ready(), self.executor, now=1000)
        self.inputs = {'private_commit': 'b' * 40, 'plan_id': self.plan['plan_id'],
                       'plan_sha256': plan_sha256(self.plan), 'executor_sha': self.executor,
                       'approval_commit': '', 'approval_sha256': '', 'dry_run': True,
                       'job_results': {'prepare': 'success', 'approve': 'success', 'execute': 'failure'}}
        self.environment = {'GITHUB_RUN_ID': '123', 'GITHUB_RUN_ATTEMPT': '2',
                            'MAINTENANCE_PIPELINE_SHA': 'c' * 40}

    def run_audit(self, inputs=None, git=None):
        git = git or Mock()
        git.fetch.return_value = canonical_json(self.plan)
        git.head.return_value = 'd' * 40
        git.read_at.return_value = None
        git.write.return_value = 'e' * 40
        with patch.dict('os.environ', self.environment), patch.object(cli, 'summary'), patch.object(cli, 'Client') as transport, patch.object(cli, 'sign_plan') as signer:
            cli.audit(self.executor, 'enjoydarts/sifto-operations', inputs or self.inputs, git)
        transport.assert_not_called()
        signer.assert_not_called()
        return git, git.write.call_args.args[1]['executions/123-2.json']

    def test_failed_preflight_durable_audit_does_not_need_root_receipt_or_host_keys(self):
        git, record = self.run_audit()
        self.assertEqual(record['job_results']['execute'], 'failure')
        self.assertEqual(record['phase'], 'execute')
        self.assertEqual(record['plan_sha256'], plan_sha256(self.plan))
        self.assertIsNone(record['signed_mode'])
        self.assertEqual(record['requested_mode'], 'dry_run')
        self.assertEqual(record['run_attempt'], '2')
        self.assertEqual(record['plan_integrity'], 'matched')
        self.assertEqual(set(record), cli.AUDIT_FIELDS)

    def test_invalid_references_and_results_are_sanitized_never_record_raw_inputs(self):
        values = copy.deepcopy(self.inputs)
        for key in ('private_commit', 'plan_id', 'plan_sha256', 'executor_sha', 'approval_commit', 'approval_sha256'):
            values[key] = 'password-secret; 10.0.0.1'
        values['job_results']['execute'] = 'raw-secret-failure'
        git, record = self.run_audit(values)
        self.assertNotIn('password-secret', json.dumps(record))
        self.assertNotIn('10.0.0.1', json.dumps(record))
        self.assertNotIn('raw-secret', json.dumps(record))
        self.assertTrue(record['invalid_fields'])
        self.assertIsNone(record['private_commit'])
        git.fetch.assert_not_called()

    def test_skipped_cancelled_and_failure_phases_are_retained(self):
        for phase, results in [('prepare', {'prepare': 'failure', 'approve': 'skipped', 'execute': 'skipped'}),
                               ('approve', {'prepare': 'success', 'approve': 'cancelled', 'execute': 'skipped'}),
                               ('execute', {'prepare': 'success', 'approve': 'success', 'execute': 'cancelled'})]:
            values = {**self.inputs, 'job_results': results}
            _, record = self.run_audit(values)
            self.assertEqual(record['phase'], phase)
            self.assertEqual(record['job_results'], results)

    def test_signed_mode_is_derived_from_exact_envelope_not_requested_mode(self):
        values = {**self.inputs, 'approval_commit': 'f' * 40, 'approval_sha256': '1' * 64}
        envelope = {'plan': self.plan, 'approval': {'plan_sha256': plan_sha256(self.plan),
                    'run_id': '123', 'approved_by': 'enjoydarts', 'approved_at': 1001,
                    'expires_at': 2000, 'workflow_sha': self.executor, 'execution_mode': 'apply'}, 'signature': 'synthetic'}
        git = Mock()
        git.fetch.side_effect = [canonical_json(self.plan), canonical_json(envelope)]
        _, record = self.run_audit(values, git)
        self.assertEqual(record['signed_mode'], 'apply')
        self.assertEqual(record['requested_mode'], 'dry_run')
        self.assertIn('execution_mode_mismatch', record['invalid_fields'])
        self.assertEqual(git.fetch.call_args_list[1].args, ('f' * 40, 'approvals/' + self.plan['plan_id'] + '/123.json'))
        self.assertEqual(git.fetch.call_args_list[1].kwargs, {'digest': '1' * 64})

    def test_only_private_audit_write_retries_on_changed_parent_without_execution(self):
        git = Mock()
        git.fetch.return_value = canonical_json(self.plan)
        git.head.side_effect = ['d' * 40, 'f' * 40]
        git.read_at.return_value = None
        git.write.side_effect = [MaintenanceError('private_git_failed'), 'e' * 40]
        with patch.dict('os.environ', self.environment), patch.object(cli, 'summary'), patch.object(cli, 'Client') as transport:
            cli.audit(self.executor, 'enjoydarts/sifto-operations', self.inputs, git)
        self.assertEqual(git.write.call_count, 2)
        self.assertEqual([call.args[0] for call in git.write.call_args_list], ['d' * 40, 'f' * 40])
        transport.assert_not_called()

    def test_same_parent_failure_is_not_retried_and_existing_audit_is_not_overwritten(self):
        git = Mock()
        git.fetch.return_value = canonical_json(self.plan)
        git.head.return_value = 'd' * 40
        git.read_at.return_value = None
        git.write.side_effect = MaintenanceError('private_git_failed')
        with patch.dict('os.environ', self.environment), patch.object(cli, 'summary'):
            with self.assertRaises(MaintenanceError):
                cli.audit(self.executor, 'enjoydarts/sifto-operations', self.inputs, git)
        self.assertEqual(git.write.call_count, 1)

    def test_existing_audit_is_immutable_and_retry_count_is_bounded(self):
        _, record = self.run_audit()
        git = Mock()
        git.fetch.return_value = canonical_json(self.plan)
        git.head.return_value = 'd' * 40
        git.read_at.return_value = canonical_json(record)
        with patch.dict('os.environ', self.environment), patch.object(cli, 'summary'):
            cli.audit(self.executor, 'enjoydarts/sifto-operations', self.inputs, git)
        git.write.assert_not_called()
        changed = {**record, 'signed_mode': 'apply'}
        git.read_at.return_value = canonical_json(changed)
        with patch.dict('os.environ', self.environment), patch.object(cli, 'summary'):
            with self.assertRaises(MaintenanceError):
                cli.audit(self.executor, 'enjoydarts/sifto-operations', self.inputs, git)
        git.head.side_effect = ['d' * 40, 'e' * 40, 'f' * 40]
        git.read_at.return_value = None
        git.write.side_effect = MaintenanceError('private_git_failed')
        with patch.dict('os.environ', self.environment), patch.object(cli, 'summary'):
            with self.assertRaises(MaintenanceError):
                cli.audit(self.executor, 'enjoydarts/sifto-operations', self.inputs, git)
        self.assertEqual(git.write.call_count, 3)

    def test_audit_configuration_holds_only_writer_and_rejects_accidental_keys(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            (root / 'secrets').mkdir()
            inputs = {**self.inputs, 'force_request': False}
            (root / 'secrets/inputs.json').write_text(json.dumps(inputs))
            env = {**self.environment, 'GITHUB_REF': 'refs/heads/main',
                   'MAINTENANCE_EXECUTOR_SHA': self.executor, 'MAINTENANCE_CHECKED_OUT_SHA': self.executor,
                   'MAINTENANCE_OPS_REPO': 'enjoydarts/sifto-operations'}
            def local_path(*parts):
                value = Path(*parts)
                return root / str(value).lstrip('/') if str(value).startswith(('/private', '/secrets')) else value
            with patch.dict('os.environ', env, clear=True), patch.object(cli, 'Path', side_effect=local_path), patch.object(cli, 'GitStore') as store:
                cli.configuration('audit')
                self.assertTrue(store.call_args.kwargs['writer'])
                self.assertEqual(store.call_args.args[2], root / 'secrets/ops_key')
                for key in ('MAINTENANCE_APPLY_SSH_KEY', 'MAINTENANCE_SCAN_SSH_KEY', 'MAINTENANCE_APPROVAL_PRIVATE_KEY', 'MAINTENANCE_OPS_READ_KEY', 'GH_TOKEN'):
                    with patch.dict('os.environ', {key: 'synthetic-key'}):
                        with self.assertRaises(MaintenanceError):
                            cli.configuration('audit')
                (root / 'secrets/host_key').write_text('synthetic-key')
                with self.assertRaises(MaintenanceError):
                    cli.configuration('audit')


if __name__ == '__main__':
    unittest.main()
