"""Temporary main-only, no-op live GITHUB_TOKEN approval diagnostic.

Offline tests prove policy, never token capability. Only both observed initial
Bot security holds, acknowledged approvals and successful actual jobs prove it.
Every mutation is issued once. Outputs permit a separate owned-artifact cleanup.
"""
import json
import os
import re
import subprocess
import time
from urllib.parse import quote
from . import pipeline, pr_ci
from .github import GitHub
from .policy import SHA, Refused, assert_unchanged, canonical, mod_shape, plan_hash, require, validate_change, verify_checks

PREFIX = 'renovate-ci/approval-probe-'
DESCRIPTION = 'Diagnostic fixture: no dependency releases changed'


def emit(**values):
    with open(os.environ['GITHUB_OUTPUT'], 'a') as output:
        for key, value in values.items():
            text = str(value)
            require(len(text) <= 500 and '\n' not in text and '\r' not in text, 'Invalid diagnostic output')
            output.write(key + '=' + text + '\n')


def context(api, *, current=True):
    identifier = os.environ.get('GITHUB_RUN_ID', '')
    source = os.environ.get('GITHUB_SHA', '')
    require(re.fullmatch(r'[1-9][0-9]{0,19}', identifier) and SHA.fullmatch(source)
            and os.environ.get('GITHUB_REPOSITORY') == api.repo
            and os.environ.get('GITHUB_REF') == 'refs/heads/main'
            and os.environ.get('GITHUB_RUN_ATTEMPT') == '1'
            and os.environ.get('GITHUB_EVENT_NAME') == 'workflow_dispatch', 'Diagnostic requires first-attempt trusted main dispatch')
    repository = api.get('')
    require(pr_ci.positive(repository.get('id')) and repository.get('full_name') == api.repo,
            'Diagnostic repository identity mismatch')
    run = api.get('/actions/runs/' + identifier)
    require(type(run.get('id')) is int and run['id'] == int(identifier)
            and type(run.get('run_attempt')) is int and run['run_attempt'] == 1
            and run.get('event') == 'workflow_dispatch' and run.get('head_branch') == 'main'
            and run.get('head_sha') == source and run.get('path') == '.github/workflows/renovate.yml'
            and all(run.get(side, {}).get('id') == repository['id']
                    and run.get(side, {}).get('full_name') == api.repo
                    for side in ('repository', 'head_repository')), 'Diagnostic workflow source identity mismatch')
    if current:
        require(pipeline.main_sha(api) == source, 'Main advanced; diagnostic authority stopped')
    return {'run': int(identifier), 'source': source, 'branch': PREFIX + identifier}, repository


def validate_record(record):
    require(type(record) is dict and pr_ci.positive(record.get('run'))
            and record['run'] == int(os.environ['GITHUB_RUN_ID'])
            and record.get('source') == os.environ['GITHUB_SHA']
            and record.get('branch') == PREFIX + str(record['run'])
            and re.fullmatch(r'[1-9][0-9]{0,19}', str(record['run']))
            and all(type(record.get(key)) is str and SHA.fullmatch(record[key])
                    for key in ('source', 'head', 'tree'))
            and (record.get('pr') is None or type(record['pr']) is int and 0 < record['pr'] < 10_000_000),
            'Malformed owned diagnostic record')


def marker(record):
    return 'Sifto CI approval probe run=%d source=%s head=%s' % (record['run'], record['source'], record['head'])


