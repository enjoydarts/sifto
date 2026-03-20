"use client";

import { Newspaper, RefreshCw, ShieldAlert, WifiOff } from "lucide-react";
import { EmptyState } from "@/components/empty-state";
import { ErrorState } from "@/components/ui/error-state";
import { SkeletonList } from "@/components/ui/skeleton-list";

function classifyError(raw: string | null) {
  if (!raw) return "none" as const;
  const text = raw.toLowerCase();
  if (text.includes("401") || text.includes("403") || text.includes("unauthorized") || text.includes("forbidden")) {
    return "permission" as const;
  }
  if (text.includes("network") || text.includes("fetch") || text.includes("offline") || text.includes("timeout")) {
    return "offline" as const;
  }
  if (text.includes("429") || text.includes("rate limit")) {
    return "rate-limited" as const;
  }
  if (text.includes("page") || text.includes("pagination")) {
    return "pagination" as const;
  }
  return "generic" as const;
}

export function ItemsListState({
  loading,
  error,
  isSearchActive,
  hasFilters,
  isPendingMode,
  onRetry,
  onResetFilters,
  t,
}: {
  loading: boolean;
  error: string | null;
  isSearchActive: boolean;
  hasFilters: boolean;
  isPendingMode: boolean;
  onRetry: () => void;
  onResetFilters: () => void;
  t: (key: string, fallback?: string) => string;
}) {
  if (loading) {
    return <SkeletonList rows={8} />;
  }

  if (error) {
    const tone = classifyError(error);
    const icon =
      tone === "permission"
        ? ShieldAlert
        : tone === "offline"
          ? WifiOff
          : RefreshCw;
    const titleKey =
      tone === "permission"
        ? "items.state.permissionTitle"
        : tone === "offline"
          ? "items.state.offlineTitle"
          : tone === "rate-limited"
            ? "items.state.rateLimitedTitle"
            : tone === "pagination"
              ? "items.state.paginationTitle"
              : "items.state.errorTitle";
    const descriptionKey =
      tone === "permission"
        ? "items.state.permissionDescription"
        : tone === "offline"
          ? "items.state.offlineDescription"
          : tone === "rate-limited"
            ? "items.state.rateLimitedDescription"
            : tone === "pagination"
              ? "items.state.paginationDescription"
              : "items.state.errorDescription";

    return (
      <ErrorState
        icon={icon}
        title={t(titleKey)}
        description={t(descriptionKey)}
        actionLabel={t("common.refresh")}
        onAction={onRetry}
      />
    );
  }

  const noResults = hasFilters || isSearchActive;

  return (
    <EmptyState
      icon={Newspaper}
      title={t(noResults ? "items.state.noResultsTitle" : isPendingMode ? "emptyState.itemsPending.title" : "emptyState.items.title")}
      description={t(noResults ? "items.state.noResultsDescription" : isPendingMode ? "emptyState.itemsPending.desc" : "emptyState.items.desc")}
      action={
        noResults
          ? { label: t("items.state.resetFilters"), onClick: onResetFilters }
          : { label: t("emptyState.items.action"), href: "/sources" }
      }
    />
  );
}
