"""Public synthetic contracts: no backup, env, DB, PHP or Docker access."""
import contextlib
import copy
import hashlib
import io
import unittest
from unittest.mock import patch, mock_open

from maintenance.coolify_rehearsal import RehearsalError
from maintenance import coolify_plan

try:
    from maintenance import coolify_evidence as evidence
except ImportError:
    evidence = None


class EvidenceContracts(unittest.TestCase):
    def setUp(self):
        self.assertIsNotNone(evidence, 'native typed Coolify evidence port is missing')
        self.catalog = evidence.fixed_catalog()

    def fixture(self, version='4.4.2', *, baseline=False, history=()):
        migrations = list(self.catalog['migrations'][version]) + list(history)
        _, required = evidence.trusted_table_catalog(migrations, version)
        digest = hashlib.sha256(b'public synthetic hmac placeholder').hexdigest()
        core = {name: {'count': 4, 'digest': digest} for name in evidence.CORE}
        return {
            'ok': True, 'category': 'verified', 'version': version,
            'db': True, 'no_pending': True, 'network': True, 'wrong_key': True,
            'ssh': not baseline, 'ssh_checked': 0 if baseline else 1,
            'decrypt': {'checked': 3, 'nonempty': 2, 'missing': 0, 'failures': 0},
            'decrypt_models': {
                name: {'checked': 3 if name == 'PrivateKey' else 0, 'failures': 0,
                       'columns': max(evidence.encrypted_column_counts(name, version))}
                for name in evidence.DECRYPT_MODELS},
            'table_coverage': {'known': len(required), 'unknown': 0, 'tables': {
                name: {'count': len(migrations) if name == 'migrations' else 4 if name in core else 0,
                       'classification': 'core' if name in core else 'runtime'}
                for name in required}},
            'ipv6_affected': {'servers': 0, 'sentinel': 0}, 'migrations': migrations,
            'core': core, 'deletions': {'surplus': 0, 'survivor_digest': digest,
                                      'expected_digest': digest, 'expected_count': 4},
            'seeder': {name: [{'id': 0, 'uuid_digest': digest}]
                       for name in ('users', 'teams', 'servers', 'private_keys')},
        }

    def args(self, name, version='4.4.2'):
        return [name, copy.deepcopy(self.catalog['encrypted_casts'][version][name]),
                self.catalog['models'][version][name], version,
                list(self.catalog['migrations'][version])]

    def presence(self, name, version='4.4.2'):
        attributes = evidence.corrected_encrypted_casts(*self.args(name, version))
        if name == 'User':
            attributes.update({'two_factor_secret': 'fortify:string',
                               'two_factor_recovery_codes': 'fortify:string'})
        return dict.fromkeys(attributes, True)

    def reject(self, action):
        stdout, stderr = io.StringIO(), io.StringIO()
        with contextlib.redirect_stdout(stdout), contextlib.redirect_stderr(stderr):
            with self.assertRaises(RehearsalError) as caught:
                action()
        self.assertEqual(stdout.getvalue() + stderr.getvalue(), '')
        self.assertNotIn('PLANTED_PRIVATE_MARKER', str(caught.exception))
        self.assertRegex(str(caught.exception), r'^[a-z_]+$')

    def mutate(self, path, value, *, baseline=False):
        selected = self.fixture(baseline=baseline)
        target = selected
        for key in path[:-1]:
            target = target[key]
        target[path[-1]] = value
        self.reject(lambda: evidence.validate_evidence(selected, '4.4.2', baseline=baseline))

    def test_public_catalog_and_native_profile_bind_exact_transition(self):
        self.assertEqual(evidence.MANIFEST_SHA,
                         'b37aff8b86a8ff20372ef5fb7e9bba40ba7b192f899b649dce0c9d0daa61e276')
        profile = coolify_plan.fixed_profile()
        self.assertEqual(tuple(profile['new_migrations']), evidence.NEW_MIGRATIONS)
        for role in ('rollback', 'target'):
            selected = profile['bundle'][role]
            self.assertEqual(self.catalog['revisions'][selected['version']], selected['release_commit'])
        self.assertEqual(len(evidence.CORE), 59)
        self.assertEqual(len(evidence.DECRYPT_MODELS), 68)
        self.assertEqual(len(evidence.MAINLINE_RETIRED_MIGRATIONS), 21)
        self.assertEqual([len(self.catalog['migrations'][v]) for v in ('4.4.2', '4.4.6')], [435, 438])
        self.assertEqual(set(self.catalog['migrations']['4.4.6']) -
                         set(self.catalog['migrations']['4.4.2']), set(evidence.NEW_MIGRATIONS))
        self.assertFalse(coolify_plan.profile_eligibility()['eligible'])

    def test_catalog_tamper_and_missing_bytes_fail_closed_without_output(self):
        for content in (b'PLANTED_PRIVATE_MARKER', b'x' * 80948, b''):
            with patch.object(evidence.Path, 'open', mock_open(read_data=content)):
                self.reject(evidence.fixed_catalog)
        with patch.object(evidence.Path, 'open', side_effect=OSError('PLANTED_PRIVATE_MARKER')):
            self.reject(evidence.fixed_catalog)
        self.assertEqual(evidence.fixed_catalog(), self.catalog)

    def test_catalog_profile_drift_is_rejected(self):
        altered = coolify_plan.fixed_profile()
        altered['new_migrations'].pop()
        with patch.object(coolify_plan, 'fixed_profile', return_value=altered):
            self.reject(evidence.fixed_catalog)

    def test_catalog_and_selected_attributes_are_detached_copies(self):
        self.catalog['models']['4.4.2']['Server'] = 'PLANTED_PRIVATE_MARKER'
        self.assertNotEqual(evidence.fixed_catalog(), self.catalog)
        args = self.args('LocalFileVolume')
        before = copy.deepcopy(args)
        selected = evidence.corrected_encrypted_casts(*args)
        selected['content'] = 'altered'
        self.assertEqual(args, before)
        self.assertEqual(evidence.corrected_encrypted_casts(*args), {'content': 'encrypted'})

    def test_old_raw_startup_target_and_fresh_original_roundtrip(self):
        raw = self.fixture(baseline=True)
        old = self.fixture()
        target = self.fixture('4.4.6')
        self.assertIs(evidence.validate_evidence(raw, '4.4.2', baseline=True), raw)
        self.assertTrue(evidence.compare_evidence(raw, old, startup=True))
        self.assertTrue(evidence.compare_evidence(old, target, forward=True))
        self.assertTrue(evidence.compare_evidence(old, self.fixture()))

    def test_exact_seventeen_success_fields_and_gate_types(self):
        selected = self.fixture()
        self.assertEqual(len(selected), 17)
        for key in selected:
            bad = copy.deepcopy(selected); del bad[key]
            self.reject(lambda: evidence.validate_evidence(bad, '4.4.2'))
        self.mutate(('extra',), 'PLANTED_PRIVATE_MARKER')
        for key in ('ok', 'db', 'no_pending', 'network', 'wrong_key'):
            for value in (False, 0, 1, None, 'true', []):
                self.mutate((key,), value)
        for value in ('failed', None, [], 'PLANTED_PRIVATE_MARKER'):
            self.mutate(('category',), value)

    def test_known_version_and_typed_flags_only(self):
        for version in (None, False, [], '4.4.3', 'PLANTED_PRIVATE_MARKER'):
            self.reject(lambda: evidence.validate_evidence(self.fixture(), version))
            self.mutate(('version',), version)
        for flag in (None, 0, 1, 'true', []):
            self.reject(lambda: evidence.validate_evidence(self.fixture(), '4.4.2', baseline=flag))
            self.reject(lambda: evidence.compare_evidence(self.fixture(), self.fixture(), startup=flag))
            self.reject(lambda: evidence.compare_evidence(self.fixture(), self.fixture(), forward=flag))
        self.reject(lambda: evidence.compare_evidence(self.fixture(), self.fixture('4.4.6'),
                                                     startup=True, forward=True))

    def test_baseline_ssh_always_boolean_and_bounded_integer(self):
        for baseline in (False, True):
            for value in (None, 0, 1, [], 'PLANTED_PRIVATE_MARKER'):
                self.mutate(('ssh',), value, baseline=baseline)
            for value in (None, True, False, -1, 10_000_001, [], 'PLANTED_PRIVATE_MARKER'):
                self.mutate(('ssh_checked',), value, baseline=baseline)
        self.mutate(('ssh',), False)
        self.mutate(('ssh_checked',), 0)

    def test_nested_object_shapes_are_strict(self):
        paths = [('decrypt',), ('decrypt_models',), ('decrypt_models', 'PrivateKey'),
                 ('table_coverage',), ('table_coverage', 'tables'),
                 ('table_coverage', 'tables', 'users'), ('ipv6_affected',),
                 ('core',), ('core', 'users'), ('deletions',), ('seeder',)]
        for path in paths:
            for value in (None, False, [], 'PLANTED_PRIVATE_MARKER', {'extra': 0}):
                self.mutate(path, value)

    def test_all_counter_types_and_bounds_reject_boolean_integers(self):
        paths = [('ssh_checked',), ('table_coverage', 'known'), ('table_coverage', 'unknown')]
        paths += [('decrypt', key) for key in ('checked', 'nonempty', 'missing', 'failures')]
        paths += [('decrypt_models', 'PrivateKey', key) for key in ('checked', 'failures', 'columns')]
        paths += [('table_coverage', 'tables', 'activity_log', 'count'), ('core', 'users', 'count')]
        paths += [('deletions', key) for key in ('surplus', 'expected_count')]
        paths += [('ipv6_affected', key) for key in ('servers', 'sentinel')]
        for path in paths:
            for value in (True, False, -1, 10_000_001, 1.0, None, [], 'PLANTED_PRIVATE_MARKER'):
                self.mutate(path, value)

    def test_decrypt_positive_nonempty_failure_and_model_sums(self):
        for path, value in ((('decrypt', 'checked'), 0), (('decrypt', 'nonempty'), 0),
                            (('decrypt', 'nonempty'), 4), (('decrypt', 'failures'), 1),
                            (('decrypt_models', 'PrivateKey', 'failures'), 1),
                            (('decrypt_models', 'PrivateKey', 'checked'), 2)):
            self.mutate(path, value)

    def test_digest_shapes_raise_only_static_safe_error(self):
        for path in (('core', 'users', 'digest'), ('deletions', 'expected_digest'),
                     ('deletions', 'survivor_digest'), ('seeder', 'users', 0, 'uuid_digest')):
            for value in (None, True, 7, [], {}, 'a' * 63, 'A' * 64,
                          'a' * 64 + '\n', 'PLANTED_PRIVATE_MARKER'):
                self.mutate(path, value)

    def test_seeder_ids_and_record_closure(self):
        for value in (None, {}, 'PLANTED_PRIVATE_MARKER', [None],
                      [{'id': 0, 'uuid_digest': 'a' * 64, 'extra': 1}],
                      [{'id': True, 'uuid_digest': 'a' * 64}],
                      [{'id': 2, 'uuid_digest': 'a' * 64}],
                      [{'id': 0, 'uuid_digest': 'a' * 64}] * 9):
            self.mutate(('seeder', 'users'), value)

    def test_input_depth_nodes_strings_bytes_and_nonjson_values(self):
        for selected in (None, [], object(), b'PLANTED_PRIVATE_MARKER'):
            self.reject(lambda: evidence.validate_evidence(selected, '4.4.2'))
        nested = 'PLANTED_PRIVATE_MARKER'
        for _ in range(12): nested = [nested]
        for value in (nested, [None] * 10001, 'x' * 1025,
                      {'x' * 129: None}, float('nan'), {'bad': object()}):
            self.mutate(('category',), value)
        # One very large graph must be rejected before expensive canonical encoding.
        selected = self.fixture()
        selected['category'] = ['x' * 1024] * 10000
        with patch.object(evidence.json, 'dumps', side_effect=AssertionError('encoded too soon')):
            self.reject(lambda: evidence.validate_evidence(selected, '4.4.2'))

    def test_encrypted_local_file_and_optional_instance_columns(self):
        for version in ('4.4.2', '4.4.6'):
            self.assertEqual(evidence.corrected_encrypted_casts(*self.args('LocalFileVolume', version)),
                             {'content': 'encrypted'})
            self.assertEqual(evidence.encrypted_column_counts('LocalFileVolume', version), {1})
            args = self.args('InstanceSettings', version)
            presence = self.presence('InstanceSettings', version)
            self.assertIn('sentinel_token', evidence.persisted_encrypted_attributes(*args, presence))
            presence['sentinel_token'] = False
            selected = evidence.persisted_encrypted_attributes(*args, presence)
            self.assertEqual(len(selected), 8)
            self.assertNotIn('sentinel_token', selected)
            self.assertEqual(evidence.encrypted_column_counts('InstanceSettings', version), {8, 9})

    def test_server_ghost_columns_zero_one_both_preserve_present_attributes(self):
        for version in ('4.4.2', '4.4.6'):
            for axiom in (False, True):
                for newrelic in (False, True):
                    presence = {'logdrain_axiom_api_key': axiom,
                                'logdrain_newrelic_license_key': newrelic}
                    selected = evidence.persisted_encrypted_attributes(*self.args('Server', version), presence)
                    self.assertEqual(set(selected), {name for name, present in presence.items() if present})
            self.assertEqual(evidence.encrypted_column_counts('Server', version), {0, 1, 2})

    def test_user_raw_fortify_fields_mandatory(self):
        presence = self.presence('User')
        selected = evidence.persisted_encrypted_attributes(*self.args('User'), presence)
        self.assertEqual(selected['two_factor_secret'], 'fortify:string')
        self.assertEqual(selected['two_factor_recovery_codes'], 'fortify:string')
        self.assertEqual(evidence.encrypted_column_counts('User', '4.4.2'), {2})

    def test_every_other_missing_column_including_server_settings_is_fatal(self):
        for version in ('4.4.2', '4.4.6'):
            for name in self.catalog['models'][version]:
                presence = self.presence(name, version)
                for attribute in presence:
                    if name == 'Server' or (name == 'InstanceSettings' and attribute == 'sentinel_token'):
                        continue
                    bad = {**presence, attribute: False}
                    with self.subTest(name=name, version=version, attribute=attribute):
                        self.reject(lambda: evidence.persisted_encrypted_attributes(*self.args(name, version), bad))

    def test_source_version_raw_declaration_and_full_profile_guards(self):
        for name in ('LocalFileVolume', 'InstanceSettings', 'Server', 'User', 'ServerSetting'):
            args = self.args(name)
            presence = self.presence(name)
            variants = {0: (None, [], 'Unknown', 'PLANTED_PRIVATE_MARKER'),
                        1: (None, [], {**args[1], 'extra': 'encrypted'}) + (({},) if args[1] else ()),
                        2: (None, [], '0' * 64, 'PLANTED_PRIVATE_MARKER'),
                        3: (None, [], '4.4.3'),
                        4: (None, [], args[4][1:], args[4] + [args[4][0]])}
            for index, values in variants.items():
                for value in values:
                    bad = list(args); bad[index] = value
                    with self.subTest(name=name, index=index, value=value):
                        self.reject(lambda: evidence.persisted_encrypted_attributes(*bad, presence))
            target_source = self.catalog['models']['4.4.6'][name]
            if target_source != args[2]:
                bad = list(args); bad[2] = target_source
                self.reject(lambda: evidence.corrected_encrypted_casts(*bad))

    def test_presence_exact_keyset_and_boolean_values(self):
        for name in ('LocalFileVolume', 'InstanceSettings', 'Server', 'User', 'ServerSetting'):
            presence = self.presence(name)
            for bad in (None, [], {'extra': True}, {**presence, 'extra': True}):
                self.reject(lambda: evidence.persisted_encrypted_attributes(*self.args(name), bad))
            for attribute in presence:
                for value in (None, 0, 1, 1.0, [], {}, 'PLANTED_PRIVATE_MARKER'):
                    bad = {**presence, attribute: value}
                    self.reject(lambda: evidence.persisted_encrypted_attributes(*self.args(name), bad))

    def test_per_model_counts_allow_only_reviewed_corrections(self):
        for name in evidence.DECRYPT_MODELS:
            allowed = evidence.encrypted_column_counts(name, '4.4.2')
            expected = {1} if name == 'LocalFileVolume' else {8, 9} if name == 'InstanceSettings' else {0, 1, 2} if name == 'Server' else {len(self.catalog['encrypted_casts']['4.4.2'][name]) + (2 if name == 'User' else 0)}
            self.assertEqual(allowed, expected)
            for count in allowed:
                selected = self.fixture(); selected['decrypt_models'][name]['columns'] = count
                evidence.validate_evidence(selected, '4.4.2')
            self.mutate(('decrypt_models', name, 'columns'), max(allowed) + 1)
        for name, version in (([], '4.4.2'), ('Unknown', '4.4.2'), ('Server', []), ('Server', '4.4.3')):
            self.reject(lambda: evidence.encrypted_column_counts(name, version))

    def test_full_current_migration_history_and_only_fixed_inert_rows(self):
        for version in ('4.4.2', '4.4.6'):
            selected = self.fixture(version, history=evidence.MAINLINE_RETIRED_MIGRATIONS)
            evidence.validate_evidence(selected, version)
            current = self.catalog['migrations'][version]
            for rows in (None, [], current[:-1], current + [current[0]], current + [True],
                         current + [None], current + ['PLANTED_PRIVATE_MARKER'],
                         current + ['future_migration'], current * 24):
                self.reject(lambda: evidence.trusted_table_catalog(rows, version))
        self.reject(lambda: evidence.trusted_table_catalog(self.catalog['migrations']['4.4.6'], '4.4.2'))

    def test_required_activity_log_and_all_three_conditional_telescope_tables(self):
        history = ('2018_08_08_100000_create_telescope_entries_table',)
        for version in ('4.4.2', '4.4.6'):
            for rows in ((), history):
                selected = self.fixture(version, history=rows)
                tables = selected['table_coverage']['tables']
                for table in ('migrations', 'activity_log') + (('telescope_entries', 'telescope_entries_tags', 'telescope_monitoring') if rows else ()):
                    bad = copy.deepcopy(selected); del bad['table_coverage']['tables'][table]
                    bad['table_coverage']['known'] -= 1
                    self.reject(lambda: evidence.validate_evidence(bad, version))
                for table in ('jobs', 'cache', 'cache_locks', 'passkeys', 'PLANTED_PRIVATE_MARKER') + (() if rows else ('telescope_entries',)):
                    bad = copy.deepcopy(selected)
                    bad['table_coverage']['tables'][table] = {'count': 0, 'classification': 'runtime'}
                    bad['table_coverage']['known'] += 1
                    self.reject(lambda: evidence.validate_evidence(bad, version))

    def test_table_classification_counts_core_closure_and_migration_count(self):
        self.mutate(('table_coverage', 'tables', 'users', 'classification'), 'runtime')
        self.mutate(('table_coverage', 'tables', 'activity_log', 'classification'), 'core')
        self.mutate(('table_coverage', 'tables', 'migrations', 'count'), 436)
        self.mutate(('table_coverage', 'tables', 'users', 'count'), 3)
        self.mutate(('table_coverage', 'known'), 0)
        self.mutate(('table_coverage', 'unknown'), 1)
        bad = self.fixture(); del bad['core']['users']
        self.reject(lambda: evidence.validate_evidence(bad, '4.4.2'))

    def test_optional_table_coverage_and_inert_history_preserved_in_each_phase(self):
        history = ('2026_09_28_093936_remove_stored_commands_from_finished_activities',)
        for flags in ({}, {'startup': True}, {'forward': True}):
            before = self.fixture(history=history, baseline=flags.get('startup', False))
            before['table_coverage']['tables']['password_reset_tokens'] = {'count': 4, 'classification': 'runtime'}
            before['table_coverage']['known'] += 1
            after = self.fixture('4.4.6' if flags.get('forward') else '4.4.2', history=history)
            after['table_coverage']['tables']['password_reset_tokens'] = {'count': 0, 'classification': 'runtime'}
            after['table_coverage']['known'] += 1
            self.assertTrue(evidence.compare_evidence(before, after, **flags))
            bad = copy.deepcopy(after); del bad['table_coverage']['tables']['password_reset_tokens']
            bad['table_coverage']['known'] -= 1
            self.reject(lambda: evidence.compare_evidence(before, bad, **flags))
            bad = copy.deepcopy(after); bad['migrations'].remove(history[0])
            bad['table_coverage']['tables']['migrations']['count'] -= 1
            self.reject(lambda: evidence.compare_evidence(before, bad, **flags))

    def test_all_core_counts_and_hmacs_are_preserved_without_general_loss(self):
        for flags in ({}, {'startup': True}, {'forward': True}):
            before = self.fixture(baseline=flags.get('startup', False))
            for table in evidence.CORE:
                for key, value in (('count', 3), ('count', 5), ('digest', 'a' * 64)):
                    after = self.fixture('4.4.6' if flags.get('forward') else '4.4.2')
                    after['core'][table][key] = value
                    if key == 'count': after['table_coverage']['tables'][table]['count'] = value
                    self.reject(lambda: evidence.compare_evidence(before, after, **flags))

    def test_only_predicted_environment_deletion_forward_is_allowed(self):
        before = self.fixture()
        before['deletions'].update({'surplus': 1, 'expected_count': 3, 'expected_digest': 'c' * 64})
        target = self.fixture('4.4.6')
        target['core']['environment_variables'] = {'count': 3, 'digest': 'c' * 64}
        target['table_coverage']['tables']['environment_variables']['count'] = 3
        self.assertTrue(evidence.compare_evidence(before, target, forward=True))
        for field, value in (('surplus', 2), ('expected_count', 2), ('expected_digest', 'd' * 64),
                             ('survivor_digest', 'd' * 64)):
            bad = copy.deepcopy(before); bad['deletions'][field] = value
            self.reject(lambda: evidence.compare_evidence(bad, target, forward=True))
        old = copy.deepcopy(target); old['version'] = '4.4.2'
        old['migrations'] = list(self.catalog['migrations']['4.4.2'])
        old['table_coverage']['tables']['migrations']['count'] = len(old['migrations'])
        self.reject(lambda: evidence.compare_evidence(before, old))

    def test_forward_requires_zero_ipv6_and_seeder_identity_unchanged(self):
        before, after = self.fixture(), self.fixture('4.4.6')
        for key in ('servers', 'sentinel'):
            bad = copy.deepcopy(after); bad['ipv6_affected'][key] = 1
            self.reject(lambda: evidence.compare_evidence(before, bad, forward=True))
        for table in before['seeder']:
            for value in ([], [{'id': 1, 'uuid_digest': 'b' * 64}]):
                bad = copy.deepcopy(after); bad['seeder'][table] = value
                self.reject(lambda: evidence.compare_evidence(before, bad, forward=True))


if __name__ == '__main__':
    unittest.main()
