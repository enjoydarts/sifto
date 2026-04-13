package service

import (
	"context"
	"encoding/json"
	"fmt"
	"slices"
	"strings"
)

const (
	DefaultUIFontSansKey  = "sawarabi-gothic"
	DefaultUIFontSerifKey = "sawarabi-mincho"
)

type UIFontCatalog struct {
	CatalogName     string              `json:"catalog_name"`
	Source          string              `json:"source"`
	SourceReference string              `json:"source_reference"`
	Fonts           []UIFontCatalogFont `json:"fonts"`
}

type UIFontCatalogFont struct {
	Key                string `json:"key"`
	Label              string `json:"label"`
	Family             string `json:"family"`
	Category           string `json:"category"`
	SelectableForSans  bool   `json:"selectable_for_sans"`
	SelectableForSerif bool   `json:"selectable_for_serif"`
	PreviewUI          string `json:"preview_ui"`
	PreviewBody        string `json:"preview_body"`
}

type UIFontCatalogService struct{}

func NewUIFontCatalogService() *UIFontCatalogService {
	return &UIFontCatalogService{}
}

func (s *UIFontCatalogService) LoadCatalog(ctx context.Context) (*UIFontCatalog, error) {
	if ctx != nil && ctx.Err() != nil {
		return nil, ctx.Err()
	}
	body, err := readSharedAsset("ui_font_catalog.json")
	if err != nil {
		return nil, err
	}
	var catalog UIFontCatalog
	if err := json.Unmarshal(body, &catalog); err != nil {
		return nil, err
	}
	catalog.CatalogName = strings.TrimSpace(catalog.CatalogName)
	catalog.Source = strings.TrimSpace(catalog.Source)
	catalog.SourceReference = strings.TrimSpace(catalog.SourceReference)
	fonts := make([]UIFontCatalogFont, 0, len(catalog.Fonts))
	seen := map[string]struct{}{}
	for _, font := range catalog.Fonts {
		key := strings.TrimSpace(strings.ToLower(font.Key))
		if key == "" {
			continue
		}
		if _, exists := seen[key]; exists {
			return nil, fmt.Errorf("duplicate ui font key: %s", key)
		}
		seen[key] = struct{}{}
		fonts = append(fonts, UIFontCatalogFont{
			Key:                key,
			Label:              strings.TrimSpace(font.Label),
			Family:             strings.TrimSpace(font.Family),
			Category:           normalizeUIFontCategory(font.Category),
			SelectableForSans:  font.SelectableForSans,
			SelectableForSerif: font.SelectableForSerif,
			PreviewUI:          strings.TrimSpace(font.PreviewUI),
			PreviewBody:        strings.TrimSpace(font.PreviewBody),
		})
	}
	slices.SortFunc(fonts, func(a, b UIFontCatalogFont) int {
		return strings.Compare(a.Label, b.Label)
	})
	catalog.Fonts = fonts
	return &catalog, nil
}

func normalizeUIFontCategory(raw string) string {
	switch strings.TrimSpace(strings.ToLower(raw)) {
	case "serif":
		return "serif"
	case "display":
		return "display"
	default:
		return "sans"
	}
}

func FindUIFontByKey(catalog *UIFontCatalog, key string) *UIFontCatalogFont {
	needle := strings.TrimSpace(strings.ToLower(key))
	if catalog == nil || needle == "" {
		return nil
	}
	for i := range catalog.Fonts {
		if catalog.Fonts[i].Key == needle {
			return &catalog.Fonts[i]
		}
	}
	return nil
}

func ValidateUIFontSelection(catalog *UIFontCatalog, sansKey, serifKey string) error {
	sans := FindUIFontByKey(catalog, sansKey)
	if sans == nil || !sans.SelectableForSans {
		return &ValidationError{Field: "ui_font_sans_key"}
	}
	serif := FindUIFontByKey(catalog, serifKey)
	if serif == nil || !serif.SelectableForSerif {
		return &ValidationError{Field: "ui_font_serif_key"}
	}
	return nil
}
