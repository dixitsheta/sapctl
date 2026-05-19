package cmd

import "github.com/spf13/cobra"

// s4Flags holds inputs shared by all `sapctl s4 ...` subcommands.
type s4Flags struct {
	credLabel string // --cred label of stored credential
	baseURL   string // --base-url override (else inferred per flow)
}

var s4Shared s4Flags

func newS4Cmd() *cobra.Command {
	c := &cobra.Command{
		Use:   "s4",
		Short: "Interact with SAP S/4HANA Cloud",
		Long: `Commands targeting SAP S/4HANA Cloud Public Edition or the SAP
Business Accelerator Hub sandbox.

Specify --cred to pick which stored credential to use (see sapctl auth login).
For the sandbox, default --base-url is https://sandbox.api.sap.com/s4hanacloud.`,
	}
	c.PersistentFlags().StringVar(&s4Shared.credLabel, "cred", "", "stored credential label (see `sapctl auth list`)")
	c.PersistentFlags().StringVar(&s4Shared.baseURL, "base-url", "", "override base URL (else inferred)")
	c.AddCommand(newS4CatalogCmd())
	c.AddCommand(newS4ODataCmd())
	c.AddCommand(newS4AuditExportCmd())
	return c
}
