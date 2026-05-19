package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/dixitsheta/sapctl/apps/cli/internal/auth"
	"github.com/dixitsheta/sapctl/apps/cli/internal/errs"
	"github.com/dixitsheta/sapctl/apps/cli/internal/sap"
	sqlitemirror "github.com/dixitsheta/sapctl/packages/sqlite-mirror"
)

// odataResponse is the OData v2 envelope: { "d": { "results": [...] } } for
// collections, or { "d": {...} } for singletons. We tolerate both by decoding
// d as raw json.RawMessage and inspecting.
type odataResponse struct {
	D json.RawMessage `json:"d"`
}

type odataCollection struct {
	Results []json.RawMessage `json:"results"`
	Next    string            `json:"__next,omitempty"`
}

type s4ODataFlags struct {
	service    string
	entity     string
	top        int
	skip       int
	selectF    string
	filter     string
	orderBy    string
	expand     string
	mirrorDB   string // --mirror: write-through to local SQLite mirror
	keyField   string // --key-field: row field used as primary key in mirror
	sinceField string // --since-field: OData timestamp field for CDC watermark
	sinceReset bool   // --since-reset: ignore stored watermark for this run
	all        bool   // --all: follow d.__next pagination links until exhausted
	maxPages   int    // --max-pages: safety cap when --all is set
}

func newS4ODataCmd() *cobra.Command {
	c := &cobra.Command{
		Use:   "odata",
		Short: "Call OData v2 services on S/4HANA Cloud",
	}
	c.AddCommand(newS4ODataGetCmd())
	return c
}

func newS4ODataGetCmd() *cobra.Command {
	var f s4ODataFlags
	c := &cobra.Command{
		Use:   "get",
		Short: "Fetch rows from an OData entity",
		Long: `Fetch rows from an OData entity. --service may be a bare service
name (auto-prefixed to /sap/opu/odata/sap/<name>) or a full path beginning
with /.

Examples:
  sapctl s4 odata get --cred sandbox --service API_BUSINESS_PARTNER \
    --entity A_BusinessPartner --top 2

  sapctl s4 odata get --cred sandbox --service API_PRODUCT_SRV \
    --entity A_Product --top 5 --select-fields 'Product,ProductType'`,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, cancel := context.WithTimeout(cmd.Context(), 120*time.Second)
			defer cancel()
			return runS4ODataGet(ctx, cmd, f)
		},
	}
	c.Flags().StringVar(&f.service, "service", "", "OData service name or full path (required)")
	c.Flags().StringVar(&f.entity, "entity", "", "entity set, e.g. A_BusinessPartner (required)")
	c.Flags().IntVar(&f.top, "top", 25, "$top")
	c.Flags().IntVar(&f.skip, "skip", 0, "$skip")
	c.Flags().StringVar(&f.selectF, "select-fields", "", "$select projection (comma-separated)")
	c.Flags().StringVar(&f.filter, "filter", "", "$filter expression")
	c.Flags().StringVar(&f.orderBy, "order-by", "", "$orderby expression")
	c.Flags().StringVar(&f.expand, "expand", "", "$expand navigation properties")
	c.Flags().StringVar(&f.mirrorDB, "mirror", "", "if set, write rows to local SQLite mirror at this path")
	c.Flags().StringVar(&f.keyField, "key-field", "", "row field name used as mirror primary key (e.g. BusinessPartner)")
	c.Flags().StringVar(&f.sinceField, "since-field", "", "OData timestamp field for CDC watermark (e.g. LastChangeDateTime); requires --mirror")
	c.Flags().BoolVar(&f.sinceReset, "since-reset", false, "ignore stored watermark for this run (full refresh)")
	c.Flags().BoolVar(&f.all, "all", false, "follow d.__next pagination until exhausted")
	c.Flags().IntVar(&f.maxPages, "max-pages", 100, "safety cap on pages when --all is set")
	_ = c.MarkFlagRequired("service")
	_ = c.MarkFlagRequired("entity")
	return c
}

