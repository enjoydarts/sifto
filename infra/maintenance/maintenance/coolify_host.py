"""Selected read-only metadata for the reviewed Coolify patch profile.

This collector cannot grant update permission. Production recreation, barriers,
credentials and full recovery evidence remain unverified until their own fixed
probes are implemented. No arbitrary commands, private env or proof requests.
"""
import copy
import hashlib
import os
from pathlib import Path
import re
import stat
import time

from . import CONTAINER_FORMAT, ContainerIdentityError, normalize_recreation_identity
from . import coolify_plan
from .plan import HASH, MaintenanceError, matches, plan_sha256, require

_BOUNDS = {'memory_bytes': 1280 * 1024 * 1024,
           'host_reserve_memory_bytes': 1024 * 1024 * 1024,
           'staging_free_bytes': 4 * 1024**3, 'cpu_millis': 1750,
           'command_timeout_seconds': 30}
_DIGEST = re.compile(r'sha256:[0-9a-f]{64}\Z')
_CONTAINER_FIELDS = {'id', 'image_id', 'image', 'status', 'health',
                     'recreation_identity', 'runtime_flags'}
_FLAGS = {'entrypoint_present', 'command_present', 'user_present', 'privileged'}
_IMAGE_FIELDS = {'engine_id', 'os', 'architecture', 'descriptor'}
_SOURCE = Path('/data/coolify/source')
_STAGING = Path('/var/lib/sifto-maintenance')

# Do not return Cmd, Entrypoint or User strings: any of them could be a private
# override. Presence alone is observation, never proof of supported startup.
SELECTED_CONTAINER_FORMAT = CONTAINER_FORMAT[:-1] + (
    ',"runtime_flags":{"entrypoint_present":{{if .Config.Entrypoint}}true{{else}}false{{end}},'
    '"command_present":{{if .Config.Cmd}}true{{else}}false{{end}},'
    '"user_present":{{if .Config.User}}true{{else}}false{{end}},'
    '"privileged":{{json .HostConfig.Privileged}}}}')
SELECTED_IMAGE_FORMAT = (
    '{"engine_id":{{json .Id}},"os":{{json .Os}},"architecture":{{json .Architecture}},'
    '"descriptor":{{with index . "Descriptor"}}'
    '{"digest":{{json .digest}},"mediaType":{{json .mediaType}}}{{else}}null{{end}}}')


def resource_bounds():
    return copy.deepcopy(_BOUNDS)


def validate_policy(value):
    require(type(value) is dict and set(value) == {'enabled', 'profile_id', 'resources'},
            'invalid Coolify root policy')
    require(type(value['enabled']) is bool and value['profile_id'] == coolify_plan.PROFILE_ID,
            'unsupported Coolify root policy')
    resources = value['resources']
    require(type(resources) is dict and set(resources) == set(_BOUNDS)
            and all(type(resources[key]) is int and resources[key] == expected
                    for key, expected in _BOUNDS.items()), 'invalid Coolify resource bounds')
    return value


def _json(raw):
    from .host import strict_json
    require(type(raw) is str and len(raw.encode('utf-8')) <= 1024 * 1024,
            'selected output exceeds bounds')
    return strict_json(raw)


