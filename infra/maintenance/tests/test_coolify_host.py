"""Selected read-only Coolify metadata; synthetic fixtures only."""
import copy
import json
import os
import tempfile
import unittest
from pathlib import Path
from unittest.mock import patch

from maintenance import host, ssh, coolify_plan
from maintenance.plan import MaintenanceError
import test_coolify_inventory


class SelectedReportTests(unittest.TestCase):
    def setUp(self):
        from maintenance import coolify_host
        self.module = coolify_host
        self.config = {'host_role': 'application', 'host_id': 'fixture-app',
                       'state_path': '/var/lib/sifto-maintenance'}

    def policy(self):
        return {'enabled': True, 'profile_id': coolify_plan.PROFILE_ID,
                'resources': self.module.resource_bounds()}

    def collector(self, changes=None):
        inventory = test_coolify_inventory.CoolifyRuntimeInventoryTests().inventory()
        containers = inventory['runtime']['containers']
        specs = coolify_plan.fixed_profile()['bundle']['rollback']['images']
        images = {}
        for role, item in zip(('coolify', 'postgres', 'redis'), containers):
            item.pop('Env'); item.pop('Labels')
            item['recreation_identity']['compose'] = {
                'project_present': False, 'project': '', 'service_present': False, 'service': ''}
            item['runtime_flags'] = {'entrypoint_present': True, 'command_present': False,
                                     'user_present': True, 'privileged': False}
            item['image'] = specs[role]['repository'] + '@' + specs[role]['platform_manifest']
            images[item['image_id']] = {'engine_id': item['image_id'], 'os': 'linux',
                'architecture': 'arm64', 'descriptor': {'digest': specs[role]['platform_manifest'],
                'mediaType': 'application/vnd.oci.image.manifest.v1+json'}}
        calls = []
        def runner(argv, **kwargs):
            calls.append((argv, kwargs))
            if argv == ['dpkg', '--print-architecture']:
                return 'arm64\n'
            if argv[:3] == ['docker', 'ps', '-aq']:
                return '\n'.join(item['id'] for item in containers)
            if argv[:2] == ['docker', 'inspect']:
                return '\n'.join(json.dumps(item) for item in containers)
            if argv[:3] == ['docker', 'image', 'inspect']:
                return json.dumps(images[argv[-1]])
            self.fail('unexpected command ' + repr(argv))
        if changes:
            changes(containers, images)
        return runner, calls

    def report(self, changes=None, *, now=None):
        runner, calls = self.collector(changes)
        config = {**self.config, 'coolify_patch_policy': self.policy()}
        with patch.object(self.module, 'source_observation', return_value={
                'status': 'unverified', 'reason': 'source_configuration_unverified'}), \
             patch.object(self.module, 'capacity_observation', return_value={'status': 'sufficient'}):
            value = self.module.collect_report(config, {}, runner=runner,
                now=coolify_plan.NOT_BEFORE if now is None else now)
        return value, calls

    def test_disabled_policy_does_not_probe_or_modify_config(self):
        original = copy.deepcopy(self.config)
        value = self.module.collect_report(self.config, {}, runner=lambda *a, **k: self.fail('disabled probe'))
        self.assertFalse(value['eligible'])
        self.assertEqual(value['status'], 'disabled')
        self.assertEqual(self.config, original)

    def test_policy_strict_profile_fields_types_and_resource_bounds(self):
        for change in [lambda p: p.update(enabled=1), lambda p: p.update(profile_id='future'),
                       lambda p: p.update(command='id'), lambda p: p['resources'].update(memory_bytes=True),
                       lambda p: p['resources'].update(memory_bytes=1),
                       lambda p: p['resources'].update(staging_path='/tmp/evil')]:
            policy = self.policy(); change(policy)
            with self.subTest(policy=policy), self.assertRaises(MaintenanceError):
                self.module.validate_policy(policy)

    def test_empty_gateway_request_only_and_scan_role_transport(self):
        self.assertEqual(host.gateway_mode('scan-gateway', 'coolify-report'), 'coolify-report')
        for mode in ['coolify-report --extra', 'coolify-apply', 'coolify-report; id']:
            with self.assertRaises(MaintenanceError):
                host.gateway_mode('scan-gateway', mode)
        with self.assertRaises(MaintenanceError):
            host.gateway_mode('apply-gateway', 'coolify-report')
        runner, _ = self.collector()
        for request in [{'ready': True}, {'proof': {}}, {'argv': ['id']}, []]:
            with self.assertRaises(MaintenanceError):
                self.module.collect_report(self.config, request, runner=runner)
        client = ssh.Client('/tmp', '/tmp/key', '/tmp/known', 'scan')
        fixture = {'host_id': 'fixture-app', 'role': 'application', 'port': 22, 'address': '192.0.2.1'}
        self.assertEqual(client.argv(fixture, 'coolify-report')[-1], 'coolify-report')

    def test_host_dispatch_does_not_enter_or_create_generic_maintenance_lock(self):
        agent = host.HostAgent(self.config)
        with patch.object(agent, 'locked', side_effect=AssertionError('report wrote lock')), \
             patch.object(self.module, 'collect_report', return_value={'eligible': False}) as collect:
            self.assertEqual(agent.run('coolify-report', {}), {'eligible': False})
        collect.assert_called_once()

    def test_digest_only_identity_has_opaque_engine_id_and_separate_arm_manifest(self):
        value, calls = self.report()
        self.assertEqual(value['observation']['status'], 'observed')
        self.assertEqual(value['observation']['current_version'], '4.4.2')
        selected = value['observation']['containers']['coolify']
        self.assertEqual(selected['platform_manifest'], coolify_plan.OLD_ARM)
        self.assertEqual(selected['engine_image_id'], 'sha256:' + '4' * 64)
        self.assertNotIn('image_config_digest', selected)
        self.assertFalse(value['eligible'])
        reasons = {reason['code'] for reason in value['reasons']}
        for code in ['full_restore_proof_unverified', 'backup_unverified',
                     'barrier_unverified', 'auth_verification_unavailable',
                     'startup_mutation_scope_unverified', 'runtime_recreation_unverified']:
            self.assertIn(code, reasons)
        for argv, kwargs in calls:
            self.assertGreater(kwargs['timeout'], 0)
            self.assertLessEqual(kwargs['timeout'], 30)
            self.assertNotIn('exec', argv)
            self.assertNotIn('compose', argv)
        serialized = json.dumps(value)
        for private in ['fixture-app', '/srv/fixture', 'fixture-alias', '192.0.2.10', '1' * 64]:
            self.assertNotIn(private, serialized)

    def test_before_age_gate_is_distinct_waiting_even_when_other_checks_block(self):
        value, _ = self.report(now=coolify_plan.NOT_BEFORE - 1)
        self.assertIn({'category': 'waiting', 'code': 'release_age_gate'}, value['reasons'])

    def test_duplicate_instance_wrong_platform_descriptor_and_noncanonical_repository_fail_closed(self):
        def wrong_descriptor(containers, images):
            images[containers[0]['image_id']]['descriptor']['digest'] = containers[0]['image_id']
        for change in [lambda c, i: c.append(copy.deepcopy(c[0])),
                       lambda c, i: i[c[0]['image_id']].update(architecture='amd64'),
                       wrong_descriptor,
                       lambda c, i: c[0].update(image='private.invalid/coolify@' + coolify_plan.OLD_ARM),
                       lambda c, i: c[0].update(image='ghcr.io/coollabsio/coolify@' + coolify_plan.OLD_ARM),
                       lambda c, i: c[0].update(Env=['SHOULD_NOT_SERIALIZE=secret-marker']),
                       lambda c, i: c[0].update(Labels={'secret-marker': 'private'}),
                       lambda c, i: c[0].update(id='id; dangerous')]:
            with self.subTest(change=change):
                value, _ = self.report(change)
                self.assertEqual(value['observation']['status'], 'unknown')
                self.assertFalse(value['eligible'])
                self.assertNotIn('secret-marker', json.dumps(value))

    def test_missing_descriptor_is_unverified_not_engine_id_promoted_to_manifest(self):
        value, _ = self.report(lambda c, i: i[c[0]['image_id']].update(descriptor=None))
        self.assertEqual(value['observation']['status'], 'unknown')
        self.assertIn({'category': 'unsupported', 'code': 'selected_runtime_unavailable'}, value['reasons'])

    def test_distinct_running_coolify_from_each_known_registry_is_ambiguous(self):
        for image in ['ghcr.io/coollabsio/coolify:4.4.2',
                      'docker.io/coollabsio/coolify:4.4.2', 'coollabsio/coolify:4.4.2']:
            def extra_instance(containers, images):
                extra = copy.deepcopy(containers[0])
                extra['id'] = 'a' * 64
                extra['recreation_identity']['name'] = '/other-coolify'
                extra['image'] = image
                containers.append(extra)
            with self.subTest(image=image):
                result, calls = self.report(extra_instance)
                self.assertEqual(result['observation']['status'], 'unknown')
                self.assertFalse(result['eligible'])
                self.assertIn({'category': 'unsupported', 'code': 'selected_runtime_unavailable'},
                              result['reasons'])
                self.assertFalse(any(argv[:3] == ['docker', 'image', 'inspect']
                                     for argv, _ in calls), 'ambiguous instance reached descriptor probes')

    def test_root_config_validates_opt_in_without_default_rewrite(self):
        self.assertTrue(hasattr(host, 'validate_coolify_policy'))
        host.validate_coolify_policy(self.config)
        self.assertNotIn('coolify_patch_policy', self.config)
        host.validate_coolify_policy({**self.config, 'coolify_patch_policy': self.policy()})
        with self.assertRaises(MaintenanceError):
            host.validate_coolify_policy({**self.config, 'coolify_patch_policy': {'enabled': True}})

    def test_selected_templates_exclude_env_arbitrary_labels_command_and_user_values(self):
        for forbidden in ['json .Config.Env', 'json .Config.Labels', 'json .Config.Cmd',
                          'json .Config.Entrypoint', 'json .Config.User', 'json .RepoDigests']:
            self.assertNotIn(forbidden, self.module.SELECTED_CONTAINER_FORMAT + self.module.SELECTED_IMAGE_FORMAT)

    def test_explicit_disabled_policy_and_database_role_do_not_probe(self):
        policy = self.policy(); policy['enabled'] = False
        config = {**self.config, 'coolify_patch_policy': policy}
        runner = lambda *a, **k: self.fail('disabled or database probe')
        self.assertEqual(self.module.collect_report(config, {}, runner=runner)['status'], 'disabled')
        policy['enabled'] = True; config['host_role'] = 'database'
        result = self.module.collect_report(config, {}, runner=runner)
        self.assertIn({'category': 'unsupported', 'code': 'unsupported_host_role'}, result['reasons'])

    def test_capacity_and_unknown_runtime_are_separate_blockers(self):
        runner, _ = self.collector()
        with patch.object(self.module, 'source_observation', return_value={
                'status': 'unsupported', 'reason': 'private_override_unsupported'}), \
             patch.object(self.module, 'capacity_observation', return_value={'status': 'insufficient'}):
            result = self.module.collect_report({**self.config, 'coolify_patch_policy': self.policy()},
                {}, runner=runner, now=coolify_plan.NOT_BEFORE + 365 * 86400)
        self.assertFalse(result['eligible'])
        for category, code in [('capacity', 'capacity_insufficient'),
                               ('unsupported', 'private_override_unsupported'),
                               ('unsupported', 'mount_policy_unverified'),
                               ('unsupported', 'network_policy_unverified'),
                               ('unsupported', 'entrypoint_policy_unverified')]:
            self.assertIn({'category': category, 'code': code}, result['reasons'])

    def test_public_source_hash_is_stable_across_reads_and_rejects_link_and_private_permissions(self):
        with tempfile.TemporaryDirectory(dir='/opt') as directory:
            path = Path(directory) / 'public-compose.yml'
            path.write_bytes(b'public fixture source')
            path.chmod(0o644)
            expected = __import__('hashlib').sha256(b'public fixture source').hexdigest()
            os.utime(path, (1, 1))
            self.assertEqual(self.module._protected_file(path), expected)
            self.assertEqual(self.module._protected_file(path), expected)
            link = Path(directory) / 'link'
            link.symlink_to(path)
            with self.assertRaises(MaintenanceError):
                self.module._protected_file(link)
            hardlink = Path(directory) / 'hardlink'; os.link(path, hardlink)
            with self.assertRaises(MaintenanceError):
                self.module._protected_file(path)
            hardlink.unlink(); path.chmod(0o666)
            with self.assertRaises(MaintenanceError):
                self.module._protected_file(path)

    def test_source_never_reads_private_override_or_env_and_reports_unknown_public_bytes(self):
        with tempfile.TemporaryDirectory(dir='/opt') as directory:
            root = Path(directory)
            (root / 'docker-compose.custom.yml').symlink_to('/never-read-private')
            with patch.object(self.module, '_SOURCE', root), \
                 patch.object(self.module, '_protected_file', side_effect=AssertionError('private read')):
                self.assertEqual(self.module.source_observation()['reason'], 'private_override_unsupported')
            (root / 'docker-compose.custom.yml').unlink()
            for name in ['docker-compose.yml', 'docker-compose.prod.yml']:
                (root / name).write_bytes(b'unsupported public source')
            (root / '.env').symlink_to('/never-read-secret')
            with patch.object(self.module, '_SOURCE', root):
                self.assertEqual(self.module.source_observation()['reason'], 'source_configuration_unsupported')

    def test_native_import_closure_and_bootstrap_cover_new_modules_before_import(self):
        root = Path(__file__).parents[1]
        entrypoint = (root / 'host_agent.py').read_text()
        installer = (root / 'bootstrap/install-host.sh').read_text()
        fixture_image = (root / 'bootstrap/Dockerfile.test').read_text()
        for name in ['coolify_host.py', 'coolify_plan.py', 'coolify_rehearsal.py',
                     'coolify_evidence.py', 'coolify_docker.py', 'coolify_rehearsal_engine.py',
                     'coolify_systemd.py', 'coolify_native_protocol.py', 'coolify_cleanup.py',
                     'coolify_native_owner.py', 'coolify-public-manifest.json', 'coolify_probe.php',
                     'coolify_startup.py', 'coolify_startup_init', 'coolify_startup_middleware.php',
                     'coolify_startup_nginx.conf', 'coolify_startup_site.conf']:
            self.assertIn('maintenance/' + name, entrypoint.split('try:\n    validate_installed_sources()')[0])
            self.assertIn(name, installer)
            self.assertIn('maintenance/' + name, fixture_image)


if __name__ == '__main__':
    unittest.main()
