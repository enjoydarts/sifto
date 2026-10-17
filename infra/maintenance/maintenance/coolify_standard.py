"""Bounded official standard update, with mutable CDN dependencies explicitly accepted.

The signed script commit/hash is not an artifact or image digest closure. The
unmodified upstream script downloads its normal mutable CDN configuration.
"""
import hashlib
import re
import time
import urllib.request

from .plan import HASH, SHA, build_plan, matches, require

MAX_SCRIPT = 256 * 1024
HELPER_VERSION = '1.0.17'
ROLES = ['coolify', 'postgres', 'redis']
STABLE = re.compile(r'(?:0|[1-9][0-9]{0,5})\.(?:0|[1-9][0-9]{0,5})\.(?:0|[1-9][0-9]{0,5})\Z')
ROLE_IMAGES = {'coolify': r'(?:docker\.io/|ghcr\.io/)?coollabsio/coolify:v?([0-9]+\.[0-9]+\.[0-9]+)',
               'postgres': r'(?:docker\.io/(?:library/)?)?postgres:[0-9]+(?:\.[0-9]+)*(?:-alpine[0-9.]*)?',
               'redis': r'(?:docker\.io/(?:library/)?)?redis:[0-9]+(?:\.[0-9]+)*(?:-alpine[0-9.]*)?'}
ROLE_NAMES = {'coolify': '/coolify', 'postgres': '/coolify-db', 'redis': '/coolify-redis'}
IDLE_SQL = """BEGIN READ ONLY;
SELECT json_build_array(
 (SELECT count(*) FROM application_deployment_queues WHERE status IS NULL OR status NOT IN ('finished','failed','cancelled-by-user')),
 (SELECT count(*) FROM scheduled_database_backup_executions WHERE status IS NULL OR status NOT IN ('success','failed')),
 (SELECT count(*) FROM scheduled_volume_backup_executions WHERE status IS NULL OR status NOT IN ('success','failed') OR stop_recovery_pending IS TRUE OR s3_cleanup_pending IS TRUE),
 (SELECT count(*) FROM scheduled_job_deliveries WHERE job_type IN ('database-backup','volume-backup') AND (status IS NULL OR status NOT IN ('failed','skipped'))),
 (SELECT CASE WHEN count(*) = 1 AND bool_and(is_auto_update_enabled IS FALSE) THEN 0 ELSE 1 END FROM instance_settings)
); COMMIT;"""
# Load the same effective configuration (including config cache) without booting
# providers, database clients, queues, scheduler or the HTTP/console kernel.
CONFIG_PROBE = """require '/var/www/html/vendor/autoload.php';
$app = require '/var/www/html/bootstrap/app.php';
(new Illuminate\\Foundation\\Bootstrap\\LoadEnvironmentVariables)->bootstrap($app);
(new Illuminate\\Foundation\\Bootstrap\\LoadConfiguration)->bootstrap($app);
echo json_encode(['version'=>$app['config']->get('constants.coolify.version'),
 'autoupdate'=>$app['config']->get('constants.coolify.autoupdate')], JSON_THROW_ON_ERROR);"""


def probe(runner, runtime, expected_version):
    """Fixed RO probes from the official 4.4.2/4.4.6 schema; unknown fails closed."""
    from .host import strict_json
    selected = roles(runtime)
    image_version = re.fullmatch(ROLE_IMAGES['coolify'] + r'(?:@sha256:[0-9a-f]{64})?', selected['coolify']['image']).group(1)
    require(image_version == expected_version, 'Coolify image version changed')
    code = runner(['docker', 'exec', selected['coolify']['id'], 'cat', '/var/www/html/config/constants.php'], timeout=30)
    versions = re.findall(r"'version'\s*=>\s*env\('COOLIFY_VERSION'\)\s*\?:\s*'([0-9]+\.[0-9]+\.[0-9]+)'", code)
    require(versions == [expected_version], 'Coolify actual code version unavailable or changed')
    config = strict_json(runner(['docker', 'exec', selected['coolify']['id'], 'php', '-r', CONFIG_PROBE], timeout=30))
    require(type(config) is dict and set(config) == {'version', 'autoupdate'} and config['version'] == expected_version
            and (config['autoupdate'] is False or config['autoupdate'] is None), 'effective Coolify auto update or version unknown')
    idle = runner(['docker', 'exec', selected['postgres']['id'], 'psql', '-X', '-q', '-A', '-t',
                   '-v', 'ON_ERROR_STOP=1', '--username=coolify', '--dbname=coolify', '--command', IDLE_SQL], timeout=30)
    counts = strict_json(idle)
    require(type(counts) is list and len(counts) == 5 and all(type(x) is int and x == 0 for x in counts),
            'Coolify deployment or backup active or unknown')
    runner(['docker', 'exec', selected['coolify']['id'], 'curl', '--fail', '--silent', '--show-error',
            '--max-time', '10', 'http://127.0.0.1:8080/api/health'], timeout=20)


def validate_action(action):
    require(type(action) is dict and set(action) == {'from_version', 'to_version', 'release_commit',
            'upgrade_sha256', 'helper_version', 'registry', 'replace_roles', 'release_published_at'}, 'invalid Coolify action fields')
    require(type(action['release_published_at']) is int and action['release_published_at'] >= 0, 'invalid release publication time')
    require(matches(STABLE, action['from_version']) and matches(STABLE, action['to_version']), 'fixed stable Coolify versions required')
    before, after = (tuple(map(int, action[key].split('.'))) for key in ('from_version', 'to_version'))
    require(before[:2] == after[:2] and before < after, 'same-series Coolify patch required')
    require(matches(SHA, action['release_commit']) and matches(HASH, action['upgrade_sha256']), 'invalid official script identity')
    require(action['helper_version'] == HELPER_VERSION and action['registry'] == 'docker.io'
            and action['replace_roles'] == ROLES, 'invalid standard update scope')