def _selected_runtime(runner):
    require(runner(['dpkg', '--print-architecture'], timeout=10).strip() == 'arm64',
            'unsupported selected platform')
    raw = runner(['docker', 'ps', '-aq', '--no-trunc'], timeout=30)
    require(type(raw) is str and len(raw) <= 65000, 'invalid selected container IDs')
    identifiers = raw.split()
    require(len(identifiers) <= 1000 and len(set(identifiers)) == len(identifiers)
            and all(matches(HASH, value) for value in identifiers), 'invalid selected container IDs')
    require(identifiers, 'selected control plane unavailable')
    raw = runner(['docker', 'inspect', '--format', SELECTED_CONTAINER_FORMAT, *identifiers], timeout=30)
    # Bound total input before allocating/decoding individual rows.
    require(type(raw) is str and len(raw.encode('utf-8')) <= 1024 * 1024,
            'selected output exceeds bounds')
    rows = [_json(line) for line in raw.splitlines()]
    require(len(rows) == len(identifiers) and all(type(row) is dict and set(row) == _CONTAINER_FIELDS
            for row in rows) and {row['id'] for row in rows} == set(identifiers),
            'invalid selected inspection closure')
    running = [row for row in rows if row['status'] == 'running']
    selected = {}
    for role, name in [('coolify', '/coolify'), ('postgres', '/coolify-db'), ('redis', '/coolify-redis')]:
        candidates = [row for row in running if type(row['recreation_identity']) is dict
                      and row['recreation_identity'].get('name') == name]
        require(len(candidates) == 1, 'ambiguous selected control plane')
        selected[role] = candidates[0]
    # Also reject an additional running Coolify image under a different name.
    require(sum(type(row['image']) is str and re.match(
        r'(?:ghcr\.io/|docker\.io/)?coollabsio/coolify(?=[:@]|$)', row['image']) is not None
        for row in running) == 1, 'ambiguous selected control plane')
    require(len({row['id'] for row in selected.values()}) == 3, 'ambiguous selected identity')
    public, stable = {}, {}
    profile = coolify_plan.fixed_profile()['bundle']
    current_version = None
    for role, row in selected.items():
        require(matches(HASH, row['id']) and matches(_DIGEST, row['image_id'])
                and row['health'] in ('healthy', 'unhealthy', 'starting', 'none'),
                'invalid selected identity')
        flags = row['runtime_flags']
        require(type(flags) is dict and set(flags) == _FLAGS
                and all(type(value) is bool for value in flags.values()) and not flags['privileged'],
                'unsupported selected runtime flags')
        image = _json(runner(['docker', 'image', 'inspect', '--format', SELECTED_IMAGE_FORMAT,
                             row['image_id']], timeout=30))
        require(type(image) is dict and set(image) == _IMAGE_FIELDS
                and image['engine_id'] == row['image_id'] and image['os'] == 'linux'
                and image['architecture'] == 'arm64', 'unsupported selected image platform')
        descriptor = image['descriptor']
        require(type(descriptor) is dict and set(descriptor) == {'digest', 'mediaType'}
                and matches(_DIGEST, descriptor['digest']) and descriptor['mediaType'] in (
                    'application/vnd.oci.image.manifest.v1+json',
                    'application/vnd.docker.distribution.manifest.v2+json'),
                'platform manifest unavailable')
        versions = [side for side in ('rollback', 'target')
                    if profile[side]['images'][role]['platform_manifest'] == descriptor['digest']]
        require(versions, 'unsupported selected platform manifest')
        spec = profile[versions[0]]['images'][role]
        references = {spec['repository'] + '@' + spec['platform_manifest']}
        if 'index_digest' in spec:
            references.add(spec['repository'] + '@' + spec['index_digest'])
        # Digest-only references are sufficient only with an actual ARM descriptor.
        # Tags alone cannot establish registry identity or a supported transition.
        require(row['image'] in references, 'unsupported selected image reference')
        if role == 'coolify':
            current_version = profile[versions[0]]['version']
        identity = normalize_recreation_identity(row['recreation_identity'], row['id'])
        stable[role] = {'container_id': row['id'], 'engine_image_id': row['image_id'],
                        'platform_manifest': descriptor['digest'], 'recreation_identity': identity,
                        'runtime_flags': flags}
        public[role] = {'identity_sha256': plan_sha256(stable[role]),
                        'engine_image_id': row['image_id'], 'platform_manifest': descriptor['digest'],
                        'health': row['health']}
    return {'status': 'observed', 'current_version': current_version,
            'instance_sha256': plan_sha256(stable), 'containers': public}


def _protected_file(path):
    """Fixed public source hashes only. Never call for .env or secret material."""
    from .host import secure_path
    secure_path(path)
    before = path.lstat()
    def identity(info):
        # A read can advance atime; content/ownership/link identity must not change.
        return (info.st_dev, info.st_ino, info.st_mode, info.st_uid, info.st_gid,
                info.st_nlink, info.st_size, info.st_mtime_ns, info.st_ctime_ns)
    require(before.st_nlink == 1 and before.st_size <= 512 * 1024,
            'unsupported source file')
    fd = os.open(path, os.O_RDONLY | os.O_NOFOLLOW | os.O_NONBLOCK)
    try:
        require(identity(os.fstat(fd)) == identity(before), 'source file changed')
        digest = hashlib.sha256()
        count = 0
        while chunk := os.read(fd, 65536):
            count += len(chunk)
            require(count <= 512 * 1024, 'unsupported source file')
            digest.update(chunk)
        require(identity(os.fstat(fd)) == identity(before)
                and identity(path.lstat()) == identity(before), 'source file changed')
        return digest.hexdigest()
    finally:
        os.close(fd)


