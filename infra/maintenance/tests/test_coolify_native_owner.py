"""PUBLIC native admission fixtures: protected real files, no private material."""
import hashlib
import json
import os
from pathlib import Path
import stat
import tempfile
import unittest
from unittest.mock import patch

try:
    from maintenance import coolify_native_owner as owner
except ImportError:
    owner = None


def request():
    return {'version': 1, 'scope': 'synthetic-only', 'request_seq': 1,
            'nonce': 'a' * 32, 'binding': {'snapshot_id': 'b' * 64,
            'host_id': 'PUBLIC-fixture', 'proof_sha256': 'c' * 64,
            'file_sha256': {name: 'd' * 64 for name in
                ('coolify.dump', 'coolify-config.tar.gz', 'created_at')}},
            'material_sha256': hashlib.sha256(b'PUBLIC material').hexdigest(),
            'material_size': len(b'PUBLIC material')}


class NativeRequestTests(unittest.TestCase):
    def setUp(self):
        self.assertIsNotNone(owner, 'N4 source owner admission is missing')
        directory = tempfile.TemporaryDirectory(prefix='PUBLIC-native-owner-')
        self.addCleanup(directory.cleanup)
        self.root = Path(directory.name)
        self.slot, self.state = self.root / 'slot', self.root / 'state'
        self.slot.mkdir(mode=0o700)
        self.state.mkdir(mode=0o700)
        self.value = request()
        self.write_request()
        (self.slot / 'material.tar').write_bytes(b'PUBLIC material')
        (self.slot / 'material.tar').chmod(0o600)

    def write_request(self):
        (self.slot / 'request.json').write_text(json.dumps(self.value))
        (self.slot / 'request.json').chmod(0o600)

    def open(self):
        result = owner._open_request(self.slot, 'PUBLIC-fixture')
        self.addCleanup(result.close)
        return result

    def test_strict_request_retains_distinct_cloexec_readonly_descriptors(self):
        opened = self.open()
        self.assertEqual(opened.binding.host_id, 'PUBLIC-fixture')
        self.assertEqual(opened.sequence, 1)
        self.assertNotEqual(opened.request_fd, opened.material_fd)
        self.assertFalse(os.get_inheritable(opened.request_fd))
        self.assertFalse(os.get_inheritable(opened.material_fd))
        opened.guard()

    def test_public_entrypoint_factories_stay_dormant_before_request_or_claim_io(self):
        for name in ('run_controller', 'run_engine'):
            self.assertTrue(hasattr(owner, name), 'explicit dormant entrypoint factory missing')
            with patch.object(os, 'open', side_effect=AssertionError('PUBLIC unexpected IO')):
                with self.assertRaisesRegex(owner.OwnerError, '^source_closure$'):
                    getattr(owner, name)(())

    def test_controller_never_reads_or_hashes_material_body(self):
        original = os.pread
        def read(fd, *args):
            self.assertNotEqual(os.fstat(fd).st_ino, (self.slot / 'material.tar').stat().st_ino)
            return original(fd, *args)
        with patch.object(os, 'pread', side_effect=read):
            opened = self.open()
            opened.guard()

    def test_schema_rejects_unknown_fields_bool_integers_scope_host_and_binding(self):
        invalid = []
        for key, value in [('version', True), ('request_seq', True), ('request_seq', 0),
                ('request_seq', 2**63), ('material_size', True), ('material_size', 0),
                ('material_size', 160 * 1024**2 + 1), ('scope', 'production'),
                ('nonce', 'A' * 32), ('material_sha256', 'x' * 64),
                ('profile', '/PUBLIC/caller-profile')]:
            candidate = request()
            candidate[key] = value
            invalid.append(candidate)
        candidate = request()
        candidate['binding']['host_id'] = 'other'
        invalid.append(candidate)
        candidate = request()
        candidate['binding']['file_sha256']['runtime.env'] = 'd' * 64
        invalid.append(candidate)
        for value in invalid:
            with self.subTest(value=value):
                self.value = value
                self.write_request()
                with self.assertRaisesRegex(owner.OwnerError, '^request_invalid$'):
                    self.open()

    def test_duplicate_json_keys_and_unsafe_files_are_rejected(self):
        (self.slot / 'request.json').write_text('{"version":1,"version":1}')
        with self.assertRaises(owner.OwnerError):
            self.open()
        self.write_request()
        material = self.slot / 'material.tar'
        os.link(material, self.root / 'hardlink')
        with self.assertRaises(owner.OwnerError):
            self.open()
        (self.root / 'hardlink').unlink()
        material.chmod(0o640)
        with self.assertRaises(owner.OwnerError):
            self.open()

    def test_replaced_input_and_mode_drift_rejected_without_foreign_fd_close(self):
        opened = self.open()
        fd = opened.material_fd
        os.close(fd)
        foreign = os.open('/dev/null', os.O_RDONLY)
        self.assertEqual(fd, foreign)
        with self.assertRaises(owner.OwnerError):
            opened.guard()
        opened.close()
        os.fstat(foreign)
        os.close(foreign)

    def test_durable_claim_consumes_sequence_before_any_run_and_never_resumes(self):
        opened = self.open()
        claimed = owner._ClaimStore(self.state)
        self.addCleanup(claimed.close)
        claimed.claim(opened)
        rows = [json.loads(line) for line in (self.state / 'claims').read_bytes().splitlines()]
        self.assertEqual(rows[-1]['state'], 'claimed')
        self.assertEqual(rows[-1]['highwater'], 1)
        self.assertEqual(rows[-1]['nonce'], 'a' * 32)
        self.assertEqual(rows[-1]['request_sha256'], opened.digest)
        self.assertIn('material_identity', rows[-1])
        claimed.close()
        restarted = owner._ClaimStore(self.state)
        self.addCleanup(restarted.close)
        with self.assertRaisesRegex(owner.OwnerError, '^request_replayed$'):
            restarted.claim(opened)
        self.assertEqual(len((self.state / 'claims').read_bytes().splitlines()), 1)

    def test_claim_bound_and_finished_are_monotonic_and_replay_stays_consumed(self):
        opened = self.open()
        claimed = owner._ClaimStore(self.state)
        self.addCleanup(claimed.close)
        claimed.claim(opened)
        claimed.bind('e' * 32)
        claimed.finish()
        rows = [json.loads(line) for line in (self.state / 'claims').read_bytes().splitlines()]
        self.assertEqual([row['state'] for row in rows], ['claimed', 'bound-to-run', 'finished'])
        self.assertEqual({row['highwater'] for row in rows}, {1})
        with self.assertRaises(owner.OwnerError):
            claimed.bind('f' * 32)

    def test_truncated_or_mutated_claim_never_reopens_as_empty_highwater(self):
        (self.state / 'claims').write_bytes(b'{"highwater":1')
        (self.state / 'claims').chmod(0o600)
        with self.assertRaisesRegex(owner.OwnerError, '^claim_invalid$'):
            owner._ClaimStore(self.state)

    def test_persisted_claim_rejects_invalid_input_metadata_without_resetting_highwater(self):
        opened = self.open()
        claimed = owner._ClaimStore(self.state)
        claimed.claim(opened)
        claimed.close()
        path = self.state / 'claims'
        original = json.loads(path.read_bytes())
        for key, index, invalid in (('request_identity', 2, stat.S_IFREG | 0o666),
                ('material_identity', 5, 2), ('material_identity', 3, 65534),
                ('request_identity', 6, 16385), ('material_identity', 6, 160 * 1024**2 + 1)):
            with self.subTest(field=key, index=index):
                row = json.loads(json.dumps(original))
                row[key][index] = invalid
                path.write_text(json.dumps(row) + '\n')
                with self.assertRaisesRegex(owner.OwnerError, '^claim_invalid$'):
                    owner._ClaimStore(self.state)


if __name__ == '__main__':
    unittest.main()
