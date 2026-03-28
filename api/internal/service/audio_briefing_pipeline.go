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
	ScriptLLMModels   []string
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
	recentPersonas, err := o.repo.ListRecentPersonasByUser(ctx, userID, 3)
	if err != nil {
		return nil, err
	}
	return o.createPendingJob(ctx, userID, settings, now, AudioBriefingManualSlotKeyAt(now), ResolvePersonaAvoidRecent(settings.DefaultPersonaMode, settings.DefaultPersona, recentPersonas))
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

	recentPersonas, err := o.repo.ListRecentPersonasByUser(ctx, userID, 3)
	if err != nil {
		return nil, err
	}
	job, err := o.createPendingJob(ctx, userID, settings, slotStartedAt, slotKey, ResolvePersonaAvoidRecent(settings.DefaultPersonaMode, settings.DefaultPersona, recentPersonas))
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
	if !AudioBriefingResumeAllowed(job) {
		return nil, repository.ErrInvalidState
	}
	return job, nil
}

func (o *AudioBriefingOrchestrator) RunPipeline(ctx context.Context, userID string, jobID string) (*model.AudioBriefingJob, error) {
	job, _, err := o.RunPipelineStep(ctx, userID, jobID)
	return job, err
}

func (o *AudioBriefingOrchestrator) RunPipelineStep(ctx context.Context, userID string, jobID string) (*model.AudioBriefingJob, bool, error) {
	if o == nil || o.repo == nil {
		return nil, false, fmt.Errorf("audio briefing orchestrator unavailable")
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

func (o *AudioBriefingOrchestrator) continuePipeline(ctx context.Context, userID string, jobID string) (*model.AudioBriefingJob, bool, error) {
	job, err := o.repo.GetJobByID(ctx, userID, jobID)
	if err != nil {
		return nil, false, err
	}
	chunks, err := o.repo.ListJobChunks(ctx, userID, jobID)
	if err != nil {
		return nil, false, err
	}
	stage, err := audioBriefingNextPipelineStage(job, chunks)
	if err != nil {
		if audioBriefingShouldContinue(job.Status) {
			return nil, false, err
		}
		return job, false, nil
	}

	switch stage {
	case audioBriefingPipelineStageScript:
		if err := o.runStageWithRecovery(ctx, userID, jobID, audioBriefingPipelineStageScript, func() error {
			return o.runScriptingStage(ctx, job)
		}); err != nil {
			nextJob, nextErr := o.loadJobAfterStageError(ctx, userID, jobID, audioBriefingPipelineStageScript, err)
			return nextJob, false, nextErr
		}
		return o.continuePipeline(ctx, userID, jobID)
	case audioBriefingPipelineStageVoice:
		if o.voiceRunner == nil {
			return nil, false, fmt.Errorf("audio briefing voice runner unavailable")
		}
		var voiceResult *AudioBriefingVoiceRunResult
		if err := o.runStageWithRecovery(ctx, userID, jobID, audioBriefingPipelineStageVoice, func() error {
			var stageErr error
			voiceResult, stageErr = o.voiceRunner.Start(ctx, userID, jobID)
			return stageErr
		}); err != nil {
			nextJob, nextErr := o.loadJobAfterStageError(ctx, userID, jobID, audioBriefingPipelineStageVoice, err)
			return nextJob, false, nextErr
		}
		if voiceResult != nil && voiceResult.Completed {
			return o.continuePipeline(ctx, userID, jobID)
		}
		nextJob, err := o.repo.GetJobByID(ctx, userID, jobID)
		if err != nil {
			return nil, false, err
		}
		if voiceResult != nil && voiceResult.ProcessedChunk {
			return nextJob, true, nil
		}
		return nextJob, false, nil
	case audioBriefingPipelineStageConcat:
		if o.concatStarter == nil {
			return nil, false, fmt.Errorf("audio briefing concat starter unavailable")
		}
		if err := o.concatStarter.Start(ctx, userID, jobID); err != nil {
			nextJob, nextErr := o.loadJobAfterStageError(ctx, userID, jobID, audioBriefingPipelineStageConcat, err)
			return nextJob, false, nextErr
		}
		nextJob, err := o.repo.GetJobByID(ctx, userID, jobID)
		return nextJob, false, err
	default:
		return job, false, nil
	}
}

func (o *AudioBriefingOrchestrator) runStageWithRecovery(ctx context.Context, userID string, jobID string, stage audioBriefingPipelineStage, run func() error) (err error) {
	defer func() {
		if recovered := recover(); recovered != nil {
			err = fmt.Errorf("audio briefing %s stage panic: %v", stage, recovered)
		}
	}()
	if run == nil {
		return nil
	}
	return run()
}

func (o *AudioBriefingOrchestrator) runScriptingStage(ctx context.Context, job *model.AudioBriefingJob) (err error) {
	if job == nil {
		return repository.ErrNotFound
	}
	defer func() {
		if recovered := recover(); recovered != nil {
			err = fmt.Errorf("audio briefing script stage panic: %v", recovered)
		}
		if err != nil {
			_, _ = o.repo.FailScriptingJob(ctx, job.ID, "script_failed", err.Error())
		}
	}()
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
		audioBriefingScriptModelsValue(draft.ScriptLLMModels),
		draft.Items,
		draft.Chunks,
	)
	return err
}

func (o *AudioBriefingOrchestrator) loadJobAfterStageError(ctx context.Context, userID string, jobID string, stage audioBriefingPipelineStage, stageErr error) (*model.AudioBriefingJob, error) {
	recoveryCtx, cancel := audioBriefingFailureContext(ctx)
	defer cancel()
	job, err := o.repo.GetJobByID(recoveryCtx, userID, jobID)
	if err == nil {
		nextJob, recoverErr := recoverAudioBriefingStageError(stage, job, stageErr, func(errorCode, errorMessage string) (*model.AudioBriefingJob, error) {
			switch stage {
			case audioBriefingPipelineStageScript:
				return o.repo.FailScriptingJob(recoveryCtx, jobID, errorCode, errorMessage)
			case audioBriefingPipelineStageVoice:
				return o.repo.FailVoicingJob(recoveryCtx, jobID, errorCode, errorMessage)
			default:
				return job, nil
			}
		})
		if recoverErr == nil && nextJob != nil {
			return nextJob, nil
		}
	}
	return nil, stageErr
}

func recoverAudioBriefingStageError(
	stage audioBriefingPipelineStage,
	job *model.AudioBriefingJob,
	stageErr error,
	fail func(errorCode, errorMessage string) (*model.AudioBriefingJob, error),
) (*model.AudioBriefingJob, error) {
	if job == nil {
		return nil, stageErr
	}
	status := strings.TrimSpace(job.Status)
	if status == "failed" {
		return job, nil
	}
	errorCode, activeStatus := audioBriefingStageFailureFallback(stage)
	if activeStatus == "" || status != activeStatus || fail == nil {
		return nil, stageErr
	}
	errorMessage := ""
	if stageErr != nil {
		errorMessage = stageErr.Error()
	}
	failedJob, err := fail(errorCode, errorMessage)
	if err != nil {
		return nil, stageErr
	}
	if failedJob != nil && strings.TrimSpace(failedJob.Status) == "failed" {
		return failedJob, nil
	}
	return nil, stageErr
}

func audioBriefingStageFailureFallback(stage audioBriefingPipelineStage) (errorCode string, activeStatus string) {
	switch stage {
	case audioBriefingPipelineStageScript:
		return "script_failed", "scripting"
	case audioBriefingPipelineStageVoice:
		return "tts_failed", "voicing"
	default:
		return "", ""
	}
}

func audioBriefingFailureContext(_ context.Context) (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), 5*time.Second)
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
	modelNames := resolveAudioBriefingScriptModels(settings)
	if len(modelNames) == 0 {
		return AudioBriefingDraft{}, fmt.Errorf("audio briefing script model is not configured")
	}

	anthropicKey, _ := loadAndDecryptAudioBriefingUserSecret(ctx, o.settingsRepo.GetAnthropicAPIKeyEncrypted, o.cipher, userID, "")
	googleKey, _ := loadAndDecryptAudioBriefingUserSecret(ctx, o.settingsRepo.GetGoogleAPIKeyEncrypted, o.cipher, userID, "")
	groqKey, _ := loadAndDecryptAudioBriefingUserSecret(ctx, o.settingsRepo.GetGroqAPIKeyEncrypted, o.cipher, userID, "")
	fireworksKey, _ := loadAndDecryptAudioBriefingUserSecret(ctx, o.settingsRepo.GetFireworksAPIKeyEncrypted, o.cipher, userID, "")
	deepseekKey, _ := loadAndDecryptAudioBriefingUserSecret(ctx, o.settingsRepo.GetDeepSeekAPIKeyEncrypted, o.cipher, userID, "")
	alibabaKey, _ := loadAndDecryptAudioBriefingUserSecret(ctx, o.settingsRepo.GetAlibabaAPIKeyEncrypted, o.cipher, userID, "")
	mistralKey, _ := loadAndDecryptAudioBriefingUserSecret(ctx, o.settingsRepo.GetMistralAPIKeyEncrypted, o.cipher, userID, "")
	moonshotKey, _ := loadAndDecryptAudioBriefingUserSecret(ctx, o.settingsRepo.GetMoonshotAPIKeyEncrypted, o.cipher, userID, "")
	xaiKey, _ := loadAndDecryptAudioBriefingUserSecret(ctx, o.settingsRepo.GetXAIAPIKeyEncrypted, o.cipher, userID, "")
	zaiKey, _ := loadAndDecryptAudioBriefingUserSecret(ctx, o.settingsRepo.GetZAIAPIKeyEncrypted, o.cipher, userID, "")
	openRouterKey, _ := loadAndDecryptAudioBriefingUserSecret(ctx, o.settingsRepo.GetOpenRouterAPIKeyEncrypted, o.cipher, userID, "")
	poeKey, _ := loadAndDecryptAudioBriefingUserSecret(ctx, o.settingsRepo.GetPoeAPIKeyEncrypted, o.cipher, userID, "")
	openAIKey, _ := loadAndDecryptAudioBriefingUserSecret(ctx, o.settingsRepo.GetOpenAIAPIKeyEncrypted, o.cipher, userID, "")

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
	scriptLLMModels := make([]string, 0, 2)
	if len(workerArticles) > 0 {
		workerCtx := WithWorkerTraceMetadata(ctx, "audio_briefing_script", &userID, nil, nil, nil)
		callScriptWorker := func(batch []AudioBriefingScriptArticle, batchTargetChars int, includeOpening, includeOverallSummary, includeArticleSegments, includeEnding bool) (*AudioBriefingScriptResponse, error) {
			var errs []string
			for idx, modelName := range modelNames {
				modelValue := modelName
				effectiveOpenAIKey := openAIKey
				switch LLMProviderForModel(&modelValue) {
				case "openrouter":
					effectiveOpenAIKey = openRouterKey
				case "moonshot":
					effectiveOpenAIKey = moonshotKey
				case "poe":
					effectiveOpenAIKey = poeKey
				}
				resp, err := generateAudioBriefingScriptWithRetry(workerCtx, func(callCtx context.Context) (*AudioBriefingScriptResponse, error) {
					return o.worker.GenerateAudioBriefingScriptWithModel(
						callCtx,
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
						effectiveOpenAIKey,
						&modelValue,
						targetDurationMinutes,
						batchTargetChars,
						audioBriefingCharsPerMinute,
						includeOpening,
						includeOverallSummary,
						includeArticleSegments,
						includeEnding,
					)
				})
				if err == nil {
					if idx > 0 {
						log.Printf("audio briefing script fallback succeeded user=%s persona=%s model=%s batch_size=%d", userID, normalizedPersona, modelValue, len(batch))
					}
					recordAudioBriefingLLMUsage(ctx, o.llmUsageRepo, o.cache, "audio_briefing_script", resp.LLM, &userID)
					return resp, nil
				}
				errs = append(errs, fmt.Sprintf("%s: %v", modelValue, err))
				if idx < len(modelNames)-1 {
					log.Printf("audio briefing script fallback retrying user=%s persona=%s model=%s batch_size=%d err=%v", userID, normalizedPersona, modelValue, len(batch), err)
				}
			}
			return nil, fmt.Errorf("audio briefing script failed across models: %s", strings.Join(errs, " | "))
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
		scriptLLMModels = appendAudioBriefingScriptModel(scriptLLMModels, frameResp.LLM)
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
			scriptLLMModels = appendAudioBriefingScriptModels(scriptLLMModels, result.ScriptLLMModels)
			for _, recovered := range result.RecoveredFailures {
				log.Printf("audio briefing script recovered user=%s persona=%s models=%s batch_size=%d err=%v", userID, normalizedPersona, strings.Join(modelNames, ","), recovered.BatchSize, recovered.Err)
			}
			for _, segment := range result.Segments {
				narration.Articles[segment.ItemID] = AudioBriefingNarrationArticle{
					Headline:   strings.TrimSpace(segment.Headline),
					Commentary: strings.TrimSpace(segment.Commentary),
				}
			}
		}
	}

	draft := BuildAudioBriefingDraftFromNarration(slotStartedAt, normalizedPersona, items, voice, narration, targetChars)
	draft.ScriptLLMModels = scriptLLMModels
	return draft, nil
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
		return audioBriefingArticleBatchResult{
			Segments:        resp.ArticleSegments,
			ScriptLLMModels: appendAudioBriefingScriptModel(nil, resp.LLM),
		}, nil
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
		ScriptLLMModels:   appendAudioBriefingScriptModels(left.ScriptLLMModels, right.ScriptLLMModels),
	}, nil
}

