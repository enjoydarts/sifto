export function formatModelDisplayName(model: string): string {
  switch (model) {
    case "deepseek-chat":
      return "deepseek-chat(V3.2)";
    case "deepseek-reasoner":
      return "deepseek-reasoner(V3.2)";
    default:
      return model;
  }
}
