"""PUBLIC synthetic protocol fixtures; no systemd, Docker or production material."""
import array
import fcntl
import json
import os
from pathlib import Path
import socket
import stat
import struct
import tempfile
import time
import unittest
from unittest.mock import patch

try:
    from maintenance import coolify_native_protocol as n
except ImportError:
    n = None


def process_peer(pid):
    raw = Path('/proc/' + str(pid) + '/stat').read_text()
    start = int(raw[raw.rfind(')') + 2:].split()[19])
    return n._Peer(pid, os.getuid(), os.getgid(), start, 'PUBLIC-invocation',
                   ('PUBLIC-cgroup', 1))


class PublicOwner:
    """Registered fake source owner, using actual protected FS and /proc evidence."""
    def __init__(self, path):
        self.path = path
        self.reservation, self.binding, self.engine = object(), object(), object()
        self.bootstrap = object()
        self.run, self.rendezvous = 'a' * 32, 'b' * 32
        self.root = os.open(path, os.O_RDONLY | os.O_DIRECTORY | os.O_CLOEXEC)
        self.fds = [self.root]
        self.controller = process_peer(os.getpid())
        self.peer = None
        self.started = time.monotonic()
        self.deadline = self.started + 2400
        self.drift = None
        self.materials = {}
        self.retained = []
        self.run_path = path / 'run'
        self.run_path.mkdir(mode=0o700)
        for name in ('old', 'fresh'):
            (self.run_path / name).mkdir(mode=0o700)
        (self.run_path / 'journal').touch(mode=0o600)
        self.allocation = tuple(self.open(self.run_path / name, directory=True)
                                for name in ('.', 'old', 'fresh')) + (
                                    self.open(self.run_path / 'journal', writable=True),)

    def open(self, path, *, directory=False, writable=False):
        fd = os.open(path, (os.O_RDWR if writable else os.O_RDONLY) | os.O_CLOEXEC
                     | os.O_NOFOLLOW | (os.O_DIRECTORY if directory else 0))
        self.fds.append(fd)
        return fd

    def guard_reservation(self, reservation):
        n._require(reservation is self.reservation, 'ownership_unknown')
        return n._Reservation(self.root, self.path, self.run, self.rendezvous,
                              self.started, self.deadline)

    def guard_peer(self, role, handle):
        n._require(handle is (self.engine if role == 'engine' else self.bootstrap),
                   'ownership_unknown')
        expected = self.peer if role == 'engine' else self.controller
        n._require(self.drift is None, 'peer_identity')
        n._require(process_peer(expected.pid) == expected, 'peer_identity')
        return expected

    def issued_allocation_bundle(self):
        return self.allocation

    def guard_issued_material_bundle(self, phase, bundle):
        n._require(type(phase) is n.Phase and bundle in self.materials
                   and self.materials[bundle][0] is phase, 'ownership_unknown')
        return self.materials[bundle][1]

    def material_owners(self, phase):
        return n._MaterialOwners(self.run, phase, (70, 70), (999, 999))

    def material(self, phase):
        from maintenance import coolify_docker as d
        directory = self.run_path / phase.value
        fds = []
        for name in d._LEAVES:
            target = directory / name
            is_dir = name in (*d._APP_DIRS, 'pg', 'redis')
            if is_dir:
                target.mkdir(mode=0o700)
            else:
                target.write_bytes(b'PUBLIC fixture\n')
                target.chmod(0o600)
            uid, gid = ((70, 70) if name == 'pg' else (999, 999) if name == 'redis'
                        else (0, 0) if name == 'coolify.dump' else (9999, 9999))
            os.chown(target, uid, gid)
            fds.append(self.open(target, directory=is_dir))
        bundle = object()
        self.materials[bundle] = phase, tuple(fds)
        return bundle

    def close(self):
        for fd in reversed(self.fds):
            os.close(fd)
        self.fds.clear()


def synthetic_controller(owner):
    class Synthetic(n.ControllerProtocol):
        def _source_owner(self, binding):
            n._require(binding is owner.binding, 'ownership_unknown')
            return owner
    return Synthetic()


def synthetic_engine(owner):
    class Synthetic(n.EngineProtocol):
        def _source_owner(self, binding):
            n._require(binding is owner.bootstrap, 'ownership_unknown')
            return owner, owner.reservation
    return Synthetic()


