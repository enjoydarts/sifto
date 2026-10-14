"""Offline Coolify schema 2 and universal signed-plan protocol.

This module validates metadata, not backup capability or execution permission.
The only public profile is ineligible for execution: ordinary Coolify startup
has effects beyond an image replacement. Owner approval history, root runtime
policy, once-only execution and receipts belong to later integrations.

Profile identity is SHA256 of canonical JSON of the complete public profile;
bundle identity is SHA256 of canonical JSON of its ``bundle`` member. Public
artifact hashes bind provenance bytes only; the scripts must never be executed.
No profile path, executable, network operation or runtime configuration is read.
"""
import base64
import copy
import time
import uuid

from cryptography.exceptions import InvalidSignature
from cryptography.hazmat.primitives.asymmetric.ed25519 import Ed25519PrivateKey, Ed25519PublicKey

from . import plan as legacy
from .plan import HASH, HOST, SHA, MaintenanceError, canonical_json, matches, plan_sha256, require

DAY = 86400
PUBLICATION = 1791571325
NOT_BEFORE = PUBLICATION + 7 * DAY
PROFILE_ID = 'coolify-4.4.2-to-4.4.6-arm64-v1'
OLD_ARM = 'sha256:a323ce1dfb54101b0c30d8bd5c92b0e4166c3b403a164316f8876be653204960'
NEW_ARM = 'sha256:f4ef1eb69daa265057ab700646485bd78bb43bcf96c691d65094947acb7f50a5'
_ARTIFACTS = {
    'docker-compose.yml': '2532ee9258e10ad4e86cb3c2b46c29fe9329f5a0dace0f7da410744feec279bd',
    'docker-compose.prod.yml': '4cd00e83e117e847e6d6bdfe64f2a2f958bb09965692c5230fa9231a0315f80f',
    '.env.production': '7895e54f7c9d40063e76afec9b35839048d91e5c3b5107d64ea49ef6987c304a',
    'scripts/upgrade.sh': '8dc02adee5ea0f27027405243a15e9be8bced5ea290870de90c8ec0ce4979fda',
    'scripts/upgrade-postgres.sh': 'aa65c277fc13ab5b27c349fbfef12defc59c084b244e4b258329e1f6c570ae8d',
}
_DEPENDENCIES = {
    'postgres': {'repository': 'docker.io/library/postgres',
                 'index_digest': 'sha256:f7d23353e1b15400d22ebe31189f4d314b87a4c129cc400c8c2d8d4ca127bf81',
                 'platform_manifest': 'sha256:91a22c143801df42c2e4a8f9926eab5e48f39f40beb5220b7c29d4478263c341'},
    'redis': {'repository': 'docker.io/library/redis',
              'index_digest': 'sha256:858f009f9709ce576febc734aa78b8f6d624b82571f9ddb6bda4377c833b3499',
              'platform_manifest': 'sha256:1f09a89a207d794a8c61d9edfc26e7c58427de10ccef7c5d18d638df79a63b85'},
}
_PROFILE = {
    'schema_version': 1, 'profile_id': PROFILE_ID,
    'publication_at': PUBLICATION, 'not_before': NOT_BEFORE,
    'controlled_startup_verified': False,
    'controlled_startup_limitations':
        'Normal boot has remote Seeder and AppInit side effects and a maintenance webhook exception; '
        'controlled startup and an image-only path have not been verified.',
    'new_migrations': [
        '2026_10_09_092903_normalize_ipv6_server_ips',
        '2026_10_09_115444_remove_duplicate_redis_username_environment_variables',
        '2026_10_09_124641_remove_duplicate_nixpacks_node_version_environment_variables',
    ],
    'bundle': {
        'schema_version': 1, 'platform': 'linux/arm64',
        'rollback': {'version': '4.4.2', 'release_commit': '76d3386a4d69f894258210ea265717a8fb8cd778',
                     'artifacts': dict(_ARTIFACTS), 'images': {
                         **copy.deepcopy(_DEPENDENCIES),
                         'coolify': {'repository': 'docker.io/coollabsio/coolify',
                                     'index_digest': 'sha256:cd884496945d4a2656a480401238e1dda4a1569424102df7474ce3ac73550cd6',
                                     'platform_manifest': OLD_ARM}}},
        'target': {'version': '4.4.6', 'release_commit': 'f55efc859dfcbbf79274c4907b3df89f9767c11d',
                   'artifacts': dict(_ARTIFACTS), 'images': {
                       **copy.deepcopy(_DEPENDENCIES),
                       'coolify': {'repository': 'docker.io/coollabsio/coolify',
                                   'platform_manifest': NEW_ARM}}},
    },
}

