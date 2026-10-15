"""PUBLIC synthetic native transport and phase contracts. No real daemon/cgroup."""
from dataclasses import replace
import json
import os
from pathlib import Path
import sys
import tarfile
import tempfile
import threading
import time
import unittest
from unittest.mock import patch

from maintenance import coolify_docker as d
try:
    from maintenance import coolify_rehearsal_engine as engine
except ImportError:
    engine = None
import test_coolify_docker as fixture
from test_coolify_rehearsal import bundle_fixture, archive, NOW
from maintenance import coolify_rehearsal as recovery
import test_coolify_evidence as evidence_fixture
from maintenance import coolify_evidence as evidence


class NativeEngineContracts(unittest.TestCase):
    setUp = fixture.OwnedDockerTests.setUp
    assert_category = fixture.OwnedDockerTests.assert_category
    def test_engine_module_and_fixed_budget_exist(self):
        self.assertIsNotNone(engine, 'N2 native engine is missing')
        self.assertEqual(d.BudgetPolicy().aggregate_memory, 1280 * 1024 * 1024)
        self.assertEqual(d.BudgetPolicy().aggregate_millicpu, 1750)

    def test_missing_budget_and_unverified_startup_reject_before_resources(self):
        self.assertIsNotNone(engine, 'N2 native engine is missing')
        before = list(self.daemon.calls)
        self.assert_category('context_policy', lambda: engine.run_synthetic(self.context, docker=self.owned))
        self.assertEqual(self.daemon.calls, before)
        context = replace(self.context, budget=d.BudgetPolicy())
        owned = fixture.SyntheticOwned(context, adapter=self.daemon)
        self.addCleanup(owned.close)
        self.assert_category('startup_profile_unverified', lambda: engine.run_synthetic(context, docker=owned))
        self.assertEqual(self.daemon.calls, before)

    def test_caller_raised_budget_and_manufactured_material_grant_no_capability(self):
        from maintenance.coolify_rehearsal import ValidatedMaterial, ValidatedArchive
        for budget in (replace(d.BudgetPolicy(), aggregate_memory=2048 * 1024**2),
                       replace(d.BudgetPolicy(), aggregate_millicpu=2000),
                       replace(d.BudgetPolicy(), host_reserve=0), None):
            context = replace(self.context, budget=budget)
            owned = fixture.SyntheticOwned(context, adapter=self.daemon)
            self.addCleanup(owned.close)
            self.assert_category('context_policy', lambda: engine.run_synthetic(context, docker=owned))
        material = ValidatedMaterial('a' * 64, (), ValidatedArchive((), True))
        self.assert_category('context_policy', lambda: engine.run_synthetic(material, docker=self.owned))
        self.assertFalse(self.daemon.calls)

    def test_float_or_boolean_budget_fields_cannot_equal_the_fixed_policy(self):
        for budget in (replace(d.BudgetPolicy(), whole_seconds=2400.0),
                       replace(d.BudgetPolicy(), cleanup_seconds=120.0)):
            owned = fixture.SyntheticOwned(replace(self.context, budget=budget), adapter=self.daemon)
            self.addCleanup(owned.close)
            self.assert_category('context_policy', lambda: engine.run_synthetic(owned.context, docker=owned))

    def test_native_adapter_has_no_unowned_create_or_generic_exec(self):
        native = d.NativeProcess(replace(self.context, budget=d.BudgetPolicy()))
        self.assert_category('container_spec', lambda: native.create(['run', fixture.MARKER], env=d._ENV, pass_fds=()))
        for name in ('exec', 'execute', 'query', 'probe', 'shell', 'publish_proof', 'run_production'):
            self.assertFalse(hasattr(native, name))
            self.assertFalse(hasattr(engine, name))

    def test_purpose_methods_reject_wrong_operation_before_adapter(self):
        run = self.owned.allocate_run()
        identity = self.owned.create(run, operation=d.Operation.IDENTITY_OLD,
                                     phase=d.Phase.RUN, resources=None)
        before = list(self.daemon.calls)
        for name in ('postgres_ready', 'read_probe', 'read_migration',
                     'read_ssh_reconstruction', 'held_ready'):
            with self.subTest(name=name):
                self.assert_category('container_spec', lambda: getattr(self.owned, name)(identity.container_id))
        self.assertEqual(self.daemon.calls, before)


def detached_evidence(version='4.4.2', *, baseline=False):
    helper = evidence_fixture.EvidenceContracts()
    helper.catalog = evidence.fixed_catalog()
    # Historical inert row proves the engine preserves non-mainline history.
    return helper.fixture(version, baseline=baseline,
        history=(evidence.MAINLINE_RETIRED_MIGRATIONS[0],))


def large_public_bundle():
    raw, binding = bundle_fixture()
    files = {name: data for name, _, data in recovery.inspect_archive(raw, recovery=False).entries}
    dump = b'PGDMP' + b'PUBLIC synthetic dump\n' * 100000
    files['coolify.dump'] = dump
    hashes = {name: fixture.sha(files[name]) for name in recovery.FILES}
    proof = json.loads(files['proof.json'])
    proof['retrieved_file_sha256'] = hashes
    proof['offhost_sha256'] = hashes['coolify.dump']
    files['proof.json'] = json.dumps(proof).encode()
    binding = replace(binding, file_sha256=tuple(sorted(hashes.items())),
                      proof_sha256=fixture.sha(files['proof.json']))
    return archive([(name, tarfile.REGTYPE, data) for name, data in files.items()]), binding, dump


class PhaseDaemon(fixture.FakeDaemon):
    def __init__(self):
        super().__init__()
        self.actions = []
        self.ready_delays = 0
        self.fault = None
        self.producer_edit = None
        self.restores = []

    def start(self, cid):
        super().start(cid)
        value = self.containers[cid]
        operation = value['Labels']['sifto.operation']
        self.actions.append((value['Labels']['sifto.phase'], operation, cid))
        if self.fault == operation:
            raise TimeoutError(fixture.MARKER)

    def wait_result(self, cid, *, seconds):
        value = self.containers[cid]
        operation = value['Labels']['sifto.operation']
        if self.fault == 'wait:' + operation:
            raise TimeoutError(fixture.MARKER)
        value['State'].update(Status='exited', Running=False, ExitCode=0)
        if operation.startswith('identity_'):
            role = operation[9:]
            uid = 70 if role == 'postgres' else 999 if role == 'redis' else 9999
            account = role if role in ('postgres', 'redis') else 'www-data'
            return f'uid={uid}({account}) gid={uid}({account}) groups={uid}({account})\n'.encode()
        if operation.startswith('network_'):
            result = {'ok': True, 'category': 'network_verified', 'network': True}
        elif operation.startswith(('verify_', 'baseline_')):
            result = detached_evidence('4.4.6' if operation.endswith('target') else '4.4.2',
                                       baseline=operation.startswith('baseline_'))
        else:
            return b'PUBLIC artisan output\n'
        if self.producer_edit:
            return self.producer_edit(operation, result)
        return json.dumps(result).encode()

    def postgres_ready(self, cid):
        self.actions.append(('ready', cid))
        if self.ready_delays:
            self.ready_delays -= 1
            return False
        return True

    def restore_original(self, cid, *, dump_fd):
        if self.fault == 'restore':
            raise TimeoutError(fixture.MARKER)
        parts = []
        while True:
            part = os.read(dump_fd, 65536)
            if not part:
                break
            parts.append(part)
        self.restores.append((cid, b''.join(parts)))
        self.actions.append(('restore', cid))


