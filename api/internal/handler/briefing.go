package handler

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/minoru-kitayama/sifto/api/internal/middleware"
	"github.com/minoru-kitayama/sifto/api/internal/model"
	"github.com/minoru-kitayama/sifto/api/internal/repository"
	"github.com/minoru-kitayama/sifto/api/internal/service"
	"github.com/minoru-kitayama/sifto/api/internal/timeutil"
)

var briefingSnapshotMaxAge = loadBriefingSnapshotMaxAge()

type BriefingHandler struct {
	itemRepo     *repository.ItemRepo
	snapshotRepo *repository.BriefingSnapshotRepo
	streakRepo   *repository.ReadingStreakRepo
	cache        service.JSONCache
}

func NewBriefingHandler(
	itemRepo *repository.ItemRepo,
	snapshotRepo *repository.BriefingSnapshotRepo,
	streakRepo *repository.ReadingStreakRepo,
	cache service.JSONCache,
) *BriefingHandler {
	return &BriefingHandler{
		itemRepo:     itemRepo,
		snapshotRepo: snapshotRepo,
		streakRepo:   streakRepo,
		cache:        cache,
	}
}

func (h *BriefingHandler) Today(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r)
	size := parseIntOrDefault(r.URL.Query().Get("size"), 12)
	if size < 1 {
		size = 12
	}
	if size > 30 {
		size = 30
	}
	cacheKey := fmt.Sprintf("briefing:today:%s:size=%d", userID, size)
	cacheBust := r.URL.Query().Get("cache_bust") == "1"
	if h.cache != nil && !cacheBust {
		var cached model.BriefingTodayResponse
		if ok, err := h.cache.GetJSON(r.Context(), cacheKey, &cached); err == nil && ok {
			_ = h.cache.IncrMetric(r.Context(), "cache", "briefing.hit", 1, time.Now(), cacheMetricTTL)
			writeJSON(w, cached)
			return
		} else if err != nil {
			_ = h.cache.IncrMetric(r.Context(), "cache", "briefing.error", 1, time.Now(), cacheMetricTTL)
			log.Printf("briefing cache get failed user_id=%s key=%s err=%v", userID, cacheKey, err)
		}
		_ = h.cache.IncrMetric(r.Context(), "cache", "briefing.miss", 1, time.Now(), cacheMetricTTL)
	} else if cacheBust && h.cache != nil {
		_ = h.cache.IncrMetric(r.Context(), "cache", "briefing.bypass", 1, time.Now(), cacheMetricTTL)
	}
	now := timeutil.NowJST()
	today := timeutil.StartOfDayJST(now)
	dateStr := today.Format("2006-01-02")
	var fallbackSnapshot *model.BriefingTodayResponse

	if h.snapshotRepo != nil {
		s, err := h.snapshotRepo.GetByUserAndDate(r.Context(), userID, dateStr)
		if err == nil {
			var payload model.BriefingTodayResponse
			if len(s.PayloadJSON) > 0 && json.Unmarshal(s.PayloadJSON, &payload) == nil {
				if payload.Date == "" {
					payload.Date = dateStr
				}
				if payload.Greeting == "" {
					payload.Greeting = service.GreetingByHour(timeutil.NowJST())
				}
				payload.Status = s.Status
				payload.GeneratedAt = s.GeneratedAt
				if isSnapshotFresh(s.GeneratedAt, now) {
					writeJSON(w, payload)
					return
				}
				payload.Status = "stale"
				fallbackSnapshot = &payload
			}
		}
	}

	payload, err := service.BuildBriefingToday(r.Context(), h.itemRepo, h.streakRepo, userID, today, size)
	if err != nil {
		if fallbackSnapshot != nil {
			if h.cache != nil {
				if cacheErr := h.cache.SetJSON(r.Context(), cacheKey, fallbackSnapshot, 45*time.Second); cacheErr != nil {
					log.Printf("briefing cache set stale failed user_id=%s key=%s err=%v", userID, cacheKey, cacheErr)
				}
			}
			writeJSON(w, fallbackSnapshot)
			return
		}
		writeRepoError(w, err)
		return
	}
	payload.Status = "ready"
	generatedAt := now
	payload.GeneratedAt = &generatedAt
	if h.snapshotRepo != nil {
		if err := h.snapshotRepo.Upsert(r.Context(), userID, dateStr, "ready", payload); err != nil {
			log.Printf("briefing snapshot upsert user=%s date=%s: %v", userID, dateStr, err)
		}
	}
	if h.cache != nil {
		if err := h.cache.SetJSON(r.Context(), cacheKey, payload, 90*time.Second); err != nil {
			_ = h.cache.IncrMetric(r.Context(), "cache", "briefing.error", 1, time.Now(), cacheMetricTTL)
			log.Printf("briefing cache set failed user_id=%s key=%s err=%v", userID, cacheKey, err)
		}
	}
	writeJSON(w, payload)
}

func isSnapshotFresh(generatedAt *time.Time, now time.Time) bool {
	if generatedAt == nil {
		return false
	}
	if now.Before(*generatedAt) {
		return true
	}
	return now.Sub(*generatedAt) <= briefingSnapshotMaxAge
}

func loadBriefingSnapshotMaxAge() time.Duration {
	const defaultMaxAge = 45 * time.Minute
	v := strings.TrimSpace(os.Getenv("BRIEFING_SNAPSHOT_MAX_AGE_SEC"))
	if v == "" {
		return defaultMaxAge
	}
	sec, err := strconv.Atoi(v)
	if err != nil || sec <= 0 {
		return defaultMaxAge
	}
	return time.Duration(sec) * time.Second
}
