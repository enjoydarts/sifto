"""Natural discovery recovers only obsolete, provably unreviewed native gates."""
import base64
import copy
import os
import unittest
from unittest.mock import patch

from dependency_updates import github, pipeline, policy
import test_updates as updates
import test_followup as followup_tests

BASE, SHA = updates.BASE, updates.SHA

REPO = {'id': 123, 'full_name': 'owner/repo'}
BOT = {'id': 41898282, 'login': 'github-actions[bot]', 'type': 'Bot'}
OLD = 'd' * 40


def workflow_blob(three=True):
    names = ['prepare', 'owner-approval', 'approve-merge-deploy'] if three else ['prepare', 'approve-merge-deploy']
    content = 'jobs:\n'
    for name in names:
        content += '  ' + name + ':\n    runs-on: ubuntu-26.04\n'
        if name == ('owner-approval' if three else 'approve-merge-deploy'):
            content += '    needs: prepare\n    environment: production-maintenance\n'
        elif name == 'approve-merge-deploy':
            content += '    needs: [prepare, owner-approval]\n'
    return {'type': 'file', 'encoding': 'base64', 'size': len(content),
            'content': base64.b64encode(content.encode()).decode()}


class StaleApprovalTests(unittest.TestCase):
    def setUp(self):
        self.fixture = updates.PipelineTests()
        self.fixture.setUp()
        self.addCleanup(self.fixture.doCleanups)
        self.run = {'id': 20, 'workflow_id': 20, 'run_number': 17,
                    'display_title': 'Dependency PR 1 ' + OLD + ' ' + BASE,
                    'status': 'waiting', 'conclusion': None, 'event': 'workflow_dispatch',
                    'head_sha': BASE, 'head_branch': 'main', 'run_attempt': 1,
                    'path': '.github/workflows/dependency-update.yml',
                    'actor': BOT, 'triggering_actor': BOT, 'repository': REPO, 'head_repository': REPO}
        self.jobs = [
            {'id': 201, 'run_id': 20, 'run_attempt': 1, 'head_sha': BASE, 'name': 'prepare',
             'status': 'completed', 'conclusion': 'success', 'steps': [{'name': 'prepare'}]},
            {'id': 202, 'run_id': 20, 'run_attempt': 1, 'head_sha': BASE, 'name': 'approve-merge-deploy',
             'status': 'waiting', 'conclusion': None, 'steps': [], 'runner_id': None,
             'runner_name': None, 'started_at': '2026-10-15T05:00:59Z'}]
        self.reviews = []
        self.pending = [{'environment': {'id': 42, 'name': 'production-maintenance'},
                         'reviewers': [{'type': 'User', 'reviewer': {'id': 19749005, 'login': 'enjoydarts', 'type': 'User'}}]}]
        self.reads, self.hook = [], None
        self.inventory_override = self.jobs_override = None
        self.api = github.GitHub(REPO['full_name'], self.transport)

    def transport(self, method, path, body):
        if method != 'GET':
            return self.fixture.transport(method, path, body)
        self.reads.append(path)
        if self.hook:
            self.hook(path)
        endpoint = path.split('?')[0]
        if endpoint == '/actions/workflows/dependency-update.yml/runs':
            return copy.deepcopy(self.inventory_override if self.inventory_override is not None else {
                'workflow_runs': [self.run], 'total_count': 1})
        if endpoint == '/actions/runs/20': return copy.deepcopy(self.run)
        if endpoint == '/actions/runs/20/jobs':
            return copy.deepcopy(self.jobs_override if self.jobs_override is not None else {'jobs': self.jobs, 'total_count': len(self.jobs)})
        if endpoint == '/actions/runs/20/approvals': return copy.deepcopy(self.reviews)
        if endpoint == '/actions/runs/20/pending_deployments': return copy.deepcopy(self.pending)
        if endpoint == '/contents/.github/workflows/dependency-update.yml': return workflow_blob(False)
        if endpoint == '/pulls': return [{'number': 1, 'head': {'sha': self.fixture.head}}]
        if endpoint == '/pulls/1':
            pr = self.fixture.transport(method, path, body)
            pr.update(number=1, user=BOT)
            pr['head']['repo'] = pr['base']['repo'] = REPO
            return pr
        return self.fixture.transport(method, path, body)

    def discover(self):
        with patch.dict(os.environ, {'GITHUB_SHA': BASE}):
            pipeline.discover(self.api)

    def test_observed_old_unreviewed_waiting_head_gets_one_fresh_daily_request(self):
        self.discover()
        self.assertEqual(self.fixture.mutations, [('POST', '/actions/workflows/dependency-update.yml/dispatches', {
            'ref': 'main', 'inputs': {'pr_number': '1', 'head_sha': SHA, 'base_sha': BASE, 'source_sha': BASE}})])
        self.assertGreaterEqual(self.reads.count('/actions/runs/20/approvals'), 4)

    def test_unchanged_running_approved_denied_or_incomplete_gate_remains_busy(self):
        for condition in ['unchanged', 'running', 'queued', 'approved', 'rejected', 'reviews',
                          'runner', 'steps', 'missing_runner', 'missing_jobs', 'pending',
                          'reviewer', 'actor', 'repo', 'registration', 'rerun', 'prepare', 'runner_bool']:
            with self.subTest(condition=condition):
                self.setUp()
                if condition == 'unchanged': self.fixture.head = OLD
                if condition in {'running', 'queued'}: self.run['status'] = 'in_progress' if condition == 'running' else 'queued'
                if condition in {'approved', 'rejected'}: self.reviews = [{'state': condition}]
                if condition == 'reviews': self.reviews = {}
                if condition == 'runner': self.jobs[-1]['runner_id'] = 1
                if condition == 'steps': self.jobs[-1]['steps'] = [{'name': 'started'}]
                if condition == 'missing_runner': del self.jobs[-1]['runner_name']
                if condition == 'missing_jobs': self.jobs = self.jobs[:1]
                if condition == 'pending': self.pending = []
                if condition == 'reviewer': self.pending[0]['reviewers'][0]['reviewer']['id'] = 123
                if condition == 'actor': self.run['actor'] = dict(BOT, id=123)
                if condition == 'repo': self.run['repository'] = dict(REPO, id=456)
                if condition == 'registration': self.run['workflow_id'] = 99
                if condition == 'rerun': self.run['run_attempt'] = 2
                if condition == 'prepare': self.jobs[0]['conclusion'] = 'failure'
                if condition == 'runner_bool': self.jobs[-1]['runner_id'] = False
                try:
                    self.discover()
                except policy.Refused:
                    pass
                self.assertEqual(self.fixture.mutations, [])

    def test_changed_authority_or_candidate_at_readback_or_predispatch_stops_request(self):
        for boundary in [2, 3, 4]:
            for condition in ['head', 'base', 'review', 'job', 'run', 'pending']:
                with self.subTest(boundary=boundary, condition=condition):
                    self.setUp()
                    def hook(path):
                        if path == '/actions/runs/20' and self.reads.count(path) == boundary:
                            if condition == 'head': self.fixture.head = 'e' * 40
                            if condition == 'base': self.fixture.base = 'e' * 40
                            if condition == 'review': self.reviews = [{'state': 'approved'}]
                            if condition == 'job': self.jobs[-1]['runner_id'] = 1
                            if condition == 'run': self.run['status'] = 'in_progress'
                            if condition == 'pending': self.pending = []
                    self.hook = hook
                    try:
                        self.discover()
                    except policy.Refused:
                        pass
                    self.assertEqual(self.fixture.mutations, [])

    def test_head_restored_before_dispatch_cannot_reuse_old_gate(self):
        def hook(path):
            if path == '/actions/runs/20' and self.reads.count(path) == 3:
                self.fixture.head = OLD
        self.hook = hook
        self.discover()
        self.assertEqual(self.fixture.mutations, [])

    def test_truncated_or_missing_inventory_never_dispatches(self):
        for response in [{'jobs': self.jobs[:1], 'total_count': 2}, {'jobs': self.jobs}]:
            with self.subTest(response=response):
                self.jobs_override = response
                with self.assertRaises(policy.Refused): self.discover()
                self.assertEqual(self.fixture.mutations, [])

    def test_new_three_job_waiting_owner_gate_can_be_replaced(self):
        self.jobs[-1].update(status='pending', started_at=None)
        self.jobs.insert(1, dict(self.jobs[-1], id=203, name='owner-approval', status='waiting',
                                 started_at='2026-10-15T05:00:59Z'))
        original = self.api.transport
        def transport(method, path, body):
            if path.startswith('/contents/.github/workflows/dependency-update.yml'): return workflow_blob()
            return original(method, path, body)
        self.api.transport = transport
        self.discover()
        self.assertEqual(len(self.fixture.mutations), 1)

    def test_waiting_three_job_dag_omits_uncreated_downstream_apply_in_real_rest_inventory(self):
        self.jobs[-1]['name'] = 'owner-approval'
        original = self.api.transport
        def transport(method, path, body):
            if path.startswith('/contents/.github/workflows/dependency-update.yml'): return workflow_blob()
            return original(method, path, body)
        self.api.transport = transport
        self.discover()
        self.assertEqual(len(self.fixture.mutations), 1)

    def test_waiting_omitted_apply_requires_exact_authenticated_owner_dag(self):
        for condition in ['owner_needs', 'apply_needs', 'apply_environment', 'duplicate_needs', 'owner_environment']:
            with self.subTest(condition=condition):
                self.setUp()
                self.jobs[-1]['name'] = 'owner-approval'
                value = workflow_blob()
                text = base64.b64decode(value['content']).decode()
                if condition == 'owner_needs': text = text.replace('    needs: prepare\n', '')
                if condition == 'apply_needs': text = text.replace('    needs: [prepare, owner-approval]', '    needs: prepare')
                if condition == 'apply_environment': text += '    environment: production-maintenance\n'
                if condition == 'duplicate_needs': text += '    needs: prepare\n'
                if condition == 'owner_environment': text = text.replace('environment: production-maintenance', 'environment: other')
                value.update(content=base64.b64encode(text.encode()).decode(), size=len(text))
                original = self.api.transport
                def transport(method, path, body):
                    if path.startswith('/contents/.github/workflows/dependency-update.yml'): return value
                    return original(method, path, body)
                self.api.transport = transport
                with self.assertRaises(policy.Refused): self.discover()
                self.assertEqual(self.fixture.mutations, [])

    def test_old_source_waiting_gate_is_replaceable_but_old_source_deployment_is_busy(self):
        for status in ['waiting', 'in_progress']:
            with self.subTest(status=status):
                self.setUp()
                self.run.update(head_sha=OLD, status=status,
                                display_title='Dependency PR 1 ' + SHA + ' ' + OLD)
                for job in self.jobs: job['head_sha'] = OLD
                self.discover()
                self.assertEqual(len(self.fixture.mutations), 1 if status == 'waiting' else 0)

    def test_empty_missing_or_changed_rest_run_and_pending_metadata_never_exempts(self):
        for condition in ['run', 'missing_title', 'pending_object', 'reviewer_object', 'missing_steps',
                          'wrong_gate', 'duplicate_job', 'job_attempt', 'source_shape']:
            with self.subTest(condition=condition):
                self.setUp()
                original = self.api.transport
                def transport(method, path, body):
                    result = original(method, path, body)
                    if path == '/actions/runs/20':
                        if condition == 'run': return {}
                        if condition == 'missing_title': result.pop('display_title')
                    if path.startswith('/contents/.github/workflows/dependency-update.yml') and condition == 'source_shape':
                        value = workflow_blob()
                        value['type'] = 'dir'
                        return value
                    return result
                self.api.transport = transport
                if condition == 'pending_object': self.pending = {}
                if condition == 'reviewer_object': self.pending[0]['reviewers'][0]['reviewer'] = None
                if condition == 'missing_steps': del self.jobs[-1]['steps']
                if condition == 'wrong_gate': self.jobs[-1]['name'] = 'unknown'
                if condition == 'duplicate_job': self.jobs[-1]['id'] = self.jobs[0]['id']
                if condition == 'job_attempt': self.jobs[-1]['run_attempt'] = 2
                try:
                    self.discover()
                except policy.Refused:
                    pass
                self.assertEqual(self.fixture.mutations, [])


