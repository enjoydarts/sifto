"""Exact declined proposal cleanup through its bounded REST and Git boundaries."""
import copy
import importlib.util
import re
import subprocess
import tempfile
import unittest
from datetime import datetime, timezone
from unittest.mock import Mock, patch

from dependency_updates import github, policy

BASE, HEAD, OLD = 'a' * 40, 'b' * 40, 'c' * 40
BOT = {'login': 'github-actions[bot]', 'id': 41898282, 'type': 'Bot'}
OWNER = {'login': 'enjoydarts', 'id': 1234, 'type': 'User'}
NOW = datetime(2026, 10, 13, 0, 10, tzinfo=timezone.utc)


class DeclinedTests(unittest.TestCase):
    def setUp(self):
        self.assertIsNotNone(importlib.util.find_spec('dependency_updates.declined'),
                             'Exact owner-declined proposal cleanup is missing')
        from dependency_updates import declined
        self.module = declined
        self.repo = {'id': 11, 'full_name': 'enjoydarts/sifto', 'default_branch': 'main', 'owner': OWNER}
        self.context = {'GITHUB_REPOSITORY': self.repo['full_name'], 'GITHUB_REPOSITORY_ID': '11',
                        'GITHUB_RUN_ID': '99', 'GITHUB_RUN_ATTEMPT': '1', 'GITHUB_SHA': BASE,
                        'GITHUB_WORKFLOW_SHA': BASE, 'GITHUB_REF': 'refs/heads/main',
                        'GITHUB_WORKFLOW_REF': self.repo['full_name'] + '/.github/workflows/renovate.yml@refs/heads/main',
                        'GITHUB_EVENT_NAME': 'schedule'}
        self.workflow = {'id': 15, 'name': 'Renovate', 'path': '.github/workflows/renovate.yml', 'state': 'active'}
        self.native = {'id': 16, 'name': 'Dependency Approve Merge and Deploy',
                       'path': '.github/workflows/dependency-update.yml', 'state': 'active'}
        self.run = {'id': 99, 'workflow_id': 15, 'path': self.workflow['path'], 'event': 'schedule',
                    'head_branch': 'main', 'head_sha': BASE, 'run_attempt': 1, 'status': 'in_progress',
                    'conclusion': None, 'repository': self.repo, 'head_repository': self.repo,
                    'actor': BOT, 'triggering_actor': BOT, 'run_started_at': '2026-10-13T00:00:00Z'}
        self.old = self.pr(34, OLD, 'closed', '2026-10-12T00:00:00Z')
        self.old['closed_at'] = '2026-10-12T01:00:00Z'
        self.current = self.pr(58, HEAD, 'open', '2026-10-13T00:01:00Z')
        self.files = [{'filename': path, 'status': 'modified', 'sha': sha}
                      for path, sha in [('infra/audio-concat/requirements.in', 'd' * 40),
                                        ('infra/audio-concat/requirements.txt', 'e' * 40),
                                        ('worker/requirements.in', 'f' * 40)]]
        self.old_files = copy.deepcopy(self.files)
        self.closed_prs = [self.old]
        self.events = [{'id': 1, 'event': 'closed', 'actor': copy.deepcopy(OWNER), 'created_at': self.old['closed_at']}]
        self.runs, self.requests, self.reads = [], [], []
        self.mutate_read = None
        self.close_error = None
        self.deleted = Mock(return_value='deleted')
        self.api = github.GitHub(self.repo['full_name'], self.transport)

    def pr(self, number, head, state, created):
        return copy.deepcopy({'number': number, 'state': state, 'merged': False, 'user': BOT, 'title': 'Update Python dependencies',
                'head': {'sha': head, 'ref': 'renovate-ci/python', 'repo': self.repo},
                'base': {'sha': BASE, 'ref': 'main', 'repo': self.repo}, 'changed_files': 3,
                'created_at': created, 'closed_at': None})

    def transport(self, method, path, body):
        endpoint = path.split('?')[0]
        if method == 'PATCH':
            self.requests.append((method, path, body))
            if self.close_error: raise self.close_error
            self.current['state'] = 'closed'
            return copy.deepcopy(self.current)
        self.reads.append(path)
        if self.mutate_read: self.mutate_read(endpoint)
        if endpoint == '': value = self.repo
        elif endpoint == '/actions/workflows/renovate.yml': value = self.workflow
        elif endpoint == '/actions/workflows/dependency-update.yml': value = self.native
        elif endpoint == '/actions/runs/99': value = self.run
        elif endpoint == '/branches/main': value = {'name': 'main', 'protected': True, 'commit': {'sha': BASE}}
        elif endpoint == '/git/ref/heads/main': value = {'object': {'sha': BASE}}
        elif endpoint == '/git/ref/heads/renovate-ci/python': value = {'object': {'sha': HEAD}}
        elif endpoint == '/actions/workflows/dependency-update.yml/runs': value = {'workflow_runs': self.runs, 'total_count': len(self.runs)}
        elif endpoint == '/pulls': value = [self.current] if 'state=open' in path else self.closed_prs
        elif endpoint == '/pulls/58': value = self.current
        elif endpoint == '/pulls/34': value = self.old
        elif endpoint == '/pulls/58/files': value = self.files
        elif endpoint == '/pulls/34/files': value = self.old_files
        elif endpoint == '/issues/34/events': value = self.events
        else: raise AssertionError('Unexpected API read: ' + path)
        if '?per_page=' in path or '&per_page=' in path:
            page = int(re.search(r'[?&]page=([0-9]+)', path)[1])
            if isinstance(value, list): value = value[(page - 1) * 100:page * 100]
            elif 'workflow_runs' in value:
                value = {**value, 'workflow_runs': value['workflow_runs'][(page - 1) * 100:page * 100]}
        return copy.deepcopy(value)

    def clean(self):
        with patch.object(self.module, 'validate_ref'), patch.object(self.module, 'delete_branch', self.deleted):
            return self.module.cleanup(self.api, self.context, now=NOW)

    def refused(self):
        try: self.clean()
        except policy.Refused: pass
        self.assertEqual(self.requests, [])
        self.deleted.assert_not_called()

    def test_exact_owner_declined_proposal_closes_once_and_deletes_exact_head(self):
        old = copy.deepcopy(self.old)
        self.clean()
        self.assertEqual(self.requests, [('PATCH', '/pulls/58', {'state': 'closed'})])
        self.deleted.assert_called_once_with(self.api, 'renovate-ci/python', HEAD)
        self.assertEqual(self.old, old)

    def test_scheduled_owner_manual_owner_and_bot_writers_are_supported(self):
        for event, actor in [('schedule', OWNER), ('workflow_dispatch', OWNER), ('workflow_dispatch', BOT)]:
            self.setUp()
            self.context['GITHUB_EVENT_NAME'] = self.run['event'] = event
            self.context['GITHUB_ACTOR'] = actor['login']
            self.run['actor'] = self.run['triggering_actor'] = copy.deepcopy(actor)
            try: self.clean()
            except policy.Refused as exc: self.fail('Trusted owner or Bot writer was refused: ' + str(exc))
            self.assertEqual(len(self.requests), 1)

    def test_foreign_writer_identity_mixed_trigger_and_context_actor_refuse(self):
        for actor, trigger, context_actor in [({**OWNER, 'id': 999}, {**OWNER, 'id': 999}, 'enjoydarts'),
                                               ({**OWNER, 'type': 'Bot'}, {**OWNER, 'type': 'Bot'}, 'enjoydarts'),
                                               (OWNER, BOT, 'enjoydarts'),
                                               (BOT, OWNER, 'github-actions[bot]'),
                                               (OWNER, OWNER, 'foreign-user')]:
            self.setUp()
            self.run['actor'], self.run['triggering_actor'] = actor, trigger
            self.context['GITHUB_ACTOR'] = context_actor
            self.refused()

    def test_new_blob_file_or_lock_inventory_is_preserved(self):
        for difference in ('hash', 'path', 'lock'):
            with self.subTest(difference=difference):
                self.setUp()
                if difference == 'hash': self.files[0]['sha'] = '9' * 40
                elif difference == 'path': self.files[0]['filename'] = 'worker/requirements.txt'
                else:
                    self.files.append({'filename': 'worker/requirements.txt', 'status': 'modified', 'sha': '9' * 40})
                    self.current['changed_files'] = 4
                self.refused()

    def test_prior_merged_foreign_owner_bot_fork_branch_and_title_are_preserved(self):
        mutations = [lambda: self.old.update(merged=True),
                     lambda: self.events[0].update(actor={**OWNER, 'id': 999}),
                     lambda: self.events[0].update(actor=BOT),
                     lambda: self.old['user'].update(id=999),
                     lambda: self.old['head'].update(repo={**self.repo, 'id': 999}),
                     lambda: self.old['base'].update(ref='release'),
                     lambda: self.old.update(title='Different update')]
        for mutation in mutations:
            with self.subTest(mutation=mutation):
                self.setUp(); mutation(); self.refused()

    def test_current_foreign_fork_author_namespace_and_old_creation_are_preserved(self):
        mutations = [lambda: self.current['head'].update(repo={**self.repo, 'id': 999}),
                     lambda: self.current.update(user=OWNER),
                     lambda: self.current['head'].update(ref='renovate-ci/approval-probe-3'),
                     lambda: self.current['head'].update(ref='dependency-security/critical'),
                     lambda: self.current.update(created_at='2026-10-12T23:59:59Z'),
                     lambda: self.current.update(created_at=self.run['run_started_at']),
                     lambda: self.current.update(created_at='2026-10-14T00:00:00Z')]
        for mutation in mutations:
            with self.subTest(mutation=mutation):
                self.setUp(); mutation(); self.refused()

    def test_complete_modified_unique_allowed_sha_inventory_required(self):
        for invalid in ('count', 'duplicate', 'rename', 'added', 'scope', 'sha'):
            with self.subTest(invalid=invalid):
                self.setUp()
                if invalid == 'count': self.current['changed_files'] = 4
                elif invalid == 'duplicate': self.files[1] = copy.deepcopy(self.files[0])
                elif invalid == 'rename': self.files[0]['previous_filename'] = 'before'
                elif invalid == 'added': self.files[0]['status'] = 'added'
                elif invalid == 'scope': self.files[0]['filename'] = '.github/workflows/renovate.yml'
                else: self.files[0]['sha'] = 'bad'
                self.refused()

    def test_writer_context_and_registered_identity_fail_closed(self):
        changes = {'GITHUB_RUN_ATTEMPT': '2', 'GITHUB_SHA': HEAD, 'GITHUB_WORKFLOW_SHA': HEAD,
                   'GITHUB_WORKFLOW_REF': 'foreign/.github/workflows/renovate.yml@refs/heads/main',
                   'GITHUB_EVENT_NAME': 'pull_request', 'GITHUB_REPOSITORY_ID': '999'}
        for key, value in changes.items():
            with self.subTest(key=key):
                self.setUp(); self.context[key] = value; self.refused()
        for key, value in [('workflow_id', 999), ('head_sha', HEAD), ('run_attempt', 2),
                           ('status', 'completed'), ('actor', OWNER), ('run_started_at', '2026-10-12T00:00:00Z')]:
            with self.subTest(run=key):
                self.setUp(); self.run[key] = value; self.refused()

    def test_closed_decision_reopen_or_new_nonowner_close_is_preserved(self):
        for event in ('reopened', 'closed'):
            self.setUp()
            self.events.append({'id': 2, 'event': event, 'actor': BOT, 'created_at': '2026-10-12T02:00:00Z'})
            self.refused()

    def test_closed_pr_and_owner_event_later_pages_are_complete(self):
        self.closed_prs = [self.pr(1000 + index, OLD, 'closed', self.old['created_at']) for index in range(100)]
        for pr in self.closed_prs: pr['title'] = 'Different group'
        self.closed_prs.append(self.old)
        self.events = [{'id': index + 10, 'event': 'labeled', 'actor': BOT,
                        'created_at': self.old['created_at']} for index in range(100)] + self.events
        self.clean()
        self.assertEqual(len(self.requests), 1)
        self.assertTrue(any('/pulls?state=closed' in path and 'page=2' in path for path in self.reads))
        self.assertTrue(any('/issues/34/events?' in path and 'page=2' in path for path in self.reads))

    def test_truncated_event_or_file_inventory_is_never_used(self):
        for endpoint in ('/issues/34/events', '/pulls/34/files', '/pulls/58/files'):
            self.setUp(); original = self.api.transport
            def transport(method, path, body):
                if path.split('?')[0] == endpoint:
                    return [self.events[0] if 'events' in endpoint else self.files[0]] * 100
                return original(method, path, body)
            self.api.transport = transport; self.refused()

    def test_earlier_native_request_on_later_page_is_preserved(self):
        self.runs = [{**self.native_run(), 'id': index + 1000,
                      'display_title': 'Dependency PR %d %s %s' % (index + 1000, HEAD, BASE)} for index in range(100)]
        self.runs.append(self.native_run())
        self.refused()
        self.assertTrue(any('/dependency-update.yml/runs?' in path and 'page=2' in path for path in self.reads))

    def test_main_source_branch_and_registered_workflow_races_refuse(self):
        for endpoint, mutate in [('/branches/main', lambda value: {**value, 'protected': False}),
                                 ('/git/ref/heads/main', lambda value: {'object': {'sha': OLD}}),
                                 ('/git/ref/heads/renovate-ci/python', lambda value: {'object': {'sha': OLD}}),
                                 ('/actions/workflows/renovate.yml', lambda value: {**value, 'state': 'disabled_manually'})]:
            self.setUp(); original = self.api.transport; count = 0
            def transport(method, path, body):
                nonlocal count
                value = original(method, path, body)
                if path.split('?')[0] == endpoint:
                    count += 1
                    if count >= (1 if 'renovate-ci/' in endpoint else 2): value = mutate(value)
                return value
            self.api.transport = transport; self.refused()

    def test_malformed_timestamps_owner_decisions_and_workflow_metadata_refuse(self):
        changes = [lambda: self.repo.update(default_branch='release'),
                   lambda: self.workflow.update(id=999),
                   lambda: self.workflow.update(path='.github/workflows/other.yml'),
                   lambda: self.workflow.update(state='disabled_manually'),
                   lambda: self.run.update(repository={**self.repo, 'id': 999}),
                   lambda: self.run.update(triggering_actor=OWNER),
                   lambda: self.current.update(created_at='2026-10-13T00:01:00'),
                   lambda: self.events[0].update(created_at='2026-10-14T00:00:00Z'),
                   lambda: self.events[0].update(actor={**OWNER, 'login': 'other'}),
                   lambda: self.old_files[0].update(sha='unknown')]
        for change in changes:
            self.setUp(); change(); self.refused()

    def native_run(self, status='completed'):
        return {'id': 80, 'workflow_id': 16, 'display_title': 'Dependency PR 58 ' + HEAD + ' ' + BASE,
                'head_sha': BASE, 'head_branch': 'main', 'path': self.native['path'],
                'status': status, 'conclusion': 'failure' if status == 'completed' else None,
                'run_attempt': 1, 'event': 'workflow_dispatch', 'repository': self.repo, 'head_repository': self.repo,
                'actor': BOT, 'triggering_actor': BOT}

    def test_active_or_already_requested_native_approval_preserves_proposal(self):
        for status in ('waiting', 'completed'):
            self.setUp(); self.runs = [self.native_run(status)]; self.refused()

    def test_owner_decision_or_head_race_is_rechecked_before_mutation(self):
        for endpoint, change in [('/pulls/34', lambda: self.old.update(state='open')),
                                 ('/pulls/58', lambda: self.current['head'].update(sha=OLD)),
                                 ('/issues/34/events', lambda: self.events[0].update(actor=BOT)),
                                 ('/pulls/58/files', lambda: self.files[0].update(sha='9' * 40))]:
            with self.subTest(endpoint=endpoint):
                self.setUp(); count = 0
                def race(path):
                    nonlocal count
                    if path == endpoint:
                        count += 1
                        if count == 2: change()
                self.mutate_read = race; self.refused()

    def test_unacknowledged_or_rejected_close_never_replays_or_deletes(self):
        for error in (TimeoutError(), policy.Refused('Rejected')):
            self.setUp(); self.close_error = error
            with self.assertRaises(policy.Refused): self.clean()
            self.assertEqual(len(self.requests), 1); self.deleted.assert_not_called()
            self.close_error = None
            self.run['run_started_at'] = '2026-10-13T00:02:00Z'
            before = len(self.requests); self.clean()
            self.assertEqual(len(self.requests), before)

    def test_close_readback_changed_head_or_fork_refuses_branch_deletion(self):
        for moved in ('sha', 'repo'):
            self.setUp(); count = 0
            def race(endpoint):
                nonlocal count
                if endpoint == '/pulls/58':
                    count += 1
                    if count == 3:
                        self.current['head'][moved] = OLD if moved == 'sha' else {**self.repo, 'id': 999}
            self.mutate_read = race
            with self.assertRaises(policy.Refused): self.clean()
            self.assertEqual(len(self.requests), 1); self.deleted.assert_not_called()

    def test_unconfirmed_git_deletion_is_audited_without_replay(self):
        self.deleted.return_value = 'unconfirmed'
        with self.assertRaisesRegex(policy.Refused, 'audit required'): self.clean()
        self.assertEqual(len(self.requests), 1); self.assertEqual(self.deleted.call_count, 1)
        self.clean()
        self.assertEqual(len(self.requests), 1); self.assertEqual(self.deleted.call_count, 1)

    def test_invalid_git_ref_is_refused_before_close(self):
        with patch.object(self.module, 'validate_ref', side_effect=policy.Refused('Invalid ref')):
            self.module.cleanup(self.api, self.context, now=NOW)
        self.assertEqual(self.requests, [])

    def test_lookup_must_return_exact_current_and_prior_pr_numbers(self):
        for number in (34, 58):
            self.setUp(); original = self.api.transport
            def transport(method, path, body):
                value = original(method, path, body)
                if path == '/pulls/%d' % number: value['number'] = 999
                return value
            self.api.transport = transport
            self.refused()

    def test_native_request_starting_during_cleanup_preserves_proposal(self):
        count = 0
        def race(endpoint):
            nonlocal count
            if endpoint == '/actions/workflows/dependency-update.yml/runs':
                count += 1
                if count == 3: self.runs = [self.native_run('waiting')]
        self.mutate_read = race; self.refused()

    def test_writer_expiry_during_inventory_reads_refuses_mutation(self):
        clock = Mock()
        clock.now.side_effect = [NOW, NOW.replace(hour=2)]
        with patch.object(self.module, 'datetime', clock), patch.object(self.module, 'timestamp', side_effect=lambda value: datetime.strptime(value, '%Y-%m-%dT%H:%M:%SZ').replace(tzinfo=timezone.utc)), patch.object(self.module, 'validate_ref'), patch.object(self.module, 'delete_branch', self.deleted):
            self.module.cleanup(self.api, self.context)
        self.assertEqual(self.requests, [])
        self.deleted.assert_not_called()


