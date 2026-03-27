package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/enjoydarts/sifto/api/internal/middleware"
	"github.com/enjoydarts/sifto/api/internal/model"
)

type stubAINavigatorBriefService struct {
	listFn           func(ctx context.Context, userID, slot string, limit int) ([]model.AINavigatorBrief, error)
	getFn            func(ctx context.Context, userID, briefID string) (*model.AINavigatorBrief, error)
	generateManualFn func(ctx context.Context, userID string) (*model.AINavigatorBrief, error)
}

func (s *stubAINavigatorBriefService) ListBriefsByUser(ctx context.Context, userID, slot string, limit int) ([]model.AINavigatorBrief, error) {
	return s.listFn(ctx, userID, slot, limit)
}

func (s *stubAINavigatorBriefService) GetBriefDetail(ctx context.Context, userID, briefID string) (*model.AINavigatorBrief, error) {
	return s.getFn(ctx, userID, briefID)
}

func (s *stubAINavigatorBriefService) GenerateManual(ctx context.Context, userID string) (*model.AINavigatorBrief, error) {
	return s.generateManualFn(ctx, userID)
}

func TestAINavigatorBriefHandlerGenerate(t *testing.T) {
	t.Parallel()

	called := ""
	h := NewAINavigatorBriefHandler(&stubAINavigatorBriefService{
		listFn: func(ctx context.Context, userID, slot string, limit int) ([]model.AINavigatorBrief, error) {
			return nil, nil
		},
		getFn: func(ctx context.Context, userID, briefID string) (*model.AINavigatorBrief, error) {
			return nil, nil
		},
		generateManualFn: func(ctx context.Context, userID string) (*model.AINavigatorBrief, error) {
			called = userID
			return &model.AINavigatorBrief{
				ID:     "brief-1",
				UserID: userID,
				Slot:   model.AINavigatorBriefSlotMorning,
				Status: model.AINavigatorBriefStatusGenerated,
				Title:  "Generated now",
			}, nil
		},
	})

	req := httptest.NewRequest(http.MethodPost, "/api/ai-navigator-briefs/generate", nil)
	req = req.WithContext(context.WithValue(req.Context(), middleware.UserIDKey, "u1"))
	rec := httptest.NewRecorder()

	h.Generate(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", rec.Code, rec.Body.String())
	}
	if called != "u1" {
		t.Fatalf("expected userID u1, got %q", called)
	}
	var payload model.AINavigatorBriefDetailResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if payload.Brief == nil || payload.Brief.ID != "brief-1" {
		t.Fatalf("unexpected payload: %+v", payload.Brief)
	}
}
