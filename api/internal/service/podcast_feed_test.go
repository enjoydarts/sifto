package service

import (
	"encoding/xml"
	"strings"
	"testing"
)

func TestPodcastRSSMarshalsItunesOwnerEmail(t *testing.T) {
	body, err := xml.MarshalIndent(podcastRSS{
		Version:     "2.0",
		XMLNSItunes: "http://www.itunes.com/dtds/podcast-1.0.dtd",
		Channel: podcastRSSChannel{
			Title:        "Test Show",
			Link:         "https://api.example.com/podcasts/test/feed.xml",
			Description:  "desc",
			Language:     "ja",
			ItunesAuthor: "Sifto",
			ItunesOwner: &podcastRSSOwner{
				ItunesName:  "Sifto",
				ItunesEmail: "owner@example.com",
			},
			ItunesSummary:  "desc",
			ItunesExplicit: "no",
		},
	}, "", "  ")
	if err != nil {
		t.Fatalf("xml.MarshalIndent(...) error = %v", err)
	}
	got := string(body)
	if !strings.Contains(got, "<itunes:owner>") {
		t.Fatalf("rss missing itunes:owner: %s", got)
	}
	if !strings.Contains(got, "<itunes:email>owner@example.com</itunes:email>") {
		t.Fatalf("rss missing itunes:email: %s", got)
	}
}
