---
title: Use with Ollama (mcphost bridge)
description: Expose sapctl tools to local Ollama models.
---

Ollama has no native MCP. Use [mcphost](https://github.com/mark3labs/mcphost):

```bash
brew install mcphost
cat > ~/.mcp.json <<'JSON'
{
  "mcpServers": {
    "sapctl": {
      "command": "/usr/local/bin/sapctl",
      "args": ["mcp", "serve"]
    }
  }
}
JSON

mcphost -m ollama:llama3.2:latest
```

Tool-calling models (llama3.2, qwen2.5-coder, mistral-nemo) work best.
