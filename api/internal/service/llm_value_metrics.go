package service

import (
	"context"
	"fmt"
	"math"

	"github.com/enjoydarts/sifto/api/internal/repository"
)

type LLMValueMetricView struct {
	WindowStart       string   `json:"window_start"`
	WindowEnd         string   `json:"window_end"`
	MonthJST          string   `json:"month_jst"`
	Purpose           string   `json:"purpose"`
	Provider          string   `json:"provider"`
	Model             string   `json:"model"`
	PricingSource     string   `json:"pricing_source"`
	Calls             int      `json:"calls"`
	TotalCostUSD      float64  `json:"total_cost_usd"`
	ItemCount         int      `json:"item_count"`
	ReadCount         int      `json:"read_count"`
	FavoriteCount     int      `json:"favorite_count"`
	InsightCount      int      `json:"insight_count"`
	CostToReadUSD     *float64 `json:"cost_to_read_usd,omitempty"`
	CostToFavoriteUSD *float64 `json:"cost_to_favorite_usd,omitempty"`
	CostToInsightUSD  *float64 `json:"cost_to_insight_usd,omitempty"`
	LowEfficiencyFlag bool     `json:"low_efficiency_flag"`
	AdvisoryCode      string   `json:"advisory_code"`
	AdvisoryReason    *string  `json:"advisory_reason,omitempty"`
	BenchmarkProvider *string  `json:"benchmark_provider,omitempty"`
	BenchmarkModel    *string  `json:"benchmark_model,omitempty"`
	BenchmarkMetric   *string  `json:"benchmark_metric,omitempty"`
}

type benchmarkCandidate struct {
	provider string
	model    string
	value    float64
}

func costPer(total float64, count int) *float64 {
	if count <= 0 {
		return nil
	}
	v := total / float64(count)
	return &v
}

func selectPreferredMetric(row LLMValueMetricView) (string, float64, bool) {
	if row.CostToInsightUSD != nil {
		return "insight", *row.CostToInsightUSD, true
	}
	if row.CostToFavoriteUSD != nil {
		return "favorite", *row.CostToFavoriteUSD, true
	}
	if row.CostToReadUSD != nil {
		return "read", *row.CostToReadUSD, true
	}
	return "", 0, false
}

func mapLLMValueMetricView(v repository.LLMValueMetricAggregate) LLMValueMetricView {
	return LLMValueMetricView{
		WindowStart:       v.WindowStart.Format("2006-01-02"),
		WindowEnd:         v.WindowEnd.Format("2006-01-02"),
		MonthJST:          v.MonthJST,
		Purpose:           v.Purpose,
		Provider:          v.Provider,
		Model:             v.Model,
		PricingSource:     v.PricingSource,
		Calls:             v.Calls,
		TotalCostUSD:      v.TotalCostUSD,
		ItemCount:         v.ItemCount,
		ReadCount:         v.ReadCount,
		FavoriteCount:     v.FavoriteCount,
		InsightCount:      v.InsightCount,
		CostToReadUSD:     costPer(v.TotalCostUSD, v.ReadCount),
		CostToFavoriteUSD: costPer(v.TotalCostUSD, v.FavoriteCount),
		CostToInsightUSD:  costPer(v.TotalCostUSD, v.InsightCount),
		AdvisoryCode:      "ok",
	}
}

