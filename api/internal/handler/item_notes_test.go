package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/enjoydarts/sifto/api/internal/middleware"
	"github.com/enjoydarts/sifto/api/internal/model"
)

type fakeItemNoteStore struct {
	note       *model.ItemNote
	highlights []model.ItemHighlight
}

func (f *fakeItemNoteStore) GetByItem(_ context.Context, userID, itemID string) (*model.ItemNote, []model.ItemHighlight, error) {
	return f.note, f.highlights, nil
}

func (f *fakeItemNoteStore) UpsertNote(_ context.Context, note model.ItemNote) (model.ItemNote, error) {
	if note.ID == "" {
		note.ID = "note-1"
	}
	now := time.Date(2026, 3, 15, 10, 0, 0, 0, time.UTC)
	note.CreatedAt = now
	note.UpdatedAt = now
	f.note = &note
	return note, nil
}

func (f *fakeItemNoteStore) CreateHighlight(_ context.Context, highlight model.ItemHighlight) (model.ItemHighlight, error) {
	if highlight.ID == "" {
		highlight.ID = "highlight-1"
	}
	highlight.CreatedAt = time.Date(2026, 3, 15, 10, 5, 0, 0, time.UTC)
	f.highlights = append(f.highlights, highlight)
	return highlight, nil
}

func (f *fakeItemNoteStore) DeleteHighlight(_ context.Context, userID, itemID, highlightID string) error {
	next := f.highlights[:0]
	for _, h := range f.highlights {
		if !(h.UserID == userID && h.ItemID == itemID && h.ID == highlightID) {
			next = append(next, h)
		}
	}
	f.highlights = next
	return nil
}

func TestItemNotesHandlerSaveNote(t *testing.T) {
	store := &fakeItemNoteStore{}
	h := NewItemNotesHandler(store, nil)
	body := bytes.NewBufferString(`{"content":"watch this trend","tags":["ai","agents"]}`)
	req := httptest.NewRequest(http.MethodPut, "/api/items/item-1/note", body)
	req = req.WithContext(context.WithValue(req.Context(), middleware.UserIDKey, "u1"))
	rr := httptest.NewRecorder()

	h.UpsertNote(rr, req, "item-1")

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rr.Code)
	}
	if store.note == nil || store.note.Content != "watch this trend" {
		t.Fatalf("note = %+v", store.note)
	}
}

func TestItemNotesHandlerCreateHighlight(t *testing.T) {
	store := &fakeItemNoteStore{}
	h := NewItemNotesHandler(store, nil)
	body := bytes.NewBufferString(`{"quote_text":"important sentence","anchor_text":"important","section":"summary"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/items/item-1/highlights", body)
	req = req.WithContext(context.WithValue(req.Context(), middleware.UserIDKey, "u1"))
	rr := httptest.NewRecorder()

	h.CreateHighlight(rr, req, "item-1")

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rr.Code)
	}
	if len(store.highlights) != 1 || store.highlights[0].QuoteText != "important sentence" {
		t.Fatalf("highlights = %+v", store.highlights)
	}
}

func TestItemNotesHandlerListHighlights(t *testing.T) {
	store := &fakeItemNoteStore{
		highlights: []model.ItemHighlight{{ID: "h1", UserID: "u1", ItemID: "item-1", QuoteText: "q"}},
	}
	h := NewItemNotesHandler(store, nil)
	req := httptest.NewRequest(http.MethodGet, "/api/items/item-1/highlights", nil)
	req = req.WithContext(context.WithValue(req.Context(), middleware.UserIDKey, "u1"))
	rr := httptest.NewRecorder()

	h.ListHighlights(rr, req, "item-1")

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rr.Code)
	}
	var resp struct {
		Highlights []model.ItemHighlight `json:"highlights"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(resp.Highlights) != 1 {
		t.Fatalf("len(highlights) = %d, want 1", len(resp.Highlights))
	}
}
