package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/enjoydarts/sifto/api/internal/model"
	"github.com/enjoydarts/sifto/api/internal/repository"
	"github.com/enjoydarts/sifto/api/internal/timeutil"
)

type audioBriefingPipelineStage string

const (
	audioBriefingPipelineStageNone   audioBriefingPipelineStage = ""
	audioBriefingPipelineStageScript audioBriefingPipelineStage = "script"
	audioBriefingPipelineStageVoice  audioBriefingPipelineStage = "voice"
	audioBriefingPipelineStageConcat audioBriefingPipelineStage = "concat"
)

type AudioBriefingOrchestrator struct {
	repo          *repository.AudioBriefingRepo
	settingsRepo  *repository.UserSettingsRepo
	llmUsageRepo  *repository.LLMUsageLogRepo
	cipher        *SecretCipher
	worker        *WorkerClient
	cache         JSONCache
	voiceRunner   *AudioBriefingVoiceRunner
	concatStarter *AudioBriefingConcatStarter
}

type audioBriefingRecoveredFailure struct {
	BatchSize int
	Err       error
}

type audioBriefingArticleBatchResult struct {
	Segments          []AudioBriefingScriptSegment
	RecoveredFailures []audioBriefingRecoveredFailure
}

func NewAudioBriefingOrchestrator(
	repo *repository.AudioBriefingRepo,
	settingsRepo *repository.UserSettingsRepo,
	llmUsageRepo *repository.LLMUsageLogRepo,
	cipher *SecretCipher,
	worker *WorkerClient,
	cache JSONCache,
	voiceRunner *AudioBriefingVoiceRunner,
	concatStarter *AudioBriefingConcatStarter,
) *AudioBriefingOrchestrator {
	return &AudioBriefingOrchestrator{
		repo:          repo,
		settingsRepo:  settingsRepo,
		llmUsageRepo:  llmUsageRepo,
		cipher:        cipher,
		worker:        worker,
		cache:         cache,
		voiceRunner:   voiceRunner,
		concatStarter: concatStarter,
	}
}

func (o *AudioBriefingOrchestrator) GenerateManual(ctx context.Context, userID string) (*model.AudioBriefingJob, error) {
	if o == nil || o.repo == nil {
		return nil, fmt.Errorf("audio briefing orchestrator unavailable")
	}
	settings, err := o.repo.EnsureSettingsDefaults(ctx, userID)
	if err != nil {
		return nil, err
	}
	now := timeutil.NowJST()
	return o.createPendingJob(ctx, userID, settings, now, AudioBriefingManualSlotKeyAt(now), settings.DefaultPersona)
}

func (o *AudioBriefingOrchestrator) GenerateScheduled(ctx context.Context, userID string, now time.Time) (*model.AudioBriefingJob, error) {
	if o == nil || o.repo == nil {
		return nil, fmt.Errorf("audio briefing orchestrator unavailable")
	}
	settings, err := o.repo.EnsureSettingsDefaults(ctx, userID)
	if err != nil {
		return nil, err
	}
	if !settings.Enabled {
		return nil, nil
	}
	slotStartedAt := AudioBriefingSlotStartAt(now, settings.IntervalHours)
	slotKey := AudioBriefingSlotKeyAt(slotStartedAt, settings.IntervalHours)
	existing, err := o.repo.GetJobBySlotKey(ctx, userID, slotKey)
	switch {
	case err == nil && existing != nil:
		return existing, nil
	case err != nil && !errors.Is(err, repository.ErrNotFound):
		return nil, err
	}

	job, err := o.createPendingJob(ctx, userID, settings, slotStartedAt, slotKey, settings.DefaultPersona)
	if err != nil {
		if errors.Is(err, repository.ErrConflict) {
			return o.repo.GetJobBySlotKey(ctx, userID, slotKey)
		}
		return nil, err
	}
	return job, nil
}

