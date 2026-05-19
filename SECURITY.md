# Security Policy

## Reporting a vulnerability

Email **security@sapctl.dev** with details. PGP key published at `/security` on sapctl.dev once live. While the repo is private, contact via GitHub Security Advisories or directly to the founder.

We follow a coordinated vulnerability disclosure (CVD) process:

- We acknowledge receipt within **72 hours**.
- We commit to triage within **5 business days**.
- We target a fix within **90 days** of acknowledgement.
- We coordinate disclosure with you and credit you in the changelog (unless you prefer anonymity).

## Out of scope

- Vulnerabilities in third-party SAP services. Report those to SAP directly.
- Issues in dependencies — report upstream first; we will respond if our integration amplifies the issue.
- Findings against pre-alpha builds (`0.x.y-dev`) not eligible for credit; mainline `0.x.y` and later are eligible.

## Supply chain

- All release binaries are signed with **cosign keyless** via Sigstore.
- Every release includes a **CycloneDX 1.7 SBOM** and **SLSA L3 provenance**.
- Verify with `cosign verify-blob` (see `/trust` once published).

## Cryptography

- ed25519 for audit chain signing.
- TLS 1.3 minimum for all outbound HTTP.
- AES-256-GCM at rest where applicable.
- No homegrown crypto. All primitives via Go standard library or `crypto/ecdsa`, `crypto/ed25519`, `golang.org/x/crypto`.
