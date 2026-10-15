"""Typed passive S6 observations cannot grant startup or signal authority."""
from dataclasses import FrozenInstanceError
import json
import unittest

from maintenance import coolify_startup as startup


NAMES = ('fix-attrs', 'legacy-cont-init', 'db-migration', 'init-seeder',
         'init-script', 'legacy-services', 'php-fpm', 'nginx', 'reverb',
         'terminal-server', 'horizon', 'scheduler-worker')


def observation():
    services = []
    for index, name in enumerate(NAMES):
        oneshot = index < 6
        services.append(dict(name=name, kind='oneshot' if oneshot else 'longrun',
            state='active' if oneshot else 'up', readiness='unsupported',
            role='none' if oneshot else ('disabled-sleep' if index >= 10 else name),
            pid=0 if oneshot else index + 100, start_time_ticks=0 if oneshot else 100,
            identity_hmac=None if oneshot else 'a' * 64))
    return dict(version=1, operation='runtime-observe', selected_version='4.4.2',
        horizon_enabled=False, scheduler_enabled=False, services=services,
        membership_state='stable', owned_process_count=20, unclassified_count=0,
        source_hmac='a' * 64, config_hmac='a' * 64,
        membership_hmac_before='a' * 64, membership_hmac_after='a' * 64)


class RuntimeObservationContracts(unittest.TestCase):
    def parse(self, value):
        self.assertTrue(callable(getattr(startup, 'parse_runtime_result', None)),
                        'runtime parser is missing')
        return startup.parse_runtime_result(json.dumps(value).encode(), selected_version='4.4.2')

    def test_exact_frozen_nested_observations_have_no_authority(self):
        value = self.parse(observation())
        self.assertIsInstance(value.services, tuple)
        self.assertFalse(hasattr(value, 'verified'))
        self.assertFalse(hasattr(value, 'drained'))
        with self.assertRaises(FrozenInstanceError):
            value.horizon_enabled = True
        with self.assertRaises(FrozenInstanceError):
            value.services[0].state = 'ready'

    def test_closed_schema_types_enums_and_size(self):
        for field in observation():
            value = observation()
            del value[field]
            with self.subTest(field=field), self.assertRaisesRegex(ValueError, '^startup_runtime_result$'):
                self.parse(value)
        for field, bad in [('version', True), ('owned_process_count', True),
                ('unclassified_count', -1), ('membership_state', 'drained'),
                ('horizon_enabled', 'false'), ('scheduler_enabled', 0),
                ('selected_version', '4.4.6'), ('operation', 'runtime-drain'),
                ('source_hmac', 'A' * 64), ('config_hmac', None), ('extra', True)]:
            value = observation()
            value[field] = bad
            with self.subTest(field=field), self.assertRaisesRegex(ValueError, '^startup_runtime_result$'):
                self.parse(value)
        for raw in (b'{}', b'[]', b'\xff', b' ' * 8193,
                    b'{"version":1,"version":1}', json.dumps(observation()),
                    json.dumps(observation()).encode() + b'{}'):
            with self.subTest(raw=repr(raw)[:40]), self.assertRaisesRegex(ValueError, '^startup_runtime_result$'):
                startup.parse_runtime_result(raw, selected_version='4.4.2')

    def test_service_set_is_exact_and_ordered_without_duplicates(self):
        for mutate in (lambda rows: rows.pop(), lambda rows: rows.reverse(),
                       lambda rows: rows.append(rows[0]),
                       lambda rows: rows[0].update(name='custom')):
            value = observation()
            mutate(value['services'])
            with self.assertRaisesRegex(ValueError, '^startup_runtime_result$'):
                self.parse(value)
        for field, bad in [('pid', True), ('pid', -1), ('start_time_ticks', True),
                ('kind', 'daemon'), ('state', 'ready'), ('role', 'worker'),
                ('readiness', True), ('identity_hmac', 'raw argv'), ('extra', 1)]:
            value = observation()
            value['services'][6][field] = bad
            with self.subTest(field=field), self.assertRaisesRegex(ValueError, '^startup_runtime_result$'):
                self.parse(value)

    def test_notification_readiness_and_process_identity_are_not_inferred(self):
        for index in range(12):
            value = observation()
            value['services'][index]['readiness'] = 'notified'
            if index in (6, 7):
                self.assertEqual(self.parse(value).services[index].readiness, 'notified')
            else:
                with self.subTest(index=index), self.assertRaisesRegex(ValueError, '^startup_runtime_result$'):
                    self.parse(value)
        for changes in (dict(pid=0), dict(start_time_ticks=0), dict(identity_hmac=None),
                        dict(role='unknown', readiness='notified'),
                        dict(state='unstable', readiness='notified')):
            value = observation()
            value['services'][6].update(changes)
            with self.subTest(changes=changes), self.assertRaisesRegex(ValueError, '^startup_runtime_result$'):
                self.parse(value)

    def test_sleep_is_disabled_and_membership_changes_remain_honest(self):
        value = observation()
        value['horizon_enabled'] = True
        with self.assertRaisesRegex(ValueError, '^startup_runtime_result$'):
            self.parse(value)
        value['services'][10]['role'] = 'horizon'
        self.assertTrue(self.parse(value).horizon_enabled)
        for changes in (dict(unclassified_count=21), dict(unclassified_count=1),
                        dict(membership_hmac_after='b' * 64)):
            value = observation()
            value.update(changes)
            with self.subTest(changes=changes), self.assertRaisesRegex(ValueError, '^startup_runtime_result$'):
                self.parse(value)
        value = observation()
        value.update(membership_state='unknown', unclassified_count=1)
        self.assertEqual(self.parse(value).membership_state, 'unknown')
        value.update(membership_state='changed', membership_hmac_after='b' * 64)
        self.assertEqual(self.parse(value).membership_state, 'changed')

    def test_owned_count_cannot_undercount_six_distinct_service_processes(self):
        value = observation()
        value['owned_process_count'] = 5
        with self.assertRaisesRegex(ValueError, '^startup_runtime_result$'):
            self.parse(value)
        value['owned_process_count'] = 6
        self.assertEqual(self.parse(value).owned_process_count, 6)

    def test_nonactive_oneshots_and_missing_longruns_are_not_ready(self):
        value = observation()
        value['services'][0]['state'] = 'not-active'
        value['services'][6].update(state='missing', role='none', pid=0,
                                   start_time_ticks=0, identity_hmac=None, readiness='unknown')
        self.assertEqual(self.parse(value).services[0].state, 'not-active')
        value['services'][6]['readiness'] = 'notified'
        with self.assertRaisesRegex(ValueError, '^startup_runtime_result$'):
            self.parse(value)