func (o *AudioBriefingOrchestrator) Resume(ctx context.Context, userID string, jobID string) (*model.AudioBriefingJob, error) {
	if o == nil || o.repo == nil {
		return nil, fmt.Errorf("audio briefing orchestrator unavailable")
	}
	job, err := o.repo.GetJobByID(ctx, userID, jobID)
	if err != nil {
		return nil, err
	}
	if !audioBriefingShouldContinue(job.Status) {
		return nil, repository.ErrInvalidState
	}
	return job, nil
}

func (o *AudioBriefingOrchestrator) RunPipeline(ctx context.Context, userID string, jobID string) (*model.AudioBriefingJob, error) {
	if o == nil || o.repo == nil {
		return nil, fmt.Errorf("audio briefing orchestrator unavailable")
	}
	return o.continuePipeline(ctx, userID, jobID)
}

func (o *AudioBriefingOrchestrator) createPendingJob(
	ctx context.Context,
	userID string,
	settings *model.AudioBriefingSettings,
	slotStartedAt time.Time,
	slotKey string,
	persona string,
) (*model.AudioBriefingJob, error) {
	if settings == nil {
		return nil, fmt.Errorf("audio briefing settings are required")
	}
	return o.repo.CreatePendingJob(ctx, userID, slotStartedAt, slotKey, persona)
}

func (o *AudioBriefingOrchestrator) continuePipeline(ctx context.Context, userID string, jobID string) (*model.AudioBriefingJob, error) {
	job, err := o.repo.GetJobByID(ctx, userID, jobID)
	if err != nil {
		return nil, err
	}
	chunks, err := o.repo.ListJobChunks(ctx, userID, jobID)
	if err != nil {
		return nil, err
	}
	stage, err := audioBriefingNextPipelineStage(job, chunks)
	if err != nil {
		if audioBriefingShouldContinue(job.Status) {
			return nil, err
		}
		return job, nil
	}

	switch stage {
	case audioBriefingPipelineStageScript:
		if err := o.runScriptingStage(ctx, job); err != nil {
			return o.loadJobAfterStageError(ctx, userID, jobID, err)
		}
		return o.continuePipeline(ctx, userID, jobID)
	case audioBriefingPipelineStageVoice:
		if o.voiceRunner == nil {
			return nil, fmt.Errorf("audio briefing voice runner unavailable")
		}
		if err := o.voiceRunner.Start(ctx, userID, jobID); err != nil {
			return o.loadJobAfterStageError(ctx, userID, jobID, err)
		}
		return o.continuePipeline(ctx, userID, jobID)
	case audioBriefingPipelineStageConcat:
		if o.concatStarter == nil {
			return nil, fmt.Errorf("audio briefing concat starter unavailable")
		}
		if err := o.concatStarter.Start(ctx, userID, jobID); err != nil {
			return o.loadJobAfterStageError(ctx, userID, jobID, err)
		}
		return o.repo.GetJobByID(ctx, userID, jobID)
	default:
		return job, nil
	}
}

func (o *AudioBriefingOrchestrator) runScriptingStage(ctx context.Context, job *model.AudioBriefingJob) error {
	if job == nil {
		return repository.ErrNotFound
	}
	if _, err := o.repo.StartScriptingJob(ctx, job.ID); err != nil {
		return err
	}

	settings, err := o.repo.EnsureSettingsDefaults(ctx, job.UserID)
	if err != nil {
		_, _ = o.repo.FailScriptingJob(ctx, job.ID, "script_settings_failed", err.Error())
		return err
	}
	items, err := o.repo.ListCandidateItems(ctx, job.UserID, settings.ArticlesPerEpisode)
	if err != nil {
		_, _ = o.repo.FailScriptingJob(ctx, job.ID, "script_candidates_failed", err.Error())
		return err
	}
	for i := range items {
		title := serviceAudioBriefingSegmentTitle(items[i])
		items[i].SegmentTitle = &title
	}
	voice, err := o.repo.GetPersonaVoice(ctx, job.UserID, job.Persona)
	if err != nil {
		_, _ = o.repo.FailScriptingJob(ctx, job.ID, "script_voice_failed", err.Error())
		return err
	}
	if voice == nil {
		err := fmt.Errorf("audio briefing voice is not configured")
		_, _ = o.repo.FailScriptingJob(ctx, job.ID, "script_voice_missing", err.Error())
		return err
	}
	draft, err := o.buildDraft(ctx, job.UserID, job.SlotStartedAtJST, job.Persona, items, voice, settings.TargetDurationMinutes)
	if err != nil {
		_, _ = o.repo.FailScriptingJob(ctx, job.ID, "script_failed", err.Error())
		return err
	}
	_, err = o.repo.CompleteScriptingJob(
		ctx,
		job.ID,
		draft.Status,
		&draft.Title,
		draft.ScriptCharCount,
		draft.Items,
		draft.Chunks,
	)
	return err
}

