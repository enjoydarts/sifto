package handler

import (
	"net/http"

	"github.com/enjoydarts/sifto/api/internal/middleware"
	"github.com/enjoydarts/sifto/api/internal/repository"
	"github.com/enjoydarts/sifto/api/internal/service"
	"github.com/go-chi/chi/v5"
)

type DigestHandler struct {
	repo   *repository.DigestRepo
	detail *service.DigestDetailService
}

func NewDigestHandler(repo *repository.DigestRepo) *DigestHandler {
	return &DigestHandler{repo: repo, detail: service.NewDigestDetailService(repo)}
}

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
	d, err := h.detail.Get(r.Context(), id, userID)
	if err != nil {
		writeRepoError(w, err)
		return
	}
	writeJSON(w, d)
}

func (h *DigestHandler) GetLatest(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r)
	d, err := h.detail.GetLatest(r.Context(), userID)
	if err != nil {
		writeRepoError(w, err)
		return
	}
	writeJSON(w, d)
}
