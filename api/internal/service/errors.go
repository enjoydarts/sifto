package service

import (
	"errors"
	"fmt"
)

type ValidationError struct {
	Field   string
	Message string
}

func (e *ValidationError) Error() string {
	if e.Message != "" {
		return e.Message
	}
	return "invalid " + e.Field
}

type ModelValidationError struct {
	SettingKey string
	Missing    bool
}

func (e *ModelValidationError) Error() string {
	if e.Missing {
		return fmt.Sprintf("model missing required capability for %s", e.SettingKey)
	}
	return fmt.Sprintf("invalid model for %s", e.SettingKey)
}

type DuplicatePersonaVoiceError struct {
	Persona string
}

func (e *DuplicatePersonaVoiceError) Error() string {
	return fmt.Sprintf("duplicate persona voice: %s", e.Persona)
}

var (
	ErrAivisModelsNotSynced          = errors.New("aivis models are not synced")
	ErrSecretEncryptionNotConfigured = errors.New("user secret encryption is not configured")
	ErrAivisAPIKeyNotConfigured      = errors.New("aivis api key is not configured")
	ErrInoreaderOAuthNotConfigured   = errors.New("inoreader oauth is not configured")
	ErrAivisDictionaryUUIDRequired   = errors.New("aivis_user_dictionary_uuid is required")
	ErrInvalidEmbeddingModel         = errors.New("invalid embedding model")
	ErrInvalidKeywordLinkMode        = errors.New("invalid keyword_link_mode")
	ErrUnsupportedArtworkContentType = errors.New("unsupported podcast artwork content_type")
	ErrPublicBaseURLNotConfigured    = errors.New("AUDIO_BRIEFING_PUBLIC_BASE_URL is not configured")
	ErrPublicBucketNotConfigured     = errors.New("AUDIO_BRIEFING_PUBLIC_BUCKET is not configured")
)

func IsUserError(err error) bool {
	var ve *ValidationError
	var me *ModelValidationError
	var dpv *DuplicatePersonaVoiceError
	return errors.As(err, &ve) || errors.As(err, &me) || errors.As(err, &dpv) ||
		errors.Is(err, ErrAivisModelsNotSynced) ||
		errors.Is(err, ErrInvalidEmbeddingModel)
}
