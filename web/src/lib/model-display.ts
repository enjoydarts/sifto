const PROVIDER_LABELS: Record<string, string> = {
  openai: "OpenAI",
  anthropic: "Anthropic",
  google: "Google",
  groq: "Groq",
  deepseek: "DeepSeek",
  alibaba: "Alibaba",
  minimax: "MiniMax",
  mistral: "Mistral",
  together: "Together AI",
  xai: "xAI",
  zai: "Z.ai",
  fireworks: "Fireworks",
  moonshot: "Moonshot",
  openrouter: "OpenRouter",
  poe: "Poe",
  siliconflow: "SiliconFlow",
};

export function normalizeProvider(provider: string): string {
  const p = provider.trim().toLowerCase();
  if (p.startsWith("poe::") || p.startsWith("poe/")) {
    return "poe";
  }
  if (p.startsWith("siliconflow::") || p.startsWith("siliconflow/")) {
    return "siliconflow";
  }
  if (p.startsWith("minimax::") || p.startsWith("minimax/")) {
    return "minimax";
  }
  return p;
}

export function providerLabel(provider: string): string {
  const normalized = normalizeProvider(provider);
  return PROVIDER_LABELS[normalized] ?? provider;
}

export function formatModelDisplayName(model: string): string {
  const raw = model.startsWith("openrouter::")
    ? model.slice("openrouter::".length)
    : model.startsWith("together::")
      ? model.slice("together::".length)
    : model.startsWith("siliconflow::")
      ? model.slice("siliconflow::".length)
    : model.startsWith("minimax::")
      ? model.slice("minimax::".length)
      : model.startsWith("minimax/")
        ? model.slice("minimax/".length)
      : model;
  switch (raw) {
    case "mistral-large-2512":
      return "Mistral Large 3";
    case "mistral-medium-2508":
      return "Mistral Medium 3.1";
    case "mistral-small-2603":
      return "Mistral Small 4";
    case "mistral-small-2506":
      return "Mistral Small 3.2";
    case "ministral-14b-2512":
      return "Ministral 3 14B";
    case "ministral-8b-2512":
      return "Ministral 3 8B";
    case "ministral-3b-2512":
      return "Ministral 3 3B";
    case "magistral-medium-2509":
      return "Magistral Medium 1.2";
    case "magistral-small-2509":
      return "Magistral Small 1.2";
    case "deepseek-chat":
      return "deepseek-chat(V3.2)";
    case "deepseek-reasoner":
      return "deepseek-reasoner(V3.2)";
    default:
      return raw;
  }
}
