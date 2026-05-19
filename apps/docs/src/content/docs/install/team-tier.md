---
title: Team-tier install
description: Activate sapctl Team-tier features with an offline-verifiable ed25519 JWT license.
---

`sapctl` is Apache-2.0 open source. Everything in the binary runs on the
free tier. Team-tier unlocks a small, named set of features behind an
offline-verifiable license (per ADR 0005):

- Extended audit-export retention (`--retain` beyond 30 days, up to 365)
- Multi-credential parallel storage (Y2)
- Priority support + SLA

If you don't need any of those, you don't need a license.

## Prerequisites

- sapctl v1.0 or later (`sapctl version`)
- An active Team-tier subscription on [sapctl.dev/pricing](https://sapctl.dev/pricing)
- The JWT delivered to your billing email after checkout

## Install the license

Three input methods. Pick whichever your environment allows.

### From the command line

```bash
sapctl license install --token "eyJhbGciOiJFZERTQSIsInR5cCI6IkpXVCJ9...."
```

### From a file

```bash
echo "$YOUR_JWT" > ~/sapctl.jwt
sapctl license install --from-file ~/sapctl.jwt
rm ~/sapctl.jwt
```

### From stdin (CI-friendly)

```bash
echo "$SAPCTL_LICENSE_JWT" | sapctl license install --from-file -
```

On success sapctl prints the tier, seat count, expiry, and feature set,
and writes the JWT to `~/.config/sapctl/license.jwt` with mode 0600.

## Verify

```bash
sapctl license verify
# license OK -- signature + audience + issuer + expiry all valid
```

If you want to inspect the claims without piping the raw JWT into jwt.io:

```bash
sapctl license show --json
```

## What's gated

| Feature flag | Free | Team | What it unlocks |
|---|---|---|---|
| `audit-export-retain-365d` | ≤ 30 d | up to 365 d | `sapctl s4 audit-export --retain N` for SOX-grade 12-month evidence runs |
| `multi-cred` | 1 | unlimited | Multiple stored credentials per service (e.g. `prod` + `non-prod` BTP at once) |

The free tier covers every read-path, every audit-chain primitive, every
MCP tool, every SBOM + cosign verification, and every air-gap bundle
operation. Don't pay for a license unless one of the flags above is the
specific thing you need.

## Verify offline

The license check is fully offline. The ed25519 public key is embedded
in the sapctl binary; no network is required at `verify` or at the use
of any gated feature. The only command that touches the network is
`sapctl license refresh`, and you only run it when you choose to.

For air-gap environments mirroring the revocation list internally, see
the `--rev-url` flag on `sapctl license refresh`.

## Subscription lifecycle

| Event | sapctl behaviour |
|---|---|
| New subscription | JWT delivered by email; install via `sapctl license install` |
| Subscription renews | New JWT emailed; reinstall over the old one |
| Subscription expires | Gated features hard-fail with exit code 5 |
| Refund / chargeback | Subject (`sub` claim) listed on revocation; takes effect on next `sapctl license refresh` |
| Key rotation (rare) | Triggers a new sapctl release; upgrade the binary, then reinstall |

## Troubleshooting

### `license signature invalid`

The JWT was signed by the wrong key, or your sapctl binary predates the
key rotation. Run `sapctl version` and confirm you're on a release that
matches the current issuer key. If unsure, upgrade.

### `license expired`

Your subscription's window has ended. Renew on
[sapctl.dev/pricing](https://sapctl.dev/pricing) and reinstall the new JWT.

### `license audience mismatch`

You installed a JWT meant for a different sapctl distribution (e.g. a
hosted-runner Pro token in the CLI). Use the JWT marked `aud: sapctl-cli`.

### `--retain N requires Team-tier`

You requested retention beyond 30 days without a license. Either drop
the `--retain` flag (uses the default 30-day annotation) or install the
license.

## Uninstalling

```bash
rm ~/.config/sapctl/license.jwt
sapctl license verify
# Error: no license installed (free-tier active)
```

The binary continues to work for everything that isn't gated.
