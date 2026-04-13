package handler

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"runtime/debug"

	"github.com/enjoydarts/sifto/api/internal/repository"
)

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(v)
}

func writeRepoError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, repository.ErrNotFound):
		http.Error(w, "not found", http.StatusNotFound)
	case errors.Is(err, repository.ErrConflict):
		http.Error(w, "conflict", http.StatusConflict)
	default:
		errID := generateErrorID()
		log.Printf("internal error [%s]: %v", errID, err)
		http.Error(w, fmt.Sprintf("internal server error (ref: %s)", errID), http.StatusInternalServerError)
	}
}

func generateErrorID() string {
	b := make([]byte, 4)
	if _, err := rand.Read(b); err != nil {
		return "00000000"
	}
	return hex.EncodeToString(b)
}

func safeGo(fn func()) {
	go func() {
		defer func() {
			if r := recover(); r != nil {
				log.Printf("goroutine panic recovered: %v\n%s", r, debug.Stack())
			}
		}()
		fn()
	}()
}
