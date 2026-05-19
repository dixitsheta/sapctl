---
title: SOX BP master deltas
description: Capture Business Partner master-data changes for SOX ITGC evidence.
---

```bash
sapctl s4 audit-export \
  --cred sandbox \
  --use-case sox-bp \
  --from 2025-01-01 \
  --to   2025-12-31 \
  --out  ./evidence
```

Pulls `A_BusinessPartner` filtered by `LastChangeDateTime`. Same signed
bundle layout as [sox-journal](/recipes/sox-journal/).
