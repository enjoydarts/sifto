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
	ErrSummaryAudioMissingVoice   = errors.New("summary audio persona voice is not configured")
)

type SummaryAudioSynthesis struct {
	Item         *model.ItemDetail `json:"item,omitempty"`
	Persona      string            `json:"persona"`
	AudioBase64  string            `json:"audio_base64"`
	ContentType  string            `json:"content_type"`
	DurationSec  int               `json:"duration_sec"`
	ResolvedText string            `json:"resolved_text"`
}

type SummaryAudioPlayerService struct {
	items        *repository.ItemRepo
	audio        *repository.AudioBriefingRepo
	userSettings *repository.UserSettingsRepo
	cipher       *SecretCipher
	worker       *WorkerClient
}

func NewSummaryAudioPlayerService(
	items *repository.ItemRepo,
	audio *repository.AudioBriefingRepo,
	userSettings *repository.UserSettingsRepo,
	cipher *SecretCipher,
	worker *WorkerClient,
) *SummaryAudioPlayerService {
	return &SummaryAudioPlayerService{
		items:        items,
		audio:        audio,
		userSettings: userSettings,
		cipher:       cipher,
		worker:       worker,
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
	if s == nil || s.items == nil || s.audio == nil || s.userSettings == nil || s.worker == nil {
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
	settings, err := s.audio.EnsureSettingsDefaults(ctx, userID)
	if err != nil {
		return nil, err
	}
	persona := ResolvePersona(settings.DefaultPersonaMode, settings.DefaultPersona)
	voice, err := s.audio.GetPersonaVoice(ctx, userID, persona)
	if err != nil {
		return nil, err
	}
	if voice == nil {
		return nil, ErrSummaryAudioMissingVoice
	}
	narration := BuildSummaryAudioNarration(derefString(item.TranslatedTitle), derefString(item.Title), summaryText)
	var aivisAPIKey *string
	var aivisUserDictionaryUUID *string
	if strings.EqualFold(strings.TrimSpace(voice.TTSProvider), "aivis") {
		aivisAPIKey, err = s.loadAivisAPIKey(ctx, userID)
		if err != nil {
			return nil, err
		}
		aivisUserDictionaryUUID, err = s.userSettings.GetAivisUserDictionaryUUID(ctx, userID)
		if err != nil {
			return nil, err
		}
	}
	resp, err := s.worker.SynthesizeSummaryAudio(
		ctx,
		voice.TTSProvider,
		voice.VoiceModel,
		voice.VoiceStyle,
		narration,
		voice.SpeechRate,
		voice.EmotionalIntensity,
		voice.TempoDynamics,
		voice.LineBreakSilenceSeconds,
		settings.ChunkTrailingSilenceSeconds,
		voice.Pitch,
		voice.VolumeGain,
		aivisUserDictionaryUUID,
		aivisAPIKey,
	)
	if err != nil {
		return nil, err
	}
	return &SummaryAudioSynthesis{
		Item:         item,
		Persona:      persona,
		AudioBase64:  resp.AudioBase64,
		ContentType:  resp.ContentType,
		DurationSec:  resp.DurationSec,
		ResolvedText: resp.ResolvedText,
	}, nil
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
