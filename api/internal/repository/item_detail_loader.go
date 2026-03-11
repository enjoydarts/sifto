package repository

import (
	"context"

	"github.com/enjoydarts/sifto/api/internal/model"
)

func (r *ItemRepo) loadFactsDetail(ctx context.Context, itemID string, detail *model.ItemDetail) error {
	var facts model.ItemFacts
	err := r.db.QueryRow(ctx, `
		SELECT id, item_id, facts, extracted_at FROM item_facts WHERE item_id = $1`, itemID,
	).Scan(&facts.ID, &facts.ItemID, &facts.Facts, &facts.ExtractedAt)
	if err != nil {
		return err
	}
	detail.Facts = &facts
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
	var summary model.ItemSummary
	err := r.db.QueryRow(ctx, `
		SELECT id, item_id, summary, topics, translated_title, score, score_breakdown, score_reason, score_policy_version, summarized_at
		FROM item_summaries WHERE item_id = $1`, itemID,
	).Scan(&summary.ID, &summary.ItemID, &summary.Summary, &summary.Topics, &summary.TranslatedTitle, &summary.Score,
		scoreBreakdownScanner{dst: &summary.ScoreBreakdown}, &summary.ScoreReason, &summary.ScorePolicyVersion, &summary.SummarizedAt)
	if err != nil {
		return err
	}
	detail.Summary = &summary
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
