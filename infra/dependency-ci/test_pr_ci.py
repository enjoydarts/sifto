"""Offline identities and race tests for bounded read-only Bot PR CI approval."""
import base64
import copy
import importlib.util
import os
from pathlib import Path
import tempfile
import unittest
from unittest.mock import patch
from urllib.error import HTTPError
from urllib.parse import parse_qs, urlsplit

from dependency_updates import github, pipeline, policy
import test_updates
from test_updates import FakeClock, SHA, BASE

WORKFLOWS = ('dependency-check.yml', 'app-runtime.yml', 'maintenance-test.yml')
BOT = {'id': 41898282, 'login': 'github-actions[bot]', 'type': 'Bot'}
REPO = {'id': 123, 'full_name': 'owner/repo', 'name': 'repo', 'url': 'https://api.github.com/repos/owner/repo',
        'owner': {'id': 42, 'login': 'enjoydarts', 'type': 'User'}}
NAMES = {
    'dependency-check.yml': set(policy.CHECKS),
    'app-runtime.yml': {'App Runtime api', 'App Runtime worker', 'App Runtime web'},
    'maintenance-test.yml': {'PostgreSQL data mount ordering', 'test'},
}


class Fixture:
    def __init__(self):
        self.candidate = test_updates.PipelineTests()
        self.candidate.setUp()
        self.plan = pipeline.candidate(self.candidate.api, 1, SHA, BASE, BASE)
        self.calls = []
        self.registered = {name: {'id': index + 100, 'path': '.github/workflows/' + name,
                                 'state': 'active'} for index, name in enumerate(WORKFLOWS)}
        self.runs = {}
        self.jobs = {}
        self.read_hook = None
        self.post_error = None
        self.initial_attempts = {}
        self.post_hook = None
        self.finish_at = 0
        self.clock = FakeClock()
        self.job_total = None
        self.hidden_jobs = False
        self.run_total = None
        self.workflow_source = None
        for index, name in enumerate(WORKFLOWS[:2]):
            run = self.run(name, index + 200)
            self.runs[run['id']] = run
            self.jobs[run['id']] = []
        self.api = github.GitHub('owner/repo', transport=self.transport)

    def run(self, workflow, identifier):
        return {'id': identifier, 'workflow_id': self.registered[workflow]['id'],
                'path': '.github/workflows/' + workflow, 'event': 'pull_request',
                'head_sha': SHA, 'head_branch': self.plan['branch'],
                'repository': copy.deepcopy(REPO), 'head_repository': copy.deepcopy(REPO),
                'actor': copy.deepcopy(BOT), 'triggering_actor': copy.deepcopy(BOT),
                'run_attempt': 1, 'status': 'completed', 'conclusion': 'action_required',
                'pull_requests': []}

    def success(self, identifier):
        run = self.runs[identifier]
        run.update(status='completed', conclusion='success')
        name = run['path'].split('/')[-1]
        self.jobs[identifier] = [{'id': identifier * 10 + index, 'run_id': identifier,
                                 'run_attempt': run['run_attempt'],
                                 'head_sha': SHA, 'name': job, 'status': 'completed',
                                 'conclusion': 'success'}
                                for index, job in enumerate(sorted(NAMES[name]))]

    def transport(self, method, path, body):
        self.calls.append((method, path, body))
        endpoint = path.split('?')[0]
        if 'pending_deployments' in path:
            raise AssertionError('Native environment approval must never be read or changed')
        if self.read_hook and method == 'GET':
            self.read_hook(path)
        if method == 'POST':
            if self.post_error:
                raise self.post_error
            identifier = int(endpoint.split('/')[3])
            self.initial_attempts[identifier] = copy.deepcopy(self.runs[identifier])
            self.runs[identifier].update(run_attempt=2, status='queued', conclusion=None)
            if self.post_hook: self.post_hook(identifier)
            return None
        if endpoint == '':
            return copy.deepcopy(REPO)
        if endpoint.startswith('/actions/workflows/'):
            name = endpoint.split('/')[3]
            if not endpoint.endswith('/runs'):
                return copy.deepcopy(self.registered[name])
            workflow_id = int(name)
            name = next(name for name, workflow in self.registered.items() if workflow['id'] == workflow_id)
            rows = [copy.deepcopy(run) for run in self.runs.values()
                    if run['path'] == '.github/workflows/' + name or run['id'] == 200 and workflow_id == 100]
            params = parse_qs(urlsplit(path).query)
            page = int(params['page'][0])
            return {'workflow_runs': rows[(page - 1) * 100:page * 100],
                    'total_count': self.run_total if self.run_total is not None else len(rows)}
        if endpoint.startswith('/actions/runs/'):
            identifier = int(endpoint.split('/')[3])
            if '/attempts/1' in endpoint:
                if endpoint.endswith('/jobs'):
                    rows = [] if identifier in self.initial_attempts else copy.deepcopy(self.jobs[identifier])
                    return {'jobs': rows, 'total_count': len(rows)}
                return copy.deepcopy(self.initial_attempts[identifier])
            if endpoint.endswith('/jobs'):
                rows = copy.deepcopy(self.jobs[identifier])
                if self.hidden_jobs and 'filter=all' in path:
                    rows = [{'id': 99, 'run_id': identifier, 'head_sha': SHA}]
                return {'jobs': rows, 'total_count': self.job_total if self.job_total is not None else len(rows)}
            if self.runs[identifier]['status'] in ('queued', 'in_progress') and self.clock.elapsed >= self.finish_at:
                self.success(identifier)
            return copy.deepcopy(self.runs[identifier])
        if endpoint.startswith('/contents/.github/workflows/'):
            name = endpoint.split('/')[-1]
            text = (Path('/repository/.github/workflows') / name).read_text()
            if self.workflow_source and 'ref=' + SHA in path:
                text = self.workflow_source
            return {'type': 'file', 'encoding': 'base64', 'size': len(text),
                    'content': base64.b64encode(text.encode()).decode()}
        if endpoint == '/pulls/1':
            pr = self.candidate.transport(method, path, body)
            pr['user'] = copy.deepcopy(BOT if self.candidate.author == BOT['login'] else REPO['owner'])
            pr['head']['repo']['id'] = REPO['id']
            pr['base']['repo']['id'] = REPO['id']
            return pr
        return self.candidate.transport(method, path, body)

    @property
    def posts(self):
        return [call for call in self.calls if call[0] == 'POST']

    def revalidate(self):
        current = pipeline.candidate(self.api, 1, SHA, BASE, BASE,
                                     python_cutoff=self.plan['python_age']['cutoff'])
        policy.assert_unchanged(self.plan, current, policy.plan_hash(self.plan))


