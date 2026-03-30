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
}