func appendAudioBriefingScriptModel(models []string, llm *LLMUsage) []string {
	if llm == nil {
		return models
	}
	name := strings.TrimSpace(llm.ResolvedModel)
	if name == "" {
		name = strings.TrimSpace(llm.Model)
	}
	if name == "" {
		name = strings.TrimSpace(llm.RequestedModel)
	}
	if name == "" {
		return models
	}
	for _, existing := range models {
		if existing == name {
			return models
		}
	}
	return append(models, name)
}

func appendAudioBriefingScriptModels(dst []string, src []string) []string {
	for _, modelName := range src {
		name := strings.TrimSpace(modelName)
		if name == "" {
			continue
		}
		found := false
		for _, existing := range dst {
			if existing == name {
				found = true
				break
			}
		}
		if !found {
			dst = append(dst, name)
		}
	}
	return dst
}

func audioBriefingScriptModelsValue(models []string) *string {
	if len(models) == 0 {
		return nil
	}
	value := strings.TrimSpace(strings.Join(models, ", "))
	if value == "" {
		return nil
	}
	return &value
}

func audioBriefingNextPipelineStage(job *model.AudioBriefingJob, chunks []model.AudioBriefingScriptChunk) (audioBriefingPipelineStage, error) {
	if job == nil {
		return audioBriefingPipelineStageNone, repository.ErrNotFound
	}
	switch strings.TrimSpace(job.Status) {
	case "pending":
		return audioBriefingPipelineStageScript, nil
	case "scripting":
		return audioBriefingPipelineStageScript, nil
	case "scripted":
		return audioBriefingPipelineStageVoice, nil
	case "voicing":
		return audioBriefingPipelineStageVoice, nil
	case "voiced":
		return audioBriefingPipelineStageConcat, nil
	case "concatenating":
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
	case "pending", "scripting", "scripted", "voicing", "voiced", "concatenating", "failed":
		return true
	default:
		return false
	}
}

