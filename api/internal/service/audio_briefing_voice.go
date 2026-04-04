package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/enjoydarts/sifto/api/internal/model"
	"github.com/enjoydarts/sifto/api/internal/repository"
)

type AudioBriefingVoiceRunResult struct {
	ProcessedChunk bool
	Completed      bool
	Waiting        bool
}

type AudioBriefingVoiceRunner struct {
	repo         *repository.AudioBriefingRepo
	userRepo     *repository.UserRepo
	userSettings *repository.UserSettingsRepo
	cipher       *SecretCipher
	worker       *WorkerClient
}

const audioBriefingChunkMaxAttempts = 3
const audioBriefingGeminiDuoSoftByteLimitDefault = 3200

type audioBriefingSpeechParams struct {
	SpeechRate                 float64
	EmotionalIntensity         float64
	TempoDynamics              float64
	LineBreakSilenceSeconds    float64
	ChunkTrailingSilenceSecond float64
	Pitch                      float64
	VolumeGain                 float64
}

type audioBriefingChunkGroup struct {
	PartType string
	ItemID   string
	Chunks   []*model.AudioBriefingScriptChunk
}

func audioBriefingVoiceConfigComplete(provider, voiceModel, voiceStyle string) bool {
	provider = strings.TrimSpace(provider)
	voiceModel = strings.TrimSpace(voiceModel)
	voiceStyle = strings.TrimSpace(voiceStyle)
	if provider == "" || voiceModel == "" {
		return false
	}
	return voiceStyle != "" || strings.EqualFold(provider, "xai") || strings.EqualFold(provider, "mock") || strings.EqualFold(provider, "openai") || strings.EqualFold(provider, "gemini_tts")
}

func audioBriefingGeminiDuoReady(hostVoice, partnerVoice *model.AudioBriefingPersonaVoice) bool {
	if hostVoice == nil || partnerVoice == nil {
		return false
	}
	return strings.EqualFold(strings.TrimSpace(hostVoice.TTSProvider), "gemini_tts") &&
		strings.EqualFold(strings.TrimSpace(partnerVoice.TTSProvider), "gemini_tts") &&
		strings.TrimSpace(hostVoice.TTSModel) != "" &&
		strings.TrimSpace(partnerVoice.TTSModel) != "" &&
		strings.TrimSpace(hostVoice.TTSModel) == strings.TrimSpace(partnerVoice.TTSModel) &&
		strings.TrimSpace(hostVoice.VoiceModel) != "" &&
		strings.TrimSpace(partnerVoice.VoiceModel) != ""
}

func NewAudioBriefingVoiceRunner(repo *repository.AudioBriefingRepo, userRepo *repository.UserRepo, userSettings *repository.UserSettingsRepo, cipher *SecretCipher, worker *WorkerClient) *AudioBriefingVoiceRunner {
	return &AudioBriefingVoiceRunner{repo: repo, userRepo: userRepo, userSettings: userSettings, cipher: cipher, worker: worker}
}

