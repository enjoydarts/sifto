package inngest

import (
	"fmt"
	"math"
	"sort"
	"strings"

	"github.com/enjoydarts/sifto/api/internal/model"
	"github.com/enjoydarts/sifto/api/internal/service"
)

func digestTextLooksComplete(text string, minLen int) bool {
	s := strings.TrimSpace(text)
	if len([]rune(s)) < minLen {
		return false
	}
	if strings.Count(s, "```")%2 != 0 {
		return false
	}
	last := []rune(s)[len([]rune(s))-1]
	switch last {
	case '。', '！', '？', '.', '!', '?', '」', '』':
		return true
	default:
		return false
	}
}

func digestClusterDraftValidationReason(text string) string {
	s := strings.TrimSpace(text)
	if len([]rune(s)) < 40 {
		return "too_short"
	}
	if strings.Count(s, "```")%2 != 0 {
		return "unclosed_code_fence"
	}
	lines := strings.Split(s, "\n")
	bullets := make([]string, 0, len(lines))
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		bullets = append(bullets, line)
	}
	if len(bullets) < 2 {
		if len(bullets) == 1 {
			last := bullets[0]
			if strings.HasPrefix(last, "-") || strings.HasPrefix(last, "・") || strings.HasPrefix(last, "•") {
				trimmed := strings.TrimSpace(strings.TrimLeft(last, "-・• "))
				if len([]rune(trimmed)) >= 20 && digestTextLooksComplete(trimmed, 20) {
					return ""
				}
			} else if digestTextLooksComplete(last, 20) {
				return ""
			}
		}
		return "too_few_lines"
	}
	last := bullets[len(bullets)-1]
	if strings.HasPrefix(last, "-") || strings.HasPrefix(last, "・") || strings.HasPrefix(last, "•") {
		trimmed := strings.TrimSpace(strings.TrimLeft(last, "-・• "))
		if len([]rune(trimmed)) < 8 {
			return "last_bullet_too_short"
		}
		if strings.HasSuffix(trimmed, "、") ||
			strings.HasSuffix(trimmed, ",") ||
			strings.HasSuffix(trimmed, "：") ||
			strings.HasSuffix(trimmed, ":") ||
			strings.HasSuffix(trimmed, "は") ||
			strings.HasSuffix(trimmed, "が") ||
			strings.HasSuffix(trimmed, "を") ||
			strings.HasSuffix(trimmed, "に") ||
			strings.HasSuffix(trimmed, "で") ||
			strings.HasSuffix(trimmed, "と") ||
			strings.HasSuffix(trimmed, "の") ||
			strings.HasSuffix(trimmed, "も") ||
			strings.HasSuffix(trimmed, "より") ||
			strings.HasSuffix(trimmed, "から") {
			return "last_bullet_ends_with_particle"
		}
		return ""
	}
	if !digestTextLooksComplete(s, 80) {
		return "text_looks_incomplete"
	}
	return ""
}

func validateDigestClusterDraftCompletion(text string) error {
	if digestClusterDraftValidationReason(text) != "" {
		return fmt.Errorf("cluster draft looks truncated")
	}
	return nil
}

func validateDigestCompletion(subject, body string) error {
	if strings.TrimSpace(subject) == "" {
		return fmt.Errorf("digest subject is empty")
	}
	if !digestTextLooksComplete(body, 220) {
		return fmt.Errorf("digest body looks truncated")
	}
	return nil
}

func digestTopicKey(topics []string) string {
	for _, t := range topics {
		t = strings.TrimSpace(t)
		if t != "" {
			return t
		}
	}
	return "__untagged__"
}

