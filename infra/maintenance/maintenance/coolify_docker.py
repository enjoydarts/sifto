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
import time

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
_SOURCE_NAMES = frozenset(('host_agent.py', 'coolify_native_controller.py', 'coolify_native_engine.py',
    'maintenance/coolify_systemd.py', 'maintenance/coolify_native_protocol.py',
    'maintenance/coolify_cleanup.py', 'maintenance/coolify_native_owner.py',
    'systemd/sifto-coolify-native-controller.service', 'maintenance/__init__.py', 'maintenance/plan.py',
    'maintenance/host.py', 'maintenance/coolify_host.py', 'maintenance/coolify_plan.py',
    'maintenance/coolify_rehearsal.py', 'maintenance/coolify_evidence.py',
    'maintenance/coolify_docker.py', 'maintenance/coolify_rehearsal_engine.py', 'maintenance/coolify-public-manifest.json',
    'maintenance/coolify_probe.php', 'maintenance/coolify_startup.php', 'maintenance/coolify_startup.py',
    'maintenance/coolify_startup_init', 'maintenance/coolify_startup_middleware.php',
    'maintenance/coolify_startup_nginx.conf', 'maintenance/coolify_horizon_inspect.php', 'maintenance/coolify_startup_site.conf'))
_ENV = {'PATH': '/usr/bin:/bin', 'HOME': '/nonexistent', 'LC_ALL': 'C'}
_PROCESS_CLEANUP = object()
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
            safe = DockerError(str(error))
            safe.cleanup_complete = getattr(error, 'cleanup_complete', False) is True
            raise safe from None
        except (KeyboardInterrupt, SystemExit):
            raise DockerError('interrupted') from None
        except BaseException:
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
    budget: 'BudgetPolicy | None' = None


@dataclass(frozen=True, repr=False)
class BudgetPolicy:
    aggregate_memory: int = 1280 * 1024 * 1024
    aggregate_millicpu: int = 1750
    host_reserve: int = 1024 * 1024 * 1024
    staging_free: int = 4 * 1024 * 1024 * 1024
    whole_seconds: int = 2400
    cleanup_seconds: int = 120


def _budget(context):
    _require(type(context.budget) is BudgetPolicy and context.budget == BudgetPolicy()
             and all(type(value) is int for value in vars(context.budget).values()), 'context_policy')
    return context.budget


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
        self.cleanup_complete = False


class OwnedRun:
    __slots__ = ()

    def __init__(self):
        raise DockerError('stage_identity')


class PhaseResources:
    __slots__ = ()

    def __init__(self):
        raise DockerError('stage_identity')


class OwnedDocker:
    def _require_verified_startup(self):
        # Deliberately empty Task4 registry; no caller record can fill it.
        raise DockerError('startup_profile_unverified')

    def _capacity_observations(self):
        values = {}
        for line in Path('/proc/meminfo').read_text().splitlines():
            if line.startswith('MemAvailable:'):
                values['available'] = int(line.split()[1]) * 1024
        space = os.fstatvfs(self._stage_fd)
        return values.get('available', 0), space.f_bavail * space.f_frsize

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
            if isinstance(adapter, NativeProcess):
                _budget(context)
                adapter._bind_owner(self)
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

    def _exclusive_hashed_file(self, parent, name, content):
        record = self._exclusive_file(parent, name, content)
        record['sha256'] = _hash(record.pop('bytes'))
        self._guard_hashed_file(parent, name, record, 'material_binding')
        return record

    def _guard_hashed_file(self, parent, name, record, category):
        try:
            before = os.fstat(record['fd'])
            _require(_metadata(before) == record['identity']
                     and _metadata(os.stat(name, dir_fd=parent, follow_symlinks=False)) == record['identity']
                     and stat.S_ISREG(before.st_mode) and before.st_nlink == 1
                     and before.st_uid == 0 and stat.S_IMODE(before.st_mode) == 0o600
                     and 0 <= before.st_size <= 64 * 1024 * 1024, category)
            digest, offset = hashlib.sha256(), 0
            while offset < before.st_size:
                chunk = os.pread(record['fd'], min(1024 * 1024, before.st_size - offset), offset)
                _require(bool(chunk), category)
                digest.update(chunk)
                offset += len(chunk)
            _require(digest.hexdigest() == record['sha256']
                     and _metadata(os.fstat(record['fd'])) == record['identity']
                     and _metadata(os.stat(name, dir_fd=parent, follow_symlinks=False)) == record['identity'], category)
        except OSError:
            raise DockerError(category) from None

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
        except DockerError:
            raise
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
            'CgroupnsMode': 'private', 'CgroupParent': self._adapter._parent(run)
                if isinstance(self._adapter, NativeProcess) else str(self.context.cgroup_parent), 'Privileged': False,
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
            if isinstance(self._adapter, NativeProcess):
                self._adapter._authorize_create(record, env_fd)
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
            failure = DockerError(str(error) if isinstance(error, DockerError) else
                                  'interrupted' if isinstance(error, (KeyboardInterrupt, SystemExit))
                                  else 'command_failed')
            if record['id'] is None:
                try:
                    self._reconcile(record)
                except BaseException:
                    # Reconciliation grants ownership only through its exact spec
                    # checks; failure leaves the intent unresolved and the cause intact.
                    pass
            raise failure from None
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

    def _purpose(self, container_id, operations):
        # Purpose check must precede even metadata adapter calls.
        _require(type(container_id) is str and container_id in self._containers, 'ownership_unknown')
        record = self._containers[container_id]
        _require(record['operation'] in operations, 'container_spec')
        _require(record['state'] == 'started' and not self._runs[record['run']]['failed'], 'startup_state')
        observed = self._authenticate_container(record)
        return record, observed

    def _finished(self, record, seconds):
        result = self._call('wait_result', record['id'], seconds=seconds)
        _require(type(result) is bytes and len(result) <= 512 * 1024, 'output_bound')
        observed = self._authenticate_container(record)
        state = observed['State']
        _require(state['Status'] == 'exited' and not state['Running'] and state['ExitCode'] == 0,
                 'startup_state')
        return result

    @_closed
    def read_identity(self, container_id):
        record, _ = self._purpose(container_id, {Operation.IDENTITY_OLD, Operation.IDENTITY_TARGET,
            Operation.IDENTITY_POSTGRES, Operation.IDENTITY_REDIS})
        value = self._finished(record, 30)
        _require(len(value) <= 1024, 'output_bound')
        # Fixed `id account` emits UID, primary GID and a bounded group list.
        match = re.fullmatch(rb'uid=([0-9]{1,5})\(([a-z-]+)\) gid=([0-9]{1,5})\(([a-z-]+)\)'
                             rb'(?: groups=[0-9a-z(), -]+)?\n?', value)
        _require(match is not None, 'image_identity')
        account = {'old': b'www-data', 'target': b'www-data', 'postgres': b'postgres', 'redis': b'redis'}[record['role']]
        _require(match[2] == account and match[4] == account, 'image_identity')
        identity = (int(match[1]), int(match[3]), record['spec']['Image'])
        _require(all(0 < v < 65536 for v in identity[:2]), 'image_identity')
        if record['role'] in ('old', 'target'):
            _require(identity[:2] == (9999, 9999), 'hmac_identity')
        self._runs[record['run']]['identities'][record['role']] = identity
        return identity

    @_closed
    def postgres_ready(self, container_id):
        record, observed = self._purpose(container_id, {Operation.POSTGRES})
        _require(observed['State']['Running'], 'startup_state')
        result = self._call('postgres_ready', record['id'])
        _require(type(result) is bool, 'startup_state')
        self._authenticate_container(record)
        return result

    @_closed
    def restore_original(self, container_id, resources):
        record, _ = self._purpose(container_id, {Operation.POSTGRES})
        _require(record['resources'] is resources, 'material_binding')
        sealed = self._guard_resources(record['run'], record['phase'], resources, ('coolify.dump',))
        _require(not record.get('restore_attempted'), 'startup_state')
        record['restore_attempted'] = True
        fd = os.dup(sealed['leaves']['coolify.dump']['fd'])
        try:
            os.lseek(fd, 0, os.SEEK_SET)
            self._call('restore_original', container_id, dump_fd=fd)
            self._authenticate_container(record)
        except BaseException:
            self._runs[record['run']]['failed'] = True
            raise
        finally:
            os.close(fd)

    @_closed
    def read_probe(self, container_id):
        record, _ = self._purpose(container_id, {Operation.BASELINE_OLD, Operation.NETWORK_OLD,
            Operation.NETWORK_TARGET, Operation.VERIFY_OLD, Operation.VERIFY_TARGET})
        value = strict_json(self._finished(record, 30))
        if record['operation'] in (Operation.NETWORK_OLD, Operation.NETWORK_TARGET):
            _require(type(value) is dict and set(value) == {'ok', 'category', 'network'}
                     and value['ok'] is True and value['network'] is True
                     and value['category'] == 'network_verified', 'network_isolation')
        else:
            from .coolify_evidence import validate_evidence
            try:
                validate_evidence(value, '4.4.6' if record['role'] == 'target' else '4.4.2',
                                  baseline=record['operation'] is Operation.BASELINE_OLD)
            except Exception:
                raise DockerError('evidence_invalid') from None
        return value

    @_closed
    def read_migration(self, container_id):
        record, _ = self._purpose(container_id, {Operation.MIGRATE_OLD, Operation.MIGRATE_TARGET})
        _require(not record.get('result_consumed'), 'startup_state')
        record['result_consumed'] = True
        self._finished(record, 180)
        # No parsing of localized artisan prose; VERIFY must prove no_pending.
        return True

    @_closed
    def read_ssh_reconstruction(self, container_id):
        record, _ = self._purpose(container_id, {Operation.SSH_OLD, Operation.SSH_TARGET})
        # Empty reviewed Task4 registry. Never run a placeholder executable.
        raise DockerError('startup_profile_unverified')

    @_closed
    def held_ready(self, container_id):
        self._purpose(container_id, {Operation.HELD_OLD, Operation.HELD_TARGET})
        raise DockerError('startup_profile_unverified')

    def close(self):
        # No Docker action and no recursive deletion of private stage content.
        if isinstance(getattr(self, '_adapter', None), NativeProcess):
            self._adapter._close()
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


def strict_json(raw):
    _require(type(raw) is bytes and 0 < len(raw) <= 512 * 1024, 'output_bound')
    def pairs(items):
        value = {}
        for key, item in items:
            _require(key not in value, 'evidence_invalid')
            value[key] = item
        return value
    def constant(_):
        raise DockerError('evidence_invalid')
    try:
        value = json.loads(raw.decode('utf-8', 'strict'), object_pairs_hook=pairs,
                           parse_constant=constant)
        pending, nodes = [(value, 0)], 0
        while pending:
            item, depth = pending.pop()
            nodes += 1
            _require(nodes <= 20000 and depth <= 16, 'evidence_invalid')
            if type(item) is dict:
                _require(len(item) <= 2048 and all(len(k) <= 1024 for k in item), 'evidence_invalid')
                pending.extend((v, depth + 1) for v in item.values())
            elif type(item) is list:
                _require(len(item) <= 4096, 'evidence_invalid')
                pending.extend((v, depth + 1) for v in item)
            elif type(item) is str:
                _require(len(item) <= 4096, 'evidence_invalid')
            else:
                _require(type(item) in (int, bool, type(None)), 'evidence_invalid')
        return value
    except DockerError:
        raise
    except Exception:
        raise DockerError('evidence_invalid') from None


