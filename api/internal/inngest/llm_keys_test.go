package inngest

import "testing"

func TestLLMKeysTupleMapsXiaomiMiMoTokenPlanToOpenAICompatibleKey(t *testing.T) {
	key := "mimo-key"
	model := "mimo-v2-pro"

	rt, err := llmKeysTuple("xiaomi_mimo_token_plan", &key, &model)
	if err != nil {
		t.Fatalf("llmKeysTuple() error = %v", err)
	}
	if rt.OpenAIKey == nil || *rt.OpenAIKey != key {
		t.Fatalf("OpenAIKey = %v, want %q", rt.OpenAIKey, key)
	}
	if rt.AnthropicKey != nil {
		t.Fatalf("AnthropicKey = %v, want nil", rt.AnthropicKey)
	}
}

func TestLLMKeysTupleMapsCerebrasToOpenAICompatibleKey(t *testing.T) {
	key := "cerebras-key"
	model := "cerebras::llama-4-scout-17b-16e-instruct"

	rt, err := llmKeysTuple("cerebras", &key, &model)
	if err != nil {
		t.Fatalf("llmKeysTuple() error = %v", err)
	}
	if rt.OpenAIKey == nil || *rt.OpenAIKey != key {
		t.Fatalf("OpenAIKey = %v, want %q", rt.OpenAIKey, key)
	}
	if rt.AnthropicKey != nil {
		t.Fatalf("AnthropicKey = %v, want nil", rt.AnthropicKey)
	}
}
