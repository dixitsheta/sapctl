---
title: SOX journal evidence pack
description: Export a signed, hash-chained bundle of S/4HANA journal items for SOX 404 evidence.
---

The `sox-journal` recipe pulls `A_JournalEntryItemBasic` rows for a given
posting-date range, writes one signed event per row to an ed25519 audit
chain, and packages the result as a `.tar.gz` an external auditor can
verify in isolation.

## Run

```bash
sapctl s4 audit-export \
  --cred sandbox \
  --use-case sox-journal \
  --from 2025-01-01 \
  --to   2025-12-31 \
  --out  ./evidence
```

## Bundle contents

- `rows.jsonl` -- one OData row per line
- `chain.jsonl` -- ed25519 hash-chained audit events (one per row + envelopes)
- `ed25519.pub` -- verifier public key (per-bundle, no host-key trust)
- `manifest.json` -- sha256 over rows + chain, ISO timestamp, use-case metadata

## Verify (auditor side)

```bash
tar -xzf sapctl-evidence-sox-journal-*.tar.gz
sapctl audit verify --chain chain.jsonl --pub ed25519.pub
```

## Variants

| `--use-case` | What it pulls |
|---|---|
| `sox-journal` | `A_JournalEntryItemBasic` by PostingDate |
| `sox-bp` | `A_BusinessPartner` master deltas by LastChangeDateTime |
