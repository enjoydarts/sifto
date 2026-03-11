package service

import (
	"context"

	"github.com/enjoydarts/sifto/api/internal/repository"
)

type LLMUsageLogView = repository.LLMUsageLog
type LLMUsageDailySummaryView = repository.LLMUsageDailySummary
type LLMUsageModelSummaryView = repository.LLMUsageModelSummary
type LLMUsageProviderMonthSummaryView = repository.LLMUsageProviderMonthSummary
type LLMUsagePurposeMonthSummaryView = repository.LLMUsagePurposeMonthSummary
type LLMExecutionCurrentMonthSummaryView = repository.LLMExecutionCurrentMonthSummary

type LLMUsageService struct {
	repo          *repository.LLMUsageLogRepo
	executionRepo *repository.LLMExecutionEventRepo
}

func NewLLMUsageService(repo *repository.LLMUsageLogRepo, executionRepo *repository.LLMExecutionEventRepo) *LLMUsageService {
	return &LLMUsageService{repo: repo, executionRepo: executionRepo}
}

func mapSlice[T any](in []T) []T {
	out := make([]T, len(in))
	copy(out, in)
	return out
}

func (s *LLMUsageService) List(ctx context.Context, userID string, limit int) ([]LLMUsageLogView, error) {
	rows, err := s.repo.ListByUser(ctx, userID, limit)
	if err != nil {
		return nil, err
	}
	return mapSlice(rows), nil
}

func (s *LLMUsageService) DailySummary(ctx context.Context, userID string, days int) ([]LLMUsageDailySummaryView, error) {
	rows, err := s.repo.DailySummaryByUser(ctx, userID, days)
	if err != nil {
		return nil, err
	}
	return mapSlice(rows), nil
}

func (s *LLMUsageService) ModelSummary(ctx context.Context, userID string, days int) ([]LLMUsageModelSummaryView, error) {
	rows, err := s.repo.ModelSummaryByUser(ctx, userID, days)
	if err != nil {
		return nil, err
	}
	return mapSlice(rows), nil
}

func (s *LLMUsageService) ProviderSummaryCurrentMonth(ctx context.Context, userID string) ([]LLMUsageProviderMonthSummaryView, error) {
	rows, err := s.repo.ProviderSummaryCurrentMonthByUser(ctx, userID)
	if err != nil {
		return nil, err
	}
	return mapSlice(rows), nil
}

func (s *LLMUsageService) PurposeSummaryCurrentMonth(ctx context.Context, userID string) ([]LLMUsagePurposeMonthSummaryView, error) {
	rows, err := s.repo.PurposeSummaryCurrentMonthByUser(ctx, userID)
	if err != nil {
		return nil, err
	}
	return mapSlice(rows), nil
}

func (s *LLMUsageService) ExecutionSummaryCurrentMonth(ctx context.Context, userID string) ([]LLMExecutionCurrentMonthSummaryView, error) {
	rows, err := s.executionRepo.CurrentMonthSummaryByUser(ctx, userID)
	if err != nil {
		return nil, err
	}
	return mapSlice(rows), nil
}