func (o *AudioBriefingOrchestrator) loadJobAfterStageError(ctx context.Context, userID string, jobID string, stageErr error) (*model.AudioBriefingJob, error) {
	job, err := o.repo.GetJobByID(ctx, userID, jobID)
	if err == nil {
		if strings.TrimSpace(job.Status) == "failed" {
			return job, nil
		}
	}
	return nil, stageErr
}

func (o *AudioBriefingOrchestrator) buildDraft(
	ctx context.Context,
	userID string,
	slotStartedAt time.Time,
	persona string,
	items []model.AudioBriefingJobItem,
	voice *model.AudioBriefingPersonaVoice,
	targetDurationMinutes int,
) (AudioBriefingDraft, error) {
	targetChars := AudioBriefingTargetChars(targetDurationMinutes)
	if voice == nil {
		return AudioBriefingDraft{}, fmt.Errorf("audio briefing voice is not configured")
	}
	if len(items) == 0 {
		return BuildAudioBriefingDraft(slotStartedAt, persona, items, voice, targetChars), nil
	}
	if o.settingsRepo == nil || o.worker == nil || o.cipher == nil {
		return AudioBriefingDraft{}, fmt.Errorf("audio briefing script dependencies are unavailable")
	}

	settings, err := o.settingsRepo.EnsureDefaults(ctx, userID)
	if err != nil {
		return AudioBriefingDraft{}, err
	}
	modelName := resolveAudioBriefingScriptModel(settings)
	if modelName == nil {
		return AudioBriefingDraft{}, fmt.Errorf("audio briefing script model is not configured")
	}

	anthropicKey, _ := loadAndDecryptAudioBriefingUserSecret(ctx, o.settingsRepo.GetAnthropicAPIKeyEncrypted, o.cipher, userID, "")
	googleKey, _ := loadAndDecryptAudioBriefingUserSecret(ctx, o.settingsRepo.GetGoogleAPIKeyEncrypted, o.cipher, userID, "")
	groqKey, _ := loadAndDecryptAudioBriefingUserSecret(ctx, o.settingsRepo.GetGroqAPIKeyEncrypted, o.cipher, userID, "")
	fireworksKey, _ := loadAndDecryptAudioBriefingUserSecret(ctx, o.settingsRepo.GetFireworksAPIKeyEncrypted, o.cipher, userID, "")
	deepseekKey, _ := loadAndDecryptAudioBriefingUserSecret(ctx, o.settingsRepo.GetDeepSeekAPIKeyEncrypted, o.cipher, userID, "")
	alibabaKey, _ := loadAndDecryptAudioBriefingUserSecret(ctx, o.settingsRepo.GetAlibabaAPIKeyEncrypted, o.cipher, userID, "")
	mistralKey, _ := loadAndDecryptAudioBriefingUserSecret(ctx, o.settingsRepo.GetMistralAPIKeyEncrypted, o.cipher, userID, "")
	xaiKey, _ := loadAndDecryptAudioBriefingUserSecret(ctx, o.settingsRepo.GetXAIAPIKeyEncrypted, o.cipher, userID, "")
	zaiKey, _ := loadAndDecryptAudioBriefingUserSecret(ctx, o.settingsRepo.GetZAIAPIKeyEncrypted, o.cipher, userID, "")
	openRouterKey, _ := loadAndDecryptAudioBriefingUserSecret(ctx, o.settingsRepo.GetOpenRouterAPIKeyEncrypted, o.cipher, userID, "")
	poeKey, _ := loadAndDecryptAudioBriefingUserSecret(ctx, o.settingsRepo.GetPoeAPIKeyEncrypted, o.cipher, userID, "")
	openAIKey, _ := loadAndDecryptAudioBriefingUserSecret(ctx, o.settingsRepo.GetOpenAIAPIKeyEncrypted, o.cipher, userID, "")
	switch LLMProviderForModel(modelName) {
	case "openrouter":
		openAIKey = openRouterKey
	case "poe":
		openAIKey = poeKey
	}

	normalizedPersona := normalizeAudioBriefingPersona(persona)
	introContext := buildAudioBriefingIntroContext(slotStartedAt)
	workerArticles := make([]AudioBriefingScriptArticle, 0, len(items))
	for _, item := range items {
		var publishedAt *string
		if item.PublishedAt != nil {
			value := item.PublishedAt.Format(time.RFC3339)
			publishedAt = &value
		}
		workerArticles = append(workerArticles, AudioBriefingScriptArticle{
			ItemID:          item.ItemID,
			Title:           item.Title,
			TranslatedTitle: item.TranslatedTitle,
			SourceTitle:     item.SourceTitle,
			Summary:         strings.TrimSpace(serviceAudioBriefingCoalesceTitle(item.SummarySnapshot, item.SegmentTitle)),
			PublishedAt:     publishedAt,
		})
	}

	narration := AudioBriefingNarration{
		Articles: make(map[string]AudioBriefingNarrationArticle, len(items)),
	}
	if len(workerArticles) > 0 {
		workerCtx := WithWorkerTraceMetadata(ctx, "audio_briefing_script", &userID, nil, nil, nil)
		callScriptWorker := func(batch []AudioBriefingScriptArticle, batchTargetChars int, includeOpening, includeOverallSummary, includeArticleSegments, includeEnding bool) (*AudioBriefingScriptResponse, error) {
			resp, err := o.worker.GenerateAudioBriefingScriptWithModel(
				workerCtx,
				normalizedPersona,
				batch,
				introContext,
				anthropicKey,
				googleKey,
				groqKey,
				deepseekKey,
				alibabaKey,
				mistralKey,
				xaiKey,
				zaiKey,
				fireworksKey,
				openAIKey,
				modelName,
				targetDurationMinutes,
				batchTargetChars,
				audioBriefingCharsPerMinute,
				includeOpening,
				includeOverallSummary,
				includeArticleSegments,
				includeEnding,
			)
			if err != nil {
				return nil, err
			}
			recordAudioBriefingLLMUsage(ctx, o.llmUsageRepo, o.cache, "audio_briefing_script", resp.LLM, &userID)
			return resp, nil
		}

		frameResp, err := callScriptWorker(
			workerArticles,
			audioBriefingFrameTargetChars(targetChars),
			true,
			true,
			false,
			true,
		)
		if err != nil {
			return AudioBriefingDraft{}, err
		}
		narration.Opening = strings.TrimSpace(frameResp.Opening)
		narration.OverallSummary = strings.TrimSpace(frameResp.OverallSummary)
		narration.Ending = strings.TrimSpace(frameResp.Ending)

		batchSize := audioBriefingArticleBatchSize(len(workerArticles))
		for start := 0; start < len(workerArticles); start += batchSize {
			end := start + batchSize
			if end > len(workerArticles) {
				end = len(workerArticles)
			}
			batch := workerArticles[start:end]
			result, err := audioBriefingGenerateArticleSegmentsBatch(
				batch,
				targetChars,
				len(workerArticles),
				callScriptWorker,
			)
			if err != nil {
				return AudioBriefingDraft{}, err
			}
			for _, recovered := range result.RecoveredFailures {
				log.Printf("audio briefing script recovered user=%s persona=%s model=%s batch_size=%d err=%v", userID, normalizedPersona, strings.TrimSpace(*modelName), recovered.BatchSize, recovered.Err)
			}
			for _, segment := range result.Segments {
				narration.Articles[segment.ItemID] = AudioBriefingNarrationArticle{
					Headline:   strings.TrimSpace(segment.Headline),
					Commentary: strings.TrimSpace(segment.Commentary),
				}
			}
		}
	}

	return BuildAudioBriefingDraftFromNarration(slotStartedAt, normalizedPersona, items, voice, narration, targetChars), nil
}

