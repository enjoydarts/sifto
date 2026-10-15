"""Closed, non-authorizing actual-supervisor inspection receipts."""
from dataclasses import FrozenInstanceError
import json
import unittest

from maintenance import coolify_startup as startup


def receipt():
    return dict(version=1, operation='horizon-inspect', selected_version='4.4.2',
        phase='complete', supervisor_status='running', master_status='running',
        pool_count=1, worker_object_count=1, live_worker_count=1,
        terminating_count=0, maintenance='held', force=False,
        run_hmac='a' * 64, supervisor_hmac='b' * 64, master_hmac='c' * 64,
        source_hmac='d' * 64, config_hmac='e' * 64, membership_hmac='f' * 64)


class SupervisorInspectionContract(unittest.TestCase):
    def parse(self, value):
        self.assertTrue(callable(getattr(startup, 'parse_horizon_inspection', None)),
                        'inspection parser missing')
        return startup.parse_horizon_inspection(json.dumps(value).encode(), selected_version='4.4.2')

    def test_frozen_receipt_has_no_drain_or_execution_authority(self):
        value = self.parse(receipt())
        self.assertFalse(hasattr(value, 'drained'))
        self.assertFalse(hasattr(value, 'verified'))
        with self.assertRaises(FrozenInstanceError):
            value.force = True

    def test_missing_extra_duplicate_and_oversized_rejected(self):
        for field in receipt():
            value = receipt()
            del value[field]
            with self.subTest(field=field), self.assertRaisesRegex(ValueError, '^startup_horizon_inspection$'):
                self.parse(value)
        value = receipt()
        value['idle'] = True
        with self.assertRaisesRegex(ValueError, '^startup_horizon_inspection$'):
            self.parse(value)
        for raw in (b'{"phase":"complete","phase":"complete"}', b' ' * 8193):
            with self.assertRaisesRegex(ValueError, '^startup_horizon_inspection$'):
                startup.parse_horizon_inspection(raw, selected_version='4.4.2')

    def test_closed_types_counts_and_non_authorizing_states(self):
        for field, bad in [('version', True), ('force', True), ('maintenance', 'released'),
                ('phase', 'drained'), ('live_worker_count', True), ('live_worker_count', 2),
                ('worker_object_count', 0), ('pool_count', 0), ('terminating_count', 1),
                ('supervisor_status', 'idle'), ('source_hmac', 'raw-source'),
                ('selected_version', '4.4.6')]:
            value = receipt()
            value[field] = bad
            with self.subTest(field=field, bad=bad), self.assertRaisesRegex(ValueError, '^startup_horizon_inspection$'):
                self.parse(value)

    def test_pool_bound_matches_receiver(self):
        value = receipt()
        value.update(pool_count=33, live_worker_count=33, worker_object_count=33)
        with self.assertRaisesRegex(ValueError, '^startup_horizon_inspection$'):
            self.parse(value)