class Task4ModelOwned(fixture.SyntheticOwned):
    """Unit-local registry model, never accepted by NativeProcess."""
    def _require_verified_startup(self):
        if self.context.startup != d.ControlledStartup('c' * 64, 'PUBLIC-UNIT-TASK4', 'synthetic-task4'):
            raise d.DockerError('startup_profile_unverified')

    def _capacity_observations(self):
        return 8 * 1024**3, 16 * 1024**3

    @staticmethod
    def _operation(operation, phase):
        if operation in (d.Operation.SSH_OLD, d.Operation.SSH_TARGET, d.Operation.HELD_OLD, d.Operation.HELD_TARGET):
            d._require(phase in (d.Phase.OLD, d.Phase.FRESH), 'container_spec')
            return ('target' if operation.value.endswith('target') else 'old', 'migrate',
                    ('runtime.env', 'storage', 'bootstrap-cache'))
        return d.OwnedDocker._operation(operation, phase)

    def read_ssh_reconstruction(self, cid):
        record, _ = self._purpose(cid, {d.Operation.SSH_OLD, d.Operation.SSH_TARGET})
        self._require_verified_startup()
        self._finished(record, 30)
        return True

    def held_ready(self, cid):
        _, value = self._purpose(cid, {d.Operation.HELD_OLD, d.Operation.HELD_TARGET})
        self._require_verified_startup()
        return value['State']['Running']


