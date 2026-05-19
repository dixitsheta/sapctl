---
title: Command reference
description: Top-level sapctl commands.
---

| Subcommand | Purpose |
|---|---|
| `auth` | Manage SAP credentials (apikey / basic / xsuaa) |
| `s4` | SAP S/4HANA Cloud: catalog discover, odata get, audit-export |
| `btp` | SAP BTP: subaccount, service-instance, service-binding |
| `datasphere` | SAP Datasphere: space, sql exec, replication-flow |
| `aicore` | SAP AI Core + GenAI Hub: deployments, models, completions |
| `audit` | ed25519 hash-chained audit log (init / emit / verify) |
| `mirror` | Local SQLite mirror queries + watermark management |
| `bundle` | Air-gap export / install / verify (in-toto signed) |
| `mcp` | Model Context Protocol server (`serve`, `list-tools`) |
| `version` | Print build metadata |

Global flags (locked by [ADR 0002](https://github.com/dixitsheta/sapctl/blob/main/docs/adr/0002-cli-framework-cobra.md)):

\`\`\`
--json --select --dry-run --compact --quiet --yes --no-input --agent --since --audit
\`\`\`

Exit codes:

| Code | Meaning |
|---|---|
| 0 | success |
| 2 | user error |
| 3 | not found |
| 4 | conflict (also used for destructive guard) |
| 5 | auth failure |
| 7 | rate-limited |
