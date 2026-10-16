"""Trusted main orchestration: prepare, native approval, merge, explicit deploy."""
import base64
import difflib
import hashlib
import os
import re
import time
from urllib.parse import quote
from .github import Ambiguous
from . import release_age
from .policy import (Refused, SHA, assert_unchanged, canonical, plan_hash, require,
                     validate_change, verify_approval, verify_checks, surfaces)

WORKFLOW = 'dependency-update.yml'
MAX_FILES = 30
# CI jobs run in parallel (25m + 10m queue allowance). Deploy's longest
# job path is validate 5m + migrate 30m + worker/API/web 30m each = 125m;
# Inngest sync's 10m runs alongside web. Both deployments allow 20m for queues.
# Keep dependency-update.yml job timeouts and the DAG contract test aligned.
RUN_WAIT_SECONDS = {
    'dependency-check.yml': 35 * 60,
    'deploy.yml': 145 * 60,
    'deploy-audio-concat.yml': 50 * 60,
}


def main_sha(api):
    return api.get('/git/ref/heads/main')['object']['sha']


def blob(api, path, sha):
    value = api.get('/contents/' + quote(path, safe='/') + '?ref=' + sha)
    require(value.get('type') == 'file' and value.get('encoding') == 'base64' and value.get('size', 2_000_001) <= 2_000_000, 'Missing or oversized candidate blob')
    try:
        return base64.b64decode(value['content'], validate=False).decode('utf-8')
    except (ValueError, UnicodeError):
        raise Refused('Candidate file is not text') from None


def candidate(api, number, head, base, source, python_cutoff=None):
    require(isinstance(number, int) and 0 < number < 10_000_000 and all(SHA.fullmatch(s) for s in (head, base, source)), 'Invalid immutable candidate inputs')
    require(main_sha(api) == base == source, 'Candidate base and trusted workflow source must equal current main')
    pr = api.get('/pulls/' + str(number))
    require(pr.get('state') == 'open' and not pr.get('draft') and not pr.get('merged'), 'Candidate PR is not open')
    require(pr.get('user', {}).get('login') in {'github-actions[bot]', 'enjoydarts'}, 'Unsupported PR author')
    require(pr.get('head', {}).get('repo', {}).get('full_name') == api.repo and pr.get('base', {}).get('repo', {}).get('full_name') == api.repo, 'Fork candidate refused')
    branch = pr.get('head', {}).get('ref', '')
    require(not branch.startswith('renovate-ci/approval-probe-'), 'Diagnostic fixture cannot enter native approval')
    emergency = bool(re.fullmatch(r'dependency-security/[a-z0-9][a-z0-9.-]{0,79}', branch))
    require(bool(re.fullmatch(r'renovate-ci/[a-zA-Z0-9_.-]{1,120}', branch)) or emergency, 'Unsupported dependency branch')
    require(pr['user']['login'] == ('enjoydarts' if emergency else 'github-actions[bot]'), 'PR author does not match its dependency branch namespace')
    if emergency:
        require(pr['user']['login'] == 'enjoydarts' and 'dependency-security' in {l.get('name') for l in pr.get('labels', [])} and 'Security fix:' in pr.get('body', ''), 'Emergency path requires owner, security label and stated fix')
    require(pr['head']['sha'] == head and pr['base']['sha'] == base and pr['base']['ref'] == 'main', 'PR head/base changed')
    comparison = api.get('/compare/' + base + '...' + head)
    require(comparison.get('merge_base_commit', {}).get('sha') == base and comparison.get('status') == 'ahead', 'Candidate must contain exact current main')
    files = api.pages('/pulls/' + str(number) + '/files')
    require(0 < len(files) <= MAX_FILES and len(files) == pr.get('changed_files'), 'Missing, excessive or truncated dependency diff')
    from .policy import PYTHON_INPUTS
    paths = {entry.get('filename') for entry in files}
    require(len(paths) == len(files), 'Duplicate dependency file inventory')
    require(all(path.replace('.in', '.txt') in paths for path in paths & PYTHON_INPUTS),
            'Changed Python input requires its corresponding changed lock file')
    entries = []
    python_age = {'policy': 'emergency-security-bypass' if emergency else 'seven-day',
                  'cutoff': None, 'locks': []}
    metadata = {}
    def fetch_release(name, version):
        key = (name, version)
        if key not in metadata:
            require(len(metadata) < release_age.MAX_PACKAGES,
                    'Changed Python release inventory exceeds policy limit; split the PR')
            metadata[key] = release_age.PyPI().release(name, version)
        return metadata[key]
    for entry in files:
        path = entry.get('filename', '')
        require(entry.get('status') == 'modified' and 'previous_filename' not in entry, 'Dependency deletion/addition/rename refused')
        # Scope is checked before paths are sent to the content API.
        from .policy import DOCKERFILES, COMPOSE, PYTHON_INPUTS, PYTHON_LOCKS, python_lock_summary, npm_lock_summary
        require(path in DOCKERFILES | COMPOSE | PYTHON_INPUTS | PYTHON_LOCKS | {'api/go.mod', 'api/go.sum', 'web/package.json', 'web/package-lock.json'}, 'Unsupported dependency scope; executor activation is separate')
        before, after = blob(api, path, base), blob(api, path, head)
        validate_change(path, before, after)
        if path in PYTHON_LOCKS:
            changes = python_lock_summary(before, after)
            if not emergency:
                if python_age['cutoff'] is None:
                    python_age['cutoff'] = python_cutoff or release_age.release_cutoff()
                evidence = release_age.verify_python_release_age(before, after, python_age['cutoff'], fetch_release)
                # Fixed outputs are bounded. Commit to every verified artifact
                # and timestamp by digest instead of embedding thousands of
                # wheel hashes. Revalidation recomputes this same full digest.
                releases = [{'name': release['name'], 'version': release['version'],
                             'artifact_count': len(release['artifacts']),
                             'latest_upload': release_age.utc_text(max(
                                 release_age.timestamp(artifact['uploaded']) for artifact in release['artifacts'])),
                             'artifacts_sha256': plan_hash(release['artifacts'])}
                            for release in evidence]
                python_age['locks'].append({'path': path, 'releases': releases})
        elif path == 'web/package-lock.json':
            changes = npm_lock_summary(before, after)
        else:
            changes = [line for line in difflib.unified_diff(before.splitlines(), after.splitlines(), n=0) if line.startswith(('+', '-')) and not line.startswith(('+++', '---'))]
        require(sum(len(line) for line in changes) <= 16_000, 'Dependency diff exceeds approval summary limit; split the PR')
        entries.append({'path': path, 'before': hashlib.sha256(before.encode()).hexdigest(), 'after': hashlib.sha256(after.encode()).hexdigest(), 'additions': entry.get('additions'), 'deletions': entry.get('deletions'), 'changes': changes})
    if not emergency:
        statuses = api.pages('/commits/' + head + '/statuses')
        age = next((s for s in statuses if s.get('context') == 'renovate/stability-days'), None)
        require(age is not None and age.get('state') == 'success' and age.get('creator', {}).get('login') == 'github-actions[bot]', 'Renovate seven-day stability status has not succeeded')
        artifacts = next((s for s in statuses if s.get('context') == 'renovate/artifacts'), None)
        if artifacts is not None:
            creator = artifacts.get('creator', {})
            require(artifacts.get('state') == 'success' and creator.get('login') == 'github-actions[bot]'
                    and type(creator.get('id')) is int and creator['id'] == 41898282
                    and creator.get('type') == 'Bot', 'Latest Renovate artifact generation status has not succeeded from canonical Bot')
    targets = surfaces([entry['path'] for entry in entries])
    plan = {'version': 2, 'repo': api.repo, 'pr': number, 'head': head, 'base': base, 'source': source, 'branch': branch, 'emergency': emergency, 'files': entries, 'diff_hash': plan_hash(entries), 'surfaces': targets, 'python_age': python_age}
    require(len(base64.b64encode(canonical(plan).encode())) < 47_900,
            'Python artifact evidence or dependency plan exceeds immutable output limit; split the PR')
    return plan


