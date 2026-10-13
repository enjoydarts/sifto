"""One data-only Renovate notification after native approval fully completes."""
import os
import re
import time

from .github import Ambiguous, GitHub
from .policy import Refused, SHA, canonical, load_json, require, verify_approval

WORKFLOW = 'dependency-followup.yml'
PARENT_WORKFLOW = 'dependency-update.yml'
PARENT_NAME = 'Dependency Approve Merge and Deploy'
NAME = 'Dependency Completed Follow-up'
BOT_ID = 41898282


def positive(value):
    return type(value) is int and 0 < value < 10 ** 20


def context_id(value):
    require(type(value) is str and re.fullmatch(r'[1-9][0-9]{0,19}', value),
            'Invalid follow-up execution identifier')
    return int(value)


def repository(value, repo, repo_id):
    require(type(value) is dict and type(value.get('id')) is int
            and value['id'] == repo_id and value.get('full_name') == repo,
            'Follow-up repository identity mismatch')


def bot(value):
    require(type(value) is dict and type(value.get('id')) is int
            and value['id'] == BOT_ID and value.get('login') == 'github-actions[bot]'
            and value.get('type') == 'Bot', 'Dependency actor must be the canonical Actions Bot')


def registered(api, filename, name):
    workflow = api.get('/actions/workflows/' + filename)
    require(type(workflow) is dict and positive(workflow.get('id'))
            and workflow.get('name') == name
            and workflow.get('path') == '.github/workflows/' + filename
            and workflow.get('state') == 'active', 'Registered dependency workflow mismatch')
    return workflow['id']


def run_identity(run, expected, repo, repo_id):
    # REST run paths may include the documented @main suffix. Registered
    # workflow paths stay exact; branch, source and workflow ID still bind main.
    require(type(run) is dict and all(
            run.get(key) in (value, value + '@main') if key == 'path' else run.get(key) == value
            for key, value in expected.items())
            and positive(run.get('id')) and positive(run.get('workflow_id'))
            and type(run.get('run_attempt')) is int and run['run_attempt'] == 1,
            'Dependency run identity or first-attempt state mismatch')
    repository(run.get('repository'), repo, repo_id)
    repository(run.get('head_repository'), repo, repo_id)
    bot(run.get('actor'))
    bot(run.get('triggering_actor'))


def native_run(api, parent_id, repo_id):
    run = api.get('/actions/runs/' + str(parent_id))
    require(type(run) is dict and type(run.get('display_title')) is str,
            'Missing immutable dependency title')
    number, head, base = parse_title(run)
    expected = {'id': parent_id, 'workflow_id': registered(api, PARENT_WORKFLOW, PARENT_NAME),
                'display_title': run['display_title'], 'path': '.github/workflows/' + PARENT_WORKFLOW,
                'event': 'workflow_dispatch', 'head_branch': 'main', 'head_sha': base,
                'run_attempt': 1}
    # GitHub run.name can be the dynamic run-name. Registered metadata above
    # supplies the top-level workflow name; neither REST nor event run.name does.
    run_identity(run, expected, api.repo, repo_id)
    return run, number, head, base


def immutable_run(run):
    proof = {key: run.get(key) for key in ('id', 'workflow_id', 'display_title', 'event',
             'head_branch', 'head_sha', 'run_attempt')}
    for key in ('repository', 'head_repository'):
        proof[key] = identity(run[key], ('id', 'full_name'))
    for key in ('actor', 'triggering_actor'):
        proof[key] = identity(run[key], ('id', 'login', 'type'))
    proof['path'] = run['path'].removesuffix('@main')
    return proof


def identity(value, keys):
    return {key: value.get(key) for key in keys}


def context_actor(context, run):
    require(context.get('GITHUB_ACTOR') == run['actor']['login']
            and context_id(context.get('GITHUB_ACTOR_ID')) == run['actor']['id']
            and context.get('GITHUB_TRIGGERING_ACTOR') == run['triggering_actor']['login'],
            'Follow-up context actor differs from actual canonical Bot run')


def trusted_context(api, context, workflow, source):
    require(type(source) is str and SHA.fullmatch(source)
            and context.get('GITHUB_REPOSITORY') == api.repo
            and context.get('GITHUB_EVENT_NAME') == 'workflow_dispatch'
            and context.get('GITHUB_REF') == 'refs/heads/main'
            and context.get('GITHUB_RUN_ATTEMPT') == '1'
            and context.get('GITHUB_SHA') == source
            and context.get('GITHUB_WORKFLOW_SHA') == source
            and context.get('GITHUB_WORKFLOW_REF') == api.repo + '/.github/workflows/' + workflow + '@refs/heads/main',
            'Follow-up requires the first trusted current-main execution context')
    return context_id(context.get('GITHUB_RUN_ID')), context_id(context.get('GITHUB_REPOSITORY_ID'))


