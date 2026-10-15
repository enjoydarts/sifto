"""Pure evidence gates for the fixed, disabled 4.4.2 -> 4.4.6 profile.

Ports the public operator 27b67089 evidence rules only. The bundled catalog is
authenticated public metadata; no PHP, SQL, private material or host operation
is executed. Synthetic acceptance cannot establish a recovery proof or enable
the native profile. Task3a remains responsible for private material validation.
"""
import hashlib
import json
from pathlib import Path
import re

from . import coolify_plan
from .coolify_rehearsal import RehearsalError
from .plan import MaintenanceError

MANIFEST_SHA = 'b37aff8b86a8ff20372ef5fb7e9bba40ba7b192f899b649dce0c9d0daa61e276'
_MANIFEST_SIZE = 80947
_MAX_BYTES = 512 * 1024
_MAX_COUNT = 10_000_000
_VERSIONS = ('4.4.2', '4.4.6')
_HEX = re.compile(r'[0-9a-f]{64}\Z')
NEW_MIGRATIONS = (
    '2026_10_09_092903_normalize_ipv6_server_ips',
    '2026_10_09_115444_remove_duplicate_redis_username_environment_variables',
    '2026_10_09_124641_remove_duplicate_nixpacks_node_version_environment_variables',
)
# Inert names only: fixed first-parent retirement catalog SHA256
# 44798287f34608bdb47ce9f4f4643026602ec1c7515b2d2aa40d7b4dd4460e85.
# These names never select executable SQL or restored scripts.
MAINLINE_RETIRED_MIGRATIONS = (
    '2018_08_08_100000_create_telescope_entries_table',
    '2019_08_19_000000_create_failed_jobs_table',
    '2023_03_20_112810_create_jobs_table',
    '2023_03_20_112813_create_coolify_instance_settings_table',
    '2023_03_20_112813_create_instance_settings_table',
    '2023_03_24_203356_create_private_keyables_table',
    '2023_03_27_083620_create_databases_table',
    '2023_03_27_085022_create_standalone_dockers_table',
    '2023_03_27_085023_create_swarm_dockers_table',
    '2023_03_28_083722_create_gits_table',
    '2023_03_28_100003_create_git_deploy_keys_table',
    '2023_06_19_084152_add_git_related_private_key',
    '2025_10_10_120000_create_cloud_init_scripts_table',
    '2025_10_10_120000_create_webhook_notification_settings_table',
    '2025_10_10_120001_populate_webhook_notification_settings_for_existing_teams',
    '2025_10_10_120002_create_cloud_init_scripts_table',
    '2025_10_10_120002_create_webhook_notification_settings_table',
    '2025_11_25_000001_create_webhook_notification_settings_table',
    '2025_11_25_000002_create_cloud_init_scripts_table',
    '2026_09_28_093936_remove_stored_commands_from_finished_activities',
    '2026_09_28_194652_add_next_run_at_to_scheduled_jobs',
)
RUNTIME_TABLES = frozenset((
    'audit_events', 'docker_cleanup_executions', 'github_runner_executions',
    'notification_throttles', 'scheduled_database_backup_executions',
    'scheduled_job_deliveries', 'scheduled_job_states', 'scheduled_task_executions',
    'scheduled_volume_backup_executions', 'user_changelog_reads',
    'application_deployment_queues', 'jobs', 'failed_jobs', 'job_batches',
    'cache', 'cache_locks', 'sessions', 'notifications', 'migrations', 'activity_log',
    'telescope_entries', 'telescope_entries_tags', 'telescope_monitoring',
))


def _require(condition: bool, category: str) -> None:
    if not condition:
        raise RehearsalError(category)


def _bounded(value: object) -> None:
    """Bound the entire graph before encoding, hashing, sorting or lookups."""
    pending = [(value, 0)]
    nodes = size = 0
    while pending:
        selected, depth = pending.pop()
        nodes += 1
        size += 16
        _require(nodes <= 20000 and depth <= 8, 'evidence_bounds')
        if type(selected) is dict:
            _require(len(selected) <= 1000, 'evidence_bounds')
            for key in selected:
                _require(type(key) is str and len(key) <= 128, 'evidence_bounds')
                size += 6 * len(key) + 4
            pending.extend((item, depth + 1) for item in selected.values())
        elif type(selected) is list:
            _require(len(selected) <= 10000, 'evidence_bounds')
            pending.extend((item, depth + 1) for item in selected)
        elif type(selected) is str:
            _require(len(selected) <= 1024, 'evidence_bounds')
            size += 6 * len(selected)
        elif type(selected) is int:
            _require(0 <= selected <= _MAX_COUNT, 'evidence_bounds')
        else:
            _require(type(selected) is bool or selected is None, 'evidence_bounds')
        _require(size <= _MAX_BYTES, 'evidence_bounds')


