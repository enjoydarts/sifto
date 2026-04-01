package service

import (
	"os"
	"strings"
)

type PromptAdminAuthService struct {
	allowed map[string]struct{}
}

func NewPromptAdminAuthServiceFromEnv() *PromptAdminAuthService {
	return NewPromptAdminAuthService(strings.Split(strings.TrimSpace(strings.ToLower(strings.TrimSpace(getenv("PROMPT_ADMIN_EMAILS", "")))), ","))
}

func NewPromptAdminAuthService(emails []string) *PromptAdminAuthService {
	allowed := make(map[string]struct{}, len(emails))
	for _, email := range emails {
		normalized := normalizePromptAdminEmail(email)
		if normalized == "" {
			continue
		}
		allowed[normalized] = struct{}{}
	}
	return &PromptAdminAuthService{allowed: allowed}
}

func (s *PromptAdminAuthService) CanManagePrompts(email string) bool {
	if s == nil {
		return false
	}
	_, ok := s.allowed[normalizePromptAdminEmail(email)]
	return ok
}

func normalizePromptAdminEmail(email string) string {
	return strings.ToLower(strings.TrimSpace(email))
}

func getenv(key, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(key)); value != "" {
		return value
	}
	return strings.TrimSpace(fallback)
}