def verify_fixture(api, record, *, cleanup=False):
    validate_record(record)
    source, head = record['source'], record['head']
    if not cleanup:
        ref = ref_or_absent(api, record['branch'])
        require(ref is not None and ref.get('ref') == 'refs/heads/' + record['branch']
                and ref.get('object', {}).get('sha') == head, 'Diagnostic branch ref changed')
    commit = api.get('/git/commits/' + head)
    require(commit.get('sha') == head and commit.get('tree', {}).get('sha') == record['tree']
            and commit.get('parents') is not None
            and [parent.get('sha') for parent in commit['parents']] == [source], 'Diagnostic commit tree or parent mismatch')
    comparison = api.get('/compare/' + source + '...' + head)
    files = comparison.get('files')
    require(comparison.get('status') == 'ahead' and comparison.get('merge_base_commit', {}).get('sha') == source
            and comparison.get('ahead_by') == comparison.get('total_commits') == 1
            and [commit.get('sha') for commit in comparison.get('commits', [])] == [head]
            and type(files) is list and len(files) == 1
            and files[0].get('filename') == 'api/go.mod' and files[0].get('status') == 'modified'
            and 'previous_filename' not in files[0] and files[0].get('additions') == 1
            and files[0].get('deletions') == 0, 'Diagnostic must be exactly one blank-line change')
    before, after = pipeline.blob(api, 'api/go.mod', source), pipeline.blob(api, 'api/go.mod', head)
    require(after == before + '\n', 'Diagnostic bytes changed beyond one newline')
    validate_change('api/go.mod', before, after)
    require(mod_shape(before) == mod_shape(after), 'Diagnostic changed a dependency release')
    require(pipeline.blob(api, 'api/go.sum', source) == pipeline.blob(api, 'api/go.sum', head), 'Diagnostic changed go.sum')
    if record.get('pr') is not None:
        pr = api.get('/pulls/%d' % record['pr'])
        repository = api.get('')
        require(pr.get('number') == record['pr'] and pr.get('merged') is False and pr.get('draft') is False
                and pr.get('state') in ({'open', 'closed'} if cleanup else {'open'})
                and pr_ci.bot(pr.get('user')) and pr.get('body') == marker(record)
                and pr.get('head', {}).get('sha') == head and pr.get('head', {}).get('ref') == record['branch']
                and pr.get('base', {}).get('ref') == 'main' and (cleanup or pr.get('changed_files') == 1)
                and (cleanup or pr.get('base', {}).get('sha') == source)
                and all(pr.get(side, {}).get('repo', {}).get('id') == repository['id']
                        and pr.get(side, {}).get('repo', {}).get('full_name') == api.repo for side in ('head', 'base')),
                'Diagnostic PR ownership or immutable head mismatch')
        if not cleanup:
            listed = api.pages('/pulls/%d/files' % record['pr'])
            require(len(listed) == 1 and all(listed[0].get(key) == files[0].get(key)
                    for key in ('filename', 'status', 'additions', 'deletions'))
                    and 'previous_filename' not in listed[0], 'Diagnostic PR diff mismatch')
        return pr


def ref_or_absent(api, branch):
    try:
        return api.get('/git/ref/heads/' + quote(branch, safe='/'))
    except Refused as exc:
        if str(exc) == 'GitHub API request rejected (HTTP 404)':
            return None
        raise


def propose(api):
    record, _ = context(api)
    # Ref creation is not repeatable, including a previous cancelled attempt.
    require(ref_or_absent(api, record['branch']) is None, 'Diagnostic branch already exists; no mutation replay')
    before = pipeline.blob(api, 'api/go.mod', record['source'])
    after = before + '\n'
    validate_change('api/go.mod', before, after)
    require(mod_shape(before) == mod_shape(after), 'Diagnostic source is not data-only Go')
    source_commit = api.get('/git/commits/' + record['source'])
    source_tree = source_commit.get('tree', {}).get('sha', '')
    require(source_commit.get('sha') == record['source'] and SHA.fullmatch(source_tree), 'Diagnostic source tree mismatch')
    blob = api.request('POST', '/git/blobs', {'content': after, 'encoding': 'utf-8'})
    require(SHA.fullmatch(blob.get('sha', '')), 'Diagnostic blob acknowledgement malformed')
    tree = api.request('POST', '/git/trees', {'base_tree': source_tree, 'tree': [
        {'path': 'api/go.mod', 'mode': '100644', 'type': 'blob', 'sha': blob['sha']}]})
    require(SHA.fullmatch(tree.get('sha', '')), 'Diagnostic tree acknowledgement malformed')
    commit = api.request('POST', '/git/commits', {'message': 'No-op CI approval probe run %d' % record['run'],
        'tree': tree['sha'], 'parents': [record['source']]})
    require(SHA.fullmatch(commit.get('sha', '')), 'Diagnostic commit acknowledgement malformed')
    record.update(head=commit['sha'], tree=tree['sha'], pr=None)
    emit(source=record['source'], branch=record['branch'], head=record['head'], tree=record['tree'])
    context(api)
    response = api.request('POST', '/git/refs', {'ref': 'refs/heads/' + record['branch'], 'sha': record['head']})
    require(response.get('ref') == 'refs/heads/' + record['branch']
            and response.get('object', {}).get('sha') == record['head'], 'Diagnostic ref acknowledgement mismatch')
    response = api.request('POST', '/pulls', {'title': 'No-op CI approval probe run %d' % record['run'],
        'head': record['branch'], 'base': 'main', 'body': marker(record), 'draft': False})
    require(type(response.get('number')) is int and 0 < response['number'] < 10_000_000,
            'Diagnostic PR acknowledgement malformed; cleanup readback required')
    record['pr'] = response['number']
    emit(pr=record['pr'])
    verify_fixture(api, record)
    context(api)
    api.request('POST', '/statuses/' + record['head'], {'state': 'success', 'context': 'renovate/stability-days',
        'description': DESCRIPTION})
    return record


