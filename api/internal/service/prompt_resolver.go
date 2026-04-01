package service

import (
	"context"
	"strconv"
	"strings"

	"github.com/enjoydarts/sifto/api/internal/repository"
)

type PromptResolution struct {
	PromptKey             string  `json:"prompt_key"`
	PromptSource          string  `json:"prompt_source"`
	PromptText            string  `json:"prompt_text"`
	SystemInstruction     string  `json:"system_instruction"`
	PromptVersionID       *string `json:"prompt_version_id,omitempty"`
	PromptVersionNumber   *int    `json:"prompt_version_number,omitempty"`
	PromptExperimentID    *string `json:"prompt_experiment_id,omitempty"`
	PromptExperimentArmID *string `json:"prompt_experiment_arm_id,omitempty"`
}

type PromptResolver struct {
	repo *repository.PromptTemplateRepo
}

func NewPromptResolver(repo *repository.PromptTemplateRepo) *PromptResolver {
	return &PromptResolver{repo: repo}
}

func (r *PromptResolver) Resolve(ctx context.Context, promptKey string) (*PromptResolution, error) {
	if r == nil || r.repo == nil || promptKey == "" {
		return &PromptResolution{PromptKey: promptKey, PromptSource: "default_code"}, nil
	}
	active, err := r.repo.GetActiveVersionByKey(ctx, promptKey)
	if err != nil {
		return &PromptResolution{PromptKey: promptKey, PromptSource: "default_code"}, nil
	}
	if active == nil {
		return &PromptResolution{PromptKey: promptKey, PromptSource: "default_code"}, nil
	}
	versionID := active.VersionID
	versionNumber := active.VersionNumber
	return &PromptResolution{
		PromptKey:           active.TemplateKey,
		PromptSource:        "template_version",
		PromptText:          active.PromptText,
		SystemInstruction:   active.SystemInstruction,
		PromptVersionID:     &versionID,
		PromptVersionNumber: &versionNumber,
	}, nil
}

func WorkerPromptConfigFromResolution(resolution *PromptResolution) *PromptConfig {
	if resolution == nil || resolution.PromptSource == "" || resolution.PromptSource == "default_code" {
		return nil
	}
	return &PromptConfig{
		PromptKey:         resolution.PromptKey,
		PromptSource:      resolution.PromptSource,
		PromptText:        resolution.PromptText,
		SystemInstruction: resolution.SystemInstruction,
		PromptVersionID:   resolution.PromptVersionID,
		PromptVersion:     resolution.PromptVersionNumber,
	}
}

func ResolvePromptResolution(ctx context.Context, resolver *PromptResolver, promptKey string) *PromptResolution {
	if resolver == nil {
		return &PromptResolution{PromptKey: promptKey, PromptSource: "default_code"}
	}
	if ctx == nil {
		ctx = context.Background()
	}
	resolution, err := resolver.Resolve(ctx, promptKey)
	if err != nil || resolution == nil {
		return &PromptResolution{PromptKey: promptKey, PromptSource: "default_code"}
	}
	return resolution
}

func promptKey(prompt *PromptResolution) string {
	if prompt == nil {
		return ""
	}
	return strings.TrimSpace(prompt.PromptKey)
}

func promptSource(prompt *PromptResolution) string {
	if prompt == nil {
		return ""
	}
	return strings.TrimSpace(prompt.PromptSource)
}

func promptVersionID(prompt *PromptResolution) *string {
	if prompt == nil || prompt.PromptVersionID == nil || strings.TrimSpace(*prompt.PromptVersionID) == "" {
		return nil
	}
	return prompt.PromptVersionID
}

func promptVersionNumber(prompt *PromptResolution) *int {
	if prompt == nil || prompt.PromptVersionNumber == nil {
		return nil
	}
	return prompt.PromptVersionNumber
}

func promptExperimentID(prompt *PromptResolution) *string {
	if prompt == nil || prompt.PromptExperimentID == nil || strings.TrimSpace(*prompt.PromptExperimentID) == "" {
		return nil
	}
	return prompt.PromptExperimentID
}

func promptExperimentArmID(prompt *PromptResolution) *string {
	if prompt == nil || prompt.PromptExperimentArmID == nil || strings.TrimSpace(*prompt.PromptExperimentArmID) == "" {
		return nil
	}
	return prompt.PromptExperimentArmID
}

func toVal(v *string) string {
	if v == nil {
		return ""
	}
	return *v
}

func toIntVal(v *int) string {
	if v == nil {
		return ""
	}
	return strconv.Itoa(*v)
}
