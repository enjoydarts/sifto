"""Dormant, closed systemd resource ownership for the native rehearsal.

No host dispatch imports this module. The standalone cgroupfs implementation is
unchanged. Synthetic subclasses exercise public fixtures; they grant no native
Docker or systemd authority. Installation and real kernel proof are later gates.
"""
from dataclasses import dataclass
from functools import wraps
import hashlib
import json
import os
from pathlib import Path
import platform
import re
import secrets
import stat
import threading
import time

from . import coolify_docker as d
from .coolify_rehearsal import FilePolicy

CONTROLLER_UNIT = 'sifto-coolify-native-controller.service'
CONTROLLER_GROUP = '/system.slice/' + CONTROLLER_UNIT
CONTROLLER_MEMORY, WORKLOAD_MEMORY = 64 * 1024**2, 1216 * 1024**2
CONTROLLER_CPU, WORKLOAD_CPU = 25, 1725
CONTROLLER_TASKS, WORKLOAD_TASKS = 16, 560
_CONTROLLER_ADMISSIONS = set()
_ADMISSION_LOCK = threading.Lock()
_CATEGORIES = frozenset(('context_policy', 'source_closure', 'controller_identity',
    'ownership_unknown', 'stage_collision', 'stage_identity', 'journal_invalid',
    'resource_isolation', 'resource_capacity', 'unit_identity', 'manager_invalid',
    'command_failed', 'command_timeout', 'output_bound', 'cleanup_incomplete',
    'bootstrap_invalid', 'container_identity', 'interrupted'))


class SystemdError(Exception):
    def __init__(self, category='command_failed'):
        super().__init__(category if type(category) is str and category in _CATEGORIES
                         else 'command_failed')
        self.cleanup_complete = False


def _static_category(error, default='command_failed'):
    if type(error) in (KeyboardInterrupt, SystemExit):
        return 'interrupted'
    # Only these two closed error constructors carry trusted static categories.
    # Never call str() on an arbitrary adapter/OS exception.
    if type(error) in (SystemdError, d.DockerError) and len(error.args) == 1:
        category = error.args[0]
        if type(category) is str and category in _CATEGORIES:
            return category
    return default


def _require(condition, category):
    if not condition:
        raise SystemdError(category) from None


def _closed(method):
    @wraps(method)
    def call(*args, **kwargs):
        capability = args[1] if len(args) > 1 else kwargs.get('reservation', kwargs.get('handle'))
        try:
            return method(*args, **kwargs)
        except (KeyboardInterrupt, SystemExit):
            args[0]._latch_public(capability, 'interrupted')
            raise SystemdError('interrupted') from None
        except BaseException as error:
            category = _static_category(error)
            safe = SystemdError(category)
            if type(error) in (SystemdError, d.DockerError):
                safe.cleanup_complete = getattr(error, 'cleanup_complete', False) is True
            args[0]._latch_public(capability, category)
            raise safe from None
    return call


@dataclass(frozen=True, repr=False)
class _Limits:
    memory: int
    cpu: int
    tasks: int
    swap: int = 0


CONTROLLER_LIMIT = _Limits(CONTROLLER_MEMORY, CONTROLLER_CPU, CONTROLLER_TASKS)
WORKLOAD_LIMIT = _Limits(WORKLOAD_MEMORY, WORKLOAD_CPU, WORKLOAD_TASKS)
ENGINE_PREPARATION = _Limits(WORKLOAD_MEMORY, WORKLOAD_CPU, 48)
ENGINE_STEADY = _Limits(192 * 1024**2, 100, 48)
_CONTAINER_LIMITS = {'postgres': _Limits(256 * 1024**2, 250, 128),
    'redis': _Limits(128 * 1024**2, 125, 64), 'app': _Limits(512 * 1024**2, 1000, 256),
    'probe': _Limits(128 * 1024**2, 250, 64)}


def _validate_controller_envelope(value):
    _require(type(value) is _Limits and value == CONTROLLER_LIMIT
             and all(type(v) is int for v in vars(value).values()), 'controller_identity')


@dataclass(frozen=True, repr=False)
class FixedControllerPolicy:
    """Root caller's immutable NativeContext; contains no executable/unit knobs."""
    context: d.NativeContext


class _Capability:
    __slots__ = ()

    def __init__(self):
        raise SystemdError('ownership_unknown')


class ControllerHandle(_Capability):
    __slots__ = ()


class RunReservation(_Capability):
    __slots__ = ()


class EngineHandle(_Capability):
    __slots__ = ()


class SystemdSliceParent(_Capability):
    __slots__ = ()


class ExactContainerIdentity(_Capability):
    __slots__ = ()


@dataclass(frozen=True, repr=False)
class ResourceCleanupObservation:
    complete: bool
    first_failure: str | None


@dataclass(frozen=True, repr=False)
class ProtocolReservationView:
    root_fd: int
    run: str
    rendezvous: str
    started: float
    deadline: float


@dataclass(frozen=True, repr=False)
class PeerView:
    pid: int
    start: int
    invocation: str
    cgroup: tuple


def _json(value):
    return json.dumps(value, sort_keys=True, separators=(',', ':'), allow_nan=False).encode()


def _identity(fd, directory=False):
    return d._metadata(os.fstat(fd), directory)


@dataclass(frozen=True, repr=False)
class _UnitSpec:
    name: str
    kind: str
    limits: _Limits
    properties: tuple


@dataclass(frozen=True, repr=False)
class _UnitObservation:
    name: str
    invocation: str
    control_group: str
    pid: int
    start: int
    spec: _UnitSpec
    loaded: bool
    active: bool


def _slice_spec(token):
    return _UnitSpec('siftocoolify' + token + '.slice', 'slice', WORKLOAD_LIMIT, ())


def _engine_spec(token, rendezvous):
    _require(all(type(v) is str and re.fullmatch('[a-f0-9]{32}', v)
                 for v in (token, rendezvous)), 'bootstrap_invalid')
    data = _json({'version': 1, 'run': token, 'rendezvous': rendezvous})
    _require(len(data) <= 256 and data.isascii(), 'bootstrap_invalid')
    return _UnitSpec('siftocoolify' + token + 'engine.service', 'engine', ENGINE_PREPARATION,
        (('Type', 'exec'), ('Slice', _slice_spec(token).name),
         ('ExecStart', ('/usr/bin/python3', '-I', '-B',
                        '/opt/sifto-maintenance/coolify_native_engine.py')),
         ('StandardInput', 'data'), ('StandardInputData', data),
         ('StandardOutput', 'null'), ('StandardError', 'null'), ('Restart', 'no'),
         ('Delegate', False), ('TimeoutStopUSec', 30000000), ('RuntimeMaxUSec', 2400000000)))


