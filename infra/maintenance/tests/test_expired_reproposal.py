"""Fresh proposals retain immutable evidence and still need native approval."""
import copy
import json
import tempfile
import unittest
from pathlib import Path
from unittest.mock import Mock, patch

from maintenance import cli
from maintenance.plan import MaintenanceError, build_plan, canonical_json, plan_sha256
from test_controller import ready


class ExpiredReproposalTests(unittest.TestCase):
    def setUp(self):
        self.executor = 'a' * 40
        self.parent = 'b' * 40
        self.old = build_plan(ready(), self.executor, now=1000)
        self.boundary = self.old['expires_at'] + 86400
        self.fingerprint = cli.candidate_fingerprint(self.old)
        self.ledger = {self.fingerprint: {'plan_id': self.old['plan_id'], 'expires_at': self.old['expires_at']}}
        self.store = Mock()
        self.store.git.return_value = b''
        self.store.read_at.return_value = canonical_json(self.old)

    def reserve(self, now, *, force=False, ledger=None, candidate=None):
        candidate = candidate or build_plan(ready(), self.executor, now=now)
        return cli.reserve_candidate(self.ledger if ledger is None else ledger, candidate,
                                     self.store, self.parent, force=force, now=now)

    def test_unexpired_expired_and_one_second_before_cooldown_are_suppressed(self):
        before = copy.deepcopy(self.ledger)
        for now in (1001, self.old['expires_at'], self.boundary - 1):
            with self.subTest(now=now):
                self.assertFalse(self.reserve(now))
                self.assertEqual(self.ledger, before)

    def test_exact_boundary_reserves_new_uuid_once_then_suppresses_again(self):
        candidate = build_plan(ready(), self.executor, now=self.boundary)
        old_bytes = self.store.read_at.return_value
        self.assertTrue(self.reserve(self.boundary, candidate=candidate))
        self.assertNotEqual(candidate['plan_id'], self.old['plan_id'])
        self.assertEqual(self.ledger[self.fingerprint],
                         {'plan_id': candidate['plan_id'], 'expires_at': candidate['expires_at']})
        self.store.read_at.assert_called_once_with(self.parent, 'plans/' + self.old['plan_id'] + '/plan.json')
        self.assertEqual(self.store.read_at.return_value, old_bytes)
        self.store.read_at.return_value = canonical_json(candidate)
        self.assertFalse(self.reserve(self.boundary + 1))

    def test_force_bypasses_valid_deduplication_without_reusing_old_uuid(self):
        self.assertTrue(self.reserve(1001, force=True))
        self.assertNotEqual(self.ledger[self.fingerprint]['plan_id'], self.old['plan_id'])

    def test_every_ledger_record_is_validated_even_when_force_is_requested(self):
        bad_records = [None, [], {}, {'plan_id': self.old['plan_id']},
                       {'plan_id': self.old['plan_id'], 'expires_at': True},
                       {'plan_id': self.old['plan_id'], 'expires_at': 0},
                       {'plan_id': self.old['plan_id'], 'expires_at': '87400'},
                       {'plan_id': 'not-a-uuid', 'expires_at': 87400},
                       {'plan_id': 'AAAAAAAA-AAAA-4AAA-8AAA-AAAAAAAAAAAA', 'expires_at': 87400},
                       {**self.ledger[self.fingerprint], 'extra': 1}]
        for record in bad_records:
            with self.subTest(record=record):
                ledger = {**self.ledger, 'c' * 64: record}
                before = copy.deepcopy(ledger)
                with self.assertRaises(MaintenanceError):
                    self.reserve(self.boundary, force=True, ledger=ledger)
                self.assertEqual(ledger, before)
        for ledger in ([], {'invalid-fingerprint': self.ledger[self.fingerprint]},
                       {**self.ledger, 'c' * 64: self.ledger[self.fingerprint]}):
            with self.assertRaises(MaintenanceError):
                self.reserve(self.boundary, force=True, ledger=ledger)

    def test_missing_malformed_or_mismatched_immutable_prior_plan_denies_force(self):
        mismatch_id = copy.deepcopy(self.old)
        mismatch_id['plan_id'] = build_plan(ready(), self.executor, now=1000)['plan_id']
        mismatch_expiry = {**self.old, 'expires_at': self.old['expires_at'] - 1}
        mismatch_scope = {**self.old, 'workflow_sha': 'd' * 40}
        for raw in (None, b'', b'{}', b'not-json', canonical_json(mismatch_id),
                    canonical_json(mismatch_expiry), canonical_json(mismatch_scope)):
            with self.subTest(raw=raw):
                self.store.read_at.return_value = raw
                before = copy.deepcopy(self.ledger)
                with self.assertRaises(MaintenanceError):
                    self.reserve(self.boundary, force=True)
                self.assertEqual(self.ledger, before)

    def test_future_prior_plan_and_noncurrent_candidate_are_refused(self):
        future = build_plan(ready(), self.executor, now=1002)
        self.ledger[self.fingerprint] = {'plan_id': future['plan_id'], 'expires_at': future['expires_at']}
        self.store.read_at.return_value = canonical_json(future)
        with self.assertRaises(MaintenanceError):
            self.reserve(1001, force=True)
        for candidate, now in ((self.old, 999), (self.old, self.old['expires_at'])):
            with self.assertRaises(MaintenanceError):
                self.reserve(now, candidate=candidate, force=True, ledger={})

    def test_uuid_reuse_in_ledger_or_immutable_plan_storage_is_refused(self):
        candidate = build_plan(ready(), self.executor, now=self.boundary)
        candidate['plan_id'] = self.old['plan_id']
        with self.assertRaises(MaintenanceError):
            self.reserve(self.boundary, candidate=candidate, force=True)
        candidate = build_plan(ready(), self.executor, now=self.boundary)
        path = 'plans/' + candidate['plan_id'] + '/plan.json'
        self.store.git.return_value = (path + '\n').encode()
        with self.assertRaises(MaintenanceError):
            self.reserve(self.boundary, candidate=candidate, ledger={}, force=True)

    def test_missing_ledger_is_first_boot_but_empty_or_malformed_present_blob_is_not(self):
        self.assertEqual(cli.read_ledger(self.store, self.parent), {})
        self.store.read_at.assert_not_called()
        self.store.git.return_value = b'ledger.json\n'
        for raw in (None, b'', b'not-json', b'[]', b'null', b'{"bad":{}}'):
            with self.subTest(raw=raw):
                self.store.read_at.return_value = raw
                with self.assertRaises(MaintenanceError):
                    cli.read_ledger(self.store, self.parent)
        self.store.read_at.return_value = b'{}'
        self.assertEqual(cli.read_ledger(self.store, self.parent), {})

    def run_scan(self, *, history=None, force=False, conflict=False):
        if history is None:
            history = {'host_id': ready()['host_id'], 'receipt_total': 0,
                       'receipt_truncated': 0, 'receipts': [],
                       'receipt_history_complete': True, 'unresolved_execution_count': 0}
        objects = {'ledger.json': canonical_json(self.ledger),
                   'plans/' + self.old['plan_id'] + '/plan.json': canonical_json(self.old),
                   'approvals/' + self.old['plan_id'] + '/123.json': b'immutable-old-approval'}
        preserved = copy.deepcopy(objects)
        git = Mock()
        git.head.return_value = self.parent
        git.git.side_effect = lambda *args: b'ledger.json\n' if args[-1] == 'ledger.json' else b''
        git.read_at.side_effect = lambda parent, path, **kwargs: objects.get(path)
        events = []

        def write(parent, files):
            self.assertEqual(parent, self.parent)
            events.append('CAS-write')
            if conflict:
                raise MaintenanceError('private_git_failed')
            objects.update({path: value if isinstance(value, bytes) else canonical_json(value)
                            for path, value in files.items()})
            return 'e' * 40

        git.write.side_effect = write
        transport = Mock()
        transport.call.side_effect = [ready(), history]
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            (root / 'secrets').mkdir()
            (root / 'secrets/hosts.json').write_text(json.dumps([
                {'host_id': ready()['host_id'], 'address': 'example.invalid', 'port': 22, 'role': 'application'}]))

            def local_path(value):
                return root / value.lstrip('/') if str(value).startswith(('/private', '/secrets')) else Path(value)

            with patch.object(cli, 'Path', side_effect=local_path), patch.object(cli, 'Client', return_value=transport), \
                    patch.object(cli.time, 'time', return_value=self.boundary), \
                    patch.object(cli, 'ai_advice', return_value={'status': 'unavailable', 'warning_ja': 'fixture'}), \
                    patch.object(cli, 'github', side_effect=lambda *args, **kwargs: events.append('dispatch')) as dispatch:
                # AI's fixture input is public, not a provider credential.
                (root / 'secrets/anthropic_key').write_text('')
                if conflict:
                    with self.assertRaises(MaintenanceError):
                        cli.scan(self.executor, 'enjoydarts/sifto-operations',
                                 {'force_request': force, 'dry_run': False}, git)
                else:
                    cli.scan(self.executor, 'enjoydarts/sifto-operations',
                             {'force_request': force, 'dry_run': False}, git)
        for path, value in preserved.items():
            if path != 'ledger.json':
                self.assertEqual(objects[path], value)
        return git, dispatch, objects, events

    def test_fresh_scan_writes_new_plan_and_ledger_before_dispatch(self):
        git, dispatch, objects, events = self.run_scan()
        self.assertEqual(events, ['CAS-write', 'dispatch'])
        self.assertEqual(dispatch.call_count, 1)
        row = json.loads(objects['ledger.json'])[self.fingerprint]
        self.assertNotEqual(row['plan_id'], self.old['plan_id'])
        self.assertEqual(row['expires_at'], self.boundary + 86400)
        plan = json.loads(objects['plans/' + row['plan_id'] + '/plan.json'])
        self.assertEqual(plan['created_at'], self.boundary)
        self.assertEqual(plan['state_sha256'], self.old['state_sha256'])

    def test_incomplete_or_unresolved_root_history_blocks_even_force(self):
        for history in ({'receipt_history_complete': False},
                        {'receipt_total': 1, 'receipts': [{'status': 'started', 'started_at': 1000}],
                         'unresolved_execution_count': 1},
                        {'receipt_total': 1, 'receipts': [{'status': 'failed', 'failed_at': 1000}],
                         'unresolved_execution_count': 1}):
            with self.subTest(history=history):
                complete = {'host_id': ready()['host_id'], 'receipt_total': 0, 'receipt_truncated': 0,
                            'receipts': [], 'receipt_history_complete': True, 'unresolved_execution_count': 0}
                complete.update(history)
                _, dispatch, objects, _ = self.run_scan(history=complete, force=True)
                dispatch.assert_not_called()
                self.assertEqual(json.loads(objects['ledger.json']), self.ledger)

    def test_failed_cas_never_dispatches_or_overwrites_old_evidence(self):
        _, dispatch, objects, events = self.run_scan(conflict=True)
        self.assertEqual(events, ['CAS-write'])
        dispatch.assert_not_called()
        self.assertEqual(json.loads(objects['ledger.json']), self.ledger)

    def test_reproposed_plan_still_requires_fresh_native_owner_approval(self):
        _, _, objects, _ = self.run_scan()
        row = json.loads(objects['ledger.json'])[self.fingerprint]
        path = 'plans/' + row['plan_id'] + '/plan.json'
        plan = json.loads(objects[path])
        git = Mock()
        git.fetch.return_value = objects[path]
        inputs = {'private_commit': 'e' * 40, 'plan_id': plan['plan_id'],
                  'plan_sha256': plan_sha256(plan), 'dry_run': False}
        with patch.object(cli.time, 'time', return_value=self.boundary + 1), \
                patch.object(cli, 'github', return_value=[]), patch.object(cli, 'sign_plan') as signer, \
                patch.dict('os.environ', {'GITHUB_RUN_ID': '456', 'MAINTENANCE_ALLOWED_APPROVERS': '["enjoydarts"]'}):
            with self.assertRaises(MaintenanceError):
                cli.authorize(self.executor, 'enjoydarts/sifto-operations', inputs, git)
        signer.assert_not_called()
        git.write.assert_not_called()


if __name__ == '__main__':
    unittest.main()
