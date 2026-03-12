export function getInternalAPISecret(): string {
  return process.env.INTERNAL_API_SECRET ?? "";
}

export function getInternalAPISecretError(): string {
  return "INTERNAL_API_SECRET is not set";
}
