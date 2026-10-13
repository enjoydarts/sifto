"""Same-run evidence reader. No root mutation, plan generation, or dispatch."""
import hashlib
import os
import re
import sys
import time
from pathlib import Path

from .cli import github
from .host import RECREATED_CONTAINER_IMAGE, strict_json
from . import recreation_identity_matches
from .plan import (HASH, SHA, MaintenanceError, canonical_json, matches, parse_simulation,
                   plan_sha256, require, validate_approval, validate_plan)
from .ssh import Client, parse_hosts
from .store import GitStore, private_write, repo_url, validate_ref

REPOSITORY = 'enjoydarts/sifto'
REPOSITORY_ID = 1164459340
WORKFLOW_ID = 381232664
WORKFLOW_PATH = '.github/workflows/maintenance-apply.yml'
OWNER = (19749005, 'enjoydarts', 'User')
BOT = (41898282, 'github-actions[bot]', 'Bot')
INPUTS = ('private_commit', 'plan_id', 'plan_sha256', 'executor_sha', 'approval_commit', 'approval_sha256')
FORBIDDEN = ('MAINTENANCE_APPLY_SSH_KEY', 'MAINTENANCE_APPROVAL_PRIVATE_KEY',
             'MAINTENANCE_OPS_READ_KEY', 'MAINTENANCE_ANTHROPIC_API_KEY')


def identity(user):
    require(type(user) is dict and type(user.get('id')) is int, 'actor_identity_invalid')
    return user.get('id'), user.get('login'), user.get('type')


def validate_workflow(workflow):
    require(type(workflow) is dict and workflow.get('id') == WORKFLOW_ID and workflow.get('path') == WORKFLOW_PATH
            and workflow.get('state') == 'active', 'registered_workflow_mismatch')


def validate_authority(run, jobs, reviews, run_id, attempt, source):
    require(re.fullmatch(r'[1-9][0-9]{0,29}', run_id) and attempt == '1' and matches(SHA, source), 'run_context_invalid')
    require(type(run) is dict and run.get('id') == int(run_id) and run.get('run_attempt') == 1
            and run.get('workflow_id') == WORKFLOW_ID and run.get('path') == WORKFLOW_PATH
            and run.get('event') == 'workflow_dispatch' and run.get('head_branch') == 'main'
            and run.get('head_sha') == source, 'actual_run_mismatch')
    for key in ('repository', 'head_repository'):
        require(type(run.get(key)) is dict and run[key].get('id') == REPOSITORY_ID
                and run[key].get('full_name') == REPOSITORY, 'actual_repository_mismatch')
    require(identity(run.get('actor')) in (OWNER, BOT) and identity(run.get('triggering_actor')) in (OWNER, BOT),
            'actual_actor_mismatch')
    require(type(jobs) is list and 3 <= len(jobs) <= 1000, 'actual_jobs_missing')
    for name in ('prepare', 'approve', 'execute'):
        selected = [job for job in jobs if type(job) is dict and job.get('name') == name]
        require(len(selected) == 1, 'actual_job_ambiguous')
        job = selected[0]
        require(job.get('run_id') == int(run_id) and job.get('run_attempt') == 1 and job.get('head_sha') == source
                and job.get('status') == 'completed' and job.get('conclusion') == 'success', 'actual_job_mismatch')
    require(type(reviews) is list and len(reviews) <= 1000, 'actual_reviews_invalid')
    selected = [review for review in reviews if type(review) is dict and any(
        type(env) is dict and env.get('name') == 'production-maintenance' for env in review.get('environments', []))]
    require(bool(selected) and all(review.get('state') == 'approved' and identity(review.get('user')) == OWNER
                                  for review in selected), 'actual_owner_review_missing')
    return OWNER[1]