class ClosedStaleApprovalTests(unittest.TestCase):
    """Closing an unreviewed candidate must not strand another open PR."""
    discover = StaleApprovalTests.discover

    def setUp(self):
        StaleApprovalTests.setUp(self)
        self.fixture.head = OLD
        self.lifecycle = {'state': 'closed', 'merged': False}

    def transport(self, method, path, body):
        endpoint = path.split('?')[0]
        if method == 'GET' and endpoint.startswith('/pulls/2'):
            value = self.fixture.transport(method, path.replace('/pulls/2', '/pulls/1', 1), body)
            if endpoint == '/pulls/2':
                value['head']['sha'] = SHA
            return value
        value = StaleApprovalTests.transport(self, method, path, body)
        if method == 'GET' and endpoint == '/pulls':
            return [{'number': 2, 'head': {'sha': SHA}}]
        if method == 'GET' and endpoint == '/pulls/1':
            value.update(self.lifecycle)
        return value

    def test_closed_unmerged_unchanged_waiting_candidate_allows_one_other_open_request(self):
        self.discover()
        self.assertEqual(self.fixture.mutations, [('POST', '/actions/workflows/dependency-update.yml/dispatches', {
            'ref': 'main', 'inputs': {'pr_number': '2', 'head_sha': SHA, 'base_sha': BASE, 'source_sha': BASE}})])
        self.assertGreaterEqual(self.reads.count('/actions/runs/20/approvals'), 4)

    def test_reopening_or_merging_closed_candidate_at_each_readback_stops_dispatch(self):
        for boundary in [2, 3, 4]:
            for lifecycle in [{'state': 'open', 'merged': False},
                              {'state': 'closed', 'merged': True, 'merge_commit_sha': BASE},
                              {'state': 'unknown', 'merged': False}, {'state': 'closed', 'merged': None}]:
                with self.subTest(boundary=boundary, lifecycle=lifecycle):
                    self.setUp()
                    # Reopening still leaves an obsolete head. Only freezing
                    # lifecycle authority detects this change between reads.
                    self.fixture.head = SHA
                    def hook(path):
                        if path == '/actions/runs/20' and self.reads.count(path) == boundary:
                            self.lifecycle = lifecycle
                    self.hook = hook
                    try:
                        self.discover()
                    except policy.Refused:
                        pass
                    self.assertEqual(self.fixture.mutations, [])

    def test_closed_reviewed_running_or_incomplete_candidate_never_exempts(self):
        for condition in ['approved', 'rejected', 'runner', 'steps', 'missing_head', 'missing_job',
                          'missing_pending', 'unknown_state', 'malformed_state', 'unknown_merged', 'missing_merge_sha']:
            with self.subTest(condition=condition):
                self.setUp()
                if condition in {'approved', 'rejected'}: self.reviews = [{'state': condition}]
                if condition == 'runner': self.jobs[-1]['runner_id'] = 1
                if condition == 'steps': self.jobs[-1]['steps'] = [{'name': 'started'}]
                if condition == 'missing_job': self.jobs = self.jobs[:1]
                if condition == 'missing_pending': self.pending = []
                if condition == 'unknown_state': self.lifecycle['state'] = None
                if condition == 'malformed_state': self.lifecycle['state'] = []
                if condition == 'unknown_merged': self.lifecycle['merged'] = None
                if condition == 'missing_merge_sha': self.lifecycle['merged'] = True
                if condition == 'missing_head':
                    original = self.api.transport
                    def transport(method, path, body):
                        value = original(method, path, body)
                        if path == '/pulls/1': value['head'].pop('sha')
                        return value
                    self.api.transport = transport
                try:
                    self.discover()
                except policy.Refused:
                    pass
                self.assertEqual(self.fixture.mutations, [])

    def merged_waiting(self, three=False):
        old_source = 'e' * 40
        self.fixture.head = SHA
        self.run.update(head_sha=old_source, display_title='Dependency PR 1 ' + OLD + ' ' + old_source)
        for job in self.jobs: job['head_sha'] = old_source
        if three:
            self.jobs[-1]['name'] = 'owner-approval'
        completed = dict(self.run, id=21, run_number=18, status='completed', conclusion='success',
                         display_title='Dependency PR 1 ' + SHA + ' ' + old_source)
        self.inventory_override = {'workflow_runs': [self.run, completed], 'total_count': 2}
        self.lifecycle = {'state': 'closed', 'merged': True, 'merge_commit_sha': BASE}
        original = self.api.transport
        def transport(method, path, body):
            if path.startswith('/contents/.github/workflows/dependency-update.yml'):
                return workflow_blob(three)
            return original(method, path, body)
        self.api.transport = transport

    def test_merged_newer_request_leaves_old_unreviewed_gate_idle_for_next_open_pr(self):
        for three in [False, True]:
            with self.subTest(three_jobs=three):
                self.setUp()
                self.merged_waiting(three)
                self.discover()
                self.assertEqual(self.fixture.mutations, [('POST', '/actions/workflows/dependency-update.yml/dispatches', {
                    'ref': 'main', 'inputs': {'pr_number': '2', 'head_sha': SHA, 'base_sha': BASE, 'source_sha': BASE}})])
                self.assertGreaterEqual(self.reads.count('/actions/runs/20/approvals'), 4)

    def test_merged_gate_missing_invalid_binding_or_reviewed_incomplete_work_stays_busy(self):
        for condition in ['missing_sha', 'invalid_sha', 'malformed_sha', 'approved', 'rejected',
                          'runner', 'missing_job', 'missing_pending', 'open_merged']:
            with self.subTest(condition=condition):
                self.setUp()
                self.merged_waiting()
                if condition == 'missing_sha': self.lifecycle.pop('merge_commit_sha')
                if condition == 'invalid_sha': self.lifecycle['merge_commit_sha'] = 'invalid'
                if condition == 'malformed_sha': self.lifecycle['merge_commit_sha'] = []
                if condition in {'approved', 'rejected'}: self.reviews = [{'state': condition}]
                if condition == 'runner': self.jobs[-1]['runner_id'] = 1
                if condition == 'missing_job': self.jobs = self.jobs[:1]
                if condition == 'missing_pending': self.pending = []
                if condition == 'open_merged': self.lifecycle['state'] = 'open'
                try:
                    self.discover()
                except policy.Refused:
                    pass
                self.assertEqual(self.fixture.mutations, [])

    def test_merged_gate_lifecycle_or_merge_binding_change_at_readback_stops_dispatch(self):
        for boundary in [2, 3, 4]:
            for condition in ['reopened', 'unmerged', 'merge_sha']:
                with self.subTest(boundary=boundary, condition=condition):
                    self.setUp()
                    self.merged_waiting()
                    def hook(path):
                        if path == '/actions/runs/20' and self.reads.count(path) == boundary:
                            if condition == 'reopened': self.lifecycle.update(state='open', merged=False)
                            if condition == 'unmerged': self.lifecycle['merged'] = False
                            if condition == 'merge_sha': self.lifecycle['merge_commit_sha'] = 'f' * 40
                    self.hook = hook
                    try:
                        self.discover()
                    except policy.Refused:
                        pass
                    self.assertEqual(self.fixture.mutations, [])

    def test_another_running_apply_blocks_even_after_merged_gate_is_obsolete(self):
        self.merged_waiting()
        active = dict(self.run, id=22, run_number=19, status='in_progress',
                      display_title='Dependency PR 3 ' + SHA + ' ' + self.run['head_sha'])
        self.inventory_override['workflow_runs'].append(active)
        self.inventory_override['total_count'] += 1
        self.discover()
        self.assertEqual(self.fixture.mutations, [])


