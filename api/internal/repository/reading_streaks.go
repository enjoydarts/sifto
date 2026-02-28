package repository

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

type ReadingStreakRepo struct{ db *pgxpool.Pool }

func NewReadingStreakRepo(db *pgxpool.Pool) *ReadingStreakRepo { return &ReadingStreakRepo{db: db} }

func (r *ReadingStreakRepo) GetByUserAndDate(ctx context.Context, userID, date string) (readCount int, streakDays int, isCompleted bool, err error) {
	err = r.db.QueryRow(ctx, `
		SELECT read_count, streak_days, is_completed
		FROM reading_streaks
		WHERE user_id = $1 AND streak_date = $2`,
		userID, date,
	).Scan(&readCount, &streakDays, &isCompleted)
	if err != nil {
		return 0, 0, false, mapDBError(err)
	}
	return readCount, streakDays, isCompleted, nil
}

func (r *ReadingStreakRepo) IncrementRead(ctx context.Context, userID string, date time.Time, minReadCount int) error {
	if minReadCount <= 0 {
		minReadCount = 3
	}
	dateStr := date.Format("2006-01-02")
	prevDateStr := date.AddDate(0, 0, -1).Format("2006-01-02")
	_, err := r.db.Exec(ctx, `
		WITH prev AS (
			SELECT streak_days, is_completed
			FROM reading_streaks
			WHERE user_id = $1 AND streak_date = $2
		), up AS (
			INSERT INTO reading_streaks (user_id, streak_date, read_count, streak_days, is_completed)
			VALUES (
			  $1, $3, 1,
			  CASE
			    WHEN EXISTS (SELECT 1 FROM prev WHERE is_completed = true)
			      THEN COALESCE((SELECT streak_days FROM prev), 0) + 1
			    ELSE 1
			  END,
			  (1 >= $4)
			)
			ON CONFLICT (user_id, streak_date) DO UPDATE SET
			  read_count = reading_streaks.read_count + 1,
			  is_completed = (reading_streaks.read_count + 1) >= $4,
			  updated_at = NOW()
			RETURNING read_count, is_completed, streak_days
		)
		UPDATE reading_streaks rs
		SET streak_days = CASE
			WHEN up.is_completed = true AND rs.streak_days < up.streak_days THEN up.streak_days
			ELSE rs.streak_days
		END
		FROM up
		WHERE rs.user_id = $1 AND rs.streak_date = $3`,
		userID, prevDateStr, dateStr, minReadCount,
	)
	return err
}