class PhaseContracts(unittest.TestCase):
    assert_category = fixture.OwnedDockerTests.assert_category

    def setUp(self):
        fixture.OwnedDockerTests.setUp(self)
        self.owned.close()
        raw, binding = bundle_fixture()
        self.input.write_bytes(raw)
        self.daemon = PhaseDaemon()
        self.context = replace(self.context, binding=binding, budget=d.BudgetPolicy(),
            startup=d.ControlledStartup('c' * 64, 'PUBLIC-UNIT-TASK4', 'synthetic-task4'))
        self.owned = Task4ModelOwned(self.context, adapter=self.daemon)
        self.addCleanup(self.owned.close)

    def run_engine(self):
        with patch('maintenance.coolify_rehearsal.time.time', return_value=NOW):
            return engine.run_synthetic(self.context, docker=self.owned)

    def test_complete_fixed_old_target_fresh_sequence_and_original_restore(self):
        self.daemon.ready_delays = 2
        self.assertEqual(self.run_engine(), {'ok': True, 'scope': 'synthetic-only', 'category': 'synthetic_verified'})
        operations = [a[1] for a in self.daemon.actions if len(a) == 3]
        self.assertEqual(operations, ['identity_old', 'identity_target', 'identity_postgres', 'identity_redis',
            'postgres', 'redis', 'baseline_old', 'network_old', 'migrate_old', 'ssh_old', 'held_old', 'verify_old',
            'migrate_target', 'ssh_target', 'held_target', 'network_target', 'verify_target',
            'postgres', 'redis', 'baseline_old', 'network_old', 'migrate_old', 'ssh_old', 'held_old', 'verify_old'])
        self.assertEqual([v for _, v in self.daemon.restores], [b'PGDMP-synthetic'] * 2)
        self.assertEqual(len({cid for cid, _ in self.daemon.restores}), 2)
        self.assertFalse(self.daemon.containers)
        self.assertEqual(len([a for a in self.daemon.actions if a[0] == 'ready']), 4)

    def test_migration_restore_and_create_timeout_stop_before_fresh(self):
        for fault in ('restore', 'migrate_old', 'wait:migrate_target', 'postgres'):
            with self.subTest(fault=fault):
                self.daemon.fault = fault
                self.assert_category('command_timeout', self.run_engine)
                self.assertFalse(self.daemon.containers)
                self.assertFalse(any(a[0] == 'fresh' for a in self.daemon.actions))
                self.daemon.fault = None

    def test_strict_producer_invalid_utf8_duplicates_nonfinite_error_unknown(self):
        for raw in (b'\xff', b'{"ok":true,"ok":true}', b'{"x":NaN}',
                    b'{"ok":false,"category":"PUBLIC-SECRET"}', b'{}', b'x' * (512 * 1024 + 1)):
            with self.subTest(raw_kind=raw[:20]):
                self.daemon.producer_edit = lambda *_: raw
                with self.assertRaises(d.DockerError) as caught:
                    self.run_engine()
                self.assertIn(str(caught.exception), ('evidence_invalid', 'output_bound'))
                self.assertFalse(self.daemon.containers)

    def test_changed_evidence_aborts_before_fresh(self):
        def edited(operation, value):
            if operation == 'verify_target':
                value['core']['users']['digest'] = 'f' * 64
            return json.dumps(value).encode()
        self.daemon.producer_edit = edited
        self.assert_category('evidence_difference', self.run_engine)
        self.assertFalse(any(a[0] == 'fresh' for a in self.daemon.actions))

    def test_cleanup_failure_forbids_success(self):
        self.daemon.remove_fault = True
        with self.assertRaises(d.DockerError) as caught:
            self.run_engine()
        self.assertEqual(str(caught.exception), 'command_failed')
        self.assertIs(caught.exception.cleanup_complete, False)

    def test_large_original_dump_has_no_retained_bytes_and_both_original_restores_match(self):
        raw, binding, dump = large_public_bundle()
        self.owned.close()
        self.input.write_bytes(raw)
        self.context = replace(self.context, binding=binding)
        self.owned = Task4ModelOwned(self.context, adapter=self.daemon)
        self.addCleanup(self.owned.close)
        original = engine.prepare_owned_run
        registrations = []
        def prepared(*args, **kwargs):
            run = original(*args, **kwargs)
            registrations.extend(run.original_files.values())
            return run
        with patch.object(engine, 'prepare_owned_run', side_effect=prepared):
            self.assertTrue(self.run_engine()['ok'])
        self.assertTrue(registrations)
        self.assertTrue(all('bytes' not in record for record in registrations))
        self.assertEqual([fixture.sha(data) for _, data in self.daemon.restores], [fixture.sha(dump)] * 2)

    def test_original_tail_chunk_tamper_fails_before_phase_creation(self):
        raw, binding, _ = large_public_bundle()
        self.owned.close()
        self.input.write_bytes(raw)
        self.context = replace(self.context, binding=binding)
        self.owned = Task4ModelOwned(self.context, adapter=self.daemon)
        self.addCleanup(self.owned.close)
        with patch('maintenance.coolify_rehearsal.time.time', return_value=NOW):
            run = engine.prepare_owned_run(self.context, self.owned)
            record = run.original_files['original-coolify.dump']
            os.pwrite(record['fd'], b'X', 1024 * 1024 + 7)
            self.assert_category('material_binding', lambda: engine.prepare_phase(run, d.Phase.OLD))
        self.assertFalse(any(len(a) == 3 and a[0] == 'old' for a in self.daemon.actions))

    def test_removal_timeout_and_interrupt_preserve_first_static_category(self):
        for category in ('command_timeout', 'interrupted'):
            with self.subTest(category=category), patch.object(self.owned, 'remove', side_effect=d.DockerError(category)):
                with self.assertRaises(d.DockerError) as caught:
                    self.run_engine()
                self.assertEqual(str(caught.exception), category)
                self.assertIs(caught.exception.cleanup_complete, False)
            for run in self.owned._runs:
                self.owned.cleanup(run)

    def test_every_original_validation_is_under_preparation_cap_with_no_containers(self):
        self.owned.close()
        daemon = QuotaDaemon()
        self.owned = Task4ModelOwned(self.context, adapter=daemon)
        self.addCleanup(self.owned.close)
        original = engine.validate_bundle
        observed = []
        def validate(*args, **kwargs):
            observed.append((daemon._cgroup._mode, len(daemon.containers)))
            return original(*args, **kwargs)
        with patch.object(engine, 'validate_bundle', side_effect=validate):
            self.assertTrue(self.run_engine()['ok'])
        self.assertEqual(observed, [('preparation', 0)] * 3)

    def test_cleanup_success_and_failure_share_one_total_deadline(self):
        self.owned.close()
        daemon = QuotaDaemon()
        self.owned = Task4ModelOwned(self.context, adapter=daemon)
        self.addCleanup(self.owned.close)
        with patch('maintenance.coolify_rehearsal.time.time', return_value=NOW):
            run = engine.prepare_owned_run(self.context, self.owned)
        clock = [100.0]
        with patch.object(engine.time, 'monotonic', side_effect=lambda: clock[0]):
            engine.cleanup_owned_success(run)
            self.assertEqual(run.cleanup_deadline, 220.0)
            self.assertEqual(daemon._deadline, 220.0)
            clock[0] = 219.0
            self.assertTrue(engine.cleanup_owned_failure(run))
            self.assertEqual(run.cleanup_deadline, 220.0)
            clock[0] = 221.0
            self.assertFalse(engine.cleanup_owned_failure(run))

    def test_slow_success_cleanup_cannot_gain_a_second_failure_allowance(self):
        with patch('maintenance.coolify_rehearsal.time.time', return_value=NOW):
            run = engine.prepare_owned_run(self.context, self.owned)
        clock = [100.0]
        def elapsed(*_):
            clock[0] = 221.0
            return True
        with patch.object(engine.time, 'monotonic', side_effect=lambda: clock[0]), patch.object(d.OwnedDocker, 'cleanup', side_effect=elapsed) as cleanup:
            self.assert_category('cleanup_incomplete', lambda: engine.cleanup_owned_success(run))
            self.assertFalse(engine.cleanup_owned_failure(run))
            self.assertEqual(run.cleanup_deadline, 220.0)
            self.assertEqual(cleanup.call_count, 1)

    def test_owned_container_cleanup_cannot_promote_unreaped_private_transport(self):
        self.owned.close()
        daemon = QuotaDaemon()
        self.owned = Task4ModelOwned(self.context, adapter=daemon)
        self.addCleanup(self.owned.close)
        failure = d.DockerError('command_timeout')
        failure.cleanup_complete = True
        def failed(*args, **kwargs):
            daemon._transport_clean = False
            raise failure
        with patch.object(daemon, 'restore_original', side_effect=failed):
            with self.assertRaises(d.DockerError) as caught:
                self.run_engine()
        self.assertEqual(str(caught.exception), 'command_timeout')
        self.assertIs(caught.exception.cleanup_complete, False)
        self.assertFalse(daemon.containers)
        self.assertTrue(daemon._cgroup.allocated)

    def test_first_timeout_or_interrupt_survives_cleanup_failure_and_untrusted_flag(self):
        original = self.daemon.wait_result
        for category in ('command_timeout', 'interrupted'):
            failure = d.DockerError(category)
            failure.cleanup_complete = True
            def failed(cid, *, seconds):
                if self.daemon.containers[cid]['Labels']['sifto.operation'] == 'migrate_target':
                    self.daemon.remove_fault = True
                    raise failure
                return original(cid, seconds=seconds)
            with self.subTest(category=category), patch.object(self.daemon, 'wait_result', side_effect=failed):
                with self.assertRaises(d.DockerError) as caught:
                    self.run_engine()
                self.assertEqual(str(caught.exception), category)
                self.assertIs(caught.exception.cleanup_complete, False)
            self.daemon.remove_fault = False
            # Cleanup the previous public synthetic run before the next case.
            for run in self.owned._runs:
                self.owned.cleanup(run)

    def test_original_failure_records_verified_cleanup_true(self):
        self.daemon.fault = 'restore'
        with self.assertRaises(d.DockerError) as caught:
            self.run_engine()
        self.assertEqual(str(caught.exception), 'command_timeout')
        self.assertIs(caught.exception.cleanup_complete, True)

    def test_source_change_before_fresh_forbids_fresh_creation(self):
        original = engine.cleanup_phase
        def changed(run, phase):
            original(run, phase)
            (self.source / 'maintenance/coolify_plan.py').write_text('PUBLIC changed source')
        with patch.object(engine, 'cleanup_phase', side_effect=changed):
            with self.assertRaises(d.DockerError) as caught:
                self.run_engine()
        self.assertIn(str(caught.exception), ('source_closure', 'cleanup_incomplete'))
        self.assertFalse(any(a[0] == 'fresh' for a in self.daemon.actions))

    def test_descriptor_change_before_target_never_creates_target(self):
        original = engine.replace_app_and_migrate
        def changed(run):
            self.daemon.image_fault = lambda v: v['descriptor'].update(digest='sha256:' + 'f' * 64)
            return original(run)
        with patch.object(engine, 'replace_app_and_migrate', side_effect=changed):
            with self.assertRaises(d.DockerError) as caught:
                self.run_engine()
        self.assertIn(str(caught.exception), ('image_descriptor', 'cleanup_incomplete'))
        self.assertFalse(any(len(a) == 3 and a[0] == 'old' and a[1].endswith('target') for a in self.daemon.actions))

    def test_hmac_actual_copy_wrong_bytes_and_owner_reject_before_postgres(self):
        original = engine._write_leaf
        for defect in ('bytes', 'owner'):
            def changed(parent, name, content, uid, gid):
                if name == 'hmac.key':
                    content = b'X' * 32 if defect == 'bytes' else content
                    uid = 0 if defect == 'owner' else uid
                return original(parent, name, content, uid, gid)
            with self.subTest(defect=defect), patch.object(engine, '_write_leaf', side_effect=changed):
                self.assert_category('hmac_identity', self.run_engine)
            self.assertFalse(any(len(a) == 3 and a[1] == 'postgres' for a in self.daemon.actions))

    def test_binding_and_capacity_fail_before_any_container(self):
        self.input.write_bytes(bundle_fixture(proof_edit=lambda p: p.update(host_id='PUBLIC-other'))[0])
        self.owned.close()
        self.owned = Task4ModelOwned(self.context, adapter=self.daemon)
        self.addCleanup(self.owned.close)
        self.assert_category('material_binding', self.run_engine)
        self.assertFalse(any(c[0] != 'lookup' for c in self.daemon.calls))

    def test_whole_deadline_prevents_migration_retry_and_fresh_work(self):
        original = engine._one_shot
        def expired(run, operation, phase, reader):
            value = original(run, operation, phase, reader)
            if operation is d.Operation.MIGRATE_TARGET:
                run.deadline = time.monotonic() - 1
            return value
        with patch.object(engine, '_one_shot', side_effect=expired):
            self.assert_category('command_timeout', self.run_engine)
        self.assertEqual(sum(len(a) == 3 and a[1] == 'migrate_target' for a in self.daemon.actions), 1)
        self.assertFalse(any(a[0] == 'fresh' for a in self.daemon.actions))

    def test_unverified_profile_can_never_be_used_by_real_owned_docker(self):
        owned = fixture.SyntheticOwned(self.context, adapter=self.daemon)
        self.addCleanup(owned.close)
        self.assert_category('startup_profile_unverified', lambda: engine.run_synthetic(self.context, docker=owned))
        self.assertFalse(self.daemon.calls)

    def test_insufficient_capacity_rejects_before_allocation(self):
        for observations in ((d.BudgetPolicy().aggregate_memory, 16 * 1024**3),
                             (8 * 1024**3, 4 * 1024**3 - 1)):
            with self.subTest(observations=observations), patch.object(self.owned, '_capacity_observations', return_value=observations):
                self.assert_category('resource_capacity', self.run_engine)
                self.assertFalse(self.owned._runs)
                self.assertFalse(self.daemon.calls)

    def test_native_preparation_placement_precedes_bundle_validation(self):
        group = FakeCgroups()
        native = d.NativeProcess(self.context)
        # Test-only authenticated registry wiring. NativeProcess._bind_owner
        # refuses this subclass in real code; no native happy path is enabled.
        native._owner = self.owned
        self.owned._adapter = native
        def invoke(argv, **_):
            if argv[0] == 'info':
                return b'{"driver":"cgroupfs","version":"2"}'
            if argv[:2] == ['container', 'ls']:
                return b''
            raise AssertionError('unexpected native command')
        observed = []
        def validate(*args, **kwargs):
            observed.append(group.placements[os.getpid()])
            self.assertEqual(group.values[group.native / 'memory.max'], str(1280 * 1024**2))
            raise d.DockerError('material_binding')
        with patch.object(native, '_invoke', side_effect=invoke), patch.object(d, 'CgroupBudget', return_value=group), patch.object(engine, 'validate_bundle', side_effect=validate):
            self.assert_category('material_binding', lambda: engine.prepare_owned_run(self.context, self.owned))
        self.assertEqual(observed, [group.native])
        self.assertEqual(group.placements[os.getpid()], group.entry)

    def test_bad_entry_or_parent_preflight_precedes_any_docker_info_subprocess(self):
        for path_role in ('entry', 'parent'):
            group = FakeCgroups()
            group.values[getattr(group, path_role) / 'memory.max'] = 'max'
            native = d.NativeProcess(self.context)
            native._owner = self.owned  # Unit-local registry wiring only.
            run = self.owned.allocate_run()
            with self.subTest(path_role=path_role), patch.object(d, 'CgroupBudget', return_value=group), patch.object(d, '_supervise') as supervise:
                self.assert_category('resource_isolation', lambda: native._bind_run(run))
                supervise.assert_not_called()
                self.assertEqual(group.writes, [])

    def test_old_phase_cleanup_has_one_deadline_reused_on_failure(self):
        with patch('maintenance.coolify_rehearsal.time.time', return_value=NOW):
            run = engine.prepare_owned_run(self.context, self.owned)
            engine.restore_and_hold_original(run, d.Phase.OLD)
        run.deadline = 1000.0
        clock = [100.0]
        def elapsed(cid):
            clock[0] = 221.0
            raise d.DockerError('command_timeout')
        with patch.object(engine.time, 'monotonic', side_effect=lambda: clock[0]), patch.object(self.owned, 'remove', side_effect=elapsed):
            self.assert_category('command_timeout', lambda: engine.cleanup_phase(run, d.Phase.OLD))
            self.assertEqual(run.cleanup_deadline, 220.0)
            with patch.object(d.OwnedDocker, 'cleanup') as cleanup:
                self.assertFalse(engine.cleanup_owned_failure(run))
                cleanup.assert_not_called()
            self.assertEqual(run.cleanup_deadline, 220.0)

    def test_successful_old_cleanup_clears_phase_allowance_after_verified_removal(self):
        self.owned.close()
        daemon = QuotaDaemon()
        self.owned = Task4ModelOwned(self.context, adapter=daemon)
        self.addCleanup(self.owned.close)
        with patch('maintenance.coolify_rehearsal.time.time', return_value=NOW):
            run = engine.prepare_owned_run(self.context, self.owned)
            engine.restore_and_hold_original(run, d.Phase.OLD)
        run.deadline = 1000.0
        seen = []
        original = self.owned.remove
        def removing(cid):
            seen.append((run.cleanup_deadline, daemon._deadline))
            return original(cid)
        with patch.object(engine.time, 'monotonic', return_value=100.0), patch.object(self.owned, 'remove', side_effect=removing):
            engine.cleanup_phase(run, d.Phase.OLD)
        self.assertEqual(seen, [(220.0, 220.0)] * 3)
        self.assertIsNone(run.cleanup_deadline)
        self.assertEqual(daemon._deadline, 1000.0)

    def test_non_main_signal_context_rejects_before_owned_work(self):
        result = []
        def attempt():
            try:
                self.run_engine()
                result.append('unexpected_success')
            except d.DockerError as error:
                result.append(str(error))
        thread = threading.Thread(target=attempt)
        thread.start()
        thread.join(5)
        self.assertFalse(thread.is_alive())
        self.assertEqual(result, ['context_policy'])
        self.assertFalse(self.daemon.calls)
        self.assertFalse(self.owned._runs)


