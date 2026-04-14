package service

import (
	"context"
	"time"

	"github.com/enjoydarts/sifto/api/internal/repository"
)

type LLMUsageLogView struct {
	ID                       string    `json:"id"`
	UserID                   *string   `json:"user_id,omitempty"`
	SourceID                 *string   `json:"source_id,omitempty"`
	ItemID                   *string   `json:"item_id,omitempty"`
	DigestID                 *string   `json:"digest_id,omitempty"`
	Provider                 string    `json:"provider"`
	Model                    string    `json:"model"`
	RequestedModel           *string   `json:"requested_model,omitempty"`
	ResolvedModel            *string   `json:"resolved_model,omitempty"`
	PricingModelFamily       *string   `json:"pricing_model_family,omitempty"`
	PricingSource            string    `json:"pricing_source"`
	OpenRouterCostUSD        *float64  `json:"openrouter_cost_usd,omitempty"`
	OpenRouterGenerationID   *string   `json:"openrouter_generation_id,omitempty"`
	Purpose                  string    `json:"purpose"`
	InputTokens              int       `json:"input_tokens"`
	OutputTokens             int       `json:"output_tokens"`
	CacheCreationInputTokens int       `json:"cache_creation_input_tokens"`
	CacheReadInputTokens     int       `json:"cache_read_input_tokens"`
	EstimatedCostUSD         float64   `json:"estimated_cost_usd"`
	CreatedAt                time.Time `json:"created_at"`
}

type LLMUsageDailySummaryView struct {
	DateJST                  string  `json:"date_jst"`
	Provider                 string  `json:"provider"`
	Purpose                  string  `json:"purpose"`
	PricingSource            string  `json:"pricing_source"`
	Calls                    int     `json:"calls"`
	InputTokens              int64   `json:"input_tokens"`
	OutputTokens             int64   `json:"output_tokens"`
	CacheCreationInputTokens int64   `json:"cache_creation_input_tokens"`
	CacheReadInputTokens     int64   `json:"cache_read_input_tokens"`
	EstimatedCostUSD         float64 `json:"estimated_cost_usd"`
}

type LLMUsageModelSummaryView struct {
	Provider                 string  `json:"provider"`
	Model                    string  `json:"model"`
	PricingSource            string  `json:"pricing_source"`
	Calls                    int     `json:"calls"`
	InputTokens              int64   `json:"input_tokens"`
	OutputTokens             int64   `json:"output_tokens"`
	CacheCreationInputTokens int64   `json:"cache_creation_input_tokens"`
	CacheReadInputTokens     int64   `json:"cache_read_input_tokens"`
	EstimatedCostUSD         float64 `json:"estimated_cost_usd"`
}

type LLMUsageProviderMonthSummaryView struct {
	MonthJST                 string  `json:"month_jst"`
	Provider                 string  `json:"provider"`
	Calls                    int     `json:"calls"`
	InputTokens              int64   `json:"input_tokens"`
	OutputTokens             int64   `json:"output_tokens"`
	CacheCreationInputTokens int64   `json:"cache_creation_input_tokens"`
	CacheReadInputTokens     int64   `json:"cache_read_input_tokens"`
	EstimatedCostUSD         float64 `json:"estimated_cost_usd"`
}

type LLMUsagePurposeMonthSummaryView struct {
	MonthJST                 string  `json:"month_jst"`
	Purpose                  string  `json:"purpose"`
	Calls                    int     `json:"calls"`
	InputTokens              int64   `json:"input_tokens"`
	OutputTokens             int64   `json:"output_tokens"`
	CacheCreationInputTokens int64   `json:"cache_creation_input_tokens"`
	CacheReadInputTokens     int64   `json:"cache_read_input_tokens"`
	EstimatedCostUSD         float64 `json:"estimated_cost_usd"`
}

