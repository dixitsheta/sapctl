# sapctl MCP client configs

`sapctl mcp serve` is a standard MCP (Model Context Protocol) server speaking
line-delimited JSON-RPC 2.0 over stdio. It exposes every `sapctl`
sub-command as a tool (currently 32+ tools).

This directory contains drop-in configuration snippets for the major MCP
clients and several local-model bridges.

## Binary path

All snippets assume sapctl lives at:

```
/usr/local/bin/sapctl
```

Adjust the `command` field to your install location. Or symlink into PATH:

```bash
sudo ln -sf /usr/local/bin/sapctl /usr/local/bin/sapctl
```

## Environment hygiene

When invoked as a subprocess (which is how every MCP client spawns
sapctl), `HOME` may be different from your interactive shell. Always pass
through:

```json
"env": {
  "HOME": "${HOME}",
  "XDG_CONFIG_HOME": "${HOME}/.config",
  "PATH": "/usr/local/bin:/usr/bin:/bin"
}
```

Optional toggles:

| Env var | Effect |
|---|---|
| `SAPCTL_AUDIT=1` | Append signed event per HTTP call (requires `sapctl audit init`) |
| `SAPCTL_TRACE=1` | Emit OTLP-shaped JSON span on stderr per HTTP attempt |
| `SAPCTL_INTEGRATION=1` | Allow integration tests to hit a real tenant |

## Per-client files in this directory

| Client | File | Notes |
|---|---|---|
| Hermes | `hermes.json` | local agent runtime |
| Claude Desktop | `claude-desktop.json` | Anthropic native app |
| Claude Code (CLI) | `claude-code.json` | terminal-native MCP client |
| Cursor | `cursor.json` | VS-Code fork w/ MCP support |
| Continue.dev | `continue.yaml` | VS-Code/JetBrains extension |
| Cody | `cody.json` | Sourcegraph IDE assistant |
| Codeium / Windsurf | `codeium-windsurf.json` | Codeium native app |
| LM Studio | `lm-studio.json` | Local model host with MCP |
| Ollama (via bridge) | `ollama-bridge.md` | Ollama has no native MCP; use bridge |
| Generic stdio | `generic.json` | Minimal portable form |

## Quick test (no client)

```bash
printf '%s\n%s\n' \
  '{"jsonrpc":"2.0","id":1,"method":"initialize"}' \
  '{"jsonrpc":"2.0","id":2,"method":"tools/list"}' \
  | sapctl mcp serve | jq
```

Two JSON-RPC responses on stdout. Stderr reserved for log lines and MUST
NOT be redirected into the client.

## Verification

After wiring a client, ask it: *"list S/4HANA services"*. It should call the
`s4.catalog.discover` tool and return a list of OData service IDs. If you
get `401 Invalid ApiKey`, refresh your sandbox key at
https://api.sap.com/settings.
