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
    if len(jobs) != 2 or {job.get('name') for job in jobs} != {'prepare', 'approve-merge-deploy'}:
        return False
    if not all(isinstance(job.get('id'), int) and job.get('run_id') == run_id and job.get('head_sha') == plan['source'] and job.get('status') == 'completed' for job in jobs) or len({job['id'] for job in jobs}) != 2:
        return False
    prepare_job = next(job for job in jobs if job['name'] == 'prepare')
    privileged = next(job for job in jobs if job['name'] == 'approve-merge-deploy')
    if prepare_job.get('conclusion') not in ('failure', 'cancelled', 'timed_out') or not isinstance(prepare_job.get('steps'), list):
        return False
    if privileged.get('steps') != [] or 'runner_id' not in privileged or privileged['runner_id'] not in (None, 0) or 'runner_name' not in privileged or privileged['runner_name'] not in (None, ''):
        return False
    if privileged.get('conclusion') == 'skipped':
        unstarted = True
    else:
        unstarted = privileged.get('conclusion') == 'cancelled' and 'started_at' in privileged and privileged['started_at'] is None
    if not unstarted:
        return False
    # A reviewer or manual rerun can race the inventory queries. Read back both
    # authorities before permitting a separate fresh run with its own gate.
    final = api.get('/actions/runs/' + str(run_id))
    return all(final.get(key) == value for key, value in expected.items()) and api.get('/actions/runs/%d/approvals' % run_id) == []


def active_approval(api):
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
            active = True
    return active


def discover(api):
    source = main_sha(api)
    require(type(source) is str and SHA.fullmatch(source) and source == os.environ.get('GITHUB_SHA'),
            'Discovery workflow source must equal exact current main')
    if active_approval(api):
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
            if active_approval(api):
                print('Native dependency approval or deployment became active; no new request')
                return
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
    from . import pr_ci
    plan = candidate(api, number, head, base, source)
    require(os.environ.get('GITHUB_RUN_ATTEMPT') == '1', 'Reruns require a fresh candidate approval request')
    def revalidate():
        assert_unchanged(plan, candidate(api, number, head, base, source,
                                       python_cutoff=plan['python_age']['cutoff']), plan_hash(plan))
    pr_checks = pr_ci.start(api, plan, revalidate)
    run = dispatch_once(api, 'dependency-check.yml', plan['branch'], head, {'expected_head': head},
                        remaining_budget=pr_checks.remaining)
    verify_checks(run, api.pages('/actions/runs/%d/jobs?filter=latest' % run['id'], 'jobs'), head)
    pr_checks.wait()
    revalidate()
    plan['ci_run'] = run['id']
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


def execute(api, plan, digest):
    require(os.environ.get('GITHUB_RUN_ATTEMPT') == '1', 'Consumed approval cannot be rerun')
    require(plan_hash(plan) == digest and plan['source'] == os.environ['GITHUB_SHA'], 'Untrusted or altered prepared plan')
    run_id = int(os.environ['GITHUB_RUN_ID'])
    run = api.get('/actions/runs/' + str(run_id))
    require(run.get('event') == 'workflow_dispatch' and run.get('head_branch') == 'main' and run.get('head_sha') == plan['source'] and run.get('path') == '.github/workflows/' + WORKFLOW and run.get('run_attempt') == 1, 'Approval run source mismatch')
    verify_approval(api.get('/actions/runs/%d/approvals' % run_id), run.get('actor', {}).get('login'))
    require(plan.get('version') == 2 and type(plan.get('python_age')) is dict,
            'Prepared plan has no fixed Python artifact-age evidence; prepare again')
    current = candidate(api, plan['pr'], plan['head'], plan['base'], plan['source'],
                        python_cutoff=plan['python_age']['cutoff'])
    current['ci_run'] = plan['ci_run']
    assert_unchanged(plan, current, digest)
    ci = api.get('/actions/runs/%d' % plan['ci_run'])
    verify_checks(ci, api.pages('/actions/runs/%d/jobs?filter=latest' % plan['ci_run'], 'jobs'), plan['head'])
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
    with open(os.environ['GITHUB_STEP_SUMMARY'], 'a') as summary:
        summary.write('\nAll affected deployments succeeded for `%s`. Merge approval remains consumed.\n' % merged)
        summary.write('\nThe next proposal is handled after this native approval workflow fully completes, '
                      'by a separate follow-up with fresh CI and owner approval. '
                      'The daily Renovate scan remains the fallback.\n')


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
