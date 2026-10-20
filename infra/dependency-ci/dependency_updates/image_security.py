"""Dedicated read-only image advisory evidence, never a generic CI success waiver.

Artifacts contain reports, not image.tar: archive_sha256 is validated as a digest
format only. This verifier does not claim to have verified runtime archive bytes.
"""
import hashlib
import io
import json
import os
import re
import selectors
import stat
import subprocess
import time
import zipfile
from datetime import datetime, timezone

from .policy import Refused, SHA, canonical, require
from .followup import bot, context_id, positive, registered, repository, run_identity
from .github import Ambiguous

WORKFLOW = 'dependency-security.yml'
NAME = 'Dependency Security Advisory'
PARENT = 'dependency-update.yml'
PARENT_NAME = 'Dependency Approve Merge and Deploy'
WAIT_SECONDS = 80 * 60
# github.token shares 1,000 REST requests/hour with the whole repository.
# Cheap pinned-run reads use at most 60/hour. Full authority/history polling
# targets at most 180/hour, leaving ample quota for final collection and other
# repository workflows, even at the full 20-page history limit.
POLL_SECONDS = 60
AUTHORITY_SECONDS = 5 * 60
AUTHORITY_READS_PER_HOUR = 180
MAX_ZIP = 8 * 1024 * 1024
MAX_EXPANDED = 40 * 1024 * 1024
MAX_MEMBER = 12 * 1024 * 1024
DIGEST = re.compile(r'sha256:[0-9a-f]{64}')
SEVERITIES = ('UNKNOWN', 'LOW', 'MEDIUM', 'HIGH', 'CRITICAL')
# Matches infra/image-security/report.py EXPECTED and build.sh production paths.
EXPECTED = {
    'api': ('linux/arm64', 'api/Dockerfile', 'ca-certificates', 'github.com/jackc/pgx/v5'),
    'worker': ('linux/arm64', 'worker/Dockerfile', 'libc6', 'fastapi'),
    'web': ('linux/amd64', 'web/Dockerfile', 'musl', 'next'),
    'audio-concat': ('linux/amd64', 'infra/audio-concat/Dockerfile', 'ffmpeg', 'boto3'),
    'maintenance': ('linux/amd64', 'infra/maintenance/Dockerfile', 'openssh-client', 'ansible-core'),
}
REPORTS = ('sbom.cdx.json', 'vulnerabilities.json', 'scan-status.json')
FILES = frozenset(REPORTS + ('metadata.json', 'build-metadata.json', 'source-sha.txt',
    'source-dirty.txt', 'image-id.txt', 'platform.txt', 'dockerfile.txt', 'summary.md'))
GUARD = 'Verify requested scan source and dispatch identity'
SCAN = 'Generate SBOM and scan final image OS and library vulnerabilities'
CHECKOUT = 'Run actions/checkout@fbc6f3992d24b796d5a048ff273f7fcc4a7b6c09'
QEMU = 'Run docker/setup-qemu-action@c7c53464625b32c7a7e944ae62b3e17d2b600130'
BUILDX = 'Run docker/setup-buildx-action@8d2750c68a42422c14e847fe6c8ac0403b4cbd6f'
IMAGE_STEPS = ('Set up job', CHECKOUT, GUARD, QEMU, BUILDX,
    'Verify archive identity and scan evidence validation',
    'Build final production image archive for deployment platform', SCAN,
    'Publish image inventory summary', 'Preserve SBOM, findings and immutable image correlation',
    'Post ' + BUILDX, 'Post ' + QEMU, 'Post ' + CHECKOUT, 'Complete job')
INVENTORY_SCAN = 'Scan all known package vulnerabilities'
INVENTORY_STEPS = ('Set up job', CHECKOUT, 'Stage public dependency manifests only',
    INVENTORY_SCAN, 'Publish package and CVE inventory in summary',
    'Post ' + INVENTORY_SCAN, 'Post ' + CHECKOUT, 'Complete job')


def sha256(raw):
    return 'sha256:' + hashlib.sha256(raw).hexdigest()


