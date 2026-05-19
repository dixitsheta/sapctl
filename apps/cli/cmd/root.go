// Package cmd implements the sapctl command tree (Cobra).
package cmd

import (
	"fmt"
	"io"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// GlobalFlags carries the values of all root-level persistent flags.
// Locked by ADR 0002 - adding or removing any flag here is a breaking change.
type GlobalFlags struct {
	JSON    bool   // --json: machine-readable output
	Select  string // --select: projection / field filter
	DryRun  bool   // --dry-run: don't perform side-effects
	Compact bool   // --compact: minimize output volume
	Quiet   bool   // --quiet: suppress non-error output
	Yes     bool   // --yes: assume yes to confirmations
	NoInput bool   // --no-input: fail rather than prompt
	Agent   bool   // --agent: expand agent macros
	Since   string // --since: cursor for delta / CDC
	Audit   bool   // --audit: append a signed audit event per HTTP call
}

var globalFlags GlobalFlags

// Globals returns the parsed global flags. Stable accessor for subcommands.
func Globals() GlobalFlags { return globalFlags }

// NewRootCmd builds a fresh root command tree.
func NewRootCmd(version string) *cobra.Command {
	root := &cobra.Command{
		Use:           "sapctl",
		Short:         "Unified, agent-native CLI for SAP",
		Long:          "sapctl is the unified, agent-native command-line interface for the SAP product portfolio.\n\nSee https://sapctl.dev for documentation.",
		Version:       version,
		SilenceUsage:  true,
		SilenceErrors: true,
	}

	pf := root.PersistentFlags()
	pf.BoolVar(&globalFlags.JSON, "json", false, "emit machine-readable JSON")
	pf.StringVar(&globalFlags.Select, "select", "", "projection / field filter")
	pf.BoolVar(&globalFlags.DryRun, "dry-run", false, "do not perform side-effects")
	pf.BoolVar(&globalFlags.Compact, "compact", false, "minimize output volume")
	pf.BoolVar(&globalFlags.Quiet, "quiet", false, "suppress non-error output")
	pf.BoolVar(&globalFlags.Yes, "yes", false, "assume yes on confirmations")
	pf.BoolVar(&globalFlags.NoInput, "no-input", false, "fail rather than prompt")
	pf.BoolVar(&globalFlags.Agent, "agent", false, "expand agent macros")
	pf.StringVar(&globalFlags.Since, "since", "", "cursor for delta / CDC operations")
	pf.BoolVar(&globalFlags.Audit, "audit", false, "append signed audit event per HTTP call (requires `sapctl audit init`)")

	// Bind to viper for env-var support: SAPCTL_JSON, SAPCTL_QUIET, etc.
	_ = viper.BindPFlags(pf)
	viper.SetEnvPrefix("SAPCTL")
	viper.AutomaticEnv()

	root.AddCommand(newVersionCmd(version))
	root.AddCommand(newAuthCmd())
	root.AddCommand(newS4Cmd())
	root.AddCommand(newAuditCmd())
	root.AddCommand(newMirrorCmd())
	root.AddCommand(newBTPCmd())
	root.AddCommand(newMCPCmd())
	root.AddCommand(newDatasphereCmd())
	root.AddCommand(newAICoreCmd())
	root.AddCommand(newBundleCmd())
	root.AddCommand(newLicenseCmd())

	return root
}

// Execute runs the root command and returns the process exit code.
// The caller is responsible for calling os.Exit with the returned value.
func Execute(version string, stdout, stderr io.Writer) int {
	root := NewRootCmd(version)
	root.SetOut(stdout)
	root.SetErr(stderr)
	if err := root.Execute(); err != nil {
		fmt.Fprintln(stderr, "Error:", err)
		return 2 // errs.ExitUserError; avoid import cycle by hardcoding
	}
	return 0
}