def _read_catalog() -> dict:
    try:
        # Literal package resource only; callers cannot choose a path or policy.
        with Path(__file__).with_name('coolify-public-manifest.json').open('rb') as stream:
            payload = stream.read(_MANIFEST_SIZE + 1)
        _require(len(payload) == _MANIFEST_SIZE
                 and hashlib.sha256(payload).hexdigest() == MANIFEST_SHA, 'evidence_manifest')
        public = json.loads(payload)
        profile = coolify_plan.fixed_profile()
        coolify_plan.validate_profile(profile)
        _require(tuple(profile['new_migrations']) == NEW_MIGRATIONS, 'evidence_manifest')
        for role in ('rollback', 'target'):
            selected = profile['bundle'][role]
            _require(public['revisions'][selected['version']] == selected['release_commit'],
                     'evidence_manifest')
        old, target = (set(public['migrations'][version]) for version in _VERSIONS)
        _require(len(old) == 435 and len(target) == 438 and old <= target
                 and target - old == set(NEW_MIGRATIONS), 'evidence_manifest')
        return public
    except (OSError, ValueError, TypeError, KeyError, MaintenanceError):
        raise RehearsalError('evidence_manifest') from None


def fixed_catalog() -> dict:
    """Authenticate the fixed resource and return an independent public copy."""
    return _read_catalog()


# This private snapshot is authenticated before any derived constants are used.
# Public access returns a freshly authenticated, detached catalog, never this map.
_PUBLIC = _read_catalog()
CORE = tuple(sorted((set(_PUBLIC['model_tables']['4.4.2'].values())
                     | {'team_user', 'taggables'}) - RUNTIME_TABLES))
DECRYPT_MODELS = tuple(sorted(set(_PUBLIC['models']['4.4.2']) - {'BaseModel'}))


def _version(version: object, category: str) -> None:
    _require(type(version) is str and version in _VERSIONS, category)


def corrected_encrypted_casts(name: object, declared: object, source_hash: object,
                              version: object, expected_migrations: object) -> dict[str, str]:
    _bounded([name, declared, source_hash, version, expected_migrations])
    _version(version, 'evidence_decrypt')
    _require(type(name) is str and name in _PUBLIC['models'][version], 'evidence_decrypt')
    _require(type(declared) is dict and declared == _PUBLIC['encrypted_casts'][version][name]
             and type(source_hash) is str and source_hash == _PUBLIC['models'][version][name],
             'evidence_decrypt')
    _require(type(expected_migrations) is list
             and expected_migrations == _PUBLIC['migrations'][version], 'evidence_decrypt')
    attributes = dict(declared)
    if name == 'LocalFileVolume':
        _require(source_hash == 'f0bc1eb914bcb8a009c372a71babd87e4f4f2eb29737b4a1fa9b041f878ab468'
                 and attributes == dict.fromkeys(('content', 'fs_path', 'mount_path'), 'encrypted'),
                 'evidence_decrypt')
        return {'content': 'encrypted'}
    if name == 'InstanceSettings':
        _require(source_hash == 'a72d4ebbcbe9acf9443dd8dfa7ec2f82f51f08fdf45d67450a94ea4c7a2524d1'
                 and attributes == dict.fromkeys((
                     'domain_connect_private_key', 'resend_api_key', 'sentinel_token',
                     'smtp_from_address', 'smtp_from_name', 'smtp_host', 'smtp_password',
                     'smtp_recipients', 'smtp_username'), 'encrypted'), 'evidence_decrypt')
    if name == 'Server':
        sources = {
            '4.4.2': 'adc7e42d8b6d665afcb7d4f35752d8ba06c81edeaa6afd8a99b40a20a3b3eaeb',
            '4.4.6': '2f9b1de185caf15b1cb804bfcfa5c66c9e5e54650d4521b59c55885fdb17e7d4',
        }
        profiles = {
            '4.4.2': 'bd4d2bdad3d09f147f6a2726f021176d18b2dc62816effe700a09678e5fedbfe',
            '4.4.6': '258df8c9e851c19fa53d10f9b8346de9a3add4d119c270352c8893c85815bf9a',
        }
        _require(source_hash == sources[version] and attributes == dict.fromkeys((
            'logdrain_axiom_api_key', 'logdrain_newrelic_license_key'), 'encrypted')
            and len(expected_migrations) == (435 if version == '4.4.2' else 438)
            and hashlib.sha256(json.dumps(sorted(expected_migrations),
                                         separators=(',', ':')).encode()).hexdigest() == profiles[version],
            'evidence_decrypt')
    return attributes


