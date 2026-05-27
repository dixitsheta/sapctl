> **Model availability update (2026-05-26):** Names below are reference-only. These local copies have been removed from this machine: `llama3.3:70b`, `qwen2.5:14b`, `qwen2.5-coder:14b`, `qwen2.5:7b`, `qwen2.5-coder:7b`, `hermes3:8b`, `gemma3:4b`, `gemma3:latest`, `gemma4:26b`, `gemma4:e2b`, `phi4-mini`, `llava-phi3`, `llava-llama3`, `qwen3:4b`, `qwen3:1.7b`, `granite3.2-vision`, `llama3.2:3b`, `llama3.2`, `llama3.1:8b`, plus LM Studio bundles `Qwen3-Coder-Next-MLX`, `Qwen3.5-35B-A3B`, `Qwen2.5-Coder-14B-MLX`, `gpt-oss-20b-MXFP4-Q8`, `rnj-1-instruct`, `gemma-4-E4B-it-MLX`, `Qwen3.5-9B-MLX`, `Meta-Llama-3.1-8B`, `DeepSeek-R1-0528-Qwen3-8B-MLX`, `Qwen3.6-27B-Q4_K_M`, `Qwen3.6-27B-Q6_K`, `Llama-3.2-3B`, `Llama-3.2-1B`. Standard llm-server now lives at `http://localhost:8079` (llama-swap, OpenAI-compat); use aliases `default` (qwen3-8b) / `fast` (qwen3-4b) / `vision` (qwen2.5-vl-7b) / `embed` (nomic-embed). Central GGUF cache: `~/.cache/llama-models/`. Re-download from HF if you need any specific deleted model.

# Ollama + sapctl

Ollama itself does not implement MCP. Use a bridge that exposes MCP tools to
Ollama's tool-calling API.

## Option 1 -- `mcphost` (recommended)

[`mcphost`](https://github.com/mark3labs/mcphost) connects any LLM provider
(Ollama, Anthropic, OpenAI) to one or more MCP servers.

### Install

```bash
go install github.com/mark3labs/mcphost@latest
# or
brew install mcphost
```

### Configure

Create `~/.mcp.json`:

```json
{
  "mcpServers": {
    "sapctl": {
      "command": "/usr/local/bin/sapctl",
      "args": ["mcp", "serve"],
      "env": {
        "HOME": "${HOME}",
        "XDG_CONFIG_HOME": "${HOME}/.config",
        "PATH": "/usr/local/bin:/usr/bin:/bin"
      }
    }
  }
}
```

### Run

```bash
mcphost -m ollama:llama3.2:latest
mcphost -m ollama:qwen2.5-coder:7b
mcphost -m ollama:mistral-nemo:latest
```

Ask: *"list S/4HANA services"* -- mcphost routes the tool call through sapctl.

## Option 2 -- `mcp-bridge` HTTP proxy

For HTTP-only clients, run sapctl behind an SSE bridge:

```bash
go install github.com/SecretiveShell/MCP-Bridge@latest
mcp-bridge \
  --listen :8765 \
  --server '/usr/local/bin/sapctl mcp serve'
```

Now any tool-aware HTTP client (Open WebUI, AnythingLLM, custom apps) can call:

```
POST http://localhost:8765/tools/call
{"name": "s4.catalog.discover", "arguments": {"cred": "sandbox", "top": 5}}
```

## Option 3 -- Open WebUI (via mcpo)

[`mcpo`](https://github.com/open-webui/mcpo) wraps an MCP server as a
plain OpenAPI HTTP server, which Open WebUI consumes natively.

```bash
pipx install mcpo
mcpo --port 8000 -- /usr/local/bin/sapctl mcp serve
```

In Open WebUI -> Admin -> Tools -> add server at
`http://localhost:8000/openapi.json`.

## Recommended local models with tool-calling support

| Model | Tool calling | Size | Notes |
|---|---|---|---|
| `llama3.2:latest` | yes | ~2 GB | Best balance for laptops |
| `llama3.1:8b` | yes | ~5 GB | More accurate, slower |
| `qwen2.5-coder:7b` | yes | ~5 GB | Strong on code tasks |
| `mistral-nemo:latest` | yes | ~7 GB | Strong reasoning |
| `firefunction-v2:latest` | yes | ~26 GB | Tool-calling specialist |

## Troubleshooting

- **401 from sapctl through bridge:** Ollama spawns sapctl with a sparse env.
  Make sure `HOME` is exported in the bridge process or in the `env:` block
  of `~/.mcp.json` so sapctl can find `~/.config/sapctl/tokens.json`.
- **Tools not appearing:** confirm sapctl runs: `sapctl mcp list-tools` --
  should print >= 30 tools. If 0, bridge isn't spawning sapctl.
