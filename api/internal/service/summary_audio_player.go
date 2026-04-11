package service

import (
	"context"
	"errors"
	"strings"

	"github.com/enjoydarts/sifto/api/internal/model"
	"github.com/enjoydarts/sifto/api/internal/repository"
)

var (
	ErrSummaryAudioMissingSummary = errors.New("summary audio summary is not available")
	ErrSummaryAudioMissingVoice   = errors.New("summary audio voice is not configured")
	ErrSummaryAudioMissingModel   = errors.New("summary audio model is not configured")
)

type SummaryAudioSynthesis struct {
	Item             *model.ItemDetail `json:"item,omitempty"`
	Persona          string            `json:"persona"`
	AudioBase64      string            `json:"audio_base64"`
	ContentType      string            `json:"content_type"`
	DurationSec      int               `json:"duration_sec"`
	ResolvedText     string            `json:"resolved_text"`
	PreprocessedText *string           `json:"preprocessed_text,omitempty"`
}

type SummaryAudioPlayerService struct {
	items        *repository.ItemRepo
	summaryAudio *repository.SummaryAudioVoiceSettingsRepo
	userRepo     *repository.UserRepo
	userSettings *repository.UserSettingsRepo
	cipher       *SecretCipher
	worker       summaryAudioSynthesizer
	preprocess   summaryAudioTTSMarkupPreprocessor
}

type summaryAudioSynthesizer interface {
	SynthesizeSummaryAudio(
		ctx context.Context,
		provider string,
		voiceModel string,
		voiceStyle string,
		ttsModel string,
		azureSpeechRegion string,
		text string,
		speechRate float64,
		emotionalIntensity float64,
		tempoDynamics float64,
		lineBreakSilenceSeconds float64,
		chunkTrailingSilenceSeconds float64,
		pitch float64,
		volumeGain float64,
		aivisUserDictionaryUUID *string,
		aivisAPIKey *string,
		fishAudioAPIKey *string,
		elevenLabsAPIKey *string,
		googleAPIKey *string,
		xaiAPIKey *string,
		openAIAPIKey *string,
		azureSpeechAPIKey *string,
	) (*SummaryAudioSynthesizeResponse, error)
}

type summaryAudioTTSMarkupPreprocessor interface {
	PreprocessSummaryAudioText(ctx context.Context, userID, itemID, text string) (*TTSMarkupPreprocessResult, error)
	PreprocessSummaryAudioTextForProvider(ctx context.Context, userID, itemID, provider, text string) (*TTSMarkupPreprocessResult, error)
	PreprocessSummaryAudioTextForProviderWithVariables(ctx context.Context, userID, itemID, provider, text string, variables map[string]string) (*TTSMarkupPreprocessResult, error)
}

func NewSummaryAudioPlayerService(
	items *repository.ItemRepo,
	summaryAudio *repository.SummaryAudioVoiceSettingsRepo,
	userRepo *repository.UserRepo,
	userSettings *repository.UserSettingsRepo,
	cipher *SecretCipher,
	worker summaryAudioSynthesizer,
	preprocess summaryAudioTTSMarkupPreprocessor,
) *SummaryAudioPlayerService {
	return &SummaryAudioPlayerService{
		items:        items,
		summaryAudio: summaryAudio,
		userRepo:     userRepo,
		userSettings: userSettings,
		cipher:       cipher,
		worker:       worker,
		preprocess:   preprocess,
	}
}

func BuildSummaryAudioNarration(translatedTitle, originalTitle, summary string) string {
	title := strings.TrimSpace(translatedTitle)
	if title == "" {
		title = strings.TrimSpace(originalTitle)
	}
	return title + "\n\n" + strings.TrimSpace(summary)
}

func SummaryAudioRequestContext(parent context.Context) context.Context {
	if parent == nil {
		return context.Background()
	}
	return context.WithoutCancel(parent)
}