def _limit_properties(limits):
    _require(type(limits) is _Limits and limits in (CONTROLLER_LIMIT, WORKLOAD_LIMIT,
        ENGINE_PREPARATION, ENGINE_STEADY), 'resource_isolation')
    return (('MemoryMax', limits.memory), ('MemorySwapMax', 0),
            ('CPUQuotaPerSecUSec', limits.cpu * 1000), ('CPUQuotaPeriodUSec', 100000),
            ('TasksMax', limits.tasks), ('MemoryAccounting', True),
            ('CPUAccounting', True), ('TasksAccounting', True))


def _property_argv(name, value):
    if type(value) is bool:
        return [name, 'b', 'true' if value else 'false']
    if type(value) is int:
        return [name, 't', str(value)]
    if type(value) is str:
        return [name, 's', value]
    if name == 'StandardInputData' and type(value) is bytes:
        return [name, 'ay', str(len(value)), *(str(v) for v in value)]
    if name == 'ExecStart' and type(value) is tuple:
        return [name, 'a(sasb)', '1', value[0], str(len(value)), *value, 'false']
    raise SystemdError('context_policy')


def _manager_argv(method, signature, *values):
    return ['/usr/bin/busctl', '--system', '--json=short', '--timeout=30s', 'call',
            'org.freedesktop.systemd1', '/org/freedesktop/systemd1',
            'org.freedesktop.systemd1.Manager', method, signature, *values]


def _start_argv(spec):
    properties = (*_limit_properties(spec.limits), *spec.properties)
    args = _manager_argv('StartTransientUnit', 'ssa(sv)a(sa(sv))',
                         spec.name, 'fail', str(len(properties)))
    for name, value in properties:
        args.extend(_property_argv(name, value))
    return [*args, '0']


def _decode_document(raw, signature):
    try:
        _require(type(raw) is bytes and len(raw) <= 65536, 'manager_invalid')
        def pairs(items):
            result = {}
            for key, value in items:
                _require(key not in result, 'manager_invalid')
                result[key] = value
            return result
        value = json.loads(raw, object_pairs_hook=pairs,
                           parse_constant=lambda _: (_ for _ in ()).throw(SystemdError('manager_invalid')))
        _require(type(value) is dict and set(value) == {'type', 'data'}
                 and value['type'] == signature, 'manager_invalid')
        return value['data']
    except SystemdError:
        raise
    except BaseException:
        raise SystemdError('manager_invalid') from None


def _decode_method(raw, signature):
    value = _decode_document(raw, signature)
    _require(type(value) is list and len(value) == 1, 'manager_invalid')
    return _decode_property(_json({'type': signature, 'data': value[0]}), signature)


def _decode_property(raw, signature):
    try:
        # get-property transforms the variant directly; call transforms a
        # complete message and has one extra array level. Never infer a shape.
        result = _decode_document(raw, signature)
        if signature in ('s', 'o'):
            _require(type(result) is str and len(result) <= 4096 and '\x00' not in result,
                     'manager_invalid')
        elif signature in ('u', 't'):
            _require(type(result) is int and 0 <= result <= (2**(32 if signature == 'u' else 64) - 1),
                     'manager_invalid')
        elif signature == 'b':
            _require(type(result) is bool, 'manager_invalid')
        elif signature == 'ay':
            _require(type(result) is list and len(result) <= 256
                     and all(type(v) is int and 0 <= v < 256 for v in result), 'manager_invalid')
            result = bytes(result)
        elif signature == 'a(sasbttttuii)':
            _require(type(result) is list and len(result) == 1, 'manager_invalid')
            item = result[0]
            _require(type(item) is list and len(item) == 10 and type(item[0]) is str
                     and type(item[1]) is list and 0 < len(item[1]) <= 16
                     and all(type(v) is str and len(v) <= 4096 for v in item[1])
                     and item[2] is False and all(type(v) is int and 0 <= v < 2**64
                     for v in item[3:7]) and type(item[7]) is int and 0 <= item[7] < 2**32
                     and all(type(v) is int and -(2**31) <= v < 2**31 for v in item[8:])
                     and item[0] == item[1][0], 'manager_invalid')
            result = tuple(item[1])
        elif signature == 'a(ssssssouso)':
            _require(type(result) is list and len(result) <= 1, 'manager_invalid')
            for row in result:
                _require(type(row) is list and len(row) == 10
                         and all(type(row[i]) is str and len(row[i]) <= 4096
                                 for i in (0, 1, 2, 3, 4, 5, 6, 8, 9))
                         and type(row[7]) is int and 0 <= row[7] < 2**32, 'manager_invalid')
        else:
            _require(signature == 'a(sasb)' and type(result) is list and len(result) == 1,
                     'manager_invalid')
            item = result[0]
            _require(type(item) is list and len(item) == 3 and type(item[0]) is str
                     and type(item[1]) is list and all(type(v) is str for v in item[1])
                     and item[2] is False, 'manager_invalid')
            result = (item[0], tuple(item[1]), False)
        return result
    except SystemdError:
        raise
    except BaseException:
        raise SystemdError('manager_invalid') from None


def _spec_document(spec):
    return {'name': spec.name, 'kind': spec.kind, 'limits': vars(spec.limits),
            'properties': [[name, {'hex': value.hex()} if type(value) is bytes else value]
                           for name, value in spec.properties]}


def _validate_ancestor_capacity(values, *, controller_only=False):
    try:
        _require(type(values) is dict and set(values) == {'memory.max', 'pids.max', 'cpu.max',
                 'cgroup.controllers', 'cgroup.subtree_control'}
                 and all(type(v) is str and len(v) <= 4096 for v in values.values()), 'resource_capacity')
        for name in ('cgroup.controllers', 'cgroup.subtree_control'):
            _require({'cpu', 'memory', 'pids'} <= set(values[name].split()), 'resource_capacity')
        minimum_memory, minimum_tasks, minimum_cpu = ((CONTROLLER_MEMORY, CONTROLLER_TASKS, CONTROLLER_CPU)
            if controller_only else (1280 * 1024**2, 576, 1750))
        for name, minimum in (('memory.max', minimum_memory), ('pids.max', minimum_tasks)):
            _require(values[name] == 'max' or re.fullmatch('[0-9]+', values[name])
                     and int(values[name]) >= minimum, 'resource_capacity')
        quota, period = values['cpu.max'].split()
        _require(re.fullmatch('[1-9][0-9]*', period) and (quota == 'max'
                 or re.fullmatch('[1-9][0-9]*', quota)
                 and int(quota) * 1000 >= minimum_cpu * int(period)), 'resource_capacity')
    except (ValueError, TypeError):
        raise SystemdError('resource_capacity') from None


