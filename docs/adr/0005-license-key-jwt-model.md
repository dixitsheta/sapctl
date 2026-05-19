# ADR 0005 — License-key model: offline-verifiable JWT (ed25519)

**Status:** Accepted (Phase 7 -- 2026-05-19)
**Date:** 2026-05-19
**Deciders:** CTO / Architect
**Supersedes:** none
**Superseded by:** none

## Context

Phase 7 task 7.1 introduces paid tiers (`Team` at $60/seat/mo annual, `Enterprise` priced later). The binary stays Apache-2.0 + open-source under [ADR 0003]; gating must be transparent, not obfuscation. Constraints:

- **Offline-first.** sapctl runs in air-gap (per the in-toto bundle workflow), behind enterprise proxies that block phone-home, and on classified-network hosts (Section 04 design partner). License verification must work with zero network at run time.
- **Auditable.** Customers can inspect the JWT, verify the signature themselves, see the entitlement set in plaintext. No black-box DRM.
- **Revocation must exist** for the rare bad-actor case (refund chargeback, contract breach) without breaking the offline-first guarantee.
- **Zero impact on free-tier UX.** The 95% of users on the open binary must never see a license check, license-error message, or grace-period nag.

## Decision

License keys are **ed25519-signed JWTs**:

1. **Signing key:** single root ed25519 keypair, private half lives in Cloudflare Secrets (license-issuer Worker). Public half ships embedded in the `sapctl` binary as a Go `[]byte` constant.
2. **Issuer:** Cloudflare Worker at `license.sapctl.dev` -- called server-to-server from Stripe webhook on `customer.subscription.created` / `.updated`. Customer is emailed the JWT + a one-line `sapctl license install` command.
3. **JWT structure** (RFC 7519 + custom claims):
   ```json
   {
     "iss":  "sapctl.dev",
     "sub":  "cus_StripeCustomerId",
     "aud":  "sapctl-cli",
     "iat":  1747526400,
     "exp":  1779062400,
     "nbf":  1747526400,
     "tier": "team",
     "seats": 5,
     "features": ["audit-export-retain-365d", "multi-cred"],
     "rev_url": "https://license.sapctl.dev/revoked.json"
   }
   ```
   Algorithm: `EdDSA` (ed25519). Header `kid` field reserved for future key rotation.
4. **Storage:** customer runs `sapctl license install --token <JWT>`. The JWT is written verbatim to `~/.config/sapctl/license.jwt` (chmod 600). Never edited, never re-signed.
5. **Verification path on every gated command:**
   - Read `~/.config/sapctl/license.jwt`. If absent -> downgrade to free-tier; do not error.
   - Verify ed25519 signature against embedded public key. If invalid -> error with `exit 5` and `license.invalid` code.
   - Check `exp` against system clock with 60-second skew tolerance. If expired -> error with `exit 5` and `license.expired`.
   - Check `aud == "sapctl-cli"`. Mismatch -> `license.audience`.
   - Read `features[]` -> set in-memory feature flag map for the duration of the command.
   - Cache the parsed claims in process memory; never re-read the file mid-command.
6. **Revocation:**
   - The `rev_url` claim points at a small JSON file (`https://license.sapctl.dev/revoked.json`) listing revoked `sub` values + revocation timestamps.
   - Revocation is checked **only** when the user explicitly runs `sapctl license refresh`. There is no background fetch, no phone-home on every command.
   - Air-gap customers: their internal IT can mirror `revoked.json` and point `sapctl license refresh --rev-url <internal>` at it; supports Section-04 defense workflow.
   - A revoked license keeps working offline until the customer next runs `license refresh` -- this is intentional. Revocation is a contractual remedy, not a kill switch.
7. **Grace window.** A JWT within 14 days past `exp` continues to work for read-only commands (audit verify, mirror search) but blocks new mutations. Encoded by the CLI, not the JWT.

## Rationale

