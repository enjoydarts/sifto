"""Close only newly generated, exact proposals previously declined by the owner.

Generation can still create a PR. This trusted writer applies the previous
decision afterward; distinct versions, hashes and file inventories survive.
"""
from datetime import datetime, timedelta, timezone
import os
import re
import subprocess
import tempfile
from urllib.parse import quote

from . import pipeline
from .followup import (bot, context_id, current_main, positive, registered,
                       repository)
from .github import Ambiguous, GitHub
from .policy import (COMPOSE, DOCKERFILES, PYTHON_INPUTS, PYTHON_LOCKS,
                     Refused, SHA, require)

WORKFLOW = 'renovate.yml'
ALLOWED = DOCKERFILES | COMPOSE | PYTHON_INPUTS | PYTHON_LOCKS | {
    'api/go.mod', 'api/go.sum', 'web/package.json', 'web/package-lock.json'}


def timestamp(value):
    require(type(value) is str and re.fullmatch(r'20[0-9]{2}-[0-9]{2}-[0-9]{2}T[0-9]{2}:[0-9]{2}:[0-9]{2}Z', value),
            'Invalid bounded proposal timestamp')
    try:
        return datetime.strptime(value, '%Y-%m-%dT%H:%M:%SZ').replace(tzinfo=timezone.utc)
    except ValueError:
        raise Refused('Invalid proposal timestamp') from None


def authority(api, context, now):
    source, event = context.get('GITHUB_SHA'), context.get('GITHUB_EVENT_NAME')
    require(type(source) is str and SHA.fullmatch(source)
            and context.get('GITHUB_REPOSITORY') == api.repo
            and context.get('GITHUB_REF') == 'refs/heads/main'
            and context.get('GITHUB_WORKFLOW_SHA') == source
            and context.get('GITHUB_WORKFLOW_REF') == api.repo + '/.github/workflows/' + WORKFLOW + '@refs/heads/main'
            and context.get('GITHUB_RUN_ATTEMPT') == '1'
            and event in {'schedule', 'workflow_dispatch'}, 'Cleanup requires first trusted current-main Renovate execution')
    repo_id, run_id = context_id(context.get('GITHUB_REPOSITORY_ID')), context_id(context.get('GITHUB_RUN_ID'))
    repo = api.get('')
    repository(repo, api.repo, repo_id)
    owner = repo.get('owner')
    require(repo.get('default_branch') == 'main' and type(owner) is dict
            and positive(owner.get('id')) and owner.get('login') == 'enjoydarts'
            and owner.get('type') == 'User', 'Cleanup repository owner/default branch mismatch')
    owner = {key: owner[key] for key in ('id', 'login', 'type')}
    run = api.get('/actions/runs/' + str(run_id))
    expected = {'id': run_id, 'workflow_id': registered(api, WORKFLOW, 'Renovate'),
                'path': '.github/workflows/' + WORKFLOW, 'event': event,
                'head_branch': 'main', 'head_sha': source, 'run_attempt': 1,
                'status': 'in_progress', 'conclusion': None}
    require(type(run) is dict and all(
            run.get(key) in (value, value + '@main') if key == 'path' else run.get(key) == value
            for key, value in expected.items()) and type(run.get('run_attempt')) is int
            and positive(run.get('id')) and positive(run.get('workflow_id')),
            'Renovate writer run identity or first-attempt state mismatch')
    repository(run.get('repository'), api.repo, repo_id)
    repository(run.get('head_repository'), api.repo, repo_id)
    actor, trigger = run.get('actor'), run.get('triggering_actor')
    require(type(actor) is dict and type(trigger) is dict and positive(actor.get('id'))
            and type(trigger.get('id')) is int
            and all(actor.get(key) == trigger.get(key) for key in ('id', 'login', 'type')),
            'Renovate writer and triggering actor must be the same trusted principal')
    if actor.get('login') == 'github-actions[bot]':
        bot(actor)
    else:
        require(all(actor.get(key) == owner[key] for key in ('id', 'login', 'type')),
                'Renovate writer must be the actual repository owner or canonical Bot')
    require('GITHUB_ACTOR' not in context or context['GITHUB_ACTOR'] == actor.get('login'),
            'Renovate writer context actor mismatch')
    started = timestamp(run.get('run_started_at'))
    require(started <= now <= started + timedelta(minutes=45), 'Cleanup writer execution is stale or future')
    current_main(api, source)
    return repo_id, source, started, owner, expected