class SupersessionTests(unittest.TestCase):
    def setUp(self):
        self.fixture = updates.PipelineTests()
        self.fixture.setUp()
        self.addCleanup(self.fixture.doCleanups)
        self.later = dict(self.fixture.transport('GET', '/actions/runs/10', None), id=2,
                          run_number=18, status='waiting')
        self.extra, self.reads = [], 0
        original = self.fixture.transport
        def transport(method, path, body):
            if method == 'GET' and path.startswith('/actions/workflows/dependency-update.yml/runs'):
                self.reads += 1
                rows = [original('GET', '/actions/runs/10', None)] + self.extra
                return {'workflow_runs': rows, 'total_count': len(rows)}
            return original(method, path, body)
        self.fixture.api.transport = transport

    def test_newer_registered_request_for_same_pr_permanently_supersedes_restored_old_head(self):
        self.extra = [self.later]
        with self.assertRaisesRegex(policy.Refused, 'supersed'):
            self.fixture.execute(self.fixture.plan())
        self.assertEqual(self.fixture.mutations, [])

    def test_later_request_before_merge_revalidation_refuses_old_approval(self):
        self.fixture.image_validation_hook = lambda: self.extra.append(self.later)
        with self.assertRaisesRegex(policy.Refused, 'supersed'):
            self.fixture.execute(self.fixture.plan())
        self.assertEqual(self.fixture.mutations, [])

    def test_other_pr_or_older_workflow_sequence_does_not_supersede(self):
        self.extra = [dict(self.later, display_title='Dependency PR 2 ' + SHA + ' ' + BASE),
                      dict(self.later, id=999999, run_number=16)]
        self.fixture.execute(self.fixture.plan())
        self.assertEqual(sum(m[0] == 'PUT' for m in self.fixture.mutations), 1)
        self.assertEqual(self.reads, 2)

    def test_untrusted_duplicate_missing_or_malformed_native_sequence_never_merges(self):
        for condition in ['workflow', 'actor', 'repo', 'duplicate_number', 'duplicate_id', 'missing', 'bool', 'rerun', 'title']:
            with self.subTest(condition=condition):
                self.setUp()
                later = dict(self.later, display_title='Dependency PR 2 ' + SHA + ' ' + BASE)
                if condition == 'workflow': later['workflow_id'] = 99
                if condition == 'actor': later['actor'] = dict(BOT, id=99)
                if condition == 'repo': later['repository'] = dict(REPO, id=999)
                if condition == 'duplicate_number': later['run_number'] = 17
                if condition == 'duplicate_id': later['id'] = 10
                if condition == 'missing': del later['run_number']
                if condition == 'bool': later['run_number'] = True
                if condition == 'rerun': later['run_attempt'] = 2
                if condition == 'title': later['display_title'] = 'unknown'
                self.extra = [later]
                with self.assertRaises(policy.Refused): self.fixture.execute(self.fixture.plan())
                self.assertEqual(self.fixture.mutations, [])

    def test_current_execution_status_id_or_sequence_change_cannot_consume_approval(self):
        for condition in ['status', 'id', 'sequence']:
            with self.subTest(condition=condition):
                self.setUp()
                original = self.fixture.api.transport
                def transport(method, path, body):
                    value = original(method, path, body)
                    if path == '/actions/runs/10':
                        if condition == 'status': value.update(status='completed', conclusion='success')
                        if condition == 'id': value['id'] = 11
                        if condition == 'sequence' and self.reads > 0: value['run_number'] = 16
                    return value
                self.fixture.api.transport = transport
                with self.assertRaises(policy.Refused): self.fixture.execute(self.fixture.plan())
                self.assertEqual(self.fixture.mutations, [])


