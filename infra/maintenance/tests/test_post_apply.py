import copy
import hashlib
import tempfile
import unittest
from pathlib import Path
from unittest.mock import patch

from maintenance import plan as p
from test_plan import inventory
from test_container_identity import raw_identity
from maintenance import normalize_recreation_identity

try:
    from maintenance import post_apply as post
except ImportError:
    post = None

OWNER = {'id': 19749005, 'login': 'enjoydarts', 'type': 'User'}
BOT = {'id': 41898282, 'login': 'github-actions[bot]', 'type': 'Bot'}
SOURCE = 'b' * 40
EXECUTOR = 'a' * 40


def fixture():
    before = inventory()
    before['runtime']['services'] = {'docker.service': 'active'}
    before['runtime']['containers'] = [{'id': 'c' * 64, 'image_id': 'sha256:' + 'd' * 64,
        'image': 'fixture:1', 'status': 'running', 'health': 'healthy',
        'recreation_identity': normalize_recreation_identity(raw_identity(), 'c' * 64)}]
    plan = p.build_plan(before, EXECUTOR, now=1000)
    approval = {'plan_sha256': p.plan_sha256(plan), 'run_id': '123', 'approved_by': 'enjoydarts',
        'approved_at': 1001, 'expires_at': 2000, 'workflow_sha': EXECUTOR, 'execution_mode': 'apply'}
    envelope = {'plan': plan, 'approval': approval, 'signature': 'fixture-root-already-verified'}
    after = copy.deepcopy(before)
    after.update(collected_at=3000, dpkg_state={'ready': True, 'pending': []},
                 maintenance_policy={'host_role': 'application', 'database_enabled': False, 'backup_ready': False})
    after['packages'] = [{'name': x['name'], 'version': x['target']} for x in plan['packages']]
    history = {'host_id': plan['host_id'], 'receipt_history_complete': True, 'unresolved_execution_count': 0,
        'receipt_total': 1, 'receipt_truncated': 0, 'receipts': [{'plan': plan, 'approval': approval,
            'status': 'verified', 'started_at': 1002, 'completed_at': 1003, 'verified_at': 1004,
            'before_runtime': before['runtime']}]}
    host = {'host_id': plan['host_id'], 'role': 'application', 'address': '192.0.2.1', 'port': 22}
    run = {'id': 123, 'run_attempt': 1, 'workflow_id': 381232664, 'path': '.github/workflows/maintenance-apply.yml',
           'event': 'workflow_dispatch', 'head_branch': 'main', 'head_sha': SOURCE,
           'repository': {'id': 1164459340, 'full_name': 'enjoydarts/sifto'},
           'head_repository': {'id': 1164459340, 'full_name': 'enjoydarts/sifto'}, 'actor': BOT, 'triggering_actor': BOT}
    jobs = [{'id': i, 'name': name, 'run_id': 123, 'run_attempt': 1, 'head_sha': SOURCE,
             'status': 'completed', 'conclusion': 'success'} for i, name in enumerate(('prepare', 'approve', 'execute'))]
    reviews = [{'state': 'approved', 'user': OWNER, 'environments': [{'name': 'production-maintenance'}]}]
    return plan, envelope, host, after, history, run, jobs, reviews


