---
title: Use with Claude Desktop
description: Wire sapctl as an MCP server in Claude Desktop.
---

Open `~/Library/Application Support/Claude/claude_desktop_config.json`:

```json
{
  "mcpServers": {
    "sapctl": {
      "command": "/usr/local/bin/sapctl",
      "args": ["mcp", "serve"],
      "env": {
        "HOME": "/Users/you",
        "XDG_CONFIG_HOME": "/Users/you/.config"
      }
    }
  }
}
```

Restart Claude Desktop. Ask: *"list SAP S/4HANA services"* -- it invokes
`s4.catalog.discover` via MCP. See [generic config](https://github.com/dixitsheta/sapctl/blob/main/clients/generic.json)
for other clients.
