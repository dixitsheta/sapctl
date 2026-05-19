package cmd

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/dixitsheta/sapctl/apps/cli/internal/auth"
	"github.com/dixitsheta/sapctl/apps/cli/internal/errs"
	"github.com/dixitsheta/sapctl/apps/cli/internal/license"
	"github.com/dixitsheta/sapctl/apps/cli/internal/sap"
	auditchain "github.com/dixitsheta/sapctl/packages/audit-chain"
)

// freeTierRetainDaysMax caps --retain for users without the
// "audit-export-retain-365d" license feature.
const freeTierRetainDaysMax = 30

// useCase is a pre-baked OData query targeting a regulatory evidence need.
type useCase struct {
	name        string
	service     string
	entity      string
	dateField   string
	selectField string
	description string
}

// useCases is the curated catalogue. Adding entries here = new evidence
// recipes (DORA, CSRD, 21 CFR Part 11, ITAR, etc.).
var useCases = map[string]useCase{
	"sox-journal": {
		name:        "sox-journal",
		service:     "API_JOURNALENTRYITEMBASIC_SRV",
		entity:      "A_JournalEntryItemBasic",
		dateField:   "PostingDate",
		selectField: "JournalEntry,CompanyCode,FiscalYear,FiscalPeriod,PostingDate,GLAccount,DocumentItemText,AmountInTransactionCurrency,TransactionCurrency,DebitCreditCode",
		description: "SOX general-ledger journal items by posting date range",
	},
	"sox-bp": {
		name:        "sox-bp",
		service:     "API_BUSINESS_PARTNER",
		entity:      "A_BusinessPartner",
		dateField:   "LastChangeDateTime",
		selectField: "BusinessPartner,BusinessPartnerFullName,BusinessPartnerCategory,LastChangeDateTime,CreatedByUser",
		description: "SOX business-partner master deltas (master-data integrity)",
	},
}

type s4AuditExportFlags struct {
	useCase    string
	from       string
	to         string
	outDir     string
	maxRows    int
	bundle     bool
	retainDays int // 0 = unset; free-tier cap 30; team-tier up to 365 (license-gated)
}

func newS4AuditExportCmd() *cobra.Command {
	var f s4AuditExportFlags
	c := &cobra.Command{
		Use:   "audit-export",
		Short: "Export a signed evidence pack for a regulatory use case",
		Long: `Run a pre-baked OData query, write rows to a local audit chain (one
event per row, ed25519-signed, hash-linked), and emit a verifiable tar.gz
bundle ready for auditor consumption.

Available use cases:
  sox-journal   SOX general-ledger journal items by posting date range
  sox-bp        SOX business-partner master deltas (master-data integrity)

Output bundle contains:
  rows.jsonl       one OData row per line
  chain.jsonl      ed25519 hash-chained audit events (one per row + envelope)
  ed25519.pub      verifier public key
  manifest.json    {use_case, from, to, row_count, sha256_rows, sha256_chain, generated_at}

Verify with:
  tar -xzf sapctl-evidence-sox-journal-2025-01-01-to-2025-12-31.tar.gz
  sapctl audit verify --chain chain.jsonl --pub ed25519.pub
`,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, cancel := context.WithTimeout(cmd.Context(), 10*time.Minute)
			defer cancel()
			return runS4AuditExport(ctx, cmd, f)
		},
	}
	c.Flags().StringVar(&f.useCase, "use-case", "", "evidence recipe name (sox-journal | sox-bp)")
	c.Flags().StringVar(&f.from, "from", "", "lower bound (YYYY-MM-DD)")
	c.Flags().StringVar(&f.to, "to", "", "upper bound (YYYY-MM-DD)")
	c.Flags().StringVar(&f.outDir, "out", ".", "output directory for the evidence bundle")
	c.Flags().IntVar(&f.maxRows, "max-rows", 5000, "safety cap on rows fetched")
	c.Flags().BoolVar(&f.bundle, "bundle", true, "produce a tar.gz bundle (--bundle=false leaves loose files)")
	c.Flags().IntVar(&f.retainDays, "retain", 0,
		"intended retention in days (annotated on manifest; >30 requires Team-tier license)")
	_ = c.MarkFlagRequired("use-case")
	return c
}

