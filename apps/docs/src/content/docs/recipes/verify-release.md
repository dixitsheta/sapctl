---
title: Verify a sapctl release
description: Independently confirm a binary came from this repository's GitHub Actions.
---

```bash
TAG=v0.1.0-alpha.2

gh release download "$TAG" -R dixitsheta/sapctl \
  -p '*checksums*' -p '*linux_amd64.tar.gz'

cosign verify-blob \
  --certificate-identity-regexp='https://github.com/dixitsheta/sapctl/.github/workflows/release.yml@.*' \
  --certificate-oidc-issuer='https://token.actions.githubusercontent.com' \
  --certificate=sapctl_${TAG#v}_checksums.txt.pem \
  --signature=sapctl_${TAG#v}_checksums.txt.sig \
  sapctl_${TAG#v}_checksums.txt
```

The certificate is logged in the public Sigstore [Rekor transparency log](https://search.sigstore.dev/);
deletion is impossible. SBOMs (`*.cdx.json`) cover every archive.
