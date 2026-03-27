package service

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
)

type PodcastCategoryDefinition struct {
	Category      string   `json:"category"`
	Subcategories []string `json:"subcategories"`
}

var (
	podcastCategoriesOnce sync.Once
	podcastCategoriesData []PodcastCategoryDefinition
)

func PodcastCategoryDefinitions() []PodcastCategoryDefinition {
	podcastCategoriesOnce.Do(func() {
		body, err := os.ReadFile(podcastCategoriesPath())
		if err != nil {
			podcastCategoriesData = []PodcastCategoryDefinition{}
			return
		}
		var defs []PodcastCategoryDefinition
		if err := json.Unmarshal(body, &defs); err != nil {
			podcastCategoriesData = []PodcastCategoryDefinition{}
			return
		}
		podcastCategoriesData = defs
	})
	return append([]PodcastCategoryDefinition{}, podcastCategoriesData...)
}

func podcastCategoriesPath() string {
	candidates := []string{
		"/app/shared/podcast_categories.json",
		"/shared/podcast_categories.json",
		filepath.Join("shared", "podcast_categories.json"),
		filepath.Join("..", "shared", "podcast_categories.json"),
		filepath.Join("..", "..", "shared", "podcast_categories.json"),
		filepath.Join("..", "..", "..", "shared", "podcast_categories.json"),
	}
	for _, path := range candidates {
		if _, err := os.Stat(path); err == nil {
			return path
		}
	}
	return candidates[0]
}

func normalizePodcastCategorySelection(category, subcategory *string) (*string, *string, error) {
	normalizedCategory := normalizeOptionalString(category)
	normalizedSubcategory := normalizeOptionalString(subcategory)
	if normalizedCategory == nil {
		if normalizedSubcategory != nil {
			return nil, nil, errInvalidPodcastCategory
		}
		return nil, nil, nil
	}
	for _, def := range PodcastCategoryDefinitions() {
		if def.Category != *normalizedCategory {
			continue
		}
		if normalizedSubcategory == nil {
			return normalizedCategory, nil, nil
		}
		for _, child := range def.Subcategories {
			if child == *normalizedSubcategory {
				return normalizedCategory, normalizedSubcategory, nil
			}
		}
		return nil, nil, errInvalidPodcastCategory
	}
	return nil, nil, errInvalidPodcastCategory
}
