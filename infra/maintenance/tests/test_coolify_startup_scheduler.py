"""Scheduler kernel evidence; these are NOT actual Laravel/native-host proofs."""
import json
import hashlib
import importlib.util
import os
from pathlib import Path
import select
import signal
import subprocess
import sys
import time
import tempfile
from types import SimpleNamespace
import unittest
from unittest.mock import patch

from maintenance import coolify_docker as docker
from maintenance import coolify_startup as startup
from tests import test_coolify_startup_process_control as process_control


class SchedulerFixtureRecord(unittest.TestCase):
    def setUp(self):
        path = Path(__file__).resolve().parents[1] / 'startup-scheduler/check.py'
        spec = importlib.util.spec_from_file_location('scheduler_public_fixture', path)
        module = importlib.util.module_from_spec(spec)
        spec.loader.exec_module(module)
        self.fixture = object.__new__(module.Fixture)
        directory = tempfile.TemporaryDirectory()
        self.addCleanup(directory.cleanup)
        self.fixture.public = Path(directory.name)

    def records(self):
        return [json.loads(raw) for raw in (self.fixture.public / 'native-observations.jsonl').read_bytes().splitlines()]

    def test_actual_case_serialization_preserves_schema_and_image_versions(self):
        result = startup.parse_scheduler_result(json.dumps(dict(version=1, category='quiescent', live_count=0,
            finished_count=0, failed_count=0, unknown_count=0, restore_required=True,
            maintenance_held=True, identity_hmac='a' * 64)).encode())
        for service in ('old', 'target'):
            self.fixture.observe_case(service, 'idle', result)
        for service, record in zip(('old', 'target'), self.records()):
            self.assertEqual(record['version'], 1)
            self.assertEqual(record['image_version'], service)
            self.assertEqual(record['category'], 'quiescent')
            self.assertTrue(record['restore_required'])

    def test_actual_restore_serialization_preserves_schema_and_image_versions(self):
        result = startup.parse_process_control_result(json.dumps(dict(version=1, role='scheduler-worker',
            category='restored', task_count=1, restore_required=False,
            maintenance_held=True, identity_hmac='a' * 64)).encode())
        for service in ('old', 'target'):
            self.fixture.restore(service, 'idle', SimpleNamespace(restore=lambda: result))
        for service, record in zip(('old', 'target'), self.records()):
            self.assertEqual(record['version'], 1)
            self.assertEqual(record['image_version'], service)
            self.assertEqual(record['category'], 'restored')
            self.assertFalse(record['restore_required'])
            self.assertGreaterEqual(record['elapsed_ms'], 0)


