"""Dormant source-owned Unix protocol. Native owner bridge is an N2S-4 gate.

No host dispatch imports this module. Bootstrap IDs route sessions; they confer
no authority. Received descriptors never confer OwnedDocker execution authority.
Kernel credentials protect against inherited connections, not hostile host root.
"""
from dataclasses import dataclass
from enum import Enum
import array
import fcntl
import json
import math
import os
from pathlib import Path, PosixPath
import re
import select
import socket
import stat
import struct
import time

from . import coolify_docker as d

_PACKET = 16384
_FD_LIMIT = 16
_RECORD_LIMIT = 256
_CRED = struct.Struct('3i')
_ALLOCATION = ('run', 'old', 'fresh', 'engine-journal')
_CAUSES = frozenset(('source_closure', 'ownership_unknown', 'endpoint_collision',
    'endpoint_identity', 'peer_identity', 'bootstrap_invalid', 'packet_invalid',
    'sequence_invalid', 'descriptor_invalid', 'deadline', 'disconnected',
    'state_invalid', 'record_limit', 'transport_failed', 'interrupted'))


class Phase(Enum):
    OLD = 'old'
    FRESH = 'fresh'


@dataclass(frozen=True, repr=False)
class _Peer:
    pid: int
    uid: int
    gid: int
    start: int
    invocation: str
    cgroup: tuple


@dataclass(frozen=True, repr=False)
class _Reservation:
    root_fd: int
    root_path: object
    run: str
    rendezvous: str
    started: float
    deadline: float


@dataclass(frozen=True, repr=False)
class _MaterialOwners:
    """Guarded source-owner evidence for fixed image accounts, bound to run/phase."""
    run: str
    phase: Phase
    pg: tuple
    redis: tuple


def _require(condition, category):
    if not condition:
        raise ProtocolError(category) from None

class ProtocolError(Exception):
    def __init__(self, category='transport_failed'):
        super().__init__(category if type(category) is str and category in _CAUSES
                         else 'transport_failed')


def _category(error):
    if type(error) in (KeyboardInterrupt, SystemExit):
        return 'interrupted'
    if type(error) is TimeoutError:
        return 'deadline'
    if type(error) is ProtocolError and len(error.args) == 1:
        value = error.args[0]
        if type(value) is str and value in _CAUSES:
            return value
    return 'transport_failed'


def _json(value):
    return json.dumps(value, sort_keys=True, separators=(',', ':'), allow_nan=False).encode('ascii')


def _decode(raw):
    def pairs(items):
        result = {}
        for key, value in items:
            _require(key not in result, 'packet_invalid')
            result[key] = value
        return result
    def invalid(_):
        raise ProtocolError('packet_invalid')
    _require(type(raw) is bytes and 0 < len(raw) <= _PACKET, 'packet_invalid')
    try:
        value = json.loads(raw.decode('utf-8'), object_pairs_hook=pairs,
                           parse_constant=invalid)
        _require(type(value) is dict and _json(value) == raw, 'packet_invalid')
        return value
    except ProtocolError:
        raise
    except BaseException:
        raise ProtocolError('packet_invalid') from None


def _id(info):
    return info.st_dev, info.st_ino, info.st_mode, info.st_uid, info.st_gid


def _fd_id(fd):
    info = os.fstat(fd)
    return info.st_dev, info.st_ino


def _close_fd(fd, identity):
    try:
        if _fd_id(fd) == identity:
            os.close(fd)
    except OSError:
        pass


def _dispatch(capability, expected, action, *args):
    _require(type(capability) in expected, 'ownership_unknown')
    try:
        core = object.__getattribute__(capability, '_core')
    except AttributeError:
        raise ProtocolError('ownership_unknown') from None
    _require(type(core) is _ProtocolCore, 'ownership_unknown')
    return core.invoke(capability, action, *args)


class _Capability:
    __slots__ = ('_core',)

    def __init__(self):
        raise ProtocolError('ownership_unknown')


class Listener(_Capability):
    __slots__ = ()

    def accept_authenticated(self, enginehandle):
        return _dispatch(self, (Listener,), 'accept', enginehandle)

    def close(self):
        return _dispatch(self, (Listener,), 'close')


class ControllerSession(_Capability):
    __slots__ = ()

    def send_allocation(self):
        return _dispatch(self, (ControllerSession,), 'allocation_send')

    def receive_material(self, phase):
        return _dispatch(self, (ControllerSession,), 'material_receive', phase)

    def close(self):
        return _dispatch(self, (ControllerSession,), 'close')


