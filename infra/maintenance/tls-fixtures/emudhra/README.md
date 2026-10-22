# Public eMudhra distrust regression chains

These twelve public certificate chains were extracted unchanged from
https://emudhra.we-have-a-problem.com/chains.zip (downloaded 2026-10-22).
The ZIP SHA-256 is
`8a2edc81b6c51e3ad27d45ca1e441ec84c961e2dee0fc547f2a02c0f74315f95`.
The fixtures contain public certificates only, with no private keys.

The offline checks use the fixed verification time 2026-10-22 00:00:00 UTC
so that expiry does not hide a trust regression. The chains cover DV, OV,
and EV through both G3B/G3C, including the six AAA cross-signed paths.

Debian's ca-certificates security update removes the emSign ECC
Root CA - G3 and Comodo AAA Services root (DSA-6572-1;
https://security-tracker.debian.org/tracker/DSA-6572-1).
Removing AAA also prevents
other legacy chains that rely exclusively on that root from validating;
this is an intentional compatibility consequence of closing those paths.
