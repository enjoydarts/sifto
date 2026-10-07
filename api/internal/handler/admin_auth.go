package handler

import (
	"context"
	"net/http"
	"strings"

	"github.com/enjoydarts/sifto/api/internal/middleware"
	"github.com/enjoydarts/sifto/api/internal/model"
	"github.com/enjoydarts/sifto/api/internal/service"
)

type adminUserStore interface {
	GetByID(context.Context, string) (*model.User, error)
}

// Use the authenticated identity and a server-side user lookup. Request headers
// and query parameters cannot assert administrator privileges.
func canRunAdminOperation(r *http.Request, users adminUserStore, auth *service.PromptAdminAuthService) bool {
	userID := strings.TrimSpace(middleware.GetUserID(r))
	if userID == "" || users == nil || auth == nil {
		return false
	}
	user, err := users.GetByID(r.Context(), userID)
	return err == nil && user != nil && auth.CanManagePrompts(user.Email)
}
