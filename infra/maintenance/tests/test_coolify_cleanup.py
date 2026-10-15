"""PUBLIC synthetic controller authority fixtures; no real Docker or secrets."""
import copy
import ctypes
import fcntl
import importlib
import json
import multiprocessing
import os
from pathlib import Path
import select
import signal
import socket
import stat
import sys
import tempfile
import threading
import time
import unittest
from unittest.mock import patch

from maintenance import coolify_docker as d
import test_coolify_docker as n1

FakeDaemon, MARKER = n1.FakeDaemon, n1.MARKER

try:
    c = importlib.import_module('maintenance.coolify_cleanup')
except ModuleNotFoundError:
    c = None


# Both endpoints are test-owned, on a private temporary filesystem. The daemon
# outlives a killed controller/client and therefore models delayed create effects.
CLIENT = '''import json, socket, sys, time
s=socket.socket(socket.AF_UNIX); s.connect(sys.argv[1])
s.sendall(json.dumps(sys.argv[2:]).encode()+b'\\n')
data=b''
while True:
    v=s.recv(65536)
    if not v: break
    data+=v
r=json.loads(data)
if r.get('hold'): time.sleep(1.2)
sys.stdout.buffer.write(bytes.fromhex(r['out']))
sys.stderr.buffer.write(bytes.fromhex(r['err'])); sys.exit(r['code'])
'''


def daemon_main(path, ready, control):
    daemon = FakeDaemon()
    server = socket.socket(socket.AF_UNIX)
    server.bind(path)
    server.listen(8)
    ready.send(True)
    while True:
        peer, _ = server.accept()
        raw = b''
        while not raw.endswith(b'\n'):
            raw += peer.recv(65536)
        args = json.loads(raw)
        code, out, err = 0, b'', b''
        try:
            if args[0] == 'fixture':
                command = args[1]
                if command == 'shutdown':
                    peer.close()
                    break
                if command == 'state':
                    out = json.dumps({'calls': daemon.calls, 'containers': daemon.containers}).encode()
                elif command == 'mode':
                    control['mode'] = args[2]
                elif command == 'mutate':
                    cid, key, value = args[2:]
                    if key == 'State':
                        daemon.containers[cid]['State'].update(json.loads(value))
                    else:
                        daemon.containers[cid][key] = json.loads(value)
                elif command == 'foreign':
                    cid = args[2]
                    value = copy.deepcopy(next(iter(daemon.containers.values())))
                    value.update(Id=cid, User='0:0')
                    daemon.containers[cid] = value
            elif args[0] == 'create':
                control['creating'] = True
                mode = control.get('mode', '')
                if mode == 'late':
                    saved = list(args)
                    def delayed():
                        time.sleep(.3)
                        daemon.create(saved, env=d._ENV, pass_fds=())
                    threading.Thread(target=delayed, daemon=True).start()
                    code, out = 1, b''
                    peer.sendall(json.dumps({'code': code, 'out': '', 'err': ''}).encode())
                    continue
                if mode == 'delay':
                    time.sleep(.3)
                cid = daemon.create(args, env=d._ENV, pass_fds=())
                out = (cid + '\n').encode()
                if mode == 'lost':
                    code, out = 1, b''
                if mode == 'invalid':
                    out = b'not-an-id\n'
                if mode == 'flood':
                    out = b'x' * 70000
                if mode == 'deadline':
                    time.sleep(2)
            elif args[:2] == ['container', 'ls']:
                selected = {}
                for arg in args:
                    if arg.startswith('--filter=name='):
                        selected['name'] = arg.split('=', 2)[2].removeprefix('^/').removesuffix('$')
                    if arg.startswith('--filter=label=sifto.intent='):
                        selected['intent'] = arg.rsplit('=', 1)[1]
                    if arg.startswith('--filter=label=sifto.run='):
                        selected['run'] = arg.rsplit('=', 1)[1]
                out = ''.join(cid + '\n' for cid in daemon.lookup(**selected)).encode()
            elif args[:2] == ['container', 'inspect']:
                value = daemon.inspect(args[-1])
                if value is None:
                    code, err = 1, ('Error: No such container: ' + args[-1]).encode()
                else:
                    out = json.dumps(value).encode()
            elif args[:2] == ['container', 'rm']:
                daemon.remove(args[-1])
                out = (args[-1] + '\n').encode()
            else:
                code = 1
            peer.sendall(json.dumps({'code': code, 'out': out.hex(), 'err': err.hex(),
                                    'hold': args[0] == 'create' and control.get('mode') == 'held-client'}).encode())
        except (BrokenPipeError, ConnectionResetError):
            pass
        finally:
            peer.close()
    server.close()


def fixture_request(path, *args):
    peer = socket.socket(socket.AF_UNIX)
    peer.connect(str(path))
    peer.sendall(json.dumps(['fixture', *args]).encode() + b'\n')
    raw = b''
    while True:
        value = peer.recv(65536)
        if not value:
            break
        raw += value
    peer.close()
    return bytes.fromhex(json.loads(raw)['out']) if raw else b''


