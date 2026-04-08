package handler

import (
	"context"
	"net/http"
	"strings"

	"github.com/enjoydarts/sifto/api/internal/middleware"
	"github.com/enjoydarts/sifto/api/internal/service"
)

type elevenLabsVoiceCatalogFetcher interface {
	FetchVoices(ctx context.Context, apiKey string) (*service.ElevenLabsVoicesResponse, error)
}

type elevenLabsVoiceSettingsRepo interface {
	GetElevenLabsAPIKeyEncrypted(ctx context.Context, userID string) (*string, error)
}

type ElevenLabsVoicesHandler struct {
	settingsRepo elevenLabsVoiceSettingsRepo
	cipher       *service.SecretCipher
	service      elevenLabsVoiceCatalogFetcher
}

func NewElevenLabsVoicesHandler(settingsRepo elevenLabsVoiceSettingsRepo, cipher *service.SecretCipher, svc elevenLabsVoiceCatalogFetcher) *ElevenLabsVoicesHandler {
	return &ElevenLabsVoicesHandler{
		settingsRepo: settingsRepo,
		cipher:       cipher,
		service:      svc,
	}
}

func (h *ElevenLabsVoicesHandler) List(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r)
	apiKey, err := loadAndDecryptUserSecret(r.Context(), h.settingsRepo.GetElevenLabsAPIKeyEncrypted, h.cipher, userID, "")
	if err != nil {
		writeRepoError(w, err)
		return
	}
	if apiKey == nil || strings.TrimSpace(*apiKey) == "" {
		http.Error(w, "elevenlabs api key is not configured", http.StatusBadRequest)
		return
	}
	catalog, err := h.service.FetchVoices(r.Context(), strings.TrimSpace(*apiKey))
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}
	if catalog == nil {
		catalog = &service.ElevenLabsVoicesResponse{
			Provider: "elevenlabs",
			Source:   "elevenlabs_api",
			Voices:   []service.ElevenLabsVoiceCatalogEntry{},
		}
	}
	writeJSON(w, catalog)
}