def run_name(plan):
    return 'Dependency PR %d %s %s' % (plan['pr'], plan['head'], plan['base'])


def native_job_names(api, source):
    """Bind REST job inventory to the immutable trusted workflow source.

    This intentionally recognizes only the historical two-job workflow and
    the current three-job workflow, without adding a runtime YAML dependency.
    Duplicate/unknown job keys or a different jobs block fail closed.
    """
    text = '\n' + blob(api, '.github/workflows/' + WORKFLOW, source)
    require(text.count('\njobs:\n') == 1 and not re.search(r'^jobs:.*\S', text, re.M),
            'Ambiguous native workflow jobs block')
    block = text.split('\njobs:\n', 1)[1]
    require(not re.search(r'^\S', block, re.M), 'Unexpected native workflow jobs boundary')
    matches = list(re.finditer(r'^  ([a-z][a-z-]*):[ \t]*\n', block, re.M))
    names = tuple(match[1] for match in matches)
    require(names in {('prepare', 'approve-merge-deploy'),
                      ('prepare', 'owner-approval', 'approve-merge-deploy')},
            'Unsupported native workflow job shape')
    require(len(re.findall(r'^  \S', block, re.M)) == len(names),
            'Ambiguous native workflow job keys')
    sections = {match[1]: block[match.end():matches[index + 1].start() if index + 1 < len(matches) else len(block)]
                for index, match in enumerate(matches)}
    gate = 'owner-approval' if 'owner-approval' in names else 'approve-merge-deploy'
    expected_needs = {'prepare': [], gate: ['    needs: prepare']}
    if 'owner-approval' in names:
        expected_needs['approve-merge-deploy'] = ['    needs: [prepare, owner-approval]']
    require(all(re.findall(r'^    needs:[^\n]*$', sections[name], re.M) == expected_needs[name]
                for name in names)
            and re.findall(r'^    environment:[^\n]*$', block, re.M)
                == ['    environment: production-maintenance']
            and re.findall(r'^    environment:[^\n]*$', sections[gate], re.M)
                == ['    environment: production-maintenance'],
            'Native owner gate environment or dependency DAG mismatch')
    return set(names)


