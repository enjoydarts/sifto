package handler

import (
	"context"
	"net/http"
	"strings"

	"github.com/enjoydarts/sifto/api/internal/middleware"
	"github.com/enjoydarts/sifto/api/internal/service"
	"github.com/go-chi/chi/v5"
)

type cartesiaTTSCatalogFetcher interface {
	FetchCatalog(ctx context.Context, apiKey string) (*service.CartesiaTTSCatalogResponse, error)
	FetchVoicePreview(ctx context.Context, apiKey string, voiceID string) (*service.CartesiaVoicePreviewAudio, error)
}

type cartesiaTTSSettingsRepo interface {
	GetCartesiaAPIKeyEncrypted(ctx context.Context, userID string) (*string, error)
}

type CartesiaTTSCatalogHandler struct {
	settingsRepo cartesiaTTSSettingsRepo
	cipher       *service.SecretCipher
	service      cartesiaTTSCatalogFetcher
}

func NewCartesiaTTSCatalogHandler(settingsRepo cartesiaTTSSettingsRepo, cipher *service.SecretCipher, svc cartesiaTTSCatalogFetcher) *CartesiaTTSCatalogHandler {
	return &CartesiaTTSCatalogHandler{
		settingsRepo: settingsRepo,
		cipher:       cipher,
		service:      svc,
	}
}

func (h *CartesiaTTSCatalogHandler) List(w http.ResponseWriter, r *http.Request) {
	apiKey, ok := h.loadAPIKey(w, r)
	if !ok {
		return
	}
	catalog, err := h.service.FetchCatalog(r.Context(), apiKey)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}
	if catalog == nil {
		catalog = &service.CartesiaTTSCatalogResponse{
			Provider: "cartesia",
			Source:   "cartesia_api_voices_ja",
			Models:   []service.CartesiaTTSModelCatalogEntry{},
			Voices:   []service.CartesiaVoiceCatalogEntry{},
		}
	}
	writeJSON(w, catalog)
}

func (h *CartesiaTTSCatalogHandler) Preview(w http.ResponseWriter, r *http.Request) {
	apiKey, ok := h.loadAPIKey(w, r)
	if !ok {
		return
	}
	voiceID := strings.TrimSpace(chi.URLParam(r, "voiceID"))
	if voiceID == "" {
		http.Error(w, "cartesia voice id is required", http.StatusBadRequest)
		return
	}
	audio, err := h.service.FetchVoicePreview(r.Context(), apiKey, voiceID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}
	w.Header().Set("Content-Type", audio.ContentType)
	w.Header().Set("Cache-Control", "private, max-age=300")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(audio.Bytes)
}

func (h *CartesiaTTSCatalogHandler) loadAPIKey(w http.ResponseWriter, r *http.Request) (string, bool) {
	userID := middleware.GetUserID(r)
	apiKey, err := loadAndDecryptUserSecret(r.Context(), h.settingsRepo.GetCartesiaAPIKeyEncrypted, h.cipher, userID, "")
	if err != nil {
		writeRepoError(w, err)
		return "", false
	}
	if apiKey == nil || strings.TrimSpace(*apiKey) == "" {
		http.Error(w, "cartesia api key is not configured", http.StatusBadRequest)
		return "", false
	}
	return strings.TrimSpace(*apiKey), true
}
