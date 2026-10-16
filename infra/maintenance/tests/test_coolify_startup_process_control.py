"""PUBLIC same-PID-namespace kernel proof; never native App authority.

Only this harness assembles the deliberately unconstructible component. It owns
every child; the real S6 selector, container mapping and parent role are NOT proven.
Cleanup is after assertions/result classification and is never natural completion.
"""
import json
import os
from pathlib import Path
import select
import signal
import subprocess
import sys
import tempfile
import time
import unittest
from unittest.mock import patch

from maintenance import coolify_docker as docker
from maintenance import coolify_startup as startup


CHILD = '''
import os, sys, threading, subprocess
event = threading.Event()
threads = [threading.Thread(target=event.wait) for _ in range(2)]
descendants = []
exiting = []
for thread in threads: thread.start()
print("ready", flush=True)
for line in sys.stdin:
    if line.strip() == "quit": break
    if line.strip() == "spawn":
        child = subprocess.Popen([sys.executable, "-u", "-c", "import sys; print('ready', flush=True); sys.stdin.readline()"], stdin=subprocess.PIPE, stdout=subprocess.PIPE, text=True)
        assert child.stdout.readline() == "ready\\n"
        descendants.append(child)
        print(child.pid, flush=True)
    if line.startswith("spawn-exit "):
        code = "import sys; print('ready', flush=True); gate = open(sys.argv[1], 'rb', buffering=0); assert gate.read(1) == b'q'; gate.close()"
        child = subprocess.Popen([sys.executable, "-u", "-c", code, line.strip().split(" ", 1)[1]], stdin=subprocess.DEVNULL, stdout=subprocess.PIPE, text=True)
        assert child.stdout.readline() == "ready\\n"
        exiting.append(child)
        print(child.pid, flush=True)
    if line.strip() == "collect":
        for child in exiting:
            print("natural-descendant-exit=" + str(child.wait(timeout=2)), flush=True)
            child.stdout.close()
        exiting.clear()
for child in exiting:
    print("natural-descendant-exit=" + str(child.wait(timeout=2)), flush=True)
    child.stdout.close()
for child in descendants:
    child.stdin.write("quit\\n"); child.stdin.flush()
    print("descendant-exit=" + str(child.wait(timeout=2)), flush=True)
    child.stdin.close(); child.stdout.close()
event.set()
for thread in threads: thread.join()
'''


