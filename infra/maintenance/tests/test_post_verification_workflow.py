import unittest
from pathlib import Path
import yaml


class PostVerificationWorkflowTests(unittest.TestCase):
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
