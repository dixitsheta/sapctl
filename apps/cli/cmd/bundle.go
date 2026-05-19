package cmd

import (
	"archive/tar"
	"compress/gzip"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/dixitsheta/sapctl/apps/cli/internal/errs"
	auditchain "github.com/dixitsheta/sapctl/packages/audit-chain"
)

// bundleManifest is the descriptor written at the root of every air-gap
// bundle. It is itself signed (signature in manifest.intoto.jsonl.sig).
type bundleManifest struct {
	Name          string       `json:"name"`
	Version       string       `json:"version"`
	CreatedAt     string       `json:"created_at"`
	SapctlVersion string       `json:"sapctl_version"`
	Files         []bundleFile `json:"files"`
	SignatureAlg  string       `json:"signature_alg"`
	PublicKeyB64  string       `json:"public_key_b64"`
}

type bundleFile struct {
	Path   string `json:"path"`
	SHA256 string `json:"sha256"`
	Size   int64  `json:"size"`
}

type intotoSubject struct {
	Name   string            `json:"name"`
	Digest map[string]string `json:"digest"`
}

type intotoStatement struct {
	Type          string          `json:"_type"`
	PredicateType string          `json:"predicateType"`
	Subject       []intotoSubject `json:"subject"`
	Predicate     map[string]any  `json:"predicate"`
}

func newBundleCmd() *cobra.Command {
	c := &cobra.Command{
		Use:   "bundle",
		Short: "Air-gap export / install / verify of sapctl recipe + spec bundles",
		Long: `Bundle a portable set of sapctl assets (recipe definitions, OpenAPI specs,
optionally the sapctl binary) for transfer into an air-gapped network. Each
bundle is signed with a per-bundle ed25519 key and shipped with an in-toto
v1 manifest so consumers can verify the contents without sapctl already
installed.

Bundle layout:
  bundle.json                 descriptor + signed file list + public key
  manifest.intoto.jsonl       in-toto v1 statement (subjects = sha256 per file)
  manifest.intoto.jsonl.sig   base64 ed25519 signature over manifest.intoto.jsonl
  ed25519.pub                 base64 verifier key
  payload/<file>              the actual files`,
	}
	c.AddCommand(newBundleExportCmd())
	c.AddCommand(newBundleInstallCmd())
	c.AddCommand(newBundleVerifyCmd())
	return c
}

type bundleExportFlags struct {
	name    string
	version string
	include string
	dirs    []string
	out     string
}

func newBundleExportCmd() *cobra.Command {
	var f bundleExportFlags
	c := &cobra.Command{
		Use:   "export",
		Short: "Create a signed air-gap bundle",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runBundleExport(cmd, f)
		},
	}
	c.Flags().StringVar(&f.name, "name", "sapctl-bundle", "bundle name")
	c.Flags().StringVar(&f.version, "version", "0.1.0", "bundle version (semver)")
	c.Flags().StringVar(&f.include, "include", "", "comma-list of preset categories (specs,recipes,binary)")
	c.Flags().StringSliceVar(&f.dirs, "dir", nil, "extra directories to include (repeatable)")
	c.Flags().StringVar(&f.out, "out", "", "output .tar.gz path (required)")
	_ = c.MarkFlagRequired("out")
	return c
}