func buildDigestClusterDrafts(details []model.DigestItemDetail, embClusters []model.ReadingPlanCluster) []model.DigestClusterDraft {
	if len(details) == 0 {
		return nil
	}
	byID := make(map[string]model.DigestItemDetail, len(details))
	for _, d := range details {
		byID[d.Item.ID] = d
	}
	seen := map[string]struct{}{}
	out := make([]model.DigestClusterDraft, 0, len(details))

	appendDraft := func(idx int, key, label string, group []model.DigestItemDetail) {
		if len(group) == 0 {
			return
		}
		maxScore := 0.0
		hasScore := false
		lines := make([]string, 0, minInt(4, len(group)))
		for i, it := range group {
			if it.Summary.Score != nil {
				if !hasScore || *it.Summary.Score > maxScore {
					maxScore = *it.Summary.Score
					hasScore = true
				}
			}
			if i >= 4 {
				continue
			}
			title := strings.TrimSpace(coalescePtrStr(it.Item.Title, it.Item.URL))
			summary := strings.TrimSpace(it.Summary.Summary)
			factLine := ""
			if len(it.Facts) > 0 {
				facts := make([]string, 0, minInt(2, len(it.Facts)))
				for _, f := range it.Facts {
					f = strings.TrimSpace(f)
					if f == "" {
						continue
					}
					facts = append(facts, f)
					if len(facts) >= 2 {
						break
					}
				}
				if len(facts) > 0 {
					factLine = strings.Join(facts, " / ")
				}
			}
			switch {
			case summary != "" && factLine != "":
				lines = append(lines, "- "+title+": "+summary+" | facts: "+factLine)
			case summary != "":
				lines = append(lines, "- "+title+": "+summary)
			case factLine != "":
				lines = append(lines, "- "+title+": "+factLine)
			default:
				lines = append(lines, "- "+title)
			}
		}
		draftSummary := strings.Join(lines, "\n")
		if len(group) > 4 {
			draftSummary += fmt.Sprintf("\n- ...and %d more related items", len(group)-4)
		}
		var scorePtr *float64
		if hasScore {
			v := maxScore
			scorePtr = &v
		}
		out = append(out, model.DigestClusterDraft{
			ClusterKey:   key,
			ClusterLabel: label,
			Rank:         idx,
			ItemCount:    len(group),
			Topics:       group[0].Summary.Topics,
			MaxScore:     scorePtr,
			DraftSummary: draftSummary,
		})
	}

	rank := 1
	for _, c := range embClusters {
		group := make([]model.DigestItemDetail, 0, len(c.Items))
		for _, m := range c.Items {
			d, ok := byID[m.ID]
			if !ok {
				continue
			}
			if _, dup := seen[d.Item.ID]; dup {
				continue
			}
			seen[d.Item.ID] = struct{}{}
			group = append(group, d)
		}
		if len(group) == 0 {
			continue
		}
		label := c.Label
		if strings.TrimSpace(label) == "" {
			label = digestTopicKey(group[0].Summary.Topics)
		}
		appendDraft(rank, c.ID, label, group)
		rank++
	}

	for _, d := range details {
		if _, ok := seen[d.Item.ID]; ok {
			continue
		}
		seen[d.Item.ID] = struct{}{}
		key := d.Item.ID
		label := digestTopicKey(d.Summary.Topics)
		appendDraft(rank, key, label, []model.DigestItemDetail{d})
		rank++
	}
	return out
}

