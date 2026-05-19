package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/dixitsheta/sapctl/apps/cli/internal/auth"
	"github.com/dixitsheta/sapctl/apps/cli/internal/errs"
	"github.com/dixitsheta/sapctl/apps/cli/internal/sap"
)

type aiFlags struct {
	credLabel     string
	apiBase       string
	resourceGroup string
}

var aiShared aiFlags

func newAICoreCmd() *cobra.Command {
	c := &cobra.Command{
		Use:   "aicore",
		Short: "Interact with SAP AI Core + GenAI Hub",
		Long: `Commands targeting SAP AI Core (LM v2) and GenAI Hub APIs.

Auth via XSUAA service-key binding for AI Core:

  sapctl auth login --flow xsuaa --label ai-trial --client-id <ID> ...

Then:

  sapctl aicore deployment list --cred ai-trial \
    --api https://api.ai.<region>.ml.hana.ondemand.com
  sapctl aicore genai-hub models --cred ai-trial --api ...`,
	}
	c.PersistentFlags().StringVar(&aiShared.credLabel, "cred", "", "stored credential label (xsuaa)")
	c.PersistentFlags().StringVar(&aiShared.apiBase, "api", "", "AI Core / GenAI Hub API base URL (required)")
	c.PersistentFlags().StringVar(&aiShared.resourceGroup, "resource-group", "default", "AI-Resource-Group value (passed as $resourceGroup query param)")
	c.AddCommand(newAIDeploymentCmd())
	c.AddCommand(newAIGenAIHubCmd())
	return c
}

func newAIClient() (*sap.Client, error) {
	if aiShared.credLabel == "" {
		return nil, errs.New(errs.ExitUserError, "aicore.cred_required",
			"--cred is required; run `sapctl auth login --flow xsuaa` first")
	}
	if aiShared.apiBase == "" {
		return nil, errs.New(errs.ExitUserError, "aicore.api_required",
			"--api is required; pass AI Core API base URL")
	}
	cfgPath, err := auth.DefaultPath()
	if err != nil {
		return nil, errs.Wrap(errs.ExitUserError, "aicore.cache_path", "resolve cache path", err)
	}
	cache := auth.NewCache(cfgPath)
	cfg, err := cache.Load(aiShared.credLabel)
	if err != nil {
		return nil, err
	}
	if cfg.Provider != "xsuaa" {
		return nil, errs.New(errs.ExitUserError, "aicore.cred_wrong_flow",
			fmt.Sprintf("credential %q has flow=%s; expected flow=xsuaa", cfg.Label, cfg.Provider))
	}
	provider, err := auth.New(cfg)
	if err != nil {
		return nil, errs.Wrap(errs.ExitUserError, "aicore.provider", "build provider", err)
	}
	client := sap.New(aiShared.apiBase, provider)
	if auditEnabled() {
		client.Audit = NewSAPClientAuditor()
	}
	if t := sap.NewEnvTracer(); t != nil {
		client.Tracer = t
	}
	return client, nil
}

type aiDeployment struct {
	ID              string `json:"id"`
	ConfigurationID string `json:"configurationId,omitempty"`
	ScenarioID      string `json:"scenarioId,omitempty"`
	Status          string `json:"status,omitempty"`
	DeploymentURL   string `json:"deploymentUrl,omitempty"`
	CreatedAt       string `json:"createdAt,omitempty"`
	ModifiedAt      string `json:"modifiedAt,omitempty"`
}

type aiDeploymentList struct {
	Count     int            `json:"count,omitempty"`
	Resources []aiDeployment `json:"resources"`
}

func newAIDeploymentCmd() *cobra.Command {
	c := &cobra.Command{
		Use:   "deployment",
		Short: "Manage AI Core deployments",
	}
	c.AddCommand(&cobra.Command{
		Use:   "list",
		Short: "List AI Core deployments",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, cancel := context.WithTimeout(cmd.Context(), 30*time.Second)
			defer cancel()
			client, err := newAIClient()
			if err != nil {
				return err
			}
			var resp aiDeploymentList
			if err := aiGet(ctx, client, "/v2/lm/deployments", &resp); err != nil {
				return err
			}
			if globalFlags.JSON {
				return writeJSON(cmd, map[string]any{
					"count": len(resp.Resources), "deployments": resp.Resources,
				})
			}
			if len(resp.Resources) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "(no deployments)")
				return nil
			}
			fmt.Fprintf(cmd.OutOrStdout(), "%-36s %-20s %s\n", "ID", "STATUS", "SCENARIO")
			for _, d := range resp.Resources {
				fmt.Fprintf(cmd.OutOrStdout(), "%-36s %-20s %s\n",
					truncateCol(d.ID, 36), truncateCol(d.Status, 20), d.ScenarioID)
			}
			return nil
		},
	})
	c.AddCommand(&cobra.Command{
		Use:   "get <id>",
		Short: "Show a single deployment",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, cancel := context.WithTimeout(cmd.Context(), 30*time.Second)
			defer cancel()
			client, err := newAIClient()
			if err != nil {
				return err
			}
			var d aiDeployment
			if err := aiGet(ctx, client, "/v2/lm/deployments/"+url.PathEscape(args[0]), &d); err != nil {
				return err
			}
			return writeJSON(cmd, d)
		},
	})
	return c
}