def read_jobs(run_id, get=github):
    jobs, total = [], None
    for page in range(1, 11):
        response = get('/repos/' + REPOSITORY + '/actions/runs/' + run_id + '/attempts/1/jobs?per_page=100&page=' + str(page))
        require(type(response) is dict and type(response.get('total_count')) is int
                and 0 <= response['total_count'] <= 1000 and type(response.get('jobs')) is list,
                'jobs_pagination_invalid')
        require(total is None or response['total_count'] == total, 'jobs_pagination_changed')
        total = response['total_count']
        values = response['jobs']
        require(len(values) == min(100, total - len(jobs)), 'jobs_pagination_incomplete')
        jobs.extend(values)
        require(all(type(job) is dict and type(job.get('id')) is int for job in jobs)
                and len({job['id'] for job in jobs}) == len(jobs), 'jobs_pagination_duplicate')
        if len(jobs) == total:
            return jobs
    raise MaintenanceError('jobs_pagination_incomplete')


def validate_documents(plan_raw, envelope_raw, inputs, executor, run_id, reviewer):
    require(inputs.get('dry_run') is False and inputs.get('executor_sha') == executor and matches(SHA, executor), 'apply_only')
    plan = validate_plan(strict_json(plan_raw))
    require(plan['plan_id'] == inputs.get('plan_id') and plan['workflow_sha'] == executor
            and matches(HASH, inputs.get('plan_sha256')) and plan_sha256(plan) == inputs['plan_sha256'], 'immutable_plan_mismatch')
    require(matches(HASH, inputs.get('approval_sha256'))
            and hashlib.sha256(envelope_raw).hexdigest() == inputs['approval_sha256'], 'exact_envelope_mismatch')
    envelope = strict_json(envelope_raw)
    require(type(envelope) is dict and set(envelope) == {'plan', 'approval', 'signature'}
            and envelope['plan'] == plan and isinstance(envelope['signature'], str) and bool(envelope['signature']), 'signed_plan_mismatch')
    validate_approval(plan, envelope['approval'])
    approval = envelope['approval']
    require(approval['run_id'] == run_id and approval['approved_by'] == reviewer
            and approval['execution_mode'] == 'apply', 'signed_authority_mismatch')
    return plan, envelope


