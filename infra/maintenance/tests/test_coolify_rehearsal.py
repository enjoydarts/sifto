"""Synthetic-only recovery material contracts; no image or real backup reads."""
import base64
import copy
import gzip
import hashlib
import io
import json
import os
from pathlib import Path
import stat
import tarfile
import tempfile
import unittest
from unittest.mock import patch

from maintenance import coolify_rehearsal as recovery

MARKER = 'SYNTHETIC_SECRET_DO_NOT_PRINT'
NOW = 1791946200
KEY = 'base64:' + base64.b64encode(bytes(range(32))).decode()
ENV = ('APP_KEY=' + KEY + '\nAPP_NAME=Coolify\nAPP_ID=fixture\n'
       'DATABASE_URL=postgres://private.invalid\n').encode()


def archive(entries, compressed=False):
    stream = io.BytesIO()
    with tarfile.open(fileobj=stream, mode='w', format=tarfile.USTAR_FORMAT) as handle:
        for name, kind, data in entries:
            member = tarfile.TarInfo(name)
            member.mode = 0o777
            member.type = kind
            member.size = len(data) if kind == tarfile.REGTYPE else 0
            if kind in (tarfile.SYMTYPE, tarfile.LNKTYPE):
                member.linkname = '/private/' + MARKER
            handle.addfile(member, io.BytesIO(data) if kind == tarfile.REGTYPE else None)
    raw = stream.getvalue()
    return gzip.compress(raw) if compressed else raw


def entries():
    return [(root, tarfile.DIRTYPE, b'') for root in ('source', 'ssh', 'ssl', 'proxy')] + [
        ('source/.env', tarfile.REGTYPE, ENV),
        ('ssh/keys/synthetic-key', tarfile.REGTYPE, MARKER.encode()),
        ('ssl/cert', tarfile.REGTYPE, b'certificate'),
        ('proxy/config', tarfile.REGTYPE, b'not-executed')]


def bundle_fixture(proof_edit=None, created=None, config=None):
    files = {'coolify.dump': b'PGDMP-synthetic',
             'coolify-config.tar.gz': archive(entries(), True) if config is None else config,
             'created_at': str(NOW - 60).encode() if created is None else created}
    hashes = {name: hashlib.sha256(data).hexdigest() for name, data in files.items()}
    proof = {'backup_id': 'a' * 64, 'host_id': 'application', 'verified_at': NOW - 30,
             'offhost_verified_at': NOW - 30, 'restore_tested_at': NOW - 30,
             'offhost_object': 'snapshots/' + MARKER, 'offhost_sha256': hashes['coolify.dump'],
             'retrieved_file_sha256': hashes, 'image_id': 'sha256:' + 'b' * 64,
             'offhost_verified': True, 'test_location': 'retrieved-snapshot-network-none',
             'coolify_config_operator_restore_required': True}
    if proof_edit:
        proof_edit(proof)
    encoded = json.dumps(proof).encode()
    binding = recovery.RootBinding(snapshot_id='a' * 64, host_id='application',
                                   file_sha256=tuple(sorted(hashes.items())),
                                   proof_sha256=hashlib.sha256(encoded).hexdigest())
    raw = archive([(name, tarfile.REGTYPE, data) for name, data in files.items()] +
                  [('proof.json', tarfile.REGTYPE, encoded)])
    return raw, binding


