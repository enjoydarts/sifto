package repository

import (
	"context"
	"encoding/json"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

type PromptTemplateRepo struct{ db *pgxpool.Pool }

func NewPromptTemplateRepo(db *pgxpool.Pool) *PromptTemplateRepo { return &PromptTemplateRepo{db: db} }

type PromptTemplate struct {
	ID              string    `json:"id"`
	Key             string    `json:"key"`
	Purpose         string    `json:"purpose"`
	Description     string    `json:"description"`
	Status          string    `json:"status"`
	ActiveVersionID *string   `json:"active_version_id,omitempty"`
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
}

type PromptTemplateVersion struct {
	ID                 string          `json:"id"`
	TemplateID         string          `json:"template_id"`
	Version            int             `json:"version"`
	Label              string          `json:"label"`
	SystemInstruction  string          `json:"system_instruction"`
	PromptText         string          `json:"prompt_text"`
	FallbackPromptText string          `json:"fallback_prompt_text"`
	VariablesSchema    json.RawMessage `json:"variables_schema,omitempty"`
	Notes              string          `json:"notes"`
	CreatedByUserID    *string         `json:"created_by_user_id,omitempty"`
	CreatedByEmail     string          `json:"created_by_email"`
	CreatedAt          time.Time       `json:"created_at"`
}

type PromptExperiment struct {
	ID              string     `json:"id"`
	TemplateID      string     `json:"template_id"`
	Name            string     `json:"name"`
	Status          string     `json:"status"`
	AssignmentUnit  string     `json:"assignment_unit"`
	StartedAt       *time.Time `json:"started_at,omitempty"`
	EndedAt         *time.Time `json:"ended_at,omitempty"`
	CreatedByUserID *string    `json:"created_by_user_id,omitempty"`
	CreatedByEmail  string     `json:"created_by_email"`
	CreatedAt       time.Time  `json:"created_at"`
	UpdatedAt       time.Time  `json:"updated_at"`
}

type PromptExperimentArm struct {
	ID           string    `json:"id"`
	ExperimentID string    `json:"experiment_id"`
	VersionID    string    `json:"version_id"`
	Weight       int       `json:"weight"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

type PromptTemplateDetail struct {
	Template    PromptTemplate          `json:"template"`
	Versions    []PromptTemplateVersion `json:"versions"`
	Experiments []PromptExperiment      `json:"experiments"`
	Arms        []PromptExperimentArm   `json:"arms"`
}

type PromptActiveVersion struct {
	TemplateID         string
	TemplateKey        string
	Purpose            string
	VersionID          string
	VersionNumber      int
	SystemInstruction  string
	PromptText         string
	FallbackPromptText string
}

type PromptTemplateUpsertInput struct {
	Key         string
	Purpose     string
	Description string
	Status      string
}

type PromptTemplateVersionInput struct {
	TemplateID         string
	Label              string
	SystemInstruction  string
	PromptText         string
	FallbackPromptText string
	VariablesSchema    json.RawMessage
	Notes              string
	CreatedByUserID    *string
	CreatedByEmail     string
}

type PromptExperimentInput struct {
	TemplateID      string
	Name            string
	Status          string
	AssignmentUnit  string
	StartedAt       *time.Time
	EndedAt         *time.Time
	CreatedByUserID *string
	CreatedByEmail  string
}

type PromptExperimentArmInput struct {
	VersionID string `json:"version_id"`
	Weight    int    `json:"weight"`
}

type PromptAdminAuditLogInput struct {
	UserID       *string
	UserEmail    string
	Action       string
	TemplateID   *string
	VersionID    *string
	ExperimentID *string
	Metadata     json.RawMessage
}

func (r *PromptTemplateRepo) UpsertTemplate(ctx context.Context, in PromptTemplateUpsertInput) (*PromptTemplate, error) {
	var out PromptTemplate
	err := r.db.QueryRow(ctx, `
		INSERT INTO prompt_templates (key, purpose, description, status)
		VALUES ($1, $2, $3, COALESCE(NULLIF($4, ''), 'active'))
		ON CONFLICT (key) DO UPDATE
		SET purpose = EXCLUDED.purpose,
		    description = EXCLUDED.description,
		    status = EXCLUDED.status,
		    updated_at = NOW()
		RETURNING id, key, purpose, description, status, active_version_id, created_at, updated_at
	`, in.Key, in.Purpose, in.Description, in.Status).Scan(
		&out.ID, &out.Key, &out.Purpose, &out.Description, &out.Status, &out.ActiveVersionID, &out.CreatedAt, &out.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	return &out, nil
}

func (r *PromptTemplateRepo) ListTemplates(ctx context.Context) ([]PromptTemplate, error) {
	rows, err := r.db.Query(ctx, `
		SELECT id, key, purpose, description, status, active_version_id, created_at, updated_at
		FROM prompt_templates
		ORDER BY purpose ASC, key ASC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]PromptTemplate, 0)
	for rows.Next() {
		var v PromptTemplate
		if err := rows.Scan(&v.ID, &v.Key, &v.Purpose, &v.Description, &v.Status, &v.ActiveVersionID, &v.CreatedAt, &v.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, rows.Err()
}

func (r *PromptTemplateRepo) GetTemplateDetail(ctx context.Context, templateID string) (*PromptTemplateDetail, error) {
	detail := PromptTemplateDetail{
		Versions:    make([]PromptTemplateVersion, 0),
		Experiments: make([]PromptExperiment, 0),
		Arms:        make([]PromptExperimentArm, 0),
	}
	err := r.db.QueryRow(ctx, `
		SELECT id, key, purpose, description, status, active_version_id, created_at, updated_at
		FROM prompt_templates
		WHERE id = $1
	`, templateID).Scan(
		&detail.Template.ID, &detail.Template.Key, &detail.Template.Purpose, &detail.Template.Description, &detail.Template.Status,
		&detail.Template.ActiveVersionID, &detail.Template.CreatedAt, &detail.Template.UpdatedAt,
	)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	versionRows, err := r.db.Query(ctx, `
		SELECT id, template_id, version, label, system_instruction, prompt_text, fallback_prompt_text, variables_schema, notes, created_by_user_id, created_by_email, created_at
		FROM prompt_template_versions
		WHERE template_id = $1
		ORDER BY version DESC
	`, templateID)
	if err != nil {
		return nil, err
	}
	defer versionRows.Close()
	for versionRows.Next() {
		var v PromptTemplateVersion
		if err := versionRows.Scan(&v.ID, &v.TemplateID, &v.Version, &v.Label, &v.SystemInstruction, &v.PromptText, &v.FallbackPromptText, &v.VariablesSchema, &v.Notes, &v.CreatedByUserID, &v.CreatedByEmail, &v.CreatedAt); err != nil {
			return nil, err
		}
		detail.Versions = append(detail.Versions, v)
	}
	if err := versionRows.Err(); err != nil {
		return nil, err
	}
	experimentRows, err := r.db.Query(ctx, `
		SELECT id, template_id, name, status, assignment_unit, started_at, ended_at, created_by_user_id, created_by_email, created_at, updated_at
		FROM prompt_experiments
		WHERE template_id = $1
		ORDER BY created_at DESC
	`, templateID)
	if err != nil {
		return nil, err
	}
	defer experimentRows.Close()
	experimentIDs := make([]string, 0)
	for experimentRows.Next() {
		var e PromptExperiment
		if err := experimentRows.Scan(&e.ID, &e.TemplateID, &e.Name, &e.Status, &e.AssignmentUnit, &e.StartedAt, &e.EndedAt, &e.CreatedByUserID, &e.CreatedByEmail, &e.CreatedAt, &e.UpdatedAt); err != nil {
			return nil, err
		}
		detail.Experiments = append(detail.Experiments, e)
		experimentIDs = append(experimentIDs, e.ID)
	}
	if err := experimentRows.Err(); err != nil {
		return nil, err
	}
	if len(experimentIDs) == 0 {
		return &detail, nil
	}
	armRows, err := r.db.Query(ctx, `
		SELECT a.id, a.experiment_id, a.version_id, a.weight, a.created_at, a.updated_at
		FROM prompt_experiment_arms a
		INNER JOIN prompt_experiments e ON e.id = a.experiment_id
		WHERE e.template_id = $1
		ORDER BY a.created_at ASC
	`, templateID)
	if err != nil {
		return nil, err
	}
	defer armRows.Close()
	for armRows.Next() {
		var a PromptExperimentArm
		if err := armRows.Scan(&a.ID, &a.ExperimentID, &a.VersionID, &a.Weight, &a.CreatedAt, &a.UpdatedAt); err != nil {
			return nil, err
		}
		detail.Arms = append(detail.Arms, a)
	}
	return &detail, armRows.Err()
}

func (r *PromptTemplateRepo) CreateVersion(ctx context.Context, in PromptTemplateVersionInput) (*PromptTemplateVersion, error) {
	if len(in.VariablesSchema) == 0 {
		in.VariablesSchema = json.RawMessage(`{}`)
	}
	var out PromptTemplateVersion
	err := r.db.QueryRow(ctx, `
		WITH template_lock AS (
			SELECT id
			FROM prompt_templates
			WHERE id = $1
			FOR UPDATE
		), next_version AS (
			SELECT COALESCE(MAX(v.version), 0) + 1 AS version
			FROM prompt_template_versions v
			WHERE v.template_id = $1
		)
		INSERT INTO prompt_template_versions (
			template_id, version, label, system_instruction, prompt_text, fallback_prompt_text,
			variables_schema, notes, created_by_user_id, created_by_email
		)
		SELECT $1, next_version.version, $2, $3, $4, $5, $6, $7, $8, $9
		FROM template_lock, next_version
		RETURNING id, template_id, version, label, system_instruction, prompt_text, fallback_prompt_text, variables_schema, notes, created_by_user_id, created_by_email, created_at
	`, in.TemplateID, in.Label, in.SystemInstruction, in.PromptText, in.FallbackPromptText, in.VariablesSchema, in.Notes, in.CreatedByUserID, in.CreatedByEmail).Scan(
		&out.ID, &out.TemplateID, &out.Version, &out.Label, &out.SystemInstruction, &out.PromptText, &out.FallbackPromptText,
		&out.VariablesSchema, &out.Notes, &out.CreatedByUserID, &out.CreatedByEmail, &out.CreatedAt,
	)
	if err != nil {
		if mappedErr := mapPromptTemplateVersionWriteError(err); mappedErr != nil {
			return nil, mappedErr
		}
		return nil, err
	}
	return &out, nil
}

func (r *PromptTemplateRepo) ActivateVersion(ctx context.Context, templateID, versionID string) error {
	tag, err := r.db.Exec(ctx, `
		UPDATE prompt_templates
		SET active_version_id = $2,
		    updated_at = NOW()
		WHERE id = $1
		  AND EXISTS (
		  	SELECT 1
		  	FROM prompt_template_versions v
		  	WHERE v.id = $2
		  	  AND v.template_id = $1
		  )
	`, templateID, versionID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrInvalidState
	}
	return nil
}

func (r *PromptTemplateRepo) ClearActiveVersion(ctx context.Context, templateID string) error {
	_, err := r.db.Exec(ctx, `
		UPDATE prompt_templates
		SET active_version_id = NULL,
		    updated_at = NOW()
		WHERE id = $1
	`, templateID)
	return err
}

func (r *PromptTemplateRepo) CreateExperiment(ctx context.Context, in PromptExperimentInput, arms []PromptExperimentArmInput) (*PromptExperiment, []PromptExperimentArm, error) {
	tx, err := r.db.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return nil, nil, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	var exp PromptExperiment
	err = tx.QueryRow(ctx, `
		INSERT INTO prompt_experiments (
			template_id, name, status, assignment_unit, started_at, ended_at, created_by_user_id, created_by_email
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8)
		RETURNING id, template_id, name, status, assignment_unit, started_at, ended_at, created_by_user_id, created_by_email, created_at, updated_at
	`, in.TemplateID, in.Name, in.Status, in.AssignmentUnit, in.StartedAt, in.EndedAt, in.CreatedByUserID, in.CreatedByEmail).Scan(
		&exp.ID, &exp.TemplateID, &exp.Name, &exp.Status, &exp.AssignmentUnit, &exp.StartedAt, &exp.EndedAt,
		&exp.CreatedByUserID, &exp.CreatedByEmail, &exp.CreatedAt, &exp.UpdatedAt,
	)
	if err != nil {
		return nil, nil, err
	}
	outArms := make([]PromptExperimentArm, 0, len(arms))
	for _, arm := range arms {
		var created PromptExperimentArm
		if err := tx.QueryRow(ctx, `
			INSERT INTO prompt_experiment_arms (experiment_id, version_id, weight)
			VALUES ($1,$2,$3)
			RETURNING id, experiment_id, version_id, weight, created_at, updated_at
		`, exp.ID, arm.VersionID, arm.Weight).Scan(
			&created.ID, &created.ExperimentID, &created.VersionID, &created.Weight, &created.CreatedAt, &created.UpdatedAt,
		); err != nil {
			return nil, nil, err
		}
		outArms = append(outArms, created)
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, nil, err
	}
	return &exp, outArms, nil
}

func (r *PromptTemplateRepo) UpdateExperiment(ctx context.Context, experimentID, status string, startedAt, endedAt *time.Time, arms []PromptExperimentArmInput) (*PromptExperiment, []PromptExperimentArm, error) {
	tx, err := r.db.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return nil, nil, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	var exp PromptExperiment
	err = tx.QueryRow(ctx, `
		UPDATE prompt_experiments
		SET status = COALESCE(NULLIF($2, ''), status),
		    started_at = $3,
		    ended_at = $4,
		    updated_at = NOW()
		WHERE id = $1
		RETURNING id, template_id, name, status, assignment_unit, started_at, ended_at, created_by_user_id, created_by_email, created_at, updated_at
	`, experimentID, status, startedAt, endedAt).Scan(
		&exp.ID, &exp.TemplateID, &exp.Name, &exp.Status, &exp.AssignmentUnit, &exp.StartedAt, &exp.EndedAt,
		&exp.CreatedByUserID, &exp.CreatedByEmail, &exp.CreatedAt, &exp.UpdatedAt,
	)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil, nil
		}
		return nil, nil, err
	}
	if shouldReplacePromptExperimentArms(arms) {
		if _, err := tx.Exec(ctx, `DELETE FROM prompt_experiment_arms WHERE experiment_id = $1`, experimentID); err != nil {
			return nil, nil, err
		}
	}
	outArms := make([]PromptExperimentArm, 0, len(arms))
	for _, arm := range arms {
		var created PromptExperimentArm
		if err := tx.QueryRow(ctx, `
			INSERT INTO prompt_experiment_arms (experiment_id, version_id, weight)
			VALUES ($1,$2,$3)
			RETURNING id, experiment_id, version_id, weight, created_at, updated_at
		`, experimentID, arm.VersionID, arm.Weight).Scan(
			&created.ID, &created.ExperimentID, &created.VersionID, &created.Weight, &created.CreatedAt, &created.UpdatedAt,
		); err != nil {
			return nil, nil, err
		}
		outArms = append(outArms, created)
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, nil, err
	}
	return &exp, outArms, nil
}