func finalizeLLMValueMetrics(rows []LLMValueMetricView) []LLMValueMetricView {
	bestRead := map[string]benchmarkCandidate{}
	bestFavorite := map[string]benchmarkCandidate{}
	bestInsight := map[string]benchmarkCandidate{}

	for _, row := range rows {
		if row.CostToReadUSD != nil {
			cur := bestRead[row.Purpose]
			if cur.value == 0 || *row.CostToReadUSD < cur.value {
				bestRead[row.Purpose] = benchmarkCandidate{provider: row.Provider, model: row.Model, value: *row.CostToReadUSD}
			}
		}
		if row.CostToFavoriteUSD != nil {
			cur := bestFavorite[row.Purpose]
			if cur.value == 0 || *row.CostToFavoriteUSD < cur.value {
				bestFavorite[row.Purpose] = benchmarkCandidate{provider: row.Provider, model: row.Model, value: *row.CostToFavoriteUSD}
			}
		}
		if row.CostToInsightUSD != nil {
			cur := bestInsight[row.Purpose]
			if cur.value == 0 || *row.CostToInsightUSD < cur.value {
				bestInsight[row.Purpose] = benchmarkCandidate{provider: row.Provider, model: row.Model, value: *row.CostToInsightUSD}
			}
		}
	}

	out := make([]LLMValueMetricView, len(rows))
	for i, row := range rows {
		next := row
		metric, metricValue, ok := selectPreferredMetric(row)
		if !ok {
			if row.TotalCostUSD >= 0.02 && row.Calls >= 2 {
				next.LowEfficiencyFlag = true
				next.AdvisoryCode = "low_signal"
				reason := fmt.Sprintf("Cost is %s across %d calls, but no read, favorite, or insight signal is attached yet.", usdCompact(row.TotalCostUSD), row.Calls)
				next.AdvisoryReason = &reason
			}
			out[i] = next
			continue
		}

		var benchmark benchmarkCandidate
		switch metric {
		case "insight":
			benchmark = bestInsight[row.Purpose]
		case "favorite":
			benchmark = bestFavorite[row.Purpose]
		default:
			benchmark = bestRead[row.Purpose]
		}

		if benchmark.value > 0 && (benchmark.provider != row.Provider || benchmark.model != row.Model) {
			if metricValue > benchmark.value*1.75 && row.TotalCostUSD >= 0.05 {
				next.LowEfficiencyFlag = true
				next.AdvisoryCode = "review_model"
				reason := fmt.Sprintf("%s per %s is %sx higher than %s/%s in the same purpose.", usdCompact(metricValue), metric, ratioCompact(metricValue, benchmark.value), benchmark.provider, benchmark.model)
				next.AdvisoryReason = &reason
				next.BenchmarkProvider = &benchmark.provider
				next.BenchmarkModel = &benchmark.model
				next.BenchmarkMetric = &metric
			}
		}

		if metric == "read" && row.FavoriteCount == 0 && row.InsightCount == 0 && row.TotalCostUSD >= math.Max(0.05, metricValue*2) {
			next.LowEfficiencyFlag = true
			next.AdvisoryCode = "low_signal"
			reason := fmt.Sprintf("Reads happened, but favorites and insights are still 0 while cost reached %s.", usdCompact(row.TotalCostUSD))
			next.AdvisoryReason = &reason
			next.BenchmarkProvider = nil
			next.BenchmarkModel = nil
			next.BenchmarkMetric = nil
		}

		out[i] = next
	}
	return out
}

func (s *LLMUsageService) ValueMetricsCurrentMonth(ctx context.Context, userID string) ([]LLMValueMetricView, error) {
	if s.valueRepo == nil {
		return []LLMValueMetricView{}, nil
	}
	raw, err := s.valueRepo.CollectCurrentMonth(ctx, userID)
	if err != nil {
		return nil, err
	}
	rows := make([]LLMValueMetricView, len(raw))
	for i, row := range raw {
		rows[i] = mapLLMValueMetricView(row)
	}
	rows = finalizeLLMValueMetrics(rows)

	snapshots := make([]repository.LLMValueMetricSnapshot, len(rows))
	for i, row := range rows {
		snapshots[i] = repository.LLMValueMetricSnapshot{
			WindowStart:       row.WindowStart,
			WindowEnd:         row.WindowEnd,
			MonthJST:          row.MonthJST,
			Purpose:           row.Purpose,
			Provider:          row.Provider,
			Model:             row.Model,
			PricingSource:     row.PricingSource,
			Calls:             row.Calls,
			TotalCostUSD:      row.TotalCostUSD,
			ItemCount:         row.ItemCount,
			ReadCount:         row.ReadCount,
			FavoriteCount:     row.FavoriteCount,
			InsightCount:      row.InsightCount,
			CostToReadUSD:     row.CostToReadUSD,
			CostToFavoriteUSD: row.CostToFavoriteUSD,
			CostToInsightUSD:  row.CostToInsightUSD,
			LowEfficiencyFlag: row.LowEfficiencyFlag,
			AdvisoryCode:      row.AdvisoryCode,
			AdvisoryReason:    row.AdvisoryReason,
			BenchmarkProvider: row.BenchmarkProvider,
			BenchmarkModel:    row.BenchmarkModel,
			BenchmarkMetric:   row.BenchmarkMetric,
		}
	}
	if err := s.valueRepo.ReplaceCurrentMonth(ctx, userID, snapshots); err != nil {
		return nil, err
	}
	return rows, nil
}

func ratioCompact(a, b float64) string {
	if b <= 0 {
		return "0.0"
	}
	return fmt.Sprintf("%.1f", a/b)
}

func usdCompact(v float64) string {
	if v >= 1 {
		return fmt.Sprintf("$%.2f", v)
	}
	if v >= 0.01 {
		return fmt.Sprintf("$%.3f", v)
	}
	return fmt.Sprintf("$%.4f", v)
}