def registered_native(api, run, repo_id, workflow_id):
    from . import followup
    require(type(run) is dict and type(run.get('display_title')) is str,
            'Missing immutable native dependency title')
    number, head, base = followup.parse_title(run)
    followup.run_identity(run, {'workflow_id': workflow_id,
        'path': '.github/workflows/' + WORKFLOW, 'event': 'workflow_dispatch',
        'head_branch': 'main', 'head_sha': base}, api.repo, repo_id)
    require(followup.positive(run.get('run_number')), 'Invalid native workflow run sequence')
    return number, head, base


def unstarted_job(job):
    return (job.get('steps') == [] and 'runner_id' in job
            and (job['runner_id'] is None or type(job['runner_id']) is int and job['runner_id'] == 0)
            and 'runner_name' in job and job['runner_name'] in (None, ''))


def stale_waiting_snapshot(api, listed):
    """Only positively obsolete, never-reviewed waiting owner gates are idle.

    A populated waiting started_at is ordinary GitHub metadata, not execution.
    No approved, rejected, running, preparing or unknown authority is exempt.
    """
    from . import followup
    if (listed.get('status') != 'waiting' or listed.get('run_attempt') != 1
            or any(key not in listed for key in ('workflow_id', 'repository', 'head_repository',
                                                'actor', 'triggering_actor', 'run_number'))):
        return None
    repository = api.get('')
    require(type(repository) is dict and followup.positive(repository.get('id')),
            'Missing native repository identity')
    followup.repository(repository, api.repo, repository['id'])
    workflow_id = followup.registered(api, WORKFLOW, followup.PARENT_NAME)
    try:
        number, head, base = registered_native(api, listed, repository['id'], workflow_id)
    except Refused:
        return None
    run_id = listed['id']
    current = api.get('/actions/runs/' + str(run_id))
    registered_native(api, current, repository['id'], workflow_id)
    require(followup.immutable_run(current) == followup.immutable_run(listed)
            and current.get('run_number') == listed['run_number'], 'Waiting native run identity changed')
    if current.get('status') != 'waiting' or current.get('conclusion') is not None:
        return None
    if api.get('/actions/runs/%d/approvals' % run_id) != []:
        return None
    names = native_job_names(api, base)
    jobs = api.pages('/actions/runs/%d/jobs?filter=all' % run_id, 'jobs', require_total=True)
    # GitHub does not create downstream jobs while their needs are waiting.
    # The exact authenticated DAG above proves the omitted apply cannot start
    # before this owner gate succeeds. A listed apply must prove it is unstarted.
    observed_names = {job.get('name') for job in jobs if type(job) is dict}
    allowed_names = [names]
    if 'owner-approval' in names:
        allowed_names.append({'prepare', 'owner-approval'})
    if (observed_names not in allowed_names or len(jobs) != len(observed_names)
            or any(type(job) is not dict for job in jobs)
            or not all(followup.positive(job.get('id')) and job.get('run_id') == run_id
                       and type(job.get('run_attempt')) is int and job['run_attempt'] == 1
                       and job.get('head_sha') == base for job in jobs)
            or len({job['id'] for job in jobs}) != len(jobs)):
        return None
    by_name = {job['name']: job for job in jobs}
    prepared = by_name['prepare']
    gate_name = 'owner-approval' if 'owner-approval' in names else 'approve-merge-deploy'
    gate = by_name[gate_name]
    if (prepared.get('status') != 'completed' or prepared.get('conclusion') != 'success'
            or not isinstance(prepared.get('steps'), list) or gate.get('status') != 'waiting'
            or gate.get('conclusion') is not None or not unstarted_job(gate)):
        return None
    if 'owner-approval' in names and 'approve-merge-deploy' in by_name:
        apply = by_name['approve-merge-deploy']
        if (apply.get('status') not in {'queued', 'pending', 'requested'}
                or apply.get('conclusion') is not None or not unstarted_job(apply)
                or 'started_at' not in apply or apply['started_at'] is not None):
            return None
    pending = api.get('/actions/runs/%d/pending_deployments' % run_id)
    if (type(pending) is not list or len(pending) != 1 or type(pending[0]) is not dict):
        return None
    environment, reviewers = pending[0].get('environment'), pending[0].get('reviewers')
    if (type(environment) is not dict or not followup.positive(environment.get('id'))
            or environment.get('name') != 'production-maintenance'
            or type(reviewers) is not list or len(reviewers) != 1 or type(reviewers[0]) is not dict
            or reviewers[0].get('type') != 'User' or type(reviewers[0].get('reviewer')) is not dict
            or any(reviewers[0]['reviewer'].get(key) != value
                   for key, value in {'id': 19749005, 'login': 'enjoydarts', 'type': 'User'}.items())):
        return None
    source = main_sha(api)
    require(type(source) is str and SHA.fullmatch(source), 'Invalid current main for waiting gate')
    pr = api.get('/pulls/' + str(number))
    require(type(pr) is dict and pr.get('number') == number and pr.get('state') == 'open'
            and pr.get('merged') is False and type(pr.get('head')) is dict
            and type(pr.get('base')) is dict, 'Missing current waiting candidate identity')
    followup.repository(pr['head'].get('repo'), api.repo, repository['id'])
    followup.repository(pr['base'].get('repo'), api.repo, repository['id'])
    branch = pr['head'].get('ref', '')
    emergency = re.fullmatch(r'dependency-security/[a-z0-9][a-z0-9.-]{0,79}', branch) is not None
    require((emergency or re.fullmatch(r'renovate-ci/[a-zA-Z0-9_.-]{1,120}', branch))
            and not branch.startswith('renovate-ci/approval-probe-')
            and pr.get('user', {}).get('login') == ('enjoydarts' if emergency else 'github-actions[bot]')
            and pr['base'].get('ref') == 'main'
            and all(type(value) is str and SHA.fullmatch(value)
                    for value in (pr['head'].get('sha'), pr['base'].get('sha'))),
            'Invalid current waiting dependency candidate')
    if (pr['head']['sha'], pr['base']['sha'], source) == (head, base, base):
        return None
    return canonical({'run': followup.immutable_run(current), 'sequence': current['run_number'],
        'jobs': jobs, 'environment': {'id': environment['id'], 'name': environment['name']},
        'reviewer': {key: reviewers[0]['reviewer'][key] for key in ('id', 'login', 'type')},
        'main': source, 'pr': {'number': number, 'head': pr['head']['sha'],
                              'base': pr['base']['sha'], 'branch': branch}})


