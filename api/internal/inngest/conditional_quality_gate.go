package inngest

import (
	"context"
	"time"

	"github.com/enjoydarts/sifto/api/internal/service"
)

const d1GateTimeout = 10 * time.Second
const d1GatePolicyVersion = "d1-conditional-gate-v1"

func qualityGateComment(provider string) string {
	if provider == "d1" {
		return "D1の高信頼品質ゲートを通過しました。"
	}
	return "Jevの高信頼品質ゲートを通過しました。"
}

// Only uncertainty or noncritical score issues can be rescued by D1.
func selectQualityGate(jev *jevPrecheckStepResult, evaluateD1 func() *jevPrecheckStepResult) (string, bool) {
	if jev != nil && !jev.Skipped && jev.Gate.Decision == service.JevDecisionAccepted {
		return "jev", false
	}
	if jev == nil || jev.Skipped || jev.Gate.Decision != service.JevDecisionEscalated {
		return "", false
	}
	switch jev.Gate.EscalationReason {
	case service.JevEscalationLowConfidence, service.JevEscalationLowDimensionScore:
		d1 := evaluateD1()
		if d1 != nil && !d1.Skipped && d1.Gate.Decision == service.JevDecisionAccepted {
			return "d1", true
		}
		return "", true
	}
	return "", false
}

func executeConditionalQualityGate(ctx context.Context, deps processItemDeps, jevConfig, d1Config jevPrecheckConfig, shadow func()) string {
	jev := executeQualityPrecheck(ctx, deps, jevConfig)
	provider, usedD1 := selectQualityGate(jev, func() *jevPrecheckStepResult {
		d1Config.Catalog.GatePolicy.Version = d1GatePolicyVersion
		d1Config.StepName = jevStepName(d1Config.StepName, d1GatePolicyVersion, d1Config.Attempt)
		d1Config.Timeout = d1GateTimeout
		return executeQualityPrecheck(ctx, deps, d1Config)
	})
	if !usedD1 {
		shadow()
	}
	return provider
}
