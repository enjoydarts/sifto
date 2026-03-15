package handler

import (
	"net/http"

	"github.com/enjoydarts/sifto/api/internal/middleware"
	"github.com/enjoydarts/sifto/api/internal/model"
	"github.com/enjoydarts/sifto/api/internal/repository"
	"github.com/enjoydarts/sifto/api/internal/service"
	"github.com/enjoydarts/sifto/api/internal/timeutil"
)

func (h *ItemHandler) TodayQueue(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r)
	size := parseIntOrDefault(r.URL.Query().Get("size"), 6)
	if size < 1 || size > 12 {
		http.Error(w, "invalid size", http.StatusBadRequest)
		return
	}

	plan, err := h.repo.ReadingPlan(r.Context(), userID, repository.ReadingPlanParams{
		Window:          "24h",
		Size:            size * 3,
		DiversifyTopics: true,
		ExcludeRead:     true,
		ExcludeLater:    false,
	})
	if err != nil {
		writeRepoError(w, err)
		return
	}

	goals := []model.ReadingGoal{}
	if h.readingGoalRepo != nil {
		goals, err = h.readingGoalRepo.ListByUser(r.Context(), userID)
		if err != nil {
			writeRepoError(w, err)
			return
		}
	}
	candidates := make([]model.TodayQueueCandidate, 0, len(plan.Items))
	for _, item := range plan.Items {
		candidates = append(candidates, model.TodayQueueCandidate{Item: item})
	}
	writeJSON(w, model.TodayQueueResponse{
		Items: service.RankTodayQueueItems(candidates, goals, size, timeutil.NowJST()),
	})
}