func runBundleExport(cmd *cobra.Command, f bundleExportFlags) error {
	stage, err := os.MkdirTemp("", "sapctl-bundle-")
	if err != nil {
		return errs.Wrap(errs.ExitUserError, "bundle.export.tmp", "create stage dir", err)
	}
	defer os.RemoveAll(stage)
	payload := filepath.Join(stage, "payload")
	if err := os.MkdirAll(payload, 0o700); err != nil {
		return errs.Wrap(errs.ExitUserError, "bundle.export.mkpayload", "mkdir payload", err)
	}

	srcs := []string{}
	if f.include != "" {
		for _, cat := range strings.Split(f.include, ",") {
			cat = strings.TrimSpace(cat)
			switch cat {
			case "specs":
				srcs = append(srcs, "specs")
			case "recipes":
				srcs = append(srcs, "docs/adr")
			case "binary":
				if exe, err := os.Executable(); err == nil {
					srcs = append(srcs, exe)
				}
			}
		}
	}
	srcs = append(srcs, f.dirs...)

	if len(srcs) == 0 {
		return errs.New(errs.ExitUserError, "bundle.export.empty",
			"nothing to bundle: pass --include and/or --dir")
	}

	for _, src := range srcs {
		if _, err := os.Stat(src); os.IsNotExist(err) {
			continue
		}
		if err := copyIntoPayload(src, payload); err != nil {
			return errs.Wrap(errs.ExitUserError, "bundle.export.copy",
				"copy "+src+" into payload", err)
		}
	}

	files, err := hashTree(payload)
	if err != nil {
		return errs.Wrap(errs.ExitUserError, "bundle.export.hash", "hash payload", err)
	}
	if len(files) == 0 {
		return errs.New(errs.ExitUserError, "bundle.export.empty_payload",
			"payload is empty after copy; check --include / --dir paths exist")
	}

	pub, priv, err := auditchain.GenerateKey()
	if err != nil {
		return errs.Wrap(errs.ExitUserError, "bundle.export.keygen", "generate key", err)
	}

	manifest := bundleManifest{
		Name:          f.name,
		Version:       f.version,
		CreatedAt:     time.Now().UTC().Format(time.RFC3339),
		SapctlVersion: rootVersion(),
		Files:         files,
		SignatureAlg:  "ed25519",
		PublicKeyB64:  base64.StdEncoding.EncodeToString(pub),
	}
	manifestBytes, _ := json.MarshalIndent(manifest, "", "  ")
	if err := os.WriteFile(filepath.Join(stage, "bundle.json"), manifestBytes, 0o600); err != nil {
		return errs.Wrap(errs.ExitUserError, "bundle.export.write_manifest", "write bundle.json", err)
	}

	subjects := make([]intotoSubject, 0, len(files))
	for _, fl := range files {
		subjects = append(subjects, intotoSubject{
			Name:   fl.Path,
			Digest: map[string]string{"sha256": fl.SHA256},
		})
	}
	statement := intotoStatement{
		Type:          "https://in-toto.io/Statement/v1",
		PredicateType: "https://sapctl.dev/bundle/v1",
		Subject:       subjects,
		Predicate: map[string]any{
			"bundle_name":    f.name,
			"bundle_version": f.version,
			"sapctl_version": rootVersion(),
			"created_at":     manifest.CreatedAt,
		},
	}
	stmtBytes, _ := json.Marshal(statement)
	stmtPath := filepath.Join(stage, "manifest.intoto.jsonl")
	if err := os.WriteFile(stmtPath, append(stmtBytes, '\n'), 0o600); err != nil {
		return errs.Wrap(errs.ExitUserError, "bundle.export.write_intoto", "write manifest.intoto.jsonl", err)
	}

	sig := ed25519.Sign(priv, stmtBytes)
	sigPath := filepath.Join(stage, "manifest.intoto.jsonl.sig")
	if err := os.WriteFile(sigPath, []byte(base64.StdEncoding.EncodeToString(sig)), 0o600); err != nil {
		return errs.Wrap(errs.ExitUserError, "bundle.export.write_sig", "write signature", err)
	}

	pubPath := filepath.Join(stage, "ed25519.pub")
	if err := auditchain.SavePublicKey(pubPath, pub); err != nil {
		return errs.Wrap(errs.ExitUserError, "bundle.export.write_pub", "write pub key", err)
	}

	if err := writeTarGz(stage, f.out); err != nil {
		return errs.Wrap(errs.ExitUserError, "bundle.export.tar", "write tar.gz", err)
	}

	if globalFlags.JSON {
		return writeJSON(cmd, map[string]any{
			"bundle":     f.out,
			"name":       f.name,
			"version":    f.version,
			"file_count": len(files),
		})
	}
	fmt.Fprintf(cmd.OutOrStdout(), "bundle:     %s\nname:       %s\nversion:    %s\nfile count: %d\n",
		f.out, f.name, f.version, len(files))
	return nil
}

type bundleInstallFlags struct {
	bundle string
	dest   string
}