def persisted_encrypted_attributes(name: object, declared: object, source_hash: object,
                                   version: object, expected_migrations: object,
                                   presence: object) -> dict[str, str]:
    _bounded(presence)
    attributes = corrected_encrypted_casts(name, declared, source_hash, version, expected_migrations)
    if name == 'User':
        attributes.update({'two_factor_secret': 'fortify:string',
                           'two_factor_recovery_codes': 'fortify:string'})
    _require(type(presence) is dict and set(presence) == set(attributes)
             and all(type(item) is bool for item in presence.values()), 'evidence_decrypt')
    for attribute in tuple(attributes):
        if presence[attribute] is False and (
                (name == 'Server' and attribute in (
                    'logdrain_axiom_api_key', 'logdrain_newrelic_license_key'))
                or (name == 'InstanceSettings' and attribute == 'sentinel_token')):
            del attributes[attribute]
        else:
            _require(presence[attribute] is True, 'evidence_decrypt')
    return attributes


def encrypted_column_counts(name: object, version: object) -> set[int]:
    _bounded([name, version])
    _version(version, 'evidence_decrypt')
    _require(type(name) is str and name in _PUBLIC['models'][version], 'evidence_decrypt')
    declared = _PUBLIC['encrypted_casts'][version][name]
    source_hash = _PUBLIC['models'][version][name]
    migrations = _PUBLIC['migrations'][version]
    attributes = corrected_encrypted_casts(name, declared, source_hash, version, migrations)
    if name == 'User':
        attributes.update({'two_factor_secret': 'fortify:string',
                           'two_factor_recovery_codes': 'fortify:string'})
    presence = dict.fromkeys(attributes, True)
    variants = [presence]
    if name == 'InstanceSettings':
        variants.append({**presence, 'sentinel_token': False})
    if name == 'Server':
        variants = [{'logdrain_axiom_api_key': axiom, 'logdrain_newrelic_license_key': newrelic}
                    for axiom in (False, True) for newrelic in (False, True)]
    return {len(persisted_encrypted_attributes(name, declared, source_hash, version, migrations, item))
            for item in variants}


def trusted_table_catalog(migrations: object, version: object) -> tuple[set[str], set[str]]:
    _bounded([migrations, version])
    _version(version, 'evidence_migrations')
    current = set(_PUBLIC['migrations'][version])
    allowed_history = current | set(MAINLINE_RETIRED_MIGRATIONS)
    _require(type(migrations) is list
             and all(type(item) is str and item in allowed_history for item in migrations)
             and len(set(migrations)) == len(migrations) and current <= set(migrations),
             'evidence_migrations')
    # activity_log comes from the reviewed Spatie 4.12.3 source profile.
    allowed = set(_PUBLIC['tables']) | {'migrations', 'activity_log'}
    required = set(CORE) | {'migrations', 'activity_log'}
    if '2018_08_08_100000_create_telescope_entries_table' in migrations:
        # Historical public source SHA256
        # 685119f4a2ae53a57cae6a43f875660f5f1cd8e830d3f6cd12b227c279172881.
        telescope = {'telescope_entries', 'telescope_entries_tags', 'telescope_monitoring'}
        allowed |= telescope
        required |= telescope
    return allowed, required


def _fields(value: object, keys: set[str], category: str) -> None:
    _require(type(value) is dict and set(value) == keys, category)


def _count(value: object) -> bool:
    return type(value) is int and 0 <= value <= _MAX_COUNT


def _digest(value: object) -> bool:
    return type(value) is str and _HEX.fullmatch(value) is not None


