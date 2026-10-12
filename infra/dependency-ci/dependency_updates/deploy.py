"""Immutable production deployment transport with no mutation retries."""
import json
import os
import re
import time
import urllib.error
import urllib.request
from urllib.parse import urlparse
from .github import Ambiguous, GitHub, NoRedirect
from .policy import Refused, SHA, require, verify_approval, surfaces


def validate_dispatch(expected, current_main, event, approved):
    require(bool(SHA.fullmatch(expected)) and expected == current_main, 'Deployment commit must equal exact current main')
    if event == 'workflow_dispatch':
        require(approved == 'true', 'Dependency dispatch requires verified native approval')


def single_request(api, workflow, commit, approval_run, run_id, attempt):
    require(attempt == '1' and workflow in ('deploy.yml', 'deploy-audio-concat.yml'), 'Consumed deployment cannot be rerun')
    title = 'Approved dependency deployment %d %s' % (approval_run, commit)
    runs = api.pages('/actions/workflows/' + workflow + '/runs?event=workflow_dispatch&head_sha=' + commit, 'workflow_runs')
    matches = [run for run in runs if run.get('display_title') == title]
    require(len(matches) == 1 and matches[0].get('id') == run_id, 'Deployment approval already requested or run identity ambiguous')


def authorize(api):
    expected = os.environ['EXPECTED_COMMIT']
    current = api.get('/git/ref/heads/main')['object']['sha']
    event = os.environ['GITHUB_EVENT_NAME']
    if event == 'workflow_dispatch':
        source = os.environ['APPROVED_BASE']
        head = os.environ['APPROVED_HEAD']
        require(all(SHA.fullmatch(s) for s in (expected, source, head)), 'Invalid approval deployment SHA')
        run_id = int(os.environ['APPROVAL_RUN'])
        workflow = os.environ['GITHUB_WORKFLOW_REF'].split('@')[0].rsplit('/', 1)[-1]
        single_request(api, workflow, expected, run_id, int(os.environ['GITHUB_RUN_ID']), os.environ['GITHUB_RUN_ATTEMPT'])
        number = int(os.environ['APPROVED_PR'])
        run = api.get('/actions/runs/' + str(run_id))
        title = 'Dependency PR %d %s %s' % (number, head, source)
        require(run.get('head_sha') == source and run.get('head_branch') == 'main' and run.get('path') == '.github/workflows/dependency-update.yml' and run.get('event') == 'workflow_dispatch' and run.get('run_attempt') == 1 and run.get('display_title') == title, 'Deployment approval source mismatch')
        verify_approval(api.get('/actions/runs/%d/approvals' % run_id), run.get('actor', {}).get('login'))
        pr = api.get('/pulls/' + str(number))
        require(pr.get('merged') is True and pr.get('merge_commit_sha') == expected and pr.get('head', {}).get('sha') == head and pr.get('head', {}).get('repo', {}).get('full_name') == api.repo and pr.get('base', {}).get('ref') == 'main', 'Deployment is not the approved merged PR')
        commit = api.get('/git/commits/' + expected)
        require([p['sha'] for p in commit.get('parents', [])] == [source, head], 'Approved merge parent mismatch')
    validate_dispatch(expected, current, event, 'true')
    require(os.environ['GITHUB_REF'] == 'refs/heads/main' and os.environ['GITHUB_SHA'] == expected, 'Deployment workflow must originate from exact merged main')
    return expected


