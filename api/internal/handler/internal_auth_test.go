package handler

import (
	"net/http/httptest"
	"testing"
)

func TestCheckInternalSecretFailsClosed(t *testing.T) {
	t.Setenv("INTERNAL_API_SECRET", "")
	req := httptest.NewRequest("GET", "/api/internal/debug/system-status", nil)
	if checkInternalSecret(req) {
		t.Fatal("checkInternalSecret() = true with an empty configured secret")
	}
}

func TestCheckInternalSecretRequiresMatchingNonEmptyHeader(t *testing.T) {
	t.Setenv("INTERNAL_API_SECRET", "internal-secret")
	req := httptest.NewRequest("GET", "/api/internal/debug/system-status", nil)
	if checkInternalSecret(req) {
		t.Fatal("checkInternalSecret() = true without a request header")
	}
	req.Header.Set("X-Internal-Secret", "internal-secret")
	if !checkInternalSecret(req) {
		t.Fatal("checkInternalSecret() = false with the matching secret")
	}
}

func TestCheckInternalAdminRequiresAllowlistedEmail(t *testing.T) {
	t.Setenv("INTERNAL_API_SECRET", "internal-secret")
	t.Setenv("PROMPT_ADMIN_EMAILS", "admin@example.com")
	req := httptest.NewRequest("GET", "/api/internal/debug/system-status", nil)
	req.Header.Set("X-Internal-Secret", "internal-secret")
	req.Header.Set("X-Internal-User-Email", "user@example.com")
	if checkInternalAdmin(req) {
		t.Fatal("checkInternalAdmin() = true for a non-admin email")
	}
	req.Header.Set("X-Internal-User-Email", "ADMIN@example.com")
	if !checkInternalAdmin(req) {
		t.Fatal("checkInternalAdmin() = false for an allowlisted email")
	}
}
