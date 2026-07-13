package service

import (
	"context"
	"testing"
)

func TestValidatePublicHTTPURLRejectsPrivateAndUnsafeTargets(t *testing.T) {
	for _, rawURL := range []string{
		"http://127.0.0.1/admin",
		"http://10.0.0.1/",
		"http://169.254.169.254/latest/meta-data/",
		"http://[::1]/",
		"file:///etc/passwd",
		"https://user:pass@example.com/",
	} {
		if err := ValidatePublicHTTPURL(context.Background(), rawURL); err == nil {
			t.Errorf("ValidatePublicHTTPURL(%q) succeeded, want rejection", rawURL)
		}
	}
}
