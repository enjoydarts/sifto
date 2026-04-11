package handler

import (
	"context"
	"net/http"
	"strings"

	"github.com/enjoydarts/sifto/api/internal/middleware"
	"github.com/enjoydarts/sifto/api/internal/service"
)

type azureSpeechVoiceCatalogFetcher interface {
	FetchVoices(ctx context.Context, apiKey, region string) (*service.AzureSpeechVoicesResponse, error)
}

type azureSpeechVoiceSettingsRepo interface {
	GetAzureSpeechAPIKeyEncrypted(ctx context.Context, userID string) (*string, error)
	GetAzureSpeechRegion(ctx context.Context, userID string) (*string, error)
}

type AzureSpeechVoicesHandler struct {
	settingsRepo azureSpeechVoiceSettingsRepo
	cipher       *service.SecretCipher
	service      azureSpeechVoiceCatalogFetcher
}

func NewAzureSpeechVoicesHandler(settingsRepo azureSpeechVoiceSettingsRepo, cipher *service.SecretCipher, svc azureSpeechVoiceCatalogFetcher) *AzureSpeechVoicesHandler {
	return &AzureSpeechVoicesHandler{
		settingsRepo: settingsRepo,
		cipher:       cipher,
		service:      svc,
	}
}

func (h *AzureSpeechVoicesHandler) List(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r)
	apiKey, err := loadAndDecryptUserSecret(r.Context(), h.settingsRepo.GetAzureSpeechAPIKeyEncrypted, h.cipher, userID, "")
	if err != nil {
		writeRepoError(w, err)
		return
	}
	if apiKey == nil || strings.TrimSpace(*apiKey) == "" {
		http.Error(w, "azure speech api key is not configured", http.StatusBadRequest)
		return
	}
	region, err := h.settingsRepo.GetAzureSpeechRegion(r.Context(), userID)
	if err != nil {
		writeRepoError(w, err)
		return
	}
	if region == nil || strings.TrimSpace(*region) == "" {
		http.Error(w, "azure speech region is not configured", http.StatusBadRequest)
		return
	}
	catalog, err := h.service.FetchVoices(r.Context(), strings.TrimSpace(*apiKey), strings.TrimSpace(*region))
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}
	if catalog == nil {
		catalog = &service.AzureSpeechVoicesResponse{
			Provider: "azure_speech",
			Source:   "azure_speech_voices_ja",
			Region:   strings.TrimSpace(*region),
			Voices:   []service.AzureSpeechVoiceCatalogEntry{},
		}
	}
	writeJSON(w, catalog)
}
