package cmd

import (
	"encoding/json"
	"fmt"
	"sort"

	"github.com/spf13/cobra"

	"github.com/dixitsheta/sapctl/apps/cli/internal/auth"
	"github.com/dixitsheta/sapctl/apps/cli/internal/errs"
)

type authFlags struct {
	flow         string
	label        string
	apiKey       string
	username     string
	password     string
	clientID     string
	clientSecret string
	tokenURL     string
}

func newAuthCmd() *cobra.Command {
	c := &cobra.Command{
		Use:   "auth",
		Short: "Manage SAP authentication credentials",
		Long:  "Store and inspect SAP credentials at " + cacheLocationHint() + ".",
	}
	c.AddCommand(newAuthLoginCmd())
	c.AddCommand(newAuthStatusCmd())
	c.AddCommand(newAuthLogoutCmd())
	c.AddCommand(newAuthListCmd())
	return c
}

func cacheLocationHint() string {
	p, err := auth.DefaultPath()
	if err != nil {
		return "~/.config/sapctl/tokens.json"
	}
	return p
}

func newAuthLoginCmd() *cobra.Command {
	var f authFlags
	c := &cobra.Command{
		Use:   "login",
		Short: "Store credentials for a SAP service",
		Long: `Store credentials under a label for later use.

Flows:
  apikey   SAP Business Accelerator Hub sandbox APIKey header
  basic    HTTP Basic auth (S/4HANA Cloud Communication User)
  xsuaa    OAuth2 client_credentials against BTP XSUAA / IAS

Examples:
  sapctl auth login --flow apikey --label sandbox --api-key <KEY>
  sapctl auth login --flow basic  --label s4-trial --username SAPCTL_DEV_USER --password '<PW>'
  sapctl auth login --flow xsuaa  --label btp-trial --client-id <ID> --client-secret <SECRET> --token-url https://<tenant>.authentication.eu10.hana.ondemand.com/oauth/token`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if f.label == "" {
				return errs.New(errs.ExitUserError, "auth.login.label_required", "--label is required")
			}
			cfg, err := buildConfigFromFlags(f)
			if err != nil {
				return err
			}

			path, err := auth.DefaultPath()
			if err != nil {
				return errs.Wrap(errs.ExitUserError, "auth.login.path", "resolve cache path", err)
			}
			cache := auth.NewCache(path)
			if err := cache.Save(f.label, cfg); err != nil {
				return err
			}

			if globalFlags.JSON {
				return writeJSON(cmd, map[string]string{
					"label": f.label, "flow": f.flow, "status": "saved",
				})
			}
			fmt.Fprintf(cmd.OutOrStdout(), "saved credential %q (flow=%s)\n", f.label, f.flow)
			return nil
		},
	}
	c.Flags().StringVar(&f.flow, "flow", "", "auth flow: apikey | basic | xsuaa")
	c.Flags().StringVar(&f.label, "label", "", "label to store credential under (required)")
	c.Flags().StringVar(&f.apiKey, "api-key", "", "API key (flow=apikey)")
	c.Flags().StringVar(&f.username, "username", "", "username (flow=basic)")
	c.Flags().StringVar(&f.password, "password", "", "password (flow=basic)")
	c.Flags().StringVar(&f.clientID, "client-id", "", "OAuth2 client ID (flow=xsuaa)")
	c.Flags().StringVar(&f.clientSecret, "client-secret", "", "OAuth2 client secret (flow=xsuaa)")
	c.Flags().StringVar(&f.tokenURL, "token-url", "", "OAuth2 token URL (flow=xsuaa)")
	_ = c.MarkFlagRequired("flow")
	_ = c.MarkFlagRequired("label")
	return c
}

func buildConfigFromFlags(f authFlags) (auth.Config, error) {
	switch f.flow {
	case "apikey":
		if f.apiKey == "" {
			return auth.Config{}, errs.New(errs.ExitUserError, "auth.login.apikey.missing", "--api-key is required for flow=apikey")
		}
		return auth.Config{Provider: "apikey", APIKey: f.apiKey}, nil
	case "basic":
		if f.username == "" || f.password == "" {
			return auth.Config{}, errs.New(errs.ExitUserError, "auth.login.basic.missing", "--username and --password are required for flow=basic")
		}
		return auth.Config{Provider: "basic", Username: f.username, Password: f.password}, nil
	case "xsuaa":
		if f.clientID == "" || f.clientSecret == "" || f.tokenURL == "" {
			return auth.Config{}, errs.New(errs.ExitUserError, "auth.login.xsuaa.missing",
				"--client-id, --client-secret, --token-url are required for flow=xsuaa")
		}
		return auth.Config{Provider: "xsuaa", ClientID: f.clientID, ClientSecret: f.clientSecret, TokenURL: f.tokenURL}, nil
	default:
		return auth.Config{}, errs.New(errs.ExitUserError, "auth.login.flow", fmt.Sprintf("unknown flow %q", f.flow))
	}
}