func audioBriefingJobCanBeResumedAt(job *model.AudioBriefingJob, now time.Time, staleAfter time.Duration) bool {
	if job == nil {
		return false
	}
	switch strings.TrimSpace(job.Status) {
	case "pending", "scripted", "voiced", "failed":
		return true
	}
	switch strings.TrimSpace(job.Status) {
	case "scripting", "voicing", "concatenating":
		if staleAfter <= 0 {
			return false
		}
		return !job.UpdatedAt.IsZero() && now.Sub(job.UpdatedAt) >= staleAfter
	default:
		return false
	}
}

func AudioBriefingResumeAllowed(job *model.AudioBriefingJob) bool {
	return audioBriefingJobCanBeResumedAt(job, time.Now(), audioBriefingStaleDeleteAfter())
}

func normalizeAudioBriefingPersona(v string) string {
	switch strings.TrimSpace(v) {
	case "editor", "hype", "analyst", "concierge", "snark", "native", "junior", "urban":
		return strings.TrimSpace(v)
	default:
		return "editor"
	}
}

func resolveAudioBriefingScriptModels(settings *model.UserSettings) []string {
	out := make([]string, 0, 2)
	appendIfValid := func(modelName *string) {
		v := strings.TrimSpace(derefString(modelName))
		if v == "" {
			return
		}
		for _, existing := range out {
			if existing == v {
				return
			}
		}
		out = append(out, v)
	}
	if settings == nil {
		return out
	}
	appendIfValid(chooseAudioBriefingModelOverride(settings.AudioBriefingScriptModel, settings))
	appendIfValid(chooseAudioBriefingModelOverride(settings.AudioBriefingScriptFallbackModel, settings))
	if len(out) > 0 {
		return out
	}
	for _, provider := range CostEfficientLLMProviders("") {
		if !hasAudioBriefingProviderKey(settings, provider) {
			continue
		}
		v := strings.TrimSpace(DefaultLLMModelForPurpose(provider, "summary"))
		if v == "" {
			continue
		}
		out = append(out, v)
		break
	}
	return out
}