type LLMUsageAnalysisSummaryView struct {
	Provider                 string  `json:"provider"`
	Model                    string  `json:"model"`
	Purpose                  string  `json:"purpose"`
	PricingSource            string  `json:"pricing_source"`
	Calls                    int     `json:"calls"`
	InputTokens              int64   `json:"input_tokens"`
	OutputTokens             int64   `json:"output_tokens"`
	CacheCreationInputTokens int64   `json:"cache_creation_input_tokens"`
	CacheReadInputTokens     int64   `json:"cache_read_input_tokens"`
	EstimatedCostUSD         float64 `json:"estimated_cost_usd"`
}

type LLMExecutionCurrentMonthSummaryView struct {
	MonthJST         string  `json:"month_jst"`
	Purpose          string  `json:"purpose"`
	Provider         string  `json:"provider"`
	Model            string  `json:"model"`
	Attempts         int     `json:"attempts"`
	Successes        int     `json:"successes"`
	Failures         int     `json:"failures"`
	Retries          int     `json:"retries"`
	EmptyResponses   int     `json:"empty_responses"`
	EstimatedCostUSD float64 `json:"estimated_cost_usd"`
	FailureRatePct   float64 `json:"failure_rate_pct"`
	RetryRatePct     float64 `json:"retry_rate_pct"`
	EmptyRatePct     float64 `json:"empty_rate_pct"`
}

type LLMUsageService struct {
	repo          *repository.LLMUsageLogRepo
	executionRepo *repository.LLMExecutionEventRepo
	valueRepo     *repository.LLMValueMetricsRepo
}

func NewLLMUsageService(repo *repository.LLMUsageLogRepo, executionRepo *repository.LLMExecutionEventRepo, valueRepo *repository.LLMValueMetricsRepo) *LLMUsageService {
	return &LLMUsageService{repo: repo, executionRepo: executionRepo, valueRepo: valueRepo}
}

func mapSlice[T any, U any](in []T, fn func(T) U) []U {
	if len(in) == 0 {
		return []U{}
	}
	out := make([]U, len(in))
	for i, v := range in {
		out[i] = fn(v)
	}
	return out
}

func mapUsageLogView(v repository.LLMUsageLog) LLMUsageLogView {
	return LLMUsageLogView(v)
}

func mapDailySummaryView(v repository.LLMUsageDailySummary) LLMUsageDailySummaryView {
	return LLMUsageDailySummaryView(v)
}

func mapModelSummaryView(v repository.LLMUsageModelSummary) LLMUsageModelSummaryView {
	return LLMUsageModelSummaryView(v)
}

func mapProviderMonthSummaryView(v repository.LLMUsageProviderMonthSummary) LLMUsageProviderMonthSummaryView {
	return LLMUsageProviderMonthSummaryView(v)
}

func mapPurposeMonthSummaryView(v repository.LLMUsagePurposeMonthSummary) LLMUsagePurposeMonthSummaryView {
	return LLMUsagePurposeMonthSummaryView(v)
}

func mapAnalysisSummaryView(v repository.LLMUsageAnalysisSummary) LLMUsageAnalysisSummaryView {
	return LLMUsageAnalysisSummaryView(v)
}

func mapExecutionMonthSummaryView(v repository.LLMExecutionCurrentMonthSummary) LLMExecutionCurrentMonthSummaryView {
	return LLMExecutionCurrentMonthSummaryView(v)
}

func (s *LLMUsageService) List(ctx context.Context, userID string, limit int) ([]LLMUsageLogView, error) {
	rows, err := s.repo.ListByUser(ctx, userID, limit)
	if err != nil {
		return nil, err
	}
	return mapSlice(rows, mapUsageLogView), nil
}

func (s *LLMUsageService) ListMonth(ctx context.Context, userID string, limit int, month time.Time) ([]LLMUsageLogView, error) {
	rows, err := s.repo.ListByUserMonth(ctx, userID, limit, month)
	if err != nil {
		return nil, err
	}
	return mapSlice(rows, mapUsageLogView), nil
}

func (s *LLMUsageService) DailySummary(ctx context.Context, userID string, days int) ([]LLMUsageDailySummaryView, error) {
	rows, err := s.repo.DailySummaryByUser(ctx, userID, days)
	if err != nil {
		return nil, err
	}
	return mapSlice(rows, mapDailySummaryView), nil
}

