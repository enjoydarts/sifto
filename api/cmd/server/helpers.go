package main

import (
	"net/http"
)

type loggingResponseWriter struct {
	http.ResponseWriter
	status      int
	wroteHeader bool
}

func (w *loggingResponseWriter) WriteHeader(status int) {
	w.status = status
	w.wroteHeader = true
	w.ResponseWriter.WriteHeader(status)
}

func (w *loggingResponseWriter) Write(p []byte) (int, error) {
	if !w.wroteHeader {
		w.WriteHeader(http.StatusOK)
	}
	return w.ResponseWriter.Write(p)
}

func ensureInngestPutNoContent(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		lrw := &loggingResponseWriter{ResponseWriter: w}
		defer func() {
			if r.Method == http.MethodPut && !lrw.wroteHeader {
				if lrw.Header().Get("X-Inngest-Sync-Kind") != "" {
					lrw.WriteHeader(http.StatusNoContent)
				}
			}
		}()

		next.ServeHTTP(lrw, r)
	})
}
