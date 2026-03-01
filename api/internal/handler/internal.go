package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/minoru-kitayama/sifto/api/internal/model"
	"github.com/minoru-kitayama/sifto/api/internal/repository"
	"github.com/minoru-kitayama/sifto/api/internal/service"
	"github.com/minoru-kitayama/sifto/api/internal/timeutil"
)

type InternalHandler struct {
	userRepo   *repository.UserRepo
	itemRepo   *repository.ItemInngestRepo
	digestRepo *repository.DigestInngestRepo
	settings   *repository.UserSettingsRepo
	cipher     *service.SecretCipher
	publisher  *service.EventPublisher
	db         *pgxpool.Pool
	cache      service.JSONCache
	worker     *service.WorkerClient
}

func NewInternalHandler(
	userRepo *repository.UserRepo,
	itemRepo *repository.ItemInngestRepo,
	digestRepo *repository.DigestInngestRepo,
	settings *repository.UserSettingsRepo,
	cipher *service.SecretCipher,
	publisher *service.EventPublisher,
	db *pgxpool.Pool,
	cache service.JSONCache,
	worker *service.WorkerClient,
) *InternalHandler {
	return &InternalHandler{
		userRepo:   userRepo,
		itemRepo:   itemRepo,
		digestRepo: digestRepo,
		settings:   settings,
		cipher:     cipher,
		publisher:  publisher,
		db:         db,
		cache:      cache,
		worker:     worker,
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
	sendErrorSamples := make([]map[string]any, 0, 5)
	if !body.DryRun {
		for _, t := range targets {
			if err := h.publisher.SendItemEmbedE(r.Context(), t.ItemID, t.SourceID); err != nil {
				failed++
				if len(sendErrorSamples) < 5 {
					sendErrorSamples = append(sendErrorSamples, map[string]any{
						"item_id":   t.ItemID,
						"source_id": t.SourceID,
						"error":     err.Error(),
					})
				}
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
		"status":             "accepted",
		"dry_run":            body.DryRun,
		"user_filter":        body.UserID,
		"limit":              body.Limit,
		"matched":            len(targets),
		"queued_count":       queued,
		"failed_count":       failed,
		"send_error_samples": sendErrorSamples,
		"targets":            preview,
	})
}

func (h *InternalHandler) DebugBackfillTranslatedTitles(w http.ResponseWriter, r *http.Request) {
	if !checkInternalSecret(r) {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}
	if h.itemRepo == nil || h.worker == nil || h.settings == nil || h.cipher == nil {
		http.Error(w, "translated-title backfill unavailable", http.StatusInternalServerError)
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
	if body.Limit > 2000 {
		http.Error(w, "invalid limit", http.StatusBadRequest)
		return
	}

	targets, err := h.itemRepo.ListTranslatedTitleBackfillTargets(r.Context(), body.UserID, body.Limit)
	if err != nil {
		http.Error(w, fmt.Sprintf("list translated-title backfill targets: %v", err), http.StatusInternalServerError)
		return
	}

	updated := 0
	failed := 0
	empty := 0
	errorSamples := make([]map[string]any, 0, 10)
	if !body.DryRun {
		for _, t := range targets {
			cfg, err := h.settings.GetByUserID(r.Context(), t.UserID)
			if err != nil {
				failed++
				if len(errorSamples) < 10 {
					errorSamples = append(errorSamples, map[string]any{
						"item_id": t.ItemID,
						"user_id": t.UserID,
						"error":   fmt.Sprintf("load user settings: %v", err),
					})
				}
				continue
			}
			model := cfg.AnthropicSummaryModel
			isGemini := isGeminiModel(model)
			var anthropicKey *string
			var googleKey *string
			if isGemini {
				googleKey, err = h.loadGoogleAPIKey(r.Context(), t.UserID)
			} else {
				anthropicKey, err = h.loadAnthropicAPIKey(r.Context(), t.UserID)
			}
			if err != nil {
				failed++
				if len(errorSamples) < 10 {
					errorSamples = append(errorSamples, map[string]any{
						"item_id": t.ItemID,
						"user_id": t.UserID,
						"error":   fmt.Sprintf("load api key: %v", err),
					})
				}
				continue
			}
			resp, err := h.worker.TranslateTitleWithModel(r.Context(), t.Title, anthropicKey, googleKey, model)
			if err != nil {
				failed++
				if len(errorSamples) < 10 {
					errorSamples = append(errorSamples, map[string]any{
						"item_id": t.ItemID,
						"user_id": t.UserID,
						"error":   err.Error(),
					})
				}
				continue
			}
			title := strings.TrimSpace(resp.TranslatedTitle)
			if title == "" {
				empty++
				continue
			}
			if err := h.itemRepo.UpdateTranslatedTitle(r.Context(), t.ItemID, title); err != nil {
				failed++
				if len(errorSamples) < 10 {
					errorSamples = append(errorSamples, map[string]any{
						"item_id": t.ItemID,
						"user_id": t.UserID,
						"error":   fmt.Sprintf("update translated_title: %v", err),
					})
				}
				continue
			}
			updated++
		}
	}

	preview := make([]map[string]any, 0, len(targets))
	for _, t := range targets {
		preview = append(preview, map[string]any{
			"item_id":   t.ItemID,
			"source_id": t.SourceID,
			"user_id":   t.UserID,
			"title":     t.Title,
		})
	}

	w.WriteHeader(http.StatusAccepted)
	writeJSON(w, map[string]any{
		"status":        "accepted",
		"dry_run":       body.DryRun,
		"user_filter":   body.UserID,
		"limit":         body.Limit,
		"matched":       len(targets),
		"updated_count": updated,
		"empty_count":   empty,
		"failed_count":  failed,
		"error_samples": errorSamples,
		"targets":       preview,
	})
}

func (h *InternalHandler) loadAnthropicAPIKey(ctx context.Context, userID string) (*string, error) {
	enc, err := h.settings.GetAnthropicAPIKeyEncrypted(ctx, userID)
	if err != nil {
		return nil, err
	}
	if enc == nil || *enc == "" {
		return nil, fmt.Errorf("anthropic api key is not set")
	}
	if !h.cipher.Enabled() {
		return nil, fmt.Errorf("secret cipher is not configured")
	}
	plain, err := h.cipher.DecryptString(*enc)
	if err != nil {
		return nil, err
	}
	return &plain, nil
}

func (h *InternalHandler) loadGoogleAPIKey(ctx context.Context, userID string) (*string, error) {
	enc, err := h.settings.GetGoogleAPIKeyEncrypted(ctx, userID)
	if err != nil {
		return nil, err
	}
	if enc == nil || *enc == "" {
		return nil, fmt.Errorf("google api key is not set")
	}
	if !h.cipher.Enabled() {
		return nil, fmt.Errorf("secret cipher is not configured")
	}
	plain, err := h.cipher.DecryptString(*enc)
	if err != nil {
		return nil, err
	}
	return &plain, nil
}

func isGeminiModel(model *string) bool {
	if model == nil {
		return false
	}
	v := strings.ToLower(strings.TrimSpace(*model))
	if v == "" {
		return false
	}
	return strings.HasPrefix(v, "gemini-") || strings.Contains(v, "/models/gemini-")
}

func (h *InternalHandler) DebugSystemStatus(w http.ResponseWriter, r *http.Request) {
	if !checkInternalSecret(r) {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}
	type checkResult struct {
		Status    string         `json:"status"`
		LatencyMS int64          `json:"latency_ms,omitempty"`
		Detail    string         `json:"detail,omitempty"`
		HTTPCode  int            `json:"http_status,omitempty"`
		Meta      map[string]any `json:"meta,omitempty"`
	}
	now := time.Now().UTC()
	checks := map[string]checkResult{
		"api": {Status: "ok"},
	}

	run := func(name string, fn func(ctx context.Context) (string, int, map[string]any, error)) {
		start := time.Now()
		ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
		defer cancel()
		detail, code, meta, err := fn(ctx)
		lat := time.Since(start).Milliseconds()
		res := checkResult{LatencyMS: lat, HTTPCode: code, Meta: meta}
		if err != nil {
			res.Status = "error"
			res.Detail = err.Error()
		} else {
			res.Status = "ok"
			res.Detail = detail
		}
		checks[name] = res
	}

	run("db", func(ctx context.Context) (string, int, map[string]any, error) {
		if h.db == nil {
			return "", 0, nil, fmt.Errorf("db not configured")
		}
		err := h.db.Ping(ctx)
		return "ping", 0, nil, err
	})
	run("redis", func(ctx context.Context) (string, int, map[string]any, error) {
		if h.cache == nil {
			return "", 0, nil, fmt.Errorf("cache not configured")
		}
		err := h.cache.Ping(ctx)
		return "ping", 0, nil, err
	})
	run("worker", func(ctx context.Context) (string, int, map[string]any, error) {
		if h.worker == nil {
			return "", 0, nil, fmt.Errorf("worker client not configured")
		}
		err := h.worker.Health(ctx)
		return "GET /health", 200, nil, err
	})
	run("inngest", func(ctx context.Context) (string, int, map[string]any, error) {
		base := os.Getenv("INNGEST_BASE_URL")
		if base == "" {
			return "skipped", 0, map[string]any{"reason": "INNGEST_BASE_URL not set"}, nil
		}
		u := strings.TrimRight(base, "/")
		client := &http.Client{Timeout: 3 * time.Second}
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, u+"/health", nil)
		if err != nil {
			return "", 0, nil, err
		}
		resp, err := client.Do(req)
		if err != nil {
			return "", 0, nil, err
		}
		defer resp.Body.Close()
		if resp.StatusCode >= 400 {
			return "", resp.StatusCode, nil, fmt.Errorf("status %d", resp.StatusCode)
		}
		return "GET /health", resp.StatusCode, map[string]any{"base_url": base}, nil
	})

	overall := "ok"
	for k, v := range checks {
		if k == "inngest" && v.Detail == "skipped" {
			continue
		}
		if v.Status != "ok" {
			overall = "degraded"
			break
		}
	}
	cacheWindows := map[string]any{}
	if h.cache != nil {
		type winDef struct {
			Label string
			Dur   time.Duration
		}
		wins := []winDef{
			{Label: "1h", Dur: 1 * time.Hour},
			{Label: "3h", Dur: 3 * time.Hour},
			{Label: "8h", Dur: 8 * time.Hour},
			{Label: "24h", Dur: 24 * time.Hour},
			{Label: "3d", Dur: 72 * time.Hour},
			{Label: "7d", Dur: 7 * 24 * time.Hour},
		}
		for _, wd := range wins {
			sums, err := h.cache.SumMetrics(r.Context(), "cache", now.Add(-wd.Dur), now)
			if err != nil {
				cacheWindows[wd.Label] = map[string]any{"error": err.Error()}
				continue
			}
			cacheWindows[wd.Label] = map[string]any{
				"dashboard":    cacheWindowStats(sums, "dashboard"),
				"reading_plan": cacheWindowStats(sums, "reading_plan"),
				"items_list":   cacheWindowStats(sums, "items_list"),
			}
		}
	}
	writeJSON(w, map[string]any{
		"status":                overall,
		"checked_at":            now.Format(time.RFC3339Nano),
		"checks":                checks,
		"cache_stats":           cacheStatsSnapshotAll(),
		"cache_stats_by_window": cacheWindows,
	})
}

func cacheWindowStats(sums map[string]int64, prefix string) map[string]any {
	hits := sums[prefix+".hit"]
	misses := sums[prefix+".miss"]
	bypass := sums[prefix+".bypass"]
	errors := sums[prefix+".error"]
	denom := hits + misses
	var hitRate *float64
	if denom > 0 {
		v := float64(hits) / float64(denom)
		hitRate = &v
	}
	return map[string]any{
		"hits":     hits,
		"misses":   misses,
		"bypass":   bypass,
		"errors":   errors,
		"hit_rate": hitRate,
	}
}
