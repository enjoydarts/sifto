package service

import "testing"

func TestBuildSummaryAudioNarrationPrefersTranslatedTitle(t *testing.T) {
	got := BuildSummaryAudioNarration("邦題タイトル", "Original Title", "要約本文")
	want := "邦題タイトル\n\n要約本文"
	if got != want {
		t.Fatalf("BuildSummaryAudioNarration(...) = %q, want %q", got, want)
	}
}

func TestBuildSummaryAudioNarrationFallsBackToOriginalTitle(t *testing.T) {
	got := BuildSummaryAudioNarration("", "Original Title", "要約本文")
	want := "Original Title\n\n要約本文"
	if got != want {
		t.Fatalf("BuildSummaryAudioNarration(...) = %q, want %q", got, want)
	}
}