func newBundleInstallCmd() *cobra.Command {
	var f bundleInstallFlags
	c := &cobra.Command{
		Use:   "install",
		Short: "Verify + extract an air-gap bundle into a destination directory",
		RunE: func(cmd *cobra.Command, args []string) error {
			if f.bundle == "" || f.dest == "" {
				return errs.New(errs.ExitUserError, "bundle.install.args",
					"--bundle and --dest are required")
			}
			tmp, err := os.MkdirTemp("", "sapctl-install-")
			if err != nil {
				return errs.Wrap(errs.ExitUserError, "bundle.install.tmp", "tmpdir", err)
			}
			defer os.RemoveAll(tmp)
			if err := extractTarGz(f.bundle, tmp); err != nil {
				return errs.Wrap(errs.ExitUserError, "bundle.install.extract", "extract", err)
			}
			if err := verifyExtracted(tmp); err != nil {
				return err
			}
			if err := os.MkdirAll(f.dest, 0o700); err != nil {
				return errs.Wrap(errs.ExitUserError, "bundle.install.mkdest", "mkdir dest", err)
			}
			payload := filepath.Join(tmp, "payload")
			if err := copyDir(payload, f.dest); err != nil {
				return errs.Wrap(errs.ExitUserError, "bundle.install.copy", "copy payload to dest", err)
			}
			if globalFlags.JSON {
				return writeJSON(cmd, map[string]any{
					"bundle":   f.bundle,
					"dest":     f.dest,
					"verified": true,
				})
			}
			fmt.Fprintf(cmd.OutOrStdout(), "verified + installed bundle to %s\n", f.dest)
			return nil
		},
	}
	c.Flags().StringVar(&f.bundle, "bundle", "", ".tar.gz bundle path (required)")
	c.Flags().StringVar(&f.dest, "dest", "", "destination directory (required)")
	_ = c.MarkFlagRequired("bundle")
	_ = c.MarkFlagRequired("dest")
	return c
}

func newBundleVerifyCmd() *cobra.Command {
	var bundlePath string
	c := &cobra.Command{
		Use:   "verify",
		Short: "Verify a bundle's in-toto signature + file hashes without installing",
		RunE: func(cmd *cobra.Command, args []string) error {
			if bundlePath == "" {
				return errs.New(errs.ExitUserError, "bundle.verify.args", "--bundle is required")
			}
			tmp, err := os.MkdirTemp("", "sapctl-verify-")
			if err != nil {
				return errs.Wrap(errs.ExitUserError, "bundle.verify.tmp", "tmpdir", err)
			}
			defer os.RemoveAll(tmp)
			if err := extractTarGz(bundlePath, tmp); err != nil {
				return errs.Wrap(errs.ExitUserError, "bundle.verify.extract", "extract", err)
			}
			if err := verifyExtracted(tmp); err != nil {
				return err
			}
			if globalFlags.JSON {
				return writeJSON(cmd, map[string]any{"bundle": bundlePath, "verified": true})
			}
			fmt.Fprintln(cmd.OutOrStdout(), "ok: bundle signature + file hashes verified")
			return nil
		},
	}
	c.Flags().StringVar(&bundlePath, "bundle", "", ".tar.gz bundle path (required)")
	_ = c.MarkFlagRequired("bundle")
	return c
}

func verifyExtracted(dir string) error {
	pubBytes, err := os.ReadFile(filepath.Join(dir, "ed25519.pub"))
	if err != nil {
		return errs.Wrap(errs.ExitConflict, "bundle.verify.pub_missing", "missing ed25519.pub", err)
	}
	pubRaw, err := base64.StdEncoding.DecodeString(strings.TrimSpace(string(pubBytes)))
	if err != nil || len(pubRaw) != ed25519.PublicKeySize {
		return errs.New(errs.ExitConflict, "bundle.verify.pub_decode", "ed25519.pub malformed")
	}
	pub := ed25519.PublicKey(pubRaw)

	stmtBytes, err := os.ReadFile(filepath.Join(dir, "manifest.intoto.jsonl"))
	if err != nil {
		return errs.Wrap(errs.ExitConflict, "bundle.verify.stmt_missing", "missing manifest.intoto.jsonl", err)
	}
	signed := stmtBytes
	if n := len(signed); n > 0 && signed[n-1] == '\n' {
		signed = signed[:n-1]
	}
	sigB64, err := os.ReadFile(filepath.Join(dir, "manifest.intoto.jsonl.sig"))
	if err != nil {
		return errs.Wrap(errs.ExitConflict, "bundle.verify.sig_missing", "missing signature", err)
	}
	sig, err := base64.StdEncoding.DecodeString(strings.TrimSpace(string(sigB64)))
	if err != nil {
		return errs.New(errs.ExitConflict, "bundle.verify.sig_decode", "signature malformed base64")
	}
	if !ed25519.Verify(pub, signed, sig) {
		return errs.New(errs.ExitConflict, "bundle.verify.sig_invalid",
			"in-toto manifest signature does not verify against ed25519.pub")
	}

	var stmt intotoStatement
	if err := json.Unmarshal(signed, &stmt); err != nil {
		return errs.Wrap(errs.ExitConflict, "bundle.verify.stmt_parse", "parse in-toto statement", err)
	}
	for _, sub := range stmt.Subject {
		want := sub.Digest["sha256"]
		if want == "" {
			continue
		}
		fp := filepath.Join(dir, "payload", sub.Name)
		got, err := sha256File(fp)
		if err != nil {
			return errs.Wrap(errs.ExitConflict, "bundle.verify.hash_read",
				"hash payload "+sub.Name, err)
		}
		if got != want {
			return errs.New(errs.ExitConflict, "bundle.verify.hash_mismatch",
				fmt.Sprintf("hash mismatch for %s: want %s got %s", sub.Name, want, got))
		}
	}
	return nil
}

