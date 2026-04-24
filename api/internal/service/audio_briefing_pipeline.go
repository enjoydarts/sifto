package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"log"
	"math"
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
	repo           *repository.AudioBriefingRepo
	settingsRepo   *repository.UserSettingsRepo
	llmUsageRepo   *repository.LLMUsageLogRepo
	promptResolver *PromptResolver
	cipher         *SecretCipher
	worker         *WorkerClient
	cache          JSONCache
	voiceRunner    *AudioBriefingVoiceRunner
	concatStarter  *AudioBriefingConcatStarter
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

type audioBriefingTurnBatchResult struct {
	Turns             []AudioBriefingScriptTurn
	RecoveredFailures []audioBriefingRecoveredFailure
	ScriptLLMModels   []string
}

func selectAudioBriefingOpenAICompatibleKey(
	provider string,
	openAIKey, openRouterKey, togetherKey, moonshotKey, minimaxKey, xiaomiMiMoTokenPlanKey, poeKey, siliconFlowKey, featherlessKey, deepinfraKey, cerebrasKey *string,
) *string {
	switch provider {
	case "openrouter":
		return openRouterKey
	case "together":
		return togetherKey
	case "moonshot":
		return moonshotKey
	case "minimax":
		return minimaxKey
	case "xiaomi_mimo_token_plan":
		return xiaomiMiMoTokenPlanKey
	case "poe":
		return poeKey
	case "siliconflow":
		return siliconFlowKey
	case "featherless":
		return featherlessKey
	case "deepinfra":
		return deepinfraKey
	case "cerebras":
		return cerebrasKey
	default:
		return openAIKey
	}
}

