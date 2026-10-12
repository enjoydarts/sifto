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
