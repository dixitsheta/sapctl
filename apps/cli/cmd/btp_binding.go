package cmd

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"

	"github.com/spf13/cobra"

	"github.com/dixitsheta/sapctl/apps/cli/internal/errs"
	"github.com/dixitsheta/sapctl/apps/cli/internal/sap"
)

type btpServiceBinding struct {
	ID                string         `json:"id"`
	Name              string         `json:"name"`
	ServiceInstanceID string         `json:"service_instance_id"`
	Credentials       map[string]any `json:"credentials,omitempty"`
	Parameters        map[string]any `json:"parameters,omitempty"`
	BindResource      map[string]any `json:"bind_resource,omitempty"`
	Ready             bool           `json:"ready"`
}

type btpServiceBindingList struct {
	Items []btpServiceBinding `json:"items"`
}

func newBTPServiceBindingCmd() *cobra.Command {
	c := &cobra.Command{
		Use:   "service-binding",
		Short: "Manage BTP service bindings (OSB v2)",
	}
	c.AddCommand(newBTPSBListCmd())
	c.AddCommand(newBTPSBGetCmd())
	c.AddCommand(newBTPSBCreateCmd())
	c.AddCommand(newBTPSBDeleteCmd())
	return c
}

func newBTPSBListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List service bindings",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, cancel := context.WithTimeout(cmd.Context(), 30*time.Second)
			defer cancel()
			client, err := newBTPClient()
			if err != nil {
				return err
			}
			var resp btpServiceBindingList
			if err := client.Get(ctx, "/v2/service_bindings", nil, &resp); err != nil {
				return err
			}
			return emitBTPBindings(cmd, resp.Items)
		},
	}
}

func newBTPSBGetCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "get <id>",
		Short: "Show a single service binding (includes credentials)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, cancel := context.WithTimeout(cmd.Context(), 30*time.Second)
			defer cancel()
			client, err := newBTPClient()
			if err != nil {
				return err
			}
			var b btpServiceBinding
			if err := client.Get(ctx, "/v2/service_bindings/"+url.PathEscape(args[0]), nil, &b); err != nil {
				return err
			}
			return emitBTPBindings(cmd, []btpServiceBinding{b})
		},
	}
}

type btpSBCreateFlags struct {
	name       string
	instanceID string
	parameters string
}

