package repository

import (
	"errors"
	"testing"
	"time"

	"github.com/enjoydarts/sifto/api/internal/model"
)

type stubItemRowScanner struct {
	rows [][]any
	idx  int
	err  error
}

func (s *stubItemRowScanner) Next() bool {
	if s.idx >= len(s.rows) {
		return false
	}
	s.idx++
	return true
}

func (s *stubItemRowScanner) Scan(dest ...any) error {
	if s.idx == 0 || s.idx > len(s.rows) {
		return errors.New("scan called out of bounds")
	}
	row := s.rows[s.idx-1]
	if len(row) != len(dest) {
		return errors.New("destination count mismatch")
	}
	for i := range dest {
		switch d := dest[i].(type) {
		case *string:
			*d = row[i].(string)
		case **string:
			v := row[i].(string)
			*d = &v
		case *bool:
			*d = row[i].(bool)
		case *int:
			*d = row[i].(int)
		case *float64:
			*d = row[i].(float64)
		case **float64:
			v := row[i].(float64)
			*d = &v
		case *[]string:
			*d = row[i].([]string)
		case *time.Time:
			*d = row[i].(time.Time)
		case **time.Time:
			v := row[i].(time.Time)
			*d = &v
		case **model.ItemSummaryScoreBreakdown:
			v := row[i].(*model.ItemSummaryScoreBreakdown)
			*d = v
		default:
			return errors.New("unsupported scan type")
		}
	}
	return nil
}

func (s *stubItemRowScanner) Err() error { return s.err }

func TestScanItemsIncludesSourceTitle(t *testing.T) {
	now := time.Now().UTC()
	rows := &stubItemRowScanner{
		rows: [][]any{{
			"item-1",
			"source-1",
			"Source One",
			"https://example.com",
			"Title",
			"https://example.com/thumb.jpg",
			"Summary text",
			"summarized",
			"",
			"pass",
			"pass",
			false,
			true,
			1,
			0.9,
			0.82,
			"topic:ai",
			[]string{"ai"},
			"Translated",
			now,
			now,
			now,
			now,
		}},
	}

	items, err := scanItems(rows)
	if err != nil {
		t.Fatalf("scanItems err = %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("len(items) = %d, want 1", len(items))
	}
	if items[0].SourceTitle == nil || *items[0].SourceTitle != "Source One" {
		t.Fatalf("source title = %v, want Source One", items[0].SourceTitle)
	}
}

func TestScanItemsWithBreakdownIncludesSourceTitle(t *testing.T) {
	now := time.Now().UTC()
	rows := &stubItemRowScanner{
		rows: [][]any{{
			"item-1",
			"source-1",
			"Source One",
			"https://example.com",
			"Title",
			"https://example.com/thumb.jpg",
			"Summary text",
			"summarized",
			"",
			"pass",
			"pass",
			false,
			true,
			1,
			0.9,
			&model.ItemSummaryScoreBreakdown{Importance: ptrFloat64(0.5)},
			0.82,
			"topic:ai",
			[]string{"ai"},
			"Translated",
			now,
			now,
			now,
			now,
		}},
	}

	items, err := scanItemsWithBreakdown(rows)
	if err != nil {
		t.Fatalf("scanItemsWithBreakdown err = %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("len(items) = %d, want 1", len(items))
	}
	if items[0].SourceTitle == nil || *items[0].SourceTitle != "Source One" {
		t.Fatalf("source title = %v, want Source One", items[0].SourceTitle)
	}
}

func ptrFloat64(v float64) *float64 {
	return &v
}
