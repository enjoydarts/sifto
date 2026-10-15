"""Dormant controller-owned Docker create transactions and cleanup capabilities.

The native source-owner adapter, CREATE wire and global reserve/launch gate are
N2S-4 work. Both native factories reject before I/O. The private port below is
exercised by public synthetic owners only; it does not establish installation,
real Docker asynchronous behavior or the ARM controller memory envelope.

Protected filesystem ownership is the trust anchor. Ledger hashes detect
inconsistency; they do not authenticate against a hostile host root.
"""
from dataclasses import dataclass
import fcntl
import hashlib
import json
import math
import os
from pathlib import Path, PosixPath
import re
import secrets
import stat
import time

from . import coolify_docker as d

Operation, Phase = d.Operation, d.Phase
_CAUSES = frozenset(('source_closure', 'ownership_unknown', 'stage_identity',
    'stage_collision', 'container_identity', 'container_spec', 'image_identity',
    'network_isolation', 'resource_isolation', 'startup_profile_unverified',
    'ledger_invalid', 'admission_busy', 'cleanup_incomplete', 'command_failed',
    'command_timeout', 'output_bound', 'interrupted'))
_TOKEN = re.compile('[a-f0-9]{32}\\Z')
_ID = re.compile('[a-f0-9]{64}\\Z')
_DIGEST = re.compile('sha256:[a-f0-9]{64}\\Z')
_LEDGER_LIMIT, _RECORD_LIMIT = 1024 * 1024, 32768
_MAIN_COMMAND, _CLEANUP_COMMAND = object(), object()


class CleanupError(Exception):
    def __init__(self, cause='command_failed'):
        super().__init__(cause if type(cause) is str and cause in _CAUSES else 'command_failed')


def _require(condition, cause):
    if not condition:
        raise CleanupError(cause) from None


def _category(error):
    if type(error) in (KeyboardInterrupt, SystemExit):
        return 'interrupted'
    if type(error) in (CleanupError, d.DockerError) and len(error.args) == 1:
        value = error.args[0]
        if type(value) is str and value in _CAUSES:
            return value
    return 'command_failed'


def _json(value):
    return json.dumps(value, sort_keys=True, separators=(',', ':'), allow_nan=False).encode('ascii')


def _decode(raw):
    def pairs(items):
        result = {}
        for key, value in items:
            _require(key not in result, 'ledger_invalid')
            result[key] = value
        return result
    try:
        value = json.loads(raw, object_pairs_hook=pairs,
                           parse_constant=lambda _: _require(False, 'ledger_invalid'))
        _require(type(value) is dict and _json(value) == raw, 'ledger_invalid')
        return value
    except CleanupError:
        raise
    except BaseException:
        raise CleanupError('ledger_invalid') from None


def _identity(info):
    result = (info.st_dev, info.st_ino, info.st_mode, info.st_uid, info.st_gid)
    # A writable directory's contents, mtime, ctime and link count are mutable.
    return result if stat.S_ISDIR(info.st_mode) else result + (
        info.st_nlink, info.st_size, info.st_mtime_ns, info.st_ctime_ns)


def _inode(info):
    return info.st_dev, info.st_ino, info.st_mode, info.st_uid, info.st_gid


def _owned_fd_identity(info):
    # Operational guards retain the full mode/owner identity above. Close has
    # a different purpose: release our original object even after metadata drift,
    # while preserving an unrelated object reusing the same descriptor number.
    return info.st_dev, info.st_ino, stat.S_IFMT(info.st_mode)


@dataclass(frozen=True, repr=False)
class _SourceView:
    root_fd: int
    root_path: Path
    source_sha256: str
    profile_sha256: str
    controller_invocation: str
    docker_socket: Path
    socket_identity: tuple
    deadline: float


@dataclass(frozen=True, repr=False)
class _LeafView:
    name: str
    fd: int
    identity: tuple


@dataclass(frozen=True, repr=False)
class _PhaseView:
    phase: Phase
    fd: int
    path: Path
    marker_identity: tuple
    leaves: tuple


@dataclass(frozen=True, repr=False)
class _RunView:
    root_fd: int
    root_path: Path
    run: str
    lease: str
    engine_invocation: str
    marker_identity: tuple
    parent: object
    parent_slice: str
    images: tuple
    phases: tuple
    deadline: float
    controller_invocation: str


@dataclass(frozen=True, repr=False)
class ContainerCleanupObservation:
    complete: bool
    first_failure: str | None


@dataclass(frozen=True, repr=False)
class PermitProjection:
    intent: str
    operation: Operation
    phase: Phase
    spec_sha256: str


def _dispatch(capability, expected, action, *args):
    _require(type(capability) is expected, 'ownership_unknown')
    try:
        core = object.__getattribute__(capability, '_core')
    except AttributeError:
        raise CleanupError('ownership_unknown') from None
    _require(type(core) is _CleanupCore, 'ownership_unknown')
    return core._invoke(capability, action, *args)


class _Capability:
    __slots__ = ('_core',)

    def __init__(self):
        raise CleanupError('ownership_unknown')


class AdmissionTicket(_Capability):
    __slots__ = ()

    def guard(self):
        return _dispatch(self, AdmissionTicket, 'admission_guard')


class CleanupLease(_Capability):
    __slots__ = ()


class CreatePermit(_Capability):
    __slots__ = ()


class CreatedReceipt(_Capability):
    __slots__ = ()


class RegisteredDependencies(_Capability):
    __slots__ = ()


class RuntimeEnvFD(_Capability):
    __slots__ = ()


class CreateAuthority(_Capability):
    __slots__ = ()

    @classmethod
    def open(cls, source_owner):
        raise CleanupError('source_closure') from None

    def require_clear_admission(self):
        return _dispatch(self, CreateAuthority, 'admission')

    def begin(self, ticket, issuedreservation):
        return _dispatch(self, CreateAuthority, 'begin', ticket, issuedreservation)

    def register_phase(self, lease, issued_phase):
        return _dispatch(self, CreateAuthority, 'register_phase', lease, issued_phase)

    def authorize(self, lease, operation, phase, dependencies):
        return _dispatch(self, CreateAuthority, 'authorize', lease, operation, phase, dependencies)

    def create_once(self, permit, runtime_env):
        return _dispatch(self, CreateAuthority, 'create', permit, runtime_env)

    def permit_projection(self, permit):
        return _dispatch(self, CreateAuthority, 'permit_projection', permit)


class CleanupAuthority(_Capability):
    __slots__ = ()

    @classmethod
    def open(cls, source_owner):
        raise CleanupError('source_closure') from None

    def reopen_unfinished(self):
        return _dispatch(self, CleanupAuthority, 'reopen')

    def for_live_lease(self, lease):
        return _dispatch(self, CleanupAuthority, 'session', lease)


class CleanupSession(_Capability):
    __slots__ = ()

    def reconcile(self):
        return _dispatch(self, CleanupSession, 'reconcile')

    def remove_verified(self):
        return _dispatch(self, CleanupSession, 'remove')

    def verify_empty(self):
        return _dispatch(self, CleanupSession, 'empty')


