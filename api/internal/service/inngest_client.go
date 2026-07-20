package service

import (
	"bytes"
	"fmt"
	"io"
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

type inngestRegistrationResponseRoundTripper struct {
	base    http.RoundTripper
	baseURL *url.URL
}

func (rt *inngestRegistrationResponseRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	resp, err := rt.base.RoundTrip(req)
	if err != nil || resp == nil || resp.Body == nil || !rt.shouldBuffer(req.URL) {
		return resp, err
	}

	body, readErr := io.ReadAll(resp.Body)
	closeErr := resp.Body.Close()
	if readErr != nil {
		return nil, fmt.Errorf("read inngest registration response: %w", readErr)
	}
	if closeErr != nil {
		return nil, fmt.Errorf("close inngest registration response: %w", closeErr)
	}

	resp.Body = io.NopCloser(bytes.NewReader(body))
	resp.ContentLength = int64(len(body))
	return resp, nil
}

func (rt *inngestRegistrationResponseRoundTripper) shouldBuffer(target *url.URL) bool {
	if rt.baseURL == nil || target == nil {
		return false
	}
	if !strings.EqualFold(rt.baseURL.Scheme, target.Scheme) || !strings.EqualFold(rt.baseURL.Host, target.Host) {
		return false
	}
	basePath := strings.TrimRight(rt.baseURL.EscapedPath(), "/")
	targetPath := strings.TrimRight(target.EscapedPath(), "/")
	return targetPath == basePath+"/fn/register"
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
	transport = &inngestRegistrationResponseRoundTripper{
		base:    transport,
		baseURL: baseURL,
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