func NewAudioBriefingOrchestrator(
	repo *repository.AudioBriefingRepo,
	settingsRepo *repository.UserSettingsRepo,
	llmUsageRepo *repository.LLMUsageLogRepo,
	promptResolver *PromptResolver,
	cipher *SecretCipher,
	worker *WorkerClient,
	cache JSONCache,
	voiceRunner *AudioBriefingVoiceRunner,
	concatStarter *AudioBriefingConcatStarter,
) *AudioBriefingOrchestrator {
	return &AudioBriefingOrchestrator{
		repo:           repo,
		settingsRepo:   settingsRepo,
		llmUsageRepo:   llmUsageRepo,
		promptResolver: promptResolver,
		cipher:         cipher,
		worker:         worker,
		cache:          cache,
		voiceRunner:    voiceRunner,
		concatStarter:  concatStarter,
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
	scheduleMode := NormalizeAudioBriefingScheduleMode(settings.ScheduleMode)
	slotStartedAt := AudioBriefingSlotStartAtForSchedule(now, scheduleMode, settings.IntervalHours)
	slotKey := AudioBriefingSlotKeyAtForSchedule(now, scheduleMode, settings.IntervalHours)
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
	mode := normalizeAudioBriefingConversationModeValue(settings.ConversationMode)
	return o.repo.CreatePendingJob(ctx, userID, slotStartedAt, slotKey, persona, mode, audioBriefingInitialPipelineStageForMode(mode))
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
	scheduleMode := NormalizeAudioBriefingScheduleMode(settings.ScheduleMode)
	windowStart := AudioBriefingPreviousSlotStartAtForSchedule(job.SlotStartedAtJST, scheduleMode, settings.IntervalHours)
	items, err := o.repo.ListCandidateItems(ctx, job.UserID, windowStart, settings.ArticlesPerEpisode)
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
	draft, err := o.draftStrategy(job.ConversationMode).BuildDraft(ctx, job, items, voice, settings.TargetDurationMinutes)
	if err != nil {
		_, _ = o.repo.FailScriptingJob(ctx, job.ID, "script_failed", err.Error())
		return err
	}
	_, err = o.repo.CompleteScriptingJob(
		ctx,
		job.ID,
		draft.Status,
		&draft.Title,
		draft.ErrorMessage,
		draft.ScriptCharCount,
		audioBriefingScriptModelsValue(draft.ScriptLLMModels),
		repository.AudioBriefingPromptMetadata{
			PromptKey:             draft.PromptKey,
			PromptSource:          draft.PromptSource,
			PromptVersionID:       draft.PromptVersionID,
			PromptVersionNumber:   draft.PromptVersionNumber,
			PromptExperimentID:    draft.PromptExperimentID,
			PromptExperimentArmID: draft.PromptExperimentArmID,
		},
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
	return o.buildSingleDraft(ctx, userID, slotStartedAt, persona, items, voice, targetDurationMinutes)
}

func (o *AudioBriefingOrchestrator) buildSingleDraft(
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
	briefingSettings, err := o.repo.EnsureSettingsDefaults(ctx, userID)
	if err != nil {
		return AudioBriefingDraft{}, err
	}
	if len(items) == 0 {
		return BuildAudioBriefingDraft(slotStartedAt, persona, items, voice, stringValue(briefingSettings.ProgramName), targetChars), nil
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
	togetherKey, _ := loadAndDecryptAudioBriefingUserSecret(ctx, o.settingsRepo.GetTogetherAPIKeyEncrypted, o.cipher, userID, "")
	moonshotKey, _ := loadAndDecryptAudioBriefingUserSecret(ctx, o.settingsRepo.GetMoonshotAPIKeyEncrypted, o.cipher, userID, "")
	minimaxKey, _ := loadAndDecryptAudioBriefingUserSecret(ctx, o.settingsRepo.GetMiniMaxAPIKeyEncrypted, o.cipher, userID, "")
	xiaomiMiMoTokenPlanKey, _ := loadAndDecryptAudioBriefingUserSecret(ctx, o.settingsRepo.GetXiaomiMiMoTokenPlanAPIKeyEncrypted, o.cipher, userID, "")
	xaiKey, _ := loadAndDecryptAudioBriefingUserSecret(ctx, o.settingsRepo.GetXAIAPIKeyEncrypted, o.cipher, userID, "")
	zaiKey, _ := loadAndDecryptAudioBriefingUserSecret(ctx, o.settingsRepo.GetZAIAPIKeyEncrypted, o.cipher, userID, "")
	openRouterKey, _ := loadAndDecryptAudioBriefingUserSecret(ctx, o.settingsRepo.GetOpenRouterAPIKeyEncrypted, o.cipher, userID, "")
	poeKey, _ := loadAndDecryptAudioBriefingUserSecret(ctx, o.settingsRepo.GetPoeAPIKeyEncrypted, o.cipher, userID, "")
	siliconFlowKey, _ := loadAndDecryptAudioBriefingUserSecret(ctx, o.settingsRepo.GetSiliconFlowAPIKeyEncrypted, o.cipher, userID, "")
	featherlessKey, _ := loadAndDecryptAudioBriefingUserSecret(ctx, o.settingsRepo.GetFeatherlessAPIKeyEncrypted, o.cipher, userID, "")
	deepinfraKey, _ := loadAndDecryptAudioBriefingUserSecret(ctx, o.settingsRepo.GetDeepInfraAPIKeyEncrypted, o.cipher, userID, "")
	cerebrasKey, _ := loadAndDecryptAudioBriefingUserSecret(ctx, o.settingsRepo.GetCerebrasAPIKeyEncrypted, o.cipher, userID, "")
	openAIKey, _ := loadAndDecryptAudioBriefingUserSecret(ctx, o.settingsRepo.GetOpenAIAPIKeyEncrypted, o.cipher, userID, "")

	normalizedPersona := normalizeAudioBriefingPersona(persona)
	introContext := buildAudioBriefingIntroContext(slotStartedAt, briefingSettings.ProgramName)
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
	promptResolution := ResolvePromptResolution(ctx, o.promptResolver, PromptResolveInput{
		PromptKey:      "audio_briefing_script.single",
		AssignmentUnit: "user_id",
		AssignmentKey:  userID,
		AssignmentTime: slotStartedAt,
	})
	promptConfig := WorkerPromptConfigFromResolution(promptResolution)
	if len(workerArticles) > 0 {
		workerCtx := WithWorkerTraceMetadata(ctx, "audio_briefing_script", &userID, nil, nil, nil)
		callScriptWorker := func(batch []AudioBriefingScriptArticle, introContext map[string]any, batchTargetChars int, includeOpening, includeOverallSummary, includeArticleSegments, includeEnding bool) (*AudioBriefingScriptResponse, error) {
			var errs []string
			for idx, modelName := range modelNames {
				modelValue := modelName
				effectiveOpenAIKey := selectAudioBriefingOpenAICompatibleKey(
					LLMProviderForModel(&modelValue),
					openAIKey,
					openRouterKey,
					togetherKey,
					moonshotKey,
					minimaxKey,
					xiaomiMiMoTokenPlanKey,
					poeKey,
					siliconFlowKey,
					featherlessKey,
					deepinfraKey,
					cerebrasKey,
				)
				resp, err := generateAudioBriefingScriptWithRetry(workerCtx, func(callCtx context.Context) (*AudioBriefingScriptResponse, error) {
					return o.worker.GenerateAudioBriefingScriptWithModel(
						callCtx,
						normalizedPersona,
						normalizeAudioBriefingConversationModeValue("single"),
						nil,
						nil,
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
						promptConfig,
					)
				})
				if err == nil {
					recordAudioBriefingLLMUsage(ctx, o.llmUsageRepo, o.cache, "audio_briefing_script", resp.LLM, &userID, promptResolution)
					return resp, nil
				}
				errs = append(errs, fmt.Sprintf("%s: %v", modelValue, err))
				if idx < len(modelNames)-1 {
					log.Printf("audio briefing script fallback retrying user=%s persona=%s model=%s batch_size=%d err=%v", userID, normalizedPersona, modelValue, len(batch), err)
				}
			}
			return nil, fmt.Errorf("audio briefing script failed across models: %s", strings.Join(errs, " | "))
		}
		opening, openingModels, err := audioBriefingGenerateFrameSection(
			workerArticles,
			introContext,
			targetChars,
			"opening",
			callScriptWorker,
		)
		if err != nil {
			return AudioBriefingDraft{}, err
		}
		narration.Opening = opening
		scriptLLMModels = appendAudioBriefingScriptModels(scriptLLMModels, openingModels)

		overallSummary, summaryModels, err := audioBriefingGenerateFrameSection(
			workerArticles,
			introContext,
			targetChars,
			"overall_summary",
			callScriptWorker,
		)
		if err != nil {
			return AudioBriefingDraft{}, err
		}
		narration.OverallSummary = overallSummary
		scriptLLMModels = appendAudioBriefingScriptModels(scriptLLMModels, summaryModels)

		ending, endingModels, err := audioBriefingGenerateFrameSection(
			workerArticles,
			introContext,
			targetChars,
			"ending",
			callScriptWorker,
		)
		if err != nil {
			return AudioBriefingDraft{}, err
		}
		narration.Ending = ending
		scriptLLMModels = appendAudioBriefingScriptModels(scriptLLMModels, endingModels)

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
				introContext,
			)
			if err != nil {
				return AudioBriefingDraft{}, err
			}
			scriptLLMModels = appendAudioBriefingScriptModels(scriptLLMModels, result.ScriptLLMModels)
			for _, segment := range result.Segments {
				narration.Articles[segment.ItemID] = AudioBriefingNarrationArticle{
					Headline:     strings.TrimSpace(segment.Headline),
					SummaryIntro: strings.TrimSpace(segment.SummaryIntro),
					Commentary:   strings.TrimSpace(segment.Commentary),
				}
			}
		}
	}

	draft := BuildAudioBriefingDraftFromNarration(slotStartedAt, normalizedPersona, items, voice, narration, stringValue(briefingSettings.ProgramName), targetChars)
	draft.ScriptLLMModels = scriptLLMModels
	if promptResolution.PromptKey != "" {
		draft.PromptKey = &promptResolution.PromptKey
	}
	if promptResolution.PromptSource != "" {
		draft.PromptSource = &promptResolution.PromptSource
	}
	draft.PromptVersionID = promptResolution.PromptVersionID
	draft.PromptVersionNumber = promptResolution.PromptVersionNumber
	draft.PromptExperimentID = promptResolution.PromptExperimentID
	draft.PromptExperimentArmID = promptResolution.PromptExperimentArmID
	return draft, nil
}

func (o *AudioBriefingOrchestrator) buildDuoDraft(
	ctx context.Context,
	job *model.AudioBriefingJob,
	items []model.AudioBriefingJobItem,
	hostVoice *model.AudioBriefingPersonaVoice,
	targetDurationMinutes int,
) (AudioBriefingDraft, error) {
	if job == nil {
		return AudioBriefingDraft{}, repository.ErrNotFound
	}
	targetChars := AudioBriefingTargetChars(targetDurationMinutes)
	if hostVoice == nil {
		return AudioBriefingDraft{}, fmt.Errorf("audio briefing host voice is not configured")
	}
	briefingSettings, err := o.repo.EnsureSettingsDefaults(ctx, job.UserID)
	if err != nil {
		return AudioBriefingDraft{}, err
	}
	if len(items) == 0 {
		return BuildAudioBriefingDraft(job.SlotStartedAtJST, job.Persona, items, hostVoice, stringValue(briefingSettings.ProgramName), targetChars), nil
	}
	if o.settingsRepo == nil || o.worker == nil || o.cipher == nil || o.repo == nil {
		return AudioBriefingDraft{}, fmt.Errorf("audio briefing duo script dependencies are unavailable")
	}

	settings, err := o.settingsRepo.EnsureDefaults(ctx, job.UserID)
	if err != nil {
		return AudioBriefingDraft{}, err
	}
	modelNames := resolveAudioBriefingScriptModels(settings)
	if len(modelNames) == 0 {
		return AudioBriefingDraft{}, fmt.Errorf("audio briefing script model is not configured")
	}

	hostPersona := normalizeAudioBriefingPersona(job.Persona)
	partnerPersona, partnerVoice, err := o.resolveAudioBriefingPartnerVoice(
		ctx,
		job,
		hostVoice,
		NormalizePersonaMode(&briefingSettings.DefaultPersonaMode) == PersonaModeRandom,
	)
	if err != nil {
		return AudioBriefingDraft{}, err
	}

	anthropicKey, _ := loadAndDecryptAudioBriefingUserSecret(ctx, o.settingsRepo.GetAnthropicAPIKeyEncrypted, o.cipher, job.UserID, "")
	googleKey, _ := loadAndDecryptAudioBriefingUserSecret(ctx, o.settingsRepo.GetGoogleAPIKeyEncrypted, o.cipher, job.UserID, "")
	groqKey, _ := loadAndDecryptAudioBriefingUserSecret(ctx, o.settingsRepo.GetGroqAPIKeyEncrypted, o.cipher, job.UserID, "")
	fireworksKey, _ := loadAndDecryptAudioBriefingUserSecret(ctx, o.settingsRepo.GetFireworksAPIKeyEncrypted, o.cipher, job.UserID, "")
	deepseekKey, _ := loadAndDecryptAudioBriefingUserSecret(ctx, o.settingsRepo.GetDeepSeekAPIKeyEncrypted, o.cipher, job.UserID, "")
	alibabaKey, _ := loadAndDecryptAudioBriefingUserSecret(ctx, o.settingsRepo.GetAlibabaAPIKeyEncrypted, o.cipher, job.UserID, "")
	mistralKey, _ := loadAndDecryptAudioBriefingUserSecret(ctx, o.settingsRepo.GetMistralAPIKeyEncrypted, o.cipher, job.UserID, "")
	togetherKey, _ := loadAndDecryptAudioBriefingUserSecret(ctx, o.settingsRepo.GetTogetherAPIKeyEncrypted, o.cipher, job.UserID, "")
	moonshotKey, _ := loadAndDecryptAudioBriefingUserSecret(ctx, o.settingsRepo.GetMoonshotAPIKeyEncrypted, o.cipher, job.UserID, "")
	minimaxKey, _ := loadAndDecryptAudioBriefingUserSecret(ctx, o.settingsRepo.GetMiniMaxAPIKeyEncrypted, o.cipher, job.UserID, "")
	xiaomiMiMoTokenPlanKey, _ := loadAndDecryptAudioBriefingUserSecret(ctx, o.settingsRepo.GetXiaomiMiMoTokenPlanAPIKeyEncrypted, o.cipher, job.UserID, "")
	xaiKey, _ := loadAndDecryptAudioBriefingUserSecret(ctx, o.settingsRepo.GetXAIAPIKeyEncrypted, o.cipher, job.UserID, "")
	zaiKey, _ := loadAndDecryptAudioBriefingUserSecret(ctx, o.settingsRepo.GetZAIAPIKeyEncrypted, o.cipher, job.UserID, "")
	openRouterKey, _ := loadAndDecryptAudioBriefingUserSecret(ctx, o.settingsRepo.GetOpenRouterAPIKeyEncrypted, o.cipher, job.UserID, "")
	poeKey, _ := loadAndDecryptAudioBriefingUserSecret(ctx, o.settingsRepo.GetPoeAPIKeyEncrypted, o.cipher, job.UserID, "")
	siliconFlowKey, _ := loadAndDecryptAudioBriefingUserSecret(ctx, o.settingsRepo.GetSiliconFlowAPIKeyEncrypted, o.cipher, job.UserID, "")
	featherlessKey, _ := loadAndDecryptAudioBriefingUserSecret(ctx, o.settingsRepo.GetFeatherlessAPIKeyEncrypted, o.cipher, job.UserID, "")
	deepinfraKey, _ := loadAndDecryptAudioBriefingUserSecret(ctx, o.settingsRepo.GetDeepInfraAPIKeyEncrypted, o.cipher, job.UserID, "")
	cerebrasKey, _ := loadAndDecryptAudioBriefingUserSecret(ctx, o.settingsRepo.GetCerebrasAPIKeyEncrypted, o.cipher, job.UserID, "")
	openAIKey, _ := loadAndDecryptAudioBriefingUserSecret(ctx, o.settingsRepo.GetOpenAIAPIKeyEncrypted, o.cipher, job.UserID, "")

	introContext := buildAudioBriefingIntroContext(job.SlotStartedAtJST, briefingSettings.ProgramName)
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

	workerCtx := WithWorkerTraceMetadata(ctx, "audio_briefing_duo_script", &job.UserID, nil, nil, nil)
	promptResolution := ResolvePromptResolution(ctx, o.promptResolver, PromptResolveInput{
		PromptKey:      "audio_briefing_script.duo",
		AssignmentUnit: "job_id",
		AssignmentKey:  job.ID,
		AssignmentTime: job.SlotStartedAtJST,
	})
	promptConfig := WorkerPromptConfigFromResolution(promptResolution)
	callScriptWorker := func(batch []AudioBriefingScriptArticle, introContext map[string]any, batchTargetChars int, includeOpening, includeOverallSummary, includeArticleSegments, includeEnding bool) (*AudioBriefingScriptResponse, error) {
		var errs []string
		for idx, modelName := range modelNames {
			modelValue := modelName
			effectiveOpenAIKey := selectAudioBriefingOpenAICompatibleKey(
				LLMProviderForModel(&modelValue),
				openAIKey,
				openRouterKey,
				togetherKey,
				moonshotKey,
				minimaxKey,
				xiaomiMiMoTokenPlanKey,
				poeKey,
				siliconFlowKey,
				featherlessKey,
				deepinfraKey,
				cerebrasKey,
			)
			resp, err := generateAudioBriefingScriptWithRetry(workerCtx, func(callCtx context.Context) (*AudioBriefingScriptResponse, error) {
				return o.worker.GenerateAudioBriefingScriptWithModel(
					callCtx,
					hostPersona,
					"duo",
					&hostPersona,
					&partnerPersona,
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
					promptConfig,
				)
			})
			if err == nil {
				recordAudioBriefingLLMUsage(ctx, o.llmUsageRepo, o.cache, "audio_briefing_script", resp.LLM, &job.UserID, promptResolution)
				return resp, nil
			}
			errs = append(errs, fmt.Sprintf("%s: %v", modelValue, err))
			if idx < len(modelNames)-1 {
				log.Printf("audio briefing duo script fallback retrying user=%s host=%s partner=%s model=%s batch_size=%d err=%v", job.UserID, hostPersona, partnerPersona, modelValue, len(batch), err)
			}
		}
		return nil, fmt.Errorf("audio briefing duo script failed across models: %s", strings.Join(errs, " | "))
	}

	allTurns := make([]AudioBriefingScriptTurn, 0, len(workerArticles)*4)
	scriptLLMModels := make([]string, 0, 4)

	openingTurns, openingModels, err := audioBriefingGenerateTurnSection(
		workerArticles,
		audioBriefingTurnSectionIntroContext(introContext, "opening"),
		targetChars,
		"opening",
		callScriptWorker,
	)
	if err != nil {
		return AudioBriefingDraft{}, err
	}
	allTurns = append(allTurns, openingTurns...)
	scriptLLMModels = appendAudioBriefingScriptModels(scriptLLMModels, openingModels)

	summaryTurns, summaryModels, err := audioBriefingGenerateTurnSection(
		workerArticles,
		audioBriefingTurnSectionIntroContext(introContext, "overall_summary"),
		targetChars,
		"overall_summary",
		callScriptWorker,
	)
	if err != nil {
		return AudioBriefingDraft{}, err
	}
	allTurns = append(allTurns, summaryTurns...)
	scriptLLMModels = appendAudioBriefingScriptModels(scriptLLMModels, summaryModels)

	batchSize := audioBriefingDuoArticleBatchSize(len(workerArticles))
	for start := 0; start < len(workerArticles); start += batchSize {
		end := start + batchSize
		if end > len(workerArticles) {
			end = len(workerArticles)
		}
		batch := workerArticles[start:end]
		result, err := audioBriefingGenerateTurnArticleBatch(
			batch,
			targetChars,
			len(workerArticles),
			callScriptWorker,
			audioBriefingTurnArticleIntroContext(introContext, start, end, len(workerArticles)),
		)
		if err != nil {
			return AudioBriefingDraft{}, err
		}
		allTurns = append(allTurns, result.Turns...)
		scriptLLMModels = appendAudioBriefingScriptModels(scriptLLMModels, result.ScriptLLMModels)
	}

	endingTurns, endingModels, err := audioBriefingGenerateTurnSection(
		workerArticles,
		audioBriefingTurnSectionIntroContext(introContext, "ending"),
		targetChars,
		"ending",
		callScriptWorker,
	)
	if err != nil {
		return AudioBriefingDraft{}, err
	}
	allTurns = append(allTurns, endingTurns...)
	scriptLLMModels = appendAudioBriefingScriptModels(scriptLLMModels, endingModels)

	draft := BuildAudioBriefingDraftFromTurns(job.SlotStartedAtJST, hostPersona, items, hostVoice, partnerVoice, allTurns, targetChars)
	draft.ScriptLLMModels = scriptLLMModels
	if promptResolution.PromptKey != "" {
		draft.PromptKey = &promptResolution.PromptKey
	}
	if promptResolution.PromptSource != "" {
		draft.PromptSource = &promptResolution.PromptSource
	}
	draft.PromptVersionID = promptResolution.PromptVersionID
	draft.PromptVersionNumber = promptResolution.PromptVersionNumber
	draft.PromptExperimentID = promptResolution.PromptExperimentID
	draft.PromptExperimentArmID = promptResolution.PromptExperimentArmID
	return draft, nil
}

func (o *AudioBriefingOrchestrator) resolveAudioBriefingPartnerVoice(ctx context.Context, job *model.AudioBriefingJob, hostVoice *model.AudioBriefingPersonaVoice, randomPartnerAllowed bool) (string, *model.AudioBriefingPersonaVoice, error) {
	if job == nil {
		return "", nil, repository.ErrNotFound
	}
	hostRequiresGemini := hostVoice != nil && strings.EqualFold(strings.TrimSpace(hostVoice.TTSProvider), "gemini_tts")
	hostRequiresFish := hostVoice != nil && strings.EqualFold(strings.TrimSpace(hostVoice.TTSProvider), "fish")
	if existing := strings.TrimSpace(derefString(job.PartnerPersona)); existing != "" {
		voice, err := o.repo.GetPersonaVoice(ctx, job.UserID, existing)
		if err != nil {
			return "", nil, err
		}
		if voice == nil {
			return "", nil, fmt.Errorf("audio briefing duo partner voice is not configured")
		}
		if hostRequiresGemini && !audioBriefingGeminiDuoReady(hostVoice, voice) {
			return "", nil, fmt.Errorf("audio briefing duo partner must use gemini_tts with the same tts model as host")
		}
		if hostRequiresFish && !audioBriefingFishDuoReady(hostVoice, voice) {
			return "", nil, fmt.Errorf("audio briefing duo partner must use fish with s2-pro and a distinct voice from host")
		}
		return normalizeAudioBriefingPersona(existing), voice, nil
	}
	if randomPartnerAllowed {
		if o == nil || o.repo == nil {
			return "", nil, fmt.Errorf("audio briefing repo unavailable")
		}
		voices, err := o.repo.ListPersonaVoicesByUser(ctx, job.UserID)
		if err != nil {
			return "", nil, err
		}
		partnerPersona, partnerVoice, ok := resolveRandomAudioBriefingPartnerCandidate(
			job.Persona,
			hostRequiresGemini,
			hostVoice,
			voices,
			randomPersonaFromCandidates,
		)
		if !ok {
			if hostRequiresGemini {
				return "", nil, fmt.Errorf("audio briefing duo random partner persona requires another gemini_tts voice with the same tts model")
			}
			if hostRequiresFish {
				return "", nil, fmt.Errorf("audio briefing duo random partner persona requires another fish voice using s2-pro")
			}
			return "", nil, fmt.Errorf("audio briefing duo random partner persona requires another configured persona voice")
		}
		if _, err := o.repo.SetPartnerPersona(ctx, job.ID, partnerPersona); err != nil {
			return "", nil, err
		}
		return partnerPersona, partnerVoice, nil
	}
	if hostRequiresGemini {
		return "", nil, fmt.Errorf("audio briefing duo partner persona must be explicitly configured for gemini_tts")
	}
	if hostRequiresFish {
		return "", nil, fmt.Errorf("audio briefing duo partner persona must be explicitly configured for fish")
	}
	return "", nil, fmt.Errorf("audio briefing duo partner persona must be explicitly configured")
}

func audioBriefingFrameTargetChars(targetChars int) int {
	return audioBriefingOpeningBudget(targetChars) + audioBriefingSummaryBudget(targetChars) + audioBriefingEndingBudget(targetChars)
}

func audioBriefingGenerateFrameSection(
	articles []AudioBriefingScriptArticle,
	introContext map[string]any,
	targetChars int,
	section string,
	call func(batch []AudioBriefingScriptArticle, introContext map[string]any, batchTargetChars int, includeOpening, includeOverallSummary, includeArticleSegments, includeEnding bool) (*AudioBriefingScriptResponse, error),
) (string, []string, error) {
	if call == nil {
		return "", nil, fmt.Errorf("audio briefing script caller is unavailable")
	}
	sectionTarget := audioBriefingFrameSectionTargetChars(targetChars, section)
	includeOpening := section == "opening"
	includeOverallSummary := section == "overall_summary"
	includeEnding := section == "ending"

	resp, err := call(articles, introContext, sectionTarget, includeOpening, includeOverallSummary, false, includeEnding)
	if err != nil {
		return "", nil, err
	}
	text := audioBriefingFrameSectionText(resp, section)
	models := appendAudioBriefingScriptModel(nil, resp.LLM)
	if !audioBriefingFrameSectionNeedsSupplement(section, sectionTarget, text) {
		return text, models, nil
	}

	supplementContext := audioBriefingSupplementIntroContext(introContext, section, text)
	supplementTarget := audioBriefingFrameSectionSupplementTargetChars(section, sectionTarget, text)
	if supplementTarget <= 0 {
		return text, models, nil
	}
	supplementResp, err := call(articles, supplementContext, supplementTarget, includeOpening, includeOverallSummary, false, includeEnding)
	if err != nil {
		return text, models, nil
	}
	text = audioBriefingMergeSectionText(text, audioBriefingFrameSectionText(supplementResp, section))
	models = appendAudioBriefingScriptModel(models, supplementResp.LLM)
	return text, models, nil
}

func audioBriefingFrameSectionTargetChars(targetChars int, section string) int {
	switch section {
	case "opening":
		return audioBriefingOpeningBudget(targetChars)
	case "overall_summary":
		return audioBriefingSummaryBudget(targetChars)
	case "ending":
		return audioBriefingEndingBudget(targetChars)
	default:
		return 0
	}
}

func audioBriefingFrameSectionCharsPerSentence(section string) int {
	switch section {
	case "overall_summary":
		return 70
	default:
		return 65
	}
}

func audioBriefingFrameSectionMinSentences(section string, targetChars int) int {
	budget := audioBriefingFrameSectionTargetChars(targetChars, section)
	charsPerSentence := audioBriefingFrameSectionCharsPerSentence(section)
	minSentences := 2
	maxSentences := 12
	if section == "overall_summary" {
		maxSentences = 14
	}
	count := int(math.Round(float64(budget) / float64(charsPerSentence)))
	if count < minSentences {
		count = minSentences
	}
	if count > maxSentences {
		count = maxSentences
	}
	low := count - 1
	if low < minSentences {
		low = minSentences
	}
	return low
}

func audioBriefingFrameSectionNeedsSupplement(section string, targetChars int, text string) bool {
	text = strings.TrimSpace(text)
	if text == "" {
		return true
	}
	budget := audioBriefingFrameSectionTargetChars(targetChars, section)
	if budget <= 0 {
		return false
	}
	if charCount(text) < maxInt(budget*2/3, 120) {
		return true
	}
	return audioBriefingSentenceCount(text) < audioBriefingFrameSectionMinSentences(section, targetChars)
}

func audioBriefingFrameSectionSupplementTargetChars(section string, targetChars int, text string) int {
	budget := audioBriefingFrameSectionTargetChars(targetChars, section)
	if budget <= 0 {
		return 0
	}
	missingChars := budget - charCount(strings.TrimSpace(text))
	missingSentences := audioBriefingFrameSectionMinSentences(section, targetChars) - audioBriefingSentenceCount(text)
	supplementChars := maxInt(missingChars, missingSentences*audioBriefingFrameSectionCharsPerSentence(section))
	if supplementChars <= 0 {
		return 0
	}
	if supplementChars < 120 {
		supplementChars = 120
	}
	if supplementChars > budget {
		supplementChars = budget
	}
	return supplementChars
}

func audioBriefingSupplementIntroContext(base map[string]any, section string, existingText string) map[string]any {
	out := make(map[string]any, len(base)+3)
	for key, value := range base {
		out[key] = value
	}
	out["audio_briefing_generation_mode"] = "supplement"
	out["audio_briefing_generation_section"] = section
	out["audio_briefing_existing_section_text"] = strings.TrimSpace(existingText)
	return out
}

func audioBriefingFrameSectionText(resp *AudioBriefingScriptResponse, section string) string {
	if resp == nil {
		return ""
	}
	switch section {
	case "opening":
		return strings.TrimSpace(resp.Opening)
	case "overall_summary":
		return strings.TrimSpace(resp.OverallSummary)
	case "ending":
		return strings.TrimSpace(resp.Ending)
	default:
		return ""
	}
}

func audioBriefingMergeSectionText(base string, supplement string) string {
	base = strings.TrimSpace(base)
	supplement = audioBriefingTrimRepeatedLeadingSentences(base, strings.TrimSpace(supplement))
	switch {
	case base == "":
		return supplement
	case supplement == "":
		return base
	case strings.Contains(base, supplement):
		return base
	default:
		return strings.TrimSpace(base + "\n" + supplement)
	}
}

func audioBriefingTrimRepeatedLeadingSentences(base string, supplement string) string {
	base = strings.TrimSpace(base)
	supplement = strings.TrimSpace(supplement)
	if base == "" || supplement == "" {
		return supplement
	}
	baseSentences := audioBriefingSplitSentences(base)
	if len(baseSentences) == 0 {
		return supplement
	}
	baseSeen := make(map[string]struct{}, len(baseSentences))
	for _, sentence := range baseSentences {
		key := audioBriefingSentenceKey(sentence)
		if key == "" {
			continue
		}
		baseSeen[key] = struct{}{}
	}
	supplementSentences := audioBriefingSplitSentences(supplement)
	drop := 0
	for drop < len(supplementSentences) {
		key := audioBriefingSentenceKey(supplementSentences[drop])
		if key == "" {
			drop++
			continue
		}
		if _, ok := baseSeen[key]; !ok {
			break
		}
		drop++
	}
	if drop == 0 {
		return supplement
	}
	return strings.TrimSpace(strings.Join(supplementSentences[drop:], "\n"))
}

func audioBriefingSplitSentences(text string) []string {
	text = strings.TrimSpace(text)
	if text == "" {
		return nil
	}
	lines := strings.Split(text, "\n")
	out := make([]string, 0, len(lines))
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		out = append(out, line)
	}
	return out
}