func (r *AudioBriefingVoiceRunner) Start(ctx context.Context, userID string, jobID string) (result *AudioBriefingVoiceRunResult, err error) {
	if r == nil || r.repo == nil || r.worker == nil {
		return nil, fmt.Errorf("audio briefing voice runner unavailable")
	}
	defer func() {
		if recovered := recover(); recovered != nil {
			err = fmt.Errorf("audio briefing voice stage panic: %v", recovered)
		}
		if err != nil {
			r.bestEffortFailVoicing(jobID, "tts_failed", err.Error())
		}
	}()

	job, err := r.repo.GetJobByID(ctx, userID, jobID)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(job.Status) != "voicing" {
		job, err = r.repo.StartVoicingJob(ctx, jobID)
		if err != nil {
			return nil, err
		}
		if err := r.repo.ResetChunksForVoicing(ctx, jobID); err != nil {
			r.bestEffortFailVoicing(jobID, "tts_failed", err.Error())
			return nil, err
		}
	}
	chunks, err := r.repo.ListJobChunks(ctx, userID, jobID)
	if err != nil {
		r.bestEffortFailVoicing(jobID, "tts_failed", err.Error())
		return nil, err
	}
	if len(chunks) == 0 {
		r.bestEffortFailVoicing(jobID, "tts_failed", "no script chunks")
		return nil, repository.ErrInvalidState
	}
	voice, err := r.repo.GetPersonaVoice(ctx, userID, job.Persona)
	if err != nil {
		r.bestEffortFailVoicing(jobID, "tts_failed", err.Error())
		return nil, err
	}
	settings, err := r.repo.GetSettings(ctx, userID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			settings = nil
		} else {
			r.bestEffortFailVoicing(jobID, "tts_failed", err.Error())
			return nil, err
		}
	}
	partnerVoice, err := r.resolvePartnerVoiceForJob(ctx, job)
	if err != nil {
		r.bestEffortFailVoicing(jobID, "tts_failed", err.Error())
		return nil, err
	}

	selection, chunk, resetGenerating := nextAudioBriefingVoicingChunk(chunks, timeutilNow())
	switch selection {
	case audioBriefingVoicingChunkSelectionComplete:
		completedJob, err := r.repo.CompleteVoicingJob(ctx, jobID)
		if err != nil {
			r.bestEffortFailVoicing(jobID, "tts_failed", err.Error())
			return nil, err
		}
		_ = completedJob
		return &AudioBriefingVoiceRunResult{Completed: true}, nil
	case audioBriefingVoicingChunkSelectionWaiting:
		return &AudioBriefingVoiceRunResult{Waiting: true}, nil
	}
	if chunk == nil {
		return &AudioBriefingVoiceRunResult{Waiting: true}, nil
	}
	group := audioBriefingChunkGroupForSelection(chunks, chunk)
	provider := strings.TrimSpace(derefString(chunk.TTSProvider))
	useGeminiDuoGroup := len(group.Chunks) > 1 && provider == "gemini_tts" && strings.TrimSpace(job.ConversationMode) == "duo"
	if useGeminiDuoGroup {
		group = audioBriefingGeminiDuoSplitGroups(group, audioBriefingGeminiDuoSoftByteLimit())[0]
	}
	groupChunkIDs := audioBriefingChunkGroupIDs(group)
	useGeminiDuoGroup = len(groupChunkIDs) > 1 && provider == "gemini_tts" && strings.TrimSpace(job.ConversationMode) == "duo"
	if resetGenerating {
		if chunk.AttemptCount >= audioBriefingChunkMaxAttempts {
			message := "stale generating chunk exceeded retry limit"
			if useGeminiDuoGroup {
				if err := r.repo.MarkChunkGroupExhausted(ctx, groupChunkIDs, "tts_stalled", message); err != nil {
					r.bestEffortFailVoicing(jobID, "tts_failed", err.Error())
					return nil, err
				}
			} else if err := r.repo.MarkChunkExhausted(ctx, chunk.ID, "tts_stalled", message); err != nil {
				r.bestEffortFailVoicing(jobID, "tts_failed", err.Error())
				return nil, err
			}
			r.bestEffortFailVoicing(jobID, "tts_stalled", message)
			return nil, errors.New(message)
		}
		if useGeminiDuoGroup {
			if err := r.repo.MarkChunkGroupRetryWait(ctx, groupChunkIDs, "tts_stalled", "stale generating chunk reset for retry"); err != nil {
				r.bestEffortFailVoicing(jobID, "tts_failed", err.Error())
				return nil, err
			}
		} else if err := r.repo.MarkChunkRetryWait(ctx, chunk.ID, "tts_stalled", "stale generating chunk reset for retry"); err != nil {
			r.bestEffortFailVoicing(jobID, "tts_failed", err.Error())
			return nil, err
		}
	}
	rawHeartbeatToken, err := randomAudioBriefingToken(32)
	if err != nil {
		r.bestEffortFailVoicing(jobID, "tts_failed", err.Error())
		return nil, err
	}
	if useGeminiDuoGroup {
		if err := r.repo.StartChunkGroupGenerating(ctx, groupChunkIDs, HashAudioBriefingCallbackToken(rawHeartbeatToken)); err != nil {
			if err == repository.ErrInvalidState {
				return &AudioBriefingVoiceRunResult{Waiting: true}, nil
			}
			r.bestEffortFailVoicing(jobID, "tts_failed", err.Error())
			return nil, err
		}
	} else if err := r.repo.StartChunkGenerating(ctx, chunk.ID, HashAudioBriefingCallbackToken(rawHeartbeatToken)); err != nil {
		if err == repository.ErrInvalidState {
			return &AudioBriefingVoiceRunResult{Waiting: true}, nil
		}
		r.bestEffortFailVoicing(jobID, "tts_failed", err.Error())
		return nil, err
	}
	for _, groupedChunk := range group.Chunks {
		groupedChunk.AttemptCount++
	}
	voiceModel := strings.TrimSpace(derefString(chunk.VoiceModel))
	voiceStyle := strings.TrimSpace(derefString(chunk.VoiceStyle))
	if !audioBriefingVoiceConfigComplete(provider, voiceModel, voiceStyle) {
		err := fmt.Errorf("chunk tts config is incomplete")
		return r.handleChunkGenerationFailure(ctx, jobID, chunk, "tts_failed", err)
	}
	ttsModel := strings.TrimSpace(voice.TTSModel)
	var aivisAPIKey *string
	var aivisUserDictionaryUUID *string
	var googleAPIKey *string
	var xaiAPIKey *string
	var openAIAPIKey *string
	if provider == "aivis" {
		aivisAPIKey, err = r.loadAivisAPIKey(ctx, userID)
		if err != nil {
			return r.handleChunkGenerationFailure(ctx, jobID, chunk, "tts_failed", err)
		}
		aivisUserDictionaryUUID, err = r.userSettings.GetAivisUserDictionaryUUID(ctx, userID)
		if err != nil {
			return r.handleChunkGenerationFailure(ctx, jobID, chunk, "tts_failed", err)
		}
	} else if provider == "xai" {
		xaiAPIKey, err = r.loadXAIAPIKey(ctx, userID)
		if err != nil {
			return r.handleChunkGenerationFailure(ctx, jobID, chunk, "tts_failed", err)
		}
	} else if provider == "gemini_tts" {
		if ttsModel == "" {
			err := fmt.Errorf("gemini tts model is not configured")
			return r.handleChunkGenerationFailure(ctx, jobID, chunk, "tts_failed", err)
		}
		if err := EnsureGeminiTTSEnabledForUser(ctx, r.userRepo, userID); err != nil {
			return r.handleChunkGenerationFailure(ctx, jobID, chunk, "tts_failed", err)
		}
	} else if provider == "openai" {
		if ttsModel == "" {
			err := fmt.Errorf("openai tts model is not configured")
			return r.handleChunkGenerationFailure(ctx, jobID, chunk, "tts_failed", err)
		}
		openAIAPIKey, err = loadAndDecryptAudioBriefingUserSecret(ctx, r.userSettings.GetOpenAIAPIKeyEncrypted, r.cipher, userID, "openai api key is not configured")
		if err != nil {
			return r.handleChunkGenerationFailure(ctx, jobID, chunk, "tts_failed", err)
		}
	}
	speechParams := audioBriefingSpeechParamsForChunk(chunk, voice, partnerVoice, settings)
	audioObjectKey := audioBriefingChunkObjectKey(userID, jobID, chunk.Seq)
	if leadChunk := audioBriefingChunkGroupLeadChunk(group); leadChunk != nil {
		audioObjectKey = audioBriefingChunkObjectKey(userID, jobID, leadChunk.Seq)
	}
	var resp *AudioBriefingSynthesizeUploadResponse
	if useGeminiDuoGroup {
		if !audioBriefingGeminiDuoReady(voice, partnerVoice) {
			err := fmt.Errorf("gemini duo multi-speaker is not fully configured")
			return r.handleChunkGroupGenerationFailure(ctx, jobID, group, "tts_failed", err)
		}
		resp, err = r.worker.SynthesizeAudioBriefingGeminiDuoUpload(
			ctx,
			ttsModel,
			job.Persona,
			derefString(job.PartnerPersona),
			strings.TrimSpace(voice.VoiceModel),
			strings.TrimSpace(partnerVoice.VoiceModel),
			group.PartType,
			audioBriefingGeminiDuoTurns(group),
			audioObjectKey,
			googleAPIKey,
		)
	} else {
		resp, err = r.worker.SynthesizeAudioBriefingUpload(
			ctx,
			provider,
			voiceModel,
			voiceStyle,
			ttsModel,
			job.Persona,
			chunk.Text,
			speechParams.SpeechRate,
			speechParams.EmotionalIntensity,
			speechParams.TempoDynamics,
			speechParams.LineBreakSilenceSeconds,
			speechParams.ChunkTrailingSilenceSecond,
			speechParams.Pitch,
			speechParams.VolumeGain,
			audioObjectKey,
			chunk.ID,
			audioBriefingChunkHeartbeatURL(chunk.ID),
			rawHeartbeatToken,
			aivisUserDictionaryUUID,
			aivisAPIKey,
			googleAPIKey,
			xaiAPIKey,
			openAIAPIKey,
		)
	}
	if err != nil {
		if !useGeminiDuoGroup {
			return r.handleChunkGenerationFailure(ctx, jobID, chunk, "tts_failed", annotateAudioBriefingChunkError(chunk, err))
		}
		return r.handleChunkGroupGenerationFailure(ctx, jobID, group, "tts_failed", annotateAudioBriefingChunkError(chunk, err))
	}
	if useGeminiDuoGroup {
		if err := r.repo.MarkChunkGroupGenerated(ctx, groupChunkIDs, resp.AudioObjectKey, resp.DurationSec); err != nil {
			r.bestEffortFailVoicing(jobID, "tts_failed", err.Error())
			return nil, err
		}
	} else if err := r.repo.MarkChunkGenerated(ctx, chunk.ID, resp.AudioObjectKey, resp.DurationSec); err != nil {
		r.bestEffortFailVoicing(jobID, "tts_failed", err.Error())
		return nil, err
	}
	if _, err := r.repo.CompleteVoicingJob(ctx, jobID); err == nil {
		return &AudioBriefingVoiceRunResult{ProcessedChunk: true, Completed: true}, nil
	} else if err != repository.ErrInvalidState {
		r.bestEffortFailVoicing(jobID, "tts_failed", err.Error())
		return nil, err
	}
	return &AudioBriefingVoiceRunResult{ProcessedChunk: true}, nil
}

