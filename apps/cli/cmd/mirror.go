package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"time"

	"github.com/spf13/cobra"

	"github.com/dixitsheta/sapctl/apps/cli/internal/errs"
	sqlitemirror "github.com/dixitsheta/sapctl/packages/sqlite-mirror"
)

type mirrorFlags struct {
	db      string
	service string
	entity  string
	limit   int
	search  string
}

func newMirrorCmd() *cobra.Command {
	c := &cobra.Command{
		Use:   "mirror",
		Short: "Query the local SQLite mirror of fetched SAP data",
	}
	c.AddCommand(newMirrorListCmd())
	c.AddCommand(newMirrorSearchCmd())
	c.AddCommand(newMirrorStatsCmd())
	c.AddCommand(newMirrorSetWatermarkCmd())
	c.AddCommand(newMirrorResetCmd())
	c.AddCommand(newMirrorDiscoverCmd())
	c.AddCommand(newMirrorQueryCmd())
	return c
}

// resolveMirrorDB returns the effective mirror DB path. If f.db is empty,
// it computes the default from service+entity via sqlitemirror.DefaultPath.
func resolveMirrorDB(f mirrorFlags) (string, error) {
	if f.db != "" {
		return f.db, nil
	}
	return sqlitemirror.DefaultPath(f.service, f.entity)
}

