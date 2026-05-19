// Package sqlitemirror provides a local, append-friendly SQLite mirror of
// SAP entities with FTS5 search and CDC watermarks.
//
// Pure-Go driver (modernc.org/sqlite) is used so binaries cross-compile and
// air-gap installs do not require cgo + a C toolchain.
package sqlitemirror

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	_ "modernc.org/sqlite"
)

// Store wraps an opened SQLite database.
type Store struct {
	db *sql.DB
}

// Open opens (or creates) a mirror DB at path. Schema is migrated on open.
func Open(path string) (*Store, error) {
	dsn := fmt.Sprintf("file:%s?_pragma=journal_mode(WAL)&_pragma=synchronous(NORMAL)&_pragma=foreign_keys(ON)", path)
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("open: %w", err)
	}
	if err := db.Ping(); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("ping: %w", err)
	}
	s := &Store{db: db}
	if err := s.migrate(); err != nil {
		_ = s.Close()
		return nil, err
	}
	return s, nil
}

// Close closes the underlying DB.
func (s *Store) Close() error { return s.db.Close() }

func (s *Store) migrate() error {
	stmts := []string{
		`CREATE TABLE IF NOT EXISTS entities (
			service     TEXT NOT NULL,
			entity      TEXT NOT NULL,
			key         TEXT NOT NULL,
			etag        TEXT,
			fetched_at  INTEGER NOT NULL,
			raw         TEXT NOT NULL,
			PRIMARY KEY (service, entity, key)
		)`,
		`CREATE INDEX IF NOT EXISTS entities_entity_idx ON entities(service, entity)`,
		`CREATE VIRTUAL TABLE IF NOT EXISTS entities_fts USING fts5(
			service UNINDEXED, entity UNINDEXED, key UNINDEXED, raw,
			tokenize = 'porter unicode61'
		)`,
		`CREATE TABLE IF NOT EXISTS watermarks (
			service     TEXT NOT NULL,
			entity      TEXT NOT NULL,
			since       TEXT,
			updated_at  INTEGER NOT NULL,
			PRIMARY KEY (service, entity)
		)`,
	}
	for _, q := range stmts {
		if _, err := s.db.Exec(q); err != nil {
			return fmt.Errorf("migrate: %s: %w", q, err)
		}
	}
	return nil
}

// Upsert inserts-or-replaces a row in both `entities` and `entities_fts`.
// key may be empty for entities without natural keys (single-row services).
func (s *Store) Upsert(ctx context.Context, service, entity, key string, raw json.RawMessage) error {
	if service == "" || entity == "" {
		return errors.New("service and entity required")
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	now := time.Now().UnixMilli()
	if _, err := tx.ExecContext(ctx,
		`INSERT INTO entities(service, entity, key, fetched_at, raw)
		 VALUES (?, ?, ?, ?, ?)
		 ON CONFLICT(service, entity, key) DO UPDATE SET
		   fetched_at = excluded.fetched_at,
		   raw        = excluded.raw`,
		service, entity, key, now, string(raw),
	); err != nil {
		return fmt.Errorf("upsert entity: %w", err)
	}
	if _, err := tx.ExecContext(ctx,
		`DELETE FROM entities_fts WHERE service = ? AND entity = ? AND key = ?`,
		service, entity, key,
	); err != nil {
		return fmt.Errorf("fts delete: %w", err)
	}
	if _, err := tx.ExecContext(ctx,
		`INSERT INTO entities_fts(service, entity, key, raw) VALUES (?, ?, ?, ?)`,
		service, entity, key, string(raw),
	); err != nil {
		return fmt.Errorf("fts insert: %w", err)
	}
	return tx.Commit()
}

// Row represents one stored row.
type Row struct {
	Service   string          `json:"service"`
	Entity    string          `json:"entity"`
	Key       string          `json:"key"`
	FetchedAt int64           `json:"fetched_at"`
	Raw       json.RawMessage `json:"raw"`
}

// List returns rows for service+entity ordered by key. limit <= 0 means all.
func (s *Store) List(ctx context.Context, service, entity string, limit int) ([]Row, error) {
	q := `SELECT service, entity, key, fetched_at, raw FROM entities
	      WHERE service = ? AND entity = ? ORDER BY key`
	args := []any{service, entity}
	if limit > 0 {
		q += ` LIMIT ?`
		args = append(args, limit)
	}
	rows, err := s.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanRows(rows)
}

// Search runs FTS5 MATCH over `raw`. service+entity narrow the search.
func (s *Store) Search(ctx context.Context, service, entity, query string, limit int) ([]Row, error) {
	q := `SELECT e.service, e.entity, e.key, e.fetched_at, e.raw
	      FROM entities_fts f
	      JOIN entities e ON e.service = f.service AND e.entity = f.entity AND e.key = f.key
	      WHERE f.service = ? AND f.entity = ? AND entities_fts MATCH ?
	      ORDER BY bm25(entities_fts)`
	args := []any{service, entity, query}
	if limit > 0 {
		q += ` LIMIT ?`
		args = append(args, limit)
	}
	rows, err := s.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanRows(rows)
}

func scanRows(rows *sql.Rows) ([]Row, error) {
	var out []Row
	for rows.Next() {
		var r Row
		var raw string
		if err := rows.Scan(&r.Service, &r.Entity, &r.Key, &r.FetchedAt, &raw); err != nil {
			return nil, err
		}
		r.Raw = json.RawMessage(raw)
		out = append(out, r)
	}
	return out, rows.Err()
}

// SetWatermark stores the `since` cursor for a (service, entity) pair.
func (s *Store) SetWatermark(ctx context.Context, service, entity, since string) error {
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO watermarks(service, entity, since, updated_at)
		 VALUES (?, ?, ?, ?)
		 ON CONFLICT(service, entity) DO UPDATE SET
		   since      = excluded.since,
		   updated_at = excluded.updated_at`,
		service, entity, since, time.Now().UnixMilli(),
	)
	return err
}

// GetWatermark returns the cursor (empty if none).
func (s *Store) GetWatermark(ctx context.Context, service, entity string) (string, error) {
	var since sql.NullString
	err := s.db.QueryRowContext(ctx,
		`SELECT since FROM watermarks WHERE service = ? AND entity = ?`,
		service, entity,
	).Scan(&since)
	if errors.Is(err, sql.ErrNoRows) {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	return since.String, nil
}

// Count returns row count for (service, entity).
func (s *Store) Count(ctx context.Context, service, entity string) (int, error) {
	var n int
	err := s.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM entities WHERE service = ? AND entity = ?`,
		service, entity,
	).Scan(&n)
	return n, err
}

// Delete removes all rows + watermark for a (service, entity). Returns the
// number of entity rows deleted (FTS5 mirror is wiped in the same tx).
func (s *Store) Delete(ctx context.Context, service, entity string) (int, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer func() { _ = tx.Rollback() }()

	res, err := tx.ExecContext(ctx,
		`DELETE FROM entities WHERE service = ? AND entity = ?`, service, entity)
	if err != nil {
		return 0, err
	}
	n, _ := res.RowsAffected()

	if _, err := tx.ExecContext(ctx,
		`DELETE FROM entities_fts WHERE service = ? AND entity = ?`, service, entity); err != nil {
		return 0, err
	}
	if _, err := tx.ExecContext(ctx,
		`DELETE FROM watermarks WHERE service = ? AND entity = ?`, service, entity); err != nil {
		return 0, err
	}
	return int(n), tx.Commit()
}
