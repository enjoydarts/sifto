package service

import "os"

func AppBaseURLFromEnv() string {
	if v := os.Getenv("APP_BASE_URL"); v != "" {
		return v
	}
	return os.Getenv("NEXT_PUBLIC_APP_URL")
}
