"""TEMPORARY: harmless GITHUB_TOKEN dispatch and completion delivery diagnostic.

Retire with its four workflows after observing static versus dynamic filters.
This module never consults native approval or invokes dependency automation.
"""
import os
import re

from .followup import bot, context_id, positive, read_event, registered, repository
from .github import Ambiguous, GitHub
from .policy import Refused, SHA, require

FILES = {kind: 'dependency-completion-probe-' + kind + '.yml'
         for kind in ('caller', 'parent', 'static', 'dynamic')}
NAMES = {'caller': 'Dependency Completion Probe Caller',
         'parent': 'Dependency Completion Delivery Probe',
         'static': 'Dependency Completion Probe Static Receiver',
         'dynamic': 'Dependency Completion Probe Dynamic Receiver'}


def owner_actor(actor, owner):
    require(type(actor) is dict and type(actor.get('id')) is int
            and actor.get('id') == owner['id'] and actor.get('login') == owner['login']
            and actor.get('type') == 'User', 'Probe requires the actual repository owner')


def actor_identity(actor, owner, mode):
    if mode == 'owner':
        owner_actor(actor, owner)
    elif mode == 'bot':
        bot(actor)
    else:
        if type(actor) is dict and actor.get('type') == 'User':
            owner_actor(actor, owner)
        else:
            bot(actor)


def run_identity(run, expected, api, repo_id, owner, mode):
    # REST run paths can have @main; registered workflow metadata stays exact.
    require(type(run) is dict and positive(run.get('id')) and positive(run.get('workflow_id'))
            and type(run.get('run_attempt')) is int and run['run_attempt'] == 1
            and all(run.get(key) in (value, value + '@main') if key == 'path'
                    else run.get(key) == value for key, value in expected.items()),
            'Probe run identity or first-attempt state mismatch')
    repository(run.get('repository'), api.repo, repo_id)
    repository(run.get('head_repository'), api.repo, repo_id)
    actor_identity(run.get('actor'), owner, mode)
    actor_identity(run.get('triggering_actor'), owner, mode)


def expected_run(api, kind, identifier, source, title, status, conclusion):
    return {'id': identifier, 'workflow_id': registered(api, FILES[kind], NAMES[kind]),
            'path': '.github/workflows/' + FILES[kind], 'head_branch': 'main',
            'head_sha': source, 'run_attempt': 1, 'display_title': title,
            'event': 'workflow_dispatch' if kind in ('caller', 'parent') else 'workflow_run',
            'status': status, 'conclusion': conclusion}


def current_main(api, source):
    branch = api.get('/branches/main')
    require(type(branch) is dict and branch.get('name') == 'main'
            and branch.get('protected') is True and type(branch.get('commit')) is dict
            and branch['commit'].get('sha') == source, 'Probe requires protected current main')
    ref = api.get('/git/ref/heads/main')
    require(type(ref) is dict and type(ref.get('object')) is dict
            and ref['object'].get('sha') == source, 'Main advanced during completion probe')


def first_run(api, kind, identifier, title):
    runs = api.pages('/actions/workflows/' + FILES[kind] + '/runs', 'workflow_runs', require_total=True)
    identifiers, matches = set(), []
    for run in runs:
        require(type(run) is dict and positive(run.get('id')) and run['id'] not in identifiers
                and type(run.get('display_title')) is str and len(run['display_title']) <= 300,
                'Malformed probe workflow history')
        identifiers.add(run['id'])
        if run['display_title'] == title:
            matches.append(run['id'])
    require(identifier in matches and min(matches) == identifier,
            'Probe already consumed or first execution missing from workflow history')


def caller(api, caller_id, source, repo_id, owner):
    run = api.get('/actions/runs/' + str(caller_id))
    require(type(run) is dict and (run.get('status'), run.get('conclusion'))
            in (('in_progress', None), ('completed', 'success')),
            'Probe caller must be in progress or completed successfully')
    title = 'Completion delivery probe caller ' + source
    expected = expected_run(api, 'caller', caller_id, source, title, run['status'], run['conclusion'])
    run_identity(run, expected, api, repo_id, owner, 'owner')
    first_run(api, 'caller', caller_id, title)


def completed_parent(api, event_run, source, repo_id, owner):
    require(type(event_run) is dict and positive(event_run.get('id')), 'Invalid completed probe parent')
    parent_id = event_run['id']
    run = api.get('/actions/runs/' + str(parent_id))
    require(type(run) is dict and type(run.get('display_title')) is str,
            'Missing probe parent marker')
    marker = re.fullmatch(r'Dependency completion probe ([1-9][0-9]{0,19})', run['display_title'])
    require(marker is not None, 'Malformed probe parent caller marker')
    caller_id = int(marker[1])
    conclusion = run.get('conclusion')
    require(type(conclusion) is str and re.fullmatch(r'[a-z_]{1,40}', conclusion),
            'Probe parent must have a terminal conclusion')
    expected = expected_run(api, 'parent', parent_id, source, run['display_title'], 'completed', conclusion)
    run_identity(run, expected, api, repo_id, owner, 'bot')
    run_identity(event_run, expected, api, repo_id, owner, 'bot')
    first_run(api, 'parent', parent_id, run['display_title'])
    caller(api, caller_id, source, repo_id, owner)
    return {'parent_id': parent_id, 'dynamic_name': run['display_title'],
            'source': source, 'conclusion': conclusion, 'caller_id': caller_id}


