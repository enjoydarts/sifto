"""Dormant owned-Docker capabilities and durable creation journal.

Only root-trusted native callers may supply a context. The bounded native
process adapter and material/identity verification belong to N2; this module
has no CLI, subprocess, shell, arbitrary exec or production gateway dispatch.
The injected test adapter never confers production eligibility.
"""
from dataclasses import dataclass
from enum import Enum
import hashlib
import json
import os
from pathlib import Path
import platform
import re
import secrets
import stat

from .coolify_rehearsal import FILES, FilePolicy, RootBinding
from .coolify_plan import fixed_profile

_CATEGORIES = frozenset('context_policy source_closure image_descriptor image_identity '
    'stage_collision stage_identity material_binding env_policy hmac_identity container_identity '
    'container_spec ownership_unknown journal_invalid network_isolation resource_capacity '
    'resource_isolation output_bound command_timeout command_failed startup_profile_unverified '
    'startup_state evidence_invalid evidence_difference cleanup_incomplete interrupted '
    'synthetic_verified'.split())
_HASH = re.compile(r'[a-f0-9]{64}\Z')
_DIGEST = re.compile(r'sha256:[a-f0-9]{64}\Z')
_SOURCE_NAMES = frozenset(('host_agent.py', 'maintenance/__init__.py', 'maintenance/plan.py',
    'maintenance/host.py', 'maintenance/coolify_host.py', 'maintenance/coolify_plan.py',
    'maintenance/coolify_rehearsal.py', 'maintenance/coolify_evidence.py',
    'maintenance/coolify_docker.py', 'maintenance/coolify-public-manifest.json',
    'maintenance/coolify_probe.php'))
_ENV = {'PATH': '/usr/bin:/bin', 'HOME': '/nonexistent', 'LC_ALL': 'C'}
_APP_FILES = ('runtime.env', 'probe.php', 'public-manifest.json', 'hmac.key')
_APP_DIRS = ('storage', 'bootstrap-cache', 'runtime-ssh', 'runtime-ssl', 'runtime-proxy')
_PURPOSE_LEAVES = {
    'network': ('probe.php',),
    'baseline': (*_APP_FILES, 'storage', 'bootstrap-cache'),
    'verify': (*_APP_FILES, 'storage', 'bootstrap-cache', 'runtime-ssh'),
    'migrate': ('runtime.env', 'storage', 'bootstrap-cache'),
}
_LEAVES = (*_APP_FILES, *_APP_DIRS, 'pg', 'redis', 'coolify.dump')
_DESTINATIONS = {'runtime.env': '/var/www/html/.env', 'probe.php': '/probe/probe.php',
    'public-manifest.json': '/probe/public-manifest.json', 'hmac.key': '/probe/hmac.key',
    'storage': '/var/www/html/storage', 'bootstrap-cache': '/var/www/html/bootstrap/cache',
    'runtime-ssh': '/var/www/html/storage/app/ssh', 'runtime-ssl': '/data/coolify/ssl',
    'runtime-proxy': '/data/coolify/proxy', 'pg': '/var/lib/postgresql/data', 'redis': '/data'}


def _require(condition, category):
    if not condition:
        raise DockerError(category) from None


def _hash(value):
    return hashlib.sha256(value).hexdigest()


def _json(value):
    return json.dumps(value, sort_keys=True, separators=(',', ':'), allow_nan=False).encode()


def _metadata(value, directory=False):
    identity = (value.st_dev, value.st_ino, value.st_uid, value.st_gid, value.st_mode)
    # Mutable directory content changes size, link count, mtime and ctime.
    return identity if directory else identity + (value.st_size, value.st_nlink,
                                                value.st_mtime_ns, value.st_ctime_ns)


def _read(fd, limit=1024 * 1024):
    size = os.fstat(fd).st_size
    _require(0 <= size <= limit, 'output_bound')
    value = os.pread(fd, size + 1, 0)
    _require(len(value) == size, 'stage_identity')
    return value


def _protected_open(path, flags, *, category):
    """No symlink component; only root-owned protected ancestors (sticky /tmp)."""
    descriptors = []
    try:
        _require(isinstance(path, Path) and path.is_absolute() and '..' not in path.parts
                 and all(not any(c in p for c in ('\x00', '\n', '\r', ',')) for p in path.parts), category)
        fd = os.open('/', os.O_RDONLY | os.O_DIRECTORY | os.O_CLOEXEC)
        descriptors.append(fd)
        parents = [(Path('/'), _metadata(os.fstat(fd), True))]
        walked = Path('/')
        for name in path.parts[1:-1]:
            walked /= name
            fd = os.open(name, os.O_RDONLY | os.O_DIRECTORY | os.O_NOFOLLOW | os.O_CLOEXEC, dir_fd=fd)
            descriptors.append(fd)
            info = os.fstat(fd)
            _require(info.st_uid == 0 and (not info.st_mode & 0o022
                     or walked == Path('/tmp') and info.st_mode & stat.S_ISVTX), category)
            parents.append((walked, _metadata(info, True)))
        result = os.open(path.name, flags | os.O_NOFOLLOW | os.O_CLOEXEC, dir_fd=fd)
        return result, tuple(parents)
    except DockerError:
        raise
    except (OSError, ValueError, TypeError):
        raise DockerError(category) from None
    finally:
        for descriptor in reversed(descriptors):
            os.close(descriptor)