func audioBriefingFrameTargetChars(targetChars int) int {
	return audioBriefingOpeningBudget(targetChars) + audioBriefingSummaryBudget(targetChars) + audioBriefingEndingBudget(targetChars)
}

func audioBriefingArticleBatchTargetChars(targetChars, totalArticles, batchArticles int) int {
	if batchArticles <= 0 {
		return 0
	}
	perArticle := audioBriefingCommentaryBudget(targetChars, totalArticles)
	target := perArticle*batchArticles + batchArticles*40
	minimum := batchArticles * 500
	if target < minimum {
		return minimum
	}
	return target
}

func audioBriefingArticleBatchSize(itemCount int) int {
	if itemCount <= 0 {
		return 1
	}
	if itemCount < 4 {
		return itemCount
	}
	return 4
}

func audioBriefingGenerateArticleSegmentsBatch(
	batch []AudioBriefingScriptArticle,
	targetChars int,
	totalArticles int,
	call func(batch []AudioBriefingScriptArticle, batchTargetChars int, includeOpening, includeOverallSummary, includeArticleSegments, includeEnding bool) (*AudioBriefingScriptResponse, error),
) (audioBriefingArticleBatchResult, error) {
	resp, err := call(
		batch,
		audioBriefingArticleBatchTargetChars(targetChars, totalArticles, len(batch)),
		false,
		false,
		true,
		false,
	)
	if err == nil {
		return audioBriefingArticleBatchResult{Segments: resp.ArticleSegments}, nil
	}
	if len(batch) <= 1 {
		return audioBriefingArticleBatchResult{}, err
	}

	mid := len(batch) / 2
	left, leftErr := audioBriefingGenerateArticleSegmentsBatch(batch[:mid], targetChars, totalArticles, call)
	if leftErr != nil {
		return audioBriefingArticleBatchResult{}, leftErr
	}
	right, rightErr := audioBriefingGenerateArticleSegmentsBatch(batch[mid:], targetChars, totalArticles, call)
	if rightErr != nil {
		return audioBriefingArticleBatchResult{}, rightErr
	}
	recovered := make([]audioBriefingRecoveredFailure, 0, 1+len(left.RecoveredFailures)+len(right.RecoveredFailures))
	recovered = append(recovered, audioBriefingRecoveredFailure{BatchSize: len(batch), Err: err})
	recovered = append(recovered, left.RecoveredFailures...)
	recovered = append(recovered, right.RecoveredFailures...)
	return audioBriefingArticleBatchResult{
		Segments:          append(left.Segments, right.Segments...),
		RecoveredFailures: recovered,
	}, nil
}

