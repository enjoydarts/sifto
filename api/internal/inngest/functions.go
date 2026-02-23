package inngest

import (
	"crypto/sha256"
	"context"
	"encoding/hex"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/inngest/inngestgo"
	"github.com/inngest/inngestgo/step"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/mmcdole/gofeed"
	"github.com/minoru-kitayama/sifto/api/internal/repository"
	"github.com/minoru-kitayama/sifto/api/internal/service"
	"github.com/minoru-kitayama/sifto/api/internal/timeutil"
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
		UserID:                  userID,
		SourceID:                sourceID,
		ItemID:                  itemID,
		DigestID:                digestID,
		Provider:                usage.Provider,
		Model:                   usage.Model,
		PricingModelFamily:      usage.PricingModelFamily,
		PricingSource:           pricingSource,
		Purpose:                 purpose,
		InputTokens:             usage.InputTokens,
		OutputTokens:            usage.OutputTokens,
		CacheCreationInputTokens: usage.CacheCreationInputTokens,
		CacheReadInputTokens:    usage.CacheReadInputTokens,
		EstimatedCostUSD:        usage.EstimatedCostUSD,
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

// NewHandler registers all Inngest functions and returns the HTTP handler.
func NewHandler(db *pgxpool.Pool, worker *service.WorkerClient, resend *service.ResendClient) http.Handler {
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
	register(processItemFn(client, db, worker))
	register(generateDigestFn(client, db))
	register(sendDigestFn(client, db, worker, resend))

	return client.Serve()
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
					if _, err := client.Send(ctx, inngestgo.Event{
						Name: "item/created",
						Data: map[string]any{
							"item_id":   itemID,
							"source_id": src.ID,
							"url":       entry.Link,
						},
					}); err != nil {
						log.Printf("send item/created: %v", err)
					}
				}
				_ = sourceRepo.UpdateLastFetchedAt(ctx, src.ID, timeutil.NowJST())
			}
			return map[string]int{"new_items": newCount}, nil
		},
	)
}

// ② event/process-item — 本文抽出 → 事実抽出 → 要約（各stepでリトライ可能）
func processItemFn(client inngestgo.Client, db *pgxpool.Pool, worker *service.WorkerClient) (inngestgo.ServableFunction, error) {
	itemRepo := repository.NewItemInngestRepo(db)
	llmUsageRepo := repository.NewLLMUsageLogRepo(db)
	sourceRepo := repository.NewSourceRepo(db)

	type EventData struct {
		ItemID   string `json:"item_id"`
		SourceID string `json:"source_id"`
		URL      string `json:"url"`
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
			log.Printf("process-item start item_id=%s url=%s", itemID, url)

			// Step 1: 本文抽出
			extracted, err := step.Run(ctx, "extract-body", func(ctx context.Context) (*service.ExtractBodyResponse, error) {
				log.Printf("process-item extract-body start item_id=%s", itemID)
				return worker.ExtractBody(ctx, url)
			})
			if err != nil {
				log.Printf("process-item extract-body failed item_id=%s err=%v", itemID, err)
				_ = itemRepo.MarkFailed(ctx, itemID)
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
			if err := itemRepo.UpdateAfterExtract(ctx, itemID, extracted.Content, extracted.Title, publishedAt); err != nil {
				log.Printf("process-item update-after-extract failed item_id=%s err=%v", itemID, err)
				return nil, fmt.Errorf("update after extract: %w", err)
			}
			log.Printf("process-item update-after-extract done item_id=%s", itemID)

			// Step 2: 事実抽出
			factsResp, err := step.Run(ctx, "extract-facts", func(ctx context.Context) (*service.ExtractFactsResponse, error) {
				log.Printf("process-item extract-facts start item_id=%s", itemID)
				return worker.ExtractFacts(ctx, extracted.Title, extracted.Content)
			})
			if err != nil {
				log.Printf("process-item extract-facts failed item_id=%s err=%v", itemID, err)
				_ = itemRepo.MarkFailed(ctx, itemID)
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
				return worker.Summarize(ctx, extracted.Title, factsResp.Facts)
			})
			if err != nil {
				log.Printf("process-item summarize failed item_id=%s err=%v", itemID, err)
				_ = itemRepo.MarkFailed(ctx, itemID)
				return nil, fmt.Errorf("summarize: %w", err)
			}
			log.Printf("process-item summarize done item_id=%s topics=%d score=%.3f", itemID, len(summary.Topics), summary.Score)
			recordLLMUsage(ctx, llmUsageRepo, "summary", summary.LLM, userIDPtr, &data.SourceID, &itemID, nil)
			if err := itemRepo.InsertSummary(ctx, itemID, summary.Summary, summary.Topics, summary.Score); err != nil {
				log.Printf("process-item insert-summary failed item_id=%s err=%v", itemID, err)
				return nil, fmt.Errorf("insert summary: %w", err)
			}
			log.Printf("process-item complete item_id=%s", itemID)

			return map[string]string{"item_id": itemID, "status": "summarized"}, nil
		},
	)
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