def inventory(api, pr):
    rows = api.pages('/pulls/%d/files' % pr['number'])
    require(type(pr.get('changed_files')) is int and 0 < len(rows) <= pipeline.MAX_FILES
            and len(rows) == pr['changed_files'], 'Missing or truncated proposal file inventory')
    entries = []
    for row in rows:
        require(type(row) is dict and row.get('filename') in ALLOWED
                and row.get('status') == 'modified' and 'previous_filename' not in row
                and type(row.get('sha')) is str and SHA.fullmatch(row['sha']),
                'Invalid proposal file scope/status/blob SHA')
        entries.append((row['filename'], row['status'], row['sha']))
    require(len({entry[0] for entry in entries}) == len(entries), 'Duplicate proposal file inventory')
    return tuple(sorted(entries))


def proposal(pr, repo, repo_id, state):
    require(type(pr) is dict and type(pr.get('number')) is int and 0 < pr['number'] < 10_000_000
            and pr.get('state') == state and pr.get('merged') is False
            and type(pr.get('title')) is str and 0 < len(pr['title']) <= 300,
            'Proposal is not an unmerged dependency PR')
    bot(pr.get('user'))
    for side in ('head', 'base'):
        require(type(pr.get(side)) is dict and type(pr[side].get('sha')) is str
                and SHA.fullmatch(pr[side]['sha']), 'Invalid proposal head/base')
        repository(pr[side].get('repo'), repo, repo_id)
    branch = pr['head'].get('ref')
    require(type(branch) is str and re.fullmatch(r'renovate-ci/[a-zA-Z0-9_.-]{1,120}', branch)
            and not branch.startswith('renovate-ci/approval-probe-')
            and pr['base'].get('ref') == 'main', 'Unsupported proposal namespace/base')
    return branch


def proposal_snapshot(pr, repo, repo_id, state):
    """Only authority fields are immutable; REST embeds mutable repo statistics."""
    proposal(pr, repo, repo_id, state)
    require(type(pr.get('changed_files')) is int and 0 < pr['changed_files'] <= pipeline.MAX_FILES,
            'Invalid proposal changed-file count')
    created = timestamp(pr.get('created_at'))
    require('closed_at' in pr, 'Missing proposal close timestamp')
    if state == 'open':
        require(pr['closed_at'] is None, 'Open proposal has a close timestamp')
    else:
        require(created <= timestamp(pr['closed_at']), 'Invalid closed proposal lifetime')
    result = {key: pr[key] for key in ('number', 'title', 'state', 'merged', 'changed_files',
                                      'created_at', 'closed_at')}
    result['user'] = {key: pr['user'][key] for key in ('id', 'login', 'type')}
    for side in ('head', 'base'):
        result[side] = {key: pr[side][key] for key in ('sha', 'ref')}
        result[side]['repo'] = {key: pr[side]['repo'][key] for key in ('id', 'full_name')}
    return result


def decision(api, pr, owner, now):
    created, closed = timestamp(pr.get('created_at')), timestamp(pr.get('closed_at'))
    require(created <= closed <= now, 'Invalid declined proposal lifetime')
    rows = api.pages('/issues/%d/events' % pr['number'])
    decisions, identifiers, previous = [], set(), None
    for row in rows:
        require(type(row) is dict and positive(row.get('id')) and row['id'] not in identifiers
                and type(row.get('event')) is str, 'Malformed complete owner decision inventory')
        identifiers.add(row['id'])
        when = timestamp(row.get('created_at'))
        require(created <= when <= now and (previous is None or previous <= when),
                'Owner decision chronology changed or malformed')
        previous = when
        if row['event'] in {'closed', 'reopened'}:
            decisions.append(row)
    require(decisions and decisions[-1]['event'] == 'closed'
            and timestamp(decisions[-1]['created_at']) == closed
            and type(decisions[-1].get('actor')) is dict
            and all(decisions[-1]['actor'].get(key) == owner[key] for key in ('id', 'login', 'type')),
            'Latest close decision is not the actual repository owner')
    latest = decisions[-1]
    return {**{key: latest[key] for key in ('id', 'event', 'created_at')},
            'actor': {key: latest['actor'][key] for key in ('id', 'login', 'type')}}


