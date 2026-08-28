# Changelog

All notable changes to sapctl. Format based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/), versioning follows [SemVer](https://semver.org/).

## [Unreleased]

*(No changes since v1.0.0)*

## [1.0.0] - 2026-08-28

### Breaking changes

*(None — v1.0.0 is backwards-compatible with v0.1.0-alpha.2 across all documented command surfaces.)*

### Added

- **License framework.** `sapctl license install | verify | show | refresh` — ed25519-signed JWT, offline-verifiable against an embedded issuer public key per ADR 0005. Free-tier users see no change; Team-tier features gated by `features` claim.
- **Audit-export retention gate.** `sapctl s4 audit-export --retain N` — `N > 30 days` requires Team-tier license (`audit-export-retain-365d` feature flag); hard upper bound 365 days.
- **License ADR.** `docs/adr/0005-license-key-jwt-model.md` — offline-verifiable JWT model, revocation strategy, Team-tier feature matrix.
- **Team-tier install recipe.** `apps/docs/src/content/docs/install/team-tier.md` — step-by-step install + troubleshooting.
- **Trust portal.** `sapctl.dev/trust` — CRA/DORA/SOX/Part 11 regulation mapping, live SBOM from GitHub Releases API, SOC 2 Type I audit window status, Schema.org SoftwareApplication JSON-LD.
- **Status page.** `status.sapctl.dev` + `status.json` source of truth + `incidents.rss` — release pipeline, sapctl.dev, docs.sapctl.dev, SAP API Hub reachability tracked.
- **Central config loader.** `~/.config/sapctl/.env` auto-loaded on every invocation. Shell env takes precedence. `.env.example` at repo root.
- **docs.sapctl.dev.** Astro Starlight docs site (17 pages): Quickstart, Install, Auth reference, CLI reference, 10 golden recipes (sox-journal, sox-bp, dora-evidence, cra-sbom, mcp-claude-desktop, mcp-ollama-bridge, air-gap, delta-fetch, mirror-fts5, verify-release).

### Changed

- `apps/cli/main.go` calls `config.LoadDefault()` before Cobra dispatch.
- ADR 0002 ("Cobra framework"): drop "solo+AI" phrasing in Alternatives row.

### Security

- License `verify` is fully offline (embedded ed25519 pubkey, no JWKS fetch). `refresh` is the only network-touching command and only on explicit user call.
- License file stored at `~/.config/sapctl/license.jwt` with `0600`. `Install()` performs a verify-roundtrip before writing; signature failure leaves the filesystem untouched.

## [0.1.0-alpha.2] - 2026-05-19

### Fixed
- goreleaser monorepo layout. `go mod tidy` hook now scopes to `apps/cli` (no go.mod at repo root). `dir`/`main` set on each build target.
- `archives.format_overrides.format` deprecation. Switched to `formats: [tar.gz]` + per-OS `format_overrides[].formats: [zip]`.
- CI Go version. Bumped build + release workflows to 1.25 to match `go.mod` auto-bump.

## [0.1.0-alpha.1] - 2026-05-19

### Added
- **CLI scaffold.** Cobra root + global flags: `--json`, `--select`, `--dry-run`, `--compact`, `--quiet`, `--yes`, `--no-input`, `--agent`, `--since`, `--audit`. Locked exit codes: 0=success, 2=user error, 3=not found, 4=conflict, 5=auth, 7=rate-limited.
- **Auth providers.** `apikey` (Business Accelerator Hub sandbox), `basic` (S/4 Communication User), `xsuaa` (BTP OAuth2 client_credentials) via `sapctl auth login/status/logout/list`.
- **HTTP client.** `internal/sap` with auth injection, retry, gzip, exit-code mapping, audit middleware.
- **S/4HANA.** `sapctl s4 catalog discover` + `s4 odata get/list/post/patch/delete` with `--top/--select/--filter/--orderby/--expand/--mirror`.
- **CDC.** `sapctl s4 cdc pull --entity X --since <cursor>` via `Prefer: odata.track-changes`, watermark stored in SQLite mirror.
- **Audit chain.** `packages/audit-chain`: ed25519 hash-chained JSONL. Genesis = 64 zero bytes. `sapctl audit init/emit/verify`.
- **SQLite mirror.** `packages/sqlite-mirror`: FTS5 search over fetched OData rows, watermark tracking, `sapctl mirror list/search/stats`.
- **BTP.** `sapctl btp subaccount/entitlement/instance/key` CRUD via XSUAA OAuth2 client credentials.
- **Datasphere.** `sapctl datasphere space/object list`.
- **AI Core.** `sapctl aicore deployment list/get` + `genai hub model list`.
- **Audit export.** `sapctl s4 audit-export --use-case sox-journal|sox-bp --from --to --out` emits signed `.tar.gz` bundle (rows + chain + ed25519.pub + manifest).
- **Air-gap bundle.** `sapctl bundle create/verify` per in-toto v1 statement format.
- **MCP server.** `sapctl mcp serve` exposes Cobra tree as JSON-RPC 2.0 stdio tools (32 auto-emitted).
- **Tracing.** OTLP-shape JSON spans on stderr via `SAPCTL_TRACE=1`.
- **Release pipeline.** goreleaser v2 + cosign keyless OIDC + syft CycloneDX 1.7 SBOMs + SLSA L3 provenance. Multi-platform: linux/darwin/windows x amd64/arm64.
- **Marketing site.** `apps/web-static/` ported from `ref/sapctl/`. Live on Cloudflare Pages at sapctl.dev with Turnstile-protected contact form (`/api/contact` via Resend) and `/api/releases` proxy.
- **Docs scaffold.** `apps/docs/` Astro Starlight project. Quickstart, install, auth reference, CLI reference, 10 golden recipes (sox-journal, sox-bp, dora-evidence, cra-sbom, mcp-claude-desktop, mcp-ollama-bridge, air-gap, delta-fetch, mirror-fts5, verify-release).
- **Community files.** Apache-2.0 LICENSE, CONTRIBUTING.md, CODE_OF_CONDUCT.md, SECURITY.md with 90-day CVD window.
- **ADRs 0001-0004.** Go language, Cobra framework, Apache-2.0 license, Rust carve-out Y2.

[Unreleased]: https://github.com/dixitsheta/sapctl/compare/v0.1.0-alpha.2...HEAD
[0.1.0-alpha.2]: https://github.com/dixitsheta/sapctl/compare/v0.1.0-alpha.1...v0.1.0-alpha.2
[0.1.0-alpha.1]: https://github.com/dixitsheta/sapctl/releases/tag/v0.1.0-alpha.1
