"""Fixed clone startup primitives; no execution registration or authority.

The unchanged ARM images have been inspected independently. These exact guard
resources suppress app:init, automatic configuration and the webhook maintenance
exception while retaining stock entrypoint/S6. Functional fixture success is
not native source-owner, drain, SSH reconstruction or production proof. Task4B/C
must complete before OwnedDocker may accept any ControlledStartup record.

Source resources are installed as root-owned 0644 inputs. Future authenticated
staging must make the init-script leaf root-owned 0755 before mounting it RO;
this catalog never installs, stages, reads private input or executes anything.

The SSH producer requires preexisting UID/GID 9999 keys and mux directories at
0700 with no other application writer. The PUBLIC fixture supplies them. Native
prepare_phase still starts with empty runtime SSH storage: future Task4B4 staging
must create these parents before any producer integration can be accepted.

Barrier observations expose only closed categories and identity HMACs. The
clone-only producer snapshots raw Redis expiry and recognized filesystem state
at one fixed private storage leaf, and can restore PRETARGET state. Recorded
file stages may resume; an unrecorded or truncated stage fails closed. Trusted
post-target lifecycle admission and final success release remain unavailable.

The fixed Horizon PSR-4 receiver authenticates its own command and live owned
handles. Clone-only graceful drain retains those handles and original working
flags, pauses monitoring, and sends only parent-owned TERM. Expiry remains held;
replacement/readiness ACK and exact restoration require a later implementation.
Control is deferred until the stock pending batch finishes; observed conflicts
are rejected. Receipts never establish exclusive control or native authority.
The PUBLIC fixture installs into an empty narrow App tmpfs after the original
supervisor starts. Installation into an original production app remains undefined;
the root-installed source catalog alone supplies no App mount or native gate.
"""
import copy
from dataclasses import dataclass
import json
import re

from .coolify_plan import OLD_ARM, NEW_ARM, profile_sha256


RESOURCE_SHA256 = (
    ('coolify_startup.php', 'e416f42ff32b26f45777d82654e04dda8aa3d7229ed2a4b86f43b52712e91b10'),
    ('coolify_horizon_inspect.php', '2c1e314fcc66a3a030f03be93b857663b81d5c9212431954d3e01e10e5ce5620'),
    ('coolify_startup_init', 'afe14f036a705b057afcd8346b61c3ba06b34cff02199ef6b2dc663bf7196522'),
    ('coolify_startup_middleware.php', 'fe5cb12552e94585d1fa9b831b4e4bac7caf4bbbe657cb0a3cd62b80ac67186d'),
    ('coolify_startup_nginx.conf', '11b1a85b104ec8bd848d2688da694ad8f64cdc9fe88bc67c02fe01583a305284'),
    ('coolify_startup_site.conf', '7a4897ba7bd52753dd72b6c83df00630a64962eed4dbfb78ca6e9f96e851fecd'),
)
ORIGINAL_SHA256 = (
    ('/etc/s6-overlay/s6-rc.d/init-script/up', 'd5c0542b30ffe6581edd5a434cac53bac9aa70de0e628986fb90be1cb4574bbe'),
    ('/var/www/html/app/Http/Middleware/PreventRequestsDuringMaintenance.php',
     '4a4445b3e4ac17a03594e5b84e0b8ff9d24bead5790162e44e4d5832297aa994'),
    ('/etc/s6-overlay/s6-rc.d/horizon/run', '296dfda050b4d25c32fd9a858216a9a492148fd6ef9960644e08f32f7caf5e31'),
    ('/etc/s6-overlay/s6-rc.d/scheduler-worker/run', '9034571c9f328e694720f9c66c461016e96a3d932ade29f5d2498754677d906c'),
    ('/etc/nginx/nginx.conf.template', '1e1d6561af14f68b0146232cc7ee069abe68658bd62b29d5a9c8f35111402295'),
    ('/etc/nginx/site-opts.d/http.conf', '5beded419f31b70c52063ecaf7820c57ad37babbfa5b1d5cb861c4ffe4478b34'),
    ('/usr/local/etc/php-fpm.conf', 'b6b8add8db04ec8217b1d3704da775a6836d8ac8a596f58a5e180f9784fc5a32'),
)
_IMAGES = (
    ('old', OLD_ARM, 'sha256:6145ffe0650d8dcdef48ab08442dc01b066d6450e1f6df9e3cd8efacadbb0064'),
    ('target', NEW_ARM, 'sha256:292e1d3e300d6eab9598e14a017bfa2e35bd89f78597916c179c24e099e40156'),
)