def prove(api, record, clock=time.monotonic, sleep=time.sleep):
    # This runs in a separate job with exactly ordinary prepare permissions.
    context(api)
    require(pr_ci.positive(record.get('pr')), 'Diagnostic proof requires recorded PR acknowledgement')
    verify_fixture(api, record)
    def candidate():
        context(api)
        return pipeline.candidate(api, record['pr'], record['head'], record['source'], record['source'], _approval_probe=record)
    plan = candidate()
    def revalidate():
        assert_unchanged(plan, candidate(), plan_hash(plan))
    session = pr_ci.Session(api, plan, revalidate, clock, allow_approval=True)
    session.register()
    while True:
        session.remaining()
        session.candidate_identity()
        session.selected = session.inventory()
        if session.required() <= set(session.selected):
            break
        sleep(min(10, session.remaining()))
    require(set(session.selected) == set(pr_ci.WORKFLOWS[:2]), 'Unexpected diagnostic PR workflow')
    for name, initial in session.selected.items():
        rows = api.pages('/actions/workflows/%d/runs?event=pull_request&head_sha=%s' %
                         (session.workflows[name]['id'], record['head']), 'workflow_runs', require_total=True)
        require(len(rows) == 1 and rows[0] == initial, 'Inconclusive: CI did not have exactly one initial run')
        require(initial.get('conclusion') == 'action_required', 'Inconclusive: initial CI was not security-held')
        session.held(initial)
    def ownership(acknowledged):
        # Persist successful HTTP acknowledgements before any visibility read.
        # Issued intent and unknown outcomes never become cleanup authority.
        emit(readonly_ids=canonical({name: {'id': run['id'], 'acknowledged': run['id'] in acknowledged.approved}
                                     for name, run in acknowledged.selected.items()}))
    ownership(session)
    session.approve(clock, sleep, acknowledge=ownership)
    fixed = pipeline.dispatch_once(api, 'dependency-check.yml', record['branch'], record['head'],
        {'expected_head': record['head']}, remaining_budget=session.remaining)
    emit(fixed_ci=fixed['id'])
    verify_checks(fixed, api.pages('/actions/runs/%d/jobs?filter=latest' % fixed['id'], 'jobs'), record['head'])
    session.wait(clock=clock, sleep=sleep)
    require(session.approved == {run['id'] for run in session.selected.values()}, 'Inconclusive: another actor approved CI')
    revalidate()
    session.remaining()
    emit(proof='success')
    print('Live GITHUB_TOKEN approvals acknowledged; both actual PR CI job inventories and fixed CI succeeded')