class CleanupAuthorityTests(unittest.TestCase):
    def setUp(self):
        self.assertIsNotNone(c, 'controller-owned cleanup authority module is missing')
        previous_subreaper = ctypes.c_int()
        self.assertEqual(ctypes.CDLL(None).prctl(37, ctypes.byref(previous_subreaper), 0, 0, 0), 0)
        self.assertEqual(ctypes.CDLL(None).prctl(36, 1, 0, 0, 0), 0)
        self.addCleanup(lambda: ctypes.CDLL(None).prctl(36, previous_subreaper.value, 0, 0, 0))
        self.temp = tempfile.TemporaryDirectory()
        self.addCleanup(self.temp.cleanup)
        self.root = Path(self.temp.name)
        self.root.chmod(0o700)
        self.stage = self.root / 'staging'
        self.stage.mkdir(mode=0o700)
        self.run_token = '1' * 32
        self.run_path = self.stage / self.run_token
        self.run_path.mkdir(mode=0o700)
        self.fds = []
        self.addCleanup(self.close_fds)
        self.stage_fd = self.open_dir(self.stage)
        self.run_fd = self.open_dir(self.run_path)
        self.leaves, phases = {}, []
        for phase in (d.Phase.OLD, d.Phase.FRESH):
            path = self.run_path / phase.value
            path.mkdir(mode=0o700)
            fd = self.open_dir(path)
            marker = self.marker(path / 'owner-marker')
            leaves = []
            for name in d._LEAVES:
                leaf = path / name
                uid = 70 if name == 'pg' else 999 if name == 'redis' else 0 if name == 'coolify.dump' else 9999
                if name in (*d._APP_DIRS, 'pg', 'redis'):
                    leaf.mkdir(mode=0o700)
                    flags = os.O_RDONLY | os.O_DIRECTORY
                else:
                    leaf.write_bytes((MARKER + '\n').encode())
                    leaf.chmod(0o600)
                    flags = os.O_RDONLY
                os.chown(leaf, uid, uid)
                leaf_fd = os.open(leaf, flags | os.O_NOFOLLOW | os.O_CLOEXEC)
                self.fds.append(leaf_fd)
                leaves.append(c._LeafView(name, leaf_fd, c._identity(os.fstat(leaf_fd))))
                self.leaves[phase, name] = leaf
            phases.append(c._PhaseView(phase, fd, path, marker, tuple(leaves)))
        self.marker_id = self.marker(self.run_path / 'owner-marker')
        self.socket = socket.socket(socket.AF_UNIX)
        self.socket.bind(str(self.root / 'fixed-docker.sock'))
        self.addCleanup(self.socket.close)
        self.client = self.root / 'client.py'
        self.client.write_text(CLIENT)
        self.daemon_socket = self.root / 'daemon.sock'
        self.manager = multiprocessing.Manager()
        self.control = self.manager.dict()
        self.addCleanup(self.stop_manager)
        receive, send = multiprocessing.Pipe(False)
        self.daemon = multiprocessing.Process(target=daemon_main,
            args=(str(self.daemon_socket), send, self.control))
        self.daemon.start()
        self.assertTrue(receive.poll(5))
        receive.recv()
        send.close()
        receive.close()
        self.addCleanup(self.stop_daemon)
        self.reservation, self.parent = object(), object()
        self.phase_caps = {phase: object() for phase in (d.Phase.OLD, d.Phase.FRESH)}
        self.source = c._SourceView(self.stage_fd, self.stage, '2' * 64, '3' * 64,
            '4' * 32, self.root / 'fixed-docker.sock', c._identity((self.root / 'fixed-docker.sock').lstat()),
            time.monotonic() + 2400)
        self.view = c._RunView(self.run_fd, self.run_path, self.run_token, '5' * 32,
            '6' * 32, self.marker_id, self.parent, 'siftocoolify' + self.run_token + '.slice',
            tuple((role, 'repo/' + role + '@sha256:' + 'a' * 64, 'sha256:' + 'b' * 64,
                   70 if role == 'postgres' else 999 if role == 'redis' else 9999,
                   70 if role == 'postgres' else 999 if role == 'redis' else 9999)
                  for role in ('old', 'target', 'postgres', 'redis')), tuple(phases), time.monotonic() + 200,
            self.source.controller_invocation)
        outer = self
        class Owner:
            gone = True
            def guard_source(self):
                return outer.source
            def guard_reservation(self, reservation):
                if reservation is not outer.reservation:
                    raise c.CleanupError('ownership_unknown')
                return outer.view
            def recover_reservation(self, token):
                if token != outer.run_token:
                    raise c.CleanupError('ownership_unknown')
                return outer.reservation
            def guard_parent(self, parent, reservation):
                if parent is not outer.parent or reservation is not outer.reservation:
                    raise c.CleanupError('resource_isolation')
            def guard_phase(self, reservation, issued):
                self.guard_reservation(reservation)
                phase = next((key for key, value in outer.phase_caps.items() if value is issued), None)
                if phase is None:
                    raise c.CleanupError('ownership_unknown')
                return next(value for value in outer.view.phases if value.phase is phase)
            def resource_cleanup_complete(self, reservation):
                self.guard_reservation(reservation)
                return self.gone
            def attach_controller(self, pid, start):
                outer.assertGreater(pid, 0)
                outer.assertIs(type(start), bytes)
                helpers = list(outer.control.get('helpers', []))
                helpers.append((pid, start.hex()))
                outer.control['helpers'] = helpers
            def recovery_process_cleanup(self, reservation, invocation):
                self.guard_reservation(reservation)
                if invocation != outer.view.controller_invocation:
                    raise c.CleanupError('cleanup_incomplete')
                if outer.control.get('process_boolean'):
                    return True
                for pid, token in outer.control.get('helpers', []):
                    try:
                        os.waitpid(pid, os.WNOHANG)
                    except ChildProcessError:
                        pass
                    # A dead supervisor alone is insufficient: verify no process
                    # remains in its exact recorded session/group, including a
                    # helper orphaned by a killed controller. The daemon is in
                    # its own independent process group and is never a target.
                    for entry in Path('/proc').iterdir():
                        if not entry.name.isdigit():
                            continue
                        try:
                            raw = (entry / 'stat').read_bytes()
                            fields = raw[raw.rfind(b')') + 2:].split()
                            if int(fields[2]) == pid and int(fields[3]) == pid:
                                return None
                        except (FileNotFoundError, ProcessLookupError):
                            pass
                return d._PROCESS_CLEANUP
        self.owner = Owner()
        class Commands(c._DockerCommands):
            def _executable(self):
                return [sys.executable, '-I', '-B', str(outer.client), str(outer.daemon_socket)]
        self.command_type = Commands
        self.core = c._CleanupCore(self.owner, commands=Commands)
        self.addCleanup(self.core.close)
        self.create, self.cleanup = self.core.authorities()
        # Model S1 reserve occurring after clear admission: the protected fixture
        # allocation is installed beneath the ledger root only with its ticket.
        self.prepared_path = self.root / 'prepared-run'
        self.run_path.rename(self.prepared_path)

    def close_fds(self):
        for fd in self.fds:
            try:
                os.close(fd)
            except OSError:
                pass

    def open_dir(self, path):
        fd = os.open(path, os.O_RDONLY | os.O_DIRECTORY | os.O_CLOEXEC)
        self.fds.append(fd)
        return fd

    def marker(self, path):
        path.write_bytes(b'PUBLIC-owner-marker')
        path.chmod(0o600)
        return c._identity(path.lstat())

    def stop_daemon(self):
        if self.daemon.is_alive():
            fixture_request(self.daemon_socket, 'shutdown')
            self.daemon.join(5)
        if self.daemon.is_alive():
            self.daemon.kill()
            self.daemon.join(5)
        self.daemon.close()

    def stop_manager(self):
        self.control._close()
        self.manager.shutdown()
        self.manager._process.close()

    def state(self):
        return json.loads(fixture_request(self.daemon_socket, 'state'))

    def begin(self):
        ticket = self.create.require_clear_admission()
        self.install_run()
        return self.create.begin(ticket, self.reservation)

    def install_run(self):
        if self.prepared_path.exists():
            self.prepared_path.rename(self.run_path)

    def begin_in_controller(self, create):
        ticket = create.require_clear_admission()
        self.install_run()
        return create.begin(ticket, self.reservation)

    def permit(self, lease, operation=d.Operation.IDENTITY_OLD, phase=d.Phase.RUN, receipts=()):
        if phase is not d.Phase.RUN and phase.value not in self.core._lease_record(lease)['phases']:
            self.create.register_phase(lease, self.phase_caps[phase])
        dependencies = self.core._register_dependencies(lease, phase, receipts)
        return self.create.authorize(lease, operation, phase, dependencies)

    def create_identity(self):
        lease = self.begin()
        return lease, self.create.create_once(self.permit(lease), None)

    def test_permit_projection_is_guarded_frozen_and_not_caller_spec_authority(self):
        lease = self.begin()
        permit = self.permit(lease)
        self.assertTrue(hasattr(self.create, 'permit_projection'), 'guarded create permit projection missing')
        view = self.create.permit_projection(permit)
        self.assertEqual(view.operation, d.Operation.IDENTITY_OLD)
        self.assertEqual(view.phase, d.Phase.RUN)
        self.assertRegex(view.intent, '^[a-f0-9]{32}$')
        self.assertRegex(view.spec_sha256, '^[a-f0-9]{64}$')
        with self.assertRaises(Exception):
            view.intent = 'f' * 32
        self.create.create_once(permit, None)
        self.assert_category('ownership_unknown', lambda: self.create.permit_projection(permit))

    def test_original_admission_ticket_retains_same_cloexec_flock_after_begin(self):
        ticket = self.create.require_clear_admission()
        fd = self.core.lock_fd
        self.assertTrue(hasattr(ticket, 'guard'), 'retained admission lifetime guard missing')
        ticket.guard()
        self.install_run()
        self.create.begin(ticket, self.reservation)
        ticket.guard()
        self.assertEqual(self.core.lock_fd, fd)
        self.assertFalse(os.get_inheritable(fd))
        other = os.open('container-controller-lock', os.O_RDWR | os.O_CLOEXEC, dir_fd=self.stage_fd)
        try:
            with self.assertRaises(BlockingIOError):
                fcntl.flock(other, fcntl.LOCK_EX | fcntl.LOCK_NB)
        finally:
            os.close(other)
        self.core.close()
        self.assert_category('ownership_unknown', ticket.guard)

    def test_normal_receipt_removal_preserves_running_postgres_and_is_main_bounded(self):
        lease = self.begin()
        pg = self.create.create_once(self.permit(lease, d.Operation.POSTGRES, d.Phase.OLD), None)
        pg_id = self.core._cap(pg, c.CreatedReceipt)['id']
        fixture_request(self.daemon_socket, 'mutate', pg_id, 'State',
                        json.dumps({'Status': 'running', 'Running': True}))
        self.core._register_running(pg)
        probe = self.create.create_once(self.permit(lease, d.Operation.NETWORK_OLD, d.Phase.OLD, (pg,)), None)
        probe_id = self.core._cap(probe, c.CreatedReceipt)['id']
        self.assertTrue(hasattr(self.core, '_remove_receipt'), 'normal typed lifecycle hook missing')
        self.assertTrue(self.core._remove_receipt(probe))
        state = self.state()['containers']
        self.assertIn(pg_id, state)
        self.assertNotIn(probe_id, state)
        rows = [json.loads(row) for row in (self.run_path / 'container-ledger').read_bytes().splitlines()]
        self.assertEqual(rows[-1]['kind'], 'REMOVED')
        self.assert_category('ownership_unknown', lambda: self.core._remove_receipt(probe))
        self.source = c._SourceView(self.source.root_fd, self.source.root_path,
            self.source.source_sha256, self.source.profile_sha256, self.source.controller_invocation,
            self.source.docker_socket, self.source.socket_identity, time.monotonic() + 119)
        self.core.source = self.source
        self.assert_category('command_timeout', lambda: self.core._remove_receipt(pg))
        self.assertIn(pg_id, self.state()['containers'])

    def reopen(self):
        self.core.close()
        end = time.monotonic() + 3
        while self.owner.recovery_process_cleanup(self.reservation, self.view.controller_invocation) \
                is not d._PROCESS_CLEANUP and time.monotonic() < end:
            select.select([], [], [], .005)
        self.core = c._CleanupCore(self.owner, commands=self.command_type)
        self.addCleanup(self.core.close)
        self.create, self.cleanup = self.core.authorities()
        return self.cleanup.reopen_unfinished()

    def assert_category(self, category, action):
        with self.assertRaises(c.CleanupError) as caught:
            action()
        self.assertEqual(str(caught.exception), category)
        self.assertNotIn(MARKER, repr(caught.exception))
        self.assertIsNone(caught.exception.__cause__)

    def test_native_factories_reject_before_any_io(self):
        with patch.object(os, 'open', side_effect=AssertionError('I/O')):
            self.assert_category('source_closure', lambda: c.CreateAuthority.open(object()))
            self.assert_category('source_closure', lambda: c.CleanupAuthority.open(object()))

    def test_capabilities_are_nonconstructible_identity_registered_and_one_shot(self):
        for kind in (c.AdmissionTicket, c.CleanupLease, c.CreatePermit, c.CreatedReceipt,
                     c.RuntimeEnvFD, c.RegisteredDependencies, c.CleanupSession):
            self.assert_category('ownership_unknown', kind)
        lease = self.begin()
        forged = object.__new__(c.CleanupLease)
        forged._core = self.core
        self.assert_category('ownership_unknown', lambda: self.permit(forged))
        permit = self.permit(lease)
        self.create.create_once(permit, None)
        self.assert_category('ownership_unknown', lambda: self.create.create_once(permit, None))
        self.assertEqual(sum(call[0] == 'create' for call in self.state()['calls']), 1)

    def test_durable_order_full_projection_and_receipt_before_ack(self):
        lease, receipt = self.create_identity()
        rows = self.core._lease_record(lease)['ledger'].records
        self.assertEqual([r['kind'] for r in rows], ['HEADER', 'INTENT', 'DISPATCH', 'ID', 'RECEIPT'])
        self.assertEqual(rows[1]['data']['spec_sha256'], d._hash(d._json(rows[1]['data']['spec'])))
        self.assertIs(type(receipt), c.CreatedReceipt)
        info = (self.run_path / 'container-ledger').lstat()
        self.assertEqual((info.st_uid, stat.S_IMODE(info.st_mode), info.st_nlink), (0, 0o600, 1))
        self.assertNotIn(MARKER.encode(), (self.run_path / 'container-ledger').read_bytes())

    def test_startup_registry_and_illegal_phase_reject_before_ledger_or_daemon(self):
        lease = self.begin()
        before = (self.run_path / 'container-ledger').read_bytes()
        for operation in (d.Operation.SSH_OLD, d.Operation.SSH_TARGET,
                          d.Operation.HELD_OLD, d.Operation.HELD_TARGET):
            dependencies = self.core._register_dependencies(lease, d.Phase.OLD, ())
            self.assert_category('startup_profile_unverified', lambda: self.create.authorize(
                lease, operation, d.Phase.OLD, dependencies))
        self.assert_category('container_spec', lambda: self.permit(lease, d.Operation.POSTGRES, d.Phase.RUN))
        self.assertEqual(before, (self.run_path / 'container-ledger').read_bytes())
        self.assertEqual(self.state()['calls'], [])

    def test_oom_nonzero_exit_owned_container_is_still_cleanable(self):
        lease, _ = self.create_identity()
        cid = next(iter(self.state()['containers']))
        fixture_request(self.daemon_socket, 'mutate', cid, 'State', json.dumps(
            {'Status': 'exited', 'Running': False, 'ExitCode': 137, 'OOMKilled': True}))
        session = self.cleanup.for_live_lease(lease)
        self.assertTrue(session.reconcile().complete)
        self.assertTrue(session.remove_verified().complete)
        self.assertTrue(session.verify_empty().complete)
        self.assertEqual(self.state()['containers'], {})

    def test_foreign_same_label_and_projection_drift_never_removed(self):
        lease, _ = self.create_identity()
        fixture_request(self.daemon_socket, 'foreign', 'c' * 64)
        session = self.cleanup.for_live_lease(lease)
        self.assertFalse(session.reconcile().complete)
        self.assertFalse(session.remove_verified().complete)
        self.assertIn('c' * 64, self.state()['containers'])
        self.assertFalse(session.verify_empty().complete)

    def test_lost_response_reopens_for_cleanup_and_never_retries_or_mints_receipt(self):
        fixture_request(self.daemon_socket, 'mode', 'lost')
        lease = self.begin()
        self.assert_category('command_failed', lambda: self.create.create_once(self.permit(lease), None))
        sessions = self.reopen()
        self.assertEqual(len(sessions), 1)
        session = sessions[0]
        for forbidden in ('create', 'create_once', 'start', 'exec', 'retry', 'restore', 'resume', 'authorize'):
            self.assertFalse(hasattr(session, forbidden))
        self.assertTrue(session.reconcile().complete)
        self.assertTrue(session.remove_verified().complete)
        self.assertTrue(session.verify_empty().complete)
        rows = session._core._session_record(session)['ledger'].records
        self.assertNotIn('RECEIPT', [r['kind'] for r in rows])
        self.assertEqual(sum(call[0] == 'create' for call in self.state()['calls']), 1)

    def test_dispatch_authorized_empty_query_is_ambiguous_and_blocks_admission(self):
        lease = self.begin()
        permit = self.permit(lease)
        with patch.object(self.command_type, 'create', side_effect=self.dispatch_without_create):
            self.assert_category('command_failed', lambda: self.create.create_once(permit, None))
        session = self.reopen()[0]
        self.assertFalse(session.reconcile().complete)
        self.assertFalse(session.verify_empty().complete)
        self.assert_category('cleanup_incomplete', self.create.require_clear_admission)
        self.assertTrue((self.run_path / 'container-ledger').exists())

    @staticmethod
    def dispatch_without_create(spec, env_fd, dispatch):
        dispatch()
        raise RuntimeError(MARKER)

    def test_torn_final_record_preserves_dispatch_authorization(self):
        lease = self.begin()
        with patch.object(self.command_type, 'create', side_effect=self.dispatch_without_create):
            self.assert_category('command_failed', lambda: self.create.create_once(self.permit(lease), None))
        with (self.run_path / 'container-ledger').open('ab') as stream:
            stream.write(b'{"kind":"ID"')
        session = self.reopen()[0]
        self.assertFalse(session.reconcile().complete)
        self.assertEqual(session._core._session_record(session)['transactions'][0]['state'], 'DISPATCH')

    def test_ledger_corruption_duplicate_and_identity_replacement_reject(self):
        self.create_identity()
        path = self.run_path / 'container-ledger'
        data = path.read_bytes()
        lines = data.splitlines(keepends=True)
        path.write_bytes(b''.join(lines + [lines[-1]]))
        self.assert_category('ledger_invalid', self.reopen)

    def test_replaced_material_blocks_recovery_without_reading_secret_values(self):
        lease = self.begin()
        self.create.register_phase(lease, self.phase_caps[d.Phase.OLD])
        self.create.create_once(self.permit(lease, d.Operation.POSTGRES, d.Phase.OLD), None)
        path = self.leaves[d.Phase.OLD, 'runtime.env']
        path.rename(path.with_name('previous-env'))
        path.write_bytes(b'PUBLIC replacement')
        path.chmod(0o600)
        os.chown(path, 9999, 9999)
        self.assert_category('stage_identity', self.reopen)

    def test_mutable_directory_mtime_changes_do_not_break_identity(self):
        lease = self.begin()
        self.create.register_phase(lease, self.phase_caps[d.Phase.OLD])
        self.create.create_once(self.permit(lease, d.Operation.POSTGRES, d.Phase.OLD), None)
        (self.leaves[d.Phase.OLD, 'storage'] / 'new-file').write_bytes(b'PUBLIC')
        session = self.reopen()[0]
        self.assertTrue(session.reconcile().complete)

    def test_admission_requires_container_and_resource_disappearance_and_exclusive_lock(self):
        lease, _ = self.create_identity()
        session = self.cleanup.for_live_lease(lease)
        session.reconcile()
        session.remove_verified()
        session.verify_empty()
        self.owner.gone = False
        self.assert_category('cleanup_incomplete', self.create.require_clear_admission)
        other = c._CleanupCore(self.owner, commands=self.command_type)
        self.addCleanup(other.close)
        self.assert_category('admission_busy', other.authorities()[0].require_clear_admission)
        self.owner.gone = True
        self.assertIs(type(self.create.require_clear_admission()), c.AdmissionTicket)

    def test_runtime_env_is_independent_readonly_offset_zero_and_controller_reads_zero(self):
        lease = self.begin()
        pg = self.create.create_once(self.permit(lease, d.Operation.POSTGRES, d.Phase.OLD), None)
        fixture_request(self.daemon_socket, 'mutate', next(iter(self.state()['containers'])),
                        'State', json.dumps({'Status': 'running', 'Running': True}))
        self.core._register_running(pg)
        dependencies = (pg,)
        permit = self.permit(lease, d.Operation.BASELINE_OLD, d.Phase.OLD, dependencies)
        env = self.core._register_runtime_env(lease, d.Phase.OLD)
        phase = self.view.phases[0]
        original_fd = next(leaf.fd for leaf in phase.leaves if leaf.name == 'runtime.env')
        os.lseek(original_fd, 7, os.SEEK_SET)
        real_read, real_pread = os.read, os.pread
        reads = []
        protected = {(path.lstat().st_dev, path.lstat().st_ino)
                     for (phase_key, name), path in self.leaves.items()
                     if name in ('runtime.env', 'hmac.key', 'coolify.dump')}
        def guard(fd):
            info = os.fstat(fd)
            if (info.st_dev, info.st_ino) in protected:
                reads.append(fd)
        def read(fd, amount):
            guard(fd)
            return real_read(fd, amount)
        def pread(fd, amount, offset):
            guard(fd)
            return real_pread(fd, amount, offset)
        with patch.object(os, 'read', side_effect=read), patch.object(os, 'pread', side_effect=pread):
            self.create.create_once(permit, env)
        self.assertEqual(reads, [])
        self.assertEqual(os.lseek(original_fd, 0, os.SEEK_CUR), 7)
        argv = next(call[1] for call in reversed(self.state()['calls']) if call[0] == 'create')
        self.assertEqual(sum(arg.startswith('--env-file=/proc/self/fd/') for arg in argv), 1)

    def test_dependency_and_account_constraints_reject_before_dispatch(self):
        lease = self.begin()
        self.assert_category('network_isolation', lambda: self.permit(lease, d.Operation.REDIS, d.Phase.OLD))
        from dataclasses import replace
        images = list(self.view.images)
        images[0] = (*images[0][:3], 0, 0)
        self.view = replace(self.view, images=((images[0][0], images[0][1], 'sha256:' + 'c' * 64, 0, 0),
                                             *images[1:]))
        self.assert_category('stage_identity', lambda: self.permit(lease, d.Operation.IDENTITY_OLD, d.Phase.RUN))
        self.assert_category('image_identity', lambda: c._projection(d.Operation.POSTGRES,
            d.Phase.OLD, self.run_token, '8' * 32, self.run_path, self.view.parent_slice, tuple(images), None))
        self.assertEqual(self.state()['calls'], [])

    def test_supervisor_output_fault_consumes_permit_keeps_first_cause(self):
        fixture_request(self.daemon_socket, 'mode', 'flood')
        lease = self.begin()
        permit = self.permit(lease)
        self.assert_category('output_bound', lambda: self.create.create_once(permit, None))
        self.assert_category('ownership_unknown', lambda: self.create.create_once(permit, None))
        session = self.cleanup.for_live_lease(lease)
        observation = session.reconcile()
        self.assertEqual(observation.first_failure, 'output_bound')

    def test_boolean_process_cleanup_evidence_cannot_clear_ambiguity(self):
        lease = self.begin()
        def failure(spec, env_fd, dispatch):
            dispatch()
            error = d.DockerError('command_timeout')
            error.cleanup_complete = True
            error._process_cleanup = True
            raise error
        with patch.object(self.command_type, 'create', side_effect=failure):
            self.assert_category('command_timeout', lambda: self.create.create_once(self.permit(lease), None))
        record = self.core._lease_record(lease)
        self.assertFalse(record['process_clean'])
        self.assertFalse(self.cleanup.for_live_lease(lease).verify_empty().complete)

    def kill_controller_at(self, point):
        self.core.close()
        read_fd, write_fd = os.pipe()
        pid = os.fork()
        if pid == 0:
            os.close(read_fd)
            core = c._CleanupCore(self.owner, commands=self.command_type)
            create, _ = core.authorities()
            original_append, original_mint, original_fsync = c._Ledger.append, core._mint, os.fsync
            def pause():
                os.write(write_fd, b'R')
                while True:
                    signal.pause()
            def append(ledger, kind, data):
                if point == 'before-dispatch' and kind == 'DISPATCH':
                    pause()
                if point == 'after-dispatch-before-id' and kind == 'ID':
                    pause()
                return original_append(ledger, kind, data)
            def sync(fd):
                if point == 'during-id-fsync' and (self.run_path / 'container-ledger').exists() \
                        and stat.S_ISREG(os.fstat(fd).st_mode):
                    info = os.fstat(fd)
                    ledger_info = (self.run_path / 'container-ledger').lstat()
                    if (info.st_dev, info.st_ino) == (ledger_info.st_dev, ledger_info.st_ino):
                        raw = os.pread(fd, info.st_size, 0)
                        if raw and json.loads(raw.splitlines()[-1])['kind'] == 'ID':
                            pause()
                return original_fsync(fd)
            def mint(kind, data):
                if point == 'before-ack' and kind is c.CreatedReceipt:
                    pause()
                return original_mint(kind, data)
            try:
                with patch.object(c._Ledger, 'append', append), patch.object(os, 'fsync', sync), \
                        patch.object(core, '_mint', mint):
                    lease = self.begin_in_controller(create)
                    dependencies = core._register_dependencies(lease, d.Phase.RUN, ())
                    permit = create.authorize(lease, d.Operation.IDENTITY_OLD, d.Phase.RUN, dependencies)
                    create.create_once(permit, None)
                os._exit(4)
            except BaseException:
                os._exit(5)
        os.close(write_fd)
        try:
            self.assertTrue(select.select([read_fd], [], [], 8)[0], point)
            self.assertEqual(os.read(read_fd, 1), b'R', point)
        finally:
            os.close(read_fd)
            os.kill(pid, signal.SIGKILL)
            _, status = os.waitpid(pid, 0)
        self.assertEqual(os.waitstatus_to_exitcode(status), -signal.SIGKILL)
        return self.reopen()[0]

    def test_real_controller_sigkill_before_dispatch_has_no_daemon_effect(self):
        session = self.kill_controller_at('before-dispatch')
        self.assertFalse(any(call[0] == 'create' for call in self.state()['calls']))
        self.assertTrue(session.reconcile().complete)
        self.assertTrue(session.remove_verified().complete)
        self.assertTrue(session.verify_empty().complete)

    def test_real_controller_sigkill_after_dispatch_before_id_reopens_cleanup_only(self):
        session = self.kill_controller_at('after-dispatch-before-id')
        self.assertEqual(len(self.state()['containers']), 1)
        self.assertTrue(session.reconcile().complete)
        self.assertTrue(session.remove_verified().complete)
        self.assertTrue(session.verify_empty().complete)
        rows = session._core._session_record(session)['ledger'].records
        self.assertNotIn('RECEIPT', [row['kind'] for row in rows])

    def test_real_controller_sigkill_during_id_fsync_retains_cleanup_authority(self):
        session = self.kill_controller_at('during-id-fsync')
        self.assertTrue(session.reconcile().complete)
        self.assertTrue(session.remove_verified().complete)
        self.assertTrue(session.verify_empty().complete)

    def test_real_controller_sigkill_before_ack_retains_durable_receipt(self):
        session = self.kill_controller_at('before-ack')
        rows = session._core._session_record(session)['ledger'].records
        self.assertEqual(rows[-1]['kind'], 'RECEIPT')
        self.assertTrue(session.reconcile().complete)
        self.assertTrue(session.remove_verified().complete)
        self.assertTrue(session.verify_empty().complete)

    def test_engine_sigkill_does_not_cancel_controller_owned_create(self):
        fixture_request(self.daemon_socket, 'mode', 'delay')
        engine = os.fork()
        if engine == 0:
            while True:
                signal.pause()
        self.core.close()
        read_fd, write_fd = os.pipe()
        controller = os.fork()
        if controller == 0:
            os.close(read_fd)
            try:
                core = c._CleanupCore(self.owner, commands=self.command_type)
                create, _ = core.authorities()
                lease = self.begin_in_controller(create)
                deps = core._register_dependencies(lease, d.Phase.RUN, ())
                receipt = create.create_once(create.authorize(lease, d.Operation.IDENTITY_OLD, d.Phase.RUN, deps), None)
                os.write(write_fd, b'A' if type(receipt) is c.CreatedReceipt else b'F')
                core.close()
                os._exit(0)
            except BaseException:
                os._exit(5)
        os.close(write_fd)
        try:
            end = time.monotonic() + 5
            while not self.control.get('creating') and time.monotonic() < end:
                select.select([], [], [], .005)
            self.assertTrue(self.control.get('creating'))
            os.kill(engine, signal.SIGKILL)
            _, status = os.waitpid(engine, 0)
            engine = None
            self.assertEqual(os.waitstatus_to_exitcode(status), -signal.SIGKILL)
            self.assertTrue(select.select([read_fd], [], [], 8)[0])
            self.assertEqual(os.read(read_fd, 1), b'A')
            _, status = os.waitpid(controller, 0)
            controller = None
            self.assertEqual(os.waitstatus_to_exitcode(status), 0)
        finally:
            os.close(read_fd)
            for pid in (engine, controller):
                if pid is not None:
                    try:
                        os.kill(pid, signal.SIGKILL)
                        os.waitpid(pid, 0)
                    except ProcessLookupError:
                        pass
        session = self.reopen()[0]
        self.assertTrue(session.reconcile().complete)
        self.assertTrue(session.remove_verified().complete)
        self.assertTrue(session.verify_empty().complete)

    def test_late_create_empty_query_remains_incomplete_until_exact_container_resolves(self):
        fixture_request(self.daemon_socket, 'mode', 'late')
        lease = self.begin()
        self.assert_category('command_failed', lambda: self.create.create_once(self.permit(lease), None))
        session = self.cleanup.for_live_lease(lease)
        self.assertFalse(session.reconcile().complete)
        self.assertFalse(session.verify_empty().complete)
        self.assertTrue((self.run_path / 'container-ledger').exists())
        end = time.monotonic() + 5
        while not self.state()['containers'] and time.monotonic() < end:
            select.select([], [], [], .02)
        self.assertTrue(session.reconcile().complete)
        self.assertTrue(session.remove_verified().complete)
        self.assertTrue(session.verify_empty().complete)

    def test_duplicate_pending_permits_cannot_bypass_single_active_identity(self):
        lease = self.begin()
        first = self.permit(lease)
        second = self.permit(lease, d.Operation.IDENTITY_TARGET)
        self.create.create_once(first, None)
        self.assert_category('resource_isolation', lambda: self.create.create_once(second, None))
        self.assertEqual(sum(call[0] == 'create' for call in self.state()['calls']), 1)

    def test_dispatch_fsync_failure_preserves_static_cause_before_child_exec(self):
        lease = self.begin()
        permit = self.permit(lease)
        ledger = self.core._lease_record(lease)['ledger']
        original = os.fsync
        def fail(fd):
            if fd == ledger.fd:
                raw = os.pread(fd, os.fstat(fd).st_size, 0)
                if json.loads(raw.splitlines()[-1])['kind'] == 'DISPATCH':
                    raise OSError(MARKER)
            return original(fd)
        with patch.object(os, 'fsync', side_effect=fail):
            self.assert_category('ledger_invalid', lambda: self.create.create_once(permit, None))
        self.assertFalse(any(call[0] == 'create' for call in self.state()['calls']))
        # Failed ledger fsync may leave bytes the live writer cannot authenticate.
        # Diagnostics must preserve the first category even when guard also fails.
        self.assertEqual(self.core._lease_record(lease)['first_failure'], 'ledger_invalid')

    def test_name_or_intent_collision_rejects_even_when_other_selector_differs(self):
        lease = self.begin()
        permit = self.permit(lease)
        issued = self.core._cap(permit, c.CreatePermit)
        # A foreign daemon object has the exact reserved name but another intent.
        foreign = copy.deepcopy(issued['spec'])
        foreign['Labels']['sifto.intent'] = 'f' * 32
        peer = socket.socket(socket.AF_UNIX)
        peer.connect(str(self.daemon_socket))
        peer.sendall(json.dumps(d.OwnedDocker._argv(foreign, None)).encode() + b'\n')
        while peer.recv(65536):
            pass
        peer.close()
        self.assert_category('stage_collision', lambda: self.create.create_once(permit, None))
        self.assertEqual([row['kind'] for row in self.core._lease_record(lease)['ledger'].records],
                         ['HEADER', 'FAILURE'])

    def test_independent_fixed_projection_constraints_reject_every_drifted_key(self):
        spec = c._projection(d.Operation.IDENTITY_OLD, d.Phase.RUN, self.run_token, '8' * 32,
                             self.run_path, self.view.parent_slice, self.view.images, None)
        observed = {**copy.deepcopy(spec), 'Id': 'a' * 64,
                    'State': {'Status': 'exited', 'Running': False, 'ExitCode': 137, 'OOMKilled': True}}
        c._owned_projection(spec, 'a' * 64, observed)
        for key, value in spec.items():
            drift = copy.deepcopy(observed)
            if type(value) is str:
                drift[key] = value + '-foreign'
            elif type(value) is bool:
                drift[key] = not value
            elif type(value) is int:
                drift[key] = value + 1
            elif type(value) is list:
                drift[key] = [*value, 'foreign']
            elif type(value) is dict:
                drift[key] = {**value, 'foreign': 'foreign'}
            with self.subTest(key=key):
                self.assert_category('container_spec', lambda: c._owned_projection(spec, 'a' * 64, drift))

    def test_recovery_boolean_process_witness_never_authorizes_removal_or_admission(self):
        self.create_identity()
        self.control['process_boolean'] = True
        self.core.close()
        self.core = c._CleanupCore(self.owner, commands=self.command_type)
        self.addCleanup(self.core.close)
        self.create, self.cleanup = self.core.authorities()
        session = self.cleanup.reopen_unfinished()[0]
        self.assertFalse(session.reconcile().complete)
        self.assertFalse(session.remove_verified().complete)
        self.assertEqual(len(self.state()['containers']), 1)
        self.assert_category('cleanup_incomplete', self.create.require_clear_admission)

    def test_recovery_live_controller_helper_blocks_until_exact_process_witness(self):
        lease, _ = self.create_identity()
        helper = os.fork()
        if helper == 0:
            self.core.close()
            os.setsid()
            while True:
                signal.pause()
        end = time.monotonic() + 3
        while os.getsid(helper) != helper and time.monotonic() < end:
            select.select([], [], [], .005)
        helpers = list(self.control.get('helpers', []))
        helpers.append((helper, d._start_token(helper).hex()))
        self.control['helpers'] = helpers
        self.core.close()
        self.core = c._CleanupCore(self.owner, commands=self.command_type)
        self.addCleanup(self.core.close)
        self.create, self.cleanup = self.core.authorities()
        try:
            session = self.cleanup.reopen_unfinished()[0]
            self.assertFalse(session.reconcile().complete)
            self.assertFalse(session.remove_verified().complete)
            self.assertEqual(len(self.state()['containers']), 1)
            os.kill(helper, signal.SIGKILL)
            os.waitpid(helper, 0)
            helper = None
            self.assertTrue(session.reconcile().complete)
            self.assertTrue(session.remove_verified().complete)
            self.assertTrue(session.verify_empty().complete)
        finally:
            if helper is not None:
                os.kill(helper, signal.SIGKILL)
                os.waitpid(helper, 0)

    def test_base_lease_identity_does_not_require_future_phase_leaves_or_accounts(self):
        from dataclasses import replace
        self.view = replace(self.view,
            images=tuple((*image[:3], None, None) for image in self.view.images),
            phases=tuple(replace(phase, leaves=()) for phase in self.view.phases))
        lease, _ = self.create_identity()
        session = self.cleanup.for_live_lease(lease)
        self.assertTrue(session.reconcile().complete)
        self.assertTrue(session.remove_verified().complete)
        self.assertTrue(session.verify_empty().complete)

    def test_phase_registration_is_durable_owner_capability_and_ordered_without_reseal(self):
        lease = self.begin()
        old, fresh = object(), object()
        self.owner.guard_phase = lambda reservation, issued: self.view.phases[
            0 if issued is old else 1] if reservation is self.reservation and issued in (old, fresh) else None
        self.assert_category('ownership_unknown', lambda: self.create.register_phase(lease, object()))
        self.assert_category('stage_identity', lambda: self.create.register_phase(lease, fresh))
        self.create.register_phase(lease, old)
        rows = self.core._lease_record(lease)['ledger'].records
        self.assertEqual(rows[-1]['kind'], 'PHASE')
        self.assertEqual(rows[-1]['data']['phase'], 'old')
        self.assert_category('stage_collision', lambda: self.create.register_phase(lease, old))
        dependencies = self.core._register_dependencies(lease, d.Phase.OLD, ())
        permit = self.create.authorize(lease, d.Operation.POSTGRES, d.Phase.OLD, dependencies)
        self.assert_category('resource_isolation', lambda: self.create.register_phase(lease, fresh))
        self.create.create_once(permit, None)
        self.assert_category('resource_isolation', lambda: self.create.register_phase(lease, fresh))
        session = self.cleanup.for_live_lease(lease)
        self.assertTrue(session.reconcile().complete)
        self.assertTrue(session.remove_verified().complete)
        self.create.register_phase(lease, fresh)
        self.assert_category('stage_collision', lambda: self.create.register_phase(lease, fresh))

    def test_unregistered_fresh_material_can_arrive_later_without_header_rewrite(self):
        from dataclasses import replace
        original = self.view
        old, fresh = object(), object()
        self.owner.guard_phase = lambda reservation, issued: self.view.phases[
            0 if issued is old else 1] if reservation is self.reservation and issued in (old, fresh) else None
        self.view = replace(original, phases=(original.phases[0], replace(original.phases[1], leaves=())))
        lease = self.begin()
        header = copy.deepcopy(self.core._lease_record(lease)['ledger'].records[0])
        self.create.register_phase(lease, old)
        self.view = original
        self.create.register_phase(lease, fresh)
        self.assertEqual(header, self.core._lease_record(lease)['ledger'].records[0])
        session = self.reopen()[0]
        self.assertTrue(session.reconcile().complete)
        self.assertTrue(session.verify_empty().complete)

    def test_inherited_controller_capability_cannot_execute_from_forked_engine(self):
        lease = self.begin()
        permit = self.permit(lease)
        read_fd, write_fd = os.pipe()
        child = os.fork()
        if child == 0:
            os.close(read_fd)
            try:
                self.create.create_once(permit, None)
                os.write(write_fd, b'BAD')
            except c.CleanupError as error:
                os.write(write_fd, str(error).encode())
            os._exit(0)
        os.close(write_fd)
        value = os.read(read_fd, 64)
        os.close(read_fd)
        os.waitpid(child, 0)
        self.assertEqual(value, b'ownership_unknown')
        self.assertFalse(any(call[0] == 'create' for call in self.state()['calls']))

    def test_lost_response_first_static_failure_survives_ledger_reopen(self):
        fixture_request(self.daemon_socket, 'mode', 'lost')
        lease = self.begin()
        self.assert_category('command_failed', lambda: self.create.create_once(self.permit(lease), None))
        session = self.reopen()[0]
        self.assertEqual(session.reconcile().first_failure, 'command_failed')
        self.assertEqual(session.remove_verified().first_failure, 'command_failed')

    def test_deadline_is_finite_and_pinned_by_durable_header(self):
        from dataclasses import replace
        original = self.view
        self.view = replace(original, deadline=float('inf'))
        self.assert_category('stage_identity', lambda: self.core._view(self.reservation))
        self.view = original
        lease, _ = self.create_identity()
        self.view = replace(original, deadline=original.deadline + 100)
        self.assert_category('stage_identity', lambda: self.cleanup.for_live_lease(lease))

    def test_supervisor_real_deadline_reaps_cli_and_consumes_create_without_retry(self):
        fixture_request(self.daemon_socket, 'mode', 'deadline')
        lease = self.begin()
        permit = self.permit(lease)
        supervise = d._supervise
        def bounded(argv, **kwargs):
            if 'create' in argv:
                kwargs['deadline'] = time.monotonic() + .08
            return supervise(argv, **kwargs)
        with patch.object(d, '_supervise', side_effect=bounded):
            self.assert_category('command_timeout', lambda: self.create.create_once(permit, None))
        self.assertTrue(self.core._lease_record(lease)['process_clean'])
        self.assert_category('ownership_unknown', lambda: self.create.create_once(permit, None))

    def test_replaced_ledger_inode_is_not_adopted(self):
        lease, _ = self.create_identity()
        ledger = self.run_path / 'container-ledger'
        ledger.rename(self.run_path / 'previous-ledger')
        ledger.write_bytes((self.run_path / 'previous-ledger').read_bytes())
        ledger.chmod(0o600)
        self.assert_category('ledger_invalid', lambda: self.cleanup.for_live_lease(lease))

    def test_ledger_total_bound_is_rejected(self):
        self.create_identity()
        ledger = self.run_path / 'container-ledger'
        ledger.write_bytes(b'x' * (1024 * 1024 + 1))
        self.assert_category('ledger_invalid', self.reopen)

    def test_cleanup_budget_expiry_keeps_unresolved_dispatch_ledger_and_no_admission(self):
        lease = self.begin()
        with patch.object(self.command_type, 'create', side_effect=self.dispatch_without_create):
            self.assert_category('command_failed', lambda: self.create.create_once(self.permit(lease), None))
        session = self.cleanup.for_live_lease(lease)
        self.core._session_record(session)['cleanup_deadline'] = time.monotonic() - .1
        observation = session.reconcile()
        self.assertFalse(observation.complete)
        self.assertEqual(observation.first_failure, 'command_failed')
        self.assertFalse(session.verify_empty().complete)
        self.assertTrue((self.run_path / 'container-ledger').exists())
        self.assert_category('cleanup_incomplete', self.create.require_clear_admission)

    def test_ledger_hardlink_and_owner_marker_replacement_reject(self):
        lease, _ = self.create_identity()
        ledger = self.run_path / 'container-ledger'
        os.link(ledger, self.run_path / 'foreign-link')
        self.assert_category('ledger_invalid', lambda: self.cleanup.for_live_lease(lease))
        (self.run_path / 'foreign-link').unlink()
        marker = self.run_path / 'owner-marker'
        marker.rename(self.run_path / 'previous-marker')
        marker.write_bytes(b'PUBLIC-owner-marker')
        marker.chmod(0o600)
        self.assert_category('stage_identity', self.reopen)

    def test_canonical_illegal_transition_and_duplicate_json_keys_are_not_torn_records(self):
        self.create_identity()
        ledger = self.run_path / 'container-ledger'
        original = ledger.read_bytes()
        rows = [json.loads(line) for line in original.splitlines()]
        rows[-1]['kind'] = 'DISPATCH'
        raw = b''
        for row in rows:
            row['previous'] = d._hash(raw)
            raw += d._json(row) + b'\n'
        ledger.write_bytes(raw)
        self.assert_category('ledger_invalid', self.reopen)
        ledger.write_bytes(original + b'{"sequence":5,"sequence":5}\n')
        self.assert_category('ledger_invalid', self.reopen)

    def test_all_held_fds_close_and_borrowed_owner_descriptors_remain_live(self):
        self.control.get('helpers')  # Initialize the fixture manager's lazy IPC FD.
        before = len(os.listdir('/proc/self/fd'))
        self.create_identity()
        self.core.close()
        self.assertEqual(len(os.listdir('/proc/self/fd')), before)
        for fd in self.fds:
            os.fstat(fd)

    def test_controller_sigkill_with_live_create_helper_recovers_after_helper_reap(self):
        fixture_request(self.daemon_socket, 'mode', 'held-client')
        self.core.close()
        controller = os.fork()
        if controller == 0:
            try:
                core = c._CleanupCore(self.owner, commands=self.command_type)
                create, _ = core.authorities()
                lease = self.begin_in_controller(create)
                deps = core._register_dependencies(lease, d.Phase.RUN, ())
                create.create_once(create.authorize(lease, d.Operation.IDENTITY_OLD, d.Phase.RUN, deps), None)
                core.close()
                os._exit(0)
            except BaseException:
                os._exit(5)
        try:
            end = time.monotonic() + 5
            while not self.control.get('creating') and time.monotonic() < end:
                select.select([], [], [], .005)
            self.assertTrue(self.control.get('creating'))
        finally:
            os.kill(controller, signal.SIGKILL)
            os.waitpid(controller, 0)
        self.core = c._CleanupCore(self.owner, commands=self.command_type)
        self.addCleanup(self.core.close)
        self.create, self.cleanup = self.core.authorities()
        session = self.cleanup.reopen_unfinished()[0]
        self.assertFalse(session.reconcile().complete)
        self.assertFalse(session.remove_verified().complete)
        self.assert_category('cleanup_incomplete', self.create.require_clear_admission)
        end = time.monotonic() + 5
        while self.owner.recovery_process_cleanup(self.reservation, self.view.controller_invocation) \
                is not d._PROCESS_CLEANUP and time.monotonic() < end:
            select.select([], [], [], .01)
        self.assertTrue(session.reconcile().complete)
        self.assertTrue(session.remove_verified().complete)
        self.assertTrue(session.verify_empty().complete)

    def test_first_supervisor_failure_survives_output_unlink_diagnostic_fault(self):
        fixture_request(self.daemon_socket, 'mode', 'flood')
        lease = self.begin()
        permit = self.permit(lease)
        original = os.unlink
        def fail_output(path, *args, **kwargs):
            if path == 'command-3.stdout':
                raise OSError(MARKER)
            return original(path, *args, **kwargs)
        with patch.object(os, 'unlink', side_effect=fail_output):
            self.assert_category('output_bound', lambda: self.create.create_once(permit, None))
        self.assertEqual(self.core._lease_record(lease)['first_failure'], 'output_bound')

    def test_invalid_output_directory_rejection_closes_newly_opened_fd(self):
        output = self.prepared_path / 'container-output'
        output.mkdir(mode=0o777)
        output.chmod(0o777)
        before = len(os.listdir('/proc/self/fd'))
        self.assert_category('stage_identity', self.begin)
        self.core.close()
        self.assertEqual(len(os.listdir('/proc/self/fd')), before)

    def test_phase_epoch_rejects_new_run_or_old_authority_after_later_registration(self):
        lease = self.begin()
        self.create.register_phase(lease, self.phase_caps[d.Phase.OLD])
        self.assert_category('stage_identity', lambda: self.permit(lease))
        self.create.register_phase(lease, self.phase_caps[d.Phase.FRESH])
        self.assert_category('stage_identity', lambda: self.permit(lease, d.Operation.POSTGRES, d.Phase.OLD))

    def test_old_registration_blocks_pending_run_permit_and_active_run_transaction(self):
        lease = self.begin()
        permit = self.permit(lease)
        self.assert_category('resource_isolation', lambda: self.create.register_phase(lease, self.phase_caps[d.Phase.OLD]))
        self.create.create_once(permit, None)
        self.assert_category('resource_isolation', lambda: self.create.register_phase(lease, self.phase_caps[d.Phase.OLD]))
        session = self.cleanup.for_live_lease(lease)
        self.assertTrue(session.reconcile().complete)
        self.assertTrue(session.remove_verified().complete)
        self.create.register_phase(lease, self.phase_caps[d.Phase.OLD])

    def test_stale_run_permit_is_revalidated_at_commit_even_without_active_containers(self):
        lease = self.begin()
        permit = self.permit(lease)
        record = self.core._lease_record(lease)
        # Inject an authenticated durable epoch change between prepare/commit;
        # public registration additionally rejects the outstanding permit.
        self.core._append(record, 'PHASE', self.core._phase_snapshot(self.view, self.view.phases[0]))
        self.assert_category('stage_identity', lambda: self.create.create_once(permit, None))
        self.assertFalse(any(call[0] == 'create' for call in self.state()['calls']))

    def test_ledger_replay_rejects_phase_rollback_and_registration_over_active_run(self):
        lease = self.begin()
        self.create.register_phase(lease, self.phase_caps[d.Phase.OLD])
        spec = c._projection(d.Operation.IDENTITY_OLD, d.Phase.RUN, self.run_token, '8' * 32,
                             self.run_path, self.view.parent_slice, self.view.images, None)
        data = {'intent': '8' * 32, 'name': spec['Name'], 'operation': 'identity_old',
                'phase': 'run', 'spec': spec, 'spec_sha256': d._hash(d._json(spec)), 'dependencies': []}
        self.assert_category('ledger_invalid', lambda: self.core._append(self.core._lease_record(lease), 'INTENT', data))

    def test_copied_ledger_after_fd_loss_rejects_before_daemon_actions(self):
        self.create_identity()
        self.core.close()
        ledger = self.run_path / 'container-ledger'
        original = ledger.read_bytes()
        ledger.rename(self.run_path / 'original-ledger')
        ledger.write_bytes(original)
        ledger.chmod(0o600)
        before = self.state()['calls']
        self.assert_category('ledger_invalid', self.reopen)
        self.assertEqual(self.state()['calls'], before)

    def test_new_controller_invocation_recovers_historical_run_without_main_authority(self):
        self.assertIn('controller_invocation', c._RunView.__dataclass_fields__)
        from dataclasses import replace
        self.create_identity()
        original = self.view.controller_invocation
        self.source = replace(self.source, controller_invocation='e' * 32)
        received = []
        original_witness = self.owner.recovery_process_cleanup
        def witness(reservation, invocation):
            received.append(invocation)
            return original_witness(reservation, invocation)
        self.owner.recovery_process_cleanup = witness
        session = self.reopen()[0]
        self.assertTrue(session.reconcile().complete)
        self.assertTrue(session.remove_verified().complete)
        self.assertTrue(session.verify_empty().complete)
        self.assertTrue(received)
        self.assertEqual(set(received), {original})

    def test_missing_historical_ledger_never_uses_resource_cleanup_true_as_admission(self):
        self.owner.gone = True
        self.install_run()
        self.assert_category('cleanup_incomplete', self.create.require_clear_admission)

    def test_fresh_reservation_between_ticket_and_begin_is_exclusively_authenticated(self):
        prepared = self.root / 'prepared-run'
        ticket = self.create.require_clear_admission()
        prepared.rename(self.run_path)
        lease = self.create.begin(ticket, self.reservation)
        self.assertIs(type(lease), c.CleanupLease)
        self.assert_category('ownership_unknown', lambda: self.create.begin(ticket, self.reservation))

    def test_ticket_does_not_exempt_another_new_missing_ledger_directory(self):
        prepared = self.root / 'prepared-run'
        ticket = self.create.require_clear_admission()
        prepared.rename(self.run_path)
        (self.stage / ('9' * 32)).mkdir(mode=0o700)
        self.assert_category('cleanup_incomplete', lambda: self.create.begin(ticket, self.reservation))

    def test_cleanup_deadline_is_minimum_of_original_run_current_source_and_once_only(self):
        self.assertIn('deadline', c._SourceView.__dataclass_fields__)
        from dataclasses import replace
        now = time.monotonic()
        self.view = replace(self.view, deadline=now + 150)
        self.source = replace(self.source, deadline=now + 140)
        self.core.close()
        self.core = c._CleanupCore(self.owner, commands=self.command_type)
        self.addCleanup(self.core.close)
        self.create, self.cleanup = self.core.authorities()
        lease = self.begin()
        with patch.object(c.time, 'monotonic', return_value=now + 135):
            session = self.cleanup.for_live_lease(lease)
        record = self.core._session_record(session)
        self.assertEqual(record['cleanup_deadline'], now + 140)
        later = self.cleanup.for_live_lease(lease)
        self.assertEqual(record['cleanup_deadline'], self.core._session_record(later)['cleanup_deadline'])

    def test_historical_expired_deadline_uses_new_pinned_current_cleanup_budget(self):
        self.assertIn('deadline', c._SourceView.__dataclass_fields__)
        self.assertIn('controller_invocation', c._RunView.__dataclass_fields__)
        from dataclasses import replace
        self.create_identity()
        original_deadline = self.view.deadline
        self.source = replace(self.source, controller_invocation='e' * 32, deadline=original_deadline + 61)
        # Advance the synthetic current clock beyond the immutable old deadline.
        # Only the new source-owner cleanup budget may authorize this recovery.
        with patch.object(c.time, 'monotonic', return_value=original_deadline + 1):
            session = self.reopen()[0]
            record = self.core._session_record(session)
            self.assertLess(record['header']['deadline'], time.monotonic())
            self.assertLessEqual(record['cleanup_deadline'], self.source.deadline)
            self.assertEqual(record['header']['deadline'], original_deadline)
            self.assertTrue(session.reconcile().complete)
            self.assertTrue(session.remove_verified().complete)

    def test_phase_registration_never_refreshes_cleanup_deadline_or_old_session(self):
        lease = self.begin()
        session = self.cleanup.for_live_lease(lease)
        original = self.core._session_record(session)['cleanup_deadline']
        self.create.register_phase(lease, self.phase_caps[d.Phase.OLD])
        self.assert_category('ownership_unknown', session.verify_empty)
        newer = self.cleanup.for_live_lease(lease)
        self.assertEqual(self.core._session_record(newer)['cleanup_deadline'], original)
        self.create.register_phase(lease, self.phase_caps[d.Phase.FRESH])
        final = self.cleanup.for_live_lease(lease)
        self.assertEqual(self.core._session_record(final)['cleanup_deadline'], original)

    def test_historical_reservation_cannot_be_claimed_as_new_current_work(self):
        from dataclasses import replace
        ticket = self.create.require_clear_admission()
        self.install_run()
        self.view = replace(self.view, controller_invocation='d' * 32)
        self.assert_category('ownership_unknown', lambda: self.create.begin(ticket, self.reservation))
        self.assertFalse((self.run_path / 'container-ledger').exists())

    def test_current_source_deadline_cannot_be_refreshed_after_core_open(self):
        from dataclasses import replace
        self.source = replace(self.source, deadline=self.source.deadline + 1)
        self.assert_category('source_closure', self.create.require_clear_admission)

    def assert_drifted_owned_fd_closes(self, role, ownership=False):
        self.control.get('helpers')
        before = len(os.listdir('/proc/self/fd'))
        lease, _ = self.create_identity()
        session = self.cleanup.for_live_lease(lease)
        self.assertTrue(session.reconcile().complete)
        self.assertTrue(session.remove_verified().complete)
        self.assertTrue(session.verify_empty().complete)
        record = self.core._lease_record(lease)
        descriptors = {'ledger': record['ledger'].fd, 'output': record['commands'].output,
                       'lock': self.core.lock_fd}
        paths = {'ledger': self.run_path / 'container-ledger',
                 'output': self.run_path / 'container-output',
                 'lock': self.stage / 'container-controller-lock'}
        if ownership:
            os.fchown(descriptors[role], 1, 2)
        else:
            os.fchmod(descriptors[role], 0o750 if role == 'output' else 0o640)
        guard = record['ledger'].guard if role == 'ledger' else self.core._lock if role == 'lock' \
            else record['commands'].run_empty
        self.assert_category('ledger_invalid' if role == 'ledger' else 'stage_identity', guard)
        self.core.close()
        for fd in descriptors.values():
            with self.assertRaises(OSError):
                os.fstat(fd)
        self.assertEqual(len(os.listdir('/proc/self/fd')), before)
        os.chown(paths[role], 0, 0)
        os.chmod(paths[role], 0o700 if role == 'output' else 0o600)
        other = c._CleanupCore(self.owner, commands=self.command_type)
        self.addCleanup(other.close)
        self.assertIs(type(other.authorities()[0].require_clear_admission()), c.AdmissionTicket)

    def test_ledger_mode_drift_is_rejected_but_our_fd_still_closes(self):
        self.assert_drifted_owned_fd_closes('ledger')

    def test_output_mode_drift_is_rejected_but_our_fd_still_closes(self):
        self.assert_drifted_owned_fd_closes('output')

    def test_lock_mode_drift_is_rejected_but_close_releases_flock(self):
        self.assert_drifted_owned_fd_closes('lock')

    def test_ledger_owner_drift_is_rejected_but_our_fd_still_closes(self):
        self.assert_drifted_owned_fd_closes('ledger', ownership=True)

    def test_output_owner_drift_is_rejected_but_our_fd_still_closes(self):
        self.assert_drifted_owned_fd_closes('output', ownership=True)

    def test_lock_owner_drift_is_rejected_but_close_releases_flock(self):
        self.assert_drifted_owned_fd_closes('lock', ownership=True)

    def test_close_preserves_foreign_reused_ledger_output_and_lock_fd_numbers(self):
        lease = self.begin()
        record = self.core._lease_record(lease)
        descriptors = [record['ledger'].fd, record['commands'].output, self.core.lock_fd]
        foreign = []
        try:
            for index, fd in enumerate(descriptors):
                os.close(fd)
                path = self.root / ('foreign-' + str(index))
                path.write_bytes(b'PUBLIC foreign FD')
                opened = os.open(path, os.O_RDONLY | os.O_CLOEXEC)
                if opened != fd:
                    os.dup2(opened, fd)
                    os.close(opened)
                foreign.append(fd)
            self.core.close()
            for fd in foreign:
                self.assertEqual(os.pread(fd, 17, 0), b'PUBLIC foreign FD')
            for fd in self.fds:
                os.fstat(fd)
        finally:
            for fd in foreign:
                os.close(fd)

    def test_cleanup_session_cannot_extend_authorize_past_fixed_main_cutoff(self):
        lease = self.begin()
        base = self.view.deadline - 200
        with patch.object(c.time, 'monotonic', return_value=base + 70):
            self.cleanup.for_live_lease(lease)
        with patch.object(c.time, 'monotonic', return_value=base + 90):
            self.assert_category('command_timeout', lambda: self.permit(lease))
        self.assertFalse(any(call[0] == 'create' for call in self.state()['calls']))

    def test_permit_before_cutoff_is_consumed_on_late_commit_and_cleanup_still_works(self):
        lease = self.begin()
        permit = self.permit(lease)
        base = self.view.deadline - 200
        with patch.object(c.time, 'monotonic', return_value=base + 70):
            session = self.cleanup.for_live_lease(lease)
        with patch.object(c.time, 'monotonic', return_value=base + 90):
            self.assert_category('command_timeout', lambda: self.create.create_once(permit, None))
            self.assert_category('ownership_unknown', lambda: self.create.create_once(permit, None))
            self.assertTrue(session.reconcile().complete)
            self.assertTrue(session.remove_verified().complete)
            self.assertTrue(session.verify_empty().complete)
        self.assertFalse(any(call[0] == 'create' for call in self.state()['calls']))

    def test_cleanup_fixed_commands_work_after_main_cutoff_without_minting_new_main_authority(self):
        lease, _ = self.create_identity()
        base = self.view.deadline - 200
        with patch.object(c.time, 'monotonic', return_value=base + 70):
            session = self.cleanup.for_live_lease(lease)
        with patch.object(c.time, 'monotonic', return_value=base + 90):
            self.assertTrue(session.reconcile().complete)
            self.assertTrue(session.remove_verified().complete)
            self.assertTrue(session.verify_empty().complete)
        self.assertEqual(self.state()['containers'], {})

    def test_main_collision_and_inspection_cannot_borrow_cleanup_budget(self):
        lease, _ = self.create_identity()
        record = self.core._lease_record(lease)
        transaction = record['transactions'][0]
        base = self.view.deadline - 200
        with patch.object(c.time, 'monotonic', return_value=base + 70):
            self.cleanup.for_live_lease(lease)
        before = self.state()['calls']
        with patch.object(c.time, 'monotonic', return_value=base + 90):
            self.assert_category('command_timeout', lambda: record['commands'].collision(transaction))
            self.assert_category('command_timeout', lambda: record['commands'].inspect(transaction['id']))
        self.assertEqual(self.state()['calls'], before)

    def test_slow_dispatch_fsync_crossing_main_cutoff_never_releases_create_exec(self):
        lease = self.begin()
        permit = self.permit(lease)
        base = self.view.deadline - 200
        now = [base + 70]
        with patch.object(c.time, 'monotonic', side_effect=lambda: now[0]):
            session = self.cleanup.for_live_lease(lease)
            append = c._Ledger.append
            def slow(ledger, kind, data):
                result = append(ledger, kind, data)
                if kind == 'DISPATCH':
                    now[0] = base + 90
                return result
            with patch.object(c._Ledger, 'append', slow):
                self.assert_category('command_timeout', lambda: self.create.create_once(permit, None))
            self.assertFalse(session.verify_empty().complete)
        self.assertFalse(any(call[0] == 'create' for call in self.state()['calls']))


