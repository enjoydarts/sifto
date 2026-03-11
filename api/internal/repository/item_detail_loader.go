package repository

import (
	"context"

	"github.com/enjoydarts/sifto/api/internal/model"
)

func (r *ItemRepo) loadFactsDetail(ctx context.Context, itemID string, detail *model.ItemDetail) error {
	facts, err := r.queryFactsDetail(ctx, itemID)
	if err != nil {
		return err
	}
	detail.Facts = facts
	if llm, llmErr := loadLatestItemLLMUsage(ctx, r.db, itemID, "facts"); llmErr == nil {
		detail.FactsLLM = llm
	}
	if check, checkErr := loadFactsCheck(ctx, r.db, itemID); checkErr == nil {
		detail.FactsCheck = check
		if llm, llmErr := loadLatestItemLLMUsage(ctx, r.db, itemID, "facts_check"); llmErr == nil {
			detail.FactsCheckLLM = llm
		}
	}
	return nil
}

func (r *ItemRepo) loadSummaryDetail(ctx context.Context, itemID string, detail *model.ItemDetail) error {
	summary, err := r.querySummaryDetail(ctx, itemID)
	if err != nil {
		return err
	}
	detail.Summary = summary
	if llm, llmErr := loadLatestItemLLMUsage(ctx, r.db, itemID, "summary"); llmErr == nil {
		detail.SummaryLLM = llm
	}
	if check, checkErr := loadFaithfulnessCheck(ctx, r.db, itemID); checkErr == nil {
		detail.Faithfulness = check
		if llm, llmErr := loadLatestItemLLMUsage(ctx, r.db, itemID, "faithfulness_check"); llmErr == nil {
			detail.FaithfulnessLLM = llm
		}
	}
	return nil
}