def native_history(api, repo_id):
    stale_proofs = {}
    if pipeline.active_approval(api, stale_proofs):
        return None
    workflow_id = registered(api, pipeline.WORKFLOW, 'Dependency Approve Merge and Deploy')
    rows = api.pages('/actions/workflows/' + pipeline.WORKFLOW + '/runs?event=workflow_dispatch',
                     'workflow_runs', require_total=True)
    identifiers, waiting = set(), set()
    for row in rows:
        require(type(row) is dict and positive(row.get('id')) and row['id'] not in identifiers,
                'Malformed or duplicate consumed native request inventory')
        identifiers.add(row['id'])
        title = row.get('display_title')
        require(type(title) is str and re.fullmatch(r'Dependency PR [1-9][0-9]{0,6} [0-9a-f]{40} [0-9a-f]{40}', title),
                'Malformed native request title')
        # This inventory protects every previously requested PR. It grants no
        # execution authority: an owner dispatch or later manual rerun still
        # consumes that PR's request without blocking unrelated proposals.
        require(type(row.get('workflow_id')) is int and row['workflow_id'] == workflow_id
                and row.get('path') in {'.github/workflows/' + pipeline.WORKFLOW,
                                        '.github/workflows/' + pipeline.WORKFLOW + '@main'}
                and row.get('event') == 'workflow_dispatch' and row.get('head_branch') == 'main'
                and row.get('head_sha') == title.split()[-1] and positive(row.get('run_attempt')),
                'Consumed native request workflow/source/status mismatch')
        repository(row.get('repository'), api.repo, repo_id)
        repository(row.get('head_repository'), api.repo, repo_id)
        if row.get('status') == 'waiting':
            # A stale gate is idle, but its PR still consumed a native request.
            # Bind the exception to this fresh inventory and the same complete
            # authority proof; never exempt an arbitrary waiting history row.
            proof = stale_proofs.get(row['id'])
            require(proof is not None and pipeline.stale_waiting(api, row) == proof,
                    'Waiting native request proof changed during history readback')
            waiting.add(row['id'])
        else:
            require(row.get('status') == 'completed'
                    and row.get('conclusion') in {'success', 'failure', 'cancelled', 'timed_out',
                                                 'neutral', 'skipped', 'action_required', 'stale', 'startup_failure'},
                    'Consumed native request workflow/source/status mismatch')
    require(waiting == set(stale_proofs), 'Waiting native request inventory changed during history readback')
    return rows


def requested(rows, number):
    return any(row['display_title'].startswith('Dependency PR %d ' % number) for row in rows)


def git_environment():
    env = {key: value for key, value in os.environ.items()
           if not key.startswith('GIT_')}
    env.update(GIT_CONFIG_NOSYSTEM='1', GIT_CONFIG_GLOBAL='/dev/null', GIT_TERMINAL_PROMPT='0')
    return env


def validate_ref(branch):
    require(type(branch) is str and re.fullmatch(r'renovate-ci/[a-zA-Z0-9_.-]{1,120}', branch)
            and not branch.startswith('renovate-ci/approval-probe-'), 'Invalid cleanup branch')
    try:
        result = subprocess.run(['git', 'check-ref-format', 'refs/heads/' + branch],
                                shell=False, capture_output=True, timeout=10, env=git_environment())
    except (OSError, subprocess.TimeoutExpired):
        raise Refused('Git ref validation unavailable') from None
    require(result.returncode == 0, 'Invalid cleanup Git ref')


