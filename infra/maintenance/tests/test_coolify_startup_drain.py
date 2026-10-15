"""Finite clone observation is never native retirement/restoration authority."""
import json
import unittest
from maintenance import coolify_startup as startup


def receipt():
    return dict(version=1, operation='horizon-drain', selected_version='4.4.2',
        phase='quiescent', maintenance='held', force=False, restoration='required',
        pool_count=1, worker_object_count=1, exited_worker_count=1,
        live_worker_count=0, terminating_count=0, master_status='paused',
        supervisor_status='paused', run_hmac='a' * 64, original_hmac='b' * 64,
        membership_hmac='c' * 64, source_hmac='d' * 64, config_hmac='e' * 64)


class DrainObservationContract(unittest.TestCase):
    def parse(self, value):
        self.assertTrue(callable(getattr(startup, 'parse_horizon_drain', None)), 'drain parser missing')
        return startup.parse_horizon_drain(json.dumps(value).encode(), selected_version='4.4.2')

    def test_quiescent_is_finite_and_not_restored_or_authorizing(self):
        value = self.parse(receipt())
        for field in ('drained', 'verified', 'restored', 'native_eligible'):
            self.assertFalse(hasattr(value, field))

    def test_closed_types_counts_and_state(self):
        for field, bad in [('phase', 'complete'), ('restoration', 'complete'), ('force', True),
                ('live_worker_count', 1), ('exited_worker_count', 0), ('pool_count', 33),
                ('worker_object_count', True), ('terminating_count', 1), ('source_hmac', 'bad'),
                ('master_status', 'running'), ('supervisor_status', 'running')]:
            value = receipt(); value[field] = bad
            with self.subTest(field=field), self.assertRaises(ValueError):
                self.parse(value)

    def test_unknown_missing_duplicate_oversize(self):
        for field in receipt():
            value = receipt(); del value[field]
            with self.subTest(field=field), self.assertRaises(ValueError): self.parse(value)
        value = receipt(); value['drained'] = True
        with self.assertRaises(ValueError): self.parse(value)
        self.assertTrue(callable(getattr(startup, 'parse_horizon_drain', None)), 'drain parser missing')
        for raw in (b'{"phase":"quiescent","phase":"quiescent"}', b' ' * 8193):
            with self.assertRaises(ValueError): startup.parse_horizon_drain(raw, selected_version='4.4.2')
