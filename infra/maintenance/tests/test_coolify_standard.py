import copy
import hashlib
import unittest
import tempfile
from pathlib import Path
from unittest.mock import Mock, patch

from maintenance import plan as p
import test_coolify_inventory

try:
    from maintenance import coolify_standard as s
except ImportError:
    s = None


class StandardComposeProjectTests(unittest.TestCase):
    def setUp(self):
        # Selected installation: official /data/coolify/source Compose metadata.
        self.runtime = {'containers': [
            {'id': str(index) * 64, 'image': image, 'status': 'running', 'health': 'healthy',
             'recreation_identity': {'name': name, 'compose': {'project': 'source', 'service': role}}}
            for index, (role, name, image) in enumerate([
                ('coolify', '/coolify', 'docker.io/coollabsio/coolify:4.4.2'),
                ('postgres', '/coolify-db', 'postgres:15-alpine'),
                ('redis', '/coolify-redis', 'redis:7-alpine')], 1)]}

    def test_official_source_directory_selects_all_three_roles(self):
        try:
            selected = s.roles(self.runtime)
        except p.MaintenanceError as exc:
            self.fail(f'official source-directory Compose roles rejected: {exc}')
        self.assertEqual(selected, dict(zip(['coolify', 'postgres', 'redis'], self.runtime['containers'])))

    def test_other_or_missing_projects_are_rejected(self):
        for project in ('coolify', 'other', '', None):
            changed = copy.deepcopy(self.runtime)
            for item in changed['containers']:
                item['recreation_identity']['compose']['project'] = project
            with self.subTest(project=project), self.assertRaises(p.MaintenanceError):
                s.roles(changed)

    def test_wrong_or_missing_services_are_rejected_for_each_role(self):
        for index in range(3):
            for service in ('other', '', None):
                changed = copy.deepcopy(self.runtime)
                changed['containers'][index]['recreation_identity']['compose']['service'] = service
                with self.subTest(index=index, service=service), self.assertRaises(p.MaintenanceError):
                    s.roles(changed)

    def test_mixed_project_roles_are_rejected(self):
        for index in range(3):
            changed = copy.deepcopy(self.runtime)
            changed['containers'][index]['recreation_identity']['compose']['project'] = 'coolify'
            with self.subTest(index=index), self.assertRaises(p.MaintenanceError):
                s.roles(changed)

    def test_name_image_health_status_and_unique_role_guards_remain_strict(self):
        for index in range(3):
            for fault in ('name', 'image', 'health', 'status', 'missing', 'duplicate', 'shared_id'):
                changed = copy.deepcopy(self.runtime)
                item = changed['containers'][index]
                if fault == 'name':
                    item['recreation_identity']['name'] = '/other'
                elif fault == 'missing':
                    changed['containers'].pop(index)
                elif fault == 'duplicate':
                    changed['containers'].append(copy.deepcopy(item))
                elif fault == 'shared_id':
                    item['id'] = changed['containers'][(index + 1) % 3]['id']
                else:
                    item[fault] = {'image': 'private.invalid/other:1', 'health': 'unhealthy', 'status': 'exited'}[fault]
                with self.subTest(index=index, fault=fault), self.assertRaises(p.MaintenanceError):
                    s.roles(changed)


