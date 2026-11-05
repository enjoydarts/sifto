package handler

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/enjoydarts/sifto/api/internal/repository"
	"github.com/enjoydarts/sifto/api/internal/service"
)

func TestObsidianReconnectionInvalidatesCachedSettings(t *testing.T) {
	pool := testSettingsHandlerDB(t)
	const userID = "00000000-0000-4000-8000-000000000051"
	t.Setenv("INTERNAL_API_SECRET", "obsidian-reconnection-test-secret")
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("GITHUB_APP_ID", "123")
	t.Setenv("GITHUB_APP_PRIVATE_KEY", string(pem.EncodeToMemory(&pem.Block{
		Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(key),
	})))
	github := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/app/installations/42" {
			t.Errorf("unexpected GitHub request: %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":42,"account":{"login":"me"}}`))
	}))
	defer github.Close()
	t.Setenv("GITHUB_API_BASE_URL", github.URL)
	cache := newTestJSONCache()
	settingsHandler := &SettingsHandler{cache: cache}
	before, err := settingsHandler.settingsCacheKey(context.Background(), userID)
	if err != nil {
		t.Fatal(err)
	}
	repo := repository.NewObsidianExportRepo(pool)
	handler := &InternalHandler{obsidianRepo: repo, githubApp: service.NewGitHubAppClientFromEnv(), cache: cache}
	req := httptest.NewRequest(http.MethodPost, "/api/internal/settings/obsidian-github/installation",
		bytes.NewBufferString(`{"user_id":"`+userID+`","installation_id":42,"authorized_repositories":["me/vault"]}`))
	req.Header.Set("X-Internal-Secret", "obsidian-reconnection-test-secret")
	response := httptest.NewRecorder()
	handler.UpsertObsidianGitHubInstallation(response, req)
	if response.Code != http.StatusOK {
		t.Fatalf("reconnection status = %d, body=%s", response.Code, response.Body.String())
	}
	cfg, err := repo.GetByUserID(context.Background(), userID)
	if err != nil {
		t.Fatal(err)
	}
	if !service.NewObsidianExportView(cfg, nil).GitHubAuthorized {
		t.Fatal("reconnection did not persist authorized installation")
	}
	after, err := settingsHandler.settingsCacheKey(context.Background(), userID)
	if err != nil {
		t.Fatal(err)
	}
	if before == after {
		t.Fatal("settings still selects the cached disconnected response after successful reconnection")
	}
}
