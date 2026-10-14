"""Hash summaries of existing private scan metadata; no commands or secrets.

Opaque Engine image IDs identify locally running images. They do not prove registry
platform-manifest digests or authorize an update.
"""
import re

from . import canonical_recreation_identity
from .plan import HASH, matches, plan_sha256, require

COOLIFY_IMAGE = re.compile(r'(?:ghcr\.io/|docker\.io/)?coollabsio/coolify:v?'
                          r'((?:0|[1-9][0-9]{0,5})\.(?:0|[1-9][0-9]{0,5})\.(?:0|[1-9][0-9]{0,5}))'
                          r'(?:@sha256:[0-9a-f]{64})?\Z')
DATABASE_IMAGES = {
    'postgres': ('/coolify-db', re.compile(r'(?:docker\.io/(?:library/)?)?postgres:'
                                          r'[A-Za-z0-9][A-Za-z0-9_.-]{0,127}(?:@sha256:[0-9a-f]{64})?\Z')),
    'redis': ('/coolify-redis', re.compile(r'(?:docker\.io/(?:library/)?)?redis:'
                                        r'[A-Za-z0-9][A-Za-z0-9_.-]{0,127}(?:@sha256:[0-9a-f]{64})?\Z')),
}


def summarize_runtime(inventory):
    """Return hashes from selected inventory, with unavailable readiness explicit.

    The private inventory is already collected using CONTAINER_FORMAT. Fields
    named Env or Labels are ignored, never read, serialized, or hashed here.
    """
    unknown = {'schema_version': 1, 'status': 'unknown', 'reason': 'selected_runtime_unavailable'}
    try:
        require(type(inventory) is dict and type(inventory['host_id']) is str
                and 0 < len(inventory['host_id']) <= 128 and type(inventory['platform']) is dict,
                'invalid selected runtime')
        platform = inventory['platform']
        require(platform == {'id': 'ubuntu', 'version': '24.04', 'architecture': 'arm64'},
                'unsupported selected runtime platform')
        containers = inventory['runtime']['containers']
        require(type(containers) is list and len(containers) <= 1000
                and all(type(item) is dict for item in containers), 'invalid selected containers')
        selected = [item for item in containers if item.get('status') == 'running'
                    and type(item.get('image')) is str
                    and re.match(r'(?:ghcr\.io/|docker\.io/)?coollabsio/coolify(?=[:@]|$)', item['image'])]
        require(len(selected) == 1, 'ambiguous selected control plane')
        main = selected[0]
        version = COOLIFY_IMAGE.fullmatch(main['image'])
        require(version is not None, 'fixed running version unavailable')
        roles = {'coolify': main}
        for role, (name, pattern) in DATABASE_IMAGES.items():
            candidates = [item for item in containers if item.get('status') == 'running'
                          and type(item.get('recreation_identity')) is dict
                          and item['recreation_identity'].get('name') == name]
            require(len(candidates) == 1 and type(candidates[0].get('image')) is str
                    and pattern.fullmatch(candidates[0]['image']) is not None,
                    'selected control-plane dependency unavailable')
            roles[role] = candidates[0]
        result, stable = {}, {}
        for role, container in roles.items():
            require(matches(HASH, container['id']) and type(container['image_id']) is str
                    and re.fullmatch(r'sha256:[0-9a-f]{64}', container['image_id']) is not None,
                    'invalid selected image identity')
            require(container.get('health') in ('healthy', 'unhealthy', 'starting', 'none'),
                    'invalid selected container health')
            identity = canonical_recreation_identity(container['recreation_identity'])
            stable[role] = {'id': container['id'], 'engine_image_id': container['image_id'],
                            'image': container['image'], 'recreation_identity': identity}
            result[role] = {'identity_sha256': plan_sha256(stable[role]),
                            'engine_image_id': container['image_id'], 'health': container['health']}
        require(len({container['id'] for container in roles.values()}) == 3,
                'ambiguous selected container identity')
        return {'schema_version': 1, 'status': 'observed', 'current_version': version.group(1),
                'instance_sha256': plan_sha256({'host_id': inventory['host_id'], 'platform': platform,
                                              'containers': stable}),
                'containers': result,
                'unverified': ['platform_manifest_digest', 'source_configuration', 'auto_update_disabled',
                               'mutations_idle', 'backup', 'full_restore']}
    except (ValueError, KeyError, TypeError, AttributeError):
        return unknown
