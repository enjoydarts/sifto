"""PUBLIC dummy ownership fixtures; no Docker, recovery bundle or Task4 artifact."""
import copy
from contextlib import contextmanager
from dataclasses import replace
import fcntl
import hashlib
import inspect
import json
import multiprocessing
import os
from pathlib import Path
import socket
import tempfile
import unittest
from unittest.mock import patch

from maintenance import coolify_docker as d
from maintenance.coolify_plan import fixed_profile, profile_eligibility
from maintenance.coolify_rehearsal import FilePolicy, RootBinding

MARKER = 'PUBLIC-SECRET-MARKER-DO-NOT-EMIT'
ID = 'a' * 64
SOURCES = ('host_agent.py', 'coolify_native_controller.py', 'coolify_native_engine.py',
           'maintenance/coolify_systemd.py', 'maintenance/coolify_native_protocol.py',
           'maintenance/coolify_cleanup.py', 'maintenance/coolify_native_owner.py',
           'systemd/sifto-coolify-native-controller.service',
           'maintenance/__init__.py', 'maintenance/plan.py',
           'maintenance/host.py', 'maintenance/coolify_host.py',
           'maintenance/coolify_plan.py', 'maintenance/coolify_rehearsal.py',
           'maintenance/coolify_evidence.py', 'maintenance/coolify_docker.py',
           'maintenance/coolify_rehearsal_engine.py',
           'maintenance/coolify-public-manifest.json', 'maintenance/coolify_probe.php',
           'maintenance/coolify_startup.php', 'maintenance/coolify_startup.py', 'maintenance/coolify_startup_init',
           'maintenance/coolify_startup_middleware.php', 'maintenance/coolify_startup_nginx.conf',
           'maintenance/coolify_startup_site.conf', 'maintenance/coolify_horizon_inspect.php')


def sha(value):
    return hashlib.sha256(value).hexdigest()


class FakeDaemon:
    """Parse actual fixed argv; mutations act on selected inspect projections."""
    def __init__(self):
        self.calls = []
        self.containers = {}
        self.start_ids = []
        self.image_fault = None
        self.create_fault = None
        self.inspect_fault = None
        self.remove_fault = False
        self.events = []
        self.before_create = None
        self.sequence = 0

    def inspect_image(self, reference):
        self.calls.append(('image', reference))
        value = {'engine_id': 'sha256:' + 'b' * 64, 'os': 'linux', 'architecture': 'arm64',
                 'descriptor': {'digest': reference.split('@')[1],
                                'mediaType': 'application/vnd.oci.image.manifest.v1+json'}}
        if self.image_fault:
            self.image_fault(value)
        return value

    def lookup(self, *, name=None, intent=None, run=None):
        self.calls.append(('lookup', name, intent, run))
        return [cid for cid, v in self.containers.items()
                if (name is None or v['Name'] == name)
                and (intent is None or v['Labels'].get('sifto.intent') == intent)
                and (run is None or v['Labels'].get('sifto.run') == run)]

    def create(self, argv, *, env, pass_fds):
        self.calls.append(('create', tuple(argv), dict(env), tuple(pass_fds)))
        if self.before_create:
            self.before_create()
        self.assert_safe_argv(argv, env)
        options = {}
        index = 1
        while index < len(argv) and argv[index].startswith('--'):
            key, value = argv[index].split('=', 1)
            options.setdefault(key, []).append(value)
            index += 1
        image, command = argv[index], argv[index + 1:]
        one = lambda key: options[key][0]
        mounts = []
        for value in options.get('--mount', []):
            bits = dict(item.split('=', 1) for item in value.split(',') if '=' in item)
            mounts.append({'Type': bits['type'], 'Source': bits['src'],
                           'Destination': bits['dst'], 'RW': 'readonly' not in value.split(','),
                           'Propagation': 'rprivate'})
        self.sequence += 1
        cid = ID if self.sequence == 1 else sha(str(self.sequence).encode())
        projection = {'Id': cid, 'Name': one('--name'),
                      'Image': 'sha256:' + 'b' * 64, 'Reference': image,
                      'Labels': dict(value.split('=', 1) for value in options['--label']),
                      'User': one('--user'), 'Entrypoint': [one('--entrypoint')],
                      'Cmd': list(command), 'WorkingDir': one('--workdir'),
                      'Memory': int(one('--memory')), 'MemorySwap': int(one('--memory-swap')),
                      'NanoCpus': int(one('--cpus').replace('.', '').ljust(10, '0')),
                      'PidsLimit': int(one('--pids-limit')), 'CapDrop': ['ALL'], 'CapAdd': [],
                      'SecurityOpt': ['no-new-privileges'], 'ReadonlyRootfs': True,
                      'NetworkMode': one('--network'), 'PidMode': '', 'IpcMode': 'private',
                      'CgroupnsMode': 'private', 'CgroupParent': one('--cgroup-parent'),
                      'Privileged': False, 'Ports': {}, 'ExtraHosts': [], 'Devices': [],
                      'Mounts': mounts, 'Tmpfs': dict(value.split(':', 1)
                                                   for value in options.get('--tmpfs', [])),
                      'State': {'Status': 'created', 'Running': False,
                                'ExitCode': 0, 'OOMKilled': False}}
        self.containers[cid] = projection
        if self.create_fault == 'late':
            raise TimeoutError(MARKER)
        if self.create_fault == 'ambiguous':
            self.containers['c' * 64] = {**copy.deepcopy(projection), 'Id': 'c' * 64}
            raise TimeoutError(MARKER)
        if self.create_fault == 'bad-id':
            return MARKER
        return cid

    @staticmethod
    def assert_safe_argv(argv, env):
        if argv[0] != 'create' or '--pull=never' not in argv or '--platform=linux/arm64' not in argv:
            raise AssertionError('unsafe creation argv')
        if set(env) != {'PATH', 'HOME', 'LC_ALL'} or env['HOME'] != '/nonexistent':
            raise AssertionError('inherited environment')

    def inspect(self, cid):
        self.calls.append(('inspect', cid))
        value = copy.deepcopy(self.containers.get(cid))
        if self.inspect_fault and value:
            self.inspect_fault(value)
        return value

    def start(self, cid):
        self.calls.append(('start', cid))
        self.start_ids.append(cid)
        self.containers[cid]['State'].update(Status='running', Running=True)

    def stop(self, cid):
        self.calls.append(('stop', cid))
        self.containers[cid]['State'].update(Status='exited', Running=False)

    def remove(self, cid):
        self.calls.append(('rm', cid))
        if self.remove_fault:
            raise RuntimeError(MARKER)
        self.containers.pop(cid, None)


class SyntheticOwned(d.OwnedDocker):
    """Test-local factory bypasses platform only; never a native eligibility API."""
    def _require_native_platform(self):
        pass


