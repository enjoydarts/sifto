package repository

import (
	"context"
	"encoding/json"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/minoru-kitayama/sifto/api/internal/model"
)

type BriefingSnapshotRepo struct{ db *pgxpool.Pool }

func NewBriefingSnapshotRepo(db *pgxpool.Pool) *BriefingSnapshotRepo {
	return &BriefingSnapshotRepo{db: db}
}

type BriefingSnapshot struct {
	ID           string
	UserID       string
	BriefingDate string
	Status       string
	PayloadJSON  []byte
	GeneratedAt  *time.Time
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

func (r *BriefingSnapshotRepo) GetByUserAndDate(ctx context.Context, userID, date string) (*BriefingSnapshot, error) {
	var s BriefingSnapshot
	err := r.db.QueryRow(ctx, `
		SELECT id, user_id, briefing_date::text, status, COALESCE(payload_json, '{}'::jsonb), generated_at, created_at, updated_at
		FROM briefing_snapshots
		WHERE user_id = $1 AND briefing_date = $2`,
		userID, date,
	).Scan(&s.ID, &s.UserID, &s.BriefingDate, &s.Status, &s.PayloadJSON, &s.GeneratedAt, &s.CreatedAt, &s.UpdatedAt)
	if err != nil {
		return nil, mapDBError(err)
	}
	return &s, nil
}

func (r *BriefingSnapshotRepo) Upsert(ctx context.Context, userID, date, status string, payload *model.BriefingTodayResponse) error {
	var payloadJSON []byte
	if payload != nil {
		b, err := json.Marshal(payload)
		if err != nil {
			return err
		}
		payloadJSON = b
	} else {
		payloadJSON = []byte(`{}`)
	}
	_, err := r.db.Exec(ctx, `
		INSERT INTO briefing_snapshots (user_id, briefing_date, status, payload_json, generated_at)
		VALUES ($1, $2, $3, $4, NOW())
		ON CONFLICT (user_id, briefing_date) DO UPDATE SET
		  status = EXCLUDED.status,
		  payload_json = EXCLUDED.payload_json,
		  generated_at = EXCLUDED.generated_at,
		  updated_at = NOW()`,
		userID, date, status, payloadJSON,
	)
	return err
}
