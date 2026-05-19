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

// sandboxBaseURL is the SAP Business Accelerator Hub sandbox base for S/4HANA Cloud.
const sandboxBaseURL = "https://sandbox.api.sap.com/s4hanacloud"

// catalogResponse is the OData v2 envelope for /CATALOGSERVICE;v=2/ServiceCollection.
type catalogResponse struct {
	D struct {
		Results []catalogService `json:"results"`
	} `json:"d"`
}

type catalogService struct {
	ID                      string `json:"ID"`
	Description             string `json:"Description"`
	Title                   string `json:"Title"`
	Author                  string `json:"Author"`
	TechnicalServiceName    string `json:"TechnicalServiceName,omitempty"`
	TechnicalServiceVersion int    `json:"TechnicalServiceVersion,omitempty"`
}

func newS4CatalogCmd() *cobra.Command {
	var top int
	c := &cobra.Command{
		Use:   "catalog",
		Short: "S/4HANA service catalog",
	}
	disc := &cobra.Command{
		Use:   "discover",
		Short: "List available S/4HANA OData services (sandbox or real tenant)",
		Long: `Discover OData services exposed by a SAP S/4HANA Cloud tenant.

Defaults to the SAP Business Accelerator Hub sandbox; pass --base-url to point
at a real tenant. Authenticate first via:

  sapctl auth login --flow apikey --label sandbox --api-key <KEY>
  sapctl s4 catalog discover --cred sandbox --top 10`,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, cancel := context.WithTimeout(cmd.Context(), 60*time.Second)
			defer cancel()
			return runS4CatalogDiscover(ctx, cmd, top)
		},
	}
	disc.Flags().IntVar(&top, "top", 25, "max number of services to return")
	c.AddCommand(disc)
	return c
}

func runS4CatalogDiscover(ctx context.Context, cmd *cobra.Command, top int) error {
	if s4Shared.credLabel == "" {
		return errs.New(errs.ExitUserError, "s4.catalog.cred_required",
			"--cred is required; run `sapctl auth login` first")
	}

	path, err := auth.DefaultPath()
	if err != nil {
		return errs.Wrap(errs.ExitUserError, "s4.catalog.cache_path", "resolve cache path", err)
	}
	cache := auth.NewCache(path)
	cfg, err := cache.Load(s4Shared.credLabel)
	if err != nil {
		return err
	}
	provider, err := auth.New(cfg)
	if err != nil {
		return errs.Wrap(errs.ExitUserError, "s4.catalog.provider", "build provider", err)
	}

	base := s4Shared.baseURL
	if base == "" {
		base = sandboxBaseURL
	}
	client := sap.New(base, provider)
	if auditEnabled() {
		client.Audit = NewSAPClientAuditor()
	}
	if t := sap.NewEnvTracer(); t != nil {
		client.Tracer = t
	}

	q := url.Values{}
	if top > 0 {
		q.Set("$top", strconv.Itoa(top))
	}
	q.Set("$format", "json")

	var resp catalogResponse
	if err := client.Get(ctx, "/sap/opu/odata/iwfnd/CATALOGSERVICE;v=2/ServiceCollection", q, &resp); err != nil {
		return err
	}

	return emitCatalogResults(cmd, resp.D.Results)
}

func emitCatalogResults(cmd *cobra.Command, items []catalogService) error {
	if globalFlags.JSON {
		enc := json.NewEncoder(cmd.OutOrStdout())
		if !globalFlags.Compact {
			enc.SetIndent("", "  ")
		}
		return enc.Encode(map[string]any{
			"count":    len(items),
			"services": items,
		})
	}

	if len(items) == 0 {
		fmt.Fprintln(cmd.OutOrStdout(), "(no services returned)")
		return nil
	}
	fmt.Fprintf(cmd.OutOrStdout(), "%-50s %s\n", "ID", "TITLE")
	for _, it := range items {
		id := it.ID
		if len(id) > 50 {
			id = id[:47] + "..."
		}
		fmt.Fprintf(cmd.OutOrStdout(), "%-50s %s\n", id, it.Title)
	}
	return nil
}