// ④ event/send-digest — メール送信
func sendDigestFn(client inngestgo.Client, db *pgxpool.Pool, worker *service.WorkerClient, resend *service.ResendClient) (inngestgo.ServableFunction, error) {
	digestRepo := repository.NewDigestInngestRepo(db)
	llmUsageRepo := repository.NewLLMUsageLogRepo(db)

	type EventData struct {
		DigestID string `json:"digest_id"`
		UserID   string `json:"user_id"`
		To       string `json:"to"`
	}

	return inngestgo.CreateFunction(
		client,
		inngestgo.FunctionOpts{ID: "send-digest", Name: "Send Digest Email"},
		inngestgo.EventTrigger("digest/created", nil),
		func(ctx context.Context, input inngestgo.Input[EventData]) (any, error) {
			data := input.Event.Data
			log.Printf("send-digest start digest_id=%s to=%s", data.DigestID, data.To)

			// Read-only DB fetch does not need step state, and keeping large nested structs
			// out of step results avoids serialization/replay issues.
			digest, err := digestRepo.GetForEmail(ctx, data.DigestID)
			if err != nil {
				return nil, fmt.Errorf("fetch digest: %w", err)
			}
			log.Printf("send-digest fetched digest_id=%s items=%d", data.DigestID, len(digest.Items))

			if len(digest.Items) == 0 {
				log.Printf("send-digest skip-no-items digest_id=%s", data.DigestID)
				return map[string]string{"status": "skipped", "reason": "no items"}, nil
			}
			if !resend.Enabled() {
				log.Printf("send-digest skip-resend-disabled digest_id=%s", data.DigestID)
				return map[string]string{"status": "skipped", "reason": "resend_disabled"}, nil
			}

			var copy *service.ComposeDigestResponse
			if digest.EmailSubject != nil && digest.EmailBody != nil {
				copy = &service.ComposeDigestResponse{
					Subject: *digest.EmailSubject,
					Body:    *digest.EmailBody,
				}
				log.Printf("send-digest reuse-copy digest_id=%s", data.DigestID)
			} else {
				log.Printf("send-digest compose-copy start digest_id=%s", data.DigestID)
				_, err := step.Run(ctx, "compose-digest-copy", func(ctx context.Context) (string, error) {
					log.Printf("send-digest compose-copy step-exec digest_id=%s", data.DigestID)
					items := make([]service.ComposeDigestItem, 0, len(digest.Items))
					for _, it := range digest.Items {
						items = append(items, service.ComposeDigestItem{
							Rank:    it.Rank,
							Title:   it.Item.Title,
							URL:     it.Item.URL,
							Summary: it.Summary.Summary,
							Topics:  it.Summary.Topics,
							Score:   it.Summary.Score,
						})
					}
					resp, err := worker.ComposeDigest(ctx, digest.DigestDate, items)
					if err != nil {
						return "", err
					}
					recordLLMUsage(ctx, llmUsageRepo, "digest", resp.LLM, &data.UserID, nil, nil, &data.DigestID)
					log.Printf("send-digest compose-copy worker-done digest_id=%s subject_len=%d body_len=%d", data.DigestID, len(resp.Subject), len(resp.Body))
					if err := digestRepo.UpdateEmailCopy(ctx, data.DigestID, resp.Subject, resp.Body); err != nil {
						return "", err
					}
					return "stored", nil
				})
				if err != nil {
					return nil, fmt.Errorf("compose digest copy: %w", err)
				}
				digest, err = digestRepo.GetForEmail(ctx, data.DigestID)
				if err != nil {
					return nil, fmt.Errorf("refetch digest after compose: %w", err)
				}
				if digest.EmailSubject == nil || digest.EmailBody == nil {
					return nil, fmt.Errorf("compose digest copy: email copy not persisted")
				}
				copy = &service.ComposeDigestResponse{
					Subject: *digest.EmailSubject,
					Body:    *digest.EmailBody,
				}
				log.Printf("send-digest compose-copy done digest_id=%s subject_len=%d body_len=%d", data.DigestID, len(copy.Subject), len(copy.Body))
			}

			log.Printf("send-digest send-email start digest_id=%s", data.DigestID)
			_, err = step.Run(ctx, "send-email", func(ctx context.Context) (string, error) {
				if err := resend.SendDigest(ctx, data.To, digest, &service.DigestEmailCopy{
					Subject: copy.Subject,
					Body:    copy.Body,
				}); err != nil {
					return "", err
				}
				return "sent", nil
			})
			if err != nil {
				return nil, fmt.Errorf("send email: %w", err)
			}
			log.Printf("send-digest send-email done digest_id=%s", data.DigestID)

			if err := digestRepo.UpdateSentAt(ctx, data.DigestID); err != nil {
				log.Printf("update sent_at: %v", err)
			}
			log.Printf("send-digest complete digest_id=%s", data.DigestID)

			return map[string]string{"status": "sent", "to": data.To}, nil
		},
	)
}
