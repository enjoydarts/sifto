"""Release only verified read-only Bot PR runs; never approve environments.

An exact-head workflow query is an authority: unexpected rows fail closed,
including foreign rows rather than silently discarding malformed identities.
Old terminal runs may be superseded; old unfinished or held runs are conflicts.
Successful reruns are evidence only, and cannot authorize another approval POST.
"""
import os
import re
import time
from .github import Ambiguous
from .policy import Refused, require

WAIT_SECONDS = 50 * 60
WORKFLOWS = ('dependency-check.yml', 'app-runtime.yml', 'maintenance-test.yml')
BOT = {'id': 41898282, 'login': 'github-actions[bot]', 'type': 'Bot'}
JOBS = {
    'dependency-check.yml': {'Dependency API', 'Dependency Worker', 'Dependency Web', 'Dependency Policy'},
    'app-runtime.yml': {'App Runtime api', 'App Runtime worker', 'App Runtime web'},
    'maintenance-test.yml': {'PostgreSQL data mount ordering', 'test'},
}
ACTIVE = {'queued', 'in_progress', 'requested', 'pending', 'waiting'}
TERMINAL = {'success', 'failure', 'cancelled', 'timed_out', 'skipped', 'neutral', 'stale', 'startup_failure', 'action_required'}


def positive(value):
    return type(value) is int and value > 0


def bot(value):
    return type(value) is dict and all(value.get(key) == expected and type(value.get(key)) is type(expected)
                                      for key, expected in BOT.items())