func audioBriefingNextPipelineStage(job *model.AudioBriefingJob, chunks []model.AudioBriefingScriptChunk) (audioBriefingPipelineStage, error) {
	if job == nil {
		return audioBriefingPipelineStageNone, repository.ErrNotFound
	}
	switch strings.TrimSpace(job.Status) {
	case "pending":
		return audioBriefingPipelineStageScript, nil
	case "scripted":
		return audioBriefingPipelineStageVoice, nil
	case "voiced":
		return audioBriefingPipelineStageConcat, nil
	case "failed":
		if len(chunks) == 0 {
			return audioBriefingPipelineStageScript, nil
		}
		if audioBriefingHasIncompleteChunks(chunks) {
			return audioBriefingPipelineStageVoice, nil
		}
		return audioBriefingPipelineStageConcat, nil
	default:
		return audioBriefingPipelineStageNone, repository.ErrInvalidState
	}
}

func audioBriefingHasIncompleteChunks(chunks []model.AudioBriefingScriptChunk) bool {
	for _, chunk := range chunks {
		if strings.TrimSpace(chunk.TTSStatus) != "generated" {
			return true
		}
		if chunk.R2AudioObjectKey == nil || strings.TrimSpace(*chunk.R2AudioObjectKey) == "" {
			return true
		}
	}
	return false
}