def fetch(url, limit):
    request = urllib.request.Request(url, headers={'User-Agent': 'Sifto-Maintenance'}, method='GET')
    # Reject redirects before following them, including redirects to local hosts.
    class NoRedirect(urllib.request.HTTPRedirectHandler):
        def redirect_request(self, req, fp, code, msg, headers, newurl):
            return None
    with urllib.request.build_opener(NoRedirect()).open(request, timeout=20) as response:
        require(response.geturl() == url, 'unexpected official source redirect')
        raw = response.read(limit + 1)
    require(0 < len(raw) <= limit, 'official source exceeds bounds')
    return raw


def fetch_script(commit):
    require(matches(SHA, commit), 'immutable source commit required')
    raw = fetch('https://raw.githubusercontent.com/coollabsio/coolify/' + commit + '/scripts/upgrade.sh', MAX_SCRIPT)
    require(raw.startswith(b'#!/bin/bash\n') and b'\x00' not in raw, 'invalid official upgrade script')
    return raw


def release_commit(tag):
    require(isinstance(tag, str) and re.fullmatch(r'v?' + STABLE.pattern.removesuffix(r'\Z'), tag), 'invalid official release tag')
    from .host import strict_json
    value = strict_json(fetch('https://api.github.com/repos/coollabsio/coolify/commits/' + tag, 256 * 1024))
    require(type(value) is dict and matches(SHA, value.get('sha')), 'official release commit unavailable')
    return value['sha']


def roles(runtime):
    containers = runtime['containers']
    selected = {}
    for role, name in ROLE_NAMES.items():
        found = [item for item in containers if item.get('recreation_identity', {}).get('name') == name]
        require(len(found) == 1, 'Coolify role missing or ambiguous')
        item = found[0]
        identity = item['recreation_identity']
        require(identity.get('compose') == {'project': 'source', 'service': role}
                and isinstance(item.get('image'), str)
                and re.fullmatch(ROLE_IMAGES[role] + r'(?:@sha256:[0-9a-f]{64})?', item['image'])
                and item.get('status') == 'running' and item.get('health') == 'healthy', 'unhealthy or unknown Coolify role')
        selected[role] = item
    require(len({item['id'] for item in selected.values()}) == len(ROLES), 'ambiguous Coolify roles')
    return selected


def build_candidate(inventory, observation, workflow_sha, *, now=None):
    now = int(time.time()) if now is None else now
    require(inventory['maintenance_policy'].get('coolify_standard_enabled') is True
            and inventory['maintenance_policy']['host_role'] == 'application'
            and inventory['maintenance_policy'].get('backup_ready') is True, 'Coolify standard update disabled or backup unavailable')
    require(observation.get('status') == 'update_available' and observation.get('candidate_review') == 'eligible_patch_review'
            and type(observation.get('release_published_at')) is int
            and 0 <= observation['release_published_at'] <= now - 7 * 86400, 'Coolify patch waiting or unavailable')
    selected = roles(inventory['runtime'])
    before, after = observation.get('current_version'), observation.get('latest_version')
    action = {'from_version': before, 'to_version': after, 'release_commit': '0' * 40,
              'upgrade_sha256': '0' * 64, 'helper_version': HELPER_VERSION, 'registry': 'docker.io', 'replace_roles': ROLES.copy(),
              'release_published_at': observation['release_published_at']}
    validate_action(action)
    require(re.fullmatch(ROLE_IMAGES['coolify'] + r'(?:@sha256:[0-9a-f]{64})?', selected['coolify']['image']).group(1) == before,
            'Coolify observed version disagrees with inventory')
    prefix = 'https://github.com/coollabsio/coolify/releases/tag/'
    url = observation.get('release_url')
    require(url in (prefix + after, prefix + 'v' + after), 'invalid official release URL')
    action['release_commit'] = release_commit(url.removeprefix(prefix))
    action['upgrade_sha256'] = hashlib.sha256(fetch_script(action['release_commit'])).hexdigest()
    return build_plan(inventory, workflow_sha, now=now, action='coolify_update', allow_service_restarts=False,
                      coolify_update=action)


def upgrade_argv(path, action):
    validate_action(action)
    return ['/bin/bash', str(path), action['to_version'], HELPER_VERSION, 'docker.io']


def verify_containers(before, after, action):
    validate_action(action)
    previous, current = roles(before), roles(after)
    expected_image = 'docker.io/coollabsio/coolify:' + action['to_version']
    require(current['coolify']['image'] == expected_image, 'Coolify target image mismatch')
    selected_ids = {item['id'] for item in previous.values()}
    current_apps = {item['id']: item for item in after['containers'] if item['id'] not in {x['id'] for x in current.values()}}
    originals = [item for item in before['containers'] if item['id'] not in selected_ids]
    require(set(current_apps) == {item['id'] for item in originals}, 'application container set changed')
    for item in originals:
        observed = current_apps[item['id']]
        require(all(observed.get(key) == item.get(key) for key in ('id', 'image', 'image_id', 'recreation_identity'))
                and observed.get('status') == 'running' and observed.get('health') in ('healthy', 'none'),
                'application container identity or health changed')