func copyIntoPayload(src, payloadRoot string) error {
	info, err := os.Stat(src)
	if err != nil {
		return err
	}
	base := filepath.Base(src)
	if info.IsDir() {
		dst := filepath.Join(payloadRoot, base)
		return copyDir(src, dst)
	}
	dst := filepath.Join(payloadRoot, base)
	return copyFile(src, dst)
}

func copyFile(src, dst string) error {
	if err := os.MkdirAll(filepath.Dir(dst), 0o700); err != nil {
		return err
	}
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
	if err != nil {
		return err
	}
	defer out.Close()
	_, err = io.Copy(out, in)
	return err
}

func copyDir(src, dst string) error {
	return filepath.Walk(src, func(p string, info os.FileInfo, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		rel, err := filepath.Rel(src, p)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, rel)
		if info.IsDir() {
			return os.MkdirAll(target, 0o700)
		}
		return copyFile(p, target)
	})
}

func hashTree(root string) ([]bundleFile, error) {
	var out []bundleFile
	err := filepath.Walk(root, func(p string, info os.FileInfo, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if info.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(root, p)
		if err != nil {
			return err
		}
		h, err := sha256File(p)
		if err != nil {
			return err
		}
		out = append(out, bundleFile{Path: rel, SHA256: h, Size: info.Size()})
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Path < out[j].Path })
	return out, nil
}

func sha256File(p string) (string, error) {
	f, err := os.Open(p)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

func writeTarGz(srcDir, dstFile string) error {
	out, err := os.OpenFile(dstFile, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
	if err != nil {
		return err
	}
	defer out.Close()
	gz := gzip.NewWriter(out)
	defer gz.Close()
	tw := tar.NewWriter(gz)
	defer tw.Close()

	return filepath.Walk(srcDir, func(p string, info os.FileInfo, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		rel, err := filepath.Rel(srcDir, p)
		if err != nil {
			return err
		}
		if rel == "." {
			return nil
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

func extractTarGz(srcFile, dstDir string) error {
	f, err := os.Open(srcFile)
	if err != nil {
		return err
	}
	defer f.Close()
	gz, err := gzip.NewReader(f)
	if err != nil {
		return err
	}
	defer gz.Close()
	tr := tar.NewReader(gz)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
		if strings.Contains(hdr.Name, "..") || strings.HasPrefix(hdr.Name, "/") {
			return errs.New(errs.ExitConflict, "bundle.tar.traversal",
				"refusing unsafe tar entry: "+hdr.Name)
		}
		target := filepath.Join(dstDir, hdr.Name)
		switch hdr.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(target, 0o700); err != nil {
				return err
			}
		case tar.TypeReg:
			if err := os.MkdirAll(filepath.Dir(target), 0o700); err != nil {
				return err
			}
			out, err := os.OpenFile(target, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
			if err != nil {
				return err
			}
			if _, err := io.Copy(out, tr); err != nil {
				out.Close()
				return err
			}
			out.Close()
		}
	}
	return nil
}