def _projection(operation, phase, run, intent, run_path, parent, images, postgres):
    """Locally build the complete nonsecret N1 selected inspection projection.

    Only the closed operation/leaf rule and static argv builder are shared with
    N1. No N1 run, journal, registry, authentication or cgroupfs is reconstructed.
    Callers are private controller code after owner/capability validation.
    """
    role, purpose, names = d.OwnedDocker._operation(operation, phase)
    _require(type(images) is tuple and len(images) == 4, 'image_identity')
    accounts = {}
    for item in images:
        _require(type(item) is tuple and len(item) == 5, 'image_identity')
        key, reference, engine_id, uid, gid = item
        _require(key in ('old', 'target', 'postgres', 'redis') and key not in accounts
                 and type(reference) is str and re.fullmatch(
                     r'[a-zA-Z0-9./:_-]+@sha256:[a-f0-9]{64}', reference)
                 and type(engine_id) is str and _DIGEST.fullmatch(engine_id)
                 and (purpose == 'identity' or type(uid) is int and type(gid) is int
                      and 0 < uid < 65536 and 0 < gid < 65536
                      and (key not in ('old', 'target') or (uid, gid) == (9999, 9999))), 'image_identity')
        accounts[key] = reference, engine_id, uid, gid
    reference, engine_id, uid, gid = accounts[role]
    if purpose == 'identity':
        user, executable = '65534:65534', '/usr/bin/id'
        command = [{'old': 'www-data', 'target': 'www-data', 'postgres': 'postgres', 'redis': 'redis'}[role]]
        memory, cpu, pids = 128, 250, 64
    elif purpose == 'postgres':
        user, executable = str(uid) + ':' + str(gid), '/usr/local/bin/docker-entrypoint.sh'
        command = ['postgres', '-c', 'listen_addresses=127.0.0.1', '-c',
                   'unix_socket_directories=/var/run/postgresql']
        memory, cpu, pids = 256, 250, 128
    elif purpose == 'redis':
        user, executable = str(uid) + ':' + str(gid), '/usr/local/bin/redis-server'
        command = ['--bind', '127.0.0.1', '--requirepass', 'rehearsal-only-password',
                   '--save', '', '--appendonly', 'no']
        memory, cpu, pids = 128, 125, 64
    else:
        user, executable = str(uid) + ':' + str(gid), 'php'
        command = ['artisan', 'migrate', '--force', '--isolated'] if purpose == 'migrate' else [
            '/probe/probe.php', purpose]
        memory, cpu, pids = (512, 1000, 256) if purpose == 'migrate' else (128, 250, 64)
    network = 'none'
    if purpose not in ('identity', 'postgres'):
        _require(type(postgres) is str and _ID.fullmatch(postgres), 'network_isolation')
        network = 'container:' + postgres
    mounts = [{'Type': 'bind', 'Source': str(run_path / phase.value / name),
               'Destination': d._DESTINATIONS[name],
               'RW': name not in d._APP_FILES and not (purpose == 'verify' and name == 'runtime-ssh'),
               'Propagation': 'rprivate'} for name in names]
    tmpfs = {'/tmp': 'rw,nosuid,nodev,noexec,mode=1777,size=16777216'}
    if purpose == 'identity':
        for target in ('/var/lib/postgresql/data', '/data'):
            tmpfs[target] = 'rw,nosuid,nodev,noexec,mode=0700,size=1048576'
    if purpose == 'postgres':
        tmpfs['/var/run/postgresql'] = 'rw,nosuid,nodev,noexec,mode=0777,size=16777216'
    labels = {'sifto.owner': 'native-rehearsal-v1', 'sifto.run': run, 'sifto.phase': phase.value,
              'sifto.role': role, 'sifto.operation': operation.value, 'sifto.intent': intent}
    return {'Name': 'sifto-native-' + run + '-' + intent,
        'Image': engine_id, 'Reference': reference, 'Labels': labels, 'User': user,
        'Entrypoint': [executable], 'Cmd': command, 'WorkingDir': '/var/www/html' if role in ('old', 'target') else '/',
        'Memory': memory * 1024**2, 'MemorySwap': memory * 1024**2, 'NanoCpus': cpu * 1000000,
        'PidsLimit': pids, 'CapDrop': ['ALL'], 'CapAdd': [], 'SecurityOpt': ['no-new-privileges'],
        'ReadonlyRootfs': True, 'NetworkMode': network, 'PidMode': '', 'IpcMode': 'private',
        'CgroupnsMode': 'private', 'CgroupParent': parent, 'Privileged': False,
        'Ports': {}, 'ExtraHosts': [], 'Devices': [], 'Mounts': mounts, 'Tmpfs': tmpfs}


def _owned_projection(spec, cid, observed):
    _require(type(cid) is str and _ID.fullmatch(cid), 'container_identity')
    _require(type(observed) is dict and observed.get('Id') == cid, 'container_identity')
    _require(len(_json(observed)) <= 65536 and set(observed) == set(spec) | {'Id', 'State'}, 'container_spec')
    for key, expected in spec.items():
        actual = observed[key]
        if key == 'Mounts':
            _require(type(actual) is list and all(type(m) is dict and type(m.get('Destination')) is str
                     for m in actual) and len({m['Destination'] for m in actual}) == len(actual), 'container_spec')
            actual = sorted(actual, key=lambda m: m['Destination'])
            expected = sorted(expected, key=lambda m: m['Destination'])
        _require(type(actual) is type(expected) and _json(actual) == _json(expected), 'container_spec')
    state = observed['State']
    _require(type(state) is dict and set(state) == {'Status', 'Running', 'ExitCode', 'OOMKilled'}
             and state['Status'] in ('created', 'running', 'exited', 'dead', 'removing')
             and type(state['Running']) is bool and type(state['OOMKilled']) is bool
             and type(state['ExitCode']) is int and 0 <= state['ExitCode'] <= 255, 'container_spec')
    # Ownership remains exact after OOM/nonzero exit. Lifecycle success is a
    # separate create/dependency condition, never a rewritten observation.
    return state


class _Ledger:
    """One held root-0600 single-link file, exclusive writer under controller lock."""
    def __init__(self, directory, header, *, reopen=False):
        self.directory, self.fd = directory, None
        self.close_pin = None
        self.records, self.raw = [], b''
        flags = os.O_RDWR | os.O_NOFOLLOW | os.O_CLOEXEC | os.O_NONBLOCK
        if not reopen:
            flags |= os.O_CREAT | os.O_EXCL
        try:
            self.fd = os.open('container-ledger', flags, 0o600, dir_fd=directory)
            self.pin = _inode(os.fstat(self.fd))
            self.close_pin = _owned_fd_identity(os.fstat(self.fd))
            self._guard()
            # Pin the inode created by this exclusive transaction in its durable
            # header. Reopening a copied replacement must compare its inode to
            # that original record, never mint a new pin as replacement proof.
            expected_header = {**header, 'ledger_identity': list(self.pin)}
            if reopen:
                size = os.fstat(self.fd).st_size
                _require(size <= _LEDGER_LIMIT, 'ledger_invalid')
                raw = os.pread(self.fd, size + 1, 0)
                _require(len(raw) == size, 'ledger_invalid')
                boundary = raw.rfind(b'\n') + 1
                complete, torn = raw[:boundary], raw[boundary:]
                for line in complete.splitlines():
                    _require(0 < len(line) <= _RECORD_LIMIT, 'ledger_invalid')
                    row = _decode(line)
                    self._validate(row)
                    self.records.append(row)
                    self.raw += line + b'\n'
                _require(bool(self.records) and self.records[0]['data'] == expected_header, 'ledger_invalid')
                if torn:
                    _require(len(torn) <= _RECORD_LIMIT, 'ledger_invalid')
                    # Only the incomplete suffix is removed. All preceding durable
                    # DISPATCH authorization remains part of the transaction.
                    os.ftruncate(self.fd, len(self.raw))
                    os.fsync(self.fd)
                    os.fsync(directory)
            else:
                self.append('HEADER', expected_header)
                os.fsync(directory)
        except BaseException:
            self.close()
            raise

    def _guard(self):
        info = os.fstat(self.fd)
        path = os.stat('container-ledger', dir_fd=self.directory, follow_symlinks=False)
        _require(_inode(info) == self.pin == _inode(path) and stat.S_ISREG(info.st_mode)
                 and info.st_uid == info.st_gid == 0 and info.st_nlink == path.st_nlink == 1
                 and stat.S_IMODE(info.st_mode) == 0o600, 'ledger_invalid')

    def guard(self):
        self._guard()
        _require(os.fstat(self.fd).st_size == len(self.raw)
                 and os.pread(self.fd, len(self.raw) + 1, 0) == self.raw, 'ledger_invalid')

    def _validate(self, row):
        _require(set(row) == {'sequence', 'previous', 'kind', 'data'}
                 and type(row['sequence']) is int and row['sequence'] == len(self.records)
                 and row['previous'] == hashlib.sha256(self.raw).hexdigest()
                 and type(row['kind']) is str and type(row['data']) is dict, 'ledger_invalid')
        kinds = [r['kind'] for r in self.records]
        if not kinds:
            _require(row['kind'] == 'HEADER', 'ledger_invalid')
        else:
            _require(row['kind'] in ('PHASE', 'FAILURE', 'INTENT', 'DISPATCH', 'ID', 'RECEIPT', 'RESOLVED',
                                    'REMOVED', 'ABORTED', 'EMPTY') and 'EMPTY' not in kinds, 'ledger_invalid')
            _transactions([*self.records, row])

    def append(self, kind, data):
        self.guard()
        row = {'sequence': len(self.records), 'previous': hashlib.sha256(self.raw).hexdigest(),
               'kind': kind, 'data': data}
        self._validate(row)
        line = _json(row) + b'\n'
        _require(len(line) <= _RECORD_LIMIT and len(self.raw) + len(line) <= _LEDGER_LIMIT, 'ledger_invalid')
        try:
            os.lseek(self.fd, 0, os.SEEK_END)
            d.OwnedDocker._write_all(self.fd, line)
            os.fsync(self.fd)
            self.records.append(row)
            self.raw += line
        except BaseException:
            # Never infer a durable state from a failed write/fsync. Future reads
            # must reopen and validate what actually survived on protected disk.
            raise CleanupError('ledger_invalid') from None

    def close(self):
        if self.fd is not None:
            try:
                if _owned_fd_identity(os.fstat(self.fd)) == self.close_pin:
                    os.close(self.fd)
            except OSError:
                pass
            self.fd = None