def strict_json(raw):
    def pairs(items):
        value = {}
        for key, item in items:
            require(key not in value, 'Duplicate JSON key in image evidence')
            value[key] = item
        return value
    def invalid(_):
        raise Refused('Nonfinite JSON in image evidence')
    require(type(raw) is bytes and len(raw) <= MAX_MEMBER, 'Image JSON exceeds limit')
    try:
        return json.loads(raw, object_pairs_hook=pairs, parse_constant=invalid)
    except (ValueError, UnicodeError, RecursionError):
        raise Refused('Malformed image evidence JSON') from None


def verify_archive(raw, component, head):
    require(type(raw) is bytes and 0 < len(raw) <= MAX_ZIP, 'Image artifact ZIP exceeds limit')
    require(component in EXPECTED and type(head) is str and SHA.fullmatch(head), 'Invalid image source')
    try:
        with zipfile.ZipFile(io.BytesIO(raw)) as archive:
            members = archive.infolist()
            require(len(members) == len(FILES) and {item.filename for item in members} == FILES,
                    'Image artifact must contain exactly the known flat evidence files')
            require(sum(item.file_size for item in members) <= MAX_EXPANDED,
                    'Expanded image artifact exceeds limit')
            files = {}
            for item in members:
                require(not item.is_dir() and not stat.S_ISLNK(item.external_attr >> 16)
                        and not item.flag_bits & 1 and 0 <= item.file_size <= MAX_MEMBER,
                        'Unsafe or oversized image artifact member')
                with archive.open(item) as stream:
                    data = stream.read(MAX_MEMBER + 1)
                require(len(data) == item.file_size and len(data) <= MAX_MEMBER,
                        'Image artifact member size mismatch')
                files[item.filename] = data
        return verify_files(files, component, head)
    except (zipfile.BadZipFile, OSError, KeyError, TypeError, ValueError, UnicodeError,
            RuntimeError, AttributeError, RecursionError):
        raise Refused('Malformed image artifact or report data') from None