class ProcessKernel(unittest.TestCase):
    def setUp(self):
        self.temp = tempfile.TemporaryDirectory()
        self.root = Path(self.temp.name)
        self.children = []
        self.controls = []
        self.expected_exits = {}

    def tearDown(self):
        # Only harness children, AFTER the test's classification/assertions.
        for control in self.controls:
            for name in ('_pidfd', '_procfd', '_initfd', '_sourcefd', '_contextfd', '_dirfd', '_lockfd', '_exefd'):
                value = getattr(control, name, None)
                if value is not None:
                    os.close(value)
        for child in self.children:
            if child.poll() is None:
                with os.fdopen(os.pidfd_open(child.pid), 'rb') as fd:
                    signal.pidfd_send_signal(fd.fileno(), signal.SIGCONT)
                child.stdin.write('quit\n')
                child.stdin.flush()
                try:
                    child.wait(timeout=2)
                except subprocess.TimeoutExpired:
                    child.kill()
                    child.wait(timeout=2)
            child.stdin.close()
            self.assertIn(child.stdout.read(), ('', 'descendant-exit=0\n', 'natural-descendant-exit=0\n'))
            child.stdout.close()
            self.assertEqual(child.returncode, self.expected_exits.get(child.pid, 0))
        self.temp.cleanup()

    def child(self):
        child = subprocess.Popen([sys.executable, '-u', '-c', CHILD], stdin=subprocess.PIPE,
                                 stdout=subprocess.PIPE, text=True)
        self.children.append(child)
        self.assertTrue(select.select([child.stdout], [], [], 2)[0], 'owned startup latch')
        self.assertEqual(child.stdout.readline(), 'ready\n')
        return child

    def control(self, child=None, role='scheduler-worker'):
        child = child or self.child()
        # NO installed entrypoint can construct this dormant component. The
        # harness supplies its OWN child, same namespace and synthetic sources.
        control = object.__new__(docker._FixedRoleProcessControl)
        self.controls.append(control)
        control._role = role
        control._pid = child.pid
        control._pidfd = os.pidfd_open(child.pid)
        control._procfd = os.open('/proc/' + str(child.pid), os.O_RDONLY | os.O_DIRECTORY)
        control._initfd = os.open('/proc/1', os.O_RDONLY | os.O_DIRECTORY)
        source = self.root / ('source-' + str(len(self.controls)))
        source.write_bytes(b'PUBLIC fixed-role source/config component pin\n')
        control._sourcefd = os.open(source, os.O_RDONLY)
        context = self.root / ('context-' + str(len(self.controls)))
        context.write_bytes(b'PUBLIC held barrier/run-generation component pin\n')
        control._contextfd = os.open(context, os.O_RDONLY)
        control._exefd = os.open('/proc/' + str(child.pid) + '/exe', os.O_RDONLY)
        control._dirfd = os.open(self.root, os.O_RDONLY | os.O_DIRECTORY)
        control._lockfd = os.open(self.root / (role + '.lock'), os.O_RDWR | os.O_CREAT, 0o600)
        control._deadline = time.monotonic() + 5
        control._key = b'PUBLIC component HMAC key 32byte!!'
        control._original = None
        control._intent = None
        control._pins = control._read_pins()
        control._selected = control._identity()
        return control

    def test_dormant_constructor_and_native_gate_remain_closed(self):
        self.assertTrue(hasattr(docker, '_FixedRoleProcessControl'), 'K0 component missing')
        with self.assertRaisesRegex(docker.DockerError, '^startup_profile_unverified$'):
            docker._FixedRoleProcessControl()
        with self.assertRaisesRegex(docker.DockerError, '^startup_profile_unverified$'):
            object.__new__(docker.OwnedDocker)._require_verified_startup()
        self.assertFalse(startup.fixed_candidate()['registered'])
        self.assertFalse(startup.fixed_candidate()['production_eligible'])

    def test_running_all_tasks_stop_and_exact_restore(self):
        for role in ('scheduler-worker', 'terminal-server'):
            with self.subTest(role=role):
                control = self.control(role=role)
                frozen = control.freeze()
                self.assertEqual(frozen.category, 'frozen')
                self.assertEqual(frozen.task_count, 3)
                self.assertTrue(all(state == 'T' for _, state in control._tasks()))
                private = json.loads((self.root / (role + '.json')).read_bytes())
                self.assertEqual(private['stage'], 'applied')
                self.assertEqual(private['original'], control._original)
                self.assertTrue(private['restore_required'])
                restored = control.restore()
                self.assertEqual(restored.category, 'restored')
                self.assertFalse(restored.restore_required)
                self.assertTrue(all(state != 'T' for _, state in control._tasks()))

    def test_originally_stopped_is_unchanged_and_never_continued(self):
        child = self.child()
        pidfd = os.pidfd_open(child.pid)
        signal.pidfd_send_signal(pidfd, signal.SIGSTOP)
        limit = time.monotonic() + 1
        while not all(Path('/proc/' + str(child.pid) + '/task/' + tid + '/status').read_text().split('State:\t')[1][0] == 'T'
                      for tid in os.listdir('/proc/' + str(child.pid) + '/task')):
            self.assertLess(time.monotonic(), limit)
        os.close(pidfd)
        control = self.control(child)
        with patch.object(signal, 'pidfd_send_signal', wraps=signal.pidfd_send_signal) as send:
            self.assertEqual(control.freeze().category, 'already-stopped')
            self.assertEqual(control.restore().category, 'unchanged')
            send.assert_not_called()
        self.assertTrue(all(state == 'T' for _, state in control._tasks()))

    def test_result_parser_is_closed_and_bounded(self):
        self.assertTrue(hasattr(startup, 'parse_process_control_result'), 'typed K0 result missing')
        value = dict(version=1, role='scheduler-worker', category='frozen', task_count=3,
                     restore_required=True, maintenance_held=True, identity_hmac='a' * 64)
        parsed = startup.parse_process_control_result(json.dumps(value).encode())
        self.assertEqual(parsed.task_count, 3)
        defects = [dict(value, task_count=True), dict(value, task_count=257), dict(value, extra=0),
                   dict(value, category='unknown'), dict(value, maintenance_held=False),
                   dict(value, restore_required=False), dict(value, role='horizon')]
        for defect in defects:
            with self.subTest(defect=defect), self.assertRaisesRegex(ValueError, '^startup_process_control_result$'):
                startup.parse_process_control_result(json.dumps(defect).encode())
        for raw in (b'{' + b' ' * 8192 + b'}', json.dumps(value).encode()[:-1] + b',"version":1}'):
            with self.assertRaises(ValueError):
                startup.parse_process_control_result(raw)

    def test_source_or_context_drift_rejects_without_signalling(self):
        for name in ('source', 'context'):
            with self.subTest(name=name):
                control = self.control()
                (self.root / (name + '-' + str(len(self.controls)))).write_bytes(b'changed')
                with patch.object(signal, 'pidfd_send_signal', wraps=signal.pidfd_send_signal) as send:
                    self.assertEqual(control.freeze().category, 'rejected')
                    send.assert_not_called()

    def test_late_context_drift_during_identity_io_rejects_before_stop(self):
        control = self.control()
        original = control._identity
        def drift():
            identity = original()
            (self.root / 'context-1').write_bytes(b'late generation/barrier change')
            return identity
        with patch.object(control, '_identity', side_effect=drift), patch.object(
                signal, 'pidfd_send_signal', wraps=signal.pidfd_send_signal) as send:
            self.assertEqual(control.freeze().category, 'rejected')
            send.assert_not_called()

    def test_membership_and_namespace_drift_reject_before_signal(self):
        control = self.control()
        other = self.child()  # real cgroup membership changes, regardless of UID
        with patch.object(signal, 'pidfd_send_signal', wraps=signal.pidfd_send_signal) as send:
            self.assertEqual(control.freeze().category, 'rejected')
            send.assert_not_called()
        self.assertIsNone(other.poll())
        control = self.control()
        original = control._proc_identity
        def namespace(fd):
            value = original(fd)
            if fd == control._procfd:
                value['namespaces'][1] = 'mnt:[injected-read-drift]'
            return value
        with patch.object(control, '_proc_identity', side_effect=namespace), patch.object(
                signal, 'pidfd_send_signal', wraps=signal.pidfd_send_signal) as send:
            self.assertEqual(control.freeze().category, 'rejected')
            send.assert_not_called()

    def test_mixed_traced_unknown_and_oversize_task_states_reject(self):
        for state in ('T', 't', 'Z', '?'):
            with self.subTest(state=state):
                control = self.control()
                tasks = control._tasks()
                tasks[0][1] = state
                with patch.object(control, '_tasks', return_value=tasks), patch.object(
                        signal, 'pidfd_send_signal', wraps=signal.pidfd_send_signal) as send:
                    self.assertEqual(control.freeze().category, 'rejected')
                    send.assert_not_called()
        control = self.control()
        with patch.object(os, 'listdir', return_value=[str(i) for i in range(257)]):
            with self.assertRaises(docker.DockerError):
                control._tasks()

    def test_expiry_after_durable_context_io_forbids_stop(self):
        control = self.control()
        original = control._journal_matches
        def expire():
            original()
            control._deadline = time.monotonic() - 1
        with patch.object(control, '_journal_matches', side_effect=expire), patch.object(
                signal, 'pidfd_send_signal', wraps=signal.pidfd_send_signal) as send:
            self.assertEqual(control.freeze().category, 'held')
            send.assert_not_called()

    def test_unsupported_pidfd_signal_keeps_intent_and_never_falls_back(self):
        control = self.control()
        with patch.object(signal, 'pidfd_send_signal', side_effect=OSError('unsupported')) as send, patch.object(
                os, 'kill', side_effect=AssertionError('numeric signal forbidden')):
            self.assertEqual(control.freeze().category, 'held')
            self.assertEqual(control.restore().category, 'held')
            self.assertEqual(send.call_count, 1)
        self.assertEqual(control._intent['stage'], 'intent')
        self.assertTrue(all(state != 'T' for _, state in control._tasks()))

    def test_interrupted_intent_alone_cannot_authorize_cont(self):
        control = self.control()
        original = control._save
        def interrupt(stage, obligation):
            original(stage, obligation)
            raise KeyboardInterrupt()
        with patch.object(control, '_save', side_effect=interrupt), patch.object(
                signal, 'pidfd_send_signal', wraps=signal.pidfd_send_signal) as send:
            self.assertEqual(control.freeze().category, 'held')
            self.assertEqual(control.restore().category, 'held')
            send.assert_not_called()
        self.assertEqual(control._intent['stage'], 'intent')

    def test_truncated_private_state_is_held_and_original_retained(self):
        control = self.control()
        self.assertEqual(control.freeze().category, 'frozen')
        original = control._original
        (self.root / 'scheduler-worker.json').write_bytes(b'{')
        with patch.object(signal, 'pidfd_send_signal', wraps=signal.pidfd_send_signal) as send:
            result = control.restore()
            self.assertEqual(result.category, 'held')
            self.assertTrue(result.restore_required)
            send.assert_not_called()
        self.assertEqual(control._original, original)

    def test_preexisting_truncated_state_is_not_overwritten_or_signalled(self):
        control = self.control()
        leaf = self.root / 'scheduler-worker.json'
        leaf.write_bytes(b'{')
        with patch.object(signal, 'pidfd_send_signal', wraps=signal.pidfd_send_signal) as send:
            self.assertEqual(control.freeze().category, 'held')
            send.assert_not_called()
        self.assertEqual(leaf.read_bytes(), b'{')

    def test_timeout_after_stop_restores_original_but_not_as_freeze_success(self):
        control = self.control()
        original = control._drain_guard
        def timeout(limit):
            if getattr(control, '_sent', False):
                raise docker.DockerError('command_timeout')
            original(limit)
        with patch.object(control, '_drain_guard', side_effect=timeout), patch.object(
                signal, 'pidfd_send_signal', wraps=signal.pidfd_send_signal) as send:
            self.assertEqual(control.freeze().category, 'freeze-restored')
            self.assertEqual([call.args[1] for call in send.call_args_list], [signal.SIGSTOP, signal.SIGCONT])

    def test_stop_does_not_stop_live_descendant(self):
        child = self.child()
        child.stdin.write('spawn\n')
        child.stdin.flush()
        self.assertTrue(select.select([child.stdout], [], [], 2)[0], 'owned descendant latch')
        descendant = int(child.stdout.readline())
        control = self.control(child)
        self.assertEqual(control.freeze().category, 'frozen')
        state = Path('/proc/' + str(descendant) + '/status').read_text().split('State:\t')[1][0]
        self.assertIn(state, ('R', 'S'))
        self.assertEqual(control.restore().category, 'restored')

    def test_natural_descendant_exit_while_frozen_allows_exact_parent_restore(self):
        child = self.child()
        gate = self.root / 'owned-exit-gate'
        os.mkfifo(gate, 0o600)
        child.stdin.write('spawn-exit ' + str(gate) + '\n')
        child.stdin.flush()
        self.assertTrue(select.select([child.stdout], [], [], 2)[0], 'owned descendant-ready latch')
        descendant = int(child.stdout.readline())
        descendant_fd = os.pidfd_open(descendant)
        self.addCleanup(os.close, descendant_fd)
        control = self.control(child)
        self.assertEqual(control.freeze().category, 'frozen')
        original_members = control._selected['members'][:]
        with gate.open('wb', buffering=0) as output:
            output.write(b'q')
        self.assertTrue(select.select([descendant_fd], [], [], 2)[0], 'owned natural-exit latch')
        self.assertNotIn(descendant, [pid for pid, _ in control._identity()['members']])
        with patch.object(signal, 'pidfd_send_signal', wraps=signal.pidfd_send_signal) as send:
            result = control.restore()
            self.assertEqual(result.category, 'restored')
            self.assertTrue(result.maintenance_held)
            self.assertEqual([call.args for call in send.call_args_list], [(control._pidfd, signal.SIGCONT)])
        # The restoration receipt does not classify departed work. Its actual
        # status is collected by its owned parent only AFTER that parent resumes.
        child.stdin.write('collect\n')
        child.stdin.flush()
        self.assertTrue(select.select([child.stdout], [], [], 2)[0], 'owned exit-status latch')
        self.assertEqual(child.stdout.readline(), 'natural-descendant-exit=0\n')
        self.assertEqual(control._selected['members'], original_members)
        self.assertEqual(control._intent['identity']['members'], original_members)

    def test_restoration_process_cgroup_and_namespace_drift_still_hold(self):
        control = self.control()
        self.assertEqual(control.freeze().category, 'frozen')
        original = control._proc_identity
        for field in ('cgroup', 'namespaces'):
            def drift(fd):
                value = original(fd)
                if fd == control._procfd:
                    if field == 'cgroup':
                        value[field] += 'injected-role-cgroup-drift'
                    else:
                        value[field][1] = 'mnt:[injected-role-namespace-drift]'
                return value
            with self.subTest(field=field), patch.object(control, '_proc_identity', side_effect=drift), patch.object(
                    signal, 'pidfd_send_signal', wraps=signal.pidfd_send_signal) as send:
                self.assertEqual(control.restore().category, 'held')
                send.assert_not_called()

    def test_restore_journal_io_cannot_renew_deadline_running_or_stopped(self):
        for stopped in (False, True):
            role = 'terminal-server' if stopped else 'scheduler-worker'
            control = self.control(role=role)
            if stopped:
                signal.pidfd_send_signal(control._pidfd, signal.SIGSTOP)
                limit = time.monotonic() + 1
                while not all(state == 'T' for _, state in control._tasks()):
                    self.assertLess(time.monotonic(), limit)
            self.assertIn(control.freeze().category, ('frozen', 'already-stopped'))
            clock = [time.monotonic()]
            journal = control._journal_matches
            def late_journal():
                journal()
                clock[0] += 2
            with self.subTest(stopped=stopped), patch.object(control, '_journal_matches', side_effect=late_journal), patch.object(
                    time, 'monotonic', side_effect=lambda: clock[0]), patch.object(
                    signal, 'pidfd_send_signal', wraps=signal.pidfd_send_signal) as send:
                self.assertEqual(control.restore().category, 'held')
                send.assert_not_called()

    def test_restore_durable_completion_expiry_cannot_report_success(self):
        control = self.control()
        self.assertEqual(control.freeze().category, 'frozen')
        clock = [time.monotonic()]
        save = control._save
        def late_save(stage, obligation):
            save(stage, obligation)
            if stage == 'restored':
                clock[0] += 2
        with patch.object(control, '_save', side_effect=late_save), patch.object(
                time, 'monotonic', side_effect=lambda: clock[0]):
            self.assertEqual(control.restore().category, 'held')

    def test_late_source_drift_before_restore_prevents_cont(self):
        control = self.control()
        self.assertEqual(control.freeze().category, 'frozen')
        original = control._identity
        def drift():
            identity = original()
            (self.root / 'source-1').write_bytes(b'late source change')
            return identity
        with patch.object(control, '_identity', side_effect=drift), patch.object(
                signal, 'pidfd_send_signal', wraps=signal.pidfd_send_signal) as send:
            self.assertEqual(control.restore().category, 'held')
            send.assert_not_called()

    def test_expired_drain_still_restores_exact_authenticated_original(self):
        control = self.control()
        self.assertEqual(control.freeze().category, 'frozen')
        control._deadline = time.monotonic() - 1
        self.assertEqual(control.restore().category, 'restored')

    def test_exit_between_identity_read_and_signal_never_touches_replacement(self):
        child = self.child()
        control = self.control(child)
        original = control._identity
        def exit_latch():
            value = original()
            child.stdin.write('quit\n')
            child.stdin.flush()
            self.assertEqual(child.wait(timeout=2), 0)
            self.child()  # replacement is distinct, never adopted as original
            return value
        with patch.object(control, '_identity', side_effect=exit_latch), patch.object(
                signal, 'pidfd_send_signal', wraps=signal.pidfd_send_signal) as send:
            self.assertEqual(control.freeze().category, 'rejected')
            send.assert_not_called()

    def test_gone_original_restore_is_held_and_replacement_receives_no_signal(self):
        child = self.child()
        control = self.control(child)
        self.assertEqual(control.freeze().category, 'frozen')
        # Deliberate harness fault injection, classified separately from natural exit.
        self.expected_exits[child.pid] = -signal.SIGKILL
        signal.pidfd_send_signal(control._pidfd, signal.SIGKILL)
        self.assertEqual(child.wait(timeout=2), -signal.SIGKILL)
        replacement = self.child()
        with patch.object(signal, 'pidfd_send_signal', wraps=signal.pidfd_send_signal) as send:
            self.assertEqual(control.restore().category, 'held')
            send.assert_not_called()
        self.assertIsNone(replacement.poll())

    def test_nonblocking_role_lock_and_private_size_bound(self):
        import fcntl
        control = self.control()
        fd = os.open(self.root / 'scheduler-worker.lock', os.O_RDWR)
        try:
            fcntl.flock(fd, fcntl.LOCK_EX | fcntl.LOCK_NB)
            self.assertEqual(control.freeze().category, 'busy')
        finally:
            os.close(fd)
        control._original = control._tasks()
        control._selected['oversize'] = 'x' * 131072
        with self.assertRaises(docker.DockerError):
            control._save('intent', True)


if __name__ == '__main__':
    unittest.main()