class GitLeaseTests(unittest.TestCase):
    def test_real_bare_remote_deletes_expected_head_but_preserves_moved_head(self):
        self.assertIsNotNone(importlib.util.find_spec('dependency_updates.declined'))
        from dependency_updates import declined
        def git(*args, cwd=None):
            return subprocess.run(['git', *args], cwd=cwd, check=True, capture_output=True, text=True).stdout.strip()
        with tempfile.TemporaryDirectory() as directory:
            remote, local = directory + '/remote.git', directory + '/local'
            git('init', '--bare', remote); git('init', local)
            git('-C', local, 'config', 'user.email', 'fixture@example.invalid')
            git('-C', local, 'config', 'user.name', 'Fixture')
            git('-C', local, 'commit', '--allow-empty', '-m', 'first')
            head = git('-C', local, 'rev-parse', 'HEAD')
            ref = 'refs/heads/renovate-ci/python'
            git('-C', local, 'push', remote, 'HEAD:' + ref)
            # Execute the production argv with only the fixed HTTPS target
            # redirected to this isolated bare repository by the test boundary.
            real = subprocess.run
            def run(argv, **kwargs):
                argv = list(argv)
                if 'push' in argv and argv[argv.index('push') + 1].startswith('https://'):
                    self.assertIs(kwargs.get('shell'), False)
                    self.assertEqual(argv[argv.index('push') + 1], 'https://github.com/enjoydarts/sifto.git')
                    self.assertIn('--force-with-lease=' + ref + ':' + head, argv)
                    self.assertIn('core.hooksPath=/dev/null', argv)
                    self.assertIn('credential.helper=!gh auth git-credential', argv)
                    argv[argv.index('push') + 1] = remote
                return real(argv, **kwargs)
            api = github.GitHub('enjoydarts/sifto')
            with patch.object(declined.subprocess, 'run', side_effect=run):
                self.assertEqual(declined.delete_branch(api, 'renovate-ci/python', head), 'deleted')
                self.assertEqual(git('--git-dir=' + remote, 'for-each-ref', ref), '')
                git('-C', local, 'commit', '--allow-empty', '-m', 'second')
                moved = git('-C', local, 'rev-parse', 'HEAD')
                git('-C', local, 'push', remote, 'HEAD:' + ref)
                self.assertEqual(declined.delete_branch(api, 'renovate-ci/python', head), 'unconfirmed')
                self.assertEqual(git('--git-dir=' + remote, 'rev-parse', ref), moved)


if __name__ == '__main__':
    unittest.main()