class PRCITests(unittest.TestCase):
    def setUp(self):
        self.flag = patch.dict(os.environ, {'DEPENDENCY_PR_CI_AUTO_APPROVAL': 'true'})
        self.flag.start()
        self.addCleanup(self.flag.stop)
        self.assertIsNotNone(importlib.util.find_spec('dependency_updates.pr_ci'),
                             'bounded Bot PR CI approval helper must be implemented')
        from dependency_updates import pr_ci
        self.ci = pr_ci
        self.f = Fixture()

    def start(self):
        return self.ci.start(self.f.api, self.f.plan, self.f.revalidate,
                             clock=self.f.clock.monotonic, sleep=self.f.clock.sleep)

    def wait(self, session):
        return session.wait(clock=self.f.clock.monotonic, sleep=self.f.clock.sleep)

    def test_valid_bot_empty_pr_array_approves_each_held_run_once(self):
        session = self.start()
        self.assertEqual(self.f.calls[0][:2], ('GET', ''))
        self.assertEqual([p[1] for p in self.f.posts], ['/actions/runs/200/approve', '/actions/runs/201/approve'])
        self.wait(session)
        self.assertEqual(len(self.f.posts), 2)
        self.assertEqual({run['run_attempt'] for run in session.selected.values()}, {2})
        self.assertEqual(session.approved, {200, 201})

    def test_cross_workflow_change_after_first_ack_blocks_second_post(self):
        for condition in ('failure', 'attempt', 'actor', 'job_failure'):
            with self.subTest(condition=condition):
                self.f = Fixture()
                def change(identifier):
                    if condition == 'failure': self.f.runs[201]['conclusion'] = 'failure'
                    if condition == 'attempt': self.f.runs[201]['run_attempt'] = 2
                    if condition == 'actor': self.f.runs[200]['triggering_actor'] = REPO['owner']
                    if condition == 'job_failure':
                        self.f.finish_at = 100
                        self.f.runs[200].update(status='in_progress', conclusion=None)
                        self.f.jobs[200] = [{'id': 99, 'run_id': 200, 'run_attempt': 2, 'head_sha': SHA,
                                            'name': 'Dependency API', 'status': 'completed', 'conclusion': 'failure'}]
                self.f.post_hook = change
                with self.assertRaises(policy.Refused): self.start()
                self.assertEqual([p[1] for p in self.f.posts], ['/actions/runs/200/approve'])

    def test_manually_preapproved_other_run_actual_jobs_gate_every_post(self):
        for condition in ('active_failure', 'success_failure', 'attempt', 'unknown_state', 'missing_success'):
            with self.subTest(condition=condition):
                self.f = Fixture()
                self.f.finish_at = 100
                self.f.runs[201].update(run_attempt=2, triggering_actor=copy.deepcopy(REPO['owner']))
                self.f.success(201)
                if condition == 'active_failure': self.f.runs[201].update(status='in_progress', conclusion=None)
                if condition in ('active_failure', 'success_failure'): self.f.jobs[201][0]['conclusion'] = 'failure'
                if condition == 'attempt': self.f.jobs[201][0]['run_attempt'] = 1
                if condition == 'unknown_state': self.f.jobs[201][0]['status'] = 'unexpected'
                if condition == 'missing_success': self.f.jobs[201].pop()
                with self.assertRaises(policy.Refused): self.start()
                self.assertEqual(self.f.posts, [])

    def test_preapproved_job_failure_after_candidate_revalidation_blocks_post(self):
        self.f.runs[201].update(run_attempt=2, triggering_actor=copy.deepcopy(REPO['owner']))
        self.f.success(201)
        original = self.f.revalidate
        def fail():
            original()
            self.f.jobs[201][0]['conclusion'] = 'failure'
        self.f.revalidate = fail
        with self.assertRaises(policy.Refused): self.start()
        self.assertEqual(self.f.posts, [])

    def test_manually_preapproved_normal_completion_refreshes_before_other_post(self):
        self.f.finish_at = 100
        self.f.runs[201].update(run_attempt=2, status='in_progress', conclusion=None,
                                triggering_actor=copy.deepcopy(REPO['owner']))
        original = self.f.api.transport
        def complete(method, path, body):
            if '/actions/runs/201/attempts/2/jobs' in path:
                self.f.success(201)
            if path == '/actions/runs/200/approve':
                self.assertEqual(self.f.clock.elapsed, 10)
            return original(method, path, body)
        self.f.api.transport = complete
        session = self.start()
        self.assertEqual(self.f.clock.elapsed, 10)
        self.assertEqual([p[1] for p in self.f.posts], ['/actions/runs/200/approve'])
        self.wait(session)
        self.assertEqual(self.f.clock.elapsed, 100)

    def test_acknowledged_transition_never_accepts_attempt_three_or_corrupt_initial_hold(self):
        for condition in ('attempt3', 'initial_actor', 'initial_jobs', 'list_actor', 'list_failure'):
            with self.subTest(condition=condition):
                self.f = Fixture()
                def change(identifier):
                    if condition == 'attempt3': self.f.runs[identifier]['run_attempt'] = 3
                    if condition == 'initial_actor': self.f.initial_attempts[identifier]['actor'] = REPO['owner']
                self.f.post_hook = change
                original = self.f.api.transport
                def transport(method, path, body):
                    result = original(method, path, body)
                    if condition == 'initial_jobs' and '/attempts/1/jobs' in path:
                        return {'jobs': [{'id': 9}], 'total_count': 1}
                    if 200 in self.f.initial_attempts and '/actions/workflows/100/runs?' in path:
                        if condition == 'list_actor': result['workflow_runs'][0]['triggering_actor'] = REPO['owner']
                        if condition == 'list_failure': result['workflow_runs'][0].update(status='completed', conclusion='failure')
                    return result
                self.f.api.transport = transport
                with self.assertRaises(policy.Refused): self.start()
                self.assertEqual(len(self.f.posts), 1)

    def test_acknowledged_list_and_detail_lag_share_deadline_and_never_replay(self):
        for endpoint in ('list', 'detail'):
            for delay in (30, 51 * 60):
                with self.subTest(endpoint=endpoint, delay=delay):
                    self.f = Fixture()
                    original = self.f.api.transport
                    def transport(method, path, body):
                        result = original(method, path, body)
                        if 200 in self.f.initial_attempts and self.f.clock.elapsed < delay:
                            if endpoint == 'list' and '/actions/workflows/100/runs?' in path:
                                result['workflow_runs'] = [copy.deepcopy(self.f.initial_attempts[200])]
                            if endpoint == 'detail' and path == '/actions/runs/200':
                                result = copy.deepcopy(self.f.initial_attempts[200])
                        if path == '/actions/runs/201/approve':
                            self.assertGreaterEqual(self.f.clock.elapsed, delay)
                        return result
                    self.f.api.transport = transport
                    if delay < 50 * 60:
                        self.wait(self.start())
                        self.assertEqual(self.f.clock.elapsed, delay)
                        self.assertEqual(len(self.f.posts), 2)
                    else:
                        with self.assertRaisesRegex(policy.Refused, 'Timed out'): self.start()
                        self.assertEqual(self.f.clock.elapsed, 50 * 60)
                        self.assertEqual(len(self.f.posts), 1)

    def test_visibility_poll_rechecks_other_failure_head_and_source_before_second_post(self):
        for condition in ('failure', 'head', 'source'):
            with self.subTest(condition=condition):
                self.f = Fixture()
                original = self.f.api.transport
                def transport(method, path, body):
                    if self.f.clock.elapsed >= 10:
                        if condition == 'failure': self.f.runs[201]['conclusion'] = 'failure'
                        if condition == 'head': self.f.candidate.head = 'c' * 40
                        if condition == 'source': self.f.candidate.base = 'c' * 40
                    result = original(method, path, body)
                    if 200 in self.f.initial_attempts and '/actions/workflows/100/runs?' in path:
                        result['workflow_runs'] = [copy.deepcopy(self.f.initial_attempts[200])]
                    return result
                self.f.api.transport = transport
                with self.assertRaises(policy.Refused): self.start()
                self.assertEqual(len(self.f.posts), 1)
                self.assertEqual(self.f.clock.elapsed, 10)

    def test_original_attempt_visibility_and_current_status_progression_are_bounded(self):
        self.f.finish_at = 100
        original = self.f.api.transport
        progressed = False
        def transport(method, path, body):
            nonlocal progressed
            if '/attempts/1' in path and self.f.clock.elapsed < 20:
                raise HTTPError('public', 404, 'not visible', {}, None)
            if '/attempts/2/jobs' in path and not progressed:
                progressed = True
                self.f.runs[200].update(status='in_progress', conclusion=None)
            return original(method, path, body)
        self.f.api.transport = transport
        self.wait(self.start())
        self.assertEqual(self.f.clock.elapsed, 100)
        self.assertEqual(len(self.f.posts), 2)

    def test_attempt_jobs_404_visibility_uses_shared_deadline_without_reapproval(self):
        for preapproved in (False, True):
            for delay in (30, 51 * 60):
                with self.subTest(preapproved=preapproved, delay=delay):
                    self.f = Fixture()
                    if preapproved:
                        self.f.runs[200].update(run_attempt=2, status='in_progress', conclusion=None,
                                                triggering_actor=copy.deepcopy(REPO['owner']))
                        self.f.finish_at = delay
                    original = self.f.api.transport
                    def transport(method, path, body):
                        if '/actions/runs/200/attempts/2/jobs' in path and self.f.clock.elapsed < delay:
                            raise HTTPError('public', 404, 'not visible', {}, None)
                        if path == '/actions/runs/201/approve':
                            self.assertGreaterEqual(self.f.clock.elapsed, delay)
                        return original(method, path, body)
                    self.f.api.transport = transport
                    if delay < 50 * 60:
                        session = self.start()
                        self.wait(session)
                        self.assertEqual(self.f.clock.elapsed, delay)
                        self.assertEqual([p[1] for p in self.f.posts],
                                         ([] if preapproved else ['/actions/runs/200/approve']) + ['/actions/runs/201/approve'])
                        self.assertEqual(session.approved, {201} if preapproved else {200, 201})
                    else:
                        with self.assertRaisesRegex(policy.Refused, 'Timed out'): self.start()
                        self.assertEqual(self.f.clock.elapsed, 50 * 60)
                        self.assertEqual(len(self.f.posts), 0 if preapproved else 1)

    def test_attempt_jobs_404_still_checks_other_authorities_before_next_post(self):
        for condition in ('failure', 'job_failure', 'head', 'source', 'attempt', 'actor'):
            with self.subTest(condition=condition):
                self.f = Fixture()
                if condition == 'job_failure':
                    self.f.runs[201].update(run_attempt=2, triggering_actor=copy.deepcopy(REPO['owner']))
                    self.f.success(201)
                original = self.f.api.transport
                def transport(method, path, body):
                    if self.f.clock.elapsed >= 10:
                        if condition == 'failure': self.f.runs[201]['conclusion'] = 'failure'
                        if condition == 'job_failure': self.f.jobs[201][0]['conclusion'] = 'failure'
                        if condition == 'head': self.f.candidate.head = 'c' * 40
                        if condition == 'source': self.f.candidate.base = 'c' * 40
                        if condition == 'attempt': self.f.runs[200]['run_attempt'] = 3
                        if condition == 'actor': self.f.runs[200]['triggering_actor'] = REPO['owner']
                    if '/actions/runs/200/attempts/2/jobs' in path:
                        raise HTTPError('public', 404, 'not visible', {}, None)
                    return original(method, path, body)
                self.f.api.transport = transport
                with self.assertRaises(policy.Refused): self.start()
                self.assertEqual(self.f.clock.elapsed, 10)
                self.assertEqual([p[1] for p in self.f.posts], ['/actions/runs/200/approve'])

    def test_attempt_jobs_other_errors_after_ack_fail_closed_without_visibility_poll(self):
        for condition in ('forbidden', 'unknown_http', 'timeout', 'malformed', 'unknown_job'):
            with self.subTest(condition=condition):
                self.f = Fixture()
                original = self.f.api.transport
                def transport(method, path, body):
                    if '/actions/runs/200/attempts/2/jobs' in path:
                        if condition == 'forbidden': raise HTTPError('public', 403, 'forbidden', {}, None)
                        if condition == 'unknown_http': raise HTTPError('public', 418, 'unknown', {}, None)
                        if condition == 'timeout': raise TimeoutError()
                        if condition == 'malformed': return {'jobs': []}
                        if condition == 'unknown_job':
                            return {'jobs': [{'id': 1, 'run_id': 200, 'run_attempt': 2, 'head_sha': SHA,
                                              'name': 'Dependency API', 'status': 'unknown', 'conclusion': None}], 'total_count': 1}
                    return original(method, path, body)
                self.f.api.transport = transport
                with patch.object(github.time, 'sleep'), self.assertRaises(policy.Refused): self.start()
                self.assertEqual(self.f.clock.elapsed, 0)
                self.assertEqual([p[1] for p in self.f.posts], ['/actions/runs/200/approve'])

    def test_unknown_http_outcome_cannot_be_promoted_by_real_successful_jobs(self):
        session = self.ci.Session(self.f.api, self.f.plan, self.f.revalidate,
                                  self.f.clock.monotonic, allow_approval=True)
        session.register()
        session.selected = session.inventory()
        original = self.f.api.transport
        def lose(method, path, body):
            result = original(method, path, body)
            if method == 'POST':
                self.f.success(200)
                raise TimeoutError()
            return result
        self.f.api.transport = lose
        with self.assertRaisesRegex(policy.Refused, 'unknown'):
            session.approve(self.f.clock.monotonic, self.f.clock.sleep)
        self.assertEqual(session.issued, {200})
        self.assertEqual(session.approved, set())
        with self.assertRaisesRegex(policy.Refused, 'unacknowledged'): self.wait(session)
        with self.assertRaises(policy.Refused): session.approve(self.f.clock.monotonic, self.f.clock.sleep)
        self.assertEqual(len(self.f.posts), 1)

    def test_default_off_and_nonexact_flag_refuse_holds_without_post(self):
        for flag in (None, '', 'false', 'TRUE', '1'):
            with self.subTest(flag=flag), patch.dict(os.environ):
                os.environ.pop('DEPENDENCY_PR_CI_AUTO_APPROVAL', None)
                if flag is not None:
                    os.environ['DEPENDENCY_PR_CI_AUTO_APPROVAL'] = flag
                self.f = Fixture()
                with self.assertRaisesRegex(policy.Refused, 'disabled'):
                    self.start()
                self.assertEqual(self.f.posts, [])

    def test_default_off_accepts_successful_preapproved_ci(self):
        with patch.dict(os.environ, {'DEPENDENCY_PR_CI_AUTO_APPROVAL': ''}):
            for identifier in self.f.runs:
                self.f.success(identifier)
            self.wait(self.start())
            self.assertEqual(self.f.posts, [])

    def test_http_rejection_reports_safe_status_without_transport_secrets(self):
        for code in (403, 404):
            self.f = Fixture()
            self.f.post_error = HTTPError('https://secret-url', code, 'secret-token', {}, None)
            with self.subTest(code=code), self.assertRaises(policy.Refused) as failure:
                self.start()
            self.assertIn('HTTP %d' % code, str(failure.exception))
            self.assertNotIn('secret', str(failure.exception))
            self.assertEqual(len(self.f.posts), 1)

    def test_success_queued_and_in_progress_are_never_approved(self):
        for status in ('success', 'queued', 'in_progress'):
            with self.subTest(status=status):
                self.f = Fixture()
                for identifier, run in self.f.runs.items():
                    if status == 'success':
                        self.f.success(identifier)
                    else:
                        run.update(status=status, conclusion=None)
                self.wait(self.start())
                self.assertEqual(self.f.posts, [])

    def test_invalid_identity_cannot_authorize_any_mutation(self):
        changes = [('id', True), ('workflow_id', 999), ('path', '.github/workflows/deploy.yml'),
                   ('event', 'pull_request_target'), ('head_sha', 'c' * 40), ('head_branch', 'other'),
                   ('repository', {'id': 999, 'full_name': REPO['full_name']}),
                   ('head_repository', {'id': 123, 'full_name': 'fork/repo'}),
                   ('actor', {**BOT, 'id': 1}), ('actor', {**BOT, 'type': 'User'}),
                   ('actor', {**BOT, 'login': 'someone'}), ('triggering_actor', {'id': 1, 'login': 'owner', 'type': 'User'}),
                   ('run_attempt', 2), ('run_attempt', True), ('pull_requests', None),
                   ('status', 'unknown'), ('conclusion', 'failure'), ('conclusion', 'cancelled')]
        for key, value in changes:
            with self.subTest(key=key, value=value):
                self.f = Fixture()
                self.f.runs[200][key] = value
                with self.assertRaises(policy.Refused):
                    self.start()
                self.assertEqual(self.f.posts, [])

    def test_matching_nonempty_pr_identity_and_each_mismatch(self):
        reference = {'number': 1, 'head': {'sha': SHA, 'ref': self.f.plan['branch'], 'repo': copy.deepcopy(REPO)},
                     'base': {'sha': BASE, 'ref': 'main', 'repo': copy.deepcopy(REPO)}}
        self.f.runs[200]['pull_requests'] = [reference]
        self.wait(self.start())
        for path, value in [(('number',), 2), (('head', 'sha'), BASE),
                            (('base', 'sha'), SHA), (('base', 'ref'), 'other'),
                            (('head', 'repo', 'id'), 456), (('base', 'repo', 'full_name'), 'fork/repo')]:
            with self.subTest(path=path):
                self.f = Fixture()
                pr = copy.deepcopy(reference)
                parent = pr
                for key in path[:-1]:
                    parent = parent[key]
                parent[path[-1]] = value
                self.f.runs[200]['pull_requests'] = [pr]
                with self.assertRaises(policy.Refused):
                    self.start()
                self.assertEqual(self.f.posts, [])

    def test_official_mini_repository_pr_association_binds_id_name_and_url(self):
        mini = {'id': REPO['id'], 'name': REPO['name'], 'url': REPO['url']}
        reference = {'number': 1, 'head': {'sha': SHA, 'ref': self.f.plan['branch'], 'repo': copy.deepcopy(mini)},
                     'base': {'sha': BASE, 'ref': 'main', 'repo': copy.deepcopy(mini)}}
        self.f.runs[200]['pull_requests'] = [reference]
        self.wait(self.start())
        for key, value in [('id', 456), ('name', 'other'), ('url', 'https://api.github.com/repos/fork/repo'),
                           ('url', 'https://foreign/repos/owner/repo'), ('url', REPO['url'] + '/')]:
            with self.subTest(key=key, value=value):
                self.f = Fixture()
                pr = copy.deepcopy(reference)
                pr['head']['repo'][key] = value
                self.f.runs[200]['pull_requests'] = [pr]
                with self.assertRaises(policy.Refused): self.start()
                self.assertEqual(self.f.posts, [])

    def test_missing_workflow_disabled_source_mismatch_and_wrong_registration(self):
        for condition in ('disabled', 'path', 'id', 'source', 'repo'):
            with self.subTest(condition=condition):
                self.f = Fixture()
                if condition == 'disabled': self.f.registered[WORKFLOWS[0]]['state'] = 'disabled_manually'
                if condition == 'path': self.f.registered[WORKFLOWS[0]]['path'] = '.github/workflows/deploy.yml'
                if condition == 'id': self.f.registered[WORKFLOWS[0]]['id'] = True
                if condition == 'source': self.f.workflow_source = 'permissions: write-all'
                if condition == 'repo':
                    original = self.f.api.transport
                    self.f.api.transport = lambda method, path, body: {'id': True, 'full_name': 'owner/repo'} if path == '' else original(method, path, body)
                with self.assertRaises(policy.Refused): self.start()
                self.assertEqual(self.f.posts, [])

    def test_jobs_executed_missing_total_or_truncated_inventory_refuse(self):
        for condition in ('executed', 'hidden', 'truncated_jobs', 'truncated_runs'):
            with self.subTest(condition=condition):
                self.f = Fixture()
                if condition == 'executed': self.f.jobs[200] = [{'id': 1}]
                if condition == 'hidden': self.f.hidden_jobs = True
                if condition == 'truncated_jobs': self.f.job_total = 1
                if condition == 'truncated_runs': self.f.run_total = 2
                with self.assertRaises(policy.Refused): self.start()
                self.assertEqual(self.f.posts, [])

    def test_older_pending_conflict_is_not_hidden_by_latest_success(self):
        self.f.runs[202] = self.f.run(WORKFLOWS[0], 202)
        self.f.success(202)
        with self.assertRaises(policy.Refused): self.start()
        self.assertEqual(self.f.posts, [])

    def test_older_terminal_failure_can_be_superseded_by_success_without_reapproval(self):
        self.f.runs[200].update(conclusion='failure')
        self.f.runs[202] = self.f.run(WORKFLOWS[0], 202)
        self.f.runs[202]['run_attempt'] = 2
        self.f.runs[202]['triggering_actor'] = {'id': 42, 'login': 'owner', 'type': 'User'}
        self.f.success(202)
        self.wait(self.start())
        self.assertEqual([p[1] for p in self.f.posts], ['/actions/runs/201/approve'])

    def test_inventory_recheck_races_refuse_before_post(self):
        for condition in ('main', 'head', 'base', 'actor', 'attempt', 'jobs'):
            with self.subTest(condition=condition):
                self.f = Fixture()
                seen = [0]
                def hook(path):
                    if path.split('?')[0] == '/actions/runs/200':
                        seen[0] += 1
                        if seen[0] == 2:
                            if condition == 'main': self.f.candidate.base = 'c' * 40
                            if condition == 'head': self.f.candidate.head = 'c' * 40
                            if condition == 'base': self.f.candidate.base = 'c' * 40
                            if condition == 'actor': self.f.runs[200]['triggering_actor']['id'] = 1
                            if condition == 'attempt': self.f.runs[200]['run_attempt'] = 2
                            if condition == 'jobs': self.f.jobs[200] = [{'id': 1}]
                self.f.read_hook = hook
                with self.assertRaises(policy.Refused): self.start()
                self.assertEqual(self.f.posts, [])

    def test_permission_denied_and_ambiguous_mutation_are_never_retried(self):
        for error, reason in [(HTTPError('https://api.github.com', 403, 'Forbidden', {}, None), 'permission'),
                              (TimeoutError(), 'unknown')]:
            with self.subTest(error=type(error).__name__):
                self.f = Fixture()
                self.f.post_error = error
                with self.assertRaisesRegex(policy.Refused, reason): self.start()
                self.assertEqual(len(self.f.posts), 1)

    def test_wait_allows_runtime_beyond_35_minutes_but_is_bounded_at_50(self):
        self.f.finish_at = 40 * 60
        self.wait(self.start())
        self.assertEqual(self.f.clock.elapsed, 40 * 60)
        self.f = Fixture()
        self.f.finish_at = 51 * 60
        with self.assertRaisesRegex(policy.Refused, 'Timed out'): self.wait(self.start())
        self.assertEqual(self.f.clock.elapsed, 50 * 60)

    def test_failed_jobs_run_failure_changed_main_head_and_new_conflict_block_wait(self):
        for condition in ('jobs', 'failure', 'main', 'head', 'conflict'):
            with self.subTest(condition=condition):
                self.f = Fixture()
                session = self.start()
                if condition == 'jobs':
                    self.f.success(200)
                    self.f.jobs[200][0]['conclusion'] = 'failure'
                if condition == 'failure': self.f.runs[200].update(status='completed', conclusion='failure')
                if condition == 'main': self.f.candidate.base = 'c' * 40
                if condition == 'head': self.f.candidate.head = 'c' * 40
                if condition == 'conflict':
                    self.f.runs[199] = self.f.run(WORKFLOWS[0], 199)
                with self.assertRaises(policy.Refused): self.wait(session)

    def test_maintenance_fixture_is_optional_until_candidate_matches_paths(self):
        self.wait(self.start())
        self.f = Fixture()
        self.f.plan['files'].append({'path': 'infra/maintenance/bootstrap/Dockerfile.test'})
        with self.assertRaises(policy.Refused): self.start()
        self.assertEqual(self.f.posts, [])

    def test_bot_namespace_cannot_be_replaced_with_emergency_owner_hold(self):
        self.f.plan.update(emergency=True, branch='dependency-security/fix')
        with self.assertRaises(policy.Refused): self.start()
        self.assertEqual(self.f.posts, [])

    def test_candidate_revalidation_race_and_deadline_never_authorize_post(self):
        for condition in ('actor', 'jobs', 'attempt', 'conflict', 'deadline'):
            with self.subTest(condition=condition):
                self.f = Fixture()
                original = self.f.revalidate
                def revalidate():
                    original()
                    if condition == 'actor': self.f.runs[200]['triggering_actor']['id'] = 7
                    if condition == 'jobs': self.f.jobs[200] = [{'id': 1}]
                    if condition == 'attempt': self.f.runs[200]['run_attempt'] = 2
                    if condition == 'conflict': self.f.runs[199] = self.f.run(WORKFLOWS[0], 199)
                    if condition == 'deadline': self.f.clock.elapsed = self.ci.WAIT_SECONDS
                self.f.revalidate = revalidate
                with self.assertRaises(policy.Refused): self.start()
                self.assertEqual(self.f.posts, [])

    def test_other_selected_workflow_race_prevents_first_approval(self):
        for identifier in (201, 202):
            for condition in ('failure', 'cancelled', 'held_owner', 'executed_jobs'):
                with self.subTest(run=identifier, condition=condition):
                    self.f = Fixture()
                    if identifier == 202:
                        self.f.runs[202] = self.f.run(WORKFLOWS[2], 202)
                        self.f.jobs[202] = []
                    original = self.f.revalidate
                    def revalidate():
                        original()
                        if condition in ('failure', 'cancelled'):
                            self.f.runs[identifier]['conclusion'] = condition
                        if condition == 'held_owner':
                            self.f.runs[identifier]['triggering_actor'] = copy.deepcopy(REPO['owner'])
                        if condition == 'executed_jobs':
                            self.f.jobs[identifier] = [{'id': 99}]
                    self.f.revalidate = revalidate
                    with self.assertRaises(policy.Refused): self.start()
                    self.assertEqual(self.f.posts, [])

    def test_required_run_visibility_uses_same_finite_deadline(self):
        missing = self.f.runs.pop(201)
        def propagate(path):
            if self.f.clock.elapsed >= 120: self.f.runs[201] = missing
        self.f.read_hook = propagate
        self.f.finish_at = 40 * 60
        self.wait(self.start())
        self.assertEqual(self.f.clock.elapsed, 40 * 60)
        self.f = Fixture()
        self.f.runs.pop(201)
        with self.assertRaisesRegex(policy.Refused, 'Timed out'): self.start()
        self.assertEqual(self.f.clock.elapsed, 50 * 60)
        self.assertEqual(self.f.posts, [])

    def test_full_pagination_includes_the_latest_run_on_later_page(self):
        self.f.success(200)
        for identifier in range(1000, 1100):
            self.f.runs[identifier] = self.f.run(WORKFLOWS[0], identifier)
            self.f.success(identifier)
        self.f.runs[1099].update(conclusion='action_required')
        self.f.jobs[1099] = []
        self.wait(self.start())
        self.assertEqual([call[1] for call in self.f.posts], ['/actions/runs/1099/approve', '/actions/runs/201/approve'])
        self.assertTrue(any('page=2' in path for _, path, _ in self.f.calls))

    def test_malformed_total_missing_keys_or_duplicate_runs_refuse(self):
        for malformed in ({'workflow_runs': [], 'total_count': True},
                          {'workflow_runs': [], 'total_count': -1},
                          {'workflow_runs': []}, {'total_count': 1},
                          {'workflow_runs': 'not-a-list', 'total_count': 1}):
            with self.subTest(malformed=malformed):
                self.f = Fixture()
                original = self.f.api.transport
                self.f.api.transport = lambda method, path, body: malformed if '/runs?' in path else original(method, path, body)
                with self.assertRaises(policy.Refused): self.start()
                self.assertEqual(self.f.posts, [])
        self.f = Fixture()
        original = self.f.api.transport
        def duplicate(method, path, body):
            data = original(method, path, body)
            if '/actions/workflows/100/runs?' in path:
                data['workflow_runs'] *= 2
                data['total_count'] = 2
            return data
        self.f.api.transport = duplicate
        with self.assertRaisesRegex(policy.Refused, 'Duplicate'): self.start()
        self.assertEqual(self.f.posts, [])

    def test_successful_jobs_must_match_exact_expected_inventory(self):
        for condition in ('missing', 'extra', 'duplicate', 'head', 'run_id', 'run_attempt', 'status', 'id'):
            with self.subTest(condition=condition):
                self.f = Fixture()
                self.f.success(200)
                jobs = self.f.jobs[200]
                if condition == 'missing': jobs.pop()
                if condition == 'extra': jobs.append({**jobs[0], 'id': 999, 'name': 'privileged'})
                if condition == 'duplicate': jobs[1]['id'] = jobs[0]['id']
                if condition == 'head': jobs[0]['head_sha'] = BASE
                if condition == 'run_id': jobs[0]['run_id'] = 999
                if condition == 'run_attempt': jobs[0]['run_attempt'] = 1 if self.f.runs[200]['run_attempt'] == 2 else 2
                if condition == 'status': jobs[0]['status'] = 'in_progress'
                if condition == 'id': jobs[0]['id'] = True
                with self.assertRaises(policy.Refused): self.wait(self.start())

    def test_existing_owner_emergency_ci_success_needs_no_bot_authorization(self):
        self.f.candidate.author = 'enjoydarts'
        self.f.candidate.branch = 'dependency-security/fix'
        self.f.candidate.labels = [{'name': 'dependency-security'}]
        self.f.candidate.body = 'Security fix: patched vulnerable dependency'
        self.f.plan = pipeline.candidate(self.f.api, 1, SHA, BASE, BASE)
        for identifier, run in self.f.runs.items():
            run.update(head_branch=self.f.plan['branch'], actor={'id': 42, 'login': 'enjoydarts', 'type': 'User'},
                       triggering_actor={'id': 42, 'login': 'enjoydarts', 'type': 'User'})
            self.f.success(identifier)
        self.wait(self.start())
        self.assertEqual(self.f.posts, [])
        self.f.runs[200]['actor']['id'] = 999
        with self.assertRaises(policy.Refused): self.start()
        self.assertEqual(self.f.posts, [])
        self.f.runs[200]['actor']['id'] = 42
        self.f.runs[200].update(conclusion='action_required')
        self.f.jobs[200] = []
        with self.assertRaises(policy.Refused): self.start()
        self.assertEqual(self.f.posts, [])

    def test_candidate_repository_id_changes_before_approval_refuse(self):
        for side in ('head', 'base'):
            with self.subTest(side=side):
                self.f = Fixture()
                original = self.f.api.transport
                def transport(method, path, body):
                    data = original(method, path, body)
                    if path == '/pulls/1': data[side]['repo']['id'] = 999
                    return data
                self.f.api.transport = transport
                with self.assertRaises(policy.Refused): self.start()
                self.assertEqual(self.f.posts, [])


