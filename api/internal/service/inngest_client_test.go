package service

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestInngestTokensNeverSentWithoutValidOrigin(t *testing.T) {
	for _, origin := range []string{"", ":::", "relative/path", "file:///tmp/test"} {
		t.Run(origin, func(t *testing.T) {
			t.Setenv("INNGEST_BASE_URL", origin)
			t.Setenv(inngestCloudflareAccessClientIDEnv, "private-id")
			t.Setenv(inngestCloudflareAccessClientSecretEnv, "private-secret")
			base := inngestTestRoundTripper(func(req *http.Request) (*http.Response, error) {
				if req.Header.Get("CF-Access-Client-Secret") != "" || req.Header.Get("CF-Access-Client-Id") != "" {
					t.Fatal("Cloudflare credentials sent to unrelated URL")
				}
				return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader("ok"))}, nil
			})
			resp, err := newInngestInstrumentedTransport(base).RoundTrip(httptest.NewRequest("GET", "https://attacker.example/feed", nil))
			if err != nil {
				t.Fatal(err)
			}
			resp.Body.Close()
		})
	}
}

type inngestTestRoundTripper func(*http.Request) (*http.Response, error)

func (fn inngestTestRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	return fn(req)
}

type trackingReadCloser struct {
	io.Reader
	closed bool
}

func (body *trackingReadCloser) Close() error {
	body.closed = true
	return nil
}

func TestInngestRegistrationTransportClosesUpstreamBody(t *testing.T) {
	t.Setenv("INNGEST_BASE_URL", "http://inngest:8288")
	t.Setenv(inngestCloudflareAccessClientIDEnv, "")
	t.Setenv(inngestCloudflareAccessClientSecretEnv, "")

	upstreamBody := &trackingReadCloser{Reader: strings.NewReader(`{"ok":true}`)}
	base := inngestTestRoundTripper(func(*http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     make(http.Header),
			Body:       upstreamBody,
		}, nil
	})
	req, err := http.NewRequest(http.MethodPost, "http://inngest:8288/fn/register", nil)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}

	resp, err := newInngestInstrumentedTransport(base).RoundTrip(req)
	if err != nil {
		t.Fatalf("round trip: %v", err)
	}
	defer resp.Body.Close()

	if !upstreamBody.closed {
		t.Fatal("expected upstream registration response body to be closed")
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read replayed body: %v", err)
	}
	if got, want := string(body), `{"ok":true}`; got != want {
		t.Fatalf("replayed body = %q, want %q", got, want)
	}
}

func TestInngestRegistrationTransportLeavesOtherResponsesStreaming(t *testing.T) {
	t.Setenv("INNGEST_BASE_URL", "http://inngest:8288")
	t.Setenv(inngestCloudflareAccessClientIDEnv, "")
	t.Setenv(inngestCloudflareAccessClientSecretEnv, "")

	upstreamBody := &trackingReadCloser{Reader: strings.NewReader("event response")}
	base := inngestTestRoundTripper(func(*http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     make(http.Header),
			Body:       upstreamBody,
		}, nil
	})
	req, err := http.NewRequest(http.MethodGet, "http://inngest:8288/v1/events", nil)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}

	resp, err := newInngestInstrumentedTransport(base).RoundTrip(req)
	if err != nil {
		t.Fatalf("round trip: %v", err)
	}
	if upstreamBody.closed {
		t.Fatal("expected non-registration response body to remain streaming")
	}
	if err := resp.Body.Close(); err != nil {
		t.Fatalf("close response body: %v", err)
	}
}