func (s *SummaryAudioPlayerService) Synthesize(ctx context.Context, userID, itemID string) (*SummaryAudioSynthesis, error) {
	if s == nil || s.items == nil || s.summaryAudio == nil || s.userSettings == nil || s.worker == nil {
		return nil, errors.New("summary audio service is not configured")
	}
	item, err := s.items.GetDetail(ctx, strings.TrimSpace(itemID), strings.TrimSpace(userID))
	if err != nil {
		return nil, err
	}
	summaryText := summaryAudioSummaryText(item)
	if summaryText == "" {
		return nil, ErrSummaryAudioMissingSummary
	}
	settings, err := s.summaryAudio.EnsureDefaults(ctx, userID)
	if err != nil {
		return nil, err
	}
	provider := strings.TrimSpace(settings.TTSProvider)
	voiceModel := strings.TrimSpace(settings.VoiceModel)
	voiceStyle := strings.TrimSpace(settings.VoiceStyle)
	ttsModel := strings.TrimSpace(settings.TTSModel)
	providerMetadata := LookupTTSProviderMetadata(provider)
	if provider == "" || voiceModel == "" {
		return nil, ErrSummaryAudioMissingVoice
	}
	narration := BuildSummaryAudioNarration(derefString(item.TranslatedTitle), derefString(item.Title), summaryText)
	var preprocessedText *string
	var aivisAPIKey *string
	var aivisUserDictionaryUUID *string
	var fishAudioAPIKey *string
	var elevenLabsAPIKey *string
	var googleAPIKey *string
	var xaiAPIKey *string
	var openAIAPIKey *string
	var azureSpeechAPIKey *string
	azureSpeechRegion := ""
	if strings.EqualFold(provider, "aivis") {
		aivisAPIKey, err = s.loadAivisAPIKey(ctx, userID)
		if err != nil {
			return nil, err
		}
		aivisUserDictionaryUUID = settings.AivisUserDictionaryUUID
	} else if strings.EqualFold(provider, "xai") {
		xaiAPIKey, err = s.loadXAIAPIKey(ctx, userID)
		if err != nil {
			return nil, err
		}
		if s.preprocess != nil {
			preprocessed, preprocessErr := s.preprocess.PreprocessSummaryAudioTextForProvider(ctx, userID, item.ID, provider, narration)
			if preprocessErr != nil {
				return nil, preprocessErr
			}
			preprocessedText = stringPtrOrNil(preprocessed.Text)
			narration = preprocessed.Text
		}
	} else if strings.EqualFold(provider, "fish") {
		if providerMetadata.SummaryRequiresTTSModel && ttsModel == "" {
			return nil, ErrSummaryAudioMissingModel
		}
		fishAudioAPIKey, err = loadAndDecryptAudioBriefingUserSecret(ctx, s.userSettings.GetFishAudioAPIKeyEncrypted, s.cipher, userID, "fish api key is not configured")
		if err != nil {
			return nil, err
		}
		if s.preprocess != nil {
			preprocessed, preprocessErr := s.preprocess.PreprocessSummaryAudioTextForProvider(ctx, userID, item.ID, provider, narration)
			if preprocessErr != nil {
				return nil, preprocessErr
			}
			preprocessedText = stringPtrOrNil(preprocessed.Text)
			narration = preprocessed.Text
		}
	} else if strings.EqualFold(provider, "gemini_tts") {
		if providerMetadata.SummaryRequiresTTSModel && ttsModel == "" {
			return nil, ErrSummaryAudioMissingModel
		}
		if err := EnsureGeminiTTSEnabledForUser(ctx, s.userRepo, userID); err != nil {
			return nil, err
		}
		if s.preprocess != nil {
			preprocessed, preprocessErr := s.preprocess.PreprocessSummaryAudioTextForProvider(ctx, userID, item.ID, provider, narration)
			if preprocessErr != nil {
				return nil, preprocessErr
			}
			preprocessedText = stringPtrOrNil(preprocessed.Text)
			narration = preprocessed.Text
		}
	} else if strings.EqualFold(provider, "elevenlabs") {
		if providerMetadata.SummaryRequiresTTSModel && ttsModel == "" {
			return nil, ErrSummaryAudioMissingModel
		}
		elevenLabsAPIKey, err = loadAndDecryptAudioBriefingUserSecret(ctx, s.userSettings.GetElevenLabsAPIKeyEncrypted, s.cipher, userID, "elevenlabs api key is not configured")
		if err != nil {
			return nil, err
		}
		if s.preprocess != nil {
			preprocessed, preprocessErr := s.preprocess.PreprocessSummaryAudioTextForProvider(ctx, userID, item.ID, provider, narration)
			if preprocessErr != nil {
				return nil, preprocessErr
			}
			preprocessedText = stringPtrOrNil(preprocessed.Text)
			narration = preprocessed.Text
		}
	} else if strings.EqualFold(provider, "openai") {
		if providerMetadata.SummaryRequiresTTSModel && ttsModel == "" {
			return nil, ErrSummaryAudioMissingModel
		}
		openAIAPIKey, err = loadAndDecryptAudioBriefingUserSecret(ctx, s.userSettings.GetOpenAIAPIKeyEncrypted, s.cipher, userID, "openai api key is not configured")
		if err != nil {
			return nil, err
		}
	} else if strings.EqualFold(provider, "azure_speech") {
		azureSpeechAPIKey, err = loadAndDecryptAudioBriefingUserSecret(ctx, s.userSettings.GetAzureSpeechAPIKeyEncrypted, s.cipher, userID, "azure speech api key is not configured")
		if err != nil {
			return nil, err
		}
		region, regionErr := s.userSettings.GetAzureSpeechRegion(ctx, userID)
		if regionErr != nil {
			return nil, regionErr
		}
		azureSpeechRegion = strings.TrimSpace(derefString(region))
		if azureSpeechRegion == "" {
			return nil, errors.New("azure speech region is not configured")
		}
		if s.preprocess != nil {
			preprocessed, preprocessErr := s.preprocess.PreprocessSummaryAudioTextForProviderWithVariables(ctx, userID, item.ID, provider, narration, map[string]string{
				"voice_name":   strings.TrimSpace(voiceModel),
				"voice_locale": "ja-JP",
			})
			if preprocessErr != nil {
				return nil, preprocessErr
			}
			preprocessedText = stringPtrOrNil(preprocessed.Text)
			narration = preprocessed.Text
		}
	}
	resp, err := s.worker.SynthesizeSummaryAudio(
		ctx,
		provider,
		voiceModel,
		voiceStyle,
		ttsModel,
		azureSpeechRegion,
		narration,
		settings.SpeechRate,
		settings.EmotionalIntensity,
		settings.TempoDynamics,
		settings.LineBreakSilenceSeconds,
		1.0,
		settings.Pitch,
		settings.VolumeGain,
		aivisUserDictionaryUUID,
		aivisAPIKey,
		fishAudioAPIKey,
		elevenLabsAPIKey,
		googleAPIKey,
		xaiAPIKey,
		openAIAPIKey,
		azureSpeechAPIKey,
	)
	if err != nil {
		return nil, err
	}
	return &SummaryAudioSynthesis{
		Item:             item,
		Persona:          "",
		AudioBase64:      resp.AudioBase64,
		ContentType:      resp.ContentType,
		DurationSec:      resp.DurationSec,
		ResolvedText:     resp.ResolvedText,
		PreprocessedText: preprocessedText,
	}, nil
}

