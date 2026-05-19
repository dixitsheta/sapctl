package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"github.com/spf13/cobra"

	"github.com/dixitsheta/sapctl/apps/cli/internal/errs"
	"github.com/dixitsheta/sapctl/apps/cli/internal/sap"
	auditchain "github.com/dixitsheta/sapctl/packages/audit-chain"
)

const (
	auditDirEnv      = "SAPCTL_AUDIT_DIR"
	defaultChainFile = "chain.jsonl"
	defaultPrivKey   = "ed25519.key"
	defaultPubKey    = "ed25519.pub"
)

// NewSAPClientAuditor returns a sap.Auditor that appends each HTTP event to
// the configured audit chain. Loads the ed25519 private key from auditDir on
// first event; if `sapctl audit init` has not run the auditor logs once to
// stderr then disables itself (non-fatal).
func NewSAPClientAuditor() sap.Auditor {
	return &chainAuditor{}
}

type chainAuditor struct {
	mu       sync.Mutex
	chain    *auditchain.Chain
	disabled bool
}

func (a *chainAuditor) Record(ctx context.Context, ev sap.AuditEvent) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.disabled {
		return
	}
	if a.chain == nil {
		dir, err := auditDir()
		if err != nil {
			a.disable("resolve dir: " + err.Error())
			return
		}
		priv, err := auditchain.LoadKey(filepath.Join(dir, defaultPrivKey))
		if err != nil {
			a.disable("no audit key (run `sapctl audit init`): " + err.Error())
			return
		}
		c, err := auditchain.New(filepath.Join(dir, defaultChainFile), priv)
		if err != nil {
			a.disable("open chain: " + err.Error())
			return
		}
		a.chain = c
	}
	if _, err := a.chain.Append("sap.http", ev); err != nil {
		fmt.Fprintln(os.Stderr, "sapctl audit (degraded): append:", err)
	}
}

func (a *chainAuditor) disable(reason string) {
	a.disabled = true
	fmt.Fprintln(os.Stderr, "sapctl audit (degraded):", reason)
}

// auditEnabled reports whether SAP HTTP middleware should record audit events.
// True if --audit global flag set OR SAPCTL_AUDIT=1 in env.
func auditEnabled() bool {
	if globalFlags.Audit {
		return true
	}
	return os.Getenv("SAPCTL_AUDIT") == "1"
}

func newAuditCmd() *cobra.Command {
	c := &cobra.Command{
		Use:   "audit",
		Short: "Append and verify ed25519-signed audit chains",
		Long: `Tamper-evident, ed25519-signed, hash-chained audit log.

By default, key + chain files live under $SAPCTL_AUDIT_DIR (else under the
user config dir at sapctl/audit). Use --chain to override the chain path.`,
	}
	c.AddCommand(newAuditInitCmd())
	c.AddCommand(newAuditEmitCmd())
	c.AddCommand(newAuditVerifyCmd())
	return c
}

func auditDir() (string, error) {
	if d := os.Getenv(auditDirEnv); d != "" {
		return d, nil
	}
	base, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(base, "sapctl", "audit"), nil
}

func newAuditInitCmd() *cobra.Command {
	var force bool
	c := &cobra.Command{
		Use:   "init",
		Short: "Generate ed25519 keypair (write to audit dir)",
		RunE: func(cmd *cobra.Command, args []string) error {
			dir, err := auditDir()
			if err != nil {
				return errs.Wrap(errs.ExitUserError, "audit.init.dir", "resolve audit dir", err)
			}
			if err := os.MkdirAll(dir, 0o700); err != nil {
				return errs.Wrap(errs.ExitUserError, "audit.init.mkdir", "create audit dir", err)
			}
			kp := filepath.Join(dir, defaultPrivKey)
			pp := filepath.Join(dir, defaultPubKey)
			if !force {
				if _, err := os.Stat(kp); err == nil {
					return errs.New(errs.ExitConflict, "audit.init.exists",
						fmt.Sprintf("key already exists at %s (use --force to overwrite)", kp))
				}
			}
			pub, priv, err := auditchain.GenerateKey()
			if err != nil {
				return errs.Wrap(errs.ExitUserError, "audit.init.gen", "generate key", err)
			}
			if err := auditchain.SaveKey(kp, priv); err != nil {
				return errs.Wrap(errs.ExitUserError, "audit.init.save_priv", "save private key", err)
			}
			if err := auditchain.SavePublicKey(pp, pub); err != nil {
				return errs.Wrap(errs.ExitUserError, "audit.init.save_pub", "save public key", err)
			}
			if globalFlags.JSON {
				return writeJSON(cmd, map[string]string{
					"private_key": kp,
					"public_key":  pp,
					"audit_dir":   dir,
				})
			}
			fmt.Fprintf(cmd.OutOrStdout(), "wrote private key: %s\nwrote public key:  %s\n", kp, pp)
			return nil
		},
	}
	c.Flags().BoolVar(&force, "force", false, "overwrite existing key")
	return c
}

