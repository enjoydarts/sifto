"""PUBLIC synthetic systemd resource contracts; never contacts host systemd."""
from dataclasses import replace
from concurrent.futures import ThreadPoolExecutor
import json
import os
from pathlib import Path
import tempfile
import threading
import time
import unittest
from unittest.mock import patch

from maintenance import coolify_docker as docker
try:
    from maintenance import coolify_systemd as s
except ImportError:
    s = None
import test_coolify_docker as fixture


class SystemdContracts(unittest.TestCase):
    setUp = fixture.OwnedDockerTests.setUp

    def subject(self, *, controller_identity=None):
        self.assertIsNotNone(s, 'N2S-1 systemd resource module is missing')
        test = self

        class Synthetic(s.SystemdResources):
            def _native_preflight(self, policy):
                self._context_guard = fixture.SyntheticOwned(policy.context, adapter=object())
                self._context_guard._guard_context()
                self._stage_fd = self._context_guard._stage_fd
                self._controller_identity = controller_identity or ('PUBLIC-controller', s.secrets.token_hex(16))
                self._manager = FakeManager(test.stage)
                self._cgroup_root = self._manager.cgroup_root
                self._cgroup_mount = ('29', '0:26', '/', '/sys/fs/cgroup', 'cgroup2')
                self._guard_controller_local()

            def _guard_controller_local(self):
                s._validate_controller_envelope(self._manager.controller_caps)
                s._require(self._manager.membership == s.CONTROLLER_GROUP, 'controller_identity')

            def _engine_process_identity(self, observation):
                return 'PUBLIC-process-start'

            def _mount_observation(self):
                return b'29 22 0:26 / /sys/fs/cgroup rw - cgroup2 cgroup rw\n'

            def _controller_baseline(self):
                self._guard_controller_local()
                s._require(not getattr(self, '_helper_cleanup_unresolved', False)
                           and not self._sessions, 'cleanup_incomplete')

        resources = Synthetic()
        self.addCleanup(resources.close)
        policy = s.FixedControllerPolicy(replace(self.context, budget=docker.BudgetPolicy()))
        controller = resources.verify_controller(policy)
        return resources, controller

    def rejected(self, category, action):
        with self.assertRaisesRegex(s.SystemdError, '^' + category + '$'):
            action()

    def test_missing_systemd_module_is_a_behavioral_failure(self):
        self.assertIsNotNone(s, 'N2S-1 systemd resource module is missing')

    def test_budget_split_covers_exact_fixed_aggregate(self):
        self.assertIsNotNone(s)
        self.assertEqual(s.CONTROLLER_MEMORY + s.WORKLOAD_MEMORY, 1280 * 1024**2)
        self.assertEqual(s.CONTROLLER_CPU + s.WORKLOAD_CPU, 1750)
        self.assertEqual(s.CONTROLLER_TASKS + s.WORKLOAD_TASKS, 576)
        self.assertEqual(sum(v.memory for v in s._CONTAINER_LIMITS.values()) + s.ENGINE_STEADY.memory,
                         s.WORKLOAD_MEMORY)
        self.assertEqual(sum(v.cpu for v in s._CONTAINER_LIMITS.values()) + s.ENGINE_STEADY.cpu,
                         s.WORKLOAD_CPU)

    def test_reservation_is_exclusive_durable_n1_compatible_and_has_separate_ledger(self):
        resources, controller = self.subject()
        run = resources.reserve_run(controller)
        record = resources._runs[run]
        root = self.stage / record['token']
        self.assertRegex(record['token'], '^[a-f0-9]{32}$')
        self.assertEqual(root.stat().st_mode & 0o777, 0o700)
        self.assertEqual((root / '.owner').read_text(), record['token'])
        for phase in ('old', 'fresh'):
            self.assertEqual((root / phase / '.owner').read_text(), record['token'] + ':' + phase)
        self.assertEqual((root / 'journal').read_bytes(), b'')
        self.assertEqual((root / 'controller-ledger').stat().st_mode & 0o777, 0o600)
        self.assertFalse(resources._manager.calls)
        self.assertNotIsInstance(run, docker.OwnedRun)

    def test_nonce_collision_never_adopts_or_overwrites_foreign_content(self):
        resources, controller = self.subject()
        token = 'a' * 32
        foreign = self.stage / token
        foreign.mkdir(mode=0o700)
        (foreign / 'foreign').write_bytes(b'PUBLIC preserved')
        with patch.object(s.secrets, 'token_hex', return_value=token):
            self.rejected('stage_collision', lambda: resources.reserve_run(controller))
        self.assertEqual((foreign / 'foreign').read_bytes(), b'PUBLIC preserved')
        self.assertFalse(resources._manager.calls)

    def test_stale_and_forged_handles_fail_before_manager(self):
        resources, controller = self.subject()
        other, other_controller = self.subject()
        self.rejected('controller_identity', lambda: resources.reserve_run(other_controller))
        for cls in (s.ControllerHandle, s.RunReservation, s.EngineHandle, s.SystemdSliceParent,
                    s.ExactContainerIdentity):
            self.rejected('ownership_unknown', cls)
        forged = object.__new__(s.RunReservation)
        self.rejected('ownership_unknown', lambda: resources.docker_parent(forged))
        self.rejected('ownership_unknown', lambda: resources.finish_owned_run(forged))
        self.rejected('ownership_unknown', lambda: resources.stop_owned_engine(object.__new__(s.EngineHandle)))
        self.assertFalse(resources._cleaning)
        resources.close()
        self.rejected('controller_identity', lambda: resources.reserve_run(controller))
        self.assertFalse(other._manager.calls)

    def test_wrong_controller_caps_and_membership_fail_before_helpers(self):
        resources, controller = self.subject()
        resources._manager.controller_caps = replace(s.CONTROLLER_LIMIT, memory=65 * 1024**2)
        self.rejected('controller_identity', lambda: resources.reserve_run(controller))
        self.assertFalse(resources._manager.calls)
        resources, controller = self.subject()
        resources._manager.membership = '/foreign.service'
        self.rejected('controller_identity', lambda: resources.reserve_run(controller))
        self.assertFalse(resources._manager.calls)

    def test_replaced_run_or_phase_fd_stops_all_actions(self):
        resources, controller = self.subject()
        run = resources.reserve_run(controller)
        root = self.stage / resources._runs[run]['token']
        (root / 'old').rename(root / 'saved-old')
        (root / 'old').mkdir(mode=0o700)
        self.rejected('stage_identity', lambda: resources.docker_parent(run))
        self.assertFalse(resources._manager.calls)

    def test_partial_allocation_keeps_foreign_files_and_never_dispatches(self):
        resources, controller = self.subject()
        original = resources._exclusive_file
        def fail(parent, name, content):
            if name == 'controller-ledger':
                os.mkdir('foreign', 0o700, dir_fd=parent)
                raise OSError('PUBLIC private exception marker')
            return original(parent, name, content)
        with patch.object(resources, '_exclusive_file', side_effect=fail):
            self.rejected('stage_identity', lambda: resources.reserve_run(controller))
        self.assertEqual(len(list(self.stage.iterdir())), 1)
        self.assertTrue(next(self.stage.iterdir()).joinpath('foreign').is_dir())
        self.assertFalse(resources._manager.calls)

    def test_launch_is_async_and_ledger_precedes_each_fixed_unit_dispatch(self):
        resources, controller = self.subject()
        run = resources.reserve_run(controller)
        engine = resources.launch_engine(run)
        self.assertIs(type(engine), s.EngineHandle)
        record = resources._runs[run]
        self.assertEqual(record['mode'], 'preparation')
        self.assertEqual(len(resources._manager.calls), 2)
        token = record['token']
        self.assertEqual([v[1] for v in resources._manager.calls],
                         ['siftocoolify' + token + '.slice', 'siftocoolify' + token + 'engine.service'])
        self.assertTrue(all(v[2] for v in resources._manager.calls))
        self.assertFalse(resources._manager.acknowledged)
        properties = dict(record['units']['engine']['spec'].properties)
        self.assertEqual(properties['ExecStart'], ('/usr/bin/python3', '-I', '-B',
                                                '/opt/sifto-maintenance/coolify_native_engine.py'))
        envelope = properties['StandardInputData']
        self.assertLessEqual(len(envelope), 256)
        decoded = json.loads(envelope)
        self.assertEqual(set(decoded), {'version', 'run', 'rendezvous'})
        self.assertEqual(decoded['run'], token)
        self.assertRegex(decoded['rendezvous'], '^[a-f0-9]{32}$')
        self.assertNotIn(b'APP_KEY', envelope)
        self.assertNotIn(str(self.root).encode(), envelope)

    def test_lost_response_is_unresolved_and_never_retries_or_adopts_name(self):
        resources, controller = self.subject()
        run = resources.reserve_run(controller)
        resources._manager.lose_response = True
        self.rejected('command_failed', lambda: resources.launch_engine(run))
        self.assertEqual(len(resources._manager.calls), 1)
        self.assertEqual(resources._runs[run]['units']['slice']['state'], 'unresolved')
        self.rejected('ownership_unknown', lambda: resources.launch_engine(run))
        result = resources.finish_owned_run(run)
        self.assertFalse(result.complete)
        self.assertEqual(result.first_failure, 'command_failed')
        self.assertEqual(len(resources._manager.calls), 1)

    def test_prep_refuses_children_and_steady_enables_typed_parent(self):
        resources, controller = self.subject()
        run = resources.reserve_run(controller)
        engine = resources.launch_engine(run)
        self.rejected('resource_isolation', lambda: resources.docker_parent(run))
        self.assertEqual(resources._runs[run]['failure'], 'resource_isolation')
        resources, controller = self.subject()
        run = resources.reserve_run(controller)
        engine = resources.launch_engine(run)
        resources.set_steady(engine)
        parent = resources.docker_parent(run)
        self.assertIs(type(parent), s.SystemdSliceParent)
        self.rejected('resource_isolation', lambda: resources.set_preparation(engine))
        self.assertFalse(resources.finish_owned_run(run).complete)

    def test_engine_replacement_and_foreign_children_prevent_stop(self):
        resources, controller = self.subject()
        run = resources.reserve_run(controller)
        engine = resources.launch_engine(run)
        record = resources._runs[run]
        name = record['units']['engine']['name']
        snapshot = resources._manager.units[name]
        resources._manager.units[name] = replace(snapshot, invocation='f' * 32)
        self.rejected('unit_identity', lambda: resources.stop_owned_engine(engine))
        self.assertFalse(resources._manager.stops)
        resources._manager.units[name] = snapshot
        (resources._manager.cgroup_root / snapshot.control_group.lstrip('/') / 'foreign').mkdir()
        self.rejected('resource_isolation', lambda: resources.stop_owned_engine(engine))
        self.assertFalse(resources._manager.stops)

    def test_cleanup_waits_for_unit_unload_and_cgroup_disappearance(self):
        resources, controller = self.subject()
        run = resources.reserve_run(controller)
        resources.launch_engine(run)
        resources._manager.keep_after_stop = True
        resources._cleanup_deadline = time.monotonic() + 0.025
        result = resources.finish_owned_run(run)
        self.assertFalse(result.complete)
        self.assertEqual(result.first_failure, 'cleanup_incomplete')

    def test_no_new_work_inside_cleanup_reserve_but_cleanup_remains_available(self):
        resources, controller = self.subject()
        run = resources.reserve_run(controller)
        resources._started -= 2281
        self.rejected('command_timeout', lambda: resources.launch_engine(run))
        self.assertTrue(resources.finish_owned_run(run).complete)
        self.assertFalse(resources._manager.calls)

    def test_manager_json_is_closed_duplicate_nonfinite_unknown_types_and_bound(self):
        self.assertIsNotNone(s)
        for raw in (b'{"type":"s","type":"s","data":["x"]}',
                    b'{"type":"s","data":[NaN]}',
                    b'{"type":"s","data":["x"],"private":"PUBLIC"}',
                    b'{"type":"s","data":true}', b'x' * 65537):
            self.rejected('manager_invalid', lambda: s._decode_property(raw, 's'))

    def test_fixed_start_builder_encodes_raw_stdin_bytes_counts_and_no_aux_units(self):
        self.assertIsNotNone(s)
        spec = s._engine_spec('a' * 32, 'b' * 32)
        argv = s._start_argv(spec)
        self.assertEqual(argv[:2], ['/usr/bin/busctl', '--system'])
        self.assertIn('ssa(sv)a(sa(sv))', argv)
        index = argv.index('StandardInputData')
        self.assertEqual(argv[index + 1], 'ay')
        size = int(argv[index + 2])
        self.assertEqual(bytes(int(v) for v in argv[index + 3:index + 3 + size]),
                         dict(spec.properties)['StandardInputData'])
        self.assertEqual(argv[-1], '0')
        self.assertIn('fail', argv)

    def test_native_entry_rejects_uninstalled_source_before_any_transport(self):
        self.assertIsNotNone(s)
        resources = s.SystemdResources()
        self.addCleanup(resources.close)
        policy = s.FixedControllerPolicy(replace(self.context, budget=docker.BudgetPolicy()))
        with patch.object(s.platform, 'system', return_value='Linux'), \
             patch.object(s.platform, 'machine', return_value='aarch64'), \
             patch.object(s, '_NativeManager') as manager:
            self.rejected('source_closure', lambda: resources.verify_controller(policy))
        manager.assert_not_called()

    def test_ancestor_capacity_requires_effective_controllers_and_shared_aggregate(self):
        self.assertIsNotNone(s)
        valid = {'memory.max': 'max', 'pids.max': 'max', 'cpu.max': 'max 100000',
                 'cgroup.controllers': 'cpu memory pids', 'cgroup.subtree_control': 'cpu memory pids'}
        s._validate_ancestor_capacity(valid)
        for key, value in (('memory.max', str(1279 * 1024**2)), ('pids.max', '575'),
                           ('cpu.max', '174999 100000'), ('cgroup.controllers', 'cpu pids'),
                           ('cgroup.subtree_control', 'cpu memory')):
            self.rejected('resource_capacity', lambda: s._validate_ancestor_capacity({**valid, key: value}))

    def test_manager_execstart_projection_rejects_unknown_layout_and_boolean_integer(self):
        self.assertIsNotNone(s)
        argv = ['/usr/bin/python3', '-I', '-B', '/opt/sifto-maintenance/coolify_native_engine.py']
        raw = json.dumps({'type': 'a(sasbttttuii)',
                          'data': [[argv[0], argv, False, 1, 2, 3, 4, 123, 0, 0]]}).encode()
        self.assertEqual(s._decode_property(raw, 'a(sasbttttuii)'), tuple(argv))
        for bad in (raw.replace(b'false', b'0'), raw.replace(b'123', b'true'),
                    raw.replace(b', 0, 0]', b', 0, 0, 0]')):
            self.rejected('manager_invalid', lambda: s._decode_property(bad, 'a(sasbttttuii)'))

    def test_replaced_cgroup_inode_and_lower_effective_caps_reject_before_manager_mutation(self):
        resources, controller = self.subject()
        run = resources.reserve_run(controller)
        engine = resources.launch_engine(run)
        unit = resources._runs[run]['units']['engine']
        path = unit['cgroup']['path']
        (path / 'memory.max').write_text('1')
        self.rejected('resource_isolation', lambda: resources.set_steady(engine))
        FakeManager._caps(path, s.ENGINE_PREPARATION)
        path.rename(resources._manager.cgroup_root / (path.name + '-saved'))
        path.mkdir()
        self.rejected('unit_identity', lambda: resources.stop_owned_engine(engine))
        self.assertFalse(resources._manager.stops)

    def test_controller_local_membership_is_authenticated_before_first_helper(self):
        self.assertIsNotNone(s)
        resources = s.SystemdResources()
        self.addCleanup(resources.close)
        resources._cgroup_root = self.root / 'PUBLIC-cgroup'
        resources._cgroup_root.mkdir()
        with patch.object(s.SystemdResources, '_read_membership', return_value='/foreign.service'), \
             patch.object(s, '_NativeManager') as manager:
            self.rejected('controller_identity', resources._native_controller_preflight)
        manager.assert_not_called()

    def test_delayed_mainpid_is_polled_without_relaunch(self):
        resources, controller = self.subject()
        run = resources.reserve_run(controller)
        resources._manager.delay_pid = 2
        engine = resources.launch_engine(run)
        self.assertIs(type(engine), s.EngineHandle)
        self.assertEqual(len(resources._manager.calls), 2)

    def test_rendezvous_registered_before_launch_and_not_replaced(self):
        resources, controller = self.subject()
        run = resources.reserve_run(controller)
        record = resources._runs[run]
        rendezvous = record['rendezvous']
        resources.launch_engine(run)
        data = dict(record['units']['engine']['spec'].properties)['StandardInputData']
        self.assertEqual(json.loads(data)['rendezvous'], rendezvous)

    def test_complete_empty_cleanup_stops_only_owned_units_and_keeps_controller(self):
        resources, controller = self.subject()
        run = resources.reserve_run(controller)
        resources.launch_engine(run)
        result = resources.finish_owned_run(run)
        self.assertTrue(result.complete)
        self.assertIsNone(result.first_failure)
        self.assertEqual(len(resources._manager.stops), 2)
        self.assertNotIn(s.CONTROLLER_UNIT, resources._manager.stops)

    def test_no_caller_flags_grant_docker_identity_or_empty_cleanup(self):
        for value in (True, {'empty': True}, object.__new__(s.ExactContainerIdentity)):
            resources, controller = self.subject()
            run = resources.reserve_run(controller)
            self.rejected('container_identity', lambda: resources.verify_container(run, value))

    def test_primary_timeout_survives_failed_unresolved_append_and_cleanup_guard(self):
        resources, controller = self.subject()
        run = resources.reserve_run(controller)
        original = resources._record
        def fail_append(record, unit, state):
            if state == 'unresolved':
                raise s.SystemdError('journal_invalid')
            return original(record, unit, state)
        with patch.object(resources._manager, '_start', side_effect=s.SystemdError('command_timeout')), \
             patch.object(resources, '_record', side_effect=fail_append):
            self.rejected('command_timeout', lambda: resources.launch_engine(run))
        record = resources._runs[run]
        (self.stage / record['token'] / 'controller-ledger').write_bytes(b'PUBLIC replaced bytes')
        observation = resources.finish_owned_run(run)
        self.assertFalse(observation.complete)
        self.assertEqual(observation.first_failure, 'command_timeout')

    def test_manager_controlgroup_must_follow_owned_slice_ancestry(self):
        resources, controller = self.subject()
        run = resources.reserve_run(controller)
        resources._manager.foreign_placement = True
        self.rejected('resource_isolation', lambda: resources.launch_engine(run))
        self.assertFalse(resources.finish_owned_run(run).complete)
        self.assertFalse(resources._manager.stops)

    def test_async_stop_polls_and_rechecks_identity_until_unload(self):
        resources, controller = self.subject()
        run = resources.reserve_run(controller)
        resources.launch_engine(run)
        resources._manager.delayed_stop = 2
        result = resources.finish_owned_run(run)
        self.assertTrue(result.complete)
        self.assertEqual(len(resources._manager.stops), 2)

    def test_native_projector_queries_actual_values_and_rejects_boolean_limits(self):
        resources, _ = self.subject()
        spec = s._engine_spec('a' * 32, 'b' * 32)
        calls = []
        bad = False
        class Projector(s._NativeManager):
            def _transport(self, argv):
                calls.append(argv)
                if 'ListUnitsByNames' in argv:
                    value = [[spec.name, 'PUBLIC', 'loaded', 'active', 'running', '',
                        '/org/freedesktop/systemd1/unit/PUBLIC', 0, '', '/']]
                    return json.dumps({'type': 'a(ssssssouso)', 'data': [value]}).encode()
                prop = argv[-1]
                values = dict((*s._limit_properties(spec.limits), *spec.properties))
                values.update(InvocationID=bytes.fromhex('1' * 32), ControlGroup='/PUBLIC',
                              LoadState='loaded', ActiveState='active', MainPID=123,
                              ExecMainStartTimestampMonotonic=456)
                value = values[prop]
                signature = 's'
                if type(value) is bytes:
                    signature, value = 'ay', list(value)
                elif prop == 'ExecStart':
                    signature = 'a(sasbttttuii)'
                    value = [[value[0], list(value), False, 1, 2, 3, 4, 123, 0, 0]]
                elif type(value) is bool:
                    signature = 'b'
                elif type(value) is int:
                    signature = 'u' if prop == 'MainPID' else 't'
                if bad and prop == 'MemoryMax':
                    value = True
                return json.dumps({'type': signature, 'data': value}).encode()
        manager = Projector(resources, os.open(self.stage, os.O_RDONLY | os.O_DIRECTORY))
        self.addCleanup(os.close, manager._output_fd)
        manager._specs[spec.name] = spec
        observed = manager._observe(spec.name)
        self.assertEqual(observed.spec, spec)
        group_call = next(argv for argv in calls if argv[-1] == 'ControlGroup')
        self.assertEqual(group_call[-2], 'org.freedesktop.systemd1.Service')
        bad = True
        self.rejected('manager_invalid', lambda: manager._observe(spec.name))

    def test_native_transport_supervises_exact_fd_and_blocks_bad_controller_before_fork(self):
        resources, _ = self.subject()
        output = os.open(self.stage, os.O_RDONLY | os.O_DIRECTORY)
        self.addCleanup(os.close, output)
        executable = os.open(self.input, os.O_RDONLY)
        self.addCleanup(os.close, executable)
        resources._tools[Path('/usr/bin/busctl')] = {'fd': executable}
        manager = s._NativeManager(resources, output)
        manager._begin()
        argv = s._start_argv(s._slice_spec('a' * 32))
        with patch.object(resources, '_guard_tools'), \
             patch.object(docker, '_supervise', return_value=(0, b'PUBLIC', b'')) as supervise:
            self.assertEqual(manager._transport(argv), b'PUBLIC')
            actual = supervise.call_args
            self.assertEqual(actual.args[0], ['/proc/self/fd/' + str(executable), *argv[1:]])
            self.assertEqual(actual.kwargs['pass_fds'], (executable,))
            self.assertLessEqual(actual.kwargs['limit'], 65536)
            self.assertIs(actual.kwargs['attach'].__self__, resources)
            resources._manager.membership = '/foreign'
            self.rejected('controller_identity', lambda: manager._transport(argv))
            self.assertEqual(supervise.call_count, 1)

    def test_real_busctl_property_and_method_json_shapes_are_distinct(self):
        self.assertEqual(s._decode_property(b'{"type":"s","data":"PUBLIC"}', 's'), 'PUBLIC')
        self.assertEqual(s._decode_property(b'{"type":"ay","data":[1,2]}', 'ay'), b'\x01\x02')
        self.assertEqual(s._decode_method(b'{"type":"o","data":["/PUBLIC"]}', 'o'), '/PUBLIC')
        self.rejected('manager_invalid', lambda: s._decode_property(
            b'{"type":"s","data":["PUBLIC"]}', 's'))
        self.rejected('manager_invalid', lambda: s._decode_method(
            b'{"type":"o","data":"/PUBLIC"}', 'o'))

    def test_true_cgroup_root_has_no_limit_files_but_nonroot_requires_them(self):
        resources, _ = self.subject()
        root = resources._manager.cgroup_root
        for name in ('cgroup.controllers', 'cgroup.subtree_control'):
            (root / name).write_text('cpu memory pids')
        fd = os.open(root, os.O_RDONLY | os.O_DIRECTORY)
        self.addCleanup(os.close, fd)
        anchor = {'fd': fd, 'path': root, 'identity': s._identity(fd, True)}
        resources._guard_ancestor(anchor)
        child = root / 'system.slice'
        child.mkdir()
        for name in ('cgroup.controllers', 'cgroup.subtree_control'):
            (child / name).write_text('cpu memory pids')
        child_fd = os.open(child, os.O_RDONLY | os.O_DIRECTORY)
        self.addCleanup(os.close, child_fd)
        child_anchor = {'fd': child_fd, 'path': child, 'identity': s._identity(child_fd, True)}
        self.rejected('resource_capacity', lambda: resources._guard_ancestor(child_anchor))
        for name, value in (('memory.max', str(s.CONTROLLER_MEMORY)),
                            ('pids.max', '16'), ('cpu.max', '2500 100000')):
            (child / name).write_text(value)
        resources._guard_ancestor(child_anchor)
        (child / 'memory.max').write_text('1')
        self.rejected('resource_capacity', lambda: resources._guard_ancestor(child_anchor))

    def test_cleanup_reserve_covers_native_metadata_and_stopunit(self):
        resources, controller = self.subject()
        run = resources.reserve_run(controller)
        resources.launch_engine(run)
        fake = resources._manager
        output = os.open(self.stage, os.O_RDONLY | os.O_DIRECTORY)
        self.addCleanup(os.close, output)
        executable = os.open(self.input, os.O_RDONLY)
        self.addCleanup(os.close, executable)
        manager = s._NativeManager(resources, output)
        manager.controller_caps, manager.membership = s.CONTROLLER_LIMIT, s.CONTROLLER_GROUP
        manager._specs = {name: value.spec for name, value in fake.units.items()}
        resources._manager = manager
        resources._tools[Path('/usr/bin/busctl')] = {'fd': executable}
        resources._controller_identity = (123, b'PUBLIC-start', 456, '1' * 32, s.CONTROLLER_GROUP)
        resources._controllers[controller] = resources._controller_identity
        calls = []

        def supervise(argv, **kwargs):
            calls.append(argv)
            if 'ListUnitsByNames' in argv:
                name = argv[-1]
                value = ([] if name != s.CONTROLLER_UNIT and name not in fake.units else
                    [[name, 'PUBLIC', 'loaded', 'active', 'running', '',
                      '/org/freedesktop/systemd1/unit/' + ('controller' if name == s.CONTROLLER_UNIT
                                                       else 'engine' if name.endswith('.service') else 'slice'),
                      0, '', '/']])
                return 0, json.dumps({'type': 'a(ssssssouso)', 'data': [value]}).encode(), b''
            if 'StopUnit' in argv:
                fake._stop(argv[-2])
                return 0, b'{"type":"o","data":["/org/freedesktop/systemd1/job/2"]}', b''
            path, prop = argv[-3], argv[-1]
            if path.endswith('/controller'):
                values = dict(s._limit_properties(s.CONTROLLER_LIMIT))
                values.update(MainPID=123, ExecMainStartTimestampMonotonic=456,
                              InvocationID=bytes.fromhex('1' * 32), ControlGroup=s.CONTROLLER_GROUP)
            else:
                kind = 'engine' if path.endswith('/engine') else 'slice'
                old = next(value for value in fake.units.values() if value.spec.kind == kind)
                values = dict((*s._limit_properties(old.spec.limits), *old.spec.properties))
                values.update(MainPID=old.pid, ExecMainStartTimestampMonotonic=old.start,
                    InvocationID=bytes.fromhex(old.invocation), ControlGroup=old.control_group,
                    LoadState='loaded', ActiveState='active')
            value = values[prop]
            signature = 's'
            if type(value) is bytes:
                signature, value = 'ay', list(value)
            elif prop == 'ExecStart':
                signature, value = 'a(sasbttttuii)', [[value[0], list(value), False, 1, 2, 3, 4, 123, 0, 0]]
            elif type(value) is bool:
                signature = 'b'
            elif type(value) is int:
                signature = 'u' if prop == 'MainPID' else 't'
            return 0, json.dumps({'type': signature, 'data': value}).encode(), b''

        resources._started -= 2281
        with patch.object(resources, '_guard_tools'), patch.object(docker, '_supervise', side_effect=supervise):
            result = resources.finish_owned_run(run)
        self.assertTrue(result.complete)
        self.assertEqual(len([argv for argv in calls if 'StopUnit' in argv]), 2)
        self.assertTrue(any(argv[-1] == 'MainPID' for argv in calls))
        other, other_controller = self.subject()
        other_run = other.reserve_run(other_controller)
        self.rejected('ownership_unknown', lambda: other.launch_engine(object.__new__(s.RunReservation)))
        self.assertIsNone(other._runs[other_run]['failure'])
        other._started -= 2281
        self.rejected('command_timeout', lambda: other.launch_engine(reservation=other_run))
        other._started -= 120
        self.assertFalse(other.finish_owned_run(other_run).complete)
        self.assertFalse(other._manager.calls)

    def test_public_guard_first_failure_survives_later_ledger_fault_and_successful_cleanup(self):
        resources, controller = self.subject()
        run = resources.reserve_run(controller)
        resources._started -= 2281
        self.rejected('command_timeout', lambda: resources.launch_engine(run))
        record = resources._runs[run]
        (self.stage / record['token'] / 'controller-ledger').write_bytes(b'PUBLIC changed ledger')
        result = resources.finish_owned_run(run)
        self.assertFalse(result.complete)
        self.assertEqual(result.first_failure, 'command_timeout')
        other, other_controller = self.subject()
        other_run = other.reserve_run(other_controller)
        other._started -= 2281
        self.rejected('command_timeout', lambda: other.launch_engine(other_run))
        clean = other.finish_owned_run(other_run)
        self.assertTrue(clean.complete)
        self.assertEqual(clean.first_failure, 'command_timeout')

    def test_bind_subtree_never_gets_true_mount_root_limit_exemption(self):
        resources, _ = self.subject()
        root = resources._cgroup_root
        for name in ('cgroup.controllers', 'cgroup.subtree_control'):
            (root / name).write_text('cpu memory pids')
        fd = os.open(root, os.O_RDONLY | os.O_DIRECTORY)
        self.addCleanup(os.close, fd)
        anchor = {'fd': fd, 'path': root, 'identity': s._identity(fd, True)}
        resources._guard_ancestor(anchor)
        subtree = b'29 22 0:26 /delegated /sys/fs/cgroup rw - cgroup2 cgroup rw\n'
        for with_limits in (False, True):
            if with_limits:
                for name, value in (('memory.max', '1'), ('pids.max', '1'), ('cpu.max', '1 100000')):
                    (root / name).write_text(value)
            with patch.object(resources, '_mount_observation', return_value=subtree):
                self.rejected('resource_isolation', lambda: resources._guard_ancestor(anchor))

    def test_close_never_closes_reused_foreign_descriptor(self):
        resources, _ = self.subject()
        owned = os.open(self.input, os.O_RDONLY)
        resources._keep(owned)
        foreign = os.open(self.stage, os.O_RDONLY | os.O_DIRECTORY)
        self.addCleanup(os.close, foreign)
        os.dup2(foreign, owned)
        def close_replacement():
            try:
                os.close(owned)
            except OSError:
                pass
        self.addCleanup(close_replacement)
        expected = os.fstat(foreign)
        resources.close()
        try:
            observed = os.fstat(owned)
        except OSError:
            self.fail('close disposed of the reused foreign descriptor')
        self.assertEqual((observed.st_dev, observed.st_ino), (expected.st_dev, expected.st_ino))

    def test_known_context_source_failure_during_cleanup_preserves_static_first_cause(self):
        resources, controller = self.subject()
        run = resources.reserve_run(controller)
        with patch.object(resources._context_guard, '_guard_context',
                          side_effect=docker.DockerError('source_closure')):
            result = resources.finish_owned_run(run)
        self.assertFalse(result.complete)
        self.assertEqual(result.first_failure, 'source_closure')
        other, other_controller = self.subject()
        other_run = other.reserve_run(other_controller)
        other._started -= 2281
        self.rejected('command_timeout', lambda: other.launch_engine(other_run))
        with patch.object(other._context_guard, '_guard_context',
                          side_effect=docker.DockerError('source_closure')):
            result = other.finish_owned_run(other_run)
        self.assertFalse(result.complete)
        self.assertEqual(result.first_failure, 'command_timeout')

    def test_phase_and_stop_preserve_known_errors_without_trusting_arbitrary_text(self):
        for action, method, error, expected in (
            ('set_steady', '_set_limits', docker.DockerError('source_closure'), 'source_closure'),
            ('stop_owned_engine', '_stop', docker.DockerError('source_closure'), 'source_closure'),
            ('set_steady', '_set_limits', ValueError('source_closure'), 'command_failed')):
            with self.subTest(action=action, exception=type(error).__name__):
                resources, controller = self.subject()
                run = resources.reserve_run(controller)
                engine = resources.launch_engine(run)
                with patch.object(resources._manager, method, side_effect=error):
                    self.rejected(expected, lambda: getattr(resources, action)(engine))
                self.assertEqual(resources._runs[run]['failure'], expected)
                result = resources.finish_owned_run(run)
                self.assertFalse(result.complete)
                self.assertEqual(result.first_failure, expected)

    def test_controller_invocation_admits_only_one_reservation_across_instances_and_close(self):
        identity = ('PUBLIC-shared-controller', s.secrets.token_hex(16))
        resources, controller = self.subject(controller_identity=identity)
        run = resources.reserve_run(controller)
        before = set(self.stage.iterdir())
        self.rejected('resource_capacity', lambda: resources.reserve_run(controller))
        self.assertEqual(set(self.stage.iterdir()), before)
        self.assertFalse(resources._manager.calls)
        other, other_controller = self.subject(controller_identity=identity)
        self.rejected('resource_capacity', lambda: other.reserve_run(other_controller))
        self.assertFalse(other._manager.calls)
        self.assertTrue(resources.finish_owned_run(run).complete)
        resources.close()
        third, third_controller = self.subject(controller_identity=identity)
        self.rejected('resource_capacity', lambda: third.reserve_run(third_controller))
        self.assertEqual(set(self.stage.iterdir()), before)

    def test_failed_allocation_and_incomplete_cleanup_never_release_controller_admission(self):
        for fail_allocation in (True, False):
            with self.subTest(fail_allocation=fail_allocation):
                identity = ('PUBLIC-unreleased-controller', s.secrets.token_hex(16))
                resources, controller = self.subject(controller_identity=identity)
                if fail_allocation:
                    with patch.object(resources, '_exclusive_file', side_effect=OSError('PUBLIC')):
                        self.rejected('stage_identity', lambda: resources.reserve_run(controller))
                else:
                    run = resources.reserve_run(controller)
                    resources.launch_engine(run)
                    resources._manager.keep_after_stop = True
                    resources._cleanup_deadline = time.monotonic() + 0.02
                    self.assertFalse(resources.finish_owned_run(run).complete)
                resources.close()
                other, other_controller = self.subject(controller_identity=identity)
                self.rejected('resource_capacity', lambda: other.reserve_run(other_controller))
                self.assertFalse(other._manager.calls)

    def test_public_error_boundary_rejects_subclass_categories_and_throwing_stringifiers(self):
        for base in (s.SystemdError, docker.DockerError):
            for throws in (False, True):
                with self.subTest(base=base.__name__, throws=throws):
                    class Malicious(base):
                        def __str__(self):
                            if throws:
                                raise RuntimeError('PUBLIC private exception text')
                            return 'source_closure'
                    resources, controller = self.subject()
                    run = resources.reserve_run(controller)
                    error = Malicious('source_closure')
                    error.cleanup_complete = True
                    with patch.object(resources._context_guard, '_guard_context', side_effect=error):
                        with self.assertRaises(BaseException) as caught:
                            resources.launch_engine(run)
                    self.assertIs(type(caught.exception), s.SystemdError)
                    self.assertEqual(str(caught.exception), 'command_failed')
                    self.assertFalse(caught.exception.cleanup_complete)
                    self.assertEqual(resources._runs[run]['failure'], 'command_failed')

    def test_transport_diagnostic_stat_fault_cannot_replace_primary_static_failure(self):
        for primary in ('command_timeout', 'output_bound', None):
            with self.subTest(primary=primary):
                resources, _ = self.subject()
                output = os.open(self.stage, os.O_RDONLY | os.O_DIRECTORY)
                self.addCleanup(os.close, output)
                executable = os.open(self.input, os.O_RDONLY)
                self.addCleanup(os.close, executable)
                resources._tools[Path('/usr/bin/busctl')] = {'fd': executable}
                manager = s._NativeManager(resources, output)
                manager._begin()
                original_stat = os.stat
                def stat_fault(path, *args, **kwargs):
                    if type(path) is str and path.startswith('command-'):
                        raise PermissionError('PUBLIC private stat error')
                    return original_stat(path, *args, **kwargs)
                with patch.object(resources, '_guard_tools'), \
                     patch.object(docker, '_supervise', side_effect=docker.DockerError(primary)
                                  if primary else None, return_value=(0, b'PUBLIC', b'')), \
                     patch.object(s.os, 'stat', side_effect=stat_fault):
                    with self.assertRaises(BaseException) as caught:
                        manager._transport(s._start_argv(s._slice_spec('a' * 32)))
                    self.assertIs(type(caught.exception), s.SystemdError)
                    self.assertEqual(str(caught.exception), primary or 'command_failed')

    def test_unresolved_helper_cleanup_cannot_become_success_after_owned_units_disappear(self):
        for cleanup_complete, attested in ((False, False), (True, False), (True, True)):
            with self.subTest(cleanup_complete=cleanup_complete, attested=attested):
                resources, controller = self.subject()
                run = resources.reserve_run(controller)
                resources.launch_engine(run)
                fake = resources._manager
                original_identity = resources._controller_identity
                output = os.open(self.stage, os.O_RDONLY | os.O_DIRECTORY)
                self.addCleanup(os.close, output)
                executable = os.open(self.input, os.O_RDONLY)
                self.addCleanup(os.close, executable)
                manager = s._NativeManager(resources, output)
                manager.controller_caps, manager.membership = s.CONTROLLER_LIMIT, s.CONTROLLER_GROUP
                resources._manager = manager
                resources._tools[Path('/usr/bin/busctl')] = {'fd': executable}
                resources._controller_identity = (123, b'PUBLIC-start', 456, '1' * 32, s.CONTROLLER_GROUP)
                resources._controllers[controller] = resources._controller_identity
                error = docker.DockerError('command_timeout')
                error.cleanup_complete = cleanup_complete
                if attested:
                    error._process_cleanup = docker._PROCESS_CLEANUP
                else:
                    resources._sessions[777] = b'PUBLIC-known-survivor'
                with patch.object(resources, '_guard_tools'), \
                     patch.object(docker, '_supervise', side_effect=error):
                    result = resources.finish_owned_run(run)
                self.assertFalse(result.complete)
                self.assertEqual(result.first_failure, 'command_timeout')
                resources._manager = fake
                resources._controller_identity = original_identity
                resources._controllers[controller] = original_identity
                result = resources.finish_owned_run(run)
                self.assertEqual(result.complete, attested)
                self.assertEqual(result.first_failure, 'command_timeout')
                self.assertFalse(fake.units)
                self.assertFalse(list(fake.cgroup_root.iterdir()))
                if not attested:
                    resources._sessions.clear()  # Disappearance alone is not a reaping witness.
                    self.assertFalse(resources.finish_owned_run(run).complete)

    def test_final_native_controller_baseline_observes_only_self_without_helpers(self):
        self.assertTrue(hasattr(s.SystemdResources, '_controller_baseline'),
                        'helper-free final controller baseline is missing')
        resources, _ = self.subject()
        group = resources._manager.cgroup_root / 'PUBLIC-controller'
        group.mkdir()
        FakeManager._caps(group, s.CONTROLLER_LIMIT)
        (group / 'cgroup.procs').write_text(str(os.getpid()))
        resources._controller_cgroup = resources._open_cgroup('/PUBLIC-controller')
        before = list(resources._manager.calls)
        s.SystemdResources._controller_baseline(resources)
        (group / 'cgroup.procs').write_text(str(os.getpid()) + '\n777\n')
        self.rejected('cleanup_incomplete', lambda: s.SystemdResources._controller_baseline(resources))
        self.assertEqual(resources._manager.calls, before)

    def test_concurrent_same_pid_instances_cannot_allocate_two_workloads(self):
        identity = ('PUBLIC-concurrent-controller', s.secrets.token_hex(16))
        first, first_controller = self.subject(controller_identity=identity)
        second, second_controller = self.subject(controller_identity=identity)
        barrier = threading.Barrier(2)
        def reserve(pair):
            resource, handle = pair
            barrier.wait(timeout=5)
            try:
                resource.reserve_run(handle)
                return 'reserved'
            except s.SystemdError as error:
                return str(error)
        with ThreadPoolExecutor(max_workers=2) as pool:
            results = list(pool.map(reserve, ((first, first_controller), (second, second_controller))))
        self.assertCountEqual(results, ['reserved', 'resource_capacity'])
        self.assertEqual(len(list(self.stage.iterdir())), 1)
        self.assertFalse(first._manager.calls or second._manager.calls)

    def test_deferred_engine_startup_poll_interrupt_preserves_interrupted_category(self):
        for interruption in (KeyboardInterrupt, SystemExit):
            with self.subTest(interruption=interruption.__name__):
                resources, controller = self.subject()
                run = resources.reserve_run(controller)
                resources._manager.delay_pid = 1
                with patch.object(s.time, 'sleep', side_effect=interruption('PUBLIC private signal text')):
                    self.rejected('interrupted', lambda: resources.launch_engine(run))
                self.assertEqual(len(resources._manager.calls), 2)
                self.assertEqual(resources._runs[run]['failure'], 'interrupted')
                result = resources.finish_owned_run(run)
                self.assertFalse(result.complete)
                self.assertEqual(result.first_failure, 'interrupted')

    def test_phase_stop_and_cleanup_guard_interrupts_keep_static_attribution(self):
        for interruption in (KeyboardInterrupt, SystemExit):
            for action, method in (('set_steady', '_set_limits'), ('stop_owned_engine', '_stop'),
                                   ('finish_owned_run', '_guard_context')):
                with self.subTest(interruption=interruption.__name__, action=action):
                    resources, controller = self.subject()
                    run = resources.reserve_run(controller)
                    if action == 'finish_owned_run':
                        with patch.object(resources._context_guard, method,
                                          side_effect=interruption('PUBLIC private signal text')):
                            result = resources.finish_owned_run(run)
                        self.assertFalse(result.complete)
                        self.assertEqual(result.first_failure, 'interrupted')
                    else:
                        engine = resources.launch_engine(run)
                        with patch.object(resources._manager, method,
                                          side_effect=interruption('PUBLIC private signal text')):
                            self.rejected('interrupted', lambda: getattr(resources, action)(engine))
                        record = resources._runs[run]
                        self.assertEqual(record['failure'], 'interrupted')
                        (self.stage / record['token'] / 'controller-ledger').write_bytes(b'PUBLIC later fault')
                        result = resources.finish_owned_run(run)
                        self.assertFalse(result.complete)
                        self.assertEqual(result.first_failure, 'interrupted')

    def test_prior_timeout_remains_first_failure_through_cleanup_interrupt(self):
        for interruption in (KeyboardInterrupt, SystemExit):
            with self.subTest(interruption=interruption.__name__):
                resources, controller = self.subject()
                run = resources.reserve_run(controller)
                resources._started -= 2281
                self.rejected('command_timeout', lambda: resources.launch_engine(run))
                with patch.object(resources._context_guard, '_guard_context',
                                  side_effect=interruption('PUBLIC private signal text')):
                    result = resources.finish_owned_run(run)
                self.assertFalse(result.complete)
                self.assertEqual(result.first_failure, 'command_timeout')

    def test_static_mapper_and_public_boundary_never_read_unknown_exception_class_property(self):
        for base in (Exception, s.SystemdError):
            touched = []
            class Hostile(base):
                @property
                def __class__(self):
                    touched.append(True)
                    raise RuntimeError('PUBLIC private class attribute text')
            with self.subTest(base=base.__name__, boundary='mapper'):
                try:
                    category = s._static_category(Hostile('source_closure'))
                except BaseException:
                    self.fail('static mapper executed an unknown exception attribute')
                self.assertEqual(category, 'command_failed')
                self.assertFalse(touched)
            touched.clear()
            with self.subTest(base=base.__name__, boundary='public'):
                resources, controller = self.subject()
                run = resources.reserve_run(controller)
                with patch.object(resources._context_guard, '_guard_context',
                                  side_effect=Hostile('source_closure')):
                    with self.assertRaises(BaseException) as caught:
                        resources.launch_engine(run)
                self.assertIs(type(caught.exception), s.SystemdError)
                self.assertEqual(str(caught.exception), 'command_failed')
                self.assertFalse(caught.exception.cleanup_complete)
                self.assertEqual(resources._runs[run]['failure'], 'command_failed')
                self.assertFalse(touched)