class SchedulerImageIdentity(unittest.TestCase):
    reference = 'docker.io/coollabsio/coolify@' + startup._IMAGES[0][1]
    config = startup._IMAGES[0][2]
    manifest = reference.split('@')[1]
    media = 'application/vnd.oci.image.manifest.v1+json'

    def observed(self, containerd=False):
        return dict(engine_id=self.manifest if containerd else self.config, os='linux',
                    architecture='arm64', repo_digests=[self.reference.removeprefix('docker.io/')],
                    descriptor=dict(digest=self.manifest, mediaType=self.media) if containerd else None)

    def identity(self, value, **kwargs):
        self.assertTrue(hasattr(docker, '_scheduler_image_identity'), 'strict scheduler image binding missing')
        return docker._scheduler_image_identity(self.reference, value, **kwargs)

    def test_classic_config_id_with_exact_repo_and_platform(self):
        self.assertEqual(self.identity(self.observed()), self.config)

    def test_containerd_manifest_id_with_exact_leaf_descriptor(self):
        self.assertEqual(self.identity(self.observed(True)), self.manifest)

    def test_container_image_must_join_the_validated_daemon_id(self):
        for containerd in (False, True):
            value = self.observed(containerd)
            self.assertEqual(self.identity(value, container_image=value['engine_id']), value['engine_id'])
            with self.assertRaisesRegex(docker.DockerError, '^image_identity$'):
                self.identity(value, container_image=self.config if containerd else self.manifest)

    def test_wrong_platform_repo_config_or_descriptor_is_rejected(self):
        for containerd in (False, True):
            good = self.observed(containerd)
            defects = [dict(good, os='windows'), dict(good, architecture='amd64'),
                       dict(good, engine_id='sha256:' + '0' * 64),
                       dict(good, repo_digests=[]), dict(good, repo_digests=['foreign/repo@' + self.manifest]),
                       dict(good, descriptor=dict(digest='sha256:' + '0' * 64, mediaType=self.media)),
                       dict(good, descriptor=dict(digest=self.manifest, mediaType='application/vnd.oci.image.index.v1+json'))]
            if containerd:
                defects.append(dict(good, descriptor=None))
            for value in defects:
                with self.subTest(value=value), self.assertRaisesRegex(docker.DockerError, '^image_identity$'):
                    self.identity(value)

    def test_malformed_or_unbounded_selected_metadata_is_rejected(self):
        good = self.observed(True)
        for value in (None, [], dict(good, extra=True), dict(good, repo_digests='bad'),
                      dict(good, repo_digests=[self.reference] * 17),
                      dict(good, descriptor={'digest': self.manifest}),
                      dict(good, descriptor=dict(good['descriptor'], extra=True))):
            with self.subTest(value=value), self.assertRaisesRegex(docker.DockerError, '^image_identity$'):
                self.identity(value)

    def test_fixed_public_leaf_bytes_bind_every_config_pin(self):
        self.assertTrue(hasattr(docker, '_SCHEDULER_IMAGE_PINS'), 'fixed scheduler image pins missing')
        self.assertEqual(tuple((name, reference.split('@')[1], config)
                               for name, reference, config, _ in docker._SCHEDULER_IMAGE_PINS[:2]), startup._IMAGES)
        for name, reference, config, media in docker._SCHEDULER_IMAGE_PINS:
            # Flat test data preserves the pipeline source-manifest grammar.
            # Git's final newline encodes the public registry bytes, which have none.
            raw = (Path(__file__).parent / ('scheduler_image_' + name + '.json')).read_bytes().removesuffix(b'\n')
            self.assertLessEqual(len(raw), 65536)
            self.assertEqual('sha256:' + hashlib.sha256(raw).hexdigest(), reference.split('@')[1])
            manifest = docker.strict_json(raw)
            self.assertEqual(manifest['schemaVersion'], 2)
            self.assertEqual(manifest['mediaType'], media)
            self.assertEqual(manifest['config']['digest'], config)
            self.assertEqual(manifest['config']['mediaType'], 'application/vnd.oci.image.config.v1+json')
            value = dict(engine_id=reference.split('@')[1], os='linux', architecture='arm64',
                         repo_digests=[reference], descriptor=dict(digest=reference.split('@')[1], mediaType=media))
            self.assertEqual(docker._scheduler_image_identity(reference, value), value['engine_id'])