def _closed(method):
    """Public failures never carry adapter/OS exception strings or tracebacks."""
    def bounded(*args, **kwargs):
        try:
            return method(*args, **kwargs)
        except DockerError as error:
            raise DockerError(str(error)) from None
        except (KeyboardInterrupt, SystemExit):
            raise DockerError('interrupted') from None
        except Exception:
            raise DockerError('command_failed') from None
    bounded.__name__ = method.__name__
    # Preserve the deliberately closed public signature for review/introspection.
    bounded.__wrapped__ = method
    return bounded


@dataclass(frozen=True, repr=False)
class NativeContext:
    binding: RootBinding
    input_path: Path
    input_policy: FilePolicy
    staging_policy: FilePolicy
    docker_socket: Path
    cgroup_parent: Path
    source_sha256: tuple
    startup: 'ControlledStartup | None'


@dataclass(frozen=True, repr=False)
class ControlledStartup:
    profile_sha256: str
    artifact_id: str
    verified_scope: str


class Phase(Enum):
    RUN = 'run'
    OLD = 'old'
    FRESH = 'fresh'


class Operation(Enum):
    IDENTITY_OLD = 'identity_old'
    IDENTITY_TARGET = 'identity_target'
    IDENTITY_POSTGRES = 'identity_postgres'
    IDENTITY_REDIS = 'identity_redis'
    POSTGRES = 'postgres'
    REDIS = 'redis'
    BASELINE_OLD = 'baseline_old'
    NETWORK_OLD = 'network_old'
    NETWORK_TARGET = 'network_target'
    VERIFY_OLD = 'verify_old'
    VERIFY_TARGET = 'verify_target'
    MIGRATE_OLD = 'migrate_old'
    MIGRATE_TARGET = 'migrate_target'
    SSH_OLD = 'ssh_old'
    SSH_TARGET = 'ssh_target'
    HELD_OLD = 'held_old'
    HELD_TARGET = 'held_target'


@dataclass(frozen=True, repr=False)
class CreatedContainer:
    container_id: str
    intent_id: str
    phase: Phase
    operation: Operation
    image_reference: str
    image_engine_id: str
    spec_sha256: str


class DockerError(Exception):
    """Only an allowlisted static category survives construction."""
    def __init__(self, category='command_failed'):
        super().__init__(category if type(category) is str and category in _CATEGORIES else 'command_failed')


class OwnedRun:
    __slots__ = ()

    def __init__(self):
        raise DockerError('stage_identity')


class PhaseResources:
    __slots__ = ()

    def __init__(self):
        raise DockerError('stage_identity')