func audioBriefingSentenceKey(text string) string {
	text = strings.TrimSpace(text)
	if text == "" {
		return ""
	}
	text = strings.TrimRight(text, "。！？")
	return strings.TrimSpace(text)
}

func audioBriefingSentenceCount(text string) int {
	text = strings.TrimSpace(text)
	if text == "" {
		return 0
	}
	parts := strings.FieldsFunc(text, func(r rune) bool {
		switch r {
		case '\n', '。', '！', '？':
			return true
		default:
			return false
		}
	})
	count := 0
	for _, part := range parts {
		if strings.TrimSpace(part) != "" {
			count++
		}
	}
	if count == 0 {
		return 1
	}
	return count
}

func audioBriefingArticleBatchTargetChars(targetChars, totalArticles, batchArticles int) int {
	if batchArticles <= 0 {
		return 0
	}
	perArticle := audioBriefingCommentaryBudget(targetChars, totalArticles)
	target := perArticle*batchArticles + batchArticles*40
	return target
}

func audioBriefingArticleSectionBudgets(articleBudget int) (int, int, int) {
	if articleBudget <= 0 {
		return 0, 0, 0
	}
	headlineBudget := int(math.Round(float64(articleBudget) * 0.12))
	if headlineBudget < 40 {
		headlineBudget = 40
	}
	if headlineBudget > 160 {
		headlineBudget = 160
	}
	summaryIntroBudget := int(math.Round(float64(articleBudget) * 0.43))
	if summaryIntroBudget < 130 {
		summaryIntroBudget = 130
	}
	if summaryIntroBudget > 460 {
		summaryIntroBudget = 460
	}
	used := headlineBudget + summaryIntroBudget
	if used >= articleBudget {
		headlineBudget = maxInt(int(math.Round(float64(articleBudget)*0.12)), 1)
		summaryIntroBudget = maxInt(int(math.Round(float64(articleBudget)*0.43)), 1)
		used = headlineBudget + summaryIntroBudget
	}
	commentaryBudget := maxInt(articleBudget-used, 1)
	return headlineBudget, summaryIntroBudget, commentaryBudget
}

