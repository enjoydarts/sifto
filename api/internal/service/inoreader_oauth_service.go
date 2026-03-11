package service

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/enjoydarts/sifto/api/internal/model"
	"github.com/enjoydarts/sifto/api/internal/repository"
)

type InoreaderOAuthService struct {
	repo   *repository.UserSettingsRepo
	cipher *SecretCipher
}

type InoreaderConnectResult struct {
	URL    string
	State  string
	Secure bool
}

func NewInoreaderOAuthService(repo *repository.UserSettingsRepo, cipher *SecretCipher) *InoreaderOAuthService {
	return &InoreaderOAuthService{repo: repo, cipher: cipher}
}

func (s *InoreaderOAuthService) RedirectURIFromRequest(r *http.Request) string {
	if v := strings.TrimSpace(os.Getenv("INOREADER_OAUTH_REDIRECT_URI")); v != "" {
		return v
	}
	scheme := "https"
	if xf := strings.TrimSpace(r.Header.Get("X-Forwarded-Proto")); xf != "" {
		scheme = xf
	} else if r.TLS == nil {
		scheme = "http"
	}
	host := strings.TrimSpace(r.Header.Get("X-Forwarded-Host"))
	if host == "" {
		host = r.Host
	}
	return fmt.Sprintf("%s://%s/api/settings/inoreader/callback", scheme, host)
}

func (s *InoreaderOAuthService) Enabled() bool {
	return strings.TrimSpace(os.Getenv("INOREADER_CLIENT_ID")) != "" && strings.TrimSpace(os.Getenv("INOREADER_CLIENT_SECRET")) != ""
}

func (s *InoreaderOAuthService) BuildConnect(r *http.Request) (*InoreaderConnectResult, error) {
	if !s.Enabled() {
		return nil, fmt.Errorf("inoreader oauth is not configured")
	}
	state, err := randomOAuthState()
	if err != nil {
		return nil, err
	}
	q := url.Values{}
	q.Set("client_id", strings.TrimSpace(os.Getenv("INOREADER_CLIENT_ID")))
	q.Set("redirect_uri", s.RedirectURIFromRequest(r))
	q.Set("response_type", "code")
	q.Set("scope", "read")
	q.Set("state", state)
	return &InoreaderConnectResult{
		URL:    "https://www.inoreader.com/oauth2/auth?" + q.Encode(),
		State:  state,
		Secure: r.TLS != nil || strings.EqualFold(strings.TrimSpace(r.Header.Get("X-Forwarded-Proto")), "https"),
	}, nil
}

func (s *InoreaderOAuthService) Complete(ctx context.Context, userID, code, redirectURI string) error {
	if strings.TrimSpace(code) == "" {
		return fmt.Errorf("missing_code")
	}
	if s.cipher == nil || !s.cipher.Enabled() {
		return fmt.Errorf("cipher")
	}
	form := url.Values{}
	form.Set("grant_type", "authorization_code")
	form.Set("code", code)
	form.Set("redirect_uri", redirectURI)
	form.Set("client_id", strings.TrimSpace(os.Getenv("INOREADER_CLIENT_ID")))
	form.Set("client_secret", strings.TrimSpace(os.Getenv("INOREADER_CLIENT_SECRET")))

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, "https://www.inoreader.com/oauth2/token", strings.NewReader(form.Encode()))
	if err != nil {
		return fmt.Errorf("token_request")
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := (&http.Client{Timeout: 20 * time.Second}).Do(req)
	if err != nil {
		return fmt.Errorf("token_exchange")
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("token_status")
	}
	var tokenResp struct {
		AccessToken  string `json:"access_token"`
		RefreshToken string `json:"refresh_token"`
		ExpiresIn    int    `json:"expires_in"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&tokenResp); err != nil || strings.TrimSpace(tokenResp.AccessToken) == "" {
		return fmt.Errorf("token_parse")
	}
	accessEnc, err := s.cipher.EncryptString(tokenResp.AccessToken)
	if err != nil {
		return fmt.Errorf("encrypt_access")
	}
	var refreshEnc *string
	if strings.TrimSpace(tokenResp.RefreshToken) != "" {
		v, err := s.cipher.EncryptString(tokenResp.RefreshToken)
		if err != nil {
			return fmt.Errorf("encrypt_refresh")
		}
		refreshEnc = &v
	}
	var expiresAt *time.Time
	if tokenResp.ExpiresIn > 0 {
		v := time.Now().Add(time.Duration(tokenResp.ExpiresIn) * time.Second)
		expiresAt = &v
	}
	if _, err := s.repo.SetInoreaderOAuthTokens(ctx, userID, accessEnc, refreshEnc, expiresAt); err != nil {
		return fmt.Errorf("save")
	}
	return nil
}

func (s *InoreaderOAuthService) Clear(ctx context.Context, userID string) (*model.UserSettings, error) {
	return s.repo.ClearInoreaderOAuthTokens(ctx, userID)
}

func randomOAuthState() (string, error) {
	b := make([]byte, 24)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}
