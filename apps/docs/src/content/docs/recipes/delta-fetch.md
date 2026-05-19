---
title: Delta fetch with watermarks
description: Pull only what changed since the last run.
---

```bash
sapctl s4 odata get \
  --cred sandbox \
  --service API_BUSINESS_PARTNER --entity A_BusinessPartner \
  --mirror /tmp/bp.db --key-field BusinessPartner \
  --since-field LastChangeDateTime
```

First run pulls full set + stores `max(LastChangeDateTime)` in
`mirror.watermarks`. Subsequent runs auto-append
`$filter=LastChangeDateTime gt datetimeoffset'<watermark>'` and advance
the cursor.

Force full refresh: add `--since-reset`. Manual override:

```bash
sapctl mirror set-watermark --db /tmp/bp.db \
  --service API_BUSINESS_PARTNER --entity A_BusinessPartner \
  --since '2026-01-01T00:00:00Z'
```
