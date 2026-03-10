package service

import "strings"

func IsGeminiModel(model *string) bool {
	if model == nil {
		return false
	}
	v := strings.ToLower(strings.TrimSpace(*model))
	return v != "" && (strings.HasPrefix(v, "gemini-") || strings.Contains(v, "/models/gemini-"))
}

func IsGroqModel(model *string) bool {
	if model == nil {
		return false
	}
	v := strings.ToLower(strings.TrimSpace(*model))
	if v == "" {
		return false
	}
	return strings.HasPrefix(v, "openai/gpt-oss-") ||
		strings.HasPrefix(v, "qwen/") ||
		strings.HasPrefix(v, "meta-llama/") ||
		strings.HasPrefix(v, "llama-")
}

func IsDeepSeekModel(model *string) bool {
	if model == nil {
		return false
	}
	v := strings.ToLower(strings.TrimSpace(*model))
	if v == "" {
		return false
	}
	return v == "deepseek-chat" || v == "deepseek-reasoner"
}

func IsOpenAIModel(model *string) bool {
	if model == nil {
		return false
	}
	v := strings.ToLower(strings.TrimSpace(*model))
	if v == "" {
		return false
	}
	return strings.HasPrefix(v, "gpt-5")
}

func LLMProviderForModel(model *string) string {
	switch {
	case IsGeminiModel(model):
		return "google"
	case IsGroqModel(model):
		return "groq"
	case IsDeepSeekModel(model):
		return "deepseek"
	case IsOpenAIModel(model):
		return "openai"
	default:
		return "anthropic"
	}
}

func DefaultLLMModelForPurpose(provider, purpose string) string {
	switch provider {
	case "google":
		switch purpose {
		case "digest":
			return "gemini-2.5-pro"
		default:
			return "gemini-2.5-flash"
		}
	case "groq":
		switch purpose {
		case "summary", "digest_cluster_draft", "digest":
			return "openai/gpt-oss-120b"
		default:
			return "openai/gpt-oss-20b"
		}
	case "deepseek":
		switch purpose {
		case "summary", "digest_cluster_draft", "digest":
			return "deepseek-reasoner"
		default:
			return "deepseek-chat"
		}
	case "openai":
		switch purpose {
		case "facts", "source_suggestion":
			return "gpt-5-mini"
		case "summary", "ask":
			return "gpt-5"
		case "digest_cluster_draft", "digest":
			return "gpt-5.4"
		default:
			return "gpt-5-mini"
		}
	default:
		switch purpose {
		case "facts":
			return "claude-haiku-4-5"
		default:
			return "claude-sonnet-4-6"
		}
	}
}