func (r *AudioBriefingVoiceRunner) loadXAIAPIKey(ctx context.Context, userID string) (*string, error) {
	if r == nil || r.userSettings == nil || r.cipher == nil {
		return nil, errors.New("audio briefing xai key loader is not configured")
	}
	enc, err := r.userSettings.GetXAIAPIKeyEncrypted(ctx, userID)
	if err != nil {
		return nil, err
	}
	if enc == nil || strings.TrimSpace(*enc) == "" {
		return nil, errors.New("xai api key is not configured")
	}
	plain, err := r.cipher.DecryptString(*enc)
	if err != nil {
		return nil, err
	}
	plain = strings.TrimSpace(plain)
	if plain == "" {
		return nil, errors.New("xai api key is empty")
	}
	return &plain, nil
}

func audioBriefingSpeechParamsForChunk(
	chunk *model.AudioBriefingScriptChunk,
	hostVoice *model.AudioBriefingPersonaVoice,
	partnerVoice *model.AudioBriefingPersonaVoice,
	settings *model.AudioBriefingSettings,
) audioBriefingSpeechParams {
	params := audioBriefingSpeechParams{
		SpeechRate:                 1.0,
		EmotionalIntensity:         1.0,
		TempoDynamics:              1.0,
		LineBreakSilenceSeconds:    0.4,
		ChunkTrailingSilenceSecond: 1.0,
		Pitch:                      0.0,
		VolumeGain:                 0.0,
	}
	if settings != nil && settings.ChunkTrailingSilenceSeconds >= 0 {
		params.ChunkTrailingSilenceSecond = settings.ChunkTrailingSilenceSeconds
	}
	selectedVoice := hostVoice
	if chunk != nil && strings.TrimSpace(derefString(chunk.Speaker)) == "partner" && partnerVoice != nil {
		selectedVoice = partnerVoice
	}
	if selectedVoice != nil {
		params.SpeechRate = selectedVoice.SpeechRate
		params.EmotionalIntensity = selectedVoice.EmotionalIntensity
		params.TempoDynamics = selectedVoice.TempoDynamics
		params.LineBreakSilenceSeconds = selectedVoice.LineBreakSilenceSeconds
		params.Pitch = selectedVoice.Pitch
		params.VolumeGain = selectedVoice.VolumeGain
	}
	return params
}

