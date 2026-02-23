package handler

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/minoru-kitayama/sifto/api/internal/repository"
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
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}