func audioBriefingCharBounds(budget int) (int, int) {
	if budget <= 0 {
		return 0, 0
	}
	lower := maxInt(int(math.Round(float64(budget)*0.9)), 1)
	upper := maxInt(int(math.Round(float64(budget)*1.15)), lower)
	if upper-lower < 20 {
		upper = lower + 20
	}
	return lower, upper
}

func audioBriefingArticleSegmentsNeedSupplement(segments []AudioBriefingScriptSegment, targetChars, totalArticles int) bool {
	if len(segments) == 0 || totalArticles <= 0 {
		return false
	}
	articleBudget := audioBriefingCommentaryBudget(targetChars, totalArticles)
	if articleBudget <= 0 {
		return false
	}
	headlineBudget, summaryBudget, commentaryBudget := audioBriefingArticleSectionBudgets(articleBudget)
	articleMinChars, _ := audioBriefingCharBounds(articleBudget)
	headlineMinChars, _ := audioBriefingCharBounds(headlineBudget)
	summaryMinChars, _ := audioBriefingCharBounds(summaryBudget)
	commentaryMinChars, _ := audioBriefingCharBounds(commentaryBudget)
	for _, segment := range segments {
		if charCount(audioBriefingArticleText(segment.Headline, segment.SummaryIntro, segment.Commentary)) < articleMinChars {
			return true
		}
		if charCount(strings.TrimSpace(segment.Headline)) < headlineMinChars {
			return true
		}
		if charCount(strings.TrimSpace(segment.SummaryIntro)) < summaryMinChars {
			return true
		}
		if charCount(strings.TrimSpace(segment.Commentary)) < commentaryMinChars {
			return true
		}
	}
	return false
}

