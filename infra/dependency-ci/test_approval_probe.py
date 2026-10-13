"""Offline diagnostic lifecycle proof; never evidence of live token capability."""
import base64
import importlib.util
import os
import subprocess
from pathlib import Path
import tempfile
import unittest
from unittest.mock import patch, Mock
from urllib.error import HTTPError

from dependency_updates import github, pipeline, policy
from test_pr_ci import Fixture, BOT, REPO, WORKFLOWS
from test_updates import BASE, SHA

TREE = 'c' * 40
SOURCE_TREE = 'd' * 40
BLOB = 'e' * 40
MOD = 'module example.com/api\n\ngo 1.24.0\n\nrequire example.com/lib v1.0.0\n'


class ProbeFixture(Fixture):
    def __init__(self):
        super().__init__()
        self.candidate.paths = ['api/go.mod']
        self.candidate.branch = 'renovate-ci/approval-probe-10'
        self.candidate.body = 'Sifto CI approval probe run=10 source=' + BASE + ' head=' + SHA
        self.created_ref = False
        self.created_pr = False
        self.closed = False
        self.published = False
        self.after = MOD + '\n'
        self.source = BASE
        self.mutate_hook = None
        self.git = Mock(return_value=Mock(returncode=0))
        self.output = []
        for run in self.runs.values(): run['head_branch'] = self.candidate.branch
        def delete(*args, **kwargs):
            self.created_ref = False
            return Mock(returncode=0)
        self.git.side_effect = delete

    def transport(self, method, path, body):
        endpoint = path.split('?')[0]
        if method != 'GET' and (endpoint.startswith('/git/') or endpoint == '/pulls'
                                or endpoint == '/pulls/1' or endpoint.startswith('/statuses/')):
            self.calls.append((method, path, body))
            if self.mutate_hook: self.mutate_hook(method, path, body)
            if endpoint == '/git/blobs': return {'sha': BLOB}
            if endpoint == '/git/trees': return {'sha': TREE}
            if endpoint == '/git/commits': return {'sha': SHA}
            if endpoint == '/git/refs':
                self.created_ref = True
                return {'ref': body['ref'], 'object': {'sha': SHA}}
            if endpoint == '/pulls':
                self.created_pr = True
                return {'number': 1}
            if endpoint == '/pulls/1':
                self.closed = True
                return self.transport('GET', '/pulls/1', None)
            if endpoint.startswith('/statuses/'):
                self.published = True
                return {'state': 'success'}
        if endpoint == '/git/ref/heads/main':
            self.calls.append((method, path, body))
            return {'object': {'sha': self.source}}
        if endpoint.startswith('/git/ref/heads/renovate-ci/'):
            self.calls.append((method, path, body))
            if not self.created_ref:
                raise HTTPError('public', 404, 'not found', {}, None)
            return {'ref': 'refs/heads/' + self.candidate.branch, 'object': {'sha': self.candidate.head}}
        if endpoint.startswith('/git/commits/'):
            self.calls.append((method, path, body))
            identifier = endpoint.split('/')[-1]
            return {'sha': identifier, 'tree': {'sha': TREE if identifier == SHA else SOURCE_TREE},
                    'parents': [{'sha': BASE}] if identifier == SHA else []}
        if endpoint == '/actions/runs/10':
            self.calls.append((method, path, body))
            return {'id': 10, 'event': 'workflow_dispatch', 'head_branch': 'main', 'head_sha': BASE,
                    'path': '.github/workflows/renovate.yml', 'run_attempt': 1,
                    'repository': REPO, 'head_repository': REPO}
        if endpoint == '/pulls' and method == 'GET':
            self.calls.append((method, path, body))
            return [self.transport('GET', '/pulls/1', None)] if self.created_pr else []
        if endpoint == '/pulls/1':
            value = super().transport(method, path, body)
            value.update(number=1, state='closed' if self.closed else 'open')
            return value
        if endpoint.startswith('/compare/'):
            self.calls.append((method, path, body))
            return {'merge_base_commit': {'sha': BASE}, 'status': 'ahead', 'ahead_by': 1,
                    'total_commits': 1, 'commits': [{'sha': SHA}],
                    'files': [{'filename': 'api/go.mod', 'status': 'modified', 'additions': 1, 'deletions': 0}]}
        if endpoint.startswith('/contents/api/'):
            self.calls.append((method, path, body))
            text = 'sum\n' if endpoint.endswith('go.sum') else MOD if 'ref=' + BASE in path else self.after
            return {'type': 'file', 'encoding': 'base64', 'size': len(text),
                    'content': base64.b64encode(text.encode()).decode()}
        if endpoint == '/pulls/1/files':
            self.calls.append((method, path, body))
            return [{'filename': 'api/go.mod', 'status': 'modified', 'additions': 1, 'deletions': 0}]
        if endpoint.endswith('/statuses'):
            self.calls.append((method, path, body))
            return [{'context': 'renovate/stability-days', 'state': 'success', 'creator': BOT}] if self.published else []
        return super().transport(method, path, body)