func (s *SummaryAudioPlayerService) loadXAIAPIKey(ctx context.Context, userID string) (*string, error) {
	if s == nil || s.userSettings == nil || s.cipher == nil {
		return nil, errors.New("summary audio xai key loader is not configured")
	}
	enc, err := s.userSettings.GetXAIAPIKeyEncrypted(ctx, userID)
	if err != nil {
		return nil, err
	}
	if enc == nil || strings.TrimSpace(*enc) == "" {
		return nil, errors.New("xai api key is not configured")
	}
	plain, err := s.cipher.DecryptString(*enc)
	if err != nil {
		return nil, err
	}
	plain = strings.TrimSpace(plain)
	if plain == "" {
		return nil, errors.New("xai api key is empty")
	}
	return &plain, nil
}

func (s *SummaryAudioPlayerService) loadAivisAPIKey(ctx context.Context, userID string) (*string, error) {
	if s == nil || s.userSettings == nil || s.cipher == nil {
		return nil, errors.New("summary audio aivis key loader is not configured")
	}
	enc, err := s.userSettings.GetAivisAPIKeyEncrypted(ctx, userID)
	if err != nil {
		return nil, err
	}
	if enc == nil || strings.TrimSpace(*enc) == "" {
		return nil, errors.New("aivis api key is not configured")
	}
	plain, err := s.cipher.DecryptString(*enc)
	if err != nil {
		return nil, err
	}
	plain = strings.TrimSpace(plain)
	if plain == "" {
		return nil, errors.New("aivis api key is empty")
	}
	return &plain, nil
}

func summaryAudioSummaryText(item *model.ItemDetail) string {
	if item == nil {
		return ""
	}
	if item.Summary != nil {
		if text := strings.TrimSpace(item.Summary.Summary); text != "" {
			return text
		}
	}
	return strings.TrimSpace(derefString(item.Item.Summary))
}
