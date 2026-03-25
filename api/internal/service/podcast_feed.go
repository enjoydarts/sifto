package service

import (
	"context"
	"encoding/xml"
	"fmt"
	"strings"
	"time"

	"github.com/enjoydarts/sifto/api/internal/model"
	"github.com/enjoydarts/sifto/api/internal/repository"
	"github.com/enjoydarts/sifto/api/internal/timeutil"
)

type PodcastFeedService struct {
	settingsRepo      *repository.UserSettingsRepo
	audioBriefingRepo *repository.AudioBriefingRepo
	worker            *WorkerClient
}

func NewPodcastFeedService(settingsRepo *repository.UserSettingsRepo, audioBriefingRepo *repository.AudioBriefingRepo, worker *WorkerClient) *PodcastFeedService {
	return &PodcastFeedService{
		settingsRepo:      settingsRepo,
		audioBriefingRepo: audioBriefingRepo,
		worker:            worker,
	}
}

type podcastRSS struct {
	XMLName     xml.Name          `xml:"rss"`
	Version     string            `xml:"version,attr"`
	XMLNSItunes string            `xml:"xmlns:itunes,attr,omitempty"`
	Channel     podcastRSSChannel `xml:"channel"`
}

type podcastRSSChannel struct {
	Title          string           `xml:"title"`
	Link           string           `xml:"link"`
	Description    string           `xml:"description"`
	Language       string           `xml:"language"`
	ItunesAuthor   string           `xml:"itunes:author"`
	ItunesSummary  string           `xml:"itunes:summary"`
	ItunesExplicit string           `xml:"itunes:explicit"`
	ItunesImage    *podcastRSSImage `xml:"itunes:image,omitempty"`
	Items          []podcastRSSItem `xml:"item"`
}

type podcastRSSImage struct {
	Href string `xml:"href,attr"`
}

type podcastRSSEnclosure struct {
	URL    string `xml:"url,attr"`
	Length int64  `xml:"length,attr"`
	Type   string `xml:"type,attr"`
}

type podcastRSSGUID struct {
	IsPermaLink string `xml:"isPermaLink,attr,omitempty"`
	Value       string `xml:",chardata"`
}

type podcastRSSItem struct {
	Title         string              `xml:"title"`
	Description   string              `xml:"description"`
	ItunesSummary string              `xml:"itunes:summary"`
	GUID          podcastRSSGUID      `xml:"guid"`
	PubDate       string              `xml:"pubDate"`
	Enclosure     podcastRSSEnclosure `xml:"enclosure"`
}

func (s *PodcastFeedService) BuildXML(ctx context.Context, slug string) ([]byte, error) {
	settings, err := s.settingsRepo.GetByPodcastFeedSlug(ctx, slug)
	if err != nil {
		return nil, err
	}
	if settings == nil || !settings.PodcastEnabled {
		return nil, repository.ErrNotFound
	}

	now := timeutil.NowJST()
	cutoff := now.AddDate(0, 0, -AudioBriefingIAMoveAfterDaysFromEnv())
	jobs, err := s.audioBriefingRepo.ListPodcastPublishedJobsByUser(ctx, settings.UserID, cutoff, 30)
	if err != nil {
		return nil, err
	}
	items := make([]podcastRSSItem, 0, len(jobs))
	for _, job := range jobs {
		if !AudioBriefingJobIsPodcastEligible(&job, now) {
			continue
		}
		item, err := s.buildItem(ctx, settings.UserID, job)
		if err != nil {
			return nil, err
		}
		if item != nil {
			items = append(items, *item)
		}
	}

	channel := podcastRSSChannel{
		Title:          firstNonEmptyTrimmed(stringValue(settings.PodcastTitle), "Sifto Audio Briefing"),
		Link:           firstNonEmptyTrimmed(stringValue(podcastRSSURL(settings.PodcastFeedSlug)), ""),
		Description:    firstNonEmptyTrimmed(stringValue(settings.PodcastDescription), "Siftoで生成した公開音声ブリーフィングです。"),
		Language:       firstNonEmptyTrimmed(settings.PodcastLanguage, "ja"),
		ItunesAuthor:   firstNonEmptyTrimmed(stringValue(settings.PodcastAuthor), "Sifto"),
		ItunesSummary:  firstNonEmptyTrimmed(stringValue(settings.PodcastDescription), "Siftoで生成した公開音声ブリーフィングです。"),
		ItunesExplicit: podcastExplicitLabel(settings.PodcastExplicit),
		Items:          items,
	}
	if artworkURL := firstNonEmptyTrimmed(stringValue(settings.PodcastArtworkURL), stringValue(podcastDefaultArtworkURL())); artworkURL != "" {
		channel.ItunesImage = &podcastRSSImage{Href: artworkURL}
	}
	body, err := xml.MarshalIndent(podcastRSS{
		Version:     "2.0",
		XMLNSItunes: "http://www.itunes.com/dtds/podcast-1.0.dtd",
		Channel:     channel,
	}, "", "  ")
	if err != nil {
		return nil, err
	}
	return append([]byte(xml.Header), body...), nil
}

