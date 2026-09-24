package service

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type JevCatalog struct {
	SchemaVersion int              `json:"schema_version"`
	DefaultModel  string           `json:"default_model"`
	Models        []JevModelConfig `json:"models"`
	GatePolicy    JevGatePolicy    `json:"gate_policy"`
}

type JevModelConfig struct {
	ID               string   `json:"id"`
	MatchExact       []string `json:"match_exact"`
	MatchPrefix      []string `json:"match_prefix"`
	PricingSource    string   `json:"pricing_source"`
	InputPerMTokUSD  float64  `json:"input_per_mtok_usd"`
	OutputPerMTokUSD float64  `json:"output_per_mtok_usd"`
}

func LoadJevCatalog() (JevCatalog, error) {
	data, err := os.ReadFile(jevCatalogPath())
	if err != nil {
		return JevCatalog{}, fmt.Errorf("read Jev catalog: %w", err)
	}
	var catalog JevCatalog
	if err := json.Unmarshal(data, &catalog); err != nil {
		return JevCatalog{}, fmt.Errorf("decode Jev catalog: %w", err)
	}
	if strings.TrimSpace(catalog.DefaultModel) == "" || len(catalog.Models) == 0 || strings.TrimSpace(catalog.GatePolicy.Version) == "" {
		return JevCatalog{}, fmt.Errorf("invalid Jev catalog")
	}
	return catalog, nil
}

func jevCatalogPath() string {
	if value := strings.TrimSpace(os.Getenv("JEV_CATALOG_PATH")); value != "" {
		return value
	}
	if value := strings.TrimSpace(os.Getenv("LLM_CATALOG_PATH")); value != "" {
		return filepath.Join(filepath.Dir(value), "jev_catalog.json")
	}
	candidates := []string{
		"/shared/jev_catalog.json",
		filepath.Join("shared", "jev_catalog.json"),
		filepath.Join("..", "shared", "jev_catalog.json"),
		filepath.Join("..", "..", "shared", "jev_catalog.json"),
		filepath.Join("..", "..", "..", "shared", "jev_catalog.json"),
	}
	for _, candidate := range candidates {
		if _, err := os.Stat(candidate); err == nil {
			return candidate
		}
	}
	return candidates[0]
}

func (c JevCatalog) ResolvePricing(responseModel, requestedModel string) (JevModelConfig, bool) {
	for _, value := range []string{responseModel, requestedModel} {
		value = strings.TrimSpace(value)
		for _, model := range c.Models {
			if value == model.ID || containsJevModel(model.MatchExact, value) {
				return model, true
			}
		}
	}
	responseModel = strings.TrimSpace(responseModel)
	for _, model := range c.Models {
		for _, prefix := range model.MatchPrefix {
			if prefix != "" && strings.HasPrefix(responseModel, prefix) {
				return model, true
			}
		}
	}
	for _, model := range c.Models {
		if model.ID == c.DefaultModel || containsJevModel(model.MatchExact, c.DefaultModel) {
			return model, true
		}
	}
	return JevModelConfig{}, false
}

func containsJevModel(values []string, target string) bool {
	for _, value := range values {
		if strings.TrimSpace(value) == target {
			return true
		}
	}
	return false
}