func audioBriefingArticleSegmentSupplementReasons(segment AudioBriefingScriptSegment, targetChars, totalArticles int) []string {
	if totalArticles <= 0 {
		return nil
	}
	articleBudget := audioBriefingCommentaryBudget(targetChars, totalArticles)
	if articleBudget <= 0 {
		return nil
	}
	headlineBudget, summaryBudget, commentaryBudget := audioBriefingArticleSectionBudgets(articleBudget)
	articleMinChars, _ := audioBriefingCharBounds(articleBudget)
	headlineMinChars, _ := audioBriefingCharBounds(headlineBudget)
	summaryMinChars, _ := audioBriefingCharBounds(summaryBudget)
	commentaryMinChars, _ := audioBriefingCharBounds(commentaryBudget)
	reasons := make([]string, 0, 4)
	if charCount(audioBriefingArticleText(segment.Headline, segment.SummaryIntro, segment.Commentary)) < articleMinChars {
		reasons = append(reasons, "article_total")
	}
	if charCount(strings.TrimSpace(segment.Headline)) < headlineMinChars {
		reasons = append(reasons, "headline")
	}
	if charCount(strings.TrimSpace(segment.SummaryIntro)) < summaryMinChars {
		reasons = append(reasons, "summary_intro")
	}
	if charCount(strings.TrimSpace(segment.Commentary)) < commentaryMinChars {
		reasons = append(reasons, "commentary")
	}
	return reasons
}

