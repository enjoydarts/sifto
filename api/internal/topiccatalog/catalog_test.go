package topiccatalog

import (
	"encoding/json"
	"slices"
	"testing"
)

func TestMappingsJSON(t *testing.T) {
	topicJSON, genreJSON, err := MappingsJSON()
	if err != nil {
		t.Fatal(err)
	}
	var topics map[string]string
	if err := json.Unmarshal([]byte(topicJSON), &topics); err != nil {
		t.Fatal(err)
	}
	if got := topics["大規模言語モデル"]; got != "LLM" {
		t.Fatalf("legacy alias = %q, want LLM", got)
	}
	if got := topics["llm"]; got != "LLM" {
		t.Fatalf("canonical label = %q, want LLM", got)
	}
	var genres map[string]string
	if err := json.Unmarshal([]byte(genreJSON), &genres); err != nil {
		t.Fatal(err)
	}
	if got := genres["ai"]; got != "AI" {
		t.Fatalf("genre fallback = %q, want AI", got)
	}
}

func TestNormalizeBoundsAndDeduplicatesTopics(t *testing.T) {
	topics, err := Normalize([]string{
		"LLM", "大規模言語モデル", "開発ツール", "OpenAI", "AIエージェント", "クラウド",
	}, "ai")
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"LLM", "開発者ツール", "AIエージェント"}
	if len(topics) != len(want) {
		t.Fatalf("topics = %#v, want %#v", topics, want)
	}
	for i := range want {
		if topics[i] != want[i] {
			t.Fatalf("topics = %#v, want %#v", topics, want)
		}
	}
}

func TestNormalizeFallsBackToGenre(t *testing.T) {
	topics, err := Normalize([]string{"OpenAI", "local-dev"}, "ai")
	if err != nil {
		t.Fatal(err)
	}
	if len(topics) != 1 || topics[0] != "AI" {
		t.Fatalf("topics = %#v, want AI", topics)
	}
}

func TestNormalizeCommonLegacyTopicSpellings(t *testing.T) {
	topics, err := Normalize([]string{"AI エージェント", "データベース", "可観測性"}, "other")
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"AIエージェント", "データ基盤", "インフラ"}
	if !slices.Equal(topics, want) {
		t.Fatalf("topics = %#v, want %#v", topics, want)
	}
}

func TestFilterValuesExpandsCanonicalTopic(t *testing.T) {
	topics, genres, err := FilterValues("LLM")
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Contains(topics, "LLM") || !slices.Contains(topics, "大規模言語モデル") {
		t.Fatalf("topics = %#v, want canonical and legacy labels", topics)
	}
	if len(genres) != 0 {
		t.Fatalf("genres = %#v, want none", genres)
	}
	_, genres, err = FilterValues("AI")
	if err != nil || !slices.Contains(genres, "ai") {
		t.Fatalf("AI genres = %#v, err = %v", genres, err)
	}
}