class NativeMappingSnapshot(unittest.TestCase):
    def inspect(self, mounts):
        control = object.__new__(docker._SchedulerHostProcessControl)
        control._cid = 'c' * 64
        value = dict(Id=control._cid, Running=True, OOM=False, Pid=12, Mounts=mounts)
        control._docker = lambda args: json.dumps(value).encode()
        return control._inspect_fixture()

    def rows(self):
        return [dict(Destination='/z', Source='/owned/z', Type='bind', RW=False, Mode='ro'),
                dict(Destination='/a', Source='/owned/a', Type='bind', RW=True, Mode='rw')]

    def test_mount_permutation_preserves_the_complete_snapshot(self):
        rows = self.rows()
        self.assertEqual(self.inspect(rows), self.inspect(list(reversed(rows))))
        self.assertEqual(rows[0]['Destination'], '/z')

    def test_duplicate_destinations_are_rejected(self):
        rows = self.rows()
        for duplicate in (rows[0], dict(rows[0], Source='/foreign')):
            with self.assertRaisesRegex(docker.DockerError, '^container_identity$'):
                self.inspect(rows + [duplicate])

    def test_every_mount_field_remains_identity_bound(self):
        rows = self.rows()
        original = self.inspect(rows)
        for change in (dict(Source='/foreign'), dict(RW=True), dict(Mode='rw'),
                       dict(Type='volume'), dict(Propagation='rshared'), dict(extra='retained')):
            with self.subTest(change=change):
                self.assertNotEqual(original, self.inspect([dict(rows[0], **change), rows[1]]))

    def test_malformed_mount_inventory_is_rejected(self):
        for rows in (None, {}, [], [None], [dict(Destination=1)], [dict(Destination='relative')]):
            with self.subTest(rows=rows), self.assertRaisesRegex(docker.DockerError, '^container_identity$'):
                self.inspect(rows)

    def test_failed_native_placement_exposes_only_fixed_predicate_booleans(self):
        # Synthetic guard component only; this does not prove native mapping.
        control = object.__new__(docker._SchedulerHostProcessControl)
        control._container = {'fixed': True}
        control._inspect_fixture = lambda: {'fixed': False}
        control._initpid, control._init_token = 12, b'123'
        control._initfd, control._procfd, control._rootfd = 100, 101, 102
        control._time_namespace, control._cgroup = 'time:[1]', 'owned'
        metadata = os.stat(__file__)
        def row(argv):
            return dict(ids={'Uid': ['9999'] * 4, 'Gid': ['9999'] * 4, 'NSpid': ['12', '1']},
                        cgroup='owned', namespaces=['pid:[2]', 'mnt:[3]', 'net:[4]'],
                        argv=[part.hex() for part in argv], parent=12, exe=list(docker._metadata(metadata)))
        selected = row((b'php', b'artisan', b'schedule:work'))
        parent = row((b's6-supervise', b'scheduler-worker'))
        init = row((b's6-svscan',))
        with patch.object(docker, '_start_token', return_value=b'123'), \
             patch.object(docker.os, 'readlink', side_effect=lambda name, **kwargs: 'time:[1]' if name == 'ns/time' else 'pid:[9]'), \
             patch.object(docker.os, 'stat', return_value=metadata):
            with self.assertRaisesRegex(docker.DockerError, '^startup_state$') as caught:
                control._placement(selected, parent, init)
        checks = getattr(caught.exception, '_scheduler_placement_checks', None)
        self.assertIsInstance(checks, dict)
        self.assertEqual(len(checks), 15)
        self.assertTrue(all(type(value) is bool for value in checks.values()))
        self.assertFalse(checks['container'])
        self.assertTrue(all(value for key, value in checks.items() if key != 'container'))