func runS4ODataGet(ctx context.Context, cmd *cobra.Command, f s4ODataFlags) error {
	if s4Shared.credLabel == "" {
		return errs.New(errs.ExitUserError, "s4.odata.cred_required",
			"--cred is required; run `sapctl auth login` first")
	}

	cfgPath, err := auth.DefaultPath()
	if err != nil {
		return errs.Wrap(errs.ExitUserError, "s4.odata.cache_path", "resolve cache path", err)
	}
	cache := auth.NewCache(cfgPath)
	cfg, err := cache.Load(s4Shared.credLabel)
	if err != nil {
		return err
	}
	provider, err := auth.New(cfg)
	if err != nil {
		return errs.Wrap(errs.ExitUserError, "s4.odata.provider", "build provider", err)
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

	servicePath := f.service
	if !strings.HasPrefix(servicePath, "/") {
		servicePath = "/sap/opu/odata/sap/" + servicePath
	}
	fullPath := strings.TrimRight(servicePath, "/") + "/" + strings.TrimLeft(f.entity, "/")

	// Resolve effective $filter: user-supplied --filter combined (AND) with a
	// watermark predicate if --since-field + --mirror were given and a prior
	// watermark exists in the mirror.
	priorWatermark, err := readPriorWatermark(ctx, f)
	if err != nil {
		return err
	}

	q := url.Values{}
	q.Set("$format", "json")
	if f.top > 0 {
		q.Set("$top", strconv.Itoa(f.top))
	}
	if f.skip > 0 {
		q.Set("$skip", strconv.Itoa(f.skip))
	}
	if v := strings.TrimSpace(f.selectF); v != "" {
		q.Set("$select", v)
	}

	effectiveFilter := buildEffectiveFilter(f, priorWatermark)
	if effectiveFilter != "" {
		q.Set("$filter", effectiveFilter)
	}
	if v := strings.TrimSpace(f.orderBy); v != "" {
		q.Set("$orderby", v)
	} else if f.sinceField != "" {
		// Stable ordering by since-field lets us pick max safely.
		q.Set("$orderby", f.sinceField+" asc")
	}
	if v := strings.TrimSpace(f.expand); v != "" {
		q.Set("$expand", v)
	}

	rows, err := fetchPaged(ctx, client, fullPath, q, f)
	if err != nil {
		return err
	}

	if f.mirrorDB != "" {
		if err := writeMirror(ctx, f.mirrorDB, f.service, f.entity, f.keyField, rows); err != nil {
			return err
		}
		if f.sinceField != "" {
			if err := advanceWatermark(ctx, f, rows); err != nil {
				return err
			}
		}
	}

	return emitODataRows(cmd, rows)
}

// fetchPaged issues the first request and, if --all is set, follows the
// d.__next URL until exhausted or --max-pages is reached. The first request
// uses the supplied query; subsequent requests use the full __next URL
// returned by the server (which already encodes $skiptoken etc.).
func fetchPaged(ctx context.Context, client *sap.Client, firstPath string, q url.Values, f s4ODataFlags) ([]json.RawMessage, error) {
	var rows []json.RawMessage
	var resp odataResponse

	if err := client.Get(ctx, firstPath, q, &resp); err != nil {
		return nil, err
	}
	page, next, err := extractCollection(resp.D)
	if err != nil {
		return nil, errs.Wrap(errs.ExitUserError, "s4.odata.decode_d", "decode OData d envelope", err)
	}
	rows = append(rows, page...)

	if !f.all {
		return rows, nil
	}

	for pageNum := 1; next != "" && pageNum < f.maxPages; pageNum++ {
		// __next can be absolute or relative. If absolute and matches BaseURL,
		// strip the prefix; if absolute and different host, refuse (SSRF guard).
		path, err := nextPath(client.BaseURL, next)
		if err != nil {
			return rows, err
		}
		var nresp odataResponse
		if err := client.Get(ctx, path, nil, &nresp); err != nil {
			return rows, err
		}
		npage, nnext, err := extractCollection(nresp.D)
		if err != nil {
			return rows, errs.Wrap(errs.ExitUserError, "s4.odata.decode_d", "decode OData d envelope", err)
		}
		rows = append(rows, npage...)
		next = nnext
	}
	return rows, nil
}

// extractCollection returns (rows, nextURL) from an OData v2 `d` envelope.
// If d encodes a single entity (no `results`), wraps it as a one-row page
// and returns empty next.
func extractCollection(d json.RawMessage) ([]json.RawMessage, string, error) {
	trimmed := strings.TrimSpace(string(d))
	if strings.HasPrefix(trimmed, "{") && strings.Contains(trimmed, `"results"`) {
		var coll odataCollection
		if err := json.Unmarshal(d, &coll); err != nil {
			return nil, "", err
		}
		return coll.Results, coll.Next, nil
	}
	return []json.RawMessage{d}, "", nil
}

// nextPath maps a __next link (absolute or path-relative) into a path safe to
// pass back through sap.Client (which prepends BaseURL). Refuses cross-host
// jumps as a coarse SSRF guard.
func nextPath(baseURL, next string) (string, error) {
	if strings.HasPrefix(next, "/") {
		return next, nil
	}
	if !strings.HasPrefix(next, "http://") && !strings.HasPrefix(next, "https://") {
		return "/" + next, nil
	}
	if !strings.HasPrefix(next, baseURL) {
		return "", errs.New(errs.ExitUserError, "s4.odata.paging.crosshost",
			"refusing to follow __next link to different host: "+next)
	}
	return strings.TrimPrefix(next, baseURL), nil
}

// readPriorWatermark returns the stored cursor for this (service, entity) if
// CDC is enabled (--mirror + --since-field) and --since-reset is not set.
func readPriorWatermark(ctx context.Context, f s4ODataFlags) (string, error) {
	if f.mirrorDB == "" || f.sinceField == "" || f.sinceReset {
		return "", nil
	}
	store, err := sqlitemirror.Open(f.mirrorDB)
	if err != nil {
		return "", errs.Wrap(errs.ExitUserError, "s4.odata.since.open", "open mirror", err)
	}
	defer store.Close()
	w, err := store.GetWatermark(ctx, f.service, f.entity)
	if err != nil {
		return "", errs.Wrap(errs.ExitUserError, "s4.odata.since.read", "read watermark", err)
	}
	return w, nil
}

// buildEffectiveFilter combines user $filter with a `<since-field> gt
// datetimeoffset'<watermark>'` predicate. OData v2 datetime literals: SAP
// expects datetimeoffset'...' for tz-aware timestamps.
func buildEffectiveFilter(f s4ODataFlags, watermark string) string {
	user := strings.TrimSpace(f.filter)
	if f.sinceField == "" || watermark == "" {
		return user
	}
	pred := f.sinceField + " gt datetimeoffset'" + watermark + "'"
	if user == "" {
		return pred
	}
	return "(" + user + ") and (" + pred + ")"
}

// advanceWatermark scans returned rows for the maximum value of the
// since-field and stores it as the new cursor. If no row carries the field,
// the watermark is left unchanged.
func advanceWatermark(ctx context.Context, f s4ODataFlags, rows []json.RawMessage) error {
	maxVal := ""
	for _, raw := range rows {
		var m map[string]json.RawMessage
		if err := json.Unmarshal(raw, &m); err != nil {
			continue
		}
		v, ok := m[f.sinceField]
		if !ok {
			continue
		}
		s := strings.Trim(string(v), `"`)
		if s > maxVal {
			maxVal = s
		}
	}
	if maxVal == "" {
		return nil
	}
	store, err := sqlitemirror.Open(f.mirrorDB)
	if err != nil {
		return errs.Wrap(errs.ExitUserError, "s4.odata.since.open2", "open mirror", err)
	}
	defer store.Close()
	if err := store.SetWatermark(ctx, f.service, f.entity, maxVal); err != nil {
		return errs.Wrap(errs.ExitUserError, "s4.odata.since.write", "set watermark", err)
	}
	return nil
}

func writeMirror(ctx context.Context, dbPath, service, entity, keyField string, rows []json.RawMessage) error {
	store, err := sqlitemirror.Open(dbPath)
	if err != nil {
		return errs.Wrap(errs.ExitUserError, "s4.odata.mirror.open", "open mirror", err)
	}
	defer store.Close()
	for i, raw := range rows {
		key := ""
		if keyField != "" {
			var m map[string]json.RawMessage
			if err := json.Unmarshal(raw, &m); err == nil {
				if v, ok := m[keyField]; ok {
					key = strings.Trim(string(v), `"`)
				}
			}
		}
		if key == "" {
			key = fmt.Sprintf("auto-%d", i)
		}
		if err := store.Upsert(ctx, service, entity, key, raw); err != nil {
			return errs.Wrap(errs.ExitUserError, "s4.odata.mirror.upsert", "upsert mirror row", err)
		}
	}
	return nil
}

// extractRows is the pre-pagination adapter retained for backward
// compatibility in tests. Prefer extractCollection.
func extractRows(d json.RawMessage) ([]json.RawMessage, error) {
	rows, _, err := extractCollection(d)
	return rows, err
}

func emitODataRows(cmd *cobra.Command, rows []json.RawMessage) error {
	enc := json.NewEncoder(cmd.OutOrStdout())
	if !globalFlags.Compact {
		enc.SetIndent("", "  ")
	}
	return enc.Encode(map[string]any{
		"count": len(rows),
		"rows":  rows,
	})
}