def stale_waiting(api, run):
    first = stale_waiting_snapshot(api, run)
    if first is None:
        return None
    # Freeze complete authority before exempting, then compare the same proof
    # again at discovery's final dispatch boundary. These are reads, not a lock.
    second = stale_waiting_snapshot(api, run)
    require(second == first, 'Waiting gate or candidate changed during stale readback')
    return first


def require_unsuperseded(api, run, plan):
    """A newer registered same-PR request permanently consumes older authority."""
    from . import followup
    repository = api.get('')
    require(type(repository) is dict and followup.positive(repository.get('id')),
            'Missing native repository identity')
    followup.repository(repository, api.repo, repository['id'])
    workflow_id = followup.registered(api, WORKFLOW, followup.PARENT_NAME)
    require(registered_native(api, run, repository['id'], workflow_id)
            == (plan['pr'], plan['head'], plan['base']), 'Execution native candidate mismatch')
    runs = api.pages('/actions/workflows/' + WORKFLOW + '/runs?event=workflow_dispatch',
                     'workflow_runs', require_total=True)
    identifiers, sequences, found = set(), set(), False
    for other in runs:
        number, _, _ = registered_native(api, other, repository['id'], workflow_id)
        require(other['id'] not in identifiers and other['run_number'] not in sequences,
                'Duplicate native workflow run identity or sequence')
        identifiers.add(other['id'])
        sequences.add(other['run_number'])
        if other['id'] == run['id']:
            require(followup.immutable_run(other) == followup.immutable_run(run)
                    and other['run_number'] == run['run_number'], 'Execution native inventory mismatch')
            found = True
        require(number != plan['pr'] or other['run_number'] <= run['run_number'],
                'Native dependency request superseded by a newer request for this PR')
    require(found, 'Execution native run missing from complete inventory')


def deduplicated(api, plan):
    runs = api.pages('/actions/workflows/' + WORKFLOW + '/runs?event=workflow_dispatch', 'workflow_runs', require_total=True)
    for run in runs:
        if run.get('display_title') == run_name(plan) and not unconsumed_preparation(api, run, plan):
            return True
    return False