class Session:
    def __init__(self, api, plan, revalidate, clock, *, allow_approval=False):
        self.api, self.plan, self.revalidate = api, plan, revalidate
        self.allow_approval = allow_approval is True
        self.clock = clock
        self.deadline = clock() + WAIT_SECONDS
        self.repository = api.get('')
        require(type(self.repository) is dict and positive(self.repository.get('id'))
                and self.repository.get('full_name') == api.repo, 'Malformed live PR CI repository identity')
        if plan['emergency']:
            owner = self.repository.get('owner')
            require(type(owner) is dict and positive(owner.get('id')) and owner.get('login') == 'enjoydarts'
                    and owner.get('type') == 'User', 'Emergency PR CI repository owner identity mismatch')
        self.workflows, self.selected, self.approved = {}, {}, set()

    def remaining(self, clock=None):
        clock = clock or self.clock
        remaining = self.deadline - clock()
        require(remaining > 0, 'Timed out waiting for exact PR CI')
        return remaining

    def same_repo(self, value):
        return (type(value) is dict and positive(value.get('id'))
                and value['id'] == self.repository['id']
                and value.get('full_name') == self.api.repo)

    def associated_repo(self, value):
        # Actions' PR associations use a mini-repository schema (id/name/url),
        # while the run and live PR authorities use full repositories.
        if type(value) is not dict:
            return False
        if 'full_name' in value:
            return self.same_repo(value)
        return (positive(value.get('id')) and value['id'] == self.repository['id']
                and value.get('name') == self.api.repo.split('/')[1]
                and value.get('url') == 'https://api.github.com/repos/' + self.api.repo)

    def candidate_identity(self):
        # Cheap current authorities during waits; the complete semantic/age
        # proof is recomputed before mutation and before returning success.
        main = self.api.get('/git/ref/heads/main')
        pr = self.api.get('/pulls/%d' % self.plan['pr'])
        require(main.get('object', {}).get('sha') == self.plan['source'] == self.plan['base'],
                'Main changed during PR CI')
        require(pr.get('state') == 'open' and pr.get('merged') is False and pr.get('draft') is False
                and pr.get('head', {}).get('sha') == self.plan['head']
                and pr.get('base', {}).get('sha') == self.plan['base']
                and pr.get('head', {}).get('ref') == self.plan['branch']
                and pr.get('base', {}).get('ref') == 'main'
                and pr.get('user', {}).get('login') == ('enjoydarts' if self.plan['emergency'] else BOT['login'])
                and self.same_repo(pr.get('head', {}).get('repo'))
                and self.same_repo(pr.get('base', {}).get('repo')),
                'Candidate changed during PR CI')

    def register(self):
        from .pipeline import blob
        for name in WORKFLOWS:
            workflow = self.api.get('/actions/workflows/' + name)
            path = '.github/workflows/' + name
            require(type(workflow) is dict and positive(workflow.get('id'))
                    and workflow.get('path') == path and workflow.get('state') == 'active',
                    'PR CI registered workflow identity or state mismatch')
            require(workflow['id'] not in {entry['id'] for entry in self.workflows.values()},
                    'Duplicate PR CI workflow registration')
            require(blob(self.api, path, self.plan['head']) == blob(self.api, path, self.plan['source']),
                    'PR CI candidate workflow differs from trusted main source')
            self.workflows[name] = workflow

    def validate(self, run, name):
        require(type(run) is dict and positive(run.get('id')) and positive(run.get('workflow_id'))
                and run['workflow_id'] == self.workflows[name]['id']
                and run.get('path') == '.github/workflows/' + name
                and run.get('event') == 'pull_request' and run.get('head_sha') == self.plan['head']
                and run.get('head_branch') == self.plan['branch']
                and self.same_repo(run.get('repository')) and self.same_repo(run.get('head_repository'))
                and positive(run.get('run_attempt')), 'PR CI run identity mismatch')
        require(type(run.get('actor')) is dict and type(run.get('triggering_actor')) is dict
                and positive(run['actor'].get('id')) and positive(run['triggering_actor'].get('id'))
                and type(run['actor'].get('login')) is str and type(run['triggering_actor'].get('login')) is str
                and run['actor'].get('type') in {'User', 'Bot'}
                and run['triggering_actor'].get('type') in {'User', 'Bot'}, 'Malformed PR CI actor identity')
        if not self.plan['emergency']:
            require(bot(run['actor']), 'PR CI original actor is not the expected Bot')
        else:
            require(run['actor']['id'] == self.repository['owner']['id']
                    and run['actor']['login'] == 'enjoydarts' and run['actor']['type'] == 'User',
                    'Emergency PR CI original actor is not the expected owner')
        prs = run.get('pull_requests')
        require(type(prs) is list and len(prs) <= 1, 'Malformed PR CI pull request associations')
        for pr in prs:
            require(type(pr) is dict and type(pr.get('number')) is int and pr['number'] == self.plan['pr'],
                    'PR CI pull request association mismatch')
            for side, sha, ref in [('head', self.plan['head'], self.plan['branch']), ('base', self.plan['base'], 'main')]:
                value = pr.get(side)
                require(type(value) is dict and value.get('sha') == sha and value.get('ref') == ref
                        and self.associated_repo(value.get('repo')), 'PR CI pull request head/base identity mismatch')
        require((run.get('status') in ACTIVE and run.get('conclusion') is None)
                or (run.get('status') == 'completed' and run.get('conclusion') in TERMINAL),
                'Unknown or malformed PR CI status')
        return run

    def inventory(self):
        selected, identifiers = {}, set()
        for name, workflow in self.workflows.items():
            rows = self.api.pages('/actions/workflows/%d/runs?event=pull_request&head_sha=%s' %
                                  (workflow['id'], self.plan['head']), 'workflow_runs', require_total=True)
            for row in rows:
                self.validate(row, name)
                require(row['id'] not in identifiers, 'Duplicate PR CI run inventory')
                identifiers.add(row['id'])
            if rows:
                newest = max(rows, key=lambda row: row['id'])
                for row in rows:
                    require(row['id'] == newest['id'] or row['status'] == 'completed'
                            and row['conclusion'] != 'action_required', 'Older PR CI run is still pending or held')
                current = self.validate(self.api.get('/actions/runs/%d' % newest['id']), name)
                require(current['id'] == newest['id'] and current['run_attempt'] == newest['run_attempt'],
                        'PR CI run changed during inventory')
                selected[name] = current
        return selected

    def required(self):
        names = set(WORKFLOWS[:2])
        if any(entry['path'].startswith('infra/maintenance/') or entry['path'] == 'Makefile'
               or re.fullmatch(r'\.github/workflows/maintenance-.*\.yml', entry['path'])
               for entry in self.plan['files']):
            names.add('maintenance-test.yml')
        return names

    def jobs(self, run, all_attempts=False):
        return self.api.pages('/actions/runs/%d/jobs?filter=%s' % (run['id'], 'all' if all_attempts else 'latest'),
                              'jobs', require_total=True)

    def held(self, run):
        require(run['status'] == 'completed' and run['conclusion'] == 'action_required'
                and run['run_attempt'] == 1 and bot(run['actor']) and bot(run['triggering_actor'])
                and not self.plan['emergency']
                and re.fullmatch(r'renovate-ci/[a-zA-Z0-9_.-]{1,120}', self.plan['branch']),
                'PR CI security hold is not a first-attempt ordinary Bot approval')
        require(self.jobs(run, all_attempts=True) == [], 'PR CI held run has already executed jobs')

    def approval_eligibility(self, runs):
        for run in runs.values():
            if run['conclusion'] == 'action_required':
                self.held(run)
            elif run['status'] == 'completed':
                require(run['conclusion'] == 'success', 'PR CI failed; no owner approval request')

    def approve(self, clock):
        # Every selected workflow remains an authority for every mutation,
        # including workflows other than the run being approved next.
        self.approval_eligibility(self.selected)
        require(self.allow_approval or not any(run['conclusion'] == 'action_required'
                                             for run in self.selected.values()),
                'Automatic PR CI approval is disabled; no owner approval request')
        for name, listed in self.selected.items():
            if listed['conclusion'] != 'action_required':
                continue
            self.remaining(clock)
            run = self.validate(self.api.get('/actions/runs/%d' % listed['id']), name)
            require(run['id'] == listed['id'] and run['run_attempt'] == listed['run_attempt'], 'PR CI run changed before approval')
            if run['status'] in ACTIVE or run['conclusion'] == 'success':
                continue
            self.held(run)
            self.revalidate()
            self.candidate_identity()
            latest = self.inventory()
            require(set(latest) == set(self.selected)
                    and all(latest[key]['id'] == chosen['id'] and latest[key]['run_attempt'] == chosen['run_attempt']
                            for key, chosen in self.selected.items()), 'PR CI inventory changed before approval')
            self.approval_eligibility(latest)
            run = latest[name]
            if run['status'] in ACTIVE or run['conclusion'] == 'success':
                continue
            self.held(run)
            self.candidate_identity()
            final = self.validate(self.api.get('/actions/runs/%d' % run['id']), name)
            require(final == run, 'PR CI run changed immediately before approval')
            self.remaining(clock)
            require(run['id'] not in self.approved, 'PR CI approval replay refused')
            self.approved.add(run['id'])
            try:
                self.api.request('POST', '/actions/runs/%d/approve' % run['id'])
            except Ambiguous:
                raise Refused('PR CI approval outcome unknown; no retry or owner approval request') from None
            except Refused as exc:
                status = re.fullmatch(r'GitHub API request rejected \(HTTP ([0-9]{3})\)', str(exc))
                detail = ' (HTTP %s)' % status[1] if status else ''
                raise Refused('PR CI approval rejected%s: GITHUB_TOKEN permission may not support run approval; no retry or token fallback' % detail) from None

    def wait(self, clock=time.monotonic, sleep=time.sleep):
        while True:
            self.remaining(clock)
            self.candidate_identity()
            current = self.inventory()
            require(set(current) == set(self.selected), 'PR CI inventory changed during wait')
            success = True
            for name, run in current.items():
                selected = self.selected[name]
                require(run['id'] == selected['id'] and run['run_attempt'] == selected['run_attempt'],
                        'Selected PR CI run changed during wait')
                if run['status'] != 'completed' or run['conclusion'] == 'action_required':
                    success = False
                    continue
                require(run['conclusion'] == 'success', 'PR CI failed; no owner approval request')
                jobs = self.jobs(run)
                require(len(jobs) == len(JOBS[name]) and {job.get('name') for job in jobs} == JOBS[name]
                        and len({job.get('id') for job in jobs}) == len(jobs)
                        and all(positive(job.get('id')) and job.get('run_id') == run['id']
                                and job.get('head_sha') == self.plan['head'] and job.get('status') == 'completed'
                                and job.get('conclusion') == 'success' for job in jobs), 'PR CI job inventory did not succeed')
                final = self.validate(self.api.get('/actions/runs/%d' % run['id']), name)
                require(final == run, 'PR CI changed while verifying successful jobs')
            self.remaining(clock)
            if success:
                self.revalidate()
                self.candidate_identity()
                self.remaining(clock)
                return
            sleep(min(10, self.remaining(clock)))


def start(api, plan, revalidate, clock=time.monotonic, sleep=time.sleep):
    session = Session(api, plan, revalidate, clock,
                      allow_approval=os.environ.get('DEPENDENCY_PR_CI_AUTO_APPROVAL') == 'true')
    session.register()
    # GitHub may expose the Bot-created PR before its automatic runs. Missing
    # required checks may propagate, but never become permission to skip CI.
    while True:
        session.remaining(clock)
        session.candidate_identity()
        session.selected = session.inventory()
        if session.required() <= set(session.selected):
            break
        sleep(min(10, session.remaining(clock)))
    session.approve(clock)
    return session