class RehearsalTests(unittest.TestCase):
    def assert_reject(self, function, *args, **kwargs):
        with self.assertRaises(recovery.RehearsalError) as caught:
            function(*args, **kwargs)
        self.assertNotIn(MARKER, str(caught.exception))
        self.assertRegex(str(caught.exception), r'^[a-z_]+$')

    def test_valid_bundle_requires_trusted_snapshot_file_and_proof_binding(self):
        raw, binding = bundle_fixture()
        material = recovery.validate_bundle(raw, binding=binding, now=NOW)
        self.assertEqual(dict(material.files)['coolify.dump'], b'PGDMP-synthetic')
        self.assertEqual(material.snapshot_id, 'a' * 64)
        self.assertNotIn(MARKER, repr(material))

    def test_altered_outer_file_and_proof_rejected(self):
        raw, binding = bundle_fixture()
        self.assert_reject(recovery.validate_bundle, raw.replace(b'PGDMP', b'XGDMP'),
                           binding=binding, now=NOW)
        changed = recovery.RootBinding(binding.snapshot_id, binding.host_id,
                                       binding.file_sha256, 'f' * 64)
        self.assert_reject(recovery.validate_bundle, raw, binding=changed, now=NOW)

    def test_proof_exact_shape_types_flags_locations_and_snapshot(self):
        cases = [lambda p: p.update(backup_id='c' * 64),
                 lambda p: p.update(host_id='other'),
                 lambda p: p.update(offhost_verified=1),
                 lambda p: p.update(coolify_config_operator_restore_required=False),
                 lambda p: p.update(test_location='operator-local'),
                 lambda p: p.update(offhost_sha256='e' * 64),
                 lambda p: p.update(offhost_object=MARKER * 1000),
                 lambda p: p.update(extra=MARKER),
                 lambda p: p.pop('image_id')]
        for change in cases:
            with self.subTest(case=cases.index(change)):
                raw, binding = bundle_fixture(change)
                self.assert_reject(recovery.validate_bundle, raw, binding=binding, now=NOW)

    def test_all_proof_timestamps_must_be_fresh_integers(self):
        for field in ('verified_at', 'offhost_verified_at', 'restore_tested_at'):
            for value in (True, str(NOW), NOW + 1, NOW - 86400, -1):
                with self.subTest(field=field, value=value):
                    raw, binding = bundle_fixture(lambda p: p.update({field: value}))
                    self.assert_reject(recovery.validate_bundle, raw, binding=binding, now=NOW)

    def test_created_at_ascii_integer_fresh_and_before_proof(self):
        for value in (b'', b'1\n2', b'123.0', b'+123', b'\xff', str(NOW + 1).encode(),
                      str(NOW - 86400).encode(), str(NOW - 1).encode(), b'9' * 100):
            with self.subTest(value=value):
                raw, binding = bundle_fixture(created=value)
                self.assert_reject(recovery.validate_bundle, raw, binding=binding, now=NOW)

    def test_duplicate_json_nonfinite_and_deep_proof_rejected(self):
        for raw in (b'{"a":1,"a":2}', b'{"value":NaN}', b'[' * 100 + b'0' + b']' * 100,
                    ('{"bad":"' + MARKER).encode()):
            self.assert_reject(recovery.strict_proof, raw)

    def test_outer_requires_exact_four_regular_files(self):
        for bad in ([('proof.json', tarfile.REGTYPE, b'{}')],
                    [('coolify.dump', tarfile.DIRTYPE, b'')],
                    [('unexpected-' + MARKER, tarfile.REGTYPE, b'x')]):
            self.assert_reject(recovery.inspect_archive, archive(bad), recovery=False)

    def test_recovery_requires_four_explicit_roots_and_nonempty_env(self):
        for bad in (entries()[1:], entries()[:4],
                    entries()[:4] + [('source/.env', tarfile.REGTYPE, b'')]):
            self.assert_reject(recovery.inspect_archive, archive(bad, True), recovery=True)

    def test_path_and_unsupported_types_rejected(self):
        for name in ('/source/a', 'source/../a', 'source//a', 'source/./a', 'source\\a',
                     'unknown/a', 'source/a//'):
            self.assert_reject(recovery.inspect_archive,
                               archive(entries() + [(name, tarfile.REGTYPE, b'x')], True), recovery=True)
        for kind in (tarfile.SYMTYPE, tarfile.LNKTYPE, tarfile.FIFOTYPE, tarfile.CHRTYPE,
                     tarfile.BLKTYPE, tarfile.GNUTYPE_SPARSE, tarfile.XHDTYPE):
            self.assert_reject(recovery.inspect_archive,
                               archive(entries() + [('source/bad', kind, b'')], True), recovery=True)

    def test_directory_header_does_not_silently_normalize_extra_slashes(self):
        self.assert_reject(recovery.inspect_archive,
                           archive(entries() + [('source/new//', tarfile.DIRTYPE, b'')], True),
                           recovery=True)

    def test_untrusted_forged_archive_is_rejected_before_mkdir(self):
        with tempfile.TemporaryDirectory() as temporary:
            parent = Path(temporary)
            policy = recovery.FilePolicy(parent, os.getuid())
            for values in (None, (), ((MARKER, False, b'x'),),
                           (('source', True, b'not-empty'),)):
                forged = recovery.ValidatedArchive(values, True)
                self.assert_reject(recovery.extract_validated, forged, parent / 'stage', policy=policy)
                self.assertFalse((parent / 'stage').exists())

    def test_forged_archive_missing_explicit_root_rejected_before_output(self):
        valid = recovery.inspect_archive(archive(entries(), True), recovery=True)
        forged = recovery.ValidatedArchive(tuple(entry for entry in valid.entries
                                                 if entry[0] != 'source'), True)
        with tempfile.TemporaryDirectory() as temporary:
            parent = Path(temporary)
            self.assert_reject(recovery.extract_validated, forged, parent / 'stage',
                               policy=recovery.FilePolicy(parent, os.getuid()))
            self.assertFalse((parent / 'stage').exists())

    def test_forged_unsafe_env_rejected_before_output(self):
        valid = recovery.inspect_archive(archive(entries(), True), recovery=True)
        for env in (b'APP_NAME=$(forbidden)\nAPP_KEY=' + KEY.encode() + b'\n',
                    b'APP_NAME=Coolify\nAPP_KEY=invalid\n'):
            forged = recovery.ValidatedArchive(tuple((name, directory, env if name == 'source/.env'
                                                       else data)
                                                      for name, directory, data in valid.entries), True)
            with tempfile.TemporaryDirectory() as temporary:
                parent = Path(temporary)
                self.assert_reject(recovery.extract_validated, forged, parent / 'stage',
                                   policy=recovery.FilePolicy(parent, os.getuid()))
                self.assertFalse((parent / 'stage').exists())

    def test_forged_nonboolean_recovery_scope_rejected_before_output(self):
        valid = recovery.inspect_archive(archive(entries(), True), recovery=True)
        with tempfile.TemporaryDirectory() as temporary:
            parent = Path(temporary)
            self.assert_reject(recovery.extract_validated,
                               recovery.ValidatedArchive(valid.entries, 1), parent / 'stage',
                               policy=recovery.FilePolicy(parent, os.getuid()))
            self.assertFalse((parent / 'stage').exists())

    def test_forged_surrogate_path_has_static_failure_before_output(self):
        valid = recovery.inspect_archive(archive(entries(), True), recovery=True)
        forged = recovery.ValidatedArchive(valid.entries + (('source/\ud800', False, b'x'),), True)
        with tempfile.TemporaryDirectory() as temporary:
            parent = Path(temporary)
            self.assert_reject(recovery.extract_validated, forged, parent / 'stage',
                               policy=recovery.FilePolicy(parent, os.getuid()))
            self.assertFalse((parent / 'stage').exists())

    def test_parent_permissions_and_owner_are_enforced(self):
        with tempfile.TemporaryDirectory() as temporary:
            root = Path(temporary)
            (root / 'input').write_bytes(b'fixture')
            (root / 'input').chmod(0o600)
            root.chmod(0o777)
            self.assert_reject(recovery.read_regular, root / 'input',
                               policy=recovery.FilePolicy(root, os.getuid()))
            root.chmod(0o700)
            self.assert_reject(recovery.read_regular, root / 'input',
                               policy=recovery.FilePolicy(root, os.getuid() + 1))

    def test_extraction_midwrite_collision_preserves_stranger(self):
        validated = recovery.inspect_archive(archive(entries(), True), recovery=True)
        with tempfile.TemporaryDirectory() as temporary:
            parent = Path(temporary)
            original_mkdir = os.mkdir
            def insert_stranger(name, mode=0o777, *, dir_fd=None):
                original_mkdir(name, mode, dir_fd=dir_fd)
                if name == 'source':
                    (parent / 'stage/source/.env').write_bytes(b'stranger-unchanged')
            with patch.object(recovery.os, 'mkdir', side_effect=insert_stranger):
                self.assert_reject(recovery.extract_validated, validated, parent / 'stage',
                                   policy=recovery.FilePolicy(parent, os.getuid()))
            self.assertEqual((parent / 'stage/source/.env').read_bytes(), b'stranger-unchanged')

    def test_duplicates_and_file_directory_collisions_both_orders(self):
        for pair in ([('source/.env', tarfile.REGTYPE, b'x')],
                     [('source/part', tarfile.REGTYPE, b'x'), ('source/part/child', tarfile.REGTYPE, b'x')],
                     [('source/part/child', tarfile.REGTYPE, b'x'), ('source/part', tarfile.REGTYPE, b'x')],
                     [('source/part', tarfile.DIRTYPE, b''), ('source/part', tarfile.REGTYPE, b'x')]):
            self.assert_reject(recovery.inspect_archive, archive(entries() + pair, True), recovery=True)

    def test_tar_and_gzip_completion_and_padding(self):
        valid = archive(entries())
        with tarfile.open(fileobj=io.BytesIO(valid)) as parsed:
            last = list(parsed)[-1]
            end = last.offset_data + ((last.size + 511) // 512) * 512
        for bad in (valid[:end], valid[:end + 512], valid[:-1], valid + b'x',
                    valid[:end] + b'X' + valid[end + 1:]):
            self.assert_reject(recovery.inspect_archive, bad, recovery=True)
        compressed = gzip.compress(valid)
        corrupt = bytearray(compressed)
        corrupt[-8] ^= 1
        for bad in (bytes(corrupt), compressed[:-1], compressed + gzip.compress(valid)):
            self.assert_reject(recovery.inspect_archive, bad, recovery=True)

    def test_file_member_and_total_expansion_limits(self):
        raw = archive(entries(), True)
        with patch.object(recovery, 'MAX_FILE', 5):
            self.assert_reject(recovery.inspect_archive, raw, recovery=True)
        with patch.object(recovery, 'MAX_EXPANDED', 20):
            self.assert_reject(recovery.inspect_archive, raw, recovery=True)
        with patch.object(recovery, 'MAX_MEMBERS', 5):
            self.assert_reject(recovery.inspect_archive, raw, recovery=True)

    def test_implied_directory_budget_checked_before_creation(self):
        raw = archive(entries() + [('source/a/b/c/d/file', tarfile.REGTYPE, b'x')], True)
        with tempfile.TemporaryDirectory() as temporary:
            stage = Path(temporary) / 'stage'
            with patch.object(recovery, 'MAX_DIRECTORIES', 5):
                self.assert_reject(recovery.inspect_archive, raw, recovery=True)
            self.assertFalse(stage.exists())

    def test_read_regular_protected_stable_metadata(self):
        with tempfile.TemporaryDirectory() as temporary:
            root = Path(temporary)
            source = root / 'input'
            source.write_bytes(b'fixture')
            source.chmod(0o600)
            policy = recovery.FilePolicy(root, os.getuid())
            self.assertEqual(recovery.read_regular(source, policy=policy), b'fixture')
            source.chmod(0o644)
            self.assert_reject(recovery.read_regular, source, policy=policy)
            source.chmod(0o600)
            os.link(source, root / 'other')
            self.assert_reject(recovery.read_regular, source, policy=policy)

    def test_read_refuses_empty_symlink_fifo_ancestor_escape_and_change(self):
        with tempfile.TemporaryDirectory() as temporary:
            root = Path(temporary)
            policy = recovery.FilePolicy(root, os.getuid())
            source = root / 'input'
            source.write_bytes(b'')
            source.chmod(0o600)
            self.assert_reject(recovery.read_regular, source, policy=policy)
            source.write_bytes(b'fixture')
            (root / 'alias').symlink_to(source)
            self.assert_reject(recovery.read_regular, root / 'alias', policy=policy)
            os.mkfifo(root / 'fifo', 0o600)
            self.assert_reject(recovery.read_regular, root / 'fifo', policy=policy)
            (root / 'link').symlink_to(root, target_is_directory=True)
            self.assert_reject(recovery.read_regular, root / 'link/input', policy=policy)
            self.assert_reject(recovery.read_regular, root / '../elsewhere', policy=policy)
            original = os.read
            def changed(fd, size):
                result = original(fd, size)
                source.write_bytes(b'changed')
                return result
            with patch.object(recovery.os, 'read', side_effect=changed):
                self.assert_reject(recovery.read_regular, source, policy=policy)

    def test_extraction_exclusive_safe_modes_and_original_immutable(self):
        raw = archive(entries(), True)
        validated = recovery.inspect_archive(raw, recovery=True)
        with tempfile.TemporaryDirectory() as temporary:
            parent = Path(temporary)
            policy = recovery.FilePolicy(parent, os.getuid())
            stage = parent / 'material'
            recovery.extract_validated(validated, stage, policy=policy)
            self.assertEqual((stage / 'source/.env').read_bytes(), ENV)
            self.assertEqual(stat.S_IMODE(stage.stat().st_mode), 0o700)
            self.assertEqual(stat.S_IMODE((stage / 'source/.env').stat().st_mode), 0o600)
            self.assertEqual(raw, archive(entries(), True))
            self.assert_reject(recovery.extract_validated, validated, stage, policy=policy)
            self.assertEqual((stage / 'proxy/config').read_bytes(), b'not-executed')

    def test_extraction_parent_symlink_and_collision_preserve_strangers(self):
        validated = recovery.inspect_archive(archive(entries(), True), recovery=True)
        with tempfile.TemporaryDirectory() as temporary:
            parent = Path(temporary)
            policy = recovery.FilePolicy(parent, os.getuid())
            (parent / 'alias').symlink_to(parent, target_is_directory=True)
            self.assert_reject(recovery.extract_validated, validated, parent / 'alias/stage', policy=policy)
            stranger = parent / 'stranger'
            stranger.mkdir()
            (stranger / 'keep').write_bytes(b'unchanged')
            self.assert_reject(recovery.extract_validated, validated, stranger, policy=policy)
            self.assertEqual((stranger / 'keep').read_bytes(), b'unchanged')

    def test_clone_preserves_only_six_identity_fields_and_fixed_overrides(self):
        source = ENV + b'PUSHER_APP_ID=known\nPUSHER_APP_KEY=key\nPUSHER_APP_SECRET=secret\n'
        clone = recovery.clone_env(source)
        parsed = recovery.parse_env(clone)
        self.assertEqual(parsed['APP_KEY'], KEY)
        self.assertEqual(parsed['APP_NAME'], 'Coolify')
        self.assertEqual(parsed['PUSHER_APP_ID'], 'known')
        self.assertEqual(parsed['DATABASE_URL'], '')
        self.assertEqual(parsed['QUEUE_CONNECTION'], 'redis')
        self.assertEqual(parsed['REDIS_URL'], '')
        self.assertIn(b'HORIZON_ENABLED=false\n', clone)
        self.assertIn(b'SCHEDULER_ENABLED=false\n', clone)
        self.assertNotIn(b'private.invalid', clone)
        for side in ('READ', 'WRITE'):
            for field in ('HOST', 'PORT', 'USERNAME', 'PASSWORD'):
                self.assertEqual(parsed[f'DB_{side}_{field}'], '')
        self.assertEqual(source, ENV + b'PUSHER_APP_ID=known\nPUSHER_APP_KEY=key\nPUSHER_APP_SECRET=secret\n')

    def test_missing_identity_and_invalid_key_rejected_wrong_key_not_changed(self):
        for source in (b'APP_NAME=Coolify\n', ('APP_KEY=' + KEY + '\n').encode(),
                       b'APP_KEY=base64:short\nAPP_NAME=Coolify\n',
                       b'APP_KEY=base64:' + base64.b64encode(b'a' * 31) + b'\nAPP_NAME=Coolify\n'):
            self.assert_reject(recovery.clone_env, source)
        wrong_key = 'base64:' + base64.b64encode(b'z' * 32).decode()
        clone = recovery.clone_env(ENV.replace(KEY.encode(), wrong_key.encode()))
        self.assertEqual(recovery.parse_env(clone)['APP_KEY'], wrong_key)
        self.assertNotEqual(recovery.parse_env(clone)['APP_KEY'], KEY)

    def test_env_interpolation_quotes_duplicates_and_marker_redaction(self):
        for line in ('export APP_KEY=' + MARKER, 'bad-key=' + MARKER,
                     'APP_NAME="' + MARKER, 'APP_NAME=x\ny', 'APP_NAME=\'a"',
                     'APP_NAME=${VAR}', 'APP_NAME=$VAR', 'APP_NAME=$(cmd)',
                     'APP_NAME=`cmd`', 'APP_NAME=a\\b', "APP_NAME=\"can't\"", 'APP_NAME=x\x00y',
                     'APP_NAME=x\rnext'):
            self.assert_reject(recovery.clone_env, ('APP_KEY=' + KEY + '\n' + line + '\n').encode())
        self.assert_reject(recovery.clone_env, ENV + b'APP_NAME=' + MARKER.encode())


if __name__ == '__main__':
    unittest.main()
