package service

import (
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/inngest/inngestgo"
)

const (
	inngestCloudflareAccessClientIDEnv     = "INNGEST_CF_ACCESS_CLIENT_ID"
	inngestCloudflareAccessClientSecretEnv = "INNGEST_CF_ACCESS_CLIENT_SECRET"
)

var configureInngestDefaultHTTPOnce sync.Once

type ingressHeaderRoundTripper struct {
	base         http.RoundTripper
	accessID     string
	accessSecret string
	baseURL      *url.URL
}

func (rt *ingressHeaderRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	cloned := req.Clone(req.Context())
	if cloned.Header == nil {
		cloned.Header = make(http.Header)
	}
	if rt.shouldDecorate(cloned.URL) && rt.accessID != "" {
		cloned.Header.Set("CF-Access-Client-Id", rt.accessID)
	}
	if rt.shouldDecorate(cloned.URL) && rt.accessSecret != "" {
		cloned.Header.Set("CF-Access-Client-Secret", rt.accessSecret)
	}
	return rt.base.RoundTrip(cloned)
}

func (rt *ingressHeaderRoundTripper) shouldDecorate(target *url.URL) bool {
	if rt.baseURL == nil || target == nil {
		return true
	}
	return strings.EqualFold(rt.baseURL.Scheme, target.Scheme) && strings.EqualFold(rt.baseURL.Host, target.Host)
}

func InngestBaseURLFromEnv() string {
	return strings.TrimSpace(os.Getenv("INNGEST_BASE_URL"))
}

func NewInngestHTTPClient(timeout time.Duration) *http.Client {
	transport := http.DefaultTransport
	baseURL := parseInngestBaseURL()
	accessID := strings.TrimSpace(os.Getenv(inngestCloudflareAccessClientIDEnv))
	accessSecret := strings.TrimSpace(os.Getenv(inngestCloudflareAccessClientSecretEnv))
	if accessID != "" || accessSecret != "" {
		transport = &ingressHeaderRoundTripper{
			base:         transport,
			accessID:     accessID,
			accessSecret: accessSecret,
			baseURL:      baseURL,
		}
	}
	return &http.Client{
		Timeout:   timeout,
		Transport: transport,
	}
}

func NewInngestClient(appID string) (inngestgo.Client, error) {
	configureInngestDefaultHTTPClient()
	opts := inngestgo.ClientOpts{
		AppID:      appID,
		HTTPClient: NewInngestHTTPClient(15 * time.Second),
	}
	if baseURL := InngestBaseURLFromEnv(); baseURL != "" {
		baseURL = strings.TrimRight(baseURL, "/")
		opts.APIBaseURL = &baseURL
		opts.EventAPIBaseURL = &baseURL
		registerURL := baseURL + "/fn/register"
		opts.RegisterURL = &registerURL
	}
	return inngestgo.NewClient(opts)
}

func configureInngestDefaultHTTPClient() {
	configureInngestDefaultHTTPOnce.Do(func() {
		http.DefaultTransport = newInngestInstrumentedTransport(http.DefaultTransport)
		http.DefaultClient.Transport = http.DefaultTransport
	})
}

func newInngestInstrumentedTransport(base http.RoundTripper) http.RoundTripper {
	if base == nil {
		base = http.DefaultTransport
	}
	baseURL := parseInngestBaseURL()
	accessID := strings.TrimSpace(os.Getenv(inngestCloudflareAccessClientIDEnv))
	accessSecret := strings.TrimSpace(os.Getenv(inngestCloudflareAccessClientSecretEnv))
	transport := base
	if accessID != "" || accessSecret != "" {
		transport = &ingressHeaderRoundTripper{
			base:         transport,
			accessID:     accessID,
			accessSecret: accessSecret,
			baseURL:      baseURL,
		}
	}
	return transport
}

func parseInngestBaseURL() *url.URL {
	base := InngestBaseURLFromEnv()
	if base == "" {
		return nil
	}
	u, err := url.Parse(base)
	if err != nil {
		return nil
	}
	return u
}