class PostApplyTests(unittest.TestCase):
    def setUp(self):
        self.assertIsNotNone(post, 'same-run read-only verifier is missing')
        self.plan, self.envelope, self.host, self.after, self.history, self.run, self.jobs, self.reviews = fixture()

    def observe(self):
        return post.validate_observation(self.plan, self.envelope, self.host, self.after, self.history)

    def authority(self):
        return post.validate_authority(self.run, self.jobs, self.reviews, '123', '1', SOURCE)

    def test_completed_historical_apply_with_unrelated_pending_updates(self):
        self.after['simulation'] = 'Inst new-package (1 Ubuntu [arm64])\n'
        result = self.observe()
        self.assertEqual(result['status'], 'verified')
        self.assertEqual(result['target_count'], len(self.plan['packages']))
        self.assertEqual(result['pending_update_count'], 1)
        self.assertEqual(self.authority(), 'enjoydarts')
        self.run['actor'] = self.run['triggering_actor'] = OWNER
        self.authority()

    def test_refuses_missing_duplicate_wrong_or_nonterminal_receipts(self):
        for change in ('missing', 'duplicate', 'wrong_plan', 'wrong_approval', 'failed', 'started', 'completed', 'history', 'unresolved', 'truncated', 'count'):
            with self.subTest(change=change):
                saved = copy.deepcopy(self.history)
                receipt = self.history['receipts'][0]
                if change == 'missing': self.history['receipts'] = []; self.history['receipt_total'] = 0
                elif change == 'duplicate': self.history['receipts'].append(copy.deepcopy(receipt)); self.history['receipt_total'] = 2
                elif change == 'wrong_plan': receipt['plan'] = {**self.plan, 'workflow_sha': 'e' * 40}
                elif change == 'wrong_approval': receipt['approval'] = {**self.envelope['approval'], 'run_id': '999'}
                elif change in ('failed', 'started', 'completed'): receipt['status'] = change
                elif change == 'history': self.history['receipt_history_complete'] = False
                elif change == 'unresolved': self.history['unresolved_execution_count'] = 1
                elif change == 'truncated': self.history['receipt_truncated'] = 1
                else: self.history['receipt_total'] = True
                with self.assertRaises(p.MaintenanceError): self.observe()
                self.history = saved

    def test_complete_root_history_can_have_older_receipt_bodies_truncated(self):
        self.history.update(receipt_truncated=30, receipt_total=31)
        self.observe()
        self.history['receipts'].append({'plan': {'plan_id': 'older'}, 'status': 'failed', 'operator_diagnosed_at': 900})
        self.history['receipt_total'] = 32
        self.observe()

    def test_refuses_target_runtime_container_service_role_and_backup_drift(self):
        for change in ('target', 'runtime', 'boot', 'container_id', 'image', 'identity', 'health', 'stopped', 'service', 'role', 'dpkg', 'host'):
            with self.subTest(change=change):
                saved = copy.deepcopy(self.after)
                container = self.after['runtime']['containers'][0]
                if change == 'target': self.after['packages'][0]['version'] = '0'
                elif change == 'runtime': self.after['runtime']['docker_version'] = '0'
                elif change == 'boot': self.after['runtime']['boot_id'] = 'other'
                elif change == 'container_id': container['id'] = 'e' * 64
                elif change == 'image': container['image_id'] = 'sha256:' + 'e' * 64
                elif change == 'identity': container['recreation_identity']['name'] = '/other'
                elif change == 'health': container['health'] = 'unhealthy'
                elif change == 'stopped': container['status'] = 'exited'
                elif change == 'service': self.after['runtime']['services'] = {}
                elif change == 'role': self.after['maintenance_policy']['host_role'] = 'database'
                elif change == 'dpkg': self.after['dpkg_state']['ready'] = False
                else: self.after['host_id'] = 'wrong'
                with self.assertRaises(p.MaintenanceError): self.observe()
                self.after = saved

    def test_database_requires_backup_enable_pg_pgbouncer_and_read_only_route(self):
        self.host['role'] = self.after['maintenance_policy']['host_role'] = 'database'
        self.after['maintenance_policy'].update(database_enabled=True, backup_ready=True)
        self.after['runtime']['services'].update({'postgresql@16-main.service': 'active', 'pgbouncer.service': 'active'})
        self.after['runtime']['database_pool_health'] = {'status': 'healthy', 'reason': 'read_only_route_verified'}
        self.observe()
        for section, key, value in [('maintenance_policy', 'backup_ready', False), ('maintenance_policy', 'database_enabled', False),
                                    ('runtime', 'database_pool_health', {'status': 'healthy', 'reason': 'wrong'})]:
            saved = copy.deepcopy(self.after)
            self.after[section][key] = value
            with self.assertRaises(p.MaintenanceError): self.observe()
            self.after = saved
        for service in ('postgresql@16-main.service', 'pgbouncer.service'):
            value = self.after['runtime']['services'].pop(service)
            with self.assertRaises(p.MaintenanceError): self.observe()
            self.after['runtime']['services'][service] = value

    def test_receipt_times_are_ordered_and_were_within_signed_periods(self):
        for key, value in [('started_at', 1000), ('started_at', True), ('completed_at', 1001), ('started_at', 2000)]:
            saved = copy.deepcopy(self.history)
            self.history['receipts'][0][key] = value
            with self.assertRaises(p.MaintenanceError): self.observe()
            self.history = saved

    def test_valid_started_transaction_may_finish_after_authorization_expiry(self):
        self.history['receipts'][0].update(completed_at=2100, verified_at=2200)
        self.after['collected_at'] = 3000
        self.observe()
        self.history['receipts'][0]['started_at'] = 2000
        with self.assertRaises(p.MaintenanceError): self.observe()

    def test_legitimate_signed_reboot_requires_changed_boot_and_expected_kernel(self):
        before = inventory()
        before['runtime']['latest_installed_kernel'] = '6.8.0-60-generic'
        self.plan = p.build_plan(before, EXECUTOR, now=1000, action='reboot', allow_reboot=True)
        self.envelope['plan'] = self.plan
        self.envelope['approval']['plan_sha256'] = p.plan_sha256(self.plan)
        self.history['receipts'][0].update(plan=self.plan, approval=self.envelope['approval'])
        del self.history['receipts'][0]['completed_at']
        self.after['runtime'].update(boot_id='boot-b', running_kernel='6.8.0-60-generic')
        self.observe()
        raw = p.canonical_json(self.envelope)
        inputs = {'plan_id': self.plan['plan_id'], 'plan_sha256': p.plan_sha256(self.plan), 'executor_sha': EXECUTOR,
                  'dry_run': False, 'approval_sha256': hashlib.sha256(raw).hexdigest()}
        post.validate_documents(p.canonical_json(self.plan), raw, inputs, EXECUTOR, '123', 'enjoydarts')
        self.after['runtime']['boot_id'] = 'boot-a'
        with self.assertRaises(p.MaintenanceError): self.observe()

    def test_actual_run_jobs_and_owner_review_are_required(self):
        for obj, key, value in [(self.run, 'head_sha', EXECUTOR), (self.run, 'run_attempt', 2),
            (self.run, 'path', '.github/workflows/other.yml'), (self.run, 'workflow_id', 1),
            (self.run, 'head_repository', {'full_name': 'other/sifto', 'id': 1164459340}),
            (self.run, 'actor', {**BOT, 'id': 1}), (self.jobs[0], 'conclusion', 'failure'),
            (self.jobs[1], 'head_sha', EXECUTOR), (self.jobs[2], 'run_attempt', 2),
            (self.reviews[0], 'state', 'rejected'), (self.reviews[0], 'user', {**OWNER, 'type': 'Bot'})]:
            saved = copy.deepcopy(obj[key]); obj[key] = value
            with self.assertRaises(p.MaintenanceError): self.authority()
            obj[key] = saved
        for jobs in (self.jobs[:-1], self.jobs + [self.jobs[0]]):
            with self.assertRaises(p.MaintenanceError): post.validate_authority(self.run, jobs, self.reviews, '123', '1', SOURCE)

    def test_immutable_plan_and_exact_envelope_bytes_and_dry_run(self):
        raw = p.canonical_json(self.envelope)
        inputs = {'plan_id': self.plan['plan_id'], 'plan_sha256': p.plan_sha256(self.plan), 'executor_sha': EXECUTOR,
                  'dry_run': False, 'approval_sha256': hashlib.sha256(raw).hexdigest()}
        post.validate_documents(p.canonical_json(self.plan), raw, inputs, EXECUTOR, '123', 'enjoydarts')
        for key, value in [('dry_run', True), ('executor_sha', SOURCE), ('plan_sha256', '0' * 64), ('approval_sha256', '0' * 64)]:
            saved = inputs[key]; inputs[key] = value
            with self.assertRaises(p.MaintenanceError): post.validate_documents(p.canonical_json(self.plan), raw, inputs, EXECUTOR, '123', 'enjoydarts')
            inputs[key] = saved
        with self.assertRaises(p.MaintenanceError): post.validate_documents(p.canonical_json(self.plan), raw + b'\n', inputs, EXECUTOR, '123', 'enjoydarts')