def fixed_candidate():
    """Detached non-authorizing metadata, bound to the complete fixed profile."""
    return copy.deepcopy({'schema_version': 1, 'profile_sha256': profile_sha256(),
        'images': _IMAGES, 'original_sha256': ORIGINAL_SHA256,
        'resource_sha256': RESOURCE_SHA256, 'registered': False,
        'production_eligible': False})


@dataclass(frozen=True, repr=False)
class SshObservation:
    """Typed observation only; construction grants no execution capability."""
    version: int
    operation: str
    selected_version: str
    rows_checked: int
    keys_created: int
    keys_matched: int
    unrelated_key_count: int
    mux_entry_count: int
    db_identity_hmac: str
    existing_keys_hmac_before: str
    existing_keys_hmac_after: str
    mux_hmac_before: str
    mux_hmac_after: str
    created_inventory_hmac: str


def parse_ssh_result(raw, *, selected_version):
    """Parse the fixed producer's exact bounded schema without authority."""
    def reject():
        raise ValueError('startup_ssh_result')

    def pairs(items):
        result = {}
        for key, value in items:
            if key in result:
                reject()
            result[key] = value
        return result

    try:
        if type(raw) is not bytes or not 0 < len(raw) <= 8192 or selected_version not in ('4.4.2', '4.4.6'):
            reject()
        value = json.loads(raw.decode('utf-8', 'strict'), object_pairs_hook=pairs,
                           parse_constant=lambda _: reject())
        if type(value) is not dict or set(value) != set(SshObservation.__dataclass_fields__):
            reject()
        if (type(value['version']) is not int or value['version'] != 1
                or value['operation'] != 'ssh-reconstruct' or value['selected_version'] != selected_version):
            reject()
        for name in ('rows_checked', 'keys_created', 'keys_matched', 'unrelated_key_count', 'mux_entry_count'):
            if type(value[name]) is not int or not 0 <= value[name] <= 1024:
                reject()
        if (value['keys_created'] + value['keys_matched'] != value['rows_checked']
                or value['rows_checked'] + value['unrelated_key_count'] > 1024):
            reject()
        for name in (key for key in value if 'hmac' in key):
            if type(value[name]) is not str or re.fullmatch('[a-f0-9]{64}', value[name]) is None:
                reject()
        if (value['existing_keys_hmac_before'] != value['existing_keys_hmac_after']
                or value['mux_hmac_before'] != value['mux_hmac_after']):
            reject()
        return SshObservation(**value)
    except (ValueError, UnicodeError, TypeError, RecursionError):
        raise ValueError('startup_ssh_result') from None


@dataclass(frozen=True, repr=False)
class BarrierObservation:
    """Closed clone observation. Has no lifecycle or execution capability."""
    version: int
    operation: str
    selected_version: str
    driver: str
    store: str
    connection: str
    database: int
    redis_identity_hmac: str
    cache_prefix_hmac: str
    redis_prefix_hmac: str
    logical_key: str
    key_state: str
    ttl_state: str
    payload_profile: str
    maintenance_file_state: str
    down_file_state: str
    snapshot_state: str
    original_state_hmac: str
    current_state_hmac: str


