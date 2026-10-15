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
"""
import copy
from dataclasses import dataclass
import json
import re

from .coolify_plan import OLD_ARM, NEW_ARM, profile_sha256


RESOURCE_SHA256 = (
    ('coolify_startup.php', '2a563c60bddea003c4d230a0c1084910265617420559cbf6c993158ebc9bc356'),
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
