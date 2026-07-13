package service

import (
	"os"
	"strings"
)

func InternalAPISecretFromEnv() string {
	return strings.TrimSpace(os.Getenv("INTERNAL_API_SECRET"))
}