class StandardUpdateTests(unittest.TestCase):
    def setUp(self):
        self.assertIsNotNone(s, 'standard update implementation missing')
        self.inventory = test_coolify_inventory.CoolifyRuntimeInventoryTests().inventory()
        app = copy.deepcopy(self.inventory['runtime']['containers'][0])
        app.update(id='9' * 64, image='example/app:1')
        app['recreation_identity']['name'] = '/app'
        self.inventory['runtime']['containers'].insert(0, app)
        for item, role in zip(self.inventory['runtime']['containers'][1:], ['coolify', 'postgres', 'redis']):
            item['recreation_identity']['compose'] = {'project': 'source', 'service': role}
        self.inventory['maintenance_policy']['coolify_standard_enabled'] = True
        self.inventory['maintenance_policy']['backup_ready'] = True
        self.observation = {'status': 'update_available', 'candidate_review': 'eligible_patch_review',
            'current_version': '4.4.2', 'latest_version': '4.4.6', 'release_published_at': 1,
            'release_url': 'https://github.com/coollabsio/coolify/releases/tag/v4.4.6'}
        self.script = b'#!/bin/bash\necho official\n'
        self.action = {'from_version': '4.4.2', 'to_version': '4.4.6', 'release_commit': 'b' * 40,
            'upgrade_sha256': hashlib.sha256(self.script).hexdigest(), 'helper_version': '1.0.17',
            'registry': 'docker.io', 'replace_roles': ['coolify', 'postgres', 'redis'], 'release_published_at': 1}

    def candidate(self):
        with patch.object(s, 'release_commit', return_value='b' * 40), patch.object(s, 'fetch_script', return_value=self.script):
            return s.build_candidate(self.inventory, self.observation, 'a' * 40, now=700000)

    def test_candidate_binds_fixed_source_standard_argv_and_no_apt_authority(self):
        plan = self.candidate()
        self.assertEqual(plan['coolify_update'], self.action)
        self.assertEqual(plan['action'], 'coolify_update')
        self.assertEqual(plan['packages'], [])
        self.assertFalse(plan['allow_reboot'])
        self.assertEqual(s.upgrade_argv('/private/upgrade.sh', self.action),
                         ['/bin/bash', '/private/upgrade.sh', '4.4.6', '1.0.17', 'docker.io'])

    def test_disabled_waiting_cross_series_alias_and_ambiguous_deny_before_fetch(self):
        for key, value in [('candidate_review', 'waiting'), ('latest_version', '4.5.0'),
                           ('latest_version', 'latest'), ('release_published_at', 699999)]:
            observed = dict(self.observation, **{key: value})
            with self.subTest(key=key), patch.object(s, 'fetch_script') as fetch, self.assertRaises(p.MaintenanceError):
                s.build_candidate(self.inventory, observed, 'a' * 40, now=700000)
            fetch.assert_not_called()
        del self.inventory['maintenance_policy']['coolify_standard_enabled']
        with self.assertRaises(p.MaintenanceError):
            self.candidate()

    def test_signed_action_rejects_extra_url_script_mixed_apt_and_helper_override(self):
        plan = self.candidate()
        for change in [{'url': 'https://evil.invalid'}, {'helper_version': 'latest'}, {'registry': 'ghcr.io'},
                       {'to_version': '4.5.0'}, {'release_commit': 'main'}]:
            changed = copy.deepcopy(plan)
            changed['coolify_update'].update(change)
            with self.subTest(change=change), self.assertRaises(p.MaintenanceError):
                p.validate_plan(changed)
        for key, value in [('allow_reboot', True), ('allow_service_restarts', True),
                           ('packages', [{'name': 'openssl', 'installed': '1', 'target': '2'}])]:
            with self.subTest(key=key), self.assertRaises(p.MaintenanceError):
                p.validate_plan(dict(plan, **{key: value}))

    def test_script_fetch_has_fixed_url_bounded_bytes_and_no_redirect(self):
        response = Mock()
        response.__enter__ = Mock(return_value=response)
        response.__exit__ = Mock(return_value=False)
        response.geturl.return_value = 'https://raw.githubusercontent.com/coollabsio/coolify/' + 'b' * 40 + '/scripts/upgrade.sh'
        response.read.return_value = self.script
        with patch.object(s.urllib.request, 'build_opener') as opener:
            opener.return_value.open.return_value = response
            self.assertEqual(s.fetch_script('b' * 40), self.script)
            response.read.return_value = b'x' * (s.MAX_SCRIPT + 1)
            with self.assertRaises(p.MaintenanceError):
                s.fetch_script('b' * 40)
            response.read.return_value = self.script
            response.geturl.return_value = 'https://evil.invalid'
            with self.assertRaises(p.MaintenanceError):
                s.fetch_script('b' * 40)

    def test_only_signed_control_plane_replacements_allowed_apps_remain_strict(self):
        after = copy.deepcopy(self.inventory)
        for item in after['runtime']['containers'][1:]:
            item['id'] = 'f' + item['id'][1:]
        after['runtime']['containers'][1]['image'] = 'docker.io/coollabsio/coolify:4.4.6'
        s.verify_containers(self.inventory['runtime'], after['runtime'], self.action)
        for key, value in [('id', 'e' * 64), ('image_id', 'sha256:' + 'e' * 64)]:
            changed = copy.deepcopy(after)
            changed['runtime']['containers'][0][key] = value
            with self.assertRaises(p.MaintenanceError):
                s.verify_containers(self.inventory['runtime'], changed['runtime'], self.action)
        after['runtime']['containers'][1]['health'] = 'none'
        with self.assertRaises(p.MaintenanceError):
            s.verify_containers(self.inventory['runtime'], after['runtime'], self.action)

    def test_daily_fallback_keeps_apt_and_unresolved_history_priority(self):
        from maintenance import cli
        self.assertTrue(hasattr(cli, 'coolify_fallback'), 'daily Coolify fallback missing')
        for status in ('eligible', 'blocked', 'unknown'):
            result = {'status': status, 'plan': {'fixture': 'apt'} if status == 'eligible' else None}
            with patch.object(s, 'build_candidate') as candidate:
                self.assertEqual(cli.coolify_fallback(result, self.inventory, self.observation, 'a' * 40), result)
            candidate.assert_not_called()
        with patch.object(s, 'build_candidate', return_value={'action': 'coolify_update'}):
            result = cli.coolify_fallback({'status': 'current', 'plan': None}, self.inventory, self.observation, 'a' * 40)
        self.assertEqual(result['plan']['action'], 'coolify_update')
        self.assertEqual(result['status'], 'eligible')

    def test_post_apply_recognizes_signed_roles_and_keeps_apps_strict(self):
        from maintenance.post_apply import validate_observation
        plan = self.candidate()
        approval = {'approved_at': 700000, 'expires_at': 701000}
        after = copy.deepcopy(self.inventory)
        after['collected_at'] = 700005
        for item in after['runtime']['containers'][1:]:
            item['id'] = 'f' + item['id'][1:]
        after['runtime']['containers'][1]['image'] = 'docker.io/coollabsio/coolify:4.4.6'
        history = {'host_id': plan['host_id'], 'receipt_total': 1, 'receipt_truncated': 0,
            'receipt_history_complete': True, 'unresolved_execution_count': 0,
            'receipts': [{'plan': plan, 'approval': approval, 'status': 'verified', 'started_at': 700001,
                          'completed_at': 700002, 'verified_at': 700003, 'before_runtime': self.inventory['runtime']}]}
        host = {'host_id': plan['host_id'], 'role': 'application'}
        result = validate_observation(plan, {'approval': approval}, host, after, history)
        self.assertEqual(result['recreated_container_count'], 3)
        after['runtime']['containers'][0]['image_id'] = 'sha256:' + 'f' * 64
        with self.assertRaises(p.MaintenanceError):
            validate_observation(plan, {'approval': approval}, host, after, history)

    def test_bootstrap_installs_and_validates_new_root_source_disabled_by_default(self):
        import json
        source = Path(__file__).parents[1]
        for path in ('host_agent.py', 'bootstrap/install-host.sh', 'bootstrap/Dockerfile.test', 'bootstrap/Dockerfile.test.dockerignore'):
            self.assertIn('coolify_standard.py', (source / path).read_text())
        self.assertEqual(json.loads((source / 'bootstrap/config.example.json').read_text())['coolify_standard'], {'enabled': False})

    def test_signed_plan_binds_seven_day_wait_and_root_probe_fails_unknown(self):
        plan = self.candidate()
        self.assertEqual(plan['coolify_update']['release_published_at'], 1)
        plan['coolify_update']['release_published_at'] = plan['created_at']
        with self.assertRaises(p.MaintenanceError):
            p.validate_plan(plan)
        runtime = self.inventory['runtime']
        config = '{"version":"4.4.2","autoupdate":false}'
        runner = Mock(side_effect=["'version' => env('COOLIFY_VERSION') ?: '4.4.2',", config, '[0,0,0,0,0]', '{}'])
        s.probe(runner, runtime, '4.4.2')
        self.assertIn('BEGIN READ ONLY', runner.call_args_list[2].args[0][-1])
        for counts in ('[1,0,0,0,0]', '[0,0,0,1,0]', '[0,0,0,0,1]', '[false,0,0,0,0]', '[]', 'unknown'):
            runner = Mock(side_effect=["'version' => env('COOLIFY_VERSION') ?: '4.4.2',", config, counts])
            with self.subTest(counts=counts), self.assertRaises(p.MaintenanceError):
                s.probe(runner, runtime, '4.4.2')

    def test_effective_autoupdate_enabled_unknown_config_or_overridden_version_denies(self):
        for config in ('{"version":"4.4.2","autoupdate":true}', '{"version":"4.4.2","autoupdate":"false"}',
                       '{"version":"9.0.0","autoupdate":false}', '{}'):
            runner = Mock(side_effect=["'version' => env('COOLIFY_VERSION') ?: '4.4.2',", config, '[0,0,0,0,0]', '{}'])
            with self.subTest(config=config), self.assertRaises(p.MaintenanceError):
                s.probe(runner, self.inventory['runtime'], '4.4.2')

    def test_approval_report_discloses_target_mutable_dependencies_and_backup_split(self):
        from maintenance import report
        text = report.render({'status': 'eligible', 'reason': 'coolify_patch_human_approval_required', 'plan': self.candidate()}, {'status': 'unavailable'})
        for expected in ('4.4.2 → 4.4.6', '1.0.17', 'docker.io', 'CDN', '24時間', 'ローカル', '全 artifact'):
            self.assertIn(expected, text)


