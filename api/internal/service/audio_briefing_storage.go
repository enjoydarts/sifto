package service

import (
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/enjoydarts/sifto/api/internal/model"
)

type AudioBriefingObjectRef struct {
	Bucket    string
	ObjectKey string
}

func AudioBriefingStandardBucketFromEnv() string {
	return firstNonEmptyTrimmed(
		os.Getenv("AUDIO_BRIEFING_R2_STANDARD_BUCKET"),
		os.Getenv("AUDIO_BRIEFING_R2_BUCKET"),
	)
}

func AudioBriefingIABucketFromEnv() string {
	return strings.TrimSpace(os.Getenv("AUDIO_BRIEFING_R2_IA_BUCKET"))
}

func AudioBriefingPublicBucketFromEnv() string {
	return strings.TrimSpace(os.Getenv("AUDIO_BRIEFING_PUBLIC_BUCKET"))
}

func AudioBriefingPublicBaseURLFromEnv() string {
	return strings.TrimRight(strings.TrimSpace(os.Getenv("AUDIO_BRIEFING_PUBLIC_BASE_URL")), "/")
}

func AudioBriefingIAMoveAfterDaysFromEnv() int {
	return envIntOrDefault("AUDIO_BRIEFING_IA_MOVE_AFTER_DAYS", 30)
}

func AudioBriefingIAMoveBatchLimitFromEnv() int {
	return envIntOrDefault("AUDIO_BRIEFING_IA_MOVE_BATCH_LIMIT", 50)
}

func NormalizeAudioBriefingStorageBucket(bucket string) string {
	return firstNonEmptyTrimmed(bucket, AudioBriefingStandardBucketFromEnv())
}

func AudioBriefingPublicObjectURL(objectKey string) *string {
	key := strings.TrimLeft(strings.TrimSpace(objectKey), "/")
	base := AudioBriefingPublicBaseURLFromEnv()
	if key == "" || base == "" {
		return nil
	}
	v := base + "/" + key
	return &v
}

func AudioBriefingPodcastPublicObjectURL(bucket string, objectKey string) *string {
	if NormalizeAudioBriefingStorageBucket(bucket) == AudioBriefingPublicBucketFromEnv() || strings.TrimSpace(bucket) == AudioBriefingPublicBucketFromEnv() {
		return AudioBriefingPublicObjectURL(objectKey)
	}
	return nil
}

func AudioBriefingPodcastExpiresAt(publishedAt *time.Time) *time.Time {
	if publishedAt == nil {
		return nil
	}
	v := publishedAt.AddDate(0, 0, AudioBriefingIAMoveAfterDaysFromEnv())
	return &v
}

func AudioBriefingJobIsPodcastEligible(job *model.AudioBriefingJob, now time.Time) bool {
	if job == nil || job.PublishedAt == nil {
		return false
	}
	if NormalizeAudioBriefingArchiveStatus(job.ArchiveStatus) != model.AudioBriefingArchiveStatusActive {
		return false
	}
	if strings.TrimSpace(ptrString(job.PodcastPublicObjectKey)) == "" {
		return false
	}
	if strings.TrimSpace(job.PodcastPublicBucket) != AudioBriefingPublicBucketFromEnv() {
		return false
	}
	if job.PodcastPublicDeletedAt != nil {
		return false
	}
	expiresAt := AudioBriefingPodcastExpiresAt(job.PublishedAt)
	return expiresAt != nil && now.Before(*expiresAt)
}

func CollectAudioBriefingObjectRefs(job *model.AudioBriefingJob, chunks []model.AudioBriefingScriptChunk) []AudioBriefingObjectRef {
	seen := map[string]struct{}{}
	out := make([]AudioBriefingObjectRef, 0, len(chunks)+2)
	appendRef := func(bucket string, objectKey *string) {
		key := strings.TrimSpace(ptrString(objectKey))
		if key == "" {
			return
		}
		ref := AudioBriefingObjectRef{
			Bucket:    NormalizeAudioBriefingStorageBucket(bucket),
			ObjectKey: key,
		}
		seenKey := ref.Bucket + "\n" + ref.ObjectKey
		if _, ok := seen[seenKey]; ok {
			return
		}
		seen[seenKey] = struct{}{}
		out = append(out, ref)
	}
	if job != nil {
		appendRef(job.R2StorageBucket, job.R2AudioObjectKey)
		appendRef(job.R2StorageBucket, job.R2ManifestObjectKey)
	}
	for _, chunk := range chunks {
		appendRef(chunk.R2StorageBucket, chunk.R2AudioObjectKey)
	}
	return out
}

func firstNonEmptyTrimmed(values ...string) string {
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return ""
}

func envIntOrDefault(name string, fallback int) int {
	raw := strings.TrimSpace(os.Getenv(name))
	if raw == "" {
		return fallback
	}
	value, err := strconv.Atoi(raw)
	if err != nil || value <= 0 {
		return fallback
	}
	return value
}