def verify_files(files, component, head):
    platform, dockerfile, expected_os, expected_library = EXPECTED[component]
    metadata = strict_json(files['metadata.json'])
    build = strict_json(files['build-metadata.json'])
    sbom = strict_json(files['sbom.cdx.json'])
    findings = strict_json(files['vulnerabilities.json'])
    status = strict_json(files['scan-status.json'])
    require(all(type(value) is dict for value in (metadata, build, sbom, findings, status)),
            'Image evidence JSON must contain objects')
    image_id = metadata.get('image_id')
    require(type(image_id) is str and DIGEST.fullmatch(image_id)
            and metadata.get('component') == component and metadata.get('source_commit') == head
            and metadata.get('source_tree_dirty') is False and metadata.get('stage') == 'final'
            and metadata.get('platform') == platform and metadata.get('dockerfile') == dockerfile
            and metadata.get('build_image_id') == image_id
            and build.get('containerimage.config.digest') == image_id
            and type(metadata.get('archive_sha256')) is str and DIGEST.fullmatch(metadata['archive_sha256'])
            and metadata.get('trivy_version') == '0.75.0', 'Image metadata identity mismatch')
    for name, expected in (('source-sha.txt', head), ('source-dirty.txt', 'false'),
                           ('image-id.txt', image_id), ('platform.txt', platform), ('dockerfile.txt', dockerfile)):
        require(len(files[name]) <= 512 and files[name].decode('utf-8').strip() == expected,
                'Image source/build identity file mismatch')
    require(0 < len(files['summary.md']) <= 128 * 1024, 'Image summary missing or oversized')
    hashes = {name: sha256(files[name]) for name in REPORTS}
    require(metadata.get('report_sha256') == hashes, 'Image report digest mismatch')
    require(sbom.get('bomFormat') == 'CycloneDX', 'Image SBOM must be CycloneDX')
    properties = sbom.get('metadata', {}).get('component', {}).get('properties')
    require(type(properties) is list and all(type(p) is dict and type(p.get('name')) is str
            and type(p.get('value')) is str for p in properties), 'Malformed SBOM image properties')
    ids = [p['value'] for p in properties if p['name'] == 'aquasecurity:trivy:ImageID']
    require(ids == [image_id] and findings.get('Metadata', {}).get('ImageID') == image_id,
            'SBOM/findings refer to another image')
    components = sbom.get('components')
    require(type(components) is list and all(type(p) is dict and type(p.get('name')) is str
            for p in components), 'Malformed SBOM inventory')
    sbom_names = {p['name'] for p in components}
    results = findings.get('Results')
    require(type(results) is list and all(type(r) is dict and r.get('Class') in {'os-pkgs', 'lang-pkgs'}
            for r in results), 'Malformed OS/library findings inventory')
    inventory = {'os-pkgs': set(), 'lang-pkgs': set()}
    counts = dict.fromkeys(SEVERITIES, 0)
    for result in results:
        packages = result.get('Packages', [])
        vulnerabilities = result.get('Vulnerabilities', [])
        require(type(packages) is list and all(type(p) is dict and type(p.get('Name')) is str
                and 0 < len(p['Name']) <= 512 for p in packages), 'Malformed package inventory')
        inventory[result['Class']].update(p['Name'] for p in packages)
        require(type(vulnerabilities) is list, 'Malformed vulnerability inventory')
        for vulnerability in vulnerabilities:
            require(type(vulnerability) is dict and vulnerability.get('Severity') in SEVERITIES
                    and type(vulnerability.get('VulnerabilityID')) is str
                    and 0 < len(vulnerability['VulnerabilityID']) <= 512, 'Malformed vulnerability severity/ID')
            counts[vulnerability['Severity']] += 1
    for name, names in ((expected_os, inventory['os-pkgs']), (expected_library, inventory['lang-pkgs'])):
        require(name in names and name in sbom_names, 'Known OS/library fixture missing from both inventories')
    package_counts = {'os': len(inventory['os-pkgs']), 'library': len(inventory['lang-pkgs'])}
    require(type(metadata.get('package_counts')) is dict
            and all(type(v) is int for v in metadata['package_counts'].values())
            and metadata['package_counts'] == package_counts
            and type(metadata.get('vulnerability_counts')) is dict
            and all(type(v) is int for v in metadata['vulnerability_counts'].values())
            and metadata['vulnerability_counts'] == counts, 'Image inventory/severity counts mismatch')
    require(type(status.get('sbom_exit_code')) is int and status['sbom_exit_code'] == 0
            and type(status.get('vulnerability_exit_code')) is int
            and status['vulnerability_exit_code'] == int(bool(sum(counts.values()))),
            'Scanner failure or status/report mismatch')
    result = {'component': component, 'platform': platform, 'image_id': image_id,
            'archive_sha256': metadata['archive_sha256'], 'report_sha256': hashes,
            'package_counts': package_counts, 'vulnerability_counts': counts}
    if 'findings_fail_job' in metadata:
        require(metadata['findings_fail_job'] is False, 'Unknown findings outcome format')
        result['findings_fail_job'] = False
    return result