class StandardHostTests(StandardUpdateTests):
    def setUp(self):
        super().setUp()
        from cryptography.hazmat.primitives.asymmetric.ed25519 import Ed25519PrivateKey
        from maintenance.host import HostAgent
        self.tmp = tempfile.TemporaryDirectory()
        self.addCleanup(self.tmp.cleanup)
        self.key = Ed25519PrivateKey.generate()
        self.config = {'host_id': self.inventory['host_id'], 'trusted_public_key': self.key.public_key(),
            'approved_workflow_sha': 'a' * 40, 'state_path': self.tmp.name,
            'allowed_approvers': ['human'], 'allowed_restart_services': [], 'database_enabled': False,
            'host_role': 'application', 'health_wait_seconds': 0, 'coolify_standard': {'enabled': True, 'wait_seconds': 0}}
        self.runner = Mock(return_value='')
        self.agent = HostAgent(self.config, self.runner, clock=lambda: 700000, lock_probe=lambda: [])
        self.agent.inventory = Mock(side_effect=lambda **kwargs: copy.deepcopy(self.inventory))
        self.agent._backup_ready = Mock(return_value=True)
        self.agent._coolify_preflight = Mock(return_value=None)
        self.agent._coolify_backup = Mock(return_value={'local_snapshot': 'fixture'})

    def request(self, dry_run=False):
        plan = self.candidate()
        approval = {'plan_sha256': p.plan_sha256(plan), 'run_id': '123', 'approved_by': 'human',
            'approved_at': 700000, 'expires_at': 701000, 'workflow_sha': 'a' * 40,
            'execution_mode': 'dry_run' if dry_run else 'apply'}
        return {'workflow_sha': 'a' * 40, 'envelope': p.sign_plan(plan, approval, self.key), 'dry_run': dry_run}

    def test_dry_run_does_not_fetch_backup_consume_or_execute(self):
        request = self.request(True)
        with self.agent.locked():
            pass
        with patch.object(s, 'fetch_script') as fetch:
            self.assertEqual(self.agent.run('apply', request)['status'], 'preflight_passed')
        fetch.assert_not_called()
        self.agent._coolify_backup.assert_not_called()
        self.runner.assert_not_called()
        self.assertEqual(list((Path(self.tmp.name) / 'plans').glob('*.json')), [])

    def test_disabled_or_missing_offhost_proof_refuses_even_dry_run(self):
        request = self.request(True)
        with self.agent.locked():
            pass
        self.config['coolify_standard']['enabled'] = False
        with self.assertRaises(p.MaintenanceError):
            self.agent.run('apply', request)
        self.config['coolify_standard']['enabled'] = True
        self.agent._backup_ready.return_value = False
        with self.assertRaises(p.MaintenanceError):
            self.agent.run('apply', request)

    def test_backup_failure_is_consumed_and_never_retries_script(self):
        request = self.request()
        def failed_backup():
            record = self.agent._record_path(request['envelope']['plan']['plan_id'])
            self.assertEqual(__import__('json').loads(record.read_text())['status'], 'started')
            raise p.MaintenanceError('backup failed')
        self.agent._coolify_backup.side_effect = failed_backup
        with patch.object(s, 'fetch_script', return_value=self.script), self.assertRaises(p.MaintenanceError):
            self.agent.run('apply', request)
        record = __import__('json').loads(self.agent._record_path(request['envelope']['plan']['plan_id']).read_text())
        self.assertEqual(record['status'], 'failed')
        with self.assertRaisesRegex(p.MaintenanceError, 'consumed'):
            self.agent.run('apply', request)
        self.runner.assert_not_called()

    def test_exit_zero_without_target_observation_stays_failed_and_consumed(self):
        request = self.request()
        with patch.object(s, 'fetch_script', return_value=self.script), self.assertRaises(p.MaintenanceError):
            self.agent.run('apply', request)
        self.assertEqual(__import__('json').loads(self.agent._record_path(request['envelope']['plan']['plan_id']).read_text())['status'], 'failed')
        self.assertEqual(self.runner.call_args.args[0][-3:], ['4.4.6', '1.0.17', 'docker.io'])

    def test_success_is_verified_from_target_and_consumes_exactly_once(self):
        request = self.request()
        def upgraded(*args, **kwargs):
            self.inventory['runtime']['containers'][1]['image'] = 'docker.io/coollabsio/coolify:4.4.6'
            for item in self.inventory['runtime']['containers'][1:]:
                item['id'] = 'f' + item['id'][1:]
            return ''
        self.runner.side_effect = upgraded
        with patch.object(s, 'fetch_script', return_value=self.script):
            result = self.agent.run('apply', request)
        self.assertEqual(result['status'], 'completed')
        self.assertIsNone(self.runner.call_args.kwargs['timeout'])
        self.assertEqual(self.agent.run('verify', {'plan_id': result['plan_id']})['status'], 'verified')
        record = __import__('json').loads(self.agent._record_path(result['plan_id']).read_text())
        self.assertEqual(record['local_backup'], {'local_snapshot': 'fixture'})
        self.assertIn('offhost_backup_proof', record)
        with self.assertRaisesRegex(p.MaintenanceError, 'consumed'):
            self.agent.run('apply', request)

    def test_backup_time_drift_expiry_hash_change_and_apt_lock_never_start_upgrade(self):
        for fault in ('state', 'expiry', 'hash', 'apt'):
            with self.subTest(fault=fault):
                request = self.request()
                if fault == 'apt':
                    self.agent.lock_probe = lambda: ['dpkg']
                elif fault == 'state':
                    self.agent._coolify_backup.side_effect = lambda: self.inventory['runtime'].update(boot_id='changed')
                elif fault == 'expiry':
                    self.agent._coolify_backup.side_effect = lambda: setattr(self.agent, 'clock', lambda: 702000)
                with patch.object(s, 'fetch_script', return_value=b'changed' if fault == 'hash' else self.script), self.assertRaises(p.MaintenanceError):
                    self.agent.run('apply', request)
                self.runner.assert_not_called()
                self.setUp()

    def test_real_preflight_refuses_upgrade_status_process_and_held_backup_lock(self):
        import fcntl
        from maintenance.host import HostAgent
        source = Path(self.tmp.name) / 'source'
        source.mkdir()
        for name in ('backup.lock', 'coolify-backup.lock'):
            (Path(self.tmp.name) / name).touch(mode=0o600)
        with patch('maintenance.host.Path', return_value=source), patch.object(s, 'probe'):
            (source / '.upgrade-status').write_text('6|complete|unproven')
            with self.assertRaises(p.MaintenanceError):
                HostAgent._coolify_preflight(self.agent, self.inventory, '4.4.2')
            (source / '.upgrade-status').unlink()
            self.runner.return_value = '/bin/bash /data/coolify/source/upgrade.sh 4.4.6\n'
            with self.assertRaises(p.MaintenanceError):
                HostAgent._coolify_preflight(self.agent, self.inventory, '4.4.2')
            self.runner.return_value = 'python host_agent.py\n'
            with (Path(self.tmp.name) / 'backup.lock').open('r+') as stream:
                fcntl.flock(stream, fcntl.LOCK_EX | fcntl.LOCK_NB)
                with self.assertRaises(p.MaintenanceError):
                    HostAgent._coolify_preflight(self.agent, self.inventory, '4.4.2')
            HostAgent._coolify_preflight(self.agent, self.inventory, '4.4.2')

    def test_target_success_with_unapproved_boot_change_stays_failed(self):
        request = self.request()
        def upgraded_with_reboot(*args, **kwargs):
            self.inventory['runtime']['containers'][1]['image'] = 'docker.io/coollabsio/coolify:4.4.6'
            self.inventory['runtime']['boot_id'] = 'unexpected-reboot'
            return ''
        self.runner.side_effect = upgraded_with_reboot
        with patch.object(s, 'fetch_script', return_value=self.script), self.assertRaisesRegex(p.MaintenanceError, 'boot'):
            self.agent.run('apply', request)
        record = __import__('json').loads(self.agent._record_path(request['envelope']['plan']['plan_id']).read_text())
        self.assertEqual(record['status'], 'failed')

    def test_verify_rejects_boot_change_after_successful_apply(self):
        request = self.request()
        def upgraded(*args, **kwargs):
            self.inventory['runtime']['containers'][1]['image'] = 'docker.io/coollabsio/coolify:4.4.6'
            return ''
        self.runner.side_effect = upgraded
        with patch.object(s, 'fetch_script', return_value=self.script):
            result = self.agent.run('apply', request)
        self.inventory['runtime']['boot_id'] = 'unexpected-reboot'
        with self.assertRaisesRegex(p.MaintenanceError, 'boot'):
            self.agent.run('verify', {'plan_id': result['plan_id']})

    def test_root_and_readonly_verification_reject_later_full_package_drift(self):
        from maintenance.post_apply import validate_observation
        request = self.request()
        def upgraded(*args, **kwargs):
            self.inventory['runtime']['containers'][1]['image'] = 'docker.io/coollabsio/coolify:4.4.6'
            return ''
        self.runner.side_effect = upgraded
        with patch.object(s, 'fetch_script', return_value=self.script):
            result = self.agent.run('apply', request)
        self.assertEqual(self.agent.run('verify', {'plan_id': result['plan_id']})['status'], 'verified')
        self.inventory['collected_at'] = 700000
        record = __import__('json').loads(self.agent._record_path(result['plan_id']).read_text())
        history = {'host_id': self.inventory['host_id'], 'receipt_total': 1, 'receipt_truncated': 0,
            'receipt_history_complete': True, 'unresolved_execution_count': 0, 'receipts': [record]}
        host = {'host_id': self.inventory['host_id'], 'role': 'application'}
        self.assertEqual(validate_observation(record['plan'], request['envelope'], host, self.inventory, history)['status'], 'verified')
        self.inventory['packages'][0]['version'] = '9.9.9'
        with self.subTest(boundary='root'), self.assertRaisesRegex(p.MaintenanceError, 'scope'):
            self.agent.run('verify', {'plan_id': result['plan_id']})
        with self.subTest(boundary='read_only'), self.assertRaisesRegex(p.MaintenanceError, 'scope'):
            validate_observation(record['plan'], request['envelope'], host, self.inventory, history)
