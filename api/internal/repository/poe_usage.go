package repository

import (
	"context"
	"encoding/json"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type PoeUsageRepo struct{ db *pgxpool.Pool }

func NewPoeUsageRepo(db *pgxpool.Pool) *PoeUsageRepo {
	return &PoeUsageRepo{db: db}
}

type PoeUsageSyncRun struct {
	ID            string     `json:"id"`
	UserID        string     `json:"user_id"`
	StartedAt     time.Time  `json:"started_at"`
	FinishedAt    *time.Time `json:"finished_at,omitempty"`
	Status        string     `json:"status"`
	SyncSource    string     `json:"sync_source"`
	FetchedCount  int        `json:"fetched_count"`
	InsertedCount int        `json:"inserted_count"`
	UpdatedCount  int        `json:"updated_count"`
	LatestEntryAt *time.Time `json:"latest_entry_at,omitempty"`
	OldestEntryAt *time.Time `json:"oldest_entry_at,omitempty"`
	ErrorMessage  *string    `json:"error_message,omitempty"`
}

type PoeUsageEntryRecord struct {
	QueryID    string
	BotName    string
	CreatedAt  time.Time
	CostUSD    float64
	RawCostUSD string
	CostPoints int
	Breakdown  map[string]string
	UsageType  string
	ChatName   string
}

type PoeUsageSummaryRow struct {
	EntryCount      int
	APIEntryCount   int
	ChatEntryCount  int
	TotalCostPoints int
	TotalCostUSD    float64
	LatestEntryAt   *time.Time
}

type PoeUsageModelRollupRow struct {
	BotName         string
	EntryCount      int
	TotalCostPoints int
	TotalCostUSD    float64
	LatestEntryAt   *time.Time
}

func (r *PoeUsageRepo) StartSyncRun(ctx context.Context, userID, syncSource string) (string, error) {
	var id string
	err := r.db.QueryRow(ctx, `
		INSERT INTO poe_usage_sync_runs (user_id, sync_source, status)
		VALUES ($1, $2, 'running')
		RETURNING id
	`, userID, syncSource).Scan(&id)
	return id, err
}

func (r *PoeUsageRepo) FinishSyncRun(
	ctx context.Context,
	syncRunID string,
	fetchedCount, insertedCount, updatedCount int,
	oldestEntryAt, latestEntryAt *time.Time,
	errMsg *string,
) error {
	status := "success"
	if errMsg != nil && *errMsg != "" {
		status = "failed"
	}
	_, err := r.db.Exec(ctx, `
		UPDATE poe_usage_sync_runs
		SET finished_at = NOW(),
		    status = $2,
		    fetched_count = $3,
		    inserted_count = $4,
		    updated_count = $5,
		    oldest_entry_at = $6,
		    latest_entry_at = $7,
		    error_message = $8
		WHERE id = $1
	`, syncRunID, status, fetchedCount, insertedCount, updatedCount, oldestEntryAt, latestEntryAt, errMsg)
	return err
}

func (r *PoeUsageRepo) GetLatestSyncRun(ctx context.Context, userID string) (*PoeUsageSyncRun, error) {
	var run PoeUsageSyncRun
	err := r.db.QueryRow(ctx, `
		SELECT id, user_id, started_at, finished_at, status, sync_source, fetched_count, inserted_count, updated_count,
		       latest_entry_at, oldest_entry_at, error_message
		FROM poe_usage_sync_runs
		WHERE user_id = $1
		ORDER BY started_at DESC
		LIMIT 1
	`, userID).Scan(
		&run.ID, &run.UserID, &run.StartedAt, &run.FinishedAt, &run.Status, &run.SyncSource, &run.FetchedCount,
		&run.InsertedCount, &run.UpdatedCount, &run.LatestEntryAt, &run.OldestEntryAt, &run.ErrorMessage,
	)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	return &run, nil
}

func (r *PoeUsageRepo) UpsertEntries(ctx context.Context, userID, syncRunID string, entries []PoeUsageEntryRecord) (insertedCount, updatedCount int, err error) {
	if len(entries) == 0 {
		return 0, 0, nil
	}
	tx, err := r.db.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return 0, 0, err
	}
	defer tx.Rollback(ctx)
	for _, entry := range entries {
		breakdownJSON, err := json.Marshal(entry.Breakdown)
		if err != nil {
			return 0, 0, err
		}
		var xmax uint32
		if err := tx.QueryRow(ctx, `
			INSERT INTO poe_usage_entries (
				user_id, sync_run_id, query_id, bot_name, created_at, cost_usd, raw_cost_usd,
				cost_points, cost_breakdown_in_points, usage_type, chat_name
			) VALUES (
				$1, $2, $3, $4, $5, $6, $7,
				$8, $9::jsonb, $10, $11
			)
			ON CONFLICT (user_id, query_id) DO UPDATE
			SET sync_run_id = EXCLUDED.sync_run_id,
			    bot_name = EXCLUDED.bot_name,
			    created_at = EXCLUDED.created_at,
			    cost_usd = EXCLUDED.cost_usd,
			    raw_cost_usd = EXCLUDED.raw_cost_usd,
			    cost_points = EXCLUDED.cost_points,
			    cost_breakdown_in_points = EXCLUDED.cost_breakdown_in_points,
			    usage_type = EXCLUDED.usage_type,
			    chat_name = EXCLUDED.chat_name,
			    updated_at = NOW()
			RETURNING xmax
		`, userID, syncRunID, entry.QueryID, entry.BotName, entry.CreatedAt, entry.CostUSD, entry.RawCostUSD, entry.CostPoints, string(breakdownJSON), entry.UsageType, entry.ChatName).Scan(&xmax); err != nil {
			return 0, 0, err
		}
		if xmax == 0 {
			insertedCount++
		} else {
			updatedCount++
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return 0, 0, err
	}
	return insertedCount, updatedCount, nil
}

func (r *PoeUsageRepo) SummaryBetween(ctx context.Context, userID string, from, until time.Time) (PoeUsageSummaryRow, error) {
	var out PoeUsageSummaryRow
	err := r.db.QueryRow(ctx, `
		SELECT
			COUNT(*)::int,
			COUNT(*) FILTER (WHERE LOWER(usage_type) <> 'chat')::int,
			COUNT(*) FILTER (WHERE LOWER(usage_type) = 'chat')::int,
			COALESCE(SUM(cost_points), 0)::int,
			COALESCE(SUM(cost_usd), 0)::float8,
			MAX(created_at)
		FROM poe_usage_entries
		WHERE user_id = $1
		  AND created_at >= $2
		  AND created_at < $3
	`, userID, from, until).Scan(
		&out.EntryCount, &out.APIEntryCount, &out.ChatEntryCount, &out.TotalCostPoints, &out.TotalCostUSD, &out.LatestEntryAt,
	)
	return out, err
}

func (r *PoeUsageRepo) ListModelRollupsBetween(ctx context.Context, userID string, from, until time.Time, limit int) ([]PoeUsageModelRollupRow, error) {
	if limit <= 0 {
		limit = 20
	}
	rows, err := r.db.Query(ctx, `
		SELECT
			COALESCE(NULLIF(bot_name, ''), 'Unknown') AS bot_name,
			COUNT(*)::int,
			COALESCE(SUM(cost_points), 0)::int,
			COALESCE(SUM(cost_usd), 0)::float8,
			MAX(created_at)
		FROM poe_usage_entries
		WHERE user_id = $1
		  AND created_at >= $2
		  AND created_at < $3
		GROUP BY 1
		ORDER BY 3 DESC, 4 DESC, 1 ASC
		LIMIT $4
	`, userID, from, until, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]PoeUsageModelRollupRow, 0)
	for rows.Next() {
		var row PoeUsageModelRollupRow
		if err := rows.Scan(&row.BotName, &row.EntryCount, &row.TotalCostPoints, &row.TotalCostUSD, &row.LatestEntryAt); err != nil {
			return nil, err
		}
		out = append(out, row)
	}
	return out, rows.Err()
}

