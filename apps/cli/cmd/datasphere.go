package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"time"

	"github.com/spf13/cobra"

	"github.com/dixitsheta/sapctl/apps/cli/internal/auth"
	"github.com/dixitsheta/sapctl/apps/cli/internal/errs"
	"github.com/dixitsheta/sapctl/apps/cli/internal/sap"
)

type dsFlags struct {
	credLabel string
	apiBase   string
}

var dsShared dsFlags

func newDatasphereCmd() *cobra.Command {
	c := &cobra.Command{
		Use:   "datasphere",
		Short: "Interact with SAP Datasphere (Consumption API)",
		Long: `Commands targeting the SAP Datasphere Consumption API.

Authenticate with the Datasphere service-key XSUAA binding:

  sapctl auth login --flow xsuaa --label ds-trial \
    --client-id <ID> --client-secret <SECRET> \
    --token-url https://<tenant>.authentication.eu10.hana.ondemand.com/oauth/token

Then:

  sapctl datasphere space list --cred ds-trial \
    --api https://<tenant>.eu10.hcs.cloud.sap`,
	}
	c.PersistentFlags().StringVar(&dsShared.credLabel, "cred", "", "stored credential label (see `sapctl auth list`)")
	c.PersistentFlags().StringVar(&dsShared.apiBase, "api", "", "Datasphere tenant API base URL (required)")
	c.AddCommand(newDSSpaceCmd())
	c.AddCommand(newDSReplicationFlowCmd())
	c.AddCommand(newDSSQLCmd())
	return c
}

func newDSClient() (*sap.Client, error) {
	if dsShared.credLabel == "" {
		return nil, errs.New(errs.ExitUserError, "datasphere.cred_required",
			"--cred is required; run `sapctl auth login --flow xsuaa` first")
	}
	if dsShared.apiBase == "" {
		return nil, errs.New(errs.ExitUserError, "datasphere.api_required",
			"--api is required; pass tenant Datasphere API base URL")
	}
	cfgPath, err := auth.DefaultPath()
	if err != nil {
		return nil, errs.Wrap(errs.ExitUserError, "datasphere.cache_path", "resolve cache path", err)
	}
	cache := auth.NewCache(cfgPath)
	cfg, err := cache.Load(dsShared.credLabel)
	if err != nil {
		return nil, err
	}
	if cfg.Provider != "xsuaa" {
		return nil, errs.New(errs.ExitUserError, "datasphere.cred_wrong_flow",
			fmt.Sprintf("credential %q has flow=%s; expected flow=xsuaa", cfg.Label, cfg.Provider))
	}
	provider, err := auth.New(cfg)
	if err != nil {
		return nil, errs.Wrap(errs.ExitUserError, "datasphere.provider", "build provider", err)
	}
	client := sap.New(dsShared.apiBase, provider)
	if auditEnabled() {
		client.Audit = NewSAPClientAuditor()
	}
	if t := sap.NewEnvTracer(); t != nil {
		client.Tracer = t
	}
	return client, nil
}

type dsSpace struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	DisplayName string `json:"display_name,omitempty"`
	Description string `json:"description,omitempty"`
	State       string `json:"state,omitempty"`
}

type dsSpaceList struct {
	Value []dsSpace `json:"value"`
}

func newDSSpaceCmd() *cobra.Command {
	c := &cobra.Command{
		Use:   "space",
		Short: "Manage Datasphere spaces",
	}
	c.AddCommand(newDSSpaceListCmd())
	c.AddCommand(newDSSpaceGetCmd())
	return c
}

func newDSSpaceListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List spaces in the tenant",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, cancel := context.WithTimeout(cmd.Context(), 30*time.Second)
			defer cancel()
			client, err := newDSClient()
			if err != nil {
				return err
			}
			var resp dsSpaceList
			if err := client.Get(ctx, "/api/v1/dwc/consumption/spaces", nil, &resp); err != nil {
				return err
			}
			return emitDSSpaces(cmd, resp.Value)
		},
	}
}

func newDSSpaceGetCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "get <id>",
		Short: "Show a single space",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, cancel := context.WithTimeout(cmd.Context(), 30*time.Second)
			defer cancel()
			client, err := newDSClient()
			if err != nil {
				return err
			}
			var s dsSpace
			if err := client.Get(ctx, "/api/v1/dwc/consumption/spaces/"+url.PathEscape(args[0]), nil, &s); err != nil {
				return err
			}
			return emitDSSpaces(cmd, []dsSpace{s})
		},
	}
}

