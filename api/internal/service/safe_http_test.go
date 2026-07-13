package service

import (
	"context"
	"net/http"
	"testing"
	"time"
)

type testRoundTripper func(*http.Request) (*http.Response, error)

func (fn testRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	return fn(req)
}

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

func TestNewPublicHTTPClientDoesNotAssumeDefaultTransportType(t *testing.T) {
	previous := http.DefaultTransport
	http.DefaultTransport = testRoundTripper(func(*http.Request) (*http.Response, error) { return nil, nil })
	t.Cleanup(func() { http.DefaultTransport = previous })

	client := NewPublicHTTPClient(time.Second)
	if client == nil || client.Transport == nil {
		t.Fatal("NewPublicHTTPClient() returned an unusable client")
	}
}
