"""Public output must exclude private selected metadata and raw Docker config."""
import copy
import json
import unittest

from maintenance import normalize_recreation_identity
from test_container_identity import raw_identity
from test_controller import ready


class CoolifyRuntimeInventoryTests(unittest.TestCase):
    def inventory(self):
        inventory = ready()
        for index, (name, image) in enumerate([
                ('coolify', 'docker.io/coollabsio/coolify:4.4.2'),
                ('coolify-db', 'postgres:15-alpine'), ('coolify-redis', 'redis:7-alpine')], 1):
            identity = raw_identity()
            identity['name'] = '/' + name
            inventory['runtime']['containers'].append({
                'id': str(index) * 64, 'image_id': 'sha256:' + str(index + 3) * 64,
                'image': image, 'status': 'running', 'health': 'healthy',
                'recreation_identity': normalize_recreation_identity(identity, str(index) * 64),
                'Env': ['FIXTURE_SECRET=fixture-not-for-output'],
                'Labels': {'fixture-secret-label': 'fixture-not-for-output'}})
        return inventory

    def summary(self, inventory):
        from maintenance import coolify_inventory
        return coolify_inventory.summarize_runtime(inventory)

    def test_collects_only_hashes_from_existing_selected_inventory_without_reading_secrets(self):
        inventory = self.inventory()
        original = copy.deepcopy(inventory)
        result = self.summary(inventory)
        self.assertEqual(result['status'], 'observed')
        self.assertEqual(set(result['containers']), {'coolify', 'postgres', 'redis'})
        self.assertEqual(result['current_version'], '4.4.2')
        self.assertRegex(result['instance_sha256'], r'^[0-9a-f]{64}$')
        for container in result['containers'].values():
            self.assertEqual(set(container), {'identity_sha256', 'image_config_digest', 'health'})
        encoded = json.dumps(result)
        for private in ['fixture-not-for-output', inventory['host_id'], '/srv/fixture',
                        'fixture-alias', '192.0.2.10', '1' * 64]:
            self.assertNotIn(private, encoded)
        self.assertIn('platform_manifest_digest', result['unverified'])
        self.assertIn('full_restore', result['unverified'])
        self.assertEqual(inventory, original)

    def test_identity_changes_are_bound_and_health_changes_are_observations(self):
        inventory = self.inventory()
        baseline = self.summary(inventory)
        for path, value in [(['host_id'], 'fixture-other-host'),
                            (['runtime', 'containers', 0, 'image_id'], 'sha256:' + 'f' * 64),
                            (['runtime', 'containers', 0, 'recreation_identity', 'name'], '/other-instance'),
                            (['runtime', 'containers', 1, 'recreation_identity', 'mounts', 0, 'source'], '/srv/fixture/other')]:
            changed = copy.deepcopy(inventory)
            node = changed
            for key in path[:-1]:
                node = node[key]
            node[path[-1]] = value
            with self.subTest(path=path):
                result = self.summary(changed)
                self.assertNotEqual(result.get('instance_sha256'), baseline['instance_sha256'])
        inventory['runtime']['containers'][0]['health'] = 'unhealthy'
        result = self.summary(inventory)
        self.assertEqual(result['status'], 'observed')
        self.assertEqual(result['instance_sha256'], baseline['instance_sha256'])
        self.assertEqual(result['containers']['coolify']['health'], 'unhealthy')

    def test_missing_duplicate_and_untrusted_control_plane_are_unknown(self):
        for change in ['duplicate', 'missing', 'private_image', 'missing_identity', 'alias', 'bad_id']:
            inventory = self.inventory()
            if change == 'duplicate':
                inventory['runtime']['containers'].append(copy.deepcopy(inventory['runtime']['containers'][0]))
            elif change == 'missing':
                inventory['runtime']['containers'].pop()
            elif change == 'private_image':
                inventory['runtime']['containers'][1]['image'] = 'private.invalid/secret:1'
            elif change == 'missing_identity':
                del inventory['runtime']['containers'][0]['recreation_identity']
            elif change == 'alias':
                inventory['runtime']['containers'][0]['image'] = 'coollabsio/coolify:latest'
            else:
                inventory['runtime']['containers'][0]['id'] = 'invalid'
            with self.subTest(change=change):
                self.assertEqual(self.summary(inventory), {'schema_version': 1, 'status': 'unknown',
                    'reason': 'selected_runtime_unavailable'})


if __name__ == '__main__':
    unittest.main()