func draftSourceLines(draftSummary string) []string {
	lines := strings.Split(draftSummary, "\n")
	out := make([]string, 0, len(lines))
	for _, l := range lines {
		l = strings.TrimSpace(l)
		if l == "" {
			continue
		}
		out = append(out, l)
	}
	return out
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func buildBroadDigestDraftFromChunk(chunk []model.DigestClusterDraft, key, label string) model.DigestClusterDraft {
	itemCount := 0
	var maxScore *float64
	lines := make([]string, 0, len(chunk))
	topicsSet := map[string]struct{}{}
	for _, d := range chunk {
		itemCount += d.ItemCount
		if d.MaxScore != nil && (maxScore == nil || *d.MaxScore > *maxScore) {
			v := *d.MaxScore
			maxScore = &v
		}
		for _, t := range d.Topics {
			t = strings.TrimSpace(t)
			if t == "" {
				continue
			}
			topicsSet[t] = struct{}{}
		}
		line := strings.TrimSpace(d.DraftSummary)
		if line == "" {
			continue
		}
		first := strings.Split(line, "\n")[0]
		lines = append(lines, fmt.Sprintf("- [%s] %s", d.ClusterLabel, first))
	}
	topics := make([]string, 0, len(topicsSet))
	for t := range topicsSet {
		topics = append(topics, t)
	}
	sort.Strings(topics)
	return model.DigestClusterDraft{
		ClusterKey:   key,
		ClusterLabel: label,
		ItemCount:    itemCount,
		Topics:       topics,
		MaxScore:     maxScore,
		DraftSummary: strings.Join(lines, "\n"),
	}
}

func compressDigestClusterDrafts(drafts []model.DigestClusterDraft, target int) []model.DigestClusterDraft {
	if target <= 0 {
		target = 20
	}
	if len(drafts) <= target {
		return drafts
	}

	keep := make([]model.DigestClusterDraft, 0, len(drafts))
	tail := make([]model.DigestClusterDraft, 0, len(drafts))
	for i, d := range drafts {
		if i < 10 || d.ItemCount >= 3 {
			keep = append(keep, d)
			continue
		}
		tail = append(tail, d)
	}
	broadCount := 0
	if len(tail) >= 4 {
		broadCount = 1
	}
	if len(tail) >= 10 {
		broadCount = 2
	}
	if len(keep) >= target {
		cut := target - broadCount
		if cut < 1 {
			cut = target
			broadCount = 0
		}
		keep = keep[:cut]
		if broadCount > 0 {
			if broadCount == 1 {
				keep = append(keep, buildBroadDigestDraftFromChunk(tail, "broad-1", "幅広い話題（横断）"))
			} else {
				mid := len(tail) / 2
				if mid < 1 {
					mid = 1
				}
				keep = append(keep, buildBroadDigestDraftFromChunk(tail[:mid], "broad-1", "幅広い話題（横断）A"))
				keep = append(keep, buildBroadDigestDraftFromChunk(tail[mid:], "broad-2", "幅広い話題（横断）B"))
			}
		}
		for i := range keep {
			keep[i].Rank = i + 1
		}
		return keep
	}

	remainingSlots := target - len(keep)
	if remainingSlots <= 0 || len(tail) == 0 {
		for i := range keep {
			keep[i].Rank = i + 1
		}
		return keep
	}

	chunkSize := int(math.Ceil(float64(len(tail)) / float64(remainingSlots)))
	if chunkSize < 2 {
		chunkSize = 2
	}
	for i := 0; i < len(tail) && len(keep) < target; i += chunkSize {
		end := i + chunkSize
		if end > len(tail) {
			end = len(tail)
		}
		chunk := tail[i:end]
		if len(chunk) == 1 {
			keep = append(keep, chunk[0])
			continue
		}
		keep = append(keep, buildBroadDigestDraftFromChunk(chunk, fmt.Sprintf("merged-tail-%d", len(keep)+1), "その他の話題"))
	}

	for i := range keep {
		keep[i].Rank = i + 1
	}
	return keep
}

func buildComposeItemsFromClusterDrafts(drafts []model.DigestClusterDraft, maxItems int) []service.ComposeDigestItem {
	_ = maxItems
	out := make([]service.ComposeDigestItem, 0, len(drafts))
	for i, d := range drafts {
		title := d.ClusterLabel
		if d.ItemCount > 1 {
			title = fmt.Sprintf("%s (%d items)", d.ClusterLabel, d.ItemCount)
		}
		summary := d.DraftSummary
		if i >= 12 {
			lines := strings.Split(strings.TrimSpace(d.DraftSummary), "\n")
			if len(lines) > 0 && strings.TrimSpace(lines[0]) != "" {
				summary = lines[0]
			}
			if len(lines) > 1 {
				summary += fmt.Sprintf("\n- ...%d more lines omitted in compose input", len(lines)-1)
			}
		}
		titlePtr := title
		out = append(out, service.ComposeDigestItem{
			Rank:    i + 1,
			Title:   &titlePtr,
			URL:     "",
			Summary: summary,
			Topics:  d.Topics,
			Score:   d.MaxScore,
		})
	}
	return out
}

func coalescePtrStr(a *string, b string) string {
	if a != nil && strings.TrimSpace(*a) != "" {
		return *a
	}
	return b
}
