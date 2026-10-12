"""Offline staged-public-artifact integrity. No network, restore or execution.

The caller must obtain expected_manifest_sha256 from an independently trusted
plan. This check does not establish release provenance, platform-manifest
resolution, a restore rehearsal, or authority to execute the bundle.
"""
import hashlib
import os
from pathlib import Path
import re
import stat

from .plan import HASH, SHA, MaintenanceError, matches, plan_sha256, require

ARTIFACTS = ('docker-compose.yml', 'docker-compose.prod.yml', '.env.production',
             'scripts/upgrade.sh', 'scripts/upgrade-postgres.sh')
MAX_ARTIFACT_BYTES = 1024 * 1024
STABLE = re.compile(r'(?:0|[1-9][0-9]{0,5})\.(?:0|[1-9][0-9]{0,5})\.(?:0|[1-9][0-9]{0,5})\Z')
IMAGE_REPOSITORIES = {
    'coolify': r'(?:docker\.io|ghcr\.io)/coollabsio/coolify',
    'postgres': r'docker\.io/library/postgres',
    'redis': r'docker\.io/library/redis',
    'helper': r'(?:docker\.io|ghcr\.io)/coollabsio/coolify-helper',
}


def _validate_manifest(manifest, expected_hash):
    require(matches(HASH, expected_hash), 'trusted bundle hash required')
    require(type(manifest) is dict and set(manifest) == {'schema_version', 'platform', 'rollback', 'target'},
            'invalid bundle fields')
    require(type(manifest['schema_version']) is int and manifest['schema_version'] == 1,
            'unsupported bundle schema')
    require(manifest['platform'] == 'linux/arm64', 'unsupported bundle platform')
    versions = []
    for phase in ('rollback', 'target'):
        release = manifest[phase]
        require(type(release) is dict and set(release) == {'version', 'release_commit', 'artifacts', 'images'},
                'invalid bundle release fields')
        require(matches(STABLE, release['version']) and matches(SHA, release['release_commit']),
                'invalid bundle release identity')
        versions.append(tuple(map(int, release['version'].split('.'))))
        artifacts = release['artifacts']
        require(type(artifacts) is dict and set(artifacts) == set(ARTIFACTS)
                and all(matches(HASH, value) for value in artifacts.values()),
                'invalid bundle artifact closure')
        images = release['images']
        require(type(images) is dict and set(images) == set(IMAGE_REPOSITORIES),
                'invalid bundle image closure')
        for role, repository in IMAGE_REPOSITORIES.items():
            require(type(images[role]) is str
                    and re.fullmatch(repository + r'@sha256:[0-9a-f]{64}', images[role]) is not None,
                    'bundle images require canonical public digest references')
    require(versions[0][:2] == versions[1][:2] and versions[0][2] < versions[1][2],
            'bundle requires a same-series patch increase')
    for role in ('postgres', 'redis'):
        require(manifest['rollback']['images'][role] == manifest['target']['images'][role],
                'database and Redis changes require a separate plan')
    require(plan_sha256(manifest) == expected_hash, 'bundle manifest hash mismatch')


def _open_directory(parent, name):
    return os.open(name, os.O_RDONLY | os.O_DIRECTORY | os.O_NOFOLLOW, dir_fd=parent)


def _open_root(root):
    path = Path(root)
    require(path.is_absolute() and '..' not in path.parts, 'bundle directory must be absolute')
    descriptor = os.open('/', os.O_RDONLY | os.O_DIRECTORY | os.O_NOFOLLOW)
    try:
        for part in path.parts[1:]:
            following = _open_directory(descriptor, part)
            os.close(descriptor)
            descriptor = following
        return descriptor
    except BaseException:
        os.close(descriptor)
        raise


def _check_file(directory, name, expected_hash):
    descriptor = os.open(name, os.O_RDONLY | os.O_NOFOLLOW | os.O_NONBLOCK, dir_fd=directory)
    try:
        info = os.fstat(descriptor)
        require(stat.S_ISREG(info.st_mode) and info.st_nlink == 1
                and 0 < info.st_size <= MAX_ARTIFACT_BYTES, 'invalid bundle artifact type or size')
        digest = hashlib.sha256()
        total = 0
        while chunk := os.read(descriptor, 65536):
            total += len(chunk)
            require(total <= MAX_ARTIFACT_BYTES, 'bundle artifact exceeds size limit')
            digest.update(chunk)
        after = os.fstat(descriptor)
        require((info.st_size, info.st_mtime_ns, info.st_ctime_ns)
                == (after.st_size, after.st_mtime_ns, after.st_ctime_ns)
                and total == info.st_size and digest.hexdigest() == expected_hash,
                'bundle artifact hash mismatch')
    finally:
        os.close(descriptor)


def verify_staged_bundle(manifest, root, *, expected_manifest_sha256):
    """Read only the exact public artifact closure; return integrity evidence.

    No caller-supplied filename is opened. Secret-bearing runtime overrides and
    production .env are outside this public bundle and require separate binding.
    """
    _validate_manifest(manifest, expected_manifest_sha256)
    try:
        root_fd = _open_root(root)
        try:
            require(set(os.listdir(root_fd)) == {'rollback', 'target'}, 'unexpected bundle root entries')
            for phase in ('rollback', 'target'):
                phase_fd = _open_directory(root_fd, phase)
                try:
                    require(set(os.listdir(phase_fd)) == {'docker-compose.yml', 'docker-compose.prod.yml',
                                                          '.env.production', 'scripts'},
                            'unexpected bundle release entries')
                    for artifact in ARTIFACTS[:3]:
                        _check_file(phase_fd, artifact, manifest[phase]['artifacts'][artifact])
                    scripts_fd = _open_directory(phase_fd, 'scripts')
                    try:
                        require(set(os.listdir(scripts_fd)) == {'upgrade.sh', 'upgrade-postgres.sh'},
                                'unexpected bundle script entries')
                        for artifact in ARTIFACTS[3:]:
                            _check_file(scripts_fd, artifact.split('/')[1], manifest[phase]['artifacts'][artifact])
                    finally:
                        os.close(scripts_fd)
                finally:
                    os.close(phase_fd)
        finally:
            os.close(root_fd)
    except OSError:
        raise MaintenanceError('bundle artifact integrity unavailable') from None
    return {'status': 'integrity_verified', 'manifest_sha256': expected_manifest_sha256, 'artifact_count': 10}
