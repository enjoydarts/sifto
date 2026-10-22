# eMudhra certificate chain regression fixtures

Public certificate chains from <https://emudhra.we-have-a-problem.com/chains.zip>,
downloaded on 2026-10-22. SHA-256 of the source ZIP:
`8a2edc81b6c51e3ad27d45ca1e441ec84c961e2dee0fc547f2a02c0f74315f95`.

The selected PEM files contain public leaf and issuer certificates only, without
private keys. They cover the EM OV TLS CA - G3C key under both the emSign and
AAA Certificate Services issuer paths affected by CVE-2026-111943.

The test parses every certificate, trusts the last issuer explicitly, and checks
the leaf against the hostname from its filename at 2026-10-22T00:00:00Z. Trusting
the issuer makes the test independent of OS root stores, external services, and
future certificate expiry. Go 1.27.2 accepts these otherwise valid chains;
Go 1.27.3 rejects them through its compromised-CA SPKI blocklist, including when
the blocked key belongs to an explicitly trusted certificate.

A separately generated local CA and server certificate must still verify under
the same options. That control generates keys only in memory during the test.