def download_zip(repo, identifier):
    """Use the official CLI's authenticated redirect handling; bound stdout.

    Credentials stay in the environment. Neither CLI stderr nor argv can expose
    the token; downloaded bytes are data and are never extracted or executed.
    """
    require(type(repo) is str and re.fullmatch(r'[A-Za-z0-9_.-]+/[A-Za-z0-9_.-]+', repo)
            and positive(identifier), 'Invalid artifact download identity')
    require(type(os.environ.get('GH_TOKEN')) is str and bool(os.environ['GH_TOKEN']),
            'Artifact download requires GH_TOKEN')
    environment = dict(os.environ, GH_HOST='github.com', GH_PROMPT_DISABLED='1')
    environment.pop('GITHUB_TOKEN', None)
    environment.pop('GH_DEBUG', None)
    process = None
    try:
        process = subprocess.Popen(['gh', 'api', '--hostname', 'github.com',
            'repos/%s/actions/artifacts/%d/zip' % (repo, identifier)],
            env=environment, stdin=subprocess.DEVNULL, stdout=subprocess.PIPE, stderr=subprocess.DEVNULL)
        deadline, chunks, size = time.monotonic() + 120, [], 0
        with selectors.DefaultSelector() as selector:
            selector.register(process.stdout, selectors.EVENT_READ)
            while True:
                remaining = deadline - time.monotonic()
                require(remaining > 0, 'Artifact CLI download timed out')
                events = selector.select(min(1, remaining))
                if not events:
                    continue
                data = os.read(process.stdout.fileno(), min(65536, MAX_ZIP + 1 - size))
                if not data:
                    break
                chunks.append(data)
                size += len(data)
                require(size <= MAX_ZIP, 'Compressed image artifact exceeds limit')
            require(process.wait(timeout=max(0.01, deadline - time.monotonic())) == 0,
                    'Artifact CLI download unavailable')
        return b''.join(chunks)
    except (OSError, subprocess.TimeoutExpired):
        raise Refused('Artifact CLI download unavailable') from None
    finally:
        if process is not None:
            if process.poll() is None:
                process.kill()
                process.wait(timeout=5)
            if process.stdout is not None:
                process.stdout.close()


def protected_main(api, source):
    branch = api.get('/branches/main')
    require(type(branch) is dict and branch.get('name') == 'main' and branch.get('protected') is True
            and branch.get('commit', {}).get('sha') == source
            and api.get('/git/ref/heads/main').get('object', {}).get('sha') == source,
            'Image evidence requires unchanged protected main')


def caller(api, plan, phase, head, ref):
    context = os.environ
    parent_id, repo_id = context_id(context.get('GITHUB_RUN_ID')), context_id(context.get('GITHUB_REPOSITORY_ID'))
    require(phase in ('candidate', 'merged') and type(head) is str and SHA.fullmatch(head)
            and type(plan) is dict and plan.get('repo') == api.repo
            and type(plan.get('source')) is str and SHA.fullmatch(plan['source'])
            and plan.get('source') == plan.get('base')
            and type(plan.get('head')) is str and SHA.fullmatch(plan['head'])
            and type(plan.get('pr')) is int and 0 < plan['pr'] < 10_000_000,
            'Malformed native image scan candidate')
    require((phase == 'candidate' and head == plan['head'] and ref == plan['branch'])
            or (phase == 'merged' and ref == 'main' and head != plan['base']),
            'Image phase/source/ref mismatch')
    require(context.get('GITHUB_RUN_ATTEMPT') == '1' and context.get('GITHUB_SHA') == plan['source']
            and context.get('GITHUB_WORKFLOW_SHA') == plan['source']
            and context.get('GITHUB_REF') == 'refs/heads/main'
            and context.get('GITHUB_EVENT_NAME') == 'workflow_dispatch'
            and context.get('GITHUB_REPOSITORY') == api.repo
            and context.get('GITHUB_WORKFLOW_REF') == api.repo + '/.github/workflows/' + PARENT + '@refs/heads/main',
            'Image scan requires actual first-attempt trusted main native caller')
    repository(api.get(''), api.repo, repo_id)
    parent_workflow = registered(api, PARENT, PARENT_NAME)
    run = api.get('/actions/runs/%d' % parent_id)
    expected = {'id': parent_id, 'workflow_id': parent_workflow, 'event': 'workflow_dispatch',
        'head_branch': 'main', 'head_sha': plan['source'], 'path': '.github/workflows/' + PARENT,
        'display_title': 'Dependency PR %d %s %s' % (plan['pr'], plan['head'], plan['base']),
        'status': 'in_progress', 'conclusion': None}
    run_identity(run, expected, api.repo, repo_id)
    require(context.get('GITHUB_ACTOR') == run['actor']['login']
            and context_id(context.get('GITHUB_ACTOR_ID')) == run['actor']['id']
            and context.get('GITHUB_TRIGGERING_ACTOR') == run['triggering_actor']['login'],
            'Native caller actor differs from actual run')
    protected_main(api, plan['base'] if phase == 'candidate' else head)
    return parent_id, repo_id, parent_workflow


