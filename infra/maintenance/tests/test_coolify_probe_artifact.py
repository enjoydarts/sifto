"""Frozen PUBLIC PHP artifact contracts; never execute PHP or read private input.

Real helper/block contracts run separately in the pinned offline ARM images.
This artifact alone does not grant native execution or recovery eligibility.
"""
import hashlib
import json
from pathlib import Path
import re
import unittest

from maintenance import coolify_evidence as evidence
from maintenance import coolify_plan


PROBE_SHA = '06081ecc1a56cdb7875b5469af320ba7356a9241e79e6509cdeedf2dff8d2a31'
CATALOG_SHA = 'b37aff8b86a8ff20372ef5fb7e9bba40ba7b192f899b649dce0c9d0daa61e276'
PACKAGE = Path(evidence.__file__).parent


class SourceArtifactContracts(unittest.TestCase):
    def setUp(self):
        artifact = PACKAGE / 'coolify_probe.php'
        self.assertTrue(artifact.is_file(), 'native frozen PUBLIC PHP artifact is missing')
        self.payload = artifact.read_bytes()
        self.source = self.payload.decode('utf-8')
        self.catalog = evidence.fixed_catalog()

    def section(self, start, end):
        self.assertIn(start, self.source)
        self.assertIn(end, self.source)
        return self.source.split(start, 1)[1].split(end, 1)[0]

    def test_full_artifact_bytes_bind_reviewed_public_origin(self):
        self.assertEqual(len(self.payload), 72970)
        self.assertEqual(hashlib.sha256(self.payload).hexdigest(), PROBE_SHA)

    def test_bundled_catalog_and_future_container_mount_are_bound(self):
        payload = (PACKAGE / 'coolify-public-manifest.json').read_bytes()
        self.assertEqual(len(payload), 80947)
        self.assertEqual(hashlib.sha256(payload).hexdigest(), CATALOG_SHA)
        self.assertEqual(evidence.MANIFEST_SHA, CATALOG_SHA)
        bootstrap = self.section("$stage = 'bootstrap';", "$stage = 'migrations';")
        self.assertIn("file_get_contents('/probe/public-manifest.json')", bootstrap)
        self.assertIn("hash('sha256', $manifestBytes) === '" + CATALOG_SHA + "'", bootstrap)
        self.assertIn("$hmacKey = readHmacKey('/probe/hmac.key');", bootstrap)
        self.assertNotIn("file_get_contents('/probe/hmac.key')", self.source)
        self.assertLess(bootstrap.index("readHmacKey('/probe/hmac.key')"),
                        bootstrap.index("require '/var/www/html/vendor/autoload.php';"))

    def test_hmac_identity_binds_all_security_metadata_and_excludes_atime(self):
        helper = self.section('function hmacStatIdentity(', 'function readHmacKey(')
        fields = re.findall(r"'([a-z]+)'", helper.split(' as $field)', 1)[0])
        self.assertEqual(fields, ['dev', 'ino', 'size', 'mode', 'nlink', 'uid',
                                  'gid', 'mtime', 'ctime'])
        self.assertIn('gate(array_key_exists($field, $info) && is_int($info[$field]))', helper)
        self.assertIn('$identity[$field] = $info[$field]', helper)
        self.assertIn('return $identity;', helper)

    def test_hmac_reader_requires_positive_posix_pair_and_exact_file_owner(self):
        helper = self.section('function readHmacKey(', 'function activityLogSourceProfile(')
        self.assertIn("gate(function_exists('posix_geteuid') && function_exists('posix_getegid'))", helper)
        self.assertIn('$uid = posix_geteuid(); $gid = posix_getegid();', helper)
        self.assertIn('gate(is_int($uid) && $uid > 0 && is_int($gid) && $gid > 0)', helper)
        self.assertIn("gate(($before['mode'] & 0170000) === 0100000 && ($before['mode'] & 07777) === 0600 && $before['size'] === 32 && $before['nlink'] === 1)", helper)
        owner_gate = "gate($before['uid'] === $uid && $before['gid'] === $gid);"
        self.assertIn(owner_gate, helper)
        self.assertLess(helper.index('$uid = posix_geteuid()'), helper.index('fopen('))
        self.assertLess(helper.index(owner_gate), helper.index('fopen('))

    def test_hmac_owner_wait_is_root_pair_only_and_bounded_before_any_key_io(self):
        helper = self.section('function readHmacKey(', 'function activityLogSourceProfile(')
        loop = "for ($attempt = 0; $before['uid'] === 0 && $before['gid'] === 0; $attempt++) {"
        self.assertIn(loop, helper)
        waiting = helper.split(loop, 1)[1].split("    gate($before['uid'] === $uid && $before['gid'] === $gid);", 1)[0]
        self.assertIn('gate($attempt < 8);', waiting)
        self.assertEqual(waiting.count('usleep('), 1)
        self.assertIn('usleep(250000);', waiting)
        self.assertLess(waiting.index('gate($attempt < 8);'), waiting.index('usleep('))
        self.assertEqual(helper.count('fopen('), 1)
        for forbidden in ('fopen(', 'fread(', 'file_get_contents(', 'chown(', 'chgrp(',
                          'chmod(', 'file_put_contents(', 'random_bytes(', 'require ', 'include '):
            self.assertNotIn(forbidden, waiting)
        self.assertLess(helper.index('gate(is_int($uid) && $uid > 0 && is_int($gid) && $gid > 0)'), helper.index(loop))
        self.assertLess(helper.index("gate(($before['mode'] & 0170000)"), helper.index(loop))
        self.assertLess(helper.index(loop), helper.index('fopen('))

    def test_hmac_owner_wait_rechecks_typed_canonical_nonowner_identity(self):
        helper = self.section('function readHmacKey(', 'function activityLogSourceProfile(')
        self.assertIn("$nonowner = $identity; unset($nonowner['uid'], $nonowner['gid']);", helper)
        waiting = helper.split('        usleep(250000);', 1)[1].split("    gate($before['uid'] === $uid && $before['gid'] === $gid);", 1)[0]
        markers = ['clearstatcache(true, $path);', '$before = lstat($path);',
                   'gate(is_array($before) && realpath($path) === $path);',
                   '$identity = hmacStatIdentity($before);',
                   "$observedNonowner = $identity; unset($observedNonowner['uid'], $observedNonowner['gid']);",
                   'gate($observedNonowner === $nonowner);']
        positions = [waiting.index(marker) for marker in markers]
        self.assertEqual(positions, sorted(positions))
        self.assertEqual(waiting.count('unset('), 1)
        self.assertNotIn('$nonowner =', waiting)

    def test_hmac_reader_binds_canonical_path_and_stable_descriptor_metadata(self):
        helper = self.section('function readHmacKey(', 'function activityLogSourceProfile(')
        self.assertEqual(helper.count('clearstatcache(true, $path)'), 3)
        self.assertIn('$before = lstat($path);', helper)
        self.assertIn('gate(is_array($before) && realpath($path) === $path)', helper)
        self.assertIn('$identity = hmacStatIdentity($before);', helper)
        self.assertIn('$opened = fstat($stream);', helper)
        self.assertIn('gate(is_array($opened) && hmacStatIdentity($opened) === $identity)', helper)
        self.assertIn('$after = fstat($stream);', helper)
        self.assertIn('gate(is_array($after) && hmacStatIdentity($after) === $identity)', helper)
        self.assertIn('$final = lstat($path);', helper)
        self.assertIn('gate(is_array($final) && hmacStatIdentity($final) === $identity && realpath($path) === $path)', helper)
        markers = ['$before = lstat(', "$stream = fopen($path, 'rb')",
                   '$opened = fstat(', '$payload = fread(', '$after = fstat(',
                   '$final = lstat(', 'return $payload;']
        positions = [helper.index(marker) for marker in markers]
        self.assertEqual(positions, sorted(positions))

    def test_hmac_reader_uses_bounded_fd_read_and_closes_before_bootstrap(self):
        helper = self.section('function readHmacKey(', 'function activityLogSourceProfile(')
        self.assertIn("$stream = fopen($path, 'rb');", helper)
        self.assertIn("gate(is_resource($stream) && get_resource_type($stream) === 'stream')", helper)
        self.assertEqual(helper.count('fread('), 1)
        self.assertIn('$payload = fread($stream, 33);', helper)
        self.assertIn('gate(is_string($payload) && strlen($payload) === 32)', helper)
        self.assertIn('} finally {\n        if (is_resource($stream)) { fclose($stream); }', helper)
        self.assertNotIn('file_get_contents(', helper)
        bootstrap = self.section("$stage = 'bootstrap';", "$stage = 'migrations';")
        self.assertLess(bootstrap.index("$hmacKey = readHmacKey('/probe/hmac.key');"),
                        bootstrap.index("require '/var/www/html/vendor/autoload.php';"))

    def test_public_profile_revisions_and_complete_migration_digests(self):
        profile = coolify_plan.fixed_profile()
        self.assertEqual(profile['bundle']['platform'], 'linux/arm64')
        for role, size in (('rollback', 435), ('target', 438)):
            selected = profile['bundle'][role]
            version = selected['version']
            self.assertEqual(self.catalog['revisions'][version], selected['release_commit'])
            migrations = self.catalog['migrations'][version]
            self.assertEqual(len(migrations), size)
            digest = hashlib.sha256(json.dumps(
                sorted(migrations), separators=(',', ':')).encode()).hexdigest()
            self.assertIn("'" + version + "' => '" + digest + "'", self.source)
        self.assertEqual(set(self.catalog['migrations']['4.4.6']) -
                         set(self.catalog['migrations']['4.4.2']), set(profile['new_migrations']))
        self.assertEqual(tuple(profile['new_migrations']), evidence.NEW_MIGRATIONS)
        self.assertFalse(profile['controlled_startup_verified'])
        self.assertFalse(coolify_plan.profile_eligibility()['eligible'])

    def test_model_source_inventory_is_authenticated_before_decryption(self):
        block = self.section("$stage = 'core_data';", "$stage = 'migration_prediction';")
        self.assertIn("hash_file('sha256', $path) === $sourceHash", block)
        self.assertIn("realpath($path) === $path", block)
        self.assertEqual(len(self.catalog['models']['4.4.2']), 69)
        self.assertEqual(len(evidence.DECRYPT_MODELS), 68)
        self.assertEqual(len(evidence.CORE), 59)
        decrypt = self.section("$stage = 'decryption';", "$stage = 'wrong_key';")
        self.assertIn("persistedEncryptedAttributes($name, $declaredCasts, $manifest['models'][$version][$name], $version, $manifest['migrations'][$version], $presence)", decrypt)
        self.assertIn("gate($attributes === $expectedCasts)", decrypt)

    def test_optional_casts_remain_exact_source_profile_guarded(self):
        guards = self.section('function correctedEncryptedCasts(', 'function predictedIpv6Changes(')
        for name in ('LocalFileVolume', 'InstanceSettings', 'Server'):
            for version in ('4.4.2', '4.4.6'):
                self.assertIn(self.catalog['models'][version][name], guards)
        self.assertIn("$presence[$attribute] === false", guards)
        self.assertIn("gate(is_bool($presence[$attribute]))", guards)
        self.assertIn("gate($presence[$attribute] === true)", guards)
        self.assertIn("$name === 'InstanceSettings' && $attribute === 'sentinel_token'", guards)
        self.assertIn("$name === 'Server' && in_array($attribute, ['logdrain_axiom_api_key', 'logdrain_newrelic_license_key'], true)", guards)

    def test_success_and_failure_schema_reach_output_boundary(self):
        result = self.section('    $result = [', '    while (ob_get_level()')
        keys = re.findall(r"'([a-z0-9_]+)' =>", result)
        self.assertEqual(set(keys), {'ok', 'category', 'version', 'db', 'no_pending',
                                    'network', 'wrong_key', 'ssh', 'decrypt',
                                    'decrypt_models', 'table_coverage', 'ipv6_affected',
                                    'migrations', 'core', 'deletions', 'seeder', 'ssh_checked'})
        self.assertEqual(len(keys), 17)
        failures = self.section('function probeFailureRecord(', "$stage = 'arguments';")
        keys = {'ok', 'category'} | set(re.findall(r"\$record\['([a-z_]+)'\] =", failures))
        self.assertEqual(keys, {'ok', 'category', 'decryption_diagnostics', 'core_checkpoint',
                                'table_inventory_diagnostics', 'table_inventory_classification',
                                'migration_diagnostics', 'migration_classification'})
        diagnostics = self.section('function decryptionDiagnostics(', 'function probeFailureRecord(')
        zero = diagnostics.split('$safe = [', 1)[1].split('];', 1)[0]
        self.assertEqual(set(re.findall(r"'([a-z_]+)' =>", zero)),
                         {'available', 'checkpoint', 'model_ordinal', 'checked',
                          'nonempty', 'missing', 'failures'})
        tail = self.source[self.source.rfind('} catch (Throwable $failure) {'):]
        self.assertIn('json_encode(probeFailureRecord(', tail)
        self.assertIn('ob_end_clean()', tail)
        self.assertNotIn('$failure->', self.source)
        self.assertIn('echo json_encode($result, JSON_THROW_ON_ERROR)', self.source)

    def test_wrong_key_control_and_runtime_abi_remain_present(self):
        tail = self.section("$stage = 'wrong_key';", '    $result = [')
        self.assertIn('new Illuminate\\Encryption\\Encrypter(random_bytes(32)', tail)
        self.assertIn('catch (Illuminate\\Contracts\\Encryption\\DecryptException $expectedFailure)', tail)
        self.assertIn('gate($wrongKey)', tail)
        self.assertIn('gate($sshGood && $sshChecked > 0)', tail)
        for endpoint in ("httpGate('/api/health', 8080, true)", "httpGate('/up', 6001)",
                         "httpGate('/login', 8080)"):
            self.assertIn(endpoint, tail)

    def test_artifact_imports_no_operator_identity_or_native_execution(self):
        for forbidden in ('/Users/', '/private/', 'cold/runtime', 'snapshot_id',
                          'backup_id', 'backup_sha256', 'MAC_UID', 'PLANTED_PRIVATE_MARKER',
                          'run-202610', 'maintenance-operator-recovery'):
            self.assertNotIn(forbidden, self.source)
        self.assertNotRegex(self.source, r'posix_geteuid\(\)\s*===?\s*\d+')
        for module in ('host.py', 'cli.py', 'coolify_host.py', 'coolify_rehearsal.py'):
            self.assertNotIn('coolify_probe.php', (PACKAGE / module).read_text())
        entrypoint = (PACKAGE.parent / 'host_agent.py').read_text()
        validation, runtime = entrypoint.split('sys.path.insert(0,', 1)
        self.assertIn('maintenance/coolify_probe.php', validation)
        self.assertNotIn('coolify_probe.php', runtime)
        for module in ('coolify_rehearsal', 'coolify_evidence'):
            self.assertNotRegex(runtime, rf'\b(?:from|import)\s+(?:maintenance\.)?{module}\b')


if __name__ == '__main__':
    unittest.main()
