"""Authorization append safety with deterministic private-storage failures."""
import base64
import json
import tempfile
import unittest
from pathlib import Path
from unittest.mock import Mock, patch

from cryptography.hazmat.primitives import serialization
from cryptography.hazmat.primitives.asymmetric.ed25519 import Ed25519PrivateKey

from maintenance import cli
from maintenance.plan import MaintenanceError, build_plan, canonical_json
from test_controller import ready


class AuthorizationAppendTests(unittest.TestCase):
    def setUp(self):
        directory = tempfile.TemporaryDirectory()
        self.addCleanup(directory.cleanup)
        self.root = Path(directory.name)
        (self.root / 'secrets').mkdir()
        key = Ed25519PrivateKey.generate().private_bytes(serialization.Encoding.Raw,
            serialization.PrivateFormat.Raw, serialization.NoEncryption())
        (self.root / 'secrets/approval_key').write_bytes(base64.b64encode(key))
        self.executor = 'a' * 40
        self.plan = build_plan(ready(), self.executor, now=1000)
        self.inputs = {'private_commit': 'b' * 40, 'dry_run': False}
        self.git = Mock()
        self.git.head.return_value = 'c' * 40
        self.git.git.return_value = b''
        self.git.read_at.return_value = None
        self.git.write.return_value = 'd' * 40
        self.paths = ['approvals/' + self.plan['plan_id'] + '/123.json', 'audit/approval-123.json']
        history = [{'state': 'approved', 'user': {'login': 'fixture-owner'},
                    'environments': [{'name': 'production-maintenance'}]}]
        environment = {'GITHUB_RUN_ID': '123', 'MAINTENANCE_ALLOWED_APPROVERS': '["fixture-owner"]',
                       'MAINTENANCE_PIPELINE_SHA': 'e' * 40}
        patches = [patch.object(cli, 'Path', side_effect=lambda value: self.root / str(value).lstrip('/')),
                   patch.object(cli, 'load_plan', return_value=self.plan),
                   patch.object(cli, 'github', return_value=history),
                   patch.object(cli.time, 'time', return_value=1001),
                   patch.object(cli, 'sign_plan', wraps=cli.sign_plan),
                   patch.object(cli, 'output'), patch.object(cli, 'summary'),
                   patch.object(cli, 'Client'), patch.object(cli, 'execute'),
                   patch.dict('os.environ', environment)]
        mocks = [p.start() for p in patches]
        for p in patches:
            self.addCleanup(p.stop)
        self.history, self.signer, self.outputs, self.transport, self.executor_call = mocks[2], mocks[4], mocks[5], mocks[7], mocks[8]

    def authorize(self):
        return cli.authorize(self.executor, 'synthetic/fixture', self.inputs, self.git)

    def assert_authorized(self):
        try:
            self.authorize()
        except MaintenanceError as error:
            self.fail('expected durable authorization, got ' + str(error))

    def assert_sign_once_without_execution(self):
        self.history.assert_called_once()
        self.signer.assert_called_once()
        self.transport.assert_not_called()
        self.executor_call.assert_not_called()

    def test_changed_parent_retries_identical_signed_bytes(self):
        self.git.head.side_effect = ['c' * 40, 'f' * 40]
        self.git.write.side_effect = [MaintenanceError('private_git_failed'), 'd' * 40]
        self.assert_authorized()
        self.assertEqual(self.git.write.call_count, 2)
        first, second = [call.args for call in self.git.write.call_args_list]
        self.assertEqual([first[0], second[0]], ['c' * 40, 'f' * 40])
        self.assertEqual(first[1], second[1])
        self.assertTrue(all(isinstance(raw, bytes) for raw in first[1].values()))
        envelope = json.loads(first[1][self.paths[0]])
        self.assertEqual(envelope['plan'], self.plan)
        self.assertEqual(envelope['approval']['execution_mode'], 'apply')
        self.assertEqual(envelope['approval']['approved_by'], 'fixture-owner')
        self.assert_sign_once_without_execution()

    def test_same_parent_failure_stops_without_second_write(self):
        self.git.write.side_effect = MaintenanceError('private_git_failed')
        with self.assertRaisesRegex(MaintenanceError, 'authorization_write_failed_without_new_parent'):
            self.authorize()
        self.assertEqual(self.git.write.call_count, 1)
        self.outputs.assert_not_called()
        self.assert_sign_once_without_execution()

    def test_three_changed_parent_failures_exhaust_bound(self):
        self.git.head.side_effect = ['c' * 40, 'f' * 40, '1' * 40, '2' * 40]
        self.git.write.side_effect = MaintenanceError('private_git_failed')
        with self.assertRaisesRegex(MaintenanceError, 'authorization_write_retries_exhausted'):
            self.authorize()
        self.assertEqual(self.git.write.call_count, 3)
        self.outputs.assert_not_called()
        self.assert_sign_once_without_execution()

    def test_final_ambiguous_push_is_reconciled_without_fourth_write(self):
        persisted = {}
        def failed_response(parent, files):
            if self.git.write.call_count == 3:
                persisted.update(files)
            raise MaintenanceError('private_git_failed')
        self.git.write.side_effect = failed_response
        self.git.head.side_effect = ['c' * 40, 'f' * 40, '1' * 40, '2' * 40]
        self.git.git.side_effect = [b'', b'', b'', ('\n'.join(self.paths) + '\n').encode()]
        self.git.read_at.side_effect = lambda parent, path, **kwargs: persisted[path]
        self.assert_authorized()
        self.assertEqual(self.git.write.call_count, 3)
        self.assertEqual(self.outputs.call_args_list[0].args, ('approval_commit', '2' * 40))
        self.assert_sign_once_without_execution()

    def test_ambiguous_success_reconciles_exact_both_records(self):
        persisted = {}
        def committed_but_response_lost(parent, files):
            persisted.update(files)
            raise MaintenanceError('private_git_failed')
        self.git.write.side_effect = committed_but_response_lost
        self.git.head.side_effect = ['c' * 40, 'f' * 40]
        self.git.git.side_effect = [b'', ('\n'.join(self.paths) + '\n').encode()]
        self.git.read_at.side_effect = lambda parent, path, **kwargs: persisted[path]
        self.assert_authorized()
        self.assertEqual(self.git.write.call_count, 1)
        self.assertEqual(self.outputs.call_args_list[0].args, ('approval_commit', 'f' * 40))
        import hashlib
        self.assertEqual(self.outputs.call_args_list[1].args,
                         ('approval_sha256', hashlib.sha256(persisted[self.paths[0]]).hexdigest()))
        self.assert_sign_once_without_execution()

    def test_partial_existing_records_are_not_overwritten(self):
        for path in self.paths:
            with self.subTest(path=path):
                self.git.git.return_value = (path + '\n').encode()
                with self.assertRaisesRegex(MaintenanceError, 'immutable_authorization_conflict'):
                    self.authorize()
                self.git.write.assert_not_called()
                self.outputs.assert_not_called()

    def test_differing_or_empty_existing_records_are_not_overwritten(self):
        self.git.git.return_value = ('\n'.join(self.paths) + '\n').encode()
        for raw in (b'', b'{}', canonical_json({'signature': 'different'})):
            with self.subTest(raw=raw):
                self.git.read_at.return_value = raw
                with self.assertRaisesRegex(MaintenanceError, 'immutable_authorization_conflict'):
                    self.authorize()
                self.git.write.assert_not_called()
                self.outputs.assert_not_called()

    def test_head_transport_failure_is_not_retried(self):
        self.git.head.side_effect = MaintenanceError('private_git_failed')
        with self.assertRaisesRegex(MaintenanceError, 'private_git_failed'):
            self.authorize()
        self.git.head.assert_called_once()
        self.git.write.assert_not_called()
        self.outputs.assert_not_called()
        self.assert_sign_once_without_execution()


if __name__ == '__main__':
    unittest.main()
