---
title: Quickstart
description: First end-to-end run in under five minutes.
---

```bash
# 1. Sign in to SAP Business Accelerator Hub, grab the sandbox API key,
#    then save it locally.
sapctl auth login --flow apikey --label sandbox --api-key "$KEY"

# 2. Discover available OData services.
sapctl s4 catalog discover --cred sandbox --top 5

# 3. Pull Business Partner records.
sapctl s4 odata get \
  --cred sandbox \
  --service API_BUSINESS_PARTNER \
  --entity A_BusinessPartner \
  --top 2 \
  --select-fields 'BusinessPartner,BusinessPartnerFullName'

# 4. Initialise the audit chain (one-time).
sapctl audit init

# 5. Re-run any command with --audit to record signed events.
sapctl --audit s4 catalog discover --cred sandbox --top 5

# 6. Verify the chain.
sapctl audit verify
```
