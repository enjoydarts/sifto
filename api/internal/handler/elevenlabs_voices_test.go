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

type fakeElevenLabsVoiceFetcher struct {
	resp *service.ElevenLabsVoicesResponse
	err  error
}

func (f *fakeElevenLabsVoiceFetcher) FetchVoices(_ context.Context, _ string) (*service.ElevenLabsVoicesResponse, error) {
	if f.err != nil {
		return nil, f.err
	}
	return f.resp, nil
}

type fakeElevenLabsSettingsRepo struct {
	encryptedKey *string
	err          error
}

func (f *fakeElevenLabsSettingsRepo) GetElevenLabsAPIKeyEncrypted(_ context.Context, _ string) (*string, error) {
	if f.err != nil {
		return nil, f.err
	}
	return f.encryptedKey, nil
}

func TestElevenLabsVoicesHandlerList(t *testing.T) {
	t.Setenv("USER_SECRET_ENCRYPTION_KEY", "elevenlabs-voices-test-key")
	cipher := service.NewSecretCipher()
	encryptedKey, err := cipher.EncryptString("eleven-key")
	if err != nil {
		t.Fatalf("EncryptString() error = %v", err)
	}

	handler := NewElevenLabsVoicesHandler(
		&fakeElevenLabsSettingsRepo{encryptedKey: &encryptedKey},
		cipher,
		&fakeElevenLabsVoiceFetcher{resp: &service.ElevenLabsVoicesResponse{
			Provider: "elevenlabs",
			Source:   "elevenlabs_api",
			Voices: []service.ElevenLabsVoiceCatalogEntry{
				{VoiceID: "voice-1", Name: "Beta"},
				{VoiceID: "voice-2", Name: "Alpha"},
			},
		}},
	)

	req := httptest.NewRequest(http.MethodGet, "/api/elevenlabs-voices", nil)
	req = req.WithContext(context.WithValue(req.Context(), middleware.UserIDKey, "user-1"))
	rec := httptest.NewRecorder()

	handler.List(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 body=%s", rec.Code, rec.Body.String())
	}
	var resp service.ElevenLabsVoicesResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp.Provider != "elevenlabs" || len(resp.Voices) != 2 {
		t.Fatalf("resp = %#v, want two voices", resp)
	}
}

func TestElevenLabsVoicesHandlerListRequiresAPIKey(t *testing.T) {
	handler := NewElevenLabsVoicesHandler(
		&fakeElevenLabsSettingsRepo{},
		service.NewSecretCipher(),
		&fakeElevenLabsVoiceFetcher{},
	)

	req := httptest.NewRequest(http.MethodGet, "/api/elevenlabs-voices", nil)
	req = req.WithContext(context.WithValue(req.Context(), middleware.UserIDKey, "user-1"))
	rec := httptest.NewRecorder()

	handler.List(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400 body=%s", rec.Code, rec.Body.String())
	}
}
