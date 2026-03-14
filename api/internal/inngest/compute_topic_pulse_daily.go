package inngest

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/enjoydarts/sifto/api/internal/repository"
	"github.com/inngest/inngestgo"
	"github.com/jackc/pgx/v5/pgxpool"
)

func computeTopicPulseDailyFn(client inngestgo.Client, db *pgxpool.Pool) (inngestgo.ServableFunction, error) {
	itemRepo := repository.NewItemRepo(db)

	return inngestgo.CreateFunction(
		client,
		inngestgo.FunctionOpts{ID: "compute-topic-pulse-daily", Name: "Compute Topic Pulse Daily"},
		inngestgo.CronTrigger("10 * * * *"),
		func(ctx context.Context, input inngestgo.Input[any]) (any, error) {
			if err := itemRepo.RebuildTopicPulseDaily(ctx, 35); err != nil {
				return nil, fmt.Errorf("rebuild topic pulse daily: %w", err)
			}
			slog.Info("compute-topic-pulse-daily: rebuilt recent topic pulse aggregates")
			return map[string]any{"rebuilt_days": 35}, nil
		},
	)
}
