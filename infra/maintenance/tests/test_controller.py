import copy
import base64
import json
import tempfile
import unittest
from pathlib import Path
from unittest.mock import patch
from unittest.mock import Mock
from cryptography.hazmat.primitives import serialization
from cryptography.hazmat.primitives.asymmetric.ed25519 import Ed25519PrivateKey

from maintenance.plan import MaintenanceError, build_plan, plan_sha256
from test_plan import inventory

try:
    from maintenance import cli, report, ssh, store
except ImportError:
    cli = report = ssh = store = None


def ready():
    value = inventory()
    value.update(metadata={"fresh": True}, apt_locks=[], dpkg_state={"ready": True},
                 simulation_available=True, reboot_required=False,
                 maintenance_policy={"host_role": "application", "database_enabled": False,
                                     "backup_ready": False, "allowed_restart_services": [],
                                     "database_restart_services": []})
    value["runtime"].update(services={"docker.service": "active"})
    return value


class ControllerTests(unittest.TestCase):
    def setUp(self):
        self.assertIsNotNone(cli, "controller implementation missing")
        self.sha = "a" * 40

    def test_unknown_simulation_is_never_zero_updates(self):
        inv = ready()
        inv["simulation_available"] = False
        inv["simulation"] = ""
        result = cli.assess(inv, self.sha, now=1000)
        self.assertEqual(result["status"], "unknown")
        self.assertIsNone(result["update_count"])
        self.assertIsNone(result["plan"])

    def test_backup_blocked_database_cannot_request_approval(self):
        inv = ready()
        inv["maintenance_policy"]["host_role"] = "database"
        result = cli.assess(inv, self.sha, now=1000)
        self.assertEqual(result["status"], "blocked")
        self.assertIsNone(result["plan"])

    def test_stale_locks_pending_unhealthy_block(self):
        for key, value in [("metadata", {"fresh": False}), ("apt_locks", ["dpkg"]),
                           ("dpkg_state", {"ready": False})]:
            inv = ready()
            inv[key] = value
            self.assertNotEqual(cli.assess(inv, self.sha, now=1000)["status"], "eligible")
        inv = ready()
        inv["runtime"]["services"]["docker.service"] = "failed"
        self.assertEqual(cli.assess(inv, self.sha, now=1000)["status"], "blocked")

    def test_reboot_is_separate_and_only_after_empty_package_closure(self):
        inv = ready()
        inv["reboot_required"] = True
        self.assertEqual(cli.assess(inv, self.sha, now=1000)["plan"]["action"], "apt_upgrade")
        inv["simulation"] = ""
        self.assertEqual(cli.assess(inv, self.sha, now=1000)["plan"]["action"], "reboot")

    def test_ai_payload_only_contains_typed_changes_runtime_and_references(self):
        inv = ready()
        inv["runtime"]["secret"] = "password-secret"
        plan = build_plan(inv, self.sha, now=1000)
        before = copy.deepcopy(plan)
        payload = report.ai_payload(plan)
        self.assertEqual(set(payload), {"packages", "expected_runtime", "references"})
        self.assertNotIn(inv["host_id"], json.dumps(payload))
        self.assertNotIn("password-secret", json.dumps(payload))
        with patch.object(report.urllib.request, "urlopen", side_effect=OSError("private IP")):
            result = report.ai_advice(plan, "key")
        self.assertEqual(result["status"], "unavailable")
        self.assertNotIn("private IP", json.dumps(result))
        self.assertEqual(plan, before)

    def test_ai_cannot_add_execution_fields(self):
        with self.assertRaises(MaintenanceError):
            report.validate_advice({"summary_ja": "確認", "warnings_ja": [], "commands": ["id"]})

    def test_report_discloses_package_script_restart_scope_and_exact_versions(self):
        result = cli.assess(ready(), self.sha, now=1000)
        rendered = report.render(result, {"status": "unavailable", "warning_ja": "AI 利用不可"})
        self.assertIn("3.0.13-0ubuntu3.5", rendered)
        self.assertIn("package_scripts_and_listed_services", rendered)
        self.assertIn("AI 利用不可", rendered)
        self.assertIn(plan_sha256(result["plan"]), rendered)

    def test_dedupe_survives_new_uuid_and_expiry_until_force(self):
        first = build_plan(ready(), self.sha, now=1000)
        second = build_plan(ready(), self.sha, now=99999)
        ledger = {}
        self.assertTrue(cli.reserve_candidate(ledger, first))
        self.assertFalse(cli.reserve_candidate(ledger, second))
        self.assertTrue(cli.reserve_candidate(ledger, second, force=True))

    def test_actual_review_history_identity_required_not_actor(self):
        approved = [{"state": "approved", "user": {"login": "enjoydarts"},
                     "environments": [{"name": "production-maintenance", "id": 1}]}]
        self.assertEqual(cli.actual_approver(approved, ["enjoydarts"]), "enjoydarts")
        for history in [[], [{**approved[0], "user": {"login": "github-actions[bot]"}}],
                        [{**approved[0], "environments": [{"name": "other"}]}],
                        approved + [{**approved[0], "state": "rejected"}],
                        [{**approved[0], "state": "rejected"}] + approved]:
            with self.assertRaises(MaintenanceError):
                cli.actual_approver(history, ["enjoydarts"])

    def test_host_config_is_typed_and_rejects_injection(self):
        value = [{"host_id": "app", "address": "example.invalid", "port": 22, "role": "application"}]
        self.assertEqual(ssh.parse_hosts(json.dumps(value)), value)
        for bad in ["-oProxyCommand=id", "example.invalid;id", "user@example.invalid", "a\nb"]:
            value[0]["address"] = bad
            with self.assertRaises(MaintenanceError):
                ssh.parse_hosts(json.dumps(value))

    def test_ssh_fixed_command_and_stdin_data_only(self):
        with tempfile.TemporaryDirectory() as directory:
            client = ssh.Client(Path(directory), Path("/secrets/scan_key"), Path("/secrets/known_hosts"), "scan")
            host = {"host_id": "app", "address": "example.invalid", "port": 22, "role": "application"}
            argv = client.argv(host, "scan")
            self.assertEqual(argv[-1], "scan")
            self.assertIn("sifto-maint-scan@example.invalid", argv)
            for item in ["-T", "StrictHostKeyChecking=yes", "BatchMode=yes", "IdentitiesOnly=yes",
                         "ForwardAgent=no", "ControlMaster=no"]:
                self.assertIn(item, argv)
            with self.assertRaises(MaintenanceError):
                client.argv(host, "apply")
            with self.assertRaises(MaintenanceError):
                client.argv(host, "scan;id")

    def test_private_files_and_store_identifiers(self):
        with tempfile.TemporaryDirectory() as directory:
            target = Path(directory) / "private"
            store.private_write(target / "file.json", {"secret": "x"})
            self.assertEqual(target.stat().st_mode & 0o777, 0o700)
            self.assertEqual((target / "file.json").stat().st_mode & 0o777, 0o600)
        self.assertEqual(store.repo_url("enjoydarts/sifto-operations"), "ssh://git@github.com/enjoydarts/sifto-operations.git")
        for bad in ["x/y;id", "-x/y", "https://github.com/x/y", "x/../y"]:
            with self.assertRaises(MaintenanceError):
                store.repo_url(bad)
        for commit, path in [("main", "ledger.json"), ("a" * 40, "../secret"), ("a" * 40, "plans/a.json;id")]:
            with self.assertRaises(MaintenanceError):
                store.validate_ref(commit, path)

    def test_immutable_plan_rejects_hash_pin_path_and_expiry(self):
        plan = build_plan(ready(), self.sha, now=1000)
        for digest, identifier, executor in [("0" * 64, plan["plan_id"], self.sha),
                                             (plan_sha256(plan), "../evil", self.sha),
                                             (plan_sha256(plan), plan["plan_id"], "b" * 40)]:
            with self.assertRaises(MaintenanceError):
                cli.checked_plan(plan, identifier, digest, executor, now=1001)
        self.assertEqual(cli.checked_plan(plan, plan["plan_id"], plan_sha256(plan), self.sha, now=1001), plan)
        with self.assertRaises(MaintenanceError):
            cli.checked_plan(plan, plan["plan_id"], plan_sha256(plan), self.sha, now=999999)

    def test_refresh_failure_is_unknown_and_keeps_private_inventory(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            (root / 'secrets').mkdir()
            (root / 'secrets/hosts.json').write_text(json.dumps([{'host_id': 'sifto-app', 'address': 'example.invalid', 'port': 22, 'role': 'application'}]))
            inventory = ready()
            inventory['metadata']['fresh'] = False
            transport = Mock()
            transport.call.side_effect = [inventory, MaintenanceError('private-address-failed')]
            git = Mock()
            git.head.return_value = 'b' * 40
            git.read_at.return_value = None
            git.write.return_value = 'c' * 40
            def local_path(value):
                return root / value.lstrip('/') if str(value).startswith(('/private', '/secrets')) else Path(value)
            with patch.object(cli, 'Path', side_effect=local_path), patch.object(cli, 'Client', return_value=transport), patch.object(cli, 'github') as api:
                cli.scan(self.sha, 'enjoydarts/sifto-operations', {'force_request': False, 'dry_run': True}, git)
            files = git.write.call_args.args[1]
            assessment = next(v for k, v in files.items() if k.endswith('/assessment.json'))
            self.assertEqual(assessment['status'], 'unknown')
            self.assertIsNone(assessment['update_count'])
            api.assert_not_called()
            self.assertNotIn('private-address', (root / 'private/summary.md').read_text())

    def test_no_native_approval_never_reads_signer_or_writes_envelope(self):
        plan = build_plan(ready(), self.sha, now=int(__import__('time').time()) - 1)
        inputs = {'private_commit': 'b' * 40, 'plan_id': plan['plan_id'], 'plan_sha256': plan_sha256(plan), 'dry_run': True}
        git = Mock()
        with patch.object(cli, 'load_plan', return_value=plan), patch.object(cli, 'github', return_value=[]), patch.dict('os.environ', {'GITHUB_RUN_ID': '123', 'MAINTENANCE_ALLOWED_APPROVERS': '["enjoydarts"]'}), patch.object(cli, 'sign_plan') as signer:
            with self.assertRaises(MaintenanceError):
                cli.authorize(self.sha, 'enjoydarts/sifto-operations', inputs, git)
        signer.assert_not_called()
        git.write.assert_not_called()

    def test_signed_plan_alteration_never_reaches_ssh(self):
        plan = build_plan(ready(), self.sha, now=1000)
        changed = copy.deepcopy(plan)
        changed['packages'][0]['target'] = '9.0'
        git = Mock()
        git.fetch.return_value = json.dumps({'plan': changed, 'approval': {}, 'signature': 'fake'}).encode()
        inputs = {'plan_id': plan['plan_id'], 'approval_commit': 'b' * 40, 'approval_sha256': 'c' * 64}
        with patch.object(cli, 'load_plan', return_value=plan), patch.dict('os.environ', {'GITHUB_RUN_ID': '123'}), patch.object(cli, 'Client') as transport:
            with self.assertRaises(MaintenanceError):
                cli.execute(self.sha, 'enjoydarts/sifto-operations', inputs, git)
        transport.assert_not_called()

    def test_immutable_git_fetch_validates_raw_digest_without_checkout(self):
        git = store.GitStore.__new__(store.GitStore)
        git.url = store.repo_url('enjoydarts/sifto-operations')
        git.git = Mock(side_effect=[b'', b'raw private data'])
        import hashlib
        self.assertEqual(git.fetch('a' * 40, 'plans/test.json', digest=hashlib.sha256(b'raw private data').hexdigest()), b'raw private data')
        self.assertEqual(git.git.call_args_list[0].args, ('fetch', '--depth=1', '--no-tags', git.url, 'a' * 40))
        self.assertEqual(git.git.call_args_list[1].args, ('show', 'a' * 40 + ':plans/test.json'))
        git.git = Mock(side_effect=[b'', b'changed'])
        with self.assertRaises(MaintenanceError):
            git.fetch('a' * 40, 'plans/test.json', digest='b' * 64)

    def test_ssh_failure_diagnostics_are_private_and_exception_sanitized(self):
        with tempfile.TemporaryDirectory() as directory:
            client = ssh.Client(Path(directory), '/secrets/key', '/secrets/known_hosts', 'scan')
            host = {'host_id': 'app', 'address': 'example.invalid', 'port': 22, 'role': 'application'}
            with patch.object(ssh, 'capture', return_value=(255, b'private inventory', b'address secret', False)):
                with self.assertRaisesRegex(MaintenanceError, '^ssh_failed$'):
                    client.call(host, 'scan')
            self.assertTrue(any(path.read_bytes() == b'address secret' for path in Path(directory).rglob('stderr.bin')))

    def test_ai_malicious_extra_execution_fields_are_unavailable(self):
        plan = build_plan(ready(), self.sha, now=1000)
        response = Mock()
        response.__enter__ = Mock(return_value=response)
        response.__exit__ = Mock(return_value=False)
        response.read.return_value = json.dumps({'stop_reason': 'end_turn', 'content': [{'type': 'text', 'text': '{"summary_ja":"説明","warnings_ja":[],"allow_reboot":true}'}]}).encode()
        before = copy.deepcopy(plan)
        with patch.object(report.urllib.request, 'urlopen', return_value=response):
            self.assertEqual(report.ai_advice(plan, 'synthetic-key')['status'], 'unavailable')
        self.assertEqual(plan, before)

    def test_signed_synthetic_execute_applies_once_and_verifies_frozen_envelope(self):
        from maintenance.plan import sign_plan
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            (root / 'secrets').mkdir()
            (root / 'secrets/hosts.json').write_text(json.dumps([{'host_id': 'sifto-app', 'address': 'example.invalid', 'port': 22, 'role': 'application'}]))
            plan = build_plan(ready(), self.sha, now=1000)
            approval = {'plan_sha256': plan_sha256(plan), 'run_id': '123', 'approved_by': 'synthetic-human', 'approved_at': 1001, 'expires_at': 2000, 'workflow_sha': self.sha, 'execution_mode': 'apply'}
            envelope = sign_plan(plan, approval, Ed25519PrivateKey.generate())
            git = Mock()
            git.fetch.return_value = json.dumps(envelope).encode()
            transport = Mock()
            transport.call.side_effect = [{'plan_id': plan['plan_id'], 'status': 'completed'}, {'plan_id': plan['plan_id'], 'status': 'verified'}]
            inputs = {'plan_id': plan['plan_id'], 'approval_commit': 'b' * 40, 'approval_sha256': 'c' * 64, 'dry_run': False}
            def local_path(value):
                return root / value.lstrip('/') if str(value).startswith(('/private', '/secrets')) else Path(value)
            with patch.object(cli, 'Path', side_effect=local_path), patch.object(cli, 'load_plan', return_value=plan), patch.object(cli, 'Client', return_value=transport), patch.dict('os.environ', {'GITHUB_RUN_ID': '123'}):
                cli.execute(self.sha, 'enjoydarts/sifto-operations', inputs, git)
            self.assertEqual([call.args[1] for call in transport.call.call_args_list], ['apply', 'verify'])
            self.assertEqual(transport.call.call_args_list[0].args[2], {'workflow_sha': self.sha, 'envelope': envelope, 'dry_run': False})
            git.write.assert_not_called()

    def test_failed_synthetic_apply_is_never_retried(self):
        from maintenance.plan import sign_plan
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            (root / 'secrets').mkdir()
            (root / 'secrets/hosts.json').write_text(json.dumps([{'host_id': 'sifto-app', 'address': 'example.invalid', 'port': 22, 'role': 'application'}]))
            plan = build_plan(ready(), self.sha, now=1000)
            envelope = sign_plan(plan, {'plan_sha256': plan_sha256(plan), 'run_id': '123', 'approved_by': 'synthetic-human', 'approved_at': 1001, 'expires_at': 2000, 'workflow_sha': self.sha, 'execution_mode': 'apply'}, Ed25519PrivateKey.generate())
            git = Mock()
            git.fetch.return_value = json.dumps(envelope).encode()
            transport = Mock()
            transport.call.side_effect = MaintenanceError('ssh_failed')
            inputs = {'plan_id': plan['plan_id'], 'approval_commit': 'b' * 40, 'approval_sha256': 'c' * 64, 'dry_run': False}
            with patch.object(cli, 'Path', side_effect=lambda value: root / value.lstrip('/')), patch.object(cli, 'load_plan', return_value=plan), patch.object(cli, 'Client', return_value=transport), patch.dict('os.environ', {'GITHUB_RUN_ID': '123'}):
                with self.assertRaises(MaintenanceError):
                    cli.execute(self.sha, 'enjoydarts/sifto-operations', inputs, git)
            self.assertEqual(transport.call.call_count, 1)
            git.write.assert_not_called()

    def test_legacy_signed_plan_without_action_runs_post_apply_verify(self):
        from maintenance.plan import sign_plan
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            (root / 'secrets').mkdir()
            (root / 'secrets/hosts.json').write_text(json.dumps([{'host_id': 'sifto-app', 'address': 'example.invalid', 'port': 22, 'role': 'application'}]))
            plan = build_plan(ready(), self.sha, now=1000)
            del plan['action']
            envelope = sign_plan(plan, {'plan_sha256': plan_sha256(plan), 'run_id': '123', 'approved_by': 'synthetic-human', 'approved_at': 1001, 'expires_at': 2000, 'workflow_sha': self.sha, 'execution_mode': 'apply'}, Ed25519PrivateKey.generate())
            git = Mock()
            git.fetch.return_value = json.dumps(envelope).encode()
            transport = Mock()
            transport.call.side_effect = [{'plan_id': plan['plan_id'], 'status': 'completed'}, {'plan_id': plan['plan_id'], 'status': 'verified'}]
            inputs = {'plan_id': plan['plan_id'], 'approval_commit': 'b' * 40, 'approval_sha256': 'c' * 64, 'dry_run': False}
            with patch.object(cli, 'Path', side_effect=lambda value: root / value.lstrip('/')), patch.object(cli, 'load_plan', return_value=plan), patch.object(cli, 'Client', return_value=transport), patch.object(cli, 'summary') as summary, patch.dict('os.environ', {'GITHUB_RUN_ID': '123'}):
                cli.execute(self.sha, 'enjoydarts/sifto-operations', inputs, git)
            self.assertEqual([call.args[1] for call in transport.call.call_args_list], ['apply', 'verify'])
            self.assertEqual(transport.call.call_args_list[0].args[2]['envelope'], envelope)
            self.assertNotIn('action', envelope['plan'])
            self.assertIn('verified', summary.call_args.args[0])

    def test_capture_bounds_both_streams_without_cancelling_mutation(self):
        import sys
        code, stdout, stderr, capped = ssh.capture([sys.executable, '-c', 'import sys; sys.stdout.write("o"*2000); sys.stderr.write("e"*2000)'], limit=100, timeout=None)
        self.assertEqual(code, 0)
        self.assertEqual(len(stdout), 100)
        self.assertEqual(len(stderr), 100)
        self.assertTrue(capped)

    def test_signer_uses_actual_reviewer_and_executor_pin_with_separate_audit_head(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            (root / 'secrets').mkdir()
            private = Ed25519PrivateKey.generate()
            raw = private.private_bytes(serialization.Encoding.Raw, serialization.PrivateFormat.Raw, serialization.NoEncryption())
            (root / 'secrets/approval_key').write_text(base64.b64encode(raw).decode())
            plan = build_plan(ready(), self.sha, now=1000)
            history = [{'state': 'approved', 'user': {'login': 'enjoydarts'}, 'environments': [{'name': 'production-maintenance'}]}]
            git = Mock()
            git.head.return_value = 'b' * 40
            git.write.return_value = 'c' * 40
            inputs = {'private_commit': 'd' * 40, 'dry_run': True}
            def local_path(value):
                return root / value.lstrip('/') if str(value).startswith(('/private', '/secrets')) else Path(value)
            with patch.object(cli, 'Path', side_effect=local_path), patch.object(cli, 'load_plan', return_value=plan), patch.object(cli, 'github', return_value=history), patch.object(cli.time, 'time', return_value=1001), patch.object(cli, 'output') as outputs, patch.object(cli, 'summary'), patch.dict('os.environ', {'GITHUB_RUN_ID': '123', 'GITHUB_ACTOR': 'wrong-actor', 'MAINTENANCE_ALLOWED_APPROVERS': '["enjoydarts"]', 'MAINTENANCE_PIPELINE_SHA': 'e' * 40}):
                cli.authorize(self.sha, 'enjoydarts/sifto-operations', inputs, git)
            files = git.write.call_args.args[1]
            envelope = next(v for k, v in files.items() if k.startswith('approvals/'))
            self.assertEqual(envelope['approval']['approved_by'], 'enjoydarts')
            self.assertEqual(envelope['approval']['workflow_sha'], self.sha)
            self.assertEqual(envelope['approval']['execution_mode'], 'dry_run')
            self.assertLessEqual(envelope['approval']['expires_at'], plan['expires_at'])
            self.assertEqual(files['audit/approval-123.json']['pipeline_sha'], 'e' * 40)
            self.assertEqual(outputs.call_args_list[0].args, ('approval_commit', 'c' * 40))

    def test_execution_history_blocks_unresolved_and_unknown_but_root_diagnosis_preserves_consumption(self):
        self.assertTrue(hasattr(cli, 'execution_block'), 'receipt safety guard missing')
        for status in ('started', 'failed'):
            history = {'receipt_total': 1, 'receipt_truncated': 0, 'receipts': [{'status': status, 'started_at': 900}]}
            self.assertEqual(cli.execution_block(history, now=1000), {'status': 'blocked', 'reason': 'operator_diagnosis_required'})
            history['receipts'][0]['operator_diagnosed_at'] = 950
            self.assertIsNone(cli.execution_block(history, now=1000))
        for history in [{}, {'receipt_total': 21, 'receipt_truncated': 1, 'receipts': [{'status': 'verified'}] * 20}]:
            self.assertEqual(cli.execution_block(history, now=1000), {'status': 'unknown', 'reason': 'execution_history_incomplete'})
        aggregate = {'receipt_total': 21, 'receipt_truncated': 1, 'receipts': [{'status': 'verified'}] * 20,
                     'receipt_history_complete': True, 'unresolved_execution_count': 0}
        self.assertIsNone(cli.execution_block(aggregate, now=1000))
        aggregate['unresolved_execution_count'] = 1
        self.assertEqual(cli.execution_block(aggregate, now=1000)['status'], 'blocked')
        aggregate['receipt_history_complete'] = False
        self.assertEqual(cli.execution_block(aggregate, now=1000)['status'], 'unknown')
        history = {'receipt_total': 1, 'receipt_truncated': 0, 'receipts': [{'status': 'failed', 'operator_diagnosed_at': True}]}
        self.assertEqual(cli.execution_block(history, now=1000)['status'], 'blocked')

    def test_real_local_git_object_write_never_executes_downloaded_content(self):
        with tempfile.TemporaryDirectory() as directory:
            git = store.GitStore(Path(directory), 'enjoydarts/sifto-operations', '/secrets/synthetic-key', '/secrets/known_hosts', writer=True)
            tree = git.git('mktree', data=b'').decode().strip()
            parent = git.git('commit-tree', tree, data=b'Synthetic seed\n').decode().strip()
            original = git.git
            def without_network(*args, **kwargs):
                return b'' if args[0] == 'push' else original(*args, **kwargs)
            git.git = without_network
            marker = Path(directory) / 'should-not-exist'
            content = ('#!/bin/sh\ntouch ' + str(marker) + '\n').encode()
            commit = git.write(parent, {'plans/synthetic/content.py': content, 'ledger.json': {'synthetic': True}})
            self.assertEqual(git.read_at(commit, 'plans/synthetic/content.py'), content)
            self.assertFalse(marker.exists())
            self.assertEqual(original('config', 'core.hooksPath').strip(), b'/dev/null')
            git.writer = False
            with self.assertRaises(MaintenanceError):
                git.write(commit, {'ledger.json': {}})


if __name__ == "__main__":
    unittest.main()
