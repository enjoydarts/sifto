package repository

import (
	"strings"
	"testing"
)

func TestListPodcastPublishedJobsByUserQueryOrdersByCreatedAtDesc(t *testing.T) {
	query := listPodcastPublishedJobsByUserQuery()

	if strings.Contains(query, "ORDER BY published_at DESC") {
		t.Fatalf("podcast feed query must not order by published_at: %s", query)
	}
	if !strings.Contains(query, "ORDER BY created_at DESC") {
		t.Fatalf("podcast feed query must order by created_at desc: %s", query)
	}
}