class AppendTests(unittest.TestCase):
    def setUp(self): self.assertIsNotNone(post, 'same-run append is missing')

    def test_retry_only_new_parent_and_reconcile_same_record(self):
        class Store:
            def __init__(self): self.parents = iter(['a' * 40, 'b' * 40]); self.writes = []
            def head(self): return next(self.parents)
            def git(self, *args): return b''
            def write(self, parent, files):
                self.writes.append(parent)
                if len(self.writes) == 1: raise p.MaintenanceError('race')
                return 'c' * 40
        store = Store()
        self.assertEqual(post.append_record(store, {'post-verifications/123-1/observation.json': b'{}'}), 'c' * 40)
        self.assertEqual(len(store.writes), 2)
        store = Store(); store.parents = iter(['a' * 40, 'a' * 40])
        with self.assertRaises(p.MaintenanceError): post.append_record(store, {'post-verifications/123-1/observation.json': b'{}'})
        self.assertEqual(len(store.writes), 1)

    def test_existing_semantic_record_acknowledged_but_conflict_refused(self):
        class Store:
            def head(self): return 'a' * 40
            def git(self, *args): return b'post-verifications/123-1/observation.json\n'
            def read_at(self, parent, name): return b'{"status":"verified","observed_at":100}'
            def write(self, *args): raise AssertionError('existing record overwritten')
        files = {'post-verifications/123-1/observation.json': b'{"status":"verified","observed_at":200}'}
        self.assertEqual(post.append_record(Store(), files), 'a' * 40)
        files[next(iter(files))] = b'{"status":"failed","observed_at":200}'
        with self.assertRaises(p.MaintenanceError): post.append_record(Store(), files)

    def test_unknown_final_write_reconciles_but_never_attempts_a_fourth_write(self):
        class Store:
            def __init__(self): self.reads = 0; self.writes = 0
            def head(self): self.reads += 1; return str(self.reads) * 40
            def git(self, *args): return b''
            def write(self, *args): self.writes += 1; raise p.MaintenanceError('unknown')
        store = Store()
        with self.assertRaises(p.MaintenanceError): post.append_record(store, {'post-verifications/123-1/observation.json': b'{}'})
        self.assertEqual((store.reads, store.writes), (4, 3))

    def test_private_transport_bytes_are_reconciled_without_json_parsing(self):
        files = {'post-verifications/123-1/observation.json': b'{"status":"verified"}',
                 'post-verifications/123-1/diagnostics/fixture/stderr.bin': b'private stderr\x00'}
        class Store:
            def head(self): return 'a' * 40
            def git(self, *args): return ('\n'.join(files) + '\n').encode()
            def read_at(self, parent, name): return files[name]
            def write(self, *args): raise AssertionError('must acknowledge existing exact record')
        self.assertEqual(post.append_record(Store(), files), 'a' * 40)


