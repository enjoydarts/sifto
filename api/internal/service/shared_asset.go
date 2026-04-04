package service

import (
	"os"
	"path/filepath"
)

func readSharedAsset(filename string) ([]byte, error) {
	return os.ReadFile(sharedAssetPath(filename))
}

func sharedAssetPath(filename string) string {
	candidates := []string{
		filepath.Join("/app/shared", filename),
		filepath.Join("/shared", filename),
		filepath.Join("shared", filename),
		filepath.Join("..", "shared", filename),
		filepath.Join("..", "..", "shared", filename),
		filepath.Join("..", "..", "..", "shared", filename),
	}
	for _, path := range candidates {
		if _, err := os.Stat(path); err == nil {
			return path
		}
	}
	return candidates[0]
}
