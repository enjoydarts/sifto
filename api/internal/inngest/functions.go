package inngest

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"log"
	"math"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/inngest/inngestgo"
	"github.com/inngest/inngestgo/step"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/minoru-kitayama/sifto/api/internal/model"
	"github.com/minoru-kitayama/sifto/api/internal/repository"
	"github.com/minoru-kitayama/sifto/api/internal/service"
	"github.com/minoru-kitayama/sifto/api/internal/timeutil"
	"github.com/mmcdole/gofeed"
)

func recordLLMUsage(ctx context.Context, repo *repository.LLMUsageLogRepo, purpose string, usage *service.LLMUsage, userID, sourceID, itemID, digestID *string) {
	if repo == nil || usage == nil {
		return
	}
	if usage.Provider == "" || usage.Model == "" {
		return
	}
	idempotencyKey := llmUsageIdempotencyKey(purpose, usage, userID, sourceID, itemID, digestID)
	pricingSource := usage.PricingSource
	if pricingSource == "" {
		pricingSource = "unknown"
	}
	if err := repo.Insert(ctx, repository.LLMUsageLogInput{
		IdempotencyKey:           &idempotencyKey,
		UserID:                   userID,
		SourceID:                 sourceID,
		ItemID:                   itemID,
		DigestID:                 digestID,
		Provider:                 usage.Provider,
		Model:                    usage.Model,
		PricingModelFamily:       usage.PricingModelFamily,
		PricingSource:            pricingSource,
		Purpose:                  purpose,
		InputTokens:              usage.InputTokens,
		OutputTokens:             usage.OutputTokens,
		CacheCreationInputTokens: usage.CacheCreationInputTokens,
		CacheReadInputTokens:     usage.CacheReadInputTokens,
		EstimatedCostUSD:         usage.EstimatedCostUSD,
	}); err != nil {
		log.Printf("record llm usage purpose=%s: %v", purpose, err)
	}
}

func llmUsageIdempotencyKey(purpose string, usage *service.LLMUsage, userID, sourceID, itemID, digestID *string) string {
	toVal := func(v *string) string {
		if v == nil {
			return ""
		}
		return *v
	}
	raw := fmt.Sprintf(
		"purpose=%s|provider=%s|model=%s|u=%s|s=%s|i=%s|d=%s|in=%d|out=%d|cw=%d|cr=%d",
		purpose,
		usage.Provider,
		usage.Model,
		toVal(userID),
		toVal(sourceID),
		toVal(itemID),
		toVal(digestID),
		usage.InputTokens,
		usage.OutputTokens,
		usage.CacheCreationInputTokens,
		usage.CacheReadInputTokens,
	)
	sum := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(sum[:])
}

func loadUserAnthropicAPIKey(ctx context.Context, settingsRepo *repository.UserSettingsRepo, cipher *service.SecretCipher, userID *string) (*string, error) {
	if settingsRepo == nil || userID == nil || *userID == "" {
		return nil, fmt.Errorf("user anthropic api key is required")
	}
	enc, err := settingsRepo.GetAnthropicAPIKeyEncrypted(ctx, *userID)
	if err != nil || enc == nil {
		if err != nil {
			return nil, err
		}
		return nil, fmt.Errorf("user anthropic api key is required")
	}
	if cipher == nil || !cipher.Enabled() {
		return nil, fmt.Errorf("user secret encryption is not configured")
	}
	plain, err := cipher.DecryptString(*enc)
	if err != nil {
		return nil, fmt.Errorf("decrypt user anthropic key: %w", err)
	}
	return &plain, nil
}

func loadUserOpenAIAPIKey(ctx context.Context, settingsRepo *repository.UserSettingsRepo, cipher *service.SecretCipher, userID *string) (*string, error) {
	if settingsRepo == nil || userID == nil || *userID == "" {
		return nil, fmt.Errorf("user openai api key is required")
	}
	enc, err := settingsRepo.GetOpenAIAPIKeyEncrypted(ctx, *userID)
	if err != nil || enc == nil {
		if err != nil {
			return nil, err
		}
		return nil, fmt.Errorf("user openai api key is required")
	}
	if cipher == nil || !cipher.Enabled() {
		return nil, fmt.Errorf("user secret encryption is not configured")
	}
	plain, err := cipher.DecryptString(*enc)
	if err != nil {
		return nil, fmt.Errorf("decrypt user openai key: %w", err)
	}
	return &plain, nil
}

func loadUserGoogleAPIKey(ctx context.Context, settingsRepo *repository.UserSettingsRepo, cipher *service.SecretCipher, userID *string) (*string, error) {
	if settingsRepo == nil || userID == nil || *userID == "" {
		return nil, fmt.Errorf("user google api key is required")
	}
	enc, err := settingsRepo.GetGoogleAPIKeyEncrypted(ctx, *userID)
	if err != nil || enc == nil {
		if err != nil {
			return nil, err
		}
		return nil, fmt.Errorf("user google api key is required")
	}
	if cipher == nil || !cipher.Enabled() {
		return nil, fmt.Errorf("user secret encryption is not configured")
	}
	plain, err := cipher.DecryptString(*enc)
	if err != nil {
		return nil, fmt.Errorf("decrypt user google key: %w", err)
	}
	return &plain, nil
}