class FakeManager:
    def __init__(self, stage):
        self.stage = stage
        self.calls = []
        self.controller_caps = s.CONTROLLER_LIMIT
        self.membership = s.CONTROLLER_GROUP
        self.cgroup_root = stage.parent / ('cgroup-' + str(id(self)))
        self.cgroup_root.mkdir(mode=0o700)
        self.units = {}
        self.stops = []
        self.lose_response = False
        self.keep_after_stop = False
        self.acknowledged = False
        self.delay_pid = 0
        self.foreign_placement = False
        self.delayed_stop = 0
        self.pending_stop = None

    def _start(self, spec):
        ledger = next(self.stage.glob('*/controller-ledger')).read_bytes()
        rows = [json.loads(line) for line in ledger.splitlines()]
        durable = any(row['state'] == 'intent' and row['name'] == spec.name for row in rows)
        self.calls.append(('start', spec.name, durable))
        group = '/' + spec.name
        if spec.kind == 'engine' and not self.foreign_placement:
            group = '/' + dict(spec.properties)['Slice'] + '/' + spec.name
        path = self.cgroup_root / group.lstrip('/')
        path.mkdir(mode=0o700)
        observation = s._UnitObservation(spec.name, '1' * 32, group,
            123 if spec.kind == 'engine' else 0, 456 if spec.kind == 'engine' else 0,
            spec, True, True)
        self.units[spec.name] = observation
        self._caps(path, spec.limits)
        if self.lose_response:
            raise OSError('PUBLIC secret error')
        return '/org/freedesktop/systemd1/job/1'

    @staticmethod
    def _caps(path, limits):
        for name, content in {'memory.max': str(limits.memory), 'memory.swap.max': '0',
            'cpu.max': str(limits.cpu * 100) + ' 100000', 'pids.max': str(limits.tasks),
            'cgroup.controllers': 'memory cpu pids', 'cgroup.procs': ''}.items():
            (path / name).write_text(content)

    def _observe(self, name):
        if self.pending_stop == name:
            self.delayed_stop -= 1
            if self.delayed_stop <= 0:
                self.pending_stop = None
                self._remove(name)
        value = self.units.get(name)
        if value is not None and value.spec.kind == 'engine' and self.delay_pid:
            self.delay_pid -= 1
            return replace(value, pid=0, start=0)
        return value

    def _set_limits(self, name, limits):
        old = self.units[name]
        self.units[name] = replace(old, spec=replace(old.spec, limits=limits))
        self._caps(self.cgroup_root / old.control_group.lstrip('/'), limits)

    def _stop(self, name):
        self.stops.append(name)
        if self.keep_after_stop:
            return
        if self.delayed_stop:
            self.pending_stop = name
            return
        self._remove(name)

    def _remove(self, name):
        old = self.units.pop(name)
        path = self.cgroup_root / old.control_group.lstrip('/')
        for child in path.iterdir():
            child.unlink()
        path.rmdir()


if __name__ == '__main__':
    unittest.main()