def _transactions(records):
    transactions, by_intent, ids, names = [], {}, set(), set()
    phases, failure = [], None
    for row in records[1:]:
        kind, data = row['kind'], row['data']
        if kind == 'FAILURE':
            _require(failure is None and set(data) == {'cause'} and type(data['cause']) is str
                     and data['cause'] in _CAUSES, 'ledger_invalid')
            failure = data['cause']
            continue
        if kind == 'PHASE':
            _require(set(data) == {'phase', 'identity', 'owner_marker', 'leaves', 'images'}
                     and data['phase'] == ('old' if not phases else 'fresh')
                     and len(phases) < 2 and type(data['leaves']) is list and type(data['images']) is list,
                     'ledger_invalid')
            previous = 'old' if phases else 'run'
            _require(all(t['phase'] != previous or t['state'] in ('REMOVED', 'ABORTED')
                         for t in transactions), 'ledger_invalid')
            phases.append(data['phase'])
            continue
        if kind == 'EMPTY':
            _require(data == {} and all(t['state'] in ('REMOVED', 'ABORTED') for t in transactions), 'ledger_invalid')
            continue
        _require(type(data.get('intent')) is str and _TOKEN.fullmatch(data['intent']), 'ledger_invalid')
        intent = data['intent']
        if kind == 'INTENT':
            _require(set(data) == {'intent', 'name', 'operation', 'phase', 'spec', 'spec_sha256', 'dependencies'}
                     and intent not in by_intent and data['name'] not in names
                     and type(data['spec']) is dict and data['spec_sha256'] == d._hash(_json(data['spec']))
                     and data['spec'].get('Name') == data['name']
                     and type(data['dependencies']) is list, 'ledger_invalid')
            try:
                operation, phase = Operation(data['operation']), Phase(data['phase'])
                d.OwnedDocker._operation(operation, phase)
                _require(phase.value == (phases[-1] if phases else 'run'), 'ledger_invalid')
            except BaseException:
                raise CleanupError('ledger_invalid') from None
            transaction = {**data, 'state': kind, 'id': None}
            by_intent[intent] = transaction
            transactions.append(transaction)
            names.add(data['name'])
            continue
        _require(intent in by_intent, 'ledger_invalid')
        transaction = by_intent[intent]
        transitions = {'DISPATCH': ('INTENT',), 'ID': ('DISPATCH',),
            'RECEIPT': ('ID',), 'RESOLVED': ('DISPATCH',),
            'REMOVED': ('ID', 'RECEIPT', 'RESOLVED'), 'ABORTED': ('INTENT',)}
        _require(kind in transitions and transaction['state'] in transitions[kind], 'ledger_invalid')
        if kind in ('ID', 'RESOLVED'):
            _require(set(data) == {'intent', 'id'} and type(data['id']) is str
                     and _ID.fullmatch(data['id']) and data['id'] not in ids, 'ledger_invalid')
            transaction['id'] = data['id']
            ids.add(data['id'])
        else:
            _require(set(data) == {'intent'}, 'ledger_invalid')
        transaction['state'] = kind
    return transactions


def _inspect_template():
    # Fixed selected nonsecret projection: never Env, logs, health or full config.
    fields = {'Image': '.Image', 'Reference': '.Config.Image', 'Labels': '.Config.Labels',
        'User': '.Config.User', 'Entrypoint': '.Config.Entrypoint', 'Cmd': '.Config.Cmd',
        'WorkingDir': '.Config.WorkingDir', 'Mounts': '.Mounts', 'Tmpfs': '.HostConfig.Tmpfs'}
    for key in ('Memory', 'MemorySwap', 'NanoCpus', 'PidsLimit', 'CapDrop', 'CapAdd',
                'SecurityOpt', 'ReadonlyRootfs', 'NetworkMode', 'PidMode', 'IpcMode',
                'CgroupnsMode', 'CgroupParent', 'Privileged', 'ExtraHosts', 'Devices'):
        fields[key] = '.HostConfig.' + key
    fields.update(Id='.Id', Name='.Name', Ports='.HostConfig.PortBindings')
    return '{' + ','.join('"' + k + '":{{json ' + v + '}}' for k, v in fields.items()) + (
        ',"State":{"Status":{{json .State.Status}},"Running":{{json .State.Running}},'
        '"ExitCode":{{json .State.ExitCode}},"OOMKilled":{{json .State.OOMKilled}}}}')


