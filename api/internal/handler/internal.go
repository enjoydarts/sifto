package handler

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/minoru-kitayama/sifto/api/internal/model"
	"github.com/minoru-kitayama/sifto/api/internal/repository"
	"github.com/minoru-kitayama/sifto/api/internal/service"
	"github.com/minoru-kitayama/sifto/api/internal/timeutil"
)

type InternalHandler struct {
	userRepo   *repository.UserRepo
	itemRepo   *repository.ItemInngestRepo
	digestRepo *repository.DigestInngestRepo
	publisher  *service.EventPublisher
}

func NewInternalHandler(
	userRepo *repository.UserRepo,
	itemRepo *repository.ItemInngestRepo,
	digestRepo *repository.DigestInngestRepo,
	publisher *service.EventPublisher,
) *InternalHandler {
	return &InternalHandler{
		userRepo:   userRepo,
		itemRepo:   itemRepo,
		digestRepo: digestRepo,
		publisher:  publisher,
	}
}

func checkInternalSecret(r *http.Request) bool {
	secret := os.Getenv("NEXTAUTH_SECRET")
	return r.Header.Get("X-Internal-Secret") == secret
}

// UpsertUser はメールアドレスでユーザーを取得または作成して UUID を返す内部エンドポイント。
// Next.js の NextAuth jwt コールバックから呼ばれる。X-Internal-Secret で保護。
func (h *InternalHandler) UpsertUser(w http.ResponseWriter, r *http.Request) {
	if !checkInternalSecret(r) {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}

	var body struct {
		Email string  `json:"email"`
		Name  *string `json:"name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.Email == "" {
		http.Error(w, "invalid request", http.StatusBadRequest)
		return
	}

	user, err := h.userRepo.Upsert(r.Context(), body.Email, body.Name)
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	writeJSON(w, map[string]string{"id": user.ID})
}

func (h *InternalHandler) DebugGenerateDigest(w http.ResponseWriter, r *http.Request) {
	if !checkInternalSecret(r) {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}
	if h.userRepo == nil || h.itemRepo == nil || h.digestRepo == nil || h.publisher == nil {
		http.Error(w, "debug digest unavailable", http.StatusInternalServerError)
		return
	}

	var body struct {
		UserID     *string `json:"user_id"`
		DigestDate *string `json:"digest_date"` // JST date, YYYY-MM-DD
		SkipSend   bool    `json:"skip_send"`
	}
	_ = json.NewDecoder(r.Body).Decode(&body)

	targetDate := timeutil.StartOfDayJST(timeutil.NowJST())
	if body.DigestDate != nil && *body.DigestDate != "" {
		t, err := time.ParseInLocation("2006-01-02", *body.DigestDate, time.FixedZone("JST", 9*60*60))
		if err != nil {
			http.Error(w, "invalid digest_date", http.StatusBadRequest)
			return
		}
		targetDate = timeutil.StartOfDayJST(t)
	}
	since := targetDate.AddDate(0, 0, -1)
	until := targetDate

	users, err := h.userRepo.ListAll(r.Context())
	if err != nil {
		http.Error(w, fmt.Sprintf("list users: %v", err), http.StatusInternalServerError)
		return
	}
	if body.UserID != nil && *body.UserID != "" {
		filtered := make([]model.User, 0, 1)
		for _, u := range users {
			if u.ID == *body.UserID {
				filtered = append(filtered, u)
				break
			}
		}
		users = filtered
	}

	type resultItem struct {
		UserID      string `json:"user_id"`
		Email       string `json:"email"`
		DigestID    string `json:"digest_id,omitempty"`
		Status      string `json:"status"`
		ItemCount   int    `json:"item_count"`
		AlreadySent bool   `json:"already_sent,omitempty"`
		Error       string `json:"error,omitempty"`
	}
	results := make([]resultItem, 0, len(users))
	created := 0
	enqueued := 0
	skippedNoItems := 0
	skippedSent := 0
	failed := 0

	for _, u := range users {
		items, err := h.itemRepo.ListSummarizedForUser(r.Context(), u.ID, since, until)
		if err != nil {
			results = append(results, resultItem{UserID: u.ID, Email: u.Email, Status: "error", Error: err.Error()})
			failed++
			continue
		}
		if len(items) == 0 {
			results = append(results, resultItem{UserID: u.ID, Email: u.Email, Status: "skipped_no_items", ItemCount: 0})
			skippedNoItems++
			continue
		}

		digestID, alreadySent, err := h.digestRepo.Create(r.Context(), u.ID, targetDate, items)
		if err != nil {
			results = append(results, resultItem{UserID: u.ID, Email: u.Email, Status: "error", ItemCount: len(items), Error: err.Error()})
			failed++
			continue
		}
		if alreadySent {
			results = append(results, resultItem{
				UserID: u.ID, Email: u.Email, DigestID: digestID, ItemCount: len(items), Status: "skipped_sent", AlreadySent: true,
			})
			skippedSent++
			continue
		}
		created++
		status := "created"
		if !body.SkipSend {
			if err := h.publisher.SendDigestCreatedE(r.Context(), digestID, u.ID, u.Email); err != nil {
				results = append(results, resultItem{
					UserID: u.ID, Email: u.Email, DigestID: digestID, ItemCount: len(items), Status: "send_event_failed", Error: err.Error(),
				})
				failed++
				continue
			}
			enqueued++
			status = "created_enqueued"
		}
		results = append(results, resultItem{
			UserID: u.ID, Email: u.Email, DigestID: digestID, ItemCount: len(items), Status: status,
		})
	}

	w.WriteHeader(http.StatusAccepted)
	writeJSON(w, map[string]any{
		"status":           "accepted",
		"digest_date":      targetDate.Format("2006-01-02"),
		"since_jst":        since.Format(time.RFC3339),
		"until_jst":        until.Format(time.RFC3339),
		"user_filter":      body.UserID,
		"skip_send":        body.SkipSend,
		"users_checked":    len(users),
		"digests_created":  created,
		"events_enqueued":  enqueued,
		"skipped_no_items": skippedNoItems,
		"skipped_sent":     skippedSent,
		"errors":           failed,
		"results":          results,
	})
}

func (h *InternalHandler) DebugSendDigest(w http.ResponseWriter, r *http.Request) {
	if !checkInternalSecret(r) {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}
	if h.digestRepo == nil || h.publisher == nil {
		http.Error(w, "debug digest unavailable", http.StatusInternalServerError)
		return
	}

	var body struct {
		DigestID string `json:"digest_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.DigestID == "" {
		http.Error(w, "invalid request", http.StatusBadRequest)
		return
	}

	digest, err := h.digestRepo.GetForEmail(r.Context(), body.DigestID)
	if err != nil {
		http.Error(w, fmt.Sprintf("fetch digest: %v", err), http.StatusNotFound)
		return
	}
	userEmail := ""
	if len(digest.Items) > 0 {
		// no-op: email is sourced from users table in generate flow; for debug send use user's email via users table list
	}
	users, err := h.userRepo.ListAll(r.Context())
	if err != nil {
		http.Error(w, fmt.Sprintf("list users: %v", err), http.StatusInternalServerError)
		return
	}
	for _, u := range users {
		if u.ID == digest.UserID {
			userEmail = u.Email
			break
		}
	}
	if userEmail == "" {
		http.Error(w, "digest user email not found", http.StatusNotFound)
		return
	}

	if err := h.publisher.SendDigestCreatedE(r.Context(), digest.ID, digest.UserID, userEmail); err != nil {
		http.Error(w, "failed to enqueue digest send", http.StatusBadGateway)
		return
	}

	w.WriteHeader(http.StatusAccepted)
	writeJSON(w, map[string]any{
		"status":    "queued",
		"digest_id": digest.ID,
		"user_id":   digest.UserID,
		"to":        userEmail,
	})
}