func logAudioBriefingArticleSupplementDecision(targetChars, totalArticles int, before []AudioBriefingScriptSegment) {
	for _, segment := range before {
		reasons := audioBriefingArticleSegmentSupplementReasons(segment, targetChars, totalArticles)
		if len(reasons) == 0 {
			continue
		}
		log.Printf(
			"audio briefing article supplement needed item_id=%s reasons=%s before_headline_chars=%d before_summary_intro_chars=%d before_commentary_chars=%d before_total_chars=%d",
			strings.TrimSpace(segment.ItemID),
			strings.Join(reasons, ","),
			charCount(strings.TrimSpace(segment.Headline)),
			charCount(strings.TrimSpace(segment.SummaryIntro)),
			charCount(strings.TrimSpace(segment.Commentary)),
			charCount(audioBriefingArticleText(segment.Headline, segment.SummaryIntro, segment.Commentary)),
		)
	}
}

func logAudioBriefingArticleSupplementResult(before []AudioBriefingScriptSegment, after []AudioBriefingScriptSegment) {
	if len(before) == 0 || len(after) == 0 {
		return
	}
	byID := make(map[string]AudioBriefingScriptSegment, len(after))
	for _, segment := range after {
		itemID := strings.TrimSpace(segment.ItemID)
		if itemID == "" {
			continue
		}
		byID[itemID] = segment
	}
	for _, segment := range before {
		itemID := strings.TrimSpace(segment.ItemID)
		merged, ok := byID[itemID]
		if !ok {
			continue
		}
		log.Printf(
			"audio briefing article supplement result item_id=%s after_headline_chars=%d after_summary_intro_chars=%d after_commentary_chars=%d after_total_chars=%d delta_total_chars=%d",
			itemID,
			charCount(strings.TrimSpace(merged.Headline)),
			charCount(strings.TrimSpace(merged.SummaryIntro)),
			charCount(strings.TrimSpace(merged.Commentary)),
			charCount(audioBriefingArticleText(merged.Headline, merged.SummaryIntro, merged.Commentary)),
			charCount(audioBriefingArticleText(merged.Headline, merged.SummaryIntro, merged.Commentary))-charCount(audioBriefingArticleText(segment.Headline, segment.SummaryIntro, segment.Commentary)),
		)
	}
}

func audioBriefingArticleSegmentsSupplementTargetChars(segments []AudioBriefingScriptSegment, targetChars, totalArticles int) int {
	if len(segments) == 0 || totalArticles <= 0 {
		return 0
	}
	articleBudget := audioBriefingCommentaryBudget(targetChars, totalArticles)
	if articleBudget <= 0 {
		return 0
	}
	headlineBudget, summaryBudget, commentaryBudget := audioBriefingArticleSectionBudgets(articleBudget)
	articleMinChars, _ := audioBriefingCharBounds(articleBudget)
	commentaryMinChars, _ := audioBriefingCharBounds(commentaryBudget)
	headlineMinChars, _ := audioBriefingCharBounds(headlineBudget)
	summaryMinChars, _ := audioBriefingCharBounds(summaryBudget)
	missing := 0
	for _, segment := range segments {
		missing += maxInt(articleMinChars-charCount(audioBriefingArticleText(segment.Headline, segment.SummaryIntro, segment.Commentary)), 0)
		missing += maxInt(headlineMinChars-charCount(strings.TrimSpace(segment.Headline)), 0)
		missing += maxInt(summaryMinChars-charCount(strings.TrimSpace(segment.SummaryIntro)), 0)
		missing += maxInt(commentaryMinChars-charCount(strings.TrimSpace(segment.Commentary)), 0)
	}
	if missing <= 0 {
		return 0
	}
	if missing < 160 {
		missing = 160
	}
	maxTarget := audioBriefingArticleBatchTargetChars(targetChars, totalArticles, len(segments))
	if missing > maxTarget {
		missing = maxTarget
	}
	return missing
}

func audioBriefingArticleSupplementIntroContext(base map[string]any, segments []AudioBriefingScriptSegment) map[string]any {
	out := make(map[string]any, len(base)+3)
	for key, value := range base {
		out[key] = value
	}
	out["audio_briefing_generation_mode"] = "supplement"
	out["audio_briefing_generation_section"] = "article_segments"
	out["audio_briefing_existing_article_segments"] = segments
	return out
}

func audioBriefingTurnSectionIntroContext(base map[string]any, section string) map[string]any {
	out := make(map[string]any, len(base)+3)
	for key, value := range base {
		out[key] = value
	}
	out["audio_briefing_generation_section"] = section
	switch section {
	case "opening":
		out["audio_briefing_program_position"] = "program_start"
	case "overall_summary":
		out["audio_briefing_program_position"] = "after_opening_before_articles"
	case "ending":
		out["audio_briefing_program_position"] = "after_articles_program_end"
	default:
		out["audio_briefing_program_position"] = section
	}
	return out
}

func audioBriefingTurnArticleIntroContext(base map[string]any, start int, end int, totalArticles int) map[string]any {
	out := make(map[string]any, len(base)+6)
	for key, value := range base {
		out[key] = value
	}
	out["audio_briefing_generation_section"] = "article_segments"
	out["audio_briefing_program_position"] = "article_midstream"
	out["audio_briefing_article_batch_start_index"] = start + 1
	out["audio_briefing_article_batch_end_index"] = end
	out["audio_briefing_total_articles"] = totalArticles
	return out
}

