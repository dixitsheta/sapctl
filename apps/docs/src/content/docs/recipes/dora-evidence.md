---
title: DORA register-of-information
description: Build a signed evidence pack for DORA Article 28 reporting.
---

```bash
sapctl audit init
sapctl --audit s4 odata get \
  --cred sandbox \
  --service API_BUSINESS_PARTNER --entity A_BusinessPartner \
  --mirror /var/lib/sapctl/dora.db --key-field BusinessPartner \
  --since-field LastChangeDateTime --all
sapctl audit verify
```

Pair the mirror DB + signed audit chain in the bundle handed to your
operational-resilience function. Hash chain links every fetch to genesis;
ed25519 signature proves origin.