class TransportContracts(unittest.TestCase):
    def setUp(self):
        self.temp = tempfile.TemporaryDirectory()
        self.addCleanup(self.temp.cleanup)
        self.path = Path(self.temp.name)
        self.fd = os.open(self.path, os.O_RDONLY | os.O_DIRECTORY)
        self.addCleanup(os.close, self.fd)
        self.serial = 0

    def command(self, source, *, limit=65536, seconds=2, pass_fds=()):
        self.serial += 1
        return d._supervise([sys.executable, '-c', source], output_fd=self.fd,
            prefix='command-' + str(self.serial), pass_fds=pass_fds,
            deadline=time.monotonic() + seconds, limit=limit)

    def test_supervisor_concurrently_drains_both_pipes_and_private_modes(self):
        code, stdout, stderr = self.command('import os\nfor i in range(32):\n os.write(1,b"a"*1024)\n os.write(2,b"b"*1024)')
        self.assertEqual((code, len(stdout), len(stderr)), (0, 32768, 32768))
        self.assertTrue(all((p.stat().st_mode & 0o777) == 0o600 for p in self.path.iterdir()))

    def test_both_pipe_floods_and_descendant_early_exit_are_bounded_and_reaped(self):
        for fd in (1, 2):
            before = time.monotonic()
            with self.assertRaisesRegex(d.DockerError, '^output_bound$'):
                self.command(f'import os\nwhile True: os.write({fd},b"x"*4096)', limit=4096)
            self.assertLess(time.monotonic() - before, 3)
        before = time.monotonic()
        with self.assertRaisesRegex(d.DockerError, '^command_timeout$'):
            self.command('import os,time\np=os.fork()\nif p: os._exit(0)\ntime.sleep(30)', seconds=0.1)
        self.assertLess(time.monotonic() - before, 3)
        with self.assertRaises(ChildProcessError):
            os.waitpid(-1, os.WNOHANG)

    def test_only_exact_pass_fds_and_minimal_environment_reach_exec(self):
        one, two = os.pipe()
        self.addCleanup(os.close, one)
        self.addCleanup(os.close, two)
        with patch.dict(os.environ, {'APP_KEY': fixture.MARKER, 'SSH_AUTH_SOCK': fixture.MARKER}):
            _, raw, _ = self.command('import os,json\nprint(json.dumps(dict(os.environ)))')
        value = json.loads(raw)
        self.assertEqual(value, {'PATH': '/usr/bin:/bin', 'HOME': '/nonexistent', 'LC_ALL': 'C'})
        code, raw, _ = self.command(f'import os\nprint(os.fstat({one}).st_mode)', pass_fds=(one,))
        self.assertEqual(code, 0)
        self.assertTrue(raw.strip().isdigit())
        with self.assertRaisesRegex(d.DockerError, '^command_failed$'):
            self.command('print("PUBLIC")', pass_fds=(999999,))

    def test_pid_start_token_drift_never_signals_reused_process_group(self):
        # The real supervisor is still our child. Simulated /proc PID reuse must
        # prevent the group signal while pidfd cleanup still targets that child.
        original = d._start_token
        calls = [0]
        def changed(pid):
            calls[0] += 1
            return original(pid) if calls[0] == 1 else b'PUBLIC-REUSED-TOKEN'
        with patch.object(d, '_start_token', side_effect=changed), patch('os.killpg', wraps=os.killpg) as kill_group:
            with self.assertRaisesRegex(d.DockerError, '^(command_failed|ownership_unknown|command_timeout)$'):
                self.command('import time; time.sleep(0.1)', seconds=0.05)
            kill_group.assert_not_called()

    def test_sigterm_resistant_descendants_are_killed_and_reaped(self):
        source = 'import os,time,signal\nsignal.signal(signal.SIGTERM,signal.SIG_IGN)\np=os.fork()\nif p: os._exit(0)\ntime.sleep(30)'
        before = time.monotonic()
        with self.assertRaisesRegex(d.DockerError, '^command_timeout$'):
            self.command(source, seconds=0.05)
        self.assertLess(time.monotonic() - before, 3)
        with self.assertRaises(ChildProcessError):
            os.waitpid(-1, os.WNOHANG)

    def test_output_failure_is_not_overwritten_by_later_flood_or_timeout(self):
        source = 'import os,signal,time\nsignal.signal(signal.SIGTERM,signal.SIG_IGN)\nwhile True:\n os.write(1,b"a"*8192)\n os.write(2,b"b"*8192)'
        with self.assertRaisesRegex(d.DockerError, '^output_bound$') as caught:
            self.command(source, limit=4096, seconds=0.1)
        self.assertIs(caught.exception.cleanup_complete, True)

    def test_output_failure_precedes_interrupt_during_drain(self):
        import selectors
        real_selector = selectors.DefaultSelector
        class InterruptAfterData:
            def __init__(self):
                self.inner = real_selector()
                self.armed = self.interrupted = False
            def __getattr__(self, name):
                return getattr(self.inner, name)
            def select(self, timeout):
                if self.armed and not self.interrupted:
                    self.interrupted = True
                    raise KeyboardInterrupt(fixture.MARKER)
                ready = self.inner.select(timeout)
                self.armed = self.armed or bool(ready)
                return ready
        source = 'import os,signal,time\nsignal.signal(signal.SIGTERM,signal.SIG_IGN)\nos.write(1,b"a"*8192)\ntime.sleep(30)'
        with patch.object(selectors, 'DefaultSelector', InterruptAfterData):
            with self.assertRaisesRegex(d.DockerError, '^output_bound$'):
                self.command(source, limit=4096, seconds=0.2)

    def test_actual_staged_file_usage_rejects_sparse_growth_without_reading_it(self):
        leaf = os.open('PUBLIC-sparse-data', os.O_WRONLY | os.O_CREAT | os.O_EXCL, 0o600, dir_fd=self.fd)
        try:
            os.ftruncate(leaf, 4 * 1024**3 + 1)
        finally:
            os.close(leaf)
        self.assertTrue(hasattr(engine, '_staged_usage'), 'actual staged byte bound is missing')
        with self.assertRaisesRegex(d.DockerError, '^resource_capacity$'):
            engine._staged_usage(self.fd)

    def test_staged_scan_does_not_follow_app_owned_symlinks(self):
        os.symlink('/PUBLIC-not-read', self.path / 'PUBLIC-link')
        self.assertTrue(hasattr(engine, '_staged_usage'), 'actual staged byte bound is missing')
        with self.assertRaisesRegex(d.DockerError, '^material_binding$'):
            engine._staged_usage(self.fd)

    def test_real_child_sigterm_runs_owned_cleanup_and_restores_handler(self):
        source = '''import sys,os,signal,json
sys.path.insert(0,"/src/tests")
import test_coolify_rehearsal_engine as f
case=f.PhaseContracts()
case.setUp()
prior=signal.getsignal(signal.SIGTERM)
def terminate(*args,**kwargs): os.kill(os.getpid(),signal.SIGTERM)
case.daemon.restore_original=terminate
try:
 case.run_engine()
 result={"unexpected_success":True}
except f.d.DockerError as error:
 result={"category":str(error),"clean":error.cleanup_complete,"empty":not case.daemon.containers,"restored":signal.getsignal(signal.SIGTERM)==prior}
finally: case.doCleanups()
print(json.dumps(result))
'''
        code, raw, _ = self.command(source, seconds=10)
        self.assertEqual(code, 0)
        self.assertEqual(json.loads(raw), {'category': 'interrupted', 'clean': True, 'empty': True, 'restored': True})

    def test_sigterm_during_child_cleanup_preserves_first_error_and_handler(self):
        source = '''import sys,os,signal,json
sys.path.insert(0,"/src/tests")
import test_coolify_rehearsal_engine as f
case=f.PhaseContracts()
case.setUp()
prior=signal.getsignal(signal.SIGTERM)
def failed(*args,**kwargs): raise f.d.DockerError("evidence_invalid")
case.daemon.restore_original=failed
original=case.daemon.stop
def stopped(cid):
 if case.daemon.containers[cid]['Labels']['sifto.phase'] != 'run':
  os.kill(os.getpid(),signal.SIGTERM)
 original(cid)
case.daemon.stop=stopped
try:
 case.run_engine()
 result={"unexpected_success":True}
except f.d.DockerError as error:
 result={"category":str(error),"clean":error.cleanup_complete,"empty":not case.daemon.containers,"restored":signal.getsignal(signal.SIGTERM)==prior}
finally: case.doCleanups()
print(json.dumps(result))
'''
        code, raw, _ = self.command(source, seconds=10)
        self.assertEqual(code, 0)
        self.assertEqual(json.loads(raw), {'category': 'evidence_invalid', 'clean': True, 'empty': True, 'restored': True})

    def test_real_supervisor_identity_is_verified_before_child_exec(self):
        marker = self.path / 'PUBLIC-exec-marker'
        observed = []
        def inherited(pid, token):
            self.assertEqual(d._start_token(pid), token)
            self.assertEqual(os.getsid(pid), pid)
            self.assertEqual(os.getpgid(pid), pid)
            self.assertFalse(marker.exists())
            observed.append(pid)
        code, raw, _ = d._supervise([sys.executable, '-c',
            'from pathlib import Path; Path(' + repr(str(marker)) + ').touch(); print("PUBLIC child")'],
            output_fd=self.fd, prefix='command-1', deadline=time.monotonic() + 3,
            limit=4096, attach=inherited)
        self.assertEqual((code, raw), (0, b'PUBLIC child\n'))
        self.assertTrue(marker.exists())
        self.assertEqual(len(observed), 1)

    def test_real_child_sigterm_inside_entry_write_keeps_interrupted(self):
        source = '''import sys,os,signal,json
sys.path.insert(0,"/src/tests")
import test_coolify_rehearsal_engine as f
case=f.PhaseContracts()
case.setUp()
case.owned.close()
case.daemon=f.QuotaDaemon()
case.owned=f.Task4ModelOwned(case.context,adapter=case.daemon)
case.addCleanup(case.owned.close)
prior=signal.getsignal(signal.SIGTERM)
original=f.FakeCgroups._write
def interrupted(self,path,value):
 original(self,path,value)
 if path==self.entry/"memory.max" and value=="268435456":
  os.kill(os.getpid(),signal.SIGTERM)
f.FakeCgroups._write=interrupted
try:
 case.run_engine()
 result={"unexpected_success":True}
except f.d.DockerError as error:
 result={"category":str(error),"clean":error.cleanup_complete,"empty":not case.daemon.containers,"restored":signal.getsignal(signal.SIGTERM)==prior}
finally: case.doCleanups()
print(json.dumps(result))
'''
        code, raw, _ = self.command(source, seconds=10)
        self.assertEqual(code, 0)
        self.assertEqual(json.loads(raw), {'category': 'interrupted', 'clean': True, 'empty': True, 'restored': True})