class OwnedDockerTests(unittest.TestCase):
    @contextmanager
    def _cleanup_correction_runs(self):
        before = set(self.owned._runs)
        try:
            yield
        finally:
            for run in set(self.owned._runs) - before:
                self.owned.cleanup(run)

    def setUp(self):
        self.temp = tempfile.TemporaryDirectory()
        self.addCleanup(self.temp.cleanup)
        self.root = Path(self.temp.name)
        self.root.chmod(0o700)
        self.source = self.root / 'source'
        self.source.mkdir(mode=0o700)
        repository = Path(__file__).parents[1]
        closure = []
        for name in SOURCES:
            target = self.source / name
            target.parent.mkdir(mode=0o700, exist_ok=True)
            target.write_bytes((repository / name).read_bytes())
            target.chmod(0o644)
            closure.append((target, sha(target.read_bytes())))
        self.stage = self.root / 'stage'
        self.stage.mkdir(mode=0o700)
        self.input = self.root / 'dummy-public-input'
        self.input.write_bytes(b'PUBLIC dummy input, not a backup')
        self.input.chmod(0o600)
        self.sock = socket.socket(socket.AF_UNIX)
        self.sock.bind(str(self.root / 'docker.sock'))
        self.addCleanup(self.sock.close)
        self.context = d.NativeContext(
            RootBinding(sha(b'PUBLIC snapshot'), 'PUBLIC-host',
                        (('coolify.dump', sha(b'PUBLIC dump')),
                         ('coolify-config.tar.gz', sha(b'PUBLIC config')),
                         ('created_at', sha(b'PUBLIC timestamp'))), sha(b'PUBLIC proof')),
            self.input, FilePolicy(self.root, 0), FilePolicy(self.stage, 0),
            self.root / 'docker.sock', Path('/PUBLIC-owned-cgroup'), tuple(closure), None)
        self.daemon = FakeDaemon()
        self.owned = SyntheticOwned(self.context, adapter=self.daemon)
        self.addCleanup(self.owned.close)

    def assert_category(self, category, action):
        with self.assertRaisesRegex(d.DockerError, '^' + category + '$') as raised:
            action()
        self.assertNotIn(MARKER, str(raised.exception))
        self.assertNotIn(MARKER, repr(raised.exception))
        self.assertIsNone(raised.exception.__cause__)

    def populate(self, run, phase, *, names=None):
        # Private synthetic registration belongs solely to this test fixture.
        record = self.owned._runs[run]
        engine_id = 'sha256:' + 'b' * 64
        record['identities'] = {'old': (9999, 9999, engine_id), 'target': (9999, 9999, engine_id),
                                'postgres': (70, 70, engine_id), 'redis': (999, 999, engine_id)}
        material = {'runtime.env': b'PUBLIC curated env\n', 'hmac.key': b'x' * 32,
                    'coolify.dump': b'PUBLIC dump'}
        record['material'][phase] = {name: sha(data) for name, data in material.items()}
        phase_path = self.stage / record['token'] / phase.value
        fd = self.owned.preparation_fd(run, phase)
        try:
            selected = names or ('runtime.env', 'probe.php', 'public-manifest.json', 'hmac.key',
                                 'storage', 'bootstrap-cache', 'runtime-ssh', 'runtime-ssl',
                                 'runtime-proxy', 'pg', 'redis', 'coolify.dump')
            for name in selected:
                if name in ('storage', 'bootstrap-cache', 'runtime-ssh', 'runtime-ssl',
                            'runtime-proxy', 'pg', 'redis'):
                    os.mkdir(name, mode=0o700, dir_fd=fd)
                    owner = (70, 70) if name == 'pg' else ((999, 999) if name == 'redis' else (9999, 9999))
                    os.chown(name, *owner, dir_fd=fd, follow_symlinks=False)
                else:
                    data = material.get(name)
                    if data is None:
                        source = 'coolify_probe.php' if name == 'probe.php' else 'coolify-public-manifest.json'
                        data = (self.source / 'maintenance' / source).read_bytes()
                    leaf = os.open(name, os.O_WRONLY | os.O_CREAT | os.O_EXCL | os.O_NOFOLLOW, 0o600, dir_fd=fd)
                    try:
                        os.write(leaf, data)
                        os.fchown(leaf, 0 if name == 'coolify.dump' else 9999,
                                  0 if name == 'coolify.dump' else 9999)
                        os.fsync(leaf)
                    finally:
                        os.close(leaf)
        finally:
            os.close(fd)
        return phase_path

    def postgres(self):
        run = self.owned.allocate_run()
        path = self.populate(run, d.Phase.OLD)
        resources = self.owned.seal_phase(run, d.Phase.OLD)
        container = self.owned.create(run, operation=d.Operation.POSTGRES,
                                      phase=d.Phase.OLD, resources=resources)
        return run, path, resources, container

    def test_unknown_id_is_never_removed(self):
        self.assert_category('ownership_unknown', lambda: self.owned.remove('f' * 64))
        self.assertFalse(any(c[0] in ('stop', 'rm') for c in self.daemon.calls))

    def test_native_context_is_fail_closed_on_wrong_platform(self):
        with patch('platform.system', return_value='Darwin'):
            self.assert_category('context_policy', lambda: d.OwnedDocker(self.context, adapter=self.daemon))

    def test_context_requires_root_and_the_complete_reviewed_source_closure(self):
        with patch.object(d.os, 'geteuid', return_value=1000):
            self.assert_category('context_policy', lambda: SyntheticOwned(self.context, adapter=self.daemon))
        context = replace(self.context, source_sha256=self.context.source_sha256[:-1])
        self.assert_category('source_closure', lambda: SyntheticOwned(context, adapter=self.daemon))

    def test_sticky_public_input_policy_root_is_not_a_protected_root(self):
        context = replace(self.context, input_policy=FilePolicy(Path('/tmp'), 0))
        self.assert_category('context_policy', lambda: SyntheticOwned(context, adapter=self.daemon))

    def test_context_rejects_unprotected_sources_socket_and_staging(self):
        cases = ((self.source / 'maintenance/coolify_docker.py', 'source_closure'),
                 (self.stage, 'context_policy'), (self.root / 'docker.sock', 'context_policy'))
        for path, category in cases:
            original = path.stat().st_mode & 0o777
            with self.subTest(path=path.name):
                path.chmod(0o777)
                self.assert_category(category, lambda: SyntheticOwned(self.context, adapter=self.daemon))
                path.chmod(original)

    def test_context_rejects_invalid_trusted_binding_shape(self):
        valid = self.context.binding
        for binding in (replace(valid, snapshot_id=MARKER),
                        replace(valid, file_sha256=(('coolify.dump', '1' * 64),)),
                        replace(valid, file_sha256=(('coolify.dump', '1' * 64),) * 3),
                        replace(valid, proof_sha256=MARKER), replace(valid, host_id='../' + MARKER)):
            with self.subTest(binding=binding):
                context = replace(self.context, binding=binding)
                self.assert_category('context_policy', lambda: SyntheticOwned(context, adapter=self.daemon))

    def test_source_hash_change_rejects_before_create(self):
        run = self.owned.allocate_run()
        (self.source / 'maintenance/coolify_docker.py').write_bytes(b'PUBLIC changed source')
        self.assert_category('source_closure', lambda: self.owned.create(
            run, operation=d.Operation.IDENTITY_OLD, phase=d.Phase.RUN, resources=None))
        self.assertFalse(any(c[0] == 'create' for c in self.daemon.calls))

    def test_run_and_phase_allocation_are_exclusive_and_handles_unforgeable(self):
        run = self.owned.allocate_run()
        record = self.owned._runs[run]
        self.assertRegex(record['token'], '^[a-f0-9]{32}$')
        for name in ('', 'old', 'fresh'):
            path = self.stage / record['token'] / name
            self.assertEqual(path.stat().st_mode & 0o777, 0o700)
        self.assert_category('stage_identity', lambda: self.owned.preparation_fd(object(), d.Phase.OLD))
        self.assert_category('stage_identity', lambda: self.owned.preparation_fd(copy.copy(run), d.Phase.OLD))
        with patch.object(d.secrets, 'token_hex', return_value=record['token']):
            self.assert_category('stage_collision', self.owned.allocate_run)
        self.assertEqual((self.stage / record['token'] / '.owner').stat().st_mode & 0o777, 0o600)

    def test_preparation_is_phase_specific_cloexec_and_unavailable_after_seal(self):
        run = self.owned.allocate_run()
        fd = self.owned.preparation_fd(run, d.Phase.OLD)
        self.assertFalse(os.get_inheritable(fd))
        os.close(fd)
        self.assert_category('stage_identity', lambda: self.owned.preparation_fd(run, d.Phase.RUN))
        self.populate(run, d.Phase.OLD)
        self.owned.seal_phase(run, d.Phase.OLD)
        self.assert_category('stage_identity', lambda: self.owned.preparation_fd(run, d.Phase.OLD))
        fd = self.owned.preparation_fd(run, d.Phase.FRESH)
        os.close(fd)

    def test_create_requires_durable_intent_then_exact_id_then_verified_before_start(self):
        sync_events = []
        original = os.fsync
        with patch.object(d.os, 'fsync', side_effect=lambda fd: (sync_events.append(fd), original(fd))[1]):
            run = self.owned.allocate_run()
            self.populate(run, d.Phase.OLD)
            resources = self.owned.seal_phase(run, d.Phase.OLD)
            def before_create():
                record = self.owned._runs[run]
                journal = (self.stage / record['token'] / 'journal').read_text()
                self.assertEqual(json.loads(journal.splitlines()[-1])['state'], 'intent')
                self.assertTrue(sync_events)
            self.daemon.before_create = before_create
            container = self.owned.create(run, operation=d.Operation.POSTGRES, phase=d.Phase.OLD, resources=resources)
        self.assertEqual(self.owned.journal_state(ID), 'verified')
        self.assertEqual(self.daemon.start_ids, [])
        self.owned.start(container.container_id)
        self.assertEqual(self.daemon.start_ids, [ID])
        self.assertEqual(self.owned.journal_state(ID), 'started')

    def test_illegal_operations_reject_before_intent(self):
        run = self.owned.allocate_run()
        before = list(self.daemon.calls)
        for operation, phase in ((MARKER, d.Phase.RUN), (d.Operation.IDENTITY_OLD, d.Phase.OLD),
                                 (d.Operation.POSTGRES, d.Phase.RUN), (d.Operation.VERIFY_TARGET, d.Phase.FRESH),
                                 (d.Operation.NETWORK_TARGET, d.Phase.FRESH), (d.Operation.MIGRATE_TARGET, d.Phase.FRESH)):
            with self.subTest(operation=operation, phase=phase):
                self.assert_category('container_spec', lambda: self.owned.create(
                    run, operation=operation, phase=phase, resources=None))
        self.assertEqual(self.daemon.calls, before)

    def test_task4_operations_reject_before_create_even_if_context_claims_verified(self):
        self.owned.close()
        context = replace(self.context, startup=d.ControlledStartup('1' * 64, MARKER, 'synthetic-task4'))
        self.owned = SyntheticOwned(context, adapter=self.daemon)
        self.addCleanup(self.owned.close)
        run = self.owned.allocate_run()
        self.populate(run, d.Phase.OLD)
        resources = self.owned.seal_phase(run, d.Phase.OLD)
        for operation in (d.Operation.SSH_OLD, d.Operation.SSH_TARGET, d.Operation.HELD_OLD, d.Operation.HELD_TARGET):
            self.assert_category('startup_profile_unverified', lambda: self.owned.create(
                run, operation=operation, phase=d.Phase.OLD, resources=resources))
        self.assertFalse(any(c[0] in ('create', 'start') for c in self.daemon.calls))
        self.assertFalse(profile_eligibility()['eligible'])

    def test_identity_operations_have_no_material_mount_and_fixed_account(self):
        run = self.owned.allocate_run()
        for operation, account in ((d.Operation.IDENTITY_OLD, 'www-data'), (d.Operation.IDENTITY_TARGET, 'www-data'),
                                   (d.Operation.IDENTITY_POSTGRES, 'postgres'), (d.Operation.IDENTITY_REDIS, 'redis')):
            container = self.owned.create(run, operation=operation, phase=d.Phase.RUN, resources=None)
            observed = self.daemon.containers[container.container_id]
            self.assertEqual(observed['Entrypoint'], ['/usr/bin/id'])
            self.assertEqual(observed['Cmd'], [account])
            self.assertEqual(observed['Mounts'], [])
            self.assertEqual(observed['NetworkMode'], 'none')
            self.assertEqual(observed['Memory'], 128 * 1024 * 1024)
            self.assertEqual(observed['MemorySwap'], observed['Memory'])
            self.owned.remove(container.container_id)

    def test_all_legal_phase_operations_use_fixed_image_command_mounts_and_namespace(self):
        bundle = fixed_profile()['bundle']
        for phase in (d.Phase.OLD, d.Phase.FRESH):
            run = self.owned.allocate_run()
            path = self.populate(run, phase)
            resources = self.owned.seal_phase(run, phase)
            pg = self.owned.create(run, operation=d.Operation.POSTGRES, phase=phase, resources=resources)
            self.owned.start(pg.container_id)
            rows = [(d.Operation.REDIS, 'redis', '/usr/local/bin/redis-server', None),
                    (d.Operation.BASELINE_OLD, 'coolify', 'php', 'baseline'),
                    (d.Operation.NETWORK_OLD, 'coolify', 'php', 'network'),
                    (d.Operation.VERIFY_OLD, 'coolify', 'php', 'verify'),
                    (d.Operation.MIGRATE_OLD, 'coolify', 'php', 'migrate')]
            if phase is d.Phase.OLD:
                rows += [(d.Operation.NETWORK_TARGET, 'coolify', 'php', 'network'),
                         (d.Operation.VERIFY_TARGET, 'coolify', 'php', 'verify'),
                         (d.Operation.MIGRATE_TARGET, 'coolify', 'php', 'migrate')]
            for operation, image_role, executable, purpose in rows:
                with self.subTest(phase=phase, operation=operation):
                    container = self.owned.create(run, operation=operation, phase=phase, resources=resources)
                    observed = self.daemon.containers[container.container_id]
                    target = operation.value.endswith('_target')
                    image = bundle['target' if target else 'rollback']['images'][image_role]
                    self.assertEqual(observed['Reference'], image['repository'] + '@' + image['platform_manifest'])
                    self.assertEqual(observed['Entrypoint'], [executable])
                    self.assertEqual(observed['NetworkMode'], 'container:' + pg.container_id)
                    self.assertEqual(observed['ReadonlyRootfs'], True)
                    self.assertEqual(observed['CapDrop'], ['ALL'])
                    self.assertEqual(observed['Ports'], {})
                    self.assertTrue(all(m['Source'].startswith(str(path) + '/') for m in observed['Mounts']))
                    self.assertFalse(any(m['Source'].endswith(('/.owner', '/journal', '/coolify.dump'))
                                         for m in observed['Mounts']))
                    if purpose in ('baseline', 'network', 'verify'):
                        self.assertEqual(observed['Cmd'], ['/probe/probe.php', purpose])
                        self.assertEqual(observed['User'], '9999:9999')
                        for mount in observed['Mounts']:
                            if mount['Destination'].startswith('/probe/') or mount['Destination'].endswith('/.env'):
                                self.assertFalse(mount['RW'])
                    elif purpose == 'migrate':
                        self.assertEqual(observed['Cmd'], ['artisan', 'migrate', '--force', '--isolated'])
                        self.assertEqual(observed['WorkingDir'], '/var/www/html')
                    self.owned.remove(container.container_id)
            self.assertTrue(self.owned.cleanup(run))

    def test_dependent_operations_require_a_started_verified_same_phase_postgres(self):
        run = self.owned.allocate_run()
        self.populate(run, d.Phase.OLD)
        resources = self.owned.seal_phase(run, d.Phase.OLD)
        self.assert_category('network_isolation', lambda: self.owned.create(
            run, operation=d.Operation.REDIS, phase=d.Phase.OLD, resources=resources))
        pg = self.owned.create(run, operation=d.Operation.POSTGRES, phase=d.Phase.OLD, resources=resources)
        self.assert_category('network_isolation', lambda: self.owned.create(
            run, operation=d.Operation.BASELINE_OLD, phase=d.Phase.OLD, resources=resources))
        self.owned.start(pg.container_id)
        self.daemon.containers[pg.container_id]['State']['Running'] = False
        self.assert_category('network_isolation', lambda: self.owned.create(
            run, operation=d.Operation.REDIS, phase=d.Phase.OLD, resources=resources))

    def test_plain_handle_construction_does_not_grant_ownership(self):
        for kind in (d.OwnedRun, d.PhaseResources):
            with self.subTest(kind=kind):
                self.assert_category('stage_identity', kind)

    def test_phase_marker_and_open_descriptor_are_revalidated(self):
        run = self.owned.allocate_run()
        path = self.populate(run, d.Phase.OLD)
        resources = self.owned.seal_phase(run, d.Phase.OLD)
        (path / '.owner').write_text(MARKER)
        self.assert_category('stage_identity', lambda: self.owned.create(
            run, operation=d.Operation.POSTGRES, phase=d.Phase.OLD, resources=resources))
        self.assertFalse(any(c[0] == 'create' for c in self.daemon.calls))

    def test_migrate_target_is_once_only_after_creation_even_when_removed(self):
        run, _, resources, pg = self.postgres()
        self.owned.start(pg.container_id)
        migration = self.owned.create(run, operation=d.Operation.MIGRATE_TARGET,
                                      phase=d.Phase.OLD, resources=resources)
        self.owned.remove(migration.container_id)
        self.assert_category('container_spec', lambda: self.owned.create(
            run, operation=d.Operation.MIGRATE_TARGET, phase=d.Phase.OLD, resources=resources))

    def test_material_registration_is_required_not_inferred_from_leaf_hashes(self):
        run = self.owned.allocate_run()
        self.populate(run, d.Phase.OLD)
        self.owned._runs[run]['material'].clear()
        self.assert_category('material_binding', lambda: self.owned.seal_phase(run, d.Phase.OLD))

    def test_owned_id_already_absent_is_idempotent_only_after_ownership_proof(self):
        _, _, _, container = self.postgres()
        self.owned.remove(container.container_id)
        self.owned.remove(container.container_id)
        self.assertEqual(sum(c[0] == 'rm' for c in self.daemon.calls), 1)
        self.assert_category('ownership_unknown', lambda: self.owned.remove('d' * 64))

    def test_closed_public_api_has_no_caller_argv_image_mount_path_or_role(self):
        parameters = inspect.signature(d.OwnedDocker.create).parameters
        self.assertEqual(tuple(parameters), ('self', 'run', 'operation', 'phase', 'resources'))
        self.assertEqual(tuple(inspect.signature(d.OwnedDocker.preparation_fd).parameters),
                         ('self', 'run', 'phase'))
        self.assertEqual(tuple(inspect.signature(d.OwnedDocker.seal_phase).parameters),
                         ('self', 'run', 'phase'))
        self.assertFalse(any(hasattr(d.OwnedDocker, name)
                             for name in ('exec', 'run_command', 'shell', 'restore', 'publish')))

    def test_arbitrary_error_text_is_not_a_public_category(self):
        error = d.DockerError(MARKER)
        self.assertEqual(str(error), 'command_failed')
        self.assertNotIn(MARKER, repr(error))

    def test_phase_env_is_passed_by_only_authenticated_fd_and_is_readonly_mount(self):
        run, _, resources, pg = self.postgres()
        self.owned.start(pg.container_id)
        def before_create():
            call = self.daemon.calls[-1]
            self.assertEqual(call[0], 'create')
            self.assertEqual(len(call[3]), 1)
            fd = call[3][0]
            self.assertIn('--env-file=/proc/self/fd/' + str(fd), call[1])
            self.assertEqual(os.fstat(fd).st_uid, 9999)
            self.assertEqual(os.fstat(fd).st_mode & 0o777, 0o600)
            self.assertEqual(os.read(fd, 1024), b'PUBLIC curated env\n')
            self.assertNotIn('PUBLIC curated env', repr(call[1:3]))
        self.daemon.before_create = before_create
        self.owned.create(run, operation=d.Operation.BASELINE_OLD, phase=d.Phase.OLD, resources=resources)

    def test_no_resource_handle_is_accepted_for_identity_operation(self):
        run = self.owned.allocate_run()
        self.populate(run, d.Phase.OLD)
        resources = self.owned.seal_phase(run, d.Phase.OLD)
        self.assert_category('container_spec', lambda: self.owned.create(
            run, operation=d.Operation.IDENTITY_OLD, phase=d.Phase.RUN, resources=resources))

    def test_input_file_and_socket_replacement_invalidate_existing_run(self):
        run = self.owned.allocate_run()
        self.input.rename(self.root / 'PUBLIC-displaced-input')
        self.input.write_bytes(b'PUBLIC replacement')
        self.input.chmod(0o600)
        self.assert_category('context_policy', lambda: self.owned.create(
            run, operation=d.Operation.IDENTITY_OLD, phase=d.Phase.RUN, resources=None))
        self.assertFalse(any(c[0] == 'create' for c in self.daemon.calls))

    def test_wrong_image_identity_registration_cannot_authorize_pg_leaf(self):
        run = self.owned.allocate_run()
        self.populate(run, d.Phase.OLD)
        self.owned._runs[run]['identities']['postgres'] = (70, 70, 'sha256:' + 'c' * 64)
        resources = self.owned.seal_phase(run, d.Phase.OLD)
        self.assert_category('image_identity', lambda: self.owned.create(
            run, operation=d.Operation.POSTGRES, phase=d.Phase.OLD, resources=resources))

    def test_exact_id_inspection_is_required_and_bad_return_id_is_not_authority(self):
        self.daemon.create_fault = 'bad-id'
        run = self.owned.allocate_run()
        self.assert_category('container_identity', lambda: self.owned.create(
            run, operation=d.Operation.IDENTITY_OLD, phase=d.Phase.RUN, resources=None))
        self.assertEqual(self.daemon.start_ids, [])
        # An actual returned invalid ID cannot be passed to stop/rm; reconciliation
        # may authenticate the unique real ID by the already-durable intent.
        self.assertTrue(self.owned.cleanup(run))
        self.assertFalse(any(c[1] == MARKER for c in self.daemon.calls if c[0] in ('start', 'stop', 'rm')))

    def test_failed_create_cannot_be_resumed_by_start(self):
        self.daemon.create_fault = 'late'
        run = self.owned.allocate_run()
        self.assert_category('command_timeout', lambda: self.owned.create(
            run, operation=d.Operation.IDENTITY_OLD, phase=d.Phase.RUN, resources=None))
        self.assert_category('interrupted', lambda: self.owned.start(ID))
        self.assertTrue(self.owned.cleanup(run))

    def test_phase_immutable_probe_hash_and_environment_binding_are_checked(self):
        for name in ('probe.php', 'public-manifest.json', 'runtime.env', 'coolify.dump'):
            with self.subTest(name=name):
                run = self.owned.allocate_run()
                path = self.populate(run, d.Phase.OLD)
                (path / name).write_bytes(b'PUBLIC changed bytes')
                self.assert_category('material_binding', lambda: self.owned.seal_phase(run, d.Phase.OLD))

    def test_journal_and_marker_metadata_must_remain_private_and_single_link(self):
        run, _, _, container = self.postgres()
        journal = self.stage / self.owned._runs[run]['token'] / 'journal'
        journal.chmod(0o644)
        self.assert_category('journal_invalid', lambda: self.owned.start(container.container_id))
        self.assertFalse(self.daemon.start_ids)

    def test_replaced_socket_and_closed_phase_fd_invalidate_capabilities(self):
        run = self.owned.allocate_run()
        fd = self.owned._runs[run]['phases'][d.Phase.OLD]['fd']
        os.close(fd)
        self.assert_category('stage_identity', lambda: self.owned.preparation_fd(run, d.Phase.OLD))
        self.owned.close()
        self.owned = SyntheticOwned(self.context, adapter=self.daemon)
        self.addCleanup(self.owned.close)
        run = self.owned.allocate_run()
        (self.root / 'docker.sock').unlink()
        replacement = socket.socket(socket.AF_UNIX)
        self.addCleanup(replacement.close)
        replacement.bind(str(self.root / 'docker.sock'))
        self.assert_category('context_policy', lambda: self.owned.create(
            run, operation=d.Operation.IDENTITY_OLD, phase=d.Phase.RUN, resources=None))

    def test_child_create_security_failure_never_reaches_start(self):
        run = self.owned.allocate_run()
        self.daemon.inspect_fault = lambda v: v.update(Privileged=True)
        self.assert_category('container_spec', lambda: self.owned.create(
            run, operation=d.Operation.IDENTITY_OLD, phase=d.Phase.RUN, resources=None))
        self.assertEqual(self.daemon.start_ids, [])
        self.assertFalse(self.owned.cleanup(run))

    def test_identity_declared_image_volumes_are_fixed_tmpfs_not_anonymous_volumes(self):
        run = self.owned.allocate_run()
        container = self.owned.create(run, operation=d.Operation.IDENTITY_POSTGRES, phase=d.Phase.RUN, resources=None)
        self.assertIn('/var/lib/postgresql/data', self.daemon.containers[container.container_id]['Tmpfs'])
        self.assertIn('/data', self.daemon.containers[container.container_id]['Tmpfs'])

    def test_old_and_fresh_phases_and_identity_work_cannot_overlap(self):
        run, _, _, _ = self.postgres()
        self.populate(run, d.Phase.FRESH)
        fresh = self.owned.seal_phase(run, d.Phase.FRESH)
        self.assert_category('resource_isolation', lambda: self.owned.create(
            run, operation=d.Operation.POSTGRES, phase=d.Phase.FRESH, resources=fresh))
        self.assert_category('resource_isolation', lambda: self.owned.create(
            run, operation=d.Operation.IDENTITY_OLD, phase=d.Phase.RUN, resources=None))

    def test_simultaneous_probe_operations_are_not_a_quota_bypass(self):
        run, _, resources, pg = self.postgres()
        self.owned.start(pg.container_id)
        self.owned.create(run, operation=d.Operation.BASELINE_OLD, phase=d.Phase.OLD, resources=resources)
        self.assert_category('resource_isolation', lambda: self.owned.create(
            run, operation=d.Operation.NETWORK_OLD, phase=d.Phase.OLD, resources=resources))

    def test_sealed_material_and_identity_registrations_cannot_be_replaced(self):
        run, _, resources, pg = self.postgres()
        self.owned.start(pg.container_id)
        self.owned._runs[run]['material'][d.Phase.OLD]['runtime.env'] = sha(b'PUBLIC replacement')
        self.assert_category('material_binding', lambda: self.owned.create(
            run, operation=d.Operation.BASELINE_OLD, phase=d.Phase.OLD, resources=resources))

    def test_cleanup_lookup_failure_is_incomplete_without_raw_exception(self):
        run, _, _, _ = self.postgres()
        with patch.object(self.daemon, 'lookup', side_effect=RuntimeError(MARKER)):
            self.assertFalse(self.owned.cleanup(run))

    def test_fsync_failure_prevents_daemon_creation(self):
        run = self.owned.allocate_run()
        with patch.object(d.os, 'fsync', side_effect=OSError(MARKER)):
            self.assert_category('journal_invalid', lambda: self.owned.create(
                run, operation=d.Operation.IDENTITY_OLD, phase=d.Phase.RUN, resources=None))
        self.assertFalse(any(c[0] == 'create' for c in self.daemon.calls))

    def test_exact_mounts_are_inspected_without_relying_on_engine_list_order(self):
        run, _, resources, pg = self.postgres()
        self.owned.start(pg.container_id)
        self.daemon.inspect_fault = lambda v: v['Mounts'].reverse()
        probe = self.owned.create(run, operation=d.Operation.BASELINE_OLD, phase=d.Phase.OLD, resources=resources)
        self.owned.start(probe.container_id)

    def test_reused_foreign_fd_is_not_closed_by_owned_close(self):
        run = self.owned.allocate_run()
        old_fd = self.owned._runs[run]['phases'][d.Phase.OLD]['fd']
        os.close(old_fd)
        foreign_fd = os.open(self.input, os.O_RDONLY)
        self.assertEqual(foreign_fd, old_fd)
        try:
            self.owned.close()
            os.fstat(foreign_fd)
        finally:
            try:
                os.close(foreign_fd)
            except OSError:
                pass

    def test_removed_intent_token_cannot_be_reused_to_overwrite_history(self):
        run = self.owned.allocate_run()
        container = self.owned.create(run, operation=d.Operation.IDENTITY_OLD, phase=d.Phase.RUN, resources=None)
        self.owned.remove(container.container_id)
        before = dict(self.owned._runs[run]['intents'])
        with patch.object(d.secrets, 'token_hex', return_value=container.intent_id):
            self.assert_category('stage_collision', lambda: self.owned.create(
                run, operation=d.Operation.IDENTITY_TARGET, phase=d.Phase.RUN, resources=None))
        self.assertEqual(self.owned._runs[run]['intents'], before)

    def test_missing_descriptor_wrong_platform_and_changed_image_identity_reject(self):
        run = self.owned.allocate_run()
        for category, defect in (('image_descriptor', lambda v: v.pop('descriptor')),
                                 ('image_identity', lambda v: v.update(architecture='amd64')),
                                 ('image_descriptor', lambda v: v['descriptor'].update(digest='sha256:' + 'c' * 64)),
                                 ('image_descriptor', lambda v: v['descriptor'].pop('mediaType')),
                                 ('image_descriptor', lambda v: v['descriptor'].update(
                                     mediaType='application/vnd.oci.image.index.v1+json'))):
            with self.subTest(category=category):
                self.daemon.image_fault = defect
                self.assert_category(category, lambda: self.owned.create(
                    run, operation=d.Operation.IDENTITY_OLD, phase=d.Phase.RUN, resources=None))
        self.assertFalse(any(c[0] == 'create' for c in self.daemon.calls))

    def test_phase_handles_missing_foreign_and_wrong_phase_are_rejected(self):
        run, _, resources, container = self.postgres()
        foreign = self.owned.allocate_run()
        for selected_run, phase, selected in ((run, d.Phase.OLD, None), (foreign, d.Phase.OLD, resources),
                                               (run, d.Phase.FRESH, resources), (run, d.Phase.OLD, copy.copy(resources))):
            self.assert_category('stage_identity', lambda: self.owned.create(
                selected_run, operation=d.Operation.POSTGRES, phase=phase, resources=selected))
        self.owned.remove(container.container_id)

    def test_incomplete_resource_subset_cannot_create_probe(self):
        run = self.owned.allocate_run()
        self.populate(run, d.Phase.OLD, names=('pg',))
        resources = self.owned.seal_phase(run, d.Phase.OLD)
        self.owned.create(run, operation=d.Operation.POSTGRES, phase=d.Phase.OLD, resources=resources)
        self.assert_category('material_binding', lambda: self.owned.create(
            run, operation=d.Operation.BASELINE_OLD, phase=d.Phase.OLD, resources=resources))

    def test_immutable_leaf_bytes_path_metadata_and_parent_replacement_reject_start(self):
        run, path, resources, container = self.postgres()
        leaf = path / 'hmac.key'
        original = leaf.read_bytes()
        leaf.write_bytes(b'z' * 32)
        # POSTGRES selects only its PG leaf: unrelated app material does not leak into mounts.
        self.owned.start(container.container_id)
        self.assert_category('material_binding', lambda: self.owned.create(
            run, operation=d.Operation.BASELINE_OLD, phase=d.Phase.OLD, resources=resources))
        leaf.write_bytes(original)
        pg = path / 'pg'
        pg.rename(path / 'pg-displaced')
        pg.mkdir(mode=0o700)
        os.chown(pg, 70, 70)
        self.assert_category('stage_identity', lambda: self.owned.start(container.container_id))

    def test_writable_directory_contents_may_change_but_identity_must_stay(self):
        _, path, _, container = self.postgres()
        (path / 'pg' / 'PUBLIC-owned-db-change').write_bytes(b'PUBLIC')
        self.owned.start(container.container_id)
        self.assertEqual(self.daemon.start_ids, [ID])

    def test_seal_rejects_symlink_hardlink_wrong_owner_mode_and_hmac_size(self):
        for defect in ('symlink', 'hardlink', 'owner', 'mode', 'size'):
            with self.subTest(defect=defect):
                run = self.owned.allocate_run()
                path = self.populate(run, d.Phase.OLD)
                leaf = path / 'hmac.key'
                if defect == 'symlink':
                    leaf.unlink(); leaf.symlink_to(path / 'runtime.env')
                elif defect == 'hardlink':
                    os.link(leaf, path / 'PUBLIC-link')
                elif defect == 'owner':
                    os.chown(leaf, 0, 0)
                elif defect == 'mode':
                    leaf.chmod(0o644)
                else:
                    leaf.write_bytes(b'x' * 31)
                self.assert_category('hmac_identity', lambda: self.owned.seal_phase(run, d.Phase.OLD))

    def test_drifted_container_projection_never_starts_or_removes_stranger(self):
        _, _, _, container = self.postgres()
        for field, value in (('Image', MARKER), ('User', '0'), ('NetworkMode', 'host'),
                             ('MemorySwap', 0), ('CapAdd', ['SYS_ADMIN']), ('Ports', {'80/tcp': MARKER}),
                             ('Mounts', [{'Destination': '/var/run/docker.sock', 'Source': MARKER}])):
            with self.subTest(field=field):
                self.daemon.inspect_fault = lambda v: v.update({field: value})
                self.assert_category('container_spec', lambda: self.owned.start(container.container_id))
                self.assert_category('container_spec', lambda: self.owned.remove(container.container_id))
        self.assertEqual(self.daemon.start_ids, [])
        self.assertFalse(any(c[0] in ('stop', 'rm') for c in self.daemon.calls))

    def test_late_create_reconciles_exact_intent_without_starting(self):
        self.daemon.create_fault = 'late'
        run = self.owned.allocate_run()
        self.assert_category('command_timeout', lambda: self.owned.create(
            run, operation=d.Operation.IDENTITY_OLD, phase=d.Phase.RUN, resources=None))
        self.assertEqual(self.owned.journal_state(ID), 'verified')
        self.assertEqual(self.daemon.start_ids, [])
        self.assertTrue(self.owned.cleanup(run))
        self.assertEqual(self.daemon.containers, {})

    def test_ambiguous_late_create_is_unowned_and_never_cleaned(self):
        self.daemon.create_fault = 'ambiguous'
        run = self.owned.allocate_run()
        self.assert_category('command_timeout', lambda: self.owned.create(
            run, operation=d.Operation.IDENTITY_OLD, phase=d.Phase.RUN, resources=None))
        self.assertFalse(self.owned.cleanup(run))
        self.assertEqual(len(self.daemon.containers), 2)
        self.assertFalse(any(c[0] in ('start', 'stop', 'rm') for c in self.daemon.calls))

    def test_native_create_failure_keeps_first_cause_and_unresolved_intent(self):
        for observation in ('absent', 'ambiguous'):
            for category in ('command_timeout', 'interrupted', 'output_bound'):
                with self.subTest(observation=observation, category=category):
                    daemon = FakeDaemon()
                    owned = SyntheticOwned(self.context, adapter=daemon)
                    self.addCleanup(owned.close)
                    run = owned.allocate_run()
                    original = daemon.create

                    def failed_create(*args, **kwargs):
                        cid = original(*args, **kwargs)
                        if observation == 'absent':
                            daemon.containers.pop(cid)
                        else:
                            daemon.containers['c' * 64] = {
                                **copy.deepcopy(daemon.containers[cid]), 'Id': 'c' * 64}
                        error = d.DockerError(category)
                        error.cleanup_complete = True  # Incoming claims grant no cleanup authority.
                        raise error

                    with patch.object(daemon, 'create', side_effect=failed_create):
                        with self.assertRaises(d.DockerError) as caught:
                            owned.create(run, operation=d.Operation.IDENTITY_OLD,
                                         phase=d.Phase.RUN, resources=None)
                    self.assertEqual(str(caught.exception), category)
                    self.assertFalse(caught.exception.cleanup_complete)
                    intent, = owned._runs[run]['intents'].values()
                    self.assertIsNone(intent['id'])
                    self.assertEqual(intent['state'], 'intent')
                    self.assertEqual(owned._containers, {})
                    strangers = copy.deepcopy(daemon.containers)
                    self.assertFalse(owned.cleanup(run))
                    self.assertEqual(daemon.containers, strangers)
                    self.assertFalse(any(c[0] in ('start', 'stop', 'rm') for c in daemon.calls))

    def test_failed_create_cleanup_removes_only_previously_verified_id(self):
        run, _, resources, container = self.postgres()
        self.owned.start(container.container_id)
        self.daemon.create_fault = 'ambiguous'
        with self.assertRaises(d.DockerError) as caught:
            self.owned.create(run, operation=d.Operation.REDIS,
                              phase=d.Phase.OLD, resources=resources)
        self.assertEqual(str(caught.exception), 'command_timeout')
        self.assertFalse(caught.exception.cleanup_complete)
        strangers = set(self.daemon.containers) - {container.container_id}
        self.assertEqual(len(strangers), 2)
        self.assertFalse(self.owned.cleanup(run))
        self.assertEqual(set(self.daemon.containers), strangers)
        self.assertEqual({c[1] for c in self.daemon.calls if c[0] in ('stop', 'rm')},
                         {container.container_id})

    def test_planted_same_label_container_is_preserved_and_blocks_clean_success(self):
        run, _, _, container = self.postgres()
        planted = copy.deepcopy(self.daemon.containers[ID])
        planted.update(Id='f' * 64, Name=MARKER)
        self.daemon.containers['f' * 64] = planted
        self.assertFalse(self.owned.cleanup(run))
        self.assertNotIn(ID, self.daemon.containers)
        self.assertIn('f' * 64, self.daemon.containers)
        self.assertEqual(self.owned.journal_state(container.container_id), 'removed')

    def test_native_unresolved_intent_authorizes_inspection_only(self):
        for observation in ('absent', 'ambiguous'):
            with self.subTest(observation=observation):
                owned = SyntheticOwned(self.context, adapter=self.daemon)
                self.addCleanup(owned.close)
                run = owned.allocate_run()
                if observation == 'absent':
                    failed = patch.object(self.daemon, 'create',
                                          side_effect=d.DockerError('command_timeout'))
                else:
                    self.daemon.create_fault = 'ambiguous'
                    failed = patch.object(self.daemon, 'create', wraps=self.daemon.create)
                with failed:
                    self.assert_category('command_timeout', lambda: owned.create(
                        run, operation=d.Operation.IDENTITY_OLD, phase=d.Phase.RUN, resources=None))
                native = d.NativeProcess(replace(self.context, budget=d.BudgetPolicy()))
                # Unit-only wiring; real native owner binding refuses SyntheticOwned.
                native._owner, native._run = owned, run
                foreign = 'f' * 64
                with patch.object(native, '_invoke', return_value=b'0') as invoke:
                    for operation in ('start', 'stop', 'remove', 'wait_result'):
                        with self.subTest(operation=operation):
                            action = getattr(native, operation)
                            self.assert_category('ownership_unknown', lambda: action(
                                foreign, **({'seconds': 30} if operation == 'wait_result' else {})))
                    invoke.assert_not_called()
                if observation == 'absent':
                    response = (1, b'', ('Error: No such container: ' + foreign).encode())
                    expected = None
                else:
                    expected = copy.deepcopy(self.daemon.containers['c' * 64])
                    response = (0, json.dumps(expected).encode(), b'')
                    foreign = 'c' * 64
                with patch.object(native, '_invoke_status', return_value=response) as inspect_call:
                    self.assertEqual(native.inspect(foreign), expected)
                    self.assertEqual(inspect_call.call_args.args[0][-1], foreign)
                self.daemon.create_fault = None

    def test_native_mutations_and_results_require_registered_bound_run_id(self):
        run, _, _, container = self.postgres()
        native = d.NativeProcess(replace(self.context, budget=d.BudgetPolicy()))
        native._owner, native._run = self.owned, run
        def invoke(argv, **kwargs):
            return b'PUBLIC result' if argv[:2] == ['container', 'logs'] else b'0'
        with patch.object(native, '_invoke', side_effect=invoke) as calls:
            native.start(container.container_id)
            self.assertEqual(native.wait_result(container.container_id, seconds=30), b'PUBLIC result')
            native.stop(container.container_id)
            native.remove(container.container_id)
            self.assertEqual([c.args[0][1] for c in calls.call_args_list],
                             ['start', 'wait', 'logs', 'stop', 'rm'])
            self.assertTrue(all(c.args[0][-1] == container.container_id for c in calls.call_args_list))
        native._run = self.owned.allocate_run()
        with patch.object(native, '_invoke') as calls:
            for operation in ('start', 'stop', 'remove', 'wait_result'):
                with self.subTest(operation=operation):
                    self.assert_category('ownership_unknown', lambda: getattr(native, operation)(
                        container.container_id, **({'seconds': 30} if operation == 'wait_result' else {})))
            calls.assert_not_called()

    def test_cleanup_failure_sanitizes_exception_and_retains_journal(self):
        run, _, _, _ = self.postgres()
        self.daemon.remove_fault = True
        self.assertFalse(self.owned.cleanup(run))
        journal = (self.stage / self.owned._runs[run]['token'] / 'journal').read_text()
        self.assertNotIn(MARKER, journal)
        self.assertNotIn('PUBLIC curated env', journal)
        self.assertIn(ID, journal)

    def test_inherited_environment_is_never_forwarded(self):
        run = self.owned.allocate_run()
        with patch.dict(os.environ, {'DOCKER_HOST': MARKER, 'DOCKER_CONTEXT': MARKER,
                                     'DOCKER_CONFIG': MARKER, 'PATH': MARKER, 'APP_KEY': MARKER}):
            self.owned.create(run, operation=d.Operation.IDENTITY_OLD, phase=d.Phase.RUN, resources=None)
        create = next(c for c in self.daemon.calls if c[0] == 'create')
        self.assertNotIn(MARKER, repr(create))
        self.assertEqual(create[3], ())

    def test_closed_handles_and_journal_drift_are_rejected(self):
        run, _, _, container = self.postgres()
        journal = self.stage / self.owned._runs[run]['token'] / 'journal'
        journal.write_text(MARKER)
        self.assert_category('journal_invalid', lambda: self.owned.start(container.container_id))
        self.owned.close()
        self.assert_category('stage_identity', lambda: self.owned.preparation_fd(run, d.Phase.OLD))

    def test_new_module_is_in_protected_closure_without_runtime_import(self):
        root = Path(__file__).parents[1]
        entrypoint = (root / 'host_agent.py').read_text()
        self.assertIn('maintenance/coolify_docker.py', entrypoint.split('try:\n    validate_installed_sources()')[0])
        self.assertNotIn('from maintenance.coolify_docker', entrypoint)
        self.assertNotIn('import coolify_docker', (root / 'maintenance/host.py').read_text())
        for name in ('bootstrap/install-host.sh', 'bootstrap/Dockerfile.test', 'bootstrap/test-install.sh'):
            self.assertIn('coolify_docker.py', (root / name).read_text())


    def test_baseline_minimal_resources_do_not_require_or_mount_task4_ssh(self):
        run = self.owned.allocate_run()
        self.populate(run, d.Phase.OLD, names=('pg', 'runtime.env', 'probe.php',
                      'public-manifest.json', 'hmac.key', 'storage', 'bootstrap-cache'))
        resources = self.owned.seal_phase(run, d.Phase.OLD)
        pg = self.owned.create(run, operation=d.Operation.POSTGRES,
                               phase=d.Phase.OLD, resources=resources)
        self.owned.start(pg.container_id)
        probe = self.owned.create(run, operation=d.Operation.BASELINE_OLD,
                                  phase=d.Phase.OLD, resources=resources)
        self.assertEqual({m['Destination'] for m in self.daemon.containers[probe.container_id]['Mounts']},
                         {'/var/www/html/.env', '/probe/probe.php', '/probe/public-manifest.json',
                          '/probe/hmac.key', '/var/www/html/storage', '/var/www/html/bootstrap/cache'})

    def test_network_needs_only_public_probe_and_owns_no_environment_fd(self):
        for operation in (d.Operation.NETWORK_OLD, d.Operation.NETWORK_TARGET):
            with self.subTest(operation=operation), self._cleanup_correction_runs():
                run = self.owned.allocate_run()
                self.populate(run, d.Phase.OLD, names=('pg', 'probe.php'))
                resources = self.owned.seal_phase(run, d.Phase.OLD)
                pg = self.owned.create(run, operation=d.Operation.POSTGRES,
                                       phase=d.Phase.OLD, resources=resources)
                self.owned.start(pg.container_id)
                probe = self.owned.create(run, operation=operation, phase=d.Phase.OLD, resources=resources)
                observed = self.daemon.containers[probe.container_id]
                self.assertEqual(observed['Mounts'], [{'Type': 'bind',
                    'Source': str(self.stage / self.owned._runs[run]['token'] / 'old' / 'probe.php'),
                    'Destination': '/probe/probe.php', 'RW': False, 'Propagation': 'rprivate'}])
                call = next(c for c in reversed(self.daemon.calls) if c[0] == 'create')
                self.assertEqual(call[3], ())
                self.assertFalse(any(a.startswith('--env-file=') for a in call[1]))
                self.assertTrue(self.owned.cleanup(run))

    def test_verify_ssh_is_readonly_and_ssl_proxy_copies_are_not_mounted(self):
        run, _, resources, pg = self.postgres()
        self.owned.start(pg.container_id)
        probe = self.owned.create(run, operation=d.Operation.VERIFY_OLD,
                                  phase=d.Phase.OLD, resources=resources)
        mounts = {m['Destination']: m for m in self.daemon.containers[probe.container_id]['Mounts']}
        self.assertFalse(mounts['/var/www/html/storage/app/ssh']['RW'])
        self.assertNotIn('/data/coolify/ssl', mounts)
        self.assertNotIn('/data/coolify/proxy', mounts)

    def test_migration_needs_only_environment_storage_and_bootstrap_cache(self):
        for operation in (d.Operation.MIGRATE_OLD, d.Operation.MIGRATE_TARGET):
            with self.subTest(operation=operation), self._cleanup_correction_runs():
                run = self.owned.allocate_run()
                self.populate(run, d.Phase.OLD, names=('pg', 'runtime.env', 'storage', 'bootstrap-cache'))
                resources = self.owned.seal_phase(run, d.Phase.OLD)
                pg = self.owned.create(run, operation=d.Operation.POSTGRES,
                                       phase=d.Phase.OLD, resources=resources)
                self.owned.start(pg.container_id)
                migration = self.owned.create(run, operation=operation,
                                              phase=d.Phase.OLD, resources=resources)
                self.assertEqual({m['Destination'] for m in self.daemon.containers[migration.container_id]['Mounts']},
                                 {'/var/www/html/.env', '/var/www/html/storage', '/var/www/html/bootstrap/cache'})
                self.assertTrue(self.owned.cleanup(run))

    def test_start_attempt_is_durable_before_daemon_start(self):
        run = self.owned.allocate_run()
        container = self.owned.create(run, operation=d.Operation.IDENTITY_OLD,
                                      phase=d.Phase.RUN, resources=None)
        original = self.daemon.start
        observed = []
        journal_fd = self.owned._runs[run]['journal']['fd']
        original_sync = os.fsync
        def sync(fd):
            original_sync(fd)
            if fd == journal_fd:
                latest = json.loads(os.pread(fd, 1024 * 1024, 0).splitlines()[-1])
                observed.append(('synced', latest['state'], latest['container_id']))
        def start(cid):
            path = self.stage / self.owned._runs[run]['token'] / 'journal'
            latest = json.loads(path.read_text().splitlines()[-1])
            observed.append(('daemon', latest['state'], latest['container_id']))
            original(cid)
        with patch.object(d.os, 'fsync', side_effect=sync), patch.object(self.daemon, 'start', side_effect=start):
            self.owned.start(container.container_id)
        self.assertEqual(observed, [('synced', 'started', container.container_id),
                                    ('daemon', 'started', container.container_id)])

    def test_start_effect_then_failure_consumes_attempt_and_latches_run(self):
        for failure, category in ((TimeoutError(MARKER), 'command_timeout'),
                                  (KeyboardInterrupt(MARKER), 'interrupted'),
                                  (RuntimeError(MARKER), 'command_failed')):
            with self.subTest(category=category), self._cleanup_correction_runs():
                run = self.owned.allocate_run()
                container = self.owned.create(run, operation=d.Operation.IDENTITY_OLD,
                                              phase=d.Phase.RUN, resources=None)
                original = self.daemon.start
                def start(cid):
                    original(cid)
                    raise failure
                with patch.object(self.daemon, 'start', side_effect=start):
                    self.assert_category(category, lambda: self.owned.start(container.container_id))
                self.assertEqual(self.owned.journal_state(container.container_id), 'started')
                before = list(self.daemon.start_ids)
                self.assert_category('interrupted', lambda: self.owned.start(container.container_id))
                self.assert_category('interrupted', lambda: self.owned.create(
                    run, operation=d.Operation.IDENTITY_TARGET, phase=d.Phase.RUN, resources=None))
                self.assertEqual(self.daemon.start_ids, before)
                self.assertTrue(self.owned.cleanup(run))

    def test_post_start_inspection_failure_cannot_replay_daemon_start(self):
        run = self.owned.allocate_run()
        container = self.owned.create(run, operation=d.Operation.IDENTITY_OLD,
                                      phase=d.Phase.RUN, resources=None)
        original = self.daemon.start
        def start(cid):
            original(cid)
            self.daemon.inspect_fault = lambda value: value.update(User=MARKER)
        with patch.object(self.daemon, 'start', side_effect=start):
            self.assert_category('container_spec', lambda: self.owned.start(container.container_id))
        self.daemon.inspect_fault = None
        self.assert_category('interrupted', lambda: self.owned.start(container.container_id))
        self.assertEqual(self.daemon.start_ids, [container.container_id])
        self.assertTrue(self.owned.cleanup(run))

    def test_immediate_successful_exit_is_valid_for_every_one_shot_purpose(self):
        run = self.owned.allocate_run()
        identity = self.owned.create(run, operation=d.Operation.IDENTITY_OLD,
                                     phase=d.Phase.RUN, resources=None)
        original = self.daemon.start
        def finish(cid):
            original(cid)
            self.daemon.containers[cid]['State'].update(Status='exited', Running=False, ExitCode=0)
        with patch.object(self.daemon, 'start', side_effect=finish):
            self.owned.start(identity.container_id)
        self.assertEqual(self.owned.journal_state(identity.container_id), 'started')
        self.owned.remove(identity.container_id)
        self.populate(run, d.Phase.OLD)
        resources = self.owned.seal_phase(run, d.Phase.OLD)
        pg = self.owned.create(run, operation=d.Operation.POSTGRES, phase=d.Phase.OLD, resources=resources)
        self.owned.start(pg.container_id)
        for operation in (d.Operation.BASELINE_OLD, d.Operation.NETWORK_OLD, d.Operation.NETWORK_TARGET,
                          d.Operation.VERIFY_OLD, d.Operation.VERIFY_TARGET,
                          d.Operation.MIGRATE_OLD, d.Operation.MIGRATE_TARGET):
            with self.subTest(operation=operation):
                container = self.owned.create(run, operation=operation, phase=d.Phase.OLD, resources=resources)
                with patch.object(self.daemon, 'start', side_effect=finish):
                    self.owned.start(container.container_id)
                self.assertEqual(self.owned.journal_state(container.container_id), 'started')
                self.owned.remove(container.container_id)

    def test_services_exiting_immediately_are_failure_and_cannot_restart(self):
        for operation in (d.Operation.POSTGRES, d.Operation.REDIS):
            with self.subTest(operation=operation), self._cleanup_correction_runs():
                run, _, resources, pg = self.postgres()
                if operation is d.Operation.REDIS:
                    self.owned.start(pg.container_id)
                    container = self.owned.create(run, operation=operation, phase=d.Phase.OLD, resources=resources)
                else:
                    container = pg
                original = self.daemon.start
                def finish(cid):
                    original(cid)
                    self.daemon.containers[cid]['State'].update(Status='exited', Running=False, ExitCode=0)
                with patch.object(self.daemon, 'start', side_effect=finish):
                    self.assert_category('startup_state', lambda: self.owned.start(container.container_id))
                before = list(self.daemon.start_ids)
                self.assert_category('interrupted', lambda: self.owned.start(container.container_id))
                self.assertEqual(self.daemon.start_ids, before)
                self.assertTrue(self.owned.cleanup(run))

    def test_one_shot_nonzero_exit_is_failure_and_cannot_restart(self):
        run = self.owned.allocate_run()
        container = self.owned.create(run, operation=d.Operation.IDENTITY_OLD, phase=d.Phase.RUN, resources=None)
        original = self.daemon.start
        def finish(cid):
            original(cid)
            self.daemon.containers[cid]['State'].update(Status='exited', Running=False, ExitCode=2)
        with patch.object(self.daemon, 'start', side_effect=finish):
            self.assert_category('startup_state', lambda: self.owned.start(container.container_id))
        self.assert_category('interrupted', lambda: self.owned.start(container.container_id))
        self.assertEqual(self.daemon.start_ids, [container.container_id])
        self.assertTrue(self.owned.cleanup(run))


    def _assert_bounded_context_rejection(self, action):
        context = multiprocessing.get_context('fork')
        reader, writer = context.Pipe(duplex=False)
        def attempt():
            reader.close()
            try:
                action()
                result = 'unexpected_success'
            except d.DockerError as error:
                result = str(error)
            except BaseException:
                result = 'unexpected_error'
            try:
                writer.send(result)
            finally:
                writer.close()
        process = context.Process(target=attempt)
        process.start()
        writer.close()
        try:
            self.assertTrue(reader.poll(2), 'FIFO input validation blocked instead of rejecting')
            self.assertEqual(reader.recv(), 'context_policy')
        finally:
            if process.is_alive():
                process.kill()
            process.join(2)
            reader.close()
            self.assertFalse(process.is_alive(), 'owned FIFO test child was not reaped')
            process.close()

    def test_fifo_input_constructor_rejects_without_blocking(self):
        self.input.unlink()
        os.mkfifo(self.input, mode=0o600)
        self._assert_bounded_context_rejection(
            lambda: SyntheticOwned(self.context, adapter=FakeDaemon()))

    def test_guard_reopen_fifo_race_rejects_without_blocking(self):
        original = d._protected_open
        def raced_open(path, flags, *, category):
            if path == self.input:
                # Replace after the held-descriptor identity check and just before
                # the actual pathname reopen. No OS metadata/open is mocked.
                self.input.rename(self.root / 'PUBLIC-displaced-regular-input')
                os.mkfifo(self.input, mode=0o600)
            return original(path, flags, category=category)
        def action():
            with patch.object(d, '_protected_open', side_effect=raced_open):
                self.owned.allocate_run()
        self._assert_bounded_context_rejection(action)

    def test_regular_input_and_guard_reopen_keep_nonblocking_fd_and_bytes(self):
        anchor = next(value for value in self.owned._anchors if value[0] == self.input)
        self.assertTrue(fcntl.fcntl(anchor[1], fcntl.F_GETFL) & os.O_NONBLOCK)
        self.assertEqual(os.pread(anchor[1], 128, 0), b'PUBLIC dummy input, not a backup')
        original = d._protected_open
        observed = []
        def checked_open(path, flags, *, category):
            fd, parents = original(path, flags, category=category)
            if path == self.input:
                observed.append(bool(fcntl.fcntl(fd, fcntl.F_GETFL) & os.O_NONBLOCK))
                self.assertEqual(os.pread(fd, 128, 0), b'PUBLIC dummy input, not a backup')
            return fd, parents
        with patch.object(d, '_protected_open', side_effect=checked_open):
            run = self.owned.allocate_run()
        self.assertTrue(observed)
        self.assertTrue(all(observed))
        self.assertTrue(self.owned.cleanup(run))


if __name__ == '__main__':
    unittest.main()