def unconsumed_preparation(api, listed, plan):
    """A fresh run is possible only after a provably unapproved prepare failure.

    Completed runs can contain consumed or rejected approvals, cancelled merge
    steps, or previous attempts. None is a request to replay those mutations.
    The first-attempt full job inventory and empty native review history must
    both prove that the privileged job never ran.
    """
    if listed.get('status') != 'completed' or listed.get('conclusion') not in ('failure', 'cancelled', 'timed_out') or listed.get('run_attempt') != 1 or not isinstance(listed.get('id'), int):
        return False
    run_id = listed['id']
    expected = {'id': run_id, 'display_title': run_name(plan), 'status': 'completed', 'conclusion': listed['conclusion'], 'event': 'workflow_dispatch', 'head_sha': plan['source'], 'head_branch': 'main', 'path': '.github/workflows/' + WORKFLOW, 'run_attempt': 1}
    if any(listed.get(key) != value for key, value in expected.items()):
        return False
    current = api.get('/actions/runs/' + str(run_id))
    if any(current.get(key) != value for key, value in expected.items()):
        return False
    reviews = api.get('/actions/runs/%d/approvals' % run_id)
    # No automatic reproposal after denial, owner approval, or ambiguous review
    # metadata. This endpoint returns the whole review history (no pagination).
    if not isinstance(reviews, list) or reviews:
        return False
    jobs = api.pages('/actions/runs/%d/jobs?filter=all' % run_id, 'jobs', require_total=True)
    names = native_job_names(api, plan['source'])
    if len(jobs) != len(names) or {job.get('name') for job in jobs} != names:
        return False
    if not all(type(job.get('id')) is int and job.get('run_id') == run_id and job.get('head_sha') == plan['source'] and job.get('status') == 'completed' for job in jobs) or len({job['id'] for job in jobs}) != len(names):
        return False
    if 'owner-approval' in names and not all(type(job.get('run_attempt')) is int
                                            and job['run_attempt'] == 1 for job in jobs):
        return False
    prepare_job = next(job for job in jobs if job['name'] == 'prepare')
    if prepare_job.get('conclusion') not in ('failure', 'cancelled', 'timed_out') or not isinstance(prepare_job.get('steps'), list):
        return False
    for privileged in (job for job in jobs if job['name'] != 'prepare'):
        if not unstarted_job(privileged):
            return False
        if privileged.get('conclusion') != 'skipped' and not (
                privileged.get('conclusion') == 'cancelled' and 'started_at' in privileged
                and privileged['started_at'] is None):
            return False
    # A reviewer or manual rerun can race the inventory queries. Read back both
    # authorities before permitting a separate fresh run with its own gate.
    final = api.get('/actions/runs/' + str(run_id))
    return all(final.get(key) == value for key, value in expected.items()) and api.get('/actions/runs/%d/approvals' % run_id) == []


def active_approval(api, stale_proofs=None):
    # Read the full workflow inventory, without a head filter that could hide
    # an approval or deployment still running from the previous main.
    runs = api.pages('/actions/workflows/' + WORKFLOW + '/runs?event=workflow_dispatch',
                     'workflow_runs', require_total=True)
    active, identifiers = False, set()
    for run in runs:
        require(type(run) is dict and type(run.get('id')) is int and run['id'] > 0
                and run['id'] not in identifiers and type(run.get('display_title')) is str
                and type(run.get('status')) is str, 'Invalid native approval run inventory')
        identifiers.add(run['id'])
        title = re.fullmatch(r'Dependency PR [1-9][0-9]{0,6} [0-9a-f]{40} ([0-9a-f]{40})',
                             run.get('display_title', ''))
        require(title is not None and title[1] == run.get('head_sha')
                and run.get('event') == 'workflow_dispatch' and run.get('head_branch') == 'main'
                and run.get('path') == '.github/workflows/' + WORKFLOW
                and type(run.get('run_attempt')) is int and run['run_attempt'] > 0
                and run.get('status') in {'completed', 'queued', 'waiting', 'in_progress', 'requested', 'pending'},
                'Native approval run identity or status mismatch')
        if run['status'] != 'completed':
            # After merge, this run keeps its old source while deployment uses
            # the merged main. A coherent old-source run is still ordinary busy.
            require(run['run_attempt'] == 1 and run.get('conclusion') is None,
                    'Active native approval metadata mismatch')
            proof = stale_waiting(api, run)
            if proof is None:
                active = True
            elif stale_proofs is not None:
                stale_proofs[run['id']] = proof
    return active


def discover(api):
    source = main_sha(api)
    require(type(source) is str and SHA.fullmatch(source) and source == os.environ.get('GITHUB_SHA'),
            'Discovery workflow source must equal exact current main')
    stale_proofs = {}
    if active_approval(api, stale_proofs):
        print('Native dependency approval or deployment is active; no new request')
        return
    for pr in api.pages('/pulls?state=open&base=main'):
        if pr.get('head', {}).get('ref', '').startswith('renovate-ci/approval-probe-'):
            continue
        try:
            plan = candidate(api, pr['number'], pr['head']['sha'], source, source)
        except Refused as exc:
            print('PR %d skipped: %s' % (pr['number'], exc))
            continue
        if not deduplicated(api, plan):
            require(main_sha(api) == source, 'Main advanced during dependency discovery')
            final_proofs = {}
            if active_approval(api, final_proofs):
                print('Native dependency approval or deployment became active; no new request')
                return
            require(final_proofs == stale_proofs,
                    'Waiting gate or candidate changed before fresh dispatch')
            api.request('POST', '/actions/workflows/' + WORKFLOW + '/dispatches', {'ref': 'main', 'inputs': {'pr_number': str(plan['pr']), 'head_sha': plan['head'], 'base_sha': source, 'source_sha': source}})
            print('Requested native dependency approval for PR %d' % pr['number'])
            return