func (s *LLMUsageService) DailySummaryMonth(ctx context.Context, userID string, month time.Time) ([]LLMUsageDailySummaryView, error) {
	rows, err := s.repo.DailySummaryByUserMonth(ctx, userID, month)
	if err != nil {
		return nil, err
	}
	return mapSlice(rows, mapDailySummaryView), nil
}

func (s *LLMUsageService) ModelSummary(ctx context.Context, userID string, days int) ([]LLMUsageModelSummaryView, error) {
	rows, err := s.repo.ModelSummaryByUser(ctx, userID, days)
	if err != nil {
		return nil, err
	}
	return mapSlice(rows, mapModelSummaryView), nil
}

func (s *LLMUsageService) ModelSummaryMonth(ctx context.Context, userID string, month time.Time) ([]LLMUsageModelSummaryView, error) {
	rows, err := s.repo.ModelSummaryByUserMonth(ctx, userID, month)
	if err != nil {
		return nil, err
	}
	return mapSlice(rows, mapModelSummaryView), nil
}

func (s *LLMUsageService) ProviderSummaryCurrentMonth(ctx context.Context, userID string) ([]LLMUsageProviderMonthSummaryView, error) {
	rows, err := s.repo.ProviderSummaryCurrentMonthByUser(ctx, userID)
	if err != nil {
		return nil, err
	}
	return mapSlice(rows, mapProviderMonthSummaryView), nil
}

func (s *LLMUsageService) ProviderSummaryMonth(ctx context.Context, userID string, month time.Time) ([]LLMUsageProviderMonthSummaryView, error) {
	rows, err := s.repo.ProviderSummaryByUserMonth(ctx, userID, month)
	if err != nil {
		return nil, err
	}
	return mapSlice(rows, mapProviderMonthSummaryView), nil
}

func (s *LLMUsageService) PurposeSummaryCurrentMonth(ctx context.Context, userID string) ([]LLMUsagePurposeMonthSummaryView, error) {
	rows, err := s.repo.PurposeSummaryCurrentMonthByUser(ctx, userID)
	if err != nil {
		return nil, err
	}
	return mapSlice(rows, mapPurposeMonthSummaryView), nil
}

func (s *LLMUsageService) PurposeSummaryMonth(ctx context.Context, userID string, month time.Time) ([]LLMUsagePurposeMonthSummaryView, error) {
	rows, err := s.repo.PurposeSummaryByUserMonth(ctx, userID, month)
	if err != nil {
		return nil, err
	}
	return mapSlice(rows, mapPurposeMonthSummaryView), nil
}

func (s *LLMUsageService) ExecutionSummaryCurrentMonth(ctx context.Context, userID string) ([]LLMExecutionCurrentMonthSummaryView, error) {
	rows, err := s.executionRepo.CurrentMonthSummaryByUser(ctx, userID)
	if err != nil {
		return nil, err
	}
	return mapSlice(rows, mapExecutionMonthSummaryView), nil
}

func (s *LLMUsageService) ExecutionSummaryMonth(ctx context.Context, userID string, month time.Time) ([]LLMExecutionCurrentMonthSummaryView, error) {
	rows, err := s.executionRepo.SummaryByUserMonth(ctx, userID, month)
	if err != nil {
		return nil, err
	}
	return mapSlice(rows, mapExecutionMonthSummaryView), nil
}

func (s *LLMUsageService) ExecutionSummary(ctx context.Context, userID string, days int) ([]LLMExecutionCurrentMonthSummaryView, error) {
	rows, err := s.executionRepo.SummaryByUser(ctx, userID, days)
	if err != nil {
		return nil, err
	}
	return mapSlice(rows, mapExecutionMonthSummaryView), nil
}

func (s *LLMUsageService) AnalysisSummary(ctx context.Context, userID string, days int) ([]LLMUsageAnalysisSummaryView, error) {
	rows, err := s.repo.AnalysisSummaryByUser(ctx, userID, days)
	if err != nil {
		return nil, err
	}
	return mapSlice(rows, mapAnalysisSummaryView), nil
}