def parse_barrier_result(raw, *, selected_version, operation):
    """Exact bounded data only; PRETARGET restore never proves final release."""
    def reject():
        raise ValueError('startup_barrier_result')

    def pairs(items):
        result = {}
        for key, value in items:
            if key in result:
                reject()
            result[key] = value
        return result

    try:
        if (type(raw) is not bytes or not 0 < len(raw) <= 8192
                or selected_version not in ('4.4.2', '4.4.6')
                or operation not in ('barrier-observe', 'barrier-enter', 'barrier-restore')):
            reject()
        value = json.loads(raw.decode('utf-8', 'strict'), object_pairs_hook=pairs,
                           parse_constant=lambda _: reject())
        if type(value) is not dict or set(value) != set(BarrierObservation.__dataclass_fields__):
            reject()
        for name, expected in dict(version=1, operation=operation, selected_version=selected_version,
                driver='cache', store='redis', connection='cache', database=1,
                logical_key='illuminate:foundation:down').items():
            if type(value[name]) is not type(expected) or value[name] != expected:
                reject()
        for name, allowed in dict(key_state=('absent', 'persistent', 'expiring', 'unknown'),
                ttl_state=('absent', 'persistent', 'expiring', 'unknown'),
                payload_profile=('absent', 'plain503', 'original', 'unexpected'),
                maintenance_file_state=('absent', 'stock'), down_file_state=('absent', 'original'),
                snapshot_state=('absent', 'preparing', 'held', 'restoring', 'restored')).items():
            if type(value[name]) is not str or value[name] not in allowed:
                reject()
        if (value['key_state'] != value['ttl_state']
                or (value['key_state'] == 'absent') != (value['payload_profile'] == 'absent')
                or (value['key_state'] == 'unknown') != (value['payload_profile'] == 'unexpected')):
            reject()
        if operation == 'barrier-enter' and any(value[name] != expected for name, expected in dict(
                key_state='persistent', payload_profile='plain503', maintenance_file_state='absent',
                down_file_state='absent', snapshot_state='held').items()):
            reject()
        if operation == 'barrier-restore' and value['snapshot_state'] != 'restored':
            reject()
        for name in (key for key in value if 'hmac' in key):
            if type(value[name]) is not str or re.fullmatch('[a-f0-9]{64}', value[name]) is None:
                reject()
        return BarrierObservation(**value)
    except (ValueError, UnicodeError, TypeError, RecursionError):
        raise ValueError('startup_barrier_result') from None


RUNTIME_SERVICES = ('fix-attrs', 'legacy-cont-init', 'db-migration', 'init-seeder',
                    'init-script', 'legacy-services', 'php-fpm', 'nginx', 'reverb',
                    'terminal-server', 'horizon', 'scheduler-worker')


@dataclass(frozen=True, repr=False)
class RuntimeServiceObservation:
    """Ephemeral identity data. Never a capability to signal this PID."""
    name: str
    kind: str
    state: str
    readiness: str
    role: str
    pid: int
    start_time_ticks: int
    identity_hmac: str | None


@dataclass(frozen=True, repr=False)
class RuntimeObservation:
    """Passive clone snapshot, without startup, drain or execution authority."""
    version: int
    operation: str
    selected_version: str
    horizon_enabled: bool
    scheduler_enabled: bool
    services: tuple[RuntimeServiceObservation, ...]
    membership_state: str
    owned_process_count: int
    unclassified_count: int
    source_hmac: str
    config_hmac: str
    membership_hmac_before: str
    membership_hmac_after: str


