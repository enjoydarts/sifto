package service

import (
	"context"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

type ClerkClaims struct {
	Subject string
	Email   string
}

type ClerkTokenVerifier struct {
	jwksURL  string
	issuer   string
	audience string
	client   *http.Client

	mu        sync.RWMutex
	keys      map[string]*rsa.PublicKey
	fetchedAt time.Time
}

type clerkJWKS struct {
	Keys []clerkJWK `json:"keys"`
}

type clerkJWK struct {
	KeyID string `json:"kid"`
	Kty   string `json:"kty"`
	Alg   string `json:"alg"`
	N     string `json:"n"`
	E     string `json:"e"`
}

func NewClerkTokenVerifierFromEnv() *ClerkTokenVerifier {
	jwksURL := strings.TrimSpace(os.Getenv("CLERK_JWKS_URL"))
	issuer := strings.TrimSpace(os.Getenv("CLERK_JWT_ISSUER"))
	if jwksURL == "" && issuer != "" {
		jwksURL = strings.TrimRight(issuer, "/") + "/.well-known/jwks.json"
	}
	if jwksURL == "" {
		return nil
	}
	return &ClerkTokenVerifier{
		jwksURL:  jwksURL,
		issuer:   issuer,
		audience: strings.TrimSpace(os.Getenv("CLERK_JWT_AUDIENCE")),
		client: &http.Client{
			Timeout: 10 * time.Second,
		},
		keys: map[string]*rsa.PublicKey{},
	}
}

func (v *ClerkTokenVerifier) Enabled() bool {
	return v != nil && v.jwksURL != ""
}

func (v *ClerkTokenVerifier) Verify(ctx context.Context, token string) (*ClerkClaims, error) {
	if !v.Enabled() {
		return nil, errors.New("clerk verifier disabled")
	}

	claims := jwt.MapClaims{}
	parser := jwt.NewParser(
		jwt.WithValidMethods([]string{"RS256", "RS384", "RS512"}),
		jwt.WithIssuedAt(),
		jwt.WithExpirationRequired(),
	)

	parsed, err := parser.ParseWithClaims(token, claims, func(t *jwt.Token) (any, error) {
		kid, _ := t.Header["kid"].(string)
		if strings.TrimSpace(kid) == "" {
			return nil, errors.New("clerk token missing kid")
		}
		return v.getKey(ctx, kid)
	})
	if err != nil || !parsed.Valid {
		if err == nil {
			err = errors.New("invalid clerk token")
		}
		return nil, err
	}

	subject, _ := claims["sub"].(string)
	if strings.TrimSpace(subject) == "" {
		return nil, errors.New("clerk token missing sub")
	}
	if v.issuer != "" {
		issuer, _ := claims["iss"].(string)
		if issuer != v.issuer {
			return nil, fmt.Errorf("unexpected clerk issuer: %s", issuer)
		}
	}
	if v.audience != "" && !claimsHasAudience(claims["aud"], v.audience) {
		return nil, errors.New("unexpected clerk audience")
	}

	email, _ := claims["email"].(string)
	return &ClerkClaims{
		Subject: subject,
		Email:   email,
	}, nil
}

func (v *ClerkTokenVerifier) getKey(ctx context.Context, kid string) (*rsa.PublicKey, error) {
	v.mu.RLock()
	if key, ok := v.keys[kid]; ok && time.Since(v.fetchedAt) < 15*time.Minute {
		v.mu.RUnlock()
		return key, nil
	}
	v.mu.RUnlock()

	if err := v.refreshKeys(ctx); err != nil {
		return nil, err
	}

	v.mu.RLock()
	defer v.mu.RUnlock()
	key, ok := v.keys[kid]
	if !ok {
		return nil, fmt.Errorf("clerk jwks key not found: %s", kid)
	}
	return key, nil
}

func (v *ClerkTokenVerifier) refreshKeys(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, v.jwksURL, nil)
	if err != nil {
		return err
	}
	resp, err := v.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		return fmt.Errorf("clerk jwks fetch failed: %s", resp.Status)
	}

	var payload clerkJWKS
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return err
	}

	keys := make(map[string]*rsa.PublicKey, len(payload.Keys))
	for _, jwk := range payload.Keys {
		if jwk.Kty != "RSA" || jwk.KeyID == "" || jwk.N == "" || jwk.E == "" {
			continue
		}
		key, err := parseRSAPublicKey(jwk.N, jwk.E)
		if err != nil {
			return err
		}
		keys[jwk.KeyID] = key
	}
	if len(keys) == 0 {
		return errors.New("clerk jwks returned no rsa keys")
	}

	v.mu.Lock()
	defer v.mu.Unlock()
	v.keys = keys
	v.fetchedAt = time.Now()
	return nil
}

func parseRSAPublicKey(modulusB64, exponentB64 string) (*rsa.PublicKey, error) {
	modulusBytes, err := base64.RawURLEncoding.DecodeString(modulusB64)
	if err != nil {
		return nil, err
	}
	exponentBytes, err := base64.RawURLEncoding.DecodeString(exponentB64)
	if err != nil {
		return nil, err
	}
	exponent := 0
	for _, b := range exponentBytes {
		exponent = exponent<<8 + int(b)
	}
	if exponent == 0 {
		return nil, errors.New("invalid rsa exponent")
	}
	return &rsa.PublicKey{
		N: new(big.Int).SetBytes(modulusBytes),
		E: exponent,
	}, nil
}

func claimsHasAudience(raw any, expected string) bool {
	switch value := raw.(type) {
	case string:
		return value == expected
	case []any:
		for _, entry := range value {
			text, _ := entry.(string)
			if text == expected {
				return true
			}
		}
	case []string:
		for _, entry := range value {
			if entry == expected {
				return true
			}
		}
	}
	return false
}