def _authenticate_cgroup_mount(raw):
    """Only a complete cgroup2 hierarchy mount earns true-root semantics."""
    _require(type(raw) is bytes and len(raw) <= 1024**2, 'resource_isolation')
    matches = []
    for line in raw.splitlines():
        fields = line.split()
        if len(fields) > 6 and fields[4] == b'/sys/fs/cgroup':
            _require(fields[3] == b'/' and b'-' in fields, 'resource_isolation')
            separator = fields.index(b'-')
            _require(separator >= 6 and len(fields) >= separator + 4
                     and fields[separator + 1] == b'cgroup2', 'resource_isolation')
            matches.append(tuple(value.decode('ascii') for value in
                (fields[0], fields[2], fields[3], fields[4], fields[separator + 1])))
    _require(len(matches) == 1, 'resource_isolation')
    return matches[0]


class _NativeManager:
    """Private fixed-operation busctl transport; never a caller adapter API."""
    def __init__(self, owner, output_fd):
        self._owner, self._output_fd = owner, output_fd
        self._specs = {}
        self._count = self._total = 0
        self._until = 0

    def _begin(self):
        self._until = min(time.monotonic() + 30, self._owner._operation_deadline())

    def _transport(self, argv):
        owner = self._owner
        owner._guard_controller_local()
        owner._guard_tools()
        owner._deadline(cleanup=owner._cleaning)
        _require(time.monotonic() < self._until, 'command_timeout')
        self._count += 1
        _require(self._count <= 99999999 and self._total <= 16 * 1024**2 - 2 * 65536,
                 'output_bound')
        # Execute exactly the anchored binary via its FD. No executable lookup,
        # shell or environment from the unit/gateway/caller participates.
        executable = owner._tools[Path('/usr/bin/busctl')]['fd']
        exact = ['/proc/self/fd/' + str(executable), *argv[1:]]
        prefix = 'command-' + str(self._count)
        failure = None
        out = b''
        try:
            try:
                code, out, err = d._supervise(exact, output_fd=self._output_fd,
                    prefix=prefix, pass_fds=(executable,), deadline=self._until, limit=65536,
                    attach=owner._attach_helper)
            except BaseException as error:
                # Only the exact native supervisor's private reaping witness
                # proves its failed command left no helper processes behind.
                if not (type(error) is d.DockerError
                        and getattr(error, 'cleanup_complete', False) is True
                        and getattr(error, '_process_cleanup', None) is d._PROCESS_CLEANUP):
                    owner._helper_cleanup_unresolved = True
                raise
            self._total += len(out) + len(err)
            _require(self._total <= 16 * 1024**2, 'output_bound')
            _require(code == 0, 'command_failed')
        except BaseException as error:
            failure = ('interrupted' if isinstance(error, (KeyboardInterrupt, SystemExit))
                       else _static_category(error))
        # Include bytes written on failed commands. Diagnostic accounting must
        # not overwrite the primary command failure, and cannot silently pass.
        try:
            size = 0
            for suffix in ('stdout', 'stderr'):
                try:
                    size += os.stat(prefix + '.' + suffix, dir_fd=self._output_fd,
                                    follow_symlinks=False).st_size
                except FileNotFoundError:
                    pass
            self._total = max(self._total, getattr(self, '_previous_total', 0) + size)
            self._previous_total = self._total
        except BaseException as error:
            self._total = 16 * 1024**2  # Unknown accounting cannot grant further output allowance.
            if failure is None:
                failure = _static_category(error)
        if failure is not None:
            raise SystemdError(failure) from None
        return out

    def _get(self, path, interface, name, signature):
        raw = self._transport(['/usr/bin/busctl', '--system', '--json=short', '--timeout=30s',
            'get-property', 'org.freedesktop.systemd1', path,
            'org.freedesktop.systemd1.' + interface, name])
        return _decode_property(raw, signature)

    def _unit(self, name):
        rows = _decode_method(self._transport(_manager_argv('ListUnitsByNames', 'as', '1', name)),
                                'a(ssssssouso)')
        if not rows:
            return None
        row = rows[0]
        _require(row[0] == name and re.fullmatch('/org/freedesktop/systemd1/unit/[A-Za-z0-9_]+', row[6])
                 and row[2] in ('loaded', 'not-found')
                 and row[3] in ('active', 'inactive', 'activating', 'deactivating', 'failed', 'reloading'),
                 'manager_invalid')
        return None if row[2] == 'not-found' else row[6]

    def _limits(self, path, interface):
        observed = {}
        for name, _ in _limit_properties(CONTROLLER_LIMIT):
            observed[name] = self._get(path, interface, name, 'b' if name.endswith('Accounting') else 't')
        _require(observed['MemorySwapMax'] == 0 and observed['CPUQuotaPeriodUSec'] == 100000
                 and all(observed[name] is True for name in
                         ('MemoryAccounting', 'CPUAccounting', 'TasksAccounting'))
                 and observed['CPUQuotaPerSecUSec'] % 1000 == 0, 'resource_isolation')
        return _Limits(observed['MemoryMax'], observed['CPUQuotaPerSecUSec'] // 1000,
                       observed['TasksMax'])

    def _controller(self):
        self._begin()
        path = self._unit(CONTROLLER_UNIT)
        _require(path is not None, 'controller_identity')
        invocation = self._get(path, 'Unit', 'InvocationID', 'ay')
        _require(len(invocation) == 16, 'controller_identity')
        identity = (self._get(path, 'Service', 'MainPID', 'u'),
            self._get(path, 'Service', 'ExecMainStartTimestampMonotonic', 't'), invocation.hex(),
            self._get(path, 'Service', 'ControlGroup', 's'))
        _validate_controller_envelope(self._limits(path, 'Service'))
        _require(self._get(path, 'Unit', 'InvocationID', 'ay') == invocation,
                 'controller_identity')
        return identity

    def _start(self, spec):
        self._begin()
        _require(spec.name not in self._specs, 'ownership_unknown')
        self._specs[spec.name] = spec
        return _decode_method(self._transport(_start_argv(spec)), 'o')

    def _observe(self, name):
        self._begin()
        _require(name in self._specs, 'ownership_unknown')
        path = self._unit(name)
        if path is None:
            return None
        expected = self._specs[name]
        interface = 'Slice' if expected.kind == 'slice' else 'Service'
        invocation = self._get(path, 'Unit', 'InvocationID', 'ay')
        _require(len(invocation) == 16, 'manager_invalid')
        group = self._get(path, interface, 'ControlGroup', 's')
        loaded = self._get(path, 'Unit', 'LoadState', 's')
        active = self._get(path, 'Unit', 'ActiveState', 's')
        _require(loaded in ('loaded', 'not-found') and active in
            ('active', 'inactive', 'activating', 'deactivating', 'failed', 'reloading'), 'manager_invalid')
        limits = self._limits(path, interface)
        properties = []
        for prop, expected_value in expected.properties:
            signature = ('a(sasbttttuii)' if prop == 'ExecStart' else 'ay' if prop == 'StandardInputData'
                else 'b' if type(expected_value) is bool else 't' if type(expected_value) is int else 's')
            properties.append((prop, self._get(path, interface, prop, signature)))
        pid = self._get(path, interface, 'MainPID', 'u') if expected.kind == 'engine' else 0
        start = self._get(path, interface, 'ExecMainStartTimestampMonotonic', 't') if pid else 0
        _require(self._get(path, 'Unit', 'InvocationID', 'ay') == invocation, 'unit_identity')
        return _UnitObservation(name, invocation.hex(), group, pid, start,
            _UnitSpec(name, expected.kind, limits, tuple(properties)), loaded == 'loaded', active == 'active')

    def _set_limits(self, name, limits):
        self._begin()
        _require(name in self._specs and self._specs[name].kind == 'engine', 'ownership_unknown')
        props = _limit_properties(limits)
        args = _manager_argv('SetUnitProperties', 'sba(sv)', name, 'true', str(len(props)))
        for prop, value in props:
            args.extend(_property_argv(prop, value))
        raw = self._transport(args)
        _require(raw.strip() in (b'', b'{"type":"","data":[]}'), 'manager_invalid')
        old = self._specs[name]
        self._specs[name] = _UnitSpec(old.name, old.kind, limits, old.properties)

    def _stop(self, name):
        self._begin()
        _require(name in self._specs, 'ownership_unknown')
        job = _decode_method(self._transport(_manager_argv('StopUnit', 'ss', name, 'fail')), 'o')
        _require(re.fullmatch('/org/freedesktop/systemd1/job/[1-9][0-9]*', job), 'manager_invalid')