def validate_evidence(value: object, version: object, baseline: bool = False) -> dict:
    _bounded(value)
    _version(version, 'evidence_schema')
    _require(type(baseline) is bool, 'evidence_schema')
    _fields(value, {'ok', 'category', 'version', 'db', 'no_pending', 'network', 'wrong_key',
                    'ssh', 'decrypt', 'decrypt_models', 'table_coverage', 'ipv6_affected',
                    'migrations', 'core', 'deletions', 'seeder', 'ssh_checked'}, 'evidence_schema')
    _require(value['ok'] is True and value['category'] == 'verified'
             and value['version'] == version, 'evidence_schema')
    _require(all(value[key] is True for key in ('db', 'no_pending', 'network', 'wrong_key')),
             'evidence_gate')
    _require(type(value['ssh']) is bool and _count(value['ssh_checked']), 'evidence_ssh')
    if not baseline:
        _require(value['ssh'] is True and value['ssh_checked'] > 0, 'evidence_ssh')
    decrypt = value['decrypt']
    _fields(decrypt, {'checked', 'nonempty', 'missing', 'failures'}, 'evidence_decrypt')
    _require(all(_count(item) for item in decrypt.values())
             and 0 < decrypt['nonempty'] <= decrypt['checked'] and decrypt['failures'] == 0,
             'evidence_decrypt')
    allowed_tables, required_tables = trusted_table_catalog(value['migrations'], version)
    per_model = value['decrypt_models']
    _fields(per_model, set(DECRYPT_MODELS), 'evidence_decrypt')
    for model, counters in per_model.items():
        _fields(counters, {'checked', 'failures', 'columns'}, 'evidence_decrypt')
        _require(all(_count(item) for item in counters.values()) and counters['failures'] == 0
                 and counters['columns'] in encrypted_column_counts(model, version), 'evidence_decrypt')
    _require(sum(counters['checked'] for counters in per_model.values()) == decrypt['checked'],
             'evidence_decrypt')
    coverage = value['table_coverage']
    _fields(coverage, {'known', 'unknown', 'tables'}, 'evidence_core')
    tables = coverage['tables']
    _require(_count(coverage['known']) and _count(coverage['unknown']) and coverage['unknown'] == 0
             and type(tables) is dict and coverage['known'] == len(tables)
             and set(tables) <= allowed_tables and required_tables <= set(tables), 'evidence_core')
    for table, counters in tables.items():
        _fields(counters, {'count', 'classification'}, 'evidence_core')
        _require(_count(counters['count']) and counters['classification'] == (
            'core' if table in CORE else 'runtime'), 'evidence_core')
    _require(tables['migrations']['count'] == len(value['migrations']), 'evidence_core')
    ipv6 = value['ipv6_affected']
    _fields(ipv6, {'servers', 'sentinel'}, 'evidence_core')
    _require(all(_count(item) for item in ipv6.values()), 'evidence_core')
    _fields(value['core'], set(CORE), 'evidence_core')
    for table, record in value['core'].items():
        _fields(record, {'count', 'digest'}, 'evidence_core')
        _require(_count(record['count']) and _digest(record['digest'])
                 and tables[table]['count'] == record['count'], 'evidence_core')
    deletion = value['deletions']
    _fields(deletion, {'surplus', 'survivor_digest', 'expected_digest', 'expected_count'},
            'evidence_deletion')
    _require(_count(deletion['surplus']) and _count(deletion['expected_count'])
             and all(_digest(deletion[key]) for key in ('survivor_digest', 'expected_digest')),
             'evidence_deletion')
    _fields(value['seeder'], {'users', 'teams', 'servers', 'private_keys'}, 'evidence_seeder')
    for records in value['seeder'].values():
        _require(type(records) is list and len(records) <= 8, 'evidence_seeder')
        for record in records:
            _fields(record, {'id', 'uuid_digest'}, 'evidence_seeder')
            _require(type(record['id']) is int and 0 <= record['id'] <= 1
                     and _digest(record['uuid_digest']), 'evidence_seeder')
    return value


def compare_evidence(before: object, after: object, *, forward: bool = False,
                     startup: bool = False) -> bool:
    _require(type(forward) is bool and type(startup) is bool and not (forward and startup),
             'evidence_schema')
    validate_evidence(before, '4.4.2', baseline=startup)
    validate_evidence(after, '4.4.6' if forward else '4.4.2')
    _require(set(after['table_coverage']['tables']) == set(before['table_coverage']['tables']),
             'core_difference')
    _require(set(after['migrations']) == set(before['migrations'])
             | (set(NEW_MIGRATIONS) if forward else set()), 'migration_difference')
    if forward:
        _require(not (set(before['migrations']) & set(NEW_MIGRATIONS)), 'migration_baseline')
    for table in CORE:
        expected = before['core'][table]
        if forward and table == 'environment_variables':
            expected = {'count': before['deletions']['expected_count'],
                        'digest': before['deletions']['expected_digest']}
        _require(after['core'][table] == expected, 'core_difference')
    if forward:
        _require(before['core']['environment_variables']['count']
                 - after['core']['environment_variables']['count'] == before['deletions']['surplus']
                 and before['deletions']['survivor_digest'] == after['deletions']['survivor_digest'],
                 'deletion_difference')
        _require(after['ipv6_affected'] == {'servers': 0, 'sentinel': 0}, 'migration_difference')
    _require(before['seeder'] == after['seeder'], 'seeder_difference')
    return True
