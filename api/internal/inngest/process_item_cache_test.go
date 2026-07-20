package inngest

import "testing"

func TestItemDetailCacheVersionKeyMatchesHandlerSchema(t *testing.T) {
	if got, want := itemDetailCacheVersionKey("item-1"), "cache_version:item_detail:v3:item-1"; got != want {
		t.Fatalf("item detail cache version key = %q, want %q", got, want)
	}
}
