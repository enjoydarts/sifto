package handler

import (
	"context"
	"fmt"
	"log"
	"strings"

	"github.com/enjoydarts/sifto/api/internal/model"
	"github.com/enjoydarts/sifto/api/internal/service"
)

func resolveBriefingNavigatorModels(settings *model.UserSettings) []string {
	if settings == nil {
		return nil
	}
	models := make([]string, 0, 2)
	for _, configured := range []*string{settings.NavigatorModel, settings.NavigatorFallbackModel} {
		selected := chooseNavigatorModelOverride(configured, settings)
		if selected == nil {
			continue
		}
		duplicate := false
		for _, existing := range models {
			if strings.EqualFold(existing, *selected) {
				duplicate = true
				break
			}
		}
		if !duplicate {
			models = append(models, *selected)
		}
	}
	if len(models) > 0 {
		return models
	}
	for _, provider := range service.CostEfficientLLMProviders("") {
		if !hasNavigatorProviderKey(settings, provider) {
			continue
		}
		if selected := strings.TrimSpace(service.DefaultLLMModelForPurpose(provider, "summary")); selected != "" {
			return []string{selected}
		}
	}
	return nil
}

func generateNavigatorWithFallback[T any](
	ctx context.Context,
	userID, purpose string,
	models []string,
	call func(modelName *string) (*T, error),
) (*T, string, error) {
	errs := make([]string, 0, len(models))
	for idx, modelName := range models {
		if err := ctx.Err(); err != nil {
			return nil, "", err
		}
		resp, err := call(&modelName)
		if err == nil && resp != nil {
			return resp, modelName, nil
		}
		if ctxErr := ctx.Err(); ctxErr != nil {
			return nil, "", ctxErr
		}
		if err == nil {
			err = fmt.Errorf("worker returned nil response")
		}
		errs = append(errs, fmt.Sprintf("%s: %v", modelName, err))
		if idx < len(models)-1 {
			log.Printf("navigator fallback retrying purpose=%s user=%s primary_model=%s fallback_model=%s err=%v", purpose, userID, modelName, models[idx+1], err)
		}
	}
	if len(errs) == 0 {
		return nil, "", fmt.Errorf("%s model not configured", purpose)
	}
	return nil, "", fmt.Errorf("%s failed across models: %s", purpose, strings.Join(errs, " | "))
}
