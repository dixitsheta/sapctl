# ADR 0004 — Go for CLI v1, Rust carve-outs Y2

**Status:** Accepted
**Date:** 2026-05-18
**Supersedes:** none
**Related:** ADR 0001 (Go), ADR 0002 (Cobra)

## Context

Founder asked: can sapctl be built in Rust with same architecture, plus desktop apps (Win/Mac/Linux) and mobile (iOS/Android) from one codebase? Tradeoff analysis required across velocity, ecosystem, AI codegen quality, and user benefit.

## Decision

1. **CLI v1 stays Go.** No Rust in v1 hot path.
2. **No native desktop app v1.** Web (Next.js) covers cross-platform browser UX. Optional Tauri wrap of Next.js Y2 if buyer demand emerges.
3. **No mobile app v1 or v2.** Buyer persona does not run CLI from mobile. Mobile = read-only PWA from same Next.js app if ever needed.
4. **Y2 selective Rust carve-outs:**
   - `packages/audit-core` — hash-chain + ed25519 + cosign verify, compiled to **WASM + C ABI**. Linked from Go CLI via cgo; reused in browser trust portal.
   - `packages/verifier-wasm` — browser-side SBOM + cosign + Rekor proof verification widget.

## Rationale

| Factor | Go | Rust | Winner |
|---|---|---|---|
| Compile speed incremental | 2-5s | 30-90s | Go (10x dev velocity) |
| AI codegen quality (Claude/Cursor) for CLI patterns | excellent | good (lifetimes confuse LLMs) | Go (30-50% faster solo) |
| OpenAPI -> CLI scaffold | mature OpenAPI Generator + Cobra template | progenitor or hand-roll | Go (1-2 mo saved) |
| Sigstore/cosign/SLSA reference impl | Go | port-of-port (sigstore-rs beta) | Go (sec confidence) |
| SAP ecosystem precedent | abapGit, BTP CLI patterns adjacent | none | Go |
| Hire pool when team grows | huge (k8s devs) | smaller, costlier | Go |
| Static binary | yes | yes | tie |
| Perf at CLI scale (IO-bound SAP HTTP) | fine | fine | tie |
| Mobile/desktop one-codebase story | needs wrap | Tauri 2 (beta mobile) | Rust — but wrong problem; see below |

**Cross-platform reality check:** sapctl is a CLI. Buyers (enterprise architect, DevOps eng, GSI consultant, auditor) work in terminal + CI + air-gapped VM. No buyer needs sapctl on iPhone. "Cross-platform" = static binary across linux/macos/windows, which Go ships natively via `GOOS`/`GOARCH`. Browser is the cross-platform UI runtime (Next.js, already in plan).

**Where Rust earns its weight (Y2):**
- Memory-safe crypto loop (audit chain + signature verification) maps cleanly to Rust ownership model.
- Rust -> WASM enables **browser-side artifact verification** without trusting our server = differentiating compliance feature.
- Reused via C ABI from Go CLI = no duplication.

## Alternatives considered

- **Full Rust rewrite:** rejected. +30-50% calendar time, no user-facing benefit, weakens AI velocity, weakens Sigstore alignment.
- **Tauri desktop app v1:** rejected. No buyer demand. Adds maintenance surface for zero ARR impact.
- **Flutter/React Native mobile:** rejected. Wrong persona for a CLI product.
- **Rust for entire backend, Go for CLI:** rejected. Two languages = double cognitive load solo.

## Consequences

- Y1 stays Go-only (CLI + backend services + MCP emitter + SQLite mirror).
- Y2 introduces 2 Rust crates with strict scope (no Rust beyond `audit-core` + `verifier-wasm` without new ADR).
- Need cgo build pipeline for `audit-core` Y2 (acceptable; standard practice).
- Need wasm-pack toolchain Y2 for browser verifier.
- Marketing claim "audit chain verified in-browser via Rust+WASM" preserved as Y2 differentiator.

## Revisit trigger

Reopen this ADR if:
- 3+ design partners explicitly require native desktop app.
- Auditor persona requests mobile read-only access (likely PWA, still no Rust mobile).
- Go-side cosign / SLSA tooling fragments and Rust ecosystem becomes reference.
- Team grows to 5+ engineers with Rust expertise on board.