type auditEmitFlags struct {
	kind    string
	payload string
	chain   string
	credLbl string
}

func newAuditEmitCmd() *cobra.Command {
	var f auditEmitFlags
	c := &cobra.Command{
		Use:   "emit",
		Short: "Append a signed event to the audit chain",
		Long: `Append an event to the audit chain. --payload accepts a JSON value
(object, array, string, etc). Use --cred to tag the event with a credential
label (does not load or persist the credential's secret).`,
		RunE: func(cmd *cobra.Command, args []string) error {
			dir, err := auditDir()
			if err != nil {
				return errs.Wrap(errs.ExitUserError, "audit.emit.dir", "resolve audit dir", err)
			}
			chainPath := f.chain
			if chainPath == "" {
				chainPath = filepath.Join(dir, defaultChainFile)
			}
			priv, err := auditchain.LoadKey(filepath.Join(dir, defaultPrivKey))
			if err != nil {
				return errs.Wrap(errs.ExitAuth, "audit.emit.load_key",
					"load private key (run `sapctl audit init` first)", err)
			}
			c, err := auditchain.New(chainPath, priv)
			if err != nil {
				return errs.Wrap(errs.ExitUserError, "audit.emit.open", "open chain", err)
			}

			var payload any
			if f.payload != "" {
				if err := json.Unmarshal([]byte(f.payload), &payload); err != nil {
					return errs.Wrap(errs.ExitUserError, "audit.emit.payload",
						"--payload must be valid JSON", err)
				}
			}
			if f.credLbl != "" {
				payload = map[string]any{"data": payload, "cred_label": f.credLbl}
			}

			ev, err := c.Append(f.kind, payload)
			if err != nil {
				return errs.Wrap(errs.ExitUserError, "audit.emit.append", "append event", err)
			}
			if globalFlags.JSON {
				return writeJSON(cmd, ev)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "seq=%d hash=%s\n", ev.Seq, ev.Hash)
			return nil
		},
	}
	c.Flags().StringVar(&f.kind, "kind", "", "event kind (required)")
	c.Flags().StringVar(&f.payload, "payload", "", "JSON payload")
	c.Flags().StringVar(&f.chain, "chain", "", "override chain file path")
	c.Flags().StringVar(&f.credLbl, "cred", "", "tag event with this credential label")
	_ = c.MarkFlagRequired("kind")
	return c
}

func newAuditVerifyCmd() *cobra.Command {
	var chainPath, pubPath string
	c := &cobra.Command{
		Use:   "verify",
		Short: "Verify the hash-chain and signatures of an audit log",
		RunE: func(cmd *cobra.Command, args []string) error {
			dir, err := auditDir()
			if err != nil {
				return errs.Wrap(errs.ExitUserError, "audit.verify.dir", "resolve audit dir", err)
			}
			if chainPath == "" {
				chainPath = filepath.Join(dir, defaultChainFile)
			}
			if pubPath == "" {
				pubPath = filepath.Join(dir, defaultPubKey)
			}
			pub, err := auditchain.LoadPublicKey(pubPath)
			if err != nil {
				return errs.Wrap(errs.ExitUserError, "audit.verify.load_pub", "load public key", err)
			}
			n, err := auditchain.Verify(chainPath, pub)
			if err != nil {
				return errs.Wrap(errs.ExitConflict, "audit.verify.invalid",
					fmt.Sprintf("verification failed at event %d", n+1), err)
			}
			if globalFlags.JSON {
				return writeJSON(cmd, map[string]any{
					"verified": true, "events": n, "chain": chainPath,
				})
			}
			fmt.Fprintf(cmd.OutOrStdout(), "ok: %d events verified\n", n)
			return nil
		},
	}
	c.Flags().StringVar(&chainPath, "chain", "", "chain file path (default: $SAPCTL_AUDIT_DIR/chain.jsonl)")
	c.Flags().StringVar(&pubPath, "pub", "", "public key path (default: $SAPCTL_AUDIT_DIR/ed25519.pub)")
	return c
}
