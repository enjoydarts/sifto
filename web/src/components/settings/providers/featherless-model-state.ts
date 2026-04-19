type FeatherlessLikeModel = {
  provider?: string | null;
  availability?: string | null;
  raw_availability?: string | null;
  reason?: string | null;
  gated?: boolean | null;
  available_on_current_plan?: boolean | null;
};

export type FeatherlessModelStateKind = "available" | "gated" | "removed";
export type FeatherlessModelAvailabilityKind = FeatherlessModelStateKind | "unavailable";

export function getFeatherlessModelState(model: FeatherlessLikeModel): {
  kind: FeatherlessModelAvailabilityKind;
  selectable: boolean;
} {
  const provider = (model.provider ?? "").trim().toLowerCase();
  if (provider !== "featherless") {
    return { kind: "available", selectable: true };
  }

  const availability = (model.availability ?? "").trim().toLowerCase();
  const rawAvailability = (model.raw_availability ?? "").trim().toLowerCase();
  const reason = (model.reason ?? "").trim().toLowerCase();
  const gated = model.gated === true;
  const removed = availability === "removed" || rawAvailability === "removed" || reason === "removed";
  const unavailable = model.available_on_current_plan === false || availability === "unavailable" || rawAvailability === "unavailable" || rawAvailability === "not_on_plan" || reason === "not_on_plan";

  if (removed) {
    return { kind: "removed", selectable: false };
  }
  if (unavailable) {
    return { kind: "unavailable", selectable: false };
  }
  if (gated) {
    return { kind: "gated", selectable: true };
  }
  return { kind: "available", selectable: true };
}

export function isFeatherlessModelSelectable(model: FeatherlessLikeModel): boolean {
  return getFeatherlessModelState(model).selectable;
}

export function featherlessModelBadgeLabel(kind: FeatherlessModelAvailabilityKind): string | null {
  switch (kind) {
    case "gated":
      return "Gated";
    case "unavailable":
      return "Unavailable";
    case "removed":
      return "Removed";
    default:
      return null;
  }
}
