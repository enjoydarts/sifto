"""Offline regression checks for the trust stores shipped in each Python image."""

import hashlib
import importlib.util
import os
from pathlib import Path
import re
import ssl
import subprocess
import tempfile
import unittest


FIXTURES = Path(__file__).resolve().parents[1] / "tls-fixtures" / "emudhra"
VERIFY_TIME = "1792627200"  # 2026-10-22 00:00:00 UTC; independent of test date.
CAFILE = "/etc/ssl/certs/ca-certificates.crt"
CAPATH = "/etc/ssl/certs"
BAD_ROOTS = {
    "mozilla/emSign_ECC_Root_CA_-_G3.crt":
        "86a1ecba089c4a8d3bbe2734c612ba341d813e043cf9e8a862cd5c57a36bbe6b",
    "mozilla/Comodo_AAA_Services_root.crt":
        "d7a7a0fb5d7e2731d771e9484ebcdef71d5f0c3e0a2948782bc83ee0ea699ef4",
}


class CATrustTests(unittest.TestCase):
    def verify(self, certificate, trust_args, intermediates=None, hostname=None):
        command = ["openssl", "verify", "-attime", VERIFY_TIME, *trust_args]
        if intermediates:
            command += ["-untrusted", str(intermediates), "-purpose", "sslserver"]
        if hostname:
            command += ["-verify_hostname", hostname]
        environment = os.environ.copy()
        environment.pop("SSL_CERT_FILE", None)
        environment.pop("SSL_CERT_DIR", None)
        return subprocess.run(command + [str(certificate)], env=environment,
                              capture_output=True, text=True, check=False)

    def assert_chains_rejected(self, trust_args):
        fixtures = sorted(FIXTURES.glob("*.pem"))
        self.assertEqual(len(fixtures), 12, "all twelve public chain fixtures are required")
        with tempfile.TemporaryDirectory() as directory:
            leaf, intermediates = Path(directory) / "leaf.pem", Path(directory) / "chain.pem"
            for fixture in fixtures:
                certificates = re.findall(
                    r"-----BEGIN CERTIFICATE-----.*?-----END CERTIFICATE-----",
                    fixture.read_text(), re.DOTALL)
                self.assertGreaterEqual(len(certificates), 2)
                leaf.write_text(certificates[0] + "\n")
                intermediates.write_text("\n".join(certificates[1:]) + "\n")
                with self.subTest(chain=fixture.name):
                    result = self.verify(leaf, trust_args, intermediates, fixture.stem)
                    self.assertNotEqual(result.returncode, 0,
                                        f"distrusted chain accepted: {fixture.name}")
                    self.assertRegex(result.stderr, r"unable to get (local )?issuer certificate|self-signed certificate")

    def test_openssl_default_trust_rejects_all_emudhra_chains(self):
        self.assert_chains_rejected([])

    def test_openssl_explicit_cafile_rejects_all_emudhra_chains(self):
        self.assert_chains_rejected(["-CAfile", CAFILE, "-no-CApath", "-no-CAstore"])

    def test_openssl_explicit_capath_rejects_all_emudhra_chains(self):
        self.assert_chains_rejected(["-CApath", CAPATH, "-no-CAfile", "-no-CAstore"])

    def assert_context_excludes_bad_roots(self, context):
        fingerprints = {hashlib.sha256(cert).hexdigest()
                        for cert in context.get_ca_certs(binary_form=True)}
        self.assertTrue(fingerprints)
        self.assertTrue(fingerprints.isdisjoint(BAD_ROOTS.values()),
                        "Python context still trusts a distrusted root")

    def test_python_default_context_excludes_bad_roots(self):
        self.assert_context_excludes_bad_roots(ssl.create_default_context())

    def test_certifi_bundle_when_installed(self):
        if importlib.util.find_spec("certifi") is None:
            self.skipTest("certifi is not installed in this image")
        import certifi
        self.assert_chains_rejected(["-CAfile", certifi.where(), "-no-CApath", "-no-CAstore"])

    def test_httpcore_effective_context_when_installed(self):
        if importlib.util.find_spec("httpcore") is None:
            self.skipTest("httpcore is not installed in this image")
        import httpcore
        context = httpcore.default_ssl_context()
        self.assert_context_excludes_bad_roots(context)
        with tempfile.TemporaryDirectory() as directory:
            bundle = Path(directory) / "httpcore.pem"
            bundle.write_text("".join(ssl.DER_cert_to_PEM_cert(cert)
                                      for cert in context.get_ca_certs(binary_form=True)))
            self.assert_chains_rejected(["-CAfile", str(bundle), "-no-CApath", "-no-CAstore"])

    def test_botocore_actual_client_trust_when_installed(self):
        if importlib.util.find_spec("botocore") is None:
            self.skipTest("botocore is not installed in this image")
        import botocore.session
        from botocore.httpsession import get_cert_path
        client = botocore.session.get_session().create_client(
            "s3", region_name="us-east-1", aws_access_key_id="dummy",
            aws_secret_access_key="dummy")
        try:
            selected = client._endpoint.http_session._verify
            if "AWS_CA_BUNDLE" in os.environ:
                self.assertEqual(selected, CAFILE, "botocore must select the hardened OS bundle")
            else:
                self.assertIs(selected, True, "botocore TLS verification must remain enabled")
            bundle = get_cert_path(selected)
            self.assert_chains_rejected(["-CAfile", bundle, "-no-CApath", "-no-CAstore"])
        finally:
            client.close()

    def test_unrelated_public_roots_remain_trusted(self):
        for root in ("ISRG_Root_X1", "GTS_Root_R1", "DigiCert_Global_Root_G2"):
            certificate = Path("/usr/share/ca-certificates/mozilla") / f"{root}.crt"
            for trust_args in ([], ["-CAfile", CAFILE, "-no-CApath", "-no-CAstore"],
                               ["-CApath", CAPATH, "-no-CAfile", "-no-CAstore"]):
                with self.subTest(root=root, trust_args=trust_args):
                    result = self.verify(certificate, trust_args)
                    self.assertEqual(result.returncode, 0, result.stderr)


if __name__ == "__main__":
    unittest.main()
