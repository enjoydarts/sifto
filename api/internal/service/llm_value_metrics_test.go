package service

import (
	"math"
	"testing"

	"github.com/enjoydarts/sifto/api/internal/repository"
)

func TestLLMValueMetricsCostToRead(t *testing.T) {
	row := mapLLMValueMetricView(fakeLLMValueMetricAggregate(0.12, 4, 0, 0))
	if row.CostToReadUSD == nil || math.Abs(*row.CostToReadUSD-0.03) > 1e-9 {
		t.Fatalf("cost_to_read = %#v, want 0.03", row.CostToReadUSD)
	}
}

func TestLLMValueMetricsCostToFavorite(t *testing.T) {
	row := mapLLMValueMetricView(fakeLLMValueMetricAggregate(0.25, 0, 5, 0))
	if row.CostToFavoriteUSD == nil || math.Abs(*row.CostToFavoriteUSD-0.05) > 1e-9 {
		t.Fatalf("cost_to_favorite = %#v, want 0.05", row.CostToFavoriteUSD)
	}
}

func TestLLMValueMetricsCostToInsight(t *testing.T) {
	row := mapLLMValueMetricView(fakeLLMValueMetricAggregate(0.42, 0, 0, 3))
	if row.CostToInsightUSD == nil || math.Abs(*row.CostToInsightUSD-0.14) > 1e-9 {
		t.Fatalf("cost_to_insight = %#v, want 0.14", row.CostToInsightUSD)
	}
}

func TestLLMValueMetricsComparesModelChangeBeforeAfter(t *testing.T) {
	rows := finalizeLLMValueMetrics([]LLMValueMetricView{
		{
			Purpose:           "summary",
			Provider:          "openai",
			Model:             "gpt-4.1",
			TotalCostUSD:      0.36,
			Calls:             6,
			FavoriteCount:     1,
			CostToFavoriteUSD: metricFloatPtr(0.36),
			AdvisoryCode:      "ok",
		},
		{
			Purpose:           "summary",
			Provider:          "openai",
			Model:             "gpt-4.1-mini",
			TotalCostUSD:      0.09,
			Calls:             6,
			FavoriteCount:     2,
			CostToFavoriteUSD: metricFloatPtr(0.045),
			AdvisoryCode:      "ok",
		},
	})

	if rows[0].AdvisoryCode != "review_model" {
		t.Fatalf("expensive row advisory = %s, want review_model", rows[0].AdvisoryCode)
	}
	if rows[0].AdvisoryReason == nil || *rows[0].AdvisoryReason == "" {
		t.Fatalf("expensive row advisory reason is empty")
	}
	if rows[0].BenchmarkModel == nil || *rows[0].BenchmarkModel != "gpt-4.1-mini" {
		t.Fatalf("benchmark model = %#v, want gpt-4.1-mini", rows[0].BenchmarkModel)
	}
	if rows[1].LowEfficiencyFlag {
		t.Fatalf("cheaper row low_efficiency = true, want false")
	}
}

func TestLLMValueMetricsLowSignalIncludesReason(t *testing.T) {
	rows := finalizeLLMValueMetrics([]LLMValueMetricView{
		{
			Purpose:       "summary",
			Provider:      "openai",
			Model:         "gpt-4.1",
			TotalCostUSD:  0.14,
			Calls:         5,
			ReadCount:     2,
			FavoriteCount: 0,
			InsightCount:  0,
			CostToReadUSD: metricFloatPtr(0.07),
			AdvisoryCode:  "ok",
		},
	})

	if rows[0].AdvisoryCode != "low_signal" {
		t.Fatalf("advisory = %s, want low_signal", rows[0].AdvisoryCode)
	}
	if rows[0].AdvisoryReason == nil || *rows[0].AdvisoryReason == "" {
		t.Fatalf("low_signal advisory reason is empty")
	}
}

func fakeLLMValueMetricAggregate(total float64, reads, favorites, insights int) repository.LLMValueMetricAggregate {
	return repository.LLMValueMetricAggregate{
		MonthJST:      "2026-03",
		Purpose:       "summary",
		Provider:      "openai",
		Model:         "gpt-4.1-mini",
		PricingSource: "catalog",
		Calls:         4,
		TotalCostUSD:  total,
		ReadCount:     reads,
		FavoriteCount: favorites,
		InsightCount:  insights,
	}
}

func metricFloatPtr(v float64) *float64 { return &v }
