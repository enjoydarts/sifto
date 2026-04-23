package service

import (
	"encoding/json"
	"log"
	"os"
	"path/filepath"
	"regexp"
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
	Label         string            `json:"label,omitempty"`
	APIKeyHeader  string            `json:"api_key_header"`
	MatchExact    []string          `json:"match_exact"`
	MatchPrefixes []string          `json:"match_prefixes"`
	DefaultModels map[string]string `json:"default_models"`
}

type LLMModelCatalog struct {
	ID                string                `json:"id"`
	Provider          string                `json:"provider"`
	SourceProvider    string                `json:"source_provider,omitempty"`
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
	dynamicCatalogMu   sync.RWMutex
	dynamicChatModels  = map[string][]LLMModelCatalog{}
)

const openRouterAliasPrefix = "openrouter::"
const poeAliasPrefix = "poe::"
const siliconFlowAliasPrefix = "siliconflow::"
const togetherAliasPrefix = "together::"
const featherlessAliasPrefix = "featherless::"
const deepInfraAliasPrefix = "deepinfra::"
const miniMaxAliasPrefix = "minimax::"
const miniMaxSlashPrefix = "minimax/"

var anthropicOpenRouterResolvedPattern = regexp.MustCompile(`^anthropic/claude-(\d+(?:\.\d+)*)-(opus|sonnet|haiku)-\d{8}$`)

func OpenRouterAliasModelID(model string) string {
	m := strings.TrimSpace(model)
	if m == "" {
		return ""
	}
	if strings.HasPrefix(m, openRouterAliasPrefix) {
		return m
	}
	return openRouterAliasPrefix + m
}

func ResolveOpenRouterModelID(model string) string {
	m := strings.TrimSpace(model)
	return strings.TrimPrefix(m, openRouterAliasPrefix)
}

func CanonicalizeOpenRouterModelID(model string) string {
	resolved := ResolveOpenRouterModelID(model)
	if resolved == "" {
		return ""
	}
	if match := anthropicOpenRouterResolvedPattern.FindStringSubmatch(resolved); len(match) == 3 {
		return "anthropic/claude-" + match[2] + "-" + match[1]
	}
	return resolved
}

func IsOpenRouterAliasedModel(model string) bool {
	return strings.HasPrefix(strings.TrimSpace(model), openRouterAliasPrefix)
}

func PoeAliasModelID(model string) string {
	m := strings.TrimSpace(model)
	if m == "" {
		return ""
	}
	if strings.HasPrefix(m, poeAliasPrefix) {
		return m
	}
	return poeAliasPrefix + m
}

func ResolvePoeModelID(model string) string {
	m := strings.TrimSpace(model)
	return strings.TrimPrefix(m, poeAliasPrefix)
}

func IsPoeAliasedModel(model string) bool {
	return strings.HasPrefix(strings.TrimSpace(model), poeAliasPrefix)
}

func SiliconFlowAliasModelID(model string) string {
	m := strings.TrimSpace(model)
	if m == "" {
		return ""
	}
	if strings.HasPrefix(m, siliconFlowAliasPrefix) {
		return m
	}
	return siliconFlowAliasPrefix + m
}

func ResolveSiliconFlowModelID(model string) string {
	m := strings.TrimSpace(model)
	return strings.TrimPrefix(m, siliconFlowAliasPrefix)
}

func IsSiliconFlowAliasedModel(model string) bool {
	return strings.HasPrefix(strings.TrimSpace(model), siliconFlowAliasPrefix)
}

func TogetherAliasModelID(model string) string {
	m := strings.TrimSpace(model)
	if m == "" {
		return ""
	}
	if strings.HasPrefix(m, togetherAliasPrefix) {
		return m
	}
	return togetherAliasPrefix + m
}

func ResolveTogetherModelID(model string) string {
	m := strings.TrimSpace(model)
	return strings.TrimPrefix(m, togetherAliasPrefix)
}

func IsTogetherAliasedModel(model string) bool {
	return strings.HasPrefix(strings.TrimSpace(model), togetherAliasPrefix)
}

func FeatherlessAliasModelID(model string) string {
	m := strings.TrimSpace(model)
	if m == "" {
		return ""
	}
	if strings.HasPrefix(m, featherlessAliasPrefix) {
		return m
	}
	return featherlessAliasPrefix + m
}

func ResolveFeatherlessModelID(model string) string {
	m := strings.TrimSpace(model)
	return strings.TrimPrefix(m, featherlessAliasPrefix)
}

func IsFeatherlessAliasedModel(model string) bool {
	return strings.HasPrefix(strings.TrimSpace(model), featherlessAliasPrefix)
}

func DeepInfraAliasModelID(model string) string {
	m := strings.TrimSpace(model)
	if m == "" {
		return ""
	}
	if strings.HasPrefix(m, deepInfraAliasPrefix) {
		return m
	}
	return deepInfraAliasPrefix + m
}

