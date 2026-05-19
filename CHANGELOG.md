# Changelog

All notable changes to sapctl. Format based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/), versioning follows [SemVer](https://semver.org/).

## [Unreleased]

### Added
- Central `~/.config/sapctl/.env` auto-loader (`internal/config/env.go`). Auto-loads on every invocation. Shell env takes precedence. Format: `KEY=VALUE` with `#` comments, optional `export `, optional double-quoted value.
- `.env.example` at repo root documenting every key sapctl + Pages Functions read.
- `docs/manual-setup.md` Sections H-K: GitHub repo flip + branch protection, docs.sapctl.dev Cloudflare Pages project, Google Search Console verification, central `.env` migration walkthrough.
- `apps/web-static/google41f66520f7aade66.html` for Search Console verification.

### Changed
- `apps/cli/main.go` now calls `config.LoadDefault()` before Cobra dispatch.

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