def delete_branch(api, branch, head):
    validate_ref(branch)
    require(type(head) is str and SHA.fullmatch(head)
            and re.fullmatch(r'[a-zA-Z0-9_.-]+/[a-zA-Z0-9_.-]+', api.repo), 'Invalid branch deletion identity')
    ref = 'refs/heads/' + branch
    # REST DELETE cannot compare a ref atomically. Git receive-pack applies
    # this exact SHA lease, so a changed head survives even after PR close.
    with tempfile.TemporaryDirectory(prefix='sifto-declined-') as directory:
        try:
            init = subprocess.run(['git', 'init', '--bare', directory], shell=False,
                                  capture_output=True, timeout=10, env=git_environment())
            require(init.returncode == 0, 'Cleanup Git initialization failed')
            result = subprocess.run(['git', '-C', directory, '-c', 'core.hooksPath=/dev/null',
                                     '-c', 'credential.helper=', '-c', 'credential.helper=!gh auth git-credential',
                                     'push', 'https://github.com/' + api.repo + '.git',
                                     '--force-with-lease=' + ref + ':' + head, ':' + ref],
                                    shell=False, capture_output=True, timeout=60, env=git_environment())
        except (OSError, subprocess.TimeoutExpired):
            return 'unconfirmed'
    return 'deleted' if result.returncode == 0 else 'unconfirmed'


