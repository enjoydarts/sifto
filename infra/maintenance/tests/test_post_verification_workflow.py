import unittest
import json
from pathlib import Path
import yaml

from maintenance import post_apply, plan as plan_module
from test_post_apply import fixture


class PostVerificationWorkflowTests(unittest.TestCase):
    def test_bootstrap_monitoring_can_verify_approved_container_runtime_restarts(self):
        for filename in ('config.example.json', 'config.database.example.json'):
            with self.subTest(config=filename):
                config = json.loads(Path('/repository/infra/maintenance/bootstrap', filename).read_text())
                plan, envelope, host, after, history, *_ = fixture()
                plan['restart_services'] = ['containerd.service', 'docker.service']
                envelope['approval']['plan_sha256'] = plan_module.plan_sha256(plan)
                host['role'] = config['host_role']
                # Model a provisioned DB host; its bootstrap default requires explicit opt-in.
                after['maintenance_policy'].update(host_role=config['host_role'],
                                                  database_enabled=config['host_role'] == 'database', backup_ready=True)
                after['runtime']['services'] = {unit: 'active' for unit in config['monitored_services']}
                history['receipts'][0]['before_runtime']['services'] = dict(after['runtime']['services'])
                if config['host_role'] == 'database':
                    after['runtime']['database_pool_health'] = {'status': 'healthy', 'reason': 'read_only_route_verified'}
                result = post_apply.validate_observation(plan, envelope, host, after, history)
                self.assertEqual(result['status'], 'verified')

    def test_same_run_job_uses_pipeline_source_and_read_only_scope(self):
        workflow = yaml.safe_load(Path('/repository/.github/workflows/maintenance-apply.yml').read_text())
        self.assertIn('post-verification', workflow['jobs'], 'same-run verification job missing')
        job = workflow['jobs']['post-verification']
        self.assertEqual(job['needs'], ['approve', 'execute', 'audit'])
        self.assertIn('always()', job['if'])
        self.assertIn("needs.approve.result == 'success'", job['if'])
        self.assertIn("needs.execute.result == 'success'", job['if'])
        self.assertIn('!inputs.dry_run', job['if'])
        self.assertNotIn('needs.audit.result', job['if'])
        self.assertEqual(job['environment'], 'maintenance-scan')
        self.assertEqual(job['timeout-minutes'], 15)
        self.assertEqual(job['permissions'], {'contents': 'read', 'actions': 'read'})
        self.assertEqual(job['concurrency'], {'group': 'production-deploy', 'cancel-in-progress': False})
        checkout = next(step for step in job['steps'] if 'uses' in step)
        self.assertEqual(checkout['with']['ref'], '${{ github.sha }}')
        env = job['steps'][-1]['env']
        secrets = {key for key in env if 'secrets.' in str(env[key])}
        self.assertEqual(secrets, {'MAINTENANCE_SCAN_SSH_KEY', 'MAINTENANCE_OPS_WRITE_KEY', 'MAINTENANCE_HOSTS_JSON', 'MAINTENANCE_KNOWN_HOSTS'})
        self.assertEqual(job['steps'][-1]['run'], 'sh infra/maintenance/run-post-verification.sh')
