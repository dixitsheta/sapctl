---
title: Local SQLite mirror with FTS5
description: Query mirrored SAP data with full-text search.
---

```bash
sapctl mirror stats  --db /tmp/bp.db --service API_BUSINESS_PARTNER --entity A_BusinessPartner
sapctl mirror list   --db /tmp/bp.db --service API_BUSINESS_PARTNER --entity A_BusinessPartner --limit 5
sapctl mirror search --db /tmp/bp.db --service API_BUSINESS_PARTNER --entity A_BusinessPartner \
  --query 'Frankfurt OR Berlin'
```

Schema is documented in `packages/sqlite-mirror/store.go`. Pure-Go driver
(`modernc.org/sqlite`); no cgo, runs on every arch in CI.