class SchedulerDrain(unittest.TestCase):
    setUp = process_control.ProcessKernel.setUp
    tearDown = process_control.ProcessKernel.tearDown
    child = process_control.ProcessKernel.child
    control = process_control.ProcessKernel.control

    def drain(self, control):
        self.assertTrue(hasattr(docker, '_SchedulerDrain'), 'finite scheduler drain missing')
        # Explicit PUBLIC harness ownership, never a product default whitelist.
        rows = {}
        for pid, _ in control._selected['members']:
            fd = os.open('/proc/' + str(pid), os.O_RDONLY | os.O_DIRECTORY)
            try:
                rows[pid] = control._proc_identity(fd)
            finally:
                os.close(fd)
        descendants = {control._pid}
        for _ in range(256):
            added = {pid for pid, row in rows.items() if row['parent'] in descendants} - descendants
            if not added:
                break
            descendants |= added
        control._scheduler_stable = {pid: row for pid, row in rows.items() if pid not in descendants}
        drain = docker._SchedulerDrain(control)
        self.addCleanup(drain.close)
        return drain

    def test_idle_frozen_observation_and_original_state_restore(self):
        control = self.control()
        drain = self.drain(control)
        result = drain.run()
        self.assertEqual(result.category, 'quiescent')
        self.assertTrue(result.restore_required)
        self.assertTrue(all(state == 'T' for _, state in control._tasks()))
        self.assertEqual(control.restore().category, 'restored')

    def test_missing_individual_stable_authentication_holds_unknown(self):
        control = self.control()
        drain = docker._SchedulerDrain(control)
        self.addCleanup(drain.close)
        result = drain.run()
        self.assertEqual(result.category, 'held')
        self.assertGreater(result.unknown_count, 0)
        self.assertFalse(result.restore_required)

    def test_new_member_between_classification_and_final_sweep_holds(self):
        control = self.control()
        drain = self.drain(control)
        self.assertEqual(control.freeze().category, 'frozen')
        inventory = drain._inventory
        calls = 0
        def arriving():
            nonlocal calls
            calls += 1
            if calls == 2:
                self.child()
            return inventory()
        with patch.object(drain, '_inventory', side_effect=arriving):
            result = drain.observe()
        self.assertEqual(calls, 2)
        self.assertEqual(result.category, 'held')
        self.assertGreater(result.unknown_count, 0)
        self.assertEqual(control.restore().category, 'restored')

    def test_running_child_times_out_without_stopping_work_and_restores_launcher(self):
        child = self.child()
        child.stdin.write('spawn\n'); child.stdin.flush()
        descendant = int(child.stdout.readline())
        control = self.control(child)
        drain = self.drain(control)
        control._deadline = time.monotonic() + .2
        result = drain.run()
        self.assertEqual(result.category, 'held')
        self.assertEqual(result.live_count, 1)
        self.assertFalse(result.restore_required)
        self.assertTrue(all(state != 'T' for _, state in control._tasks()))
        self.assertIn(Path('/proc/' + str(descendant) + '/status').read_text().split('State:\t')[1][0], 'RS')

    def test_original_stopped_idle_is_never_resumed(self):
        control = self.control()
        signal.pidfd_send_signal(control._pidfd, signal.SIGSTOP)
        deadline = time.monotonic() + 1
        while not all(state == 'T' for _, state in control._tasks()):
            self.assertLess(time.monotonic(), deadline)
        result = self.drain(control).run()
        self.assertEqual(result.category, 'quiescent')
        self.assertFalse(result.restore_required)
        self.assertEqual(control.restore().category, 'unchanged')

    def test_known_zombie_leaves_cgroup_procs_but_positive_exit_evidence_is_supported(self):
        child = self.child()
        gate = self.root / 'scheduler-exit-gate'
        os.mkfifo(gate, 0o600)
        child.stdin.write('spawn-exit ' + str(gate) + '\n'); child.stdin.flush()
        descendant = int(child.stdout.readline())
        control = self.control(child)
        drain = self.drain(control)
        self.assertEqual(control.freeze().category, 'frozen')
        _, membership = control._placement(control._selected['process'], control._selected['parent'],
                                            control._selected['init'])
        with gate.open('wb', buffering=0) as output:
            output.write(b'q')
        deadline = time.monotonic() + 1
        with membership.open('rb') as group:
            while True:
                group.seek(0)
                members = group.read(8193)
                self.assertLessEqual(len(members), 8192)
                self.assertLess(time.monotonic(), deadline)
                if str(descendant).encode() not in members.split():
                    break
                time.sleep(.001)
        # Linux removes cgroup membership before exit_notify publishes Z.
        # Await that independent precondition on the already retained procfd.
        procfd = drain._known[descendant][0]
        while True:
            with os.fdopen(os.open('stat', os.O_RDONLY | os.O_CLOEXEC, dir_fd=procfd), 'rb') as stream:
                raw = stream.read(8192)
            self.assertLess(time.monotonic(), deadline)
            if raw[raw.rfind(b')') + 2:].split()[0] == b'Z':
                break
            time.sleep(.001)
        result = drain.observe()
        self.assertEqual(result.category, 'quiescent')
        self.assertEqual(result.finished_count, 1)
        self.assertEqual(control.restore().category, 'restored')

    def test_reaped_individually_stable_member_is_unknown(self):
        extra = self.child()
        control = self.control()
        drain = self.drain(control)
        self.assertEqual(control.freeze().category, 'frozen')
        extra.stdin.write('quit\n'); extra.stdin.flush()
        self.assertEqual(extra.wait(timeout=2), 0)
        result = drain.observe()
        self.assertEqual(result.category, 'held')
        self.assertGreater(result.unknown_count, 0)
        self.assertEqual(control.restore().category, 'restored')


