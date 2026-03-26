package service

import (
	"testing"
	"time"

	"github.com/enjoydarts/sifto/api/internal/model"
)

func TestAudioBriefingListTabMatches(t *testing.T) {
	t.Setenv("AUDIO_BRIEFING_R2_IA_BUCKET", "briefings-ia")

	tests := []struct {
		name string
		job  model.AudioBriefingJob
		tab  string
		want bool
	}{
		{
			name: "published active appears in published tab",
			job: model.AudioBriefingJob{
				Status:          "published",
				ArchiveStatus:   model.AudioBriefingArchiveStatusActive,
				R2StorageBucket: "briefings-standard",
			},
			tab:  audioBriefingListTabPublished,
			want: true,
		},
		{
			name: "published archived appears in archived tab",
			job: model.AudioBriefingJob{
				Status:          "published",
				ArchiveStatus:   model.AudioBriefingArchiveStatusArchived,
				R2StorageBucket: "briefings-standard",
			},
			tab:  audioBriefingListTabArchived,
			want: true,
		},
		{
			name: "non published active appears in pending tab",
			job: model.AudioBriefingJob{
				Status:          "failed",
				ArchiveStatus:   model.AudioBriefingArchiveStatusActive,
				R2StorageBucket: "briefings-standard",
			},
			tab:  audioBriefingListTabPending,
			want: true,
		},
		{
			name: "ia bucket appears in storage tab",
			job: model.AudioBriefingJob{
				Status:          "published",
				ArchiveStatus:   model.AudioBriefingArchiveStatusActive,
				R2StorageBucket: "briefings-ia",
			},
			tab:  audioBriefingListTabStorage,
			want: true,
		},
		{
			name: "archived does not appear in published tab",
			job: model.AudioBriefingJob{
				Status:          "published",
				ArchiveStatus:   model.AudioBriefingArchiveStatusArchived,
				R2StorageBucket: "briefings-standard",
			},
			tab:  audioBriefingListTabPublished,
			want: false,
		},
		{
			name: "pending does not appear in archived tab",
			job: model.AudioBriefingJob{
				Status:          "failed",
				ArchiveStatus:   model.AudioBriefingArchiveStatusActive,
				R2StorageBucket: "briefings-standard",
			},
			tab:  audioBriefingListTabArchived,
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := AudioBriefingListTabMatches(&tt.job, tt.tab); got != tt.want {
				t.Fatalf("AudioBriefingListTabMatches(%q) = %v, want %v", tt.tab, got, tt.want)
			}
		})
	}
}

func TestAudioBriefingJobIsPodcastEligibleExcludesArchived(t *testing.T) {
	t.Setenv("AUDIO_BRIEFING_PUBLIC_BUCKET", "briefings-public")
	t.Setenv("AUDIO_BRIEFING_IA_MOVE_AFTER_DAYS", "30")

	now := time.Date(2026, 3, 26, 10, 0, 0, 0, time.UTC)
	publishedAt := now.Add(-24 * time.Hour)

	activeJob := &model.AudioBriefingJob{
		Status:                 "published",
		ArchiveStatus:          model.AudioBriefingArchiveStatusActive,
		PublishedAt:            &publishedAt,
		PodcastPublicBucket:    "briefings-public",
		PodcastPublicObjectKey: stringPtr("podcasts/audio/user/job.mp3"),
	}
	if !AudioBriefingJobIsPodcastEligible(activeJob, now) {
		t.Fatalf("active published job should be podcast eligible")
	}

	archivedJob := *activeJob
	archivedJob.ArchiveStatus = model.AudioBriefingArchiveStatusArchived
	if AudioBriefingJobIsPodcastEligible(&archivedJob, now) {
		t.Fatalf("archived job should not be podcast eligible")
	}
}

func TestAudioBriefingArchiveActionsAllowed(t *testing.T) {
	t.Setenv("AUDIO_BRIEFING_R2_IA_BUCKET", "briefings-ia")

	published := &model.AudioBriefingJob{
		Status:          "published",
		ArchiveStatus:   model.AudioBriefingArchiveStatusActive,
		R2StorageBucket: "briefings-standard",
	}
	if !AudioBriefingArchiveAllowed(published) {
		t.Fatalf("published active job should allow archive")
	}

	archived := &model.AudioBriefingJob{
		Status:          "published",
		ArchiveStatus:   model.AudioBriefingArchiveStatusArchived,
		R2StorageBucket: "briefings-standard",
	}
	if !AudioBriefingUnarchiveAllowed(archived) {
		t.Fatalf("published archived job should allow unarchive")
	}

	iaJob := &model.AudioBriefingJob{
		Status:          "published",
		ArchiveStatus:   model.AudioBriefingArchiveStatusArchived,
		R2StorageBucket: "briefings-ia",
	}
	if AudioBriefingArchiveAllowed(iaJob) || AudioBriefingUnarchiveAllowed(iaJob) {
		t.Fatalf("ia job should not allow archive toggles")
	}

	pending := &model.AudioBriefingJob{
		Status:          "failed",
		ArchiveStatus:   model.AudioBriefingArchiveStatusActive,
		R2StorageBucket: "briefings-standard",
	}
	if AudioBriefingArchiveAllowed(pending) || AudioBriefingUnarchiveAllowed(pending) {
		t.Fatalf("non-published job should not allow archive toggles")
	}
}