func (r *AudioBriefingVoiceRunner) resolvePartnerVoiceForJob(ctx context.Context, job *model.AudioBriefingJob) (*model.AudioBriefingPersonaVoice, error) {
	if r == nil || r.repo == nil || job == nil {
		return nil, nil
	}
	if strings.TrimSpace(job.ConversationMode) != "duo" {
		return nil, nil
	}
	partnerPersona := strings.TrimSpace(derefString(job.PartnerPersona))
	if partnerPersona == "" {
		return nil, nil
	}
	return r.repo.GetPersonaVoice(ctx, job.UserID, partnerPersona)
}

func (r *AudioBriefingVoiceRunner) handleChunkGenerationFailure(ctx context.Context, jobID string, chunk *model.AudioBriefingScriptChunk, errorCode string, err error) (*AudioBriefingVoiceRunResult, error) {
	if r == nil || r.repo == nil {
		return nil, err
	}
	if chunk == nil {
		r.bestEffortFailVoicing(jobID, errorCode, err.Error())
		return nil, err
	}
	errorMessage := err.Error()
	if chunk.AttemptCount >= audioBriefingChunkMaxAttempts {
		if markErr := r.repo.MarkChunkExhausted(ctx, chunk.ID, errorCode, errorMessage); markErr != nil {
			r.bestEffortFailVoicing(jobID, errorCode, markErr.Error())
			return nil, markErr
		}
		r.bestEffortFailVoicing(jobID, errorCode, errorMessage)
		return nil, err
	}
	if markErr := r.repo.MarkChunkRetryWait(ctx, chunk.ID, errorCode, errorMessage); markErr != nil {
		r.bestEffortFailVoicing(jobID, errorCode, markErr.Error())
		return nil, markErr
	}
	return &AudioBriefingVoiceRunResult{ProcessedChunk: true}, nil
}

