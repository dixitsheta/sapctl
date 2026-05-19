package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"
	"time"

	"github.com/spf13/cobra"

	"github.com/dixitsheta/sapctl/apps/cli/internal/auth"
	"github.com/dixitsheta/sapctl/apps/cli/internal/errs"
	"github.com/dixitsheta/sapctl/apps/cli/internal/sap"
)

// defaultAccountsServiceBase is the SAP BTP Cloud Management / Accounts Service
// base for trial in EU10. Override with --api.
const defaultAccountsServiceBase = "https://accounts-service.cfapps.eu10.hana.ondemand.com"

type btpFlags struct {
	credLabel string
	apiBase   string
}

var btpShared btpFlags

func newBTPCmd() *cobra.Command {
	c := &cobra.Command{
		Use:   "btp",
		Short: "Interact with SAP BTP (Cloud Management / Accounts Service)",
		Long: `Commands targeting the SAP BTP Cloud Management API.

Authenticate first with an XSUAA service-key binding:

  sapctl auth login --flow xsuaa --label btp-trial \
    --client-id <ID> --client-secret <SECRET> \
    --token-url https://<subdomain>.authentication.eu10.hana.ondemand.com/oauth/token

Then:

  sapctl btp subaccount list --cred btp-trial`,
	}
	c.PersistentFlags().StringVar(&btpShared.credLabel, "cred", "", "stored credential label (see `sapctl auth list`)")
	c.PersistentFlags().StringVar(&btpShared.apiBase, "api", "", "override API base URL (default: "+defaultAccountsServiceBase+")")
	c.AddCommand(newBTPSubaccountCmd())
	c.AddCommand(newBTPServiceInstanceCmd())
	c.AddCommand(newBTPServiceBindingCmd())
	return c
}

func newBTPClient() (*sap.Client, error) {
	if btpShared.credLabel == "" {
		return nil, errs.New(errs.ExitUserError, "btp.cred_required",
			"--cred is required; run `sapctl auth login --flow xsuaa` first")
	}
	cfgPath, err := auth.DefaultPath()
	if err != nil {
		return nil, errs.Wrap(errs.ExitUserError, "btp.cache_path", "resolve cache path", err)
	}
	cache := auth.NewCache(cfgPath)
	cfg, err := cache.Load(btpShared.credLabel)
	if err != nil {
		return nil, err
	}
	if cfg.Provider != "xsuaa" {
		return nil, errs.New(errs.ExitUserError, "btp.cred_wrong_flow",
			fmt.Sprintf("credential %q has flow=%s; expected flow=xsuaa", cfg.Label, cfg.Provider))
	}
	provider, err := auth.New(cfg)
	if err != nil {
		return nil, errs.Wrap(errs.ExitUserError, "btp.provider", "build provider", err)
	}

	base := btpShared.apiBase
	if base == "" {
		base = defaultAccountsServiceBase
	}
	client := sap.New(base, provider)
	if auditEnabled() {
		client.Audit = NewSAPClientAuditor()
	}
	if t := sap.NewEnvTracer(); t != nil {
		client.Tracer = t
	}
	return client, nil
}

// ---- subaccount ----

type btpSubaccount struct {
	GUID          string `json:"guid"`
	TechnicalName string `json:"technicalName"`
	DisplayName   string `json:"displayName"`
	Subdomain     string `json:"subdomain"`
	Region        string `json:"region"`
	BetaEnabled   bool   `json:"betaEnabled"`
	State         string `json:"state"`
	StateMessage  string `json:"stateMessage,omitempty"`
}

type btpSubaccountList struct {
	Value []btpSubaccount `json:"value"`
}

func newBTPSubaccountCmd() *cobra.Command {
	c := &cobra.Command{
		Use:   "subaccount",
		Short: "Manage BTP subaccounts",
	}
	c.AddCommand(newBTPSubaccountListCmd())
	c.AddCommand(newBTPSubaccountGetCmd())
	return c
}

func newBTPSubaccountListCmd() *cobra.Command {
	var top int
	c := &cobra.Command{
		Use:   "list",
		Short: "List subaccounts in the global account",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, cancel := context.WithTimeout(cmd.Context(), 30*time.Second)
			defer cancel()
			client, err := newBTPClient()
			if err != nil {
				return err
			}
			q := url.Values{}
			if top > 0 {
				q.Set("$top", strconv.Itoa(top))
			}
			var resp btpSubaccountList
			if err := client.Get(ctx, "/accounts/v1/subaccounts", q, &resp); err != nil {
				return err
			}
			return emitBTPSubaccounts(cmd, resp.Value)
		},
	}
	c.Flags().IntVar(&top, "top", 0, "max number of subaccounts (0 = all)")
	return c
}

