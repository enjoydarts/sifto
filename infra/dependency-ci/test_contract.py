"""Guard the privilege and dependency policy of public dependency automation."""
import json
from pathlib import Path
import re
import unittest

import yaml


ROOT = Path('/repository')


def read_yaml(path):
    # YAML 1.1 parses the GitHub Actions key `on` as True.
    return yaml.safe_load((ROOT / path).read_text())


class DependencyAutomationContract(unittest.TestCase):
    def test_renovate_cannot_merge_before_native_owner_approval(self):
        config = json.loads((ROOT / 'renovate.json').read_text())
        self.assertIs(config['automerge'], False)
        self.assertIs(config['platformAutomerge'], False)
        self.assertEqual(config['minimumReleaseAge'], '7 days')
        self.assertEqual(config['rebaseWhen'], 'behind-base-branch')
        self.assertLessEqual(config['prHourlyLimit'], 2)
        self.assertLessEqual(config['prConcurrentLimit'], 5)
        for rule in config.get('packageRules', []):
            self.assertIsNot(rule.get('automerge'), True)
        self.assertTrue(config['pinDigests'])
        self.assertTrue({'dockerfile', 'docker-compose', 'gomod', 'pip-compile', 'npm', 'custom.regex'}.issubset(config['enabledManagers']))
        self.assertNotIn('pip_requirements', config['enabledManagers'])
        self.assertNotIn('github-actions', config['enabledManagers'])

    def test_renovate_pr_text_points_to_native_approval_and_automatic_merge(self):
        config = json.loads((ROOT / 'renovate.json').read_text())
        self.assertIn('production-maintenance', config.get('prFooter', ''))
        self.assertIn('merges and deploys', config.get('prFooter', ''))
        self.assertNotIn('{{{configDescription}}}', config.get('prBodyTemplate', '{{{configDescription}}}'))

    def test_live_executor_dependencies_have_an_exclusive_activation_group(self):
        rules = json.loads((ROOT / 'renovate.json').read_text())['packageRules']
        rule = rules[-1]
        self.assertEqual(set(rule.get('matchFileNames', [])), {'infra/maintenance/Dockerfile', 'infra/maintenance/requirements.in', 'infra/maintenance/requirements.txt'})
        self.assertEqual(rule.get('groupName'), 'Maintenance executor activation dependencies')
        self.assertEqual(rule.get('groupSlug'), 'maintenance-executor-activation')
        self.assertIs(rule.get('automerge'), False)

    def test_air_version_is_extractable_and_llm_catalog_is_not_managed(self):
        config = json.loads((ROOT / 'renovate.json').read_text())
        managers = config['customManagers']
        self.assertEqual(len(managers), 1)
        manager = managers[0]
        self.assertEqual(manager['depNameTemplate'], 'github.com/air-verse/air')
        regex = manager['matchStrings'][0].replace('(?<currentValue>', '(?P<currentValue>')
        match = re.search(regex, (ROOT / 'api/Dockerfile').read_text())
        self.assertIsNotNone(match)
        self.assertRegex(match.group('currentValue'), r'^v\d+\.\d+\.\d+$')
        self.assertIn('shared/llm_catalog.json', config['ignorePaths'])

    def test_bot_uses_only_ephemeral_token_and_main(self):
        workflow = read_yaml('.github/workflows/renovate.yml')
        events = workflow.get('on', workflow.get(True))
        self.assertEqual(set(events), {'schedule', 'workflow_dispatch'})
        job = workflow['jobs']['renovate']
        self.assertIn("github.ref == 'refs/heads/main'", job['if'])
        action = next(s for s in job['steps'] if s.get('uses', '').startswith('renovatebot/'))
        self.assertEqual(action['with']['token'], '${{ github.token }}')
        self.assertRegex(action['with']['renovate-version'], r'^\d+\.\d+\.\d+$')
        self.assertNotIn('secrets.', (ROOT / '.github/workflows/renovate.yml').read_text())

    def test_bot_can_publish_release_age_status_without_broader_permissions(self):
        workflow = read_yaml('.github/workflows/renovate.yml')
        self.assertEqual(workflow['permissions'], {})
        self.assertEqual(workflow['jobs']['renovate']['permissions'], {
            'contents': 'write',
            'pull-requests': 'write',
            # Renovate publishes pending/success for minimumReleaseAge before PR creation.
            'statuses': 'write',
            'actions': 'write',
        })

    def test_public_pr_checks_have_no_write_permissions_or_secrets(self):
        path = '.github/workflows/dependency-check.yml'
        workflow = read_yaml(path)
        events = workflow.get('on', workflow.get(True))
        self.assertIn('pull_request', events)
        self.assertNotIn('pull_request_target', events)
        self.assertEqual(workflow['permissions'], {'contents': 'read'})
        self.assertNotIn('secrets.', (ROOT / path).read_text())
        for job in workflow['jobs'].values():
            self.assertNotIn('environment', job)
            self.assertNotIn('permissions', job)
        text = (ROOT / path).read_text()
        for target in ['dependency-api', 'dependency-worker', 'dependency-web', 'maintenance-test', 'maintenance-syntax', 'dependency-contract', 'dependency-renovate-validate', 'dependency-python-locks-check', 'dependency-audio']:
            self.assertIn(target, text)

    def test_new_actions_are_immutable_and_checkout_drops_credentials(self):
        for name in ['renovate', 'dependency-check', 'dependency-security', 'dependency-update', 'deploy', 'deploy-audio-concat']:
            workflow = read_yaml(f'.github/workflows/{name}.yml')
            for job in workflow['jobs'].values():
                for step in job['steps']:
                    if 'uses' not in step:
                        continue
                    self.assertRegex(step['uses'], r'@[a-f0-9]{40}$')
                    if step['uses'].startswith('actions/checkout@'):
                        self.assertIs(step['with']['persist-credentials'], False)

    def test_approval_pipeline_uses_trusted_main_and_one_environment_gate(self):
        workflow = read_yaml('.github/workflows/dependency-update.yml')
        self.assertEqual(set(workflow.get('on', workflow.get(True))), {'workflow_dispatch'})
        self.assertEqual(workflow['permissions'], {})
        gates = [job.get('environment') for job in workflow['jobs'].values() if 'environment' in job]
        self.assertEqual(gates, ['production-maintenance'])
        for job in workflow['jobs'].values():
            self.assertIn("github.ref == 'refs/heads/main'", job['if'])
            checkout = next(step for step in job['steps'] if step.get('uses', '').startswith('actions/checkout@'))
            self.assertEqual(checkout['with']['ref'], '${{ github.sha }}')
        execution = workflow['jobs']['approve-merge-deploy']
        self.assertEqual(execution['needs'], 'prepare')
        self.assertIn('dependency_updates.pipeline execute', str(execution))
        self.assertIn('dependency_updates.pipeline discover', (ROOT / '.github/workflows/renovate.yml').read_text())

    def test_workflow_waits_and_approval_job_cover_the_actual_job_dag(self):
        from dependency_updates import pipeline

        def longest_path(workflow):
            jobs = read_yaml('.github/workflows/' + workflow)['jobs']
            elapsed = {}

            def duration(name):
                if name not in elapsed:
                    job = jobs[name]
                    timeout = job.get('timeout-minutes')
                    self.assertIs(type(timeout), int, workflow + '/' + name + ' needs an explicit finite timeout')
                    self.assertGreater(timeout, 0)
                    self.assertLessEqual(timeout, 360)
                    needs = job.get('needs', [])
                    if isinstance(needs, str):
                        needs = [needs]
                    elapsed[name] = timeout + max((duration(parent) for parent in needs), default=0)
                return elapsed[name]

            return max(duration(name) for name in jobs)

        budgets = pipeline.RUN_WAIT_SECONDS
        self.assertEqual(set(budgets), {'dependency-check.yml', 'deploy.yml', 'deploy-audio-concat.yml'})
        for workflow, seconds in budgets.items():
            self.assertIs(type(seconds), int)
            self.assertGreater(seconds, 0)
            self.assertLess(seconds, 360 * 60)
            # Allow queue/setup time beyond native job execution, including all
            # sequential dependencies. Web and Inngest sync run in parallel.
            queue_minutes = 10 if workflow == 'dependency-check.yml' else 20
            self.assertGreaterEqual(seconds, (longest_path(workflow) + queue_minutes) * 60)
        approval = read_yaml('.github/workflows/dependency-update.yml')['jobs']
        self.assertGreaterEqual(approval['prepare']['timeout-minutes'] * 60, budgets['dependency-check.yml'] + 10 * 60)
        execution_timeout = approval['approve-merge-deploy']['timeout-minutes']
        self.assertLessEqual(execution_timeout, 360)
        self.assertGreaterEqual(execution_timeout * 60, budgets['deploy.yml'] + budgets['deploy-audio-concat.yml'] + 10 * 60)

    def test_bot_merge_has_an_explicit_exact_commit_deployment_entrypoint(self):
        workflow = read_yaml('.github/workflows/deploy.yml')
        events = workflow.get('on', workflow.get(True))
        self.assertIn('push', events)
        self.assertIn('workflow_dispatch', events)
        self.assertIn('expected_commit', events['workflow_dispatch']['inputs'])
        self.assertEqual(workflow['concurrency']['group'], 'production-deploy')

    def test_audio_dispatch_proves_the_exact_deployed_image(self):
        workflow = read_yaml('.github/workflows/deploy-audio-concat.yml')
        inputs = workflow.get('on', workflow.get(True))['workflow_dispatch']['inputs']
        self.assertIn('expected_commit', inputs)
        self.assertEqual(workflow['concurrency']['group'], 'production-deploy')
        text = (ROOT / '.github/workflows/deploy-audio-concat.yml').read_text()
        self.assertIn('dependency_updates.deploy validate', text)
        self.assertIn('gcloud run jobs describe', text)
        self.assertIn('Actual Audio Concat image differs', text)

    def test_ci_compose_has_no_host_ports_or_real_credentials(self):
        config = read_yaml('infra/dependency-ci/compose.yml')
        self.assertEqual(config['name'], 'sifto-dependency-ci')
        for service in config['services'].values():
            for field in ['ports', 'env_file', 'privileged']:
                self.assertNotIn(field, service)
            for volume in service.get('volumes', []):
                self.assertNotIn('docker.sock', volume)
                self.assertNotIn('.env', volume)
        api = config['services']['api']
        self.assertEqual(set(api['depends_on']), {'postgres', 'redis'})
        self.assertEqual(config['services']['postgres']['environment']['POSTGRES_PASSWORD'], 'dependency-ci-dummy')
        self.assertIn('@postgres:5432/dependency_ci?', api['environment']['DATABASE_URL'])
        self.assertEqual(config['services']['postgres']['tmpfs'], ['/var/lib/postgresql/data'])
        self.assertIn('--env-file /dev/null', (ROOT / 'Makefile').read_text())

    def test_security_scans_only_public_manifests_and_reports_failures(self):
        workflow = read_yaml('.github/workflows/dependency-security.yml')
        events = workflow.get('on', workflow.get(True))
        self.assertEqual(set(events), {'schedule', 'push', 'workflow_dispatch'})
        self.assertEqual(workflow['permissions'], {'contents': 'read'})
        scan = workflow['jobs']['inventory']
        self.assertIn("github.ref == 'refs/heads/main'", scan['if'])
        self.assertNotIn('continue-on-error', scan)
        action = next(s for s in scan['steps'] if s.get('uses', '').startswith('aquasecurity/trivy-action@'))
        self.assertEqual(action['with']['scanners'], 'vuln')
        self.assertEqual(action['with']['exit-code'], '1')
        self.assertEqual(action['with']['version'], 'v0.75.0')
        self.assertEqual(action['with']['scan-ref'], '.dependency-inventory')
        self.assertNotIn('continue-on-error', action)

    def test_sensitive_changes_require_owner_review(self):
        owners = (ROOT / '.github/CODEOWNERS').read_text().splitlines()
        entries = {line.split()[0]: line.split()[1:] for line in owners if line and not line.startswith('#')}
        for pattern in ['/.github/**', '/infra/**', '**/Dockerfile*', '**/requirements*.txt', '**/requirements*.in', '**/package*.json', '**/go.mod', '**/go.sum', '/Makefile', '/renovate.json']:
            self.assertIn('@enjoydarts', entries[pattern])

    def test_worker_direct_requirements_are_fixed_and_install_is_bounded(self):
        requirements = (ROOT / 'worker/requirements.in').read_text().splitlines()
        for line in requirements:
            if line and not line.startswith('#'):
                self.assertRegex(line, r'^[a-zA-Z0-9_.-]+(?:\[[a-zA-Z0-9_,.-]+\])?==[a-zA-Z0-9_.-]+$')
        self.assertTrue(any(line.startswith('yt-dlp==') for line in requirements))
        self.assertTrue(any(line.startswith('bgutil-ytdlp-pot-provider==') for line in requirements))
        self.assertNotIn('pip install --no-cache-dir -U', (ROOT / 'worker/Dockerfile').read_text())

    def test_all_python_installations_require_full_hashed_locks(self):
        from dependency_updates.policy import python_lock, UV_HEADER
        for directory in ['worker', 'infra/maintenance', 'infra/audio-concat']:
            pins = python_lock((ROOT / directory / 'requirements.txt').read_text())
            self.assertGreater(len(pins), 1)
            self.assertIn('pip install --no-cache-dir --require-hashes -r requirements.txt', (ROOT / directory / 'Dockerfile').read_text())
            for line in (ROOT / directory / 'requirements.in').read_text().splitlines():
                if not line or line.startswith('#'): continue
                name, version = line.split('==')
                name = re.sub(r'[-_.]+', '-', name.split('[')[0]).lower()
                self.assertEqual(pins[name]['version'], version)
        config = json.loads((ROOT / 'renovate.json').read_text())
        self.assertFalse(config['pip_requirements']['enabled'])
        self.assertEqual(config['constraints']['uv'], '0.13.0')
        patterns = config['pip-compile']['managerFilePatterns']
        for path in ['worker/requirements.txt', 'infra/maintenance/requirements.txt', 'infra/audio-concat/requirements.txt']:
            self.assertTrue(any(re.search(pattern[1:-1], path) for pattern in patterns))
        self.assertIn('--python-version=3.12', UV_HEADER)
        compose = read_yaml('infra/dependency-ci/compose.python-locks.yml')
        locks = compose['services']['locks']
        self.assertRegex(locks['image'], r'uv:0\.13\.0-python3\.12-.*@sha256:[0-9a-f]{64}$')
        self.assertIn('../..:/repository:ro', locks['volumes'])
        self.assertTrue(locks['read_only'])


if __name__ == '__main__':
    unittest.main()