func generateAudioBriefingScriptWithRetry(
	ctx context.Context,
	call func(callCtx context.Context) (*AudioBriefingScriptResponse, error),
) (*AudioBriefingScriptResponse, error) {
	var lastErr error
	for attempt := 0; attempt < 3; attempt++ {
		resp, err := call(ctx)
		if err == nil {
			return resp, nil
		}
		lastErr = err
		if !isRetryableAudioBriefingScriptWorkerError(err) || attempt >= 2 {
			break
		}
		delay := time.Duration(attempt+1) * time.Second
		log.Printf("audio briefing script retrying attempt=%d delay=%s err=%v", attempt+2, delay, err)
		select {
		case <-ctx.Done():
			return nil, err
		case <-time.After(delay):
		}
	}
	return nil, lastErr
}

func isRetryableAudioBriefingScriptWorkerError(err error) bool {
	if err == nil {
		return false
	}
	message := strings.ToLower(err.Error())
	if strings.Contains(message, "worker /audio-briefing-script: status 429") ||
		strings.Contains(message, "worker /audio-briefing-script: status 500") ||
		strings.Contains(message, "worker /audio-briefing-script: status 502") ||
		strings.Contains(message, "worker /audio-briefing-script: status 503") ||
		strings.Contains(message, "worker /audio-briefing-script: status 504") {
		return true
	}
	if strings.Contains(message, "client.timeout exceeded") ||
		strings.Contains(message, "context deadline exceeded") ||
		strings.Contains(message, "request canceled") ||
		strings.Contains(message, "connection reset by peer") ||
		strings.Contains(message, "server disconnected without sending a response") {
		return true
	}
	return false
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
	case "moonshot":
		return settings.HasMoonshotAPIKey
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
