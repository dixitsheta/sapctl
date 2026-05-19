# ADR 0002 — CLI framework: Cobra

**Status:** Accepted
**Date:** 2026-05-18
**Deciders:** CTO / Senior Architect

## Context

Need consistent subcommand UX across 8+ SAP product trees (s4, btp, datasphere, aicore, joule, cpi, event-mesh, sf, ariba, concur, etc.), with shared global flags, locked exit codes, machine-readable JSON output, and auto-generated docs + MCP server emission.

## Decision

Use **`github.com/spf13/cobra` v1.8+** for command tree, `github.com/spf13/viper` for layered config (env > flag > file).

## Rationale

- kubectl/gh/helm/terraform precedent = users already know conventions.
- Built-in `--help`, completion (bash/zsh/fish/powershell), nested subcommands.
- AST is walkable -> drives MCP server emission + A2A Agent Card generation.
- Generates man pages + markdown docs auto.
- Compatible with OpenAPI Generator custom template.

## Constraints / locked conventions

- **Global flags** (root, inherited): `--json`, `--select`, `--dry-run`, `--compact`, `--quiet`, `--yes`, `--no-input`, `--agent`, `--since`.
- **Exit codes** (centralized in `internal/errs`): 0=success, 2=user error, 3=not found, 4=conflict, 5=auth, 7=rate-limited.
- **Help width** <= 80 chars per line.
- **Default output** = human-readable table; `--json` switches to machine output.
- **No subcommand** may exceed 250ms wall time for non-network ops.

## Alternatives considered

- `urfave/cli` — simpler, weaker subcommand depth, no AST walking.
- `kong` — newer, smaller community.
- Hand-rolled flag parser — non-starter solo+AI.

## Consequences

- Locked into Cobra conventions; migration cost high if ecosystem dies (low probability).
- MCP emitter package depends on Cobra AST shape (`packages/mcp-emitter`).
