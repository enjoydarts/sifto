package handler

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/enjoydarts/sifto/api/internal/middleware"
	"github.com/enjoydarts/sifto/api/internal/model"
	"github.com/enjoydarts/sifto/api/internal/repository"
	"github.com/enjoydarts/sifto/api/internal/service"
	"github.com/go-chi/chi/v5"
)

type fakeAudioBriefingPresetsService struct {
	presets   []model.AudioBriefingPreset
	createErr error
	updateErr error
	deleteErr error
	listErr   error
}

func (f *fakeAudioBriefingPresetsService) ListAudioBriefingPresets(_ context.Context, _ string) ([]model.AudioBriefingPreset, error) {
	if f.listErr != nil {
		return nil, f.listErr
	}
	return append([]model.AudioBriefingPreset(nil), f.presets...), nil
}

func (f *fakeAudioBriefingPresetsService) CreateAudioBriefingPreset(_ context.Context, userID string, in service.SaveAudioBriefingPresetInput) (*model.AudioBriefingPreset, error) {
	if f.createErr != nil {
		return nil, f.createErr
	}
	preset := model.AudioBriefingPreset{
		ID:                 "preset-1",
		UserID:             userID,
		Name:               in.Name,
		DefaultPersonaMode: in.DefaultPersonaMode,
		DefaultPersona:     in.DefaultPersona,
		ConversationMode:   in.ConversationMode,
		Voices:             convertVoiceInputs(in.Voices),
	}
	f.presets = append(f.presets, preset)
	return &preset, nil
}

func (f *fakeAudioBriefingPresetsService) UpdateAudioBriefingPreset(_ context.Context, userID, presetID string, in service.SaveAudioBriefingPresetInput) (*model.AudioBriefingPreset, error) {
	if f.updateErr != nil {
		return nil, f.updateErr
	}
	for i := range f.presets {
		if f.presets[i].ID == presetID && f.presets[i].UserID == userID {
			f.presets[i].Name = in.Name
			f.presets[i].DefaultPersonaMode = in.DefaultPersonaMode
			f.presets[i].DefaultPersona = in.DefaultPersona
			f.presets[i].ConversationMode = in.ConversationMode
			f.presets[i].Voices = convertVoiceInputs(in.Voices)
			return &f.presets[i], nil
		}
	}
	return nil, repository.ErrNotFound
}

func (f *fakeAudioBriefingPresetsService) DeleteAudioBriefingPreset(_ context.Context, _ string, presetID string) error {
	if f.deleteErr != nil {
		return f.deleteErr
	}
	next := f.presets[:0]
	for _, preset := range f.presets {
		if preset.ID != presetID {
			next = append(next, preset)
		}
	}
	f.presets = next
	return nil
}

func TestAudioBriefingPresetsHandlerCRUD(t *testing.T) {
	svc := &fakeAudioBriefingPresetsService{}
	h := NewAudioBriefingPresetsHandler(svc)

	createBody := bytes.NewBufferString(`{"name":"Morning","default_persona_mode":"fixed","default_persona":"editor","conversation_mode":"single","voices":[{"persona":"editor","tts_provider":"xai","voice_model":"voice-1"}]}`)
	req := httptest.NewRequest(http.MethodPost, "/api/audio-briefing-presets", createBody)
	req = req.WithContext(context.WithValue(req.Context(), middleware.UserIDKey, "u1"))
	rr := httptest.NewRecorder()
	h.Create(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("create status = %d, body=%s", rr.Code, rr.Body.String())
	}

	req = httptest.NewRequest(http.MethodGet, "/api/audio-briefing-presets", nil)
	req = req.WithContext(context.WithValue(req.Context(), middleware.UserIDKey, "u1"))
	rr = httptest.NewRecorder()
	h.List(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("list status = %d, body=%s", rr.Code, rr.Body.String())
	}

	updateBody := bytes.NewBufferString(`{"name":"Morning v2","default_persona_mode":"random","default_persona":"host","conversation_mode":"duo","voices":[]}`)
	req = httptest.NewRequest(http.MethodPut, "/api/audio-briefing-presets/preset-1", updateBody)
	req = req.WithContext(context.WithValue(req.Context(), middleware.UserIDKey, "u1"))
	routeCtx := chi.NewRouteContext()
	routeCtx.URLParams.Add("id", "preset-1")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, routeCtx))
	rr = httptest.NewRecorder()
	h.Update(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("update status = %d, body=%s", rr.Code, rr.Body.String())
	}

	req = httptest.NewRequest(http.MethodDelete, "/api/audio-briefing-presets/preset-1", nil)
	req = req.WithContext(context.WithValue(req.Context(), middleware.UserIDKey, "u1"))
	routeCtx = chi.NewRouteContext()
	routeCtx.URLParams.Add("id", "preset-1")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, routeCtx))
	rr = httptest.NewRecorder()
	h.Delete(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("delete status = %d, body=%s", rr.Code, rr.Body.String())
	}
}

