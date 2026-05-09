package handler

import (
	"context"
	"testing"
)

type fakeDBLatencyQuerier struct {
	calls int
}

func (f *fakeDBLatencyQuerier) QueryRow(ctx context.Context, sql string, args ...any) dbLatencyRow {
	f.calls++
	return fakeDBLatencyRow{}
}

type fakeDBLatencyRow struct{}

func (fakeDBLatencyRow) Scan(dest ...any) error {
	if len(dest) > 0 {
		if v, ok := dest[0].(*int); ok {
			*v = 1
		}
	}
	return nil
}

func TestMeasureDBSelectLatencyRunsMultipleSelects(t *testing.T) {
	db := &fakeDBLatencyQuerier{}

	meta, err := measureDBSelectLatency(context.Background(), db, 5)
	if err != nil {
		t.Fatalf("measureDBSelectLatency returned error: %v", err)
	}
	if db.calls != 5 {
		t.Fatalf("calls = %d, want 5", db.calls)
	}
	if meta["query"] != "SELECT 1" {
		t.Fatalf("query meta = %v, want SELECT 1", meta["query"])
	}
	if meta["samples"] != 5 {
		t.Fatalf("samples meta = %v, want 5", meta["samples"])
	}
	for _, key := range []string{"min_ms", "avg_ms", "max_ms", "p95_ms"} {
		if _, ok := meta[key]; !ok {
			t.Fatalf("meta missing %s: %#v", key, meta)
		}
	}
}