func newMirrorDiscoverCmd() *cobra.Command {
	var db string
	c := &cobra.Command{
		Use:   "discover",
		Short: "List all mirrored services and entities",
		Long: `Enumerate every (service, entity) pair stored in the mirror
directory with row counts and watermarks. Scans all .db files under
~/.config/sapctl/mirror/ unless --db is given.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, cancel := context.WithTimeout(cmd.Context(), 30*time.Second)
			defer cancel()

			// If --db is given, query that single file.
			if db != "" {
				return discoverFromFile(ctx, cmd, db)
			}

			// Otherwise scan the default mirror directory for .db files.
			dir, err := sqlitemirror.DefaultDir()
			if err != nil {
				return errs.Wrap(errs.ExitUserError, "mirror.discover.dir", "resolve default dir", err)
			}
			entries, err := filepath.Glob(filepath.Join(dir, "*.db"))
			if err != nil {
				return errs.Wrap(errs.ExitUserError, "mirror.discover.glob", "glob mirror dir", err)
			}
			if len(entries) == 0 {
				if globalFlags.JSON {
					return writeJSON(cmd, map[string]any{"count": 0, "services": []any{}})
				}
				fmt.Fprintln(cmd.OutOrStdout(), "no mirrored services found in "+dir)
				return nil
			}

			var all []sqlitemirror.ServiceInfo
			for _, f := range entries {
				s, err := sqlitemirror.Open(f)
				if err != nil {
					continue // skip unopenable DBs
				}
				services, err := s.ListServices(ctx)
				_ = s.Close()
				if err != nil {
					continue
				}
				all = append(all, services...)
			}

			if globalFlags.JSON {
				return writeJSON(cmd, map[string]any{
					"count":    len(all),
					"services": all,
				})
			}
			if len(all) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "no mirrored services found")
				return nil
			}
			for _, si := range all {
				w := si.Watermark
				if w == "" {
					w = "-"
				}
				fmt.Fprintf(cmd.OutOrStdout(), "%-30s %-30s %6d rows  watermark=%s\n",
					si.Service, si.Entity, si.RowCount, w)
			}
			return nil
		},
	}
	c.Flags().StringVar(&db, "db", "", "single mirror DB path (default: scan ~/.config/sapctl/mirror/*.db)")
	return c
}

// discoverFromFile queries ListServices from a single mirror DB file.
func discoverFromFile(ctx context.Context, cmd *cobra.Command, path string) error {
	s, err := sqlitemirror.Open(path)
	if err != nil {
		return errs.Wrap(errs.ExitUserError, "mirror.discover.open", "open mirror", err)
	}
	defer s.Close()
	services, err := s.ListServices(ctx)
	if err != nil {
		return errs.Wrap(errs.ExitUserError, "mirror.discover.query", "list services", err)
	}
	if globalFlags.JSON {
		return writeJSON(cmd, map[string]any{
			"count":    len(services),
			"services": services,
		})
	}
	if len(services) == 0 {
		fmt.Fprintln(cmd.OutOrStdout(), "no mirrored services found")
		return nil
	}
	for _, si := range services {
		w := si.Watermark
		if w == "" {
			w = "-"
		}
		fmt.Fprintf(cmd.OutOrStdout(), "%-30s %-30s %6d rows  watermark=%s\n",
			si.Service, si.Entity, si.RowCount, w)
	}
	return nil
}

func newMirrorSetWatermarkCmd() *cobra.Command {
	var f mirrorFlags
	var since string
	c := &cobra.Command{
		Use:   "set-watermark",
		Short: "Set the CDC cursor for a service+entity",
		Long:  "Manually override the watermark used by `sapctl s4 odata get --since-field`.",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, cancel := context.WithTimeout(cmd.Context(), 10*time.Second)
			defer cancel()
			dbPath, err := resolveMirrorDB(f)
			if err != nil {
				return errs.Wrap(errs.ExitUserError, "mirror.set-watermark.path", "resolve db path", err)
			}
			s, err := sqlitemirror.Open(dbPath)
			if err != nil {
				return errs.Wrap(errs.ExitUserError, "mirror.set-watermark.open", "open mirror", err)
			}
			defer s.Close()
			if err := s.SetWatermark(ctx, f.service, f.entity, since); err != nil {
				return errs.Wrap(errs.ExitUserError, "mirror.set-watermark.write", "set watermark", err)
			}
			if globalFlags.JSON {
				return writeJSON(cmd, map[string]any{
					"service": f.service, "entity": f.entity, "watermark": since,
				})
			}
			fmt.Fprintf(cmd.OutOrStdout(), "watermark set: %s/%s -> %q\n", f.service, f.entity, since)
			return nil
		},
	}
	c.Flags().StringVar(&f.db, "db", "", "mirror DB path (default: ~/.config/sapctl/mirror/<service>_<entity>.db)")
	c.Flags().StringVar(&f.service, "service", "", "service (required)")
	c.Flags().StringVar(&f.entity, "entity", "", "entity (required)")
	c.Flags().StringVar(&since, "since", "", "new watermark value (empty = clear)")
	_ = c.MarkFlagRequired("service")
	_ = c.MarkFlagRequired("entity")
	return c
}

func newMirrorResetCmd() *cobra.Command {
	var f mirrorFlags
	var yes bool
	c := &cobra.Command{
		Use:   "reset",
		Short: "Wipe all rows + watermark for a service+entity",
		Long:  "Destructive: removes mirrored rows and the CDC cursor. Requires --yes.",
		RunE: func(cmd *cobra.Command, args []string) error {
			if !yes && !globalFlags.Yes {
				return errs.New(errs.ExitUserError, "mirror.reset.confirm",
					"reset is destructive; pass --yes to confirm")
			}
			ctx, cancel := context.WithTimeout(cmd.Context(), 30*time.Second)
			defer cancel()
			dbPath, err := resolveMirrorDB(f)
			if err != nil {
				return errs.Wrap(errs.ExitUserError, "mirror.reset.path", "resolve db path", err)
			}
			s, err := sqlitemirror.Open(dbPath)
			if err != nil {
				return errs.Wrap(errs.ExitUserError, "mirror.reset.open", "open mirror", err)
			}
			defer s.Close()
			n, err := s.Delete(ctx, f.service, f.entity)
			if err != nil {
				return errs.Wrap(errs.ExitUserError, "mirror.reset.delete", "delete rows", err)
			}
			if globalFlags.JSON {
				return writeJSON(cmd, map[string]any{
					"service": f.service, "entity": f.entity, "deleted": n,
				})
			}
			fmt.Fprintf(cmd.OutOrStdout(), "deleted %d rows + watermark for %s/%s\n", n, f.service, f.entity)
			return nil
		},
	}
	c.Flags().StringVar(&f.db, "db", "", "mirror DB path (default: ~/.config/sapctl/mirror/<service>_<entity>.db)")
	c.Flags().StringVar(&f.service, "service", "", "service (required)")
	c.Flags().StringVar(&f.entity, "entity", "", "entity (required)")
	c.Flags().BoolVar(&yes, "yes", false, "confirm destructive operation")
	_ = c.MarkFlagRequired("service")
	_ = c.MarkFlagRequired("entity")
	return c
}

func newMirrorListCmd() *cobra.Command {
	var f mirrorFlags
	c := &cobra.Command{
		Use:   "list",
		Short: "List rows in the mirror for a service+entity",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, cancel := context.WithTimeout(cmd.Context(), 30*time.Second)
			defer cancel()
			dbPath, err := resolveMirrorDB(f)
			if err != nil {
				return errs.Wrap(errs.ExitUserError, "mirror.list.path", "resolve db path", err)
			}
			s, err := sqlitemirror.Open(dbPath)
			if err != nil {
				return errs.Wrap(errs.ExitUserError, "mirror.list.open", "open mirror", err)
			}
			defer s.Close()
			rows, err := s.List(ctx, f.service, f.entity, f.limit)
			if err != nil {
				return errs.Wrap(errs.ExitUserError, "mirror.list.query", "list rows", err)
			}
			return emitMirrorRows(cmd, rows)
		},
	}
	c.Flags().StringVar(&f.db, "db", "", "mirror DB path (default: ~/.config/sapctl/mirror/<service>_<entity>.db)")
	c.Flags().StringVar(&f.service, "service", "", "service (required)")
	c.Flags().StringVar(&f.entity, "entity", "", "entity (required)")
	c.Flags().IntVar(&f.limit, "limit", 25, "max rows (0 = all)")
	_ = c.MarkFlagRequired("service")
	_ = c.MarkFlagRequired("entity")
	return c
}

func newMirrorSearchCmd() *cobra.Command {
	var f mirrorFlags
	c := &cobra.Command{
		Use:   "search",
		Short: "Full-text search over mirrored rows (FTS5)",
		Long:  "Run an FTS5 MATCH query over the `raw` column.",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, cancel := context.WithTimeout(cmd.Context(), 30*time.Second)
			defer cancel()
			dbPath, err := resolveMirrorDB(f)
			if err != nil {
				return errs.Wrap(errs.ExitUserError, "mirror.search.path", "resolve db path", err)
			}
			s, err := sqlitemirror.Open(dbPath)
			if err != nil {
				return errs.Wrap(errs.ExitUserError, "mirror.search.open", "open mirror", err)
			}
			defer s.Close()
			rows, err := s.Search(ctx, f.service, f.entity, f.search, f.limit)
			if err != nil {
				return errs.Wrap(errs.ExitUserError, "mirror.search.query", "search rows", err)
			}
			return emitMirrorRows(cmd, rows)
		},
	}
	c.Flags().StringVar(&f.db, "db", "", "mirror DB path (default: ~/.config/sapctl/mirror/<service>_<entity>.db)")
	c.Flags().StringVar(&f.service, "service", "", "service (required)")
	c.Flags().StringVar(&f.entity, "entity", "", "entity (required)")
	c.Flags().StringVar(&f.search, "query", "", "FTS5 MATCH expression (required)")
	c.Flags().IntVar(&f.limit, "limit", 25, "max rows (0 = all)")
	_ = c.MarkFlagRequired("service")
	_ = c.MarkFlagRequired("entity")
	_ = c.MarkFlagRequired("query")
	return c
}

func newMirrorStatsCmd() *cobra.Command {
	var f mirrorFlags
	c := &cobra.Command{
		Use:   "stats",
		Short: "Show row counts for a service+entity",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, cancel := context.WithTimeout(cmd.Context(), 30*time.Second)
			defer cancel()
			dbPath, err := resolveMirrorDB(f)
			if err != nil {
				return errs.Wrap(errs.ExitUserError, "mirror.stats.path", "resolve db path", err)
			}
			s, err := sqlitemirror.Open(dbPath)
			if err != nil {
				return errs.Wrap(errs.ExitUserError, "mirror.stats.open", "open mirror", err)
			}
			defer s.Close()
			n, err := s.Count(ctx, f.service, f.entity)
			if err != nil {
				return errs.Wrap(errs.ExitUserError, "mirror.stats.query", "count rows", err)
			}
			watermark, _ := s.GetWatermark(ctx, f.service, f.entity)
			if globalFlags.JSON {
				return writeJSON(cmd, map[string]any{
					"service": f.service, "entity": f.entity, "count": n, "watermark": watermark,
				})
			}
			fmt.Fprintf(cmd.OutOrStdout(), "service: %s\nentity:  %s\ncount:   %d\nwatermark: %s\n",
				f.service, f.entity, n, watermark)
			return nil
		},
	}
	c.Flags().StringVar(&f.db, "db", "", "mirror DB path (default: ~/.config/sapctl/mirror/<service>_<entity>.db)")
	c.Flags().StringVar(&f.service, "service", "", "service (required)")
	c.Flags().StringVar(&f.entity, "entity", "", "entity (required)")
	_ = c.MarkFlagRequired("service")
	_ = c.MarkFlagRequired("entity")
	return c
}

func newMirrorQueryCmd() *cobra.Command {
	var db string
	var query string
	var limit int
	c := &cobra.Command{
		Use:   "query",
		Short: "Cross-entity FTS5 search across all mirrored data",
		Long: `Search across ALL mirrored databases. Unlike 'mirror search',
this does not require --service/--entity — it searches every .db file under
~/.config/sapctl/mirror/ and returns results ranked by BM25 relevance.

Useful for agents: "find all rows mentioning 'overdue' across all entities".`,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, cancel := context.WithTimeout(cmd.Context(), 60*time.Second)
			defer cancel()

			// If --db is given, search that single file.
			if db != "" {
				return queryFromFile(ctx, cmd, db, query, limit)
			}

			// Otherwise scan all .db files in the mirror directory.
			dir, err := sqlitemirror.DefaultDir()
			if err != nil {
				return errs.Wrap(errs.ExitUserError, "mirror.query.dir", "resolve default dir", err)
			}
			entries, err := filepath.Glob(filepath.Join(dir, "*.db"))
			if err != nil {
				return errs.Wrap(errs.ExitUserError, "mirror.query.glob", "glob mirror dir", err)
			}
			if len(entries) == 0 {
				if globalFlags.JSON {
					return writeJSON(cmd, map[string]any{"count": 0, "rows": []any{}})
				}
				fmt.Fprintln(cmd.OutOrStdout(), "no mirrored databases found in "+dir)
				return nil
			}

			var all []sqlitemirror.Row
			for _, f := range entries {
				s, err := sqlitemirror.Open(f)
				if err != nil {
					continue
				}
				rows, err := s.Search(ctx, "", "", query, limit)
				_ = s.Close()
				if err != nil {
					continue
				}
				all = append(all, rows...)
				if limit > 0 && len(all) >= limit {
					all = all[:limit]
					break
				}
			}
			return emitMirrorRows(cmd, all)
		},
	}
	c.Flags().StringVar(&db, "db", "", "single mirror DB path (default: scan all ~/.config/sapctl/mirror/*.db)")
	c.Flags().StringVar(&query, "query", "", "FTS5 MATCH expression (required)")
	c.Flags().IntVar(&limit, "limit", 25, "max rows (0 = all)")
	_ = c.MarkFlagRequired("query")
	return c
}

// queryFromFile searches a single mirror DB file.
func queryFromFile(ctx context.Context, cmd *cobra.Command, path, query string, limit int) error {
	s, err := sqlitemirror.Open(path)
	if err != nil {
		return errs.Wrap(errs.ExitUserError, "mirror.query.open", "open mirror", err)
	}
	defer s.Close()
	rows, err := s.Search(ctx, "", "", query, limit)
	if err != nil {
		return errs.Wrap(errs.ExitUserError, "mirror.query.search", "query rows", err)
	}
	return emitMirrorRows(cmd, rows)
}

func emitMirrorRows(cmd *cobra.Command, rows []sqlitemirror.Row) error {
	enc := json.NewEncoder(cmd.OutOrStdout())
	if !globalFlags.Compact {
		enc.SetIndent("", "  ")
	}
	return enc.Encode(map[string]any{
		"count": len(rows),
		"rows":  rows,
	})
}