def history(api, marker):
    parsed = re.fullmatch(r'Dependency image scan ([1-9][0-9]{0,19}) (candidate|merged) [0-9a-f]{40}', marker)
    require(parsed is not None, 'Malformed expected image workflow marker')
    phase_marker = 'Dependency image scan %s %s' % (parsed[1], parsed[2])
    runs = api.pages('/actions/workflows/' + WORKFLOW + '/runs', 'workflow_runs', require_total=True)
    identifiers, matches = set(), []
    for run in runs:
        require(type(run) is dict and positive(run.get('id')) and run['id'] not in identifiers
                and type(run.get('display_title')) is str and len(run['display_title']) <= 512,
                'Malformed or duplicate image workflow history')
        identifiers.add(run['id'])
        # A parent/phase owns one dispatch chance, even if its previous source
        # was rejected or unknown. Exact SHA selection below is not replay
        # authorization for a different SHA under the same native parent.
        if run['display_title'] == phase_marker or run['display_title'].startswith(phase_marker + ' '):
            require(run['display_title'] == marker,
                    'Image parent/phase already consumed by another source; no replay')
        if run['display_title'] == marker:
            matches.append(run)
    require(len(matches) <= 1, 'Ambiguous image workflow marker; no replay')
    return matches, max(identifiers, default=0), len(runs) // 100 + 1


def start(api, plan, phase, head, ref):
    parent_id, repo_id, parent_workflow = caller(api, plan, phase, head, ref)
    workflow_id = registered(api, WORKFLOW, NAME)
    marker = 'Dependency image scan %d %s %s' % (parent_id, phase, head)
    matches, after_id, _ = history(api, marker)
    require(not matches, 'Image marker already consumed; no replay')
    # Recheck source and native identity before the sole mutation. Unknown POST
    # results fail the parent and are audited, never redispatched or adopted.
    require(caller(api, plan, phase, head, ref) == (parent_id, repo_id, parent_workflow),
            'Native caller changed before image dispatch')
    try:
        api.request('POST', '/actions/workflows/' + WORKFLOW + '/dispatches',
            {'ref': ref, 'inputs': {'expected_head': head, 'parent_run': str(parent_id), 'phase': phase}})
    except Ambiguous:
        raise Refused('Image dispatch acknowledgement lost; audit required, no mutation replay') from None
    return {'parent_run': parent_id, 'parent_workflow': parent_workflow, 'repo_id': repo_id,
            'workflow_id': workflow_id, 'phase': phase, 'head': head, 'ref': ref,
            'marker': marker, 'after_id': after_id, 'deadline': time.monotonic() + WAIT_SECONDS}


def validate_ticket(api, plan, ticket):
    require(type(ticket) is dict and ticket.get('phase') in ('candidate', 'merged')
            and positive(ticket.get('parent_run')) and positive(ticket.get('workflow_id'))
            and positive(ticket.get('repo_id')) and positive(ticket.get('parent_workflow'))
            and type(ticket.get('after_id')) is int and ticket['after_id'] >= 0
            and type(ticket.get('ref')) is str and type(ticket.get('head')) is str
            and SHA.fullmatch(ticket['head']), 'Prepared image scan ticket missing or malformed')
    actual = caller(api, plan, ticket['phase'], ticket['head'], ticket['ref'])
    require(actual == (ticket['parent_run'], ticket['repo_id'], ticket['parent_workflow'])
            and ticket['workflow_id'] == registered(api, WORKFLOW, NAME)
            and ticket.get('marker') == 'Dependency image scan %d %s %s' % (
                ticket['parent_run'], ticket['phase'], ticket['head']), 'Prepared image scan authority changed')


