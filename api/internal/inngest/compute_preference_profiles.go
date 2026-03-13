package inngest

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/enjoydarts/sifto/api/internal/repository"
	"github.com/inngest/inngestgo"
	"github.com/jackc/pgx/v5/pgxpool"
)

func computePreferenceProfilesFn(client inngestgo.Client, db *pgxpool.Pool) (inngestgo.ServableFunction, error) {
	prefRepo := repository.NewPreferenceProfileRepo(db)

	return inngestgo.CreateFunction(
		client,
		inngestgo.FunctionOpts{ID: "compute-preference-profiles", Name: "Compute Preference Profiles"},
		inngestgo.CronTrigger("0 20 * * *"),
		func(ctx context.Context, input inngestgo.Input[any]) (any, error) {
			userIDs, err := listRecentlyActiveUserIDs(ctx, db)
			if err != nil {
				return nil, fmt.Errorf("list active users: %w", err)
			}
			if len(userIDs) == 0 {
				slog.Info("compute-preference-profiles: no active users found")
				return map[string]any{"updated": 0, "failed": 0}, nil
			}

			updated := 0
			failed := 0
			for _, uid := range userIDs {
				profile, err := prefRepo.BuildProfileForUser(ctx, uid)
				if err != nil {
					slog.Error("compute-preference-profiles: build profile failed", "user_id", uid, "error", err)
					failed++
					continue
				}
				if err := prefRepo.UpsertProfile(ctx, profile); err != nil {
					slog.Error("compute-preference-profiles: upsert profile failed", "user_id", uid, "error", err)
					failed++
					continue
				}
				updated++
			}

			slog.Info("compute-preference-profiles: done", "updated", updated, "failed", failed)
			return map[string]any{"updated": updated, "failed": failed}, nil
		},
	)
}

// listRecentlyActiveUserIDs returns user IDs that have read or given feedback in the last 7 days.
func listRecentlyActiveUserIDs(ctx context.Context, db *pgxpool.Pool) ([]string, error) {
	rows, err := db.Query(ctx, `
		SELECT DISTINCT user_id FROM (
			SELECT user_id FROM item_reads WHERE read_at >= NOW() - INTERVAL '7 days'
			UNION
			SELECT user_id FROM item_feedbacks WHERE updated_at >= NOW() - INTERVAL '7 days'
		) AS active_users`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}