func audioBriefingMergeArticleSegments(base []AudioBriefingScriptSegment, supplement []AudioBriefingScriptSegment) []AudioBriefingScriptSegment {
	if len(base) == 0 || len(supplement) == 0 {
		return base
	}
	byID := make(map[string]AudioBriefingScriptSegment, len(supplement))
	for _, segment := range supplement {
		itemID := strings.TrimSpace(segment.ItemID)
		if itemID == "" {
			continue
		}
		byID[itemID] = segment
	}
	merged := make([]AudioBriefingScriptSegment, 0, len(base))
	for _, segment := range base {
		itemID := strings.TrimSpace(segment.ItemID)
		supp, ok := byID[itemID]
		if !ok {
			merged = append(merged, segment)
			continue
		}
		merged = append(merged, AudioBriefingScriptSegment{
			ItemID:       segment.ItemID,
			Headline:     audioBriefingMergeSectionText(segment.Headline, supp.Headline),
			SummaryIntro: audioBriefingMergeSectionText(segment.SummaryIntro, supp.SummaryIntro),
			Commentary:   audioBriefingMergeSectionText(segment.Commentary, supp.Commentary),
		})
	}
	return merged
}

func audioBriefingValidateArticleSegments(segments []AudioBriefingScriptSegment) error {
	for _, segment := range segments {
		itemID := strings.TrimSpace(segment.ItemID)
		if itemID == "" {
			itemID = "<unknown>"
		}
		if strings.TrimSpace(segment.Headline) == "" {
			return fmt.Errorf("audio briefing script invalid article segment item_id=%s missing headline", itemID)
		}
		if strings.TrimSpace(segment.SummaryIntro) == "" {
			return fmt.Errorf("audio briefing script invalid article segment item_id=%s missing summary_intro", itemID)
		}
		if strings.TrimSpace(segment.Commentary) == "" {
			return fmt.Errorf("audio briefing script invalid article segment item_id=%s missing commentary", itemID)
		}
	}
	return nil
}

func audioBriefingArticleBatchSize(itemCount int) int {
	if itemCount <= 0 {
		return 1
	}
	if itemCount < 3 {
		return itemCount
	}
	return 3
}