func (r *AudioBriefingVoiceRunner) handleChunkGroupGenerationFailure(ctx context.Context, jobID string, group audioBriefingChunkGroup, errorCode string, err error) (*AudioBriefingVoiceRunResult, error) {
	if len(group.Chunks) <= 1 {
		if len(group.Chunks) == 1 {
			return r.handleChunkGenerationFailure(ctx, jobID, group.Chunks[0], errorCode, err)
		}
		r.bestEffortFailVoicing(jobID, errorCode, err.Error())
		return nil, err
	}
	chunkIDs := audioBriefingChunkGroupIDs(group)
	if len(chunkIDs) == 0 {
		r.bestEffortFailVoicing(jobID, errorCode, err.Error())
		return nil, err
	}
	leadChunk := group.Chunks[0]
	errorMessage := err.Error()
	if leadChunk != nil && leadChunk.AttemptCount >= audioBriefingChunkMaxAttempts {
		if markErr := r.repo.MarkChunkGroupExhausted(ctx, chunkIDs, errorCode, errorMessage); markErr != nil {
			r.bestEffortFailVoicing(jobID, errorCode, markErr.Error())
			return nil, markErr
		}
		r.bestEffortFailVoicing(jobID, errorCode, errorMessage)
		return nil, err
	}
	if markErr := r.repo.MarkChunkGroupRetryWait(ctx, chunkIDs, errorCode, errorMessage); markErr != nil {
		r.bestEffortFailVoicing(jobID, errorCode, markErr.Error())
		return nil, markErr
	}
	return &AudioBriefingVoiceRunResult{ProcessedChunk: true}, nil
}

func annotateAudioBriefingChunkError(chunk *model.AudioBriefingScriptChunk, err error) error {
	if chunk == nil || err == nil {
		return err
	}
	return fmt.Errorf(
		"chunk_id=%s seq=%d part=%s text_preview=%q: %w",
		strings.TrimSpace(chunk.ID),
		chunk.Seq,
		strings.TrimSpace(chunk.PartType),
		audioBriefingChunkTextPreview(chunk.Text),
		err,
	)
}

