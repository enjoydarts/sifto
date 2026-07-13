package service

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"strings"
	"time"
)

func NewPublicHTTPClient(timeout time.Duration) *http.Client {
	dialer := &net.Dialer{Timeout: 10 * time.Second, KeepAlive: 30 * time.Second}
	transport := &http.Transport{
		Proxy:                 http.ProxyFromEnvironment,
		ForceAttemptHTTP2:     true,
		MaxIdleConns:          100,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ExpectContinueTimeout: time.Second,
	}
	transport.DialContext = func(ctx context.Context, network, address string) (net.Conn, error) {
		host, port, err := net.SplitHostPort(address)
		if err != nil {
			return nil, err
		}
		ips, err := net.DefaultResolver.LookupNetIP(ctx, "ip", host)
		if err != nil {
			return nil, fmt.Errorf("resolve public URL host: %w", err)
		}
		if len(ips) == 0 {
			return nil, fmt.Errorf("public URL host has no IP addresses")
		}
		for _, ip := range ips {
			if !isPublicIP(ip) {
				return nil, fmt.Errorf("public URL resolves to a non-public IP")
			}
		}
		return dialer.DialContext(ctx, network, net.JoinHostPort(ips[0].String(), port))
	}
	client := &http.Client{Transport: transport, Timeout: timeout}
	client.CheckRedirect = func(req *http.Request, via []*http.Request) error {
		if len(via) >= 5 {
			return fmt.Errorf("too many redirects")
		}
		return ValidatePublicHTTPURL(req.Context(), req.URL.String())
	}
	return client
}

func ValidatePublicHTTPURL(ctx context.Context, rawURL string) error {
	parsed, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil {
		return fmt.Errorf("invalid URL: %w", err)
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return fmt.Errorf("URL scheme must be http or https")
	}
	if parsed.User != nil || parsed.Hostname() == "" {
		return fmt.Errorf("URL must not contain credentials and must have a host")
	}
	ips, err := net.DefaultResolver.LookupNetIP(ctx, "ip", parsed.Hostname())
	if err != nil {
		return fmt.Errorf("resolve URL host: %w", err)
	}
	if len(ips) == 0 {
		return fmt.Errorf("URL host has no IP addresses")
	}
	for _, ip := range ips {
		if !isPublicIP(ip) {
			return fmt.Errorf("URL resolves to a non-public IP")
		}
	}
	return nil
}

func isPublicIP(ip netip.Addr) bool {
	return ip.IsValid() && ip.IsGlobalUnicast() && !ip.IsPrivate() && !ip.IsLoopback() && !ip.IsLinkLocalUnicast()
}
