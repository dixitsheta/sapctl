# Local LLM — central GGUF cache

This repo uses `llama-swap` on `http://localhost:8079` for all LLM / vision /
embedding calls (OpenAI-compatible). Models live centrally at
`~/.cache/llama-models/` — see that dir's README for the full layout, aliases,
and add-model instructions.

## Quick reference

| Alias    | Concrete model       | Use case                                |
|----------|----------------------|------------------------------------------|
| `default` / `qwen3`   | qwen3-8b              | chat, enhance, plan      |
| `fast`                | qwen3-4b              | classify, field rewrites |
| `vision` / `moondream`| qwen2.5-vl-7b         | vision QA                |
| `embed`               | nomic-embed-text-v1.5 | embeddings (768 d)       |

## Endpoint

```bash
curl -s http://localhost:8079/v1/models
```

Env: set `LLM_BASE_URL=http://localhost:8079` (fallback for repos that still
read this var). `LLM_BACKEND=llamacpp` (default) or `ollama` for fallback.

If the daemon is not running:

```bash
launchctl load -w ~/Library/LaunchAgents/com.local.llama-swap.plist
launchctl list | grep llama-swap
```

Full doc: `~/.cache/llama-models/README.md`.