- **ed25519 over RSA/HMAC:** smallest signature (64 bytes), constant-time verify, no padding-attack surface, fast Go stdlib implementation.
- **JWT over custom binary format:** developers can paste it into jwt.io to debug; we don't have to write a parser; revocation list semantics are well-trodden.
- **Embedded public key over fetched JWKS:** offline-first kills any JWKS rotation strategy that requires network. We accept the cost of needing a CLI release to rotate the issuer key; rotation will happen at most once per 2 years.
- **No license server dependency at run-time:** every dependency that must respond for sapctl to work is a procurement red flag for regulated buyers. The verify path runs entirely on local CPU.
- **Stripe over LemonSqueezy / Paddle:** simplest webhook surface, lowest fees on $600-1200 annual, no MoR overhead for our jurisdictions.

## Constraints / locked conventions

- JWT issuer `iss` is always the string `"sapctl.dev"`. Different value -> reject.
- JWT audience `aud` is always the string `"sapctl-cli"`. Different value -> reject (reserves namespace for a future Pro UI or hosted runner).
- Public key bytes are vendored in `apps/cli/internal/license/pubkey.go` -- single source of truth. CI fails if anyone tries to read it from a file path or env var.
- The feature flag map is a fixed enum in code. Adding a new gated feature requires a code change AND an ADR (no string-based dynamic entitlements).
- License-installed state is logged to the audit chain as `license.install` and `license.verify.ok` events. License rejections log `license.verify.fail` with claim hash (never raw JWT).

## Alternatives considered

- **Phone-home auth every N hours.** Rejected: kills air-gap + procurement story (R7-4 mitigation in Phase 7 plan).
- **HMAC-signed tokens with shared secret.** Rejected: any leak of the shared secret means forging arbitrary licenses; ed25519 lets us publish the public half.
- **Off-the-shelf license server (Keygen, Cryptlex).** Rejected: vendor lock-in, $200+/mo, breaks Apache-2.0 narrative, none ship pure offline verify.
- **No license at all; donation-only.** Rejected: Phase 7 gate-7 close criterion includes "first paying customer"; donations don't qualify.
- **Hardware-bound license (per-machine fingerprint).** Rejected: regulated buyers run sapctl in disposable CI runners + containers, fingerprints break constantly, support cost spikes.

## Consequences

- Issuer signing key compromise = forge-anywhere disaster. Mitigation: CF Workers Secrets binding, 1Password vault backup, monthly rotation rehearsal documented in the maintainer runbook.
- We can never charge for binary-level features that need to phone home in real time (e.g. "queries this month"); those become hosted-runner Pro features (Y2 scope).
- Revocation has a 1-`license refresh`-interval blast radius. Acceptable for $60/seat/mo product; revisit when first $10k+ contract lands.
- Adding a new gated feature is a code change + ADR -- intentionally high friction so we don't accidentally gate something the free tier needs.

## Implementation pointers (for Phase 7 tasks 7.1.1-7.1.5)

- New package: `apps/cli/internal/license/` (license.go, verify.go, install.go, pubkey.go, license_test.go).
- New command tree: `apps/cli/cmd/license.go` -- subcommands `install`, `verify`, `show`, `refresh`, `revoke-check`.
- New audit event types: `license.install`, `license.verify.ok`, `license.verify.fail`, `license.refresh`.
- New CF Worker: `apps/license-issuer/` (TypeScript, deploys via wrangler) -- handles Stripe webhook -> JWT mint.
- New page: `apps/web-static/pricing/index.html` + portal redirect to Stripe.
- Embed-keygen build step: `scripts/gen-license-pubkey.go` reads `LICENSE_ISSUER_PRIVATE_KEY` from env, writes the public half to `apps/cli/internal/license/pubkey.go`. Run **once** per issuer-key rotation; output diff is reviewed in PR.

[ADR 0003]: 0003-license-apache-2.md
