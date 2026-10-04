package handler

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/enjoydarts/sifto/api/internal/repository"
)

func TestInternalUserRegistrationIsClosed(t *testing.T) {
	t.Setenv("INTERNAL_API_SECRET", "registration-test-secret")
	ctx := context.Background()
	pool, err := repository.NewPool(ctx)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	const email = "registration-policy-test@example.com"
	if _, err := pool.Exec(ctx, "DELETE FROM users WHERE email = $1", email); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _, _ = pool.Exec(context.Background(), "DELETE FROM users WHERE email = $1", email) })
	h := &InternalHandler{userRepo: repository.NewUserRepo(pool), identityRepo: repository.NewUserIdentityRepo(pool)}

	for _, tc := range []struct {
		name   string
		body   string
		handle http.HandlerFunc
	}{
		{"upsert", `{"email":"registration-policy-test@example.com","name":"New user"}`, h.UpsertUser},
		{"resolve-identity", `{"provider":"clerk","provider_user_id":"registration-policy-new-user","email":"registration-policy-test@example.com"}`, h.ResolveIdentity},
	} {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(tc.body))
			req.Header.Set("X-Internal-Secret", "registration-test-secret")
			response := httptest.NewRecorder()
			tc.handle(response, req)
			if response.Code != http.StatusForbidden {
				t.Errorf("new user status = %d, want 403; body=%s", response.Code, response.Body.String())
			}
			var count int
			if err := pool.QueryRow(ctx, "SELECT count(*) FROM users WHERE email = $1", email).Scan(&count); err != nil {
				t.Fatal(err)
			}
			if count != 0 {
				t.Errorf("new user was persisted: count=%d", count)
			}
			_, _ = pool.Exec(ctx, "DELETE FROM users WHERE email = $1", email)
		})
	}

	if _, err := pool.Exec(ctx, "INSERT INTO users (email, name) VALUES ($1, 'Existing user')", email); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name   string
		body   string
		handle http.HandlerFunc
	}{
		{"existing-upsert", `{"email":"registration-policy-test@example.com","name":"Updated user"}`, h.UpsertUser},
		{"existing-email-identity", `{"provider":"clerk","provider_user_id":"registration-policy-existing-user","email":"registration-policy-test@example.com"}`, h.ResolveIdentity},
		{"existing-identity", `{"provider":"clerk","provider_user_id":"registration-policy-existing-user","email":"registration-policy-test@example.com"}`, h.ResolveIdentity},
	} {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(tc.body))
			req.Header.Set("X-Internal-Secret", "registration-test-secret")
			response := httptest.NewRecorder()
			tc.handle(response, req)
			if response.Code != http.StatusOK {
				t.Errorf("existing user status = %d, want 200; body=%s", response.Code, response.Body.String())
			}
		})
	}
}
