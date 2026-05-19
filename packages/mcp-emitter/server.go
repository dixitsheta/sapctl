// Package mcpemitter walks a Cobra command tree and exposes every leaf
// command as a tool over the Model Context Protocol (MCP).
//
// Transport: line-delimited JSON-RPC 2.0 over stdin/stdout.
//
// Protocol methods implemented (subset of MCP 2024-11-05):
//   - initialize        : returns server capabilities + serverInfo
//   - tools/list        : enumerates all Cobra leaf commands as tools
//   - tools/call        : invokes a leaf command via in-process Cobra
//                         Execute; stdout is returned as a single text
//                         content item.
//   - ping              : returns empty response (liveness)
//
// Stderr is reserved for log output; it MUST NOT be used for protocol
// traffic.
package mcpemitter

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
	"sync"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

// protocolVersion is the MCP protocol version this server speaks.
const protocolVersion = "2024-11-05"

// JSON-RPC 2.0 message structures.
type rpcRequest struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

type rpcResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Result  any             `json:"result,omitempty"`
	Error   *rpcError       `json:"error,omitempty"`
}

type rpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    any    `json:"data,omitempty"`
}

// JSON-RPC standard error codes.
const (
	errParseError     = -32700
	errInvalidRequest = -32600
	errMethodNotFound = -32601
	errInvalidParams  = -32602
	errInternalError  = -32603
)

// ToolDescriptor is the MCP wire shape for a tool.
type ToolDescriptor struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	InputSchema map[string]any `json:"inputSchema"`
}

// Server holds the state of a running MCP server.
type Server struct {
	root *cobra.Command
	mu   sync.Mutex
	// excluded subcommand names that should NOT be exposed as MCP tools.
	// `mcp` is excluded to prevent recursion.
	excluded map[string]bool
}

// NewServer constructs a server around a built Cobra root.
// excludeNames are subcommand names to skip (e.g. "mcp", "completion", "help").
func NewServer(root *cobra.Command, excludeNames ...string) *Server {
	ex := map[string]bool{
		"help":       true,
		"completion": true,
		"mcp":        true,
	}
	for _, n := range excludeNames {
		ex[n] = true
	}
	return &Server{root: root, excluded: ex}
}

// Serve runs the JSON-RPC loop until in returns EOF or ctx is cancelled.
func (s *Server) Serve(ctx context.Context, in io.Reader, out io.Writer) error {
	scanner := bufio.NewScanner(in)
	scanner.Buffer(make([]byte, 1<<16), 1<<20)

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
		if !scanner.Scan() {
			if err := scanner.Err(); err != nil {
				return err
			}
			return nil // EOF
		}
		line := bytes.TrimSpace(scanner.Bytes())
		if len(line) == 0 {
			continue
		}
		s.dispatch(ctx, line, out)
	}
}

func (s *Server) dispatch(ctx context.Context, line []byte, out io.Writer) {
	var req rpcRequest
	if err := json.Unmarshal(line, &req); err != nil {
		s.writeError(out, nil, errParseError, "parse error", err.Error())
		return
	}
	if req.JSONRPC != "2.0" {
		s.writeError(out, req.ID, errInvalidRequest, "jsonrpc must be 2.0", nil)
		return
	}

	switch req.Method {
	case "initialize":
		s.writeResult(out, req.ID, map[string]any{
			"protocolVersion": protocolVersion,
			"capabilities": map[string]any{
				"tools": map[string]any{},
			},
			"serverInfo": map[string]any{
				"name":    "sapctl-mcp",
				"version": "0.1.0",
			},
		})

	case "ping":
		s.writeResult(out, req.ID, map[string]any{})

	case "tools/list":
		tools := s.ListTools()
		s.writeResult(out, req.ID, map[string]any{"tools": tools})

	case "tools/call":
		var params struct {
			Name      string         `json:"name"`
			Arguments map[string]any `json:"arguments"`
		}
		if err := json.Unmarshal(req.Params, &params); err != nil {
			s.writeError(out, req.ID, errInvalidParams, "invalid params", err.Error())
			return
		}
		result, isError, err := s.CallTool(ctx, params.Name, params.Arguments)
		if err != nil {
			s.writeError(out, req.ID, errInternalError, err.Error(), nil)
			return
		}
		s.writeResult(out, req.ID, map[string]any{
			"content": []map[string]any{
				{"type": "text", "text": result},
			},
			"isError": isError,
		})

	default:
		s.writeError(out, req.ID, errMethodNotFound, "method not found: "+req.Method, nil)
	}
}

