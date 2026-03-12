package service

import "os"

func InternalAPISecretFromEnv() string {
	return os.Getenv("INTERNAL_API_SECRET")
}