func (r *PoeUsageRepo) ListEntriesBetween(ctx context.Context, userID string, from, until time.Time, limit int) ([]PoeUsageEntryRecord, error) {
	if limit <= 0 {
		limit = 100
	}
	rows, err := r.db.Query(ctx, `
		SELECT query_id, bot_name, created_at, cost_usd::float8, raw_cost_usd, cost_points, cost_breakdown_in_points, usage_type, chat_name
		FROM poe_usage_entries
		WHERE user_id = $1
		  AND created_at >= $2
		  AND created_at < $3
		ORDER BY created_at DESC, query_id DESC
		LIMIT $4
	`, userID, from, until, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]PoeUsageEntryRecord, 0)
	for rows.Next() {
		var row PoeUsageEntryRecord
		var breakdownBytes []byte
		if err := rows.Scan(
			&row.QueryID, &row.BotName, &row.CreatedAt, &row.CostUSD, &row.RawCostUSD, &row.CostPoints, &breakdownBytes, &row.UsageType, &row.ChatName,
		); err != nil {
			return nil, err
		}
		if len(breakdownBytes) > 0 {
			_ = json.Unmarshal(breakdownBytes, &row.Breakdown)
		}
		if row.Breakdown == nil {
			row.Breakdown = make(map[string]string)
		}
		out = append(out, row)
	}
	return out, rows.Err()
}
