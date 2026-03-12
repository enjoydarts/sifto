package repository

import (
	"context"

	"github.com/enjoydarts/sifto/api/internal/model"
	"github.com/jackc/pgx/v5/pgxpool"
)

type UserIdentityRepo struct{ db *pgxpool.Pool }

func NewUserIdentityRepo(db *pgxpool.Pool) *UserIdentityRepo { return &UserIdentityRepo{db: db} }

func (r *UserIdentityRepo) GetByProviderUserID(ctx context.Context, provider, providerUserID string) (*model.UserIdentity, error) {
	var identity model.UserIdentity
	err := r.db.QueryRow(ctx, `
		SELECT id, user_id, provider, provider_user_id, email, created_at, updated_at
		FROM user_identities
		WHERE provider = $1 AND provider_user_id = $2
	`, provider, providerUserID).Scan(
		&identity.ID,
		&identity.UserID,
		&identity.Provider,
		&identity.ProviderUserID,
		&identity.Email,
		&identity.CreatedAt,
		&identity.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	return &identity, nil
}

func (r *UserIdentityRepo) Upsert(ctx context.Context, userID, provider, providerUserID string, email *string) (*model.UserIdentity, error) {
	var identity model.UserIdentity
	err := r.db.QueryRow(ctx, `
		INSERT INTO user_identities (user_id, provider, provider_user_id, email)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (provider, provider_user_id)
		DO UPDATE SET
			user_id = EXCLUDED.user_id,
			email = EXCLUDED.email,
			updated_at = NOW()
		RETURNING id, user_id, provider, provider_user_id, email, created_at, updated_at
	`, userID, provider, providerUserID, email).Scan(
		&identity.ID,
		&identity.UserID,
		&identity.Provider,
		&identity.ProviderUserID,
		&identity.Email,
		&identity.CreatedAt,
		&identity.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	return &identity, nil
}
