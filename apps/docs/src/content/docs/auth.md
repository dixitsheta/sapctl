---
title: Authentication
description: Auth flows supported by sapctl and where each applies.
---

sapctl stores credentials under `~/.config/sapctl/tokens.json` with mode
0600. Bearer tokens are never persisted; OAuth2 refreshes happen in
memory only.

## Flows

| Flow | Use it when | Example |
|---|---|---|
| `apikey` | SAP Business Accelerator Hub sandbox | see [Quickstart](/getting-started/) |
| `basic` | S/4HANA Cloud Communication User | `--username SAPCTL_DEV_USER --password '<pw>'` |
| `xsuaa` | BTP service-key bindings (Cloud Management, Datasphere, AI Core, ...) | `--client-id <c> --client-secret <s> --token-url '<uaa>/oauth/token'` |

## Manage stored credentials

```bash
sapctl auth list
sapctl auth status --label sandbox
sapctl auth logout --label sandbox
```

Status output redacts secrets to `abc***xyz` shape.

## Wrong-flow guard

Each product subcommand enforces the expected flow. `sapctl btp` with an
`apikey` credential returns `ExitUserError` with `expected flow=xsuaa`.