def await_run(api, workflow, head, after_id, timeout_seconds, event='workflow_dispatch', remaining_budget=None):
    deadline = time.monotonic() + timeout_seconds
    while time.monotonic() < deadline:
        if remaining_budget is not None:
            remaining_budget()
        runs = api.pages('/actions/workflows/' + workflow + '/runs?event=' + event + '&head_sha=' + head, 'workflow_runs')
        if remaining_budget is not None:
            remaining_budget()
        require(time.monotonic() < deadline, 'Timed out waiting for exact workflow run')
        matches = [r for r in runs if r['id'] > after_id and r.get('head_sha') == head and r.get('event') == event]
        require(len(matches) <= 1, 'Ambiguous workflow run identity')
        if matches and matches[0].get('status') == 'completed':
            require(matches[0].get('conclusion') == 'success', 'Dispatched workflow failed')
            return matches[0]
        remaining = deadline - time.monotonic()
        if remaining_budget is not None:
            remaining = min(remaining, remaining_budget())
        if remaining > 0:
            time.sleep(min(10, remaining))
    raise Refused('Timed out waiting for exact workflow run')


def dispatch_once(api, workflow, ref, head, inputs, remaining_budget=None):
    require(workflow in RUN_WAIT_SECONDS, 'Unsupported workflow wait budget')
    if remaining_budget is not None:
        remaining_budget()
    existing = api.pages('/actions/workflows/' + workflow + '/runs?event=workflow_dispatch&head_sha=' + head, 'workflow_runs')
    latest = max((r['id'] for r in existing), default=0)
    if remaining_budget is not None:
        remaining_budget()
    api.request('POST', '/actions/workflows/' + workflow + '/dispatches', {'ref': ref, 'inputs': inputs})
    if remaining_budget is not None:
        remaining_budget()
    return await_run(api, workflow, head, latest, RUN_WAIT_SECONDS[workflow], remaining_budget=remaining_budget)


def prepare(api, number, head, base, source):
    from . import pr_ci, image_security
    plan = candidate(api, number, head, base, source)
    require(os.environ.get('GITHUB_RUN_ATTEMPT') == '1', 'Reruns require a fresh candidate approval request')
    def revalidate():
        assert_unchanged(plan, candidate(api, number, head, base, source,
                                       python_cutoff=plan['python_age']['cutoff']), plan_hash(plan))
    image_scan = image_security.start(api, plan, 'candidate', head, plan['branch'])
    pr_checks = pr_ci.start(api, plan, revalidate)
    run = dispatch_once(api, 'dependency-check.yml', plan['branch'], head, {'expected_head': head},
                        remaining_budget=pr_checks.remaining)
    verify_checks(run, api.pages('/actions/runs/%d/jobs?filter=latest' % run['id'], 'jobs'), head)
    pr_checks.wait()
    revalidate()
    image_evidence = image_security.wait(api, plan, image_scan)
    revalidate()
    plan['ci_run'] = run['id']
    plan['image_security'] = image_evidence
    digest = plan_hash(plan)
    encoded = base64.b64encode(canonical(plan).encode()).decode()
    require(len(encoded) < 48_000, 'Dependency plan exceeds immutable output limit; split the PR')
    # Immutable job outputs originate only in this trusted prepare job.
    with open(os.environ['GITHUB_OUTPUT'], 'a') as output:
        output.write('plan=' + encoded + '\nplan_hash=' + digest + '\n')
    with open(os.environ['GITHUB_STEP_SUMMARY'], 'a') as summary:
        summary.write('## Dependency candidate requires owner approval\n\n')
        summary.write('PR [%d](https://github.com/%s/pull/%d)\n\n' % (number, api.repo, number))
        for key in ('head', 'base', 'source', 'diff_hash', 'ci_run'):
            summary.write('**%s:** `%s`\n\n' % (key, plan[key]))
        summary.write('**Plan SHA256:** `%s`\n\n' % digest)
        image_security.summary(summary, image_evidence)
        summary.write('**Affected resources:** %s\n\n' % ', '.join(plan['surfaces']))
        summary.write('Operating system package updates require the separate maintenance approval workflow. This dependency plan does not activate OS updates or the pinned live maintenance executor.\n\n')
        if 'development' in plan['surfaces']:
            summary.write('Development environment image references only; live PostgreSQL/Redis/Inngest are not activated by this change.\n\n')
        if 'ci' in plan['surfaces']:
            summary.write('CI/bootstrap test image references apply to subsequent tests; no live executor activation.\n\n')
        summary.write('**Release-age policy:** %s\n\n' % ('Emergency security fix: explicit seven-day bypass; CI and owner gate remain required.' if plan['emergency'] else 'Renovate stability check succeeded; changed Python artifacts independently verified against official PyPI hashes and publication times.'))
        if plan['python_age']['cutoff']:
            summary.write('**Fixed Python publication cutoff (UTC):** `%s`\n\n' % plan['python_age']['cutoff'])
            for lock in plan['python_age']['locks']:
                summary.write('**Python artifact evidence for `%s`:**\n\n' % lock['path'])
                for release in lock['releases']:
                    summary.write('- `%s==%s`: %d introduced artifacts; latest publication `%s`; complete artifact evidence SHA256 `%s`\n' % (
                        release['name'], release['version'], release['artifact_count'],
                        release['latest_upload'], release['artifacts_sha256']))
                summary.write('\n')
        for entry in plan['files']:
            summary.write('- `%s`: +%s / -%s; candidate blob SHA256 `%s`\n' % (entry['path'], entry['additions'], entry['deletions'], entry['after']))
            summary.write('\n```diff\n' + '\n'.join(entry['changes']) + '\n```\n\n')