func audioBriefingChunkTextPreview(text string) string {
	preview := strings.TrimSpace(text)
	if preview == "" {
		return ""
	}
	runes := []rune(preview)
	if len(runes) > 120 {
		preview = string(runes[:120])
	}
	return preview
}

func audioBriefingChunkGroupForSelection(chunks []model.AudioBriefingScriptChunk, selected *model.AudioBriefingScriptChunk) audioBriefingChunkGroup {
	if selected == nil {
		return audioBriefingChunkGroup{}
	}
	selectedPartType := strings.TrimSpace(selected.PartType)
	selectedItemID := strings.TrimSpace(derefString(selected.ItemID))
	group := audioBriefingChunkGroup{
		PartType: selectedPartType,
		ItemID:   selectedItemID,
		Chunks:   make([]*model.AudioBriefingScriptChunk, 0, 4),
	}
	for i := range chunks {
		chunk := &chunks[i]
		if chunk.R2AudioObjectKey != nil && strings.TrimSpace(*chunk.R2AudioObjectKey) != "" && strings.TrimSpace(chunk.TTSStatus) == "generated" {
			continue
		}
		if strings.TrimSpace(chunk.PartType) != selectedPartType {
			continue
		}
		if selectedPartType == "article" {
			if strings.TrimSpace(derefString(chunk.ItemID)) != selectedItemID {
				continue
			}
		}
		group.Chunks = append(group.Chunks, chunk)
	}
	if len(group.Chunks) == 0 {
		group.Chunks = append(group.Chunks, selected)
	}
	sort.SliceStable(group.Chunks, func(i, j int) bool {
		if group.Chunks[i] == nil {
			return false
		}
		if group.Chunks[j] == nil {
			return true
		}
		return group.Chunks[i].Seq < group.Chunks[j].Seq
	})
	return group
}

func audioBriefingChunkGroupLeadChunk(group audioBriefingChunkGroup) *model.AudioBriefingScriptChunk {
	for _, chunk := range group.Chunks {
		if chunk != nil {
			return chunk
		}
	}
	return nil
}

func audioBriefingChunkGroupIDs(group audioBriefingChunkGroup) []string {
	ids := make([]string, 0, len(group.Chunks))
	for _, chunk := range group.Chunks {
		if chunk == nil {
			continue
		}
		if id := strings.TrimSpace(chunk.ID); id != "" {
			ids = append(ids, id)
		}
	}
	return ids
}

func audioBriefingGeminiDuoTurns(group audioBriefingChunkGroup) []AudioBriefingGeminiDuoTurn {
	turns := make([]AudioBriefingGeminiDuoTurn, 0, len(group.Chunks))
	for _, chunk := range group.Chunks {
		if chunk == nil {
			continue
		}
		text := strings.TrimSpace(chunk.Text)
		if text == "" {
			continue
		}
		speaker := strings.TrimSpace(derefString(chunk.Speaker))
		if speaker == "" {
			speaker = "host"
		}
		turns = append(turns, AudioBriefingGeminiDuoTurn{
			Speaker: speaker,
			Text:    text,
		})
	}
	return turns
}

func audioBriefingGeminiDuoSplitGroups(group audioBriefingChunkGroup, maxBytes int) []audioBriefingChunkGroup {
	if len(group.Chunks) <= 1 || strings.TrimSpace(group.PartType) != "article" {
		return []audioBriefingChunkGroup{group}
	}
	if maxBytes <= 0 {
		maxBytes = audioBriefingGeminiDuoSoftByteLimitDefault
	}
	split := make([]audioBriefingChunkGroup, 0, 2)
	current := audioBriefingChunkGroup{
		PartType: group.PartType,
		ItemID:   group.ItemID,
		Chunks:   make([]*model.AudioBriefingScriptChunk, 0, len(group.Chunks)),
	}
	for _, chunk := range group.Chunks {
		if chunk == nil {
			continue
		}
		next := audioBriefingChunkGroup{
			PartType: group.PartType,
			ItemID:   group.ItemID,
			Chunks:   append(append([]*model.AudioBriefingScriptChunk{}, current.Chunks...), chunk),
		}
		nextBytes := audioBriefingGeminiDuoRequestEstimatedBytes(next)
		if len(current.Chunks) > 0 && nextBytes > maxBytes {
			split = append(split, current)
			current = audioBriefingChunkGroup{
				PartType: group.PartType,
				ItemID:   group.ItemID,
				Chunks:   make([]*model.AudioBriefingScriptChunk, 0, len(group.Chunks)),
			}
			next = audioBriefingChunkGroup{
				PartType: group.PartType,
				ItemID:   group.ItemID,
				Chunks:   []*model.AudioBriefingScriptChunk{chunk},
			}
			nextBytes = audioBriefingGeminiDuoRequestEstimatedBytes(next)
		}
		current.Chunks = append(current.Chunks, chunk)
	}
	if len(current.Chunks) > 0 {
		split = append(split, current)
	}
	if len(split) == 0 {
		return []audioBriefingChunkGroup{group}
	}
	return split
}