def child(api, ticket, run):
    require(type(run) is dict and positive(run.get('id')) and run['id'] > ticket['after_id']
            and type(run.get('run_attempt')) is int and run['run_attempt'] == 1
            and type(run.get('workflow_id')) is int and run['workflow_id'] == ticket['workflow_id']
            and run.get('event') == 'workflow_dispatch' and run.get('head_sha') == ticket['head']
            and run.get('head_branch') == ticket['ref'] and run.get('display_title') == ticket['marker']
            and run.get('path') in ('.github/workflows/' + WORKFLOW,
                '.github/workflows/' + WORKFLOW + '@' + ticket['ref']),
            'Image scan child source/actor/first-attempt identity mismatch')
    repository(run.get('repository'), api.repo, ticket['repo_id'])
    repository(run.get('head_repository'), api.repo, ticket['repo_id'])
    bot(run.get('actor'))
    bot(run.get('triggering_actor'))
    require((run.get('status') == 'completed' and run.get('conclusion') in ('success', 'failure'))
            or (run.get('status') in ('queued', 'in_progress', 'waiting', 'requested', 'pending')
                and run.get('conclusion') is None), 'Image scan is cancelled, timed out or unconfirmed')


def step_proof(job, names, allowed_failure, expected_failure=None):
    steps = job.get('steps')
    require(type(steps) is list and len(steps) == len(names)
            and [step.get('name') for step in steps if type(step) is dict] == list(names),
            'Missing, duplicate or unknown image evidence stage')
    failures = []
    for step in steps:
        require(step.get('status') == 'completed'
                and (step.get('conclusion') == 'success' or
                     (step['name'] == allowed_failure and step.get('conclusion') == 'failure')),
                'Image build/setup/validation/report/upload stage incomplete')
        if step['conclusion'] == 'failure': failures.append(step['name'])
    require(job.get('conclusion') == ('failure' if failures else 'success'), 'Image job/stage status mismatch')
    if expected_failure is not None:
        require(bool(failures) == expected_failure, 'Image scan failure is not coherent with actual findings')


def jobs_proof(api, ticket, run_id, images):
    modes = {value.get('findings_fail_job', True) for value in images.values()}
    require(len(modes) == 1, 'Mixed image findings outcome formats')
    legacy_findings_failure = modes.pop()
    jobs = api.pages('/actions/runs/%d/jobs?filter=all' % run_id, 'jobs', require_total=True)
    names, identifiers = set(), set()
    inventory, image_jobs = None, {}
    for job in jobs:
        require(type(job) is dict and positive(job.get('id')) and job['id'] not in identifiers
                and type(job.get('run_id')) is int and job['run_id'] == run_id
                and type(job.get('run_attempt')) is int and job['run_attempt'] == 1
                and job.get('head_sha') == ticket['head'] and job.get('status') == 'completed'
                and type(job.get('name')) is str and job['name'] not in names,
                'Missing/duplicate/foreign/rerun image job proof')
        identifiers.add(job['id'])
        names.add(job['name'])
        if job['name'] == 'inventory':
            if ticket['ref'] == 'main':
                step_proof(job, INVENTORY_STEPS, INVENTORY_SCAN,
                           None if legacy_findings_failure else False)
            else:
                require(job.get('conclusion') == 'skipped' and job.get('steps') == [],
                        'Candidate filesystem advisory inventory should be skipped')
            inventory = {'job_id': job['id'], 'conclusion': job['conclusion']}
        else:
            component = next((name for name in EXPECTED if job['name'] == 'images (' + name + ')'), None)
            require(component is not None, 'Unknown image advisory job')
            expected_failure = legacy_findings_failure and bool(sum(images[component]['vulnerability_counts'].values()))
            step_proof(job, IMAGE_STEPS, SCAN, expected_failure)
            image_jobs[component] = job['id']
    require(set(image_jobs) == set(EXPECTED) and (ticket['ref'] != 'main' or inventory is not None),
            'All five production image jobs and expected inventory are required')
    return image_jobs, inventory, jobs


