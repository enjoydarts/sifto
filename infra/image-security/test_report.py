"""Archive identity and incomplete scan evidence must fail closed."""
import hashlib
import io
import json
from pathlib import Path
import tarfile
import tempfile
import unittest

from report import prepare, verify, write_json


class ImageEvidenceTests(unittest.TestCase):
    def setUp(self):
        self.temp = tempfile.TemporaryDirectory()
        self.addCleanup(self.temp.cleanup)
        self.directory = Path(self.temp.name)
        self.sha = "a" * 40
        config = json.dumps({"os": "linux", "architecture": "arm64", "config": {
            "Labels": {"org.opencontainers.image.revision": self.sha}}}).encode()
        self.image_id = "sha256:" + hashlib.sha256(config).hexdigest()
        with tarfile.open(self.directory / "image.tar", "w") as archive:
            for name, data in (("config.json", config), ("manifest.json", b'[{"Config":"config.json"}]')):
                member = tarfile.TarInfo(name)
                member.size = len(data)
                archive.addfile(member, io.BytesIO(data))
        for name, content in {"source-sha.txt": self.sha, "source-dirty.txt": "false",
                              "platform.txt": "linux/arm64", "dockerfile.txt": "api/Dockerfile",
                              "image-id.txt": self.image_id}.items():
            (self.directory / name).write_text(content)
        prepare(self.directory, "api")
        write_json(self.directory / "sbom.cdx.json", {"bomFormat": "CycloneDX", "components": [
            {"name": "ca-certificates"}, {"name": "github.com/jackc/pgx/v5"}],
            "metadata": {"component": {"properties": [
                {"name": "aquasecurity:trivy:ImageID", "value": self.image_id}]}}})
        self.report = {"Metadata": {"ImageID": self.image_id}, "Results": [
            {"Class": "os-pkgs", "Packages": [{"Name": "ca-certificates"}],
             "Vulnerabilities": [{"Severity": "LOW"}]},
            {"Class": "lang-pkgs", "Packages": [{"Name": "github.com/jackc/pgx/v5"}]}]}
        write_json(self.directory / "vulnerabilities.json", self.report)
        write_json(self.directory / "scan-status.json", {"sbom_exit_code": 0, "vulnerability_exit_code": 1})

    def test_findings_are_retained_with_nonzero_scan_status(self):
        counts = verify(self.directory, "api")
        self.assertEqual(counts["LOW"], 1)
        metadata = json.loads((self.directory / "metadata.json").read_text())
        self.assertEqual(metadata["image_id"], self.image_id)
        self.assertIn("sbom.cdx.json", metadata["report_sha256"])

    def summary_with_findings(self, findings):
        self.report["Results"][0]["Vulnerabilities"] = findings
        write_json(self.directory / "vulnerabilities.json", self.report)
        write_json(self.directory / "scan-status.json", {
            "sbom_exit_code": 0, "vulnerability_exit_code": 1 if findings else 0})
        counts = verify(self.directory, "api")
        return (self.directory / "summary.md").read_text(), counts

    def test_summary_lists_update_versions_and_unfixed_findings(self):
        summary, counts = self.summary_with_findings([
            {"Severity": "HIGH", "VulnerabilityID": "CVE-2026-1111", "PkgName": "zlib",
             "InstalledVersion": "1.3.2-r0", "FixedVersion": "1.3.2-r1"},
            {"Severity": "CRITICAL", "VulnerabilityID": "CVE-2026-2222", "PkgName": "libc6",
             "InstalledVersion": "2.41", "FixedVersion": " \t "},
        ])
        self.assertIn("| Severity | CVE | Package | Installed | Fixed | Action |", summary)
        self.assertIn("| HIGH | CVE-2026-1111 | zlib | 1.3.2-r0 | 1.3.2-r1 | Update available |", summary)
        self.assertIn("| CRITICAL | CVE-2026-2222 | libc6 | 2.41 | — | No fix advertised |", summary)
        self.assertIn("scanner advertises a fixed version", summary)
        self.assertIn("distro/vendor advisory and runtime reachability", summary)
        self.assertLess(summary.index("| CRITICAL |"), summary.index("| HIGH |"))
        self.assertEqual(counts["CRITICAL"], 1)
        self.assertEqual(counts["HIGH"], 1)

    def test_summary_marks_missing_fixed_version_as_unfixed(self):
        summary, _ = self.summary_with_findings([
            {"Severity": "LOW", "VulnerabilityID": "CVE-2026-1111", "PkgName": "zlib",
             "InstalledVersion": "1.3.2-r0"},
        ])
        self.assertIn("| LOW | CVE-2026-1111 | zlib | 1.3.2-r0 | — | No fix advertised |", summary)

    def test_summary_escapes_untrusted_markdown_and_html_fields(self):
        unsafe = "evil|<script>\n[click](javascript:x)`*_\\"
        safe = r"evil\|&lt;script&gt; \[click\]\(javascript:x\)\`\*\_\\"
        summary, _ = self.summary_with_findings([
            {"Severity": "HIGH", "VulnerabilityID": unsafe, "PkgName": unsafe,
             "InstalledVersion": unsafe, "FixedVersion": unsafe},
        ])
        self.assertIn(f"| HIGH | {safe} | {safe} | {safe} | {safe} | Update available |", summary)
        self.assertNotIn("<script>", summary)
        self.assertNotIn("[click](javascript:x)", summary)

    def test_summary_prioritizes_severity_and_limits_rows_with_full_artifact_reference(self):
        findings = [
            {"Severity": "LOW", "VulnerabilityID": f"CVE-2026-{number:04}", "PkgName": "zlib",
             "InstalledVersion": "1", "FixedVersion": "2"}
            for number in range(31, 0, -1)
        ]
        findings.extend([
            {"Severity": severity, "VulnerabilityID": f"CVE-2026-{number}", "PkgName": "zlib",
             "InstalledVersion": "1", "FixedVersion": "2"}
            for severity, number in (("MEDIUM", "9001"), ("HIGH", "9002"), ("CRITICAL", "9003"))
        ])
        summary, counts = self.summary_with_findings(findings)
        rows = [line for line in summary.splitlines() if line.startswith((
            "| CRITICAL |", "| HIGH |", "| MEDIUM |", "| LOW |", "| UNKNOWN |"))]
        self.assertEqual(len(rows), 30)
        self.assertIn("| CRITICAL | CVE-2026-9003 |", rows[0])
        self.assertIn("| HIGH | CVE-2026-9002 |", rows[1])
        self.assertIn("| MEDIUM | CVE-2026-9001 |", rows[2])
        self.assertIn("| LOW | CVE-2026-0001 |", rows[3])
        self.assertIn("| LOW | CVE-2026-0027 |", rows[-1])
        self.assertIn("4 additional findings", summary)
        self.assertIn("`vulnerabilities.json`", summary)
        self.assertIn(f"attached artifact matching `image-security-api-{self.sha}-*`", summary)
        self.assertEqual(sum(counts.values()), 34)

    def test_summary_states_when_no_vulnerabilities_were_found(self):
        summary, counts = self.summary_with_findings([])
        self.assertIn("No vulnerabilities found in the scanned OS and library packages.", summary)
        self.assertNotIn("| Severity |", summary)
        self.assertEqual(sum(counts.values()), 0)

    def test_summary_separates_completed_scan_from_unresolved_findings(self):
        summary, counts = self.summary_with_findings([
            {'Severity': 'HIGH', 'VulnerabilityID': 'CVE-2026-1111', 'PkgName': 'zlib',
             'InstalledVersion': '1', 'FixedVersion': '2'},
            {'Severity': 'CRITICAL', 'VulnerabilityID': 'CVE-2026-2222', 'PkgName': 'libc6',
             'InstalledVersion': '1', 'FixedVersion': ' \t '},
        ])
        self.assertIn('検査処理: 完了', summary)
        self.assertIn('未解決の脆弱性: 2件', summary)
        self.assertIn('修正版の提示あり: 1件', summary)
        self.assertIn('修正版の提示なし: 1件', summary)
        self.assertNotIn('findings fail this advisory job', summary)
        metadata = json.loads((self.directory / 'metadata.json').read_text())
        self.assertIs(metadata['findings_fail_job'], False)
        self.assertEqual(metadata['vulnerability_counts'], counts)

    def test_wrong_scan_image_is_rejected(self):
        self.report["Metadata"]["ImageID"] = "sha256:" + "0" * 64
        write_json(self.directory / "vulnerabilities.json", self.report)
        with self.assertRaisesRegex(ValueError, "different image"):
            verify(self.directory, "api")

    def test_wrong_sbom_image_is_rejected(self):
        sbom = json.loads((self.directory / "sbom.cdx.json").read_text())
        sbom["metadata"]["component"]["properties"][0]["value"] = "sha256:" + "0" * 64
        write_json(self.directory / "sbom.cdx.json", sbom)
        with self.assertRaisesRegex(ValueError, "SBOM refers"):
            verify(self.directory, "api")

    def test_changed_metadata_source_commit_is_rejected(self):
        metadata = json.loads((self.directory / "metadata.json").read_text())
        metadata["source_commit"] = "b" * 40
        write_json(self.directory / "metadata.json", metadata)
        with self.assertRaisesRegex(ValueError, "source commit differs"):
            verify(self.directory, "api")

    def test_os_only_scan_cannot_claim_library_coverage(self):
        self.report["Results"].pop()
        write_json(self.directory / "vulnerabilities.json", self.report)
        with self.assertRaisesRegex(ValueError, "Missing known installed"):
            verify(self.directory, "api")

    def test_wrong_production_platform_is_rejected(self):
        (self.directory / "platform.txt").write_text("linux/amd64")
        with self.assertRaisesRegex(ValueError, "production platform"):
            prepare(self.directory, "api")

    def test_mutated_archive_is_rejected(self):
        with (self.directory / "image.tar").open("ab") as stream:
            stream.write(b"changed")
        with self.assertRaisesRegex(ValueError, "Archive changed"):
            verify(self.directory, "api")

    def test_clean_exit_with_low_severity_finding_is_rejected(self):
        write_json(self.directory / "scan-status.json", {"sbom_exit_code": 0, "vulnerability_exit_code": 0})
        with self.assertRaisesRegex(ValueError, "exit status"):
            verify(self.directory, "api")


if __name__ == "__main__":
    unittest.main()
