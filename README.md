# sapctl

> One CLI for the SAP portfolio. Designed so an LLM agent can drive it as easily as a human.

`sapctl` started because I got tired of bouncing between BTP Cockpit, S/4 Fiori, and twelve different OData docs to pull one journal extract for an auditor. The plan is a single binary that speaks to S/4HANA, BTP, Datasphere, and AI Core today — and to Integration Suite, SuccessFactors, Ariba, Concur, SAC, Signavio, LeanIX, and Cloud ALM over time — with the same flags, the same JSON schema, and the same signed audit chain on every call.

Three things it tries to get right:

- **Agent-native output.** Every command takes `--json` and emits MCP tool descriptors — Claude, Cursor, and Ollama can call it without screen-scraping.
- **Compliance-ready by default.** ed25519 hash-chained audit log, CycloneDX 1.7 SBOM, cosign keyless signature, and SLSA L3 provenance on every release — built for the EU Cyber Resilience Act, DORA, and SOX 404.
- **Works disconnected.** Air-gap bundle format (in-toto v1) — the same chain that runs in a German bank's prod also runs on a USB stick into a classified network.

**Status:** v0.1.0-alpha. v1.0 GA targeted Q1 2027.

---

## Install

```bash
# macOS / Linux -- grab a signed release
curl -L https://github.com/dixitsheta/sapctl/releases/latest/download/sapctl_$(uname -s)_$(uname -m).tar.gz | tar xz
sudo install sapctl /usr/local/bin/

# Verify the binary (cosign keyless)
cosign verify-blob \
  --certificate-identity-regexp 'https://github\.com/dixitsheta/sapctl' \
  --certificate-oidc-issuer 'https://token.actions.githubusercontent.com' \
  --signature sapctl.sig sapctl

sapctl version
```

Full walkthroughs: <https://docs.sapctl.dev>

---

## 30-second taste

```bash
# Authenticate against a BTP subaccount (XSUAA service-key flow)
sapctl auth login --flow xsuaa --label trial \
  --client-id $CID --client-secret $CSECRET --token-url $TURL

# Pull a SOX-grade journal extract with signed audit chain
sapctl s4 audit-export \
  --cred trial \
  --use-case sox-journal \
  --from 2025-01-01 --to 2025-12-31 \
  --out ./evidence

# Hand the .tar.gz to an external auditor; they verify with just:
sapctl audit verify --chain chain.jsonl --pub ed25519.pub
```

---

## Repo layout

```
/apps/
  /cli/             Go: sapctl binary (Cobra + Viper)
  /docs/            Astro Starlight (docs.sapctl.dev)
  /web-static/      Marketing site (sapctl.dev)
/packages/
  /audit-chain/     ed25519 hash-chained JSONL library
  /sqlite-mirror/   FTS5 local mirror with watermarks
  /mcp-emitter/     MCP + A2A Agent Card generator
/clients/           Drop-in MCP configs for Claude Code/Desktop, Cursor, Cody, etc.
/docs/adr/          Architectural decision records
/specs/             OpenAPI specs harvested from SAP Business Accelerator Hub
/tests/e2e/         Shell-based end-to-end harnesses
```

---

## Contributing

See [CONTRIBUTING.md](CONTRIBUTING.md) for the workflow. Security disclosures: [SECURITY.md](SECURITY.md). Conduct: [CODE_OF_CONDUCT.md](CODE_OF_CONDUCT.md).

---

## License

Apache-2.0. See [LICENSE](LICENSE).

> Independent open-source project. Not affiliated with SAP SE.