def wait_parent(api, parent_id, repo_id, source):
    deadline = time.monotonic() + 600
    pinned = None
    while True:
        run, number, head, base = native_run(api, parent_id, repo_id)
        identity = canonical(immutable_run(run))
        require(pinned is None or pinned == identity, 'Native parent identity changed while waiting')
        pinned = identity
        require(time.monotonic() < deadline, 'Timed out waiting for native parent completion; daily Renovate scan remains fallback')
        if run.get('status') == 'completed':
            require(run.get('conclusion') == 'success', 'Native parent did not complete successfully')
            return run, number, head, base
        require(run.get('status') in {'queued', 'in_progress', 'waiting', 'pending', 'requested'}
                and run.get('conclusion') is None, 'Native parent is not a valid pending execution')
        current_main(api, source)
        remaining = deadline - time.monotonic()
        require(remaining > 0, 'Timed out waiting for native parent completion; daily Renovate scan remains fallback')
        time.sleep(min(60, remaining))


def parent(api, event_run, parent_id, repo_id, source, active=False, plan=None):
    # Keep the prior callable shape for read-only historical audits. A dispatch
    # child supplies the REST-verified run, never a workflow_run event payload.
    run, number, head, base = native_run(api, parent_id, repo_id) if event_run is None else (
        event_run, *parse_title(event_run))
    require(run.get('status') == ('in_progress' if active else 'completed')
            and run.get('conclusion') == (None if active else 'success'),
            'Native parent is not in the required execution state')
    if plan is not None:
        require((number, head, base) == (plan['pr'], plan['head'], plan['base'])
                and base == plan['source'], 'Queued parent does not match the exact approved dependency plan')
    approvals = api.get('/actions/runs/%d/approvals' % parent_id)
    require(type(approvals) is list and len(approvals) <= 100
            and all(type(review) is dict and type(review.get('user')) is dict
                    and type(review.get('environments')) is list
                    and all(type(environment) is dict for environment in review['environments'])
                    for review in approvals), 'Malformed native approval inventory')
    verify_approval(approvals, run['actor']['login'])
    jobs = api.pages('/actions/runs/%d/jobs?filter=all' % parent_id, 'jobs', require_total=True)
    require(len(jobs) == 2 and all(type(job) is dict and positive(job.get('id'))
            and type(job.get('run_id')) is int and job['run_id'] == parent_id
            and type(job.get('run_attempt')) is int and job['run_attempt'] == 1
            and job.get('head_sha') == base
            and job.get('status') == ('in_progress' if active and job.get('name') == 'approve-merge-deploy' else 'completed')
            and job.get('conclusion') == (None if active and job.get('name') == 'approve-merge-deploy' else 'success') for job in jobs)
            and len({job['id'] for job in jobs}) == 2
            and {job.get('name') for job in jobs} == {'prepare', 'approve-merge-deploy'},
            'Complete first-attempt native job success is required')
    pr = api.get('/pulls/' + str(number))
    require(type(pr) is dict and type(pr.get('number')) is int and pr['number'] == number
            and pr.get('state') == 'closed' and pr.get('merged') is True
            and pr.get('merge_commit_sha') == source and type(pr.get('head')) is dict
            and type(pr.get('base')) is dict and pr['head'].get('sha') == head
            and pr['base'].get('ref') == 'main', 'PR is not the exact approved merged dependency')
    repository(pr['head'].get('repo'), api.repo, repo_id)
    repository(pr['base'].get('repo'), api.repo, repo_id)
    branch = pr['head'].get('ref')
    require(type(branch) is str and not branch.startswith('renovate-ci/approval-probe-'),
            'Diagnostic fixture cannot notify dependency follow-up')
    emergency = re.fullmatch(r'dependency-security/[a-z0-9][a-z0-9.-]{0,79}', branch) is not None
    require(emergency or re.fullmatch(r'renovate-ci/[a-zA-Z0-9_.-]{1,120}', branch),
            'Unsupported dependency follow-up branch')
    body, labels = pr.get('body'), pr.get('labels')
    require((body is None or type(body) is str and len(body) <= 65_536)
            and type(labels) is list and len(labels) <= 100
            and all(type(label) is dict and type(label.get('name')) is str
                    and len(label['name']) <= 100 for label in labels),
            'Malformed dependency PR security metadata')
    if emergency:
        author = pr.get('user')
        require(type(author) is dict and author.get('login') == 'enjoydarts'
                and author.get('type') == 'User' and positive(author.get('id'))
                and 'dependency-security' in {label['name'] for label in labels}
                and type(body) is str and 'Security fix:' in body,
                'Emergency follow-up requires existing owner security policy')
    else:
        bot(pr.get('user'))
    commit = api.get('/git/commits/' + source)
    require(type(commit) is dict and commit.get('sha') == source
            and type(commit.get('parents')) is list
            and all(type(item) is dict for item in commit['parents'])
            and [item.get('sha') for item in commit['parents']] == [base, head],
            'Approved immutable merge parent mismatch')
    # Closed PR base.sha can move with main. Only immutable Git parents prove
    # which base/head consumed this native approval.
    if plan is not None:
        require(branch == plan['branch'],
                'Queued parent does not match the exact approved dependency plan')
    from .pipeline import native_authority
    normalized_run = dict(run, path=run['path'].removesuffix('@main'))
    return canonical({'run': immutable_run(run), 'authority': native_authority(normalized_run, approvals),
        'jobs': [{key: job.get(key) for key in ('id', 'name', 'run_id', 'run_attempt', 'head_sha', 'status', 'conclusion')}
                 for job in sorted(jobs, key=lambda job: job['id'])],
        'pr': {**identity(pr, ('number', 'state', 'merged', 'merge_commit_sha')),
               'user': identity(pr['user'], ('id', 'login', 'type')),
               'head': {**identity(pr['head'], ('sha', 'ref')), 'repo': identity(pr['head']['repo'], ('id', 'full_name'))},
               'security': {'emergency': emergency, 'label': emergency and 'dependency-security' in {label['name'] for label in labels},
                            'stated_fix': emergency and type(body) is str and 'Security fix:' in body}},
        'base': {'ref': pr['base']['ref'], 'repo': identity(pr['base']['repo'], ('id', 'full_name'))},
        'merge': {'sha': commit['sha'], 'parents': [item['sha'] for item in commit['parents']]}})


