export function formatModelDisplayName(model: string): string {
  const raw = model.startsWith("openrouter::") ? model.slice("openrouter::".length) : model;
  switch (raw) {
    case "deepseek-chat":
      return "deepseek-chat(V3.2)";
    case "deepseek-reasoner":
      return "deepseek-reasoner(V3.2)";
    default:
      return raw;
  }
}