class FakeCgroups(d.CgroupBudget):
    ROOT = Path('/PUBLIC-FAKE-CGROUP-ROOT')

    def __init__(self):
        super().__init__(self.ROOT / 'delegated', 'a' * 32)
        self.entry = self.parent / 'native-entry'
        self.dirs = {self.ROOT, self.parent, self.entry}
        self.values = {self.parent / 'cgroup.controllers': 'memory cpu pids',
                       self.parent / 'cgroup.subtree_control': 'memory cpu pids',
                       self.parent / 'cgroup.type': 'domain'}
        self.placements = {os.getpid(): self.entry}
        self.values[self.entry / 'cgroup.procs'] = str(os.getpid())
        self.values.update({self.entry / 'memory.max': '1342177280', self.entry / 'memory.swap.max': '0',
                           self.entry / 'cpu.max': '175000 100000', self.entry / 'pids.max': '64'})
        self.values.update({self.parent / 'memory.max': '1342177280', self.parent / 'memory.swap.max': '0',
                           self.parent / 'cpu.max': '175000 100000', self.parent / 'pids.max': '576'})
        for path in (self.parent, self.entry):
            self.values[path / 'memory.events'] = 'oom 0\noom_kill 0'
            self.values[path / 'memory.current'] = '0'
            self.values[path / 'cpu.stat'] = 'nr_periods 0\nnr_throttled 0\nthrottled_usec 0'
        self.processes = {os.getpid(): {'token': d._start_token(os.getpid()), 'ppid': 1, 'pgid': 1, 'sid': 1}}
        self.writes = []

    def _identity(self, path):
        d._require(path in self.dirs, 'resource_isolation')
        return (str(path),)

    def _open_directory(self, path):
        return path

    def _held_identity(self, path):
        return (str(path),)

    def _close_directory(self, fd):
        pass

    def _read(self, path):
        return self.values.get(path, '')

    def _write(self, path, value):
        self.values[path] = value
        self.writes.append((path, value))

    def _mkdir(self, path):
        d._require(path not in self.dirs, 'resource_isolation')
        self.dirs.add(path)
        self.values[path / 'memory.events'] = 'oom 0\noom_kill 0'
        self.values[path / 'memory.current'] = '0'
        self.values[path / 'cpu.stat'] = 'nr_periods 0\nnr_throttled 0\nthrottled_usec 0'

    def _directories(self, path):
        return [p for p in self.dirs if p.parent == path]

    def _rmdir(self, path):
        d._require(not self._directories(path), 'cleanup_incomplete')
        self.dirs.remove(path)

    def _placement(self, pid):
        return self.placements[pid]

    def _process(self, pid):
        if pid not in self.processes:
            if pid not in self.placements:
                raise ProcessLookupError()
            return {'token': str(pid).encode(), 'ppid': 0, 'pgid': pid, 'sid': pid}
        return dict(self.processes[pid])