def cleanup(api, context, now=None):
    fixed_now = now
    now = now or datetime.now(timezone.utc)
    trusted = authority(api, context, now)
    repo_id, source, started, owner, _ = trusted
    history = native_history(api, repo_id)
    if history is None:
        print('Native dependency approval or deployment is active; declined cleanup skipped')
        return
    old_inventory = api.pages('/pulls?state=closed&base=main')
    open_inventory = api.pages('/pulls?state=open&base=main')
    require(len(open_inventory) <= pipeline.MAX_FILES, 'Too many open proposals; cleanup requires a bounded inventory')
    for listed in open_inventory:
        try:
            number = listed.get('number')
            require(type(number) is int and 0 < number < 10_000_000, 'Invalid current proposal number')
            current = api.get('/pulls/%d' % number)
            branch = proposal(current, api.repo, repo_id, 'open')
            require(current['number'] == number, 'Current proposal lookup number mismatch')
            created = timestamp(current.get('created_at'))
            require(started < created <= now and current['base']['sha'] == source,
                    'Proposal predates this writer or uses stale main')
            require(not requested(history, number), 'Proposal already entered native approval')
            current_snapshot = proposal_snapshot(current, api.repo, repo_id, 'open')
            files = inventory(api, current)
            matched = None
            priors = [prior for prior in old_inventory if prior.get('number') != number
                      and prior.get('title') == current['title'] and prior.get('head', {}).get('ref') == branch]
            require(len(priors) <= pipeline.MAX_FILES, 'Too many matching declined proposals; split cleanup audit')
            for prior in priors:
                try:
                    old = api.get('/pulls/%d' % prior['number'])
                    proposal(old, api.repo, repo_id, 'closed')
                    require(old['number'] == prior['number'], 'Declined proposal lookup number mismatch')
                    require(old['title'] == current['title'] and old['head']['ref'] == branch
                            and timestamp(old['closed_at']) < created, 'Declined identity/lifetime mismatch')
                    vote = decision(api, old, owner, now)
                    if inventory(api, old) == files:
                        matched = (old, vote)
                        break
                except Refused:
                    continue
            if matched is None:
                continue
            old, vote = matched
            old_snapshot = proposal_snapshot(old, api.repo, repo_id, 'closed')
            validate_ref(branch)
            now = fixed_now or datetime.now(timezone.utc)
            require(authority(api, context, now) == trusted, 'Trusted writer changed before cleanup')
            final_history = native_history(api, repo_id)
            require(final_history is not None and not requested(final_history, number), 'Native approval raced cleanup')
            observed_old = api.get('/pulls/%d' % old['number'])
            require(proposal_snapshot(observed_old, api.repo, repo_id, 'closed') == old_snapshot
                    and decision(api, observed_old, owner, now) == vote and inventory(api, observed_old) == files,
                    'Owner declined proposal changed before cleanup')
            observed_current = api.get('/pulls/%d' % number)
            require(proposal_snapshot(observed_current, api.repo, repo_id, 'open') == current_snapshot
                    and inventory(api, observed_current) == files,
                    'Current proposal changed before cleanup')
            require(api.get('/git/ref/heads/' + quote(branch, safe='/')).get('object', {}).get('sha') == current['head']['sha'],
                    'Proposal branch head changed before cleanup')
            current_main(api, source)
            require((fixed_now or datetime.now(timezone.utc)) <= started + timedelta(minutes=45),
                    'Cleanup writer expired immediately before mutation')
        except Refused as exc:
            print('Declined cleanup skipped: %s' % exc)
            continue
        # There is no retry after any unknown or rejected mutation. A later
        # writer cannot replay it because this PR predates that writer's start.
        try:
            result = api.request('PATCH', '/pulls/%d' % number, {'state': 'closed'})
        except Ambiguous:
            state = api.get('/pulls/%d' % number)
            print('Declined close unconfirmed; audit state=%s; no replay or branch deletion' % state.get('state'))
            raise Refused('Declined close acknowledgement lost; audit required') from None
        require(type(result) is dict and result.get('number') == number and result.get('state') == 'closed'
                and result.get('merged') is False, 'Declined close acknowledgement rejected')
        closed = api.get('/pulls/%d' % number)
        closed_snapshot = proposal_snapshot(closed, api.repo, repo_id, 'closed')
        immutable = set(current_snapshot) - {'state', 'closed_at'}
        require(all(closed_snapshot[key] == current_snapshot[key] for key in immutable),
                'Closed proposal changed; branch deletion refused')
        # The acknowledged close consumes the mutation; these are fresh read
        # authorities for the separate Git mutation. A native request or owner
        # decision can change while PATCH and its readback are in flight.
        now = fixed_now or datetime.now(timezone.utc)
        require(authority(api, context, now) == trusted,
                'Trusted writer changed after close; branch retained')
        observed_old = api.get('/pulls/%d' % old['number'])
        require(proposal_snapshot(observed_old, api.repo, repo_id, 'closed') == old_snapshot
                and decision(api, observed_old, owner, now) == vote and inventory(api, observed_old) == files,
                'Owner declined proposal changed after close; branch retained')
        closed = api.get('/pulls/%d' % number)
        require(proposal_snapshot(closed, api.repo, repo_id, 'closed') == closed_snapshot
                and timestamp(closed['closed_at']) <= now,
                'Closed proposal changed during deletion validation; branch retained')
        require(inventory(api, closed) == files,
                'Closed proposal file inventory changed; branch retained')
        require(api.get('/git/ref/heads/' + quote(branch, safe='/')).get('object', {}).get('sha') == current['head']['sha'],
                'Proposal branch head changed after close; branch retained')
        deletion_history = native_history(api, repo_id)
        require(deletion_history is not None and not requested(deletion_history, number),
                'Native approval started after close; branch retained')
        current_main(api, source)
        require((fixed_now or datetime.now(timezone.utc)) <= started + timedelta(minutes=45),
                'Cleanup writer expired before branch deletion; branch retained')
        outcome = delete_branch(api, branch, current['head']['sha'])
        print('Exact owner-declined PR %d closed; branch deletion %s' % (number, outcome))
        if outcome != 'deleted':
            # Audit only; readback never grants permission to retry deletion.
            try:
                ref = api.get('/git/ref/heads/' + quote(branch, safe='/'))
                print('Branch deletion audit head=%s; no retry' % ref.get('object', {}).get('sha'))
            except Refused:
                print('Branch deletion audit unavailable; no retry')
            raise Refused('Branch deletion unconfirmed; audit required, no retry')


def entrypoint():
    cleanup(GitHub(os.environ['GITHUB_REPOSITORY']), os.environ)


if __name__ == '__main__':
    try:
        entrypoint()
    except Exception as exc:
        print(str(exc) if isinstance(exc, Refused) else 'Malformed declined proposal data; stopped')
        raise SystemExit(1) from None
