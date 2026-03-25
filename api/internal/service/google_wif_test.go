package service

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/golang-jwt/jwt/v5"
)

func TestParseSubjectTokenText(t *testing.T) {
	got, err := parseSubjectToken([]byte("  token-value \n"), struct {
		Type                  string `json:"type"`
		SubjectTokenFieldName string `json:"subject_token_field_name"`
	}{})
	if err != nil {
		t.Fatalf("parseSubjectToken returned error: %v", err)
	}
	if got != "token-value" {
		t.Fatalf("parseSubjectToken text = %q, want %q", got, "token-value")
	}
}

func TestParseSubjectTokenJSON(t *testing.T) {
	got, err := parseSubjectToken([]byte(`{"id_token":"subject-token"}`), struct {
		Type                  string `json:"type"`
		SubjectTokenFieldName string `json:"subject_token_field_name"`
	}{
		Type:                  "json",
		SubjectTokenFieldName: "id_token",
	})
	if err != nil {
		t.Fatalf("parseSubjectToken returned error: %v", err)
	}
	if got != "subject-token" {
		t.Fatalf("parseSubjectToken json = %q, want %q", got, "subject-token")
	}
}

func TestGoogleWIFReadConfigSupportsInlineJSON(t *testing.T) {
	t.Setenv("GOOGLE_APPLICATION_CREDENTIALS", `{
		"type":"external_account",
		"audience":"//iam.googleapis.com/projects/123/locations/global/workloadIdentityPools/pool/providers/provider",
		"subject_token_type":"urn:ietf:params:oauth:token-type:id_token",
		"token_url":"https://sts.googleapis.com/v1/token",
		"credential_source":{"file":"/tmp/token"}
	}`)

	source := NewGoogleWIFTokenSource()
	cfg, err := source.readConfig()
	if err != nil {
		t.Fatalf("readConfig returned error: %v", err)
	}
	if cfg.Audience == "" {
		t.Fatalf("expected audience to be parsed")
	}
	if cfg.TokenURL != "https://sts.googleapis.com/v1/token" {
		t.Fatalf("unexpected token url: %q", cfg.TokenURL)
	}
}

func TestGoogleWIFReadConfigSupportsFilePath(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "google-wif.json")
	if err := os.WriteFile(configPath, []byte(`{
		"type":"external_account",
		"audience":"//iam.googleapis.com/projects/123/locations/global/workloadIdentityPools/pool/providers/provider",
		"subject_token_type":"urn:ietf:params:oauth:token-type:id_token",
		"token_url":"https://sts.googleapis.com/v1/token",
		"credential_source":{"file":"/tmp/token"}
	}`), 0o600); err != nil {
		t.Fatalf("WriteFile returned error: %v", err)
	}
	t.Setenv("GOOGLE_APPLICATION_CREDENTIALS", configPath)

	source := NewGoogleWIFTokenSource()
	cfg, err := source.readConfig()
	if err != nil {
		t.Fatalf("readConfig returned error: %v", err)
	}
	if cfg.Audience == "" {
		t.Fatalf("expected audience to be parsed")
	}
	if cfg.TokenURL != "https://sts.googleapis.com/v1/token" {
		t.Fatalf("unexpected token url: %q", cfg.TokenURL)
	}
}

func TestGoogleWIFAccessTokenSupportsServiceAccountJSON(t *testing.T) {
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("GenerateKey returned error: %v", err)
	}
	privateKeyPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(privateKey),
	})

	var tokenURL string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("unexpected method: %s", r.Method)
		}
		if err := r.ParseForm(); err != nil {
			t.Fatalf("ParseForm returned error: %v", err)
		}
		if got := r.Form.Get("grant_type"); got != "urn:ietf:params:oauth:grant-type:jwt-bearer" {
			t.Fatalf("grant_type = %q, want jwt-bearer", got)
		}
		assertion := r.Form.Get("assertion")
		if assertion == "" {
			t.Fatalf("expected assertion to be present")
		}
		token, err := jwt.Parse(assertion, func(token *jwt.Token) (any, error) {
			if token.Method.Alg() != jwt.SigningMethodRS256.Alg() {
				t.Fatalf("unexpected signing method: %s", token.Method.Alg())
			}
			return &privateKey.PublicKey, nil
		})
		if err != nil {
			t.Fatalf("Parse returned error: %v", err)
		}
		claims, ok := token.Claims.(jwt.MapClaims)
		if !ok {
			t.Fatalf("unexpected claims type: %T", token.Claims)
		}
		if got := claims["iss"]; got != "runner@example.iam.gserviceaccount.com" {
			t.Fatalf("iss = %v, want client email", got)
		}
		if got := claims["sub"]; got != "runner@example.iam.gserviceaccount.com" {
			t.Fatalf("sub = %v, want client email", got)
		}
		if got := claims["aud"]; got != tokenURL {
			t.Fatalf("aud = %v, want %s", got, tokenURL)
		}
		if got := claims["scope"]; got != "https://www.googleapis.com/auth/cloud-platform" {
			t.Fatalf("scope = %v, want cloud-platform scope", got)
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]string{"access_token": "service-account-token"})
	}))
	defer server.Close()
	tokenURL = server.URL

	source := &GoogleWIFTokenSource{
		http:       server.Client(),
		configPath: mustServiceAccountJSON(t, tokenURL, string(privateKeyPEM)),
		scope:      "https://www.googleapis.com/auth/cloud-platform",
	}
	got, err := source.AccessToken(context.Background())
	if err != nil {
		t.Fatalf("AccessToken returned error: %v", err)
	}
	if got != "service-account-token" {
		t.Fatalf("AccessToken = %q, want %q", got, "service-account-token")
	}
}

func mustServiceAccountJSON(t *testing.T, tokenURL string, privateKeyPEM string) string {
	t.Helper()
	raw := map[string]string{
		"type":         "service_account",
		"client_email": "runner@example.iam.gserviceaccount.com",
		"private_key":  privateKeyPEM,
		"token_uri":    tokenURL,
	}
	encoded, err := json.Marshal(raw)
	if err != nil {
		t.Fatalf("Marshal returned error: %v", err)
	}
	return string(encoded)
}
