package service

import (
	"bytes"
	"context"
	"crypto/rsa"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

type googleCredentialsConfig struct {
	Type                           string `json:"type"`
	Audience                       string `json:"audience"`
	SubjectTokenType               string `json:"subject_token_type"`
	TokenURL                       string `json:"token_url"`
	ServiceAccountImpersonationURL string `json:"service_account_impersonation_url"`
	ClientEmail                    string `json:"client_email"`
	PrivateKey                     string `json:"private_key"`
	TokenURI                       string `json:"token_uri"`
	CredentialSource               struct {
		File    string            `json:"file"`
		URL     string            `json:"url"`
		Headers map[string]string `json:"headers"`
		Format  struct {
			Type                  string `json:"type"`
			SubjectTokenFieldName string `json:"subject_token_field_name"`
		} `json:"format"`
	} `json:"credential_source"`
}

type GoogleWIFTokenSource struct {
	http       *http.Client
	configPath string
	scope      string
}

func NewGoogleWIFTokenSource() *GoogleWIFTokenSource {
	return &GoogleWIFTokenSource{
		http:       &http.Client{Timeout: 20 * time.Second},
		configPath: strings.TrimSpace(os.Getenv("GOOGLE_APPLICATION_CREDENTIALS")),
		scope:      "https://www.googleapis.com/auth/cloud-platform",
	}
}

func (s *GoogleWIFTokenSource) Enabled() bool {
	return s != nil && s.configPath != ""
}

func (s *GoogleWIFTokenSource) AccessToken(ctx context.Context) (string, error) {
	if !s.Enabled() {
		return "", fmt.Errorf("google wif is not configured")
	}
	cfg, err := s.readConfig()
	if err != nil {
		return "", err
	}
	if strings.EqualFold(strings.TrimSpace(cfg.Type), "service_account") {
		return s.serviceAccountAccessToken(ctx, cfg)
	}
	subjectToken, err := s.subjectToken(ctx, cfg)
	if err != nil {
		return "", err
	}
	federatedToken, err := s.exchangeToken(ctx, cfg, subjectToken)
	if err != nil {
		return "", err
	}
	if strings.TrimSpace(cfg.ServiceAccountImpersonationURL) == "" {
		return federatedToken, nil
	}
	return s.impersonatedAccessToken(ctx, cfg.ServiceAccountImpersonationURL, federatedToken)
}

func (s *GoogleWIFTokenSource) readConfig() (*googleCredentialsConfig, error) {
	raw, err := s.readConfigBytes()
	if err != nil {
		return nil, err
	}
	var cfg googleCredentialsConfig
	if err := json.Unmarshal(raw, &cfg); err != nil {
		return nil, err
	}
	switch strings.TrimSpace(cfg.Type) {
	case "service_account":
		if strings.TrimSpace(cfg.ClientEmail) == "" || strings.TrimSpace(cfg.PrivateKey) == "" || strings.TrimSpace(cfg.TokenURI) == "" {
			return nil, fmt.Errorf("invalid google service account config")
		}
	default:
		if strings.TrimSpace(cfg.Audience) == "" || strings.TrimSpace(cfg.SubjectTokenType) == "" || strings.TrimSpace(cfg.TokenURL) == "" {
			return nil, fmt.Errorf("invalid google external account config")
		}
	}
	if strings.TrimSpace(cfg.Type) == "" {
		return nil, fmt.Errorf("invalid google external account config")
	}
	return &cfg, nil
}

func (s *GoogleWIFTokenSource) readConfigBytes() ([]byte, error) {
	config := strings.TrimSpace(s.configPath)
	if config == "" {
		return nil, fmt.Errorf("google wif is not configured")
	}
	if strings.HasPrefix(config, "{") {
		return []byte(config), nil
	}
	return os.ReadFile(config)
}

func (s *GoogleWIFTokenSource) subjectToken(ctx context.Context, cfg *googleCredentialsConfig) (string, error) {
	if path := strings.TrimSpace(cfg.CredentialSource.File); path != "" {
		raw, err := os.ReadFile(path)
		if err != nil {
			return "", err
		}
		return parseSubjectToken(raw, cfg.CredentialSource.Format)
	}
	if sourceURL := strings.TrimSpace(cfg.CredentialSource.URL); sourceURL != "" {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, sourceURL, nil)
		if err != nil {
			return "", err
		}
		for k, v := range cfg.CredentialSource.Headers {
			req.Header.Set(k, v)
		}
		resp, err := s.http.Do(req)
		if err != nil {
			return "", err
		}
		defer resp.Body.Close()
		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
			return "", fmt.Errorf("credential source status %s: %s", resp.Status, strings.TrimSpace(string(body)))
		}
		raw, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
		if err != nil {
			return "", err
		}
		return parseSubjectToken(raw, cfg.CredentialSource.Format)
	}
	return "", fmt.Errorf("unsupported google external account credential source")
}