def merge_once(api, plan):
    current = api.get('/pulls/%d' % plan['pr'])
    branch = api.get('/branches/main')
    require(current.get('state') == 'open' and current.get('merged') is False and current.get('head', {}).get('sha') == plan['head'] and current.get('base', {}).get('sha') == plan['base'] and current.get('base', {}).get('ref') == 'main', 'PR changed immediately before merge')
    require(branch.get('protected') is True and branch.get('commit', {}).get('sha') == plan['base'], 'Protected main base changed immediately before merge')
    # Optional preview failures can mark the PR unstable even after fixed CI
    # passed. The ordinary merge endpoint still enforces all protected rules.
    require(current.get('mergeable') is True and current.get('mergeable_state') in {'clean', 'unstable'}, 'GitHub has not confirmed the candidate is ready to merge')
    try:
        result = api.request('PUT', '/pulls/%d/merge' % plan['pr'], {'sha': plan['head'], 'merge_method': 'merge', 'commit_title': 'Approved dependency update PR #%d' % plan['pr']})
    except Ambiguous:
        # The read is an audit only. An unknown merge commit cannot be promoted
        # to authorization or followed by a deployment after lost acknowledgement.
        state = api.get('/pulls/%d' % plan['pr'])
        print('Merge outcome requires audit; merged=%s; no replay or deployment' % bool(state.get('merged')))
        raise Refused('Merge acknowledgement lost; audit required, no mutation replay') from None
    require(result.get('merged') is True and SHA.fullmatch(result.get('sha', '')), 'Protected branch merge rejected')
    return result['sha']


def native_authority(run, approvals):
    """Freeze only native execution, reviewer and environment authority fields."""
    require(type(approvals) is list and len(approvals) <= 100
            and all(type(review) is dict and type(review.get('state')) is str
                    and type(review.get('user')) is dict and type(review.get('environments')) is list
                    and all(type(environment) is dict and type(environment.get('name')) is str
                            for environment in review['environments']) for review in approvals),
            'Malformed native approval authority')
    def identity(value):
        require(type(value) is dict, 'Malformed native actor authority')
        return {key: value.get(key) for key in ('id', 'login', 'type')}
    proof = {'run': {key: run.get(key) for key in ('id', 'workflow_id', 'run_number', 'display_title', 'event', 'head_branch',
             'head_sha', 'path', 'run_attempt', 'status', 'conclusion')},
             'actor': identity(run.get('actor')),
             'triggering_actor': identity(run.get('triggering_actor', {})),
             'reviews': [{'state': review['state'], 'user': identity(review['user']),
                 'environments': [{key: environment.get(key) for key in ('id', 'name')}
                                  for environment in review['environments']]} for review in approvals]}
    # Materialize now: transports may reuse mutable objects during downloads.
    return canonical(proof)


