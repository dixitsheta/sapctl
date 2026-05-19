# ADR 0003 — License: Apache-2.0 (deferred file commit)

**Status:** Accepted
**Date:** 2026-05-18
**Deciders:** CEO + CTO + Sec lead

## Context

sapctl distribution strategy is open-core: free OSS CLI + paid Team/Business/Enterprise tiers. License must (a) maximize enterprise adoption (no copyleft drag on GSI integrators), (b) preserve commercial layer (Stripe + license keys), (c) align with Sigstore + CNCF ecosystem norms, (d) be defensible under SAP API Policy v4 interpretation.

## Decision

**Apache License 2.0** for all open-source code in this repo.

Repo currently **private**. `LICENSE` file is NOT committed yet — added at the moment of public flip. Until then, default = all-rights-reserved by founder.

## Rationale

- Apache-2.0 is the de-facto standard for cloud-native infrastructure (kubernetes, terraform, opentofu, cosign, syft, SLSA tooling).
- Includes explicit patent grant -> reduces enterprise legal-review friction.
- No copyleft -> GSI consultants (Accenture/Deloitte/Capgemini/TCS) can integrate freely.
- Compatible with closed-source commercial layer (License-key gating in CLI, Stripe billing, hosted services).
- SAP Partner programs accept Apache-2.0 deps.

## Alternatives considered

- **MIT** — simpler but no patent grant; weaker enterprise legal posture.
- **MPL-2.0** — file-level copyleft; acceptable but creates contributor confusion.
- **AGPL** — kills enterprise adoption + GSI integration; rejected.
- **BSL (Business Source License)** — viable later for commercial layer code, NOT for the OSS CLI. May revisit for `apps/web` or paid services packages.
- **No license / source-available** — kills community contribution; rejected.

## Consequences

- All dependencies must be Apache-2.0 / MIT / BSD / ISC / MPL-2.0 / Unlicense compatible. **No GPL, no AGPL, no SSPL deps.** Verify in CI with license scanner (`go-licenses`, `licensecheck`).
- Contributor License Agreement (CLA) or DCO required from external contributors when repo flips public.
- License-key code in `apps/cli/internal/license` may be relicensed (BSL / proprietary) at v1.0 GA if needed.
