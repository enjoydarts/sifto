package handler

import (
	"encoding/json"
	"net/http"
	"os"

	"github.com/minoru-kitayama/sifto/api/internal/repository"
)

type InternalHandler struct {
	userRepo *repository.UserRepo
}

func NewInternalHandler(userRepo *repository.UserRepo) *InternalHandler {
	return &InternalHandler{userRepo: userRepo}
}

// UpsertUser はメールアドレスでユーザーを取得または作成して UUID を返す内部エンドポイント。
// Next.js の NextAuth jwt コールバックから呼ばれる。X-Internal-Secret で保護。
func (h *InternalHandler) UpsertUser(w http.ResponseWriter, r *http.Request) {
	secret := os.Getenv("NEXTAUTH_SECRET")
	if r.Header.Get("X-Internal-Secret") != secret {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}

	var body struct {
		Email string  `json:"email"`
		Name  *string `json:"name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.Email == "" {
		http.Error(w, "invalid request", http.StatusBadRequest)
		return
	}

	user, err := h.userRepo.Upsert(r.Context(), body.Email, body.Name)
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	writeJSON(w, map[string]string{"id": user.ID})
}