class QuotaDaemon(PhaseDaemon, d.NativeProcess):
    """Unit-only closed daemon + fake cgroups. Never a NativeProcess registry."""
    def _bind_owner(self, owner):
        self._owner = owner
        self.context = owner.context
        self._run = None
        self._cgroup = None
        self._authorization = None
        self._transport_clean = True

    def _bind_run(self, run):
        self._run = run
        self._cgroup = FakeCgroups()
        self._cgroup.allocate({'driver': 'cgroupfs', 'version': '2'})


class CgroupContracts(unittest.TestCase):
    def test_allocate_first_cause_survives_failed_partial_cleanup(self):
        for failure, category in ((KeyboardInterrupt(), 'interrupted'),
                                  (d.DockerError('command_timeout'), 'command_timeout')):
            group = FakeCgroups()
            with self.subTest(category=category), patch.object(group, '_write', side_effect=failure), patch.object(group, '_remove_empty_run', side_effect=d.DockerError('cleanup_incomplete')):
                with self.assertRaises(d.DockerError) as caught:
                    group.allocate({'driver': 'cgroupfs', 'version': '2'})
                self.assertEqual(str(caught.exception), category)
                self.assertIs(caught.exception.cleanup_complete, False)
                self.assertIn(group.path, group.dirs)

    def test_transition_write_preserves_interrupt_and_typed_error_without_trusting_flag(self):
        for failure, category in ((KeyboardInterrupt(), 'interrupted'),
                                  (d.DockerError('command_timeout'), 'command_timeout')):
            group = FakeCgroups()
            group.allocate({'driver': 'cgroupfs', 'version': '2'})
            failure.cleanup_complete = True
            original = group._write
            def after_effect(path, value):
                original(path, value)
                raise failure
            with self.subTest(category=category), patch.object(group, '_write', side_effect=after_effect):
                with self.assertRaises(d.DockerError) as caught:
                    group.steady()
                self.assertEqual(str(caught.exception), category)
                self.assertIs(caught.exception.cleanup_complete, False)
            # The recognized partial write is retained for guarded cleanup.
            self.assertEqual(group._entry_limits['memory.max'], '268435456')

    def test_owned_physical_parent_mapping_and_caps_transition(self):
        group = FakeCgroups()
        group.allocate({'driver': 'cgroupfs', 'version': '2'})
        self.assertEqual(group.placements[os.getpid()], group.native)
        self.assertEqual(group.docker_parent, '/delegated/sifto-' + 'a' * 32)
        self.assertEqual(group.values[group.native / 'memory.max'], str(1280 * 1024**2))
        group.placements[100] = group.entry
        group.processes[100] = {'token': b'PUBLIC-child', 'ppid': os.getpid(), 'pgid': 100, 'sid': 100}
        group.values[group.entry / 'cgroup.procs'] = str(os.getpid()) + '\n100'
        group.attach(100, b'PUBLIC-child')
        self.assertEqual(group.placements[100], group.native)
        group.values[group.native / 'cgroup.procs'] = str(os.getpid())
        group.placements.pop(100)
        group.processes.pop(100)
        group.steady()
        self.assertEqual(group.values[group.native / 'memory.max'], str(256 * 1024**2))
        self.assertEqual(group.values[group.native / 'cpu.max'], '12500 100000')
        group.preparation()
        self.assertEqual(group.values[group.native / 'memory.max'], str(1280 * 1024**2))
        group.release()
        self.assertEqual(group.dirs, {group.ROOT, group.parent, group.entry})
        self.assertEqual(group.placements[os.getpid()], group.entry)

    def test_systemd_v1_missing_controllers_and_collision_fail_closed(self):
        for info in ({'driver': 'systemd', 'version': '2'}, {'driver': 'cgroupfs', 'version': '1'}, {}):
            group = FakeCgroups()
            with self.assertRaisesRegex(d.DockerError, '^resource_isolation$'):
                group.allocate(info)
            self.assertFalse(group.writes)
        group = FakeCgroups()
        group.values[group.parent / 'cgroup.controllers'] = 'cpu pids'
        with self.assertRaisesRegex(d.DockerError, '^resource_isolation$'):
            group.allocate({'driver': 'cgroupfs', 'version': '2'})
        self.assertFalse(group.writes)

    def test_changed_effective_placement_quota_oom_and_old_children_block_lift(self):
        group = FakeCgroups()
        group.allocate({'driver': 'cgroupfs', 'version': '2'})
        group.steady()
        group._mkdir(group.path / 'PUBLIC-daemon-child')
        with self.assertRaisesRegex(d.DockerError, '^resource_isolation$'):
            group.preparation()
        group.values[group.native / 'memory.max'] = 'max'
        with self.assertRaisesRegex(d.DockerError, '^resource_isolation$'):
            group.guard()

    def test_wrong_starting_placement_and_changed_entry_refuse_adoption(self):
        group = FakeCgroups()
        group.placements[os.getpid()] = group.ROOT / 'PUBLIC-production'
        with self.assertRaisesRegex(d.DockerError, '^resource_isolation$'):
            group.allocate({'driver': 'cgroupfs', 'version': '2'})
        self.assertFalse(group.writes)
        group = FakeCgroups()
        group.allocate({'driver': 'cgroupfs', 'version': '2'})
        group.dirs.remove(group.entry)
        with self.assertRaisesRegex(d.DockerError, '^resource_isolation$'):
            group.release()
        self.assertEqual(group.placements[os.getpid()], group.native)

    def test_wrong_effective_container_placement_oom_and_limits_reject(self):
        for defect in ('outside', 'limit', 'oom'):
            group = FakeCgroups()
            group.allocate({'driver': 'cgroupfs', 'version': '2'})
            group.steady()
            path = group.path / ('b' * 64)
            group._expected_ids = frozenset(('b' * 64,))
            group._mkdir(path)
            group.placements[100] = path if defect != 'outside' else group.ROOT / 'PUBLIC-production'
            group.values.update({path / 'memory.max': '268435456', path / 'memory.swap.max': '0',
                                 path / 'cpu.max': '25000 100000', path / 'pids.max': '128'})
            if defect == 'limit':
                group.values[path / 'memory.max'] = 'max'
            elif defect == 'oom':
                group.values[path / 'memory.events'] = 'oom 1\noom_kill 1'
            value = {'Memory': 268435456, 'NanoCpus': 250000000, 'PidsLimit': 128,
                     'Id': 'b' * 64, 'Labels': {'sifto.operation': 'postgres'}}
            with self.subTest(defect=defect), self.assertRaisesRegex(d.DockerError, '^resource_isolation$'):
                group.verify_container(100, value)

    def test_successful_cpu_throttling_is_observed_without_failure(self):
        group = FakeCgroups()
        group.allocate({'driver': 'cgroupfs', 'version': '2'})
        group.steady()
        path = group.path / ('b' * 64)
        group._expected_ids = frozenset(('b' * 64,))
        group._mkdir(path)
        group.placements[100] = path
        group.values.update({path / 'memory.max': '268435456', path / 'memory.swap.max': '0',
            path / 'cpu.max': '25000 100000', path / 'pids.max': '128',
            path / 'cpu.stat': 'usage_usec 999\nnr_periods 10\nnr_throttled 4\nthrottled_usec 50'})
        group.verify_container(100, {'Memory': 268435456, 'NanoCpus': 250000000, 'PidsLimit': 128,
                                    'Id': 'b' * 64, 'Labels': {'sifto.operation': 'postgres'}})
        self.assertEqual(group._cpu_samples[path]['nr_throttled'], 4)

    def test_changed_entry_limits_refuse_return_and_preserve_owned_run(self):
        group = FakeCgroups()
        group.allocate({'driver': 'cgroupfs', 'version': '2'})
        group.values[group.entry / 'memory.max'] = 'max'
        with self.assertRaisesRegex(d.DockerError, '^resource_isolation$'):
            group.release()
        self.assertIn(group.path, group.dirs)
        self.assertEqual(group.placements[os.getpid()], group.native)

    def test_parent_envelope_covers_retained_entry_and_container_totals_without_migration(self):
        group = FakeCgroups()
        group.allocate({'driver': 'cgroupfs', 'version': '2'})
        self.assertEqual(group.native, group.entry)
        self.assertEqual(group.values[group.path / 'memory.max'], str(1024 * 1024**2))
        self.assertEqual(group.values[group.path / 'cpu.max'], '162500 100000')
        self.assertEqual(group.values[group.path / 'pids.max'], '512')
        group.values[group.entry / 'memory.current'] = str(300 * 1024**2)
        before = list(group.writes)
        with self.assertRaisesRegex(d.DockerError, '^resource_capacity$'):
            group.steady()
        self.assertEqual(group.writes, before)
        self.assertFalse(any(path.name == 'cgroup.procs' for path, _ in group.writes))
        self.assertFalse(any(path.parent == group.parent for path, _ in group.writes))

    def test_extra_sibling_entry_child_and_foreign_pid_fail_before_mutation(self):
        for defect in ('sibling', 'entry_child', 'foreign_pid'):
            group = FakeCgroups()
            if defect == 'sibling':
                group._mkdir(group.parent / 'PUBLIC-foreign')
            elif defect == 'entry_child':
                group._mkdir(group.entry / 'PUBLIC-foreign')
            else:
                group.values[group.entry / 'cgroup.procs'] += '\n123'
            with self.subTest(defect=defect), self.assertRaisesRegex(d.DockerError, '^resource_isolation$'):
                group.preflight_entry()
            self.assertEqual(group.writes, [])

    def test_partial_entry_transition_is_tracked_and_drift_is_never_overwritten(self):
        group = FakeCgroups()
        group.allocate({'driver': 'cgroupfs', 'version': '2'})
        original = group._write
        def after_effect(path, value):
            original(path, value)
            if path == group.entry / 'cpu.max':
                raise OSError('PUBLIC write response lost')
        with patch.object(group, '_write', side_effect=after_effect):
            with self.assertRaisesRegex(d.DockerError, '^resource_isolation$'):
                group.steady()
        group.values[group.entry / 'memory.max'] = 'max'
        before = list(group.writes)
        with self.assertRaisesRegex(d.DockerError, '^resource_isolation$'):
            group.release()
        self.assertEqual(group.writes, before)
        self.assertIn(group.path, group.dirs)

    def test_live_or_unresolved_container_and_helper_forbid_baseline_restore(self):
        for defect in ('intent', 'helper'):
            group = FakeCgroups()
            group.allocate({'driver': 'cgroupfs', 'version': '2'})
            group.steady()
            if defect == 'intent':
                group._expected_ids = frozenset(('b' * 64,))
            else:
                group.values[group.entry / 'cgroup.procs'] += '\n123'
            before = list(group.writes)
            with self.subTest(defect=defect), self.assertRaises(d.DockerError):
                group.release()
            self.assertEqual(group.writes, before)

if __name__ == '__main__':
    unittest.main()
