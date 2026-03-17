package repository

import (
	"context"
	"encoding/json"
	"time"

	"github.com/enjoydarts/sifto/api/internal/model"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

type SourceOptimizationRepo struct{ db *pgxpool.Pool }

func NewSourceOptimizationRepo(db *pgxpool.Pool) *SourceOptimizationRepo {
	return &SourceOptimizationRepo{db: db}
}

type SourceOptimizationMetrics struct {
	UnreadBacklog        int     `json:"unread_backlog"`
	ReadRate             float64 `json:"read_rate"`
	FavoriteRate         float64 `json:"favorite_rate"`
	NotificationOpenRate float64 `json:"notification_open_rate"`
	AverageSummaryScore  float64 `json:"average_summary_score"`
}

func (r *SourceOptimizationRepo) ListLatestByUser(ctx context.Context, userID string) ([]model.SourceOptimizationSnapshot, error) {
	rows, err := r.db.Query(ctx, `
		SELECT DISTINCT ON (sos.source_id)
			sos.id, sos.user_id, sos.source_id, sos.window_start, sos.window_end,
			sos.metrics, sos.recommendation, sos.reason, sos.created_at
		FROM source_optimization_snapshots sos
		WHERE sos.user_id = $1
		ORDER BY sos.source_id, sos.created_at DESC`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []model.SourceOptimizationSnapshot{}
	for rows.Next() {
		var snap model.SourceOptimizationSnapshot
		var (
			windowStart time.Time
			windowEnd   time.Time
			metricsRaw  []byte
			metrics     map[string]any
		)
		if err := rows.Scan(&snap.ID, &snap.UserID, &snap.SourceID, &windowStart, &windowEnd, &metricsRaw, &snap.Recommendation, &snap.Reason, &snap.CreatedAt); err != nil {
			return nil, err
		}
		_ = json.Unmarshal(metricsRaw, &metrics)
		snap.WindowStart = windowStart.Format("2006-01-02")
		snap.WindowEnd = windowEnd.Format("2006-01-02")
		snap.Metrics = metrics
		out = append(out, snap)
	}
	return out, rows.Err()
}

func (r *SourceOptimizationRepo) CollectMetrics(ctx context.Context, userID, sourceID string, since time.Time) (SourceOptimizationMetrics, error) {
	var out SourceOptimizationMetrics
	err := r.db.QueryRow(ctx, `
		WITH scoped AS (
			SELECT i.id, i.source_id, COALESCE(sm.score, 0) AS score,
			       (ir.item_id IS NOT NULL) AS is_read,
			       COALESCE(fb.is_favorite, false) AS is_favorite
			FROM items i
			JOIN sources s ON s.id = i.source_id
			LEFT JOIN item_summaries sm ON sm.item_id = i.id
			LEFT JOIN item_reads ir ON ir.item_id = i.id AND ir.user_id = $1
			LEFT JOIN item_feedbacks fb ON fb.item_id = i.id AND fb.user_id = $1
			WHERE s.user_id = $1
			  AND i.source_id = $2
			  AND i.deleted_at IS NULL
			  AND i.created_at >= $3
		)
		SELECT
			COUNT(*) FILTER (WHERE is_read = false)::int AS unread_backlog,
			COALESCE(AVG(CASE WHEN is_read THEN 1.0 ELSE 0.0 END), 0),
			COALESCE(AVG(CASE WHEN is_favorite THEN 1.0 ELSE 0.0 END), 0),
			COALESCE(AVG(score), 0)
		FROM scoped`, userID, sourceID, since,
	).Scan(&out.UnreadBacklog, &out.ReadRate, &out.FavoriteRate, &out.AverageSummaryScore)
	if err != nil {
		return out, err
	}
	// Open rate is not yet captured; keep this field stable for future use.
	out.NotificationOpenRate = 0
	return out, nil
}

func (r *SourceOptimizationRepo) InsertSnapshot(ctx context.Context, userID, sourceID string, windowStart, windowEnd time.Time, metrics SourceOptimizationMetrics, recommendation, reason string) error {
	metricsRaw, err := json.Marshal(metrics)
	if err != nil {
		return err
	}
	_, err = r.db.Exec(ctx, `
		INSERT INTO source_optimization_snapshots (id, user_id, source_id, window_start, window_end, metrics, recommendation, reason)
		VALUES ($1, $2, $3, $4::date, $5::date, $6, $7, $8)`,
		uuid.NewString(), userID, sourceID, windowStart.Format("2006-01-02"), windowEnd.Format("2006-01-02"), metricsRaw, recommendation, reason,
	)
	return err
}
