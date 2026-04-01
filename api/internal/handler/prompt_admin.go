package handler

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/enjoydarts/sifto/api/internal/middleware"
	"github.com/enjoydarts/sifto/api/internal/repository"
	"github.com/enjoydarts/sifto/api/internal/service"
	"github.com/go-chi/chi/v5"
)

type PromptAdminHandler struct {
	repo  *repository.PromptTemplateRepo
	auth  *service.PromptAdminAuthService
	users *repository.UserRepo
}

type promptTemplateDetailResponse struct {
	Template        repository.PromptTemplate          `json:"template"`
	Versions        []repository.PromptTemplateVersion `json:"versions"`
	Experiments     []repository.PromptExperiment      `json:"experiments"`
	Arms            []repository.PromptExperimentArm   `json:"arms"`
	DefaultTemplate service.PromptTemplateDefault      `json:"default_template"`
}

func NewPromptAdminHandler(repo *repository.PromptTemplateRepo, auth *service.PromptAdminAuthService, users *repository.UserRepo) *PromptAdminHandler {
	return &PromptAdminHandler{repo: repo, auth: auth, users: users}
}

type promptAdminActor struct {
	userID string
	email  string
}

func (h *PromptAdminHandler) capabilities(r *http.Request) (*promptAdminActor, bool) {
	if h == nil || h.auth == nil || h.users == nil {
		return nil, false
	}
	userID := strings.TrimSpace(middleware.GetUserID(r))
	if userID == "" {
		return nil, false
	}
	user, err := h.users.GetByID(r.Context(), userID)
	if err != nil || user == nil {
		return nil, false
	}
	email := strings.TrimSpace(user.Email)
	if !h.auth.CanManagePrompts(email) {
		return &promptAdminActor{userID: userID, email: email}, false
	}
	return &promptAdminActor{userID: userID, email: email}, true
}

func (h *PromptAdminHandler) GetCapabilities(w http.ResponseWriter, r *http.Request) {
	actor, allowed := h.capabilities(r)
	writeJSON(w, map[string]any{
		"can_manage_prompts": allowed,
		"user_email":         actorEmail(actor),
		"purposes": []string{
			"summary",
			"facts",
			"digest",
			"audio_briefing_script",
		},
	})
}

func (h *PromptAdminHandler) ListTemplates(w http.ResponseWriter, r *http.Request) {
	if _, allowed := h.capabilities(r); !allowed {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}
	out, err := h.repo.ListTemplates(r.Context())
	if err != nil {
		writeRepoError(w, err)
		return
	}
	writeJSON(w, map[string]any{"templates": out})
}

