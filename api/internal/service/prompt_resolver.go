package service

import (
	"context"
	"hash/fnv"
	"log"
	"strconv"
	"strings"
	"time"

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

type PromptResolveInput struct {
	PromptKey      string
	AssignmentUnit string
	AssignmentKey  string
	AssignmentTime time.Time
}

func NewPromptResolver(repo *repository.PromptTemplateRepo) *PromptResolver {
	return &PromptResolver{repo: repo}
}

func (r *PromptResolver) Resolve(ctx context.Context, input PromptResolveInput) (*PromptResolution, error) {
	promptKey := strings.TrimSpace(input.PromptKey)
	if r == nil || r.repo == nil || promptKey == "" {
		return &PromptResolution{PromptKey: promptKey, PromptSource: "default_code"}, nil
	}
	active, err := r.repo.GetActiveVersionByKey(ctx, promptKey)
	if err != nil {
		log.Printf("prompt resolver fallback to default_code prompt_key=%s err=%v", promptKey, err)
		return &PromptResolution{PromptKey: promptKey, PromptSource: "default_code"}, nil
	}
	if active == nil {
		return &PromptResolution{PromptKey: promptKey, PromptSource: "default_code"}, nil
	}
	if activeExperiment, err := r.repo.GetActiveExperimentByKey(ctx, promptKey, strings.TrimSpace(input.AssignmentUnit), promptAssignmentTime(input.AssignmentTime)); err != nil {
		log.Printf("prompt resolver experiment lookup fallback prompt_key=%s assignment_unit=%s err=%v", promptKey, input.AssignmentUnit, err)
	} else if activeExperiment != nil {
		chosenArm := choosePromptExperimentArm(activeExperiment.Arms, input.AssignmentKey, activeExperiment.Experiment.ID)
		if chosenArm != nil {
			chosenVersion, err := r.repo.GetVersionByID(ctx, chosenArm.VersionID)
			if err != nil {
				log.Printf("prompt resolver experiment version fallback prompt_key=%s version_id=%s err=%v", promptKey, chosenArm.VersionID, err)
			} else if chosenVersion != nil {
				experimentID := activeExperiment.Experiment.ID
				armID := chosenArm.ID
				versionID := chosenVersion.VersionID
				versionNumber := chosenVersion.VersionNumber
				return &PromptResolution{
					PromptKey:             chosenVersion.TemplateKey,
					PromptSource:          "template_version",
					PromptText:            chosenVersion.PromptText,
					SystemInstruction:     chosenVersion.SystemInstruction,
					PromptVersionID:       &versionID,
					PromptVersionNumber:   &versionNumber,
					PromptExperimentID:    &experimentID,
					PromptExperimentArmID: &armID,
				}, nil
			}
		}
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

func ResolvePromptResolution(ctx context.Context, resolver *PromptResolver, input PromptResolveInput) *PromptResolution {
	if resolver == nil {
		return &PromptResolution{PromptKey: input.PromptKey, PromptSource: "default_code"}
	}
	if ctx == nil {
		ctx = context.Background()
	}
	resolution, err := resolver.Resolve(ctx, input)
	if err != nil || resolution == nil {
		return &PromptResolution{PromptKey: input.PromptKey, PromptSource: "default_code"}
	}
	return resolution
}

func promptAssignmentTime(now time.Time) time.Time {
	if now.IsZero() {
		return time.Now()
	}
	return now
}

func choosePromptExperimentArm(arms []repository.PromptExperimentArm, assignmentKey, experimentID string) *repository.PromptExperimentArm {
	if len(arms) == 0 || strings.TrimSpace(assignmentKey) == "" || strings.TrimSpace(experimentID) == "" {
		return nil
	}
	totalWeight := 0
	for _, arm := range arms {
		if arm.Weight > 0 {
			totalWeight += arm.Weight
		}
	}
	if totalWeight <= 0 {
		return nil
	}
	hasher := fnv.New64a()
	_, _ = hasher.Write([]byte(experimentID))
	_, _ = hasher.Write([]byte(":"))
	_, _ = hasher.Write([]byte(assignmentKey))
	bucket := int(hasher.Sum64() % uint64(totalWeight))
	running := 0
	for i := range arms {
		if arms[i].Weight <= 0 {
			continue
		}
		running += arms[i].Weight
		if bucket < running {
			return &arms[i]
		}
	}
	return nil
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