class NativeProtocolTests(unittest.TestCase):
    def setUp(self):
        self.assertIsNotNone(n, 'N2S-2 authenticated Unix protocol is missing')

    def test_native_factories_reject_before_namespace_io(self):
        with patch.object(socket, 'socket', side_effect=AssertionError('PUBLIC unexpected IO')):
            for action in (lambda: n.ControllerProtocol().listen(object(), object()),
                           lambda: n.EngineProtocol().connect(object(), b'{}')):
                with self.assertRaisesRegex(n.ProtocolError, '^source_closure$'):
                    action()

    def test_capabilities_have_no_public_constructor(self):
        for cls in (n.Listener, n.ControllerSession, n.EngineSession,
                    n.AllocationReceipt, n.ReceivedAllocation,
                    n.RegisteredMaterial, n.MaterialReceipt):
            with self.assertRaisesRegex(n.ProtocolError, '^ownership_unknown$'):
                cls()

    def owner(self):
        directory = tempfile.TemporaryDirectory(prefix='PUBLIC-native-')
        self.addCleanup(directory.cleanup)
        owner = PublicOwner(Path(directory.name))
        self.addCleanup(owner.close)
        return owner

    def envelope(self, owner):
        return json.dumps({'version': 1, 'run': owner.run, 'rendezvous': owner.rendezvous},
                          separators=(',', ':'), sort_keys=True).encode('ascii')

    def listen(self, owner):
        listener = synthetic_controller(owner).listen(owner.binding, owner.reservation)
        self.assertIs(type(listener), n.Listener, 'source-owned listener must be issued')
        self.addCleanup(listener.close)
        return listener

    def test_exclusive_pathname_listener_collision_replacement_and_cwd(self):
        owner = self.owner()
        cwd = os.getcwd()
        path = owner.path / (owner.rendezvous + '.sock')
        path.write_bytes(b'PUBLIC foreign')
        with self.assertRaisesRegex(n.ProtocolError, '^endpoint_collision$'):
            self.listen(owner)
        self.assertEqual(path.read_bytes(), b'PUBLIC foreign')
        path.unlink()
        listener = self.listen(owner)
        self.assertTrue(stat.S_ISSOCK(path.lstat().st_mode))
        path.unlink()
        path.write_bytes(b'PUBLIC replacement')
        listener.close()
        self.assertEqual(path.read_bytes(), b'PUBLIC replacement')
        self.assertEqual(os.getcwd(), cwd)

    def test_replaced_root_rejects_without_removing_foreign_endpoint(self):
        owner = self.owner()
        listener = self.listen(owner)
        moved = owner.path.with_name(owner.path.name + '-saved')
        owner.path.rename(moved)
        owner.path.mkdir(mode=0o700)
        self.addCleanup(lambda: moved.rename(owner.path))
        self.addCleanup(owner.path.rmdir)
        with self.assertRaisesRegex(n.ProtocolError, '^endpoint_identity$'):
            listener.accept_authenticated(owner.engine)
        listener.close()
        self.assertTrue((moved / (owner.rendezvous + '.sock')).exists())

    def fork(self, owner, action):
        ready_r, ready_w = os.pipe()
        result_r, result_w = os.pipe()
        done_r, done_w = os.pipe()
        pid = os.fork()
        if pid == 0:
            os.close(ready_w)
            os.close(result_r)
            os.close(done_w)
            try:
                os.read(ready_r, 1)
                owner.peer = process_peer(os.getpid())
                value = action(owner)
                payload = json.dumps({'ok': True, 'value': value}).encode()
            except BaseException as error:
                payload = json.dumps({'ok': False, 'error': type(error).__name__,
                                      'detail': str(error)}).encode()
            os.write(result_w, payload)
            os.read(done_r, 1)
            os._exit(0)
        os.close(ready_r)
        os.close(result_w)
        os.close(done_r)
        owner.peer = process_peer(pid)
        os.write(ready_w, b'!')
        os.close(ready_w)
        def finish():
            result = os.read(result_r, 16384)
            os.close(result_r)
            os.write(done_w, b'!')
            os.close(done_w)
            _, status = os.waitpid(pid, 0)
            self.assertEqual(status, 0)
            result = json.loads(result)
            self.assertTrue(result['ok'], result)
            return result.get('value')
        return finish

    @staticmethod
    def packet(owner, kind='HELLO', seq=1, **extra):
        value = {'version': 1, 'kind': kind, 'run': owner.run,
                 'rendezvous': owner.rendezvous, 'seq': seq}
        value.update(extra)
        return json.dumps(value, separators=(',', ':'), sort_keys=True).encode()

    @staticmethod
    def raw_socket(owner):
        sock = socket.socket(socket.AF_UNIX, socket.SOCK_SEQPACKET)
        sock.setsockopt(socket.SOL_SOCKET, socket.SO_PASSCRED, 1)
        sock.settimeout(5)
        sock.connect(str(owner.path / (owner.rendezvous + '.sock')))
        return sock

    def test_real_packet_rejections_inventory_all_rights_without_fd_leaks(self):
        cases = ('duplicate', 'boolean', 'nonfinite', 'malformed', 'unknown', 'sequence',
                 'truncation', 'extra-rights', 'control-truncation', 'wrong-uid', 'inherited')
        for case in cases:
            with self.subTest(case=case):
                owner = self.owner()
                # Baseline excludes this helper's coordination pipes and listener.
                before = len(os.listdir('/proc/self/fd'))
                listener = self.listen(owner)
                def engine(o):
                    sock = self.raw_socket(o)
                    raw = self.packet(o)
                    fds = [o.allocation[3]] if case in (
                        'extra-rights', 'control-truncation', 'wrong-uid', 'inherited') else []
                    if case == 'duplicate':
                        raw = raw.replace(b'"version":1', b'"version":1,"version":1')
                    elif case == 'boolean':
                        raw = raw.replace(b'"version":1', b'"version":true')
                    elif case == 'nonfinite':
                        raw = raw.replace(b'"version":1', b'"version":NaN')
                    elif case == 'malformed':
                        raw = b'\xff{'
                    elif case == 'unknown':
                        raw = self.packet(o, payload='PUBLIC forbidden')
                    elif case == 'sequence':
                        raw = self.packet(o, seq=2)
                    elif case == 'truncation':
                        raw = b'x' * 17000
                    elif case == 'control-truncation':
                        fds *= 40
                    elif case == 'wrong-uid':
                        os.setgid(65534)
                        os.setuid(65534)
                    ancillary = ([(socket.SOL_SOCKET, socket.SCM_RIGHTS, array.array('i', fds))]
                                 if fds else [])
                    if case == 'inherited':
                        sender = os.fork()
                        if sender == 0:
                            sock.sendmsg([raw], ancillary)
                            os._exit(0)
                        os.waitpid(sender, 0)
                    else:
                        sock.sendmsg([raw], ancillary)
                    sock.close()
                    return True
                finish = self.fork(owner, engine)
                category = ('peer_identity' if case in ('wrong-uid', 'inherited') else
                            'descriptor_invalid' if case == 'extra-rights' else
                            'sequence_invalid' if case == 'sequence' else 'packet_invalid')
                with self.assertRaisesRegex(n.ProtocolError, '^' + category + '$'):
                    listener.accept_authenticated(owner.engine)
                self.assertTrue(finish())
                listener.close()
                self.assertEqual(len(os.listdir('/proc/self/fd')), before)

    def test_real_wrong_peer_pid_fails_peercred_before_hello(self):
        owner = self.owner()
        listener = self.listen(owner)
        finish = self.fork(owner, lambda o: self.raw_socket(o).close())
        owner.peer = owner.controller
        with self.assertRaisesRegex(n.ProtocolError, '^peer_identity$'):
            listener.accept_authenticated(owner.engine)
        finish()

    def test_real_replay_and_disconnect_during_allocation_revoke_session(self):
        for case in ('replay', 'bad-ack', 'disconnect'):
            with self.subTest(case=case):
                owner = self.owner()
                before = len(os.listdir('/proc/self/fd'))
                listener = self.listen(owner)
                def engine(o):
                    sock = self.raw_socket(o)
                    sock.sendmsg([self.packet(o)])
                    sock.recvmsg(16384, 1024, socket.MSG_CMSG_CLOEXEC)
                    if case == 'disconnect':
                        sock.close()
                        return True
                    raw, ancillary, _, _ = sock.recvmsg(16384, 1024, socket.MSG_CMSG_CLOEXEC)
                    rights = []
                    for level, typ, data in ancillary:
                        if level == socket.SOL_SOCKET and typ == socket.SCM_RIGHTS:
                            values = array.array('i')
                            values.frombytes(data)
                            rights.extend(values)
                    for fd in rights:
                        os.close(fd)
                    if case == 'replay':
                        raw = self.packet(o, 'ALLOCATION_ACK', 1, ack=2)
                    else:
                        raw = self.packet(o, 'ALLOCATION_ACK', 2, ack=99)
                    sock.sendmsg([raw])
                    o.retained.append(sock)
                    return True
                finish = self.fork(owner, engine)
                session = listener.accept_authenticated(owner.engine)
                with self.assertRaises(n.ProtocolError):
                    session.send_allocation()
                with self.assertRaises(n.ProtocolError):
                    session.send_allocation()
                finish()
                listener.close()
                self.assertEqual(len(os.listdir('/proc/self/fd')), before)

    def test_actual_leaf_wrong_role_owner_and_writable_env_fd_rejected(self):
        for case in ('app-owner', 'pg-owner', 'writable-env', 'path-only-env'):
            with self.subTest(case=case):
                owner = self.owner()
                bundle = owner.material(n.Phase.OLD)
                listener = self.listen(owner)
                trigger_r, trigger_w = os.pipe()
                self.addCleanup(os.close, trigger_r)
                self.addCleanup(os.close, trigger_w)
                if case == 'app-owner':
                    os.chown(owner.run_path / 'old/runtime.env', 0, 0)
                elif case == 'pg-owner':
                    os.chown(owner.run_path / 'old/pg', 9999, 9999)
                else:
                    fds = owner.materials[bundle][1]
                    if case == 'path-only-env':
                        writable = os.open(owner.run_path / 'old/runtime.env', os.O_PATH | os.O_CLOEXEC)
                        owner.fds.append(writable)
                    else:
                        writable = owner.open(owner.run_path / 'old/runtime.env', writable=True)
                    owner.materials[bundle] = n.Phase.OLD, (writable, *fds[1:])
                def engine(o):
                    session = synthetic_engine(o).connect(o.bootstrap, self.envelope(o))
                    session.receive_allocation()
                    os.read(trigger_r, 1)
                    try:
                        session.register_material(n.Phase.OLD, bundle)
                        raise AssertionError('invalid role-bound material accepted')
                    except n.ProtocolError as error:
                        assert error.args == ('descriptor_invalid',)
                    session.close()
                    return True
                finish = self.fork(owner, engine)
                session = listener.accept_authenticated(owner.engine)
                session.send_allocation()
                os.write(trigger_w, b'!')
                with self.assertRaises(n.ProtocolError):
                    session.receive_material(n.Phase.OLD)
                self.assertTrue(finish())

    def test_supplemental_malformed_ancillary_closes_every_delivered_fd(self):
        real_recvmsg = socket.socket.recvmsg
        for case in ('unknown', 'duplicate-credentials', 'short-credentials', 'malformed-rights'):
            with self.subTest(case=case):
                owner = self.owner()
                before = len(os.listdir('/proc/self/fd'))
                listener = self.listen(owner)
                def engine(o):
                    sock = self.raw_socket(o)
                    sock.sendmsg([self.packet(o)], [(socket.SOL_SOCKET, socket.SCM_RIGHTS,
                                                    array.array('i', [o.allocation[3]]))])
                    sock.close()
                    return True
                finish = self.fork(owner, engine)
                def malformed(sock, *args):
                    raw, anc, flags, address = real_recvmsg(sock, *args)
                    changed = []
                    for level, typ, data in anc:
                        if typ == socket.SCM_CREDENTIALS:
                            if case == 'duplicate-credentials':
                                changed.append((level, typ, data))
                            elif case == 'short-credentials':
                                data = data[:-1]
                        elif typ == socket.SCM_RIGHTS and case == 'malformed-rights':
                            data += b'x'
                        changed.append((level, typ, data))
                    if case == 'unknown':
                        changed.append((socket.SOL_SOCKET, 0x7fff, b'PUBLIC'))
                    return raw, changed, flags, address
                with patch.object(socket.socket, 'recvmsg', malformed):
                    with self.assertRaises(n.ProtocolError):
                        listener.accept_authenticated(owner.engine)
                self.assertTrue(finish())
                listener.close()
                self.assertEqual(len(os.listdir('/proc/self/fd')), before)

    def test_forged_cross_instance_and_closed_capability_cannot_gain_fds(self):
        for cls in (n.Listener, n.ControllerSession, n.EngineSession,
                    n.ReceivedAllocation, n.RegisteredMaterial):
            forged = object.__new__(cls)
            action = (forged.accept_authenticated if cls is n.Listener else
                      forged.send_allocation if cls is n.ControllerSession else
                      forged.receive_allocation if cls is n.EngineSession else forged.borrowed_fds)
            with self.assertRaisesRegex(n.ProtocolError, '^ownership_unknown$'):
                action(object()) if cls is n.Listener else action()
        first, other = self.owner(), self.owner()
        controller = synthetic_controller(first)
        with self.assertRaisesRegex(n.ProtocolError, '^ownership_unknown$'):
            controller.listen(other.binding, other.reservation)
        with self.assertRaisesRegex(n.ProtocolError, '^ownership_unknown$'):
            controller.listen(first.binding, other.reservation)

    def test_guard_fault_keeps_first_static_cause_and_never_formats_exception(self):
        owner = self.owner()
        listener = self.listen(owner)
        owner.peer = owner.controller
        class Hostile(Exception):
            def __str__(self):
                raise AssertionError('PUBLIC exception must not be formatted')
        with patch.object(owner, 'guard_peer', side_effect=Hostile('PUBLIC marker')):
            with self.assertRaisesRegex(n.ProtocolError, '^transport_failed$'):
                listener.accept_authenticated(owner.engine)
        with self.assertRaisesRegex(n.ProtocolError, '^transport_failed$'):
            listener.accept_authenticated(owner.engine)
        listener.close()

    def test_public_session_phase_order_and_repeated_allocation_reject(self):
        for role in ('controller', 'engine'):
            with self.subTest(role=role):
                owner = self.owner()
                listener = self.listen(owner)
                trigger_r, trigger_w = os.pipe()
                self.addCleanup(os.close, trigger_r)
                self.addCleanup(os.close, trigger_w)
                def engine(o):
                    session = synthetic_engine(o).connect(o.bootstrap, self.envelope(o))
                    session.receive_allocation()
                    if role == 'controller':
                        o.retained.append(session)
                    else:
                        os.read(trigger_r, 1)
                        try:
                            session.register_material(n.Phase.FRESH, object())
                            raise AssertionError('out of order material was accepted')
                        except n.ProtocolError as error:
                            assert error.args == ('state_invalid',)
                        session.close()
                    return True
                finish = self.fork(owner, engine)
                session = listener.accept_authenticated(owner.engine)
                session.send_allocation()
                if role == 'controller':
                    with self.assertRaisesRegex(n.ProtocolError, '^state_invalid$'):
                        session.send_allocation()
                else:
                    os.write(trigger_w, b'!')
                self.assertTrue(finish())

    def test_real_received_allocation_validation_and_ack_failure_close_all_fds(self):
        for case in ('wrong-role', 'boolean-metadata', 'swapped', 'extra', 'ack-loss'):
            with self.subTest(case=case):
                owner = self.owner()
                path = owner.path / (owner.rendezvous + '.sock')
                server = socket.socket(socket.AF_UNIX, socket.SOCK_SEQPACKET)
                server.setsockopt(socket.SOL_SOCKET, socket.SO_PASSCRED, 1)
                server.bind(str(path))
                path.chmod(0o600)
                server.listen(1)
                self.addCleanup(server.close)
                self.addCleanup(lambda path=path: path.unlink(missing_ok=True))
                def engine(o):
                    baseline = len(os.listdir('/proc/self/fd'))
                    session = synthetic_engine(o).connect(o.bootstrap, self.envelope(o))
                    actual_send = socket.socket.sendmsg
                    def lose_ack(sock, buffers, *args):
                        if b'"kind":"ALLOCATION_ACK"' in buffers[0]:
                            raise BrokenPipeError('PUBLIC ACK lost')
                        return actual_send(sock, buffers, *args)
                    try:
                        if case == 'ack-loss':
                            with patch.object(socket.socket, 'sendmsg', lose_ack):
                                session.receive_allocation()
                        else:
                            session.receive_allocation()
                        raise AssertionError('invalid allocation/ACK was accepted')
                    except n.ProtocolError:
                        pass
                    session.close()
                    assert len(os.listdir('/proc/self/fd')) == baseline
                    return True
                finish = self.fork(owner, engine)
                peer, _ = server.accept()
                peer.recvmsg(16384, 1024, socket.MSG_CMSG_CLOEXEC)
                peer.sendmsg([self.packet(owner, 'HELLO_ACK', 1, ack=1)])
                fds = owner.allocation
                roles = []
                for fd, role in zip(fds, ('run', 'old', 'fresh', 'engine-journal')):
                    info = os.fstat(fd)
                    roles.append({'role': role, 'device': info.st_dev, 'inode': info.st_ino,
                                  'mode': info.st_mode, 'uid': info.st_uid, 'gid': info.st_gid,
                                  'nlink': info.st_nlink,
                                  'access': fcntl.fcntl(fd, fcntl.F_GETFL) & os.O_ACCMODE})
                if case == 'wrong-role':
                    roles[-1]['role'] = 'controller-ledger'
                elif case == 'boolean-metadata':
                    roles[0]['uid'] = False
                elif case == 'swapped':
                    fds = (fds[0], fds[2], fds[1], fds[3])
                elif case == 'extra':
                    fds = (*fds, fds[3])
                peer.sendmsg([self.packet(owner, 'ALLOCATION', 2, roles=roles)],
                             [(socket.SOL_SOCKET, socket.SCM_RIGHTS, array.array('i', fds))])
                self.assertTrue(finish())
                peer.close()
                server.close()

    def test_owner_deadline_cannot_extend_and_metadata_io_is_bounded(self):
        owner = self.owner()
        listener = self.listen(owner)
        owner.deadline += 1
        with self.assertRaisesRegex(n.ProtocolError, '^deadline$'):
            listener.accept_authenticated(owner.engine)
        owner = self.owner()
        listener = self.listen(owner)
        owner.peer = owner.controller
        # Future source owner may shorten an absolute budget, never extend it.
        owner.deadline = time.monotonic() + 120.02
        with self.assertRaisesRegex(n.ProtocolError, '^deadline$'):
            listener.accept_authenticated(owner.engine)
        self.assertLess(owner.deadline - 120, time.monotonic() + 0.05)

    def test_handshake_has_one_30_second_budget_across_all_io(self):
        owner = self.owner()
        listener = self.listen(owner)
        finish = self.fork(owner, lambda o: self.raw_handshake(o))
        actual_recv = socket.socket.recvmsg
        clock = [time.monotonic()]
        def delayed(sock, *args):
            result = actual_recv(sock, *args)
            # Advance the parent's clock instead of waiting 31 real seconds.
            clock[0] += 31
            return result
        with patch.object(socket.socket, 'recvmsg', delayed), patch.object(
                n.time, 'monotonic', side_effect=lambda: clock[0]):
            with self.assertRaisesRegex(n.ProtocolError, '^deadline$'):
                listener.accept_authenticated(owner.engine)
        self.assertTrue(finish())

    def raw_handshake(self, owner):
        sock = self.raw_socket(owner)
        sock.sendmsg([self.packet(owner)])
        sock.close()
        return True

    def test_borrowed_descriptor_replacement_close_preserves_foreign_fd(self):
        owner = self.owner()
        listener = self.listen(owner)
        trigger_r, trigger_w = os.pipe()
        self.addCleanup(os.close, trigger_r)
        self.addCleanup(os.close, trigger_w)
        def engine(o):
            session = synthetic_engine(o).connect(o.bootstrap, self.envelope(o))
            allocation = session.receive_allocation()
            victim = allocation.borrowed_fds()[3]
            os.read(trigger_r, 1)
            os.close(victim)
            foreign = os.open(o.run_path / 'journal', os.O_RDONLY | os.O_CLOEXEC)
            os.dup2(foreign, victim)
            # Same inode, but distinct borrowed descriptor ownership cannot be
            # distinguished by dev/inode. Use a different public fixture inode.
            os.close(victim)
            foreign_path = o.path / 'foreign'
            foreign_path.write_bytes(b'PUBLIC foreign')
            replacement = os.open(foreign_path, os.O_RDONLY | os.O_CLOEXEC)
            if replacement != victim:
                os.dup2(replacement, victim)
                os.close(replacement)
            session.close()
            assert os.pread(victim, 64, 0) == b'PUBLIC foreign'
            os.close(victim)
            if foreign != victim:
                os.close(foreign)
            return True
        finish = self.fork(owner, engine)
        session = listener.accept_authenticated(owner.engine)
        session.send_allocation()
        os.write(trigger_w, b'!')
        self.assertTrue(finish())
        session.close()

    def test_close_received_fd_after_mode_drift_does_not_leak(self):
        owner = self.owner()
        listener = self.listen(owner)
        trigger_r, trigger_w = os.pipe()
        self.addCleanup(os.close, trigger_r)
        self.addCleanup(os.close, trigger_w)
        def engine(o):
            before = len(os.listdir('/proc/self/fd'))
            session = synthetic_engine(o).connect(o.bootstrap, self.envelope(o))
            allocation = session.receive_allocation()
            fd = allocation.borrowed_fds()[3]
            os.read(trigger_r, 1)
            os.fchmod(fd, 0o640)
            try:
                allocation.borrowed_fds()
                raise AssertionError('mode-drifted descriptor remained usable')
            except n.ProtocolError:
                pass
            session.close()
            try:
                os.fstat(fd)
                raise AssertionError('received descriptor leaked after mode drift')
            except OSError:
                pass
            assert len(os.listdir('/proc/self/fd')) == before
            return True
        finish = self.fork(owner, engine)
        session = listener.accept_authenticated(owner.engine)
        session.send_allocation()
        os.write(trigger_w, b'!')
        self.assertTrue(finish())
        session.close()

    def test_real_process_allocation_and_both_material_phases_roundtrip(self):
        owner = self.owner()
        old, fresh = owner.material(n.Phase.OLD), owner.material(n.Phase.FRESH)
        listener = self.listen(owner)
        trigger_r, trigger_w = os.pipe()
        self.addCleanup(os.close, trigger_r)
        self.addCleanup(os.close, trigger_w)
        offset = os.lseek(owner.allocation[3], 0, os.SEEK_CUR)
        def engine(o):
            session = synthetic_engine(o).connect(o.bootstrap, self.envelope(o))
            allocation = session.receive_allocation()
            fds = allocation.borrowed_fds()
            assert len(fds) == 4
            assert all(fcntl.fcntl(fd, fcntl.F_GETFD) & fcntl.FD_CLOEXEC for fd in fds)
            assert all(fcntl.fcntl(fd, fcntl.F_GETFL) & os.O_ACCMODE == os.O_RDONLY
                       for fd in fds[:3])
            assert os.lseek(fds[3], 0, os.SEEK_CUR) == offset
            os.write(fds[3], b'PUBLIC journal')
            for phase, bundle in ((n.Phase.OLD, old), (n.Phase.FRESH, fresh)):
                receipt = session.register_material(phase, bundle)
                assert type(receipt) is n.MaterialReceipt
            os.read(trigger_r, 1)
            session.close()
            try:
                allocation.borrowed_fds()
                raise AssertionError('stale allocation accepted')
            except n.ProtocolError:
                pass
            return True
        finish = self.fork(owner, engine)
        session = listener.accept_authenticated(owner.engine)
        self.addCleanup(session.close)
        receipt = session.send_allocation()
        self.assertIs(type(receipt), n.AllocationReceipt)
        for phase in (n.Phase.OLD, n.Phase.FRESH):
            material = session.receive_material(phase)
            self.assertIs(type(material), n.RegisteredMaterial)
            self.assertEqual(len(material.borrowed_fds()), 12)
        os.write(trigger_w, b'!')
        self.assertTrue(finish())
        self.assertEqual(os.pread(owner.allocation[3], 64, 0), b'PUBLIC journal')
        self.assertEqual(os.lseek(owner.allocation[3], 0, os.SEEK_CUR), 14)
        session.close()
        with self.assertRaisesRegex(n.ProtocolError, '^ownership_unknown$'):
            material.borrowed_fds()

    def test_noncanonical_bootstrap_and_foreign_reservations_reject_before_io(self):
        owner = self.owner()
        for raw in (b'{}', b'{"version":true}', b'{"version":NaN}', b'\xff',
                    self.envelope(owner) + b' ', self.envelope(owner).replace(b'"version":1',
                         b'"version":1,"version":1')):
            with self.subTest(raw=raw):
                with self.assertRaises(n.ProtocolError):
                    synthetic_engine(owner).connect(owner.bootstrap, raw)
        with self.assertRaisesRegex(n.ProtocolError, '^ownership_unknown$'):
            synthetic_controller(owner).listen(owner.binding, object())

    def test_deadline_and_source_identity_drift_reject_before_accept(self):
        for drift in ('pid-start', 'invocation', 'cgroup'):
            owner = self.owner()
            listener = self.listen(owner)
            owner.peer = owner.controller
            owner.drift = drift
            with self.assertRaisesRegex(n.ProtocolError, '^peer_identity$'):
                listener.accept_authenticated(owner.engine)
        owner = self.owner()
        listener = self.listen(owner)
        owner.deadline = time.monotonic() - 1
        with self.assertRaisesRegex(n.ProtocolError, '^deadline$'):
            listener.accept_authenticated(owner.engine)

    def test_controller_close_revokes_engine_allocation_while_controller_pid_alive(self):
        owner = self.owner()
        ready_r, ready_w = os.pipe()
        trigger_r, trigger_w = os.pipe()
        for fd in (ready_r, ready_w, trigger_r, trigger_w):
            self.addCleanup(os.close, fd)
        listener = self.listen(owner)
        def engine(o):
            baseline = len(os.listdir('/proc/self/fd'))
            session = synthetic_engine(o).connect(o.bootstrap, self.envelope(o))
            allocation = session.receive_allocation()
            received = allocation.borrowed_fds()
            os.write(ready_w, b'!')
            os.read(trigger_r, 1)
            assert process_peer(o.controller.pid) == o.controller
            try:
                allocation.borrowed_fds()
                raise AssertionError('allocation survived controller connection close')
            except n.ProtocolError as error:
                assert error.args == ('disconnected',)
            for fd in received:
                try:
                    os.fstat(fd)
                    raise AssertionError('disconnected allocation descriptor leaked')
                except OSError:
                    pass
            session.close()
            assert len(os.listdir('/proc/self/fd')) == baseline
            return True
        finish = self.fork(owner, engine)
        session = listener.accept_authenticated(owner.engine)
        session.send_allocation()
        os.read(ready_r, 1)
        session.close()
        os.write(trigger_w, b'!')
        self.assertTrue(finish())

    def test_engine_close_and_halfclose_revoke_registered_material_while_engine_pid_alive(self):
        for halfclose in (False, True):
            with self.subTest(halfclose=halfclose):
                owner = self.owner()
                bundle = owner.material(n.Phase.OLD)
                ready_r, ready_w = os.pipe()
                trigger_r, trigger_w = os.pipe()
                for fd in (ready_r, ready_w, trigger_r, trigger_w):
                    self.addCleanup(os.close, fd)
                baseline = len(os.listdir('/proc/self/fd'))
                listener = self.listen(owner)
                def engine(o):
                    session = synthetic_engine(o).connect(o.bootstrap, self.envelope(o))
                    session.receive_allocation()
                    session.register_material(n.Phase.OLD, bundle)
                    os.read(trigger_r, 1)
                    kept = []
                    actual_close = socket.socket.close
                    def shutdown_write(sock):
                        # Real kernel half-close; retain this fixture socket until
                        # the parent has checked revocation with the sender alive.
                        sock.shutdown(socket.SHUT_WR)
                        kept.append(sock)
                    if halfclose:
                        with patch.object(socket.socket, 'close', shutdown_write):
                            session.close()
                    else:
                        session.close()
                    os.write(ready_w, b'!')
                    os.read(trigger_r, 1)
                    for sock in kept:
                        actual_close(sock)
                    return True
                finish = self.fork(owner, engine)
                session = listener.accept_authenticated(owner.engine)
                session.send_allocation()
                material = session.receive_material(n.Phase.OLD)
                os.write(trigger_w, b'!')
                os.read(ready_r, 1)
                self.assertEqual(process_peer(owner.peer.pid), owner.peer)
                try:
                    with self.assertRaisesRegex(n.ProtocolError, '^disconnected$'):
                        material.borrowed_fds()
                finally:
                    session.close()
                    os.write(trigger_w, b'!')
                    self.assertTrue(finish())
                self.assertEqual(len(os.listdir('/proc/self/fd')), baseline)

    def test_registered_material_path_replacement_revokes_pinned_received_identity(self):
        owner = self.owner()
        bundle = owner.material(n.Phase.OLD)
        ready_r, ready_w = os.pipe()
        trigger_r, trigger_w = os.pipe()
        for fd in (ready_r, ready_w, trigger_r, trigger_w):
            self.addCleanup(os.close, fd)
        listener = self.listen(owner)
        def engine(o):
            session = synthetic_engine(o).connect(o.bootstrap, self.envelope(o))
            session.receive_allocation()
            session.register_material(n.Phase.OLD, bundle)
            os.write(ready_w, b'!')
            os.read(trigger_r, 1)
            session.close()
            return True
        finish = self.fork(owner, engine)
        session = listener.accept_authenticated(owner.engine)
        session.send_allocation()
        material = session.receive_material(n.Phase.OLD)
        os.read(ready_r, 1)
        leaf = owner.run_path / 'old/runtime.env'
        leaf.rename(owner.run_path / 'old/PUBLIC-saved')
        leaf.write_bytes(b'PUBLIC replacement')
        leaf.chmod(0o600)
        os.chown(leaf, 9999, 9999)
        with self.assertRaisesRegex(n.ProtocolError, '^descriptor_invalid$'):
            material.borrowed_fds()
        os.write(trigger_w, b'!')
        self.assertTrue(finish())

    def test_engine_observes_endpoint_drift_without_owning_listener_unlink(self):
        for change in ('chmod', 'unlink', 'replacement', 'no-change'):
            with self.subTest(change=change):
                owner = self.owner()
                ready_r, ready_w = os.pipe()
                trigger_r, trigger_w = os.pipe()
                for fd in (ready_r, ready_w, trigger_r, trigger_w):
                    self.addCleanup(os.close, fd)
                listener = self.listen(owner)
                path = owner.path / (owner.rendezvous + '.sock')
                def engine(o):
                    baseline = len(os.listdir('/proc/self/fd'))
                    session = synthetic_engine(o).connect(o.bootstrap, self.envelope(o))
                    allocation = session.receive_allocation()
                    received = allocation.borrowed_fds()
                    os.write(ready_w, b'!')
                    os.read(trigger_r, 1)
                    assert process_peer(o.controller.pid) == o.controller
                    if change == 'no-change':
                        assert allocation.borrowed_fds() == received
                    else:
                        try:
                            allocation.borrowed_fds()
                            raise AssertionError('engine allocation survived endpoint drift')
                        except n.ProtocolError as error:
                            assert error.args == ('endpoint_identity',)
                    session.close()
                    for fd in received:
                        try:
                            os.fstat(fd)
                            raise AssertionError('endpoint-invalidated allocation FD leaked')
                        except OSError:
                            pass
                    assert len(os.listdir('/proc/self/fd')) == baseline
                    return True
                finish = self.fork(owner, engine)
                session = listener.accept_authenticated(owner.engine)
                session.send_allocation()
                os.read(ready_r, 1)
                if change == 'chmod':
                    path.chmod(0o640)
                elif change in ('unlink', 'replacement'):
                    path.unlink()
                    if change == 'replacement':
                        path.write_bytes(b'PUBLIC foreign endpoint')
                identity = path.lstat() if path.exists() else None
                os.write(trigger_w, b'!')
                self.assertTrue(finish())
                # Engine close has observer evidence only, including the valid
                # socket case. It must never remove the controller's pathname.
                if identity is None:
                    self.assertFalse(path.exists())
                else:
                    observed = path.lstat()
                    self.assertEqual((observed.st_dev, observed.st_ino, observed.st_mode),
                                     (identity.st_dev, identity.st_ino, identity.st_mode))
                if change == 'replacement':
                    self.assertEqual(path.read_bytes(), b'PUBLIC foreign endpoint')
                session.close()
