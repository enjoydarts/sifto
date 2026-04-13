package service

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"

	"github.com/enjoydarts/sifto/api/internal/repository"
)

type AivisUserDictionary struct {
	UUID        string  `json:"uuid"`
	Name        string  `json:"name"`
	Description *string `json:"description,omitempty"`
	WordCount   int     `json:"word_count"`
	CreatedAt   string  `json:"created_at"`
	UpdatedAt   string  `json:"updated_at"`
}

type aivisUserDictionaryListResponse struct {
	UserDictionaries []AivisUserDictionary `json:"user_dictionaries"`
}

type AivisUserDictionaryService struct {
	repo    *repository.UserSettingsRepo
	cipher  *SecretCipher
	http    *http.Client
	baseURL string
}

func NewAivisUserDictionaryService(repo *repository.UserSettingsRepo, cipher *SecretCipher) *AivisUserDictionaryService {
	baseURL := strings.TrimSpace(os.Getenv("AIVIS_USER_DICTIONARIES_API_URL"))
	if baseURL == "" {
		baseURL = "https://api.aivis-project.com/v1/user-dictionaries"
	}
	return &AivisUserDictionaryService{
		repo:    repo,
		cipher:  cipher,
		http:    &http.Client{},
		baseURL: baseURL,
	}
}

func (s *AivisUserDictionaryService) List(ctx context.Context, userID string) ([]AivisUserDictionary, error) {
	if s == nil || s.repo == nil {
		return nil, fmt.Errorf("aivis user dictionary service unavailable")
	}
	enc, err := s.repo.GetAivisAPIKeyEncrypted(ctx, userID)
	if err != nil {
		return nil, err
	}
	if enc == nil || strings.TrimSpace(*enc) == "" {
		return nil, ErrAivisAPIKeyNotConfigured
	}
	if s.cipher == nil || !s.cipher.Enabled() {
		return nil, ErrSecretEncryptionNotConfigured
	}
	token, err := s.cipher.DecryptString(*enc)
	if err != nil {
		return nil, err
	}
	token = strings.TrimSpace(token)
	if token == "" {
		return nil, fmt.Errorf("aivis api key is not configured")
	}
	return s.listWithToken(ctx, token)
}

func (s *AivisUserDictionaryService) listWithToken(ctx context.Context, token string) ([]AivisUserDictionary, error) {
	if s == nil {
		return nil, fmt.Errorf("aivis user dictionary service unavailable")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, s.baseURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Authorization", "Bearer "+strings.TrimSpace(token))
	resp, err := s.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("aivis user dictionaries request failed: %s", resp.Status)
	}
	var payload aivisUserDictionaryListResponse
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return nil, err
	}
	return payload.UserDictionaries, nil
}
