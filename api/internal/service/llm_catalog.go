package service

import (
	"encoding/json"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

type LLMCatalog struct {
	Providers       []LLMProviderCatalog `json:"providers"`
	ChatModels      []LLMModelCatalog    `json:"chat_models"`
	EmbeddingModels []LLMModelCatalog    `json:"embedding_models"`
}

type LLMProviderCatalog struct {
	ID            string            `json:"id"`
	APIKeyHeader  string            `json:"api_key_header"`
	MatchExact    []string          `json:"match_exact"`
	MatchPrefixes []string          `json:"match_prefixes"`
	DefaultModels map[string]string `json:"default_models"`
}

type LLMModelCatalog struct {
	ID                string                `json:"id"`
	Provider          string                `json:"provider"`
	AvailablePurposes []string              `json:"available_purposes"`
	Recommendation    string                `json:"recommendation"`
	BestFor           string                `json:"best_for"`
	Highlights        []string              `json:"highlights"`
	Comment           string                `json:"comment"`
	Capabilities      *LLMModelCapabilities `json:"capabilities,omitempty"`
	Pricing           *LLMModelPricing      `json:"pricing,omitempty"`
}

type LLMModelCapabilities struct {
	SupportsStructuredOutput  bool `json:"supports_structured_output"`
	SupportsStrictJSONSchema  bool `json:"supports_strict_json_schema"`
	SupportsReasoning         bool `json:"supports_reasoning"`
	SupportsToolCalling       bool `json:"supports_tool_calling"`
	SupportsCacheReadPricing  bool `json:"supports_cache_read_pricing"`
	SupportsCacheWritePricing bool `json:"supports_cache_write_pricing"`
}

type LLMModelPricing struct {
	PricingSource        string  `json:"pricing_source"`
	InputPerMTokUSD      float64 `json:"input_per_mtok_usd"`
	OutputPerMTokUSD     float64 `json:"output_per_mtok_usd"`
	CacheWritePerMTokUSD float64 `json:"cache_write_per_mtok_usd"`
	CacheReadPerMTokUSD  float64 `json:"cache_read_per_mtok_usd"`
}

var (
	llmCatalogMu       sync.RWMutex
	llmCatalogData     LLMCatalog
	llmCatalogLoadedAt time.Time
	llmCatalogPath     string
)

func LLMCatalogData() *LLMCatalog {
	path := catalogPath()
	info, err := os.Stat(path)
	if err != nil {
		log.Printf("load llm catalog failed: %v", err)
		llmCatalogMu.Lock()
		llmCatalogData = LLMCatalog{}
		llmCatalogLoadedAt = time.Time{}
		llmCatalogPath = path
		llmCatalogMu.Unlock()
		return &llmCatalogData
	}

	llmCatalogMu.RLock()
	loaded := llmCatalogLoadedAt
	loadedPath := llmCatalogPath
	if !loaded.IsZero() && loadedPath == path && !info.ModTime().After(loaded) {
		snapshot := llmCatalogData
		llmCatalogMu.RUnlock()
		return &snapshot
	}
	llmCatalogMu.RUnlock()

	raw, err := os.ReadFile(path)
	if err != nil {
		log.Printf("load llm catalog failed: %v", err)
		llmCatalogMu.RLock()
		snapshot := llmCatalogData
		llmCatalogMu.RUnlock()
		return &snapshot
	}
	var next LLMCatalog
	if err := json.Unmarshal(raw, &next); err != nil {
		log.Printf("load llm catalog failed: %v", err)
		llmCatalogMu.RLock()
		snapshot := llmCatalogData
		llmCatalogMu.RUnlock()
		return &snapshot
	}

	llmCatalogMu.Lock()
	llmCatalogData = next
	llmCatalogLoadedAt = info.ModTime()
	llmCatalogPath = path
	llmCatalogMu.Unlock()

	return &next
}

func catalogPath() string {
	if v := strings.TrimSpace(os.Getenv("LLM_CATALOG_PATH")); v != "" {
		return v
	}
	candidates := []string{
		"/app/shared/llm_catalog.json",
		"/shared/llm_catalog.json",
		filepath.Join("shared", "llm_catalog.json"),
		filepath.Join("..", "shared", "llm_catalog.json"),
		filepath.Join("..", "..", "shared", "llm_catalog.json"),
		filepath.Join("..", "..", "..", "shared", "llm_catalog.json"),
	}
	for _, path := range candidates {
		if _, err := os.Stat(path); err == nil {
			return path
		}
	}
	return candidates[0]
}

func findModelCatalog(model string) *LLMModelCatalog {
	m := strings.TrimSpace(model)
	if m == "" {
		return nil
	}
	catalog := LLMCatalogData()
	for i := range catalog.ChatModels {
		if catalog.ChatModels[i].ID == m {
			return &catalog.ChatModels[i]
		}
	}
	for i := range catalog.EmbeddingModels {
		if catalog.EmbeddingModels[i].ID == m {
			return &catalog.EmbeddingModels[i]
		}
	}
	return nil
}

func CatalogModelByID(model string) *LLMModelCatalog {
	return findModelCatalog(model)
}

func providerCatalogByID(provider string) *LLMProviderCatalog {
	p := strings.TrimSpace(provider)
	if p == "" {
		return nil
	}
	for i := range LLMCatalogData().Providers {
		if LLMCatalogData().Providers[i].ID == p {
			return &LLMCatalogData().Providers[i]
		}
	}
	return nil
}

func CatalogProviderForModel(model string) string {
	m := strings.TrimSpace(model)
	if m == "" {
		return ""
	}
	if entry := findModelCatalog(m); entry != nil && entry.Provider != "" {
		return entry.Provider
	}
	lower := strings.ToLower(m)
	for _, provider := range LLMCatalogData().Providers {
		for _, exact := range provider.MatchExact {
			if lower == strings.ToLower(strings.TrimSpace(exact)) {
				return provider.ID
			}
		}
		for _, prefix := range provider.MatchPrefixes {
			p := strings.ToLower(strings.TrimSpace(prefix))
			if p != "" && strings.HasPrefix(lower, p) {
				return provider.ID
			}
		}
	}
	return ""
}

func CatalogDefaultModelForPurpose(provider, purpose string) string {
	p := providerCatalogByID(provider)
	if p == nil || p.DefaultModels == nil {
		return ""
	}
	return strings.TrimSpace(p.DefaultModels[strings.TrimSpace(purpose)])
}

func CatalogSupportedEmbeddingModels(provider string) []string {
	out := make([]string, 0, len(LLMCatalogData().EmbeddingModels))
	for _, m := range LLMCatalogData().EmbeddingModels {
		if provider != "" && m.Provider != provider {
			continue
		}
		out = append(out, m.ID)
	}
	return out
}

func CatalogModelCapabilities(model string) *LLMModelCapabilities {
	entry := findModelCatalog(model)
	if entry == nil {
		return nil
	}
	return entry.Capabilities
}

func CatalogModelSupportsCapability(model, capability string) bool {
	caps := CatalogModelCapabilities(model)
	if caps == nil {
		return false
	}
	switch strings.TrimSpace(capability) {
	case "structured_output":
		return caps.SupportsStructuredOutput
	case "strict_json_schema":
		return caps.SupportsStrictJSONSchema
	case "reasoning":
		return caps.SupportsReasoning
	case "tool_calling":
		return caps.SupportsToolCalling
	case "cache_read_pricing":
		return caps.SupportsCacheReadPricing
	case "cache_write_pricing":
		return caps.SupportsCacheWritePricing
	default:
		return false
	}
}

func CatalogModelSupportsPurpose(model, purpose string) bool {
	entry := findModelCatalog(model)
	if entry == nil {
		return false
	}
	want := strings.TrimSpace(purpose)
	if want == "" {
		return false
	}
	for _, available := range entry.AvailablePurposes {
		if strings.TrimSpace(available) == want {
			return true
		}
	}
	return false
}

func CatalogIsEmbeddingModel(model string) bool {
	entry := findModelCatalog(model)
	if entry == nil {
		return false
	}
	for _, candidate := range LLMCatalogData().EmbeddingModels {
		if candidate.ID == entry.ID {
			return true
		}
	}
	return false
}
