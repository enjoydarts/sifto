package service

import (
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/inngest/inngestgo"
)

const (
	inngestCloudflareAccessClientIDEnv     = "INNGEST_CF_ACCESS_CLIENT_ID"
	inngestCloudflareAccessClientSecretEnv = "INNGEST_CF_ACCESS_CLIENT_SECRET"
)

type ingressHeaderRoundTripper struct {
	base         http.RoundTripper
	accessID     string
	accessSecret string
}

func (rt *ingressHeaderRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	cloned := req.Clone(req.Context())
	if cloned.Header == nil {
		cloned.Header = make(http.Header)
	}
	if rt.accessID != "" {
		cloned.Header.Set("CF-Access-Client-Id", rt.accessID)
	}
	if rt.accessSecret != "" {
		cloned.Header.Set("CF-Access-Client-Secret", rt.accessSecret)
	}
	return rt.base.RoundTrip(cloned)
}

func InngestBaseURLFromEnv() string {
	return strings.TrimSpace(os.Getenv("INNGEST_BASE_URL"))
}

func NewInngestHTTPClient(timeout time.Duration) *http.Client {
	transport := http.DefaultTransport
	accessID := strings.TrimSpace(os.Getenv(inngestCloudflareAccessClientIDEnv))
	accessSecret := strings.TrimSpace(os.Getenv(inngestCloudflareAccessClientSecretEnv))
	if accessID != "" || accessSecret != "" {
		transport = &ingressHeaderRoundTripper{
			base:         transport,
			accessID:     accessID,
			accessSecret: accessSecret,
		}
	}
	return &http.Client{
		Timeout:   timeout,
		Transport: transport,
	}
}

func NewInngestClient(appID string) (inngestgo.Client, error) {
	opts := inngestgo.ClientOpts{
		AppID:      appID,
		HTTPClient: NewInngestHTTPClient(15 * time.Second),
	}
	if baseURL := InngestBaseURLFromEnv(); baseURL != "" {
		baseURL = strings.TrimRight(baseURL, "/")
		opts.APIBaseURL = &baseURL
		opts.EventAPIBaseURL = &baseURL
		opts.RegisterURL = &baseURL
	}
	return inngestgo.NewClient(opts)
}