def artifact_proof(artifact, ticket, run_id, component):
    require(type(artifact) is dict and positive(artifact.get('id'))
            and artifact.get('name') == 'image-security-' + component + '-' + ticket['head'] + '-1'
            and artifact.get('expired') is False and type(artifact.get('size_in_bytes')) is int
            and 0 < artifact['size_in_bytes'] <= MAX_ZIP
            and type(artifact.get('digest')) is str and DIGEST.fullmatch(artifact['digest'])
            and type(artifact.get('expires_at')) is str
            and re.fullmatch(r'\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}Z', artifact['expires_at']),
            'Image artifact missing/expired/oversized or malformed')
    try:
        expiry = datetime.fromisoformat(artifact['expires_at'].replace('Z', '+00:00'))
    except ValueError:
        raise Refused('Malformed image artifact expiry') from None
    require(expiry > datetime.now(timezone.utc), 'Image artifact expired')
    value = artifact.get('workflow_run')
    require(type(value) is dict and type(value.get('id')) is int and value['id'] == run_id
            and type(value.get('repository_id')) is int and value['repository_id'] == ticket['repo_id']
            and type(value.get('head_repository_id')) is int and value['head_repository_id'] == ticket['repo_id']
            and value.get('head_sha') == ticket['head'] and value.get('head_branch') == ticket['ref'],
            'Image artifact source/repository/run identity mismatch')


def collect(api, plan, ticket, run_id):
    def budget():
        if 'deadline' in ticket:
            require(time.monotonic() < ticket['deadline'], 'Image scan evidence deadline exhausted')
    budget()
    validate_ticket(api, plan, ticket)
    matches, _, _ = history(api, ticket['marker'])
    require(len(matches) == 1 and matches[0]['id'] == run_id, 'Image scan marker missing or ambiguous')
    child(api, ticket, matches[0])
    run = api.get('/actions/runs/%d' % run_id)
    child(api, ticket, run)
    require(run['id'] == run_id and run.get('status') == 'completed'
            and run.get('conclusion') == matches[0].get('conclusion'),
            'Image scan must be completely terminal')
    path = '/actions/runs/%d/artifacts' % run_id
    artifacts = api.pages(path, 'artifacts', require_total=True)
    require(len(artifacts) == len(EXPECTED) and all(type(a) is dict and positive(a.get('id')) for a in artifacts)
            and len({a['id'] for a in artifacts}) == len(EXPECTED), 'Exactly five unique image artifacts are required')
    images = {}
    for component in EXPECTED:
        budget()
        name = 'image-security-' + component + '-' + ticket['head'] + '-1'
        matches_artifact = [a for a in artifacts if a.get('name') == name]
        require(len(matches_artifact) == 1, 'Image artifact name missing or duplicated')
        artifact = matches_artifact[0]
        artifact_proof(artifact, ticket, run_id, component)
        raw = download_zip(api.repo, artifact['id'])
        budget()
        require(len(raw) == artifact['size_in_bytes'] and sha256(raw) == artifact['digest'],
                'Downloaded image artifact byte size/digest mismatch')
        images[component] = dict(verify_archive(raw, component, ticket['head']),
            artifact_id=artifact['id'], artifact_name=name, artifact_sha256=artifact['digest'],
            artifact_bytes=artifact['size_in_bytes'], artifact_expires_at=artifact['expires_at'])
    budget()
    image_jobs, inventory, jobs = jobs_proof(api, ticket, run_id, images)
    failure = any(job['conclusion'] == 'failure' for job in jobs)
    require(run['conclusion'] == ('failure' if failure else 'success'), 'Image advisory conclusion/findings mismatch')
    final = api.get('/actions/runs/%d' % run_id)
    child(api, ticket, final)
    require(final['id'] == run_id and final.get('status') == 'completed'
            and final.get('conclusion') == run['conclusion']
            and api.pages(path, 'artifacts', require_total=True) == artifacts
            and api.pages('/actions/runs/%d/jobs?filter=all' % run_id, 'jobs', require_total=True) == jobs,
            'Image evidence changed during validation')
    validate_ticket(api, plan, ticket)
    final_matches, _, _ = history(api, ticket['marker'])
    require(len(final_matches) == 1 and final_matches[0]['id'] == run_id, 'Image scan history changed during validation')
    child(api, ticket, final_matches[0])
    require(final_matches[0].get('status') == 'completed'
            and final_matches[0].get('conclusion') == run['conclusion'], 'Image terminal history changed during validation')
    budget()
    return {'version': 1, 'run_id': run_id, 'conclusion': run['conclusion'],
            'ticket': {k: v for k, v in ticket.items() if k != 'deadline'}, 'inventory': inventory,
            'images': [dict(images[name], job_id=image_jobs[name]) for name in sorted(EXPECTED)]}


