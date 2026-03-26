package service

import (
	"strings"

	"github.com/enjoydarts/sifto/api/internal/model"
)

const (
	audioBriefingListTabPublished = "published"
	audioBriefingListTabArchived  = "archived"
	audioBriefingListTabPending   = "pending"
	audioBriefingListTabStorage   = "storage"
)

func NormalizeAudioBriefingArchiveStatus(status string) string {
	if strings.TrimSpace(status) == model.AudioBriefingArchiveStatusArchived {
		return model.AudioBriefingArchiveStatusArchived
	}
	return model.AudioBriefingArchiveStatusActive
}

func AudioBriefingListTabMatches(job *model.AudioBriefingJob, tab string) bool {
	if job == nil {
		return false
	}
	switch strings.TrimSpace(tab) {
	case "", audioBriefingListTabPublished:
		return audioBriefingListTab(job) == audioBriefingListTabPublished
	case audioBriefingListTabArchived:
		return audioBriefingListTab(job) == audioBriefingListTabArchived
	case audioBriefingListTabPending:
		return audioBriefingListTab(job) == audioBriefingListTabPending
	case audioBriefingListTabStorage:
		return audioBriefingListTab(job) == audioBriefingListTabStorage
	default:
		return false
	}
}

func AudioBriefingArchiveAllowed(job *model.AudioBriefingJob) bool {
	if job == nil || audioBriefingJobIsStoredInIA(job) {
		return false
	}
	return job.Status == "published" && NormalizeAudioBriefingArchiveStatus(job.ArchiveStatus) == model.AudioBriefingArchiveStatusActive
}

func AudioBriefingUnarchiveAllowed(job *model.AudioBriefingJob) bool {
	if job == nil || audioBriefingJobIsStoredInIA(job) {
		return false
	}
	return job.Status == "published" && NormalizeAudioBriefingArchiveStatus(job.ArchiveStatus) == model.AudioBriefingArchiveStatusArchived
}

func audioBriefingListTab(job *model.AudioBriefingJob) string {
	if audioBriefingJobIsStoredInIA(job) {
		return audioBriefingListTabStorage
	}
	if NormalizeAudioBriefingArchiveStatus(job.ArchiveStatus) == model.AudioBriefingArchiveStatusArchived {
		return audioBriefingListTabArchived
	}
	if job.Status == "published" {
		return audioBriefingListTabPublished
	}
	return audioBriefingListTabPending
}

func audioBriefingJobIsStoredInIA(job *model.AudioBriefingJob) bool {
	if job == nil {
		return false
	}
	iaBucket := strings.TrimSpace(AudioBriefingIABucketFromEnv())
	if iaBucket == "" {
		return false
	}
	return NormalizeAudioBriefingStorageBucket(job.R2StorageBucket) == iaBucket || strings.TrimSpace(job.R2StorageBucket) == iaBucket
}