class NativePhaseGuard(unittest.TestCase):
    """Real owned child + native final guard only; NOT host/bind mapping proof.

    Assemble the dormant control like K0. Two independent private files model
    the mounted phase and its host source: atomic source replacement leaves the
    model mount unchanged. Factory binding of a real Docker mount is CI's job.
    """
    setUp = process_control.ProcessKernel.setUp
    tearDown = process_control.ProcessKernel.tearDown
    child = process_control.ProcessKernel.child
    control = process_control.ProcessKernel.control
    drain = SchedulerDrain.drain

    def native_guard(self):
        component = self.control()
        control = object.__new__(docker._SchedulerHostProcessControl)
        control.__dict__.update(component.__dict__)
        # Only the native final phase guard is under test; no Docker/S6 mapping
        # or Laravel context checks are fabricated as actual native proof.
        control._placement = docker._FixedRoleProcessControl._placement.__get__(control)
        control._read_pins = docker._FixedRoleProcessControl._read_pins.__get__(control)
        mount = self.root / 'mounted'
        (mount / 'probe').mkdir(parents=True)
        control._rootfd = os.open(mount, os.O_RDONLY | os.O_DIRECTORY)
        self.addCleanup(os.close, control._rootfd)
        control._request_source_path = self.root / 'request.json'
        self.mounted_request = mount / 'probe/scheduler.request.json'
        request = dict(version=1, phase='scheduler-proof-held', generation='a' * 32,
                       deadline_ns=time.monotonic_ns() + 10_000_000_000)
        raw = json.dumps(request, separators=(',', ':')).encode()
        for path in (control._request_source_path, self.mounted_request):
            path.write_bytes(raw)
            path.chmod(0o444)
        control._request_source_fd = os.open(control._request_source_path, os.O_RDONLY | os.O_NOFOLLOW)
        control._request_mount_fd = os.open(self.mounted_request, os.O_RDONLY | os.O_NOFOLLOW)
        self.addCleanup(os.close, control._request_source_fd)
        self.addCleanup(os.close, control._request_mount_fd)
        control._request_pins = [[list(docker._metadata(os.fstat(fd))), hashlib.sha256(raw).hexdigest()]
            for fd in (control._request_source_fd, control._request_mount_fd)]
        control._request_data = request
        return control

    def assert_late_phase_change(self, operation, atomic):
        control = self.native_guard()
        if operation in ('restore', 'observe'):
            drain = self.drain(control) if operation == 'observe' else None
            self.assertEqual(control.freeze().category, 'frozen')
        identity = control._identity
        changed = False
        def late():
            nonlocal changed
            result = identity()
            if not changed:
                changed = True
                updated = dict(control._request_data, generation='b' * 32)
                raw = json.dumps(updated, separators=(',', ':')).encode()
                if atomic:
                    replacement = self.root / 'replacement.json'
                    replacement.write_bytes(raw)
                    replacement.chmod(0o444)
                    os.replace(replacement, control._request_source_path)
                else:
                    control._request_source_path.chmod(0o644)
                    control._request_source_path.write_bytes(raw)
                    control._request_source_path.chmod(0o444)
                # Simulated Docker bind stays on its original file. A mounted
                # request check alone cannot see this host-source revocation.
                self.assertEqual(json.loads(self.mounted_request.read_bytes()), control._request_data)
            return result
        with patch.object(control, '_identity', side_effect=late), patch.object(
                signal, 'pidfd_send_signal', wraps=signal.pidfd_send_signal) as send:
            if operation == 'freeze':
                result = control.freeze()
                self.assertEqual(result.category, 'rejected')
            elif operation == 'restore':
                result = control.restore()
                self.assertEqual(result.category, 'held')
                self.assertTrue(result.restore_required)
            else:
                result = drain.observe()
                self.assertEqual(result.category, 'held')
                self.assertTrue(result.restore_required)
            send.assert_not_called()
        self.assertTrue(changed)

    def test_late_inplace_phase_change_blocks_stop(self):
        self.assert_late_phase_change('freeze', False)

    def test_late_atomic_host_phase_replacement_blocks_stop(self):
        self.assert_late_phase_change('freeze', True)

    def test_late_inplace_phase_change_blocks_cont(self):
        self.assert_late_phase_change('restore', False)

    def test_late_atomic_host_phase_replacement_blocks_cont(self):
        self.assert_late_phase_change('restore', True)

    def test_late_inplace_phase_change_blocks_quiescence(self):
        self.assert_late_phase_change('observe', False)

    def test_late_atomic_host_phase_replacement_blocks_quiescence(self):
        self.assert_late_phase_change('observe', True)

    def assert_request_io_cannot_renew_budget(self, restoring):
        control = self.native_guard()
        if restoring:
            self.assertEqual(control.freeze().category, 'frozen')
        clock = [time.monotonic()]
        request_guard = control._request_guard
        def late_request_io():
            request_guard()
            clock[0] += 2
        with patch.object(control, '_request_guard', side_effect=late_request_io), patch.object(
                time, 'monotonic', side_effect=lambda: clock[0]), patch.object(
                signal, 'pidfd_send_signal', wraps=signal.pidfd_send_signal) as send:
            result = control.restore() if restoring else control.freeze()
            self.assertEqual(result.category, 'held' if restoring else 'rejected')
            send.assert_not_called()

    def test_request_io_cannot_renew_original_stop_budget(self):
        self.assert_request_io_cannot_renew_budget(False)

    def test_request_io_cannot_renew_separate_restore_budget(self):
        self.assert_request_io_cannot_renew_budget(True)


