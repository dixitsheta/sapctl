package mcpemitter

import (
	"bytes"
	"context"
	"encoding/json"
	"strconv"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

// buildTestRoot returns a Cobra tree mirroring sapctl's structure for tests.
func buildTestRoot() *cobra.Command {
	root := &cobra.Command{Use: "sapctl"}

	version := &cobra.Command{
		Use:   "version",
		Short: "Print the version",
		RunE: func(cmd *cobra.Command, args []string) error {
			_, err := cmd.OutOrStdout().Write([]byte("0.0.0-test"))
			return err
		},
	}
	root.AddCommand(version)

	auth := &cobra.Command{Use: "auth", Short: "Auth"}
	authLogin := &cobra.Command{
		Use:   "login",
		Short: "Store credentials",
		RunE: func(cmd *cobra.Command, args []string) error {
			label, _ := cmd.Flags().GetString("label")
			_, err := cmd.OutOrStdout().Write([]byte("saved:" + label))
			return err
		},
	}
	authLogin.Flags().String("label", "", "credential label")
	authLogin.Flags().Bool("force", false, "overwrite existing")
	auth.AddCommand(authLogin)
	root.AddCommand(auth)

	s4 := &cobra.Command{Use: "s4", Short: "S/4HANA"}
	catalog := &cobra.Command{Use: "catalog", Short: "Catalog ops"}
	discover := &cobra.Command{
		Use:   "discover",
		Short: "Discover services",
		RunE: func(cmd *cobra.Command, args []string) error {
			top, _ := cmd.Flags().GetInt("top")
			_, _ = cmd.OutOrStdout().Write([]byte(`{"count":` + strconv.Itoa(top) + `}`))
			return nil
		},
	}
	discover.Flags().Int("top", 5, "max services")
	catalog.AddCommand(discover)
	s4.AddCommand(catalog)
	root.AddCommand(s4)

	// Excluded subcommand (should not appear in tools/list).
	root.AddCommand(&cobra.Command{Use: "mcp", Short: "should not appear", Run: func(*cobra.Command, []string) {}})

	return root
}

func TestListTools(t *testing.T) {
	s := NewServer(buildTestRoot())
	tools := s.ListTools()

	names := map[string]ToolDescriptor{}
	for _, td := range tools {
		names[td.Name] = td
	}

	want := []string{"version", "auth.login", "s4.catalog.discover"}
	for _, n := range want {
		if _, ok := names[n]; !ok {
			t.Errorf("missing tool %q. got: %v", n, keysOf(names))
		}
	}
	if _, ok := names["mcp"]; ok {
		t.Errorf("mcp subcommand should be excluded")
	}

	// auth.login should expose --label + --force in inputSchema.
	loginSchema := names["auth.login"].InputSchema
	props, ok := loginSchema["properties"].(map[string]any)
	if !ok {
		t.Fatalf("auth.login properties not map: %T", loginSchema["properties"])
	}
	if _, ok := props["label"]; !ok {
		t.Errorf("auth.login missing 'label' prop")
	}
	if force, ok := props["force"].(map[string]any); !ok || force["type"] != "boolean" {
		t.Errorf("auth.login force prop wrong: %+v", props["force"])
	}
}

func keysOf(m map[string]ToolDescriptor) []string {
	out := []string{}
	for k := range m {
		out = append(out, k)
	}
	return out
}

func TestCallTool(t *testing.T) {
	s := NewServer(buildTestRoot())
	ctx := context.Background()

	out, isErr, err := s.CallTool(ctx, "auth.login", map[string]any{"label": "foo"})
	if err != nil {
		t.Fatalf("CallTool: %v", err)
	}
	if isErr {
		t.Errorf("isError = true, output=%q", out)
	}
	if !strings.Contains(out, "saved:foo") {
		t.Errorf("output=%q", out)
	}

	out, _, _ = s.CallTool(ctx, "s4.catalog.discover", map[string]any{"top": 3})
	if !strings.Contains(out, `"count":3`) {
		t.Errorf("discover output=%q", out)
	}

	out, _, _ = s.CallTool(ctx, "version", nil)
	if !strings.Contains(out, "0.0.0-test") {
		t.Errorf("version output=%q", out)
	}
}

func TestServeInitialize(t *testing.T) {
	s := NewServer(buildTestRoot())
	in := strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"initialize"}` + "\n")
	var out bytes.Buffer

	if err := s.Serve(context.Background(), in, &out); err != nil {
		t.Fatalf("Serve: %v", err)
	}

	var resp map[string]any
	if err := json.Unmarshal(bytes.TrimSpace(out.Bytes()), &resp); err != nil {
		t.Fatalf("decode: %v\n%s", err, out.String())
	}
	if resp["jsonrpc"] != "2.0" {
		t.Fatalf("jsonrpc=%v", resp["jsonrpc"])
	}
	result, ok := resp["result"].(map[string]any)
	if !ok {
		t.Fatalf("no result: %+v", resp)
	}
	if result["protocolVersion"] != protocolVersion {
		t.Fatalf("protocolVersion=%v", result["protocolVersion"])
	}
}

func TestServeToolsListAndCall(t *testing.T) {
	s := NewServer(buildTestRoot())
	in := strings.NewReader(
		`{"jsonrpc":"2.0","id":1,"method":"tools/list"}` + "\n" +
			`{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"version","arguments":{}}}` + "\n",
	)
	var out bytes.Buffer

	if err := s.Serve(context.Background(), in, &out); err != nil {
		t.Fatalf("Serve: %v", err)
	}

	lines := bytes.Split(bytes.TrimSpace(out.Bytes()), []byte("\n"))
	if len(lines) != 2 {
		t.Fatalf("want 2 responses, got %d:\n%s", len(lines), out.String())
	}

	var listResp struct {
		Result struct {
			Tools []ToolDescriptor `json:"tools"`
		} `json:"result"`
	}
	if err := json.Unmarshal(lines[0], &listResp); err != nil {
		t.Fatalf("decode list: %v", err)
	}
	if len(listResp.Result.Tools) < 3 {
		t.Fatalf("expected >=3 tools, got %d", len(listResp.Result.Tools))
	}

	var callResp struct {
		Result struct {
			Content []struct {
				Type, Text string
			} `json:"content"`
			IsError bool `json:"isError"`
		} `json:"result"`
	}
	if err := json.Unmarshal(lines[1], &callResp); err != nil {
		t.Fatalf("decode call: %v\n%s", err, lines[1])
	}
	if callResp.Result.IsError {
		t.Fatalf("isError true: %+v", callResp.Result)
	}
	if !strings.Contains(callResp.Result.Content[0].Text, "0.0.0-test") {
		t.Fatalf("content=%q", callResp.Result.Content[0].Text)
	}
}

func TestServeMethodNotFound(t *testing.T) {
	s := NewServer(buildTestRoot())
	in := strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"bogus/method"}` + "\n")
	var out bytes.Buffer
	_ = s.Serve(context.Background(), in, &out)

	var resp map[string]any
	_ = json.Unmarshal(bytes.TrimSpace(out.Bytes()), &resp)
	errObj, ok := resp["error"].(map[string]any)
	if !ok {
		t.Fatalf("no error object: %+v", resp)
	}
	if errObj["code"].(float64) != -32601 {
		t.Fatalf("code=%v", errObj["code"])
	}
}

func TestServeParseError(t *testing.T) {
	s := NewServer(buildTestRoot())
	in := strings.NewReader("not json\n")
	var out bytes.Buffer
	_ = s.Serve(context.Background(), in, &out)

	var resp map[string]any
	_ = json.Unmarshal(bytes.TrimSpace(out.Bytes()), &resp)
	errObj := resp["error"].(map[string]any)
	if errObj["code"].(float64) != -32700 {
		t.Fatalf("code=%v", errObj["code"])
	}
}