class _DockerCommands:
    """Private fixed Docker commands, supervised in the controller budget.

    The source owner must authenticate controller placement on each attach. No
    engine PID or engine-local cgroupfs is used as the create process owner.
    """
    def __init__(self, core, record):
        self.core, self.record = core, record
        self.serial, self.total = 0, 0
        directory = record['view'].root_fd
        try:
            os.mkdir('container-output', 0o700, dir_fd=directory)
            os.fsync(directory)
        except FileExistsError:
            pass
        self.output = os.open('container-output', os.O_RDONLY | os.O_DIRECTORY | os.O_NOFOLLOW | os.O_CLOEXEC,
                              dir_fd=directory)
        try:
            info = os.fstat(self.output)
            _require(info.st_uid == info.st_gid == 0 and stat.S_IMODE(info.st_mode) == 0o700, 'stage_identity')
            self.pin = _inode(info)
            self.close_pin = _owned_fd_identity(info)
            # A killed controller can leave its own bounded output files. Never
            # reuse or truncate those names while a helper or recovery may still
            # refer to them. Account their bytes without reading their contents.
            for name in os.listdir(self.output):
                match = re.fullmatch(r'command-([0-9]{1,8})\.(stdout|stderr)', name)
                _require(match is not None, 'stage_identity')
                leaf = os.stat(name, dir_fd=self.output, follow_symlinks=False)
                _require(stat.S_ISREG(leaf.st_mode) and leaf.st_uid == leaf.st_gid == 0
                         and leaf.st_nlink == 1 and stat.S_IMODE(leaf.st_mode) == 0o600
                         and 0 <= leaf.st_size <= 65536, 'stage_identity')
                self.serial = max(self.serial, int(match[1]))
                self.total += leaf.st_size
            _require(self.total <= 16 * 1024**2, 'output_bound')
        except BaseException:
            os.close(self.output)
            raise

    def _executable(self):
        return ['/usr/bin/docker', '--host=unix://' + str(self.core.source.docker_socket)]

    def _run(self, arguments, *, budget, env_fd=None, dispatch=None, limit=65536):
        _require(budget is _MAIN_COMMAND or budget is _CLEANUP_COMMAND, 'ownership_unknown')
        self.core._guard_record(self.record)
        if budget is _MAIN_COMMAND:
            bound = self.core._main_bound(self.record)
        else:
            _require('cleanup_deadline' in self.record, 'ownership_unknown')
            bound = min(self.record['cleanup_deadline'], self.core.source.deadline)
        _require(self.total + limit * 2 <= 16 * 1024**2, 'output_bound')
        _require(_inode(os.fstat(self.output)) == self.pin == _inode(os.stat('container-output',
                 dir_fd=self.record['view'].root_fd, follow_symlinks=False)), 'stage_identity')
        _require(self.serial < 99999999, 'output_bound')
        self.serial += 1
        prefix = 'command-' + str(self.serial)
        for suffix in ('stdout', 'stderr'):
            try:
                os.stat(prefix + '.' + suffix, dir_fd=self.output, follow_symlinks=False)
            except FileNotFoundError:
                continue
            raise CleanupError('stage_collision')
        deadline = min(bound, time.monotonic() + 30) - 3
        _require(time.monotonic() < deadline, 'command_timeout')
        def attach(pid, start):
            try:
                self.core.owner.attach_controller(pid, start)
                self.core._guard_record(self.record)
                if budget is _MAIN_COMMAND:
                    self.core._main_bound(self.record)
                _require(time.monotonic() < deadline, 'command_timeout')
                if dispatch is not None:
                    dispatch()
                if budget is _MAIN_COMMAND:
                    self.core._main_bound(self.record)
                _require(time.monotonic() < deadline, 'command_timeout')
            except BaseException as error:
                # N2's generic callback boundary cannot carry every N3 category.
                # Latch before conversion, including slow dispatch fsync expiry.
                if self.record['first_failure'] is None:
                    self.record['first_failure'] = _category(error)
                raise
        try:
            result = d._supervise([*self._executable(), *arguments],
                output_fd=self.output, prefix=prefix, pass_fds=() if env_fd is None else (env_fd,),
                deadline=deadline, limit=limit, attach=attach)
            return result
        except BaseException as error:
            if self.record['first_failure'] is None:
                self.record['first_failure'] = _category(error)
            if getattr(error, '_process_cleanup', None) is not d._PROCESS_CLEANUP:
                self.record['process_clean'] = False
            raise
        finally:
            for suffix in ('stdout', 'stderr'):
                name = prefix + '.' + suffix
                try:
                    self.total += os.stat(name, dir_fd=self.output, follow_symlinks=False).st_size
                    os.unlink(name, dir_fd=self.output)
                except FileNotFoundError:
                    pass
                except BaseException:
                    if self.record['first_failure'] is None:
                        self.record['first_failure'] = 'command_failed'
                    raise CleanupError(self.record['first_failure']) from None

    def create(self, spec, env_fd, dispatch):
        code, out, _ = self._run(d.OwnedDocker._argv(spec, env_fd), budget=_MAIN_COMMAND,
                                  env_fd=env_fd, dispatch=dispatch)
        _require(code == 0, 'command_failed')
        _require(re.fullmatch(rb'[a-f0-9]{64}\n?', out) is not None, 'container_identity')
        return out.decode('ascii').strip()

    def lookup(self, transaction):
        spec = transaction['spec']
        code, out, _ = self._run(['container', 'ls', '--all', '--no-trunc', '--format={{.ID}}',
            '--filter=name=^/' + spec['Name'] + '$',
            '--filter=label=sifto.intent=' + transaction['intent'],
            '--filter=label=sifto.run=' + self.record['view'].run], budget=_CLEANUP_COMMAND)
        _require(code == 0, 'command_failed')
        values = out.decode('ascii').splitlines()
        _require(len(values) <= 64 and len(values) == len(set(values))
                 and all(_ID.fullmatch(value) for value in values), 'container_identity')
        return values

    def collision(self, transaction):
        # A collision on either selector is fatal; intersecting the selectors
        # would miss an occupied name with a foreign intent (or vice versa).
        for selector in ('name=^/' + transaction['name'] + '$',
                         'label=sifto.intent=' + transaction['intent']):
            code, out, _ = self._run(['container', 'ls', '--all', '--no-trunc',
                                      '--format={{.ID}}', '--filter=' + selector], budget=_MAIN_COMMAND)
            _require(code == 0, 'command_failed')
            if out:
                return True
        return False

    def run_empty(self):
        code, out, _ = self._run(['container', 'ls', '--all', '--no-trunc', '--format={{.ID}}',
            '--filter=label=sifto.run=' + self.record['view'].run], budget=_CLEANUP_COMMAND)
        _require(code == 0, 'command_failed')
        return out == b''

    def inspect(self, cid):
        return self._inspect(cid, _MAIN_COMMAND)

    def inspect_cleanup(self, cid):
        return self._inspect(cid, _CLEANUP_COMMAND)

    def _inspect(self, cid, budget):
        _require(type(cid) is str and _ID.fullmatch(cid), 'container_identity')
        code, out, error = self._run(['container', 'inspect', '--format=' + _inspect_template(), cid], budget=budget)
        if code:
            _require(error.strip() in (('Error: No such container: ' + cid).encode(),
                     ('Error response from daemon: No such container: ' + cid).encode()), 'command_failed')
            return None
        value = d.strict_json(out)
        _require(type(value) is dict, 'container_spec')
        value['Name'] = value['Name'].removeprefix('/')
        for key in ('CapAdd', 'Devices', 'ExtraHosts'):
            if value[key] is None:
                value[key] = []
        if value['Ports'] is None:
            value['Ports'] = {}
        value['Mounts'] = [{k: m[k] for k in ('Type', 'Source', 'Destination', 'RW', 'Propagation')}
                           for m in value['Mounts']]
        return value

    def remove(self, cid):
        _require(type(cid) is str and _ID.fullmatch(cid), 'container_identity')
        code, out, _ = self._run(['container', 'rm', '--force', '--volumes', cid], budget=_CLEANUP_COMMAND)
        _require(code == 0 and out.strip() == cid.encode(), 'command_failed')

    def remove_main(self, cid):
        _require(type(cid) is str and _ID.fullmatch(cid), 'container_identity')
        code, out, _ = self._run(['container', 'rm', '--force', '--volumes', cid], budget=_MAIN_COMMAND)
        _require(code == 0 and out.strip() == cid.encode(), 'command_failed')

    def close(self):
        if self.output is None:
            return
        try:
            if _owned_fd_identity(os.fstat(self.output)) == self.close_pin:
                os.close(self.output)
        except OSError:
            pass
        self.output = None


