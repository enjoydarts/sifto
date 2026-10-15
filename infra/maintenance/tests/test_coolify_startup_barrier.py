"""Barrier data is exact, immutable observation; never startup authority."""
from dataclasses import FrozenInstanceError
import json
import unittest

from maintenance import coolify_startup as startup


def observation(operation='barrier-observe'):
    return dict(version=1, operation=operation, selected_version='4.4.2',
                driver='cache', store='redis', connection='cache', database=1,
                logical_key='illuminate:foundation:down', key_state='absent',
                ttl_state='absent', payload_profile='absent',
                maintenance_file_state='absent', down_file_state='absent',
                snapshot_state='absent',
                **{name: 'a' * 64 for name in ('redis_identity_hmac', 'cache_prefix_hmac',
                    'redis_prefix_hmac', 'original_state_hmac', 'current_state_hmac')})


class StartupBarrierContracts(unittest.TestCase):
    def parse(self, value, operation='barrier-observe'):
        self.assertTrue(callable(getattr(startup, 'parse_barrier_result', None)), 'barrier parser is missing')
        return startup.parse_barrier_result(json.dumps(value).encode(),
            selected_version='4.4.2', operation=operation)

    def test_closed_frozen_observation_has_no_execution_authority(self):
        value = self.parse(observation())
        self.assertEqual(value.database, 1)
        self.assertFalse(hasattr(value, 'verified'))
        with self.assertRaises(FrozenInstanceError):
            value.database = 0

    def test_exact_schema_and_bound_literals(self):
        for field in observation():
            value = observation()
            del value[field]
            with self.subTest(field=field), self.assertRaisesRegex(ValueError, '^startup_barrier_result$'):
                self.parse(value)
        for field, bad in [('version', True), ('database', True), ('database', 0),
                ('driver', 'file'), ('store', 'array'), ('connection', 'default'),
                ('logical_key', 'queues:default'), ('selected_version', '4.4.6'),
                ('operation', 'barrier-enter'), ('payload_profile', 'secret'),
                ('snapshot_state', 'verified'), ('key_state', 'list')]:
            value = observation()
            value[field] = bad
            with self.subTest(field=field), self.assertRaisesRegex(ValueError, '^startup_barrier_result$'):
                self.parse(value)
        value = observation()
        value['secret'] = 'never expose'
        with self.assertRaisesRegex(ValueError, '^startup_barrier_result$'):
            self.parse(value)

    def test_cross_field_state_is_consistent(self):
        for field, bad in [('ttl_state', 'persistent'), ('payload_profile', 'plain503')]:
            value = observation()
            value[field] = bad
            with self.assertRaisesRegex(ValueError, '^startup_barrier_result$'):
                self.parse(value)
        for operation in ('barrier-enter', 'barrier-restore'):
            with self.assertRaisesRegex(ValueError, '^startup_barrier_result$'):
                self.parse(observation(operation), operation)
        value = observation('barrier-enter')
        value.update(key_state='persistent', ttl_state='persistent', payload_profile='plain503', snapshot_state='held')
        self.assertEqual(self.parse(value, 'barrier-enter').snapshot_state, 'held')
        value = observation('barrier-restore')
        value['snapshot_state'] = 'restored'
        self.assertEqual(self.parse(value, 'barrier-restore').snapshot_state, 'restored')

    def test_observer_accepts_closed_non_authorizing_state_categories(self):
        for state, profile in [('persistent', 'original'), ('expiring', 'plain503'), ('unknown', 'unexpected')]:
            value = observation()
            value.update(key_state=state, ttl_state=state, payload_profile=profile)
            self.assertEqual(self.parse(value).key_state, state)
        for field in ('maintenance_file_state', 'down_file_state', 'snapshot_state'):
            value = observation()
            value[field] = True
            with self.assertRaisesRegex(ValueError, '^startup_barrier_result$'):
                self.parse(value)

    def test_preparing_is_observation_only_and_cannot_be_successful_enter(self):
        value = observation()
        value['snapshot_state'] = 'preparing'
        try:
            parsed = self.parse(value)
        except ValueError:
            self.fail('preparing observation is unsupported')
        self.assertEqual(parsed.snapshot_state, 'preparing')
        value.update(operation='barrier-enter', key_state='persistent', ttl_state='persistent', payload_profile='plain503')
        with self.assertRaisesRegex(ValueError, '^startup_barrier_result$'):
            self.parse(value, 'barrier-enter')

    def test_hmacs_are_exact_lowercase_and_no_unknown_authority(self):
        for field in (key for key in observation() if 'hmac' in key):
            for bad in (False, None, 'A' * 64, 'a' * 63, [], 12):
                value = observation()
                value[field] = bad
                with self.subTest(field=field), self.assertRaisesRegex(ValueError, '^startup_barrier_result$'):
                    self.parse(value)

    def test_invalid_wire_format_and_duplicate_keys_are_rejected(self):
        for raw in (b'{}', b'[]', b'null', b'\xff', b' ' * 8193,
                    b'{"version":1,"version":1}', json.dumps(observation()),
                    json.dumps(observation()).encode() + b'{}'):
            with self.assertRaisesRegex(ValueError, '^startup_barrier_result$'):
                startup.parse_barrier_result(raw, selected_version='4.4.2', operation='barrier-observe')
        for operation in ('final-success', 'barrier-restore-final', True, None):
            with self.assertRaisesRegex(ValueError, '^startup_barrier_result$'):
                startup.parse_barrier_result(json.dumps(observation()).encode(),
                    selected_version='4.4.2', operation=operation)
