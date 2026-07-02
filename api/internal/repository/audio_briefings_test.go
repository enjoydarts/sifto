package repository

import (
	"reflect"
	"strings"
	"testing"
)

func TestAudioBriefingListTabWhereClause(t *testing.T) {
	tests := []struct {
		name      string
		tab       string
		iaBucket  string
		wantParts []string
		wantArgs  []any
		wantEmpty bool
	}{
		{
			name:      "published excludes archived and IA storage",
			tab:       "published",
			iaBucket:  "briefings-ia",
			wantParts: []string{"status = 'published'", "archive_status, ''), 'active') = 'active'", "r2_storage_bucket", "<> $2"},
			wantArgs:  []any{"briefings-ia"},
		},
		{
			name:      "archived excludes IA storage",
			tab:       "archived",
			iaBucket:  "briefings-ia",
			wantParts: []string{"archive_status, ''), 'active') = 'archived'", "r2_storage_bucket", "<> $2"},
			wantArgs:  []any{"briefings-ia"},
		},
		{
			name:      "pending excludes published archived and IA storage",
			tab:       "pending",
			iaBucket:  "briefings-ia",
			wantParts: []string{"status <> 'published'", "archive_status, ''), 'active') = 'active'", "r2_storage_bucket", "<> $2"},
			wantArgs:  []any{"briefings-ia"},
		},
		{
			name:      "storage requires IA bucket match",
			tab:       "storage",
			iaBucket:  "briefings-ia",
			wantParts: []string{"r2_storage_bucket", "= $2"},
			wantArgs:  []any{"briefings-ia"},
		},
		{
			name:      "storage is empty when IA bucket is not configured",
			tab:       "storage",
			iaBucket:  "",
			wantEmpty: true,
		},
		{
			name:      "published does not add IA predicate when IA bucket is not configured",
			tab:       "published",
			iaBucket:  "",
			wantParts: []string{"status = 'published'", "archive_status, ''), 'active') = 'active'"},
			wantArgs:  []any{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, gotArgs, gotEmpty := audioBriefingListTabWhereClause(tt.tab, tt.iaBucket, 2)
			if gotEmpty != tt.wantEmpty {
				t.Fatalf("empty = %v, want %v", gotEmpty, tt.wantEmpty)
			}
			if !reflect.DeepEqual(gotArgs, tt.wantArgs) {
				t.Fatalf("args = %#v, want %#v", gotArgs, tt.wantArgs)
			}
			for _, part := range tt.wantParts {
				if !strings.Contains(got, part) {
					t.Fatalf("where clause %q does not contain %q", got, part)
				}
			}
		})
	}
}
