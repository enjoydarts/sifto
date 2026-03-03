package repository

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

type PushNotificationLogRepo struct {
	db *pgxpool.Pool
}

func NewPushNotificationLogRepo(db *pgxpool.Pool) *PushNotificationLogRepo {
	return &PushNotificationLogRepo{db: db}
}

func (r *PushNotificationLogRepo) ExistsByUserKindItem(ctx context.Context, userID, kind, itemID string) (bool, error) {
	var exists bool
	err := r.db.QueryRow(ctx, `
		SELECT EXISTS(
			SELECT 1 FROM push_notification_logs
			WHERE user_id = $1 AND kind = $2 AND item_id = $3
		)
	`, userID, kind, itemID).Scan(&exists)
	return exists, err
}

func (r *PushNotificationLogRepo) CountByUserKindDay(ctx context.Context, userID, kind string, dayJST time.Time) (int, error) {
	var n int
	err := r.db.QueryRow(ctx, `
		SELECT COUNT(*)::int
		FROM push_notification_logs
		WHERE user_id = $1 AND kind = $2 AND day_jst = $3::date
	`, userID, kind, dayJST.Format("2006-01-02")).Scan(&n)
	return n, err
}

type PushNotificationLogInput struct {
	UserID                  string
	Kind                    string
	ItemID                  *string
	DayJST                  time.Time
	Title                   string
	Message                 string
	OneSignalNotificationID *string
	Recipients              int
}

func (r *PushNotificationLogRepo) Insert(ctx context.Context, in PushNotificationLogInput) error {
	_, err := r.db.Exec(ctx, `
		INSERT INTO push_notification_logs (
			user_id, kind, item_id, day_jst, title, message, onesignal_notification_id, recipients
		) VALUES ($1, $2, $3, $4::date, $5, $6, $7, $8)
	`, in.UserID, in.Kind, in.ItemID, in.DayJST.Format("2006-01-02"), in.Title, in.Message, in.OneSignalNotificationID, in.Recipients)
	return err
}
