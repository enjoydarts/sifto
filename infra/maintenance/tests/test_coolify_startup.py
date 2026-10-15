"""PUBLIC controlled-startup contracts; never grant startup authority."""
from pathlib import Path
import hashlib
import os
import subprocess
import tempfile
import unittest


PACKAGE = Path(__file__).resolve().parents[1] / 'maintenance'


class ControlledStartupResources(unittest.TestCase):
    def test_fixed_guards_and_readonly_nginx_boot_resources_exist(self):
        for name in ('coolify_startup_init', 'coolify_startup_middleware.php',
                     'coolify_startup_nginx.conf', 'coolify_startup_site.conf'):
            with self.subTest(name=name):
                self.assertTrue((PACKAGE / name).is_file(),
                                'fixed reviewed startup resource is missing')

    def test_cache_and_automatic_boot_controls_are_safe_in_cloned_environment(self):
        from maintenance.coolify_rehearsal import clone_env, parse_env
        source = (b'APP_NAME=Coolify\nAPP_KEY=base64:' + b'QUFBQUFBQUFBQUFBQUFBQUFBQUFBQUFBQUFBQUFBQUE=\n'
                  b'HORIZON_ENABLED=true\nSCHEDULER_ENABLED=true\nMIGRATION_ENABLED=true\nSEEDER_ENABLED=true\n')
        cloned = clone_env(source)
        values = parse_env(cloned)
        expected = {'HORIZON_ENABLED': 'false', 'SCHEDULER_ENABLED': 'false',
                    'MIGRATION_ENABLED': 'false', 'SEEDER_ENABLED': 'false',
                    'AUTOUPDATE': 'false', 'APP_DEBUG': 'false',
                    'DISABLE_DEFAULT_CONFIG': 'true', 'AUTORUN_ENABLED': 'false',
                    'SSL_MODE': 'off', 'APP_MAINTENANCE_DRIVER': 'cache',
                    'APP_MAINTENANCE_STORE': 'redis'}
        for key, expected_value in expected.items():
            with self.subTest(key=key):
                self.assertEqual(values.get(key), expected_value)
                if key in ('HORIZON_ENABLED', 'SCHEDULER_ENABLED'):
                    self.assertIn((key + '=' + expected_value + '\n').encode(), cloned)

    def test_source_closure_covers_all_startup_resources_before_import(self):
        from maintenance import coolify_docker
        expected = ('coolify_startup.php', 'coolify_startup.py', 'coolify_startup_init',
                    'coolify_startup_middleware.php', 'coolify_startup_nginx.conf',
                    'coolify_startup_site.conf')
        root = PACKAGE.parent
        for name in expected:
            with self.subTest(name=name):
                self.assertIn('maintenance/' + name, coolify_docker._SOURCE_NAMES)
                for entrypoint in ('host_agent.py', 'coolify_native_controller.py', 'coolify_native_engine.py'):
                    self.assertIn('maintenance/' + name, (root / entrypoint).read_text())

    def test_fixed_resources_match_catalog_digest(self):
        import importlib.util
        self.assertIsNotNone(importlib.util.find_spec('maintenance.coolify_startup'),
                             'fixed startup catalog is missing')
        from maintenance import coolify_startup
        self.assertEqual(len(coolify_startup.RESOURCE_SHA256), 5)
        for name, digest in coolify_startup.RESOURCE_SHA256:
            with self.subTest(name=name):
                self.assertEqual(hashlib.sha256((PACKAGE / name).read_bytes()).hexdigest(), digest)

    def test_candidate_metadata_and_caller_records_cannot_enable_startup(self):
        from maintenance import coolify_startup, coolify_docker, coolify_plan
        from types import SimpleNamespace
        candidate = coolify_startup.fixed_candidate()
        self.assertEqual(candidate['profile_sha256'], coolify_plan.profile_sha256())
        self.assertFalse(candidate['registered'])
        self.assertFalse(candidate['production_eligible'])
        candidate['registered'] = True
        self.assertFalse(coolify_startup.fixed_candidate()['registered'])
        for scope in ('synthetic-task4', 'rehearsal-only', 'production', True):
            owned = object.__new__(coolify_docker.OwnedDocker)
            owned.context = SimpleNamespace(startup=coolify_docker.ControlledStartup(
                coolify_plan.profile_sha256(), 'Task4A-public-candidate', scope))
            with self.subTest(scope=scope), self.assertRaisesRegex(
                    coolify_docker.DockerError, '^startup_profile_unverified$'):
                owned._require_verified_startup()
        self.assertFalse(coolify_plan.fixed_profile()['controlled_startup_verified'])
        self.assertFalse(coolify_plan.profile_eligibility()['eligible'])

    def test_public_image_source_fixture_is_bound_to_same_original_hashes(self):
        from maintenance import coolify_startup
        source = (PACKAGE.parent / 'startup/fixture.php').read_text()
        for path, digest in coolify_startup.ORIGINAL_SHA256:
            self.assertIn(path, source)
            self.assertIn(digest, source)

    def test_fixture_rejects_each_existing_resource_and_query_error_before_mutation(self):
        for defect in ('container', 'volume', 'network', 'ps-error', 'volume-error', 'network-error'):
            with self.subTest(defect=defect), tempfile.TemporaryDirectory() as selected:
                root = Path(selected)
                executable = root / 'fake-docker.sh'
                log = root / 'calls'
                executable.write_text('''#!/bin/sh
set -eu
printf '%s\\n' "$*" >> "$SIFTO_STARTUP_EMPTY_TEST_LOG"
case "$1:$SIFTO_STARTUP_EMPTY_TEST_CASE" in
  ps:container|volume:volume|network:network) printf 'PUBLIC-existing-resource\\n' ;;
  ps:ps-error|volume:volume-error|network:network-error) exit 83 ;;
  ps:*|volume:*|network:*) : ;;
  *) exit 90 ;;
esac
''')
                # /tmp stays noexec. Interpret the fake CLI through a shell
                # function while sourcing the unchanged fixture entrypoint.
                harness = ('docker() { /bin/sh "$SIFTO_STARTUP_EMPTY_FAKE_CLI" "$@"; }; '
                    'mktemp() { printf "mktemp\\n" >> "$SIFTO_STARTUP_EMPTY_TEST_LOG"; return 91; }; . "$0"')
                result = subprocess.run(['sh', '-c', harness, str(PACKAGE.parent / 'startup/test.sh')],
                    env={'PATH': os.defpath, 'TMPDIR': str(root),
                         'SIFTO_STARTUP_EMPTY_FAKE_CLI': str(executable),
                         'SIFTO_STARTUP_EMPTY_TEST_LOG': str(log),
                         'SIFTO_STARTUP_EMPTY_TEST_CASE': defect},
                    stdout=subprocess.PIPE, stderr=subprocess.PIPE, timeout=10)
                self.assertEqual(result.returncode, 1)
                self.assertEqual(result.stderr, b'startup fixture requires an empty owned namespace\n')
                calls = [line.split(' ', 1)[0] for line in log.read_text().splitlines()]
                self.assertNotIn('compose', calls)
                self.assertNotIn('image', calls)
                self.assertNotIn('mktemp', calls)
                self.assertEqual(sorted(path.name for path in root.iterdir()), ['calls', 'fake-docker.sh'])


if __name__ == '__main__':
    unittest.main()