def validate_observation(plan, envelope, host, inventory, history):
    require(type(inventory) is dict and type(history) is dict and inventory.get('host_id') == plan['host_id']
            and history.get('host_id') == plan['host_id'] == host['host_id'], 'observation_host_mismatch')
    receipts = history.get('receipts')
    require(type(receipts) is list and len(receipts) <= 20 and type(history.get('receipt_total')) is int
            and type(history.get('receipt_truncated')) is int and history['receipt_truncated'] >= 0
            and history['receipt_total'] == len(receipts) + history['receipt_truncated'] and history.get('receipt_history_complete') is True
            and type(history.get('unresolved_execution_count')) is int and history['unresolved_execution_count'] == 0,
            'receipt_history_incomplete')
    require(all(type(receipt) is dict and type(receipt.get('plan')) is dict for receipt in receipts), 'receipt_invalid')
    selected = [receipt for receipt in receipts if receipt['plan'].get('plan_id') == plan['plan_id']]
    require(len(selected) == 1, 'selected_receipt_ambiguous')
    receipt = selected[0]
    require(receipt.get('status') == 'verified' and receipt['plan'] == plan
            and receipt.get('approval') == envelope['approval'], 'selected_receipt_mismatch')
    approval = envelope['approval']
    times = [receipt.get(key) for key in ('started_at', 'completed_at', 'verified_at')]
    if plan.get('action') == 'reboot' and 'completed_at' not in receipt:
        times[1] = times[2]
    require(all(type(value) is int for value in times)
            and plan['created_at'] <= approval['approved_at'] <= times[0] <= times[1] <= times[2]
            and times[0] < min(plan['expires_at'], approval['expires_at']), 'receipt_time_invalid')
    require(type(inventory.get('collected_at')) is int and inventory['collected_at'] >= times[2], 'observation_time_invalid')
    policy, runtime, before = inventory.get('maintenance_policy'), inventory.get('runtime'), receipt.get('before_runtime')
    require(type(policy) is dict and policy.get('host_role') == host['role']
            and type(runtime) is dict and type(before) is dict, 'observation_role_mismatch')
    packages = inventory.get('packages')
    require(type(packages) is list and all(type(x) is dict and isinstance(x.get('name'), str)
            and isinstance(x.get('version'), str) for x in packages), 'observed_packages_invalid')
    actual = {x['name']: x['version'] for x in packages}
    require(len(actual) == len(packages) and all(actual.get(x['name']) == x['target'] for x in plan['packages']), 'installed_targets_mismatch')
    require(isinstance(before.get('boot_id'), str) and isinstance(runtime.get('boot_id'), str), 'boot_identity_missing')
    boot_matches = runtime['boot_id'] != before['boot_id'] if plan.get('action') == 'reboot' else runtime['boot_id'] == before['boot_id']
    require(all(runtime.get(key) == value for key, value in plan['expected_runtime'].items()) and boot_matches, 'runtime_mismatch')
    require(type(inventory.get('dpkg_state')) is dict and inventory['dpkg_state'].get('ready') is True, 'dpkg_not_ready')
    services, before_services = runtime.get('services'), before.get('services')
    require(type(services) is dict and type(before_services) is dict and bool(before_services)
            and set(before_services) <= set(services) and set(plan['restart_services']) <= set(services)
            and all(value == 'active' for value in services.values()), 'monitored_service_inactive')
    containers, originals = runtime.get('containers'), before.get('containers')
    require(type(containers) is list and type(originals) is list and all(type(x) is dict for x in containers + originals), 'containers_invalid')
    current = {x.get('id'): x for x in containers}
    require(len(current) == len(containers) and len({x.get('id') for x in originals}) == len(originals), 'container_identity_ambiguous')
    recreated = []
    for original in originals:
        observed = current.get(original.get('id'))
        if observed is None:
            # Recognition of the exact receipt already VERIFIED by native root,
            # not a new opt-in or authority to mutate root configuration.
            restart_permitted = ((plan.get('action', 'apt_upgrade') == 'apt_upgrade'
                                  and plan.get('allow_service_restarts') is True
                                  and plan.get('restart_scope') == 'package_scripts_and_listed_services')
                                 or (plan.get('action') == 'reboot' and plan.get('allow_reboot') is True))
            same_image = lambda item: item.get('image') == original.get('image') and item.get('image_id') == original.get('image_id')
            replacements = [item for item in containers if same_image(item)]
            require(host['role'] == 'application' and restart_permitted and original.get('status') == 'running'
                    and matches(RECREATED_CONTAINER_IMAGE, original.get('image'))
                    and len([item for item in originals if same_image(item)]) == 1 and len(replacements) == 1,
                    'original_container_changed')
            observed = replacements[0]
            require(observed.get('id') != original.get('id') and observed.get('status') == 'running'
                    and observed.get('health') == 'healthy'
                    and recreation_identity_matches(original.get('recreation_identity'), observed.get('recreation_identity')),
                    'root_verified_sentinel_recreation_changed')
            recreated.append({'original_id': original['id'], 'replacement_id': observed['id'],
                              'image': original['image'], 'image_id': original['image_id']})
            continue
        require(observed is not None and all(key in original and observed.get(key) == original[key]
                for key in ('id', 'image_id', 'image', 'recreation_identity'))
                and observed.get('status') == 'running' and observed.get('health') in ('healthy', 'none'), 'original_container_changed')
    require(all(x.get('status') == 'running' and x.get('health') in ('healthy', 'none') for x in containers), 'observed_container_unhealthy')
    if host['role'] == 'database':
        require(policy.get('database_enabled') is True and policy.get('backup_ready') is True
                and 'pgbouncer.service' in services and any(re.fullmatch(r'postgresql@[^/]+\.service', name) for name in services)
                and runtime.get('database_pool_health') == {'status': 'healthy', 'reason': 'read_only_route_verified'}, 'database_health_invalid')
    pending = None
    try:
        pending = len(parse_simulation(inventory.get('simulation', '')))
    except MaintenanceError:
        pass
    return {'status': 'verified', 'target_count': len(plan['packages']), 'container_count': len(originals),
            'preserved_container_count': len(originals) - len(recreated), 'recreated_container_count': len(recreated),
            'recreated_containers': recreated,
            'service_count': len(services), 'pending_update_count': pending, 'observed_at': inventory['collected_at']}


def semantic(raw, path):
    if not path.endswith('/observation.json'):
        return raw
    value = strict_json(raw)
    if type(value) is dict:
        value = {key: item for key, item in value.items() if key != 'observed_at'}
    return value