class ProbeTests(unittest.TestCase):
    def setUp(self):
        self.assertIsNotNone(importlib.util.find_spec('dependency_updates.approval_probe'),
                             'focused diagnostic controller must exist')
        from dependency_updates import approval_probe
        self.probe = approval_probe
        self.assertTrue(callable(getattr(self.probe, 'propose', None)) and callable(getattr(self.probe, 'prove', None)),
                        'Writer creation and ordinary-scope token proof must be separate')
        self.f = ProbeFixture()
        self.directory = tempfile.TemporaryDirectory()
        self.addCleanup(self.directory.cleanup)
        self.env = patch.dict(os.environ, {'GITHUB_REPOSITORY': 'owner/repo', 'GITHUB_REF': 'refs/heads/main',
            'GITHUB_SHA': BASE, 'GITHUB_RUN_ID': '10', 'GITHUB_RUN_ATTEMPT': '1',
            'GITHUB_EVENT_NAME': 'workflow_dispatch', 'GITHUB_OUTPUT': self.directory.name + '/output'})
        self.env.start()
        self.addCleanup(self.env.stop)

    def run_probe(self):
        def dispatch(api, workflow, branch, head, inputs, remaining_budget=None):
            self.assertEqual((workflow, branch, head, inputs),
                             ('dependency-check.yml', self.f.candidate.branch, SHA, {'expected_head': SHA}))
            remaining_budget()
            self.f.calls.append(('POST', '/actions/workflows/dependency-check.yml/dispatches', inputs))
            return {'id': 15, 'event': 'workflow_dispatch', 'head_sha': SHA,
                    'path': '.github/workflows/dependency-check.yml', 'status': 'completed', 'conclusion': 'success'}
        original = self.f.api.transport
        def transport(method, path, body):
            if path.startswith('/actions/runs/15/jobs'):
                return {'jobs': [{'name': name, 'head_sha': SHA, 'conclusion': 'success'} for name in policy.CHECKS], 'total_count': 4}
            return original(method, path, body)
        self.f.api.transport = transport
        with patch.object(pipeline, 'dispatch_once', side_effect=dispatch):
            record = self.probe.propose(self.f.api)
            return self.probe.prove(self.f.api, record, clock=self.f.clock.monotonic, sleep=self.f.clock.sleep)

    def record(self):
        return {'run': 10, 'source': BASE, 'branch': self.f.candidate.branch,
                'head': SHA, 'tree': TREE, 'pr': 1}

    def test_complete_noop_proof_records_runs_before_each_approval(self):
        def check_output(method, path, body):
            if path == '/git/refs':
                output = Path(os.environ['GITHUB_OUTPUT']).read_text()
                self.assertIn('head=' + SHA, output)
                self.assertIn('source=' + BASE, output)
                self.assertIn('branch=' + self.f.candidate.branch, output)
        self.f.mutate_hook = check_output
        original = self.f.api.transport
        def transport(method, path, body):
            if path.endswith('/approve'):
                output = Path(os.environ['GITHUB_OUTPUT']).read_text()
                self.assertIn('readonly_ids={"app-runtime.yml":{"acknowledged":false,"id":201},"dependency-check.yml":{"acknowledged":false,"id":200}}', output)
                self.assertIn('pr=1', output)
            return original(method, path, body)
        self.f.api.transport = transport
        self.run_probe()
        self.assertIn('proof=success', Path(os.environ['GITHUB_OUTPUT']).read_text())
        posts = [path for method, path, _ in self.f.calls if method == 'POST']
        self.assertEqual(posts.count('/git/refs'), 1)
        self.assertEqual(posts.count('/pulls'), 1)
        self.assertEqual(posts.count('/actions/runs/200/approve'), 1)
        self.assertEqual(posts.count('/actions/runs/201/approve'), 1)
        self.assertFalse(any('deploy' in path or 'dependency-update' in path for path in posts))

    def test_ack_is_output_before_transition_reads_and_survives_second_failure(self):
        original = self.f.api.transport
        def transport(method, path, body):
            if method == 'GET' and self.f.initial_attempts:
                output = Path(os.environ['GITHUB_OUTPUT']).read_text()
                self.assertIn('"dependency-check.yml":{"acknowledged":true,"id":200}', output)
            if path == '/actions/runs/201/approve': raise TimeoutError()
            return original(method, path, body)
        self.f.api.transport = transport
        with self.assertRaisesRegex(policy.Refused, 'unknown'): self.run_probe()
        output = Path(os.environ['GITHUB_OUTPUT']).read_text()
        self.assertNotIn('"app-runtime.yml":{"acknowledged":true', output)
        self.assertNotIn('proof=success', output)

    def test_cleanup_acknowledged_second_attempt_cancels_only_owned_current_jobs(self):
        self.f.created_ref = self.f.created_pr = True
        self.f.initial_attempts[200] = self.f.run(WORKFLOWS[0], 200)
        self.f.initial_attempts[200]['head_branch'] = self.f.candidate.branch
        self.f.runs[200].update(run_attempt=2, status='queued', conclusion=None)
        self.f.finish_at = 100
        self.f.jobs[200] = [{'id': 900, 'run_id': 200, 'run_attempt': 2, 'head_sha': SHA,
                             'name': 'Dependency API', 'status': 'queued', 'conclusion': None}]
        original = self.f.api.transport
        def cancel(method, path, body):
            if path == '/actions/runs/200/cancel' and method == 'POST':
                self.f.calls.append((method, path, body))
                return None
            return original(method, path, body)
        self.f.api.transport = cancel
        self.probe.cleanup(self.f.api, self.record(), {'dependency-check.yml': {'id': 200, 'acknowledged': True}}, git=self.f.git)
        self.assertEqual([p for m, p, _ in self.f.calls if m == 'POST'], ['/actions/runs/200/cancel'])

    def test_cleanup_never_accepts_unacknowledged_rerun_or_foreign_second_attempt(self):
        for condition in ('unacknowledged', 'third', 'trigger', 'initial_actor', 'job_attempt', 'history_missing', 'initial_jobs'):
            with self.subTest(condition=condition):
                self.f = ProbeFixture()
                self.f.created_ref = self.f.created_pr = True
                self.f.initial_attempts[200] = self.f.run(WORKFLOWS[0], 200)
                self.f.initial_attempts[200]['head_branch'] = self.f.candidate.branch
                self.f.runs[200].update(run_attempt=2, status='queued', conclusion=None)
                self.f.finish_at = 100
                if condition == 'third': self.f.runs[200]['run_attempt'] = 3
                if condition == 'trigger': self.f.runs[200]['triggering_actor'] = REPO['owner']
                if condition == 'initial_actor': self.f.initial_attempts[200]['actor'] = REPO['owner']
                if condition == 'job_attempt':
                    self.f.jobs[200] = [{'id': 1, 'run_id': 200, 'run_attempt': 1, 'name': 'Dependency API', 'head_sha': SHA}]
                original = self.f.api.transport
                def missing_history(method, path, body):
                    if condition == 'history_missing' and path == '/actions/runs/200/attempts/1':
                        raise HTTPError('public', 404, 'missing', {}, None)
                    if condition == 'initial_jobs' and '/attempts/1/jobs' in path:
                        return {'jobs': [{'id': 9}], 'total_count': 1}
                    return original(method, path, body)
                self.f.api.transport = missing_history
                with self.assertRaises(policy.Refused):
                    self.probe.cleanup(self.f.api, self.record(), {'dependency-check.yml': {'id': 200, 'acknowledged': condition != 'unacknowledged'}}, git=self.f.git)
                self.assertEqual(self.f.posts, [])
                self.f.git.assert_not_called()

    def test_source_and_first_attempt_invariants_refuse_before_mutation(self):
        for key, value in [('GITHUB_REF', 'refs/heads/other'), ('GITHUB_RUN_ATTEMPT', '2'),
                           ('GITHUB_RUN_ID', '0'), ('GITHUB_RUN_ID', '10\npr=999'),
                           ('GITHUB_SHA', SHA), ('GITHUB_EVENT_NAME', 'schedule')]:
            with self.subTest(key=key), patch.dict(os.environ, {key: value}), self.assertRaises(policy.Refused):
                self.run_probe()
            self.assertEqual(self.f.posts, [])

    def test_live_dispatch_identity_cannot_be_forged(self):
        for key, value in [('run_attempt', True), ('run_attempt', 2), ('event', 'pull_request'),
                           ('path', '.github/workflows/dependency-update.yml'),
                           ('head_branch', 'other'), ('head_sha', SHA), ('repository', {'id': 999, 'full_name': 'owner/repo'})]:
            self.f = ProbeFixture()
            original = self.f.api.transport
            def altered(method, path, body):
                result = original(method, path, body)
                if path == '/actions/runs/10': result[key] = value
                return result
            self.f.api.transport = altered
            with self.subTest(key=key, value=value), self.assertRaises(policy.Refused): self.run_probe()
            self.assertEqual(self.f.posts, [])

    def test_proof_requires_recorded_pr_and_current_source(self):
        self.f.created_ref = self.f.created_pr = self.f.published = True
        record = self.record()
        record['pr'] = None
        with self.assertRaisesRegex(policy.Refused, 'PR'):
            self.probe.prove(self.f.api, record, clock=self.f.clock.monotonic, sleep=self.f.clock.sleep)
        self.assertEqual(self.f.posts, [])

    def test_moved_fixture_branch_ref_prevents_proof(self):
        self.f.created_ref = self.f.created_pr = self.f.published = True
        original = self.f.api.transport
        def moved(method, path, body):
            value = original(method, path, body)
            if path.startswith('/git/ref/heads/renovate-ci/'): value['object']['sha'] = 'f' * 40
            return value
        self.f.api.transport = moved
        with patch.object(pipeline, 'dispatch_once', side_effect=AssertionError('Moved ref must not reach dispatch')), \
                self.assertRaisesRegex(policy.Refused, 'ref'):
            self.probe.prove(self.f.api, self.record(), clock=self.f.clock.monotonic, sleep=self.f.clock.sleep)
        self.assertEqual(self.f.posts, [])

    def test_duplicate_fixture_branch_never_replays_creation(self):
        self.f.created_ref = True
        with self.assertRaisesRegex(policy.Refused, 'already exists'): self.run_probe()
        self.assertEqual(self.f.posts, [])

    def test_noop_status_published_once_only_with_verified_fixture_description(self):
        self.run_probe()
        statuses = [body for method, path, body in self.f.calls if method == 'POST' and path.startswith('/statuses/')]
        self.assertEqual(statuses, [{'state': 'success', 'context': 'renovate/stability-days',
                                    'description': 'Diagnostic fixture: no dependency releases changed'}])

    def test_missing_initial_run_uses_shared_fifty_minute_deadline(self):
        self.f.runs.pop(201)
        with self.assertRaisesRegex(policy.Refused, 'Timed out'): self.run_probe()
        self.assertEqual(self.f.clock.elapsed, 50 * 60)
        self.assertFalse(any(path.endswith('/approve') for _, path, _ in self.f.calls))

    def test_source_advance_during_full_candidate_callback_prevents_approval(self):
        reads = 0
        def advance(path):
            nonlocal reads
            if path == '/pulls/1':
                reads += 1
                if reads >= 5: self.f.source = 'f' * 40
        self.f.read_hook = advance
        with self.assertRaises(policy.Refused): self.run_probe()
        self.assertFalse(any(path.endswith('/approve') for _, path, _ in self.f.calls))

    def test_wrong_successful_pr_job_or_elapsed_dispatch_never_claims_proof(self):
        for condition in ('jobs', 'deadline'):
            self.f = ProbeFixture()
            original = self.f.success
            def bad_success(identifier):
                original(identifier)
                if condition == 'jobs': self.f.jobs[identifier][0]['name'] = 'Unexpected writer'
                else: self.f.clock.elapsed = 51 * 60
            self.f.success = bad_success
            with self.subTest(condition=condition), self.assertRaises(policy.Refused): self.run_probe()
            self.assertNotIn('proof=success', Path(os.environ['GITHUB_OUTPUT']).read_text())

    def test_initial_success_rerun_or_manual_approval_is_inconclusive(self):
        for condition in ('success', 'rerun', 'owner', 'jobs'):
            self.f = ProbeFixture()
            if condition == 'success': self.f.success(200)
            if condition == 'rerun': self.f.runs[200]['run_attempt'] = 2
            if condition == 'owner': self.f.runs[200]['triggering_actor'] = REPO['owner']
            if condition == 'jobs': self.f.jobs[200] = [{'id': 99}]
            with self.subTest(condition=condition), self.assertRaises(policy.Refused): self.run_probe()
            self.assertFalse(any(path.endswith('/approve') for _, path, _ in self.f.calls))

    def test_previous_completed_run_cannot_be_superseded_to_prove_initial_hold(self):
        self.f.runs[199] = self.f.run(WORKFLOWS[0], 199)
        self.f.runs[199]['head_branch'] = self.f.candidate.branch
        self.f.success(199)
        with self.assertRaisesRegex(policy.Refused, 'initial'):
            self.run_probe()
        self.assertFalse(any(path.endswith('/approve') for _, path, _ in self.f.calls))

    def test_real_http403_or_lost_approval_ack_never_replays_or_claims_success(self):
        for error in (HTTPError('public', 403, 'forbidden', {}, None), TimeoutError()):
            self.f = ProbeFixture()
            self.f.post_error = error
            with self.subTest(error=type(error).__name__), self.assertRaises(policy.Refused): self.run_probe()
            self.assertEqual(len([path for _, path, _ in self.f.calls if path.endswith('/approve')]), 1)
            self.assertNotIn('proof=success', Path(os.environ['GITHUB_OUTPUT']).read_text())

    def test_semantic_or_extra_byte_changes_never_publish_age_or_approve(self):
        for after in (MOD.replace('v1.0.0', 'v1.0.1') + '\n', MOD + '\n\n', MOD + 'replace example.com/lib => ../lib\n'):
            self.f = ProbeFixture()
            self.f.after = after
            with self.subTest(after=after), self.assertRaises(policy.Refused): self.run_probe()
            self.assertFalse(self.f.published)
            self.assertFalse(any(path.endswith('/approve') for _, path, _ in self.f.calls))

    def test_fixture_tree_parent_comparison_and_pr_tampering_refuse(self):
        changes = [('/git/commits/' + SHA, 'tree', {'sha': SOURCE_TREE}),
                   ('/git/commits/' + SHA, 'parents', [{'sha': SHA}]),
                   ('/compare/' + BASE + '...' + SHA, 'ahead_by', 2),
                   ('/compare/' + BASE + '...' + SHA, 'files', [{'filename': 'api/go.sum'}]),
                   ('/pulls/1', 'body', 'unowned'), ('/pulls/1', 'user', REPO['owner'])]
        for endpoint, key, value in changes:
            self.f = ProbeFixture()
            original = self.f.api.transport
            def tamper(method, path, body):
                result = original(method, path, body)
                if method == 'GET' and path == endpoint: result[key] = value
                return result
            self.f.api.transport = tamper
            with self.subTest(endpoint=endpoint, key=key), self.assertRaises(policy.Refused): self.run_probe()
            self.assertFalse(self.f.published)

    def test_ambiguous_creation_is_not_replayed_or_promoted_to_proof(self):
        for stage in ('/git/blobs', '/git/trees', '/git/commits', '/git/refs', '/pulls'):
            self.f = ProbeFixture()
            def lose(method, path, body):
                if path == stage: raise TimeoutError()
            self.f.mutate_hook = lose
            with self.subTest(stage=stage), self.assertRaises(policy.Refused): self.run_probe()
            self.assertEqual(len([p for m, p, _ in self.f.calls if m == 'POST' and p == stage]), 1)
            self.assertFalse(self.f.published)

    def test_cleanup_closes_owned_pr_after_main_advance_and_deletes_with_exact_lease(self):
        self.f.created_ref = self.f.created_pr = True
        self.f.source = 'f' * 40
        original = self.f.api.transport
        def advanced(method, path, body):
            value = original(method, path, body)
            if method == 'GET' and path == '/pulls/1':
                value['base']['sha'] = self.f.source
                value['changed_files'] = 2
            if method == 'GET' and path.startswith('/pulls/1/files'):
                return [{'filename': 'api/go.mod'}, {'filename': 'web/new-main-file'}]
            return value
        self.f.api.transport = advanced
        self.probe.cleanup(self.f.api, self.record(), {}, git=self.f.git)
        self.assertTrue(self.f.closed)
        argv = self.f.git.call_args.args[0]
        self.assertIn('--force-with-lease=refs/heads/' + self.f.candidate.branch + ':' + SHA, argv)
        self.assertIn('credential.helper=!gh auth git-credential', argv)
        self.assertIn('core.hooksPath=/dev/null', argv)
        self.assertIn('https://github.com/owner/repo.git', argv)
        self.assertIn(':refs/heads/' + self.f.candidate.branch, argv)
        self.assertFalse(any(m == 'DELETE' for m, _, _ in self.f.calls))

    def test_cleanup_unknown_git_result_reads_back_without_replaying_push(self):
        for deleted in (True, False):
            self.f = ProbeFixture()
            self.f.created_ref = self.f.created_pr = True
            def lose(*args, **kwargs):
                if deleted: self.f.created_ref = False
                raise subprocess.TimeoutExpired('git', 60)
            self.f.git.side_effect = lose
            with self.subTest(deleted=deleted):
                if deleted:
                    self.probe.cleanup(self.f.api, self.record(), {}, git=self.f.git)
                else:
                    with self.assertRaisesRegex(policy.Refused, 'retained'):
                        self.probe.cleanup(self.f.api, self.record(), {}, git=self.f.git)
                self.f.git.assert_called_once()

    def test_cleanup_git_success_requires_branch_absence_readback(self):
        self.f.created_ref = self.f.created_pr = True
        self.f.git.side_effect = None
        with self.assertRaisesRegex(policy.Refused, 'retained'):
            self.probe.cleanup(self.f.api, self.record(), {}, git=self.f.git)
        self.f.git.assert_called_once()

    def test_cleanup_wrong_owner_marker_or_changed_ref_retains_artifact(self):
        for condition in ('owner', 'marker', 'ref', 'branch'):
            self.f = ProbeFixture()
            self.f.created_ref = self.f.created_pr = True
            record = self.record()
            if condition == 'owner': self.f.candidate.author = 'enjoydarts'
            if condition == 'marker': self.f.candidate.body = 'unowned'
            if condition == 'ref': self.f.candidate.head = 'f' * 40
            if condition == 'branch': record['branch'] = 'renovate-ci/approval-probe-11'
            with self.subTest(condition=condition), self.assertRaises(policy.Refused):
                self.probe.cleanup(self.f.api, record, {}, git=self.f.git)
            self.assertFalse(self.f.closed)
            self.f.git.assert_not_called()

    def test_cleanup_recovers_unknown_pr_creation_only_with_exact_marker(self):
        self.f.created_ref = self.f.created_pr = True
        record = self.record()
        record['pr'] = None
        self.probe.cleanup(self.f.api, record, {}, git=self.f.git)
        self.assertTrue(self.f.closed)
        self.assertFalse(self.f.published)

    def test_cleanup_only_cancels_recorded_verified_readonly_runs(self):
        self.f.created_ref = self.f.created_pr = True
        self.f.runs[200].update(status='queued', conclusion=None)
        self.f.finish_at = 100
        original = self.f.api.transport
        def cancel(method, path, body):
            if path == '/actions/runs/200/cancel' and method == 'POST':
                self.f.calls.append((method, path, body))
                self.f.runs[200].update(status='completed', conclusion='cancelled')
                return None
            return original(method, path, body)
        self.f.api.transport = cancel
        self.probe.cleanup(self.f.api, self.record(), {'dependency-check.yml': 200}, git=self.f.git)
        self.assertEqual([p for m, p, _ in self.f.calls if m == 'POST'], ['/actions/runs/200/cancel'])


if __name__ == '__main__':
    unittest.main()