def source_observation():
    from .host import secure_path
    try:
        secure_path(_SOURCE, directory=True)
        # An unknown custom override is never executed, read or hashed.
        try:
            (_SOURCE / 'docker-compose.custom.yml').lstat()
        except FileNotFoundError:
            pass
        else:
            return {'status': 'unsupported', 'reason': 'private_override_unsupported'}
        artifacts = coolify_plan.fixed_profile()['bundle']['rollback']['artifacts']
        selected = {name: _protected_file(_SOURCE / name) for name in
                    ('docker-compose.yml', 'docker-compose.prod.yml')}
        if any(selected[name] != artifacts[name] for name in selected):
            return {'status': 'unsupported', 'reason': 'source_configuration_unsupported'}
        return {'status': 'unverified', 'reason': 'source_configuration_unverified',
                'public_compose_sha256': plan_sha256(selected)}
    except (MaintenanceError, OSError):
        return {'status': 'unverified', 'reason': 'source_configuration_unverified'}


def capacity_observation():
    from .host import secure_path
    try:
        secure_path(_STAGING, directory=True)
        with open('/proc/meminfo', 'rb') as stream:
            raw = stream.read(65537)
        require(len(raw) <= 65536, 'invalid memory observation')
        match = re.search(rb'^MemAvailable: +([0-9]+) kB$', raw, re.MULTILINE)
        require(match is not None, 'invalid memory observation')
        memory = int(match.group(1)) * 1024
        disk = os.statvfs(_STAGING)
        free = disk.f_bavail * disk.f_frsize
        cpus = len(os.sched_getaffinity(0))
        sufficient = (memory >= _BOUNDS['memory_bytes'] + _BOUNDS['host_reserve_memory_bytes']
                      and free >= _BOUNDS['staging_free_bytes'] and cpus * 1000 >= _BOUNDS['cpu_millis'])
        return {'status': 'sufficient' if sufficient else 'insufficient'}
    except (MaintenanceError, OSError, ValueError):
        return {'status': 'unverified'}


def collect_report(config, request, *, runner=None, now=None):
    require(type(request) is dict and not request, 'invalid Coolify report request')
    require(type(config) is dict, 'invalid Coolify root configuration')
    now = int(time.time()) if now is None else now
    require(type(now) is int and now >= 0, 'invalid observation time')
    result = {'schema_version': 1, 'action': 'coolify_patch', 'eligible': False,
              'profile_id': coolify_plan.PROFILE_ID, 'profile_sha256': coolify_plan.profile_sha256(),
              'observed_at': now, 'status': 'disabled', 'reasons': [],
              'observation': {'status': 'unknown'}}
    policy = config.get('coolify_patch_policy')
    if policy is None:
        result['reasons'] = [{'category': 'policy', 'code': 'root_opt_in_disabled'}]
        return result
    validate_policy(policy)
    if not policy['enabled']:
        result['reasons'] = [{'category': 'policy', 'code': 'root_opt_in_disabled'}]
        return result
    result['status'] = 'blocked'
    if config.get('host_role') != 'application':
        result['reasons'] = [{'category': 'unsupported', 'code': 'unsupported_host_role'}]
        return result
    if runner is None:
        from .host import SubprocessRunner
        runner = SubprocessRunner()
    try:
        result['observation'] = _selected_runtime(runner)
    except (MaintenanceError, ContainerIdentityError, OSError, TypeError, KeyError, AttributeError):
        result['reasons'].append({'category': 'unsupported', 'code': 'selected_runtime_unavailable'})
    if now < coolify_plan.NOT_BEFORE:
        result['reasons'].append({'category': 'waiting', 'code': 'release_age_gate'})
    source = source_observation()
    result['source'] = source
    result['reasons'].append({'category': 'unsupported', 'code': source['reason']})
    capacity = capacity_observation()
    result['capacity'] = capacity
    if capacity['status'] != 'sufficient':
        result['reasons'].append({'category': 'capacity', 'code': 'capacity_' + capacity['status']})
    # These require additional reviewed root-local probes and exact proof binding;
    # caller/root-config flags and existing DB-only backup proofs cannot satisfy them.
    for category, code in (
            ('unsupported', 'startup_mutation_scope_unverified'),
            ('unsupported', 'runtime_recreation_unverified'),
            ('unsupported', 'mount_policy_unverified'),
            ('unsupported', 'network_policy_unverified'),
            ('unsupported', 'entrypoint_policy_unverified'),
            ('unsupported', 'model_version_unverified'),
            ('backup', 'backup_unverified'), ('full_proof', 'full_restore_proof_unverified'),
            ('barrier', 'barrier_unverified'), ('barrier', 'mutations_idle_unverified'),
            ('barrier', 'auto_update_policy_unverified'),
            ('barrier', 'protected_runtime_unverified'),
            ('barrier', 'github_job_history_unverified'),
            ('auth', 'auth_verification_unavailable')):
        result['reasons'].append({'category': category, 'code': code})
    return result
