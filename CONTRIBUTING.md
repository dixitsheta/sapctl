# Contributing to sapctl

Thanks for considering a contribution. sapctl is a solo + AI-agent project
in pre-alpha; the bar for accepting outside contributions is unusual.

## Quick rules

- **By contributing you agree your code is licensed under Apache-2.0.** No CLA.
- **Open an issue before opening a PR for anything > 20 lines.** Drive-by
  refactors are routinely closed.
- **Every PR must be green** (CI matrix + tests + go vet).
- **Conventional Commits** for messages (`feat:`, `fix:`, `docs:`, etc).
- **No SAP IP.** OpenAPI specs, recipe queries, brand names are user-fetched
  at runtime; do not commit SAP-copyrighted artifacts.
- **No telemetry, no analytics, no phone-home code.** Hard rule.
- **Locked exit codes (0/2/3/4/5/7) and the global flag list never change.**
  Both are part of the public CLI contract per ADR 0002.

## Local dev

```bash
git clone https://github.com/dixitsheta/sapctl.git
cd sapctl/apps/cli
go vet ./... && go test -race ./...
```

For multi-module changes:

```bash
cd packages/audit-chain   && go test ./...
cd packages/sqlite-mirror && go test ./...
cd packages/mcp-emitter   && go test ./...
```

## Areas where help is most welcome

| Area | What we need |
|---|---|
| Recipes | New `s4 audit-export --use-case <name>` entries for regulatory needs (CSRD, AI Act Annex IV, 21 CFR Part 11) |
| OpenAPI specs | Verified Published-API specs from the SAP Business Accelerator Hub |
| MCP client configs | Cover any client missing from `clients/` |
| Bug reports against real SAP tenants | We have trial-only coverage; production breakage reports are gold |

## Areas where help is NOT welcome

| Area | Why |
|---|---|
| Cosmetic refactors | Architecture is settled per ADRs 0001-0004 |
| New SAP CLI dependencies | `sapctl` deliberately replaces them |
| Rust rewrites (yet) | Y2 carve-out per ADR 0004 |
| Closed-source forks | Allowed by license, but won't be supported |

## Security disclosures

Do **not** open public issues for security. See [SECURITY.md](SECURITY.md).
