package repository

import (
	"context"
	"time"

	"github.com/enjoydarts/sifto/api/internal/model"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

type ReadingGoalRepo struct{ db *pgxpool.Pool }

func NewReadingGoalRepo(db *pgxpool.Pool) *ReadingGoalRepo { return &ReadingGoalRepo{db: db} }

func (r *ReadingGoalRepo) ListByUser(ctx context.Context, userID string) ([]model.ReadingGoal, error) {
	rows, err := r.db.Query(ctx, `
		SELECT id, user_id, title, description, priority, status, due_date, created_at, updated_at
		FROM reading_goals
		WHERE user_id = $1
		ORDER BY
			CASE WHEN status = 'active' THEN 0 ELSE 1 END,
			priority DESC,
			updated_at DESC`,
		userID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := []model.ReadingGoal{}
	for rows.Next() {
		var goal model.ReadingGoal
		if err := rows.Scan(
			&goal.ID,
			&goal.UserID,
			&goal.Title,
			&goal.Description,
			&goal.Priority,
			&goal.Status,
			&goal.DueDate,
			&goal.CreatedAt,
			&goal.UpdatedAt,
		); err != nil {
			return nil, err
		}
		out = append(out, goal)
	}
	return out, rows.Err()
}

func (r *ReadingGoalRepo) Upsert(ctx context.Context, goal model.ReadingGoal) (model.ReadingGoal, error) {
	if goal.ID == "" {
		goal.ID = uuid.NewString()
	}
	if goal.Status == "" {
		goal.Status = "active"
	}
	err := r.db.QueryRow(ctx, `
		INSERT INTO reading_goals (id, user_id, title, description, priority, status, due_date)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		ON CONFLICT (id) DO UPDATE
		SET title = EXCLUDED.title,
		    description = EXCLUDED.description,
		    priority = EXCLUDED.priority,
		    status = EXCLUDED.status,
		    due_date = EXCLUDED.due_date,
		    updated_at = now()
		RETURNING created_at, updated_at`,
		goal.ID, goal.UserID, goal.Title, goal.Description, goal.Priority, goal.Status, goal.DueDate,
	).Scan(&goal.CreatedAt, &goal.UpdatedAt)
	if err != nil {
		return model.ReadingGoal{}, err
	}
	return goal, nil
}

func (r *ReadingGoalRepo) Delete(ctx context.Context, userID, id string) error {
	_, err := r.db.Exec(ctx, `DELETE FROM reading_goals WHERE user_id = $1 AND id = $2`, userID, id)
	return err
}

func (r *ReadingGoalRepo) SetStatus(ctx context.Context, userID, id, status string) (model.ReadingGoal, error) {
	var goal model.ReadingGoal
	var dueDate *time.Time
	err := r.db.QueryRow(ctx, `
		UPDATE reading_goals
		SET status = $3, updated_at = now()
		WHERE user_id = $1 AND id = $2
		RETURNING id, user_id, title, description, priority, status, due_date, created_at, updated_at`,
		userID, id, status,
	).Scan(
		&goal.ID,
		&goal.UserID,
		&goal.Title,
		&goal.Description,
		&goal.Priority,
		&goal.Status,
		&dueDate,
		&goal.CreatedAt,
		&goal.UpdatedAt,
	)
	if err != nil {
		return model.ReadingGoal{}, mapDBError(err)
	}
	goal.DueDate = dueDate
	return goal, nil
}
