package middleware

import (
	"context"
	"net/http"
	"os"
	"regexp"
	"strings"

	"github.com/enjoydarts/sifto/api/internal/repository"
	"github.com/enjoydarts/sifto/api/internal/service"
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
			if bearerToken == "" {
				http.Error(w, "unauthorized", http.StatusUnauthorized)
				return
			}

			if identityRepo == nil || clerkVerifier == nil || !clerkVerifier.Enabled() {
				http.Error(w, "unauthorized", http.StatusUnauthorized)
				return
			}

			claims, err := clerkVerifier.Verify(r.Context(), bearerToken)
			if err != nil {
				http.Error(w, "unauthorized", http.StatusUnauthorized)
				return
			}
			identity, lookupErr := identityRepo.GetByProviderUserID(r.Context(), "clerk", claims.Subject)
			if lookupErr != nil || !uuidPattern.MatchString(identity.UserID) {
				http.Error(w, "unauthorized", http.StatusUnauthorized)
				return
			}

			ctx := context.WithValue(r.Context(), UserIDKey, identity.UserID)
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
