package middleware

import (
	"context"
	"errors"
	"net/http"
	"os"
	"regexp"
	"strings"

	"github.com/enjoydarts/sifto/api/internal/repository"
	"github.com/enjoydarts/sifto/api/internal/service"
	"github.com/golang-jwt/jwt/v5"
)

type contextKey string

const UserIDKey contextKey = "userID"

var uuidPattern = regexp.MustCompile("^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[1-5][0-9a-fA-F]{3}-[89abAB][0-9a-fA-F]{3}-[0-9a-fA-F]{12}$")

func Auth(identityRepo *repository.UserIdentityRepo, clerkVerifier *service.ClerkTokenVerifier) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if os.Getenv("INNGEST_DEV") == "true" && os.Getenv("ALLOW_DEV_AUTH_BYPASS") == "true" {
				if userID := devUserID(r); userID != "" {
					ctx := context.WithValue(r.Context(), UserIDKey, userID)
					next.ServeHTTP(w, r.WithContext(ctx))
					return
				}
			}

			bearerToken := extractBearerToken(r)
			cookieToken := extractNextAuthCookieToken(r)
			if bearerToken == "" && cookieToken == "" {
				http.Error(w, "unauthorized", http.StatusUnauthorized)
				return
			}

			if bearerToken != "" && identityRepo != nil && clerkVerifier != nil && clerkVerifier.Enabled() {
				claims, err := clerkVerifier.Verify(r.Context(), bearerToken)
				if err == nil {
					identity, lookupErr := identityRepo.GetByProviderUserID(r.Context(), "clerk", claims.Subject)
					if lookupErr == nil && uuidPattern.MatchString(identity.UserID) {
						ctx := context.WithValue(r.Context(), UserIDKey, identity.UserID)
						next.ServeHTTP(w, r.WithContext(ctx))
						return
					}
					http.Error(w, "unauthorized", http.StatusUnauthorized)
					return
				}
			}

			token := bearerToken
			if token == "" {
				token = cookieToken
			}
			userID, err := parseNextAuthUserID(token)
			if err != nil || userID == "" || !uuidPattern.MatchString(userID) {
				http.Error(w, "unauthorized", http.StatusUnauthorized)
				return
			}

			ctx := context.WithValue(r.Context(), UserIDKey, userID)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func devUserID(r *http.Request) string {
	if v := strings.TrimSpace(r.Header.Get("X-Dev-User-Id")); v != "" {
		return v
	}
	if v := strings.TrimSpace(os.Getenv("DEV_AUTH_USER_ID")); v != "" {
		return v
	}
	return ""
}

func GetUserID(r *http.Request) string {
	v, _ := r.Context().Value(UserIDKey).(string)
	return v
}

func extractBearerToken(r *http.Request) string {
	if h := r.Header.Get("Authorization"); strings.HasPrefix(h, "Bearer ") {
		return strings.TrimPrefix(h, "Bearer ")
	}
	return ""
}

func extractNextAuthCookieToken(r *http.Request) string {
	if c, err := r.Cookie("next-auth.session-token"); err == nil {
		return c.Value
	}
	if c, err := r.Cookie("__Secure-next-auth.session-token"); err == nil {
		return c.Value
	}
	return ""
}

func parseNextAuthUserID(token string) (string, error) {
	secret := os.Getenv("NEXTAUTH_SECRET")
	parsed, err := jwt.Parse(token, func(t *jwt.Token) (any, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, jwt.ErrSignatureInvalid
		}
		return []byte(secret), nil
	})
	if err != nil || !parsed.Valid {
		if err == nil {
			err = errors.New("invalid nextauth token")
		}
		return "", err
	}
	claims, ok := parsed.Claims.(jwt.MapClaims)
	if !ok {
		return "", errors.New("invalid nextauth claims")
	}
	userID, _ := claims["sub"].(string)
	return userID, nil
}
