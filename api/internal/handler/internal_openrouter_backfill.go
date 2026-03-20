package handler

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/enjoydarts/sifto/api/internal/repository"
	"github.com/enjoydarts/sifto/api/internal/service"
	"github.com/enjoydarts/sifto/api/internal/timeutil"
)

type openRouterBackfillUpdate struct {
	PricingModelFamily *string
	PricingSource      string
	EstimatedCostUSD   float64
	Zeroed             bool
}

func strPtr(v string) *string {
	if strings.TrimSpace(v) == "" {
		return nil
	}
	value := strings.TrimSpace(v)
	return &value
}

func buildOpenRouterBackfillUpdate(row repository.LLMUsageLog) openRouterBackfillUpdate {
	usage := &service.LLMUsage{
		Provider:                 strings.TrimSpace(row.Provider),
		Model:                    strings.TrimSpace(row.Model),
		RequestedModel:           strings.TrimSpace(derefString(row.RequestedModel)),
		ResolvedModel:            strings.TrimSpace(derefString(row.ResolvedModel)),
		PricingModelFamily:       strings.TrimSpace(derefString(row.PricingModelFamily)),
		PricingSource:            strings.TrimSpace(row.PricingSource),
		OpenRouterCostUSD:        row.OpenRouterCostUSD,
		OpenRouterGenerationID:   strings.TrimSpace(derefString(row.OpenRouterGenerationID)),
		InputTokens:              row.InputTokens,
		OutputTokens:             row.OutputTokens,
		CacheCreationInputTokens: row.CacheCreationInputTokens,
		CacheReadInputTokens:     row.CacheReadInputTokens,
		EstimatedCostUSD:         row.EstimatedCostUSD,
	}
	normalized := service.NormalizeCatalogPricedUsage(row.Purpose, usage)
	if normalized != nil && normalized.EstimatedCostUSD >= 0 {
		family := strings.TrimSpace(normalized.PricingModelFamily)
		if family != "" && family != service.OpenRouterAliasModelID("auto") {
			return openRouterBackfillUpdate{
				PricingModelFamily: strPtr(family),
				PricingSource:      strings.TrimSpace(normalized.PricingSource),
				EstimatedCostUSD:   normalized.EstimatedCostUSD,
			}
		}
	}
	return openRouterBackfillUpdate{
		PricingModelFamily: nil,
		PricingSource:      "openrouter_backfill_zeroed",
		EstimatedCostUSD:   0,
		Zeroed:             true,
	}
}

func derefString(v *string) string {
	if v == nil {
		return ""
	}
	return *v
}

func parseOptionalBackfillTime(raw *string, inclusiveEnd bool) (*time.Time, error) {
	if raw == nil || strings.TrimSpace(*raw) == "" {
		return nil, nil
	}
	value := strings.TrimSpace(*raw)
	if ts, err := time.Parse(time.RFC3339, value); err == nil {
		return &ts, nil
	}
	if day, err := time.ParseInLocation("2006-01-02", value, timeutil.JST); err == nil {
		if inclusiveEnd {
			next := day.Add(24 * time.Hour)
			return &next, nil
		}
		return &day, nil
	}
	return nil, fmt.Errorf("invalid time format: %s", value)
}

func (h *InternalHandler) DebugBackfillOpenRouterCosts(w http.ResponseWriter, r *http.Request) {
	if !checkInternalSecret(r) {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}
	if h.db == nil {
		http.Error(w, "openrouter backfill unavailable", http.StatusInternalServerError)
		return
	}

	var body struct {
		UserID *string `json:"user_id"`
		Limit  int     `json:"limit"`
		DryRun bool    `json:"dry_run"`
		From   *string `json:"from"`
		To     *string `json:"to"`
	}
	_ = json.NewDecoder(r.Body).Decode(&body)
	if body.Limit <= 0 {
		body.Limit = 200
	}
	if body.Limit > 5000 {
		http.Error(w, "invalid limit", http.StatusBadRequest)
		return
	}
	from, err := parseOptionalBackfillTime(body.From, false)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	to, err := parseOptionalBackfillTime(body.To, true)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	repo := repository.NewLLMUsageLogRepo(h.db)
	targets, err := repo.ListOpenRouterBackfillCandidates(r.Context(), body.UserID, body.Limit, from, to)
	if err != nil {
		http.Error(w, fmt.Sprintf("list openrouter backfill targets: %v", err), http.StatusInternalServerError)
		return
	}

	repaired := 0
	zeroed := 0
	failed := 0
	errorSamples := make([]map[string]any, 0, 10)
	preview := make([]map[string]any, 0, len(targets))
	touchedUsers := make(map[string]struct{})
	for _, row := range targets {
		update := buildOpenRouterBackfillUpdate(row)
		preview = append(preview, map[string]any{
			"id":                   row.ID,
			"user_id":              derefString(row.UserID),
			"requested_model":      derefString(row.RequestedModel),
			"resolved_model":       derefString(row.ResolvedModel),
			"old_pricing_source":   row.PricingSource,
			"old_estimated_cost":   row.EstimatedCostUSD,
			"new_pricing_source":   update.PricingSource,
			"new_estimated_cost":   update.EstimatedCostUSD,
			"pricing_model_family": derefString(update.PricingModelFamily),
			"zeroed":               update.Zeroed,
		})
		if body.DryRun {
			if update.Zeroed {
				zeroed++
			} else {
				repaired++
			}
			continue
		}
		if err := repo.UpdateOpenRouterBackfill(r.Context(), row.ID, update.PricingModelFamily, update.PricingSource, update.EstimatedCostUSD); err != nil {
			failed++
			if len(errorSamples) < 10 {
				errorSamples = append(errorSamples, map[string]any{
					"id":    row.ID,
					"error": err.Error(),
				})
			}
			continue
		}
		if update.Zeroed {
			zeroed++
		} else {
			repaired++
		}
		if row.UserID != nil && strings.TrimSpace(*row.UserID) != "" {
			touchedUsers[strings.TrimSpace(*row.UserID)] = struct{}{}
		}
	}
	if !body.DryRun {
		for userID := range touchedUsers {
			_ = service.BumpUserLLMUsageCacheVersion(r.Context(), h.cache, userID)
		}
	}

	w.WriteHeader(http.StatusAccepted)
	writeJSON(w, map[string]any{
		"status":        "accepted",
		"dry_run":       body.DryRun,
		"user_filter":   body.UserID,
		"limit":         body.Limit,
		"from":          body.From,
		"to":            body.To,
		"matched":       len(targets),
		"repaired":      repaired,
		"zeroed":        zeroed,
		"failed":        failed,
		"error_samples": errorSamples,
		"targets":       preview,
	})
}
