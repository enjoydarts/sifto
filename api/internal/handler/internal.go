package handler

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/enjoydarts/sifto/api/internal/model"
	"github.com/enjoydarts/sifto/api/internal/repository"
	"github.com/enjoydarts/sifto/api/internal/service"
	"github.com/enjoydarts/sifto/api/internal/timeutil"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type InternalHandler struct {
	userRepo     *repository.UserRepo
	identityRepo *repository.UserIdentityRepo
	obsidianRepo *repository.ObsidianExportRepo
	itemRepo     *repository.ItemInngestRepo
	digestRepo   *repository.DigestInngestRepo
	settings     *repository.UserSettingsRepo
	cipher       *service.SecretCipher
	publisher    *service.EventPublisher
	db           *pgxpool.Pool
	cache        service.JSONCache
	worker       *service.WorkerClient
	oneSignal    *service.OneSignalClient
	githubApp    *service.GitHubAppClient
	search       *service.MeilisearchService
}

func NewInternalHandler(
	userRepo *repository.UserRepo,
	identityRepo *repository.UserIdentityRepo,
	obsidianRepo *repository.ObsidianExportRepo,
	itemRepo *repository.ItemInngestRepo,
	digestRepo *repository.DigestInngestRepo,
	settings *repository.UserSettingsRepo,
	cipher *service.SecretCipher,
	publisher *service.EventPublisher,
	db *pgxpool.Pool,
	cache service.JSONCache,
	worker *service.WorkerClient,
	oneSignal *service.OneSignalClient,
	githubApp *service.GitHubAppClient,
	search *service.MeilisearchService,
) *InternalHandler {
	return &InternalHandler{
		userRepo:     userRepo,
		identityRepo: identityRepo,
		obsidianRepo: obsidianRepo,
		itemRepo:     itemRepo,
		digestRepo:   digestRepo,
		settings:     settings,
		cipher:       cipher,
		publisher:    publisher,
		db:           db,
		cache:        cache,
		worker:       worker,
		oneSignal:    oneSignal,
		githubApp:    githubApp,
		search:       search,
	}
}

func checkInternalSecret(r *http.Request) bool {
	secret := service.InternalAPISecretFromEnv()
	return r.Header.Get("X-Internal-Secret") == secret
}

// UpsertUser はメールアドレスでユーザーを取得または作成して UUID を返す内部エンドポイント。
// Next.js の auth bridge / debug route から呼ばれる。X-Internal-Secret で保護。
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
		log.Printf("internal users upsert failed: email=%s err=%v", body.Email, err)
		http.Error(w, fmt.Sprintf("upsert user failed: %v", err), http.StatusInternalServerError)
		return
	}

	writeJSON(w, map[string]string{"id": user.ID})
}