func emitDSSpaces(cmd *cobra.Command, items []dsSpace) error {
	if globalFlags.JSON {
		enc := json.NewEncoder(cmd.OutOrStdout())
		if !globalFlags.Compact {
			enc.SetIndent("", "  ")
		}
		return enc.Encode(map[string]any{"count": len(items), "spaces": items})
	}
	if len(items) == 0 {
		fmt.Fprintln(cmd.OutOrStdout(), "(no spaces)")
		return nil
	}
	fmt.Fprintf(cmd.OutOrStdout(), "%-36s %-30s %s\n", "ID", "NAME", "STATE")
	for _, s := range items {
		fmt.Fprintf(cmd.OutOrStdout(), "%-36s %-30s %s\n",
			truncateCol(s.ID, 36), truncateCol(displayName(s), 30), s.State)
	}
	return nil
}

func displayName(s dsSpace) string {
	if s.DisplayName != "" {
		return s.DisplayName
	}
	return s.Name
}

type dsReplicationFlow struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Source    string `json:"source,omitempty"`
	Target    string `json:"target,omitempty"`
	State     string `json:"state,omitempty"`
	LastRunAt string `json:"last_run_at,omitempty"`
}

type dsReplicationFlowList struct {
	Value []dsReplicationFlow `json:"value"`
}

func newDSReplicationFlowCmd() *cobra.Command {
	c := &cobra.Command{
		Use:   "replication-flow",
		Short: "Manage Datasphere replication flows",
	}
	c.AddCommand(&cobra.Command{
		Use:   "list",
		Short: "List replication flows",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, cancel := context.WithTimeout(cmd.Context(), 30*time.Second)
			defer cancel()
			client, err := newDSClient()
			if err != nil {
				return err
			}
			var resp dsReplicationFlowList
			if err := client.Get(ctx, "/api/v1/dwc/replication-flows", nil, &resp); err != nil {
				return err
			}
			if globalFlags.JSON {
				return writeJSON(cmd, map[string]any{"count": len(resp.Value), "replication_flows": resp.Value})
			}
			if len(resp.Value) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "(no replication flows)")
				return nil
			}
			fmt.Fprintf(cmd.OutOrStdout(), "%-36s %-30s %s\n", "ID", "NAME", "STATE")
			for _, r := range resp.Value {
				fmt.Fprintf(cmd.OutOrStdout(), "%-36s %-30s %s\n",
					truncateCol(r.ID, 36), truncateCol(r.Name, 30), r.State)
			}
			return nil
		},
	})
	return c
}

type dsSQLFlags struct {
	spaceID string
	query   string
}

func newDSSQLCmd() *cobra.Command {
	var f dsSQLFlags
	c := &cobra.Command{
		Use:   "sql",
		Short: "Execute SQL against a Datasphere space (Consumption SQL endpoint)",
	}
	exec := &cobra.Command{
		Use:   "exec",
		Short: "Run a SQL statement (--query) against --space and stream the result",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, cancel := context.WithTimeout(cmd.Context(), 5*time.Minute)
			defer cancel()
			if f.spaceID == "" || f.query == "" {
				return errs.New(errs.ExitUserError, "datasphere.sql.args",
					"--space and --query are required")
			}
			client, err := newDSClient()
			if err != nil {
				return err
			}
			body, _ := json.Marshal(map[string]string{"query": f.query})
			var resp json.RawMessage
			if err := postJSON(ctx, client,
				"/api/v1/dwc/consumption/spaces/"+url.PathEscape(f.spaceID)+"/sql",
				body, &resp); err != nil {
				return err
			}
			enc := json.NewEncoder(cmd.OutOrStdout())
			if !globalFlags.Compact {
				enc.SetIndent("", "  ")
			}
			return enc.Encode(resp)
		},
	}
	exec.Flags().StringVar(&f.spaceID, "space", "", "space ID (required)")
	exec.Flags().StringVar(&f.query, "query", "", "SQL statement to execute (required)")
	_ = exec.MarkFlagRequired("space")
	_ = exec.MarkFlagRequired("query")
	c.AddCommand(exec)
	return c
}
