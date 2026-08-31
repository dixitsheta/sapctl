# ADR 0001 — Primary CLI language: Go

**Status:** Accepted
**Date:** 2026-05-18
**Deciders:** CTO / Senior Architect (Claude composite)

## Context

sapctl is a unified CLI consuming SAP REST/OData/SQL APIs, emitting MCP servers + signed audit chains, distributed as a single static binary across linux/macos/windows x amd64/arm64. 95% of runtime is HTTP IO against SAP APIs.

## Decision

Use **Go 1.23+** as the primary implementation language for the sapctl CLI and all backend services.

## Rationale

- Static single-binary distribution out-of-box (`GOOS`/`GOARCH` matrix).
- Cobra ecosystem (kubectl, helm, gh, terraform precedent) = mature CLI UX patterns.
- OpenAPI Generator has first-class Go target; custom Cobra template feasible.
- Sigstore (cosign), SLSA builders, CycloneDX tooling, in-toto, opa, syft = reference implementations in Go.
- Largest LLM training corpus for CLI patterns = best AI-assisted velocity for a small team.
- Compile time 2-5s incremental = tight feedback loop.
- Hiring pool when team grows (k8s/cloud-native devs) is the largest.

## Alternatives considered

- **Rust** — see ADR 0004. Rejected for v1 on velocity + ecosystem maturity grounds; selective Y2 carve-outs planned for crypto-heavy components.
- **TypeScript (Node/Bun)** — single-binary story weaker; native dep hell on Windows; not used.
- **Python** — packaging story unacceptable for enterprise air-gap distribution; not used.

## Consequences

- Bound to Go module ecosystem for next 24 months.
- Web/docs apps may use TypeScript (separate ADR).
- Y2 may introduce Rust crates for `audit-core` + WASM verifier (ADR 0004).