def append_record(store, files):
    """At most three CAS appends; an unknown write gets only a new-parent reconciliation."""
    previous = None
    for index in range(4):
        parent = store.head()
        present = set(store.git('ls-tree', '--name-only', parent, '--', *sorted(files)).decode().splitlines())
        if present:
            # GitStore maps empty blobs to None; ls-tree above proves presence,
            # so preserve an empty diagnostic as exact zero-length bytes.
            require(present == set(files) and all(semantic(store.read_at(parent, name) or b'', name) == semantic(raw, name)
                                                 for name, raw in files.items()), 'immutable_post_verification_conflict')
            return parent
        require(parent != previous, 'post_verification_write_unknown_without_new_parent')
        require(index < 3, 'post_verification_write_retries_exhausted')
        try:
            return store.write(parent, files)
        except MaintenanceError:
            previous = parent
    raise MaintenanceError('post_verification_write_retries_exhausted')


def validate_source(source, env=os.environ, image=Path('/src')):
    require(env.get('GITHUB_REF') == 'refs/heads/main' and env.get('GITHUB_REPOSITORY') == REPOSITORY
            and env.get('GITHUB_SHA') == env.get('MAINTENANCE_PIPELINE_SHA') == env.get('MAINTENANCE_CHECKED_OUT_SHA')
            and matches(SHA, env.get('GITHUB_SHA')), 'pipeline_checkout_mismatch')
    manifest = source.decode().splitlines()
    names = set()
    for line in manifest:
        match = re.fullmatch(r'100644 blob ([0-9a-f]{40})\tinfra/maintenance/(requirements.txt|(?:maintenance|tests|playbooks)/[A-Za-z0-9_.-]+)', line)
        require(match is not None, 'pipeline_source_manifest_invalid')
        digest, name = match.groups()
        raw = (image / name).read_bytes()
        require(hashlib.sha1(b'blob ' + str(len(raw)).encode() + b'\0' + raw).hexdigest() == digest, 'pipeline_image_mismatch')
        names.add(name)
    expected = {'requirements.txt'} | {str(path.relative_to(image)) for directory in ('maintenance', 'tests', 'playbooks')
        for path in (image / directory).iterdir() if path.is_file()}
    require(names == expected and 'maintenance/post_apply.py' in names, 'pipeline_image_incomplete')


def prepare():
    require(not any(os.environ.get(key) for key in FORBIDDEN), 'post_verification_credentials_scope_invalid')
    secrets = Path('/secrets')
    for name, key in (('host_key', 'MAINTENANCE_SCAN_SSH_KEY'), ('ops_key', 'MAINTENANCE_OPS_WRITE_KEY'),
                      ('hosts.json', 'MAINTENANCE_HOSTS_JSON'), ('known_hosts', 'MAINTENANCE_KNOWN_HOSTS')):
        require(bool(os.environ.get(key)), 'post_verification_credentials_missing')
        private_write(secrets / name, (os.environ[key] + '\n').encode())
    inputs = {key: os.environ.get('INPUT_' + key.upper(), '') for key in INPUTS}
    require(os.environ.get('INPUT_DRY_RUN') == 'false', 'post_verification_apply_only')
    inputs['dry_run'] = False
    private_write(secrets / 'inputs.json', inputs)