def request_json(origin, token):
    require(urlparse(origin).scheme == 'https' and urlparse(origin).hostname and not urlparse(origin).username and not urlparse(origin).query and not urlparse(origin).fragment, 'Invalid HTTPS deployment API origin')
    def request(method, path, body=None):
        require(path.startswith('/') and not path.startswith('//'), 'Invalid deployment API path')
        for attempt in range(3 if method == 'GET' else 1):
            try:
                req = urllib.request.Request(origin.rstrip('/') + path, method=method, data=None if body is None else json.dumps(body).encode(), headers={'Authorization': 'Bearer ' + token, 'Content-Type': 'application/json'})
                with urllib.request.build_opener(NoRedirect).open(req, timeout=30) as response:
                    data = response.read(5_000_001)
                    require(len(data) <= 5_000_000, 'Deployment response too large')
                    return json.loads(data) if data else None
            except urllib.error.HTTPError as exc:
                if method != 'GET' or exc.code not in (429, 502, 503, 504):
                    raise Refused('Deployment API request rejected (HTTP %d)' % exc.code) from None
            except (TimeoutError, OSError, ValueError):
                if method != 'GET':
                    raise Ambiguous('Deployment mutation outcome unknown; no replay') from None
            if attempt == 2:
                raise Refused('Deployment read unavailable') from None
            time.sleep(2)
    return request


def source_fields(app):
    require(isinstance(app.get('git_commit_sha'), str) and app.get('git_branch') == 'main', 'Coolify source is not the expected main application')
    return {key: app.get(key) for key in ('git_commit_sha', 'git_branch', 'git_repository', 'source_id')}


def coolify(request, app_id, commit, health, sleep=time.sleep):
    require(re.fullmatch(r'[a-zA-Z0-9_-]{1,100}', app_id) and SHA.fullmatch(commit), 'Invalid immutable Coolify deployment')
    path = '/applications/' + app_id
    original = source_fields(request('GET', path))
    pinned = dict(original, git_commit_sha=commit)
    # Coolify v4.4.2 supports application git_commit_sha, not POST /deploy SHA.
    # Pin source and verify readback before consuming exactly one deployment.
    request('PATCH', path, {'git_commit_sha': commit})
    require(source_fields(request('GET', path)) == pinned, 'Coolify source pin did not persist')
    consumed = False
    try:
        consumed = True
        result = request('POST', '/deploy', {'uuid': app_id, 'force': False})
        require(isinstance(result, dict), 'Coolify deployment acknowledgement missing')
        entries = result.get('deployments', [])
        deployment = result.get('deployment_uuid') or (entries[0].get('deployment_uuid') if len(entries) == 1 else None)
        require(isinstance(deployment, str) and re.fullmatch(r'[a-zA-Z0-9_-]{1,100}', deployment), 'Coolify deployment UUID missing')
        for _ in range(120):
            state = request('GET', '/deployments/' + deployment)
            status = state.get('status')
            if status in ('finished', 'success'):
                require(state.get('commit') == commit and state.get('application', {}).get('uuid') == app_id, 'Coolify built a different commit or application')
                health()
                break
            require(status not in ('failed', 'cancelled', 'cancelled_by_user'), 'Coolify deployment failed')
            sleep(10)
        else:
            raise Refused('Timed out waiting for immutable Coolify deployment')
    except Ambiguous:
        # Leave the immutable pin intact until the consumed deployment is audited.
        raise Refused('Coolify deployment acknowledgement lost; audit required, no replay') from None
    except Refused:
        # A consumed failed/timed-out operation may still be running. Do not
        # change its source or retry it without separate operator investigation.
        raise
    if consumed:
        require(source_fields(request('GET', path)) == pinned, 'Foreign Coolify source change detected; restore refused')
        request('PATCH', path, {'git_commit_sha': original['git_commit_sha']})
        require(source_fields(request('GET', path)) == original, 'Coolify source restore did not persist')


def health(url, commit=None):
    for _ in range(30):
        try:
            with urllib.request.build_opener(NoRedirect).open(url, timeout=10) as response:
                data = json.loads(response.read(100_001))
                if response.status == 200 and (commit is None or data.get('commit') == commit):
                    return
        except (OSError, ValueError):
            pass
        time.sleep(10)
    raise Refused('Production health or served commit verification failed')


def verify_vercel_alias(request, team, domain, deployment, project):
    alias = request('GET', '/v4/aliases/' + domain + '?teamId=' + team)
    require(alias.get('deploymentId') == deployment and alias.get('projectId') == project and alias.get('alias') == domain and not alias.get('redirect'), 'Production Web alias points to another deployment, project or redirect')
    # Deployment API proves commit and alias identity; the public origin must
    # also answer HTTPS successfully after promotion.
    for _ in range(30):
        try:
            with urllib.request.urlopen('https://' + domain, timeout=10) as response:
                if response.status == 200:
                    return
        except OSError:
            pass
        time.sleep(10)
    raise Refused('Production Web origin is unhealthy')


