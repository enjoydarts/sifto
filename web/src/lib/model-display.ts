export function formatModelDisplayName(model: string): string {
  const raw = model.startsWith("openrouter::")
    ? model.slice("openrouter::".length)
    : model.startsWith("together::")
      ? model.slice("together::".length)
    : model.startsWith("siliconflow::")
      ? model.slice("siliconflow::".length)
      : model;
  switch (raw) {
    case "deepseek-chat":
      return "deepseek-chat(V3.2)";
    case "deepseek-reasoner":
      return "deepseek-reasoner(V3.2)";
    default:
      return raw;
  }
}