func newAuthStatusCmd() *cobra.Command {
	var label string
	c := &cobra.Command{
		Use:   "status",
		Short: "Show stored credential metadata (never prints secrets)",
		RunE: func(cmd *cobra.Command, args []string) error {
			if label == "" {
				return errs.New(errs.ExitUserError, "auth.status.label_required", "--label is required")
			}
			path, err := auth.DefaultPath()
			if err != nil {
				return errs.Wrap(errs.ExitUserError, "auth.status.path", "resolve cache path", err)
			}
			cache := auth.NewCache(path)
			cfg, err := cache.Load(label)
			if err != nil {
				return err
			}

			redacted := redact(cfg)
			if globalFlags.JSON {
				return writeJSON(cmd, redacted)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "label:    %s\nprovider: %s\n", redacted.Label, redacted.Provider)
			if redacted.Username != "" {
				fmt.Fprintf(cmd.OutOrStdout(), "username: %s\n", redacted.Username)
			}
			if redacted.ClientID != "" {
				fmt.Fprintf(cmd.OutOrStdout(), "client_id: %s\n", redacted.ClientID)
			}
			if redacted.TokenURL != "" {
				fmt.Fprintf(cmd.OutOrStdout(), "token_url: %s\n", redacted.TokenURL)
			}
			return nil
		},
	}
	c.Flags().StringVar(&label, "label", "", "credential label (required)")
	_ = c.MarkFlagRequired("label")
	return c
}

func newAuthLogoutCmd() *cobra.Command {
	var label string
	c := &cobra.Command{
		Use:   "logout",
		Short: "Remove a stored credential",
		RunE: func(cmd *cobra.Command, args []string) error {
			if label == "" {
				return errs.New(errs.ExitUserError, "auth.logout.label_required", "--label is required")
			}
			path, err := auth.DefaultPath()
			if err != nil {
				return errs.Wrap(errs.ExitUserError, "auth.logout.path", "resolve cache path", err)
			}
			cache := auth.NewCache(path)
			if err := cache.Delete(label); err != nil {
				return err
			}
			if globalFlags.JSON {
				return writeJSON(cmd, map[string]string{"label": label, "status": "deleted"})
			}
			fmt.Fprintf(cmd.OutOrStdout(), "deleted credential %q\n", label)
			return nil
		},
	}
	c.Flags().StringVar(&label, "label", "", "credential label (required)")
	_ = c.MarkFlagRequired("label")
	return c
}

func newAuthListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List stored credential labels",
		RunE: func(cmd *cobra.Command, args []string) error {
			path, err := auth.DefaultPath()
			if err != nil {
				return errs.Wrap(errs.ExitUserError, "auth.list.path", "resolve cache path", err)
			}
			cache := auth.NewCache(path)
			labels, err := cache.List()
			if err != nil {
				return err
			}
			sort.Strings(labels)
			if globalFlags.JSON {
				return writeJSON(cmd, map[string]any{"labels": labels})
			}
			if len(labels) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "(no stored credentials)")
				return nil
			}
			for _, l := range labels {
				fmt.Fprintln(cmd.OutOrStdout(), l)
			}
			return nil
		},
	}
}

func redact(c auth.Config) auth.Config {
	out := c
	out.APIKey = redactString(c.APIKey)
	out.Password = redactString(c.Password)
	out.ClientSecret = redactString(c.ClientSecret)
	return out
}

func redactString(s string) string {
	if s == "" {
		return ""
	}
	if len(s) <= 6 {
		return "***"
	}
	return s[:3] + "***" + s[len(s)-3:]
}

func writeJSON(cmd *cobra.Command, v any) error {
	enc := json.NewEncoder(cmd.OutOrStdout())
	if globalFlags.Compact {
		return enc.Encode(v)
	}
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}
