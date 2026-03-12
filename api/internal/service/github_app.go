package service

import (
	"bytes"
	"context"
	"crypto/rsa"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

type GitHubAppClient struct {
	appID      string
	privateKey *rsa.PrivateKey
	installURL string
	apiBaseURL string
	httpClient *http.Client
}

type GitHubInstallation struct {
	ID      int64
	Account *struct {
		Login string `json:"login"`
	} `json:"account"`
}

func NewGitHubAppClientFromEnv() *GitHubAppClient {
	appID := strings.TrimSpace(os.Getenv("GITHUB_APP_ID"))
	privateKeyPEM := strings.TrimSpace(os.Getenv("GITHUB_APP_PRIVATE_KEY"))
	installURL := strings.TrimSpace(os.Getenv("GITHUB_APP_INSTALL_URL"))
	if appID == "" || privateKeyPEM == "" {
		return &GitHubAppClient{installURL: installURL}
	}
	key, err := parseGitHubAppPrivateKey(privateKeyPEM)
	if err != nil {
		return &GitHubAppClient{installURL: installURL}
	}
	apiBaseURL := strings.TrimSpace(os.Getenv("GITHUB_API_BASE_URL"))
	if apiBaseURL == "" {
		apiBaseURL = "https://api.github.com"
	}
	return &GitHubAppClient{
		appID:      appID,
		privateKey: key,
		installURL: installURL,
		apiBaseURL: strings.TrimRight(apiBaseURL, "/"),
		httpClient: &http.Client{Timeout: 20 * time.Second},
	}
}

func (c *GitHubAppClient) Enabled() bool {
	return c != nil && c.appID != "" && c.privateKey != nil
}

func (c *GitHubAppClient) InstallURL() string { return c.installURL }

func parseGitHubAppPrivateKey(raw string) (*rsa.PrivateKey, error) {
	normalized := strings.ReplaceAll(raw, `\n`, "\n")
	block, _ := pem.Decode([]byte(normalized))
	if block == nil {
		return nil, fmt.Errorf("github app private key decode failed")
	}
	if key, err := x509.ParsePKCS1PrivateKey(block.Bytes); err == nil {
		return key, nil
	}
	keyAny, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		return nil, err
	}
	key, ok := keyAny.(*rsa.PrivateKey)
	if !ok {
		return nil, fmt.Errorf("github app private key is not rsa")
	}
	return key, nil
}

func (c *GitHubAppClient) appJWT() (string, error) {
	if !c.Enabled() {
		return "", fmt.Errorf("github app disabled")
	}
	now := time.Now().UTC()
	token := jwt.NewWithClaims(jwt.SigningMethodRS256, jwt.MapClaims{
		"iat": now.Add(-30 * time.Second).Unix(),
		"exp": now.Add(9 * time.Minute).Unix(),
		"iss": c.appID,
	})
	return token.SignedString(c.privateKey)
}

func (c *GitHubAppClient) GetInstallation(ctx context.Context, installationID int64) (*GitHubInstallation, error) {
	j, err := c.appJWT()
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, fmt.Sprintf("%s/app/installations/%d", c.apiBaseURL, installationID), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+j)
	req.Header.Set("Accept", "application/vnd.github+json")
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return nil, fmt.Errorf("github get installation failed: %s %s", resp.Status, strings.TrimSpace(string(body)))
	}
	var out GitHubInstallation
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *GitHubAppClient) CreateInstallationToken(ctx context.Context, installationID int64) (string, error) {
	j, err := c.appJWT()
	if err != nil {
		return "", err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, fmt.Sprintf("%s/app/installations/%d/access_tokens", c.apiBaseURL, installationID), bytes.NewReader([]byte(`{}`)))
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Bearer "+j)
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return "", fmt.Errorf("github create installation token failed: %s %s", resp.Status, strings.TrimSpace(string(body)))
	}
	var payload struct {
		Token string `json:"token"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return "", err
	}
	if strings.TrimSpace(payload.Token) == "" {
		return "", fmt.Errorf("github installation token missing")
	}
	return payload.Token, nil
}

type GitHubContentFile struct {
	SHA string `json:"sha"`
}

func (c *GitHubAppClient) GetFile(ctx context.Context, token, owner, repo, branch, path string) (*GitHubContentFile, error) {
	url := fmt.Sprintf("%s/repos/%s/%s/contents/%s?ref=%s", c.apiBaseURL, owner, repo, path, branch)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/vnd.github+json")
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotFound {
		return nil, nil
	}
	if resp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return nil, fmt.Errorf("github get file failed: %s %s", resp.Status, strings.TrimSpace(string(body)))
	}
	var out GitHubContentFile
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *GitHubAppClient) UpsertFile(ctx context.Context, token, owner, repo, branch, path, message string, content []byte, currentSHA *string) (string, error) {
	body := map[string]any{
		"message": message,
		"content": base64.StdEncoding.EncodeToString(content),
		"branch":  branch,
	}
	if currentSHA != nil && strings.TrimSpace(*currentSHA) != "" {
		body["sha"] = strings.TrimSpace(*currentSHA)
	}
	b, err := json.Marshal(body)
	if err != nil {
		return "", err
	}
	url := fmt.Sprintf("%s/repos/%s/%s/contents/%s", c.apiBaseURL, owner, repo, path)
	req, err := http.NewRequestWithContext(ctx, http.MethodPut, url, bytes.NewReader(b))
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return "", fmt.Errorf("github upsert file failed: %s %s", resp.Status, strings.TrimSpace(string(body)))
	}
	var payload struct {
		Content struct {
			SHA string `json:"sha"`
		} `json:"content"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return "", err
	}
	return payload.Content.SHA, nil
}

func ParseGitHubInstallationID(raw string) (int64, error) {
	return strconv.ParseInt(strings.TrimSpace(raw), 10, 64)
}
