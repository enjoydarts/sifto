package repository

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/enjoydarts/sifto/api/internal/model"
	"github.com/jackc/pgx/v5/pgxpool"
)

type SourceRepo struct{ db *pgxpool.Pool }

func NewSourceRepo(db *pgxpool.Pool) *SourceRepo { return &SourceRepo{db} }

func (r *SourceRepo) CountByUser(ctx context.Context, userID string) (int, error) {
	var n int
	if err := r.db.QueryRow(ctx, `SELECT COUNT(*)::int FROM sources WHERE user_id = $1`, userID).Scan(&n); err != nil {
		return 0, err
	}
	return n, nil
}

func (r *SourceRepo) List(ctx context.Context, userID string) ([]model.Source, error) {
	rows, err := r.db.Query(ctx, `
		SELECT id, user_id, url, type, title, enabled, last_fetched_at, created_at, updated_at
		FROM sources WHERE user_id = $1 ORDER BY created_at DESC`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var sources []model.Source
	for rows.Next() {
		var s model.Source
		if err := rows.Scan(&s.ID, &s.UserID, &s.URL, &s.Type, &s.Title,
			&s.Enabled, &s.LastFetchedAt, &s.CreatedAt, &s.UpdatedAt); err != nil {
			return nil, err
		}
		sources = append(sources, s)
	}
	return sources, nil
}

func (r *SourceRepo) Create(ctx context.Context, userID, url, srcType string, title *string) (*model.Source, error) {
	var s model.Source
	err := r.db.QueryRow(ctx, `
		INSERT INTO sources (user_id, url, type, title)
		VALUES ($1, $2, $3, $4)
		RETURNING id, user_id, url, type, title, enabled, last_fetched_at, created_at, updated_at`,
		userID, url, srcType, title,
	).Scan(&s.ID, &s.UserID, &s.URL, &s.Type, &s.Title,
		&s.Enabled, &s.LastFetchedAt, &s.CreatedAt, &s.UpdatedAt)
	if err != nil {
		return nil, mapDBError(err)
	}
	return &s, nil
}

func (r *SourceRepo) Update(ctx context.Context, id, userID string, enabled *bool, updateTitle bool, title *string) (*model.Source, error) {
	var s model.Source
	err := r.db.QueryRow(ctx, `
		UPDATE sources
		SET enabled = COALESCE($1, enabled),
		    title = CASE WHEN $2 THEN $3 ELSE title END,
		    updated_at = NOW()
		WHERE id = $4 AND user_id = $5
		RETURNING id, user_id, url, type, title, enabled, last_fetched_at, created_at, updated_at`,
		enabled, updateTitle, title, id, userID,
	).Scan(&s.ID, &s.UserID, &s.URL, &s.Type, &s.Title,
		&s.Enabled, &s.LastFetchedAt, &s.CreatedAt, &s.UpdatedAt)
	if err != nil {
		return nil, mapDBError(err)
	}
	return &s, nil
}

func (r *SourceRepo) Delete(ctx context.Context, id, userID string) error {
	tag, err := r.db.Exec(ctx,
		`DELETE FROM sources WHERE id = $1 AND user_id = $2`, id, userID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *SourceRepo) ListEnabled(ctx context.Context) ([]model.Source, error) {
	rows, err := r.db.Query(ctx, `
		SELECT id, user_id, url, type, title, enabled, last_fetched_at, created_at, updated_at
		FROM sources WHERE enabled = true AND type = 'rss'`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var sources []model.Source
	for rows.Next() {
		var s model.Source
		if err := rows.Scan(&s.ID, &s.UserID, &s.URL, &s.Type, &s.Title,
			&s.Enabled, &s.LastFetchedAt, &s.CreatedAt, &s.UpdatedAt); err != nil {
			return nil, err
		}
		sources = append(sources, s)
	}
	return sources, nil
}

func (r *SourceRepo) UpdateLastFetchedAt(ctx context.Context, id string, fetchedAt time.Time) error {
	_, err := r.db.Exec(ctx, `
		UPDATE sources
		SET last_fetched_at = $1, updated_at = NOW()
		WHERE id = $2`,
		fetchedAt, id)
	return err
}

func (r *SourceRepo) GetUserIDBySourceID(ctx context.Context, sourceID string) (string, error) {
	var userID string
	err := r.db.QueryRow(ctx, `SELECT user_id FROM sources WHERE id = $1`, sourceID).Scan(&userID)
	if err != nil {
		return "", mapDBError(err)
	}
	return userID, nil
}

func (r *SourceRepo) HealthByUser(ctx context.Context, userID string) ([]model.SourceHealth, error) {
	sources, err := r.List(ctx, userID)
	if err != nil {
		return nil, err
	}
	snapshotBySourceID := map[string]model.SourceHealth{}
	rows, err := r.db.Query(ctx, `
		SELECT sh.source_id, sh.total_items, sh.failed_items, sh.summarized_items,
		       sh.failure_rate, sh.last_item_at, sh.last_fetched_at, sh.status
		FROM source_health_snapshots sh
		JOIN sources s ON s.id = sh.source_id
		WHERE s.user_id = $1`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var h model.SourceHealth
		if err := rows.Scan(
			&h.SourceID,
			&h.TotalItems,
			&h.FailedItems,
			&h.Summarized,
			&h.FailureRate,
			&h.LastItemAt,
			&h.LastFetchedAt,
			&h.Status,
		); err != nil {
			return nil, err
		}
		snapshotBySourceID[h.SourceID] = h
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	aggBySourceID := map[string]model.SourceHealth{}
	aggRows, err := r.db.Query(ctx, `
		SELECT
			s.id AS source_id,
			s.enabled AS enabled,
			s.last_fetched_at,
			COUNT(i.id)::int AS total_items,
			COUNT(*) FILTER (WHERE i.status = 'failed')::int AS failed_items,
			COUNT(*) FILTER (WHERE i.status = 'summarized')::int AS summarized_items,
			MAX(i.created_at) AS last_item_at
		FROM sources s
		LEFT JOIN items i ON i.source_id = s.id AND i.deleted_at IS NULL
		WHERE s.user_id = $1
		GROUP BY s.id, s.enabled, s.last_fetched_at`, userID)
	if err != nil {
		return nil, err
	}
	defer aggRows.Close()
	for aggRows.Next() {
		var (
			h       model.SourceHealth
			enabled bool
		)
		if err := aggRows.Scan(
			&h.SourceID,
			&enabled,
			&h.LastFetchedAt,
			&h.TotalItems,
			&h.FailedItems,
			&h.Summarized,
			&h.LastItemAt,
		); err != nil {
			return nil, err
		}
		h.Status = deriveSourceHealthStatus(enabled, h.TotalItems, h.FailedItems, h.FailureRate, h.LastFetchedAt)
		if h.TotalItems > 0 && h.FailureRate == 0 {
			h.FailureRate = float64(h.FailedItems) / float64(h.TotalItems)
		}
		aggBySourceID[h.SourceID] = h
	}
	if err := aggRows.Err(); err != nil {
		return nil, err
	}

	out := make([]model.SourceHealth, 0, len(sources))
	for _, s := range sources {
		if snap, ok := snapshotBySourceID[s.ID]; ok {
			out = append(out, snap)
			continue
		}
		h, ok := aggBySourceID[s.ID]
		if !ok {
			h = model.SourceHealth{
				SourceID:      s.ID,
				TotalItems:    0,
				FailedItems:   0,
				Summarized:    0,
				FailureRate:   0,
				LastFetchedAt: s.LastFetchedAt,
				Status:        deriveSourceHealthStatus(s.Enabled, 0, 0, 0, s.LastFetchedAt),
			}
		}
		out = append(out, h)
	}
	return out, nil
}

func (r *SourceRepo) ItemStatsByUser(ctx context.Context, userID string) ([]model.SourceItemStats, error) {
	rows, err := r.db.Query(ctx, `
		SELECT
			s.id AS source_id,
			COUNT(i.id)::int AS total_items,
			COUNT(i.id) FILTER (WHERE ir.item_id IS NULL)::int AS unread_items,
			COUNT(i.id) FILTER (WHERE ir.item_id IS NOT NULL)::int AS read_items,
			COALESCE(
				COUNT(*) FILTER (WHERE i.created_at >= NOW() - INTERVAL '30 days')::float8 /
					NULLIF(
						COUNT(DISTINCT CASE
							WHEN i.created_at >= NOW() - INTERVAL '30 days'
							THEN (i.created_at AT TIME ZONE 'Asia/Tokyo')::date
						END),
						0
					),
				0
			) AS avg_items_per_day_30d
		FROM sources s
		LEFT JOIN items i ON i.source_id = s.id AND i.deleted_at IS NULL
		LEFT JOIN item_reads ir ON ir.item_id = i.id AND ir.user_id = $1
		WHERE s.user_id = $1
		GROUP BY s.id
		ORDER BY total_items DESC, s.created_at DESC`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := []model.SourceItemStats{}
	for rows.Next() {
		var stat model.SourceItemStats
		if err := rows.Scan(
			&stat.SourceID,
			&stat.TotalItems,
			&stat.UnreadItems,
			&stat.ReadItems,
			&stat.AvgItemsPerDay30Days,
		); err != nil {
			return nil, err
		}
		out = append(out, stat)
	}
	return out, rows.Err()
}

func (r *SourceRepo) DailyStatsByUser(ctx context.Context, userID string, days int) ([]model.SourceDailyStats, error) {
	if days <= 0 {
		days = 30
	}
	if days > 90 {
		days = 90
	}

	type dayRow struct {
		SourceID string
		Day      time.Time
		Count    int
	}

	rows, err := r.db.Query(ctx, `
		SELECT
			i.source_id,
			(i.created_at AT TIME ZONE 'Asia/Tokyo')::date AS day,
			COUNT(*)::int AS item_count
		FROM items i
		JOIN sources s ON s.id = i.source_id
		WHERE s.user_id = $1
		  AND i.deleted_at IS NULL
		  AND (i.created_at AT TIME ZONE 'Asia/Tokyo')::date >= ((NOW() AT TIME ZONE 'Asia/Tokyo')::date - ($2::int - 1))
		GROUP BY i.source_id, day
		ORDER BY day ASC`, userID, days)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	seriesBySource := map[string]map[string]int{}
	for rows.Next() {
		var row dayRow
		if err := rows.Scan(&row.SourceID, &row.Day, &row.Count); err != nil {
			return nil, err
		}
		if _, ok := seriesBySource[row.SourceID]; !ok {
			seriesBySource[row.SourceID] = map[string]int{}
		}
		seriesBySource[row.SourceID][row.Day.Format("2006-01-02")] = row.Count
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	sources, err := r.List(ctx, userID)
	if err != nil {
		return nil, err
	}

	jst := time.FixedZone("Asia/Tokyo", 9*60*60)
	now := time.Now().In(jst)
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, jst)
	start := today.AddDate(0, 0, -(days - 1))

	out := make([]model.SourceDailyStats, 0, len(sources))
	for _, src := range sources {
		countsByDay := seriesBySource[src.ID]
		stats := model.SourceDailyStats{SourceID: src.ID}
		if countsByDay == nil {
			countsByDay = map[string]int{}
		}
		stats.DailyCounts = make([]model.SourceDailyCount, 0, days)
		for i := 0; i < days; i++ {
			day := start.AddDate(0, 0, i)
			key := day.Format("2006-01-02")
			count := countsByDay[key]
			stats.DailyCounts = append(stats.DailyCounts, model.SourceDailyCount{
				Day:   key,
				Count: count,
			})
			if i == days-1 {
				stats.TodayCount = count
			}
			if i == days-2 {
				stats.YesterdayCount = count
			}
			if count > 0 {
				stats.ActiveDays30d++
			}
			stats.Last30DaysTotal += count
			if i >= days-7 {
				stats.Last7DaysTotal += count
			}
		}
		if stats.ActiveDays30d > 0 {
			stats.AvgItemsPerActiveDay30 = float64(stats.Last30DaysTotal) / float64(stats.ActiveDays30d)
		}
		out = append(out, stats)
	}

	sort.Slice(out, func(i, j int) bool {
		if out[i].Last30DaysTotal == out[j].Last30DaysTotal {
			return out[i].SourceID < out[j].SourceID
		}
		return out[i].Last30DaysTotal > out[j].Last30DaysTotal
	})
	return out, nil
}

func (r *SourceRepo) NavigatorCandidates30d(ctx context.Context, userID string) ([]model.SourceNavigatorCandidate, error) {
	sources, err := r.List(ctx, userID)
	if err != nil {
		return nil, err
	}
	healthRows, err := r.HealthByUser(ctx, userID)
	if err != nil {
		return nil, err
	}
	itemStatsRows, err := r.ItemStatsByUser(ctx, userID)
	if err != nil {
		return nil, err
	}
	dailyStatsRows, err := r.DailyStatsByUser(ctx, userID, 30)
	if err != nil {
		return nil, err
	}

	favoriteCounts := map[string]int{}
	favRows, err := r.db.Query(ctx, `
		SELECT s.id AS source_id, COUNT(*)::int AS favorite_count
		FROM sources s
		LEFT JOIN items i ON i.source_id = s.id AND i.deleted_at IS NULL
		LEFT JOIN item_feedbacks fb ON fb.item_id = i.id AND fb.user_id = s.user_id
		WHERE s.user_id = $1
		  AND fb.is_favorite = true
		  AND i.created_at >= NOW() - INTERVAL '30 days'
		GROUP BY s.id`, userID)
	if err != nil {
		return nil, err
	}
	defer favRows.Close()
	for favRows.Next() {
		var sourceID string
		var count int
		if err := favRows.Scan(&sourceID, &count); err != nil {
			return nil, err
		}
		favoriteCounts[sourceID] = count
	}
	if err := favRows.Err(); err != nil {
		return nil, err
	}

	healthBySourceID := map[string]model.SourceHealth{}
	for _, row := range healthRows {
		healthBySourceID[row.SourceID] = row
	}
	itemStatsBySourceID := map[string]model.SourceItemStats{}
	for _, row := range itemStatsRows {
		itemStatsBySourceID[row.SourceID] = row
	}
	dailyStatsBySourceID := map[string]model.SourceDailyStats{}
	for _, row := range dailyStatsRows {
		dailyStatsBySourceID[row.SourceID] = row
	}

	out := make([]model.SourceNavigatorCandidate, 0, len(sources))
	for _, source := range sources {
		title := strings.TrimSpace(source.URL)
		if source.Title != nil && strings.TrimSpace(*source.Title) != "" {
			title = strings.TrimSpace(*source.Title)
		}
		health := healthBySourceID[source.ID]
		itemStat := itemStatsBySourceID[source.ID]
		dailyStat := dailyStatsBySourceID[source.ID]
		out = append(out, model.SourceNavigatorCandidate{
			SourceID:               source.ID,
			Title:                  title,
			URL:                    source.URL,
			Enabled:                source.Enabled,
			Status:                 health.Status,
			LastFetchedAt:          source.LastFetchedAt,
			LastItemAt:             health.LastItemAt,
			TotalItems30d:          dailyStat.Last30DaysTotal,
			UnreadItems30d:         itemStat.UnreadItems,
			ReadItems30d:           itemStat.ReadItems,
			FavoriteCount30d:       favoriteCounts[source.ID],
			AvgItemsPerDay30d:      itemStat.AvgItemsPerDay30Days,
			ActiveDays30d:          dailyStat.ActiveDays30d,
			AvgItemsPerActiveDay30: dailyStat.AvgItemsPerActiveDay30,
			FailureRate:            health.FailureRate,
		})
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].FavoriteCount30d != out[j].FavoriteCount30d {
			return out[i].FavoriteCount30d > out[j].FavoriteCount30d
		}
		if out[i].ReadItems30d != out[j].ReadItems30d {
			return out[i].ReadItems30d > out[j].ReadItems30d
		}
		return out[i].Title < out[j].Title
	})
	return out, nil
}

func BuildSourcesDailyOverview(rows []model.SourceDailyStats) model.SourcesDailyOverview {
	byDay := map[string]int{}
	for _, row := range rows {
		for _, daily := range row.DailyCounts {
			byDay[daily.Day] += daily.Count
		}
	}

	ordered := make([]string, 0, len(byDay))
	for day := range byDay {
		ordered = append(ordered, day)
	}
	sort.Strings(ordered)

	out := model.SourcesDailyOverview{
		DailyCounts: make([]model.SourceDailyCount, 0, len(ordered)),
	}
	for idx, day := range ordered {
		count := byDay[day]
		out.DailyCounts = append(out.DailyCounts, model.SourceDailyCount{Day: day, Count: count})
		out.Last30DaysTotal += count
		if idx >= len(ordered)-7 {
			out.Last7DaysTotal += count
		}
		if idx == len(ordered)-1 {
			out.TodayCount = count
		}
		if idx == len(ordered)-2 {
			out.YesterdayCount = count
		}
		if count > 0 {
			out.ActiveDays30d++
		}
	}
	if out.ActiveDays30d > 0 {
		out.AvgItemsPerActiveDay30 = float64(out.Last30DaysTotal) / float64(out.ActiveDays30d)
	}
	return out
}

func (r *SourceRepo) RefreshHealthSnapshot(ctx context.Context, sourceID string, reason *string) error {
	var (
		h       model.SourceHealth
		enabled bool
	)
	err := r.db.QueryRow(ctx, `
		SELECT
			s.id AS source_id,
			s.enabled AS enabled,
			s.last_fetched_at,
			COUNT(i.id)::int AS total_items,
			COUNT(*) FILTER (WHERE i.status = 'failed')::int AS failed_items,
			COUNT(*) FILTER (WHERE i.status = 'summarized')::int AS summarized_items,
			MAX(i.created_at) AS last_item_at
		FROM sources s
		LEFT JOIN items i ON i.source_id = s.id AND i.deleted_at IS NULL
		WHERE s.id = $1
		GROUP BY s.id, s.enabled, s.last_fetched_at`, sourceID,
	).Scan(
		&h.SourceID,
		&enabled,
		&h.LastFetchedAt,
		&h.TotalItems,
		&h.FailedItems,
		&h.Summarized,
		&h.LastItemAt,
	)
	if err != nil {
		return mapDBError(err)
	}
	if h.TotalItems > 0 {
		h.FailureRate = float64(h.FailedItems) / float64(h.TotalItems)
	}
	h.Status = deriveSourceHealthStatus(enabled, h.TotalItems, h.FailedItems, h.FailureRate, h.LastFetchedAt)
	if reason != nil && *reason != "" {
		h.Status = "error"
	}
	if _, err := r.db.Exec(ctx, `
		INSERT INTO source_health_snapshots (
			source_id, total_items, failed_items, summarized_items, failure_rate,
			last_item_at, last_fetched_at, status, reason, checked_at, updated_at
		) VALUES (
			$1, $2, $3, $4, $5,
			$6, $7, $8, $9, NOW(), NOW()
		)
		ON CONFLICT (source_id) DO UPDATE SET
			total_items = EXCLUDED.total_items,
			failed_items = EXCLUDED.failed_items,
			summarized_items = EXCLUDED.summarized_items,
			failure_rate = EXCLUDED.failure_rate,
			last_item_at = EXCLUDED.last_item_at,
			last_fetched_at = EXCLUDED.last_fetched_at,
			status = EXCLUDED.status,
			reason = EXCLUDED.reason,
			checked_at = NOW(),
			updated_at = NOW()`,
		h.SourceID, h.TotalItems, h.FailedItems, h.Summarized, h.FailureRate,
		h.LastItemAt, h.LastFetchedAt, h.Status, reason,
	); err != nil {
		return fmt.Errorf("upsert source health snapshot: %w", err)
	}
	return nil
}

func (r *SourceRepo) RecommendedByUser(ctx context.Context, userID string, limit int) ([]model.RecommendedSource, error) {
	return r.affinityByUser(ctx, userID, limit, false)
}

func (r *SourceRepo) LowAffinityByUser(ctx context.Context, userID string, limit int) ([]model.RecommendedSource, error) {
	return r.affinityByUser(ctx, userID, limit, true)
}

func (r *SourceRepo) affinityByUser(ctx context.Context, userID string, limit int, asc bool) ([]model.RecommendedSource, error) {
	if limit <= 0 {
		limit = 8
	}
	if limit > 30 {
		limit = 30
	}
	order := "DESC"
	if asc {
		order = "ASC"
	}
	rows, err := r.db.Query(ctx, `
		WITH base AS (
			SELECT
				s.id AS source_id,
				s.url,
				s.title,
				COUNT(i.id)::int AS item_count_30d,
				COUNT(ir.item_id)::int AS read_count_30d,
				COUNT(fb.item_id)::int AS feedback_count_30d,
				COUNT(*) FILTER (WHERE fb.is_favorite = true)::int AS favorite_count_30d,
				COALESCE(SUM(
					CASE
						WHEN fb.is_favorite = true THEN 2.0
						WHEN fb.rating > 0 THEN 1.0
						WHEN fb.rating < 0 THEN -1.0
						ELSE 0.0
					END
				), 0)::double precision AS feedback_signal,
				MAX(COALESCE(i.published_at, i.created_at)) AS last_item_at
			FROM sources s
			LEFT JOIN items i
			       ON i.source_id = s.id
			      AND i.deleted_at IS NULL
			      AND COALESCE(i.published_at, i.created_at) >= NOW() - INTERVAL '30 days'
			LEFT JOIN item_reads ir
			       ON ir.item_id = i.id
			      AND ir.user_id = $1
			LEFT JOIN item_feedbacks fb
			       ON fb.item_id = i.id
			      AND fb.user_id = $1
			WHERE s.user_id = $1
			  AND s.enabled = true
			GROUP BY s.id, s.url, s.title
		)
		SELECT
			source_id,
			url,
			title,
			(
				feedback_signal * 0.7
				+ CASE WHEN item_count_30d > 0 THEN (read_count_30d::double precision / item_count_30d::double precision) * 1.8 ELSE 0 END
				+ CASE
					WHEN last_item_at >= NOW() - INTERVAL '24 hours' THEN 0.35
					WHEN last_item_at >= NOW() - INTERVAL '72 hours' THEN 0.15
					ELSE 0
				  END
			)::double precision AS affinity_score,
			read_count_30d,
			feedback_count_30d,
			favorite_count_30d,
			last_item_at
		FROM base
		WHERE item_count_30d > 0
		ORDER BY affinity_score `+order+`, favorite_count_30d `+order+`, read_count_30d `+order+`
		LIMIT $2`, userID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]model.RecommendedSource, 0, limit)
	for rows.Next() {
		var v model.RecommendedSource
		if err := rows.Scan(
			&v.SourceID,
			&v.URL,
			&v.Title,
			&v.AffinityScore,
			&v.ReadCount30d,
			&v.Feedback30d,
			&v.FavoriteCount30d,
			&v.LastItemAt,
		); err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, rows.Err()
}

func deriveSourceHealthStatus(enabled bool, totalItems, failedItems int, failureRate float64, lastFetchedAt *time.Time) string {
	switch {
	case !enabled:
		return "disabled"
	case totalItems == 0:
		return "new"
	case failedItems >= 3 && failureRate >= 0.5:
		return "error"
	case lastFetchedAt == nil || time.Since(*lastFetchedAt) > 72*time.Hour:
		return "stale"
	default:
		return "ok"
	}
}