def execute(api, plan, digest):
    from . import followup, image_security
    require(os.environ.get('GITHUB_RUN_ATTEMPT') == '1', 'Consumed approval cannot be rerun')
    require(plan_hash(plan) == digest and plan['source'] == os.environ['GITHUB_SHA'], 'Untrusted or altered prepared plan')
    run_id = int(os.environ['GITHUB_RUN_ID'])
    run = api.get('/actions/runs/' + str(run_id))
    require(run.get('id') == run_id and run.get('status') == 'in_progress'
            and run.get('conclusion') is None and run.get('event') == 'workflow_dispatch'
            and run.get('head_branch') == 'main' and run.get('head_sha') == plan['source']
            and run.get('path') == '.github/workflows/' + WORKFLOW
            and run.get('run_attempt') == 1, 'Approval run source mismatch')
    approvals = api.get('/actions/runs/%d/approvals' % run_id)
    authority = native_authority(run, approvals)
    verify_approval(approvals, run.get('actor', {}).get('login'))
    require(native_job_names(api, plan['source']) == {'prepare', 'owner-approval', 'approve-merge-deploy'},
            'Execution requires the current separate native owner gate')
    require_unsuperseded(api, run, plan)
    require(plan.get('version') == 2 and type(plan.get('python_age')) is dict,
            'Prepared plan has no fixed Python artifact-age evidence; prepare again')
    require(type(plan.get('image_security')) is dict,
            'Prepared plan has no mandatory candidate image evidence; prepare again')
    current = candidate(api, plan['pr'], plan['head'], plan['base'], plan['source'],
                        python_cutoff=plan['python_age']['cutoff'])
    current['ci_run'] = plan['ci_run']
    current['image_security'] = image_security.revalidate(api, current, plan['image_security'])
    assert_unchanged(plan, current, digest)
    ci = api.get('/actions/runs/%d' % plan['ci_run'])
    verify_checks(ci, api.pages('/actions/runs/%d/jobs?filter=latest' % plan['ci_run'], 'jobs'), plan['head'])
    # Artifact downloads and candidate/CI reads can outlive a native review.
    # Recheck that same authority immediately before the ordinary merge guard.
    final_run = api.get('/actions/runs/' + str(run_id))
    final_approvals = api.get('/actions/runs/%d/approvals' % run_id)
    verify_approval(final_approvals, final_run.get('actor', {}).get('login'))
    require(native_authority(final_run, final_approvals) == authority,
            'Native approval authority changed during revalidation; fresh approval required')
    # Supersession cannot retroactively revoke a legitimate running mutation.
    # The global apply job lock plus exact protected-main/head merge guards
    # serialize writes and prevent another queued request applying stale state.
    require_unsuperseded(api, final_run, plan)
    merged = merge_once(api, plan)
    require(main_sha(api) == merged, 'Main advanced after merge; deployment stopped')
    with open(os.environ['GITHUB_STEP_SUMMARY'], 'a') as summary:
        summary.write('\nMerged commit: `%s`. Merge approval consumed; no replay.\n' % merged)
    approval_inputs = {'expected_commit': merged, 'approval_run': str(run_id), 'approved_pr': str(plan['pr']), 'approved_head': plan['head'], 'approved_base': plan['base']}
    if set(plan['surfaces']) & {'api', 'worker', 'web'}:
        deployment = dispatch_once(api, 'deploy.yml', 'main', merged, approval_inputs)
        require(deployment.get('path') == '.github/workflows/deploy.yml', 'Unexpected deployment workflow')
        with open(os.environ['GITHUB_STEP_SUMMARY'], 'a') as summary:
            summary.write('\nVerified affected application Deploy run [%d](https://github.com/%s/actions/runs/%d) succeeded for `%s`.\n' % (deployment['id'], api.repo, deployment['id'], merged))
    if 'audio' in plan['surfaces']:
        audio = dispatch_once(api, 'deploy-audio-concat.yml', 'main', merged, approval_inputs)
        require(audio.get('path') == '.github/workflows/deploy-audio-concat.yml', 'Unexpected Audio Concat workflow')
        with open(os.environ['GITHUB_STEP_SUMMARY'], 'a') as summary:
            summary.write('\nAudio Concat deployment run %d succeeded for `%s`.\n' % (audio['id'], merged))
    require(main_sha(api) == merged, 'Main advanced after deployment; completion stopped')
    merged_scan = image_security.start(api, plan, 'merged', merged, 'main')
    merged_evidence = image_security.wait(api, plan, merged_scan)
    require(main_sha(api) == merged, 'Main advanced after merged image scan; completion stopped')
    with open(os.environ['GITHUB_STEP_SUMMARY'], 'a') as summary:
        image_security.summary(summary, merged_evidence)
        summary.write('\nAll affected deployments succeeded for `%s`. Merge approval remains consumed.\n' % merged)
    try:
        outcome = followup.queue(api, plan, merged, os.environ)
    except Refused as exc:
        outcome = 'refused: ' + str(exc)
    with open(os.environ['GITHUB_STEP_SUMMARY'], 'a') as summary:
        summary.write('\nFollow-up queue: %s. Merge, affected deployments and merged image evidence are already applied/recorded; '
                      'approval remains consumed, no replay or deployment rerun and no old owner gate reuse. '
                      'Audit this native run and the follow-up history if acknowledgement is unconfirmed or rejected. '
                      'The daily Renovate scan remains the fallback. '
                      'A requested queue acknowledges dispatch only; the child waits until after this native approval workflow fully completes '
                      'and verifies terminal success before requesting proposals with their own fresh CI and owner approval.\n' % outcome)
    require(outcome == 'requested', 'Follow-up queue %s; audit required, no replay; daily Renovate scan remains fallback' % outcome)


def entrypoint():
    import sys
    from .github import GitHub
    api = GitHub(os.environ['GITHUB_REPOSITORY'])
    require(os.environ['GITHUB_REF'] == 'refs/heads/main', 'Trusted main execution required')
    mode = sys.argv[1]
    if mode == 'discover':
        discover(api)
    elif mode == 'prepare':
        prepare(api, int(os.environ['INPUT_PR']), os.environ['INPUT_HEAD'], os.environ['INPUT_BASE'], os.environ['INPUT_SOURCE'])
    elif mode == 'execute':
        plan = json_plan(os.environ['PREPARED_PLAN'])
        execute(api, plan, os.environ['PREPARED_HASH'])
    else:
        raise Refused('Unsupported pipeline command')


def json_plan(encoded):
    import json
    require(len(encoded) < 48_000, 'Prepared plan too large')
    return json.loads(base64.b64decode(encoded, validate=True))


if __name__ == '__main__':
    try:
        entrypoint()
    except Exception as exc:
        print(str(exc) if isinstance(exc, Refused) else 'Malformed pipeline data; stopped')
        raise SystemExit(1) from None
