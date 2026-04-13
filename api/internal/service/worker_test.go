package service

import "testing"

func workerTestStringPtr(v string) *string { return &v }

func TestSelectOpenAICompatibleKeyPrefersProviderSpecificKey(t *testing.T) {
	togetherKey := workerTestStringPtr("together-key")
	moonshotKey := workerTestStringPtr("moonshot-key")
	openRouterKey := workerTestStringPtr("openrouter-key")
	poeKey := workerTestStringPtr("poe-key")
	siliconFlowKey := workerTestStringPtr("siliconflow-key")
	openAIKey := workerTestStringPtr("openai-key")

	tests := []struct {
		name  string
		model *string
		want  *string
	}{
		{name: "together", model: workerTestStringPtr("together::openai/gpt-oss-20b"), want: togetherKey},
		{name: "moonshot", model: workerTestStringPtr("kimi-k2-turbo-preview"), want: moonshotKey},
		{name: "openrouter", model: workerTestStringPtr("openrouter::openai/gpt-5.4-mini"), want: openRouterKey},
		{name: "poe", model: workerTestStringPtr("poe::claude-sonnet-4"), want: poeKey},
		{name: "siliconflow", model: workerTestStringPtr("siliconflow::Qwen/Qwen3-Next-80B-A3B-Instruct"), want: siliconFlowKey},
		{name: "openai fallback", model: workerTestStringPtr("gpt-5.4-mini"), want: openAIKey},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := selectOpenAICompatibleKey(tt.model, togetherKey, moonshotKey, openRouterKey, poeKey, siliconFlowKey, openAIKey)
			if got == nil || tt.want == nil || *got != *tt.want {
				t.Fatalf("got %v, want %v", workerTestDerefString(got), workerTestDerefString(tt.want))
			}
		})
	}
}

func workerTestDerefString(v *string) string {
	if v == nil {
		return ""
	}
	return *v
}
