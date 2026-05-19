---
title: Install
description: Download a signed sapctl binary or build from source.
---

## Signed releases

Every tag publishes signed binaries for linux/macos/windows x amd64/arm64
with CycloneDX SBOM + cosign keyless signature.

```bash
gh release download v0.1.0-alpha.2 -R dixitsheta/sapctl \
  -p '*linux_amd64*' -p '*checksums*'

cosign verify-blob \
  --certificate-identity-regexp='https://github.com/dixitsheta/sapctl/.github/workflows/release.yml@.*' \
  --certificate-oidc-issuer='https://token.actions.githubusercontent.com' \
  --certificate=sapctl_0.1.0-alpha.2_checksums.txt.pem \
  --signature=sapctl_0.1.0-alpha.2_checksums.txt.sig \
  sapctl_0.1.0-alpha.2_checksums.txt

tar -xzf sapctl_0.1.0-alpha.2_linux_amd64.tar.gz
sudo install sapctl /usr/local/bin/
sapctl version
```

## Build from source

```bash
git clone https://github.com/dixitsheta/sapctl.git
cd sapctl/apps/cli
go build -o bin/sapctl .
sudo ln -sf "$PWD/bin/sapctl" /usr/local/bin/sapctl
```

Go 1.25+ required.