func ptrStringOrNil(v *string) *string {
	if v == nil || *v == "" {
		return nil
	}
	s := *v
	return &s
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

func digestTopicKey(topics []string) string {
	for _, t := range topics {
		t = strings.TrimSpace(t)
		if t != "" {
			return t
		}
	}
	return "__untagged__"
}

// buildDigestClusterDrafts compacts digest inputs into topic buckets and stores
// representative snippets per bucket. This becomes the intermediate artifact for final compose.
func buildDigestClusterDrafts(details []model.DigestItemDetail, embClusters []model.ReadingPlanCluster) []model.DigestClusterDraft {
	if len(details) == 0 {
		return nil
	}
	byID := make(map[string]model.DigestItemDetail, len(details))
	for _, d := range details {
		byID[d.Item.ID] = d
	}
	seen := map[string]struct{}{}
	out := make([]model.DigestClusterDraft, 0, len(details))

	appendDraft := func(idx int, key, label string, group []model.DigestItemDetail) {
		if len(group) == 0 {
			return
		}
		maxScore := 0.0
		hasScore := false
		lines := make([]string, 0, minInt(4, len(group)))
		for i, it := range group {
			if it.Summary.Score != nil {
				if !hasScore || *it.Summary.Score > maxScore {
					maxScore = *it.Summary.Score
					hasScore = true
				}
			}
			if i >= 4 {
				continue
			}
			title := strings.TrimSpace(coalescePtrStr(it.Item.Title, it.Item.URL))
			summary := strings.TrimSpace(it.Summary.Summary)
			factLine := ""
			if len(it.Facts) > 0 {
				facts := make([]string, 0, minInt(2, len(it.Facts)))
				for _, f := range it.Facts {
					f = strings.TrimSpace(f)
					if f == "" {
						continue
					}
					facts = append(facts, f)
					if len(facts) >= 2 {
						break
					}
				}
				if len(facts) > 0 {
					factLine = strings.Join(facts, " / ")
				}
			}
			switch {
			case summary != "" && factLine != "":
				lines = append(lines, "- "+title+": "+summary+" | facts: "+factLine)
			case summary != "":
				lines = append(lines, "- "+title+": "+summary)
			case factLine != "":
				lines = append(lines, "- "+title+": "+factLine)
			default:
				lines = append(lines, "- "+title)
			}
		}
		draftSummary := strings.Join(lines, "\n")
		if len(group) > 4 {
			draftSummary += fmt.Sprintf("\n- ...and %d more related items", len(group)-4)
		}
		var scorePtr *float64
		if hasScore {
			v := maxScore
			scorePtr = &v
		}
		out = append(out, model.DigestClusterDraft{
			ClusterKey:   key,
			ClusterLabel: label,
			Rank:         idx,
			ItemCount:    len(group),
			Topics:       group[0].Summary.Topics,
			MaxScore:     scorePtr,
			DraftSummary: draftSummary,
		})
	}

	rank := 1
	for _, c := range embClusters {
		group := make([]model.DigestItemDetail, 0, len(c.Items))
		for _, m := range c.Items {
			d, ok := byID[m.ID]
			if !ok {
				continue
			}
			if _, dup := seen[d.Item.ID]; dup {
				continue
			}
			seen[d.Item.ID] = struct{}{}
			group = append(group, d)
		}
		if len(group) == 0 {
			continue
		}
		label := c.Label
		if strings.TrimSpace(label) == "" {
			label = digestTopicKey(group[0].Summary.Topics)
		}
		appendDraft(rank, c.ID, label, group)
		rank++
	}

	// Add remaining singletons so the first-stage processing still covers all items.
	for _, d := range details {
		if _, ok := seen[d.Item.ID]; ok {
			continue
		}
		seen[d.Item.ID] = struct{}{}
		key := d.Item.ID
		label := digestTopicKey(d.Summary.Topics)
		appendDraft(rank, key, label, []model.DigestItemDetail{d})
		rank++
	}
	return out
}

func draftSourceLines(draftSummary string) []string {
	lines := strings.Split(draftSummary, "\n")
	out := make([]string, 0, len(lines))
	for _, l := range lines {
		l = strings.TrimSpace(l)
		if l == "" {
			continue
		}
		out = append(out, l)
	}
	return out
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func buildBroadDigestDraftFromChunk(chunk []model.DigestClusterDraft, key, label string) model.DigestClusterDraft {
	itemCount := 0
	var maxScore *float64
	lines := make([]string, 0, len(chunk))
	topicsSet := map[string]struct{}{}
	for _, d := range chunk {
		itemCount += d.ItemCount
		if d.MaxScore != nil && (maxScore == nil || *d.MaxScore > *maxScore) {
			v := *d.MaxScore
			maxScore = &v
		}
		for _, t := range d.Topics {
			t = strings.TrimSpace(t)
			if t == "" {
				continue
			}
			topicsSet[t] = struct{}{}
		}
		line := strings.TrimSpace(d.DraftSummary)
		if line == "" {
			continue
		}
		first := strings.Split(line, "\n")[0]
		lines = append(lines, fmt.Sprintf("- [%s] %s", d.ClusterLabel, first))
	}
	topics := make([]string, 0, len(topicsSet))
	for t := range topicsSet {
		topics = append(topics, t)
	}
	sort.Strings(topics)
	return model.DigestClusterDraft{
		ClusterKey:   key,
		ClusterLabel: label,
		ItemCount:    itemCount,
		Topics:       topics,
		MaxScore:     maxScore,
		DraftSummary: strings.Join(lines, "\n"),
	}
}

func compressDigestClusterDrafts(drafts []model.DigestClusterDraft, target int) []model.DigestClusterDraft {
	if target <= 0 {
		target = 20
	}
	if len(drafts) <= target {
		return drafts
	}

	// Keep larger/more informative clusters first; merge tail singletons/small clusters.
	keep := make([]model.DigestClusterDraft, 0, len(drafts))
	tail := make([]model.DigestClusterDraft, 0, len(drafts))
	for i, d := range drafts {
		if i < 10 || d.ItemCount >= 3 {
			keep = append(keep, d)
			continue
		}
		tail = append(tail, d)
	}
	broadCount := 0
	if len(tail) >= 4 {
		broadCount = 1
	}
	if len(tail) >= 10 {
		broadCount = 2
	}
	if len(keep) >= target {
		cut := target - broadCount
		if cut < 1 {
			cut = target
			broadCount = 0
		}
		keep = keep[:cut]
		if broadCount > 0 {
			if broadCount == 1 {
				keep = append(keep, buildBroadDigestDraftFromChunk(tail, "broad-1", "幅広い話題（横断）"))
			} else {
				mid := len(tail) / 2
				if mid < 1 {
					mid = 1
				}
				keep = append(keep, buildBroadDigestDraftFromChunk(tail[:mid], "broad-1", "幅広い話題（横断）A"))
				keep = append(keep, buildBroadDigestDraftFromChunk(tail[mid:], "broad-2", "幅広い話題（横断）B"))
			}
		}
		for i := range keep {
			keep[i].Rank = i + 1
		}
		return keep
	}

	remainingSlots := target - len(keep)
	if remainingSlots <= 0 || len(tail) == 0 {
		for i := range keep {
			keep[i].Rank = i + 1
		}
		return keep
	}

	// Merge tail clusters into grouped "other" buckets to preserve coverage.
	chunkSize := int(math.Ceil(float64(len(tail)) / float64(remainingSlots)))
	if chunkSize < 2 {
		chunkSize = 2
	}
	for i := 0; i < len(tail) && len(keep) < target; i += chunkSize {
		end := i + chunkSize
		if end > len(tail) {
			end = len(tail)
		}
		chunk := tail[i:end]
		if len(chunk) == 1 {
			keep = append(keep, chunk[0])
			continue
		}
		keep = append(keep, buildBroadDigestDraftFromChunk(chunk, fmt.Sprintf("merged-tail-%d", len(keep)+1), "その他の話題"))
	}

	for i := range keep {
		keep[i].Rank = i + 1
	}
	return keep
}

func buildComposeItemsFromClusterDrafts(drafts []model.DigestClusterDraft, maxItems int) []service.ComposeDigestItem {
	_ = maxItems // keep signature compatible; compose now uses all cluster drafts by default.
	out := make([]service.ComposeDigestItem, 0, len(drafts))
	for i, d := range drafts {
		title := d.ClusterLabel
		if d.ItemCount > 1 {
			title = fmt.Sprintf("%s (%d items)", d.ClusterLabel, d.ItemCount)
		}
		summary := d.DraftSummary
		// Keep coverage across all cluster drafts, while reducing detail for lower-ranked clusters.
		// This avoids "top clusters only" behavior without sending every draft at full verbosity.
		if i >= 12 {
			lines := strings.Split(strings.TrimSpace(d.DraftSummary), "\n")
			if len(lines) > 0 && strings.TrimSpace(lines[0]) != "" {
				summary = lines[0]
			}
			if len(lines) > 1 {
				summary += fmt.Sprintf("\n- ...%d more lines omitted in compose input", len(lines)-1)
			}
		}
		titlePtr := title
		out = append(out, service.ComposeDigestItem{
			Rank:    i + 1,
			Title:   &titlePtr,
			URL:     "",
			Summary: summary,
			Topics:  d.Topics,
			Score:   d.MaxScore,
		})
	}
	return out
}

func coalescePtrStr(a *string, b string) string {
	if a != nil && strings.TrimSpace(*a) != "" {
		return *a
	}
	return b
}

// Event payloads

type ItemCreatedData struct {
	ItemID   string `json:"item_id"`
	SourceID string `json:"source_id"`
	URL      string `json:"url"`
}

type DigestCreatedData struct {
	DigestID string `json:"digest_id"`
	UserID   string `json:"user_id"`
	To       string `json:"to"`
}

type DigestCopyComposedData struct {
	DigestID string `json:"digest_id"`
	UserID   string `json:"user_id"`
	To       string `json:"to"`
}

// NewHandler registers all Inngest functions and returns the HTTP handler.
func NewHandler(db *pgxpool.Pool, worker *service.WorkerClient, resend *service.ResendClient) http.Handler {
	secretCipher := service.NewSecretCipher()
	openAI := service.NewOpenAIClient()
	client, err := inngestgo.NewClient(inngestgo.ClientOpts{
		AppID: "sifto-api",
	})
	if err != nil {
		log.Fatalf("inngest client: %v", err)
	}

	register := func(f inngestgo.ServableFunction, err error) {
		if err != nil {
			log.Fatalf("register function: %v", err)
		}
	}

	register(fetchRSSFn(client, db))
	register(processItemFn(client, db, worker, openAI, secretCipher))
	register(embedItemFn(client, db, openAI, secretCipher))
	register(generateBriefingSnapshotsFn(client, db))
	register(generateDigestFn(client, db))
	register(composeDigestCopyFn(client, db, worker, secretCipher))
	register(sendDigestFn(client, db, worker, resend, secretCipher))
	register(checkBudgetAlertsFn(client, db, resend))

	return client.Serve()
}

// cron/generate-briefing-snapshots — 30分ごとに当日ブリーフィングのスナップショットを更新
func generateBriefingSnapshotsFn(client inngestgo.Client, db *pgxpool.Pool) (inngestgo.ServableFunction, error) {
	userRepo := repository.NewUserRepo(db)
	itemRepo := repository.NewItemRepo(db)
	streakRepo := repository.NewReadingStreakRepo(db)
	snapshotRepo := repository.NewBriefingSnapshotRepo(db)

	return inngestgo.CreateFunction(
		client,
		inngestgo.FunctionOpts{ID: "generate-briefing-snapshots", Name: "Generate Briefing Snapshots"},
		inngestgo.CronTrigger("*/30 * * * *"),
		func(ctx context.Context, input inngestgo.Input[any]) (any, error) {
			users, err := userRepo.ListAll(ctx)
			if err != nil {
				return nil, fmt.Errorf("list users: %w", err)
			}
			today := timeutil.StartOfDayJST(timeutil.NowJST())
			dateStr := today.Format("2006-01-02")
			updated := 0
			failed := 0
			for _, u := range users {
				payload, err := service.BuildBriefingToday(ctx, itemRepo, streakRepo, u.ID, today, 18)
				if err != nil {
					failed++
					log.Printf("generate-briefing-snapshots build user=%s: %v", u.ID, err)
					continue
				}
				payload.Status = "ready"
				if err := snapshotRepo.Upsert(ctx, u.ID, dateStr, "ready", payload); err != nil {
					failed++
					log.Printf("generate-briefing-snapshots upsert user=%s: %v", u.ID, err)
					continue
				}
				updated++
			}
			return map[string]any{
				"date":    dateStr,
				"users":   len(users),
				"updated": updated,
				"failed":  failed,
			}, nil
		},
	)
}

// ① cron/fetch-rss — 10分ごとにRSSを取得し新規アイテムを登録
func fetchRSSFn(client inngestgo.Client, db *pgxpool.Pool) (inngestgo.ServableFunction, error) {
	sourceRepo := repository.NewSourceRepo(db)
	itemRepo := repository.NewItemRepo(db)

	return inngestgo.CreateFunction(
		client,
		inngestgo.FunctionOpts{ID: "fetch-rss", Name: "Fetch RSS Feeds"},
		inngestgo.CronTrigger("*/10 * * * *"),
		func(ctx context.Context, input inngestgo.Input[any]) (any, error) {
			sources, err := sourceRepo.ListEnabled(ctx)
			if err != nil {
				return nil, fmt.Errorf("list sources: %w", err)
			}

			fp := gofeed.NewParser()
			newCount := 0

			for _, src := range sources {
				feed, err := fp.ParseURLWithContext(src.URL, ctx)
				if err != nil {
					log.Printf("fetch rss %s: %v", src.URL, err)
					_ = sourceRepo.UpdateLastFetchedAt(ctx, src.ID, timeutil.NowJST())
					reason := fmt.Sprintf("fetch error: %v", err)
					_ = sourceRepo.RefreshHealthSnapshot(ctx, src.ID, &reason)
					continue
				}

				for _, entry := range feed.Items {
					if entry.Link == "" {
						continue
					}
					var title *string
					if entry.Title != "" {
						title = &entry.Title
					}
					itemID, created, err := itemRepo.UpsertFromFeed(ctx, src.ID, entry.Link, title)
					if err != nil {
						log.Printf("upsert item %s: %v", entry.Link, err)
						continue
					}
					if !created {
						continue
					}
					newCount++
					payload := map[string]any{
						"item_id":   itemID,
						"source_id": src.ID,
						"url":       entry.Link,
					}
					if title != nil && strings.TrimSpace(*title) != "" {
						payload["title"] = strings.TrimSpace(*title)
					}
					if _, err := client.Send(ctx, inngestgo.Event{
						Name: "item/created",
						Data: payload,
					}); err != nil {
						log.Printf("send item/created: %v", err)
					}
				}
				_ = sourceRepo.UpdateLastFetchedAt(ctx, src.ID, timeutil.NowJST())
				_ = sourceRepo.RefreshHealthSnapshot(ctx, src.ID, nil)
			}
			return map[string]int{"new_items": newCount}, nil
		},
	)
}

// ② event/process-item — 本文抽出 → 事実抽出 → 要約（各stepでリトライ可能）
func processItemFn(client inngestgo.Client, db *pgxpool.Pool, worker *service.WorkerClient, openAI *service.OpenAIClient, secretCipher *service.SecretCipher) (inngestgo.ServableFunction, error) {
	itemRepo := repository.NewItemInngestRepo(db)
	llmUsageRepo := repository.NewLLMUsageLogRepo(db)
	sourceRepo := repository.NewSourceRepo(db)
	userSettingsRepo := repository.NewUserSettingsRepo(db)

	type EventData struct {
		ItemID   string `json:"item_id"`
		SourceID string `json:"source_id"`
		URL      string `json:"url"`
		Title    string `json:"title"`
	}

	return inngestgo.CreateFunction(
		client,
		inngestgo.FunctionOpts{ID: "process-item", Name: "Process Item"},
		inngestgo.EventTrigger("item/created", nil),
		func(ctx context.Context, input inngestgo.Input[EventData]) (any, error) {
			data := input.Event.Data
			itemID := data.ItemID
			url := data.URL
			var userIDPtr *string
			if data.SourceID != "" {
				if uid, err := sourceRepo.GetUserIDBySourceID(ctx, data.SourceID); err == nil {
					userIDPtr = &uid
				} else {
					log.Printf("process-item source owner lookup failed source_id=%s err=%v", data.SourceID, err)
				}
			}
			var userModelSettings *model.UserSettings
			if userIDPtr != nil && *userIDPtr != "" {
				userModelSettings, _ = userSettingsRepo.GetByUserID(ctx, *userIDPtr)
			}
			log.Printf("process-item start item_id=%s url=%s", itemID, url)

			// Step 1: 本文抽出
			extracted, err := step.Run(ctx, "extract-body", func(ctx context.Context) (*service.ExtractBodyResponse, error) {
				log.Printf("process-item extract-body start item_id=%s", itemID)
				return worker.ExtractBody(ctx, url)
			})
			if err != nil {
				log.Printf("process-item extract-body failed item_id=%s err=%v", itemID, err)
				msg := fmt.Sprintf("extract body: %v", err)
				_ = itemRepo.MarkFailed(ctx, itemID, &msg)
				return nil, fmt.Errorf("extract body: %w", err)
			}
			log.Printf("process-item extract-body done item_id=%s content_len=%d", itemID, len(extracted.Content))

			var publishedAt *time.Time
			if extracted.PublishedAt != nil {
				t, err := timeutil.ParseToJST(*extracted.PublishedAt)
				if err == nil {
					publishedAt = &t
				}
			}
			if err := itemRepo.UpdateAfterExtract(ctx, itemID, extracted.Content, extracted.Title, extracted.ImageURL, publishedAt); err != nil {
				log.Printf("process-item update-after-extract failed item_id=%s err=%v", itemID, err)
				return nil, fmt.Errorf("update after extract: %w", err)
			}
			log.Printf("process-item update-after-extract done item_id=%s", itemID)
			titleForLLM := extracted.Title
			if titleForLLM == nil || strings.TrimSpace(*titleForLLM) == "" {
				eventTitle := strings.TrimSpace(data.Title)
				if eventTitle != "" {
					titleForLLM = &eventTitle
				}
			}

			// Step 2: 事実抽出
			factsResp, err := step.Run(ctx, "extract-facts", func(ctx context.Context) (*service.ExtractFactsResponse, error) {
				log.Printf("process-item extract-facts start item_id=%s", itemID)
				var modelOverride *string
				if userModelSettings != nil {
					modelOverride = ptrStringOrNil(userModelSettings.AnthropicFactsModel)
				}
				var userAnthropicKey *string
				var userGoogleKey *string
				if !isGeminiModel(modelOverride) {
					key, err := loadUserAnthropicAPIKey(ctx, userSettingsRepo, secretCipher, userIDPtr)
					if err != nil {
						return nil, err
					}
					userAnthropicKey = key
				} else {
					key, err := loadUserGoogleAPIKey(ctx, userSettingsRepo, secretCipher, userIDPtr)
					if err != nil {
						return nil, err
					}
					userGoogleKey = key
				}
				return worker.ExtractFactsWithModel(ctx, titleForLLM, extracted.Content, userAnthropicKey, userGoogleKey, modelOverride)
			})
			if err != nil {
				log.Printf("process-item extract-facts failed item_id=%s err=%v", itemID, err)
				msg := fmt.Sprintf("extract facts: %v", err)
				_ = itemRepo.MarkFailed(ctx, itemID, &msg)
				return nil, fmt.Errorf("extract facts: %w", err)
			}
			log.Printf("process-item extract-facts done item_id=%s facts=%d", itemID, len(factsResp.Facts))
			recordLLMUsage(ctx, llmUsageRepo, "facts", factsResp.LLM, userIDPtr, &data.SourceID, &itemID, nil)
			if err := itemRepo.InsertFacts(ctx, itemID, factsResp.Facts); err != nil {
				log.Printf("process-item insert-facts failed item_id=%s err=%v", itemID, err)
				return nil, fmt.Errorf("insert facts: %w", err)
			}
			log.Printf("process-item insert-facts done item_id=%s", itemID)

			// Step 3: 要約
			summary, err := step.Run(ctx, "summarize", func(ctx context.Context) (*service.SummarizeResponse, error) {
				log.Printf("process-item summarize start item_id=%s", itemID)
				var modelOverride *string
				if userModelSettings != nil {
					modelOverride = ptrStringOrNil(userModelSettings.AnthropicSummaryModel)
				}
				var userAnthropicKey *string
				var userGoogleKey *string
				if !isGeminiModel(modelOverride) {
					key, err := loadUserAnthropicAPIKey(ctx, userSettingsRepo, secretCipher, userIDPtr)
					if err != nil {
						return nil, err
					}
					userAnthropicKey = key
				} else {
					key, err := loadUserGoogleAPIKey(ctx, userSettingsRepo, secretCipher, userIDPtr)
					if err != nil {
						return nil, err
					}
					userGoogleKey = key
				}
				sourceChars := len(extracted.Content)
				return worker.SummarizeWithModel(ctx, titleForLLM, factsResp.Facts, &sourceChars, userAnthropicKey, userGoogleKey, modelOverride)
			})
			if err != nil {
				log.Printf("process-item summarize failed item_id=%s err=%v", itemID, err)
				msg := fmt.Sprintf("summarize: %v", err)
				_ = itemRepo.MarkFailed(ctx, itemID, &msg)
				return nil, fmt.Errorf("summarize: %w", err)
			}
			log.Printf("process-item summarize done item_id=%s topics=%d score=%.3f", itemID, len(summary.Topics), summary.Score)
			recordLLMUsage(ctx, llmUsageRepo, "summary", summary.LLM, userIDPtr, &data.SourceID, &itemID, nil)
			if err := itemRepo.InsertSummary(
				ctx,
				itemID,
				summary.Summary,
				summary.Topics,
				summary.TranslatedTitle,
				summary.Score,
				summary.ScoreBreakdown,
				summary.ScoreReason,
				summary.ScorePolicyVersion,
			); err != nil {
				log.Printf("process-item insert-summary failed item_id=%s err=%v", itemID, err)
				return nil, fmt.Errorf("insert summary: %w", err)
			}

			// Step 4: Embedding生成（関連記事用）: 失敗しても記事処理全体は成功扱い
			if userOpenAIKey, err := loadUserOpenAIAPIKey(ctx, userSettingsRepo, secretCipher, userIDPtr); err != nil {
				log.Printf("process-item embedding skip item_id=%s reason=%v", itemID, err)
			} else {
				inputText := buildItemEmbeddingInput(titleForLLM, summary.Summary, summary.Topics, factsResp.Facts)
				embModel := service.OpenAIEmbeddingModel()
				if userModelSettings != nil && userModelSettings.OpenAIEmbeddingModel != nil && service.IsSupportedOpenAIEmbeddingModel(*userModelSettings.OpenAIEmbeddingModel) {
					embModel = *userModelSettings.OpenAIEmbeddingModel
				}
				embResp, err := step.Run(ctx, "create-embedding", func(ctx context.Context) (*service.CreateEmbeddingResponse, error) {
					log.Printf("process-item create-embedding start item_id=%s model=%s", itemID, embModel)
					return openAI.CreateEmbedding(ctx, *userOpenAIKey, embModel, inputText)
				})
				if err != nil {
					log.Printf("process-item create-embedding failed item_id=%s err=%v", itemID, err)
				} else {
					if err := itemRepo.UpsertEmbedding(ctx, itemID, embModel, embResp.Embedding); err != nil {
						log.Printf("process-item upsert-embedding failed item_id=%s err=%v", itemID, err)
					} else {
						recordLLMUsage(ctx, llmUsageRepo, "embedding", embResp.LLM, userIDPtr, &data.SourceID, &itemID, nil)
						log.Printf("process-item create-embedding done item_id=%s dims=%d", itemID, len(embResp.Embedding))
					}
				}
			}
			log.Printf("process-item complete item_id=%s", itemID)

			return map[string]string{"item_id": itemID, "status": "summarized"}, nil
		},
	)
}

func embedItemFn(client inngestgo.Client, db *pgxpool.Pool, openAI *service.OpenAIClient, secretCipher *service.SecretCipher) (inngestgo.ServableFunction, error) {
	itemRepo := repository.NewItemInngestRepo(db)
	llmUsageRepo := repository.NewLLMUsageLogRepo(db)
	userSettingsRepo := repository.NewUserSettingsRepo(db)

	type EventData struct {
		ItemID   string `json:"item_id"`
		SourceID string `json:"source_id"`
	}

	return inngestgo.CreateFunction(
		client,
		inngestgo.FunctionOpts{ID: "embed-item", Name: "Create Item Embedding"},
		inngestgo.EventTrigger("item/embed", nil),
		func(ctx context.Context, input inngestgo.Input[EventData]) (any, error) {
			data := input.Event.Data
			if data.ItemID == "" {
				return nil, fmt.Errorf("item_id is required")
			}

			candidate, err := itemRepo.GetEmbeddingCandidate(ctx, data.ItemID)
			if err != nil {
				return nil, fmt.Errorf("get embedding candidate: %w", err)
			}
			userID := candidate.UserID
			userOpenAIKey, err := loadUserOpenAIAPIKey(ctx, userSettingsRepo, secretCipher, &userID)
			if err != nil {
				return nil, err
			}
			userModelSettings, _ := userSettingsRepo.GetByUserID(ctx, userID)

			inputText := buildItemEmbeddingInput(candidate.Title, candidate.Summary, candidate.Topics, candidate.Facts)
			embModel := service.OpenAIEmbeddingModel()
			if userModelSettings != nil && userModelSettings.OpenAIEmbeddingModel != nil && service.IsSupportedOpenAIEmbeddingModel(*userModelSettings.OpenAIEmbeddingModel) {
				embModel = *userModelSettings.OpenAIEmbeddingModel
			}
			embResp, err := step.Run(ctx, "create-embedding", func(ctx context.Context) (*service.CreateEmbeddingResponse, error) {
				return openAI.CreateEmbedding(ctx, *userOpenAIKey, embModel, inputText)
			})
			if err != nil {
				return nil, err
			}
			if err := itemRepo.UpsertEmbedding(ctx, candidate.ItemID, embModel, embResp.Embedding); err != nil {
				return nil, fmt.Errorf("upsert embedding: %w", err)
			}

			recordLLMUsage(ctx, llmUsageRepo, "embedding", embResp.LLM, &candidate.UserID, &candidate.SourceID, &candidate.ItemID, nil)
			return map[string]any{
				"item_id":    candidate.ItemID,
				"source_id":  candidate.SourceID,
				"dimensions": len(embResp.Embedding),
				"status":     "embedded",
				"model":      embModel,
			}, nil
		},
	)
}

func buildItemEmbeddingInput(title *string, summary string, topics, facts []string) string {
	out := ""
	if title != nil && *title != "" {
		out += "title: " + *title + "\n"
	}
	if summary != "" {
		out += "summary: " + summary + "\n"
	}
	if len(topics) > 0 {
		out += "topics: " + fmt.Sprintf("%v", topics) + "\n"
	}
	if len(facts) > 0 {
		out += "facts:\n"
		limit := len(facts)
		if limit > 12 {
			limit = 12
		}
		for i := 0; i < limit; i++ {
			out += "- " + facts[i] + "\n"
		}
	}
	return out
}

// ③ cron/generate-digest — 毎朝6:00 JST (UTC 21:00) にDigest生成
func generateDigestFn(client inngestgo.Client, db *pgxpool.Pool) (inngestgo.ServableFunction, error) {
	userRepo := repository.NewUserRepo(db)
	itemRepo := repository.NewItemInngestRepo(db)
	digestRepo := repository.NewDigestInngestRepo(db)

	return inngestgo.CreateFunction(
		client,
		inngestgo.FunctionOpts{ID: "generate-digest", Name: "Generate Daily Digest"},
		inngestgo.CronTrigger("0 21 * * *"),
		func(ctx context.Context, input inngestgo.Input[any]) (any, error) {
			users, err := userRepo.ListAll(ctx)
			if err != nil {
				return nil, fmt.Errorf("list users: %w", err)
			}

			today := timeutil.StartOfDayJST(timeutil.NowJST())
			since := today.AddDate(0, 0, -1)

			created := 0
			skippedSent := 0
			for _, u := range users {
				items, err := itemRepo.ListSummarizedForUser(ctx, u.ID, since, today)
				if err != nil || len(items) == 0 {
					continue
				}

				digestID, alreadySent, err := digestRepo.Create(ctx, u.ID, today, items)
				if err != nil {
					log.Printf("create digest for %s: %v", u.Email, err)
					continue
				}
				if alreadySent {
					skippedSent++
					continue
				}

				if _, err := client.Send(ctx, inngestgo.Event{
					Name: "digest/created",
					Data: map[string]any{
						"digest_id": digestID,
						"user_id":   u.ID,
						"to":        u.Email,
					},
				}); err != nil {
					log.Printf("send digest/created: %v", err)
				}
				created++
			}
			return map[string]int{
				"digests_created":      created,
				"digests_skipped_sent": skippedSent,
			}, nil
		},
	)
}

// ④ event/compose-digest-copy — メール本文生成（重い処理を分離）
func composeDigestCopyFn(client inngestgo.Client, db *pgxpool.Pool, worker *service.WorkerClient, secretCipher *service.SecretCipher) (inngestgo.ServableFunction, error) {
	digestRepo := repository.NewDigestInngestRepo(db)
	itemRepo := repository.NewItemRepo(db)
	llmUsageRepo := repository.NewLLMUsageLogRepo(db)
	userSettingsRepo := repository.NewUserSettingsRepo(db)

	return inngestgo.CreateFunction(
		client,
		inngestgo.FunctionOpts{ID: "compose-digest-copy", Name: "Compose Digest Email Copy"},
		inngestgo.EventTrigger("digest/created", nil),
		func(ctx context.Context, input inngestgo.Input[DigestCreatedData]) (any, error) {
			data := input.Event.Data
			log.Printf("compose-digest-copy start digest_id=%s", data.DigestID)
			markStatus := func(status string, sendErr error) {
				var msg *string
				if sendErr != nil {
					s := sendErr.Error()
					if len(s) > 2000 {
						s = s[:2000]
					}
					msg = &s
				}
				if err := digestRepo.UpdateSendStatus(ctx, data.DigestID, status, msg); err != nil {
					log.Printf("compose-digest-copy update-status failed digest_id=%s status=%s err=%v", data.DigestID, status, err)
				}
			}
			userModelSettings, _ := userSettingsRepo.GetByUserID(ctx, data.UserID)

			// Read-only DB fetch does not need step state, and keeping large nested structs
			// out of step results avoids serialization/replay issues.
			digest, err := digestRepo.GetForEmail(ctx, data.DigestID)
			if err != nil {
				markStatus("fetch_failed", err)
				return nil, fmt.Errorf("fetch digest: %w", err)
			}
			log.Printf("compose-digest-copy fetched digest_id=%s items=%d", data.DigestID, len(digest.Items))

			if len(digest.Items) == 0 {
				log.Printf("compose-digest-copy skip-no-items digest_id=%s", data.DigestID)
				markStatus("skipped_no_items", nil)
				return map[string]string{"status": "skipped", "reason": "no items"}, nil
			}
			markStatus("processing", nil)

			if digest.EmailSubject != nil && digest.EmailBody != nil {
				log.Printf("compose-digest-copy reuse-copy digest_id=%s", data.DigestID)
			} else {
				_, err := step.Run(ctx, "compose-digest-copy", func(ctx context.Context) (string, error) {
					log.Printf("compose-digest-copy step-exec digest_id=%s", data.DigestID)
					clusterItems := make([]model.Item, 0, len(digest.Items))
					for _, di := range digest.Items {
						it := di.Item
						it.SummaryScore = di.Summary.Score
						it.SummaryTopics = di.Summary.Topics
						clusterItems = append(clusterItems, it)
					}
					embClusters, err := itemRepo.ClusterItemsByEmbeddings(ctx, clusterItems)
					if err != nil {
						return "", fmt.Errorf("cluster digest items: %w", err)
					}
					drafts := buildDigestClusterDrafts(digest.Items, embClusters)
					drafts = compressDigestClusterDrafts(drafts, 20)
					var clusterDraftModel *string
					if userModelSettings != nil {
						clusterDraftModel = ptrStringOrNil(userModelSettings.AnthropicDigestClusterModel)
					}
					var clusterDraftAnthropicKey *string
					var clusterDraftGoogleKey *string
					if !isGeminiModel(clusterDraftModel) {
						key, keyErr := loadUserAnthropicAPIKey(ctx, userSettingsRepo, secretCipher, &data.UserID)
						if keyErr != nil {
							return "", keyErr
						}
						clusterDraftAnthropicKey = key
					} else {
						key, keyErr := loadUserGoogleAPIKey(ctx, userSettingsRepo, secretCipher, &data.UserID)
						if keyErr != nil {
							return "", keyErr
						}
						clusterDraftGoogleKey = key
					}
					for i := range drafts {
						sourceLines := draftSourceLines(drafts[i].DraftSummary)
						if len(sourceLines) == 0 {
							continue
						}
						resp, err := worker.ComposeDigestClusterDraftWithModel(
							ctx,
							drafts[i].ClusterLabel,
							drafts[i].ItemCount,
							drafts[i].Topics,
							sourceLines,
							clusterDraftAnthropicKey,
							clusterDraftGoogleKey,
							clusterDraftModel,
						)
						if err != nil {
							return "", fmt.Errorf("compose digest cluster draft rank=%d: %w", drafts[i].Rank, err)
						}
						if resp != nil && strings.TrimSpace(resp.DraftSummary) != "" {
							drafts[i].DraftSummary = resp.DraftSummary
						}
						if resp != nil {
							recordLLMUsage(ctx, llmUsageRepo, "digest_cluster_draft", resp.LLM, &data.UserID, nil, nil, &data.DigestID)
						}
					}
					if err := digestRepo.ReplaceClusterDrafts(ctx, data.DigestID, drafts); err != nil {
						return "", fmt.Errorf("store digest cluster drafts: %w", err)
					}
					storedDrafts, err := digestRepo.ListClusterDrafts(ctx, data.DigestID)
					if err != nil {
						return "", fmt.Errorf("reload digest cluster drafts: %w", err)
					}
					items := buildComposeItemsFromClusterDrafts(storedDrafts, len(storedDrafts))
					log.Printf(
						"compose-digest-copy compacted digest_id=%s source_items=%d cluster_drafts=%d compose_items=%d",
						data.DigestID, len(digest.Items), len(storedDrafts), len(items),
					)
					var modelOverride *string
					if userModelSettings != nil {
						modelOverride = ptrStringOrNil(userModelSettings.AnthropicDigestModel)
					}
					var digestAnthropicKey *string
					var digestGoogleKey *string
					if !isGeminiModel(modelOverride) {
						key, keyErr := loadUserAnthropicAPIKey(ctx, userSettingsRepo, secretCipher, &data.UserID)
						if keyErr != nil {
							return "", keyErr
						}
						digestAnthropicKey = key
					} else {
						key, keyErr := loadUserGoogleAPIKey(ctx, userSettingsRepo, secretCipher, &data.UserID)
						if keyErr != nil {
							return "", keyErr
						}
						digestGoogleKey = key
					}
					resp, err := worker.ComposeDigestWithModel(ctx, digest.DigestDate, items, digestAnthropicKey, digestGoogleKey, modelOverride)
					if err != nil {
						return "", err
					}
					recordLLMUsage(ctx, llmUsageRepo, "digest", resp.LLM, &data.UserID, nil, nil, &data.DigestID)
					log.Printf("compose-digest-copy worker-done digest_id=%s subject_len=%d body_len=%d", data.DigestID, len(resp.Subject), len(resp.Body))
					if err := digestRepo.UpdateEmailCopy(ctx, data.DigestID, resp.Subject, resp.Body); err != nil {
						return "", err
					}
					return "stored", nil
				})
				if err != nil {
					markStatus("compose_failed", err)
					return nil, fmt.Errorf("compose digest copy: %w", err)
				}
			}

			if _, err := client.Send(ctx, inngestgo.Event{
				Name: "digest/copy-composed",
				Data: map[string]any{
					"digest_id": data.DigestID,
					"user_id":   data.UserID,
					"to":        data.To,
				},
			}); err != nil {
				markStatus("enqueue_send_failed", err)
				return nil, fmt.Errorf("send digest/copy-composed: %w", err)
			}
			log.Printf("compose-digest-copy complete digest_id=%s", data.DigestID)
			return map[string]string{"status": "composed", "digest_id": data.DigestID}, nil
		},
	)
}

// ⑤ event/send-digest — メール送信（compose完了後）
func sendDigestFn(client inngestgo.Client, db *pgxpool.Pool, worker *service.WorkerClient, resend *service.ResendClient, secretCipher *service.SecretCipher) (inngestgo.ServableFunction, error) {
	_ = worker
	_ = secretCipher
	digestRepo := repository.NewDigestInngestRepo(db)
	userSettingsRepo := repository.NewUserSettingsRepo(db)

	return inngestgo.CreateFunction(
		client,
		inngestgo.FunctionOpts{ID: "send-digest", Name: "Send Digest Email"},
		inngestgo.EventTrigger("digest/copy-composed", nil),
		func(ctx context.Context, input inngestgo.Input[DigestCopyComposedData]) (any, error) {
			data := input.Event.Data
			log.Printf("send-digest start digest_id=%s to=%s", data.DigestID, data.To)
			markStatus := func(status string, sendErr error) {
				var msg *string
				if sendErr != nil {
					s := sendErr.Error()
					if len(s) > 2000 {
						s = s[:2000]
					}
					msg = &s
				}
				if err := digestRepo.UpdateSendStatus(ctx, data.DigestID, status, msg); err != nil {
					log.Printf("send-digest update-status failed digest_id=%s status=%s err=%v", data.DigestID, status, err)
				}
			}

			digest, err := digestRepo.GetForEmail(ctx, data.DigestID)
			if err != nil {
				markStatus("fetch_failed", err)
				return nil, fmt.Errorf("fetch digest: %w", err)
			}
			if digest.EmailSubject == nil || digest.EmailBody == nil {
				err := fmt.Errorf("digest email copy is missing")
				markStatus("compose_failed", err)
				return nil, err
			}
			if !resend.Enabled() {
				markStatus("skipped_resend_disabled", nil)
				return map[string]string{"status": "skipped", "reason": "resend_disabled"}, nil
			}
			digestEmailEnabled, err := userSettingsRepo.IsDigestEmailEnabled(ctx, data.UserID)
			if err != nil {
				markStatus("user_settings_failed", err)
				return nil, fmt.Errorf("load user digest email setting: %w", err)
			}
			if !digestEmailEnabled {
				markStatus("skipped_user_disabled", nil)
				return map[string]string{"status": "skipped", "reason": "user_disabled"}, nil
			}
			markStatus("processing", nil)

			_, err = step.Run(ctx, "send-email", func(ctx context.Context) (string, error) {
				if err := resend.SendDigest(ctx, data.To, digest, &service.DigestEmailCopy{
					Subject: *digest.EmailSubject,
					Body:    *digest.EmailBody,
				}); err != nil {
					return "", err
				}
				return "sent", nil
			})
			if err != nil {
				markStatus("send_email_failed", err)
				return nil, fmt.Errorf("send email: %w", err)
			}
			if err := digestRepo.UpdateSentAt(ctx, data.DigestID); err != nil {
				log.Printf("update sent_at: %v", err)
			}
			log.Printf("send-digest complete digest_id=%s", data.DigestID)
			return map[string]string{"status": "sent", "to": data.To}, nil
		},
	)
}

func checkBudgetAlertsFn(client inngestgo.Client, db *pgxpool.Pool, resend *service.ResendClient) (inngestgo.ServableFunction, error) {
	settingsRepo := repository.NewUserSettingsRepo(db)
	alertLogRepo := repository.NewBudgetAlertLogRepo(db)
	llmUsageRepo := repository.NewLLMUsageLogRepo(db)

	return inngestgo.CreateFunction(
		client,
		inngestgo.FunctionOpts{ID: "check-budget-alerts", Name: "Check Monthly Budget Alerts"},
		inngestgo.CronTrigger("0 0 * * *"), // 09:00 JST
		func(ctx context.Context, input inngestgo.Input[any]) (any, error) {
			if !resend.Enabled() {
				return map[string]any{"status": "skipped", "reason": "resend_disabled"}, nil
			}

			targets, err := settingsRepo.ListBudgetAlertTargets(ctx)
			if err != nil {
				return nil, fmt.Errorf("list budget alert targets: %w", err)
			}

			nowJST := timeutil.NowJST()
			monthStartJST := time.Date(nowJST.Year(), nowJST.Month(), 1, 0, 0, 0, 0, timeutil.JST)
			nextMonthJST := monthStartJST.AddDate(0, 1, 0)
			checked := 0
			sent := 0
			skipped := 0

			for _, tgt := range targets {
				checked++
				usedCostUSD, err := llmUsageRepo.SumEstimatedCostByUserBetween(ctx, tgt.UserID, monthStartJST, nextMonthJST)
				if err != nil {
					log.Printf("check-budget-alerts sum cost user_id=%s: %v", tgt.UserID, err)
					continue
				}
				if tgt.MonthlyBudgetUSD <= 0 {
					skipped++
					continue
				}
				remainingRatio := (tgt.MonthlyBudgetUSD - usedCostUSD) / tgt.MonthlyBudgetUSD
				thresholdRatio := float64(tgt.BudgetAlertThresholdPct) / 100.0
				if remainingRatio >= thresholdRatio {
					skipped++
					continue
				}
				alreadySent, err := alertLogRepo.Exists(ctx, tgt.UserID, monthStartJST, tgt.BudgetAlertThresholdPct)
				if err != nil {
					log.Printf("check-budget-alerts exists user_id=%s: %v", tgt.UserID, err)
					continue
				}
				if alreadySent {
					skipped++
					continue
				}

				remainingUSD := tgt.MonthlyBudgetUSD - usedCostUSD
				if err := resend.SendBudgetAlert(ctx, tgt.Email, service.BudgetAlertEmail{
					MonthJST:           monthStartJST.Format("2006-01"),
					MonthlyBudgetUSD:   tgt.MonthlyBudgetUSD,
					UsedCostUSD:        usedCostUSD,
					RemainingBudgetUSD: remainingUSD,
					RemainingPct:       remainingRatio * 100,
					ThresholdPct:       tgt.BudgetAlertThresholdPct,
				}); err != nil {
					log.Printf("check-budget-alerts send user_id=%s email=%s: %v", tgt.UserID, tgt.Email, err)
					continue
				}
				if err := alertLogRepo.Insert(ctx, tgt.UserID, monthStartJST, tgt.BudgetAlertThresholdPct, tgt.MonthlyBudgetUSD, usedCostUSD, remainingRatio); err != nil {
					log.Printf("check-budget-alerts log user_id=%s: %v", tgt.UserID, err)
				}
				sent++
			}

			return map[string]any{
				"checked":   checked,
				"sent":      sent,
				"skipped":   skipped,
				"month_jst": monthStartJST.Format("2006-01"),
			}, nil
		},
	)
}
