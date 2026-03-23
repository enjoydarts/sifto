package repository

import (
	"strings"
	"testing"
)

func TestReviewQueueListDueQueryCastsLegacyTextUserIDToUUID(t *testing.T) {
	query := reviewQueueListDueQuery()

	if !strings.Contains(query, "s.user_id = rq.user_id::uuid") {
		t.Fatalf("review queue query must cast rq.user_id to uuid for source join")
	}
	if !strings.Contains(query, "n.user_id = rq.user_id") {
		t.Fatalf("review queue query must keep item_notes join on legacy text user_id")
	}
}

func TestWeeklyReviewQueriesUseReadAtInsteadOfMissingCreatedAt(t *testing.T) {
	queries := []string{
		weeklyReviewReadCountQuery(),
		weeklyReviewTopicsQuery(),
	}

	for _, query := range queries {
		if strings.Contains(query, "ir.created_at") {
			t.Fatalf("weekly review query must not reference ir.created_at: %s", query)
		}
		if !strings.Contains(query, "ir.read_at") {
			t.Fatalf("weekly review query must reference ir.read_at: %s", query)
		}
	}
}