def parse_runtime_result(raw, *, selected_version):
    """Parse exact bounded data only. No runtime observation authorizes work."""
    def reject():
        raise ValueError('startup_runtime_result')

    def pairs(items):
        value = {}
        for key, item in items:
            if key in value:
                reject()
            value[key] = item
        return value

    def hmac(value):
        return type(value) is str and re.fullmatch('[a-f0-9]{64}', value) is not None

    try:
        if (type(raw) is not bytes or not 0 < len(raw) <= 8192
                or type(selected_version) is not str or selected_version not in ('4.4.2', '4.4.6')):
            reject()
        value = json.loads(raw.decode('utf-8', 'strict'), object_pairs_hook=pairs,
                           parse_constant=lambda _: reject())
        if type(value) is not dict or set(value) != set(RuntimeObservation.__dataclass_fields__):
            reject()
        for name, expected in dict(version=1, operation='runtime-observe', selected_version=selected_version).items():
            if type(value[name]) is not type(expected) or value[name] != expected:
                reject()
        for name in ('horizon_enabled', 'scheduler_enabled'):
            if type(value[name]) is not bool:
                reject()
        for name in ('owned_process_count', 'unclassified_count'):
            if type(value[name]) is not int or not 0 <= value[name] <= 256:
                reject()
        if value['unclassified_count'] > value['owned_process_count']:
            reject()
        for name in ('source_hmac', 'config_hmac', 'membership_hmac_before', 'membership_hmac_after'):
            if not hmac(value[name]):
                reject()
        if (type(value['membership_state']) is not str
                or value['membership_state'] not in ('stable', 'changed', 'unknown')):
            reject()
        equal = value['membership_hmac_before'] == value['membership_hmac_after']
        if (value['membership_state'] == 'stable' and (not equal or value['unclassified_count'])
                or value['membership_state'] == 'changed' and equal):
            reject()
        rows = value['services']
        if type(rows) is not list or len(rows) != len(RUNTIME_SERVICES):
            reject()
        parsed = []
        seen_pids = set()
        for index, (row, name) in enumerate(zip(rows, RUNTIME_SERVICES)):
            if type(row) is not dict or set(row) != set(RuntimeServiceObservation.__dataclass_fields__):
                reject()
            oneshot = index < 6
            if row['name'] != name or row['kind'] != ('oneshot' if oneshot else 'longrun'):
                reject()
            for field in ('pid', 'start_time_ticks'):
                if type(row[field]) is not int or not 0 <= row[field] <= 9007199254740991:
                    reject()
            for field, allowed in dict(
                    state=('active', 'not-active', 'unknown') if oneshot else ('up', 'down', 'missing', 'unknown', 'unstable'),
                    readiness=('notified', 'not-notified', 'unsupported', 'unknown'),
                    role=('none',) if oneshot else (name, 'unknown', 'none', 'disabled-sleep')).items():
                if type(row[field]) is not str or row[field] not in allowed:
                    reject()
            has_identity = row['pid'] > 0
            if (has_identity != (row['start_time_ticks'] > 0)
                    or has_identity != hmac(row['identity_hmac'])
                    or not has_identity and row['identity_hmac'] is not None):
                reject()
            if has_identity:
                if row['pid'] in seen_pids or row['pid'] > 4194304:
                    reject()
                seen_pids.add(row['pid'])
            if oneshot:
                if has_identity or row['readiness'] != 'unsupported':
                    reject()
            else:
                if ((row['role'] == 'none') != (not has_identity)
                        or row['state'] == 'up' and not has_identity
                        or row['state'] in ('down', 'missing') and has_identity):
                    reject()
                if row['role'] == 'disabled-sleep':
                    if name not in ('horizon', 'scheduler-worker') or value[
                            'horizon_enabled' if name == 'horizon' else 'scheduler_enabled']:
                        reject()
                if name in ('horizon', 'scheduler-worker') and row['role'] == name and not value[
                        'horizon_enabled' if name == 'horizon' else 'scheduler_enabled']:
                    reject()
                if row['readiness'] in ('notified', 'not-notified'):
                    if name not in ('php-fpm', 'nginx') or row['state'] != 'up' or row['role'] != name:
                        reject()
            parsed.append(RuntimeServiceObservation(**row))
        if len(seen_pids) > value['owned_process_count']:
            reject()
        value['services'] = tuple(parsed)
        return RuntimeObservation(**value)
    except (ValueError, UnicodeError, TypeError, RecursionError, KeyError):
        raise ValueError('startup_runtime_result') from None


@dataclass(frozen=True, repr=False)
class HorizonInspection:
    """Execution acknowledgement and inventory only. Never idle/drain authority."""
    version: int
    operation: str
    selected_version: str
    phase: str
    supervisor_status: str
    master_status: str
    pool_count: int
    worker_object_count: int
    live_worker_count: int
    terminating_count: int
    maintenance: str
    force: bool
    run_hmac: str
    supervisor_hmac: str
    master_hmac: str
    source_hmac: str
    config_hmac: str
    membership_hmac: str


