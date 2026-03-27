package repository

import (
	"strings"
	"testing"
)

func TestLatestPlaybackSessionByModeQueryOrdersByUpdatedAtDesc(t *testing.T) {
	query := latestPlaybackSessionByModeQuery()
	if !strings.Contains(query, "ORDER BY updated_at DESC") {
		t.Fatalf("latestPlaybackSessionByModeQuery() must order by updated_at desc: %s", query)
	}
}

func TestListPlaybackSessionsQuerySupportsModeAndStatusFilters(t *testing.T) {
	query := listPlaybackSessionsQuery()
	if !strings.Contains(query, "($2 = '' OR mode = $2)") {
		t.Fatalf("listPlaybackSessionsQuery() must filter by mode: %s", query)
	}
	if !strings.Contains(query, "($3 = '' OR status = $3)") {
		t.Fatalf("listPlaybackSessionsQuery() must filter by status: %s", query)
	}
	if !strings.Contains(query, "ORDER BY updated_at DESC") {
		t.Fatalf("listPlaybackSessionsQuery() must order by updated_at desc: %s", query)
	}
}