type aiModel struct {
	Model              string   `json:"model"`
	Description        string   `json:"description,omitempty"`
	StreamingSupported bool     `json:"streamingSupported,omitempty"`
	Versions           []string `json:"versions,omitempty"`
}

type aiModelList struct {
	Resources []aiModel `json:"resources"`
}

func newAIGenAIHubCmd() *cobra.Command {
	c := &cobra.Command{
		Use:   "genai-hub",
		Short: "GenAI Hub: foundation models + completion",
	}
	c.AddCommand(newAIModelListCmd())
	c.AddCommand(newAICompletionCmd())
	return c
}

func newAIModelListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "models",
		Short: "List foundation models available via GenAI Hub",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, cancel := context.WithTimeout(cmd.Context(), 30*time.Second)
			defer cancel()
			client, err := newAIClient()
			if err != nil {
				return err
			}
			var resp aiModelList
			if err := aiGet(ctx, client, "/v2/admin/scenarios/foundation-models/models", &resp); err != nil {
				return err
			}
			if globalFlags.JSON {
				return writeJSON(cmd, map[string]any{
					"count": len(resp.Resources), "models": resp.Resources,
				})
			}
			if len(resp.Resources) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "(no models)")
				return nil
			}
			fmt.Fprintf(cmd.OutOrStdout(), "%-40s %s\n", "MODEL", "DESCRIPTION")
			for _, m := range resp.Resources {
				fmt.Fprintf(cmd.OutOrStdout(), "%-40s %s\n", truncateCol(m.Model, 40), m.Description)
			}
			return nil
		},
	}
}

type aiCompletionFlags struct {
	deploymentID string
	model        string
	prompt       string
}

func newAICompletionCmd() *cobra.Command {
	var f aiCompletionFlags
	c := &cobra.Command{
		Use:   "complete",
		Short: "Run a chat completion against a deployment",
		RunE: func(cmd *cobra.Command, args []string) error {
			if f.deploymentID == "" || f.prompt == "" {
				return errs.New(errs.ExitUserError, "aicore.complete.args",
					"--deployment and --prompt are required")
			}
			ctx, cancel := context.WithTimeout(cmd.Context(), 2*time.Minute)
			defer cancel()
			client, err := newAIClient()
			if err != nil {
				return err
			}
			body := map[string]any{
				"messages": []map[string]string{
					{"role": "user", "content": f.prompt},
				},
			}
			if f.model != "" {
				body["model"] = f.model
			}
			payload, _ := json.Marshal(body)
			var resp json.RawMessage
			path := "/v2/inference/deployments/" + url.PathEscape(f.deploymentID) + "/chat/completions"
			path = appendResourceGroup(path)
			if err := postJSON(ctx, client, path, payload, &resp); err != nil {
				return err
			}
			enc := json.NewEncoder(cmd.OutOrStdout())
			if !globalFlags.Compact {
				enc.SetIndent("", "  ")
			}
			return enc.Encode(resp)
		},
	}
	c.Flags().StringVar(&f.deploymentID, "deployment", "", "deployment ID (required)")
	c.Flags().StringVar(&f.model, "model", "", "model name (optional, deployment may bind one)")
	c.Flags().StringVar(&f.prompt, "prompt", "", "user prompt text (required)")
	_ = c.MarkFlagRequired("deployment")
	_ = c.MarkFlagRequired("prompt")
	return c
}

// aiGet wraps sap.Client.Get adding the AI-Resource-Group query param.
func aiGet(ctx context.Context, c *sap.Client, path string, out any) error {
	q := url.Values{}
	if aiShared.resourceGroup != "" {
		q.Set("$resourceGroup", aiShared.resourceGroup)
	}
	return c.Get(ctx, path, q, out)
}

// appendResourceGroup encodes resource-group on a path that already has a
// hand-built query string (POST path).
func appendResourceGroup(path string) string {
	if aiShared.resourceGroup == "" {
		return path
	}
	sep := "?"
	if strings.Contains(path, "?") {
		sep = "&"
	}
	return path + sep + "$resourceGroup=" + url.QueryEscape(aiShared.resourceGroup)
}