class NativeProcess:
    """Closed local Docker transport. Only OwnedDocker can authorize creation.

    Every command runs below a live session supervisor that reaps descendants.
    The supervisor remains the process-group leader after an early CLI exit;
    signalling is guarded by its pidfd and /proc start token, never a saved PGID.
    """
    def __init__(self, context):
        _require(type(context) is NativeContext, 'context_policy')
        _budget(context)
        self.context = context
        self._owner = None
        self._run = None
        self._authorization = None
        self._output_total = 0
        self._serial = 0
        self._deadline = time.monotonic() + 2400
        self._cgroup = None
        self._transport_clean = True

    def _bind_owner(self, owner):
        _require(self._owner is None and type(owner) is OwnedDocker and owner.context is self.context,
                 'context_policy')
        self._owner = owner

    def _bind_run(self, run):
        _require(self._owner is not None and self._run is None, 'stage_identity')
        record = self._owner._guard_run(run)
        self._run = run
        self._output_fd = record['fd']
        self._cgroup = CgroupBudget(self.context.cgroup_parent, record['token'])
        self._cgroup.preflight_entry()
        # Even the first metadata CLI inherits the authenticated entry envelope.
        info = strict_json(self._invoke(['info', '--format',
            '{"driver":{{json .CgroupDriver}},"version":{{json .CgroupVersion}}}'], limit=65536))
        self._cgroup.allocate(info)

    def _parent(self, run):
        _require(run is self._run and self._cgroup is not None, 'resource_isolation')
        self._sync_cgroups()
        self._cgroup.guard()
        return self._cgroup.docker_parent

    def _steady(self):
        _require(self._cgroup is not None, 'resource_isolation')
        self._sync_cgroups()
        self._cgroup.steady()

    def _preparation(self):
        _require(self._owner is not None and self._run is not None
                 and all(c['state'] == 'removed' for c in self._owner._runs[self._run]['intents'].values()),
                 'resource_isolation')
        self._sync_cgroups()
        self._cgroup.preparation()

    def _finish(self):
        _require(self._transport_clean is True and self._owner is not None and self._run is not None
                 and self._owner.cleanup(self._run), 'cleanup_incomplete')
        if self._cgroup is not None:
            self._sync_cgroups()
            self._cgroup.release()

    def _sync_cgroups(self):
        if self._cgroup is not None:
            self._cgroup._expect_containers(frozenset(cid for cid, record in self._owner._containers.items()
                if record['run'] is self._run and record['state'] != 'removed'))

    def _authorize_create(self, record, env_fd):
        _require(self._owner is not None and record is
                 self._owner._runs[record['run']]['intents'][record['intent']], 'container_spec')
        _require(record['run'] is self._run, 'stage_identity')
        self._parent(record['run'])
        _require(self._cgroup._mode == 'steady', 'resource_isolation')
        self._authorization = (tuple(self._owner._argv(record['spec'], env_fd)),
            () if env_fd is None else (env_fd,))

    def _close(self):
        if self._cgroup is not None:
            self._cgroup._close_pins()

    @_closed
    def create(self, argv, *, env, pass_fds):
        _require(self._authorization is not None
                 and (tuple(argv), tuple(pass_fds)) == self._authorization and env == _ENV, 'container_spec')
        self._authorization = None
        return self._invoke(argv, pass_fds=pass_fds, limit=65536).decode('ascii').strip()

    @_closed
    def lookup(self, *, name=None, intent=None, run=None):
        _require(sum(v is not None for v in (name, intent, run)) == 1, 'ownership_unknown')
        if name is not None:
            _require(type(name) is str and re.fullmatch(r'sifto-native-[a-f0-9]{32}-[a-f0-9]{32}', name),
                     'ownership_unknown')
            selected = 'name=^/' + name + '$'
        else:
            value = intent if intent is not None else run
            _require(type(value) is str and re.fullmatch(r'[a-f0-9]{32}', value), 'ownership_unknown')
            selected = 'label=sifto.' + ('intent' if intent is not None else 'run') + '=' + value
        raw = self._invoke(['container', 'ls', '--all', '--no-trunc', '--format={{.ID}}',
                            '--filter=' + selected], limit=65536)
        ids = raw.decode('ascii', 'strict').splitlines()
        _require(len(ids) <= 64 and all(_HASH.fullmatch(v) for v in ids), 'ownership_unknown')
        return ids

    @_closed
    def inspect_image(self, reference):
        legal = {i['repository'] + '@' + i['platform_manifest']
                 for side in ('rollback', 'target') for i in fixed_profile()['bundle'][side]['images'].values()}
        _require(reference in legal, 'image_descriptor')
        template = '{"engine_id":{{json .Id}},"os":{{json .Os}},"architecture":{{json .Architecture}},"descriptor":{{json .Descriptor}}}'
        value = strict_json(self._invoke(['image', 'inspect', '--format=' + template, reference], limit=65536))
        _require(type(value) is dict and type(value.get('descriptor')) is dict, 'image_descriptor')
        descriptor = value['descriptor']
        value['descriptor'] = {k: descriptor.get(k) for k in ('digest', 'mediaType')}
        return value

    def _id(self, cid, *, candidate=False):
        _require(type(cid) is str and _HASH.fullmatch(cid), 'container_identity')
        _require(self._owner is not None and self._run in self._owner._runs, 'ownership_unknown')
        record = self._owner._containers.get(cid)
        _require((record is not None and record['run'] is self._run)
                 or (candidate and any(c['id'] is None
                     for c in self._owner._runs[self._run]['intents'].values())), 'ownership_unknown')

    @_closed
    def inspect(self, cid):
        self._id(cid, candidate=True)
        # Selected keys only: never Config.Env, full config, health logs or logs stderr.
        config = {'Image': '.Image', 'Reference': '.Config.Image', 'Labels': '.Config.Labels',
            'User': '.Config.User', 'Entrypoint': '.Config.Entrypoint', 'Cmd': '.Config.Cmd',
            'WorkingDir': '.Config.WorkingDir', 'Mounts': '.Mounts', 'Tmpfs': '.HostConfig.Tmpfs'}
        for key in ('Memory', 'MemorySwap', 'NanoCpus', 'PidsLimit', 'CapDrop', 'CapAdd',
                    'SecurityOpt', 'ReadonlyRootfs', 'NetworkMode', 'PidMode', 'IpcMode',
                    'CgroupnsMode', 'CgroupParent', 'Privileged', 'ExtraHosts', 'Devices'):
            config[key] = '.HostConfig.' + key
        config.update(Id='.Id', Name='.Name', Ports='.HostConfig.PortBindings')
        template = '{' + ','.join('"' + k + '":{{json ' + v + '}}' for k, v in config.items())
        template += ',"State":{"Status":{{json .State.Status}},"Running":{{json .State.Running}},"ExitCode":{{json .State.ExitCode}},"OOMKilled":{{json .State.OOMKilled}}}}'
        code, raw, error = self._invoke_status(['container', 'inspect', '--format=' + template, cid], limit=65536)
        if code:
            # Only a typed absent result is accepted; arbitrary errors are failures.
            _require(error.strip() in (('Error: No such container: ' + cid).encode(),
                     ('Error response from daemon: No such container: ' + cid).encode()), 'command_failed')
            return None
        value = strict_json(raw)
        _require(type(value) is dict, 'container_spec')
        value['Name'] = value['Name'].removeprefix('/')
        for key in ('CapAdd', 'Devices', 'ExtraHosts'):
            if value[key] is None:
                value[key] = []
        if value['Ports'] is None:
            value['Ports'] = {}
        value['Mounts'] = [{k: m[k] for k in ('Type', 'Source', 'Destination', 'RW', 'Propagation')}
                           for m in value['Mounts']]
        if value['State']['Running']:
            self._verify_placement(cid, value)
        return value

    def _verify_placement(self, cid, value):
        pid = self._invoke(['container', 'inspect', '--format={{.State.Pid}}', cid], limit=64)
        _require(re.fullmatch(rb'[1-9][0-9]{0,9}\n?', pid) is not None, 'resource_isolation')
        self._sync_cgroups()
        self._cgroup.verify_container(int(pid), value)

    @_closed
    def start(self, cid):
        self._id(cid)
        self._invoke(['container', 'start', cid], limit=65536)

    @_closed
    def stop(self, cid):
        self._id(cid)
        self._invoke(['container', 'stop', '--time=5', cid], limit=65536)

    @_closed
    def remove(self, cid):
        self._id(cid)
        self._invoke(['container', 'rm', '--volumes', cid], limit=65536)

    @_closed
    def wait_result(self, cid, *, seconds):
        self._id(cid)
        _require(seconds in (30, 180, 900), 'context_policy')
        result = self._invoke(['container', 'wait', cid], seconds=seconds, limit=64)
        _require(result.strip() == b'0', 'startup_state')
        # stdout only is a producer protocol. Never publish or read Docker stderr logs.
        return self._invoke(['container', 'logs', cid], limit=512 * 1024)

    @_closed
    def postgres_ready(self, cid):
        self._id(cid)
        _require(self._owner._containers[cid]['operation'] is Operation.POSTGRES, 'container_spec')
        code, value, error = self._invoke_status(['exec', '--user=postgres', cid, 'psql',
            '--no-psqlrc', '--host=127.0.0.1', '--username=coolify', '--dbname=coolify',
            '--no-password', '--tuples-only', '--no-align', '--command=SELECT 1'], limit=4096)
        if code == 0:
            _require(value == b'1\n' and error == b'', 'startup_state')
            return True
        # libpq's two fixed readiness failures only. Authentication/SQL errors fail closed.
        _require(code == 2 and value == b'' and error.startswith(b'psql: error: connection to server at "127.0.0.1", port 5432 failed: ')
                 and (b'Connection refused' in error or b'the database system is starting up' in error)
                 and len(error) <= 2048, 'startup_state')
        return False

    @_closed
    def restore_original(self, cid, *, dump_fd):
        self._id(cid)
        record = self._owner._containers[cid]
        _require(record['operation'] is Operation.POSTGRES and record.get('restore_attempted'), 'container_spec')
        sealed = self._owner._guard_resources(record['run'], record['phase'], record['resources'], ('coolify.dump',))
        _require(_metadata(os.fstat(dump_fd)) == sealed['leaves']['coolify.dump']['identity'], 'material_binding')
        self._invoke(['exec', '-i', '--user=postgres', cid, 'pg_restore', '--exit-on-error',
            '--no-owner', '--no-privileges', '--host=127.0.0.1', '--username=coolify', '--dbname=coolify'],
            seconds=180, stdin_fd=dump_fd, limit=65536)

    def _invoke(self, argv, **kwargs):
        code, value, _ = self._invoke_status(argv, **kwargs)
        _require(code == 0, 'command_failed')
        return value

    def _invoke_status(self, argv, *, pass_fds=(), stdin_fd=None, seconds=30, limit=65536):
        _require(self._owner is not None and self._run is not None, 'stage_identity')
        self._owner._guard_run(self._run)
        _require(self._cgroup is not None and self._cgroup.preflighted, 'resource_isolation')
        self._sync_cgroups()
        self._cgroup.guard()
        _require(type(seconds) is int and 0 < seconds <= 900 and 0 < limit <= 1024 * 1024,
                 'context_policy')
        # Reserve process TERM/KILL/reap time inside both the operation and the
        # shared whole/cleanup deadline; a last command cannot extend either.
        deadline = min(self._deadline, time.monotonic() + seconds) - 3
        _require(time.monotonic() < deadline, 'command_timeout')
        remaining = 16 * 1024 * 1024 - self._output_total
        _require(remaining >= 2, 'output_bound')
        limit = min(limit, remaining // 2)
        self._serial += 1
        prefix = 'command-' + str(self._serial)
        try:
            result = _supervise(['/usr/bin/docker', '--host=unix://' + str(self.context.docker_socket), *argv],
                output_fd=self._output_fd, prefix=prefix, pass_fds=tuple(pass_fds),
                stdin_fd=stdin_fd, deadline=deadline, limit=limit,
                attach=self._cgroup.attach)
        except DockerError as error:
            if getattr(error, '_process_cleanup', None) is not _PROCESS_CLEANUP:
                self._transport_clean = False
            raise
        finally:
            # Failed producers/timeouts also consume the private output budget.
            for suffix in ('stdout', 'stderr'):
                try:
                    self._output_total += os.stat(prefix + '.' + suffix,
                        dir_fd=self._output_fd, follow_symlinks=False).st_size
                except FileNotFoundError:
                    pass
        _require(self._output_total <= 16 * 1024 * 1024, 'output_bound')
        return result


def _start_token(pid):
    # comm may contain spaces or parentheses; field 22 follows the last ')'.
    value = Path('/proc/' + str(pid) + '/stat').read_bytes()
    return value[value.rfind(b')') + 2:].split()[19]


def _scheduler_token(procfd):
    raw = _FixedRoleProcessControl._proc_read(procfd, 'stat', 8192)
    fields = raw[raw.rfind(b')') + 2:].split()
    status = _FixedRoleProcessControl._proc_read(procfd, 'status', 16384).splitlines()
    ids = [line.split()[1:] for line in status if line.startswith((b'Uid:', b'Gid:'))]
    _require(len(fields) == 50 and len(ids) == 2 and all(len(row) == 4 for row in ids), 'startup_state')
    return [int(fields[19]), int(fields[1]), [[int(part) for part in row] for row in ids]]


def _scheduler_terminal(procfd, token):
    """Known Z plus positive proc access witness; disappearance is unknown.

    Upstream proc_pid_syscall checks PTRACE_MODE_ATTACH_FSCREDS before emitting
    metadata. That successful check is stronger than stat's READ_FSCREDS check
    under the supported standard Linux/LSM profile. Root/capability names alone
    are NOT evidence. Do not expose the syscall metadata or read process memory.
    """
    try:
        if _scheduler_token(procfd) != token:
            return 'unknown'
        witness = _FixedRoleProcessControl._proc_read(procfd, 'syscall', 256)
        if witness != b'-1 0x0 0x0\n':
            return 'unknown'
        raw = _FixedRoleProcessControl._proc_read(procfd, 'stat', 8192)
        fields = raw[raw.rfind(b')') + 2:].split()
        if len(fields) != 50 or fields[0] != b'Z' or _scheduler_token(procfd) != token:
            return 'unknown'
        code = int(fields[49])
        if not 0 <= code <= 65535:
            return 'unknown'
        if os.WIFEXITED(code):
            return 'finished' if os.WEXITSTATUS(code) == 0 else 'failed'
        return 'signaled' if os.WIFSIGNALED(code) else 'unknown'
    except (OSError, ValueError, DockerError):
        return 'unknown'


class _SchedulerDrain:
    """Private finite observer; only its already-authenticated control signals.

    Keep the launcher's full original immutable snapshot separate from work
    inventory. No reaping, pipe closing, handle replacement or job signals.
    """
    def __init__(self, control):
        _require(type(control._role) is str and control._role == 'scheduler-worker', 'startup_state')
        self._control = control
        self._known = {}
        self._unknown = set()
        self._stable = {}
        rows = self._inventory()
        descendants = {control._pid}
        for _ in range(256):
            added = {pid for pid, (_, token) in rows.items() if token[1] in descendants} - descendants
            if not added:
                break
            descendants |= added
        expected_stable = getattr(control, '_scheduler_stable', None)
        for pid, (fd, token) in rows.items():
            if pid == control._pid:
                os.close(fd)
            elif pid in descendants:
                self._known[pid] = [fd, token]
            elif expected_stable is not None and pid in expected_stable:
                identity = control._proc_identity(fd)
                _require(identity == expected_stable[pid], 'startup_state')
                self._stable[pid] = identity
                os.close(fd)
            else:
                self._unknown.add(pid)
                os.close(fd)
        if expected_stable is not None:
            _require(set(self._stable) == set(expected_stable), 'startup_state')

    def close(self):
        for fd, _ in self._known.values():
            os.close(fd)
        self._known.clear()

    def _inventory(self):
        control = self._control
        identity = control._identity()
        rows = {}
        try:
            for pid, start in identity['members']:
                fd = os.open('/proc/' + str(pid), os.O_RDONLY | os.O_DIRECTORY | os.O_NOFOLLOW)
                try:
                    token = _scheduler_token(fd)
                    _require(str(token[0]) == start, 'startup_state')
                except BaseException:
                    os.close(fd)
                    raise
                rows[pid] = (fd, token)
            _require(identity['members'] == control._identity()['members'], 'startup_state')
            return rows
        except BaseException:
            for fd, _ in rows.values():
                os.close(fd)
            raise

    def _result(self, category, counts):
        import hmac
        from .coolify_startup import parse_scheduler_result
        control = self._control
        return parse_scheduler_result(_json(dict(version=1, category=category,
            live_count=counts[0], finished_count=counts[1], failed_count=counts[2], unknown_count=counts[3],
            restore_required=bool(control._intent and control._intent['restore_required']),
            maintenance_held=True, identity_hmac=hmac.new(control._key,
                _json([control._selected, control._pins]), hashlib.sha256).hexdigest())))

    def observe(self):
        control = self._control
        counts = [0, 0, 0, 0]
        rows = {}
        try:
            control._journal_matches()
            control._restoration_guard(control._deadline)
            _require(all(state == 'T' for _, state in control._tasks()), 'startup_state')
            rows = self._inventory()
            # A later dueEvents entry can launch after the barrier. Include new
            # observed descendants; do not whitelist unrelated role descendants.
            for _ in range(256):
                added = False
                for pid, (fd, token) in list(rows.items()):
                    if pid not in self._known and pid not in self._stable and pid != control._pid:
                        if token[1] in self._known or token[1] == control._pid:
                            self._known[pid] = [os.dup(fd), token]
                            added = True
                if not added:
                    break
            for pid, original in self._stable.items():
                if pid not in rows or control._proc_identity(rows[pid][0]) != original:
                    self._unknown.add(pid)
            for pid in rows:
                if pid != control._pid and pid not in self._known and pid not in self._stable:
                    self._unknown.add(pid)
            for pid, (fd, token) in self._known.items():
                # Linux excludes zombies from cgroup.procs. A retained procfd
                # can still supply identity-bound positive terminal evidence;
                # an absent/reaped proc or pidfd readiness alone never can.
                if (pid in rows and rows[pid][1] != token) or _scheduler_token(fd) != token:
                    self._unknown.add(pid)
                    continue
                raw = control._proc_read(fd, 'stat', 8192)
                state = raw[raw.rfind(b')') + 2:].split()[0]
                if state == b'Z':
                    outcome = _scheduler_terminal(fd, token)
                    if outcome == 'finished':
                        counts[1] += 1
                    elif outcome == 'failed':
                        counts[2] += 1
                    else:
                        self._unknown.add(pid)
                elif state in (b'R', b'S', b'D', b'I'):
                    counts[0] += 1
                else:
                    self._unknown.add(pid)
            counts[3] = len(self._unknown)
            _require(sum(counts) <= 256, 'output_bound')
            control._restoration_guard(control._deadline)
            _require(all(state == 'T' for _, state in control._tasks()), 'startup_state')
            final = self._inventory()
            try:
                _require({pid: token for pid, (_, token) in rows.items()}
                         == {pid: token for pid, (_, token) in final.items()}, 'startup_state')
                for pid, original in self._stable.items():
                    _require(pid in final and control._proc_identity(final[pid][0]) == original, 'startup_state')
            finally:
                for fd, _ in final.values():
                    os.close(fd)
            _require(all(state == 'T' for _, state in control._tasks()), 'startup_state')
            control._final_guard(control._deadline)
            return self._result('quiescent' if not (counts[0] or counts[2] or counts[3]) else 'held', counts)
        except (OSError, ValueError, DockerError):
            return self._result('held', [0, 0, 0, max(1, min(256, len(self._unknown)))])
        finally:
            for fd, _ in rows.values():
                os.close(fd)

    def run(self):
        control = self._control
        frozen = control.freeze()
        if frozen.category not in ('frozen', 'already-stopped'):
            return self._result('rejected', [0, 0, 0, 1])
        result = self.observe()
        while (result.live_count and not result.unknown_count and not result.failed_count
               and time.monotonic() < control._deadline):
            time.sleep(.005)
            if time.monotonic() >= control._deadline:
                break
            observed = self.observe()
            if time.monotonic() >= control._deadline:
                break
            result = observed
        if result.category != 'quiescent':
            control.restore()
            # Restoration is independent evidence, never work completion.
            result = self._result('held', [result.live_count, result.finished_count,
                                            result.failed_count, result.unknown_count])
        return result


class _FixedRoleProcessControl:
    """Dormant K0 same-PID-namespace component, NOT native App authority.

    No constructor/selector is registered. PUBLIC tests alone assemble this
    object from harness-owned children and synthetic pinned context descriptors.
    The private PUBLIC native-fixture subclass joins authenticated init/S6 role,
    host namespace/cgroup mapping and barrier/generation projection. Production
    selection and the separate full native allocation gate remain closed.
    Caller observations, a PID, role string or boolean cannot enable that join.

    STOP/CONT here address ONLY a retained pidfd, never service names, process
    groups or descendants. Snapshots do not prove exclusive foreign signal
    ownership. Durable state records an obligation, not arbitrary crash recovery.
    """
    def __init__(self):
        raise DockerError('startup_profile_unverified')

    @staticmethod
    def _proc_read(fd, leaf, limit=131072):
        opened = os.open(leaf, os.O_RDONLY | os.O_CLOEXEC | os.O_NOFOLLOW, dir_fd=fd)
        try:
            value = os.read(opened, limit + 1)
            _require(len(value) <= limit, 'output_bound')
            return value
        finally:
            os.close(opened)

    def _read_pins(self):
        values = []
        for fd in (self._sourcefd, self._contextfd):
            before = _metadata(os.fstat(fd))
            raw = _read(fd, 131072)
            _require(before == _metadata(os.fstat(fd)), 'startup_state')
            values.append([list(before), _hash(raw)])
        return values

    def _tasks(self):
        tasksfd = os.open('task', os.O_RDONLY | os.O_DIRECTORY | os.O_NOFOLLOW, dir_fd=self._procfd)
        try:
            names = sorted(os.listdir(tasksfd), key=int)
            _require(0 < len(names) <= 256, 'startup_state')
            values = []
            for name in names:
                raw = self._proc_read(tasksfd, name + '/stat', 8192)
                fields = raw[raw.rfind(b')') + 2:].split()
                values.append([int(name), fields[0].decode('ascii')])
            _require(names == sorted(os.listdir(tasksfd), key=int), 'startup_state')
            return values
        finally:
            os.close(tasksfd)

    def _proc_identity(self, fd, *, native_titles=False):
        raw = self._proc_read(fd, 'stat', 8192)
        fields = raw[raw.rfind(b')') + 2:].split()
        status = self._proc_read(fd, 'status').decode('ascii')
        wanted = ('Uid:', 'Gid:', 'NSpid:', 'Groups:')
        ids = {line.split(':', 1)[0]: line.split()[1:] for line in status.splitlines()
               if line.startswith(wanted)}
        _require(set(ids) == {'Uid', 'Gid', 'NSpid', 'Groups'} and ids['NSpid']
                 and all(part.isascii() and part.isdecimal() for values in ids.values() for part in values),
                 'startup_state')
        namespaces = [os.readlink('ns/' + name, dir_fd=fd) for name in ('pid', 'mnt', 'net')]
        exe = os.stat('exe', dir_fd=fd)
        command = self._proc_read(fd, 'cmdline', 32768)
        # These fixed native services use setproctitle's NUL padding. No other
        # argv grammar or the default K0 component is broadened.
        if native_titles and command.startswith((b'php-fpm: ', b'nginx: ')):
            command = command.rstrip(b'\0') + b'\0'
        argv = command.split(b'\0')
        _require(argv[-1] == b'' and all(argv[:-1]), 'startup_state')
        return dict(start=fields[19].decode('ascii'), parent=int(fields[1]), ids=ids,
                    namespaces=namespaces, cgroup=self._proc_read(fd, 'cgroup', 16384).decode('ascii'),
                    exe=list(_metadata(exe)), argv=[part.hex() for part in argv[:-1]])

    def _placement(self, selected, parent, init):
        # Default component remains restricted to its own namespace/cgroup.
        # The PUBLIC native fixture overrides this with authenticated daemon
        # ownership, init and host/container namespace mapping checks.
        current = Path('/proc/self/cgroup').read_text()
        _require(selected['cgroup'] == current and parent['cgroup'] == current
                 and selected['namespaces'] == [os.readlink('/proc/self/ns/' + name)
                     for name in ('pid', 'mnt', 'net')], 'startup_state')
        return 'same-pid-namespace-component', Path('/sys/fs/cgroup/cgroup.procs')

    def _identity(self):
        selected = self._proc_identity(self._procfd)
        parentfd = os.open('/proc/' + str(selected['parent']), os.O_RDONLY | os.O_DIRECTORY | os.O_NOFOLLOW)
        try:
            parent = self._proc_identity(parentfd)
        finally:
            os.close(parentfd)
        init = self._proc_identity(self._initfd)
        # No assumption that UID 9999 defines membership; include EVERY member.
        scope, membership = self._placement(selected, parent, init)
        _require(_start_token(self._pid).decode('ascii') == selected['start'], 'startup_state')
        members = sorted(int(part) for part in membership.read_text().split())
        _require(0 < len(members) <= 256 and self._pid in members
                 and all(pid > 0 for pid in members), 'startup_state')
        member_tokens = [[pid, _start_token(pid).decode('ascii')] for pid in members]
        _require(members == sorted(int(part) for part in membership.read_text().split()),
                 'startup_state')
        executable_limit = self._executable_limit()
        _require(list(_metadata(os.fstat(self._exefd))) == selected['exe']
                 and 0 < os.fstat(self._exefd).st_size <= executable_limit, 'startup_state')
        task_tokens = []
        for tid, _ in self._tasks():
            raw = self._proc_read(self._procfd, 'task/' + str(tid) + '/stat', 8192)
            task_tokens.append([tid, raw[raw.rfind(b')') + 2:].split()[19].decode('ascii')])
        return dict(scope=scope, role=self._role, task_tokens=task_tokens,
                    boot=Path('/proc/sys/kernel/random/boot_id').read_text().strip(),
                    process=selected, parent=parent, init=init, members=member_tokens,
                    executable_sha256=self._executable_hash(executable_limit))

    def _executable_hash(self, limit):
        before = _metadata(os.fstat(self._exefd))
        size = os.fstat(self._exefd).st_size
        _require(0 < size <= limit, 'startup_state')
        digest = hashlib.sha256()
        offset = 0
        while offset < size:
            chunk = os.pread(self._exefd, min(1024 * 1024, size - offset), offset)
            _require(chunk, 'startup_state')
            digest.update(chunk)
            offset += len(chunk)
        _require(before == _metadata(os.fstat(self._exefd)), 'startup_state')
        return digest.hexdigest()

    def _executable_limit(self):
        return 16 * 1024 * 1024

    def _alive(self):
        import select
        _require(not select.select([self._pidfd], [], [], 0)[0], 'startup_state')

    def _guard(self, deadline):
        # All context I/O precedes a full identity reread and the final pidfd /
        # generation/deadline barrier immediately adjacent to send_signal.
        _require(self._read_pins() == self._pins, 'startup_state')
        _require(self._identity() == self._selected, 'startup_state')
        self._final_guard(deadline)

    def _restoration_guard(self, deadline):
        _require(self._read_pins() == self._pins, 'startup_state')
        current = self._identity()  # still reads and bounds the complete closure
        # Other cgroup members may finish while the exact launcher is stopped.
        # Their changing inventory is not restoration authority or work success.
        _require({key: value for key, value in current.items() if key != 'members'}
                 == {key: value for key, value in self._selected.items() if key != 'members'}, 'startup_state')
        self._final_guard(deadline)

    def _final_guard(self, deadline):
        _require([list(_metadata(os.fstat(fd))) for fd in (self._sourcefd, self._contextfd)]
                 == [pin[0] for pin in self._pins], 'startup_state')
        self._alive()
        _require(time.monotonic() < deadline, 'command_timeout')

    def _drain_guard(self, deadline):
        self._guard(min(deadline, self._deadline))

    def _save(self, stage, restore_required):
        value = dict(version=1, role=self._role, stage=stage, original=self._original,
                     identity=self._selected, pins=self._pins, restore_required=restore_required)
        raw = _json(value)
        _require(len(raw) <= 131072, 'output_bound')
        pending = self._role + '.pending'
        fd = os.open(pending, os.O_WRONLY | os.O_CREAT | os.O_EXCL | os.O_NOFOLLOW | os.O_CLOEXEC,
                     0o600, dir_fd=self._dirfd)
        try:
            _require(os.write(fd, raw) == len(raw), 'startup_state')
            os.fsync(fd)
        finally:
            os.close(fd)
        os.rename(pending, self._role + '.json', src_dir_fd=self._dirfd, dst_dir_fd=self._dirfd)
        os.fsync(self._dirfd)
        self._intent = value

    def _journal_matches(self):
        fd = os.open(self._role + '.json', os.O_RDONLY | os.O_NOFOLLOW | os.O_CLOEXEC, dir_fd=self._dirfd)
        try:
            info = os.fstat(fd)
            _require(stat.S_ISREG(info.st_mode) and info.st_nlink == 1
                     and info.st_uid == os.geteuid() and stat.S_IMODE(info.st_mode) == 0o600, 'startup_state')
            raw = _read(fd, 131072)
            _require(strict_json(raw) == self._intent and raw == _json(self._intent), 'startup_state')
        finally:
            os.close(fd)

    def _result(self, category):
        import hmac
        from .coolify_startup import parse_process_control_result
        count = len(self._original or []) if category not in ('rejected', 'busy') else 0
        return parse_process_control_result(_json(dict(version=1, role=self._role, category=category,
            task_count=count, restore_required=category in ('frozen', 'held'), maintenance_held=True,
            identity_hmac=hmac.new(self._key, _json([self._selected, self._pins]), hashlib.sha256).hexdigest())))

    def _lock(self):
        import fcntl
        directory = os.fstat(self._dirfd)
        info = os.fstat(self._lockfd)
        _require(directory.st_uid == os.geteuid() == 0 and stat.S_IMODE(directory.st_mode) == 0o700
                 and stat.S_ISREG(info.st_mode) and info.st_uid == 0 and info.st_nlink == 1
                 and stat.S_IMODE(info.st_mode) == 0o600
                 and _metadata(info) == _metadata(os.stat(self._role + '.lock',
                     dir_fd=self._dirfd, follow_symlinks=False)), 'startup_state')
        fcntl.flock(self._lockfd, fcntl.LOCK_EX | fcntl.LOCK_NB)

    def freeze(self):
        import fcntl
        import signal
        try:
            _require(self._role in ('scheduler-worker', 'terminal-server'), 'startup_state')
            self._lock()
        except (OSError, DockerError):
            return self._result('busy')
        try:
            # Existing/partial private state is ambiguous, never overwritten.
            _require(self._intent is None and self._original is None, 'startup_state')
            for leaf in (self._role + '.json', self._role + '.pending'):
                try:
                    os.stat(leaf, dir_fd=self._dirfd, follow_symlinks=False)
                except FileNotFoundError:
                    continue
                self._ambiguous = True
                raise DockerError('startup_state')
            limit = min(self._deadline, time.monotonic() + 1)
            self._drain_guard(limit)
            original = self._tasks()
            states = {state for _, state in original}
            _require(states == {'T'} or states <= {'R', 'S', 'D', 'I'}, 'startup_state')
            _require(original == self._tasks(), 'startup_state')
            self._original = original
            self._sent = False
            self._save('intent', states != {'T'})
            self._journal_matches()
            if states == {'T'}:
                self._drain_guard(limit)
                self._save('unchanged', False)
                self._drain_guard(limit)
                return self._result('already-stopped')
            self._drain_guard(limit)
            signal.pidfd_send_signal(self._pidfd, signal.SIGSTOP)
            self._sent = True
            self._save('stop-sent', True)
            while time.monotonic() < limit:
                self._drain_guard(limit)
                tasks = self._tasks()
                if ([tid for tid, _ in tasks] == [tid for tid, _ in original]
                        and all(state == 'T' for _, state in tasks) and tasks == self._tasks()):
                    self._drain_guard(limit)
                    self._save('applied', True)
                    self._drain_guard(limit)
                    return self._result('frozen')
                # A bounded poll, never used to position an adversarial race.
                time.sleep(.001)
            raise DockerError('command_timeout')
        except BaseException:
            if self._original is None:
                return self._result('held' if getattr(self, '_ambiguous', False) else 'rejected')
            if getattr(self, '_sent', False):
                restored = self._restore_locked()
                return self._result('freeze-restored') if restored.category == 'restored' else restored
            return self._result('held')
        finally:
            fcntl.flock(self._lockfd, fcntl.LOCK_UN)

    def _restore_locked(self):
        import signal
        limit = time.monotonic() + 1  # one budget starts BEFORE any journal I/O
        try:
            self._journal_matches()
            _require(self._original is not None, 'startup_state')
            if all(state == 'T' for _, state in self._original):
                _require(self._tasks() == self._original, 'startup_state')
                self._restoration_guard(limit)
                return self._result('unchanged')
            # An intent alone or a restarted object cannot assert that its STOP
            # succeeded. Only this live attempt's acknowledged syscall may CONT.
            _require(self._sent and self._intent['restore_required'], 'startup_state')
            _require([tid for tid, _ in self._tasks()] == [tid for tid, _ in self._original], 'startup_state')
            self._restoration_guard(limit)
            signal.pidfd_send_signal(self._pidfd, signal.SIGCONT)
            while time.monotonic() < limit:
                self._restoration_guard(limit)
                tasks = self._tasks()
                if ([tid for tid, _ in tasks] == [tid for tid, _ in self._original]
                        and all(state in ('R', 'S', 'D', 'I') for _, state in tasks)):
                    self._save('restored', False)
                    self._restoration_guard(limit)
                    return self._result('restored')
                time.sleep(.001)
        except BaseException:
            pass
        return self._result('held')

    def restore(self):
        import fcntl
        try:
            self._lock()
        except (OSError, DockerError):
            return self._result('busy')
        try:
            return self._restore_locked()
        finally:
            fcntl.flock(self._lockfd, fcntl.LOCK_UN)


# PUBLIC fixture only. Exact raw leaf manifests in tests/scheduler_image_*.json
# cryptographically bind these reviewed config IDs to the pinned references.
# Docker's classic store reports the config ID; containerd reports the target
# manifest ID and its Descriptor. Neither shape changes the fixed image pins.
_SCHEDULER_IMAGE_PINS = (
    ('old', 'docker.io/coollabsio/coolify@sha256:a323ce1dfb54101b0c30d8bd5c92b0e4166c3b403a164316f8876be653204960', 'sha256:6145ffe0650d8dcdef48ab08442dc01b066d6450e1f6df9e3cd8efacadbb0064', 'application/vnd.oci.image.manifest.v1+json'),
    ('target', 'docker.io/coollabsio/coolify@sha256:f4ef1eb69daa265057ab700646485bd78bb43bcf96c691d65094947acb7f50a5', 'sha256:292e1d3e300d6eab9598e14a017bfa2e35bd89f78597916c179c24e099e40156', 'application/vnd.oci.image.manifest.v1+json'),
    ('postgres', 'docker.io/library/postgres@sha256:91a22c143801df42c2e4a8f9926eab5e48f39f40beb5220b7c29d4478263c341', 'sha256:00f747f11f25a40d61020089b8cafd867de9f8716fd054ea661a2f6a5b7c4f76', 'application/vnd.oci.image.manifest.v1+json'),
    ('redis', 'docker.io/library/redis@sha256:1f09a89a207d794a8c61d9edfc26e7c58427de10ccef7c5d18d638df79a63b85', 'sha256:ec4f171e9ff071bb2a1c936057d3be00206069b028b6c0d61df2477e1dd02c42', 'application/vnd.oci.image.manifest.v1+json'),
)


def _scheduler_image_format():
    from .coolify_host import SELECTED_IMAGE_FORMAT
    return SELECTED_IMAGE_FORMAT[:-1] + ',"repo_digests":{{json .RepoDigests}}}'


def _scheduler_image_identity(reference, value, *, container_image=()):
    """Exact fixed PUBLIC leaf/config/platform join across the two Docker stores."""
    pins = [row for row in _SCHEDULER_IMAGE_PINS if row[1] == reference]
    _require(len(pins) == 1 and type(value) is dict
             and set(value) == {'engine_id', 'os', 'architecture', 'descriptor', 'repo_digests'}, 'image_identity')
    _, _, config, media = pins[0]
    manifest = reference.split('@')[1]
    _require(value['os'] == 'linux' and value['architecture'] == 'arm64'
             and type(value['engine_id']) is str and value['engine_id'] in (config, manifest), 'image_identity')
    # Docker Hub familiar names are the only normalization; no foreign repo or
    # index digest can stand in for this exact pinned platform leaf.
    repos = value['repo_digests']
    aliases = (reference, reference.removeprefix('docker.io/'),
               reference.removeprefix('docker.io/library/'))
    _require(type(repos) is list and 0 < len(repos) <= 16
             and all(type(row) is str and len(row) <= 256 for row in repos)
             and any(row in aliases for row in repos), 'image_identity')
    descriptor = value['descriptor']
    _require(descriptor is None or type(descriptor) is dict
             and set(descriptor) == {'digest', 'mediaType'}
             and descriptor['digest'] == manifest and descriptor['mediaType'] == media, 'image_identity')
    _require(value['engine_id'] != manifest or descriptor is not None, 'image_identity')
    _require(container_image == () or type(container_image) is str
             and container_image == value['engine_id'], 'image_identity')
    return value['engine_id']


class _SchedulerHostProcessControl(_FixedRoleProcessControl):
    """Private fixed PUBLIC-owner join, never production registration.

    The fixture's exact newly created namespace is the only entry. Selection
    uses daemon state + stock S6 internally, never a caller PID/public result.
    This pins the daemon-created cgroup for scheduler proof; it does not claim
    the separate full CgroupBudget/native-entry allocation protocol.
    """
    _project = 'sifto-startup-scheduler-public'

    def __init__(self):
        raise DockerError('startup_profile_unverified')

    @staticmethod
    def _docker(args, *, timeout=2, limit=65536):
        import subprocess
        try:
            result = subprocess.run(['/usr/bin/docker', *args], stdin=subprocess.DEVNULL,
                stdout=subprocess.PIPE, stderr=subprocess.PIPE, timeout=timeout,
                env={'PATH': '/usr/bin:/bin', 'HOME': '/nonexistent', 'LC_ALL': 'C'}, check=False)
        except (OSError, subprocess.TimeoutExpired):
            raise DockerError('command_failed') from None
        _require(result.returncode == 0 and len(result.stdout) <= limit
                 and len(result.stderr) <= 4096, 'command_failed')
        return result.stdout

    def _inspect_fixture(self):
        fields = {'Id': '.Id', 'Image': '.Image', 'Reference': '.Config.Image',
            'Labels': '.Config.Labels', 'User': '.Config.User', 'Entrypoint': '.Config.Entrypoint',
            'Cmd': '.Config.Cmd', 'Pid': '.State.Pid', 'Running': '.State.Running', 'OOM': '.State.OOMKilled',
            'Mounts': '.Mounts', 'Tmpfs': '.HostConfig.Tmpfs'}
        for name in ('Memory', 'MemorySwap', 'NanoCpus', 'PidsLimit', 'Privileged', 'ReadonlyRootfs',
                     'CapDrop', 'CapAdd', 'SecurityOpt', 'NetworkMode', 'PidMode', 'CgroupnsMode', 'Devices'):
            fields[name] = '.HostConfig.' + name
        template = '{' + ','.join('"' + k + '":{{json ' + v + '}}' for k, v in fields.items()) + '}'
        value = strict_json(self._docker(['container', 'inspect', '--format=' + template, self._cid]))
        _require(type(value) is dict and value['Id'] == self._cid and value['Running'] is True
                 and value['OOM'] is False and type(value['Pid']) is int and value['Pid'] > 0, 'container_identity')
        return value

    @classmethod
    def _for_public_fixture(cls, *, service, run, root, private):
        # root/private come only from the source-owned fixture creator, which
        # requires an initially empty fixed project before any Docker creation.
        _require(platform.system() == 'Linux' and os.geteuid() == 0
                 and service in ('old', 'target') and type(run) is str
                 and re.fullmatch('[a-f0-9]{32}', run) is not None, 'startup_profile_unverified')
        for path in (root, private):
            _require(isinstance(path, Path) and path.is_absolute() and path.resolve() == path
                     and os.lstat(path).st_uid == 0 and stat.S_IMODE(os.lstat(path).st_mode) == 0o700,
                     'startup_state')
        info = strict_json(cls._docker(['info', '--format', '{"driver":{{json .CgroupDriver}},"version":{{json .CgroupVersion}},"os":{{json .OSType}},"arch":{{json .Architecture}}}']))
        _require(info['os'] == 'linux' and info['arch'] in ('aarch64', 'arm64')
                 and info['version'] == '2' and info['driver'] in ('systemd', 'cgroupfs'), 'resource_isolation')
        raw = cls._docker(['container', 'ls', '--no-trunc', '--format={{.ID}}',
            '--filter=label=com.docker.compose.project=' + cls._project,
            '--filter=label=com.docker.compose.service=' + service,
            '--filter=label=sifto.scheduler.run=' + run], limit=128).strip()
        _require(re.fullmatch(rb'[a-f0-9]{64}', raw) is not None, 'ownership_unknown')
        self = object.__new__(cls)
        self._cid = raw.decode('ascii')
        self._role = 'scheduler-worker'
        self._driver = info['driver']
        self._root = root
        self._run = run
        self._service = service
        self._fds = []
        try:
            observed = self._inspect_fixture()
            from .coolify_startup import RESOURCE_SHA256
            _, reference, _, _ = next(row for row in _SCHEDULER_IMAGE_PINS if row[0] == service)
            image = strict_json(cls._docker(['image', 'inspect', '--format=' + _scheduler_image_format(), reference]))
            _scheduler_image_identity(reference, image, container_image=observed['Image'])
            labels = observed['Labels']
            _require(observed['Reference'] == reference
                     and labels['sifto.owner'] == 'startup-scheduler-public-fixture-v1'
                     and labels['sifto.scheduler.run'] == run
                     and labels['com.docker.compose.project'] == cls._project
                     and labels['com.docker.compose.service'] == service
                     and observed['User'] == '9999:9999'
                     and observed['Entrypoint'] == ['docker-php-serversideup-entrypoint']
                     and observed['Cmd'] == ['/init'], 'container_spec')
            _require((observed['Memory'], observed['MemorySwap'], observed['NanoCpus'], observed['PidsLimit'])
                     == (512 * 1024 * 1024, 512 * 1024 * 1024, 1000000000, 256)
                     and observed['Privileged'] is False and observed['ReadonlyRootfs'] is True
                     and observed['CapDrop'] == ['ALL'] and not observed['CapAdd'] and not observed['Devices']
                     and observed['SecurityOpt'] == ['no-new-privileges:true']
                     and observed['PidMode'] == '' and observed['CgroupnsMode'] == 'private'
                     and re.fullmatch('container:[a-f0-9]{64}', observed['NetworkMode']) is not None,
                     'resource_isolation')
            source_root = Path(__file__).resolve().parents[1]
            expected = {'/var/www/html/.env': (root / 'canonical.env', True),
                '/etc/s6-overlay/s6-rc.d/init-script/up': (root / 'init', True),
                '/probe/startup.php': (source_root / 'maintenance/coolify_startup.php', True),
                '/probe/scheduler.request.json': (root / 'request.json', True),
                '/scheduler/fixture.php': (source_root / 'startup-scheduler/fixture.php', True),
                '/scheduler/wait-ready.sh': (source_root / 'startup-runtime/wait-ready.sh', True),
                '/var/www/html/app/Console/Kernel.php': (source_root / 'startup-scheduler/fixture.php', True),
                '/scheduler-state': (root / 'state', False),
                '/var/www/html/app/Http/Middleware/PreventRequestsDuringMaintenance.php': (source_root / 'maintenance/coolify_startup_middleware.php', True),
                '/etc/nginx/nginx.conf': (source_root / 'maintenance/coolify_startup_nginx.conf', True),
                '/etc/nginx/conf.d/default.conf': (source_root / 'maintenance/coolify_startup_site.conf', True),
                '/startup/fixture.php': (source_root / 'startup/fixture.php', True),
                '/data/coolify/ssl/public-fixture.txt': (source_root / 'startup/public-ssl.txt', True),
                '/data/coolify/proxy/public-fixture.txt': (source_root / 'startup/public-proxy.txt', True)}
            mounts = {row['Destination']: row for row in observed['Mounts']}
            _require(len(mounts) == len(observed['Mounts'])
                     and set(mounts) == set(expected) | {'/var/www/html/storage', '/var/www/html/bootstrap/cache'}
                     and observed['Tmpfs'] == {
                         '/run': 'rw,nosuid,nodev,exec,uid=9999,gid=9999,mode=0755,size=33554432',
                         '/tmp': 'rw,nosuid,nodev,noexec,mode=1777,size=16777216',
                         '/probe': 'rw,nosuid,nodev,noexec,uid=9999,gid=9999,mode=0700,size=1048576'}, 'container_spec')
            for destination, (path, readonly) in expected.items():
                mount = mounts[destination]
                _require(mount['Type'] == 'bind' and mount['Source'] == str(path)
                         and mount['RW'] is not readonly, 'container_spec')
            for destination in ('/var/www/html/storage', '/var/www/html/bootstrap/cache'):
                _require(mounts[destination]['Type'] == 'volume' and mounts[destination]['RW'] is True
                         and mounts[destination]['Name'].startswith(cls._project + '_'), 'container_spec')
            self._container = observed
            self._initpid = observed['Pid']
            def opened(path, flags):
                fd = os.open(path, flags | os.O_CLOEXEC)
                self._fds.append(fd)
                return fd
            self._initfd = opened('/proc/' + str(self._initpid), os.O_RDONLY | os.O_DIRECTORY | os.O_NOFOLLOW)
            self._rootfd = os.open('root', os.O_RDONLY | os.O_DIRECTORY | os.O_CLOEXEC, dir_fd=self._initfd)
            self._fds.append(self._rootfd)
            self._init_token = _start_token(self._initpid)
            self._time_namespace = os.readlink('/proc/self/ns/time')
            _require(os.readlink('ns/time', dir_fd=self._initfd) == self._time_namespace, 'startup_state')
            self._cgroup = self._proc_read(self._initfd, 'cgroup', 16384).decode('ascii')
            match = re.fullmatch(r'0::(/[^\n]+)\n', self._cgroup)
            _require(match is not None and '..' not in match[1].split('/'), 'resource_isolation')
            group = match[1]
            _require(group.endswith('/docker-' + self._cid + '.scope') if self._driver == 'systemd'
                     else group.endswith('/' + self._cid), 'resource_isolation')
            self._group_path = Path('/sys/fs/cgroup' + group)
            self._groupfd = opened(self._group_path, os.O_RDONLY | os.O_DIRECTORY | os.O_NOFOLLOW)
            self._group_identity = _metadata(os.fstat(self._groupfd), directory=True)
            _require(self._proc_read(self._groupfd, 'cgroup.type', 64) == b'domain\n', 'resource_isolation')
            _require(self._proc_read(self._groupfd, 'memory.max', 64) == b'536870912\n'
                     and self._proc_read(self._groupfd, 'memory.swap.max', 64) == b'0\n'
                     and self._proc_read(self._groupfd, 'pids.max', 64) == b'256\n', 'resource_isolation')
            quota = self._proc_read(self._groupfd, 'cpu.max', 64).split()
            _require(len(quota) == 2 and quota[0].isdigit() and quota[1].isdigit()
                     and int(quota[0]) == int(quota[1]) > 0, 'resource_isolation')
            self._membership = self._group_path / 'cgroup.procs'
            self._sourcefd = os.open('etc/s6-overlay/s6-rc.d/scheduler-worker/run', os.O_RDONLY | os.O_NOFOLLOW, dir_fd=self._rootfd)
            self._fds.append(self._sourcefd)
            _require(_hash(_read(self._sourcefd, 4096)) == '9034571c9f328e694720f9c66c461016e96a3d932ade29f5d2498754677d906c', 'source_closure')
            helper = os.open('probe/startup.php', os.O_RDONLY | os.O_NOFOLLOW, dir_fd=self._rootfd)
            try:
                _require(_hash(_read(helper, 131072)) == dict(RESOURCE_SHA256)['coolify_startup.php'], 'source_closure')
            finally:
                os.close(helper)
            observed_context = self._context('scheduler-context', timeout=15)
            self._context_observation = observed_context
            self._contextfd = os.open('probe/scheduler.context.json', os.O_RDONLY | os.O_NOFOLLOW, dir_fd=self._rootfd)
            self._fds.append(self._contextfd)
            projection = strict_json(_read(self._contextfd, 131072))
            _require(projection['hmac'] == observed_context['context_hmac'], 'startup_state')
            self._pin_request(projection['data']['request'])
            status = projection['data']['s6']
            _require(type(status) is str, 'startup_state')
            match = re.fullmatch(r'true true ([1-9][0-9]{0,9})\n', status)
            _require(match is not None, 'startup_state')
            selected = []
            for pid in self._members():
                fd = opened('/proc/' + str(pid), os.O_RDONLY | os.O_DIRECTORY | os.O_NOFOLLOW)
                row = self._proc_identity(fd)
                if row['ids']['NSpid'] == [str(pid), match[1]]:
                    selected.append(pid)
            _require(len(selected) == 1, 'startup_state')
            self._pid = selected[0]
            self._pidfd = opened_pidfd = os.pidfd_open(self._pid)
            self._fds.append(opened_pidfd)
            self._procfd = opened('/proc/' + str(self._pid), os.O_RDONLY | os.O_DIRECTORY | os.O_NOFOLLOW)
            self._exefd = opened('/proc/' + str(self._pid) + '/exe', os.O_RDONLY)
            self._dirfd = opened(private, os.O_RDONLY | os.O_DIRECTORY | os.O_NOFOLLOW)
            self._lockfd = os.open('scheduler-worker.lock', os.O_RDWR | os.O_CREAT | os.O_EXCL | os.O_NOFOLLOW, 0o600, dir_fd=self._dirfd)
            self._fds.append(self._lockfd)
            self._deadline = time.monotonic() + 20
            self._key = secrets.token_bytes(32)
            self._original = None
            self._intent = None
            self._pins = self._read_pins()
            self._selected = self._identity()
            self._scheduler_stable = self._stable_members()
            self._guard(self._deadline)
            return self
        except BaseException:
            self.close()
            raise

    def close(self):
        for fd in reversed(self._fds):
            os.close(fd)
        self._fds.clear()

    def _context(self, operation, *, timeout=.35):
        _require(operation in ('scheduler-context', 'scheduler-context-check'), 'startup_state')
        value = strict_json(self._docker(['exec', '--user=9999:9999', self._cid,
            'php', '/probe/startup.php', operation], timeout=timeout, limit=8192))
        _require(type(value) is dict and set(value) == {'version', 'operation', 'selected_version', 'generation', 'context_hmac'}
                 and type(value['version']) is int and value['version'] == 1
                 and value['operation'] == 'scheduler-context'
                 and value['selected_version'] == ('4.4.2' if self._service == 'old' else '4.4.6')
                 and type(value['generation']) is str and re.fullmatch('[a-f0-9]{32}', value['generation'])
                 and type(value['context_hmac']) is str and _HASH.fullmatch(value['context_hmac']), 'startup_state')
        return value

    def _read_pins(self):
        pins = super()._read_pins()
        _require(self._context('scheduler-context-check') == self._context_observation, 'startup_state')
        return pins

    @staticmethod
    def _request_snapshot(fd):
        info = os.fstat(fd)
        metadata = list(_metadata(info))
        _require(stat.S_ISREG(info.st_mode) and info.st_uid == info.st_gid == 0
                 and stat.S_IMODE(info.st_mode) == 0o444 and info.st_nlink == 1, 'startup_state')
        raw = _read(fd, 2048)
        _require(metadata == list(_metadata(os.fstat(fd))), 'startup_state')
        return [metadata, _hash(raw)]

    def _pin_request(self, expected):
        _require(type(expected) is list and len(expected) == 3 and type(expected[0]) is dict
                 and type(expected[1]) is str and _HASH.fullmatch(expected[1])
                 and type(expected[2]) is dict, 'startup_state')
        self._request_source_path = self._root / 'request.json'
        self._request_source_fd = os.open(self._request_source_path, os.O_RDONLY | os.O_NOFOLLOW | os.O_CLOEXEC)
        self._fds.append(self._request_source_fd)
        self._request_mount_fd = os.open('probe/scheduler.request.json',
            os.O_RDONLY | os.O_NOFOLLOW | os.O_CLOEXEC, dir_fd=self._rootfd)
        self._fds.append(self._request_mount_fd)
        self._request_pins = [self._request_snapshot(fd)
            for fd in (self._request_source_fd, self._request_mount_fd)]
        info = os.fstat(self._request_mount_fd)
        php_identity = dict(dev=info.st_dev, ino=info.st_ino, mode=info.st_mode, nlink=info.st_nlink,
            uid=info.st_uid, gid=info.st_gid, size=info.st_size, mtime=int(info.st_mtime), ctime=int(info.st_ctime))
        self._request_data = expected[2]
        _require(self._request_pins[0] == self._request_pins[1]
                 and self._request_pins[0][1] == expected[1] and php_identity == expected[0]
                 and strict_json(_read(self._request_mount_fd, 2048)) == self._request_data
                 and set(self._request_data) == {'version', 'phase', 'generation', 'deadline_ns'}
                 and type(self._request_data['version']) is int and self._request_data['version'] == 1
                 and self._request_data['phase'] == 'scheduler-proof-held'
                 and self._request_data['generation'] == self._context_observation['generation']
                 and type(self._request_data['deadline_ns']) is int, 'startup_state')
        self._request_guard()

    def _request_guard(self):
        # A Docker bind may keep its old inode after the source pathname is
        # replaced. Both source and mounted paths must still name the original
        # root-owned phase file; a matching mounted projection alone is stale.
        for index, fd in enumerate((self._request_source_fd, self._request_mount_fd)):
            _require(self._request_snapshot(fd) == self._request_pins[index], 'startup_state')
        _require(list(_metadata(os.stat(self._request_source_path, follow_symlinks=False))) == self._request_pins[0][0]
                 and list(_metadata(os.stat('probe/scheduler.request.json', dir_fd=self._rootfd,
                     follow_symlinks=False))) == self._request_pins[1][0], 'startup_state')
        _require(time.monotonic_ns() < self._request_data['deadline_ns'], 'command_timeout')

    def _final_guard(self, deadline):
        # Revoke late generation/source changes AFTER identity/Docker/cgroup I/O.
        # The inherited post-I/O original deadline and pidfd check remain last;
        # this does not renew either the drain or separate restoration budget.
        self._request_guard()
        super()._final_guard(deadline)

    def _proc_identity(self, fd):
        return super()._proc_identity(fd, native_titles=True)

    def _executable_limit(self):
        # The authenticated fixed PHP executable is larger than Python K0's
        # component ceiling; this remains a bounded exact-image file identity.
        return 64 * 1024 * 1024

    def _executable_hash(self, limit):
        metadata = _metadata(os.fstat(self._exefd))
        if not hasattr(self, '_fixed_php_hash'):
            self._fixed_php_hash = (metadata, super()._executable_hash(limit))
        # The daemon-authenticated image/rootfs is RO, the executable is the
        # root-owned fixed-image PHP inode, and _placement rebinds that inode on
        # every reread. Rehashing the same 19MiB inode at every 1ms poll is not
        # additional application-write protection and can exhaust restoration.
        _require(metadata == self._fixed_php_hash[0] and metadata[2] == 0, 'startup_state')
        return self._fixed_php_hash[1]

    def _members(self):
        _require(_metadata(os.stat(self._group_path, follow_symlinks=False), directory=True)
                 == self._group_identity == _metadata(os.fstat(self._groupfd), directory=True), 'resource_isolation')
        values = [int(part) for part in self._proc_read(self._groupfd, 'cgroup.procs', 8192).split()]
        _require(0 < len(values) <= 256 and len(values) == len(set(values)), 'resource_isolation')
        return sorted(values)

    def _placement(self, selected, parent, init):
        _require(self._inspect_fixture() == self._container and _start_token(self._initpid) == self._init_token
                 and os.readlink('ns/time', dir_fd=self._initfd) == self._time_namespace
                 and os.readlink('ns/time', dir_fd=self._procfd) == self._time_namespace
                 and init['ids']['NSpid'] == [str(self._initpid), '1']
                 and selected['cgroup'] == parent['cgroup'] == init['cgroup'] == self._cgroup
                 and selected['namespaces'] == parent['namespaces'] == init['namespaces']
                 and selected['namespaces'][0] != os.readlink('/proc/self/ns/pid')
                 and all(row['ids']['Uid'] == row['ids']['Gid'] == ['9999'] * 4 for row in (selected, parent, init))
                 and selected['argv'] == [part.hex() for part in (b'php', b'artisan', b'schedule:work')]
                 and parent['argv'] == [part.hex() for part in (b's6-supervise', b'scheduler-worker')]
                 and parent['parent'] == self._initpid
                 and selected['exe'] == list(_metadata(os.stat('usr/local/bin/php', dir_fd=self._rootfd)))
                 and parent['exe'] == list(_metadata(os.stat('package/admin/s6-2.13.2.0/command/s6-supervise', dir_fd=self._rootfd)))
                 and init['exe'] == list(_metadata(os.stat('package/admin/s6-2.13.2.0/command/s6-svscan', dir_fd=self._rootfd))), 'startup_state')
        _require(self._members() == sorted(int(part) for part in self._membership.read_text().split()), 'resource_isolation')
        return 'owned-public-native-scheduler', self._membership

    def _stable_members(self):
        rows = {}
        for pid in self._members():
            fd = os.open('/proc/' + str(pid), os.O_RDONLY | os.O_DIRECTORY | os.O_NOFOLLOW)
            try:
                rows[pid] = self._proc_identity(fd)
            finally:
                os.close(fd)
        roles = ('php-fpm', 'nginx', 'reverb', 'terminal-server', 'horizon', 'scheduler-worker',
                 's6rc-fdholder', 's6rc-oneshot-runner', 's6-linux-init-shutdownd')
        stable = {}
        supervisor = {}
        for pid, row in rows.items():
            argv = [bytes.fromhex(part) for part in row['argv']]
            if pid == self._initpid and argv == [b'/package/admin/s6/command/s6-svscan', b'-d4', b'--', b'/run/service']:
                stable[pid] = row
            elif row['parent'] == self._initpid and len(argv) == 2 and argv[0] == b's6-supervise' and argv[1].decode('ascii') in roles:
                if row['exe'] == list(_metadata(os.stat('package/admin/s6-2.13.2.0/command/s6-supervise', dir_fd=self._rootfd))):
                    stable[pid] = row
                    supervisor[pid] = argv[1].decode('ascii')
        exact = {
            'php-fpm': ('usr/local/sbin/php-fpm', [b'php-fpm: master process (/usr/local/etc/php-fpm.conf)']),
            'nginx': ('usr/sbin/nginx', [b'nginx: master process nginx']),
            'reverb': ('usr/local/bin/php', [b'php', b'artisan', b'reverb:start', b'--host=0.0.0.0', b'--port=6001']),
            'terminal-server': ('usr/bin/node', [b'node', b'/terminal/terminal-server.js']),
            'horizon': ('bin/busybox', [b'sleep', b'infinity']),
            's6-linux-init-shutdownd': ('package/admin/s6-linux-init-1.1.3.0/command/s6-linux-init-shutdownd',
                [b'/package/admin/s6-linux-init/command/s6-linux-init-shutdownd', b'-d3', b'-c', b'/run/s6/basedir', b'-g', b'3000', b'-C', b'-B']),
            's6rc-oneshot-runner': ('package/admin/s6-2.13.2.0/command/s6-ipcserverd',
                [b'/package/admin/s6/command/s6-ipcserverd', b'-1', b'--', b'/package/admin/s6/command/s6-ipcserver-access',
                 b'-v0', b'-E', b'-l0', b'-i', b'data/rules', b'--', b'/package/admin/s6/command/s6-sudod', b'-t',
                 b'30000', b'--', b'/package/admin/s6-rc/command/s6-rc-oneshot-run', b'-l', b'../..', b'--'])}
        masters = {}
        for pid, row in rows.items():
            role = supervisor.get(row['parent'])
            if role in exact:
                executable, argv = exact[role]
                if row['argv'] == [part.hex() for part in argv] and row['exe'] == list(_metadata(os.stat(executable, dir_fd=self._rootfd))):
                    stable[pid] = row
                    masters[pid] = role
        for pid, row in rows.items():
            role = masters.get(row['parent'])
            if role in ('php-fpm', 'nginx'):
                executable = exact[role][0]
                argv = b'php-fpm: pool www' if role == 'php-fpm' else b'nginx: worker process'
                if row['argv'] == [argv.hex()] and row['exe'] == list(_metadata(os.stat(executable, dir_fd=self._rootfd))):
                    stable[pid] = row
        for pid, row in stable.items():
            _require(row['cgroup'] == self._cgroup and row['namespaces'] == self._selected['process']['namespaces']
                     and row['ids']['Uid'] == row['ids']['Gid'] == ['9999'] * 4
                     and len(row['ids']['NSpid']) == 2 and row['ids']['NSpid'][0] == str(pid), 'startup_state')
        return stable


def _supervise(argv, *, output_fd, prefix, pass_fds=(), stdin_fd=None,
               deadline, limit, attach=None):
    """Private transport primitive, exercised with synthetic exec children only.

    argv never reaches a public adapter method; NativeProcess fixes the binary
    and operation. The native session supervisor is Python fork code, not shell.
    """
    import ctypes
    import errno
    import select
    import selectors
    import signal
    descriptors, files, pipes = [], [], []
    pid = pidfd = None
    reaped = False
    failure = None
    pending_error = None
    released = False
    process_clean = False
    def fail(category):
        nonlocal failure
        if failure is None:
            failure = DockerError(category)
    streams = [bytearray(), bytearray()]
    exit_bytes = bytearray()
    try:
        _require(type(argv) is list and all(type(v) is str and '\x00' not in v for v in argv), 'container_spec')
        _require(type(prefix) is str and re.fullmatch(r'command-[0-9]{1,8}', prefix), 'stage_identity')
        _require(type(limit) is int and 0 < limit <= 1024 * 1024, 'output_bound')
        _require(type(pass_fds) is tuple and len(pass_fds) <= 1
                 and all(type(fd) is int and fd > 2 for fd in pass_fds), 'container_spec')
        for fd in pass_fds:
            os.fstat(fd)
        if stdin_fd is not None:
            os.fstat(stdin_fd)
        for suffix in ('stdout', 'stderr'):
            fd = os.open(prefix + '.' + suffix, os.O_WRONLY | os.O_CREAT | os.O_EXCL | os.O_NOFOLLOW | os.O_CLOEXEC,
                         0o600, dir_fd=output_fd)
            descriptors.append(fd)
            info = os.fstat(fd)
            _require(stat.S_ISREG(info.st_mode) and info.st_uid == 0 and info.st_nlink == 1
                     and stat.S_IMODE(info.st_mode) == 0o600, 'stage_identity')
            files.append(fd)
        for _ in range(4):
            pair = os.pipe2(os.O_CLOEXEC)
            pipes.append(pair)
            descriptors.extend(pair)
        out, err, status, ready = pipes
        pid = os.fork()
        if pid == 0:
            try:
                os.setsid()
                # Adopt orphaned grandchildren, so the live session leader owns
                # the group until every inherited output writer has exited.
                if ctypes.CDLL(None, use_errno=True).prctl(36, 1, 0, 0, 0) != 0:
                    os._exit(125)
                signal.signal(signal.SIGTERM, signal.SIG_IGN)
                force = [False]
                signal.signal(signal.SIGUSR1, lambda *_: force.__setitem__(0, True))
                for fd in (out[0], err[0], status[0], ready[1], *files):
                    os.close(fd)
                # Parent places and verifies this supervisor before the CLI fork.
                if os.read(ready[0], 1) != b'1':
                    os._exit(125)
                os.close(ready[0])
                child = os.fork()
                if child == 0:
                    signal.signal(signal.SIGTERM, signal.SIG_DFL)
                    signal.signal(signal.SIGUSR1, signal.SIG_DFL)
                    os.dup2(out[1], 1)
                    os.dup2(err[1], 2)
                    zero = os.open('/dev/null', os.O_RDONLY) if stdin_fd is None else stdin_fd
                    os.dup2(zero, 0)
                    retained = {0, 1, 2, *pass_fds}
                    for fd in pass_fds:
                        os.set_inheritable(fd, True)
                    for name in os.listdir('/proc/self/fd'):
                        fd = int(name)
                        if fd not in retained:
                            try:
                                os.close(fd)
                            except OSError:
                                pass
                    os.execve(argv[0], argv, dict(_ENV))
                os.close(out[1]); os.close(err[1])
                # No private FD is inherited by the supervisor unnecessarily.
                for name in os.listdir('/proc/self/fd'):
                    fd = int(name)
                    if fd > 2 and fd != status[1]:
                        try:
                            os.close(fd)
                        except OSError:
                            pass
                code = 125
                while True:
                    if force[0]:
                        # These are unreaped children: their PIDs cannot be reused.
                        owned = Path('/proc/self/task/' + str(os.getpid()) + '/children').read_text().split()
                        for selected in owned:
                            try:
                                os.kill(int(selected), signal.SIGKILL)
                            except ProcessLookupError:
                                pass
                    try:
                        selected, state = os.waitpid(-1, os.WNOHANG)
                    except ChildProcessError:
                        break
                    if selected == child:
                        code = os.waitstatus_to_exitcode(state)
                    if selected == 0:
                        select.select([], [], [], 0.01)
                os.write(status[1], str(max(0, min(255, code if code >= 0 else 128 - code))).encode())
                os._exit(0)
            except BaseException:
                os._exit(125)
        pidfd = os.pidfd_open(pid)
        descriptors.append(pidfd)
        token = _start_token(pid)
        for fd in (out[1], err[1], status[1], ready[0]):
            os.close(fd); descriptors.remove(fd)
        while os.getsid(pid) != pid:
            _require(time.monotonic() < deadline, 'command_timeout')
            select.select([], [], [], 0.001)
        if attach is not None:
            attach(pid, token)
        released = True
        os.write(ready[1], b'1')
        os.close(ready[1]); descriptors.remove(ready[1])
        selector = selectors.DefaultSelector()
        try:
            for index, fd in enumerate((out[0], err[0], status[0])):
                os.set_blocking(fd, False)
                selector.register(fd, selectors.EVENT_READ, index)
            def live():
                # A ready pidfd means dead; never signal a stale/reused process group.
                return not select.select([pidfd], [], [], 0)[0] and _start_token(pid) == token
            def signal_owned(sig, group=False):
                if live():
                    if group:
                        _require(os.getpgid(pid) == pid and os.getsid(pid) == pid, 'ownership_unknown')
                        os.killpg(pid, sig)
                    else:
                        signal.pidfd_send_signal(pidfd, sig)
            terminate_at = kill_at = None
            while selector.get_map() or live():
                now = time.monotonic()
                if failure is None and now >= deadline:
                    fail('command_timeout')
                if failure is not None and terminate_at is None:
                    signal_owned(signal.SIGTERM, group=True)
                    terminate_at = now
                if terminate_at is not None and now - terminate_at >= 0.25 and kill_at is None:
                    signal_owned(signal.SIGUSR1)
                    kill_at = now
                if kill_at is not None and now - kill_at > 2:
                    fail('cleanup_incomplete')
                    signal_owned(signal.SIGKILL)
                    break
                try:
                    events = selector.select(0.02)
                except (KeyboardInterrupt, SystemExit):
                    fail('interrupted')
                    continue
                for key, _ in events:
                    value = os.read(key.fd, 65536)
                    if not value:
                        selector.unregister(key.fd)
                        continue
                    if key.data == 2:
                        exit_bytes.extend(value)
                        if len(exit_bytes) > 3:
                            fail('command_failed')
                    else:
                        index = key.data
                        if len(streams[index]) + len(value) > limit:
                            fail('output_bound')
                        elif failure is None:
                            streams[index].extend(value)
                            OwnedDocker._write_all(files[index], value)
            _, supervisor_state = os.waitpid(pid, 0)
            reaped = True
            process_clean = os.waitstatus_to_exitcode(supervisor_state) == 0
            for fd in files:
                os.fsync(fd)
            if failure is not None:
                raise failure
            _require(process_clean, 'cleanup_incomplete')
            _require(re.fullmatch(rb'[0-9]{1,3}', exit_bytes) is not None, 'command_failed')
            return int(exit_bytes), bytes(streams[0]), bytes(streams[1])
        finally:
            selector.close()
    except DockerError as error:
        pending_error = failure if failure is not None else error
        raise pending_error from None
    except (KeyboardInterrupt, SystemExit):
        pending_error = failure if failure is not None else DockerError('interrupted')
        raise pending_error from None
    except BaseException:
        pending_error = failure if failure is not None else DockerError('command_failed')
        raise pending_error from None
    finally:
        if pid is not None and pid > 0 and not reaped:
            # Setup failure: handshake prevents exec. Keep exact supervisor identity.
            try:
                if pidfd is not None:
                    signal.pidfd_send_signal(pidfd, signal.SIGUSR1)
                for pair in pipes:
                    if pair[1] in descriptors:
                        os.close(pair[1]); descriptors.remove(pair[1])
                end = time.monotonic() + 2
                while True:
                    observed, supervisor_state = os.waitpid(pid, os.WNOHANG)
                    if observed == pid:
                        reaped = True
                        process_clean = not released or os.waitstatus_to_exitcode(supervisor_state) == 0
                        break
                    if time.monotonic() >= end:
                        if pidfd is not None:
                            signal.pidfd_send_signal(pidfd, signal.SIGKILL)
                        os.waitpid(pid, 0)
                        reaped = True
                        process_clean = False
                        break
                    select.select([], [], [], 0.01)
            except (OSError, ChildProcessError):
                pass
        for fd in reversed(descriptors):
            try:
                os.close(fd)
            except OSError:
                pass
        if pending_error is not None:
            pending_error.cleanup_complete = pid is None or process_clean
            if pending_error.cleanup_complete:
                pending_error._process_cleanup = _PROCESS_CLEANUP


class CgroupBudget:
    """Closed cgroupfs-v2 envelope; native processes never migrate.

    A trusted launcher must start fresh Python inside native-entry BEFORE Python
    initialization. Current membership cannot prove page-charge origin; N3 must
    provide a real-kernel witness. Parent quota covers entry and container pages.
    """
    ROOT = Path('/sys/fs/cgroup')
    PARENT = {'memory.max': '1342177280', 'memory.swap.max': '0',
              'cpu.max': '175000 100000', 'pids.max': '576'}
    PREPARATION = {**PARENT, 'pids.max': '64'}
    STEADY = {'memory.max': '268435456', 'memory.swap.max': '0',
              'cpu.max': '12500 100000', 'pids.max': '64'}
    CONTAINERS = {'memory.max': '1073741824', 'memory.swap.max': '0',
                  'cpu.max': '162500 100000', 'pids.max': '512'}

    def __init__(self, parent, token):
        _require(isinstance(parent, Path) and parent.is_relative_to(self.ROOT) and parent != self.ROOT
                 and re.fullmatch(r'[a-f0-9]{32}', token), 'resource_isolation')
        self.parent = parent
        self.path = parent / ('sifto-' + token)
        self.native = self._entry = parent / 'native-entry'
        self.docker_parent = '/' + str(self.path.relative_to(self.ROOT))
        self.allocated = self.preflighted = False
        self._mode = 'preparation'
        self._entry_limits = dict(self.PREPARATION)
        self._limits_trusted = True
        self._pins, self._identities, self._container_pins = {}, {}, {}
        self._expected_ids = frozenset()
        self._sessions, self._cpu_samples = {}, {}
        self._created = []
        self._engine_pid = os.getpid()
        self._engine_token = _start_token(self._engine_pid)

    def _mkdir(self, path):
        path.mkdir(mode=0o700)

    def _directories(self, path):
        return [p for p in path.iterdir() if p.is_dir()]

    def _rmdir(self, path):
        path.rmdir()

    def _read(self, path):
        return path.read_text().strip()

    def _write(self, path, value):
        # Parent policy and membership are never mutated by this implementation.
        _require(path.parent in (self.native, self.path)
                 and path.name in self.PARENT, 'resource_isolation')
        with path.open('w') as stream:
            stream.write(value)

    def _identity(self, path):
        info = path.lstat()
        _require(stat.S_ISDIR(info.st_mode) and info.st_uid == 0 and not info.st_mode & 0o022,
                 'resource_isolation')
        return _metadata(info, True)

    def _open_directory(self, path):
        fd, _ = _protected_open(path, os.O_RDONLY | os.O_DIRECTORY, category='resource_isolation')
        return fd

    def _held_identity(self, fd):
        return _metadata(os.fstat(fd), True)

    def _close_directory(self, fd):
        os.close(fd)

    def _pin(self, path):
        identity = self._identity(path)
        fd = self._open_directory(path)
        self._pins[path], self._identities[path] = fd, identity
        _require(self._held_identity(fd) == identity, 'resource_isolation')

    def _guard_pins(self):
        _require(all(self._identity(path) == identity
                 and self._held_identity(self._pins[path]) == identity
                 for path, identity in self._identities.items()), 'resource_isolation')

    def _limits(self, path):
        return {key: self._read(path / key) for key in self.PARENT}

    def _observe(self, path):
        try:
            events = dict(line.split() for line in self._read(path / 'memory.events').splitlines())
            _require(events.get('oom') == events.get('oom_kill') == '0', 'resource_isolation')
            values = dict(line.split() for line in self._read(path / 'cpu.stat').splitlines())
            sample = {key: int(values[key]) for key in ('nr_periods', 'nr_throttled', 'throttled_usec')}
            _require(all(value >= 0 for value in sample.values()), 'resource_isolation')
            # Successful CPU throttling is normal; failed children stay failures.
            self._cpu_samples[path] = sample
        except (ValueError, KeyError):
            raise DockerError('resource_isolation') from None

    def _placement(self, pid):
        raw = Path('/proc/' + str(pid) + '/cgroup').read_text().strip()
        _require(raw.startswith('0::/') and '\n' not in raw, 'resource_isolation')
        return self.ROOT / raw[4:]

    def _process(self, pid):
        value = Path('/proc/' + str(pid) + '/stat').read_bytes()
        fields = value[value.rfind(b')') + 2:].split()
        return {'token': fields[19], 'ppid': int(fields[1]),
                'pgid': int(fields[2]), 'sid': int(fields[3])}

    def _pids(self):
        raw = self._read(self.native / 'cgroup.procs').split()
        _require(len(raw) <= 64 and all(re.fullmatch(r'[1-9][0-9]{0,9}', pid) for pid in raw),
                 'resource_isolation')
        pids = set(map(int, raw))
        _require(len(pids) == len(raw) and self._engine_pid in pids, 'resource_isolation')
        return pids

    def _native_members(self, candidate=None):
        pids = self._pids()
        main = self._process(self._engine_pid)
        _require(main['token'] == self._engine_token
                 and self._placement(self._engine_pid) == self.native, 'resource_isolation')
        for pid in pids - {self._engine_pid}:
            try:
                info = self._process(pid)
            except (ProcessLookupError, FileNotFoundError):
                _require(pid not in self._pids(), 'resource_isolation')
                continue
            if candidate is not None and pid == candidate[0]:
                _require(info['token'] == candidate[1] and info['ppid'] == self._engine_pid
                         and info['sid'] == info['pgid'] == pid, 'resource_isolation')
            else:
                leader = info['sid']
                _require(info['pgid'] == leader and leader in self._sessions, 'resource_isolation')
                try:
                    owned = self._process(leader)
                except (ProcessLookupError, FileNotFoundError):
                    raise DockerError('resource_isolation') from None
                _require(owned['token'] == self._sessions[leader], 'resource_isolation')
            _require(self._placement(pid) == self.native
                     and self._process(pid)['token'] == info['token'], 'resource_isolation')
        for leader, token in list(self._sessions.items()):
            try:
                _require(self._process(leader)['token'] == token, 'resource_isolation')
            except (ProcessLookupError, FileNotFoundError):
                _require(not any(self._process(pid)['sid'] == leader for pid in pids - {self._engine_pid}),
                         'resource_isolation')
                del self._sessions[leader]
        return pids

    def preflight_entry(self):
        _require(not self.allocated, 'resource_isolation')
        if self.preflighted:
            self.guard()
            return
        _require({'memory', 'cpu', 'pids'} <= set(self._read(self.parent / 'cgroup.controllers').split())
                 and {'memory', 'cpu', 'pids'} <= set(self._read(self.parent / 'cgroup.subtree_control').split())
                 and self._read(self.parent / 'cgroup.type') == 'domain', 'resource_isolation')
        path = self.ROOT
        self._pin(path)
        for name in self.parent.relative_to(self.ROOT).parts:
            path /= name
            self._pin(path)
        self._pin(self.native)
        _require(self._limits(self.parent) == self.PARENT
                 and self._limits(self.native) == self.PREPARATION
                 and self._read(self.parent / 'cgroup.procs') == ''
                 and set(self._directories(self.parent)) == {self.native}
                 and not self._directories(self.native)
                 and self._pids() == {self._engine_pid}, 'resource_isolation')
        self._native_members()
        self._observe(self.parent)
        self._observe(self.native)
        self.preflighted = True
        self.guard()

    def _expect_containers(self, ids):
        _require(type(ids) is frozenset and all(type(cid) is str and _HASH.fullmatch(cid) for cid in ids),
                 'resource_isolation')
        self._expected_ids = ids

    def guard(self, candidate=None):
        _require(self.preflighted and self._limits_trusted, 'resource_isolation')
        self._guard_pins()
        _require(self._limits(self.parent) == self.PARENT
                 and self._limits(self.native) == self._entry_limits
                 and self._read(self.parent / 'cgroup.procs') == ''
                 and not self._directories(self.native)
                 and set(self._directories(self.parent)) ==
                     ({self.native, self.path} if self.allocated or self._created else {self.native}),
                 'resource_isolation')
        self._native_members(candidate)
        self._observe(self.parent)
        self._observe(self.native)
        if self.allocated:
            _require(self._limits(self.path) == self.CONTAINERS
                     and self._read(self.path / 'cgroup.procs') == '', 'resource_isolation')
            children = self._directories(self.path)
            _require(all(path.name in self._expected_ids and path.parent == self.path for path in children),
                     'resource_isolation')
            self._observe(self.path)
            for path, (fd, identity) in self._container_pins.items():
                _require(self._held_identity(fd) == identity, 'resource_isolation')
                try:
                    if path in children:
                        _require(self._identity(path) == identity, 'resource_isolation')
                except FileNotFoundError:
                    pass  # A removed daemon cgroup is never adopted if recreated.

    def allocate(self, info):
        _require(info == {'driver': 'cgroupfs', 'version': '2'} and not self.allocated,
                 'resource_isolation')
        if not self.preflighted:
            self.preflight_entry()
        self.guard()
        try:
            self._mkdir(self.path)
            self._created.append(self.path)
            self._pin(self.path)
            for key, value in self.CONTAINERS.items():
                self._write(self.path / key, value)
                _require(self._read(self.path / key) == value, 'resource_isolation')
            self.allocated = True
            self.guard()
        except BaseException as error:
            first = self._first_failure(error)
            try:
                self._remove_empty_run()
                self.guard()
            except BaseException:
                pass
            else:
                first.cleanup_complete = True
            raise first from None

    @staticmethod
    def _first_failure(error):
        if isinstance(error, DockerError):
            return DockerError(str(error))
        return DockerError('interrupted' if isinstance(error, (KeyboardInterrupt, SystemExit))
                           else 'resource_isolation')

    def attach(self, pid, token):
        # _supervise supplies an exact forked session leader before the CLI fork.
        # Verify inherited placement; NEVER write cgroup.procs or move any PID.
        self.guard(candidate=(pid, token))
        info = self._process(pid)
        _require(info['token'] == token and info['ppid'] == self._engine_pid
                 and info['sid'] == info['pgid'] == pid and self._placement(pid) == self.native,
                 'resource_isolation')
        self._sessions[pid] = token

    def _transition(self, target, mode):
        self.guard()
        if target == self.STEADY:
            current = self._read(self.native / 'memory.current')
            _require(current.isdigit() and int(current) < 256 * 1024 * 1024, 'resource_capacity')
        for key, value in target.items():
            if self._entry_limits[key] == value:
                continue
            previous = self._entry_limits[key]
            try:
                self._write(self.native / key, value)
            except BaseException as error:
                first = self._first_failure(error)
                try:
                    observed = self._read(self.native / key)
                    if observed == value:
                        self._entry_limits[key] = value
                    elif observed != previous:
                        self._limits_trusted = False
                except BaseException:
                    self._limits_trusted = False
                raise first from None
            if self._read(self.native / key) != value:
                self._limits_trusted = False
                raise DockerError('resource_isolation')
            self._entry_limits[key] = value
        self._mode = mode
        self.guard()

    def preparation(self):
        self.guard()
        _require(self.allocated and not self._expected_ids and not self._directories(self.path)
                 and self._read(self.path / 'cgroup.procs') == ''
                 and self._native_members() == {self._engine_pid}, 'resource_isolation')
        self._transition(self.PREPARATION, 'preparation')

    def steady(self):
        self._transition(self.STEADY, 'steady')

    def verify_container(self, pid, value):
        self.guard()
        cid = value.get('Id')
        _require(cid in self._expected_ids and self._mode == 'steady', 'resource_isolation')
        token = self._process(pid)['token']
        path = self._placement(pid)
        _require(path == self.path / cid, 'resource_isolation')
        identity = self._identity(path)
        if path not in self._container_pins:
            fd = self._open_directory(path)
            self._container_pins[path] = (fd, identity)
        fd, expected = self._container_pins[path]
        _require(identity == self._held_identity(fd) == expected
                 and self._limits(path) == {'memory.max': str(value['Memory']), 'memory.swap.max': '0',
                     'cpu.max': str(value['NanoCpus'] // 10000) + ' 100000', 'pids.max': str(value['PidsLimit'])}
                 and self._placement(pid) == path and self._process(pid)['token'] == token,
                 'resource_isolation')
        self._observe(path)

    def _remove_empty_run(self):
        self._guard_pins()
        _require(not self._expected_ids and self._native_members() == {self._engine_pid},
                 'cleanup_incomplete')
        for path in reversed(self._created):
            _require(not self._directories(path) and self._read(path / 'cgroup.procs') == '',
                     'cleanup_incomplete')
            self._rmdir(path)
            self._close_directory(self._pins.pop(path))
            del self._identities[path]
        self._created = []
        self.allocated = False

    def release(self):
        if not self.preflighted:
            _require(not self._created, 'cleanup_incomplete')
            self._close_pins()
            return
        self.guard()
        _require(not self._expected_ids and self._native_members() == {self._engine_pid},
                 'cleanup_incomplete')
        self._remove_empty_run()
        # All containers and CLI helpers are gone before restoring the entry cap.
        self.guard()
        self._transition(self.PREPARATION, 'preparation')

    def _close_pins(self):
        for path, fd in list(self._pins.items()):
            try:
                if self._held_identity(fd) == self._identities[path]:
                    self._close_directory(fd)
            except OSError:
                pass
        for fd, identity in self._container_pins.values():
            try:
                if self._held_identity(fd) == identity:
                    self._close_directory(fd)
            except OSError:
                pass
        self._pins, self._identities, self._container_pins = {}, {}, {}
