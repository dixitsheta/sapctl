package cmd

import (
	"os"

	"github.com/spf13/cobra"

	"github.com/dixitsheta/sapctl/apps/cli/internal/errs"
	mcpemitter "github.com/dixitsheta/sapctl/packages/mcp-emitter"
)

func newMCPCmd() *cobra.Command {
	c := &cobra.Command{
		Use:   "mcp",
		Short: "Model Context Protocol (MCP) server",
	}
	c.AddCommand(newMCPServeCmd())
	c.AddCommand(newMCPListToolsCmd())
	return c
}

func newMCPServeCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "serve",
		Short: "Run sapctl as an MCP server over stdio",
		Long: `Run sapctl as a Model Context Protocol server. Every sapctl
sub-command is exposed as a tool. Protocol: line-delimited JSON-RPC 2.0 over
stdin/stdout. Stderr is reserved for log lines and MUST NOT be redirected
into the client.

Example (Claude Desktop / MCP-compatible client config):
  {
    "mcpServers": {
      "sapctl": {
        "command": "/path/to/sapctl",
        "args": ["mcp", "serve"]
      }
    }
  }`,
		RunE: func(cmd *cobra.Command, args []string) error {
			// Build a fresh root for the server so flag state from the outer
			// Cobra invocation does not bleed into tool calls.
			root := NewRootCmd(rootVersion())
			srv := mcpemitter.NewServer(root)
			if err := srv.Serve(cmd.Context(), os.Stdin, os.Stdout); err != nil {
				return errs.Wrap(errs.ExitUserError, "mcp.serve", "mcp server stopped", err)
			}
			return nil
		},
	}
}

// rootVersion returns the version baked into the binary. Stashed here so the
// MCP server can label itself correctly when invoked as a subcommand.
func rootVersion() string { return "0.0.0-dev" }

func newMCPListToolsCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list-tools",
		Short: "Print the MCP tool descriptors that `mcp serve` would expose",
		RunE: func(cmd *cobra.Command, args []string) error {
			root := NewRootCmd(rootVersion())
			srv := mcpemitter.NewServer(root)
			tools := srv.ListTools()
			return writeJSON(cmd, map[string]any{
				"count": len(tools),
				"tools": tools,
			})
		},
	}
}