class SourceAndScopeTests(unittest.TestCase):
    def setUp(self): self.assertIsNotNone(post, 'pipeline source verification missing')

    def test_actual_registered_workflow_must_be_active_and_match(self):
        self.assertTrue(hasattr(post, 'validate_workflow'), 'registered workflow reader missing')
        workflow = {'id': 381232664, 'path': '.github/workflows/maintenance-apply.yml', 'state': 'active'}
        post.validate_workflow(workflow)
        for key, value in [('id', 1), ('path', '.github/workflows/other.yml'), ('state', 'disabled_manually')]:
            saved = workflow[key]; workflow[key] = value
            with self.assertRaises(p.MaintenanceError): post.validate_workflow(workflow)
            workflow[key] = saved

    def test_distinct_pipeline_source_must_match_image_bytes_and_context(self):
        with tempfile.TemporaryDirectory() as directory:
            image = Path(directory)
            names = ['requirements.txt', 'maintenance/post_apply.py', 'tests/test.py', 'playbooks/scan.yml']
            lines = []
            for name in names:
                path = image / name; path.parent.mkdir(parents=True, exist_ok=True); path.write_bytes(name.encode())
                raw = path.read_bytes()
                digest = hashlib.sha1(b'blob ' + str(len(raw)).encode() + b'\0' + raw).hexdigest()
                lines.append('100644 blob ' + digest + '\tinfra/maintenance/' + name)
            manifest = '\n'.join(lines).encode()
            env = {'GITHUB_REF': 'refs/heads/main', 'GITHUB_REPOSITORY': 'enjoydarts/sifto', 'GITHUB_SHA': SOURCE,
                   'MAINTENANCE_CHECKED_OUT_SHA': SOURCE, 'MAINTENANCE_PIPELINE_SHA': SOURCE, 'MAINTENANCE_EXECUTOR_SHA': EXECUTOR}
            post.validate_source(manifest, env, image)
            for key in ('GITHUB_SHA', 'MAINTENANCE_CHECKED_OUT_SHA', 'MAINTENANCE_PIPELINE_SHA', 'GITHUB_REPOSITORY'):
                saved = env[key]; env[key] = EXECUTOR
                with self.assertRaises(p.MaintenanceError): post.validate_source(manifest, env, image)
                env[key] = saved
            (image / 'maintenance/post_apply.py').write_bytes(b'untrusted image')
            with self.assertRaises(p.MaintenanceError): post.validate_source(manifest, env, image)

    def test_reader_only_calls_selected_host_scan_and_report_and_never_dispatches(self):
        plan, envelope, host, after, history, run, jobs, reviews = fixture()
        jobs += [{**jobs[0], 'id': 10, 'name': 'audit', 'conclusion': 'failure'},
                 {**jobs[0], 'id': 11, 'name': 'post-verification', 'status': 'in_progress', 'conclusion': None}]
        raw = p.canonical_json(envelope)
        inputs = {'private_commit': 'c' * 40, 'plan_id': plan['plan_id'], 'plan_sha256': p.plan_sha256(plan),
                  'executor_sha': EXECUTOR, 'approval_commit': 'd' * 40, 'approval_sha256': hashlib.sha256(raw).hexdigest(), 'dry_run': False}
        calls = []
        class Store:
            def __init__(self, *args, **kwargs): pass
            def fetch(self, commit, path, **kwargs): return raw if path.startswith('approvals/') else p.canonical_json(plan)
        class Client:
            def __init__(self, *args): self.role = args[-1]; assert self.role == 'scan'
            def call(self, selected, mode):
                calls.append((selected, mode))
                assert selected == host and mode in ('scan', 'report')
                return after if mode == 'scan' else history
        paths = {'/secrets/inputs.json': p.canonical_json(inputs), '/secrets/source.txt': b'fixture',
                 '/secrets/hosts.json': p.canonical_json([host, {**host, 'host_id': 'other'}])}
        def get(path):
            self.assertNotIn('dispatch', path)
            if '/actions/workflows/' in path:
                return {'id': 381232664, 'path': '.github/workflows/maintenance-apply.yml', 'state': 'active'}
            return reviews if path.endswith('/approvals') else run
        env = {'GITHUB_SHA': SOURCE, 'MAINTENANCE_EXECUTOR_SHA': EXECUTOR, 'GITHUB_RUN_ID': '123', 'GITHUB_RUN_ATTEMPT': '1',
               'MAINTENANCE_OPS_REPO': 'enjoydarts/private-evidence'}
        with patch.dict('os.environ', env, clear=True), patch.object(post, 'validate_source'), patch.object(post, 'github', get), \
             patch.object(post, 'read_jobs', return_value=jobs), patch.object(post, 'GitStore', Store), patch.object(post, 'Client', Client), \
             patch.object(Path, 'read_bytes', lambda path: paths[str(path)]), \
             patch.object(Path, 'iterdir', return_value=[Path('/secrets', name) for name in ('source.txt', 'host_key', 'ops_key', 'hosts.json', 'known_hosts', 'inputs.json')]), \
             patch.object(post, 'append_record', return_value='e' * 40) as append, patch.object(post, 'private_write'):
            post.run()
            for key, value in [('head_sha', EXECUTOR), ('head_repository', {'id': 1, 'full_name': 'fork/sifto'})]:
                saved = run[key]; run[key] = value
                with self.assertRaises(p.MaintenanceError): post.run()
                self.assertEqual(len(calls), 2, 'invalid actual source/fork must fail before host access')
                run[key] = saved
        self.assertEqual([mode for _, mode in calls], ['scan', 'report'])
        saved = append.call_args.args[1]
        self.assertIn('post-verifications/123-1/root-receipts.json', saved)
        self.assertEqual(set(saved), {f'post-verifications/123-1/{name}.json' for name in ('references', 'inventory', 'root-receipts', 'observation')})

    def test_run_reconciles_ack_lost_append_including_real_collected_transport_bytes(self):
        plan, envelope, host, after, history, run, jobs, reviews = fixture()
        envelope_raw = p.canonical_json(envelope)
        inputs = {'private_commit': 'c' * 40, 'plan_id': plan['plan_id'], 'plan_sha256': p.plan_sha256(plan),
                  'executor_sha': EXECUTOR, 'approval_commit': 'd' * 40,
                  'approval_sha256': hashlib.sha256(envelope_raw).hexdigest(), 'dry_run': False}
        class Store:
            def __init__(self): self.files = {}; self.writes = 0; self.parent = 'e' * 40
            def fetch(self, commit, path, **kwargs): return envelope_raw if path.startswith('approvals/') else p.canonical_json(plan)
            def head(self): return self.parent
            def git(self, *args): return ('\n'.join(self.files) + ('\n' if self.files else '')).encode()
            def read_at(self, parent, name): return self.files[name] or None
            def write(self, parent, files):
                self.files = files; self.writes += 1; self.parent = 'f' * 40
                raise p.MaintenanceError('ack lost after persisted append')
        store = Store()
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            for name in ('secrets', 'private'): (root / name).mkdir()
            for name in ('source.txt', 'host_key', 'ops_key', 'known_hosts'): (root / 'secrets' / name).write_bytes(b'fixture')
            (root / 'secrets/inputs.json').write_bytes(p.canonical_json(inputs))
            (root / 'secrets/hosts.json').write_bytes(p.canonical_json([host]))
            class Client:
                def __init__(self, *args): assert args[-1] == 'scan'
                def call(self, selected, mode):
                    assert selected == host and mode in ('scan', 'report')
                    location = root / 'private/transport' / mode; location.mkdir(parents=True)
                    (location / 'stdout.bin').write_bytes(b'\xffprivate\x00')
                    (location / 'stderr.bin').write_bytes(b'')
                    return after if mode == 'scan' else history
            def local_path(value):
                return root / value.lstrip('/') if value in ('/secrets', '/private') else Path(value)
            def get(path):
                if '/actions/workflows/' in path: return {'id': 381232664, 'path': '.github/workflows/maintenance-apply.yml', 'state': 'active'}
                return reviews if path.endswith('/approvals') else run
            env = {'GITHUB_SHA': SOURCE, 'MAINTENANCE_EXECUTOR_SHA': EXECUTOR, 'GITHUB_RUN_ID': '123',
                   'GITHUB_RUN_ATTEMPT': '1', 'MAINTENANCE_OPS_REPO': 'enjoydarts/private-evidence'}
            with patch.dict('os.environ', env, clear=True), patch.object(post, 'Path', local_path), \
                 patch.object(post, 'validate_source'), patch.object(post, 'github', get), patch.object(post, 'read_jobs', return_value=jobs), \
                 patch.object(post, 'GitStore', return_value=store), patch.object(post, 'Client', Client):
                post.run()
            self.assertEqual(store.writes, 1)
            self.assertEqual(store.files['post-verifications/123-1/diagnostics/scan/stderr.bin'], b'')
            self.assertEqual(store.files['post-verifications/123-1/diagnostics/report/stdout.bin'], b'\xffprivate\x00')
            summary = (root / 'private/summary.md').read_text()
            self.assertIn('post-verification: verified', summary)
            self.assertNotIn('private\x00', summary)


class PaginationTests(unittest.TestCase):
    def setUp(self): self.assertIsNotNone(post, 'bounded GitHub jobs reader is missing')
    def test_jobs_pagination_is_complete_and_bounded(self):
        calls = []
        def get(path):
            calls.append(path)
            return {'total_count': 101, 'jobs': [{'id': i} for i in (range(100) if '&page=1' in path else [100])]}
        self.assertEqual(len(post.read_jobs('123', get)), 101)
        self.assertEqual(len(calls), 2)
        for response in ({'total_count': 101, 'jobs': []}, {'total_count': 1001, 'jobs': []}, {'total_count': True, 'jobs': []}):
            with self.assertRaises(p.MaintenanceError): post.read_jobs('123', lambda path: response)