class PrepareTests(unittest.TestCase):
    def setUp(self):
        self.flag = patch.dict(os.environ, {'DEPENDENCY_PR_CI_AUTO_APPROVAL': 'true'})
        self.flag.start()
        self.addCleanup(self.flag.stop)
        self.assertIsNotNone(importlib.util.find_spec('dependency_updates.pr_ci'))

    def test_pr_approval_starts_before_fixed_dispatch_and_both_checks_gate_output(self):
        from dependency_updates import pr_ci
        for fixed_fails, pr_fails in ((False, False), (True, False), (False, True)):
            with self.subTest(fixed_fails=fixed_fails, pr_fails=pr_fails):
                f = Fixture()
                events = []
                real_start = pr_ci.start
                def start(*args, **kwargs):
                    events.append('approve-pr')
                    session = real_start(*args, clock=f.clock.monotonic, sleep=f.clock.sleep, **kwargs)
                    real_wait = session.wait
                    def wait():
                        events.append('verify-pr')
                        if pr_fails: raise policy.Refused('PR CI failed')
                        return real_wait(clock=f.clock.monotonic, sleep=f.clock.sleep)
                    session.wait = wait
                    return session
                def dispatch(*args, **kwargs):
                    events.append('fixed-dispatch')
                    return {'id': 15, 'event': 'workflow_dispatch', 'head_sha': SHA,
                            'path': '.github/workflows/dependency-check.yml',
                            'status': 'completed', 'conclusion': 'failure' if fixed_fails else 'success'}
                original = f.api.transport
                def transport(method, path, body):
                    if path.startswith('/actions/runs/15/jobs'):
                        return {'total_count': 4, 'jobs': [{'name': name, 'head_sha': SHA, 'conclusion': 'success'} for name in policy.CHECKS]}
                    return original(method, path, body)
                f.api.transport = transport
                with tempfile.TemporaryDirectory() as directory, patch.dict(os.environ, {
                    'GITHUB_RUN_ATTEMPT': '1', 'GITHUB_OUTPUT': directory + '/output',
                    'GITHUB_STEP_SUMMARY': directory + '/summary'}), patch.object(pr_ci, 'start', side_effect=start), patch.object(pipeline, 'dispatch_once', side_effect=dispatch):
                    if fixed_fails or pr_fails:
                        with self.assertRaises(policy.Refused): pipeline.prepare(f.api, 1, SHA, BASE, BASE)
                        self.assertFalse(Path(directory + '/output').exists())
                    else:
                        pipeline.prepare(f.api, 1, SHA, BASE, BASE)
                        encoded = Path(directory + '/output').read_text().splitlines()[0].split('=', 1)[1]
                        self.assertEqual(pipeline.json_plan(encoded)['ci_run'], 15)
                    self.assertEqual(events[:2], ['approve-pr', 'fixed-dispatch'])

    def test_fixed_ci_consumes_remaining_pr_deadline_without_owner_output(self):
        from dependency_updates import pr_ci
        f = Fixture()
        real_start = pr_ci.start
        def start(*args, **kwargs):
            session = real_start(*args, clock=f.clock.monotonic, sleep=f.clock.sleep, **kwargs)
            f.clock.elapsed = 20 * 60
            session.wait = lambda: pr_ci.Session.wait(session, clock=f.clock.monotonic, sleep=f.clock.sleep)
            return session
        def dispatch(*args, **kwargs):
            self.assertEqual(kwargs['remaining_budget'](), 30 * 60)
            f.clock.elapsed = 51 * 60
            return {'id': 15, 'event': 'workflow_dispatch', 'head_sha': SHA,
                    'path': '.github/workflows/dependency-check.yml', 'status': 'completed', 'conclusion': 'success'}
        original = f.api.transport
        def transport(method, path, body):
            if path.startswith('/actions/runs/15/jobs'):
                return {'jobs': [{'name': name, 'head_sha': SHA, 'conclusion': 'success'} for name in policy.CHECKS], 'total_count': 4}
            return original(method, path, body)
        f.api.transport = transport
        with tempfile.TemporaryDirectory() as directory, patch.dict(os.environ, {
            'GITHUB_RUN_ATTEMPT': '1', 'GITHUB_OUTPUT': directory + '/output',
            'GITHUB_STEP_SUMMARY': directory + '/summary'}), patch.object(pr_ci, 'start', side_effect=start), patch.object(pipeline, 'dispatch_once', side_effect=dispatch):
            with self.assertRaisesRegex(policy.Refused, 'Timed out'): pipeline.prepare(f.api, 1, SHA, BASE, BASE)
            self.assertFalse(Path(directory + '/output').exists())

    def test_disabled_hold_never_dispatches_fixed_ci_or_outputs_owner_plan(self):
        from dependency_updates import pr_ci
        f = Fixture()
        real_start = pr_ci.start
        def start(*args, **kwargs):
            return real_start(*args, clock=f.clock.monotonic, sleep=f.clock.sleep, **kwargs)
        with tempfile.TemporaryDirectory() as directory, patch.dict(os.environ, {
            'GITHUB_RUN_ATTEMPT': '1', 'DEPENDENCY_PR_CI_AUTO_APPROVAL': '',
            'GITHUB_OUTPUT': directory + '/output', 'GITHUB_STEP_SUMMARY': directory + '/summary'}), \
            patch.object(pr_ci, 'start', side_effect=start), patch.object(pipeline, 'dispatch_once') as dispatch:
            with self.assertRaisesRegex(policy.Refused, 'disabled'):
                pipeline.prepare(f.api, 1, SHA, BASE, BASE)
            dispatch.assert_not_called()
            self.assertFalse(Path(directory + '/output').exists())
            self.assertEqual(f.posts, [])


if __name__ == '__main__':
    unittest.main()
