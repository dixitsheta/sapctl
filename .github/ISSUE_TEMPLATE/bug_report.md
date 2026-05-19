---
name: Bug report
about: Something broke at runtime
title: 'bug: '
labels: ['bug', 'triage']
---

## What happened

<!-- Exact command + observed output. Use code fences. Redact secrets. -->

## What you expected

## sapctl version + environment

```
sapctl version --json
```

| Field | Value |
|---|---|
| OS | |
| Arch | |
| sapctl | |
| SAP product | (S/4HANA Cloud / BTP / Datasphere / AI Core / ...) |
| Auth flow | (apikey / basic / xsuaa) |

## Reproduction

Minimal steps. Synthetic / sanitised inputs only.

## Logs

`SAPCTL_TRACE=1 sapctl <your command>` output, with any sensitive data
redacted.