func audioBriefingGeminiDuoRequestEstimatedBytes(group audioBriefingChunkGroup) int {
	body := map[string]any{
		"input": map[string]any{
			"prompt": buildGeminiDuoAudioBriefingPrompt(group.PartType),
			"multiSpeakerMarkup": map[string]any{
				"turns": audioBriefingGeminiDuoRequestTurns(group),
			},
		},
		"voice": map[string]any{
			"languageCode": "ja-JP",
			"modelName":    "gemini-2.5-flash-tts",
			"multiSpeakerVoiceConfig": map[string]any{
				"speakerVoiceConfigs": []map[string]any{
					{
						"speakerAlias": "HOST",
						"speakerId":    "host",
					},
					{
						"speakerAlias": "PARTNER",
						"speakerId":    "partner",
					},
				},
			},
		},
		"audioConfig": map[string]any{
			"audioEncoding":   "MP3",
			"sampleRateHertz": 48000,
		},
	}
	raw, err := json.Marshal(body)
	if err != nil {
		return 0
	}
	return len(raw)
}

func audioBriefingGeminiDuoRequestTurns(group audioBriefingChunkGroup) []map[string]any {
	turns := make([]map[string]any, 0, len(group.Chunks))
	for _, turn := range audioBriefingGeminiDuoTurns(group) {
		speaker := "PARTNER"
		if strings.TrimSpace(turn.Speaker) == "host" {
			speaker = "HOST"
		}
		turns = append(turns, map[string]any{
			"speaker": speaker,
			"text":    strings.TrimSpace(turn.Text),
		})
	}
	return turns
}

func buildGeminiDuoAudioBriefingPrompt(sectionType string) string {
	sectionLabel := map[string]string{
		"opening": "オープニング",
		"summary": "総括セクション",
		"article": "記事セクション",
		"ending":  "エンディング",
	}[strings.TrimSpace(sectionType)]
	if sectionLabel == "" {
		sectionLabel = "会話セクション"
	}
	lines := []string{
		"あなたは音声ブリーフィング番組の二人会話を音声化するAIです。",
		fmt.Sprintf("以下は %s の台本です。会話の本文は改変せず、日本語の自然な掛け合いとして読み上げてください。", sectionLabel),
		"",
		"HOST は briefing host persona、PARTNER は briefing partner persona として演じてください。",
		"各 speaker の voice は別途指定されています。voice の割り当ては変えないでください。",
		"テキストに書かれていない補足、要約、言い換え、締めコメントは追加しないでください。",
		"追加の説明や要約は入れないでください。",
	}
	return strings.Join(lines, "\n")
}

func audioBriefingGeminiDuoSoftByteLimit() int {
	raw := strings.TrimSpace(os.Getenv("AUDIO_BRIEFING_GEMINI_DUO_SOFT_BYTE_LIMIT"))
	if raw == "" {
		return audioBriefingGeminiDuoSoftByteLimitDefault
	}
	value, err := strconv.Atoi(raw)
	if err != nil || value < 512 {
		return audioBriefingGeminiDuoSoftByteLimitDefault
	}
	return value
}

func (r *AudioBriefingVoiceRunner) bestEffortFailVoicing(jobID string, errorCode string, errorMessage string) {
	if r == nil || r.repo == nil || strings.TrimSpace(jobID) == "" {
		return
	}
	ctx, cancel := audioBriefingFailureContext(context.Background())
	defer cancel()
	_, _ = r.repo.FailVoicingJob(ctx, jobID, errorCode, errorMessage)
}