def wait(api, plan, ticket):
    require(type(ticket.get('deadline')) in (int, float), 'Bounded image scan deadline missing')
    run_id, next_authority = None, 0
    while time.monotonic() < ticket['deadline']:
        if time.monotonic() >= next_authority:
            validate_ticket(api, plan, ticket)
            matches, _, pages = history(api, ticket['marker'])
            if matches:
                child(api, ticket, matches[0])
                require(run_id is None or matches[0]['id'] == run_id, 'Pinned image run changed during wait')
                run_id = matches[0]['id']
            else:
                require(run_id is None, 'Pinned image workflow history disappeared')
            # Six authority GETs plus every full history page are paced as one
            # batch. Missing marker visibility cannot cause a fast full scan.
            next_authority = time.monotonic() + max(AUTHORITY_SECONDS,
                (6 + pages) * 3600 / AUTHORITY_READS_PER_HOUR)
        require(time.monotonic() < ticket['deadline'], 'Image scan evidence deadline exhausted')
        if run_id is not None:
            run = api.get('/actions/runs/%d' % run_id)
            require(type(run) is dict and run.get('id') == run_id, 'Pinned actual image run ID changed')
            child(api, ticket, run)
            if run.get('status') == 'completed':
                # No cached parent/history authority can authorize success.
                evidence = collect(api, plan, ticket, run_id)
                require(time.monotonic() < ticket['deadline'], 'Image scan evidence deadline exhausted')
                return evidence
        time.sleep(min(POLL_SECONDS, max(0, ticket['deadline'] - time.monotonic())))
    raise Refused('Timed out waiting for complete image evidence; audit required, no replay')


def revalidate(api, plan, evidence):
    require(type(evidence) is dict and evidence.get('version') == 1 and positive(evidence.get('run_id'))
            and type(evidence.get('ticket')) is dict and evidence['ticket'].get('phase') == 'candidate',
            'Prepared plan has no mandatory candidate image evidence; prepare again')
    current = collect(api, plan, evidence['ticket'], evidence['run_id'])
    require(canonical(current) == canonical(evidence), 'Prepared image evidence changed; fresh approval required')
    return current


def summary(stream, evidence):
    ticket = evidence['ticket']
    stream.write('\n### Verified %s image advisory\n\n' % ticket['phase'])
    stream.write('Scan run `%d`, source `%s`, native parent `%d`: **%s**.\n\n' % (
        evidence['run_id'], ticket['head'], ticket['parent_run'], evidence['conclusion']))
    stream.write('| Image | Platform | UNKNOWN | LOW | MEDIUM | HIGH | CRITICAL |\n'
                 '| --- | --- | ---: | ---: | ---: | ---: | ---: |\n')
    for value in evidence['images']:
        stream.write('| %s | %s | %s |\n' % (value['component'], value['platform'],
            ' | '.join(str(value['vulnerability_counts'][severity]) for severity in SEVERITIES)))
    stream.write('\nAll severities and unfixed CVEs remain visible. Finding counts are separate from '
        'scan completion; legacy finding failures remain readable. '
        'Build/scanner/report/upload incompleteness stops this pipeline. '
        'Artifacts correlate image IDs and report bytes; the image archive itself is not uploaded '
        'or independently verified here. Maintenance executor activation remains separate. '
        'Web coverage is the production Docker image; Vercel managed runtime OS is not inspected.\n')
    if evidence.get('inventory'):
        stream.write('\nSeparate filesystem package inventory: **%s**.\n' % evidence['inventory']['conclusion'])
