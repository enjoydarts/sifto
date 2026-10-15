"""Fixed PUBLIC Task4A startup candidate; no execution registration or authority.

The unchanged ARM images have been inspected independently. These exact guard
resources suppress app:init, automatic configuration and the webhook maintenance
exception while retaining stock entrypoint/S6. Functional fixture success is
not native source-owner, drain, SSH reconstruction or production proof. Task4B/C
must complete before OwnedDocker may accept any ControlledStartup record.

Source resources are installed as root-owned 0644 inputs. Future authenticated
staging must make the init-script leaf root-owned 0755 before mounting it RO;
this catalog never installs, stages, reads private input or executes anything.
"""
import copy

from .coolify_plan import OLD_ARM, NEW_ARM, profile_sha256


RESOURCE_SHA256 = (
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
