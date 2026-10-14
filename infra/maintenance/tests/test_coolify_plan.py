"""Offline protocol tests: synthetic keys, hashes and backup metadata only."""
import base64
import copy
import hashlib
import json
from pathlib import Path
import unittest
from unittest.mock import patch

from cryptography.hazmat.primitives.asymmetric.ed25519 import Ed25519PrivateKey
from maintenance import plan as legacy
from test_plan import inventory

try:
    from maintenance import coolify_plan as protocol
except ImportError:
    protocol = None

PUBLICATION = 1791571325  # 2026-10-09T18:42:05Z
NOT_BEFORE = PUBLICATION + 7 * 86400
NOW = NOT_BEFORE + 3600
OLD_ARM = 'sha256:a323ce1dfb54101b0c30d8bd5c92b0e4166c3b403a164316f8876be653204960'
NEW_ARM = 'sha256:f4ef1eb69daa265057ab700646485bd78bb43bcf96c691d65094947acb7f50a5'
FILES = {'coolify.dump': '1' * 64, 'coolify-config.tar.gz': '2' * 64, 'created_at': '3' * 64}


class CoolifyPlanTests(unittest.TestCase):
    def setUp(self):
        self.assertIsNotNone(protocol, 'Coolify protocol implementation is missing')
        self.key = Ed25519PrivateKey.from_private_bytes(bytes(range(32)))

    def candidate(self, now=NOW):
        return {'schema_version': 2, 'action': 'coolify_patch',
                'plan_id': '11111111-2222-4333-8444-555555555555', 'host_id': 'fixture-app',
                'created_at': now, 'expires_at': now + 600, 'workflow_sha': 'a' * 40,
                'state_sha256': 'b' * 64, 'profile_id': 'coolify-4.4.2-to-4.4.6-arm64-v1',
                'profile_sha256': protocol.profile_sha256(), 'bundle_sha256': protocol.bundle_sha256(),
                'from_version': '4.4.2', 'to_version': '4.4.6',
                'from_platform_manifest': OLD_ARM, 'to_platform_manifest': NEW_ARM,
                'publication_at': PUBLICATION, 'not_before': NOT_BEFORE,
                'source_configuration_sha256': 'c' * 64, 'dependencies_sha256': 'd' * 64,
                'protected_runtime_sha256': 'e' * 64, 'barrier_policy_sha256': 'f' * 64,
                'backup_binding': {'snapshot_id': '4' * 64, 'proof_sha256': '5' * 64,
                    'file_sha256': dict(FILES), 'verified_at': now - 60,
                    'offhost_verified_at': now - 120, 'restore_tested_at': now - 180,
                    'expires_at': now + 1200},
                'rehearsal_binding': {'receipt_sha256': '6' * 64,
                    'profile_sha256': protocol.profile_sha256(), 'source_configuration_sha256': 'c' * 64,
                    'snapshot_id': '4' * 64, 'file_sha256': dict(FILES),
                    'old_boot': True, 'target_migrations': True, 'fresh_original_restore': True,
                    'verified_at': now - 30, 'expires_at': now + 900}}

    def approval(self, value, mode='apply'):
        return {'plan_sha256': legacy.plan_sha256(value), 'workflow_sha': value['workflow_sha'],
                'execution_mode': mode, 'run_id': '123', 'approved_by': 'fixture-owner',
                'approved_at': value['created_at'], 'expires_at': value['expires_at']}

    def signed(self, mode='apply'):
        value = self.candidate()
        return protocol.sign_plan(value, self.approval(value, mode), self.key, now=NOW)

    def rejected(self, value, now=NOW):
        with self.assertRaises(legacy.MaintenanceError):
            protocol.validate_plan(value, now=now)

    def test_fixed_plan_can_be_validated_and_signed_without_granting_eligibility(self):
        value = self.candidate()
        self.assertEqual(protocol.validate_plan(value, now=NOW), value)
        envelope = self.signed()
        self.assertEqual(protocol.verify_envelope(envelope, self.key.public_key(), 'fixture-app',
                                                'a' * 40, now=NOW), value)
        self.assertIs(protocol.profile_eligibility()['eligible'], False)
        self.assertIs(protocol.fixed_profile()['controlled_startup_verified'], False)

    def test_public_profile_file_has_canonical_identity_and_honest_startup_limits(self):
        path = Path(__file__).parents[1] / 'coolify/profiles/4.4.2-to-4.4.6.json'
        profile = json.loads(path.read_text())
        self.assertEqual(protocol.validate_profile(profile), protocol.fixed_profile())
        self.assertEqual(hashlib.sha256(legacy.canonical_json(profile)).hexdigest(), protocol.profile_sha256())
        self.assertEqual(hashlib.sha256(legacy.canonical_json(profile['bundle'])).hexdigest(), protocol.bundle_sha256())
        self.assertEqual(profile['publication_at'], PUBLICATION)
        self.assertEqual(profile['not_before'], NOT_BEFORE)
        self.assertIn('remote Seeder', profile['controlled_startup_limitations'])
        self.assertIn('AppInit', profile['controlled_startup_limitations'])
        self.assertIn('maintenance webhook', profile['controlled_startup_limitations'])

    def test_public_provenance_artifacts_repositories_and_arm_children_are_exact(self):
        profile = protocol.fixed_profile()
        bundle = profile['bundle']
        self.assertEqual(bundle['platform'], 'linux/arm64')
        self.assertEqual(bundle['rollback']['release_commit'], '76d3386a4d69f894258210ea265717a8fb8cd778')
        self.assertEqual(bundle['target']['release_commit'], 'f55efc859dfcbbf79274c4907b3df89f9767c11d')
        expected = {'docker-compose.yml': '2532ee9258e10ad4e86cb3c2b46c29fe9329f5a0dace0f7da410744feec279bd',
            'docker-compose.prod.yml': '4cd00e83e117e847e6d6bdfe64f2a2f958bb09965692c5230fa9231a0315f80f',
            '.env.production': '7895e54f7c9d40063e76afec9b35839048d91e5c3b5107d64ea49ef6987c304a',
            'scripts/upgrade.sh': '8dc02adee5ea0f27027405243a15e9be8bced5ea290870de90c8ec0ce4979fda',
            'scripts/upgrade-postgres.sh': 'aa65c277fc13ab5b27c349fbfef12defc59c084b244e4b258329e1f6c570ae8d'}
        for phase, arm in [('rollback', OLD_ARM), ('target', NEW_ARM)]:
            self.assertEqual(bundle[phase]['artifacts'], expected)
            self.assertEqual(bundle[phase]['images']['coolify']['repository'], 'docker.io/coollabsio/coolify')
            self.assertEqual(bundle[phase]['images']['coolify']['platform_manifest'], arm)
        self.assertEqual(profile['new_migrations'], [
            '2026_10_09_092903_normalize_ipv6_server_ips',
            '2026_10_09_115444_remove_duplicate_redis_username_environment_variables',
            '2026_10_09_124641_remove_duplicate_nixpacks_node_version_environment_variables'])

    def test_profile_artifact_platform_repository_or_verified_flag_tampering_is_rejected(self):
        changes = [('controlled_startup_verified', True), ('not_before', NOT_BEFORE - 1),
                   ('bundle.target.artifacts.scripts/upgrade.sh', '0' * 64),
                   ('bundle.target.images.coolify.repository', 'coollabsio/coolify'),
                   ('bundle.target.images.coolify.repository', 'https://example.invalid/image'),
                   ('bundle.target.images.coolify.platform_manifest', 'sha256:' + '0' * 64),
                   ('bundle.platform', 'linux/amd64')]
        for field, value in changes:
            profile = protocol.fixed_profile()
            parts = (['bundle', 'target', 'artifacts', 'scripts/upgrade.sh']
                     if field == 'bundle.target.artifacts.scripts/upgrade.sh' else field.split('.'))
            selected = profile
            for part in parts[:-1]:
                selected = selected[part]
            selected[parts[-1]] = value
            with self.subTest(field=field), self.assertRaises(legacy.MaintenanceError):
                protocol.validate_profile(profile)

    def test_schema_dispatch_rejects_missing_unknown_and_boolean_schema(self):
        for value in [None, [], {}, {'schema_version': True}, {'schema_version': '2'}, {'schema_version': 3}]:
            self.rejected(value)

    def test_every_field_is_required_and_unknown_capability_fields_are_rejected(self):
        value = self.candidate()
        for field in value:
            changed = copy.deepcopy(value)
            del changed[field]
            with self.subTest(missing=field):
                self.rejected(changed)
        for field in ['shell', 'packages', 'restart_services', 'image_url', 'target_override',
                      'owner_gate_obtained', 'ready', 'now', 'private_path', 'env', 'archive_body']:
            changed = copy.deepcopy(value)
            changed[field] = True
            with self.subTest(unknown=field):
                self.rejected(changed)

    def test_identifiers_hashes_and_fixed_transition_cannot_be_overridden(self):
        changes = [('action', 'coolify_restore'), ('action', 'apt_upgrade'), ('action', 'script'),
                   ('plan_id', '11111111222243338444555555555555'), ('host_id', 'https://fixture.invalid'),
                   ('workflow_sha', 'main'), ('state_sha256', 'b' * 63), ('profile_id', '../profile'),
                   ('profile_sha256', '0' * 64), ('bundle_sha256', '0' * 64),
                   ('from_version', '4.4.3'), ('to_version', '4.4.7'), ('to_version', '4.5.0'),
                   ('to_version', '5.0.0'), ('to_version', 'nightly'), ('to_version', 'latest'),
                   ('from_platform_manifest', 'sha256:cd884496945d4a2656a480401238e1dda4a1569424102df7474ce3ac73550cd6'),
                   ('to_platform_manifest', 'docker.io/coollabsio/coolify:4.4.6'),
                   ('to_platform_manifest', 'sha256:' + '0' * 64),
                   ('source_configuration_sha256', {}), ('dependencies_sha256', 'D' * 64),
                   ('protected_runtime_sha256', None), ('barrier_policy_sha256', 'secret-value')]
        for field, value in changes:
            changed = self.candidate()
            changed[field] = value
            with self.subTest(field=field, value=value):
                self.rejected(changed)

    def test_integer_timestamps_lifetime_and_trusted_clock_are_strict(self):
        changes = [('created_at', True), ('expires_at', True), ('publication_at', True),
                   ('not_before', True), ('created_at', -1), ('created_at', NOW + 1),
                   ('expires_at', NOW), ('expires_at', NOW + 86401),
                   ('created_at', float('nan')), ('expires_at', float('inf')),
                   ('publication_at', PUBLICATION - 1), ('not_before', NOT_BEFORE - 1)]
        for field, value in changes:
            changed = self.candidate()
            changed[field] = value
            with self.subTest(field=field, value=value):
                self.rejected(changed)
        for now in [True, -1, float('inf'), 'future']:
            self.rejected(self.candidate(), now=now)
        self.rejected(self.candidate(), now=NOW + 600)

    def test_seven_day_age_is_enforced_at_exact_threshold_in_all_v2_entry_points(self):
        value = self.candidate(NOT_BEFORE)
        approval = self.approval(value)
        self.rejected(value, now=NOT_BEFORE - 1)
        with self.assertRaises(legacy.MaintenanceError):
            protocol.sign_plan(value, approval, self.key, now=NOT_BEFORE - 1)
        envelope = protocol.sign_plan(value, approval, self.key, now=NOT_BEFORE)
        with self.assertRaises(legacy.MaintenanceError):
            protocol.verify_envelope(envelope, self.key.public_key(), 'fixture-app', 'a' * 40, now=NOT_BEFORE - 1)
        self.assertEqual(protocol.verify_envelope(envelope, self.key.public_key(), 'fixture-app',
                                                'a' * 40, now=NOT_BEFORE), value)
        with patch('time.time', return_value=NOT_BEFORE - 1):
            self.rejected(value, now=None)

    def test_nested_backup_and_rehearsal_field_sets_are_exact(self):
        for binding in ['backup_binding', 'rehearsal_binding']:
            for field in self.candidate()[binding]:
                value = self.candidate()
                del value[binding][field]
                with self.subTest(binding=binding, missing=field):
                    self.rejected(value)
            for field in ['ready', 'offhost_location', 'private_path', 'config', 'key']:
                value = self.candidate()
                value[binding][field] = 'private-sentinel'
                with self.subTest(binding=binding, unknown=field):
                    self.rejected(value)

    def test_backup_three_proofs_are_fresh_nonfuture_and_expire_with_oldest(self):
        for field in ['verified_at', 'offhost_verified_at', 'restore_tested_at']:
            for timestamp in [True, -1, NOW + 1, NOW - 86400, 1.5]:
                value = self.candidate()
                value['backup_binding'][field] = timestamp
                with self.subTest(field=field, timestamp=timestamp):
                    self.rejected(value)
        for expiry in [NOW, True, NOW - 180 + 86401]:
            value = self.candidate()
            value['backup_binding']['expires_at'] = expiry
            self.rejected(value)

    def test_exact_backup_file_closure_and_snapshot_hashes_are_required(self):
        for binding in ['backup_binding', 'rehearsal_binding']:
            for files in [{}, {**FILES, 'env': '7' * 64}, {**FILES, 'coolify.dump': '/private/archive'},
                          {**FILES, 'created_at': True}, []]:
                value = self.candidate()
                value[binding]['file_sha256'] = files
                with self.subTest(binding=binding, files=files):
                    self.rejected(value)
            value = self.candidate()
            value[binding]['snapshot_id'] = 'snapshot-1'
            self.rejected(value)
        for field in ['proof_sha256']:
            value = self.candidate()
            value['backup_binding'][field] = 'invalid'
            self.rejected(value)

    def test_rehearsal_must_bind_exact_profile_configuration_snapshot_and_files(self):
        for field in ['profile_sha256', 'source_configuration_sha256', 'snapshot_id', 'receipt_sha256']:
            value = self.candidate()
            value['rehearsal_binding'][field] = 'invalid' if field == 'receipt_sha256' else '0' * 64
            with self.subTest(field=field):
                self.rejected(value)
        value = self.candidate()
        value['rehearsal_binding']['file_sha256']['coolify.dump'] = '0' * 64
        self.rejected(value)

    def test_rehearsal_results_require_boolean_true_and_fresh_bounded_time(self):
        for field in ['old_boot', 'target_migrations', 'fresh_original_restore']:
            for flag in [False, 1, 'true', None, {}]:
                value = self.candidate()
                value['rehearsal_binding'][field] = flag
                with self.subTest(field=field, flag=flag):
                    self.rejected(value)
        for field, timestamp in [('verified_at', True), ('verified_at', NOW + 1),
                                 ('verified_at', NOW - 86400), ('expires_at', NOW),
                                 ('expires_at', NOW - 30 + 86401), ('expires_at', NOW + 1201)]:
            value = self.candidate()
            value['rehearsal_binding'][field] = timestamp
            self.rejected(value)

    def test_plan_cannot_outlive_backup_or_rehearsal(self):
        for binding in ['backup_binding', 'rehearsal_binding']:
            value = self.candidate()
            value[binding]['expires_at'] = NOW + 599
            with self.subTest(binding=binding):
                self.rejected(value)

    def test_oversized_deep_nonfinite_or_non_json_data_fail_with_static_errors(self):
        deep = {}
        for _ in range(100):
            deep = {'x': deep}
        for payload in ['private-sentinel' * 5000, deep, [None] * 10000, float('nan'), object()]:
            value = self.candidate()
            value['source_configuration_sha256'] = payload
            with self.assertRaises(legacy.MaintenanceError) as caught:
                protocol.validate_plan(value, now=NOW)
            self.assertNotIn('private-sentinel', str(caught.exception))

    def test_signature_is_exact_existing_canonical_plan_approval_body(self):
        envelope = self.signed()
        body = {'plan': envelope['plan'], 'approval': envelope['approval']}
        expected = base64.b64encode(self.key.sign(legacy.canonical_json(body))).decode('ascii')
        self.assertEqual(envelope['signature'], expected)
        envelope = protocol.sign_plan(body['plan'], body['approval'], bytes(range(32)), now=NOW)
        public = self.key.public_key()
        from cryptography.hazmat.primitives import serialization
        raw = public.public_bytes(serialization.Encoding.Raw, serialization.PublicFormat.Raw)
        for key in [public, raw, base64.b64encode(raw).decode('ascii')]:
            self.assertEqual(protocol.verify_envelope(envelope, key, 'fixture-app', 'a' * 40, now=NOW), body['plan'])

    def test_wrong_key_host_workflow_approval_time_and_envelope_shape_fail(self):
        envelope = self.signed()
        contexts = [(Ed25519PrivateKey.generate().public_key(), 'fixture-app', 'a' * 40, NOW),
                    (self.key.public_key(), 'other', 'a' * 40, NOW),
                    (self.key.public_key(), 'fixture-app', 'b' * 40, NOW),
                    (self.key.public_key(), 'fixture-app', 'a' * 40, NOW + 600)]
        for key, host, workflow, now in contexts:
            with self.subTest(host=host, workflow=workflow, now=now), self.assertRaises(legacy.MaintenanceError):
                protocol.verify_envelope(envelope, key, host, workflow, now=now)
        for field, value in [('signature', '%%%'), ('signature', None), ('signature', 'A' * 50000),
                             ('private', 'private-sentinel')]:
            changed = copy.deepcopy(envelope)
            changed[field] = value
            with self.assertRaises(legacy.MaintenanceError):
                protocol.verify_envelope(changed, self.key.public_key(), 'fixture-app', 'a' * 40, now=NOW)

    def test_tampered_approval_mode_reviewer_run_time_and_plan_cannot_reuse_signature(self):
        envelope = self.signed()
        for field, value in [('execution_mode', 'dry_run'), ('approved_by', 'other-owner'),
                             ('run_id', '456'), ('approved_at', NOW + 1), ('expires_at', NOW + 599)]:
            changed = copy.deepcopy(envelope)
            changed['approval'][field] = value
            with self.subTest(field=field), self.assertRaises(legacy.MaintenanceError):
                protocol.verify_envelope(changed, self.key.public_key(), 'fixture-app', 'a' * 40, now=NOW + 2)
        for field in ['schema_version', 'action', 'profile_id', 'from_version', 'to_version',
                      'state_sha256', 'source_configuration_sha256', 'dependencies_sha256',
                      'protected_runtime_sha256', 'barrier_policy_sha256', 'created_at']:
            changed = copy.deepcopy(envelope)
            changed['plan'][field] = 1 if field in ['schema_version', 'created_at'] else '0' * 64
            with self.subTest(field=field), self.assertRaises(legacy.MaintenanceError):
                protocol.verify_envelope(changed, self.key.public_key(), 'fixture-app', 'a' * 40, now=NOW)
        changed = copy.deepcopy(envelope)
        changed['plan']['backup_binding']['snapshot_id'] = '0' * 64
        changed['plan']['rehearsal_binding']['snapshot_id'] = '0' * 64
        with self.assertRaises(legacy.MaintenanceError):
            protocol.sign_plan(changed['plan'], changed['approval'], self.key, now=NOW)

    def test_recomputed_plan_hash_without_resigning_still_fails(self):
        envelope = self.signed()
        envelope['plan']['source_configuration_sha256'] = '0' * 64
        envelope['plan']['rehearsal_binding']['source_configuration_sha256'] = '0' * 64
        envelope['approval']['plan_sha256'] = legacy.plan_sha256(envelope['plan'])
        with self.assertRaises(legacy.MaintenanceError):
            protocol.verify_envelope(envelope, self.key.public_key(), 'fixture-app', 'a' * 40, now=NOW)

    def test_approval_cannot_be_future_or_contain_new_gate_claims(self):
        value = self.candidate()
        for field, changed in [('approved_at', NOW + 1), ('execution_mode', True),
                               ('ready', True), ('approved_by', 'https://private.invalid')]:
            approval = self.approval(value)
            approval[field] = changed
            with self.subTest(field=field), self.assertRaises(legacy.MaintenanceError):
                protocol.sign_plan(value, approval, self.key, now=NOW)

    def test_dry_run_and_apply_modes_are_both_signed_and_verifiable(self):
        for mode in ['dry_run', 'apply']:
            envelope = self.signed(mode)
            self.assertEqual(envelope['approval']['execution_mode'], mode)
            self.assertEqual(protocol.verify_envelope(envelope, self.key.public_key(), 'fixture-app',
                                                    'a' * 40, now=NOW), envelope['plan'])

    def test_legacy_apt_and_reboot_envelopes_and_validation_are_unchanged(self):
        for action in ['apt_upgrade', 'reboot']:
            value = legacy.build_plan(inventory(), 'a' * 40, now=1000, action=action,
                                      allow_reboot=action == 'reboot')
            approval = self.approval(value)
            direct = legacy.sign_plan(value, approval, self.key)
            universal = protocol.sign_plan(value, approval, self.key, now=NOW)
            self.assertEqual(legacy.canonical_json(universal), legacy.canonical_json(direct))
            self.assertEqual(protocol.validate_plan(value, now=NOW), legacy.validate_plan(value))
            self.assertEqual(protocol.verify_envelope(universal, self.key.public_key(), value['host_id'],
                'a' * 40, now=1000), legacy.verify_envelope(direct, self.key.public_key(), value['host_id'],
                'a' * 40, now=1000))
            value['allow_reboot'] = 'invalid'
            for validator in [legacy.validate_plan, protocol.validate_plan]:
                with self.assertRaises(legacy.MaintenanceError):
                    validator(value)


if __name__ == '__main__':
    unittest.main()
