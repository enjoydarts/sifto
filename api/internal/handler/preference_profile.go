package handler

import (
	"context"
	"log"
	"net/http"
	"time"

	"github.com/enjoydarts/sifto/api/internal/middleware"
	"github.com/enjoydarts/sifto/api/internal/model"
)

const (
	preferenceProfileCacheTTL        = 5 * time.Minute
	preferenceProfileSummaryCacheTTL = 10 * time.Minute
)

func (h *SettingsHandler) preferenceProfileCacheKey(ctx context.Context, userID string, summary bool) (string, error) {
	version := int64(0)
	if h.cache != nil {
		var err error
		version, err = h.cache.GetVersion(ctx, cacheVersionKeyUserPreferenceProfile(userID))
		if err != nil {
			return "", err
		}
	}
	if summary {
		return cacheKeyPreferenceProfileSummary(userID, version), nil
	}
	return cacheKeyPreferenceProfile(userID, version), nil
}

func (h *SettingsHandler) bumpPreferenceProfileVersion(ctx context.Context, userID string) error {
	if h.cache == nil || userID == "" {
		return nil
	}
	_, err := h.cache.BumpVersion(ctx, cacheVersionKeyUserPreferenceProfile(userID))
	return err
}

func (h *SettingsHandler) GetPreferenceProfile(w http.ResponseWriter, r *http.Request) {
	if h.prefProfileRepo == nil {
		http.Error(w, "preference profile is not available", http.StatusServiceUnavailable)
		return
	}
	userID := middleware.GetUserID(r)
	cacheKey, cacheKeyErr := h.preferenceProfileCacheKey(r.Context(), userID, false)
	if cacheKeyErr != nil {
		log.Printf("preference profile cache key failed user_id=%s err=%v", userID, cacheKeyErr)
	}
	if h.cache != nil && cacheKeyErr == nil && r.URL.Query().Get("cache_bust") != "1" {
		var cached model.PreferenceProfileResponse
		if ok, err := h.cache.GetJSON(r.Context(), cacheKey, &cached); err == nil && ok {
			writeJSON(w, &cached)
			return
		} else if err != nil {
			log.Printf("preference profile cache get failed user_id=%s key=%s err=%v", userID, cacheKey, err)
		}
	}

	payload, err := h.prefProfileRepo.GetProfileView(r.Context(), userID)
	if err != nil {
		log.Printf("preference profile load failed user_id=%s err=%v", userID, err)
		writeRepoError(w, err)
		return
	}
	if h.cache != nil && cacheKeyErr == nil {
		if err := h.cache.SetJSON(r.Context(), cacheKey, payload, preferenceProfileCacheTTL); err != nil {
			log.Printf("preference profile cache set failed user_id=%s key=%s err=%v", userID, cacheKey, err)
		}
	}
	writeJSON(w, payload)
}

func (h *SettingsHandler) GetPreferenceProfileSummary(w http.ResponseWriter, r *http.Request) {
	if h.prefProfileRepo == nil {
		http.Error(w, "preference profile is not available", http.StatusServiceUnavailable)
		return
	}
	userID := middleware.GetUserID(r)
	cacheKey, cacheKeyErr := h.preferenceProfileCacheKey(r.Context(), userID, true)
	if cacheKeyErr != nil {
		log.Printf("preference profile summary cache key failed user_id=%s err=%v", userID, cacheKeyErr)
	}
	if h.cache != nil && cacheKeyErr == nil && r.URL.Query().Get("cache_bust") != "1" {
		var cached model.PreferenceProfileSummaryResponse
		if ok, err := h.cache.GetJSON(r.Context(), cacheKey, &cached); err == nil && ok {
			writeJSON(w, &cached)
			return
		} else if err != nil {
			log.Printf("preference profile summary cache get failed user_id=%s key=%s err=%v", userID, cacheKey, err)
		}
	}

	payload, err := h.prefProfileRepo.GetProfileSummary(r.Context(), userID)
	if err != nil {
		log.Printf("preference profile summary load failed user_id=%s err=%v", userID, err)
		writeRepoError(w, err)
		return
	}
	if h.cache != nil && cacheKeyErr == nil {
		if err := h.cache.SetJSON(r.Context(), cacheKey, payload, preferenceProfileSummaryCacheTTL); err != nil {
			log.Printf("preference profile summary cache set failed user_id=%s key=%s err=%v", userID, cacheKey, err)
		}
	}
	writeJSON(w, payload)
}

func (h *SettingsHandler) ResetPreferenceProfile(w http.ResponseWriter, r *http.Request) {
	if h.prefProfileRepo == nil {
		http.Error(w, "preference profile is not available", http.StatusServiceUnavailable)
		return
	}
	userID := middleware.GetUserID(r)
	if err := h.prefProfileRepo.DeleteProfile(r.Context(), userID); err != nil {
		writeRepoError(w, err)
		return
	}
	if err := h.bumpPreferenceProfileVersion(r.Context(), userID); err != nil {
		log.Printf("preference profile version bump failed user_id=%s err=%v", userID, err)
	}
	writeJSON(w, map[string]any{"success": true})
}