func (s *PodcastFeedService) buildItem(ctx context.Context, userID string, job model.AudioBriefingJob) (*podcastRSSItem, error) {
	audioURL := AudioBriefingPodcastPublicObjectURL(job.PodcastPublicBucket, stringValue(job.PodcastPublicObjectKey))
	if audioURL == nil {
		return nil, nil
	}
	sizeBytes := int64(0)
	if s.worker != nil {
		stat, err := s.worker.StatAudioBriefingObject(ctx, strings.TrimSpace(job.PodcastPublicBucket), stringValue(job.PodcastPublicObjectKey))
		if err != nil {
			return nil, err
		}
		sizeBytes = stat.SizeBytes
	}
	description, err := s.buildDescription(ctx, userID, job.ID)
	if err != nil {
		return nil, err
	}
	title := strings.TrimSpace(stringValue(job.Title))
	if title == "" {
		title = "Sifto Audio Briefing"
		if job.PublishedAt != nil {
			title = fmt.Sprintf("Sifto Audio Briefing %s", job.PublishedAt.In(timeutil.JST).Format("2006-01-02"))
		}
	}
	return &podcastRSSItem{
		Title:         title,
		Description:   description,
		ItunesSummary: description,
		GUID: podcastRSSGUID{
			IsPermaLink: "false",
			Value:       job.ID,
		},
		PubDate:   job.PublishedAt.In(timeutil.JST).Format(time.RFC1123Z),
		Enclosure: podcastRSSEnclosure{URL: *audioURL, Length: sizeBytes, Type: "audio/mpeg"},
	}, nil
}

func (s *PodcastFeedService) buildDescription(ctx context.Context, userID, jobID string) (string, error) {
	chunks, err := s.audioBriefingRepo.ListJobChunks(ctx, userID, jobID)
	if err != nil {
		return "", err
	}
	parts := make([]string, 0, 2)
	for _, partType := range []string{"opening", "overall_summary"} {
		text := strings.TrimSpace(joinChunkText(chunks, partType))
		if text != "" {
			parts = append(parts, text)
		}
	}
	return strings.Join(parts, "\n\n"), nil
}

func joinChunkText(chunks []model.AudioBriefingScriptChunk, partType string) string {
	parts := make([]string, 0)
	for _, chunk := range chunks {
		if chunk.PartType != partType {
			continue
		}
		text := strings.TrimSpace(chunk.Text)
		if text == "" {
			continue
		}
		parts = append(parts, text)
	}
	return strings.Join(parts, "")
}

func podcastExplicitLabel(explicit bool) string {
	if explicit {
		return "yes"
	}
	return "no"
}

func stringValue(v *string) string {
	if v == nil {
		return ""
	}
	return strings.TrimSpace(*v)
}