// ResolveIdentity は external auth provider の subject を internal user_id へ解決する。
// identity が未登録なら email ベースで既存/新規 user を解決し、provider identity を保存する。
func (h *InternalHandler) ResolveIdentity(w http.ResponseWriter, r *http.Request) {
	if !checkInternalSecret(r) {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}

	var body struct {
		Provider       string  `json:"provider"`
		ProviderUserID string  `json:"provider_user_id"`
		Email          string  `json:"email"`
		Name           *string `json:"name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "invalid request", http.StatusBadRequest)
		return
	}
	body.Provider = strings.TrimSpace(body.Provider)
	body.ProviderUserID = strings.TrimSpace(body.ProviderUserID)
	body.Email = strings.TrimSpace(strings.ToLower(body.Email))
	if body.Provider == "" || body.ProviderUserID == "" || body.Email == "" {
		http.Error(w, "invalid request", http.StatusBadRequest)
		return
	}

	identity, err := h.identityRepo.GetByProviderUserID(r.Context(), body.Provider, body.ProviderUserID)
	if err == nil {
		writeJSON(w, map[string]any{
			"id":               identity.UserID,
			"identity_id":      identity.ID,
			"identity_created": false,
			"user_created":     false,
			"resolved_by":      "identity",
			"provider":         identity.Provider,
			"provider_user_id": identity.ProviderUserID,
		})
		return
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		log.Printf("internal resolve identity lookup failed: provider=%s provider_user_id=%s err=%v", body.Provider, body.ProviderUserID, err)
		http.Error(w, fmt.Sprintf("lookup identity failed: %v", err), http.StatusInternalServerError)
		return
	}

	user, getErr := h.userRepo.GetByEmail(r.Context(), body.Email)
	userCreated := false
	if getErr != nil && !errors.Is(getErr, pgx.ErrNoRows) {
		log.Printf("internal resolve identity user lookup failed: email=%s err=%v", body.Email, getErr)
		http.Error(w, fmt.Sprintf("resolve identity failed: %v", getErr), http.StatusInternalServerError)
		return
	}
	if errors.Is(getErr, pgx.ErrNoRows) {
		user, err = h.userRepo.Upsert(r.Context(), body.Email, body.Name)
		if err != nil {
			log.Printf("internal resolve identity user upsert failed: provider=%s provider_user_id=%s email=%s err=%v", body.Provider, body.ProviderUserID, body.Email, err)
			http.Error(w, fmt.Sprintf("resolve identity failed: %v", err), http.StatusInternalServerError)
			return
		}
		userCreated = true
	}

	identity, err = h.identityRepo.Upsert(r.Context(), user.ID, body.Provider, body.ProviderUserID, &body.Email)
	if err != nil {
		log.Printf("internal resolve identity upsert failed: provider=%s provider_user_id=%s user_id=%s err=%v", body.Provider, body.ProviderUserID, user.ID, err)
		http.Error(w, fmt.Sprintf("upsert identity failed: %v", err), http.StatusInternalServerError)
		return
	}

	writeJSON(w, map[string]any{
		"id":               user.ID,
		"identity_id":      identity.ID,
		"identity_created": true,
		"user_created":     userCreated,
		"resolved_by":      "email",
		"provider":         identity.Provider,
		"provider_user_id": identity.ProviderUserID,
	})
}

func (h *InternalHandler) UpsertObsidianGitHubInstallation(w http.ResponseWriter, r *http.Request) {
	if !checkInternalSecret(r) {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}
	if h.obsidianRepo == nil || h.githubApp == nil || !h.githubApp.Enabled() {
		http.Error(w, "github app unavailable", http.StatusInternalServerError)
		return
	}

	var body struct {
		UserID         string `json:"user_id"`
		InstallationID int64  `json:"installation_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "invalid request", http.StatusBadRequest)
		return
	}
	body.UserID = strings.TrimSpace(body.UserID)
	if body.UserID == "" || body.InstallationID <= 0 {
		http.Error(w, "invalid request", http.StatusBadRequest)
		return
	}

	installation, err := h.githubApp.GetInstallation(r.Context(), body.InstallationID)
	if err != nil {
		http.Error(w, fmt.Sprintf("get installation failed: %v", err), http.StatusBadGateway)
		return
	}
	var owner *string
	if installation != nil && installation.Account != nil {
		v := strings.TrimSpace(installation.Account.Login)
		if v != "" {
			owner = &v
		}
	}
	settings, err := h.obsidianRepo.UpsertInstallation(r.Context(), body.UserID, body.InstallationID, owner)
	if err != nil {
		http.Error(w, fmt.Sprintf("save installation failed: %v", err), http.StatusInternalServerError)
		return
	}
	writeJSON(w, map[string]any{
		"user_id": settings.UserID,
		"obsidian_export": map[string]any{
			"enabled":                settings.Enabled,
			"github_installation_id": settings.GitHubInstallationID,
			"github_repo_owner":      settings.GitHubRepoOwner,
			"github_repo_name":       settings.GitHubRepoName,
			"github_repo_branch":     settings.GitHubRepoBranch,
			"vault_root_path":        settings.VaultRootPath,
			"keyword_link_mode":      settings.KeywordLinkMode,
			"last_run_at":            settings.LastRunAt,
			"last_success_at":        settings.LastSuccessAt,
		},
	})
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

func (h *InternalHandler) DebugSendPushTest(w http.ResponseWriter, r *http.Request) {
	if !checkInternalSecret(r) {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}
	if h.oneSignal == nil || !h.oneSignal.Enabled() {
		http.Error(w, "onesignal is not configured", http.StatusBadRequest)
		return
	}
	var body struct {
		ExternalID     *string        `json:"external_id"`
		SubscriptionID *string        `json:"subscription_id"`
		Title          string         `json:"title"`
		Message        string         `json:"message"`
		URL            string         `json:"url"`
		Data           map[string]any `json:"data"`
	}
	_ = json.NewDecoder(r.Body).Decode(&body)
	title := strings.TrimSpace(body.Title)
	if title == "" {
		title = "Sifto: テスト通知"
	}
	message := strings.TrimSpace(body.Message)
	if message == "" {
		message = "OneSignalテスト通知です。"
	}
	externalID := ""
	if body.ExternalID != nil {
		externalID = strings.TrimSpace(*body.ExternalID)
	}
	subscriptionID := ""
	if body.SubscriptionID != nil {
		subscriptionID = strings.TrimSpace(*body.SubscriptionID)
	}
	if externalID == "" && subscriptionID == "" {
		http.Error(w, "external_id or subscription_id is required", http.StatusBadRequest)
		return
	}
	var (
		res    *service.OneSignalSendResult
		err    error
		target string
	)
	if subscriptionID != "" {
		target = subscriptionID
		res, err = h.oneSignal.SendToSubscriptionID(r.Context(), subscriptionID, title, message, body.URL, body.Data)
	} else {
		target = externalID
		res, err = h.oneSignal.SendToExternalID(r.Context(), externalID, title, message, body.URL, body.Data)
	}
	if err != nil {
		http.Error(w, fmt.Sprintf("send push: %v", err), http.StatusBadGateway)
		return
	}
	writeJSON(w, map[string]any{
		"status":          "sent",
		"target":          target,
		"external_id":     externalID,
		"subscription_id": subscriptionID,
		"title":           title,
		"message":         message,
		"result":          res,
	})
}

func (h *InternalHandler) DebugBackfillItemSearch(w http.ResponseWriter, r *http.Request) {
	if !checkInternalSecret(r) {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}
	if h.publisher == nil {
		http.Error(w, "event publisher unavailable", http.StatusInternalServerError)
		return
	}

	docRepo := repository.NewItemSearchDocumentRepo(h.db)
	runRepo := repository.NewSearchBackfillRunRepo(h.db)
	offset := parseIntOrDefault(strings.TrimSpace(r.URL.Query().Get("offset")), 0)
	limit := parseIntOrDefault(strings.TrimSpace(r.URL.Query().Get("limit")), 500)
	allItems := parseBoolQuery(r.URL.Query().Get("all"))
	if limit < 1 || limit > 5000 {
		http.Error(w, "invalid limit", http.StatusBadRequest)
		return
	}
	if offset < 0 {
		http.Error(w, "invalid offset", http.StatusBadRequest)
		return
	}

	totalSummarized, err := docRepo.CountSummarized(r.Context())
	if err != nil {
		http.Error(w, fmt.Sprintf("count search targets failed: %v", err), http.StatusInternalServerError)
		return
	}
	remaining := totalSummarized - offset
	if remaining < 0 {
		remaining = 0
	}
	totalItems := limit
	queuedBatches := 1
	if allItems {
		totalItems = remaining
		if totalItems == 0 {
			queuedBatches = 0
		} else {
			queuedBatches = (totalItems + limit - 1) / limit
		}
	} else {
		if remaining <= 0 {
			totalItems = 0
			queuedBatches = 0
		} else if remaining < limit {
			totalItems = remaining
		}
	}

	run, err := runRepo.Create(r.Context(), offset, limit, allItems, totalItems, queuedBatches)
	if err != nil {
		http.Error(w, fmt.Sprintf("create backfill run failed: %v", err), http.StatusInternalServerError)
		return
	}

	if queuedBatches > 0 {
		if err := h.publisher.SendItemSearchBackfillRunE(r.Context(), run.ID); err != nil {
			if _, markErr := runRepo.MarkFanoutFailed(r.Context(), run.ID, err.Error()); markErr != nil {
				http.Error(w, fmt.Sprintf("mark backfill run failed: %v", markErr), http.StatusInternalServerError)
				return
			}
			http.Error(w, fmt.Sprintf("enqueue backfill failed: %v", err), http.StatusBadGateway)
			return
		}
	}

	writeJSON(w, map[string]any{
		"ok":             true,
		"run_id":         run.ID,
		"offset":         offset,
		"limit":          limit,
		"all":            allItems,
		"indexes":        []string{"items", "search_suggestions"},
		"total_items":    totalItems,
		"queued_batches": queuedBatches,
	})
}

func (h *InternalHandler) DebugGetItemSearchBackfillRuns(w http.ResponseWriter, r *http.Request) {
	if !checkInternalSecret(r) {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}

	runRepo := repository.NewSearchBackfillRunRepo(h.db)
	limit := parseIntOrDefault(strings.TrimSpace(r.URL.Query().Get("limit")), 10)
	if limit < 1 || limit > 100 {
		http.Error(w, "invalid limit", http.StatusBadRequest)
		return
	}

	runs, err := runRepo.ListRecent(r.Context(), limit)
	if err != nil {
		http.Error(w, fmt.Sprintf("list backfill runs failed: %v", err), http.StatusInternalServerError)
		return
	}

	writeJSON(w, map[string]any{
		"runs": runs,
	})
}

func (h *InternalHandler) DebugDeleteFinishedItemSearchBackfillRuns(w http.ResponseWriter, r *http.Request) {
	if !checkInternalSecret(r) {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}

	runRepo := repository.NewSearchBackfillRunRepo(h.db)
	deleted, err := runRepo.DeleteFinished(r.Context())
	if err != nil {
		http.Error(w, fmt.Sprintf("delete finished backfill runs failed: %v", err), http.StatusInternalServerError)
		return
	}

	writeJSON(w, map[string]any{
		"ok":            true,
		"deleted_count": deleted,
	})
}

func parseBoolQuery(value string) bool {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "1", "true", "yes", "on":
		return true
	default:
		return false
	}
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
			model := cfg.SummaryModel
			var anthropicKey *string
			var googleKey *string
			var groqKey *string
			var deepseekKey *string
			var alibabaKey *string
			var mistralKey *string
			var xaiKey *string
			var zaiKey *string
			var fireworksKey *string
			var openAIKey *string
			switch service.LLMProviderForModel(model) {
			case "google":
				googleKey, err = h.loadGoogleAPIKey(r.Context(), t.UserID)
			case "groq":
				groqKey, err = h.loadGroqAPIKey(r.Context(), t.UserID)
			case "deepseek":
				deepseekKey, err = h.loadDeepSeekAPIKey(r.Context(), t.UserID)
			case "alibaba":
				alibabaKey, err = h.loadAlibabaAPIKey(r.Context(), t.UserID)
			case "mistral":
				mistralKey, err = h.loadMistralAPIKey(r.Context(), t.UserID)
			case "moonshot":
				openAIKey, err = h.loadMoonshotAPIKey(r.Context(), t.UserID)
			case "xai":
				xaiKey, err = h.loadXAIAPIKey(r.Context(), t.UserID)
			case "zai":
				zaiKey, err = h.loadZAIAPIKey(r.Context(), t.UserID)
			case "fireworks":
				fireworksKey, err = h.loadFireworksAPIKey(r.Context(), t.UserID)
			case "openai":
				openAIKey, err = h.loadOpenAIAPIKey(r.Context(), t.UserID)
			case "poe":
				openAIKey, err = h.loadPoeAPIKey(r.Context(), t.UserID)
			case "siliconflow":
				openAIKey, err = h.loadSiliconFlowAPIKey(r.Context(), t.UserID)
			default:
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
			resp, err := h.worker.TranslateTitleWithModel(r.Context(), t.Title, anthropicKey, googleKey, groqKey, deepseekKey, alibabaKey, mistralKey, xaiKey, zaiKey, fireworksKey, openAIKey, model)
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

func (h *InternalHandler) loadGroqAPIKey(ctx context.Context, userID string) (*string, error) {
	enc, err := h.settings.GetGroqAPIKeyEncrypted(ctx, userID)
	if err != nil {
		return nil, err
	}
	if enc == nil || *enc == "" {
		return nil, fmt.Errorf("groq api key is not set")
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

func (h *InternalHandler) loadDeepSeekAPIKey(ctx context.Context, userID string) (*string, error) {
	enc, err := h.settings.GetDeepSeekAPIKeyEncrypted(ctx, userID)
	if err != nil {
		return nil, err
	}
	if enc == nil || *enc == "" {
		return nil, fmt.Errorf("deepseek api key is not set")
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

func (h *InternalHandler) loadAlibabaAPIKey(ctx context.Context, userID string) (*string, error) {
	enc, err := h.settings.GetAlibabaAPIKeyEncrypted(ctx, userID)
	if err != nil {
		return nil, err
	}
	if enc == nil || *enc == "" {
		return nil, fmt.Errorf("alibaba api key is not set")
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

func (h *InternalHandler) loadMistralAPIKey(ctx context.Context, userID string) (*string, error) {
	enc, err := h.settings.GetMistralAPIKeyEncrypted(ctx, userID)
	if err != nil {
		return nil, err
	}
	if enc == nil || *enc == "" {
		return nil, fmt.Errorf("mistral api key is not set")
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

func (h *InternalHandler) loadXAIAPIKey(ctx context.Context, userID string) (*string, error) {
	enc, err := h.settings.GetXAIAPIKeyEncrypted(ctx, userID)
	if err != nil {
		return nil, err
	}
	if enc == nil || *enc == "" {
		return nil, fmt.Errorf("xai api key is not set")
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

func (h *InternalHandler) loadMoonshotAPIKey(ctx context.Context, userID string) (*string, error) {
	enc, err := h.settings.GetMoonshotAPIKeyEncrypted(ctx, userID)
	if err != nil {
		return nil, err
	}
	if enc == nil || *enc == "" {
		return nil, fmt.Errorf("moonshot api key is not set")
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

func (h *InternalHandler) loadZAIAPIKey(ctx context.Context, userID string) (*string, error) {
	enc, err := h.settings.GetZAIAPIKeyEncrypted(ctx, userID)
	if err != nil {
		return nil, err
	}
	if enc == nil || *enc == "" {
		return nil, fmt.Errorf("zai api key is not set")
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

func (h *InternalHandler) loadFireworksAPIKey(ctx context.Context, userID string) (*string, error) {
	enc, err := h.settings.GetFireworksAPIKeyEncrypted(ctx, userID)
	if err != nil {
		return nil, err
	}
	if enc == nil || *enc == "" {
		return nil, fmt.Errorf("fireworks api key is not set")
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

func (h *InternalHandler) loadOpenAIAPIKey(ctx context.Context, userID string) (*string, error) {
	enc, err := h.settings.GetOpenAIAPIKeyEncrypted(ctx, userID)
	if err != nil {
		return nil, err
	}
	if enc == nil || *enc == "" {
		return nil, fmt.Errorf("openai api key is not set")
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

func (h *InternalHandler) loadPoeAPIKey(ctx context.Context, userID string) (*string, error) {
	enc, err := h.settings.GetPoeAPIKeyEncrypted(ctx, userID)
	if err != nil {
		return nil, err
	}
	if enc == nil || *enc == "" {
		return nil, fmt.Errorf("poe api key is not set")
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

func (h *InternalHandler) loadSiliconFlowAPIKey(ctx context.Context, userID string) (*string, error) {
	enc, err := h.settings.GetSiliconFlowAPIKeyEncrypted(ctx, userID)
	if err != nil {
		return nil, err
	}
	if enc == nil || *enc == "" {
		return nil, fmt.Errorf("siliconflow api key is not set")
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
	run("meilisearch", func(ctx context.Context) (string, int, map[string]any, error) {
		if h.search == nil {
			return "", 0, nil, fmt.Errorf("meilisearch client not configured")
		}
		err := h.search.Health(ctx)
		return "GET /health", 200, map[string]any{"items_index": h.search.ItemsIndexName()}, err
	})
	run("inngest", func(ctx context.Context) (string, int, map[string]any, error) {
		base := service.InngestBaseURLFromEnv()
		if base == "" {
			return "skipped", 0, map[string]any{"reason": "INNGEST_BASE_URL not set"}, nil
		}
		u := strings.TrimRight(base, "/")
		client := service.NewInngestHTTPClient(3 * time.Second)
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
	cacheWindowsUser := map[string]any{}
	metricUserID := strings.TrimSpace(r.URL.Query().Get("user_id"))
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
				"focus_queue":  cacheWindowStats(sums, "focus_queue"),
				"triage_all":   cacheWindowStats(sums, "triage_all"),
				"related":      cacheWindowStats(sums, "related"),
			}
			if metricUserID != "" {
				userNamespace := fmt.Sprintf("cache_user:%s", metricUserID)
				userSums, userErr := h.cache.SumMetrics(r.Context(), userNamespace, now.Add(-wd.Dur), now)
				if userErr != nil {
					cacheWindowsUser[wd.Label] = map[string]any{"error": userErr.Error()}
				} else {
					cacheWindowsUser[wd.Label] = map[string]any{
						"dashboard":    cacheWindowStats(userSums, "dashboard"),
						"reading_plan": cacheWindowStats(userSums, "reading_plan"),
						"items_list":   cacheWindowStats(userSums, "items_list"),
						"focus_queue":  cacheWindowStats(userSums, "focus_queue"),
						"triage_all":   cacheWindowStats(userSums, "triage_all"),
						"related":      cacheWindowStats(userSums, "related"),
						"briefing":     cacheWindowStats(userSums, "briefing"),
					}
				}
			}
		}
	}
	writeJSON(w, map[string]any{
		"status":                     overall,
		"checked_at":                 now.Format(time.RFC3339Nano),
		"checks":                     checks,
		"cache_stats":                cacheStatsSnapshotAll(),
		"cache_stats_by_window":      cacheWindows,
		"cache_metrics_user_id":      metricUserID,
		"cache_stats_by_window_user": cacheWindowsUser,
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
