"""Temporary, harmless workflow completion delivery experiment."""
import copy
import importlib.util
import json
import os
from pathlib import Path
import tempfile
import unittest
import urllib.error
from unittest.mock import patch

import yaml

from dependency_updates.github import GitHub
from dependency_updates.policy import Refused

SPEC = importlib.util.find_spec('dependency_updates.completion_probe')
if SPEC:
    from dependency_updates import completion_probe as probe

SOURCE = 'a' * 40
OWNER = {'id': 777, 'login': 'enjoydarts', 'type': 'User'}
BOT = {'id': 41898282, 'login': 'github-actions[bot]', 'type': 'Bot'}
REPO = {'id': 123, 'full_name': 'enjoydarts/sifto', 'owner': OWNER}
FILES = {kind: 'dependency-completion-probe-' + kind + '.yml'
         for kind in ('caller', 'parent', 'static', 'dynamic')}
NAMES = {'caller': 'Dependency Completion Probe Caller',
         'parent': 'Dependency Completion Delivery Probe',
         'static': 'Dependency Completion Probe Static Receiver',
         'dynamic': 'Dependency Completion Probe Dynamic Receiver'}
ROOT = Path('/repository')


class CompletionProbeTests(unittest.TestCase):
    def setUp(self):
        self.assertIsNotNone(SPEC, 'Temporary completion delivery controller must exist')
        self.workflows = {kind: {'id': 20 + index, 'name': NAMES[kind],
                                'path': '.github/workflows/' + FILES[kind], 'state': 'active'}
                          for index, kind in enumerate(FILES)}
        self.runs = {}
        for index, kind in enumerate(FILES):
            actor = OWNER if kind == 'caller' else BOT
            self.runs[kind] = {'id': 100 + index, 'workflow_id': self.workflows[kind]['id'],
                               'path': self.workflows[kind]['path'], 'head_branch': 'main',
                               'head_sha': SOURCE, 'run_attempt': 1, 'status': 'in_progress',
                               'conclusion': None, 'repository': REPO, 'head_repository': REPO,
                               'actor': actor, 'triggering_actor': actor,
                               'event': 'workflow_dispatch' if kind in ('caller', 'parent') else 'workflow_run',
                               'display_title': {'caller': 'Completion delivery probe caller ' + SOURCE,
                                                 'parent': 'Dependency completion probe 100',
                                                 'static': 'Completion delivery probe static receiver 101',
                                                 'dynamic': 'Completion delivery probe dynamic receiver 101'}[kind]}
        self.histories = {kind: [self.runs[kind]] for kind in FILES}
        self.main, self.protected = SOURCE, True
        self.reads, self.posts = [], []
        self.error, self.hook = None, None
        self.api = GitHub(REPO['full_name'], self.transport)
        self.context, self.event = self.execution('caller')

    def execution(self, kind):
        run = self.runs[kind]
        context = {'GITHUB_REPOSITORY': REPO['full_name'], 'GITHUB_REPOSITORY_ID': '123',
                   'GITHUB_EVENT_NAME': run['event'], 'GITHUB_REF': 'refs/heads/main',
                   'GITHUB_SHA': SOURCE, 'GITHUB_WORKFLOW_SHA': SOURCE,
                   'GITHUB_WORKFLOW_REF': REPO['full_name'] + '/' + run['path'] + '@refs/heads/main',
                   'GITHUB_RUN_ID': str(run['id']), 'GITHUB_RUN_ATTEMPT': '1',
                   'GITHUB_ACTOR': run['actor']['login'], 'GITHUB_ACTOR_ID': str(run['actor']['id']),
                   'GITHUB_TRIGGERING_ACTOR': run['triggering_actor']['login']}
        event = {'repository': copy.deepcopy(REPO), 'inputs': {'expected_head': SOURCE}}
        if kind == 'parent':
            event['inputs'] = {'caller_run_id': '100', 'source_sha': SOURCE}
        if kind in ('static', 'dynamic'):
            self.runs['parent'].update(status='completed', conclusion='success')
            self.runs['caller'].update(status='completed', conclusion='success')
            event = {'repository': copy.deepcopy(REPO), 'action': 'completed',
                     'workflow_run': copy.deepcopy(self.runs['parent'])}
        return context, event

    def transport(self, method, path, body):
        if method != 'GET':
            self.posts.append((method, path, body))
            if self.error:
                raise self.error
            return None
        self.reads.append(path)
        if self.hook:
            self.hook(path)
        endpoint = path.split('?')[0]
        routes = {'': REPO, '/branches/main': {'name': 'main', 'protected': self.protected,
                                              'commit': {'sha': self.main}},
                  '/git/ref/heads/main': {'object': {'sha': self.main}}}
        for index, kind in enumerate(FILES):
            routes['/actions/workflows/' + FILES[kind]] = self.workflows[kind]
            routes['/actions/runs/' + str(100 + index)] = self.runs[kind]
            routes['/actions/workflows/' + FILES[kind] + '/runs'] = {
                'workflow_runs': self.histories[kind], 'total_count': len(self.histories[kind])}
        self.assertIn(endpoint, routes, 'Diagnostic must not access native approval or production')
        return copy.deepcopy(routes[endpoint])

    def execute(self, kind='caller'):
        return probe.execute(self.api, self.event, self.context, kind)

    def refused(self, kind='caller'):
        with self.assertRaises(Refused):
            self.execute(kind)
        self.assertEqual(self.posts, [])

    def test_owner_caller_dispatches_only_harmless_parent_once_with_exact_inputs(self):
        result = self.execute()
        self.assertEqual(result, {'kind': 'caller', 'outcome': 'requested', 'caller_id': 100, 'source': SOURCE})
        self.assertEqual(self.posts, [('POST', '/actions/workflows/' + FILES['parent'] + '/dispatches',
                                      {'ref': 'main', 'inputs': {'caller_run_id': '100', 'source_sha': SOURCE}})])

    def test_unknown_or_rejected_post_is_never_retried_and_rerun_is_refused(self):
        for error, outcome in [(TimeoutError('token-secret https://private'), 'unconfirmed'),
                               (urllib.error.HTTPError('https://private', 403, 'token-secret', {}, None), 'rejected')]:
            with self.subTest(outcome=outcome):
                self.setUp()
                self.error = error
                self.assertEqual(self.execute()['outcome'], outcome)
                self.assertEqual(len(self.posts), 1)
                self.context['GITHUB_RUN_ATTEMPT'] = '2'
                with self.assertRaises(Refused):
                    self.execute()
                self.assertEqual(len(self.posts), 1)

    def test_repeated_caller_same_source_is_refused_even_after_failed_or_unknown_first_attempt(self):
        self.histories['caller'].append(dict(self.runs['caller'], id=99, status='completed', conclusion='failure'))
        self.refused()

    def test_context_mismatches_fail_closed_for_every_role(self):
        for kind in FILES:
            for key, value in {'GITHUB_REPOSITORY': 'foreign/repo', 'GITHUB_REPOSITORY_ID': '456',
                               'GITHUB_EVENT_NAME': 'push', 'GITHUB_REF': 'refs/heads/other',
                               'GITHUB_SHA': 'b' * 40, 'GITHUB_WORKFLOW_SHA': 'b' * 40,
                               'GITHUB_WORKFLOW_REF': 'foreign/file@refs/heads/main',
                               'GITHUB_RUN_ID': '0', 'GITHUB_RUN_ATTEMPT': '2',
                               'GITHUB_ACTOR': 'outsider', 'GITHUB_ACTOR_ID': '8',
                               'GITHUB_TRIGGERING_ACTOR': 'outsider'}.items():
                with self.subTest(kind=kind, key=key):
                    self.setUp()
                    self.context, self.event = self.execution(kind)
                    self.context[key] = value
                    self.refused(kind)

    def test_self_metadata_and_registration_mismatches_fail_closed(self):
        for kind in FILES:
            for key, value in {'workflow_id': 999, 'path': '.github/workflows/deploy.yml',
                               'head_sha': 'b' * 40, 'head_branch': 'other', 'run_attempt': 2,
                               'status': 'completed', 'conclusion': 'failure', 'event': 'push',
                               'actor': dict(OWNER, id=8), 'triggering_actor': dict(OWNER, id=8),
                               'repository': dict(REPO, id=456), 'head_repository': dict(REPO, id=456)}.items():
                with self.subTest(kind=kind, key=key):
                    self.setUp()
                    self.context, self.event = self.execution(kind)
                    self.runs[kind][key] = value
                    self.refused(kind)
            for key, value in {'name': 'Wrong', 'path': '.github/workflows/deploy.yml', 'id': 0, 'state': 'disabled_manually'}.items():
                with self.subTest(kind=kind, registered=key):
                    self.setUp()
                    self.context, self.event = self.execution(kind)
                    self.workflows[kind][key] = value
                    self.refused(kind)

    def test_current_protected_main_and_repo_owner_are_required(self):
        for kind in FILES:
            for condition in ('main', 'protected', 'owner', 'event_repository'):
                with self.subTest(kind=kind, condition=condition):
                    self.setUp()
                    self.context, self.event = self.execution(kind)
                    if condition == 'main': self.main = 'b' * 40
                    if condition == 'protected': self.protected = False
                    if condition == 'event_repository': self.event['repository']['id'] = 456
                    if condition == 'owner':
                        original = self.api.transport
                        self.api.transport = lambda m, p, b: dict(REPO, owner=BOT) if p == '' else original(m, p, b)
                    self.refused(kind)

    def test_parent_is_read_only_and_accepts_in_progress_or_successful_owner_caller(self):
        for status, conclusion in [('in_progress', None), ('completed', 'success')]:
            with self.subTest(status=status):
                self.setUp()
                self.context, self.event = self.execution('parent')
                self.runs['caller'].update(status=status, conclusion=conclusion)
                self.assertEqual(self.execute('parent'), {'kind': 'parent', 'parent_id': 101,
                                 'dynamic_name': 'Dependency completion probe 100', 'caller_id': 100, 'source': SOURCE})
                self.assertEqual(self.posts, [])

    def test_parent_inputs_and_marker_are_exact_bounded_numeric_authority(self):
        for value in ('', '0', '01', '-1', '100;bad', '9' * 21, 100, None):
            with self.subTest(value=value):
                self.setUp()
                self.context, self.event = self.execution('parent')
                self.event['inputs']['caller_run_id'] = value
                self.refused('parent')
        for key, value in [('source_sha', 'b' * 40), ('caller_run_id', '102')]:
            self.setUp()
            self.context, self.event = self.execution('parent')
            self.event['inputs'][key] = value
            self.refused('parent')

    def test_parent_and_receivers_reject_untrusted_or_failed_caller(self):
        for kind in ('parent', 'static', 'dynamic'):
            for key, value in {'workflow_id': 999, 'path': '.github/workflows/deploy.yml', 'head_sha': 'b' * 40,
                               'head_branch': 'other', 'run_attempt': 2, 'event': 'push', 'actor': BOT,
                               'triggering_actor': BOT, 'repository': dict(REPO, id=456),
                               'head_repository': dict(REPO, id=456), 'conclusion': 'failure',
                               'display_title': 'Completion delivery probe caller ' + 'b' * 40}.items():
                with self.subTest(kind=kind, key=key):
                    self.setUp()
                    self.context, self.event = self.execution(kind)
                    self.runs['caller'][key] = value
                    self.refused(kind)

    def test_receivers_observe_success_and_failure_without_any_mutation(self):
        for kind in ('static', 'dynamic'):
            for conclusion in ('success', 'failure', 'cancelled', 'timed_out', 'skipped', 'neutral', 'action_required', 'stale'):
                with self.subTest(kind=kind, conclusion=conclusion):
                    self.setUp()
                    self.context, self.event = self.execution(kind)
                    self.runs['parent']['conclusion'] = conclusion
                    self.event['workflow_run']['conclusion'] = conclusion
                    self.assertEqual(self.execute(kind), {'kind': kind, 'parent_id': 101,
                                     'dynamic_name': 'Dependency completion probe 100', 'source': SOURCE,
                                     'conclusion': conclusion, 'caller_id': 100, 'self_source': SOURCE})
                    self.assertEqual(self.posts, [])

    def test_receivers_accept_validated_owner_or_bot_identity(self):
        for actor in (OWNER, BOT):
            self.setUp()
            self.runs['static'].update(actor=actor, triggering_actor=actor)
            self.context, self.event = self.execution('static')
            self.assertEqual(self.execute('static')['kind'], 'static')

    def test_event_and_rest_parent_must_both_match_terminal_first_attempt_identity(self):
        for target in ('rest', 'event'):
            for key, value in {'id': 0, 'workflow_id': 999, 'path': '.github/workflows/deploy.yml',
                               'status': 'in_progress', 'conclusion': None, 'run_attempt': 2,
                               'head_sha': 'b' * 40, 'head_branch': 'other', 'event': 'push',
                               'display_title': 'Dependency completion probe 100;evil',
                               'actor': OWNER, 'triggering_actor': OWNER,
                               'repository': dict(REPO, id=456), 'head_repository': dict(REPO, id=456)}.items():
                with self.subTest(target=target, key=key):
                    self.setUp()
                    self.context, self.event = self.execution('static')
                    (self.runs['parent'] if target == 'rest' else self.event['workflow_run'])[key] = value
                    self.refused('static')
        self.setUp()
        self.context, self.event = self.execution('static')
        self.event['workflow_run']['conclusion'] = 'failure'
        self.refused('static')

    def test_documented_main_run_path_suffix_is_accepted_without_trusting_dynamic_run_name(self):
        for kind in FILES:
            self.setUp()
            self.context, self.event = self.execution(kind)
            for run in self.runs.values():
                run['path'] += '@main'
                run['name'] = 'arbitrary observed dynamic name'
            if kind in ('static', 'dynamic'):
                self.event['workflow_run'] = copy.deepcopy(self.runs['parent'])
            self.execute(kind)

    def test_missing_duplicate_or_earlier_history_refuses_dispatch(self):
        for condition in ('missing', 'duplicate', 'earlier', 'missing_total'):
            with self.subTest(condition=condition):
                self.setUp()
                if condition == 'missing': self.histories['caller'] = []
                if condition == 'duplicate': self.histories['caller'].append(self.runs['caller'])
                if condition == 'earlier': self.histories['caller'].append(dict(self.runs['caller'], id=99))
                if condition == 'missing_total':
                    original = self.api.transport
                    def transport(method, path, body):
                        result = original(method, path, body)
                        if '/runs?' in path: del result['total_count']
                        return result
                    self.api.transport = transport
                self.refused()

    def test_final_self_or_main_drift_prevents_the_only_post(self):
        for condition in ('main', 'self'):
            with self.subTest(condition=condition):
                self.setUp()
                def hook(path):
                    if path == '/actions/runs/100' and self.reads.count(path) == 2:
                        if condition == 'self': self.runs['caller']['run_attempt'] = 2
                        else: self.main = 'b' * 40
                self.hook = hook
                self.refused()

    def test_entrypoint_only_reports_selected_validated_metadata_without_errors_or_secrets(self):
        for kind in FILES:
            self.setUp()
            self.context, self.event = self.execution(kind)
            self.event['token'] = 'token-secret https://private'
            self.error = TimeoutError('token-secret https://private') if kind == 'caller' else None
            with tempfile.TemporaryDirectory() as directory:
                event_path, summary_path = Path(directory) / 'event', Path(directory) / 'summary'
                event_path.write_text(json.dumps(self.event))
                env = dict(self.context, GITHUB_EVENT_PATH=str(event_path), GITHUB_STEP_SUMMARY=str(summary_path),
                           COMPLETION_PROBE_KIND=kind, GH_TOKEN='token-secret')
                with patch.dict(os.environ, env), patch.object(probe, 'GitHub', return_value=self.api), patch('builtins.print') as output:
                    probe.entrypoint()
                summary = summary_path.read_text()
                printed = ' '.join(str(call) for call in output.call_args_list)
                for text in (summary, printed):
                    self.assertNotIn('token-secret', text)
                    self.assertNotIn('https://', text)
                    self.assertIn(kind, text)
                if kind == 'caller':
                    self.assertIn('unconfirmed', summary)
                    self.assertIn('Do not retry', summary)
                    self.assertIn('workflow history', summary)