class EngineSession(_Capability):
    __slots__ = ()

    def receive_allocation(self):
        return _dispatch(self, (EngineSession,), 'allocation_receive')

    def register_material(self, phase, issued_material_bundle):
        return _dispatch(self, (EngineSession,), 'material_send', phase, issued_material_bundle)

    def close(self):
        return _dispatch(self, (EngineSession,), 'close')


class AllocationReceipt(_Capability):
    __slots__ = ()


class ReceivedAllocation(_Capability):
    __slots__ = ()

    def borrowed_fds(self):
        return _dispatch(self, (ReceivedAllocation,), 'borrow')


class RegisteredMaterial(_Capability):
    __slots__ = ()

    def borrowed_fds(self):
        return _dispatch(self, (RegisteredMaterial,), 'borrow')


class MaterialReceipt(_Capability):
    __slots__ = ()


class ControllerProtocol:
    def _source_owner(self, binding):
        raise ProtocolError('source_closure')

    def listen(self, ownerbinding, reservation):
        core = None
        try:
            owner = self._source_owner(ownerbinding)
            core = _ProtocolCore(owner, reservation)
            return core.listen()
        except BaseException as error:
            cause = _category(error)
            if core is not None:
                core.close(cause)
            raise ProtocolError(cause) from None


class EngineProtocol:
    def _source_owner(self, binding):
        raise ProtocolError('source_closure')

    def connect(self, verified_bootstrap_owner, strict_envelope):
        core = None
        try:
            owner, reservation = self._source_owner(verified_bootstrap_owner)
            core = _ProtocolCore(owner, reservation)
            value = _decode(strict_envelope)
            _require(len(strict_envelope) <= 256 and set(value) == {'version', 'run', 'rendezvous'}
                     and type(value['version']) is int and value['version'] == 1
                     and value['run'] == core.view.run
                     and value['rendezvous'] == core.view.rendezvous, 'bootstrap_invalid')
            return core.connect(verified_bootstrap_owner)
        except BaseException as error:
            cause = _category(error)
            if core is not None:
                core.close(cause)
            raise ProtocolError(cause) from None