def parse_title(run):
    title = re.fullmatch(r'Dependency PR ([1-9][0-9]{0,6}) ([0-9a-f]{40}) ([0-9a-f]{40})', run['display_title'])
    require(title is not None, 'Invalid immutable dependency title')
    return int(title[1]), title[2], title[3]


def current_main(api, source):
    branch = api.get('/branches/main')
    require(type(branch) is dict and branch.get('name') == 'main'
            and branch.get('protected') is True and type(branch.get('commit')) is dict
            and branch['commit'].get('sha') == source,
            'Follow-up requires protected current merged main')
    ref = api.get('/git/ref/heads/main')
    require(type(ref) is dict and type(ref.get('object')) is dict
            and ref['object'].get('sha') == source, 'Main advanced before Renovate notification')


def first_child(api, self_id, parent_id):
    # Full registered-workflow history deliberately has no head_sha filter:
    # even an earlier failed/unknown child on an older main consumes the chance.
    runs = api.pages('/actions/workflows/' + WORKFLOW + '/runs', 'workflow_runs', require_total=True)
    identifiers = set()
    matches = []
    for run in runs:
        require(type(run) is dict and positive(run.get('id')) and run['id'] not in identifiers
                and type(run.get('display_title')) is str and len(run['display_title']) <= 300,
                'Malformed completed follow-up history')
        identifiers.add(run['id'])
        if run['display_title'] == 'Dependency follow-up %d' % parent_id:
            matches.append(run['id'])
    require((not matches if self_id is None else self_id in matches and min(matches) == self_id),
            'Parent follow-up already consumed or current child identity missing')