func audioBriefingDuoArticleBatchSize(itemCount int) int {
	if itemCount <= 0 {
		return 1
	}
	if itemCount < 3 {
		return itemCount
	}
	return 3
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func audioBriefingGenerateArticleSegmentsBatch(
	batch []AudioBriefingScriptArticle,
	targetChars int,
	totalArticles int,
	call func(batch []AudioBriefingScriptArticle, introContext map[string]any, batchTargetChars int, includeOpening, includeOverallSummary, includeArticleSegments, includeEnding bool) (*AudioBriefingScriptResponse, error),
	introContext map[string]any,
) (audioBriefingArticleBatchResult, error) {
	batchTargetChars := audioBriefingArticleBatchTargetChars(targetChars, totalArticles, len(batch))
	resp, err := call(
		batch,
		introContext,
		batchTargetChars,
		false,
		false,
		true,
		false,
	)
	if err == nil {
		segments := resp.ArticleSegments
		models := appendAudioBriefingScriptModel(nil, resp.LLM)
		if validationErr := audioBriefingValidateArticleSegments(segments); validationErr != nil {
			log.Printf("audio briefing article batch invalid models=%s err=%v", strings.Join(models, ","), validationErr)
			err = validationErr
		}
	}
	if err == nil {
		segments := resp.ArticleSegments
		models := appendAudioBriefingScriptModel(nil, resp.LLM)
		if audioBriefingArticleSegmentsNeedSupplement(segments, targetChars, totalArticles) {
			logAudioBriefingArticleSupplementDecision(targetChars, totalArticles, segments)
			supplementContext := audioBriefingArticleSupplementIntroContext(introContext, segments)
			supplementTargetChars := audioBriefingArticleSegmentsSupplementTargetChars(segments, targetChars, totalArticles)
			if supplementTargetChars > 0 {
				supplementResp, supplementErr := call(
					batch,
					supplementContext,
					supplementTargetChars,
					false,
					false,
					true,
					false,
				)
				if supplementErr == nil {
					segments = audioBriefingMergeArticleSegments(segments, supplementResp.ArticleSegments)
					logAudioBriefingArticleSupplementResult(resp.ArticleSegments, segments)
					models = appendAudioBriefingScriptModel(models, supplementResp.LLM)
				}
			}
		}
		return audioBriefingArticleBatchResult{
			Segments:        segments,
			ScriptLLMModels: models,
		}, nil
	}
	if len(batch) <= 1 {
		return audioBriefingArticleBatchResult{}, err
	}

	mid := len(batch) / 2
	left, leftErr := audioBriefingGenerateArticleSegmentsBatch(batch[:mid], targetChars, totalArticles, call, introContext)
	if leftErr != nil {
		return audioBriefingArticleBatchResult{}, leftErr
	}
	right, rightErr := audioBriefingGenerateArticleSegmentsBatch(batch[mid:], targetChars, totalArticles, call, introContext)
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

func audioBriefingGenerateTurnSection(
	articles []AudioBriefingScriptArticle,
	introContext map[string]any,
	targetChars int,
	section string,
	call func(batch []AudioBriefingScriptArticle, introContext map[string]any, batchTargetChars int, includeOpening, includeOverallSummary, includeArticleSegments, includeEnding bool) (*AudioBriefingScriptResponse, error),
) ([]AudioBriefingScriptTurn, []string, error) {
	if call == nil {
		return nil, nil, fmt.Errorf("audio briefing duo script caller is unavailable")
	}
	sectionTarget := audioBriefingFrameSectionTargetChars(targetChars, section)
	includeOpening := section == "opening"
	includeOverallSummary := section == "overall_summary"
	includeEnding := section == "ending"
	resp, err := call(articles, introContext, sectionTarget, includeOpening, includeOverallSummary, false, includeEnding)
	if err != nil {
		return nil, nil, err
	}
	if len(resp.Turns) == 0 {
		return nil, appendAudioBriefingScriptModel(nil, resp.LLM), fmt.Errorf("audio briefing duo section %s returned no turns", section)
	}
	return append([]AudioBriefingScriptTurn{}, resp.Turns...), appendAudioBriefingScriptModel(nil, resp.LLM), nil
}

func audioBriefingGenerateTurnArticleBatch(
	batch []AudioBriefingScriptArticle,
	targetChars int,
	totalArticles int,
	call func(batch []AudioBriefingScriptArticle, introContext map[string]any, batchTargetChars int, includeOpening, includeOverallSummary, includeArticleSegments, includeEnding bool) (*AudioBriefingScriptResponse, error),
	introContext map[string]any,
) (audioBriefingTurnBatchResult, error) {
	batchTargetChars := audioBriefingArticleBatchTargetChars(targetChars, totalArticles, len(batch))
	resp, err := call(
		batch,
		introContext,
		batchTargetChars,
		false,
		false,
		true,
		false,
	)
	if err == nil {
		if len(resp.Turns) == 0 {
			err = fmt.Errorf("audio briefing duo article batch returned no turns")
		}
	}
	if err == nil {
		return audioBriefingTurnBatchResult{
			Turns:           append([]AudioBriefingScriptTurn{}, resp.Turns...),
			ScriptLLMModels: appendAudioBriefingScriptModel(nil, resp.LLM),
		}, nil
	}
	if len(batch) <= 1 {
		return audioBriefingTurnBatchResult{}, err
	}
	mid := len(batch) / 2
	left, leftErr := audioBriefingGenerateTurnArticleBatch(batch[:mid], targetChars, totalArticles, call, introContext)
	if leftErr != nil {
		return audioBriefingTurnBatchResult{}, leftErr
	}
	right, rightErr := audioBriefingGenerateTurnArticleBatch(batch[mid:], targetChars, totalArticles, call, introContext)
	if rightErr != nil {
		return audioBriefingTurnBatchResult{}, rightErr
	}
	recovered := make([]audioBriefingRecoveredFailure, 0, 1+len(left.RecoveredFailures)+len(right.RecoveredFailures))
	recovered = append(recovered, audioBriefingRecoveredFailure{BatchSize: len(batch), Err: err})
	recovered = append(recovered, left.RecoveredFailures...)
	recovered = append(recovered, right.RecoveredFailures...)
	return audioBriefingTurnBatchResult{
		Turns:             append(left.Turns, right.Turns...),
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
	name = audioBriefingScriptModelLabel(strings.TrimSpace(llm.Provider), name)
	for _, existing := range models {
		if existing == name {
			return models
		}
	}
	return append(models, name)
}

func audioBriefingScriptModelLabel(provider string, modelName string) string {
	name := strings.TrimSpace(modelName)
	if name == "" {
		return ""
	}
	label := audioBriefingProviderLabel(provider)
	if label == "" {
		return name
	}
	return label + " / " + name
}

func audioBriefingProviderLabel(provider string) string {
	switch strings.ToLower(strings.TrimSpace(provider)) {
	case "anthropic":
		return "Anthropic"
	case "google":
		return "Google"
	case "groq":
		return "Groq"
	case "deepseek":
		return "DeepSeek"
	case "alibaba":
		return "Alibaba"
	case "mistral":
		return "Mistral"
	case "moonshot":
		return "Moonshot"
	case "minimax":
		return "MiniMax"
	case "xiaomi_mimo_token_plan":
		return "Xiaomi MiMo (TokenPlan)"
	case "together":
		return "Together AI"
	case "xai":
		return "xAI"
	case "zai":
		return "Z.ai"
	case "fireworks":
		return "Fireworks"
	case "openai":
		return "OpenAI"
	case "openrouter":
		return "OpenRouter"
	case "poe":
		return "Poe"
	case "siliconflow":
		return "SiliconFlow"
	case "cerebras":
		return "Cerebras AI"
	default:
		return strings.TrimSpace(provider)
	}
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

func resolveRandomAudioBriefingPartnerCandidate(
	hostPersona string,
	hostRequiresGemini bool,
	hostVoice *model.AudioBriefingPersonaVoice,
	voices []model.AudioBriefingPersonaVoice,
	picker func([]string) (string, bool),
) (string, *model.AudioBriefingPersonaVoice, bool) {
	normalizedHost := normalizeAudioBriefingPersona(hostPersona)
	if picker == nil {
		picker = randomPersonaFromCandidates
	}
	candidateVoices := make(map[string]model.AudioBriefingPersonaVoice, len(voices))
	candidates := make([]string, 0, len(voices))
	for _, voice := range voices {
		persona := normalizeAudioBriefingPersona(voice.Persona)
		if persona == normalizedHost {
			continue
		}
		if strings.TrimSpace(voice.TTSProvider) == "" || strings.TrimSpace(voice.VoiceModel) == "" {
			continue
		}
		if hostRequiresGemini && !audioBriefingGeminiDuoReady(hostVoice, &voice) {
			continue
		}
		if hostVoice != nil && strings.EqualFold(strings.TrimSpace(hostVoice.TTSProvider), "fish") && !audioBriefingFishDuoReady(hostVoice, &voice) {
			continue
		}
		if _, exists := candidateVoices[persona]; exists {
			continue
		}
		candidateVoices[persona] = voice
		candidates = append(candidates, persona)
	}
	if len(candidates) == 0 {
		return "", nil, false
	}
	picked, ok := picker(candidates)
	if !ok {
		return "", nil, false
	}
	partnerPersona := normalizeAudioBriefingPersona(picked)
	voice, exists := candidateVoices[partnerPersona]
	if !exists {
		return "", nil, false
	}
	return partnerPersona, &voice, true
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
	case "together":
		return settings.HasTogetherAPIKey
	case "moonshot":
		return settings.HasMoonshotAPIKey
	case "minimax":
		return settings.HasMiniMaxAPIKey
	case "xiaomi_mimo_token_plan":
		return settings.HasXiaomiMiMoTokenPlanAPIKey
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
	case "siliconflow":
		return settings.HasSiliconFlowAPIKey
	case "deepinfra":
		return settings.HasDeepInfraAPIKey
	case "featherless":
		return settings.HasFeatherlessAPIKey
	case "cerebras":
		return settings.HasCerebrasAPIKey
	default:
		return settings.HasAnthropicAPIKey
	}
}

func buildAudioBriefingIntroContext(now time.Time, programName *string) map[string]any {
	now = now.In(timeutil.JST)
	out := map[string]any{
		"now_jst":     now.Format(time.RFC3339),
		"date_jst":    now.Format("2006-01-02"),
		"weekday_jst": now.Weekday().String(),
		"time_of_day": audioBriefingTimeOfDay(now.Hour()),
		"season_hint": audioBriefingSeasonHint(now),
	}
	if programName != nil && strings.TrimSpace(*programName) != "" {
		out["program_name"] = strings.TrimSpace(*programName)
	}
	return out
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

func recordAudioBriefingLLMUsage(ctx context.Context, repo *repository.LLMUsageLogRepo, cache JSONCache, purpose string, usage *LLMUsage, userID *string, prompt *PromptResolution) {
	usage = NormalizeCatalogPricedUsage(purpose, usage)
	if repo == nil || usage == nil || userID == nil || *userID == "" {
		return
	}
	sum := sha256.Sum256([]byte(fmt.Sprintf("%s|%s|%s|%s|%s|%s|%s|%s|%d|%d|%d|%d", purpose, usage.Provider, usage.Model, *userID, promptKey(prompt), promptSource(prompt), toVal(promptVersionID(prompt)), toIntVal(promptVersionNumber(prompt)), usage.InputTokens, usage.OutputTokens, usage.CacheCreationInputTokens, usage.CacheReadInputTokens)))
	key := hex.EncodeToString(sum[:])
	pricingSource := usage.PricingSource
	if pricingSource == "" {
		pricingSource = "unknown"
	}
	if err := repo.Insert(ctx, repository.LLMUsageLogInput{
		IdempotencyKey:           &key,
		UserID:                   userID,
		PromptKey:                promptKey(prompt),
		PromptSource:             promptSource(prompt),
		PromptVersionID:          promptVersionID(prompt),
		PromptVersionNumber:      promptVersionNumber(prompt),
		PromptExperimentID:       promptExperimentID(prompt),
		PromptExperimentArmID:    promptExperimentArmID(prompt),
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
