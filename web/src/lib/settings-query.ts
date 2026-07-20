import { queryOptions } from "@tanstack/react-query";
import { api } from "@/lib/api";
import { queryKeys } from "@/lib/query-keys";

export function settingsQueryOptions() {
  return queryOptions({
    queryKey: queryKeys.settings.all(),
    queryFn: () => api.getSettings(),
    staleTime: 60_000,
  });
}
