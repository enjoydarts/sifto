package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"path"
	"regexp"
	"strings"
	"time"

	"github.com/enjoydarts/sifto/api/internal/model"
	"github.com/enjoydarts/sifto/api/internal/repository"
	"github.com/enjoydarts/sifto/api/internal/timeutil"
)

const ObsidianExportTarget = "obsidian_github"

type ObsidianExportService struct {
	itemRepo     *repository.ItemRepo
	exportRepo   *repository.ItemExportRepo
	settingsRepo *repository.ObsidianExportRepo
	github       *GitHubAppClient
}

func NewObsidianExportService(itemRepo *repository.ItemRepo, exportRepo *repository.ItemExportRepo, settingsRepo *repository.ObsidianExportRepo, github *GitHubAppClient) *ObsidianExportService {
	return &ObsidianExportService{
		itemRepo:     itemRepo,
		exportRepo:   exportRepo,
		settingsRepo: settingsRepo,
		github:       github,
	}
}

type ObsidianExportRunResult struct {
	UserID  string `json:"user_id"`
	Updated int    `json:"updated"`
	Skipped int    `json:"skipped"`
	Failed  int    `json:"failed"`
}

func (s *ObsidianExportService) RunUser(ctx context.Context, cfg model.ObsidianExportSettings, limit int) (*ObsidianExportRunResult, error) {
	if s.github == nil || !s.github.Enabled() {
		return nil, fmt.Errorf("github app disabled")
	}
	if cfg.GitHubInstallationID == nil || cfg.GitHubRepoOwner == nil || cfg.GitHubRepoName == nil || cfg.VaultRootPath == nil {
		return nil, fmt.Errorf("obsidian export config incomplete")
	}
	items, err := s.itemRepo.FavoriteExportItems(ctx, cfg.UserID, 0, limit)
	if err != nil {
		return nil, err
	}
	itemIDs := make([]string, 0, len(items))
	for _, item := range items {
		itemIDs = append(itemIDs, item.ID)
	}
	existing, err := s.exportRepo.GetByUserTargetItemIDs(ctx, cfg.UserID, ObsidianExportTarget, itemIDs)
	if err != nil {
		return nil, err
	}
	token, err := s.github.CreateInstallationToken(ctx, *cfg.GitHubInstallationID)
	if err != nil {
		return nil, err
	}
	result := &ObsidianExportRunResult{UserID: cfg.UserID}
	for _, item := range items {
		content, githubPath, hash := BuildObsidianFavoriteMarkdown(item, cfg)
		if prev, ok := existing[item.ID]; ok && prev.ContentHash != nil && *prev.ContentHash == hash {
			result.Skipped++
			continue
		}
		current, err := s.github.GetFile(ctx, token, *cfg.GitHubRepoOwner, *cfg.GitHubRepoName, cfg.GitHubRepoBranch, githubPath)
		if err != nil {
			_ = s.exportRepo.UpsertFailure(ctx, cfg.UserID, item.ID, ObsidianExportTarget, err.Error())
			result.Failed++
			continue
		}
		var currentSHA *string
		if current != nil && strings.TrimSpace(current.SHA) != "" {
			currentSHA = &current.SHA
		}
		message := fmt.Sprintf("Add favorite export for %s", pickFavoriteTitle(item))
		sha, err := s.github.UpsertFile(ctx, token, *cfg.GitHubRepoOwner, *cfg.GitHubRepoName, cfg.GitHubRepoBranch, githubPath, message, content, currentSHA)
		if err != nil {
			_ = s.exportRepo.UpsertFailure(ctx, cfg.UserID, item.ID, ObsidianExportTarget, err.Error())
			result.Failed++
			continue
		}
		if err := s.exportRepo.UpsertSuccess(ctx, cfg.UserID, item.ID, ObsidianExportTarget, githubPath, sha, hash); err != nil {
			result.Failed++
			continue
		}
		result.Updated++
	}
	_ = s.settingsRepo.MarkRun(ctx, cfg.UserID, result.Failed == 0)
	return result, nil
}