class NativeShapeTests(unittest.TestCase):
    def test_three_job_completed_and_active_followup_require_successful_owner_gate(self):
        for active in [False, True]:
            for missing in [False, True]:
                with self.subTest(active=active, missing=missing):
                    fixture = followup_tests.QueueFollowupTests() if active else followup_tests.CompletedFollowupTests()
                    fixture.setUp()
                    fixture.three_jobs = True
                    if active: fixture.active()
                    if not missing:
                        fixture.jobs.append(dict(fixture.jobs[0], id=102, name='owner-approval'))
                    if missing:
                        with self.assertRaises(policy.Refused): fixture.queue() if active else fixture.notify()
                        self.assertEqual(fixture.mutations, [])
                    else:
                        self.assertEqual(fixture.queue() if active else fixture.notify(), 'requested')

    def test_three_job_prepare_failure_retries_only_with_both_gates_provably_unstarted(self):
        from test_updates import ReproposalTests
        for condition in ['valid', 'missing_owner', 'owner_started', 'owner_denied', 'job_rerun']:
            with self.subTest(condition=condition):
                fixture = ReproposalTests()
                fixture.setUp()
                for job in fixture.jobs: job['run_attempt'] = 1
                original = fixture.api.transport
                def transport(method, path, body):
                    if path.startswith('/contents/.github/workflows/dependency-update.yml'): return workflow_blob()
                    return original(method, path, body)
                fixture.api.transport = transport
                if condition != 'missing_owner':
                    fixture.jobs.append(dict(fixture.jobs[1], id=103, name='owner-approval'))
                if condition == 'owner_started': fixture.jobs[-1]['runner_id'] = 1
                if condition == 'owner_denied': fixture.approvals = [{'state': 'rejected'}]
                if condition == 'job_rerun': fixture.jobs[-1]['run_attempt'] = 2
                self.assertEqual(pipeline.deduplicated(fixture.api, fixture.plan), condition != 'valid')

    def test_authenticated_source_rejects_unknown_or_ambiguous_job_shapes(self):
        api = github.GitHub('owner/repo')
        for content in [
                'jobs:\n  prepare:\n  approve-merge-deploy:\n  extra:\n',
                'jobs:\n  prepare:\n  owner-approval:\n  owner-approval:\n  approve-merge-deploy:\n',
                'jobs:\n  prepare:\n  owner-approval: {}\n  approve-merge-deploy:\n',
                'jobs:\n  prepare:\n  approve-merge-deploy:\njobs:\n  prepare:\n',
                'jobs: &other\n  prepare:\n  approve-merge-deploy:\n']:
            with self.subTest(content=content), patch.object(pipeline, 'blob', return_value=content):
                with self.assertRaises(policy.Refused): pipeline.native_job_names(api, BASE)


if __name__ == '__main__':
    unittest.main()
