"""One data-only Renovate notification after native approval fully completes."""
import os
import re

from .github import Ambiguous, GitHub
from .policy import Refused, SHA, load_json, require, verify_approval

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
    require(type(run) is dict and all(run.get(key) == value for key, value in expected.items())
            and positive(run.get('id')) and positive(run.get('workflow_id'))
            and type(run.get('run_attempt')) is int and run['run_attempt'] == 1,
            'Dependency run identity or first-attempt state mismatch')
    repository(run.get('repository'), repo, repo_id)
    repository(run.get('head_repository'), repo, repo_id)
    bot(run.get('actor'))
    bot(run.get('triggering_actor'))


def parent(api, event_run, parent_id, repo_id, source):
    run = api.get('/actions/runs/' + str(parent_id))
    require(type(run) is dict and type(run.get('display_title')) is str,
            'Missing immutable dependency title')
    title = re.fullmatch(r'Dependency PR ([1-9][0-9]{0,6}) ([0-9a-f]{40}) ([0-9a-f]{40})',
                         run['display_title'])
    require(title is not None, 'Invalid immutable dependency title')
    number, head, base = int(title[1]), title[2], title[3]
    expected = {'id': parent_id, 'workflow_id': registered(api, PARENT_WORKFLOW, PARENT_NAME),
                'display_title': run['display_title'], 'path': '.github/workflows/' + PARENT_WORKFLOW,
                'event': 'workflow_dispatch', 'head_branch': 'main', 'head_sha': base,
                'run_attempt': 1, 'status': 'completed', 'conclusion': 'success'}
    # GitHub run.name can be the dynamic run-name. Registered metadata above
    # supplies the top-level workflow name; neither REST nor event run.name does.
    run_identity(run, expected, api.repo, repo_id)
    run_identity(event_run, expected, api.repo, repo_id)
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
            and job.get('head_sha') == base and job.get('status') == 'completed'
            and job.get('conclusion') == 'success' for job in jobs)
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
    return number, head, base


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
    require(self_id in matches and min(matches) == self_id,
            'Parent follow-up already consumed or current child identity missing')


def notify(api, event, context):
    require(type(event) is dict and event.get('action') == 'completed'
            and type(event.get('workflow_run')) is dict,
            'Only a completed native workflow event may notify')
    parent_id = event['workflow_run'].get('id')
    require(positive(parent_id), 'Invalid completed parent run identifier')
    self_id, repo_id = context_id(context.get('GITHUB_RUN_ID')), context_id(context.get('GITHUB_REPOSITORY_ID'))
    source = context.get('GITHUB_SHA')
    require(type(source) is str and SHA.fullmatch(source)
            and context.get('GITHUB_REPOSITORY') == api.repo
            and context.get('GITHUB_EVENT_NAME') == 'workflow_run'
            and context.get('GITHUB_REF') == 'refs/heads/main'
            and context.get('GITHUB_RUN_ATTEMPT') == '1'
            and context.get('GITHUB_WORKFLOW_SHA') == source
            and context.get('GITHUB_WORKFLOW_REF') == api.repo + '/.github/workflows/' + WORKFLOW + '@refs/heads/main',
            'Follow-up must use the first trusted current-main execution')
    repository(api.get(''), api.repo, repo_id)
    repository(event.get('repository'), api.repo, repo_id)
    expected = {'id': self_id, 'workflow_id': registered(api, WORKFLOW, NAME),
                'display_title': 'Dependency follow-up %d' % parent_id,
                'path': '.github/workflows/' + WORKFLOW, 'event': 'workflow_run',
                'head_branch': 'main', 'head_sha': source, 'run_attempt': 1,
                'status': 'in_progress', 'conclusion': None}
    run_identity(api.get('/actions/runs/' + str(self_id)), expected, api.repo, repo_id)
    approved = parent(api, event['workflow_run'], parent_id, repo_id, source)
    current_main(api, source)
    first_child(api, self_id, parent_id)
    # Read back the whole terminal authority immediately before the only POST.
    # A parent rerun, changed review/jobs or main drift fails closed.
    require(parent(api, event['workflow_run'], parent_id, repo_id, source) == approved,
            'Completed dependency authority changed before notification')
    current_main(api, source)
    try:
        api.request('POST', '/actions/workflows/renovate.yml/dispatches', {'ref': 'main'})
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