func BuildObsidianFavoriteMarkdown(item model.FavoriteExportItem, cfg model.ObsidianExportSettings) ([]byte, string, string) {
	now := timeutil.NowJST()
	favoritedAt := item.FavoritedAt.In(timeutil.JST)
	root := strings.Trim(strings.TrimSpace(valueOrDefault(cfg.VaultRootPath, "Sifto/Favorites")), "/")
	dir := path.Join(root, favoritedAt.Format("2006"), favoritedAt.Format("01"))
	filename := fmt.Sprintf("%s_%s.md", favoritedAt.Format("2006-01-02"), slugify(pickFavoriteTitle(item), item.ID))
	githubPath := path.Join(dir, filename)
	vaultPath := dir

	var b strings.Builder
	b.WriteString("---\n")
	fmt.Fprintf(&b, "type: sifto-favorite\n")
	fmt.Fprintf(&b, "export_source: sifto\n")
	fmt.Fprintf(&b, "item_id: \"%s\"\n", item.ID)
	fmt.Fprintf(&b, "title: %q\n", pickFavoriteTitle(item))
	if item.SourceTitle != nil {
		fmt.Fprintf(&b, "source: %q\n", strings.TrimSpace(*item.SourceTitle))
	}
	fmt.Fprintf(&b, "source_url: %q\n", item.URL)
	if item.PublishedAt != nil {
		fmt.Fprintf(&b, "published_at: %s\n", item.PublishedAt.UTC().Format(time.RFC3339))
	}
	fmt.Fprintf(&b, "favorited_at: %s\n", item.FavoritedAt.UTC().Format(time.RFC3339))
	fmt.Fprintf(&b, "exported_at: %s\n", now.UTC().Format(time.RFC3339))
	fmt.Fprintf(&b, "is_favorite: true\n")
	fmt.Fprintf(&b, "summary_score: %s\n", formatScore(item.SummaryScore))
	b.WriteString("topics:\n")
	for _, topic := range item.Topics {
		if strings.TrimSpace(topic) == "" {
			continue
		}
		fmt.Fprintf(&b, "  - %q\n", wikilink(topic))
	}
	if item.SummaryLLM != nil {
		fmt.Fprintf(&b, "summary_provider: %s\n", item.SummaryLLM.Provider)
		fmt.Fprintf(&b, "summary_model: %s\n", item.SummaryLLM.Model)
	}
	if item.FactsLLM != nil {
		fmt.Fprintf(&b, "facts_provider: %s\n", item.FactsLLM.Provider)
		fmt.Fprintf(&b, "facts_model: %s\n", item.FactsLLM.Model)
	}
	if item.EmbeddingModel != nil {
		fmt.Fprintf(&b, "embedding_provider: openai\n")
		fmt.Fprintf(&b, "embedding_model: %s\n", strings.TrimSpace(*item.EmbeddingModel))
	}
	fmt.Fprintf(&b, "vault_root: %q\n", root)
	fmt.Fprintf(&b, "vault_path: %q\n", vaultPath)
	b.WriteString("---\n\n")
	fmt.Fprintf(&b, "# %s\n\n", pickFavoriteTitle(item))
	b.WriteString("## Summary\n")
	if item.Summary != nil && strings.TrimSpace(*item.Summary) != "" {
		b.WriteString(strings.TrimSpace(*item.Summary))
		b.WriteString("\n\n")
	} else {
		b.WriteString("Summary: (not available)\n\n")
	}
	b.WriteString("## Key Topics\n")
	for _, topic := range item.Topics {
		if strings.TrimSpace(topic) == "" {
			continue
		}
		fmt.Fprintf(&b, "- %s\n", wikilink(topic))
	}
	b.WriteString("\n## Link\n")
	b.WriteString(item.URL)
	b.WriteString("\n")

	content := []byte(b.String())
	sum := sha256.Sum256(content)
	return content, githubPath, hex.EncodeToString(sum[:])
}

func valueOrDefault(v *string, fallback string) string {
	if v == nil || strings.TrimSpace(*v) == "" {
		return fallback
	}
	return strings.TrimSpace(*v)
}

func pickFavoriteTitle(item model.FavoriteExportItem) string {
	if item.TranslatedTitle != nil && strings.TrimSpace(*item.TranslatedTitle) != "" {
		return strings.TrimSpace(*item.TranslatedTitle)
	}
	if item.Title != nil && strings.TrimSpace(*item.Title) != "" {
		return strings.TrimSpace(*item.Title)
	}
	return item.URL
}

func formatScore(score *float64) string {
	if score == nil {
		return "null"
	}
	return fmt.Sprintf("%.2f", *score)
}

func wikilink(v string) string {
	s := strings.TrimSpace(v)
	if s == "" {
		return s
	}
	return "[[" + s + "]]"
}

var slugUnsafe = regexp.MustCompile(`[^a-z0-9]+`)

func slugify(title, fallback string) string {
	s := strings.ToLower(strings.TrimSpace(title))
	s = strings.ReplaceAll(s, "'", "")
	s = slugUnsafe.ReplaceAllString(s, "-")
	s = strings.Trim(s, "-")
	if s == "" {
		return fallback
	}
	if len(s) > 80 {
		s = strings.Trim(s[:80], "-")
	}
	return s
}
