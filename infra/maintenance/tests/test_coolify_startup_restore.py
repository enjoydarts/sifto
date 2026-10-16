"""Exact finite restoration observation carries no native/barrier authority."""
import json
import unittest
from maintenance import coolify_startup as startup


def receipt():
    return dict(version=1, operation='horizon-restore', selected_version='4.4.2',
        phase='restored', maintenance='held', force=False, pool_count=2,
        worker_object_count=4, ready_worker_count=4, terminating_count=0,
        master_status='running', supervisor_status='paused', run_hmac='a' * 64,
        original_hmac='b' * 64, readiness_hmac='c' * 64, source_hmac='d' * 64, config_hmac='e' * 64)


class RestorationObservationContract(unittest.TestCase):
    def parse(self, value):
        self.assertTrue(callable(getattr(startup, 'parse_horizon_restore', None)), 'restore parser missing')
        return startup.parse_horizon_restore(json.dumps(value).encode(), selected_version='4.4.2')

    def test_transition_observation_never_grants_release_or_native_authority(self):
        parsed = self.parse(receipt())
        self.assertEqual(parsed.ready_worker_count, 4)
        for field in ('verified', 'native_eligible', 'release_barrier', 'permanent'):
            self.assertFalse(hasattr(parsed, field))

    def test_exact_counts_and_closed_states(self):
        for field, bad in [('phase', 'quiescent'), ('maintenance', 'released'), ('force', True),
                ('pool_count', 33), ('pool_count', 5), ('worker_object_count', True),
                ('ready_worker_count', 3), ('terminating_count', 1), ('source_hmac', 'bad'),
                ('master_status', 'unknown'), ('supervisor_status', 'mixed')]:
            value = receipt(); value[field] = bad
            with self.subTest(field=field), self.assertRaises(ValueError): self.parse(value)

    def test_missing_extra_duplicate_and_bounded_json(self):
        for field in receipt():
            value = receipt(); del value[field]
            with self.subTest(field=field), self.assertRaises(ValueError): self.parse(value)
        value = receipt(); value['release_barrier'] = True
        with self.assertRaises(ValueError): self.parse(value)
        self.assertTrue(callable(getattr(startup, 'parse_horizon_restore', None)), 'restore parser missing')
        for raw in (b'{"phase":"restored","phase":"restored"}', b' ' * 8193):
            with self.assertRaises(ValueError): startup.parse_horizon_restore(raw, selected_version='4.4.2')
