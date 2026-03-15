package handler

import (
	"net/http"
	"time"

	"github.com/enjoydarts/sifto/api/internal/middleware"
	"github.com/enjoydarts/sifto/api/internal/model"
	"github.com/enjoydarts/sifto/api/internal/repository"
	"github.com/enjoydarts/sifto/api/internal/service"
	"github.com/enjoydarts/sifto/api/internal/timeutil"
)

type ReviewsHandler struct {
	queueRepo  *repository.ReviewQueueRepo
	weeklyRepo *repository.WeeklyReviewRepo
}

func NewReviewsHandler(queueRepo *repository.ReviewQueueRepo, weeklyRepo *repository.WeeklyReviewRepo) *ReviewsHandler {
	return &ReviewsHandler{queueRepo: queueRepo, weeklyRepo: weeklyRepo}
}

func (h *ReviewsHandler) Due(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r)
	size := parseIntOrDefault(r.URL.Query().Get("size"), 5)
	items, err := h.queueRepo.ListDue(r.Context(), userID, timeutil.NowJST(), size)
	if err != nil {
		writeRepoError(w, err)
		return
	}
	writeJSON(w, model.ReviewQueueResponse{Items: service.RankReviewQueueAt(items, timeutil.NowJST())})
}

func (h *ReviewsHandler) MarkDone(w http.ResponseWriter, r *http.Request, queueID string) {
	userID := middleware.GetUserID(r)
	if err := h.queueRepo.MarkDone(r.Context(), userID, queueID); err != nil {
		writeRepoError(w, err)
		return
	}
	writeJSON(w, map[string]any{"status": "ok", "id": queueID})
}

func (h *ReviewsHandler) Snooze(w http.ResponseWriter, r *http.Request, queueID string) {
	userID := middleware.GetUserID(r)
	days := parseIntOrDefault(r.URL.Query().Get("days"), 3)
	if days < 1 {
		days = 3
	}
	if err := h.queueRepo.Snooze(r.Context(), userID, queueID, time.Duration(days)*24*time.Hour); err != nil {
		writeRepoError(w, err)
		return
	}
	writeJSON(w, map[string]any{"status": "ok", "id": queueID, "days": days})
}

func (h *ReviewsHandler) WeeklyLatest(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r)
	today := timeutil.StartOfDayJST(timeutil.NowJST())
	weekEnd := today
	weekStart := today.AddDate(0, 0, -6)
	readCount, noteCount, insightCount, favoriteCount, topics, missed, err := h.weeklyRepo.CollectInputs(
		r.Context(),
		userID,
		weekStart.Format("2006-01-02"),
		weekEnd.Format("2006-01-02"),
	)
	if err != nil {
		writeRepoError(w, err)
		return
	}
	snapshot := service.BuildWeeklyReviewSnapshot(userID, weekStart.Format("2006-01-02"), weekEnd.Format("2006-01-02"), service.WeeklyReviewInputs{
		ReadCount:       readCount,
		NoteCount:       noteCount,
		InsightCount:    insightCount,
		FavoriteCount:   favoriteCount,
		Topics:          topics,
		MissedHighValue: missed,
	})
	saved, err := h.weeklyRepo.Upsert(r.Context(), userID, snapshot)
	if err != nil {
		writeRepoError(w, err)
		return
	}
	writeJSON(w, saved)
}