func ResolveDeepInfraModelID(model string) string {
	m := strings.TrimSpace(model)
	return strings.TrimPrefix(m, deepInfraAliasPrefix)
}

func IsDeepInfraAliasedModel(model string) bool {
	return strings.HasPrefix(strings.TrimSpace(model), deepInfraAliasPrefix)
}

func MiniMaxAliasModelID(model string) string {
	m := strings.TrimSpace(model)
	if m == "" {
		return ""
	}
	if strings.HasPrefix(m, miniMaxAliasPrefix) || strings.HasPrefix(m, miniMaxSlashPrefix) {
		return m
	}
	return miniMaxAliasPrefix + m
}

func ResolveMiniMaxModelID(model string) string {
	m := strings.TrimSpace(model)
	if strings.HasPrefix(m, miniMaxAliasPrefix) {
		return strings.TrimPrefix(m, miniMaxAliasPrefix)
	}
	if strings.HasPrefix(m, miniMaxSlashPrefix) {
		return strings.TrimPrefix(m, miniMaxSlashPrefix)
	}
	return m
}

func IsMiniMaxAliasedModel(model string) bool {
	m := strings.TrimSpace(model)
	return strings.HasPrefix(m, miniMaxAliasPrefix) || strings.HasPrefix(m, miniMaxSlashPrefix)
}

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
		snapshot := llmCatalogData
		return mergedCatalog(&snapshot)
	}

	llmCatalogMu.RLock()
	loaded := llmCatalogLoadedAt
	loadedPath := llmCatalogPath
	if !loaded.IsZero() && loadedPath == path && !info.ModTime().After(loaded) {
		snapshot := llmCatalogData
		llmCatalogMu.RUnlock()
		return mergedCatalog(&snapshot)
	}
	llmCatalogMu.RUnlock()

	raw, err := os.ReadFile(path)
	if err != nil {
		log.Printf("load llm catalog failed: %v", err)
		llmCatalogMu.RLock()
		snapshot := llmCatalogData
		llmCatalogMu.RUnlock()
		return mergedCatalog(&snapshot)
	}
	var next LLMCatalog
	if err := json.Unmarshal(raw, &next); err != nil {
		log.Printf("load llm catalog failed: %v", err)
		llmCatalogMu.RLock()
		snapshot := llmCatalogData
		llmCatalogMu.RUnlock()
		return mergedCatalog(&snapshot)
	}

	llmCatalogMu.Lock()
	llmCatalogData = next
	llmCatalogLoadedAt = info.ModTime()
	llmCatalogPath = path
	llmCatalogMu.Unlock()

	return mergedCatalog(&next)
}

func mergedCatalog(base *LLMCatalog) *LLMCatalog {
	if base == nil {
		return &LLMCatalog{}
	}
	dynamicCatalogMu.RLock()
	defer dynamicCatalogMu.RUnlock()
	merged := *base
	totalDynamic := 0
	for _, models := range dynamicChatModels {
		totalDynamic += len(models)
	}
	seen := make(map[string]struct{}, len(base.ChatModels)+totalDynamic)
	merged.ChatModels = make([]LLMModelCatalog, 0, len(base.ChatModels)+totalDynamic)
	for _, model := range base.ChatModels {
		merged.ChatModels = append(merged.ChatModels, model)
		seen[model.ID] = struct{}{}
	}
	if len(dynamicChatModels) == 0 {
		return &merged
	}
	for _, models := range dynamicChatModels {
		for _, model := range models {
			if _, exists := seen[model.ID]; exists {
				continue
			}
			merged.ChatModels = append(merged.ChatModels, model)
			seen[model.ID] = struct{}{}
		}
	}
	return &merged
}

func SetDynamicChatModels(models []LLMModelCatalog) {
	SetDynamicChatModelsForProvider("default", models)
}

