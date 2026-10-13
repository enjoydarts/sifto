"""Synthetic public artifacts only; no production secrets or restore claims."""
import copy
import hashlib
from pathlib import Path
import tempfile
import unittest

from maintenance import plan


class CoolifyBundleTests(unittest.TestCase):
    def setUp(self):
        self.temporary = tempfile.TemporaryDirectory()
        self.addCleanup(self.temporary.cleanup)
        self.root = Path(self.temporary.name)
        self.artifacts = ['docker-compose.yml', 'docker-compose.prod.yml', '.env.production',
                          'scripts/upgrade.sh', 'scripts/upgrade-postgres.sh']
        self.manifest = {'schema_version': 1, 'platform': 'linux/arm64', 'rollback': {}, 'target': {}}
        for phase, version, letter in [('rollback', '4.4.2', 'a'), ('target', '4.4.6', 'b')]:
            hashes = {}
            for artifact in self.artifacts:
                path = self.root / phase / artifact
                path.parent.mkdir(parents=True, exist_ok=True)
                data = ('synthetic public artifact: ' + phase + '/' + artifact).encode()
                path.write_bytes(data)
                hashes[artifact] = hashlib.sha256(data).hexdigest()
            self.manifest[phase] = {'version': version, 'release_commit': letter * 40,
                'artifacts': hashes, 'images': {
                    'coolify': 'docker.io/coollabsio/coolify@sha256:' + letter * 64,
                    'postgres': 'docker.io/library/postgres@sha256:' + 'c' * 64,
                    'redis': 'docker.io/library/redis@sha256:' + 'd' * 64,
                    'helper': 'docker.io/coollabsio/coolify-helper@sha256:' + 'e' * 64}}

    def verifier(self):
        from maintenance import coolify_bundle
        return coolify_bundle.verify_staged_bundle

    def verify(self, manifest=None, expected=None):
        manifest = self.manifest if manifest is None else manifest
        return self.verifier()(manifest, self.root, expected_manifest_sha256=(
            plan.plan_sha256(manifest) if expected is None else expected))

    def test_hash_checked_public_bundle_is_integrity_evidence_only(self):
        result = self.verify()
        self.assertEqual(result, {'status': 'integrity_verified',
                                 'manifest_sha256': plan.plan_sha256(self.manifest),
                                 'artifact_count': 10})
        self.assertNotIn('ready', str(result))

    def test_manifest_and_artifact_drift_fail_independently(self):
        trusted_hash = plan.plan_sha256(self.manifest)
        mutated = copy.deepcopy(self.manifest)
        mutated['target']['release_commit'] = 'f' * 40
        with self.assertRaises(plan.MaintenanceError):
            self.verify(mutated, trusted_hash)
        (self.root / 'target/docker-compose.yml').write_text('changed')
        with self.assertRaises(plan.MaintenanceError):
            self.verify()

    def test_only_patch_and_explicit_platform_public_digest_closure_are_accepted(self):
        for path, value in [(['target', 'version'], '4.5.0'),
                            (['target', 'version'], '5.0.0'),
                            (['target', 'version'], '4.4.2'),
                            (['target', 'version'], '4.4.6-beta'),
                            (['platform'], 'linux/amd64'),
                            (['target', 'images', 'coolify'], 'docker.io/coollabsio/coolify:4.4.6'),
                            (['target', 'images', 'coolify'], 'private.invalid/secret@sha256:' + 'b' * 64),
                            (['target', 'images', 'postgres'], 'docker.io/library/postgres@sha256:' + 'f' * 64),
                            (['target', 'artifacts', '../secret'], 'f' * 64),
                            (['target', 'secret'], 'fixture-forbidden')]:
            with self.subTest(path=path, value=value):
                mutated = copy.deepcopy(self.manifest)
                node = mutated
                for key in path[:-1]:
                    node = node[key]
                node[path[-1]] = value
                with self.assertRaises(plan.MaintenanceError) as caught:
                    self.verify(mutated)
                self.assertNotIn('fixture-forbidden', str(caught.exception))
                self.assertNotIn('private.invalid', str(caught.exception))

    def test_missing_empty_oversized_linked_special_and_extra_artifacts_fail(self):
        artifact = self.root / 'target/docker-compose.yml'
        original = artifact.read_bytes()
        artifact.unlink()
        with self.assertRaises(plan.MaintenanceError):
            self.verify()
        artifact.symlink_to(self.root / 'rollback/docker-compose.yml')
        with self.assertRaises(plan.MaintenanceError):
            self.verify()
        artifact.unlink()
        artifact.mkdir()
        with self.assertRaises(plan.MaintenanceError):
            self.verify()
        artifact.rmdir()
        for data in [b'', b'x' * (1024 * 1024 + 1)]:
            artifact.write_bytes(data)
            self.manifest['target']['artifacts']['docker-compose.yml'] = hashlib.sha256(data).hexdigest()
            with self.assertRaises(plan.MaintenanceError):
                self.verify()
        artifact.write_bytes(original)
        self.manifest['target']['artifacts']['docker-compose.yml'] = hashlib.sha256(original).hexdigest()
        (self.root / 'target/secret.env').write_text('synthetic-forbidden')
        with self.assertRaises(plan.MaintenanceError):
            self.verify()

    def test_symlinked_bundle_directory_is_rejected(self):
        (self.root / 'target/scripts/upgrade.sh').unlink()
        (self.root / 'target/scripts/upgrade-postgres.sh').unlink()
        (self.root / 'target/scripts').rmdir()
        (self.root / 'target/scripts').symlink_to(self.root / 'rollback/scripts')
        with self.assertRaises(plan.MaintenanceError):
            self.verify()


if __name__ == '__main__':
    unittest.main()