class SchedulerEvidence(unittest.TestCase):
    def test_native_mapping_and_scheduler_context_are_private_and_gate_stays_closed(self):
        self.assertTrue(hasattr(docker, '_SchedulerHostProcessControl'), 'owned native S6 mapping missing')
        with self.assertRaisesRegex(docker.DockerError, '^startup_profile_unverified$'):
            docker._SchedulerHostProcessControl()
        source = (Path(__file__).resolve().parents[1] / 'maintenance/coolify_startup.php').read_text()
        self.assertIn('function startupSchedulerContext(', source)
        self.assertIn("$argv[1] === 'scheduler-context'", source)
        self.assertFalse(startup.fixed_candidate()['registered'])

    def test_parser_rejects_contradictions_boolean_counts_duplicates_and_size(self):
        self.assertTrue(hasattr(startup, 'parse_scheduler_result'), 'S3 typed result missing')
        value = dict(version=1, category='quiescent', live_count=0, finished_count=1,
                     failed_count=0, unknown_count=0, restore_required=True,
                     maintenance_held=True, identity_hmac='a' * 64)
        self.assertEqual(startup.parse_scheduler_result(json.dumps(value).encode()).finished_count, 1)
        for defect in (dict(value, live_count=True), dict(value, unknown_count=257),
                       dict(value, live_count=1), dict(value, failed_count=1),
                       dict(value, unknown_count=1), dict(value, maintenance_held=False),
                       dict(value, category='success'), dict(value, extra=1)):
            with self.subTest(defect=defect), self.assertRaises(ValueError):
                startup.parse_scheduler_result(json.dumps(defect).encode())
        for raw in (json.dumps(value).encode()[:-1] + b',"version":1}', b' ' * 8193):
            with self.assertRaises(ValueError):
                startup.parse_scheduler_result(raw)

    def kernel_child(self, action, different_uid=False):
        # This harness is the parent and deliberately does NOT poll/wait before
        # the observer's classification. No subprocess auto-reaping shortcut.
        code = ('import os,sys,signal; '
                + ('os.setgid(9999); os.setuid(9999); ' if different_uid else '')
                + 'print("ready",flush=True); sys.stdin.readline(); ' + action)
        child = subprocess.Popen([sys.executable, '-u', '-c', code],
                                 stdin=subprocess.PIPE, stdout=subprocess.PIPE, text=True)
        self.addCleanup(child.stdout.close)
        self.addCleanup(child.stdin.close)
        self.addCleanup(lambda: child.wait(timeout=2))
        self.assertTrue(select.select([child.stdout], [], [], 2)[0])
        self.assertEqual(child.stdout.readline(), 'ready\n')
        procfd = os.open('/proc/' + str(child.pid), os.O_RDONLY | os.O_DIRECTORY)
        pidfd = os.pidfd_open(child.pid)
        self.addCleanup(os.close, procfd)
        self.addCleanup(os.close, pidfd)
        token = docker._scheduler_token(procfd)
        child.stdin.write('exit\n'); child.stdin.flush()
        self.assertTrue(select.select([pidfd], [], [], 2)[0])
        return procfd, token

    def test_real_known_zombie_normal_zero_nonzero_and_signal(self):
        self.assertTrue(hasattr(docker, '_scheduler_terminal'), 'positive terminal evidence missing')
        for action, expected in (('os._exit(0)', 'finished'), ('os._exit(7)', 'failed'),
                                 ('os.kill(os.getpid(),signal.SIGTERM)', 'signaled')):
            with self.subTest(action=action):
                procfd, token = self.kernel_child(action)
                self.assertEqual(docker._scheduler_terminal(procfd, token), expected)

    def test_actual_different_uid_stat_zero_redaction_does_not_mean_success(self):
        self.assertTrue(hasattr(docker, '_scheduler_terminal'), 'positive access witness missing')
        procfd, token = self.kernel_child('os._exit(7)', different_uid=True)
        # Drop privileges in a separate observer; parent remains able to reap.
        readfd, writefd = os.pipe()
        observer = os.fork()
        if observer == 0:
            try:
                os.close(readfd); os.setgroups([]); os.setgid(10000); os.setuid(10000)
                raw = docker._FixedRoleProcessControl._proc_read(procfd, 'stat', 8192)
                fields = raw[raw.rfind(b')') + 2:].split()
                value = [int(fields[49]), docker._scheduler_terminal(procfd, token)]
                os.write(writefd, json.dumps(value).encode())
            finally:
                os._exit(0)
        os.close(writefd)
        try:
            self.assertEqual(json.loads(os.read(readfd, 8192)), [0, 'unknown'])
            self.assertEqual(os.waitpid(observer, 0)[1], 0)
        finally:
            os.close(readfd)

    def test_changed_identity_or_parent_is_unknown(self):
        self.assertTrue(hasattr(docker, '_scheduler_terminal'), 'identity binding missing')
        procfd, token = self.kernel_child('os._exit(0)')
        for index in (0, 1):
            altered = token[:]
            altered[index] += 1
            self.assertEqual(docker._scheduler_terminal(procfd, altered), 'unknown')


if __name__ == '__main__':
    unittest.main()