func TestAudioBriefingPresetsHandlerCreateMapsConflictTo409(t *testing.T) {
	svc := &fakeAudioBriefingPresetsService{createErr: repository.ErrConflict}
	h := NewAudioBriefingPresetsHandler(svc)
	req := httptest.NewRequest(http.MethodPost, "/api/audio-briefing-presets", bytes.NewBufferString(`{"name":"Morning"}`))
	req = req.WithContext(context.WithValue(req.Context(), middleware.UserIDKey, "u1"))
	rr := httptest.NewRecorder()
	h.Create(rr, req)
	if rr.Code != http.StatusConflict {
		t.Fatalf("status = %d, want 409 body=%s", rr.Code, rr.Body.String())
	}
}

func TestAudioBriefingPresetsHandlerRejectsInvalidJSON(t *testing.T) {
	h := NewAudioBriefingPresetsHandler(&fakeAudioBriefingPresetsService{})
	req := httptest.NewRequest(http.MethodPost, "/api/audio-briefing-presets", bytes.NewBufferString(`{"name":`))
	req = req.WithContext(context.WithValue(req.Context(), middleware.UserIDKey, "u1"))
	rr := httptest.NewRecorder()
	h.Create(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rr.Code)
	}
}

func TestAudioBriefingPresetsHandlerDeleteRejectsMissingID(t *testing.T) {
	h := NewAudioBriefingPresetsHandler(&fakeAudioBriefingPresetsService{})
	req := httptest.NewRequest(http.MethodDelete, "/api/audio-briefing-presets/", nil)
	req = req.WithContext(context.WithValue(req.Context(), middleware.UserIDKey, "u1"))
	rr := httptest.NewRecorder()
	h.Delete(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rr.Code)
	}
}

func TestAudioBriefingPresetsHandlerDeleteMapsNotFoundTo404(t *testing.T) {
	svc := &fakeAudioBriefingPresetsService{deleteErr: repository.ErrNotFound}
	h := NewAudioBriefingPresetsHandler(svc)
	req := httptest.NewRequest(http.MethodDelete, "/api/audio-briefing-presets/preset-1", nil)
	req = req.WithContext(context.WithValue(req.Context(), middleware.UserIDKey, "u1"))
	routeCtx := chi.NewRouteContext()
	routeCtx.URLParams.Add("id", "preset-1")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, routeCtx))
	rr := httptest.NewRecorder()
	h.Delete(rr, req)
	if rr.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", rr.Code)
	}
}

func TestAudioBriefingPresetsHandlerDeleteRejectsMissingService(t *testing.T) {
	h := NewAudioBriefingPresetsHandler(nil)
	req := httptest.NewRequest(http.MethodDelete, "/api/audio-briefing-presets/preset-1", nil)
	req = req.WithContext(context.WithValue(req.Context(), middleware.UserIDKey, "u1"))
	routeCtx := chi.NewRouteContext()
	routeCtx.URLParams.Add("id", "preset-1")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, routeCtx))
	rr := httptest.NewRecorder()
	h.Delete(rr, req)
	if rr.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500", rr.Code)
	}
}

func TestAudioBriefingPresetsHandlerListReturnsPresets(t *testing.T) {
	svc := &fakeAudioBriefingPresetsService{presets: []model.AudioBriefingPreset{{ID: "preset-1", UserID: "u1", Name: "Morning"}}}
	h := NewAudioBriefingPresetsHandler(svc)
	req := httptest.NewRequest(http.MethodGet, "/api/audio-briefing-presets", nil)
	req = req.WithContext(context.WithValue(req.Context(), middleware.UserIDKey, "u1"))
	rr := httptest.NewRecorder()
	h.List(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rr.Code)
	}
}

func convertVoiceInputs(rows []service.UpdateAudioBriefingPersonaVoiceInput) []model.AudioBriefingPersonaVoice {
	out := make([]model.AudioBriefingPersonaVoice, 0, len(rows))
	for _, row := range rows {
		out = append(out, model.AudioBriefingPersonaVoice{
			Persona:                  row.Persona,
			TTSProvider:              row.TTSProvider,
			TTSModel:                 row.TTSModel,
			VoiceModel:               row.VoiceModel,
			VoiceStyle:               row.VoiceStyle,
			ProviderVoiceLabel:       row.ProviderVoiceLabel,
			ProviderVoiceDescription: row.ProviderVoiceDescription,
			SpeechRate:               row.SpeechRate,
			EmotionalIntensity:       row.EmotionalIntensity,
			TempoDynamics:            row.TempoDynamics,
			LineBreakSilenceSeconds:  row.LineBreakSilenceSeconds,
			Pitch:                    row.Pitch,
			VolumeGain:               row.VolumeGain,
		})
	}
	return out
}