func parseSubjectToken(raw []byte, format struct {
	Type                  string `json:"type"`
	SubjectTokenFieldName string `json:"subject_token_field_name"`
}) (string, error) {
	if strings.EqualFold(strings.TrimSpace(format.Type), "json") {
		var payload map[string]any
		if err := json.Unmarshal(raw, &payload); err != nil {
			return "", err
		}
		field := strings.TrimSpace(format.SubjectTokenFieldName)
		if field == "" {
			field = "subject_token"
		}
		value, _ := payload[field].(string)
		value = strings.TrimSpace(value)
		if value == "" {
			return "", fmt.Errorf("subject token field missing")
		}
		return value, nil
	}
	value := strings.TrimSpace(string(raw))
	if value == "" {
		return "", fmt.Errorf("subject token missing")
	}
	return value, nil
}

func (s *GoogleWIFTokenSource) exchangeToken(ctx context.Context, cfg *googleCredentialsConfig, subjectToken string) (string, error) {
	form := url.Values{}
	form.Set("grant_type", "urn:ietf:params:oauth:grant-type:token-exchange")
	form.Set("audience", cfg.Audience)
	form.Set("subject_token_type", cfg.SubjectTokenType)
	form.Set("requested_token_type", "urn:ietf:params:oauth:token-type:access_token")
	form.Set("subject_token", subjectToken)
	form.Set("scope", s.scope)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, cfg.TokenURL, strings.NewReader(form.Encode()))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := s.http.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return "", fmt.Errorf("sts token exchange failed: %s: %s", resp.Status, strings.TrimSpace(string(body)))
	}
	var payload struct {
		AccessToken string `json:"access_token"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return "", err
	}
	if strings.TrimSpace(payload.AccessToken) == "" {
		return "", fmt.Errorf("sts access token missing")
	}
	return payload.AccessToken, nil
}

func (s *GoogleWIFTokenSource) impersonatedAccessToken(ctx context.Context, impersonationURL string, federatedToken string) (string, error) {
	body := bytes.NewReader([]byte(`{"scope":["https://www.googleapis.com/auth/cloud-platform"]}`))
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, impersonationURL, body)
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+federatedToken)
	resp, err := s.http.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		raw, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return "", fmt.Errorf("service account impersonation failed: %s: %s", resp.Status, strings.TrimSpace(string(raw)))
	}
	var payload struct {
		AccessToken string `json:"accessToken"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return "", err
	}
	if strings.TrimSpace(payload.AccessToken) == "" {
		return "", fmt.Errorf("impersonated access token missing")
	}
	return payload.AccessToken, nil
}

func (s *GoogleWIFTokenSource) serviceAccountAccessToken(ctx context.Context, cfg *googleCredentialsConfig) (string, error) {
	privateKey, err := parseServiceAccountPrivateKey(cfg.PrivateKey)
	if err != nil {
		return "", err
	}
	now := time.Now().UTC()
	claims := jwt.MapClaims{
		"iss":   cfg.ClientEmail,
		"sub":   cfg.ClientEmail,
		"aud":   cfg.TokenURI,
		"scope": s.scope,
		"iat":   now.Unix(),
		"exp":   now.Add(time.Hour).Unix(),
	}
	assertion, err := jwt.NewWithClaims(jwt.SigningMethodRS256, claims).SignedString(privateKey)
	if err != nil {
		return "", err
	}
	form := url.Values{}
	form.Set("grant_type", "urn:ietf:params:oauth:grant-type:jwt-bearer")
	form.Set("assertion", assertion)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, cfg.TokenURI, strings.NewReader(form.Encode()))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := s.http.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return "", fmt.Errorf("service account token exchange failed: %s: %s", resp.Status, strings.TrimSpace(string(body)))
	}
	var payload struct {
		AccessToken string `json:"access_token"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return "", err
	}
	if strings.TrimSpace(payload.AccessToken) == "" {
		return "", fmt.Errorf("service account access token missing")
	}
	return payload.AccessToken, nil
}

func parseServiceAccountPrivateKey(raw string) (*rsa.PrivateKey, error) {
	block, _ := pem.Decode([]byte(strings.TrimSpace(raw)))
	if block == nil {
		return nil, fmt.Errorf("service account private key PEM is invalid")
	}
	if key, err := x509.ParsePKCS1PrivateKey(block.Bytes); err == nil {
		return key, nil
	}
	parsed, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		return nil, err
	}
	key, ok := parsed.(*rsa.PrivateKey)
	if !ok {
		return nil, fmt.Errorf("service account private key must be RSA")
	}
	return key, nil
}
