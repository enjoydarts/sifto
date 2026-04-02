package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/enjoydarts/sifto/api/internal/middleware"
	"github.com/enjoydarts/sifto/api/internal/model"
	"github.com/enjoydarts/sifto/api/internal/repository"
)

type itemNotesStore interface {
	GetByItem(ctx context.Context, userID, itemID string) (*model.ItemNote, []model.ItemHighlight, error)
	UpsertNote(ctx context.Context, note model.ItemNote) (model.ItemNote, error)
	CreateHighlight(ctx context.Context, highlight model.ItemHighlight) (model.ItemHighlight, error)
	DeleteHighlight(ctx context.Context, userID, itemID, highlightID string) error
}

type ItemNotesHandler struct {
	store     itemNotesStore
	queueRepo *repository.ReviewQueueRepo
	publisher itemSearchPublisher
}

type itemSearchPublisher interface {
	SendItemSearchUpsertE(ctx context.Context, itemID string) error
}

func NewItemNotesHandler(store itemNotesStore, queueRepo *repository.ReviewQueueRepo, publisher itemSearchPublisher) *ItemNotesHandler {
	return &ItemNotesHandler{store: store, queueRepo: queueRepo, publisher: publisher}
}

func (h *ItemNotesHandler) UpsertNote(w http.ResponseWriter, r *http.Request, itemID string) {
	userID := middleware.GetUserID(r)
	var body struct {
		Content string   `json:"content"`
		Tags    []string `json:"tags"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "invalid request", http.StatusBadRequest)
		return
	}
	note, err := h.store.UpsertNote(r.Context(), model.ItemNote{
		UserID:  userID,
		ItemID:  itemID,
		Content: strings.TrimSpace(body.Content),
		Tags:    body.Tags,
	})
	if err != nil {
		writeRepoError(w, err)
		return
	}
	if h.queueRepo != nil && strings.TrimSpace(note.Content) != "" {
		_ = h.queueRepo.EnqueueDefault(r.Context(), userID, itemID, "note", time.Now())
	}
	if h.publisher != nil {
		_ = h.publisher.SendItemSearchUpsertE(r.Context(), itemID)
	}
	writeJSON(w, note)
}

func (h *ItemNotesHandler) ListHighlights(w http.ResponseWriter, r *http.Request, itemID string) {
	userID := middleware.GetUserID(r)
	_, highlights, err := h.store.GetByItem(r.Context(), userID, itemID)
	if err != nil {
		writeRepoError(w, err)
		return
	}
	writeJSON(w, map[string]any{"highlights": highlights})
}

func (h *ItemNotesHandler) CreateHighlight(w http.ResponseWriter, r *http.Request, itemID string) {
	userID := middleware.GetUserID(r)
	var body struct {
		QuoteText  string `json:"quote_text"`
		AnchorText string `json:"anchor_text"`
		Section    string `json:"section"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "invalid request", http.StatusBadRequest)
		return
	}
	highlight, err := h.store.CreateHighlight(r.Context(), model.ItemHighlight{
		UserID:     userID,
		ItemID:     itemID,
		QuoteText:  strings.TrimSpace(body.QuoteText),
		AnchorText: strings.TrimSpace(body.AnchorText),
		Section:    strings.TrimSpace(body.Section),
	})
	if err != nil {
		writeRepoError(w, err)
		return
	}
	if h.publisher != nil {
		_ = h.publisher.SendItemSearchUpsertE(r.Context(), itemID)
	}
	writeJSON(w, highlight)
}

func (h *ItemNotesHandler) DeleteHighlight(w http.ResponseWriter, r *http.Request, itemID, highlightID string) {
	userID := middleware.GetUserID(r)
	if err := h.store.DeleteHighlight(r.Context(), userID, itemID, highlightID); err != nil {
		writeRepoError(w, err)
		return
	}
	if h.publisher != nil {
		_ = h.publisher.SendItemSearchUpsertE(r.Context(), itemID)
	}
	w.WriteHeader(http.StatusNoContent)
}
