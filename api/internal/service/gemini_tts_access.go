package service

import (
	"context"
	"errors"
	"os"
	"strings"

	"github.com/enjoydarts/sifto/api/internal/model"
)

var ErrGeminiTTSNotAllowed = errors.New("gemini tts is not enabled for this user")

type geminiTTSUserLookup interface {
	GetByID(ctx context.Context, id string) (*model.User, error)
}

func normalizeGeminiTTSEmail(email string) string {
	return strings.ToLower(strings.TrimSpace(email))
}

func GeminiTTSEnabledForEmail(email string) bool {
	email = normalizeGeminiTTSEmail(email)
	if email == "" {
		return false
	}
	allowed := strings.TrimSpace(os.Getenv("GEMINI_TTS_ALLOWED_EMAILS"))
	if allowed == "" {
		return false
	}
	for _, raw := range strings.Split(allowed, ",") {
		if normalizeGeminiTTSEmail(raw) == email {
			return true
		}
	}
	return false
}

func GeminiTTSEnabledForUser(ctx context.Context, users geminiTTSUserLookup, userID string) bool {
	if users == nil || strings.TrimSpace(userID) == "" {
		return false
	}
	user, err := users.GetByID(ctx, strings.TrimSpace(userID))
	if err != nil || user == nil {
		return false
	}
	return GeminiTTSEnabledForEmail(user.Email)
}

func EnsureGeminiTTSEnabledForUser(ctx context.Context, users geminiTTSUserLookup, userID string) error {
	if GeminiTTSEnabledForUser(ctx, users, userID) {
		return nil
	}
	return ErrGeminiTTSNotAllowed
}