def execute(api, event, context, kind):
    require(kind in FILES and type(event) is dict, 'Invalid completion probe role or event')
    self_id = context_id(context.get('GITHUB_RUN_ID'))
    repo_id = context_id(context.get('GITHUB_REPOSITORY_ID'))
    source = context.get('GITHUB_SHA')
    require(type(source) is str and SHA.fullmatch(source)
            and context.get('GITHUB_REPOSITORY') == api.repo
            and context.get('GITHUB_REF') == 'refs/heads/main'
            and context.get('GITHUB_RUN_ATTEMPT') == '1'
            and context.get('GITHUB_EVENT_NAME') == ('workflow_dispatch' if kind in ('caller', 'parent') else 'workflow_run')
            and context.get('GITHUB_WORKFLOW_SHA') == source
            and context.get('GITHUB_WORKFLOW_REF') == api.repo + '/.github/workflows/' + FILES[kind] + '@refs/heads/main',
            'Probe requires the first execution of trusted current main')
    repo = api.get('')
    repository(repo, api.repo, repo_id)
    repository(event.get('repository'), api.repo, repo_id)
    owner = repo.get('owner')
    require(type(owner) is dict and positive(owner.get('id')) and owner.get('type') == 'User'
            and owner.get('login') == 'enjoydarts' and api.repo.startswith(owner['login'] + '/'),
            'Probe requires the actual enjoydarts User repository owner')
    inputs = event.get('inputs')
    if kind == 'caller':
        require(type(inputs) is dict and inputs.get('expected_head') == source,
                'Probe caller expected_head must equal the immutable source')
        title = 'Completion delivery probe caller ' + source
        mode = 'owner'
    elif kind == 'parent':
        require(type(inputs) is dict and inputs.get('source_sha') == source,
                'Probe parent source must equal the immutable source')
        caller_id = context_id(inputs.get('caller_run_id'))
        title = 'Dependency completion probe %d' % caller_id
        mode = 'bot'
    else:
        require(event.get('action') == 'completed' and type(event.get('workflow_run')) is dict
                and positive(event['workflow_run'].get('id')), 'Probe receiver requires a completed workflow event')
        title = 'Completion delivery probe %s receiver %d' % (kind, event['workflow_run']['id'])
        mode = 'owner_or_bot'
    expected = expected_run(api, kind, self_id, source, title, 'in_progress', None)
    run = api.get('/actions/runs/' + str(self_id))
    run_identity(run, expected, api, repo_id, owner, mode)
    require(context.get('GITHUB_ACTOR') == run['actor']['login']
            and context_id(context.get('GITHUB_ACTOR_ID')) == run['actor']['id']
            and context.get('GITHUB_TRIGGERING_ACTOR') == run['triggering_actor']['login'],
            'Probe context actor differs from actual run identity')
    current_main(api, source)
    first_run(api, kind, self_id, title)
    if kind == 'caller':
        registered(api, FILES['parent'], NAMES['parent'])
        # Reread immediately before the sole POST. Unknown results consume this
        # run; attempt 2 and a fresh caller for this source are both refused.
        run_identity(api.get('/actions/runs/' + str(self_id)), expected, api, repo_id, owner, mode)
        current_main(api, source)
        try:
            api.request('POST', '/actions/workflows/' + FILES['parent'] + '/dispatches',
                        {'ref': 'main', 'inputs': {'caller_run_id': str(self_id), 'source_sha': source}})
            outcome = 'requested'
        except Ambiguous:
            outcome = 'unconfirmed'
        except Refused:
            outcome = 'rejected'
        return {'kind': kind, 'outcome': outcome, 'caller_id': self_id, 'source': source}
    if kind == 'parent':
        caller(api, caller_id, source, repo_id, owner)
        current_main(api, source)
        return {'kind': kind, 'parent_id': self_id, 'dynamic_name': title,
                'caller_id': caller_id, 'source': source}
    observed = completed_parent(api, event['workflow_run'], source, repo_id, owner)
    current_main(api, source)
    return dict(kind=kind, **observed, self_source=source)


def entrypoint():
    event = read_event(os.environ['GITHUB_EVENT_PATH'])
    result = execute(GitHub(os.environ['GITHUB_REPOSITORY']), event, os.environ,
                     os.environ['COMPLETION_PROBE_KIND'])
    text = 'Temporary completion delivery probe\n\n' + '\n'.join(
        '%s: %s' % (key, value) for key, value in result.items()) + '\n'
    if result['kind'] == 'caller':
        text += ('Do not retry or rerun this caller. Inspect the parent and receiver workflow history '
                 'for this caller_id and source; requested does not confirm delivery.\n')
    print(text.strip())
    with open(os.environ['GITHUB_STEP_SUMMARY'], 'a') as summary:
        summary.write(text)


if __name__ == '__main__':
    try:
        entrypoint()
    except Exception as exc:
        print(str(exc) if isinstance(exc, Refused) else 'Malformed completion probe data; stopped')
        raise SystemExit(1) from None