def cleanup(api, record, readonly_ids, *, git=subprocess.run):
    context(api, current=False)
    validate_record(record)
    require(type(readonly_ids) is dict and set(readonly_ids) <= set(pr_ci.WORKFLOWS[:2])
            and all(pr_ci.positive(value) or type(value) is dict and set(value) == {'id', 'acknowledged'}
                    and pr_ci.positive(value['id']) and type(value['acknowledged']) is bool
                    for value in readonly_ids.values()), 'Malformed diagnostic cleanup run IDs')
    owned = {name: {'id': value, 'acknowledged': False} if type(value) is int else value
             for name, value in readonly_ids.items()}
    require(len({value['id'] for value in owned.values()}) == len(owned), 'Duplicate diagnostic cleanup run IDs')
    ref = ref_or_absent(api, record['branch'])
    if ref is not None:
        require(ref.get('ref') == 'refs/heads/' + record['branch']
                and ref.get('object', {}).get('sha') == record['head'], 'Diagnostic branch changed; retained artifact')
    if record.get('pr') is None:
        rows = api.pages('/pulls?state=all&base=main&head=' + quote(api.repo.split('/')[0] + ':' + record['branch'], safe=''))
        require(len(rows) <= 1, 'Ambiguous diagnostic PR creation; retained artifact')
        if rows:
            record['pr'] = rows[0].get('number')
            validate_record(record)
    verify_fixture(api, record, cleanup=True)
    if readonly_ids:
        require(record.get('pr') is not None, 'No owned PR for cleanup run identity')
        plan = {'pr': record['pr'], 'head': record['head'], 'base': record['source'], 'source': record['source'],
                'branch': record['branch'], 'emergency': False, 'files': [{'path': 'api/go.mod'}]}
        session = pr_ci.Session(api, plan, lambda: None, time.monotonic)
        session.register()
        for name, ownership in owned.items():
            identifier = ownership['id']
            current = session.validate(api.get('/actions/runs/%d' % identifier), name)
            require(current['id'] == identifier and current['run_attempt'] in ({1, 2} if ownership['acknowledged'] else {1})
                    and pr_ci.bot(current['actor']) and pr_ci.bot(current['triggering_actor']), 'Cleanup run identity mismatch')
            if ownership['acknowledged']:
                # Recorded ACK + immutable owned fixture + original Bot hold
                # proves cancellation ownership, never successful CI proof.
                session.original_hold(current, name)
                if current['run_attempt'] == 1:
                    session.held(current, historical=True)
            if current['status'] in pr_ci.ACTIVE:
                jobs = session.attempt_jobs(current)
                session.readonly_jobs(current, name, jobs)
                verify_fixture(api, record, cleanup=True)
                require(session.validate(api.get('/actions/runs/%d' % identifier), name) == current, 'Cleanup run changed')
                api.request('POST', '/actions/runs/%d/cancel' % identifier)
    if record.get('pr') is not None:
        pr = verify_fixture(api, record, cleanup=True)
        if pr['state'] == 'open':
            api.request('PATCH', '/pulls/%d' % record['pr'], {'state': 'closed'})
    if ref is not None:
        verify_fixture(api, record, cleanup=True)
        current = ref_or_absent(api, record['branch'])
        require(current is not None and current.get('object', {}).get('sha') == record['head'], 'Diagnostic branch changed before lease; retained artifact')
        try:
            git(['git', '-c', 'credential.helper=', '-c', 'credential.helper=!gh auth git-credential',
                '-c', 'core.hooksPath=/dev/null', 'push', '--porcelain',
                '--force-with-lease=refs/heads/' + record['branch'] + ':' + record['head'],
                'https://github.com/' + api.repo + '.git', ':refs/heads/' + record['branch']],
                capture_output=True, text=True, timeout=60, check=False)
        except (subprocess.TimeoutExpired, OSError):
            pass  # Unknown outcome permits only a readback, never a replay.
        require(ref_or_absent(api, record['branch']) is None,
                'Diagnostic branch deletion unconfirmed; no retry, retained artifact audit required')
    print('Owned diagnostic cleanup completed; no production action requested')


def recorded():
    return {'run': int(os.environ['GITHUB_RUN_ID']), 'source': os.environ['PROBE_SOURCE'],
        'branch': os.environ['PROBE_BRANCH'], 'head': os.environ['PROBE_HEAD'], 'tree': os.environ['PROBE_TREE'],
        'pr': int(os.environ['PROBE_PR']) if os.environ.get('PROBE_PR') else None}


def entrypoint():
    import sys
    api = GitHub(os.environ['GITHUB_REPOSITORY'])
    if sys.argv[1] == 'propose':
        propose(api)
    elif sys.argv[1] == 'prove':
        prove(api, recorded())
    elif sys.argv[1] == 'cleanup':
        # No recorded intended head means no ref creation was attempted.
        if not os.environ.get('PROBE_HEAD'):
            print('No diagnostic ref creation recorded')
            return
        cleanup(api, recorded(), json.loads(os.environ.get('PROBE_READONLY_IDS') or '{}'))
    else:
        raise Refused('Unsupported diagnostic command')


if __name__ == '__main__':
    try:
        entrypoint()
    except Exception as exc:
        print(str(exc) if isinstance(exc, Refused) else 'Malformed diagnostic data; stopped; audit retained artifacts')
        raise SystemExit(1) from None
