package handler

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/minoru-kitayama/sifto/api/internal/middleware"
	"github.com/minoru-kitayama/sifto/api/internal/repository"
)

type DigestHandler struct{ repo *repository.DigestRepo }

func NewDigestHandler(repo *repository.DigestRepo) *DigestHandler { return &DigestHandler{repo} }

func (h *DigestHandler) List(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r)
	digests, err := h.repo.List(r.Context(), userID)
	if err != nil {
		writeRepoError(w, err)
		return
	}
	writeJSON(w, digests)
}

func (h *DigestHandler) GetDetail(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r)
	id := chi.URLParam(r, "id")
	d, err := h.repo.GetDetail(r.Context(), id, userID)
	if err != nil {
		writeRepoError(w, err)
		return
	}
	writeJSON(w, d)
}

func (h *DigestHandler) GetLatest(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r)
	d, err := h.repo.GetLatest(r.Context(), userID)
	if err != nil {
		writeRepoError(w, err)
		return
	}
	writeJSON(w, d)
}
