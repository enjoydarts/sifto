package repository

import (
	"context"

	"github.com/enjoydarts/sifto/api/internal/model"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type ObsidianExportRepo struct{ db *pgxpool.Pool }

func NewObsidianExportRepo(db *pgxpool.Pool) *ObsidianExportRepo { return &ObsidianExportRepo{db: db} }

func (r *ObsidianExportRepo) GetByUserID(ctx context.Context, userID string) (*model.ObsidianExportSettings, error) {
	var v model.ObsidianExportSettings
	err := r.db.QueryRow(ctx, `
		SELECT user_id, enabled, github_installation_id, github_repo_owner, github_repo_name,
		       github_repo_branch, vault_root_path, keyword_link_mode, last_run_at, last_success_at,
		       created_at, updated_at
		FROM user_obsidian_exports
		WHERE user_id = $1`, userID,
	).Scan(
		&v.UserID,
		&v.Enabled,
		&v.GitHubInstallationID,
		&v.GitHubRepoOwner,
		&v.GitHubRepoName,
		&v.GitHubRepoBranch,
		&v.VaultRootPath,
		&v.KeywordLinkMode,
		&v.LastRunAt,
		&v.LastSuccessAt,
		&v.CreatedAt,
		&v.UpdatedAt,
	)
	if err != nil {
		return nil, mapDBError(err)
	}
	return &v, nil
}

func (r *ObsidianExportRepo) EnsureDefaults(ctx context.Context, userID string) (*model.ObsidianExportSettings, error) {
	_, err := r.db.Exec(ctx, `
		INSERT INTO user_obsidian_exports (user_id)
		VALUES ($1)
		ON CONFLICT (user_id) DO NOTHING`, userID)
	if err != nil {
		return nil, err
	}
	return r.GetByUserID(ctx, userID)
}

func (r *ObsidianExportRepo) UpsertConfig(
	ctx context.Context,
	userID string,
	enabled bool,
	repoOwner, repoName, repoBranch, vaultRootPath, keywordLinkMode *string,
) (*model.ObsidianExportSettings, error) {
	_, err := r.db.Exec(ctx, `
		INSERT INTO user_obsidian_exports (
			user_id, enabled, github_repo_owner, github_repo_name, github_repo_branch, vault_root_path, keyword_link_mode
		) VALUES ($1, $2, $3, $4, COALESCE($5, 'main'), $6, COALESCE($7, 'topics_only'))
		ON CONFLICT (user_id) DO UPDATE
		SET enabled = EXCLUDED.enabled,
		    github_repo_owner = EXCLUDED.github_repo_owner,
		    github_repo_name = EXCLUDED.github_repo_name,
		    github_repo_branch = COALESCE(EXCLUDED.github_repo_branch, user_obsidian_exports.github_repo_branch),
		    vault_root_path = EXCLUDED.vault_root_path,
		    keyword_link_mode = COALESCE(EXCLUDED.keyword_link_mode, user_obsidian_exports.keyword_link_mode),
		    updated_at = NOW()`,
		userID, enabled, repoOwner, repoName, repoBranch, vaultRootPath, keywordLinkMode,
	)
	if err != nil {
		return nil, err
	}
	return r.GetByUserID(ctx, userID)
}

func (r *ObsidianExportRepo) UpsertInstallation(ctx context.Context, userID string, installationID int64, repoOwner *string) (*model.ObsidianExportSettings, error) {
	_, err := r.db.Exec(ctx, `
		INSERT INTO user_obsidian_exports (user_id, github_installation_id, github_repo_owner)
		VALUES ($1, $2, $3)
		ON CONFLICT (user_id) DO UPDATE
		SET github_installation_id = EXCLUDED.github_installation_id,
		    github_repo_owner = COALESCE(EXCLUDED.github_repo_owner, user_obsidian_exports.github_repo_owner),
		    updated_at = NOW()`,
		userID, installationID, repoOwner,
	)
	if err != nil {
		return nil, err
	}
	return r.GetByUserID(ctx, userID)
}

func (r *ObsidianExportRepo) ListEnabled(ctx context.Context) ([]model.ObsidianExportSettings, error) {
	rows, err := r.db.Query(ctx, `
		SELECT user_id, enabled, github_installation_id, github_repo_owner, github_repo_name,
		       github_repo_branch, vault_root_path, keyword_link_mode, last_run_at, last_success_at,
		       created_at, updated_at
		FROM user_obsidian_exports
		WHERE enabled = true
		  AND github_installation_id IS NOT NULL
		  AND github_repo_owner IS NOT NULL
		  AND github_repo_name IS NOT NULL
		  AND vault_root_path IS NOT NULL
		ORDER BY updated_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []model.ObsidianExportSettings{}
	for rows.Next() {
		var v model.ObsidianExportSettings
		if err := rows.Scan(
			&v.UserID,
			&v.Enabled,
			&v.GitHubInstallationID,
			&v.GitHubRepoOwner,
			&v.GitHubRepoName,
			&v.GitHubRepoBranch,
			&v.VaultRootPath,
			&v.KeywordLinkMode,
			&v.LastRunAt,
			&v.LastSuccessAt,
			&v.CreatedAt,
			&v.UpdatedAt,
		); err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, rows.Err()
}

func (r *ObsidianExportRepo) MarkRun(ctx context.Context, userID string, success bool) error {
	if success {
		_, err := r.db.Exec(ctx, `
			UPDATE user_obsidian_exports
			SET last_run_at = NOW(),
			    last_success_at = NOW(),
			    updated_at = NOW()
			WHERE user_id = $1`, userID)
		return err
	}
	_, err := r.db.Exec(ctx, `
		UPDATE user_obsidian_exports
		SET last_run_at = NOW(),
		    updated_at = NOW()
		WHERE user_id = $1`, userID)
	return err
}

type ItemExportRepo struct{ db *pgxpool.Pool }

func NewItemExportRepo(db *pgxpool.Pool) *ItemExportRepo { return &ItemExportRepo{db: db} }

func (r *ItemExportRepo) GetByUserTargetItemIDs(ctx context.Context, userID, target string, itemIDs []string) (map[string]model.ItemExportRecord, error) {
	if len(itemIDs) == 0 {
		return map[string]model.ItemExportRecord{}, nil
	}
	rows, err := r.db.Query(ctx, `
		SELECT id, user_id, item_id, target, github_path, github_sha, content_hash, status, exported_at, last_error, created_at, updated_at
		FROM item_exports
		WHERE user_id = $1
		  AND target = $2
		  AND item_id = ANY($3::uuid[])`, userID, target, itemIDs)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]model.ItemExportRecord{}
	for rows.Next() {
		var v model.ItemExportRecord
		if err := rows.Scan(&v.ID, &v.UserID, &v.ItemID, &v.Target, &v.GitHubPath, &v.GitHubSHA, &v.ContentHash, &v.Status, &v.ExportedAt, &v.LastError, &v.CreatedAt, &v.UpdatedAt); err != nil {
			return nil, err
		}
		out[v.ItemID] = v
	}
	return out, rows.Err()
}

func (r *ItemExportRepo) UpsertSuccess(ctx context.Context, userID, itemID, target, githubPath, githubSHA, contentHash string) error {
	_, err := r.db.Exec(ctx, `
		INSERT INTO item_exports (user_id, item_id, target, github_path, github_sha, content_hash, status, exported_at)
		VALUES ($1, $2, $3, $4, $5, $6, 'success', NOW())
		ON CONFLICT (user_id, item_id, target) DO UPDATE
		SET github_path = EXCLUDED.github_path,
		    github_sha = EXCLUDED.github_sha,
		    content_hash = EXCLUDED.content_hash,
		    status = 'success',
		    exported_at = NOW(),
		    last_error = NULL,
		    updated_at = NOW()`, userID, itemID, target, githubPath, githubSHA, contentHash)
	return err
}

func (r *ItemExportRepo) UpsertFailure(ctx context.Context, userID, itemID, target, errText string) error {
	_, err := r.db.Exec(ctx, `
		INSERT INTO item_exports (user_id, item_id, target, status, last_error)
		VALUES ($1, $2, $3, 'failed', $4)
		ON CONFLICT (user_id, item_id, target) DO UPDATE
		SET status = 'failed',
		    last_error = EXCLUDED.last_error,
		    updated_at = NOW()`, userID, itemID, target, errText)
	return err
}

func (r *ObsidianExportRepo) GetByUserIDNullable(ctx context.Context, userID string) (*model.ObsidianExportSettings, error) {
	v, err := r.GetByUserID(ctx, userID)
	if err != nil {
		if err == ErrNotFound || err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	return v, nil
}