class OwnedDocker:
    @_closed
    def __init__(self, context, *, adapter):
        self.context = context
        self._adapter = adapter
        self._runs = {}
        self._resources = {}
        self._containers = {}
        self._fds = set()
        self._fd_objects = {}
        self._anchors = []
        self._sources = {}
        self._closed = False
        try:
            self._require_native_platform()
            _require(os.geteuid() == 0 and type(context) is NativeContext and adapter is not None,
                     'context_policy')
            binding = context.binding
            _require(type(binding) is RootBinding and type(binding.file_sha256) is tuple
                     and len(binding.file_sha256) == 3
                     and all(type(e) is tuple and len(e) == 2 for e in binding.file_sha256)
                     and type(binding.snapshot_id) is str
                     and _HASH.fullmatch(binding.snapshot_id) and type(binding.proof_sha256) is str
                     and _HASH.fullmatch(binding.proof_sha256) and type(binding.host_id) is str
                     and re.fullmatch(r'[A-Za-z0-9_.-]{1,128}', binding.host_id), 'context_policy')
            hashes = dict(binding.file_sha256)
            _require(set(hashes) == FILES and all(type(v) is str and _HASH.fullmatch(v)
                     for v in hashes.values()), 'context_policy')
            for policy in (context.input_policy, context.staging_policy):
                _require(type(policy) is FilePolicy and type(policy.owner_uid) is int
                         and policy.owner_uid == 0 and isinstance(policy.root, Path), 'context_policy')
            _require(isinstance(context.input_path, Path) and isinstance(context.docker_socket, Path)
                     and context.input_path != context.input_policy.root
                     and context.input_path.is_relative_to(context.input_policy.root), 'context_policy')
            _require(isinstance(context.cgroup_parent, Path) and context.cgroup_parent.is_absolute()
                     and context.cgroup_parent != Path('/') and '..' not in context.cgroup_parent.parts
                     and not any(c in str(context.cgroup_parent) for c in ('\x00', '\n', '\r', ',')),
                     'context_policy')
            self._stage_fd = self._anchor(context.staging_policy.root, directory=True)
            self._anchor(context.input_policy.root, directory=True, private=False)
            self._anchor(context.input_path)
            self._anchor(context.docker_socket, socket=True)
            self._load_sources()
        except BaseException:
            self.close()
            raise

    def _require_native_platform(self):
        _require(platform.system() == 'Linux' and platform.machine() in ('aarch64', 'arm64'), 'context_policy')

    def _anchor(self, path, *, directory=False, socket=False, private=True):
        flags = os.O_PATH if socket else os.O_RDONLY
        if not directory and not socket:
            flags |= os.O_NONBLOCK
        if directory:
            flags |= os.O_DIRECTORY
        fd, parents = _protected_open(path, flags, category='context_policy')
        self._keep_fd(fd)
        info = os.fstat(fd)
        valid_type = stat.S_ISSOCK(info.st_mode) if socket else (
            stat.S_ISDIR(info.st_mode) if directory else stat.S_ISREG(info.st_mode))
        _require(valid_type and info.st_uid == 0 and not info.st_mode & 0o022
                 and (directory or socket or info.st_nlink == 1), 'context_policy')
        if directory and private:
            _require(stat.S_IMODE(info.st_mode) == 0o700, 'context_policy')
        self._anchors.append((path, fd, flags, _metadata(info, directory), parents, directory))
        return fd

    def _load_sources(self):
        entries = self.context.source_sha256
        _require(type(entries) is tuple and len(entries) == len(_SOURCE_NAMES)
                 and all(type(e) is tuple and len(e) == 2 and isinstance(e[0], Path)
                         and type(e[1]) is str and _HASH.fullmatch(e[1]) for e in entries), 'source_closure')
        roots = [path.parent for path, _ in entries if path.name == 'host_agent.py']
        _require(len(roots) == 1, 'source_closure')
        root = roots[0]
        for path, digest in entries:
            _require(path.is_relative_to(root), 'source_closure')
            name = str(path.relative_to(root))
            _require(name in _SOURCE_NAMES and name not in self._sources, 'source_closure')
            fd, parents = _protected_open(path, os.O_RDONLY | os.O_NONBLOCK, category='source_closure')
            self._keep_fd(fd)
            info = os.fstat(fd)
            _require(stat.S_ISREG(info.st_mode) and info.st_uid == 0
                     and stat.S_IMODE(info.st_mode) == 0o644 and info.st_nlink == 1, 'source_closure')
            _require(_hash(_read(fd)) == digest, 'source_closure')
            self._sources[name] = (path, fd, _metadata(info), digest, parents)

    def _parents_unchanged(self, parents, category):
        for path, identity in parents:
            info = path.lstat()
            _require(not stat.S_ISLNK(info.st_mode) and _metadata(info, True) == identity, category)

    def _guard_context(self):
        _require(not self._closed, 'stage_identity')
        _require(os.geteuid() == 0, 'context_policy')
        try:
            for path, fd, flags, identity, parents, directory in self._anchors:
                self._parents_unchanged(parents, 'context_policy')
                _require(_metadata(os.fstat(fd), directory) == identity, 'context_policy')
                observed, _ = _protected_open(path, flags, category='context_policy')
                try:
                    _require(_metadata(os.fstat(observed), directory) == identity, 'context_policy')
                finally:
                    os.close(observed)
        except OSError:
            raise DockerError('context_policy') from None
        try:
            for path, fd, identity, digest, parents in self._sources.values():
                self._parents_unchanged(parents, 'source_closure')
                _require(_metadata(os.fstat(fd)) == identity and _metadata(path.lstat()) == identity
                         and _hash(_read(fd)) == digest, 'source_closure')
        except OSError:
            raise DockerError('source_closure') from None

    def _keep_fd(self, fd):
        self._fds.add(fd)
        info = os.fstat(fd)
        self._fd_objects[fd] = (info.st_dev, info.st_ino)
        return fd

    def _exclusive_file(self, parent, name, content):
        fd = self._keep_fd(os.open(name, os.O_RDWR | os.O_CREAT | os.O_EXCL | os.O_NOFOLLOW | os.O_CLOEXEC,
                                   0o600, dir_fd=parent))
        self._write_all(fd, content)
        os.fsync(fd)
        return {'fd': fd, 'identity': _metadata(os.fstat(fd)), 'bytes': content}

    @staticmethod
    def _write_all(fd, value):
        offset = 0
        while offset < len(value):
            count = os.write(fd, value[offset:])
            _require(count > 0, 'journal_invalid')
            offset += count

    def _guard_file(self, parent, name, record, category):
        try:
            _require(_metadata(os.fstat(record['fd'])) == record['identity']
                     and _metadata(os.stat(name, dir_fd=parent, follow_symlinks=False)) == record['identity']
                     and _read(record['fd']) == record['bytes'], category)
        except OSError:
            raise DockerError(category) from None

    def _guard_run(self, run, phase=None):
        self._guard_context()
        _require(type(run) is OwnedRun and run in self._runs, 'stage_identity')
        record = self._runs[run]
        try:
            _require(_metadata(os.fstat(record['fd']), True) == record['identity']
                     and _metadata(os.stat(record['token'], dir_fd=self._stage_fd,
                                          follow_symlinks=False), True) == record['identity'], 'stage_identity')
            self._guard_file(record['fd'], '.owner', record['owner'], 'stage_identity')
            self._guard_file(record['fd'], 'journal', record['journal'], 'journal_invalid')
            if phase is not None:
                _require(type(phase) is Phase and phase in record['phases'], 'stage_identity')
                selected = record['phases'][phase]
                _require(_metadata(os.fstat(selected['fd']), True) == selected['identity']
                         and _metadata(os.stat(phase.value, dir_fd=record['fd'], follow_symlinks=False), True)
                         == selected['identity'], 'stage_identity')
                self._guard_file(selected['fd'], '.owner', selected['owner'], 'stage_identity')
        except OSError:
            raise DockerError('stage_identity') from None
        return record

    @_closed
    def allocate_run(self):
        self._guard_context()
        token = secrets.token_hex(16)
        try:
            os.mkdir(token, 0o700, dir_fd=self._stage_fd)
        except FileExistsError:
            raise DockerError('stage_collision') from None
        fd = self._keep_fd(os.open(token, os.O_RDONLY | os.O_DIRECTORY | os.O_NOFOLLOW | os.O_CLOEXEC,
                                   dir_fd=self._stage_fd))
        record = {'fd': fd, 'token': token, 'identity': _metadata(os.fstat(fd), True),
                  'owner': self._exclusive_file(fd, '.owner', token.encode()),
                  'journal': self._exclusive_file(fd, 'journal', b''), 'phases': {},
                  'identities': {}, 'material': {}, 'intents': {}, 'failed': False, 'cleaned': False}
        for phase in (Phase.OLD, Phase.FRESH):
            os.mkdir(phase.value, 0o700, dir_fd=fd)
            child = self._keep_fd(os.open(phase.value, os.O_RDONLY | os.O_DIRECTORY | os.O_NOFOLLOW | os.O_CLOEXEC,
                                          dir_fd=fd))
            record['phases'][phase] = {'fd': child, 'identity': _metadata(os.fstat(child), True),
                'owner': self._exclusive_file(child, '.owner', (token + ':' + phase.value).encode()),
                'resources': None, 'started': False}
            os.fsync(child)
        os.fsync(fd)
        os.fsync(self._stage_fd)
        handle = object.__new__(OwnedRun)
        self._runs[handle] = record
        return handle

    @_closed
    def preparation_fd(self, run, phase):
        record = self._guard_run(run, phase)
        selected = record['phases'][phase]
        _require(not record['failed'] and not record['cleaned'] and selected['resources'] is None
                 and not selected['started'], 'stage_identity')
        return os.dup(selected['fd'])

    def _leaf_owner(self, record, name):
        role = 'postgres' if name == 'pg' else ('redis' if name == 'redis' else 'old')
        identity = record['identities'].get(role)
        _require(type(identity) is tuple and len(identity) == 3 and all(type(v) is int and 0 < v < 65536
                 for v in identity[:2]) and type(identity[2]) is str and _DIGEST.fullmatch(identity[2]),
                 'material_binding')
        if role == 'old':
            target = record['identities'].get('target')
            _require(identity[:2] == (9999, 9999) and type(target) is tuple
                     and target[:2] == (9999, 9999), 'hmac_identity')
        return identity[:2]

    def _seal_leaf(self, record, phase, name):
        parent = record['phases'][phase]['fd']
        directory = name in (*_APP_DIRS, 'pg', 'redis')
        category = 'hmac_identity' if name == 'hmac.key' else 'material_binding'
        flags = os.O_RDONLY | os.O_NOFOLLOW | os.O_CLOEXEC | os.O_NONBLOCK
        if directory:
            flags |= os.O_DIRECTORY
        try:
            fd = self._keep_fd(os.open(name, flags, dir_fd=parent))
            info = os.fstat(fd)
            uid, gid = (0, 0) if name == 'coolify.dump' else self._leaf_owner(record, name)
            _require((stat.S_ISDIR(info.st_mode) if directory else stat.S_ISREG(info.st_mode))
                     and (info.st_uid, info.st_gid) == (uid, gid)
                     and stat.S_IMODE(info.st_mode) == (0o700 if directory else 0o600)
                     and (directory or info.st_nlink == 1), category)
            digest = None
            if not directory:
                value = _read(fd, 64 * 1024 * 1024)
                digest = _hash(value)
                if name == 'hmac.key':
                    _require(len(value) == 32, category)
                if name in ('probe.php', 'public-manifest.json'):
                    source = 'coolify_probe.php' if name == 'probe.php' else 'coolify-public-manifest.json'
                    expected = self._sources['maintenance/' + source][3]
                else:
                    expected = record['material'].get(phase, {}).get(name)
                _require(digest == expected, category if name == 'hmac.key' else 'material_binding')
                if name == 'coolify.dump':
                    _require(digest == dict(self.context.binding.file_sha256)['coolify.dump'], 'material_binding')
            return {'fd': fd, 'identity': _metadata(info, directory), 'directory': directory, 'digest': digest}
        except OSError:
            raise DockerError(category) from None

    @_closed
    def seal_phase(self, run, phase):
        record = self._guard_run(run, phase)
        selected = record['phases'][phase]
        _require(not record['failed'] and not record['cleaned'] and selected['resources'] is None
                 and not selected['started'], 'stage_identity')
        _require(phase in record['material'] and type(record['material'][phase]) is dict, 'material_binding')
        leaves = {}
        present = set(os.listdir(selected['fd']))
        for name in _LEAVES:
            if name in present:
                leaves[name] = self._seal_leaf(record, phase, name)
        _require(present <= set(_LEAVES) | {'.owner'}, 'material_binding')
        handle = object.__new__(PhaseResources)
        self._resources[handle] = {'run': run, 'phase': phase, 'leaves': leaves,
            'material': dict(record['material'][phase]), 'identities': dict(record['identities'])}
        selected['resources'] = handle
        return handle

    def _guard_resources(self, run, phase, resources, names):
        record = self._guard_run(run, phase)
        _require(type(resources) is PhaseResources and resources in self._resources, 'stage_identity')
        sealed = self._resources[resources]
        _require(sealed['run'] is run and sealed['phase'] is phase
                 and record['phases'][phase]['resources'] is resources, 'stage_identity')
        _require(record['material'].get(phase) == sealed['material']
                 and record['identities'] == sealed['identities'], 'material_binding')
        _require(set(names) <= sealed['leaves'].keys(), 'material_binding')
        for name in names:
            leaf = sealed['leaves'][name]
            try:
                observed = os.stat(name, dir_fd=record['phases'][phase]['fd'], follow_symlinks=False)
                current = _metadata(observed, leaf['directory'])
                held = _metadata(os.fstat(leaf['fd']), leaf['directory'])
                _require(current[:5] == held[:5] == leaf['identity'][:5]
                         and (leaf['directory'] or current[6] == held[6] == leaf['identity'][6]), 'stage_identity')
                if not leaf['directory']:
                    _require(_hash(_read(leaf['fd'], 64 * 1024 * 1024)) == leaf['digest'], 'material_binding')
                _require(current == held == leaf['identity'], 'stage_identity')
            except OSError:
                raise DockerError('stage_identity') from None
        return sealed

    def _call(self, method, *args, **kwargs):
        try:
            return getattr(self._adapter, method)(*args, **kwargs)
        except TimeoutError:
            raise DockerError('command_timeout') from None
        except (KeyboardInterrupt, SystemExit):
            raise DockerError('interrupted') from None
        except Exception:
            raise DockerError('command_failed') from None

    def _lookup(self, **selected):
        values = self._call('lookup', **selected)
        _require(type(values) is list and len(values) <= 64 and len(values) == len(set(values))
                 and all(type(v) is str and _HASH.fullmatch(v) for v in values), 'ownership_unknown')
        return values

    def _image(self, role):
        bundle = fixed_profile()['bundle']
        selected = bundle['target' if role == 'target' else 'rollback']['images'][
            'coolify' if role in ('old', 'target') else role]
        reference = selected['repository'] + '@' + selected['platform_manifest']
        observed = self._call('inspect_image', reference)
        _require(type(observed) is dict and len(_json(observed)) <= 65536, 'image_descriptor')
        _require(observed.get('os') == 'linux' and observed.get('architecture') == 'arm64'
                 and type(observed.get('engine_id')) is str and _DIGEST.fullmatch(observed['engine_id']),
                 'image_identity')
        descriptor = observed.get('descriptor')
        _require(set(observed) == {'engine_id', 'os', 'architecture', 'descriptor'}
                 and type(descriptor) is dict and set(descriptor) == {'digest', 'mediaType'}
                 and descriptor['digest'] == selected['platform_manifest'] and descriptor['mediaType'] in (
                     'application/vnd.oci.image.manifest.v1+json',
                     'application/vnd.docker.distribution.manifest.v2+json'), 'image_descriptor')
        return reference, observed['engine_id']

    @staticmethod
    def _operation(operation, phase):
        _require(type(operation) is Operation and type(phase) is Phase, 'container_spec')
        if operation.value.startswith('identity_'):
            _require(phase is Phase.RUN, 'container_spec')
            return operation.value[9:], 'identity', ()
        _require(phase in (Phase.OLD, Phase.FRESH)
                 and (not operation.value.endswith('_target') or phase is Phase.OLD), 'container_spec')
        if operation in (Operation.SSH_OLD, Operation.SSH_TARGET, Operation.HELD_OLD, Operation.HELD_TARGET):
            raise DockerError('startup_profile_unverified')
        if operation is Operation.POSTGRES:
            return 'postgres', 'postgres', ('pg',)
        if operation is Operation.REDIS:
            return 'redis', 'redis', ('redis',)
        role = 'target' if operation.value.endswith('_target') else 'old'
        purpose = operation.value.rsplit('_', 1)[0]
        return role, purpose, _PURPOSE_LEAVES[purpose]

    def _postgres(self, run, phase):
        records = [c for c in self._containers.values() if c['run'] is run and c['phase'] is phase
                   and c['operation'] is Operation.POSTGRES and c['state'] != 'removed']
        _require(len(records) == 1 and records[0]['state'] == 'started', 'network_isolation')
        record = records[0]
        self._authenticate_container(record)
        observed = self._inspect(record['id'])
        _require(observed['State']['Running'] and observed['State']['Status'] == 'running', 'network_isolation')
        return record['id']

    def _spec(self, run, operation, phase, resources, intent):
        record = self._guard_run(run, None if phase is Phase.RUN else phase)
        role, purpose, names = self._operation(operation, phase)
        _require(not record['failed'] and not record['cleaned'], 'interrupted')
        if phase is Phase.RUN:
            _require(resources is None, 'container_spec')
        else:
            self._guard_resources(run, phase, resources, names)
        _require(not any(c['phase'] is phase and c['operation'] is operation
                     and (c['state'] != 'removed' or operation is Operation.MIGRATE_TARGET)
                     for c in record['intents'].values()), 'container_spec')
        active = [c for r in self._runs.values() for c in r['intents'].values() if c['state'] != 'removed']
        for current in active:
            _, current_purpose, _ = self._operation(current['operation'], current['phase'])
            _require(current['run'] is run and current['phase'] is phase
                     and purpose != 'identity' and current_purpose != 'identity', 'resource_isolation')
            probes = ('baseline', 'network', 'verify')
            _require(not (purpose in probes and current_purpose in probes)
                     and not (purpose == current_purpose == 'migrate'), 'resource_isolation')
        reference, engine_id = self._image(role)
        if purpose == 'identity':
            uid = '65534:65534'
            command = [{'old': 'www-data', 'target': 'www-data', 'postgres': 'postgres', 'redis': 'redis'}[role]]
            executable = '/usr/bin/id'
            memory, cpu, pids = 128, 250, 64
        else:
            identity = record['identities'].get(role)
            _require(type(identity) is tuple and len(identity) == 3 and identity[2] == engine_id, 'image_identity')
            uid = str(identity[0]) + ':' + str(identity[1])
            if purpose == 'postgres':
                executable = '/usr/local/bin/docker-entrypoint.sh'
                command = ['postgres', '-c', 'listen_addresses=127.0.0.1', '-c',
                           'unix_socket_directories=/var/run/postgresql']
                memory, cpu, pids = 256, 250, 128
            elif purpose == 'redis':
                executable = '/usr/local/bin/redis-server'
                command = ['--bind', '127.0.0.1', '--requirepass', 'rehearsal-only-password',
                           '--save', '', '--appendonly', 'no']
                memory, cpu, pids = 128, 125, 64
            else:
                executable = 'php'
                command = ['artisan', 'migrate', '--force', '--isolated'] if purpose == 'migrate' else [
                    '/probe/probe.php', purpose]
                memory, cpu, pids = (512, 1000, 256) if purpose == 'migrate' else (128, 250, 64)
        network = 'none' if purpose in ('identity', 'postgres') else 'container:' + self._postgres(run, phase)
        mounts = []
        if phase is not Phase.RUN:
            path = self.context.staging_policy.root / record['token'] / phase.value
            mounts = [{'Type': 'bind', 'Source': str(path / name), 'Destination': _DESTINATIONS[name],
                       'RW': name not in _APP_FILES and not (purpose == 'verify' and name == 'runtime-ssh'),
                       'Propagation': 'rprivate'} for name in names]
        tmpfs = {'/tmp': 'rw,nosuid,nodev,noexec,mode=1777,size=16777216'}
        if purpose == 'identity':
            # Shadow fixed images' declared volumes; identity creates no anonymous
            # data volume and mounts no run/material/private authentication path.
            for target in ('/var/lib/postgresql/data', '/data'):
                tmpfs[target] = 'rw,nosuid,nodev,noexec,mode=0700,size=1048576'
        if purpose == 'postgres':
            tmpfs['/var/run/postgresql'] = 'rw,nosuid,nodev,noexec,mode=0777,size=16777216'
        labels = {'sifto.owner': 'native-rehearsal-v1', 'sifto.run': record['token'], 'sifto.phase': phase.value,
                  'sifto.role': role, 'sifto.operation': operation.value, 'sifto.intent': intent}
        return {'Name': 'sifto-native-' + record['token'] + '-' + intent,
            'Image': engine_id, 'Reference': reference, 'Labels': labels, 'User': uid,
            'Entrypoint': [executable], 'Cmd': command, 'WorkingDir': '/var/www/html' if role in ('old', 'target') else '/',
            'Memory': memory * 1024 * 1024, 'MemorySwap': memory * 1024 * 1024, 'NanoCpus': cpu * 1000000,
            'PidsLimit': pids, 'CapDrop': ['ALL'], 'CapAdd': [], 'SecurityOpt': ['no-new-privileges'],
            'ReadonlyRootfs': True, 'NetworkMode': network, 'PidMode': '', 'IpcMode': 'private',
            'CgroupnsMode': 'private', 'CgroupParent': str(self.context.cgroup_parent), 'Privileged': False,
            'Ports': {}, 'ExtraHosts': [], 'Devices': [], 'Mounts': mounts, 'Tmpfs': tmpfs}, names

    @staticmethod
    def _argv(spec, env_fd):
        argv = ['create', '--pull=never', '--platform=linux/arm64', '--name=' + spec['Name'],
                '--user=' + spec['User'], '--entrypoint=' + spec['Entrypoint'][0], '--workdir=' + spec['WorkingDir'],
                '--memory=' + str(spec['Memory']), '--memory-swap=' + str(spec['MemorySwap']),
                '--cpus=' + format(spec['NanoCpus'] / 1000000000, '.3f'), '--pids-limit=' + str(spec['PidsLimit']),
                '--cap-drop=ALL', '--security-opt=no-new-privileges', '--read-only=true',
                '--network=' + spec['NetworkMode'], '--ipc=private', '--cgroupns=private',
                '--cgroup-parent=' + spec['CgroupParent']]
        argv.extend('--label=' + key + '=' + value for key, value in sorted(spec['Labels'].items()))
        argv.extend('--tmpfs=' + key + ':' + value for key, value in sorted(spec['Tmpfs'].items()))
        for mount in spec['Mounts']:
            value = 'type=bind,src=' + mount['Source'] + ',dst=' + mount['Destination'] + ',bind-propagation=rprivate'
            argv.append('--mount=' + value + ('' if mount['RW'] else ',readonly'))
        if env_fd is not None:
            argv.append('--env-file=/proc/self/fd/' + str(env_fd))
        if spec['Labels']['sifto.operation'] == Operation.POSTGRES.value:
            # Fixed synthetic loopback-only dependencies, never restored values.
            argv.extend('--env=' + value for value in ('POSTGRES_USER=coolify', 'POSTGRES_DB=coolify',
                'POSTGRES_PASSWORD=rehearsal-only-password', 'PGDATA=/var/lib/postgresql/data'))
        return [*argv, spec['Reference'], *spec['Cmd']]

    def _append_record(self, record, state):
        run = self._guard_run(record['run'])
        journal = run['journal']
        entry = {'state': state, 'intent': record['intent'], 'phase': record['phase'].value,
                 'operation': record['operation'].value, 'container_id': record['id'], 'spec_sha256': record['hash']}
        value = _json(entry) + b'\n'
        _require(len(journal['bytes']) + len(value) <= 1024 * 1024, 'journal_invalid')
        try:
            os.lseek(journal['fd'], 0, os.SEEK_END)
            self._write_all(journal['fd'], value)
            os.fsync(journal['fd'])
            journal.update(bytes=journal['bytes'] + value, identity=_metadata(os.fstat(journal['fd'])))
            record['state'] = state
        except OSError:
            raise DockerError('journal_invalid') from None

    def _inspect(self, container_id):
        _require(type(container_id) is str and _HASH.fullmatch(container_id), 'container_identity')
        observed = self._call('inspect', container_id)
        if observed is not None:
            _require(type(observed) is dict and len(_json(observed)) <= 65536, 'output_bound')
        return observed

    def _require_owned_spec(self, record, observed):
        _require(type(observed) is dict and observed.get('Id') == record['id'], 'container_identity')
        expected = record['spec']
        mounts = observed.get('Mounts')
        _require(type(mounts) is list and all(type(m) is dict and type(m.get('Destination')) is str
                 for m in mounts) and len({m['Destination'] for m in mounts}) == len(mounts), 'container_spec')
        normalized = dict(observed)
        normalized['Mounts'] = sorted(mounts, key=lambda m: m['Destination'])
        expected = dict(expected)
        expected['Mounts'] = sorted(expected['Mounts'], key=lambda m: m['Destination'])
        _require(set(normalized) == set(expected) | {'Id', 'State'}
                 and all(type(normalized[key]) is type(value) and normalized[key] == value
                         for key, value in expected.items()), 'container_spec')
        state = observed['State']
        _require(type(state) is dict and set(state) == {'Status', 'Running', 'ExitCode', 'OOMKilled'}
                 and state['Status'] in ('created', 'running', 'exited', 'dead', 'removing')
                 and type(state['Running']) is bool and type(state['OOMKilled']) is bool
                 and type(state['ExitCode']) is int and 0 <= state['ExitCode'] <= 255
                 and not state['OOMKilled'], 'startup_state')

    def _authenticate_container(self, record):
        self._guard_run(record['run'], None if record['phase'] is Phase.RUN else record['phase'])
        if record['phase'] is not Phase.RUN:
            self._guard_resources(record['run'], record['phase'], record['resources'], record['names'])
        reference, engine_id = self._image(record['role'])
        _require(reference == record['spec']['Reference'] and engine_id == record['spec']['Image'], 'image_identity')
        observed = self._inspect(record['id'])
        self._require_owned_spec(record, observed)
        return observed

    def _record_for_id(self, container_id):
        _require(type(container_id) is str and _HASH.fullmatch(container_id)
                 and container_id in self._containers, 'ownership_unknown')
        record = self._containers[container_id]
        self._guard_run(record['run'])
        return record

    def _reconcile(self, record):
        candidates = self._lookup(intent=record['intent'])
        _require(len(candidates) == 1, 'ownership_unknown')
        candidate = candidates[0]
        self._require_owned_spec({**record, 'id': candidate}, self._inspect(candidate))
        _require(candidate not in self._containers, 'ownership_unknown')
        record['id'] = candidate
        self._containers[record['id']] = record
        self._append_record(record, 'created')
        self._append_record(record, 'verified')

    @_closed
    def create(self, run, *, operation, phase, resources):
        # Legal operation/phase precedes intent allocation, journal and Docker.
        role, _, _ = self._operation(operation, phase)
        intent = secrets.token_hex(16)
        owned = self._guard_run(run, None if phase is Phase.RUN else phase)
        _require(all(intent not in r['intents'] for r in self._runs.values()), 'stage_collision')
        spec, names = self._spec(run, operation, phase, resources, intent)
        _require(not self._lookup(name=spec['Name']) and not self._lookup(intent=intent), 'stage_collision')
        record = {'run': run, 'phase': phase, 'operation': operation, 'role': role,
                  'resources': resources, 'names': names, 'intent': intent, 'id': None,
                  'spec': spec, 'hash': _hash(_json(spec)), 'state': 'intent'}
        self._append_record(record, 'intent')
        owned['intents'][intent] = record
        env_fd = None
        try:
            if phase is not Phase.RUN and 'runtime.env' in names:
                sealed = self._guard_resources(run, phase, resources, names)
                env_fd = os.dup(sealed['leaves']['runtime.env']['fd'])
                os.lseek(env_fd, 0, os.SEEK_SET)
            cid = self._call('create', self._argv(spec, env_fd), env=dict(_ENV),
                             pass_fds=() if env_fd is None else (env_fd,))
            _require(type(cid) is str and _HASH.fullmatch(cid), 'container_identity')
            _require(cid not in self._containers, 'ownership_unknown')
            record['id'] = cid
            self._containers[cid] = record
            self._append_record(record, 'created')
            self._authenticate_container(record)
            self._append_record(record, 'verified')
            return CreatedContainer(cid, intent, phase, operation, spec['Reference'], spec['Image'], record['hash'])
        except BaseException as error:
            owned['failed'] = True
            if record['id'] is None:
                try:
                    self._reconcile(record)
                except DockerError:
                    raise DockerError('ownership_unknown') from None
            if isinstance(error, DockerError):
                raise DockerError(str(error)) from None
            raise DockerError('interrupted' if isinstance(error, (KeyboardInterrupt, SystemExit))
                              else 'command_failed') from None
        finally:
            if env_fd is not None:
                os.close(env_fd)

    @_closed
    def start(self, container_id):
        record = self._record_for_id(container_id)
        run = self._runs[record['run']]
        _require(not run['failed'], 'interrupted')
        self._authenticate_container(record)
        _require(record['state'] == 'verified', 'startup_state')
        try:
            # 'started' durably consumes the attempt before asking the daemon;
            # it is not semantic success, and a lost response never permits replay.
            self._append_record(record, 'started')
            if record['phase'] is not Phase.RUN:
                run['phases'][record['phase']]['started'] = True
            self._call('start', container_id)
            observed = self._authenticate_container(record)
            state = observed['State']
            running = state['Running'] and state['Status'] == 'running'
            service = record['operation'] in (Operation.POSTGRES, Operation.REDIS)
            exited_zero = not state['Running'] and state['Status'] == 'exited' and state['ExitCode'] == 0
            # One-shot results still need N2's bounded purpose-specific validation.
            _require(running or not service and exited_zero, 'startup_state')
        except BaseException:
            run['failed'] = True
            raise

    @_closed
    def remove(self, container_id):
        record = self._record_for_id(container_id)
        if record['state'] == 'removed':
            _require(self._inspect(container_id) is None, 'ownership_unknown')
            return
        observed = self._inspect(container_id)
        if observed is None:
            _require(record['state'] in ('created', 'verified', 'started'), 'ownership_unknown')
            self._append_record(record, 'removed')
            return
        self._authenticate_container(record)
        self._call('stop', container_id)
        self._authenticate_container(record)
        self._call('remove', container_id)
        _require(self._inspect(container_id) is None, 'cleanup_incomplete')
        self._append_record(record, 'removed')

    @_closed
    def journal_state(self, container_id):
        return self._record_for_id(container_id)['state']

    @_closed
    def cleanup(self, run):
        record = self._guard_run(run)
        complete = True
        for intent in record['intents'].values():
            try:
                if intent['id'] is None:
                    self._reconcile(intent)
                self.remove(intent['id'])
            except DockerError:
                complete = False
        try:
            if self._lookup(run=record['token']):
                complete = False
        except DockerError:
            complete = False
        if complete:
            record['cleaned'] = True
        return complete

    def close(self):
        # No Docker action and no recursive deletion of private stage content.
        for fd in getattr(self, '_fds', ()):
            try:
                info = os.fstat(fd)
                if (info.st_dev, info.st_ino) == self._fd_objects[fd]:
                    os.close(fd)
            except OSError:
                pass
        self._fds = set()
        self._fd_objects = {}
        self._runs = {}
        self._resources = {}
        self._containers = {}
        self._closed = True