func (h *PromptAdminHandler) GetTemplateDetail(w http.ResponseWriter, r *http.Request) {
	if _, allowed := h.capabilities(r); !allowed {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}
	detail, err := h.repo.GetTemplateDetail(r.Context(), chi.URLParam(r, "id"))
	if err != nil {
		writeRepoError(w, err)
		return
	}
	if detail == nil {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	writeJSON(w, promptTemplateDetailResponse{
		Template:        detail.Template,
		Versions:        detail.Versions,
		Experiments:     detail.Experiments,
		Arms:            detail.Arms,
		DefaultTemplate: service.LookupPromptTemplateDefault(detail.Template.Key),
	})
}

func (h *PromptAdminHandler) CreateVersion(w http.ResponseWriter, r *http.Request) {
	actor, allowed := h.capabilities(r)
	if !allowed {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}
	var body struct {
		Label              string          `json:"label"`
		SystemInstruction  string          `json:"system_instruction"`
		PromptText         string          `json:"prompt_text"`
		FallbackPromptText string          `json:"fallback_prompt_text"`
		VariablesSchema    json.RawMessage `json:"variables_schema"`
		Notes              string          `json:"notes"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "invalid json", http.StatusBadRequest)
		return
	}
	if strings.TrimSpace(body.PromptText) == "" {
		http.Error(w, "prompt_text is required", http.StatusBadRequest)
		return
	}
	version, err := h.repo.CreateVersion(r.Context(), repository.PromptTemplateVersionInput{
		TemplateID:         chi.URLParam(r, "id"),
		Label:              body.Label,
		SystemInstruction:  body.SystemInstruction,
		PromptText:         body.PromptText,
		FallbackPromptText: body.FallbackPromptText,
		VariablesSchema:    body.VariablesSchema,
		Notes:              body.Notes,
		CreatedByUserID:    &actor.userID,
		CreatedByEmail:     actor.email,
	})
	if err != nil {
		writeRepoError(w, err)
		return
	}
	templateID := chi.URLParam(r, "id")
	_ = h.repo.InsertAuditLog(r.Context(), repository.PromptAdminAuditLogInput{
		UserID:     &actor.userID,
		UserEmail:  actor.email,
		Action:     "create_version",
		TemplateID: &templateID,
		VersionID:  &version.ID,
		Metadata:   json.RawMessage(`{}`),
	})
	writeJSON(w, version)
}

func (h *PromptAdminHandler) ActivateTemplateVersion(w http.ResponseWriter, r *http.Request) {
	actor, allowed := h.capabilities(r)
	if !allowed {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}
	var body struct {
		VersionID *string `json:"version_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "invalid json", http.StatusBadRequest)
		return
	}
	templateID := chi.URLParam(r, "id")
	var err error
	if body.VersionID == nil || strings.TrimSpace(*body.VersionID) == "" {
		err = h.repo.ClearActiveVersion(r.Context(), templateID)
	} else {
		err = h.repo.ActivateVersion(r.Context(), templateID, strings.TrimSpace(*body.VersionID))
	}
	if err != nil {
		writeRepoError(w, err)
		return
	}
	versionID := body.VersionID
	_ = h.repo.InsertAuditLog(r.Context(), repository.PromptAdminAuditLogInput{
		UserID:     &actor.userID,
		UserEmail:  actor.email,
		Action:     "activate_version",
		TemplateID: &templateID,
		VersionID:  versionID,
		Metadata:   json.RawMessage(`{}`),
	})
	writeJSON(w, map[string]any{"ok": true})
}

func (h *PromptAdminHandler) CreateExperiment(w http.ResponseWriter, r *http.Request) {
	actor, allowed := h.capabilities(r)
	if !allowed {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}
	var body struct {
		TemplateID     string                                `json:"template_id"`
		Name           string                                `json:"name"`
		Status         string                                `json:"status"`
		AssignmentUnit string                                `json:"assignment_unit"`
		StartedAt      *time.Time                            `json:"started_at"`
		EndedAt        *time.Time                            `json:"ended_at"`
		Arms           []repository.PromptExperimentArmInput `json:"arms"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "invalid json", http.StatusBadRequest)
		return
	}
	if strings.TrimSpace(body.TemplateID) == "" || strings.TrimSpace(body.Name) == "" || strings.TrimSpace(body.AssignmentUnit) == "" {
		http.Error(w, "template_id, name, assignment_unit are required", http.StatusBadRequest)
		return
	}
	exp, arms, err := h.repo.CreateExperiment(r.Context(), repository.PromptExperimentInput{
		TemplateID:      strings.TrimSpace(body.TemplateID),
		Name:            body.Name,
		Status:          body.Status,
		AssignmentUnit:  body.AssignmentUnit,
		StartedAt:       body.StartedAt,
		EndedAt:         body.EndedAt,
		CreatedByUserID: &actor.userID,
		CreatedByEmail:  actor.email,
	}, body.Arms)
	if err != nil {
		writeRepoError(w, err)
		return
	}
	_ = h.repo.InsertAuditLog(r.Context(), repository.PromptAdminAuditLogInput{
		UserID:       &actor.userID,
		UserEmail:    actor.email,
		Action:       "create_experiment",
		TemplateID:   &exp.TemplateID,
		ExperimentID: &exp.ID,
		Metadata:     json.RawMessage(`{}`),
	})
	writeJSON(w, map[string]any{"experiment": exp, "arms": arms})
}

func (h *PromptAdminHandler) UpdateExperiment(w http.ResponseWriter, r *http.Request) {
	actor, allowed := h.capabilities(r)
	if !allowed {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}
	var body struct {
		Status    string                                `json:"status"`
		StartedAt *time.Time                            `json:"started_at"`
		EndedAt   *time.Time                            `json:"ended_at"`
		Arms      []repository.PromptExperimentArmInput `json:"arms"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "invalid json", http.StatusBadRequest)
		return
	}
	exp, arms, err := h.repo.UpdateExperiment(r.Context(), chi.URLParam(r, "id"), body.Status, body.StartedAt, body.EndedAt, body.Arms)
	if err != nil {
		writeRepoError(w, err)
		return
	}
	if exp == nil {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	_ = h.repo.InsertAuditLog(r.Context(), repository.PromptAdminAuditLogInput{
		UserID:       &actor.userID,
		UserEmail:    actor.email,
		Action:       "update_experiment",
		TemplateID:   &exp.TemplateID,
		ExperimentID: &exp.ID,
		Metadata:     json.RawMessage(`{}`),
	})
	writeJSON(w, map[string]any{"experiment": exp, "arms": arms})
}

func actorEmail(actor *promptAdminActor) string {
	if actor == nil {
		return ""
	}
	return actor.email
}