func audioBriefingShouldContinue(status string) bool {
	switch strings.TrimSpace(status) {
	case "pending", "scripted", "voiced", "failed":
		return true
	default:
		return false
	}
}

func normalizeAudioBriefingPersona(v string) string {
	switch strings.TrimSpace(v) {
	case "editor", "hype", "analyst", "concierge", "snark", "native":
		return strings.TrimSpace(v)
	default:
		return "editor"
	}
}

func resolveAudioBriefingScriptModel(settings *model.UserSettings) *string {
	if settings == nil {
		return nil
	}
	if modelName := chooseAudioBriefingModelOverride(settings.AudioBriefingScriptModel, settings); modelName != nil {
		return modelName
	}
	if modelName := chooseAudioBriefingModelOverride(settings.AudioBriefingScriptFallbackModel, settings); modelName != nil {
		return modelName
	}
	for _, provider := range CostEfficientLLMProviders("") {
		if !hasAudioBriefingProviderKey(settings, provider) {
			continue
		}
		v := strings.TrimSpace(DefaultLLMModelForPurpose(provider, "summary"))
		if v == "" {
			continue
		}
		return &v
	}
	return nil
}

func chooseAudioBriefingModelOverride(modelName *string, settings *model.UserSettings) *string {
	if modelName == nil || settings == nil {
		return nil
	}
	v := strings.TrimSpace(*modelName)
	if v == "" {
		return nil
	}
	if !hasAudioBriefingProviderKey(settings, LLMProviderForModel(&v)) {
		return nil
	}
	return &v
}

func hasAudioBriefingProviderKey(settings *model.UserSettings, provider string) bool {
	if settings == nil {
		return false
	}
	switch strings.TrimSpace(provider) {
	case "google":
		return settings.HasGoogleAPIKey
	case "groq":
		return settings.HasGroqAPIKey
	case "deepseek":
		return settings.HasDeepSeekAPIKey
	case "alibaba":
		return settings.HasAlibabaAPIKey
	case "mistral":
		return settings.HasMistralAPIKey
	case "xai":
		return settings.HasXAIAPIKey
	case "zai":
		return settings.HasZAIAPIKey
	case "fireworks":
		return settings.HasFireworksAPIKey
	case "openai":
		return settings.HasOpenAIAPIKey
	case "openrouter":
		return settings.HasOpenRouterAPIKey
	case "poe":
		return settings.HasPoeAPIKey
	default:
		return settings.HasAnthropicAPIKey
	}
}

func buildAudioBriefingIntroContext(now time.Time) BriefingNavigatorIntroContext {
	now = now.In(timeutil.JST)
	return BriefingNavigatorIntroContext{
		NowJST:     now.Format(time.RFC3339),
		DateJST:    now.Format("2006-01-02"),
		WeekdayJST: now.Weekday().String(),
		TimeOfDay:  audioBriefingTimeOfDay(now.Hour()),
		SeasonHint: audioBriefingSeasonHint(now),
	}
}

func audioBriefingTimeOfDay(hour int) string {
	switch {
	case hour >= 5 && hour < 11:
		return "morning"
	case hour >= 11 && hour < 17:
		return "afternoon"
	case hour >= 17 && hour < 23:
		return "evening"
	default:
		return "late_night"
	}
}

