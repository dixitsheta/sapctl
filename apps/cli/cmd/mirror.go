package cmd

import (
	"context"
	"encoding/json"
	"fmt"
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
	return c
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
			s, err := sqlitemirror.Open(f.db)
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
	c.Flags().StringVar(&f.db, "db", "", "mirror DB path (required)")
	c.Flags().StringVar(&f.service, "service", "", "service (required)")
	c.Flags().StringVar(&f.entity, "entity", "", "entity (required)")
	c.Flags().StringVar(&since, "since", "", "new watermark value (empty = clear)")
	_ = c.MarkFlagRequired("db")
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
			s, err := sqlitemirror.Open(f.db)
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
	c.Flags().StringVar(&f.db, "db", "", "mirror DB path (required)")
	c.Flags().StringVar(&f.service, "service", "", "service (required)")
	c.Flags().StringVar(&f.entity, "entity", "", "entity (required)")
	c.Flags().BoolVar(&yes, "yes", false, "confirm destructive operation")
	_ = c.MarkFlagRequired("db")
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
			s, err := sqlitemirror.Open(f.db)
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
	c.Flags().StringVar(&f.db, "db", "", "mirror DB path (required)")
	c.Flags().StringVar(&f.service, "service", "", "service (required)")
	c.Flags().StringVar(&f.entity, "entity", "", "entity (required)")
	c.Flags().IntVar(&f.limit, "limit", 25, "max rows (0 = all)")
	_ = c.MarkFlagRequired("db")
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
			s, err := sqlitemirror.Open(f.db)
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
	c.Flags().StringVar(&f.db, "db", "", "mirror DB path (required)")
	c.Flags().StringVar(&f.service, "service", "", "service (required)")
	c.Flags().StringVar(&f.entity, "entity", "", "entity (required)")
	c.Flags().StringVar(&f.search, "query", "", "FTS5 MATCH expression (required)")
	c.Flags().IntVar(&f.limit, "limit", 25, "max rows (0 = all)")
	_ = c.MarkFlagRequired("db")
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
			s, err := sqlitemirror.Open(f.db)
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
	c.Flags().StringVar(&f.db, "db", "", "mirror DB path (required)")
	c.Flags().StringVar(&f.service, "service", "", "service (required)")
	c.Flags().StringVar(&f.entity, "entity", "", "entity (required)")
	_ = c.MarkFlagRequired("db")
	_ = c.MarkFlagRequired("service")
	_ = c.MarkFlagRequired("entity")
	return c
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
