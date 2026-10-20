"""Run the real scan wrapper and verifier with synthetic scanner responses."""
import json
import os
from pathlib import Path
import shutil
import subprocess
import tempfile
import unittest

import test_report as evidence


class ScanOutcomeTests(unittest.TestCase):
    def setUp(self):
        self.fixture = evidence.ImageEvidenceTests()
        self.fixture.setUp()
        self.addCleanup(self.fixture.doCleanups)
        self.temp = tempfile.TemporaryDirectory()
        self.addCleanup(self.temp.cleanup)
        self.directory = Path(self.temp.name)
        self.output = self.directory / '.image-security/api'
        self.output.mkdir(parents=True)
        for source in self.fixture.directory.iterdir():
            shutil.copyfile(source, self.output / source.name)

    def run_scan(self, *, findings=True, sbom_exit=0, vuln_exit=None, missing=False, wrong_image=False):
        report = self.fixture.report
        if not findings:
            report['Results'][0]['Vulnerabilities'] = []
        if wrong_image:
            report['Metadata']['ImageID'] = 'sha256:' + '0' * 64
        (self.fixture.directory / 'vulnerabilities.json').write_text(json.dumps(report))
        env = dict(os.environ, FIXTURE=str(self.fixture.directory), OUTPUT=str(self.output),
                   REAL_REPORT=str(Path(__file__).with_name('report.py')),
                   SBOM_EXIT=str(sbom_exit), VULN_EXIT=str(int(findings) if vuln_exit is None else vuln_exit),
                   WRITE_REPORT='0' if missing else '1')
        shell = '''
docker() {
  case "$*" in
    *"trivy image"*"cyclonedx"*)
      cp "$FIXTURE/sbom.cdx.json" "$OUTPUT/sbom.cdx.json"
      return "$SBOM_EXIT" ;;
    *"trivy image"*)
      if [ "$WRITE_REPORT" = 1 ]; then cp "$FIXTURE/vulnerabilities.json" "$OUTPUT/vulnerabilities.json"; fi
      return "$VULN_EXIT" ;;
    *"verifier python"*) python -B "$REAL_REPORT" verify ;;
    *) return 97 ;;
  esac
}
scan_script=$1
set -- api
. "$scan_script"
'''
        # The real verifier normally uses /reports inside Compose. Redirect
        # that one mount path without replacing any validation function.
        verifier = Path(env['REAL_REPORT']).read_text().replace('Path("/reports")', 'Path(os.environ["OUTPUT"])')
        verifier_path = self.directory / 'report.py'
        verifier_path.write_text(verifier)
        env['REAL_REPORT'] = str(verifier_path)
        return subprocess.run(['sh', '-c', shell, 'test-scan', str(Path(__file__).with_name('scan.sh'))],
                              cwd=self.directory, env=env, text=True, capture_output=True)

    def test_complete_findings_are_reported_without_a_processing_failure(self):
        result = self.run_scan()
        self.assertEqual(result.returncode, 0, result.stdout + result.stderr)
        status = json.loads((self.output / 'scan-status.json').read_text())
        self.assertEqual(status, {'sbom_exit_code': 0, 'vulnerability_exit_code': 1})
        self.assertEqual(json.loads((self.output / 'metadata.json').read_text())['vulnerability_counts']['LOW'], 1)
        self.assertIn('::warning', result.stdout)

    def test_complete_clean_scan_succeeds_without_a_vulnerability_warning(self):
        result = self.run_scan(findings=False)
        self.assertEqual(result.returncode, 0, result.stdout + result.stderr)
        self.assertNotIn('::warning', result.stdout)

    def test_scanner_failure_is_rejected_even_with_a_report(self):
        result = self.run_scan(vuln_exit=2)
        self.assertNotEqual(result.returncode, 0)

    def test_sbom_failure_is_rejected_even_with_a_report(self):
        result = self.run_scan(sbom_exit=1)
        self.assertNotEqual(result.returncode, 0)

    def test_missing_report_is_not_a_successful_scan(self):
        result = self.run_scan(missing=True)
        self.assertNotEqual(result.returncode, 0)

    def test_foreign_image_report_is_not_a_successful_scan(self):
        result = self.run_scan(wrong_image=True)
        self.assertNotEqual(result.returncode, 0)

    def test_clean_exit_with_findings_is_rejected(self):
        result = self.run_scan(vuln_exit=0)
        self.assertNotEqual(result.returncode, 0)


if __name__ == '__main__':
    unittest.main()