func (r *AudioBriefingVoiceRunner) bestEffortMarkChunkFailed(chunkID string, errorMessage string) {
	if r == nil || r.repo == nil || strings.TrimSpace(chunkID) == "" {
		return
	}
	ctx, cancel := audioBriefingFailureContext(context.Background())
	defer cancel()
	_ = r.repo.MarkChunkFailed(ctx, chunkID, errorMessage)
}

func (r *AudioBriefingVoiceRunner) loadAivisAPIKey(ctx context.Context, userID string) (*string, error) {
	if r == nil || r.userSettings == nil {
		return nil, nil
	}
	enc, err := r.userSettings.GetAivisAPIKeyEncrypted(ctx, userID)
	if err != nil || enc == nil || strings.TrimSpace(*enc) == "" {
		return nil, err
	}
	if r.cipher == nil || !r.cipher.Enabled() {
		return nil, fmt.Errorf("user secret encryption is not configured")
	}
	key, err := r.cipher.DecryptString(*enc)
	if err != nil {
		return nil, err
	}
	key = strings.TrimSpace(key)
	if key == "" {
		return nil, nil
	}
	return &key, nil
}

func audioBriefingChunkObjectKey(userID string, jobID string, seq int) string {
	return path.Join("audio-briefings", userID, jobID, fmt.Sprintf("chunk-%03d", seq))
}

func audioBriefingChunkHeartbeatURL(chunkID string) string {
	baseURL := audioBriefingCallbackBaseURL(AudioBriefingConcatModeFromEnv())
	if baseURL == "" {
		return ""
	}
	return strings.TrimRight(baseURL, "/") + "/api/internal/audio-briefings/chunks/" + strings.TrimSpace(chunkID) + "/heartbeat"
}

type audioBriefingVoicingChunkSelection string

const (
	audioBriefingVoicingChunkSelectionProcess  audioBriefingVoicingChunkSelection = "process"
	audioBriefingVoicingChunkSelectionWaiting  audioBriefingVoicingChunkSelection = "waiting"
	audioBriefingVoicingChunkSelectionComplete audioBriefingVoicingChunkSelection = "complete"
)

func nextAudioBriefingVoicingChunk(chunks []model.AudioBriefingScriptChunk, now time.Time) (audioBriefingVoicingChunkSelection, *model.AudioBriefingScriptChunk, bool) {
	retryAfter := audioBriefingChunkRetryAfter()
	for i := range chunks {
		chunk := &chunks[i]
		if chunk.R2AudioObjectKey != nil && strings.TrimSpace(*chunk.R2AudioObjectKey) != "" && chunk.TTSStatus == "generated" {
			continue
		}
		status := strings.TrimSpace(chunk.TTSStatus)
		if status == "exhausted" {
			return audioBriefingVoicingChunkSelectionWaiting, nil, false
		}
		if status == "generating" && now.Sub(audioBriefingChunkHeartbeatAt(*chunk)) < retryAfter {
			return audioBriefingVoicingChunkSelectionWaiting, nil, false
		}
		if status == "pending" || status == "retry_wait" || status == "failed" || status == "generating" {
			return audioBriefingVoicingChunkSelectionProcess, chunk, status == "generating"
		}
	}
	return audioBriefingVoicingChunkSelectionComplete, nil, false
}

func audioBriefingChunkHeartbeatAt(chunk model.AudioBriefingScriptChunk) time.Time {
	if chunk.LastHeartbeatAt != nil && !chunk.LastHeartbeatAt.IsZero() {
		return *chunk.LastHeartbeatAt
	}
	return chunk.UpdatedAt
}

func audioBriefingChunkRetryAfter() time.Duration {
	if raw := strings.TrimSpace(os.Getenv("AUDIO_BRIEFING_CHUNK_RETRY_AFTER_SEC")); raw != "" {
		if sec, err := strconv.Atoi(raw); err == nil && sec > 0 {
			return time.Duration(sec) * time.Second
		}
	}
	return 15 * time.Minute
}

func timeutilNow() time.Time {
	return time.Now()
}
