package handler

import (
	"context"
	"net/http"

	"github.com/enjoydarts/sifto/api/internal/service"
)

type geminiTTSVoiceCatalogLoader interface {
	LoadCatalog(ctx context.Context) (*service.GeminiTTSVoiceCatalog, error)
}

type GeminiTTSVoicesHandler struct {
	service geminiTTSVoiceCatalogLoader
}

func NewGeminiTTSVoicesHandler(svc geminiTTSVoiceCatalogLoader) *GeminiTTSVoicesHandler {
	return &GeminiTTSVoicesHandler{service: svc}
}

func (h *GeminiTTSVoicesHandler) List(w http.ResponseWriter, r *http.Request) {
	catalog, err := h.service.LoadCatalog(r.Context())
	if err != nil {
		http.Error(w, "failed to load gemini tts voice catalog", http.StatusInternalServerError)
		return
	}
	if catalog == nil {
		catalog = &service.GeminiTTSVoiceCatalog{}
	}
	writeJSON(w, catalog)
}
