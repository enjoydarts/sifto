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
	"github.com/enjoydarts/sifto/api/internal/repository"
)

type fakeItemNoteStore struct {
	note       *model.ItemNote
	highlights []model.ItemHighlight
	err        error
}

type fakeItemSearchPublisher struct {
	itemIDs []string
	err     error
}

func (f *fakeItemSearchPublisher) SendItemSearchUpsertE(_ context.Context, itemID string) error {
	if f.err != nil {
		return f.err
	}
	f.itemIDs = append(f.itemIDs, itemID)
	return nil
}

func (f *fakeItemNoteStore) GetByItem(_ context.Context, userID, itemID string) (*model.ItemNote, []model.ItemHighlight, error) {
	if f.err != nil {
		return nil, nil, f.err
	}
	return f.note, f.highlights, nil
}

func (f *fakeItemNoteStore) UpsertNote(_ context.Context, note model.ItemNote) (model.ItemNote, error) {
	if f.err != nil {
		return model.ItemNote{}, f.err
	}
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
	if f.err != nil {
		return model.ItemHighlight{}, f.err
	}
	if highlight.ID == "" {
		highlight.ID = "highlight-1"
	}
	highlight.CreatedAt = time.Date(2026, 3, 15, 10, 5, 0, 0, time.UTC)
	f.highlights = append(f.highlights, highlight)
	return highlight, nil
}

func (f *fakeItemNoteStore) DeleteHighlight(_ context.Context, userID, itemID, highlightID string) error {
	if f.err != nil {
		return f.err
	}
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
	publisher := &fakeItemSearchPublisher{}
	h := NewItemNotesHandler(store, nil, publisher)
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
	if len(publisher.itemIDs) != 1 || publisher.itemIDs[0] != "item-1" {
		t.Fatalf("publisher.itemIDs = %#v", publisher.itemIDs)
	}
}

func TestItemNotesHandlerCreateHighlight(t *testing.T) {
	store := &fakeItemNoteStore{}
	publisher := &fakeItemSearchPublisher{}
	h := NewItemNotesHandler(store, nil, publisher)
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
	if len(publisher.itemIDs) != 1 || publisher.itemIDs[0] != "item-1" {
		t.Fatalf("publisher.itemIDs = %#v", publisher.itemIDs)
	}
}

func TestItemNotesHandlerListHighlights(t *testing.T) {
	store := &fakeItemNoteStore{
		highlights: []model.ItemHighlight{{ID: "h1", UserID: "u1", ItemID: "item-1", QuoteText: "q"}},
	}
	h := NewItemNotesHandler(store, nil, nil)
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

func TestItemNotesHandlerSaveNoteConflict(t *testing.T) {
	store := &fakeItemNoteStore{err: repository.ErrConflict}
	h := NewItemNotesHandler(store, nil, nil)
	body := bytes.NewBufferString(`{"content":"watch this trend"}`)
	req := httptest.NewRequest(http.MethodPut, "/api/items/item-1/note", body)
	req = req.WithContext(context.WithValue(req.Context(), middleware.UserIDKey, "u1"))
	rr := httptest.NewRecorder()

	h.UpsertNote(rr, req, "item-1")

	if rr.Code != http.StatusConflict {
		t.Fatalf("status = %d, want 409", rr.Code)
	}
}

func TestItemNotesHandlerDeleteHighlightEnqueuesSearchUpsert(t *testing.T) {
	store := &fakeItemNoteStore{
		highlights: []model.ItemHighlight{{ID: "h1", UserID: "u1", ItemID: "item-1", QuoteText: "q"}},
	}
	publisher := &fakeItemSearchPublisher{}
	h := NewItemNotesHandler(store, nil, publisher)
	req := httptest.NewRequest(http.MethodDelete, "/api/items/item-1/highlights/h1", nil)
	req = req.WithContext(context.WithValue(req.Context(), middleware.UserIDKey, "u1"))
	rr := httptest.NewRecorder()

	h.DeleteHighlight(rr, req, "item-1", "h1")

	if rr.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want 204", rr.Code)
	}
	if len(publisher.itemIDs) != 1 || publisher.itemIDs[0] != "item-1" {
		t.Fatalf("publisher.itemIDs = %#v", publisher.itemIDs)
	}
}
