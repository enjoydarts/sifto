package repository

import (
	"context"
	"log"

	"github.com/enjoydarts/sifto/api/internal/model"
	"github.com/jackc/pgx/v5"
)

func normalizeExecutionAttemptsForDetail(attempts []model.ItemLLMExecutionAttempt, limit int) []model.ItemLLMExecutionAttempt {
	if len(attempts) == 0 {
		return nil
	}
	if limit > 0 && len(attempts) > limit {
		attempts = attempts[:limit]
	}
	out := make([]model.ItemLLMExecutionAttempt, len(attempts))
	for i := range attempts {
		out[len(attempts)-1-i] = attempts[i]
	}
	return out
}

func loadLatestItemLLMExecutionAttempts(ctx context.Context, r *ItemRepo, itemID string, purposes []string, limit int) ([]model.ItemLLMExecutionAttempt, error) {
	if len(purposes) == 0 {
		return nil, nil
	}
	query := `
		SELECT provider, model, purpose, status, attempt_index, error_kind, error_message, created_at
		FROM llm_execution_events
		WHERE item_id = $1 AND purpose = ANY($2)
		ORDER BY created_at DESC`
	var (
		rows pgx.Rows
		err  error
	)
	if limit > 0 {
		query += "\n\t\tLIMIT $3"
		rows, err = r.db.Query(ctx, query, itemID, purposes, limit)
	} else {
		rows, err = r.db.Query(ctx, query, itemID, purposes)
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]model.ItemLLMExecutionAttempt, 0)
	for rows.Next() {
		var attempt model.ItemLLMExecutionAttempt
		if err := rows.Scan(
			&attempt.Provider,
			&attempt.Model,
			&attempt.Purpose,
			&attempt.Status,
			&attempt.AttemptIndex,
			&attempt.ErrorKind,
			&attempt.ErrorMessage,
			&attempt.CreatedAt,
		); err != nil {
			return nil, err
		}
		out = append(out, attempt)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return normalizeExecutionAttemptsForDetail(out, limit), nil
}

func (r *ItemRepo) loadFactsDetail(ctx context.Context, itemID string, detail *model.ItemDetail) error {
	facts, err := r.queryFactsDetail(ctx, itemID)
	if err != nil {
		return err
	}
	detail.Facts = facts
	if llm, llmErr := loadLatestItemLLMUsage(ctx, r.db, itemID, "facts"); llmErr == nil {
		detail.FactsLLM = llm
	}
	if attempts, attemptsErr := loadLatestItemLLMExecutionAttempts(ctx, r, itemID, []string{"facts", "facts_localization"}, 0); attemptsErr == nil {
		detail.FactsExecutions = attempts
	} else {
		log.Printf("item detail facts executions load failed item_id=%s err=%v", itemID, attemptsErr)
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
	if attempts, attemptsErr := loadLatestItemLLMExecutionAttempts(ctx, r, itemID, []string{"summary"}, 0); attemptsErr == nil {
		detail.SummaryExecutions = attempts
	} else {
		log.Printf("item detail summary executions load failed item_id=%s err=%v", itemID, attemptsErr)
	}
	if check, checkErr := loadFaithfulnessCheck(ctx, r.db, itemID); checkErr == nil {
		detail.Faithfulness = check
		if llm, llmErr := loadLatestItemLLMUsage(ctx, r.db, itemID, "faithfulness_check"); llmErr == nil {
			detail.FaithfulnessLLM = llm
		}
	}
	return nil
}