func newBTPSBCreateCmd() *cobra.Command {
	var f btpSBCreateFlags
	c := &cobra.Command{
		Use:   "create",
		Short: "Create a service binding (returns credentials)",
		Long: `Create a new OSB v2 service binding under an existing service instance.
The response includes the credentials JSON, which can be piped to
` + "`sapctl auth login --flow xsuaa`" + ` to immediately consume the binding.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, cancel := context.WithTimeout(cmd.Context(), 60*time.Second)
			defer cancel()
			client, err := newBTPClient()
			if err != nil {
				return err
			}

			body := map[string]any{
				"name":                f.name,
				"service_instance_id": f.instanceID,
			}
			if f.parameters != "" {
				var p any
				if err := json.Unmarshal([]byte(f.parameters), &p); err != nil {
					return errs.Wrap(errs.ExitUserError, "btp.binding.create.params",
						"--parameters must be valid JSON", err)
				}
				body["parameters"] = p
			}
			payload, _ := json.Marshal(body)

			var resp btpServiceBinding
			if err := postJSON(ctx, client, "/v2/service_bindings", payload, &resp); err != nil {
				return err
			}
			return emitBTPBindings(cmd, []btpServiceBinding{resp})
		},
	}
	c.Flags().StringVar(&f.name, "name", "", "binding name (required)")
	c.Flags().StringVar(&f.instanceID, "instance-id", "", "service-instance ID to bind (required)")
	c.Flags().StringVar(&f.parameters, "parameters", "", "binding parameters as JSON (optional)")
	_ = c.MarkFlagRequired("name")
	_ = c.MarkFlagRequired("instance-id")
	return c
}

func newBTPSBDeleteCmd() *cobra.Command {
	var yes bool
	c := &cobra.Command{
		Use:   "delete <id>",
		Short: "Delete a service binding",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if !yes && !globalFlags.Yes {
				return errs.New(errs.ExitUserError, "btp.binding.delete.confirm",
					"delete is destructive; pass --yes to confirm")
			}
			ctx, cancel := context.WithTimeout(cmd.Context(), 60*time.Second)
			defer cancel()
			client, err := newBTPClient()
			if err != nil {
				return err
			}
			if err := deleteJSON(ctx, client, "/v2/service_bindings/"+url.PathEscape(args[0])); err != nil {
				return err
			}
			if globalFlags.JSON {
				return writeJSON(cmd, map[string]any{"id": args[0], "status": "deleted"})
			}
			fmt.Fprintf(cmd.OutOrStdout(), "deleted binding %s\n", args[0])
			return nil
		},
	}
	c.Flags().BoolVar(&yes, "yes", false, "confirm destructive operation")
	return c
}

// postJSON issues a POST with a JSON body and decodes the response. Reuses
// sap.Client's auth provider but bypasses Client.do because OSB POST has no
// retry semantics (idempotency keys not yet wired).
func postJSON(ctx context.Context, c *sap.Client, path string, body []byte, out any) error {
	u := c.BaseURL + path
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, u, bytes.NewReader(body))
	if err != nil {
		return errs.Wrap(errs.ExitUserError, "btp.post.request", "build request", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", c.UserAgent)
	if c.Auth != nil {
		if err := c.Auth.Apply(ctx, req); err != nil {
			return err
		}
	}
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return errs.Wrap(errs.ExitUserError, "btp.post.network", "transport error", err)
	}
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(resp.Body)
	switch {
	case resp.StatusCode == http.StatusUnauthorized:
		return errs.New(errs.ExitAuth, "btp.post.unauthorized",
			fmt.Sprintf("HTTP 401: %s", truncStr(string(respBody), 200)))
	case resp.StatusCode == http.StatusConflict:
		return errs.New(errs.ExitConflict, "btp.post.conflict",
			fmt.Sprintf("HTTP 409: %s", truncStr(string(respBody), 200)))
	case resp.StatusCode < 200 || resp.StatusCode >= 300:
		return errs.New(errs.ExitUserError, "btp.post.http",
			fmt.Sprintf("HTTP %d: %s", resp.StatusCode, truncStr(string(respBody), 200)))
	}
	if out == nil {
		return nil
	}
	return json.Unmarshal(respBody, out)
}

func deleteJSON(ctx context.Context, c *sap.Client, path string) error {
	u := c.BaseURL + path
	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, u, nil)
	if err != nil {
		return errs.Wrap(errs.ExitUserError, "btp.delete.request", "build request", err)
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", c.UserAgent)
	if c.Auth != nil {
		if err := c.Auth.Apply(ctx, req); err != nil {
			return err
		}
	}
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return errs.Wrap(errs.ExitUserError, "btp.delete.network", "transport error", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	switch {
	case resp.StatusCode == http.StatusNotFound:
		return errs.New(errs.ExitNotFound, "btp.delete.not_found", "binding not found")
	case resp.StatusCode == http.StatusUnauthorized:
		return errs.New(errs.ExitAuth, "btp.delete.unauthorized", "HTTP 401")
	case resp.StatusCode < 200 || resp.StatusCode >= 300:
		return errs.New(errs.ExitUserError, "btp.delete.http",
			fmt.Sprintf("HTTP %d: %s", resp.StatusCode, truncStr(string(body), 200)))
	}
	return nil
}

func truncStr(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}

func emitBTPBindings(cmd *cobra.Command, items []btpServiceBinding) error {
	enc := json.NewEncoder(cmd.OutOrStdout())
	if !globalFlags.Compact {
		enc.SetIndent("", "  ")
	}
	return enc.Encode(map[string]any{"count": len(items), "service_bindings": items})
}