def notify(api, event, context):
    require(type(event) is dict and type(event.get('inputs')) is dict
            and set(event['inputs']) == {'parent_run'}, 'Only an exact native-parent dispatch may notify')
    parent_id = context_id(event['inputs']['parent_run'])
    source = context.get('GITHUB_SHA')
    self_id, repo_id = trusted_context(api, context, WORKFLOW, source)
    repository(api.get(''), api.repo, repo_id)
    repository(event.get('repository'), api.repo, repo_id)
    expected = {'id': self_id, 'workflow_id': registered(api, WORKFLOW, NAME),
                'display_title': 'Dependency follow-up %d' % parent_id,
                'path': '.github/workflows/' + WORKFLOW, 'event': 'workflow_dispatch',
                'head_branch': 'main', 'head_sha': source, 'run_attempt': 1,
                'status': 'in_progress', 'conclusion': None}
    self_run = api.get('/actions/runs/' + str(self_id))
    run_identity(self_run, expected, api.repo, repo_id)
    context_actor(context, self_run)
    current_main(api, source)
    run, _, _, _ = wait_parent(api, parent_id, repo_id, source)
    approved = parent(api, run, parent_id, repo_id, source)
    first_child(api, self_id, parent_id)
    # Read back the whole terminal authority immediately before the only POST.
    # A parent rerun, changed review/jobs or main drift fails closed.
    require(parent(api, None, parent_id, repo_id, source) == approved,
            'Completed dependency authority changed before notification')
    require(registered(api, WORKFLOW, NAME) == expected['workflow_id'],
            'Follow-up workflow registration changed before notification')
    require(trusted_context(api, context, WORKFLOW, source) == (self_id, repo_id),
            'Follow-up execution context changed before notification')
    self_run = api.get('/actions/runs/' + str(self_id))
    run_identity(self_run, expected, api.repo, repo_id)
    context_actor(context, self_run)
    repository(api.get(''), api.repo, repo_id)
    first_child(api, self_id, parent_id)
    current_main(api, source)
    try:
        api.request('POST', '/actions/workflows/renovate.yml/dispatches', {'ref': 'main'})
    except Ambiguous:
        return 'unconfirmed'
    except Refused:
        return 'rejected'
    return 'requested'


def queue(api, plan, merged, context):
    """Queue once without waiting: the child must wait for this parent to finish."""
    parent_id, repo_id = trusted_context(api, context, PARENT_WORKFLOW, plan['source'])
    require(type(merged) is str and SHA.fullmatch(merged), 'Invalid merged follow-up source')
    repository(api.get(''), api.repo, repo_id)
    child_workflow = registered(api, WORKFLOW, NAME)
    run, _, _, _ = native_run(api, parent_id, repo_id)
    context_actor(context, run)
    approved = parent(api, run, parent_id, repo_id, merged, active=True, plan=plan)
    first_child(api, None, parent_id)
    current_main(api, merged)
    run, _, _, _ = native_run(api, parent_id, repo_id)
    require(trusted_context(api, context, PARENT_WORKFLOW, plan['source']) == (parent_id, repo_id),
            'Native execution context changed before queueing')
    context_actor(context, run)
    require(parent(api, run, parent_id, repo_id, merged, active=True, plan=plan) == approved,
            'Native dependency authority changed before queueing')
    repository(api.get(''), api.repo, repo_id)
    require(registered(api, WORKFLOW, NAME) == child_workflow,
            'Follow-up workflow registration changed before queueing')
    first_child(api, None, parent_id)
    current_main(api, merged)
    try:
        api.request('POST', '/actions/workflows/' + WORKFLOW + '/dispatches',
                    {'ref': 'main', 'inputs': {'parent_run': str(parent_id)}})
    except Ambiguous:
        return 'unconfirmed'
    except Refused:
        return 'rejected'
    return 'requested'


def read_event(path):
    with open(path, 'rb') as stream:
        raw = stream.read(1_000_001)
    require(len(raw) <= 1_000_000, 'Completed workflow event is too large')
    try:
        event = load_json(raw.decode('utf-8'))
    except (ValueError, UnicodeError):
        raise Refused('Malformed completed workflow event') from None
    require(type(event) is dict, 'Completed workflow event must be an object')
    return event


def entrypoint():
    event = read_event(os.environ['GITHUB_EVENT_PATH'])
    outcome = notify(GitHub(os.environ['GITHUB_REPOSITORY']), event, os.environ)
    text = ('Renovate follow-up notification: %s. The native parent has fully completed. '
            'Dispatch acknowledgement does not prove scan success; audit Renovate run history if unconfirmed or rejected. '
            'No notification replay is permitted; the daily Renovate scan remains the fallback. '
            'Fresh proposals require their own CI and native owner approval.\n') % outcome
    print(text.strip())
    with open(os.environ['GITHUB_STEP_SUMMARY'], 'a') as summary:
        summary.write(text)


if __name__ == '__main__':
    try:
        entrypoint()
    except Exception as exc:
        print(str(exc) if isinstance(exc, Refused) else 'Malformed completed follow-up data; stopped')
        raise SystemExit(1) from None
