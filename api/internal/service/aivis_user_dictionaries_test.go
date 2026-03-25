package service

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestAivisUserDictionaryServiceListUsesBearerToken(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "Bearer test-token" {
			t.Fatalf("Authorization = %q, want Bearer token", got)
		}
		if r.URL.Path != "/v1/user-dictionaries" {
			t.Fatalf("path = %q, want /v1/user-dictionaries", r.URL.Path)
		}
		_, _ = io.WriteString(w, `{"user_dictionaries":[{"uuid":"5b6f7aa3-2c34-4ad7-aad0-4e1d683d7861","name":"固有名詞辞書","description":"","word_count":12,"created_at":"2026-03-25T00:00:00Z","updated_at":"2026-03-25T01:00:00Z"}]}`)
	}))
	defer server.Close()

	svc := NewAivisUserDictionaryService(nil, nil)
	svc.http = server.Client()
	svc.baseURL = server.URL + "/v1/user-dictionaries"

	items, err := svc.listWithToken(context.Background(), "test-token")
	if err != nil {
		t.Fatalf("listWithToken(...) error = %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("len(items) = %d, want 1", len(items))
	}
	if items[0].UUID != "5b6f7aa3-2c34-4ad7-aad0-4e1d683d7861" {
		t.Fatalf("uuid = %q, want expected uuid", items[0].UUID)
	}
}