func (h *InternalHandler) DebugBackfillEmbeddings(w http.ResponseWriter, r *http.Request) {
	if !checkInternalSecret(r) {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}
	if h.itemRepo == nil || h.publisher == nil {
		http.Error(w, "embedding backfill unavailable", http.StatusInternalServerError)
		return
	}

	var body struct {
		UserID *string `json:"user_id"`
		Limit  int     `json:"limit"`
		DryRun bool    `json:"dry_run"`
	}
	_ = json.NewDecoder(r.Body).Decode(&body)
	if body.Limit <= 0 {
		body.Limit = 100
	}
	if body.Limit > 1000 {
		http.Error(w, "invalid limit", http.StatusBadRequest)
		return
	}

	targets, err := h.itemRepo.ListEmbeddingBackfillTargets(r.Context(), body.UserID, body.Limit)
	if err != nil {
		http.Error(w, fmt.Sprintf("list embedding backfill targets: %v", err), http.StatusInternalServerError)
		return
	}

	queued := 0
	failed := 0
	if !body.DryRun {
		for _, t := range targets {
			if err := h.publisher.SendItemEmbedE(r.Context(), t.ItemID, t.SourceID); err != nil {
				failed++
				continue
			}
			queued++
		}
	}

	preview := make([]map[string]any, 0, len(targets))
	for _, t := range targets {
		preview = append(preview, map[string]any{
			"item_id":   t.ItemID,
			"source_id": t.SourceID,
			"user_id":   t.UserID,
			"url":       t.URL,
		})
	}

	w.WriteHeader(http.StatusAccepted)
	writeJSON(w, map[string]any{
		"status":       "accepted",
		"dry_run":      body.DryRun,
		"user_filter":  body.UserID,
		"limit":        body.Limit,
		"matched":      len(targets),
		"queued_count": queued,
		"failed_count": failed,
		"targets":      preview,
	})
}
