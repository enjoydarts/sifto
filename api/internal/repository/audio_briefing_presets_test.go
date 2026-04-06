package repository

import (
	"context"
	"testing"
	"time"

	"github.com/enjoydarts/sifto/api/internal/model"
	"github.com/jackc/pgx/v5/pgxpool"
)

func TestAudioBriefingPresetRepoCreateListUpdateDelete(t *testing.T) {
	db := testAudioBriefingPresetDB(t)
	repo := NewAudioBriefingPresetRepo(db)
	ctx := context.Background()
	userID := "00000000-0000-4000-8000-000000000031"

	now := time.Date(2026, 4, 6, 12, 0, 0, 0, time.UTC)
	created, err := repo.Create(ctx, model.AudioBriefingPreset{
		UserID:             userID,
		Name:               "Morning",
		DefaultPersonaMode: "fixed",
		DefaultPersona:     "editor",
		ConversationMode:   "single",
		Voices: []model.AudioBriefingPersonaVoice{
			{Persona: "editor", TTSProvider: "xai", VoiceModel: "voice-1", CreatedAt: now, UpdatedAt: now},
		},
	})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if created.Name != "Morning" || len(created.Voices) != 1 {
		t.Fatalf("created = %#v", created)
	}

	rows, err := repo.ListByUser(ctx, userID)
	if err != nil {
		t.Fatalf("ListByUser() error = %v", err)
	}
	if len(rows) != 1 || rows[0].Name != "Morning" {
		t.Fatalf("rows = %#v", rows)
	}
	if len(rows[0].Voices) != 1 || rows[0].Voices[0].Persona != "editor" {
		t.Fatalf("voices = %#v", rows[0].Voices)
	}

	updated, err := repo.Update(ctx, model.AudioBriefingPreset{
		ID:                 created.ID,
		UserID:             userID,
		Name:               "Morning v2",
		DefaultPersonaMode: "random",
		DefaultPersona:     "host",
		ConversationMode:   "duo",
		Voices: []model.AudioBriefingPersonaVoice{
			{Persona: "host", TTSProvider: "gemini_tts", TTSModel: "gemini-tts", VoiceModel: "Kore", CreatedAt: now, UpdatedAt: now},
		},
	})
	if err != nil {
		t.Fatalf("Update() error = %v", err)
	}
	if updated.Name != "Morning v2" || len(updated.Voices) != 1 || updated.Voices[0].Persona != "host" {
		t.Fatalf("updated = %#v", updated)
	}

	if err := repo.Delete(ctx, userID, created.ID); err != nil {
		t.Fatalf("Delete() error = %v", err)
	}
	rows, err = repo.ListByUser(ctx, userID)
	if err != nil {
		t.Fatalf("ListByUser() after delete error = %v", err)
	}
	if len(rows) != 0 {
		t.Fatalf("rows after delete = %#v, want empty", rows)
	}
}

func TestAudioBriefingPresetRepoNameMustBeUniquePerUser(t *testing.T) {
	db := testAudioBriefingPresetDB(t)
	repo := NewAudioBriefingPresetRepo(db)
	ctx := context.Background()

	if _, err := repo.Create(ctx, model.AudioBriefingPreset{
		UserID:           "00000000-0000-4000-8000-000000000031",
		Name:             "Daily",
		ConversationMode: "single",
	}); err != nil {
		t.Fatalf("Create(first) error = %v", err)
	}
	if _, err := repo.Create(ctx, model.AudioBriefingPreset{
		UserID:           "00000000-0000-4000-8000-000000000031",
		Name:             "Daily",
		ConversationMode: "single",
	}); err != ErrConflict {
		t.Fatalf("Create(duplicate) error = %v, want ErrConflict", err)
	}
	if _, err := repo.Create(ctx, model.AudioBriefingPreset{
		UserID:           "00000000-0000-4000-8000-000000000032",
		Name:             "Daily",
		ConversationMode: "single",
	}); err != nil {
		t.Fatalf("Create(other user) error = %v, want nil", err)
	}
}

func testAudioBriefingPresetDB(t *testing.T) *pgxpool.Pool {
	t.Helper()

	pool, err := NewPool(context.Background())
	if err != nil {
		t.Fatalf("NewPool() error = %v", err)
	}
	t.Cleanup(pool.Close)
	lockAudioBriefingPresetRepoTestDB(t, pool)
	if _, err := pool.Exec(context.Background(), `
		DELETE FROM audio_briefing_preset_voices;
		DELETE FROM audio_briefing_presets;
		DELETE FROM users WHERE id IN ('00000000-0000-4000-8000-000000000031', '00000000-0000-4000-8000-000000000032');
		INSERT INTO users (id, email, name) VALUES
			('00000000-0000-4000-8000-000000000031', 'preset-1@example.com', 'Preset One'),
			('00000000-0000-4000-8000-000000000032', 'preset-2@example.com', 'Preset Two');
	`); err != nil {
		t.Fatalf("reset preset repo tables: %v", err)
	}
	return pool
}

func lockAudioBriefingPresetRepoTestDB(t *testing.T, pool *pgxpool.Pool) {
	t.Helper()

	const key int64 = 74231004
	if _, err := pool.Exec(context.Background(), `SELECT pg_advisory_lock($1)`, key); err != nil {
		t.Fatalf("pg_advisory_lock() error = %v", err)
	}
	t.Cleanup(func() {
		if _, err := pool.Exec(context.Background(), `SELECT pg_advisory_unlock($1)`, key); err != nil {
			t.Fatalf("pg_advisory_unlock() error = %v", err)
		}
	})
}
