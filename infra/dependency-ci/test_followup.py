"""Completed native dependency approval is the only follow-up authority."""
import copy
import importlib.util
from itertools import product
import json
import os
from pathlib import Path
import tempfile
import unittest
import urllib.error
from unittest.mock import patch

from dependency_updates.github import GitHub
from dependency_updates.policy import Refused

SPEC = importlib.util.find_spec('dependency_updates.followup')
if SPEC:
    from dependency_updates import followup

BASE, HEAD, MERGED = 'a' * 40, 'b' * 40, 'c' * 40
REPO = {'id': 123, 'full_name': 'owner/repo'}
BOT = {'id': 41898282, 'login': 'github-actions[bot]', 'type': 'Bot'}


class CompletedFollowupTests(unittest.TestCase):
    def setUp(self):
        self.assertIsNotNone(SPEC, 'Completed-parent follow-up controller must exist')
        self.context = {'GITHUB_REPOSITORY': REPO['full_name'], 'GITHUB_REPOSITORY_ID': '123',
                        'GITHUB_EVENT_NAME': 'workflow_run', 'GITHUB_REF': 'refs/heads/main',
                        'GITHUB_SHA': MERGED, 'GITHUB_WORKFLOW_SHA': MERGED,
                        'GITHUB_WORKFLOW_REF': 'owner/repo/.github/workflows/dependency-followup.yml@refs/heads/main',
                        'GITHUB_RUN_ID': '30', 'GITHUB_RUN_ATTEMPT': '1'}
        self.parent_workflow = {'id': 20, 'name': 'Dependency Approve Merge and Deploy',
                                'path': '.github/workflows/dependency-update.yml', 'state': 'active'}
        self.self_workflow = {'id': 21, 'name': 'Dependency Completed Follow-up',
                              'path': '.github/workflows/dependency-followup.yml', 'state': 'active'}
        self.parent = {'id': 10, 'workflow_id': 20, 'name': 'Dependency PR 1 ' + HEAD + ' ' + BASE,
                       'display_title': 'Dependency PR 1 ' + HEAD + ' ' + BASE,
                       'path': self.parent_workflow['path'], 'event': 'workflow_dispatch',
                       'head_branch': 'main', 'head_sha': BASE, 'run_attempt': 1,
                       'status': 'completed', 'conclusion': 'success', 'repository': REPO,
                       'head_repository': REPO, 'actor': BOT, 'triggering_actor': BOT}
        self.current = dict(self.parent, id=30, workflow_id=21, name='Dependency Completed Follow-up',
                            display_title='Dependency follow-up 10', path=self.self_workflow['path'],
                            event='workflow_run', head_sha=MERGED, status='in_progress', conclusion=None)
        self.event = {'action': 'completed', 'repository': copy.deepcopy(REPO),
                      'workflow_run': copy.deepcopy(self.parent)}
        self.jobs = [{'id': 100 + index, 'name': name, 'run_id': 10, 'run_attempt': 1,
                      'head_sha': BASE, 'status': 'completed', 'conclusion': 'success'}
                     for index, name in enumerate(['prepare', 'approve-merge-deploy'])]
        self.approvals = [{'state': 'approved', 'user': {'login': 'enjoydarts'},
                           'environments': [{'name': 'production-maintenance'}]}]
        self.pr = {'number': 1, 'state': 'closed', 'merged': True, 'merge_commit_sha': MERGED,
                   'user': BOT, 'head': {'sha': HEAD, 'ref': 'renovate-ci/pip', 'repo': REPO},
                   'base': {'sha': MERGED, 'ref': 'main', 'repo': REPO}, 'labels': [], 'body': ''}
        self.commit = {'sha': MERGED, 'parents': [{'sha': BASE}, {'sha': HEAD}]}
        self.main, self.protected = MERGED, True
        self.history = [self.current]
        self.mutations, self.reads = [], []
        self.hook = None
        self.error = None
        self.api = GitHub(REPO['full_name'], self.transport)

    def transport(self, method, path, body):
        if method != 'GET':
            self.mutations.append((method, path, body))
            if self.error:
                raise self.error
            return None
        self.reads.append(path)
        if self.hook:
            self.hook(path)
        endpoint = path.split('?')[0]
        routes = {'': REPO, '/actions/workflows/dependency-update.yml': self.parent_workflow,
                  '/actions/workflows/dependency-followup.yml': self.self_workflow,
                  '/actions/runs/10': self.parent, '/actions/runs/' + str(self.current['id']): self.current,
                  '/actions/runs/10/approvals': self.approvals,
                  '/actions/runs/10/jobs': {'jobs': self.jobs, 'total_count': len(self.jobs)},
                  '/actions/workflows/dependency-followup.yml/runs': {'workflow_runs': self.history, 'total_count': len(self.history)},
                  '/pulls/1': self.pr, '/git/commits/' + MERGED: self.commit,
                  '/branches/main': {'name': 'main', 'protected': self.protected, 'commit': {'sha': self.main}},
                  '/git/ref/heads/main': {'object': {'sha': self.main}}}
        self.assertIn(endpoint, routes, 'Unexpected API request')
        return copy.deepcopy(routes[endpoint])

    def notify(self):
        return followup.notify(self.api, self.event, self.context)

    def refused(self):
        with self.assertRaises(Refused):
            self.notify()
        self.assertEqual(self.mutations, [])

    def test_completed_success_requests_only_fresh_main_renovate(self):
        self.assertEqual(self.notify(), 'requested')
        self.assertEqual(self.mutations, [('POST', '/actions/workflows/renovate.yml/dispatches', {'ref': 'main'})])
        self.assertEqual(self.reads.count('/actions/runs/10'), 2)
        self.assertEqual(self.reads.count('/actions/runs/10/approvals'), 2)
        self.assertEqual(sum(path.startswith('/actions/runs/10/jobs?filter=all') for path in self.reads), 2)

    def test_parent_event_and_self_accept_only_plain_or_documented_main_qualified_run_paths(self):
        for parent_suffix, event_suffix, self_suffix in product(['', '@main'], repeat=3):
            with self.subTest(parent=parent_suffix, event=event_suffix, child=self_suffix):
                self.setUp()
                self.parent['path'] += parent_suffix
                self.event['workflow_run']['path'] += event_suffix
                self.current['path'] += self_suffix
                try:
                    outcome = self.notify()
                except Refused as exc:
                    self.fail('Expected trusted main path representation to be accepted: ' + str(exc))
                self.assertEqual(outcome, 'requested')
                self.assertEqual(self.mutations, [('POST', '/actions/workflows/renovate.yml/dispatches', {'ref': 'main'})])

    def test_final_parent_reread_accepts_same_registered_path_with_documented_main_suffix(self):
        def hook(path):
            if path == '/actions/runs/10' and self.reads.count(path) == 2:
                self.parent['path'] += '@main'
        self.hook = hook
        try:
            outcome = self.notify()
        except Refused as exc:
            self.fail('Expected immutable main path representation to be accepted on final reread: ' + str(exc))
        self.assertEqual(outcome, 'requested')
        self.assertEqual(len(self.mutations), 1)

    def test_unexpected_run_path_files_refs_and_suffixes_are_refused(self):
        for target in ['parent', 'event', 'current']:
            for value in ['@other', '@refs/heads/main', '@refs/tags/main', '@' + MERGED,
                          '@main@main', '@main/extra', '@main\n', '.backup', '', None]:
                with self.subTest(target=target, suffix=value):
                    self.setUp()
                    run = self.event['workflow_run'] if target == 'event' else getattr(self, target)
                    run['path'] = run['path'] + value if value else value
                    self.refused()
            with self.subTest(target=target, path='wrong filename'):
                self.setUp()
                run = self.event['workflow_run'] if target == 'event' else getattr(self, target)
                run['path'] = '.github/workflows/other.yml@main'
                self.refused()

    def test_registered_workflow_paths_remain_plain_even_when_runs_are_main_qualified(self):
        for target in ['parent_workflow', 'self_workflow']:
            with self.subTest(target=target):
                self.setUp()
                self.parent['path'] += '@main'
                self.event['workflow_run']['path'] += '@main'
                self.current['path'] += '@main'
                getattr(self, target)['path'] += '@main'
                self.refused()

    def test_main_qualified_paths_cannot_bypass_branch_source_or_workflow_id_identity(self):
        for target in ['parent', 'event', 'current']:
            for field, value in [('head_branch', 'other'), ('head_sha', 'd' * 40), ('workflow_id', 999)]:
                with self.subTest(target=target, field=field):
                    self.setUp()
                    self.parent['path'] += '@main'
                    self.event['workflow_run']['path'] += '@main'
                    self.current['path'] += '@main'
                    run = self.event['workflow_run'] if target == 'event' else getattr(self, target)
                    run[field] = value
                    self.refused()

    def test_valid_owner_security_pr_uses_same_completed_bot_parent_and_native_approval(self):
        self.pr.update(user={'id': 456, 'login': 'enjoydarts', 'type': 'User'},
                       labels=[{'name': 'dependency-security'}], body='Security fix: patched vulnerability')
        self.pr['head']['ref'] = 'dependency-security/python-fix'
        self.assertEqual(self.notify(), 'requested')

    def test_parent_active_failed_cancelled_rerun_or_wrong_source_is_refused(self):
        for field, value in [('status', 'in_progress'), ('conclusion', 'failure'),
                             ('conclusion', 'cancelled'), ('run_attempt', 2), ('run_attempt', True),
                             ('event', 'push'), ('head_branch', 'other'), ('head_sha', MERGED),
                             ('path', '.github/workflows/other.yml'), ('workflow_id', 22),
                             ('id', 11), ('display_title', 'Dependency PR 0 ' + HEAD + ' ' + BASE)]:
            with self.subTest(field=field, value=value):
                self.setUp()
                self.parent[field] = value
                self.refused()

    def test_event_and_parent_identity_must_match(self):
        for field in ['head_sha', 'workflow_id', 'run_attempt', 'display_title', 'path', 'head_branch', 'event', 'conclusion']:
            with self.subTest(field=field):
                self.setUp()
                self.event['workflow_run'][field] = 'foreign'
                self.refused()

    def test_foreign_uncompleted_or_invalid_event_is_refused(self):
        for condition in ['foreign_repo', 'missing_repo_id', 'active', 'missing_run', 'string_id', 'zero_id', 'boolean_id']:
            with self.subTest(condition=condition):
                self.setUp()
                if condition == 'foreign_repo': self.event['repository']['full_name'] = 'other/repo'
                if condition == 'missing_repo_id': del self.event['repository']['id']
                if condition == 'active': self.event['action'] = 'requested'
                if condition == 'missing_run': del self.event['workflow_run']
                if condition == 'string_id': self.event['workflow_run']['id'] = '10'
                if condition == 'zero_id': self.event['workflow_run']['id'] = 0
                if condition == 'boolean_id': self.event['workflow_run']['id'] = True
                self.refused()

    def test_registered_workflow_name_path_state_and_ids_are_authority(self):
        for target in ['parent_workflow', 'self_workflow']:
            for field, value in [('name', 'other'), ('path', '.github/workflows/other.yml'), ('state', 'disabled_manually'), ('id', 999)]:
                with self.subTest(target=target, field=field):
                    self.setUp()
                    getattr(self, target)[field] = value
                    self.refused()

    def test_parent_and_self_require_canonical_same_repository_and_bot_identity(self):
        for target in ['parent', 'current']:
            for field in ['repository', 'head_repository', 'actor', 'triggering_actor']:
                for condition in ['missing', 'wrong_id', 'wrong_name', 'wrong_type']:
                    with self.subTest(target=target, field=field, condition=condition):
                        self.setUp()
                        run = getattr(self, target)
                        if condition == 'missing': del run[field]
                        else:
                            run[field] = dict(run[field])
                            key = 'id' if condition == 'wrong_id' else ('type' if condition == 'wrong_type' else ('login' if 'actor' in field else 'full_name'))
                            run[field][key] = 'foreign'
                        # Repository API identity has no type field contract.
                        if condition == 'wrong_type' and 'repository' in field: continue
                        self.refused()

    def test_owner_parent_cannot_reuse_owner_native_approval(self):
        self.parent['actor'] = {'id': 456, 'login': 'enjoydarts', 'type': 'User'}
        self.event['workflow_run']['actor'] = self.parent['actor']
        self.refused()

    def test_self_rerun_context_or_foreign_current_source_cannot_notify(self):
        for field, value in [('GITHUB_RUN_ATTEMPT', '2'), ('GITHUB_RUN_ID', '0'),
                             ('GITHUB_REPOSITORY_ID', '124'), ('GITHUB_EVENT_NAME', 'workflow_dispatch'),
                             ('GITHUB_REF', 'refs/heads/other'), ('GITHUB_SHA', BASE),
                             ('GITHUB_WORKFLOW_SHA', BASE), ('GITHUB_WORKFLOW_REF', 'owner/repo/.github/workflows/other.yml@refs/heads/main')]:
            with self.subTest(field=field):
                self.setUp()
                self.context[field] = value
                self.refused()
        for field, value in [('run_attempt', 2), ('head_sha', BASE), ('event', 'push'), ('status', 'completed'), ('display_title', 'other')]:
            with self.subTest(field=field):
                self.setUp()
                self.current[field] = value
                self.refused()

    def test_full_first_attempt_successful_job_inventory_is_required(self):
        for condition in ['missing', 'extra', 'duplicate_id', 'unknown_name', 'failed', 'active', 'foreign_run', 'foreign_sha', 'rerun', 'missing_attempt']:
            with self.subTest(condition=condition):
                self.setUp()
                if condition == 'missing': self.jobs.pop()
                if condition == 'extra': self.jobs.append(dict(self.jobs[0], id=102, name='extra'))
                if condition == 'duplicate_id': self.jobs[1]['id'] = 100
                if condition == 'unknown_name': self.jobs[1]['name'] = 'other'
                if condition == 'failed': self.jobs[1]['conclusion'] = 'failure'
                if condition == 'active': self.jobs[1]['status'] = 'in_progress'
                if condition == 'foreign_run': self.jobs[1]['run_id'] = 11
                if condition == 'foreign_sha': self.jobs[1]['head_sha'] = HEAD
                if condition == 'rerun': self.jobs[1]['run_attempt'] = 2
                if condition == 'missing_attempt': del self.jobs[1]['run_attempt']
                self.refused()

    def test_actual_native_owner_environment_approval_is_required(self):
        for condition in ['missing', 'reviewer', 'environment', 'rejected', 'malformed']:
            with self.subTest(condition=condition):
                self.setUp()
                if condition == 'missing': self.approvals = []
                if condition == 'reviewer': self.approvals[0]['user']['login'] = 'someone'
                if condition == 'environment': self.approvals[0]['environments'][0]['name'] = 'other'
                if condition == 'rejected': self.approvals[0]['state'] = 'rejected'
                if condition == 'malformed': self.approvals = {}
                self.refused()

    def test_pr_must_be_exact_same_repo_merged_dependency_candidate(self):
        for condition in ['open', 'not_merged', 'merge_sha', 'head_sha', 'fork', 'foreign_base', 'base_branch',
                          'branch', 'probe', 'author', 'author_id', 'author_type']:
            with self.subTest(condition=condition):
                self.setUp()
                if condition == 'open': self.pr['state'] = 'open'
                if condition == 'not_merged': self.pr['merged'] = False
                if condition == 'merge_sha': self.pr['merge_commit_sha'] = BASE
                if condition == 'head_sha': self.pr['head']['sha'] = BASE
                if condition == 'fork': self.pr['head']['repo'] = dict(REPO, id=124)
                if condition == 'foreign_base': self.pr['base']['repo'] = dict(REPO, full_name='other/repo')
                if condition == 'base_branch': self.pr['base']['ref'] = 'other'
                if condition == 'branch': self.pr['head']['ref'] = 'feature/arbitrary'
                if condition == 'probe': self.pr['head']['ref'] = 'renovate-ci/approval-probe-10'
                if condition == 'author': self.pr['user'] = {'login': 'enjoydarts', 'id': 456, 'type': 'User'}
                if condition == 'author_id': self.pr['user'] = dict(BOT, id=999)
                if condition == 'author_type': self.pr['user'] = dict(BOT, type='User')
                self.refused()

    def test_security_namespace_requires_existing_owner_label_and_stated_fix(self):
        for condition in ['author', 'label', 'body', 'branch']:
            with self.subTest(condition=condition):
                self.setUp()
                self.pr.update(user={'login': 'enjoydarts', 'id': 456, 'type': 'User'},
                               labels=[{'name': 'dependency-security'}], body='Security fix: patched')
                self.pr['head']['ref'] = 'dependency-security/python-fix'
                if condition == 'author': self.pr['user'] = BOT
                if condition == 'label': self.pr['labels'] = []
                if condition == 'body': self.pr['body'] = 'Please update'
                if condition == 'branch': self.pr['head']['ref'] = 'dependency-security/../other'
                self.refused()

    def test_merge_authority_uses_immutable_ordered_parents_not_closed_pr_base_sha(self):
        self.pr['base']['sha'] = 'd' * 40
        self.assertEqual(self.notify(), 'requested')
        for parents in [[HEAD, BASE], [BASE], [BASE, HEAD, MERGED], [MERGED, HEAD]]:
            with self.subTest(parents=parents):
                self.setUp()
                self.commit['parents'] = [{'sha': sha} for sha in parents]
                self.refused()

    def test_current_main_must_be_protected_exact_merge_and_trusted_self_source(self):
        self.protected = False
        self.refused()
        self.setUp()
        self.main = 'd' * 40
        self.refused()

    def test_earlier_child_consumes_notification_even_failed_or_unknown_on_old_source(self):
        for conclusion in ['success', 'failure', None]:
            with self.subTest(conclusion=conclusion):
                self.setUp()
                self.history.insert(0, dict(self.current, id=29, head_sha=BASE, conclusion=conclusion))
                self.refused()
                self.assertFalse(any('head_sha=' in path for path in self.reads))

    def test_later_duplicate_waits_while_earliest_current_child_can_notify(self):
        self.history.append(dict(self.current, id=31))
        self.assertEqual(self.notify(), 'requested')

    def test_missing_duplicate_or_incomplete_history_refuses(self):
        for history in [[], [self.current, self.current]]:
            with self.subTest(history=history):
                self.history = history
                self.refused()
        self.setUp()
        original = self.api.transport
        def transport(method, path, body):
            result = original(method, path, body)
            if '/dependency-followup.yml/runs?' in path: result['total_count'] = 2
            return result
        self.api.transport = transport
        self.refused()

    def test_missing_history_total_or_unbounded_pagination_cannot_authorize_notification(self):
        for condition in ['missing_total', 'unbounded']:
            with self.subTest(condition=condition):
                self.setUp()
                original = self.api.transport
                def transport(method, path, body):
                    result = original(method, path, body)
                    if '/dependency-followup.yml/runs?' in path:
                        if condition == 'missing_total': del result['total_count']
                        else:
                            page = int(path.rsplit('page=', 1)[1])
                            result = {'total_count': 2001, 'workflow_runs': [
                                dict(self.current, id=1000 + (page - 1) * 100 + index, display_title='other')
                                for index in range(100)]}
                    return result
                self.api.transport = transport
                self.refused()
                if condition == 'unbounded':
                    self.assertEqual(sum('/dependency-followup.yml/runs?' in path for path in self.reads), 20)

    def test_missing_or_incomplete_native_jobs_total_is_refused(self):
        for condition in ['missing', 'incomplete']:
            with self.subTest(condition=condition):
                self.setUp()
                original = self.api.transport
                def transport(method, path, body):
                    result = original(method, path, body)
                    if path.startswith('/actions/runs/10/jobs?'):
                        if condition == 'missing': del result['total_count']
                        else: result['total_count'] = 3
                    return result
                self.api.transport = transport
                self.refused()

    def test_history_is_fully_paginated_even_when_earlier_child_is_on_later_page(self):
        self.history = [dict(self.current, id=100 + index, display_title='other') for index in range(100)]
        self.history.extend([self.current, dict(self.current, id=29, head_sha=BASE)])
        original = self.api.transport
        def transport(method, path, body):
            result = original(method, path, body)
            if '/dependency-followup.yml/runs?' in path:
                page = int(path.rsplit('page=', 1)[1])
                result['workflow_runs'] = self.history[(page - 1) * 100:page * 100]
            return result
        self.api.transport = transport
        self.refused()
        self.assertTrue(any('page=2' in path for path in self.reads))

    def test_late_parent_job_approval_or_main_change_prevents_post(self):
        for condition in ['parent', 'jobs', 'approval', 'main']:
            with self.subTest(condition=condition):
                self.setUp()
                def hook(path):
                    if path == '/actions/runs/10' and self.reads.count(path) == 2:
                        if condition == 'parent': self.parent['run_attempt'] = 2
                        if condition == 'jobs': self.jobs[1]['conclusion'] = 'failure'
                        if condition == 'approval': self.approvals = []
                        if condition == 'main': self.main = 'd' * 40
                self.hook = hook
                self.refused()

    def test_final_main_read_after_all_authorities_prevents_source_advance_before_post(self):
        def hook(path):
            if path == '/git/ref/heads/main' and self.reads.count(path) == 2:
                self.main = 'd' * 40
        self.hook = hook
        self.refused()

    def test_http_rejection_and_unknown_outcome_never_replay(self):
        for error, outcome in [(urllib.error.HTTPError('controlled', 403, '', {}, None), 'rejected'),
                               (TimeoutError('controlled'), 'unconfirmed')]:
            with self.subTest(outcome=outcome):
                self.setUp()
                self.error = error
                self.assertEqual(self.notify(), outcome)
                self.assertEqual(len(self.mutations), 1)
                self.current['run_attempt'] = 2
                with self.assertRaises(Refused): self.notify()
                self.assertEqual(len(self.mutations), 1)
                self.current['run_attempt'] = 1
                self.context['GITHUB_RUN_ID'] = '31'
                self.current = dict(self.current, id=31)
                self.history.append(self.current)
                with self.assertRaises(Refused): self.notify()
                self.assertEqual(len(self.mutations), 1)

    def test_event_file_is_bounded_and_duplicate_keys_are_refused(self):
        with tempfile.TemporaryDirectory() as directory:
            path = Path(directory) / 'event.json'
            path.write_text(json.dumps(self.event))
            self.assertEqual(followup.read_event(path), self.event)
            for content in ['{"action":"completed","action":"requested"}', ' ' * 1_000_001, '[]']:
                path.write_text(content)
                with self.assertRaises(Refused): followup.read_event(path)

    def test_entrypoint_reads_only_native_event_and_reports_outcome_without_claiming_new_run(self):
        for error, outcome in [(None, 'requested'), (TimeoutError('controlled'), 'unconfirmed'),
                               (urllib.error.HTTPError('controlled', 403, '', {}, None), 'rejected')]:
            with self.subTest(outcome=outcome), tempfile.TemporaryDirectory() as directory:
                self.setUp()
                self.error = error
                event_path, summary_path = Path(directory) / 'event.json', Path(directory) / 'summary'
                event_path.write_text(json.dumps(self.event))
                context = dict(self.context, GITHUB_EVENT_PATH=str(event_path), GITHUB_STEP_SUMMARY=str(summary_path))
                with patch.dict(os.environ, context), patch.object(followup, 'GitHub', return_value=self.api), patch('builtins.print'):
                    followup.entrypoint()
                summary = summary_path.read_text()
                self.assertIn('notification: ' + outcome, summary)
                self.assertIn('daily Renovate scan', summary)
                self.assertIn('their own CI and native owner approval', summary)
                self.assertNotIn('created', summary)
                self.assertEqual(len(self.mutations), 1)


if __name__ == '__main__':
    unittest.main()
