package service

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"time"
)

func HashAudioBriefingCallbackToken(raw string) string {
	sum := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(sum[:])
}

func IssueAudioBriefingCallbackToken(now time.Time, ttl time.Duration) (rawToken string, requestID string, tokenHash string, expiresAt time.Time, err error) {
	if now.IsZero() {
		now = time.Now().UTC()
	}
	if ttl <= 0 {
		ttl = time.Hour
	}
	rawToken, err = randomAudioBriefingToken(32)
	if err != nil {
		return "", "", "", time.Time{}, err
	}
	requestID, err = randomAudioBriefingToken(12)
	if err != nil {
		return "", "", "", time.Time{}, err
	}
	return rawToken, requestID, HashAudioBriefingCallbackToken(rawToken), now.Add(ttl), nil
}

func randomAudioBriefingToken(n int) (string, error) {
	if n <= 0 {
		return "", fmt.Errorf("token length must be positive")
	}
	buf := make([]byte, n)
	if _, err := io.ReadFull(rand.Reader, buf); err != nil {
		return "", err
	}
	return hex.EncodeToString(buf), nil
}
