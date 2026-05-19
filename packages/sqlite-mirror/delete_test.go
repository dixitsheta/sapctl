package sqlitemirror

import (
	"context"
	"encoding/json"
	"testing"
)

func TestDelete(t *testing.T) {
	s := openTmp(t)
	ctx := context.Background()
	_ = s.Upsert(ctx, "S", "E", "k1", json.RawMessage(`{"v":1}`))
	_ = s.Upsert(ctx, "S", "E", "k2", json.RawMessage(`{"v":2}`))
	_ = s.Upsert(ctx, "OTHER", "X", "k", json.RawMessage(`{"v":3}`))
	_ = s.SetWatermark(ctx, "S", "E", "2026-05-19T00:00:00Z")

	n, err := s.Delete(ctx, "S", "E")
	if err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if n != 2 {
		t.Fatalf("deleted=%d, want 2", n)
	}

	rows, _ := s.List(ctx, "S", "E", 0)
	if len(rows) != 0 {
		t.Fatalf("S/E should be empty, got %d", len(rows))
	}
	w, _ := s.GetWatermark(ctx, "S", "E")
	if w != "" {
		t.Fatalf("watermark not cleared: %q", w)
	}

	rows, _ = s.List(ctx, "OTHER", "X", 0)
	if len(rows) != 1 {
		t.Fatalf("OTHER/X should remain, got %d", len(rows))
	}

	hits, _ := s.Search(ctx, "S", "E", "v", 10)
	if len(hits) != 0 {
		t.Fatalf("FTS still has %d rows", len(hits))
	}
}
