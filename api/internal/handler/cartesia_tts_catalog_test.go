package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/enjoydarts/sifto/api/internal/middleware"
	"github.com/enjoydarts/sifto/api/internal/service"
)

type fakeCartesiaTTSCatalogFetcher struct {
	resp         *service.CartesiaTTSCatalogResponse
	previewAudio *service.CartesiaVoicePreviewAudio
	err          error
}

func (f *fakeCartesiaTTSCatalogFetcher) FetchCatalog(_ context.Context, _ string) (*service.CartesiaTTSCatalogResponse, error) {
	if f.err != nil {
		return nil, f.err
	}
	return f.resp, nil
}

func (f *fakeCartesiaTTSCatalogFetcher) FetchVoicePreview(_ context.Context, _ string, _ string) (*service.CartesiaVoicePreviewAudio, error) {
	if f.err != nil {
		return nil, f.err
	}
	return f.previewAudio, nil
}

type fakeCartesiaTTSSettingsRepo struct {
	encryptedKey *string
	err          error
}

func (f *fakeCartesiaTTSSettingsRepo) GetCartesiaAPIKeyEncrypted(_ context.Context, _ string) (*string, error) {
	if f.err != nil {
		return nil, f.err
	}
	return f.encryptedKey, nil
}

func TestCartesiaTTSCatalogHandlerList(t *testing.T) {
	t.Setenv("USER_SECRET_ENCRYPTION_KEY", "cartesia-catalog-test-key")
	cipher := service.NewSecretCipher()
	encryptedKey, err := cipher.EncryptString("cartesia-key")
	if err != nil {
		t.Fatalf("EncryptString() error = %v", err)
	}

	handler := NewCartesiaTTSCatalogHandler(
		&fakeCartesiaTTSSettingsRepo{encryptedKey: &encryptedKey},
		cipher,
		&fakeCartesiaTTSCatalogFetcher{resp: &service.CartesiaTTSCatalogResponse{
			Provider: "cartesia",
			Source:   "cartesia_api_voices_ja",
			Models: []service.CartesiaTTSModelCatalogEntry{
				{ModelID: "sonic-3.5", Name: "Sonic 3.5"},
			},
			Voices: []service.CartesiaVoiceCatalogEntry{
				{VoiceID: "ja-voice-1", Name: "Japanese Voice", Language: "ja"},
			},
		}},
	)

	req := httptest.NewRequest(http.MethodGet, "/api/cartesia-tts-catalog", nil)
	req = req.WithContext(context.WithValue(req.Context(), middleware.UserIDKey, "user-1"))
	rec := httptest.NewRecorder()

	handler.List(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 body=%s", rec.Code, rec.Body.String())
	}
	var resp service.CartesiaTTSCatalogResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp.Provider != "cartesia" || len(resp.Models) != 1 || len(resp.Voices) != 1 {
		t.Fatalf("resp = %#v, want one model and one voice", resp)
	}
}

func TestCartesiaTTSCatalogHandlerListRequiresAPIKey(t *testing.T) {
	handler := NewCartesiaTTSCatalogHandler(
		&fakeCartesiaTTSSettingsRepo{},
		service.NewSecretCipher(),
		&fakeCartesiaTTSCatalogFetcher{},
	)

	req := httptest.NewRequest(http.MethodGet, "/api/cartesia-tts-catalog", nil)
	req = req.WithContext(context.WithValue(req.Context(), middleware.UserIDKey, "user-1"))
	rec := httptest.NewRecorder()

	handler.List(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400 body=%s", rec.Code, rec.Body.String())
	}
}