def run():
    secrets, work = Path('/secrets'), Path('/private')
    validate_source((secrets / 'source.txt').read_bytes())
    require(not any(os.environ.get(key) for key in FORBIDDEN)
            and {p.name for p in secrets.iterdir()} == {'source.txt', 'host_key', 'ops_key', 'hosts.json', 'known_hosts', 'inputs.json'},
            'post_verification_credentials_scope_invalid')
    inputs = strict_json((secrets / 'inputs.json').read_bytes())
    require(type(inputs) is dict and set(inputs) == set(INPUTS) | {'dry_run'}, 'post_verification_inputs_invalid')
    executor, source = os.environ.get('MAINTENANCE_EXECUTOR_SHA', ''), os.environ['GITHUB_SHA']
    run_id, attempt = os.environ.get('GITHUB_RUN_ID', ''), os.environ.get('GITHUB_RUN_ATTEMPT', '')
    require(re.fullmatch(r'[1-9][0-9]{0,29}', run_id) and attempt == '1', 'run_context_invalid')
    path = '/repos/' + REPOSITORY + '/actions/runs/' + run_id
    validate_workflow(github('/repos/' + REPOSITORY + '/actions/workflows/' + str(WORKFLOW_ID)))
    actual = github(path)
    reviewer = validate_authority(actual, read_jobs(run_id), github(path + '/approvals'), run_id, attempt, source)
    repo = os.environ.get('MAINTENANCE_OPS_REPO', '')
    repo_url(repo)
    store = GitStore(work / 'git', repo, secrets / 'ops_key', secrets / 'known_hosts', writer=True)
    plan_path = 'plans/' + inputs['plan_id'] + '/plan.json'
    envelope_path = 'approvals/' + inputs['plan_id'] + '/' + run_id + '.json'
    validate_ref(inputs['private_commit'], plan_path)
    validate_ref(inputs['approval_commit'], envelope_path)
    plan_raw = store.fetch(inputs['private_commit'], plan_path)
    envelope_raw = store.fetch(inputs['approval_commit'], envelope_path, digest=inputs['approval_sha256'])
    plan, envelope = validate_documents(plan_raw, envelope_raw, inputs, executor, run_id, reviewer)
    hosts = parse_hosts((secrets / 'hosts.json').read_bytes())
    host = next((host for host in hosts if host['host_id'] == plan['host_id']), None)
    require(host is not None, 'selected_host_missing')
    client = Client(work, secrets / 'host_key', secrets / 'known_hosts', 'scan')
    prefix = 'post-verifications/' + run_id + '-' + attempt
    reference = {'run_id': run_id, 'run_attempt': attempt, 'pipeline_sha': source, 'executor_sha': executor,
                 **{key: inputs[key] for key in INPUTS}, 'plan_path': plan_path, 'envelope_path': envelope_path}
    files = {prefix + '/references.json': canonical_json(reference)}
    failure = None
    try:
        inventory = client.call(host, 'scan')
        files[prefix + '/inventory.json'] = canonical_json(inventory)
        history = client.call(host, 'report')
        files[prefix + '/root-receipts.json'] = canonical_json(history)
        result = validate_observation(plan, envelope, host, inventory, history)
    except Exception as error:
        failure = error
        result = {'status': 'failed', 'observed_at': int(time.time())}
        private_write(work / 'validation-error.txt', str(error).encode())
    files[prefix + '/observation.json'] = canonical_json(result)
    # Raw host/GitHub/validation diagnostics never reach the public summary.
    for diagnostic in sorted((work / 'transport').glob('*/*.bin')):
        files[prefix + '/diagnostics/' + diagnostic.parent.name + '/' + diagnostic.name] = diagnostic.read_bytes()
    if failure:
        files[prefix + '/diagnostics/validation-error.txt'] = (work / 'validation-error.txt').read_bytes()
    commit = append_record(store, files)
    message = 'Maintenance post-verification: ' + result['status'] + '.\n'
    if not failure:
        message += ('Installed targets: ' + str(result['target_count']) + '; containers checked: ' + str(result['container_count'])
                    + '; verified Sentinel recreations: ' + str(result['recreated_container_count']) + '.\n')
    message += 'Private evidence: https://github.com/' + repo + '/tree/' + commit + '/' + prefix + '\n'
    if failure:
        message += 'Do not retry apply. The daily scanner remains the next observation; inspect private evidence.\n'
    private_write(work / 'summary.md', message.encode())
    require(failure is None, 'post_verification_failed_no_apply_retry')


def main():
    try:
        if sys.argv[1:] == ['prepare']:
            prepare()
        else:
            require(not sys.argv[1:], 'post_verification_invalid_operation')
            run()
    except Exception as error:
        private_write('/private/error.txt', str(error).encode())
        if not Path('/private/summary.md').exists():
            private_write('/private/summary.md', b'Maintenance post-verification failed. Do not retry apply. The daily scanner remains the next observation; operator diagnosis is required.\n')
        print('maintenance_post_verification_failed_no_apply_retry', file=sys.stderr)
        return 1
    return 0


if __name__ == '__main__':
    raise SystemExit(main())
