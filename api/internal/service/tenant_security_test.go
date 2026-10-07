package service

import (
	"context"
	"errors"
	"github.com/enjoydarts/sifto/api/internal/repository"
	"testing"
	"time"

	"github.com/enjoydarts/sifto/api/internal/model"
)

func TestUnauthorizedVoiceStartCannotFailVictimsJob(t *testing.T) {
	ctx := context.Background()
	db, err := repository.NewPool(ctx)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(db.Close)
	lockSettingsServiceTestDB(t, db)
	const victim = "00000000-0000-4000-8000-000000000201"
	const attacker = "00000000-0000-4000-8000-000000000202"
	for _, user := range []string{victim, attacker} {
		if _, err := db.Exec(ctx, `INSERT INTO users (id,email) VALUES ($1,$2) ON CONFLICT (id) DO NOTHING`, user, user+"@security.test"); err != nil {
			t.Fatal(err)
		}
	}
	t.Cleanup(func() { db.Exec(ctx, `DELETE FROM users WHERE id=ANY($1::uuid[])`, []string{victim, attacker}) })
	repo := repository.NewAudioBriefingRepo(db)
	job, err := repo.CreateJobWithContent(ctx, victim, time.Now(), "security-test-"+time.Now().Format("150405.000000000"), "editor", "scripted", nil, 0, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Exec(ctx, `DELETE FROM audio_briefing_jobs WHERE id=$1`, job.ID) })
	runner := &AudioBriefingVoiceRunner{repo: repo, worker: &WorkerClient{}}
	if _, err := runner.Start(ctx, attacker, job.ID); !errors.Is(err, repository.ErrNotFound) {
		t.Fatalf("err=%v", err)
	}
	actual, err := repo.GetJobByID(ctx, victim, job.ID)
	if err != nil || actual.Status != "scripted" {
		t.Fatalf("victim job=%+v err=%v", actual, err)
	}
}

func TestObsidianRepositoryGrantCannotBeChangedBySettings(t *testing.T) {
	svc := newSettingsServiceForTest(t)
	ctx := context.Background()
	const user = "00000000-0000-4000-8000-000000000021"
	cfg, err := svc.obsidianRepo.UpsertInstallation(ctx, user, 42, strptr("me"), []string{"me/vault"})
	if err != nil {
		t.Fatal(err)
	}
	if len(cfg.GitHubAuthorizedRepositories) != 1 {
		t.Fatal("repository grant not persisted")
	}
	if !NewObsidianExportView(cfg, nil).GitHubAuthorized {
		t.Fatal("verified installation should be connected")
	}
	legacy := *cfg
	legacy.GitHubAuthorizedRepositories = nil
	if NewObsidianExportView(&legacy, nil).GitHubAuthorized {
		t.Fatal("legacy installation must require reauthorization")
	}
	if _, err := svc.UpdateObsidianExport(ctx, user, UpdateObsidianExportInput{Enabled: true, GitHubRepoOwner: strptr("victim"), GitHubRepoName: strptr("vault")}); err == nil {
		t.Fatal("unauthorized repository accepted")
	}
}

func TestRandomAudioJobUsesOnlyConfiguredPersona(t *testing.T) {
	svc := newSettingsServiceForTest(t)
	ctx := context.Background()
	const user = "00000000-0000-4000-8000-000000000021"
	_, err := svc.audioBriefingRepo.UpsertPersonaVoices(ctx, user, []model.AudioBriefingPersonaVoice{{Persona: "editor", TTSProvider: "xai", VoiceModel: "ara", SpeechRate: 1, EmotionalIntensity: 1, TempoDynamics: 1, LineBreakSilenceSeconds: 0.4}})
	if err != nil {
		t.Fatal(err)
	}
	o := &AudioBriefingOrchestrator{repo: svc.audioBriefingRepo}
	job, err := o.createPendingJob(ctx, user, &model.AudioBriefingSettings{DefaultPersonaMode: PersonaModeRandom, ConversationMode: "solo"}, time.Now(), "security-random-voice", "unconfigured")
	if err != nil {
		t.Fatal(err)
	}
	if job.Persona != "editor" {
		t.Fatalf("unconfigured persona selected: %s", job.Persona)
	}
}