class ProjectionDifferentialTests(unittest.TestCase):
    def test_every_supported_projection_matches_actual_n1_specimens(self):
        self.assertIsNotNone(c, 'controller-owned cleanup authority module is missing')
        fixture = n1.OwnedDockerTests('test_unknown_id_is_never_removed')
        fixture.setUp()
        self.addCleanup(fixture.doCleanups)
        for operation in d.Operation:
            for phase in d.Phase:
                try:
                    role, purpose, _ = d.OwnedDocker._operation(operation, phase)
                except d.DockerError:
                    continue
                run = fixture.owned.allocate_run()
                resources = None
                if phase is not d.Phase.RUN:
                    fixture.populate(run, phase)
                    resources = fixture.owned.seal_phase(run, phase)
                if purpose not in ('identity', 'postgres'):
                    pg = fixture.owned.create(run, operation=d.Operation.POSTGRES, phase=phase, resources=resources)
                    fixture.owned.start(pg.container_id)
                    pg_id = pg.container_id
                else:
                    pg_id = None
                intent = '8' * 32
                expected, _ = fixture.owned._spec(run, operation, phase, resources, intent)
                record = fixture.owned._runs[run]
                images = tuple((r, *fixture.owned._image(r),
                                70 if r == 'postgres' else 999 if r == 'redis' else 9999,
                                70 if r == 'postgres' else 999 if r == 'redis' else 9999)
                               for r in ('old', 'target', 'postgres', 'redis'))
                actual = c._projection(operation, phase, record['token'], intent,
                    fixture.stage / record['token'], str(fixture.context.cgroup_parent), images, pg_id)
                self.assertEqual(actual, expected, (operation, phase))
                fixture.owned.cleanup(run)


if __name__ == '__main__':
    unittest.main()
