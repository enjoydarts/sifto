package middleware

import (
	"net/http"
	"os"
)

func IgnoreClientCacheBust(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Has("cache_bust") && !(os.Getenv("INNGEST_DEV") == "true" && os.Getenv("ALLOW_DEV_AUTH_BYPASS") == "true") {
			r = r.Clone(r.Context())
			query := r.URL.Query()
			query.Del("cache_bust")
			r.URL.RawQuery = query.Encode()
		}
		next.ServeHTTP(w, r)
	})
}