func audioBriefingSeasonHint(now time.Time) string {
	month := now.Month()
	day := now.Day()
	switch month {
	case time.March:
		if day < 15 {
			return "early_spring"
		}
		return "spring"
	case time.April, time.May:
		return "spring"
	case time.June:
		if day < 10 {
			return "early_summer"
		}
		return "rainy_season"
	case time.July:
		if day < 20 {
			return "rainy_season"
		}
		return "mid_summer"
	case time.August:
		if day < 20 {
			return "mid_summer"
		}
		return "late_summer"
	case time.September, time.October:
		return "autumn"
	case time.November:
		if day < 20 {
			return "late_autumn"
		}
		return "early_winter"
	case time.December, time.January:
		return "mid_winter"
	case time.February:
		return "late_winter"
	default:
		return "seasonal"
	}
}

func loadAndDecryptAudioBriefingUserSecret(
	ctx context.Context,
	load func(context.Context, string) (*string, error),
	cipher *SecretCipher,
	userID string,
	notFoundMessage string,
) (*string, error) {
	if load == nil {
		return nil, fmt.Errorf("secret loader is not configured")
	}
	enc, err := load(ctx, userID)
	if err != nil {
		return nil, err
	}
	if enc == nil || strings.TrimSpace(*enc) == "" {
		if notFoundMessage == "" {
			return nil, nil
		}
		return nil, errors.New(notFoundMessage)
	}
	if cipher == nil || !cipher.Enabled() {
		return nil, fmt.Errorf("user secret encryption is not configured")
	}
	plain, err := cipher.DecryptString(*enc)
	if err != nil {
		return nil, err
	}
	return &plain, nil
}

func recordAudioBriefingLLMUsage(ctx context.Context, repo *repository.LLMUsageLogRepo, cache JSONCache, purpose string, usage *LLMUsage, userID *string) {
	usage = NormalizeCatalogPricedUsage(purpose, usage)
	if repo == nil || usage == nil || userID == nil || *userID == "" {
		return
	}
	sum := sha256.Sum256([]byte(fmt.Sprintf("%s|%s|%s|%s|%d|%d|%d|%d", purpose, usage.Provider, usage.Model, *userID, usage.InputTokens, usage.OutputTokens, usage.CacheCreationInputTokens, usage.CacheReadInputTokens)))
	key := hex.EncodeToString(sum[:])
	pricingSource := usage.PricingSource
	if pricingSource == "" {
		pricingSource = "unknown"
	}
	if err := repo.Insert(ctx, repository.LLMUsageLogInput{
		IdempotencyKey:           &key,
		UserID:                   userID,
		Provider:                 usage.Provider,
		Model:                    usage.Model,
		RequestedModel:           usage.RequestedModel,
		ResolvedModel:            usage.ResolvedModel,
		PricingModelFamily:       usage.PricingModelFamily,
		PricingSource:            pricingSource,
		OpenRouterCostUSD:        usage.OpenRouterCostUSD,
		OpenRouterGenerationID:   strings.TrimSpace(usage.OpenRouterGenerationID),
		Purpose:                  purpose,
		InputTokens:              usage.InputTokens,
		OutputTokens:             usage.OutputTokens,
		CacheCreationInputTokens: usage.CacheCreationInputTokens,
		CacheReadInputTokens:     usage.CacheReadInputTokens,
		EstimatedCostUSD:         usage.EstimatedCostUSD,
	}); err == nil {
		_ = BumpUserLLMUsageCacheVersion(ctx, cache, *userID)
	} else {
		log.Printf("llm usage insert failed purpose=%s user_id=%s provider=%s model=%s err=%v", purpose, *userID, usage.Provider, usage.Model, err)
	}
}

func serviceAudioBriefingSegmentTitle(item model.AudioBriefingJobItem) string {
	if title := strings.TrimSpace(serviceAudioBriefingCoalesceTitle(item.TranslatedTitle, item.Title)); title != "" {
		return title
	}
	return "無題"
}

func serviceAudioBriefingCoalesceTitle(values ...*string) string {
	for _, value := range values {
		if value == nil {
			continue
		}
		if trimmed := strings.TrimSpace(*value); trimmed != "" {
			return trimmed
		}
	}
	return ""
}
