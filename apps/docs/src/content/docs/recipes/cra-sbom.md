---
title: CRA-ready SBOM
description: Every release ships CycloneDX SBOM + cosign keyless signature.
---

Released binaries include `*.cdx.json` (CycloneDX 1.7 SBOM per platform)
plus a cosign-signed checksum file.

```bash
gh release download v0.1.0-alpha.2 -R dixitsheta/sapctl -p '*.cdx.json'
syft scan sbom:sapctl_0.1.0-alpha.2_linux_amd64.cdx.json -o table
```

For air-gapped distributions:

```bash
sapctl bundle export --include specs,recipes --out evidence.tar.gz
sapctl bundle verify --bundle evidence.tar.gz
```