class CompletionProbeWorkflowContract(unittest.TestCase):
    def test_four_temporary_workflows_have_exact_filters_and_minimal_permissions(self):
        for kind in FILES:
            with self.subTest(kind=kind):
                path = ROOT / '.github/workflows' / FILES[kind]
                self.assertTrue(path.exists(), 'Temporary ' + kind + ' workflow must exist')
                text = path.read_text()
                workflow = yaml.safe_load(text)
                self.assertEqual(workflow['name'], NAMES[kind])
                self.assertEqual(workflow['run-name'], {
                    'caller': 'Completion delivery probe caller ${{ inputs.expected_head }}',
                    'parent': 'Dependency completion probe ${{ inputs.caller_run_id }}',
                    'static': 'Completion delivery probe static receiver ${{ github.event.workflow_run.id }}',
                    'dynamic': 'Completion delivery probe dynamic receiver ${{ github.event.workflow_run.id }}'}[kind])
                self.assertEqual(workflow['permissions'], {})
                job, = workflow['jobs'].values()
                self.assertEqual(job['permissions'], {'contents': 'read', 'actions': 'write' if kind == 'caller' else 'read'})
                self.assertEqual(job['if'], "github.ref == 'refs/heads/main'")
                self.assertNotIn('environment', job)
                self.assertEqual(len(job['steps']), 2)
                checkout, run = job['steps']
                self.assertEqual(checkout['uses'], 'actions/checkout@fbc6f3992d24b796d5a048ff273f7fcc4a7b6c09')
                self.assertEqual(checkout['with'], {'ref': '${{ github.sha }}', 'persist-credentials': False})
                self.assertEqual(run['run'], 'python3 -B -m dependency_updates.completion_probe')
                self.assertEqual(run['env'], {'GH_TOKEN': '${{ github.token }}', 'PYTHONPATH': 'infra/dependency-ci',
                                             'COMPLETION_PROBE_KIND': kind})
                self.assertNotIn('secrets.', text)
                self.assertNotIn('production', text.lower())
                self.assertNotIn('renovate', text.lower())
                trigger = workflow[True]
                if kind in ('static', 'dynamic'):
                    self.assertEqual(trigger, {'workflow_run': {
                        'workflows': ['Dependency Completion Delivery Probe' if kind == 'static' else 'Dependency completion probe *'],
                        'types': ['completed'], 'branches': ['main']}})
                    self.assertNotIn('conclusion', text)
                else:
                    self.assertEqual(set(trigger), {'workflow_dispatch'})
                    inputs = trigger['workflow_dispatch']['inputs']
                    self.assertEqual(set(inputs), {'expected_head'} if kind == 'caller' else {'caller_run_id', 'source_sha'})
                    self.assertTrue(all(value['required'] is True and value['type'] == 'string' for value in inputs.values()))

    def test_contract_runs_the_temporary_controller_test(self):
        self.assertIn('contract python -B /repository/infra/dependency-ci/test_completion_probe.py',
                      (ROOT / 'Makefile').read_text())


if __name__ == '__main__':
    unittest.main()