func (s *Server) writeResult(out io.Writer, id json.RawMessage, result any) {
	resp := rpcResponse{JSONRPC: "2.0", ID: id, Result: result}
	b, _ := json.Marshal(resp)
	s.writeLine(out, b)
}

func (s *Server) writeError(out io.Writer, id json.RawMessage, code int, msg string, data any) {
	resp := rpcResponse{
		JSONRPC: "2.0", ID: id,
		Error: &rpcError{Code: code, Message: msg, Data: data},
	}
	b, _ := json.Marshal(resp)
	s.writeLine(out, b)
}

func (s *Server) writeLine(out io.Writer, b []byte) {
	s.mu.Lock()
	defer s.mu.Unlock()
	_, _ = out.Write(b)
	_, _ = out.Write([]byte{'\n'})
}

// ListTools walks the Cobra tree and returns every runnable leaf as a tool.
// Tool name format: dot-joined subcommand path under root, e.g.
//
//	sapctl auth login            -> "auth.login"
//	sapctl s4 catalog discover   -> "s4.catalog.discover"
func (s *Server) ListTools() []ToolDescriptor {
	var tools []ToolDescriptor
	s.walk(s.root, nil, &tools)
	return tools
}

func (s *Server) walk(c *cobra.Command, parents []string, out *[]ToolDescriptor) {
	if c == nil {
		return
	}
	if s.excluded[c.Name()] {
		return
	}

	// If this command is runnable AND not the root, register it as a tool.
	if c.Runnable() && len(parents) > 0 {
		desc := c.Short
		if desc == "" {
			desc = c.Long
		}
		*out = append(*out, ToolDescriptor{
			Name:        strings.Join(parents, "."),
			Description: desc,
			InputSchema: buildInputSchema(c),
		})
	}

	for _, child := range c.Commands() {
		if s.excluded[child.Name()] {
			continue
		}
		s.walk(child, append(parents, child.Name()), out)
	}
}

// buildInputSchema converts a Cobra command's flags + positional args into a
// JSON Schema (draft-07-ish) object suitable for MCP tool inputSchema.
func buildInputSchema(c *cobra.Command) map[string]any {
	props := map[string]any{}

	addFlag := func(f *pflag.Flag) {
		if f.Hidden {
			return
		}
		schema := map[string]any{
			"description": f.Usage,
		}
		switch f.Value.Type() {
		case "bool":
			schema["type"] = "boolean"
		case "int", "int8", "int16", "int32", "int64",
			"uint", "uint8", "uint16", "uint32", "uint64":
			schema["type"] = "integer"
		case "float32", "float64":
			schema["type"] = "number"
		default:
			schema["type"] = "string"
		}
		props[f.Name] = schema
	}

	c.LocalFlags().VisitAll(addFlag)
	c.InheritedFlags().VisitAll(addFlag)

	if strings.Contains(c.Use, "<") {
		props["args"] = map[string]any{
			"type":        "array",
			"items":       map[string]any{"type": "string"},
			"description": "positional arguments",
		}
	}

	return map[string]any{
		"type":       "object",
		"properties": props,
	}
}

// CallTool resolves a dotted tool name back to a Cobra command and executes
// it in-process. stdout is captured and returned as the tool result. Any
// error surfaces in isError=true; stdout is still returned for diagnostics.
func (s *Server) CallTool(ctx context.Context, name string, args map[string]any) (string, bool, error) {
	parts := strings.Split(name, ".")
	if len(parts) == 0 || (len(parts) == 1 && parts[0] == "") {
		return "", true, errors.New("empty tool name")
	}

	cliArgs := append([]string{}, parts...)
	var positional []string
	for k, v := range args {
		if k == "args" {
			if arr, ok := v.([]any); ok {
				for _, a := range arr {
					positional = append(positional, fmt.Sprint(a))
				}
			}
			continue
		}
		switch vv := v.(type) {
		case bool:
			if vv {
				cliArgs = append(cliArgs, "--"+k)
			}
		default:
			cliArgs = append(cliArgs, "--"+k, fmt.Sprint(vv))
		}
	}
	cliArgs = append(cliArgs, positional...)

	var stdout, stderr bytes.Buffer
	s.root.SetOut(&stdout)
	s.root.SetErr(&stderr)
	s.root.SetArgs(cliArgs)
	err := s.root.ExecuteContext(ctx)
	if err != nil {
		return stdout.String() + stderr.String(), true, nil
	}
	return stdout.String(), false, nil
}