def parse_horizon_inspection(raw, *, selected_version):
    def reject():
        raise ValueError('startup_horizon_inspection')

    def pairs(items):
        value = {}
        for key, item in items:
            if key in value:
                reject()
            value[key] = item
        return value

    try:
        if (type(raw) is not bytes or not 0 < len(raw) <= 8192
                or type(selected_version) is not str or selected_version not in ('4.4.2', '4.4.6')):
            reject()
        value = json.loads(raw.decode('utf-8', 'strict'), object_pairs_hook=pairs,
                           parse_constant=lambda _: reject())
        if type(value) is not dict or set(value) != set(HorizonInspection.__dataclass_fields__):
            reject()
        for field, expected in dict(version=1, operation='horizon-inspect', selected_version=selected_version,
                phase='complete', maintenance='held', force=False, terminating_count=0).items():
            if type(value[field]) is not type(expected) or value[field] != expected:
                reject()
        for field in ('master_status', 'supervisor_status'):
            if type(value[field]) is not str or value[field] not in ('running', 'paused'):
                reject()
        for field in ('pool_count', 'worker_object_count', 'live_worker_count'):
            if type(value[field]) is not int or not 1 <= value[field] <= (32 if field == 'pool_count' else 64):
                reject()
        if not value['pool_count'] <= value['live_worker_count'] == value['worker_object_count']:
            reject()
        for field in (field for field in value if field.endswith('_hmac')):
            if type(value[field]) is not str or re.fullmatch('[a-f0-9]{64}', value[field]) is None:
                reject()
        return HorizonInspection(**value)
    except (ValueError, UnicodeError, TypeError, RecursionError, KeyError):
        raise ValueError('startup_horizon_inspection') from None


@dataclass(frozen=True)
class HorizonDrain:
    version: int
    operation: str
    selected_version: str
    phase: str
    maintenance: str
    force: bool
    restoration: str
    pool_count: int
    worker_object_count: int
    exited_worker_count: int
    live_worker_count: int
    terminating_count: int
    master_status: str
    supervisor_status: str
    run_hmac: str
    original_hmac: str
    membership_hmac: str
    source_hmac: str
    config_hmac: str


def parse_horizon_drain(raw, *, selected_version):
    def reject():
        raise ValueError('startup_horizon_drain')

    def pairs(items):
        value = {}
        for key, item in items:
            if key in value:
                reject()
            value[key] = item
        return value

    try:
        if (type(raw) is not bytes or not 0 < len(raw) <= 8192
                or type(selected_version) is not str or selected_version not in ('4.4.2', '4.4.6')):
            reject()
        value = json.loads(raw.decode('utf-8', 'strict'), object_pairs_hook=pairs,
                           parse_constant=lambda _: reject())
        if type(value) is not dict or set(value) != set(HorizonDrain.__dataclass_fields__):
            reject()
        for field, expected in dict(version=1, operation='horizon-drain', selected_version=selected_version,
                phase='quiescent', maintenance='held', force=False, restoration='required',
                live_worker_count=0, terminating_count=0, master_status='paused', supervisor_status='paused').items():
            if type(value[field]) is not type(expected) or value[field] != expected:
                reject()
        for field in ('pool_count', 'worker_object_count', 'exited_worker_count'):
            if type(value[field]) is not int or not 1 <= value[field] <= (32 if field == 'pool_count' else 64):
                reject()
        if not value['pool_count'] <= value['exited_worker_count'] == value['worker_object_count']:
            reject()
        for field in (field for field in value if field.endswith('_hmac')):
            if type(value[field]) is not str or re.fullmatch('[a-f0-9]{64}', value[field]) is None:
                reject()
        return HorizonDrain(**value)
    except (ValueError, UnicodeError, TypeError, RecursionError, KeyError):
        raise ValueError('startup_horizon_drain') from None