func newBTPSubaccountGetCmd() *cobra.Command {
	c := &cobra.Command{
		Use:   "get <guid>",
		Short: "Show a single subaccount by GUID",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, cancel := context.WithTimeout(cmd.Context(), 30*time.Second)
			defer cancel()
			client, err := newBTPClient()
			if err != nil {
				return err
			}
			var sa btpSubaccount
			if err := client.Get(ctx, "/accounts/v1/subaccounts/"+url.PathEscape(args[0]), nil, &sa); err != nil {
				return err
			}
			return emitBTPSubaccounts(cmd, []btpSubaccount{sa})
		},
	}
	return c
}

func emitBTPSubaccounts(cmd *cobra.Command, items []btpSubaccount) error {
	if globalFlags.JSON {
		enc := json.NewEncoder(cmd.OutOrStdout())
		if !globalFlags.Compact {
			enc.SetIndent("", "  ")
		}
		return enc.Encode(map[string]any{"count": len(items), "subaccounts": items})
	}
	if len(items) == 0 {
		fmt.Fprintln(cmd.OutOrStdout(), "(no subaccounts)")
		return nil
	}
	fmt.Fprintf(cmd.OutOrStdout(), "%-40s %-20s %-10s %s\n", "GUID", "SUBDOMAIN", "STATE", "DISPLAY NAME")
	for _, s := range items {
		fmt.Fprintf(cmd.OutOrStdout(), "%-40s %-20s %-10s %s\n",
			truncateCol(s.GUID, 40), truncateCol(s.Subdomain, 20), truncateCol(s.State, 10), s.DisplayName)
	}
	return nil
}

func truncateCol(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n-3] + "..."
}

// ---- service-instance ----

type btpServiceInstance struct {
	ID            string `json:"id"`
	Name          string `json:"name"`
	ServiceID     string `json:"service_id"`
	ServicePlanID string `json:"service_plan_id"`
	PlatformID    string `json:"platform_id,omitempty"`
	DashboardURL  string `json:"dashboard_url,omitempty"`
	Ready         bool   `json:"ready"`
	Usable        bool   `json:"usable"`
}

type btpServiceInstanceList struct {
	Items []btpServiceInstance `json:"items"`
}

func newBTPServiceInstanceCmd() *cobra.Command {
	c := &cobra.Command{
		Use:   "service-instance",
		Short: "Manage BTP service instances",
	}
	c.AddCommand(newBTPServiceInstanceListCmd())
	c.AddCommand(newBTPServiceInstanceGetCmd())
	return c
}

func newBTPServiceInstanceListCmd() *cobra.Command {
	c := &cobra.Command{
		Use:   "list",
		Short: "List service instances",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, cancel := context.WithTimeout(cmd.Context(), 30*time.Second)
			defer cancel()
			client, err := newBTPClient()
			if err != nil {
				return err
			}
			var resp btpServiceInstanceList
			if err := client.Get(ctx, "/v2/service_instances", nil, &resp); err != nil {
				return err
			}
			return emitBTPServiceInstances(cmd, resp.Items)
		},
	}
	return c
}

func newBTPServiceInstanceGetCmd() *cobra.Command {
	c := &cobra.Command{
		Use:   "get <id>",
		Short: "Show a single service instance",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, cancel := context.WithTimeout(cmd.Context(), 30*time.Second)
			defer cancel()
			client, err := newBTPClient()
			if err != nil {
				return err
			}
			var si btpServiceInstance
			if err := client.Get(ctx, "/v2/service_instances/"+url.PathEscape(args[0]), nil, &si); err != nil {
				return err
			}
			return emitBTPServiceInstances(cmd, []btpServiceInstance{si})
		},
	}
	return c
}

func emitBTPServiceInstances(cmd *cobra.Command, items []btpServiceInstance) error {
	if globalFlags.JSON {
		enc := json.NewEncoder(cmd.OutOrStdout())
		if !globalFlags.Compact {
			enc.SetIndent("", "  ")
		}
		return enc.Encode(map[string]any{"count": len(items), "service_instances": items})
	}
	if len(items) == 0 {
		fmt.Fprintln(cmd.OutOrStdout(), "(no service instances)")
		return nil
	}
	fmt.Fprintf(cmd.OutOrStdout(), "%-36s %-30s %-6s %s\n", "ID", "NAME", "READY", "SERVICE_ID")
	for _, s := range items {
		fmt.Fprintf(cmd.OutOrStdout(), "%-36s %-30s %-6t %s\n",
			truncateCol(s.ID, 36), truncateCol(s.Name, 30), s.Ready, s.ServiceID)
	}
	return nil
}
