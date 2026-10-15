"""PUBLIC source-owned SSH observations never grant startup authority."""
from dataclasses import FrozenInstanceError
import hashlib
import json
from pathlib import Path
import unittest

from maintenance import coolify_startup as startup


def observation():
    return dict(version=1, operation='ssh-reconstruct', selected_version='4.4.2',
                rows_checked=2, keys_created=1, keys_matched=1,
                unrelated_key_count=1, mux_entry_count=1,
                **{name: 'a' * 64 for name in (
                    'db_identity_hmac', 'existing_keys_hmac_before',
                    'existing_keys_hmac_after', 'mux_hmac_before',
                    'mux_hmac_after', 'created_inventory_hmac')})


class StartupSshContracts(unittest.TestCase):
    def parse(self, value):
        return startup.parse_ssh_result(json.dumps(value).encode(), selected_version='4.4.2')

    def test_complete_observation_is_detached_and_typed(self):
        source = observation()
        parsed = self.parse(source)
        self.assertEqual(parsed.rows_checked, 2)
        self.assertEqual(parsed.keys_created, 1)
        self.assertEqual(parsed.keys_matched, 1)
        self.assertFalse(hasattr(parsed, 'verified'))
        with self.assertRaises(FrozenInstanceError):
            parsed.keys_created = 0
        source['keys_created'] = 0
        self.assertEqual(parsed.keys_created, 1)

    def test_rejects_non_exact_schema_and_sensitive_fields(self):
        for name in observation():
            value = observation()
            del value[name]
            with self.subTest(name=name), self.assertRaisesRegex(ValueError, '^startup_ssh_result$'):
                self.parse(value)
        value = observation()
        value['private_key'] = 'PUBLIC MUST NOT ECHO'
        with self.assertRaisesRegex(ValueError, '^startup_ssh_result$'):
            self.parse(value)

    def test_counts_are_bounded_integers_and_reconcile(self):
        for name in ('rows_checked', 'keys_created', 'keys_matched', 'unrelated_key_count', 'mux_entry_count'):
            for bad in (True, -1, 1025, '1', None, 0.0):
                value = observation()
                value[name] = bad
                with self.subTest(name=name, bad=bad), self.assertRaisesRegex(ValueError, '^startup_ssh_result$'):
                    self.parse(value)
        value = observation()
        value['keys_created'] = 0
        with self.assertRaisesRegex(ValueError, '^startup_ssh_result$'):
            self.parse(value)

    def test_hmacs_and_preserved_inventory_are_exact(self):
        for name in (key for key in observation() if 'hmac' in key):
            for bad in ('A' * 64, 'a' * 63, 123, None):
                value = observation()
                value[name] = bad
                with self.subTest(name=name, bad=bad), self.assertRaisesRegex(ValueError, '^startup_ssh_result$'):
                    self.parse(value)
        for name in ('existing_keys_hmac_after', 'mux_hmac_after'):
            value = observation()
            value[name] = 'b' * 64
            with self.assertRaisesRegex(ValueError, '^startup_ssh_result$'):
                self.parse(value)

    def test_version_and_operation_are_selected_literals(self):
        for name, bad in (('version', True), ('version', 2), ('operation', 'verify'),
                          ('selected_version', '4.4.6'), ('selected_version', '4.4.2\n')):
            value = observation()
            value[name] = bad
            with self.subTest(name=name), self.assertRaisesRegex(ValueError, '^startup_ssh_result$'):
                self.parse(value)
        with self.assertRaisesRegex(ValueError, '^startup_ssh_result$'):
            startup.parse_ssh_result(json.dumps(observation()).encode(), selected_version='unknown')

    def test_duplicate_keys_malformed_and_oversize_are_rejected(self):
        for raw in (b'{}', b'null', b'[]', b'\xff', b' ' * 8193, b'{"version":1,"version":1}',
                    json.dumps(observation()), json.dumps(observation()).encode() + b'{}'):
            with self.subTest(raw_type=type(raw)), self.assertRaisesRegex(ValueError, '^startup_ssh_result$'):
                startup.parse_ssh_result(raw, selected_version='4.4.2')

    def test_php_resource_is_bound_without_enabling_startup(self):
        package = Path(startup.__file__).parent
        resources = dict(startup.RESOURCE_SHA256)
        self.assertIn('coolify_startup.php', resources)
        self.assertEqual(hashlib.sha256((package / 'coolify_startup.php').read_bytes()).hexdigest(),
                         resources['coolify_startup.php'])
        self.assertFalse(startup.fixed_candidate()['registered'])
        self.assertFalse(startup.fixed_candidate()['production_eligible'])