class _ProtocolCore:
    """Private source-owner port. No adapter or endpoint constructor is public.

    N2S-4 supplies registered owner evidence via the guarded factory hook. The
    owner must authenticate the endpoint namespace, reservation, immutable unit
    invocation/cgroup evidence and issued material bundles on every guard call.
    This module never reaches into SystemdResources' private registries.
    """
    def __init__(self, owner, reservation):
        self.owner, self.reservation = owner, reservation
        self.caps, self.owned = {}, []
        self.sockets = []
        self.sock, self.listener = None, None
        self.first_failure = None
        self.closed, self.accepted = False, False
        self.tx, self.rx, self.records = 0, 0, 0
        self.io_deadline = None
        self.state, self.phase = 'new', 0
        self.peer, self.peer_role, self.peer_handle = None, None, None
        self.socket_identity = None
        self.owns_endpoint = False
        self.view = owner.guard_reservation(reservation)
        v = self.view
        _require(type(v) is _Reservation and type(v.root_fd) is int
                 and type(v.root_path) is PosixPath and v.root_path.is_absolute()
                 and all(type(token) is str and re.fullmatch('[a-f0-9]{32}', token)
                         for token in (v.run, v.rendezvous)), 'ownership_unknown')
        _require(all(type(t) in (int, float) and math.isfinite(t)
                     for t in (v.started, v.deadline))
                 and v.started <= time.monotonic() and v.started < v.deadline
                 and v.deadline <= v.started + 2400, 'deadline')
        self.deadline = min(v.started + 2280, v.deadline - 120)
        root = os.fstat(v.root_fd)
        _require(stat.S_ISDIR(root.st_mode) and root.st_uid == 0
                 and stat.S_IMODE(root.st_mode) == 0o700, 'endpoint_identity')
        self.root_identity = _id(root)
        self.anchors = []
        for path in (v.root_path, *v.root_path.parents):
            info = path.lstat()
            _require(stat.S_ISDIR(info.st_mode), 'endpoint_identity')
            self.anchors.append((path, _id(info)))
        self.path = v.root_path / (v.rendezvous + '.sock')
        _require(len(os.fsencode(self.path)) < 108, 'endpoint_identity')
        self.guard()
        # Retain our own root reference; the source owner's descriptor is borrowed.
        self.root_fd = os.dup(v.root_fd)
        self.owned.append((self.root_fd, self.root_identity[:2]))

    def issue(self, cls, descriptors=(), phase=None):
        cap = object.__new__(cls)
        cap._core = self
        self.caps[cap] = tuple(descriptors), phase
        return cap

    def guard(self):
        _require(not self.closed, 'ownership_unknown')
        v = self.owner.guard_reservation(self.reservation)
        _require(type(v) is _Reservation and type(v.root_fd) is int
                 and v.root_fd == self.view.root_fd and type(v.root_path) is PosixPath
                 and v.root_path == self.view.root_path
                 and type(v.run) is str and type(v.rendezvous) is str
                 and v.run == self.view.run and v.rendezvous == self.view.rendezvous
                 and type(v.started) in (int, float) and v.started == self.view.started,
                 'ownership_unknown')
        _require(type(v.deadline) in (int, float) and math.isfinite(v.deadline)
                 and v.deadline <= self.view.deadline, 'deadline')
        self.deadline = min(self.deadline, v.deadline - 120)
        _require(time.monotonic() < self.deadline, 'deadline')
        _require(self.io_deadline is None or time.monotonic() < self.io_deadline, 'deadline')
        _require(_id(os.fstat(v.root_fd)) == self.root_identity, 'endpoint_identity')
        if self.sock is not None and self.state in ('authenticated', 'allocated'):
            self.guard_connection()
        self.guard_namespace()
        if self.peer is not None:
            current = self.owner.guard_peer(self.peer_role, self.peer_handle)
            _require(type(current) is _Peer and current == self.peer, 'peer_identity')
            raw = Path('/proc/' + str(self.peer.pid) + '/stat').read_bytes()
            _require(len(raw) <= 4096 and int(raw[raw.rfind(b')') + 2:].split()[19])
                     == self.peer.start, 'peer_identity')

    def guard_connection(self):
        # poll observes kernel termination without consuming a record or
        # installing SCM_RIGHTS descriptors as recvmsg(MSG_PEEK) would do.
        identity = next(identity for sock, identity in self.sockets if sock is self.sock)
        _require(_fd_id(self.sock.fileno()) == identity, 'descriptor_invalid')
        termination = select.POLLHUP | select.POLLRDHUP | select.POLLERR | select.POLLNVAL
        observed = select.poll()
        observed.register(self.sock.fileno(), termination)
        _require(not any(flags & termination for _, flags in observed.poll(0)), 'disconnected')

    def guard_namespace(self):
        _require(all(_id(path.lstat()) == identity for path, identity in self.anchors)
                 and _id(os.fstat(self.view.root_fd)) == self.root_identity,
                 'endpoint_identity')
        if self.socket_identity is not None:
            try:
                observed = _id(self.path.lstat())
            except OSError:
                raise ProtocolError('endpoint_identity') from None
            _require(observed == self.socket_identity, 'endpoint_identity')

    def set_peer(self, role, handle):
        peer = self.owner.guard_peer(role, handle)
        _require(type(peer) is _Peer
                 and all(type(x) is int and x >= 0 for x in (peer.pid, peer.uid, peer.gid, peer.start))
                 and peer.pid > 0 and peer.start > 0 and type(peer.invocation) is str
                 and bool(peer.invocation) and type(peer.cgroup) is tuple and bool(peer.cgroup),
                 'peer_identity')
        self.peer, self.peer_role, self.peer_handle = peer, role, handle
        self.guard()

    def socket(self):
        sock = socket.socket(socket.AF_UNIX, socket.SOCK_SEQPACKET | socket.SOCK_CLOEXEC)
        try:
            sock.setsockopt(socket.SOL_SOCKET, socket.SO_PASSCRED, 1)
            self.sockets.append((sock, _fd_id(sock.fileno())))
            return sock
        except BaseException:
            sock.close()
            raise

    def listen(self):
        self.guard()
        _require(not self.path.exists() and not self.path.is_symlink(), 'endpoint_collision')
        self.listener = self.socket()
        try:
            self.listener.bind(os.fspath(self.path))
        except OSError:
            raise ProtocolError('endpoint_collision') from None
        self.socket_identity = _id(self.path.lstat())
        self.owns_endpoint = True
        self.guard_namespace()
        os.chmod(self.path, 0o600, follow_symlinks=False)
        self.socket_identity = _id(self.path.lstat())
        self.listener.listen(1)
        self.state = 'listening'
        return self.issue(Listener)

    def bounded(self, sock):
        self.guard()
        remaining = min(self.deadline, self.io_deadline) - time.monotonic()
        _require(remaining > 0, 'deadline')
        sock.settimeout(remaining)

    def authenticate_socket(self):
        credentials = self.sock.getsockopt(socket.SOL_SOCKET, socket.SO_PEERCRED, _CRED.size)
        _require(_CRED.unpack(credentials) == (self.peer.pid, self.peer.uid, self.peer.gid),
                 'peer_identity')
        self.guard()

    def accept(self, enginehandle):
        _require(self.state == 'listening' and not self.accepted, 'state_invalid')
        self.set_peer('engine', enginehandle)
        self.bounded(self.listener)
        self.sock, _ = self.listener.accept()
        self.sockets.append((self.sock, _fd_id(self.sock.fileno())))
        self.accepted = True
        self.sock.setsockopt(socket.SOL_SOCKET, socket.SO_PASSCRED, 1)
        self.authenticate_socket()
        hello, fds = self.receive('HELLO')
        self.send('HELLO_ACK', ack=hello['seq'])
        self.state = 'authenticated'
        return self.issue(ControllerSession)

    def connect(self, bootstrap):
        self.io_deadline = min(self.deadline, time.monotonic() + 30)
        self.set_peer('controller', bootstrap)
        try:
            endpoint = self.path.lstat()
        except OSError:
            raise ProtocolError('endpoint_identity') from None
        _require(stat.S_ISSOCK(endpoint.st_mode) and endpoint.st_uid == 0
                 and endpoint.st_gid == 0 and stat.S_IMODE(endpoint.st_mode) == 0o600,
                 'endpoint_identity')
        # Observer evidence never grants the engine unlink authority.
        self.socket_identity = _id(endpoint)
        self.sock = self.socket()
        self.bounded(self.sock)
        self.sock.connect(os.fspath(self.path))
        self.authenticate_socket()
        self.send('HELLO')
        self.receive('HELLO_ACK', ack=self.tx)
        self.state = 'authenticated'
        self.io_deadline = None
        return self.issue(EngineSession)

    def count(self):
        _require(self.records < _RECORD_LIMIT, 'record_limit')
        self.records += 1

    def send(self, kind, *, ack=None, phase=None, fds=(), metadata=None):
        self.bounded(self.sock)
        self.count()
        value = {'version': 1, 'kind': kind, 'run': self.view.run,
                 'rendezvous': self.view.rendezvous, 'seq': self.tx + 1}
        if ack is not None:
            value['ack'] = ack
        if phase is not None:
            value['phase'] = phase.value
        if metadata is not None:
            value['roles'] = metadata
        raw = _json(value)
        _require(len(raw) <= _PACKET and len(fds) <= _FD_LIMIT, 'packet_invalid')
        ancillary = [(socket.SOL_SOCKET, socket.SCM_RIGHTS, array.array('i', fds))] if fds else []
        _require(self.sock.sendmsg([raw], ancillary) == len(raw), 'transport_failed')
        self.guard()
        self.tx += 1

    def receive(self, kind, *, ack=None, phase=None, roles=()):
        received = []
        try:
            self.bounded(self.sock)
            self.count()
            raw, ancillary, flags, _ = self.sock.recvmsg(_PACKET,
                socket.CMSG_SPACE(_CRED.size) + socket.CMSG_SPACE(_FD_LIMIT * array.array('i').itemsize),
                socket.MSG_CMSG_CLOEXEC)
            # Inventory every delivered rights descriptor before parsing anything.
            malformed = False
            for level, typ, data in ancillary:
                if level == socket.SOL_SOCKET and typ == socket.SCM_RIGHTS:
                    ints = array.array('i')
                    size = ints.itemsize
                    ints.frombytes(data[:len(data) - len(data) % size])
                    received.extend(ints)
                    malformed |= bool(len(data) % size)
            _require(not flags & (socket.MSG_TRUNC | socket.MSG_CTRUNC)
                     and not malformed and len(received) <= _FD_LIMIT, 'packet_invalid')
            credentials = []
            rights = 0
            for level, typ, data in ancillary:
                _require(level == socket.SOL_SOCKET, 'packet_invalid')
                if typ == socket.SCM_CREDENTIALS:
                    _require(len(data) == _CRED.size, 'peer_identity')
                    credentials.append(_CRED.unpack(data))
                elif typ == socket.SCM_RIGHTS:
                    rights += 1
                else:
                    raise ProtocolError('packet_invalid')
            _require(credentials == [(self.peer.pid, self.peer.uid, self.peer.gid)], 'peer_identity')
            _require(rights == (1 if roles else 0) and len(received) == len(roles),
                     'descriptor_invalid')
            _require(bool(raw), 'disconnected')
            self.guard()
            value = _decode(raw)
            keys = {'version', 'kind', 'run', 'rendezvous', 'seq'}
            if ack is not None:
                keys.add('ack')
            if phase is not None:
                keys.add('phase')
            if roles:
                keys.add('roles')
            _require(set(value) == keys and type(value['version']) is int and value['version'] == 1
                     and value['kind'] == kind and value['run'] == self.view.run
                     and value['rendezvous'] == self.view.rendezvous, 'packet_invalid')
            _require(type(value['seq']) is int and value['seq'] == self.rx + 1,
                     'sequence_invalid')
            if ack is not None:
                _require(type(value['ack']) is int and value['ack'] == ack, 'sequence_invalid')
            if phase is not None:
                _require(value['phase'] == phase.value, 'packet_invalid')
            if roles:
                metadata = self.metadata(received, roles, phase)
                _require(type(value['roles']) is list and value['roles'] == metadata,
                         'descriptor_invalid')
                # Canonical equality alone must not admit bool in numeric fields.
                _require(_json(value['roles']) == _json(metadata), 'descriptor_invalid')
                self.validate_source(received, roles, phase)
            self.guard()
            self.rx += 1
            result = tuple(received)
            self.retain(result)
            received.clear()
            return value, result
        finally:
            for fd in received:
                try:
                    os.close(fd)
                except OSError:
                    pass

    def metadata(self, fds, roles, phase=None):
        _require(type(fds) is tuple or type(fds) is list, 'descriptor_invalid')
        _require(len(fds) == len(roles), 'descriptor_invalid')
        result = []
        owners = None
        if roles == d._LEAVES:
            _require(type(phase) is Phase, 'descriptor_invalid')
            owners = self.owner.material_owners(phase)
            _require(type(owners) is _MaterialOwners and owners.run == self.view.run
                     and owners.phase is phase
                     and all(type(pair) is tuple and len(pair) == 2
                             and all(type(uid) is int and 0 < uid < 65536 for uid in pair)
                             for pair in (owners.pg, owners.redis)), 'descriptor_invalid')
        for fd, role in zip(fds, roles):
            _require(type(fd) is int, 'descriptor_invalid')
            info = os.fstat(fd)
            directory = role in ('run', 'old', 'fresh', *d._APP_DIRS, 'pg', 'redis')
            access = fcntl.fcntl(fd, fcntl.F_GETFL)
            account = (0, 0)
            if owners is not None:
                account = (owners.pg if role == 'pg' else owners.redis if role == 'redis'
                           else (0, 0) if role == 'coolify.dump' else (9999, 9999))
            _require((info.st_uid, info.st_gid) == account
                     and stat.S_IMODE(info.st_mode) == (0o700 if directory else 0o600)
                     and (stat.S_ISDIR(info.st_mode) if directory else
                          stat.S_ISREG(info.st_mode) and info.st_nlink == 1)
                     and access & os.O_ACCMODE == (os.O_RDWR if role == 'engine-journal' else os.O_RDONLY)
                     and not access & os.O_PATH
                     and (not directory or access & os.O_DIRECTORY)
                     and fcntl.fcntl(fd, fcntl.F_GETFD) & fcntl.FD_CLOEXEC,
                     'descriptor_invalid')
            result.append({'role': role, 'device': info.st_dev, 'inode': info.st_ino,
                           'mode': info.st_mode, 'uid': info.st_uid, 'gid': info.st_gid,
                           'nlink': info.st_nlink, 'access': access & os.O_ACCMODE})
        _require(len({(item['device'], item['inode']) for item in result}) == len(result),
                 'descriptor_invalid')
        return result

    def allocation(self):
        self.guard()
        fds = self.owner.issued_allocation_bundle()
        _require(type(fds) is tuple, 'descriptor_invalid')
        self.metadata(fds, _ALLOCATION)
        self.validate_source(fds, _ALLOCATION, None)
        return fds

    def validate_source(self, fds, roles, phase):
        if roles == _ALLOCATION:
            expected = self.owner.issued_allocation_bundle()
            _require(type(expected) is tuple and len(expected) == 4, 'descriptor_invalid')
            _require(all(_id(os.fstat(fd)) == _id(os.fstat(original))
                         for fd, original in zip(fds, expected)), 'descriptor_invalid')
            for fd, name in zip(fds[1:], ('old', 'fresh', 'journal')):
                _require(_id(os.fstat(fd)) == _id(os.stat(name, dir_fd=fds[0], follow_symlinks=False)),
                         'descriptor_invalid')
        else:
            _require(type(phase) is Phase and roles == d._LEAVES, 'descriptor_invalid')
            allocation = self.owner.issued_allocation_bundle()
            _require(type(allocation) is tuple and len(allocation) == 4, 'descriptor_invalid')
            parent = allocation[1 if phase is Phase.OLD else 2]
            for fd, name in zip(fds, roles):
                _require(_id(os.fstat(fd)) == _id(os.stat(name, dir_fd=parent, follow_symlinks=False)),
                         'descriptor_invalid')

    def retain(self, fds):
        identities = [(fd, _fd_id(fd)) for fd in fds]
        self.owned.extend(identities)

    def allocation_send(self):
        _require(self.state == 'authenticated', 'state_invalid')
        fds = self.allocation()
        self.send('ALLOCATION', fds=fds, metadata=self.metadata(fds, _ALLOCATION))
        self.receive('ALLOCATION_ACK', ack=self.tx)
        self.state = 'allocated'
        return self.issue(AllocationReceipt)

    def allocation_receive(self):
        _require(self.state == 'authenticated', 'state_invalid')
        packet, fds = self.receive('ALLOCATION', roles=_ALLOCATION)
        self.send('ALLOCATION_ACK', ack=packet['seq'])
        self.state = 'allocated'
        return self.issue(ReceivedAllocation, fds)

    def check_phase(self, phase):
        _require(self.state == 'allocated' and type(phase) is Phase and self.phase < 2
                 and phase is (Phase.OLD, Phase.FRESH)[self.phase], 'state_invalid')

    def material_send(self, phase, bundle):
        self.check_phase(phase)
        fds = self.owner.guard_issued_material_bundle(phase, bundle)
        _require(type(fds) is tuple, 'descriptor_invalid')
        metadata = self.metadata(fds, d._LEAVES, phase)
        self.validate_source(fds, d._LEAVES, phase)
        self.send('MATERIAL', phase=phase, fds=fds, metadata=metadata)
        self.receive('MATERIAL_ACK', ack=self.tx, phase=phase)
        self.phase += 1
        return self.issue(MaterialReceipt)

    def material_receive(self, phase):
        self.check_phase(phase)
        packet, fds = self.receive('MATERIAL', phase=phase, roles=d._LEAVES)
        self.send('MATERIAL_ACK', ack=packet['seq'], phase=phase)
        self.phase += 1
        return self.issue(RegisteredMaterial, fds, phase)

    def invoke(self, cap, action, *args):
        _require(cap in self.caps, 'ownership_unknown')
        if action == 'close':
            self.close()
            return None
        try:
            self.io_deadline = (None if action == 'borrow' else
                                min(self.deadline, time.monotonic() + 30))
            self.guard()
            if action == 'borrow':
                fds, phase = self.caps[cap]
                _require(all(_fd_id(fd) == identity for fd, identity in self.owned
                             if fd in fds), 'descriptor_invalid')
                roles = _ALLOCATION if type(cap) is ReceivedAllocation else d._LEAVES
                self.metadata(fds, roles, phase)
                self.validate_source(fds, roles, phase)
                return fds
            return {'accept': self.accept, 'allocation_send': self.allocation_send,
                    'allocation_receive': self.allocation_receive,
                    'material_send': self.material_send,
                    'material_receive': self.material_receive}[action](*args)
        except BaseException as error:
            cause = self.first_failure or _category(error)
            self.close(cause)
            raise ProtocolError(cause) from None
        finally:
            self.io_deadline = None

    def close(self, cause=None):
        if self.first_failure is None and cause is not None:
            self.first_failure = cause
        if self.closed:
            return
        self.closed = True
        for sock, identity in self.sockets:
            try:
                if _fd_id(sock.fileno()) == identity:
                    sock.close()
                else:
                    sock.detach()
            except BaseException:
                pass
        if self.owns_endpoint:
            try:
                self.guard_namespace()
                os.unlink(self.view.rendezvous + '.sock', dir_fd=self.root_fd)
            except BaseException:
                pass
        for fd, identity in reversed(self.owned):
            _close_fd(fd, identity)
        self.owned.clear()