PLAN_FIELDS = {
    'schema_version', 'action', 'plan_id', 'host_id', 'created_at', 'expires_at',
    'workflow_sha', 'state_sha256', 'profile_id', 'profile_sha256', 'bundle_sha256',
    'from_version', 'to_version', 'from_platform_manifest', 'to_platform_manifest',
    'publication_at', 'not_before', 'source_configuration_sha256', 'dependencies_sha256',
    'protected_runtime_sha256', 'barrier_policy_sha256', 'backup_binding', 'rehearsal_binding',
}
BACKUP_FIELDS = {'snapshot_id', 'proof_sha256', 'file_sha256', 'verified_at',
                 'offhost_verified_at', 'restore_tested_at', 'expires_at'}
REHEARSAL_FIELDS = {'receipt_sha256', 'profile_sha256', 'source_configuration_sha256',
                    'snapshot_id', 'file_sha256', 'old_boot', 'target_migrations',
                    'fresh_original_restore', 'verified_at', 'expires_at'}
BACKUP_FILES = {'coolify.dump', 'coolify-config.tar.gz', 'created_at'}


def _bounded(value):
    """Reject untrusted nesting/size/types before canonical encoding or lookups."""
    pending = [(value, 0)]
    count = 0
    while pending:
        selected, depth = pending.pop()
        count += 1
        require(count <= 512 and depth <= 8, 'invalid protocol data bounds')
        if type(selected) is dict:
            require(len(selected) <= 64 and all(type(key) is str and len(key) <= 128
                                              for key in selected), 'invalid protocol object')
            pending.extend((item, depth + 1) for item in selected.values())
        elif type(selected) is list:
            require(len(selected) <= 64, 'invalid protocol array')
            pending.extend((item, depth + 1) for item in selected)
        elif type(selected) is str:
            require(len(selected) <= 256, 'invalid protocol text bounds')
        elif type(selected) is int:
            require(0 <= selected <= 2**63 - 1, 'invalid protocol integer')
        else:
            require(type(selected) is bool or selected is None, 'invalid protocol value')
    require(len(canonical_json(value)) <= 16384, 'invalid protocol byte bounds')


def fixed_profile():
    """Return a detached public profile, never a caller-selected filename."""
    return copy.deepcopy(_PROFILE)


def profile_sha256():
    return plan_sha256(_PROFILE)


def bundle_sha256():
    return plan_sha256(_PROFILE['bundle'])


def validate_profile(profile):
    _bounded(profile)
    require(canonical_json(profile) == canonical_json(_PROFILE), 'unsupported or altered Coolify profile')
    return profile


def profile_eligibility():
    """A signed plan cannot turn the unverified public profile into permission."""
    return {'profile_id': PROFILE_ID, 'eligible': False,
            'reason': _PROFILE['controlled_startup_limitations']}


def _schema(value):
    require(type(value) is dict and type(value.get('schema_version')) is int
            and value['schema_version'] in (1, 2), 'unsupported plan schema')
    return value['schema_version']


def _now(now):
    value = int(time.time()) if now is None else now
    require(type(value) is int and 0 <= value <= 2**63 - 1, 'invalid trusted clock')
    return value


def _fields(value, expected, category):
    require(type(value) is dict and set(value) == expected, category)


def _files(value):
    _fields(value, BACKUP_FILES, 'invalid backup file closure')
    require(all(matches(HASH, digest) for digest in value.values()), 'invalid backup file hashes')


def _fresh(value, now):
    require(type(value) is int and 0 <= value <= now and now - value < DAY,
            'stale or invalid proof timestamp')


def _expiry(value, now, ceiling):
    require(type(value) is int and now < value <= ceiling, 'invalid proof expiry')


