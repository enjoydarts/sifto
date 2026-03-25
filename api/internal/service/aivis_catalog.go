package service

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/enjoydarts/sifto/api/internal/repository"
)

type AivisCatalogService struct {
	http *http.Client
}

func NewAivisCatalogService() *AivisCatalogService {
	return &AivisCatalogService{
		http: &http.Client{Timeout: 20 * time.Second},
	}
}

type aivisSearchResponse struct {
	Total      int                `json:"total"`
	AivmModels []aivisRemoteModel `json:"aivm_models"`
}

type aivisRemoteModel struct {
	AivmModelUUID       string               `json:"aivm_model_uuid"`
	User                map[string]any       `json:"user"`
	Name                string               `json:"name"`
	Description         string               `json:"description"`
	DetailedDescription string               `json:"detailed_description"`
	Category            string               `json:"category"`
	VoiceTimbre         string               `json:"voice_timbre"`
	Visibility          string               `json:"visibility"`
	IsTagLocked         bool                 `json:"is_tag_locked"`
	TotalDownloadCount  int                  `json:"total_download_count"`
	ModelFiles          []map[string]any     `json:"model_files"`
	Tags                []map[string]any     `json:"tags"`
	LikeCount           int                  `json:"like_count"`
	IsLiked             bool                 `json:"is_liked"`
	Speakers            []aivisRemoteSpeaker `json:"speakers"`
	CreatedAt           time.Time            `json:"created_at"`
	UpdatedAt           time.Time            `json:"updated_at"`
}

type aivisRemoteSpeaker struct {
	AivmSpeakerUUID    string                    `json:"aivm_speaker_uuid"`
	Name               string                    `json:"name"`
	IconURL            string                    `json:"icon_url"`
	SupportedLanguages []string                  `json:"supported_languages"`
	LocalID            int                       `json:"local_id"`
	Styles             []aivisRemoteSpeakerStyle `json:"styles"`
}

type aivisRemoteSpeakerStyle struct {
	Name         string           `json:"name"`
	IconURL      *string          `json:"icon_url"`
	LocalID      int              `json:"local_id"`
	VoiceSamples []map[string]any `json:"voice_samples"`
}

func (s *AivisCatalogService) FetchModels(ctx context.Context) ([]repository.AivisModelSnapshot, error) {
	const pageLimit = 30
	page := 1
	total := -1
	out := make([]repository.AivisModelSnapshot, 0, pageLimit)
	fetchedAt := time.Now().UTC()
	for {
		reqURL, err := aivisModelsURL(page, pageLimit)
		if err != nil {
			return nil, err
		}
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
		if err != nil {
			return nil, err
		}
		resp, err := s.http.Do(req)
		if err != nil {
			return nil, err
		}
		if resp.StatusCode >= 400 {
			body, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
			resp.Body.Close()
			return nil, fmt.Errorf("aivis models api status %d body=%s", resp.StatusCode, strings.TrimSpace(string(body)))
		}
		var payload aivisSearchResponse
		err = json.NewDecoder(resp.Body).Decode(&payload)
		resp.Body.Close()
		if err != nil {
			return nil, err
		}
		if total < 0 {
			total = payload.Total
		}
		for _, item := range payload.AivmModels {
			out = append(out, normalizeAivisModel(item, fetchedAt))
		}
		if len(payload.AivmModels) == 0 || len(out) >= total {
			break
		}
		page++
		if page > 100 {
			break
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].TotalDownloadCount == out[j].TotalDownloadCount {
			if out[i].LikeCount == out[j].LikeCount {
				return out[i].Name < out[j].Name
			}
			return out[i].LikeCount > out[j].LikeCount
		}
		return out[i].TotalDownloadCount > out[j].TotalDownloadCount
	})
	return out, nil
}

func aivisModelsURL(page, limit int) (string, error) {
	base := strings.TrimSpace(os.Getenv("AIVIS_MODELS_API_URL"))
	if base == "" {
		base = "https://api.aivis-project.com/v1/aivm-models/search"
	}
	u, err := url.Parse(base)
	if err != nil {
		return "", err
	}
	q := u.Query()
	q.Set("sort", "download")
	q.Set("page", strconv.Itoa(page))
	q.Set("limit", strconv.Itoa(limit))
	u.RawQuery = q.Encode()
	return u.String(), nil
}

func normalizeAivisModel(item aivisRemoteModel, fetchedAt time.Time) repository.AivisModelSnapshot {
	userJSON, _ := json.Marshal(item.User)
	modelFilesJSON, _ := json.Marshal(item.ModelFiles)
	tagsJSON, _ := json.Marshal(item.Tags)
	speakersJSON, _ := json.Marshal(item.Speakers)
	styleCount := 0
	for _, speaker := range item.Speakers {
		styleCount += len(speaker.Styles)
	}
	return repository.AivisModelSnapshot{
		AivmModelUUID:       strings.TrimSpace(item.AivmModelUUID),
		Name:                strings.TrimSpace(item.Name),
		Description:         strings.TrimSpace(item.Description),
		DetailedDescription: strings.TrimSpace(item.DetailedDescription),
		Category:            strings.TrimSpace(item.Category),
		VoiceTimbre:         strings.TrimSpace(item.VoiceTimbre),
		Visibility:          strings.TrimSpace(item.Visibility),
		IsTagLocked:         item.IsTagLocked,
		TotalDownloadCount:  item.TotalDownloadCount,
		LikeCount:           item.LikeCount,
		IsLiked:             item.IsLiked,
		UserJSON:            userJSON,
		ModelFilesJSON:      modelFilesJSON,
		TagsJSON:            tagsJSON,
		SpeakersJSON:        speakersJSON,
		ModelFileCount:      len(item.ModelFiles),
		SpeakerCount:        len(item.Speakers),
		StyleCount:          styleCount,
		CreatedAt:           item.CreatedAt,
		UpdatedAt:           item.UpdatedAt,
		FetchedAt:           fetchedAt,
	}
}