func SetDynamicChatModelsForProvider(provider string, models []LLMModelCatalog) {
	dynamicCatalogMu.Lock()
	defer dynamicCatalogMu.Unlock()
	key := strings.TrimSpace(provider)
	if key == "" {
		key = "default"
	}
	if len(models) == 0 {
		delete(dynamicChatModels, key)
		return
	}
	dynamicChatModels[key] = append([]LLMModelCatalog{}, models...)
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

func findModelCatalogInCatalog(catalog *LLMCatalog, model string) *LLMModelCatalog {
	m := strings.TrimSpace(model)
	if m == "" || catalog == nil {
		return nil
	}
	if canonical := resolveCatalogAliasModelID(catalog, m); canonical != "" && canonical != m {
		m = canonical
	}
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

func resolveCatalogAliasModelID(catalog *LLMCatalog, model string) string {
	m := strings.TrimSpace(model)
	if m == "" || catalog == nil {
		return ""
	}
	if catalogHasModelID(catalog, m) {
		return m
	}
	candidates := []string{m}
	if IsTogetherAliasedModel(m) {
		candidates = append(candidates, ResolveTogetherModelID(m))
	}
	if IsFeatherlessAliasedModel(m) {
		candidates = append(candidates, ResolveFeatherlessModelID(m))
	}
	if IsDeepInfraAliasedModel(m) {
		candidates = append(candidates, ResolveDeepInfraModelID(m))
	}
	if IsMiniMaxAliasedModel(m) {
		candidates = append(candidates, ResolveMiniMaxModelID(m))
	}
	if strings.HasPrefix(m, "models/") {
		candidates = append(candidates, strings.TrimSpace(strings.TrimPrefix(m, "models/")))
	}
	if strings.HasSuffix(m, "-latest") {
		candidates = append(candidates, strings.TrimSpace(strings.TrimSuffix(m, "-latest")))
	}
	if strings.HasPrefix(m, "models/") && strings.HasSuffix(m, "-latest") {
		trimmed := strings.TrimSpace(strings.TrimSuffix(strings.TrimPrefix(m, "models/"), "-latest"))
		candidates = append(candidates, trimmed)
	}
	for _, candidate := range candidates {
		candidate = strings.TrimSpace(candidate)
		if candidate == "" {
			continue
		}
		if catalogHasModelID(catalog, candidate) {
			return candidate
		}
	}
	return ""
}

func catalogHasModelID(catalog *LLMCatalog, model string) bool {
	m := strings.TrimSpace(model)
	if m == "" || catalog == nil {
		return false
	}
	for i := range catalog.ChatModels {
		if catalog.ChatModels[i].ID == m {
			return true
		}
	}
	for i := range catalog.EmbeddingModels {
		if catalog.EmbeddingModels[i].ID == m {
			return true
		}
	}
	return false
}

func findModelCatalog(model string) *LLMModelCatalog {
	return findModelCatalogInCatalog(LLMCatalogData(), model)
}

func CatalogModelByID(model string) *LLMModelCatalog {
	return findModelCatalog(model)
}

func CatalogModelByIDInCatalog(catalog *LLMCatalog, model string) *LLMModelCatalog {
	return findModelCatalogInCatalog(catalog, model)
}

func CatalogChatModelByIDInCatalog(catalog *LLMCatalog, model string) *LLMModelCatalog {
	entry := findModelCatalogInCatalog(catalog, model)
	if entry == nil || CatalogIsEmbeddingModelInCatalog(catalog, model) {
		return nil
	}
	return entry
}

func providerCatalogByID(provider string) *LLMProviderCatalog {
	p := strings.TrimSpace(provider)
	if p == "" {
		return nil
	}
	catalog := LLMCatalogData()
	for i := range catalog.Providers {
		if catalog.Providers[i].ID == p {
			return &catalog.Providers[i]
		}
	}
	return nil
}

func CatalogProviderForModel(model string) string {
	m := strings.TrimSpace(model)
	if m == "" {
		return ""
	}
	if IsOpenRouterAliasedModel(m) {
		return "openrouter"
	}
	if IsPoeAliasedModel(m) {
		return "poe"
	}
	if IsSiliconFlowAliasedModel(m) {
		return "siliconflow"
	}
	if IsTogetherAliasedModel(m) {
		return "together"
	}
	if IsFeatherlessAliasedModel(m) {
		return "featherless"
	}
	if IsDeepInfraAliasedModel(m) {
		return "deepinfra"
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

func CatalogModelCapabilitiesInCatalog(catalog *LLMCatalog, model string) *LLMModelCapabilities {
	entry := findModelCatalogInCatalog(catalog, model)
	if entry == nil {
		return nil
	}
	return entry.Capabilities
}

func CatalogModelSupportsCapability(model, capability string) bool {
	return CatalogModelSupportsCapabilityInCatalog(LLMCatalogData(), model, capability)
}

func CatalogModelSupportsCapabilityInCatalog(catalog *LLMCatalog, model, capability string) bool {
	caps := CatalogModelCapabilitiesInCatalog(catalog, model)
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
	return CatalogModelSupportsPurposeInCatalog(LLMCatalogData(), model, purpose)
}

func CatalogModelSupportsPurposeInCatalog(catalog *LLMCatalog, model, purpose string) bool {
	entry := findModelCatalogInCatalog(catalog, model)
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
	return CatalogIsEmbeddingModelInCatalog(LLMCatalogData(), model)
}

func CatalogIsEmbeddingModelInCatalog(catalog *LLMCatalog, model string) bool {
	entry := findModelCatalogInCatalog(catalog, model)
	if entry == nil {
		return false
	}
	for _, candidate := range catalog.EmbeddingModels {
		if candidate.ID == entry.ID {
			return true
		}
	}
	return false
}
