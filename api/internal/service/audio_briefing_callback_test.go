package service

import (
	"testing"
	"time"
)

func TestHashAudioBriefingCallbackTokenIsStable(t *testing.T) {
	if got, want := HashAudioBriefingCallbackToken("token-value"), HashAudioBriefingCallbackToken("token-value"); got != want {
		t.Fatalf("HashAudioBriefingCallbackToken stable hash mismatch: got %q want %q", got, want)
	}
	if got := HashAudioBriefingCallbackToken("token-value"); len(got) != 64 {
		t.Fatalf("HashAudioBriefingCallbackToken length = %d, want 64", len(got))
	}
}

func TestIssueAudioBriefingCallbackToken(t *testing.T) {
	now := time.Date(2026, 3, 24, 12, 0, 0, 0, time.UTC)

	rawToken, requestID, tokenHash, expiresAt, err := IssueAudioBriefingCallbackToken(now, 30*time.Minute)
	if err != nil {
		t.Fatalf("IssueAudioBriefingCallbackToken returned error: %v", err)
	}
	if rawToken == "" {
		t.Fatal("rawToken is empty")
	}
	if requestID == "" {
		t.Fatal("requestID is empty")
	}
	if tokenHash != HashAudioBriefingCallbackToken(rawToken) {
		t.Fatalf("tokenHash = %q, want hash of raw token", tokenHash)
	}
	if want := now.Add(30 * time.Minute); !expiresAt.Equal(want) {
		t.Fatalf("expiresAt = %s, want %s", expiresAt, want)
	}
}

func TestIssueAudioBriefingCallbackTokenUsesDefaultTTL(t *testing.T) {
	now := time.Date(2026, 3, 24, 12, 0, 0, 0, time.UTC)

	_, _, _, expiresAt, err := IssueAudioBriefingCallbackToken(now, 0)
	if err != nil {
		t.Fatalf("IssueAudioBriefingCallbackToken returned error: %v", err)
	}
	if want := now.Add(time.Hour); !expiresAt.Equal(want) {
		t.Fatalf("expiresAt = %s, want %s", expiresAt, want)
	}
}
