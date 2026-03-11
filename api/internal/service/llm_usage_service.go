package service

import (
	"context"

	"github.com/enjoydarts/sifto/api/internal/repository"
)

type LLMUsageService struct {
	repo          *repository.LLMUsageLogRepo
	executionRepo *repository.LLMExecutionEventRepo
}

func NewLLMUsageService(repo *repository.LLMUsageLogRepo, executionRepo *repository.LLMExecutionEventRepo) *LLMUsageService {
	return &LLMUsageService{repo: repo, executionRepo: executionRepo}
}

func (s *LLMUsageService) List(ctx context.Context, userID string, limit int) ([]repository.LLMUsageLog, error) {
	return s.repo.ListByUser(ctx, userID, limit)
}

func (s *LLMUsageService) DailySummary(ctx context.Context, userID string, days int) ([]repository.LLMUsageDailySummary, error) {
	return s.repo.DailySummaryByUser(ctx, userID, days)
}

func (s *LLMUsageService) ModelSummary(ctx context.Context, userID string, days int) ([]repository.LLMUsageModelSummary, error) {
	return s.repo.ModelSummaryByUser(ctx, userID, days)
}

func (s *LLMUsageService) ProviderSummaryCurrentMonth(ctx context.Context, userID string) ([]repository.LLMUsageProviderMonthSummary, error) {
	return s.repo.ProviderSummaryCurrentMonthByUser(ctx, userID)
}

func (s *LLMUsageService) PurposeSummaryCurrentMonth(ctx context.Context, userID string) ([]repository.LLMUsagePurposeMonthSummary, error) {
	return s.repo.PurposeSummaryCurrentMonthByUser(ctx, userID)
}

func (s *LLMUsageService) ExecutionSummaryCurrentMonth(ctx context.Context, userID string) ([]repository.LLMExecutionCurrentMonthSummary, error) {
	return s.executionRepo.CurrentMonthSummaryByUser(ctx, userID)
}