func (r *PromptTemplateRepo) InsertAuditLog(ctx context.Context, in PromptAdminAuditLogInput) error {
	metadata := in.Metadata
	if len(metadata) == 0 {
		metadata = json.RawMessage(`{}`)
	}
	_, err := r.db.Exec(ctx, `
		INSERT INTO prompt_admin_audit_logs (user_id, user_email, action, template_id, version_id, experiment_id, metadata)
		VALUES ($1,$2,$3,$4,$5,$6,$7)
	`, in.UserID, in.UserEmail, in.Action, in.TemplateID, in.VersionID, in.ExperimentID, metadata)
	return err
}

func (r *PromptTemplateRepo) GetActiveVersionByKey(ctx context.Context, key string) (*PromptActiveVersion, error) {
	var out PromptActiveVersion
	err := r.db.QueryRow(ctx, `
		SELECT t.id, t.key, t.purpose, v.id, v.version, v.system_instruction, v.prompt_text, v.fallback_prompt_text
		FROM prompt_templates t
		INNER JOIN prompt_template_versions v ON v.id = t.active_version_id
		WHERE t.key = $1
	`, key).Scan(
		&out.TemplateID, &out.TemplateKey, &out.Purpose, &out.VersionID, &out.VersionNumber, &out.SystemInstruction, &out.PromptText, &out.FallbackPromptText,
	)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	return &out, nil
}

func mapPromptTemplateVersionWriteError(err error) error {
	if err == nil {
		return nil
	}
	if err == pgx.ErrNoRows {
		return ErrInvalidState
	}
	if pgErr, ok := err.(*pgconn.PgError); ok && pgErr.Code == "23505" {
		return ErrConflict
	}
	return nil
}

func shouldReplacePromptExperimentArms(arms []PromptExperimentArmInput) bool {
	return arms != nil
}