class _CleanupCore:
    """Private owner port; all issuance is identity registered per core.

    N2S-4 must implement guard_source/guard_reservation/recover_reservation,
    guard_parent/resource_cleanup_complete/attach_controller with source-bound
    capabilities and protected historical policy. Fake owner facts are test
    inputs, never proof of an installed source closure or resource disappearance.
    """
    def __init__(self, owner, *, commands=_DockerCommands):
        self.owner, self.command_type = owner, commands
        self.caps, self.runs = {}, {}
        self.lock_fd, self.lock_pin = None, None
        self.lock_close_pin = None
        self.closed, self.first_failure = False, None
        self.pid = os.getpid()
        self.source = owner.guard_source()
        _require(type(self.source) is _SourceView, 'source_closure')
        self.source_pin = self._source_guard()

    def _mint(self, kind, data):
        capability = object.__new__(kind)
        capability._core = self
        self.caps[capability] = data
        return capability

    def authorities(self):
        return self._mint(CreateAuthority, {}), self._mint(CleanupAuthority, {})

    def _cap(self, capability, kind):
        _require(type(capability) is kind and capability in self.caps
                 and capability._core is self, 'ownership_unknown')
        return self.caps[capability]

    def _lease_record(self, lease):
        return self._cap(lease, CleanupLease)

    def _session_record(self, session):
        return self._cap(session, CleanupSession)

    def _source_guard(self):
        view = self.owner.guard_source()
        _require(type(view) is _SourceView and view == self.source
                 and type(view.root_path) is PosixPath and view.root_path.is_absolute()
                 and type(view.root_fd) is int and view.root_fd > 2
                 and type(view.source_sha256) is str and _ID.fullmatch(view.source_sha256)
                 and type(view.profile_sha256) is str and _ID.fullmatch(view.profile_sha256)
                 and type(view.controller_invocation) is str and _TOKEN.fullmatch(view.controller_invocation)
                 and type(view.docker_socket) is PosixPath
                 and type(view.deadline) in (int, float) and math.isfinite(view.deadline)
                 and 0 < view.deadline <= time.monotonic() + 2400, 'source_closure')
        _require(time.monotonic() < view.deadline, 'command_timeout')
        self._directory(view.root_fd, view.root_path)
        info = view.docker_socket.lstat()
        _require(stat.S_ISSOCK(info.st_mode) and info.st_uid == 0
                 and _identity(info) == view.socket_identity, 'source_closure')
        pin = _identity(os.fstat(view.root_fd))
        _require(not hasattr(self, 'source_pin') or pin == self.source_pin, 'source_closure')
        return pin

    @staticmethod
    def _directory(fd, path):
        _require(type(path) is PosixPath and path.is_absolute() and '..' not in path.parts
                 and all(not any(ch in part for ch in ('\x00', '\n', '\r', ',')) for part in path.parts), 'stage_identity')
        opened, _ = d._protected_open(path, os.O_RDONLY | os.O_DIRECTORY, category='stage_identity')
        try:
            info = os.fstat(opened)
            _require(stat.S_ISDIR(info.st_mode) and info.st_uid == info.st_gid == 0
                     and stat.S_IMODE(info.st_mode) == 0o700 and _identity(info) == _identity(os.fstat(fd)), 'stage_identity')
        finally:
            os.close(opened)

    @staticmethod
    def _marker(fd, expected):
        opened = os.open('owner-marker', os.O_RDONLY | os.O_NOFOLLOW | os.O_CLOEXEC | os.O_NONBLOCK, dir_fd=fd)
        try:
            info = os.fstat(opened)
            _require(stat.S_ISREG(info.st_mode) and info.st_uid == info.st_gid == 0
                     and info.st_nlink == 1 and stat.S_IMODE(info.st_mode) == 0o600
                     and _identity(info) == expected, 'stage_identity')
        finally:
            os.close(opened)

    def _view(self, reservation):
        self._source_guard()
        view = self.owner.guard_reservation(reservation)
        _require(type(view) is _RunView and type(view.run) is str and _TOKEN.fullmatch(view.run)
                 and all(type(value) is str and _TOKEN.fullmatch(value)
                         for value in (view.lease, view.engine_invocation, view.controller_invocation))
                 and view.root_path == self.source.root_path / view.run
                 and view.parent_slice == 'siftocoolify' + view.run + '.slice'
                 and type(view.phases) is tuple
                 and len(view.phases) == 2 and {p.phase for p in view.phases} == {Phase.OLD, Phase.FRESH}
                 and type(view.deadline) in (int, float) and math.isfinite(view.deadline)
                 and 0 < view.deadline
                 and (view.controller_invocation != self.source.controller_invocation
                      or view.deadline <= time.monotonic() + 2400), 'stage_identity')
        self.owner.guard_parent(view.parent, reservation)
        self._directory(view.root_fd, view.root_path)
        self._marker(view.root_fd, view.marker_identity)
        for phase in view.phases:
            _require(type(phase) is _PhaseView and phase.path == view.root_path / phase.phase.value
                     and type(phase.leaves) is tuple, 'stage_identity')
            self._directory(phase.fd, phase.path)
            self._marker(phase.fd, phase.marker_identity)
        return view

    def _phase_snapshot(self, view, phase):
        _require(type(phase) is _PhaseView and phase in view.phases and type(phase.leaves) is tuple,
                 'ownership_unknown')
        # Image identities are fixed in the base header; independently observed
        # accounts become authoritative only with durable phase registration.
        _projection(Operation.POSTGRES, phase.phase, view.run, '0' * 32, view.root_path,
                    view.parent_slice, view.images, None)
        names = set()
        for leaf in phase.leaves:
            _require(type(leaf) is _LeafView and leaf.name in d._LEAVES and leaf.name not in names,
                     'stage_identity')
            names.add(leaf.name)
            self._leaf(view, phase, leaf)
        _require(bool(names), 'stage_identity')
        return json.loads(_json({'phase': phase.phase.value, 'identity': _identity(os.fstat(phase.fd)),
            'owner_marker': phase.marker_identity, 'leaves': [
                {'role': leaf.name, 'identity': leaf.identity} for leaf in phase.leaves], 'images': view.images}))

    @staticmethod
    def _leaf(view, phase, leaf):
        directory = leaf.name in (*d._APP_DIRS, 'pg', 'redis')
        flags = os.O_RDONLY | os.O_NOFOLLOW | os.O_CLOEXEC | os.O_NONBLOCK
        if directory:
            flags |= os.O_DIRECTORY
        opened = os.open(leaf.name, flags, dir_fd=phase.fd)
        try:
            info = os.fstat(opened)
            owner = (0, 0) if leaf.name == 'coolify.dump' else (9999, 9999)
            if leaf.name in ('pg', 'redis'):
                role = 'postgres' if leaf.name == 'pg' else 'redis'
                image = next((v for v in view.images if v[0] == role), ())
                _require(len(image) == 5 and all(type(v) is int and 0 < v < 65536 for v in image[3:]), 'image_identity')
                owner = image[3:]
            _require(_identity(info) == leaf.identity == _identity(os.fstat(leaf.fd))
                     and (info.st_uid, info.st_gid) == owner
                     and stat.S_IMODE(info.st_mode) == (0o700 if directory else 0o600)
                     and (stat.S_ISDIR(info.st_mode) if directory else stat.S_ISREG(info.st_mode) and info.st_nlink == 1),
                     'stage_identity')
        finally:
            os.close(opened)

    def _header(self, view):
        return json.loads(_json({'schema': 1, 'run': view.run, 'lease': view.lease,
            'controller_invocation': view.controller_invocation, 'engine_invocation': view.engine_invocation,
            'source_sha256': self.source.source_sha256, 'profile_sha256': self.source.profile_sha256,
            'deadline': view.deadline,
            'socket': str(self.source.docker_socket), 'socket_identity': self.source.socket_identity,
            'staging_identity': _identity(os.fstat(self.source.root_fd)),
            'run_identity': _identity(os.fstat(view.root_fd)), 'owner_marker': view.marker_identity,
            'parent_slice': view.parent_slice, 'images': [image[:3] for image in view.images],
            'phases': [{'phase': phase.phase.value, 'identity': _identity(os.fstat(phase.fd)),
                        'owner_marker': phase.marker_identity}
                       for phase in view.phases]}))

    def _lock(self):
        self._source_guard()
        if self.lock_fd is None:
            fd = os.open('container-controller-lock', os.O_RDWR | os.O_CREAT | os.O_NOFOLLOW | os.O_CLOEXEC | os.O_NONBLOCK,
                         0o600, dir_fd=self.source.root_fd)
            try:
                info = os.fstat(fd)
                _require(stat.S_ISREG(info.st_mode) and info.st_uid == info.st_gid == 0
                         and info.st_nlink == 1 and stat.S_IMODE(info.st_mode) == 0o600, 'stage_identity')
                try:
                    fcntl.flock(fd, fcntl.LOCK_EX | fcntl.LOCK_NB)
                except BlockingIOError:
                    raise CleanupError('admission_busy') from None
                os.fsync(fd)
                os.fsync(self.source.root_fd)
                self.lock_fd, self.lock_pin = fd, _inode(info)
                self.lock_close_pin = _owned_fd_identity(info)
            except BaseException:
                os.close(fd)
                raise
        _require(_inode(os.fstat(self.lock_fd)) == self.lock_pin == _inode(os.stat('container-controller-lock',
                 dir_fd=self.source.root_fd, follow_symlinks=False)), 'stage_identity')

    def _guard_record(self, record):
        self._lock()
        view = self._view(record['reservation'])
        _require(self._header(view) == record['header'], 'stage_identity')
        for registered in record['phases'].values():
            phase = next(p for p in view.phases if p.phase.value == registered['phase'])
            _require(self._phase_snapshot(view, phase) == registered, 'stage_identity')
        record['ledger'].guard()
        record['view'] = view

    def _open_record(self, reservation, *, reopen=False):
        view = self._view(reservation)
        header = self._header(view)
        ledger = _Ledger(view.root_fd, header, reopen=reopen)
        record = {'reservation': reservation, 'view': view, 'header': header, 'ledger': ledger,
                  'transactions': _transactions(ledger.records), 'first_failure': None,
                  'process_clean': not reopen, 'verified': set(), 'running': set(), 'recovery': reopen,
                  'create_failed': False}
        record['phases'] = {row['data']['phase']: row['data'] for row in ledger.records if row['kind'] == 'PHASE'}
        record['first_failure'] = next((row['data']['cause'] for row in ledger.records
                                        if row['kind'] == 'FAILURE'), None)
        try:
            self._validate_transactions(record)
            record['commands'] = self.command_type(self, record)
        except BaseException:
            ledger.close()
            raise
        self.runs[view.run] = record
        return record

    def _validate_transactions(self, record):
        view = record['view']
        for registered in record['phases'].values():
            phase = next(p for p in view.phases if p.phase.value == registered['phase'])
            _require(self._phase_snapshot(view, phase) == registered, 'stage_identity')
        for transaction in record['transactions']:
            operation, phase = Operation(transaction['operation']), Phase(transaction['phase'])
            dependencies = transaction['dependencies']
            _require(all(type(value) is str and _ID.fullmatch(value) for value in dependencies)
                     and len(dependencies) == len(set(dependencies)), 'ledger_invalid')
            expected = _projection(operation, phase, view.run, transaction['intent'], view.root_path,
                view.parent_slice, view.images, dependencies[0] if dependencies else None)
            _require(_json(expected) == _json(transaction['spec']), 'ledger_invalid')

    def _append(self, record, kind, data):
        try:
            record['ledger'].append(kind, data)
            record['transactions'] = _transactions(record['ledger'].records)
            record['phases'] = {row['data']['phase']: row['data'] for row in record['ledger'].records
                                if row['kind'] == 'PHASE'}
        except BaseException as error:
            if record['first_failure'] is None:
                record['first_failure'] = _category(error)
            raise

    def _process_witness(self, record):
        if record['recovery']:
            # Neither a new controller invocation, a dead old PID nor an empty
            # daemon query proves its old create/helper processes disappeared.
            # The source owner must authenticate the recorded historical epoch
            # and all its controller-owned helpers/cgroup before issuing this
            # exact private N2 sentinel. A boolean never constitutes evidence.
            record['process_clean'] = self.owner.recovery_process_cleanup(
                record['reservation'], record['header']['controller_invocation']) is d._PROCESS_CLEANUP
        return record['process_clean']

    def _inventory(self):
        result = {}
        for name in os.listdir(self.source.root_fd):
            if name == 'container-controller-lock':
                continue
            _require(_TOKEN.fullmatch(name) is not None, 'cleanup_incomplete')
            info = os.stat(name, dir_fd=self.source.root_fd, follow_symlinks=False)
            _require(stat.S_ISDIR(info.st_mode) and info.st_uid == info.st_gid == 0
                     and stat.S_IMODE(info.st_mode) == 0o700, 'stage_identity')
            result[name] = _identity(info)
        return result

    def _scan(self, fresh=None):
        self._lock()
        result = []
        for name, identity in sorted(self._inventory().items()):
            if fresh is not None and name == fresh.run:
                _require(identity == _identity(os.fstat(fresh.root_fd)), 'stage_identity')
                try:
                    os.stat('container-ledger', dir_fd=fresh.root_fd, follow_symlinks=False)
                except FileNotFoundError:
                    continue
                raise CleanupError('stage_collision')
            if name in self.runs:
                record = self.runs[name]
                self._guard_record(record)
            else:
                directory = os.open(name, os.O_RDONLY | os.O_DIRECTORY | os.O_NOFOLLOW | os.O_CLOEXEC,
                                    dir_fd=self.source.root_fd)
                try:
                    try:
                        os.stat('container-ledger', dir_fd=directory, follow_symlinks=False)
                    except FileNotFoundError:
                        raise CleanupError('cleanup_incomplete') from None
                finally:
                    os.close(directory)
                reservation = self.owner.recover_reservation(name)
                view = self._view(reservation)
                _require(view.run == name, 'ownership_unknown')
                record = self._open_record(reservation, reopen=True)
            result.append(record)
        return result

    def _admission(self):
        for record in self._scan():
            empty = any(row['kind'] == 'EMPTY' for row in record['ledger'].records)
            _require(empty and self._process_witness(record)
                     and self.owner.resource_cleanup_complete(record['reservation']) is True,
                     'cleanup_incomplete')
        return self._mint(AdmissionTicket, {'used': False, 'baseline': self._inventory()})

    def _begin(self, ticket, reservation):
        issued = self._cap(ticket, AdmissionTicket)
        _require(not issued['used'], 'ownership_unknown')
        issued['used'] = True
        self._lock()
        view = self._view(reservation)
        _require(view.controller_invocation == self.source.controller_invocation
                 and view.run not in issued['baseline'], 'ownership_unknown')
        inventory = self._inventory()
        _require(inventory.get(view.run) == _identity(os.fstat(view.root_fd)), 'stage_identity')
        _require({name: identity for name, identity in inventory.items() if name != view.run}
                 == issued['baseline'], 'cleanup_incomplete')
        # Recheck after ticket issuance; a ticket cannot hide later unfinished work.
        # Only this authenticated, same-invocation reservation, absent from the
        # locked ticket baseline and still lacking any ledger, is a fresh claim.
        for record in self._scan(fresh=view):
            _require(any(r['kind'] == 'EMPTY' for r in record['ledger'].records)
                     and self._process_witness(record)
                     and self.owner.resource_cleanup_complete(record['reservation']) is True, 'cleanup_incomplete')
        _require(time.monotonic() < min(view.deadline, self.source.deadline) - 120, 'command_timeout')
        _require(view.run not in self.runs, 'stage_collision')
        record = self._open_record(reservation)
        return self._mint(CleanupLease, record)

    def _register_dependencies(self, lease, phase, receipts):
        record = self._lease_record(lease)
        _require(type(phase) is Phase and type(receipts) is tuple, 'ownership_unknown')
        values = []
        for receipt in receipts:
            value = self._cap(receipt, CreatedReceipt)
            _require(value['record'] is record and value['phase'] is phase, 'network_isolation')
            values.append(value['id'])
        return self._mint(RegisteredDependencies, {'record': record, 'phase': phase, 'ids': tuple(values)})

    def _register_phase(self, lease, issued_phase):
        record = self._lease_record(lease)
        self._guard_record(record)
        _require(not record['recovery'] and not any(r['kind'] == 'EMPTY' for r in record['ledger'].records),
                 'ownership_unknown')
        view = self._view(record['reservation'])
        phase = self.owner.guard_phase(record['reservation'], issued_phase)
        _require(type(phase) is _PhaseView and any(phase is p for p in view.phases), 'ownership_unknown')
        _require(phase.phase.value not in record['phases'], 'stage_collision')
        _require(phase.phase is (Phase.OLD if not record['phases'] else Phase.FRESH), 'stage_identity')
        previous = Phase.RUN if phase.phase is Phase.OLD else Phase.OLD
        _require(self._process_witness(record)
                     and not any(t['phase'] == previous.value and t['state'] not in ('REMOVED', 'ABORTED')
                                 for t in record['transactions'])
                     and not any(type(cap) is CreatePermit and data['record'] is record
                                 and data['phase'] is previous and not data['used']
                                 for cap, data in self.caps.items()), 'resource_isolation')
        self._append(record, 'PHASE', self._phase_snapshot(view, phase))
        # A previous phase's live cleanup handles cannot act on the new epoch.
        # The run's already pinned cleanup deadline survives every phase change.
        for capability, bound in list(self.caps.items()):
            if type(capability) is CleanupSession and bound is record:
                del self.caps[capability]
        return None

    def _register_running(self, receipt):
        # Private owner registration hook. N4 must supply authenticated live
        # lifecycle evidence; the controller still independently inspects below.
        value = self._cap(receipt, CreatedReceipt)
        record = value['record']
        self._guard_record(record)
        transaction = next(t for t in record['transactions'] if t['id'] == value['id'])
        state = _owned_projection(transaction['spec'], value['id'], record['commands'].inspect(value['id']))
        _require(state['Running'] and state['Status'] == 'running' and not state['OOMKilled'], 'network_isolation')
        record['running'].add(value['id'])

    def _remove_receipt(self, receipt):
        """Normal one-container lifecycle; never borrows final cleanup budget."""
        value = self._cap(receipt, CreatedReceipt)
        record = value['record']
        self._guard_record(record)
        self._main_bound(record)
        transaction = next(t for t in record['transactions'] if t['id'] == value['id'])
        _require(transaction['state'] == 'RECEIPT' and value['id'] in record['verified'], 'ownership_unknown')
        observed = record['commands'].inspect(value['id'])
        _require(observed is not None, 'container_identity')
        _owned_projection(transaction['spec'], value['id'], observed)
        record['commands'].remove_main(value['id'])
        _require(record['commands'].inspect(value['id']) is None, 'cleanup_incomplete')
        self._append(record, 'REMOVED', {'intent': transaction['intent']})
        record['running'].discard(value['id'])
        self._main_bound(record)
        return True

    def _permit_projection(self, permit):
        value = self._cap(permit, CreatePermit)
        _require(not value['used'], 'ownership_unknown')
        self._guard_record(value['record'])
        self._main_bound(value['record'])
        return PermitProjection(value['intent'], value['operation'], value['phase'],
                                d._hash(_json(value['spec'])))

    def _register_runtime_env(self, lease, phase):
        record = self._lease_record(lease)
        self._guard_record(record)
        _require(phase in (Phase.OLD, Phase.FRESH), 'stage_identity')
        return self._mint(RuntimeEnvFD, {'record': record, 'phase': phase})

    def _authorize(self, lease, operation, phase, dependencies):
        record = self._lease_record(lease)
        role, purpose, names = d.OwnedDocker._operation(operation, phase)
        self._guard_record(record)
        self._main_bound(record)
        _require(not record['create_failed'], 'interrupted')
        _require(not record['recovery'] and not any(r['kind'] == 'EMPTY' for r in record['ledger'].records), 'ownership_unknown')
        self._check_active(record, operation, phase)
        bound = self._cap(dependencies, RegisteredDependencies)
        _require(bound['record'] is record and bound['phase'] is phase, 'network_isolation')
        ids = bound['ids']
        if purpose in ('identity', 'postgres'):
            _require(not ids, 'network_isolation')
        else:
            _require(len(ids) == 1 and ids[0] in record['running'], 'network_isolation')
            postgres = next((t for t in record['transactions'] if t['id'] == ids[0]), None)
            _require(postgres is not None and postgres['operation'] == Operation.POSTGRES.value
                     and postgres['phase'] == phase.value and postgres['state'] == 'RECEIPT', 'network_isolation')
            state = _owned_projection(postgres['spec'], ids[0], record['commands'].inspect(ids[0]))
            _require(state['Running'] and state['Status'] == 'running' and not state['OOMKilled'], 'network_isolation')
        view = record['view']
        if phase is not Phase.RUN:
            _require(phase.value in record['phases'], 'stage_identity')
            selected = next(p for p in view.phases if p.phase is phase)
            _require(set(names) <= {leaf.name for leaf in selected.leaves}, 'stage_identity')
        intent = secrets.token_hex(16)
        spec = _projection(operation, phase, view.run, intent, view.root_path, view.parent_slice,
                           view.images, ids[0] if ids else None)
        _require(not any(t['intent'] == intent or t['name'] == spec['Name'] for t in record['transactions']), 'stage_collision')
        self._main_bound(record)
        return self._mint(CreatePermit, {'record': record, 'operation': operation, 'phase': phase,
            'dependencies': dependencies, 'spec': spec, 'intent': intent, 'used': False,
            'needs_env': 'runtime.env' in names})

    @staticmethod
    def _check_active(record, operation, phase):
        _, purpose, _ = d.OwnedDocker._operation(operation, phase)
        current = Phase.FRESH if 'fresh' in record['phases'] else Phase.OLD if 'old' in record['phases'] else Phase.RUN
        _require(phase is current, 'stage_identity')
        for existing in record['transactions']:
            active = existing['state'] not in ('REMOVED', 'ABORTED')
            _require(not (existing['phase'] == phase.value and existing['operation'] == operation.value
                         and (active or operation is Operation.MIGRATE_TARGET)), 'container_spec')
            if active:
                _, previous, _ = d.OwnedDocker._operation(Operation(existing['operation']), Phase(existing['phase']))
                _require(existing['phase'] == phase.value and purpose != 'identity' and previous != 'identity'
                         and not (purpose in ('baseline', 'network', 'verify') and previous in ('baseline', 'network', 'verify'))
                         and not (purpose == previous == 'migrate'), 'resource_isolation')

    def _main_bound(self, record):
        _require(not record['recovery']
                 and record['view'].controller_invocation == self.source.controller_invocation,
                 'ownership_unknown')
        bound = min(record['view'].deadline, self.source.deadline) - 120
        _require(time.monotonic() < bound, 'command_timeout')
        return bound

    def _runtime(self, permit, runtime_env):
        if not permit['needs_env']:
            _require(runtime_env is None, 'stage_identity')
            return None
        value = self._cap(runtime_env, RuntimeEnvFD)
        record = permit['record']
        _require(value['record'] is record and value['phase'] is permit['phase'], 'stage_identity')
        phase = next(p for p in record['view'].phases if p.phase is permit['phase'])
        leaf = next(leaf for leaf in phase.leaves if leaf.name == 'runtime.env')
        self._leaf(record['view'], phase, leaf)
        # dup shares the open-file description and cannot lower access mode.
        # Reopen the authenticated fixed role independently, without any read.
        fd = os.open('runtime.env', os.O_RDONLY | os.O_NOFOLLOW | os.O_CLOEXEC | os.O_NONBLOCK, dir_fd=phase.fd)
        try:
            _require(_identity(os.fstat(fd)) == leaf.identity
                     and fcntl.fcntl(fd, fcntl.F_GETFL) & os.O_ACCMODE == os.O_RDONLY
                     and os.lseek(fd, 0, os.SEEK_CUR) == 0, 'stage_identity')
            return fd
        except BaseException:
            os.close(fd)
            raise

    def _create(self, permit, runtime_env):
        issued = self._cap(permit, CreatePermit)
        _require(not issued['used'], 'ownership_unknown')
        issued['used'] = True
        record = issued['record']
        self._guard_record(record)
        self._main_bound(record)
        _require(not record['create_failed'], 'interrupted')
        self._check_active(record, issued['operation'], issued['phase'])
        # Revalidate dependencies and all owner facts immediately before intent.
        dependencies = self._cap(issued['dependencies'], RegisteredDependencies)
        for cid in dependencies['ids']:
            transaction = next(t for t in record['transactions'] if t['id'] == cid)
            state = _owned_projection(transaction['spec'], cid, record['commands'].inspect(cid))
            _require(state['Running'] and state['Status'] == 'running' and not state['OOMKilled'], 'network_isolation')
        env_fd = self._runtime(issued, runtime_env)
        try:
            transaction = {'intent': issued['intent'], 'name': issued['spec']['Name'],
                'operation': issued['operation'].value, 'phase': issued['phase'].value,
                'spec': issued['spec'], 'spec_sha256': d._hash(_json(issued['spec'])),
                'dependencies': list(dependencies['ids'])}
            _require(not record['commands'].collision(transaction), 'stage_collision')
            self._append(record, 'INTENT', transaction)
            def dispatch():
                self._guard_record(record)
                self._main_bound(record)
                self._append(record, 'DISPATCH', {'intent': issued['intent']})
                self._main_bound(record)
            cid = record['commands'].create(issued['spec'], env_fd, dispatch)
            _require(type(cid) is str and _ID.fullmatch(cid), 'container_identity')
            self._append(record, 'ID', {'intent': issued['intent'], 'id': cid})
            state = _owned_projection(issued['spec'], cid, record['commands'].inspect(cid))
            _require(state == {'Status': 'created', 'Running': False, 'ExitCode': 0, 'OOMKilled': False}, 'container_spec')
            self._append(record, 'RECEIPT', {'intent': issued['intent']})
            record['verified'].add(cid)
            return self._mint(CreatedReceipt, {'record': record, 'id': cid, 'phase': issued['phase']})
        except BaseException as error:
            if type(error) is d.DockerError and getattr(error, '_process_cleanup', None) is not d._PROCESS_CLEANUP:
                record['process_clean'] = False
            if record['first_failure'] is not None:
                raise CleanupError(record['first_failure']) from None
            raise
        finally:
            if env_fd is not None:
                os.close(env_fd)

    def _session(self, record):
        self._guard_record(record)
        historical = record['view'].controller_invocation != self.source.controller_invocation
        record.setdefault('cleanup_deadline', min(time.monotonic() + 120, self.source.deadline,
                          float('inf') if historical else record['view'].deadline))
        return self._mint(CleanupSession, record)

    def _reconcile(self, record):
        self._guard_record(record)
        _require(time.monotonic() < record['cleanup_deadline'], 'command_timeout')
        complete = True
        for transaction in list(record['transactions']):
            if transaction['state'] in ('REMOVED', 'ABORTED'):
                continue
            if transaction['state'] == 'INTENT':
                self._append(record, 'ABORTED', {'intent': transaction['intent']})
                continue
            candidates = record['commands'].lookup(transaction)
            _require(len(candidates) <= 1, 'ownership_unknown')
            if transaction['id'] is None:
                if not candidates:
                    complete = False
                    continue
                cid = candidates[0]
                _owned_projection(transaction['spec'], cid, record['commands'].inspect_cleanup(cid))
                self._append(record, 'RESOLVED', {'intent': transaction['intent'], 'id': cid})
            else:
                cid = transaction['id']
                _require(not candidates or candidates == [cid], 'ownership_unknown')
                observed = record['commands'].inspect_cleanup(cid)
                if observed is None:
                    self._append(record, 'REMOVED', {'intent': transaction['intent']})
                    continue
                _owned_projection(transaction['spec'], cid, observed)
            record['verified'].add(cid)
        return complete and self._process_witness(record)

    def _remove(self, record):
        self._guard_record(record)
        if not self._process_witness(record):
            return False
        complete = True
        for transaction in list(record['transactions']):
            if transaction['state'] in ('REMOVED', 'ABORTED'):
                continue
            cid = transaction['id']
            if cid is None or cid not in record['verified']:
                complete = False
                continue
            candidates = record['commands'].lookup(transaction)
            _require(not candidates or candidates == [cid], 'ownership_unknown')
            observed = record['commands'].inspect_cleanup(cid)
            if observed is not None:
                _owned_projection(transaction['spec'], cid, observed)
                record['commands'].remove(cid)
                _require(record['commands'].inspect_cleanup(cid) is None, 'cleanup_incomplete')
            self._append(record, 'REMOVED', {'intent': transaction['intent']})
        return complete and self._process_witness(record)

    def _empty(self, record):
        self._guard_record(record)
        _require(time.monotonic() < record['cleanup_deadline'], 'command_timeout')
        if not self._process_witness(record):
            return False
        if any(r['kind'] == 'EMPTY' for r in record['ledger'].records):
            return True
        if not record['process_clean'] or any(t['state'] not in ('REMOVED', 'ABORTED') for t in record['transactions']):
            return False
        for transaction in record['transactions']:
            _require(not record['commands'].lookup(transaction), 'cleanup_incomplete')
        _require(record['commands'].run_empty(), 'cleanup_incomplete')
        self._append(record, 'EMPTY', {})
        return True

    def _invoke(self, capability, action, *args):
        record = None
        cleanup = action in ('reconcile', 'remove', 'empty')
        try:
            _require(not self.closed and os.getpid() == self.pid and capability in self.caps, 'ownership_unknown')
            if cleanup:
                record = self._session_record(capability)
                complete = {'reconcile': self._reconcile, 'remove': self._remove, 'empty': self._empty}[action](record)
                return ContainerCleanupObservation(complete, record['first_failure'])
            if action == 'admission':
                return self._admission()
            if action == 'admission_guard':
                self._cap(capability, AdmissionTicket)
                self._lock()
                return None
            if action == 'begin':
                return self._begin(*args)
            if action == 'authorize':
                record = self._lease_record(args[0])
                return self._authorize(*args)
            if action == 'permit_projection':
                return self._permit_projection(*args)
            if action == 'register_phase':
                record = self._lease_record(args[0])
                return self._register_phase(*args)
            if action == 'create':
                record = self._cap(args[0], CreatePermit)['record']
                return self._create(*args)
            if action == 'session':
                record = self._lease_record(args[0])
                return self._session(record)
            if action == 'reopen':
                return tuple(self._session(r) for r in self._scan()
                             if not any(row['kind'] == 'EMPTY' for row in r['ledger'].records))
            raise CleanupError('ownership_unknown')
        except BaseException as error:
            cause = _category(error)
            if record is not None:
                if action == 'create' and cause != 'ownership_unknown':
                    record['create_failed'] = True
                effectful = cleanup or action == 'create' and cause != 'ownership_unknown'
                if effectful and record['first_failure'] is None:
                    record['first_failure'] = cause
                if record['first_failure'] is not None and not any(
                        row['kind'] in ('FAILURE', 'EMPTY') for row in record['ledger'].records):
                    try:
                        self._append(record, 'FAILURE', {'cause': record['first_failure']})
                    except BaseException:
                        # Diagnostic durability never replaces the first cause.
                        pass
                if cleanup:
                    return ContainerCleanupObservation(False, record['first_failure'])
            if self.first_failure is None:
                self.first_failure = cause
            raise CleanupError(cause) from None

    def close(self):
        if self.closed:
            return
        for record in self.runs.values():
            record['commands'].close()
            record['ledger'].close()
        if self.lock_fd is not None:
            try:
                if _owned_fd_identity(os.fstat(self.lock_fd)) == self.lock_close_pin:
                    os.close(self.lock_fd)
            except OSError:
                pass
        self.lock_fd = None
        self.closed = True
        self.caps.clear()