def _validate_v2(value, now):
    _bounded(value)
    _fields(value, PLAN_FIELDS, 'invalid Coolify plan fields')
    require(value['action'] == 'coolify_patch', 'unsupported Coolify action')
    try:
        require(str(uuid.UUID(value['plan_id'])) == value['plan_id'], 'invalid plan ID')
    except (ValueError, TypeError, AttributeError):
        raise MaintenanceError('invalid plan ID') from None
    require(matches(HOST, value['host_id']), 'invalid host ID')
    require(matches(SHA, value['workflow_sha']), 'invalid workflow SHA')
    for field in ('state_sha256', 'profile_sha256', 'bundle_sha256', 'source_configuration_sha256',
                  'dependencies_sha256', 'protected_runtime_sha256', 'barrier_policy_sha256'):
        require(matches(HASH, value[field]), 'invalid Coolify binding hash')
    require(value['profile_id'] == PROFILE_ID and value['profile_sha256'] == profile_sha256()
            and value['bundle_sha256'] == bundle_sha256(), 'unsupported Coolify profile binding')
    require(value['from_version'] == '4.4.2' and value['to_version'] == '4.4.6'
            and value['from_platform_manifest'] == OLD_ARM and value['to_platform_manifest'] == NEW_ARM,
            'unsupported Coolify transition')
    require(type(value['publication_at']) is int and value['publication_at'] == PUBLICATION
            and type(value['not_before']) is int and value['not_before'] == NOT_BEFORE,
            'invalid Coolify release age binding')
    require(now >= NOT_BEFORE, 'Coolify release age gate not reached')
    require(type(value['created_at']) is int and type(value['expires_at']) is int
            and 0 <= value['created_at'] <= now < value['expires_at'] <= value['created_at'] + DAY,
            'invalid Coolify plan lifetime')
    backup = value['backup_binding']
    _fields(backup, BACKUP_FIELDS, 'invalid backup binding fields')
    require(matches(HASH, backup['snapshot_id']) and matches(HASH, backup['proof_sha256']),
            'invalid backup binding hashes')
    _files(backup['file_sha256'])
    times = [backup[field] for field in ('verified_at', 'offhost_verified_at', 'restore_tested_at')]
    for timestamp in times:
        _fresh(timestamp, now)
    _expiry(backup['expires_at'], now, min(times) + DAY)
    rehearsal = value['rehearsal_binding']
    _fields(rehearsal, REHEARSAL_FIELDS, 'invalid rehearsal binding fields')
    require(matches(HASH, rehearsal['receipt_sha256']), 'invalid rehearsal receipt hash')
    _files(rehearsal['file_sha256'])
    require(rehearsal['profile_sha256'] == value['profile_sha256']
            and rehearsal['source_configuration_sha256'] == value['source_configuration_sha256']
            and rehearsal['snapshot_id'] == backup['snapshot_id']
            and rehearsal['file_sha256'] == backup['file_sha256'], 'rehearsal binding mismatch')
    require(all(rehearsal[field] is True for field in
                ('old_boot', 'target_migrations', 'fresh_original_restore')), 'unverified rehearsal result')
    _fresh(rehearsal['verified_at'], now)
    _expiry(rehearsal['expires_at'], now, min(rehearsal['verified_at'] + DAY, backup['expires_at']))
    require(value['expires_at'] <= min(backup['expires_at'], rehearsal['expires_at']),
            'Coolify plan outlives proof bindings')
    return value


def validate_plan(value, *, now=None):
    if _schema(value) == 1:
        return legacy.validate_plan(value)
    return _validate_v2(value, _now(now))


def _approval(value, approval, now):
    _bounded(approval)
    legacy.validate_approval(value, approval)
    require(approval['approved_at'] <= now < approval['expires_at'], 'approval expired or not yet valid')


def sign_plan(value, approval, private_key, *, now=None):
    if _schema(value) == 1:
        return legacy.sign_plan(value, approval, private_key)
    now = _now(now)
    _validate_v2(value, now)
    _approval(value, approval, now)
    body = {'plan': value, 'approval': approval}
    try:
        if isinstance(private_key, bytes):
            private_key = Ed25519PrivateKey.from_private_bytes(private_key)
        require(isinstance(private_key, Ed25519PrivateKey), 'invalid signing key')
        signature = private_key.sign(canonical_json(body))
    except (ValueError, TypeError):
        raise MaintenanceError('invalid signing key') from None
    return {**body, 'signature': base64.b64encode(signature).decode('ascii')}


def verify_envelope(envelope, public_key, host_id, workflow_sha, *, now=None):
    _fields(envelope, {'plan', 'approval', 'signature'}, 'invalid envelope')
    if _schema(envelope['plan']) == 1:
        return legacy.verify_envelope(envelope, public_key, host_id, workflow_sha, now=now)
    _bounded(envelope)
    now = _now(now)
    value, approval = envelope['plan'], envelope['approval']
    _validate_v2(value, now)
    _approval(value, approval, now)
    require(value['host_id'] == host_id, 'host mismatch')
    require(value['workflow_sha'] == workflow_sha, 'workflow mismatch')
    try:
        if isinstance(public_key, str):
            require(len(public_key) <= 44, 'invalid approval signature')
            public_key = base64.b64decode(public_key, validate=True)
        if isinstance(public_key, bytes):
            public_key = Ed25519PublicKey.from_public_bytes(public_key)
        require(isinstance(public_key, Ed25519PublicKey), 'invalid approval signature')
        require(type(envelope['signature']) is str and len(envelope['signature']) == 88,
                'invalid approval signature')
        signature = base64.b64decode(envelope['signature'], validate=True)
        require(base64.b64encode(signature).decode('ascii') == envelope['signature'],
                'invalid approval signature')
        public_key.verify(signature, canonical_json({'plan': value, 'approval': approval}))
    except (InvalidSignature, ValueError, TypeError):
        raise MaintenanceError('invalid approval signature') from None
    return value
