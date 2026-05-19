package sqlitemirror

import (
	"context"
	"encoding/json"
	"path/filepath"
	"testing"
)

func openTmp(t *testing.T) *Store {
	t.Helper()
	path := filepath.Join(t.TempDir(), "mirror.db")
	s, err := Open(path)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { _ = s.Close() })
	return s
}

func TestUpsertAndList(t *testing.T) {
	s := openTmp(t)
	ctx := context.Background()
	mk := func(k string) json.RawMessage {
		b, _ := json.Marshal(map[string]string{"BP": k, "Name": "Alice " + k})
		return b
	}
	for _, k := range []string{"1000001", "1000002", "1000003"} {
		if err := s.Upsert(ctx, "API_BUSINESS_PARTNER", "A_BusinessPartner", k, mk(k)); err != nil {
			t.Fatalf("Upsert %s: %v", k, err)
		}
	}

	rows, err := s.List(ctx, "API_BUSINESS_PARTNER", "A_BusinessPartner", 0)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(rows) != 3 {
		t.Fatalf("len=%d", len(rows))
	}
	if rows[0].Key != "1000001" {
		t.Fatalf("order wrong: %s", rows[0].Key)
	}
}

func TestUpsertReplace(t *testing.T) {
	s := openTmp(t)
	ctx := context.Background()
	_ = s.Upsert(ctx, "S", "E", "k", json.RawMessage(`{"v":1}`))
	_ = s.Upsert(ctx, "S", "E", "k", json.RawMessage(`{"v":2}`))

	rows, _ := s.List(ctx, "S", "E", 0)
	if len(rows) != 1 {
		t.Fatalf("expected 1 row, got %d", len(rows))
	}
	if string(rows[0].Raw) != `{"v":2}` {
		t.Fatalf("raw=%s", rows[0].Raw)
	}
}

func TestFTSSearch(t *testing.T) {
	s := openTmp(t)
	ctx := context.Background()
	_ = s.Upsert(ctx, "S", "E", "1", json.RawMessage(`{"name":"alpha widget"}`))
	_ = s.Upsert(ctx, "S", "E", "2", json.RawMessage(`{"name":"beta gizmo"}`))

	rows, err := s.Search(ctx, "S", "E", "widget", 10)
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(rows) != 1 || rows[0].Key != "1" {
		t.Fatalf("search results: %+v", rows)
	}
}

func TestWatermarks(t *testing.T) {
	s := openTmp(t)
	ctx := context.Background()
	got, err := s.GetWatermark(ctx, "S", "E")
	if err != nil {
		t.Fatalf("GetWatermark empty: %v", err)
	}
	if got != "" {
		t.Fatalf("empty watermark got=%q", got)
	}
	if err := s.SetWatermark(ctx, "S", "E", "2026-05-19T10:00:00Z"); err != nil {
		t.Fatalf("SetWatermark: %v", err)
	}
	got, err = s.GetWatermark(ctx, "S", "E")
	if err != nil {
		t.Fatalf("GetWatermark: %v", err)
	}
	if got == "" {
		t.Fatalf("watermark empty after set")
	}
}