def vercel(request, team, project, repo, commit, sleep=time.sleep, verify_alias=None):
    require(all(re.fullmatch(r'[a-zA-Z0-9_-]{1,100}', value) for value in (team, project)) and SHA.fullmatch(commit), 'Invalid Vercel identifiers')
    require(re.fullmatch(r'[a-zA-Z0-9_.-]+/[a-zA-Z0-9_.-]+', repo), 'Invalid Vercel source repository')
    owner, repository = repo.split('/')
    configuration = request('GET', '/v9/projects/' + project + '?teamId=' + team)
    require(configuration.get('id') == project and re.fullmatch(r'[a-z0-9-]{1,100}', configuration.get('name', '')), 'Vercel project identity mismatch')
    result = request('POST', '/v13/deployments?teamId=' + team, {'name': configuration['name'], 'project': project, 'target': 'production', 'gitSource': {'type': 'github', 'org': owner, 'repo': repository, 'sha': commit}})
    deployment = result.get('id')
    require(isinstance(deployment, str) and re.fullmatch(r'dpl_[a-zA-Z0-9_-]{1,100}', deployment), 'Vercel deployment acknowledgement missing')
    for _ in range(120):
        state = request('GET', '/v13/deployments/' + deployment + '?teamId=' + team)
        require(state.get('readyState') not in ('ERROR', 'CANCELED'), 'Vercel deployment failed')
        if state.get('readyState') == 'READY':
            require(state.get('target') == 'production' and state.get('projectId') == project and (state.get('gitSource', {}).get('sha') == commit or state.get('meta', {}).get('githubCommitSha') == commit), 'Vercel deployed a different commit, project or target')
            require('sifto.net' in state.get('alias', []), 'Vercel production alias has not been promoted')
            (verify_alias or (lambda team_id, domain, identity: verify_vercel_alias(request, team_id, domain, identity, project)))(team, 'sifto.net', deployment)
            return deployment
        sleep(10)
    raise Refused('Timed out waiting for exact Vercel deployment')


def entrypoint():
    import sys
    api = GitHub(os.environ['GITHUB_REPOSITORY'])
    commit = authorize(api)
    mode = sys.argv[1]
    if mode == 'validate':
        if os.environ['GITHUB_EVENT_NAME'] == 'workflow_dispatch':
            files = api.pages('/pulls/' + str(int(os.environ['APPROVED_PR'])) + '/files')
            targets = surfaces([entry['filename'] for entry in files])
        else:
            targets = ['api', 'worker']
        with open(os.environ['GITHUB_OUTPUT'], 'a') as output:
            output.write('commit=' + commit + '\n')
            for target in ('api', 'worker', 'web'):
                output.write('deploy_' + target + '=' + str(target in targets).lower() + '\n')
    elif mode in ('worker', 'api'):
        request = request_json(os.environ['COOLIFY_URL'].rstrip('/') + '/api/v1', os.environ['COOLIFY_TOKEN'])
        app = os.environ['COOLIFY_WORKER_UUID' if mode == 'worker' else 'COOLIFY_API_UUID']
        coolify(request, app, commit, lambda: health('https://' + mode + '.sifto.net/health', commit if mode == 'api' else None))
    elif mode == 'web':
        request = request_json('https://api.vercel.com', os.environ['VERCEL_TOKEN'])
        vercel(request, os.environ['VERCEL_ORG_ID'], os.environ['VERCEL_PROJECT_ID'], os.environ['GITHUB_REPOSITORY'], commit)
    else:
        raise Refused('Unsupported deployment command')


if __name__ == '__main__':
    try:
        entrypoint()
    except Exception as exc:
        print(str(exc) if isinstance(exc, Refused) else 'Malformed deployment data; stopped')
        raise SystemExit(1) from None
