package inngest

import (
	"log"
	"net/http"

	"github.com/enjoydarts/sifto/api/internal/service"
	"github.com/inngest/inngestgo"
	"github.com/jackc/pgx/v5/pgxpool"
)

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

func NewHandler(db *pgxpool.Pool, worker *service.WorkerClient, resend *service.ResendClient, oneSignal *service.OneSignalClient, obsidianExport *service.ObsidianExportService, cache service.JSONCache, search *service.MeilisearchService, keyProvider *service.UserKeyProvider) http.Handler {
	openAI := service.NewOpenAIClient()
	llmUsageCache = cache
	_ = search
	client, err := service.NewInngestClient("sifto-api")
	if err != nil {
		log.Fatalf("inngest client: %v", err)
	}

	register := func(f inngestgo.ServableFunction, err error) {
		if err != nil {
			log.Fatalf("register function: %v", err)
		}
	}

	register(fetchRSSFn(client, db))
	register(processItemFn(client, db, worker, openAI, oneSignal, keyProvider, cache))
	register(itemSearchUpsertFn(client, db, search))
	register(itemSearchDeleteFn(client, search))
	register(searchSuggestionArticleUpsertFn(client, db, search))
	register(searchSuggestionArticleDeleteFn(client, search))
	register(searchSuggestionSourceUpsertFn(client, db, search))
	register(searchSuggestionSourceDeleteFn(client, search))
	register(searchSuggestionTopicsRefreshFn(client, db, search))
	register(itemSearchBackfillRunFn(client, db))
	register(itemSearchBackfillFn(client, db, search))
	register(embedItemFn(client, db, openAI, keyProvider))
	register(generateBriefingSnapshotsFn(client, db, oneSignal))
	register(notifyReviewQueueFn(client, db, oneSignal))
	register(exportObsidianFavoritesFn(client, db, obsidianExport))
	register(trackProviderModelUpdatesFn(client, db, oneSignal))
	register(syncOpenRouterModelsFn(client, db, resend, oneSignal))
	register(syncPoeUsageHistoryFn(client, db, keyProvider))
	register(generateAudioBriefingsFn(client, db, worker, cache))
	register(runAudioBriefingPipelineFn(client, db, worker, cache))
	register(failStaleAudioBriefingVoicingFn(client, db))
	register(moveAudioBriefingsToIAFn(client, db, worker))
	register(generateDigestFn(client, db))
	register(composeDigestCopyFn(client, db, worker, keyProvider))
	register(sendDigestFn(client, db, worker, resend, oneSignal))
	register(checkBudgetAlertsFn(client, db, resend, oneSignal))
	register(computePreferenceProfilesFn(client, db))
	register(computeTopicPulseDailyFn(client, db))
	register(generateAINavigatorBriefsFn(client, db, worker, oneSignal))
	register(runAINavigatorBriefPipelineFn(client, db, worker, oneSignal, llmUsageCache))

	return client.Serve()
}
