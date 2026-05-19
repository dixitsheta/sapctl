---
title: Air-gap install
description: Move sapctl + specs into a disconnected network with verifiable provenance.
---

On the internet side:

```bash
sapctl bundle export \
  --name sapctl-airgap \
  --version 1.0.0 \
  --include specs,recipes,binary \
  --dir ./company \
  --out sapctl-airgap-1.0.0.tar.gz
```

Carry over (USB, cross-domain transfer). On the air-gap side:

```bash
sapctl bundle verify  --bundle sapctl-airgap-1.0.0.tar.gz
sapctl bundle install --bundle sapctl-airgap-1.0.0.tar.gz \
                      --dest /opt/sapctl
```

Verification refuses on hash mismatch, signature failure, or path-traversal
attempts in the tar.
