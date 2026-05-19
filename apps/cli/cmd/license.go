package cmd

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/dixitsheta/sapctl/apps/cli/internal/errs"
	"github.com/dixitsheta/sapctl/apps/cli/internal/license"
)

func newLicenseCmd() *cobra.Command {
	c := &cobra.Command{
		Use:   "license",
		Short: "Install + verify + inspect sapctl licenses (Team-tier and above)",
		Long: `Offline-verifiable ed25519 JWT license keys per ADR 0005.

Free-tier users never need to run this. License gating only applies to
features tagged in the JWT 'features' claim (e.g. extended audit-export
retention, multi-credential storage). Run 'sapctl license show' to see
what your installed license entitles.`,
	}
	c.AddCommand(newLicenseInstallCmd())
	c.AddCommand(newLicenseVerifyCmd())
	c.AddCommand(newLicenseShowCmd())
	c.AddCommand(newLicenseRefreshCmd())
	return c
}

func newLicenseInstallCmd() *cobra.Command {
	var token, fromFile string
	c := &cobra.Command{
		Use:   "install",
		Short: "Install a license JWT into ~/.config/sapctl/license.jwt",
		RunE: func(cmd *cobra.Command, _ []string) error {
			t, err := resolveToken(token, fromFile, cmd.InOrStdin())
			if err != nil {
				return err
			}
			lic, err := license.Install(t)
			if err != nil {
				return mapLicenseErr("license.install", err)
			}
			p, _ := license.DefaultPath()
			fmt.Fprintf(cmd.OutOrStdout(),
				"installed: %s\ntier:      %s\nseats:     %d\nexpires:   %s (in %s)\nfeatures:  %s\n",
				p, lic.Claim.Tier, lic.Claim.Seats,
				time.Unix(lic.Claim.Exp, 0).UTC().Format(time.RFC3339),
				lic.ExpiresIn().Truncate(time.Hour),
				strings.Join(lic.Claim.Features, ", "),
			)
			return nil
		},
	}
	c.Flags().StringVar(&token, "token", "", "JWT string (or use --from-file / stdin)")
	c.Flags().StringVar(&fromFile, "from-file", "", "read JWT from file (use '-' for stdin)")
	return c
}

func newLicenseVerifyCmd() *cobra.Command {
	c := &cobra.Command{
		Use:   "verify",
		Short: "Re-verify the installed license against the embedded issuer key",
		RunE: func(cmd *cobra.Command, _ []string) error {
			lic, err := license.LoadCurrent()
			if err != nil {
				return mapLicenseErr("license.verify", err)
			}
			if !lic.Present {
				return errs.New(errs.ExitNotFound, "license.absent",
					"no license installed (free-tier active)")
			}
			fmt.Fprintln(cmd.OutOrStdout(), "license OK -- signature + audience + issuer + expiry all valid")
			return nil
		},
	}
	return c
}

func newLicenseShowCmd() *cobra.Command {
	var asJSON bool
	c := &cobra.Command{
		Use:   "show",
		Short: "Print the installed license claims (does not print the raw JWT)",
		RunE: func(cmd *cobra.Command, _ []string) error {
			lic, err := license.LoadCurrent()
			if err != nil {
				return mapLicenseErr("license.show", err)
			}
			out := cmd.OutOrStdout()
			if !lic.Present {
				if asJSON {
					_ = json.NewEncoder(out).Encode(map[string]any{"present": false})
					return nil
				}
				fmt.Fprintln(out, "no license installed (free-tier active)")
				return nil
			}
			if asJSON {
				enc := json.NewEncoder(out)
				enc.SetIndent("", "  ")
				return enc.Encode(lic.Claim)
			}
			fmt.Fprintf(out,
				"tier:     %s\nseats:    %d\nsubject:  %s\nissued:   %s\nexpires:  %s (in %s)\nfeatures: %s\n",
				lic.Claim.Tier, lic.Claim.Seats, lic.Claim.Sub,
				time.Unix(lic.Claim.Iat, 0).UTC().Format(time.RFC3339),
				time.Unix(lic.Claim.Exp, 0).UTC().Format(time.RFC3339),
				lic.ExpiresIn().Truncate(time.Hour),
				strings.Join(lic.Claim.Features, ", "),
			)
			return nil
		},
	}
	c.Flags().BoolVar(&asJSON, "json", false, "emit machine-readable JSON")
	return c
}

func newLicenseRefreshCmd() *cobra.Command {
	c := &cobra.Command{
		Use:   "refresh",
		Short: "Check the revocation list (network required; only when user runs it)",
		Long: `Fetches the revocation list at the JWT's 'rev_url' claim and rejects
the installed license if its 'sub' appears. This is the ONLY license
command that requires network -- 'install', 'verify', 'show' are all offline.

Use --rev-url to override (air-gap environments mirror revoked.json internally).`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return errs.New(errs.ExitConflict, "license.refresh.unimplemented",
				"license refresh lands with the license-issuer Worker (Phase 7 task 7.1.6)")
		},
	}
	return c
}

func resolveToken(token, fromFile string, in io.Reader) (string, error) {
	switch {
	case token != "":
		return strings.TrimSpace(token), nil
	case fromFile == "-":
		b, err := io.ReadAll(in)
		if err != nil {
			return "", errs.Wrap(errs.ExitUserError, "license.read_stdin", "read stdin", err)
		}
		return strings.TrimSpace(string(b)), nil
	case fromFile != "":
		b, err := os.ReadFile(fromFile)
		if err != nil {
			return "", errs.Wrap(errs.ExitUserError, "license.read_file", "read token file", err)
		}
		return strings.TrimSpace(string(b)), nil
	default:
		return "", errs.New(errs.ExitUserError, "license.token.missing",
			"pass --token <JWT> or --from-file <path> (or - for stdin)")
	}
}

func mapLicenseErr(op string, err error) error {
	switch err {
	case license.ErrSignature:
		return errs.New(errs.ExitAuth, op+".signature", "license signature invalid")
	case license.ErrExpired:
		return errs.New(errs.ExitAuth, op+".expired", "license expired")
	case license.ErrAudience:
		return errs.New(errs.ExitAuth, op+".audience", "license audience mismatch")
	case license.ErrIssuer:
		return errs.New(errs.ExitAuth, op+".issuer", "license issuer mismatch")
	case license.ErrMalformed:
		return errs.New(errs.ExitUserError, op+".malformed", "license token malformed")
	default:
		return errs.Wrap(errs.ExitUserError, op+".unknown", "verify license", err)
	}
}