func runS4AuditExport(ctx context.Context, cmd *cobra.Command, f s4AuditExportFlags) error {
	uc, ok := useCases[f.useCase]
	if !ok {
		return errs.New(errs.ExitUserError, "s4.audit_export.usecase",
			fmt.Sprintf("unknown use case %q; try one of: %s", f.useCase, listUseCases()))
	}
	if f.from == "" || f.to == "" {
		return errs.New(errs.ExitUserError, "s4.audit_export.range",
			"--from and --to are required (YYYY-MM-DD)")
	}
	if s4Shared.credLabel == "" {
		return errs.New(errs.ExitUserError, "s4.audit_export.cred_required",
			"--cred is required; run `sapctl auth login` first")
	}
	if err := enforceRetainLicense(f.retainDays); err != nil {
		return err
	}

	cfgPath, err := auth.DefaultPath()
	if err != nil {
		return errs.Wrap(errs.ExitUserError, "s4.audit_export.cache_path", "resolve cache path", err)
	}
	cache := auth.NewCache(cfgPath)
	cfg, err := cache.Load(s4Shared.credLabel)
	if err != nil {
		return err
	}
	provider, err := auth.New(cfg)
	if err != nil {
		return errs.Wrap(errs.ExitUserError, "s4.audit_export.provider", "build provider", err)
	}
	base := s4Shared.baseURL
	if base == "" {
		base = sandboxBaseURL
	}
	client := sap.New(base, provider)
	if auditEnabled() {
		client.Audit = NewSAPClientAuditor()
	}

	stamp := time.Now().UTC().Format("2006-01-02T150405Z")
	workName := fmt.Sprintf("sapctl-evidence-%s-%s-to-%s-%s", uc.name, f.from, f.to, stamp)
	workDir := filepath.Join(f.outDir, workName)
	if err := os.MkdirAll(workDir, 0o700); err != nil {
		return errs.Wrap(errs.ExitUserError, "s4.audit_export.mkdir", "create out dir", err)
	}

	// Fresh per-bundle ed25519 key so auditor can verify in isolation without
	// trusting the global host key.
	pub, priv, err := auditchain.GenerateKey()
	if err != nil {
		return errs.Wrap(errs.ExitUserError, "s4.audit_export.keygen", "generate key", err)
	}
	chainPath := filepath.Join(workDir, "chain.jsonl")
	chain, err := auditchain.New(chainPath, priv)
	if err != nil {
		return errs.Wrap(errs.ExitUserError, "s4.audit_export.chain_open", "open chain", err)
	}

	envelope := map[string]any{
		"use_case":   uc.name,
		"service":    uc.service,
		"entity":     uc.entity,
		"date_field": uc.dateField,
		"from":       f.from,
		"to":         f.to,
		"sap_base":   base,
		"cred_label": s4Shared.credLabel,
	}
	if _, err := chain.Append("s4.audit_export.start", envelope); err != nil {
		return errs.Wrap(errs.ExitUserError, "s4.audit_export.envelope", "append envelope", err)
	}

	rowsPath := filepath.Join(workDir, "rows.jsonl")
	rowsFile, err := os.OpenFile(rowsPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
	if err != nil {
		return errs.Wrap(errs.ExitUserError, "s4.audit_export.rows_open", "open rows file", err)
	}

	rowSum := sha256.New()
	rowCount := 0
	servicePath := "/sap/opu/odata/sap/" + uc.service
	fullPath := servicePath + "/" + uc.entity

	q := url.Values{}
	q.Set("$format", "json")
	q.Set("$filter", fmt.Sprintf(
		"%s ge datetimeoffset'%sT00:00:00Z' and %s le datetimeoffset'%sT23:59:59Z'",
		uc.dateField, f.from, uc.dateField, f.to,
	))
	q.Set("$orderby", uc.dateField+" asc")
	if uc.selectField != "" {
		q.Set("$select", uc.selectField)
	}
	q.Set("$top", "1000")

	currentPath := fullPath
	currentQ := q
	for {
		var resp odataResponse
		if err := client.Get(ctx, currentPath, currentQ, &resp); err != nil {
			rowsFile.Close()
			return err
		}
		page, next, err := extractCollection(resp.D)
		if err != nil {
			rowsFile.Close()
			return errs.Wrap(errs.ExitUserError, "s4.audit_export.decode", "decode page", err)
		}
		for _, raw := range page {
			if rowCount >= f.maxRows {
				break
			}
			line := append([]byte(raw), '\n')
			if _, err := rowsFile.Write(line); err != nil {
				rowsFile.Close()
				return errs.Wrap(errs.ExitUserError, "s4.audit_export.rows_write", "write row", err)
			}
			rowSum.Write(line)
			rowHash := sha256.Sum256(raw)
			if _, err := chain.Append("sox.journal.row", map[string]any{
				"sha256": hex.EncodeToString(rowHash[:]),
				"seq":    rowCount + 1,
			}); err != nil {
				rowsFile.Close()
				return errs.Wrap(errs.ExitUserError, "s4.audit_export.chain_append", "append row event", err)
			}
			rowCount++
		}
		if next == "" || rowCount >= f.maxRows {
			break
		}
		nextP, err := nextPath(client.BaseURL, next)
		if err != nil {
			rowsFile.Close()
			return err
		}
		currentPath = nextP
		currentQ = nil
	}
	if err := rowsFile.Close(); err != nil {
		return errs.Wrap(errs.ExitUserError, "s4.audit_export.rows_close", "close rows", err)
	}

	if _, err := chain.Append("s4.audit_export.end", map[string]any{
		"row_count":   rowCount,
		"sha256_rows": hex.EncodeToString(rowSum.Sum(nil)),
	}); err != nil {
		return errs.Wrap(errs.ExitUserError, "s4.audit_export.envelope_end", "append end envelope", err)
	}

	pubPath := filepath.Join(workDir, "ed25519.pub")
	if err := auditchain.SavePublicKey(pubPath, pub); err != nil {
		return errs.Wrap(errs.ExitUserError, "s4.audit_export.pub", "save pub key", err)
	}

	chainBytes, err := os.ReadFile(chainPath)
	if err != nil {
		return errs.Wrap(errs.ExitUserError, "s4.audit_export.chain_read", "read chain", err)
	}
	chainSum := sha256.Sum256(chainBytes)

	manifest := map[string]any{
		"use_case":       uc.name,
		"description":    uc.description,
		"service":        uc.service,
		"entity":         uc.entity,
		"date_field":     uc.dateField,
		"from":           f.from,
		"to":             f.to,
		"row_count":      rowCount,
		"sha256_rows":    hex.EncodeToString(rowSum.Sum(nil)),
		"sha256_chain":   hex.EncodeToString(chainSum[:]),
		"generated_at":   stamp,
		"sapctl_version": "0.0.0-dev",
	}
	manifestPath := filepath.Join(workDir, "manifest.json")
	manifestBytes, _ := json.MarshalIndent(manifest, "", "  ")
	if err := os.WriteFile(manifestPath, manifestBytes, 0o600); err != nil {
		return errs.Wrap(errs.ExitUserError, "s4.audit_export.manifest", "write manifest", err)
	}

	var bundlePath string
	if f.bundle {
		bundlePath = workDir + ".tar.gz"
		if err := bundleDir(workDir, bundlePath); err != nil {
			return err
		}
	}

	out := map[string]any{
		"use_case":   uc.name,
		"from":       f.from,
		"to":         f.to,
		"row_count":  rowCount,
		"work_dir":   workDir,
		"manifest":   manifestPath,
		"chain":      chainPath,
		"public_key": pubPath,
	}
	if bundlePath != "" {
		out["bundle"] = bundlePath
	}
	if globalFlags.JSON {
		return writeJSON(cmd, out)
	}
	fmt.Fprintf(cmd.OutOrStdout(), "use case:   %s\nrows:       %d\nfrom:       %s\nto:         %s\nbundle:     %s\nwork dir:   %s\n",
		uc.name, rowCount, f.from, f.to, bundlePath, workDir)
	return nil
}

func listUseCases() string {
	names := make([]string, 0, len(useCases))
	for k := range useCases {
		names = append(names, k)
	}
	return strings.Join(names, ", ")
}

// bundleDir tar+gzips dir into dst. File mode 0o600 on dst.
func bundleDir(dir, dst string) error {
	out, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
	if err != nil {
		return errs.Wrap(errs.ExitUserError, "s4.audit_export.bundle_open", "open bundle", err)
	}
	defer out.Close()
	gzw := gzip.NewWriter(out)
	defer gzw.Close()
	tw := tar.NewWriter(gzw)
	defer tw.Close()

	root := filepath.Dir(dir)
	return filepath.Walk(dir, func(p string, info os.FileInfo, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		rel, err := filepath.Rel(root, p)
		if err != nil {
			return err
		}
		hdr, err := tar.FileInfoHeader(info, "")
		if err != nil {
			return err
		}
		hdr.Name = rel
		if err := tw.WriteHeader(hdr); err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		fp, err := os.Open(p)
		if err != nil {
			return err
		}
		defer fp.Close()
		_, err = io.Copy(tw, fp)
		return err
	})
}

// enforceRetainLicense gates --retain values that exceed the free-tier cap
// behind the Team-tier "audit-export-retain-365d" feature flag.
//
// - retainDays == 0 (flag unset)       -> allow; manifest will note default 30d
// - retainDays in 1..freeTierRetainDaysMax -> allow
// - retainDays >  freeTierRetainDaysMax    -> require license with feature
// - retainDays >  365                       -> always reject (hard upper bound)
func enforceRetainLicense(retainDays int) error {
	if retainDays <= 0 || retainDays <= freeTierRetainDaysMax {
		return nil
	}
	if retainDays > 365 {
		return errs.New(errs.ExitUserError, "s4.audit_export.retain.range",
			fmt.Sprintf("--retain %d exceeds hard maximum of 365 days", retainDays))
	}
	lic, err := license.LoadCurrent()
	if err != nil {
		return errs.Wrap(errs.ExitAuth, "s4.audit_export.retain.license_load",
			"verify license", err)
	}
	if !lic.Has("audit-export-retain-365d") {
		return errs.New(errs.ExitAuth, "s4.audit_export.retain.unlicensed",
			fmt.Sprintf("--retain %d requires Team-tier (feature 'audit-export-retain-365d'); "+
				"free-tier cap is %d days. Run `sapctl license install` after subscribing.",
				retainDays, freeTierRetainDaysMax))
	}
	return nil
}
