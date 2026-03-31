package repository

import (
	"strings"
	"testing"
)

func TestListStaleVoicingJobsQueryIncludesArchiveStatus(t *testing.T) {
	query := listStaleVoicingJobsQuery()

	if !strings.Contains(query, "j.archive_status") {
		t.Fatalf("stale voicing query must select archive_status for scanAudioBriefingJob compatibility: %s", query)
	}
	if !strings.Contains(query, "j.conversation_mode") {
		t.Fatalf("stale voicing query must select conversation_mode for scanAudioBriefingJob compatibility: %s", query)
	}
	if !strings.Contains(query, "j.partner_persona") {
		t.Fatalf("stale voicing query must select partner_persona for scanAudioBriefingJob compatibility: %s", query)
	}
	if !strings.Contains(query, "j.pipeline_stage") {
		t.Fatalf("stale voicing query must select pipeline_stage for scanAudioBriefingJob compatibility: %s", query)
	}
}