class SystemdResources:
    def __init__(self):
        self._controllers, self._runs, self._engines, self._parents, self._containers = {}, {}, {}, {}, {}
        self._fds = {}
        self._closed = False
        self._context_guard = None
        self._started = time.monotonic()
        self._manager = None
        self._cleaning = False
        self._cleanup_deadline = None
        self._tools = {}
        self._sessions = {}
        self._helper_cleanup_unresolved = False

    @_closed
    def verify_controller(self, policy):
        _require(not self._closed and not self._controllers
                 and type(policy) is FixedControllerPolicy
                 and type(policy.context) is d.NativeContext, 'context_policy')
        d._budget(policy.context)
        for p in (policy.context.input_policy, policy.context.staging_policy):
            _require(type(p) is FilePolicy and type(p.owner_uid) is int and p.owner_uid == 0,
                     'context_policy')
        self._native_preflight(policy)
        handle = object.__new__(ControllerHandle)
        self._controllers[handle] = self._controller_identity
        return handle

    def _native_preflight(self, policy):
        _require(platform.system() == 'Linux' and platform.machine() in ('aarch64', 'arm64')
                 and os.geteuid() == 0, 'context_policy')
        # N2S-4 must install/authenticate this source in the closed native tree.
        expected = Path('/opt/sifto-maintenance/maintenance/coolify_systemd.py')
        _require(Path(__file__) == expected and any(path == expected for path, _
                 in policy.context.source_sha256), 'source_closure')
        self._context_guard = d.OwnedDocker(policy.context, adapter=object())
        self._stage_fd = self._context_guard._stage_fd
        self._native_controller_preflight()

    def _guard_controller_local(self):
        self._native_controller_guard()

    @staticmethod
    def _read_membership(pid):
        raw = Path('/proc/' + str(pid) + '/cgroup').read_bytes()
        _require(len(raw) <= 4096 and re.fullmatch(rb'0::/[^\x00\r\n]*\n?', raw),
                 'controller_identity')
        return raw.decode().strip()[3:]

    @staticmethod
    def _read_at(fd, name):
        selected = os.open(name, os.O_RDONLY | os.O_NOFOLLOW | os.O_CLOEXEC, dir_fd=fd)
        try:
            data = os.read(selected, 4097)
            _require(len(data) <= 4096, 'resource_isolation')
            return data.decode().strip()
        finally:
            os.close(selected)

    def _native_controller_preflight(self):
        # Authenticate current placement before a helper or private stream
        # directory is created. Nothing migrates this process into a new group.
        self._cgroup_root = Path('/sys/fs/cgroup')
        pid = os.getpid()
        _require(self._read_membership(pid) == CONTROLLER_GROUP, 'controller_identity')
        start = d._start_token(pid)
        _require(start.isdigit(), 'controller_identity')
        self._cgroup_mount = _authenticate_cgroup_mount(self._mount_observation())
        self._controller_cgroup = self._open_cgroup(CONTROLLER_GROUP)
        self._local_controller = (pid, start)
        self._native_controller_guard()
        self._ancestor_guards = []
        current = self._controller_cgroup['path'].parent
        while current.is_relative_to(self._cgroup_root):
            fd, parents = d._protected_open(current, os.O_RDONLY | os.O_DIRECTORY,
                                            category='resource_isolation')
            self._keep(fd)
            anchor = {'fd': fd, 'path': current, 'identity': _identity(fd, True), 'parents': parents}
            self._ancestor_guards.append(anchor)
            self._guard_ancestor(anchor)
            if current == self._cgroup_root:
                break
            current = current.parent
        self._native_capacity()
        for path in (Path('/usr/bin/busctl'), Path('/usr/bin/systemctl')):
            fd, parents = d._protected_open(path, os.O_RDONLY, category='context_policy')
            self._keep(fd)
            info = os.fstat(fd)
            _require(stat.S_ISREG(info.st_mode) and info.st_uid == 0
                     and not info.st_mode & 0o022 and info.st_mode & 0o111,
                     'context_policy')
            self._tools[path] = {'fd': fd, 'identity': _identity(fd), 'parents': parents}
        root = Path('/var/lib/sifto-maintenance/native-systemd-controller')
        fd, parents = d._protected_open(root, os.O_RDONLY | os.O_DIRECTORY, category='context_policy')
        self._keep(fd)
        _require(os.fstat(fd).st_uid == 0 and stat.S_IMODE(os.fstat(fd).st_mode) == 0o700,
                 'context_policy')
        token = secrets.token_hex(16)
        os.mkdir(token, 0o700, dir_fd=fd)
        child = self._keep(os.open(token, os.O_RDONLY | os.O_DIRECTORY | os.O_NOFOLLOW
                                   | os.O_CLOEXEC, dir_fd=fd))
        self._controller_streams = {'fd': child, 'identity': _identity(child, True),
            'path': root / token, 'parents': (*parents, (root, _identity(fd, True)))}
        self._controller_stream_owner = self._exclusive_file(child, '.owner', token.encode())
        os.fsync(child)
        os.fsync(fd)
        self._manager = _NativeManager(self, child)
        manager = self._manager._controller()
        _require(manager[0] == pid and manager[1] > 0 and manager[3] == CONTROLLER_GROUP,
                 'controller_identity')
        self._controller_identity = (pid, start, *manager[1:])

    def _native_capacity(self):
        available, free = self._context_guard._capacity_observations()
        _require(available >= 1280 * 1024**2 + 1024**3 and free >= 4 * 1024**3,
                 'resource_capacity')

    @staticmethod
    def _mount_observation():
        return Path('/proc/self/mountinfo').read_bytes()

    def _guard_ancestor(self, anchor):
        _require(self._cgroup_mount == _authenticate_cgroup_mount(self._mount_observation()),
                 'resource_isolation')
        _require(_identity(anchor['fd'], True) == anchor['identity']
                 and d._metadata(anchor['path'].lstat(), True) == anchor['identity'],
                 'resource_isolation')
        try:
            values = {name: self._read_at(anchor['fd'], name) for name in
                ('cgroup.controllers', 'cgroup.subtree_control')}
            if anchor['path'] == self._cgroup_root:
                # The authenticated cgroup2 mount root has no resource limit
                # interface files. Its capacity is checked from meminfo/statvfs.
                _require(all({'cpu', 'memory', 'pids'} <= set(value.split())
                             for value in values.values()), 'resource_capacity')
                return
            values.update({name: self._read_at(anchor['fd'], name) for name in
                           ('memory.max', 'pids.max', 'cpu.max')})
            _validate_ancestor_capacity(values, controller_only=True)
        except OSError:
            raise SystemdError('resource_capacity') from None

    def _native_controller_guard(self):
        pid, start = self._local_controller
        _require(os.geteuid() == 0 and os.getpid() == pid and d._start_token(pid) == start
                 and self._read_membership(pid) == CONTROLLER_GROUP, 'controller_identity')
        self._guard_cgroup(self._controller_cgroup, CONTROLLER_LIMIT)
        _require(not self._children(self._controller_cgroup), 'controller_identity')
        pids = self._read_at(self._controller_cgroup['fd'], 'cgroup.procs').split()
        _require(len(pids) <= 16 and str(pid) in pids
                 and all(re.fullmatch('[1-9][0-9]*', value) for value in pids), 'controller_identity')
        for value in pids:
            candidate = int(value)
            _require(candidate == pid or candidate in self._sessions
                     and d._start_token(candidate) == self._sessions[candidate], 'controller_identity')
        for anchor in getattr(self, '_ancestor_guards', ()):
            self._guard_ancestor(anchor)

    def _controller_baseline(self):
        # Observe the original controller without starting a fresh observer.
        # Historical registered sessions are not evidence that helpers exited.
        _require(not self._helper_cleanup_unresolved, 'cleanup_incomplete')
        self._guard_controller_local()
        self._guard_cgroup(self._controller_cgroup, CONTROLLER_LIMIT)
        _require(not self._children(self._controller_cgroup)
                 and self._read_at(self._controller_cgroup['fd'], 'cgroup.procs').split()
                 == [str(os.getpid())], 'cleanup_incomplete')

    def _guard_tools(self):
        for path, anchor in self._tools.items():
            _require(_identity(anchor['fd']) == anchor['identity']
                     and d._metadata(path.lstat()) == anchor['identity'], 'context_policy')
            for parent, identity in anchor['parents']:
                _require(d._metadata(parent.lstat(), True) == identity, 'context_policy')
        streams = self._controller_streams
        _require(_identity(streams['fd'], True) == streams['identity']
                 and d._metadata(streams['path'].lstat(), True) == streams['identity'], 'stage_identity')
        for path, identity in streams['parents']:
            _require(d._metadata(path.lstat(), True) == identity, 'stage_identity')
        self._guard_file(streams['fd'], '.owner', self._controller_stream_owner)

    def _attach_helper(self, pid, start):
        # _supervise has forked an exact unreaped session leader but has not yet
        # allowed the busctl child to exec. It inherited the controller limits.
        _require(d._start_token(pid) == start and self._read_membership(pid) == CONTROLLER_GROUP
                 and os.getsid(pid) == pid and os.getpgid(pid) == pid,
                 'controller_identity')
        self._sessions[pid] = start
        self._guard_controller_local()
        self._guard_tools()

    def _operation_deadline(self):
        if self._cleaning:
            return min(self._started + 2400, self._cleanup_deadline)
        return self._started + 2280

    def _engine_process_identity(self, observation):
        _require(self._read_membership(observation.pid) == observation.control_group,
                 'unit_identity')
        token = d._start_token(observation.pid)
        _require(token.isdigit(), 'unit_identity')
        return token.decode()

    def _guard_controller(self, handle):
        _require(not self._closed and type(handle) is ControllerHandle
                 and handle in self._controllers, 'controller_identity')
        self._context_guard._guard_context()
        self._guard_controller_local()
        _require(self._controllers[handle] == self._controller_identity, 'controller_identity')
        if type(self._manager) is _NativeManager:
            manager = self._manager._controller()
            _require(self._controller_identity[0] == manager[0]
                     and self._controller_identity[2:] == manager[1:], 'controller_identity')

    def _keep(self, fd):
        info = os.fstat(fd)
        self._fds[fd] = (info.st_dev, info.st_ino)
        return fd

    def _latch_public(self, capability, category):
        # Registration lookup performs no I/O and grants no capability. It runs
        # even when a public operation fails before reaching its full guards.
        reservation = capability if type(capability) is RunReservation else (
            self._engines.get(capability) if type(capability) is EngineHandle else None)
        if reservation in self._runs:
            self._fail(self._runs[reservation], category)

    def _exclusive_file(self, parent, name, content):
        fd = self._keep(os.open(name, os.O_RDWR | os.O_CREAT | os.O_EXCL | os.O_NOFOLLOW
                                | os.O_CLOEXEC, 0o600, dir_fd=parent))
        self._write(fd, content)
        return {'fd': fd, 'identity': _identity(fd), 'bytes': content}

    @staticmethod
    def _write(fd, content):
        offset = 0
        while offset < len(content):
            size = os.write(fd, content[offset:])
            _require(size > 0, 'journal_invalid')
            offset += size
        os.fsync(fd)

    @_closed
    def reserve_run(self, handle):
        _require(not self._closed and type(handle) is ControllerHandle
                 and handle in self._controllers, 'controller_identity')
        identity = self._controllers[handle]
        # One invocation gets one admission for its entire lifecycle. Neither
        # another instance nor failed/complete cleanup can reset the claim.
        with _ADMISSION_LOCK:
            _require(identity not in _CONTROLLER_ADMISSIONS, 'resource_capacity')
            _CONTROLLER_ADMISSIONS.add(identity)
        self._guard_controller(handle)
        self._deadline()
        token = secrets.token_hex(16)
        _require(type(token) is str and re.fullmatch('[a-f0-9]{32}', token), 'stage_identity')
        try:
            os.mkdir(token, 0o700, dir_fd=self._stage_fd)
        except FileExistsError:
            raise SystemdError('stage_collision') from None
        try:
            fd = self._keep(os.open(token, os.O_RDONLY | os.O_DIRECTORY | os.O_NOFOLLOW
                                    | os.O_CLOEXEC, dir_fd=self._stage_fd))
            record = {'fd': fd, 'token': token, 'identity': _identity(fd, True),
                      'rendezvous': secrets.token_hex(16),
                      'controller': handle, 'phases': {}, 'units': {}, 'failure': None,
                      'cleaned': False, 'parent_issued': False, 'mode': 'reserved'}
            record['owner'] = self._exclusive_file(fd, '.owner', token.encode())
            record['journal'] = self._exclusive_file(fd, 'journal', b'')
            record['ledger'] = self._exclusive_file(fd, 'controller-ledger', b'')
            for name in ('old', 'fresh'):
                os.mkdir(name, 0o700, dir_fd=fd)
                child = self._keep(os.open(name, os.O_RDONLY | os.O_DIRECTORY | os.O_NOFOLLOW
                                          | os.O_CLOEXEC, dir_fd=fd))
                record['phases'][name] = {'fd': child, 'identity': _identity(child, True),
                    'owner': self._exclusive_file(child, '.owner', (token + ':' + name).encode())}
                os.fsync(child)
            os.fsync(fd)
            os.fsync(self._stage_fd)
            reservation = object.__new__(RunReservation)
            self._runs[reservation] = record
            return reservation
        except BaseException:
            # Preserve partial allocation. Never remove a file introduced by
            # another actor, follow a replacement, or adopt a collided nonce.
            os.fsync(self._stage_fd)
            raise SystemdError('stage_identity') from None

    def _guard_file(self, parent, name, record):
        if record.get('transferred') is True:
            # The engine owns append bytes after durable transfer. The
            # controller continues to authenticate the actual file object.
            current = os.fstat(record['fd'])
            observed = os.stat(name, dir_fd=parent, follow_symlinks=False)
            stable = lambda info: (info.st_dev, info.st_ino, info.st_uid, info.st_gid,
                                    info.st_mode, info.st_nlink)
            _require(stable(current) == stable(observed) == record['stable'], 'stage_identity')
            return
        _require(_identity(record['fd']) == record['identity']
                 and d._metadata(os.stat(name, dir_fd=parent, follow_symlinks=False)) == record['identity']
                 and d._read(record['fd'], 16 * 1024**2) == record['bytes'], 'stage_identity')

    def _guard_run(self, reservation, *, cleanup=False):
        _require(type(reservation) is RunReservation and reservation in self._runs,
                 'ownership_unknown')
        record = self._runs[reservation]
        self._guard_controller(record['controller'])
        self._deadline(cleanup=cleanup)
        try:
            _require(_identity(record['fd'], True) == record['identity'] and
                d._metadata(os.stat(record['token'], dir_fd=self._stage_fd,
                                    follow_symlinks=False), True) == record['identity'], 'stage_identity')
            for name in ('owner', 'journal', 'ledger'):
                self._guard_file(record['fd'], {'owner': '.owner', 'journal': 'journal',
                    'ledger': 'controller-ledger'}[name], record[name])
            for name, child in record['phases'].items():
                _require(_identity(child['fd'], True) == child['identity'] and
                    d._metadata(os.stat(name, dir_fd=record['fd'], follow_symlinks=False), True)
                    == child['identity'], 'stage_identity')
                self._guard_file(child['fd'], '.owner', child['owner'])
        except OSError:
            raise SystemdError('stage_identity') from None
        _require(not record['cleaned'], 'ownership_unknown')
        if not cleanup:
            _require(record['failure'] is None and not self._cleaning, 'ownership_unknown')
        return record

    @_closed
    def protocol_reservation_view(self, reservation):
        record = self._guard_run(reservation)
        return ProtocolReservationView(record['fd'], record['token'], record['rendezvous'],
                                       self._started, self._started + 2400)

    @_closed
    def issued_allocation(self, reservation):
        record = self._guard_run(reservation)
        return (record['fd'], record['phases']['old']['fd'], record['phases']['fresh']['fd'],
                record['journal']['fd'])

    @_closed
    def controller_peer_view(self, controller):
        self._guard_controller(controller)
        pid, start, _, invocation, group = self._controller_identity
        return PeerView(pid, int(start), invocation,
                        (group, *self._controller_cgroup['identity'][:2]))

    @_closed
    def engine_peer_view(self, engine):
        record = self._engine_record(engine)
        self._guard_units(record)
        unit = record['units']['engine']
        invocation, group, pid, start = unit['identity']
        return PeerView(pid, int(unit['process_token']), invocation,
                        (group, *unit['cgroup']['identity'][:2]))

    @_closed
    def transfer_engine_journal(self, reservation):
        record = self._guard_run(reservation)
        journal = record['journal']
        _require(not journal.get('transferred'), 'ownership_unknown')
        info = os.fstat(journal['fd'])
        stable = (info.st_dev, info.st_ino, info.st_uid, info.st_gid, info.st_mode, info.st_nlink)
        ledger = record['ledger']
        data = _json({'kind': 'ENGINE_JOURNAL_TRANSFER', 'run': record['token'],
                      'identity': stable}) + b'\n'
        self._guard_file(record['fd'], 'controller-ledger', ledger)
        os.lseek(ledger['fd'], 0, os.SEEK_END)
        self._write(ledger['fd'], data)
        ledger['bytes'] += data
        ledger['identity'] = _identity(ledger['fd'])
        os.fsync(record['fd'])
        os.fsync(self._stage_fd)
        journal.update(transferred=True, stable=stable)

    def _deadline(self, *, cleanup=False):
        _require(time.monotonic() < self._started + (2400 if cleanup else 2280), 'command_timeout')

    def _record(self, record, unit, state):
        ledger = record['ledger']
        self._guard_file(record['fd'], 'controller-ledger', ledger)
        spec = _spec_document(unit['spec'])
        row = {'name': unit['name'], 'state': state, 'spec': spec,
               'spec_sha256': hashlib.sha256(_json(spec)).hexdigest(),
               'identity': unit.get('identity')}
        data = _json(row) + b'\n'
        _require(len(ledger['bytes']) + len(data) <= 1024**2, 'journal_invalid')
        os.lseek(ledger['fd'], 0, os.SEEK_END)
        self._write(ledger['fd'], data)
        ledger['bytes'] += data
        ledger['identity'] = _identity(ledger['fd'])
        unit['state'] = state
        os.fsync(record['fd'])
        os.fsync(self._stage_fd)

    def _fail(self, record, category):
        if record['failure'] is None:
            record['failure'] = category

    def _unresolved(self, record, unit):
        unit['state'] = 'unresolved'
        try:
            self._record(record, unit, 'unresolved')
        except BaseException:
            # The primary static cause already belongs to this run. A failed
            # forensic append must not replace it or grant cleanup success.
            pass

    def _open_cgroup(self, group):
        _require(type(group) is str and group.startswith('/') and group != '/'
                 and re.fullmatch(r'/[A-Za-z0-9_.:/-]+', group) and '..' not in group.split('/'),
                 'unit_identity')
        path = self._cgroup_root / group.lstrip('/')
        fd, parents = d._protected_open(path, os.O_RDONLY | os.O_DIRECTORY,
                                        category='resource_isolation')
        self._keep(fd)
        _require(stat.S_ISDIR(os.fstat(fd).st_mode), 'resource_isolation')
        return {'fd': fd, 'path': path, 'identity': _identity(fd, True), 'parents': parents}

    def _guard_cgroup(self, cgroup, limits):
        try:
            _require(_identity(cgroup['fd'], True) == cgroup['identity']
                     and d._metadata(cgroup['path'].lstat(), True) == cgroup['identity'], 'unit_identity')
            for path, identity in cgroup['parents']:
                _require(d._metadata(path.lstat(), True) == identity, 'unit_identity')
            expected = {'memory.max': str(limits.memory), 'memory.swap.max': '0',
                        'cpu.max': str(limits.cpu * 100) + ' 100000', 'pids.max': str(limits.tasks)}
            for name, value in expected.items():
                fd = os.open(name, os.O_RDONLY | os.O_NOFOLLOW | os.O_CLOEXEC, dir_fd=cgroup['fd'])
                try:
                    observed = os.read(fd, 4097)
                    _require(len(observed) <= 4096 and observed.decode().strip() == value,
                             'resource_isolation')
                finally:
                    os.close(fd)
        except OSError:
            raise SystemdError('unit_identity') from None

    def _children(self, cgroup):
        # cgroupfs directories are manager-owned; only observe via retained FD.
        return {name for name in os.listdir(cgroup['fd'])
                if stat.S_ISDIR(os.stat(name, dir_fd=cgroup['fd'], follow_symlinks=False).st_mode)}

    @staticmethod
    def _observation_identity(observation):
        return [observation.invocation, observation.control_group, observation.pid, observation.start]

    def _validate_observation(self, observation, spec):
        _require(type(observation) is _UnitObservation and observation.name == spec.name
                 and type(observation.invocation) is str
                 and re.fullmatch('[a-f0-9]{32}', observation.invocation)
                 and type(observation.pid) is int and observation.pid >= 0
                 and type(observation.start) is int and observation.start >= 0
                 and type(observation.loaded) is bool and observation.loaded
                 and type(observation.active) is bool and observation.active
                 and observation.spec == spec, 'unit_identity')
        if spec.kind == 'engine':
            _require(observation.pid > 0 and observation.start > 0, 'unit_identity')

    def _start_owned(self, record, spec):
        _require(spec.kind not in record['units'], 'ownership_unknown')
        unit = {'name': spec.name, 'spec': spec, 'identity': None, 'state': 'intent'}
        record['units'][spec.kind] = unit
        self._record(record, unit, 'intent')
        # Once dispatch is attempted, a failure cannot distinguish "not created"
        # from a lost response. Never retry or adopt the unit by its name.
        try:
            self._guard_controller(record['controller'])
            job = self._manager._start(spec)
            _require(type(job) is str and re.fullmatch('/org/freedesktop/systemd1/job/[1-9][0-9]*', job),
                     'manager_invalid')
            until = min(self._started + 2280, time.monotonic() + 30)
            while True:
                self._guard_controller(record['controller'])
                observation = self._manager._observe(spec.name)
                if observation is not None and (spec.kind != 'engine' or observation.pid != 0):
                    break
                _require(time.monotonic() < until, 'command_timeout')
                time.sleep(0.01)
            self._validate_observation(observation, spec)
            cgroup = self._open_cgroup(observation.control_group)
            if spec.kind == 'slice':
                _require(cgroup['path'].parent == self._cgroup_root, 'resource_isolation')
            else:
                _require(cgroup['path'].parent == record['units']['slice']['cgroup']['path'],
                         'resource_isolation')
            self._guard_cgroup(cgroup, spec.limits)
            _require(not self._children(cgroup), 'resource_isolation')
            unit.update(identity=self._observation_identity(observation), cgroup=cgroup)
            if spec.kind == 'engine':
                unit['process_token'] = self._engine_process_identity(observation)
            self._record(record, unit, 'owned')
            return unit
        except BaseException as error:
            self._fail(record, _static_category(error))
            self._unresolved(record, unit)
            raise SystemdError(record['failure']) from None

    @_closed
    def launch_engine(self, reservation):
        record = self._guard_run(reservation)
        _require(record['mode'] == 'reserved' and not record['units'], 'ownership_unknown')
        self._start_owned(record, _slice_spec(record['token']))
        self._start_owned(record, _engine_spec(record['token'], record['rendezvous']))
        record['mode'] = 'preparation'
        handle = object.__new__(EngineHandle)
        self._engines[handle] = reservation
        return handle

    def _guard_units(self, record, *, allow_stopped=False):
        for kind, unit in record['units'].items():
            _require(unit['state'] == 'owned' or allow_stopped and unit['state'] == 'stopped',
                     'ownership_unknown')
            if unit['state'] == 'stopped':
                _require(self._manager._observe(unit['name']) is None
                         and not unit['cgroup']['path'].exists(), 'unit_identity')
                continue
            self._guard_controller(record['controller'])
            observation = self._manager._observe(unit['name'])
            self._validate_observation(observation, unit['spec'])
            _require(self._observation_identity(observation) == unit['identity'], 'unit_identity')
            if kind == 'engine':
                _require(self._engine_process_identity(observation) == unit['process_token'], 'unit_identity')
            self._guard_cgroup(unit['cgroup'], unit['spec'].limits)
            # The engine has no delegated descendants in N2S-1. Any unknown
            # child means a bulk stop could touch someone else's resource.
            expected = set()
            if kind == 'slice':
                for other in record['units'].values():
                    if other is not unit and other.get('cgroup') and other['state'] == 'owned':
                        path = other['cgroup']['path']
                        if path.parent == unit['cgroup']['path']:
                            expected.add(path.name)
            _require(self._children(unit['cgroup']) == expected, 'resource_isolation')

    def _engine_record(self, handle, *, cleanup=False):
        _require(type(handle) is EngineHandle and handle in self._engines, 'ownership_unknown')
        return self._guard_run(self._engines[handle], cleanup=cleanup)

    def _set_phase(self, handle, limits, mode):
        record = self._engine_record(handle)
        _require(not record['parent_issued'], 'resource_isolation')
        self._guard_units(record)
        unit = record['units']['engine']
        previous = unit['spec']
        unit['spec'] = _UnitSpec(previous.name, previous.kind, limits, previous.properties)
        self._record(record, unit, 'changing')
        try:
            self._guard_controller(record['controller'])
            self._manager._set_limits(unit['name'], limits)
            observation = self._manager._observe(unit['name'])
            self._validate_observation(observation, unit['spec'])
            _require(self._observation_identity(observation) == unit['identity'], 'unit_identity')
            self._guard_cgroup(unit['cgroup'], limits)
            self._record(record, unit, 'owned')
            record['mode'] = mode
        except BaseException as error:
            self._fail(record, _static_category(error))
            self._unresolved(record, unit)
            raise SystemdError(record['failure']) from None

    @_closed
    def set_preparation(self, handle):
        self._set_phase(handle, ENGINE_PREPARATION, 'preparation')

    @_closed
    def set_steady(self, handle):
        self._set_phase(handle, ENGINE_STEADY, 'steady')

    @_closed
    def docker_parent(self, reservation):
        record = self._guard_run(reservation)
        _require(record['mode'] == 'steady', 'resource_isolation')
        self._guard_units(record)
        parent = object.__new__(SystemdSliceParent)
        self._parents[parent] = reservation
        record['parent_issued'] = True
        return parent

    @_closed
    def verify_container(self, reservation, identity):
        self._guard_run(reservation)
        _require(type(identity) is ExactContainerIdentity and identity in self._containers
                 and self._containers[identity] == reservation, 'container_identity')
        # The closed native Docker identity bridge is deliberately unavailable
        # until N2S-3; no caller record or boolean can manufacture this evidence.
        raise SystemdError('container_identity')

    def _stop_unit(self, record, unit):
        self._guard_units(record, allow_stopped=True)
        self._guard_controller(record['controller'])
        self._record(record, unit, 'stopping')
        try:
            self._manager._stop(unit['name'])
            until = min(self._operation_deadline(), time.monotonic() + 30)
            while True:
                self._guard_controller(record['controller'])
                observation = self._manager._observe(unit['name'])
                present = unit['cgroup']['path'].exists()
                if observation is None and not present:
                    break
                if observation is not None:
                    _require(type(observation) is _UnitObservation
                             and observation.invocation == unit['identity'][0]
                             and (observation.control_group == unit['identity'][1]
                                  or observation.control_group == '' and not present
                                  and observation.pid == 0 and not observation.active)
                             and observation.spec == unit['spec']
                             and observation.pid in (0, unit['identity'][2])
                             and observation.start in (0, unit['identity'][3]), 'unit_identity')
                if present:
                    self._guard_cgroup(unit['cgroup'], unit['spec'].limits)
                    _require(not self._children(unit['cgroup']), 'resource_isolation')
                _require(time.monotonic() < until, 'cleanup_incomplete')
                time.sleep(0.01)
            self._record(record, unit, 'stopped')
        except BaseException as error:
            self._fail(record, _static_category(error))
            self._unresolved(record, unit)
            raise SystemdError(record['failure']) from None

    def _begin_cleanup(self):
        self._cleaning = True
        if self._cleanup_deadline is None:
            self._cleanup_deadline = min(self._started + 2400, time.monotonic() + 120)
        _require(time.monotonic() < self._cleanup_deadline, 'command_timeout')

    @_closed
    def stop_owned_engine(self, handle):
        _require(type(handle) is EngineHandle and handle in self._engines
                 and self._engines[handle] in self._runs, 'ownership_unknown')
        self._begin_cleanup()
        record = self._engine_record(handle, cleanup=True)
        _require(not record['parent_issued'], 'ownership_unknown')
        self._guard_units(record)
        self._stop_unit(record, record['units']['engine'])

    @_closed
    def finish_owned_run(self, reservation):
        _require(type(reservation) is RunReservation and reservation in self._runs,
                 'ownership_unknown')
        record = self._runs[reservation]
        try:
            self._begin_cleanup()
            self._guard_run(reservation, cleanup=True)
            _require(not record['parent_issued'], 'ownership_unknown')
            self._guard_units(record, allow_stopped=True)
            for kind in ('engine', 'slice'):
                unit = record['units'].get(kind)
                if unit is not None and unit['state'] != 'stopped':
                    self._stop_unit(record, unit)
            self._controller_baseline()
            record['cleaned'] = True
            return ResourceCleanupObservation(True, record['failure'])
        except BaseException as error:
            category = _static_category(error, 'cleanup_incomplete')
            self._fail(record, category)
            return ResourceCleanupObservation(False, record['failure'])

    def close(self):
        if self._closed:
            return
        self._closed = True
        for fd, identity in self._fds.items():
            try:
                info = os.fstat(fd)
                if (info.st_dev, info.st_ino) == identity:
                    os.close(fd)
            except OSError:
                pass
        self._fds.clear()
        if self._context_guard is not None:
            self._context_guard.close()
