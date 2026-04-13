package service

import (
	"context"
	"fmt"
	"strings"

	"github.com/enjoydarts/sifto/api/internal/repository"
)

type PodcastArtworkService struct {
	repo   *repository.UserSettingsRepo
	worker *WorkerClient
}

func NewPodcastArtworkService(repo *repository.UserSettingsRepo, worker *WorkerClient) *PodcastArtworkService {
	return &PodcastArtworkService{repo: repo, worker: worker}
}

func PodcastArtworkExt(contentType string) (string, error) {
	switch strings.ToLower(strings.TrimSpace(contentType)) {
	case "image/jpeg", "image/jpg":
		return ".jpg", nil
	case "image/png":
		return ".png", nil
	case "image/webp":
		return ".webp", nil
	default:
		return "", ErrUnsupportedArtworkContentType
	}
}

func PodcastArtworkObjectKey(userID string, contentType string) (string, error) {
	ext, err := PodcastArtworkExt(contentType)
	if err != nil {
		return "", err
	}
	id := strings.TrimSpace(userID)
	if id == "" {
		return "", fmt.Errorf("user_id is required")
	}
	return "podcasts/artwork/" + id + "/current" + ext, nil
}

func (s *PodcastArtworkService) Upload(ctx context.Context, userID string, contentType string, contentBase64 string) (*string, error) {
	if s.worker == nil {
		return nil, fmt.Errorf("worker unavailable")
	}
	objectKey, err := PodcastArtworkObjectKey(userID, contentType)
	if err != nil {
		return nil, err
	}
	publicBucket := AudioBriefingPublicBucketFromEnv()
	if publicBucket == "" {
		return nil, ErrPublicBucketNotConfigured
	}
	resp, err := s.worker.UploadAudioBriefingObject(ctx, publicBucket, objectKey, contentBase64, contentType)
	if err != nil {
		return nil, err
	}
	publicURL := AudioBriefingPublicObjectURL(resp.ObjectKey)
	if publicURL == nil {
		return nil, ErrPublicBaseURLNotConfigured
	}
	settings, err := s.repo.SetPodcastArtworkURL(ctx, userID, publicURL)
	if err != nil {
		return nil, err
	}
	return settings.PodcastArtworkURL, nil
}
