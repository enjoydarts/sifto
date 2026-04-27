// API client — proxied via Next.js rewrites to Go API
// Browser calls /api/* → Next.js rewrites → http://localhost:8080/api/*

export * from "@/types/api";

import type {
  AINavigatorBriefDetailResponse,
  AINavigatorBriefListResponse,
  AINavigatorBriefSummaryAudioQueueResponse,
  AivisModelsResponse,
  AivisSyncStatusResponse,
  AivisUserDictionariesResponse,
  AskCandidate,
  AskCitation,
  AskInsight,
  AskNavigatorResponse,
  AskResponse,
  AudioBriefingDetailResponse,
  AudioBriefingJob,
  AudioBriefingPersonaVoice,
  AudioBriefingPreset,
  AudioBriefingSettings,
  DeepInfraModelsResponse,
  DeepInfraSyncStatusResponse,
  BriefingNavigatorResponse,
  BriefingTodayResponse,
  CreatePlaybackSessionRequest,
  DashboardSnapshot,
  Digest,
  DigestDetail,
  ElevenLabsVoicesResponse,
  FeatherlessModelsResponse,
  FeatherlessSyncStatusResponse,
  FishBrowseResponse,
  FishBrowseSort,
  FocusQueueResponse,
  UIFontCatalogResponse,
  GeminiTTSVoicesResponse,
  ItemDetail,
  ItemFeedbackResult,
  ItemGenreUpdateResult,
  ItemHighlight,
  ItemListResponse,
  ItemNavigatorResponse,
  ItemNote,
  ItemReadResult,
  ItemRetryResult,
  ItemSearchSuggestionResponse,
  ItemStats,
  ItemUXMetrics,
  LatestPlaybackSessionsResponse,
  LLMCatalog,
  LLMExecutionCurrentMonthSummary,
  LLMUsageAnalysisSummary,
  LLMUsageDailySummary,
  LLMUsageLog,
  LLMUsageModelSummary,
  LLMUsageProviderMonthSummary,
  LLMUsagePurposeMonthSummary,
  LLMValueMetric,
  NavigatorPersonaDefinition,
  NotificationPriorityRule,
  ObsidianExportRunResult,
  OpenAITTSVoicesResponse,
  OpenRouterModelsResponse,
  OpenRouterSyncStatusResponse,
  PlaybackMode,
  PlaybackSession,
  PlaybackSessionStatus,
  PlaybackSessionsResponse,
  PoeModelsResponse,
  PoeSyncStatusResponse,
  PoeUsageResponse,
  PodcastSettings,
  PreferenceProfile,
  PreferenceProfileSummary,
  PromptAdminCapabilities,
  PromptExperiment,
  PromptExperimentArm,
  PromptTemplate,
  PromptTemplateDetailResponse,
  PromptTemplateVersion,
  ProviderModelChangeEvent,
  ProviderModelSnapshotListResponse,
  ProviderModelSnapshotSyncSummary,
  ReadingGoal,
  ReadingPlanResponse,
  RelatedItemsResponse,
  ReviewQueueResponse,
  SaveAudioBriefingPresetRequest,
  SummaryAudioSynthesisResponse,
  SummaryAudioVoiceSettings,
  Source,
  SourceDailyStats,
  SourceHealth,
  SourceItemStats,
  SourceNavigatorResponse,
  SourceOptimizationItem,
  SourcesDailyOverview,
  SourceSuggestion,
  TodayQueueResponse,
  TopicPulseItem,
  TopicTrend,
  TriageQueueResponse,
  UpdatePlaybackSessionRequest,
  UserReadingPlanSettings,
  UserSettings,
  WeeklyReviewSnapshot,
  XAIVoicesResponse,
  XAIVoiceSyncRun,
  OpenAITTSVoiceSyncRun,
  AzureSpeechVoicesResponse,
  BulkMarkReadResult,
  BulkMarkLaterResult,
  BulkDeleteItemsResult,
  BulkRetryFailedResult,
  BulkRetryItemsResult,
  FavoritesMarkdownExportParams,
  ItemLaterResult,
} from "@/types/api";

async function clientFetch<T>(path: string, options?: RequestInit, opts?: { apiPrefix?: boolean; forceFresh?: boolean }): Promise<T> {
  const requestPath = withCacheBust(path, options?.method, opts?.forceFresh !== false);
  const targetPath = `${opts?.apiPrefix === false ? "" : "/api"}${requestPath}`;
  const authHeaders = await getAuthHeaders();
  let res = await fetch(targetPath, {
    cache: "no-store",
    ...options,
    headers: {
      "Content-Type": "application/json",
      ...authHeaders,
      ...options?.headers,
    },
  });
  if (res.status === 401 && authHeaders.Authorization) {
    await resolveClerkIdentityIfNeeded();
    const retryAuthHeaders = await getAuthHeaders();
    res = await fetch(targetPath, {
      cache: "no-store",
      ...options,
      headers: {
        "Content-Type": "application/json",
        ...retryAuthHeaders,
        ...options?.headers,
      },
    });
  }
  if (!res.ok) {
    const text = await res.text().catch(() => "");
    throw new Error(`${res.status}: ${text || res.statusText}`);
  }
  if (res.status === 204) return undefined as T;
  return res.json();
}

async function apiFetch<T>(path: string, options?: RequestInit): Promise<T> {
  return clientFetch<T>(path, options, { apiPrefix: true });
}

async function apiFetchStable<T>(path: string, options?: RequestInit): Promise<T> {
  return clientFetch<T>(path, options, { apiPrefix: true, forceFresh: false });
}

const FORCE_FRESH_UNTIL_KEY = "sifto.forceFreshUntil";

function withCacheBust(path: string, method?: string, enabled = true): string {
  const upperMethod = (method ?? "GET").toUpperCase();
  if (upperMethod !== "GET" && upperMethod !== "HEAD") return path;
  if (!enabled) return path;
  if (typeof window === "undefined") return path;
  const raw = window.sessionStorage.getItem(FORCE_FRESH_UNTIL_KEY);
  const until = raw ? Number(raw) : 0;
  if (!Number.isFinite(until) || until <= Date.now()) {
    if (raw) window.sessionStorage.removeItem(FORCE_FRESH_UNTIL_KEY);
    return path;
  }
  const hasQuery = path.includes("?");
  const sep = hasQuery ? "&" : "?";
  if (path.includes("cache_bust=")) return path;
  return `${path}${sep}cache_bust=1`;
}

export function enableForceFreshReload(windowMs = 15000) {
  if (typeof window === "undefined") return;
  window.sessionStorage.setItem(FORCE_FRESH_UNTIL_KEY, String(Date.now() + windowMs));
}

async function getAuthHeaders(): Promise<Record<string, string>> {
  if (typeof window === "undefined") return {};
  const token = await window.__siftoGetAuthToken?.().catch(() => null);
  if (!token) return {};
  return { Authorization: `Bearer ${token}` };
}

async function resolveClerkIdentityIfNeeded(): Promise<void> {
  if (typeof window === "undefined") return;
  if (window.__siftoClerkIdentityResolved) return;
  if (window.__siftoClerkIdentityPromise) {
    await window.__siftoClerkIdentityPromise.catch(() => undefined);
    return;
  }
  const promise = fetch("/api/auth/clerk/resolve-identity", {
    method: "POST",
    cache: "no-store",
  })
    .then(async (res) => {
      if (!res.ok) {
        const text = await res.text().catch(() => "");
        throw new Error(text || `resolve identity failed: ${res.status}`);
      }
      window.__siftoClerkIdentityResolved = true;
    })
    .catch(() => {
      window.__siftoClerkIdentityResolved = false;
    })
    .finally(() => {
      window.__siftoClerkIdentityPromise = undefined;
    });
  window.__siftoClerkIdentityPromise = promise;
  await promise;
}

export const api = {
  // Sources
  getSources: () => apiFetch<Source[]>("/sources"),
  getSourceItemStats: () => apiFetch<{ items: SourceItemStats[] }>("/sources/stats"),
  getSourceDailyStats: (days = 30) => apiFetch<{ items: SourceDailyStats[]; overview: SourcesDailyOverview }>(`/sources/daily-stats?days=${days}`),
  getSourceHealth: () => apiFetch<{ items: SourceHealth[] }>("/sources/health"),
  getSourceOptimization: () => apiFetch<{ items: SourceOptimizationItem[] }>("/sources/optimization"),
  getSourceNavigator: (params?: { cache_bust?: boolean }) => {
    const q = new URLSearchParams();
    if (params?.cache_bust) q.set("cache_bust", "1");
    const qs = q.toString();
    return apiFetch<SourceNavigatorResponse>(`/sources/navigator${qs ? `?${qs}` : ""}`);
  },
  exportSourcesOPML: async () => {
    const authHeaders = await getAuthHeaders();
    const res = await fetch("/api/sources/opml", {
      headers: {
        ...authHeaders,
      },
    });
    if (!res.ok) {
      const text = await res.text().catch(() => "");
      throw new Error(`${res.status}: ${text || res.statusText}`);
    }
    return res.text();
  },
  importSourcesOPML: (opml: string) =>
    apiFetch<{ status: string; total: number; added: number; skipped: number; invalid: number; error_count: number; error_sample?: string[] }>(
      "/sources/opml/import",
      {
        method: "POST",
        body: JSON.stringify({ opml }),
      }
    ),
  importInoreaderSources: (accessToken?: string) =>
    apiFetch<{ status: string; total: number; added: number; skipped: number; invalid: number; error_count: number; error_sample?: string[] }>(
      "/sources/inoreader/import",
      {
        method: "POST",
        ...(accessToken ? { body: JSON.stringify({ access_token: accessToken }) } : {}),
      }
    ),
  getSourceSuggestions: (params?: { limit?: number }) => {
    const q = new URLSearchParams();
    if (params?.limit) q.set("limit", String(params.limit));
    const qs = q.toString();
    return apiFetch<{ items: SourceSuggestion[]; limit: number; llm?: { provider?: string; model?: string; estimated_cost_usd?: number; warning?: string; error?: string; stage?: string; items_count?: number } | null }>(`/sources/suggestions${qs ? `?${qs}` : ""}`);
  },
  getRecommendedSources: (params?: { limit?: number }) => {
    const q = new URLSearchParams();
    if (params?.limit) q.set("limit", String(params.limit));
    const qs = q.toString();
    return apiFetch<{ items: SourceSuggestion[]; limit: number; llm?: { provider?: string; model?: string; estimated_cost_usd?: number; warning?: string; error?: string; stage?: string; items_count?: number } | null }>(`/sources/recommended${qs ? `?${qs}` : ""}`);
  },
  createSource: (body: { url: string; title?: string; type?: string }) =>
    apiFetch<Source>("/sources", { method: "POST", body: JSON.stringify(body) }),
  updateSource: (id: string, body: { enabled?: boolean; title?: string }) =>
    apiFetch<Source>(`/sources/${id}`, {
      method: "PATCH",
      body: JSON.stringify(body),
    }),
  deleteSource: (id: string) =>
    apiFetch<void>(`/sources/${id}`, { method: "DELETE" }),
  discoverFeeds: (url: string) =>
    apiFetch<{ feeds: { url: string; title: string | null }[] }>(
      "/sources/discover",
      { method: "POST", body: JSON.stringify({ url }) }
    ),

  // Items
  getItems: (params?: { status?: string; source_id?: string; topic?: string; genre?: string; q?: string; search_mode?: string; page?: number; page_size?: number; sort?: string; unread_only?: boolean; read_only?: boolean; favorite_only?: boolean; later_only?: boolean }) => {
    const q = new URLSearchParams();
    if (params?.status) q.set("status", params.status);
    if (params?.source_id) q.set("source_id", params.source_id);
    if (params?.topic) q.set("topic", params.topic);
    if (params?.genre) q.set("genre", params.genre);
    if (params?.q) q.set("q", params.q);
    if (params?.search_mode) q.set("search_mode", params.search_mode);
    if (params?.page) q.set("page", String(params.page));
    if (params?.page_size) q.set("page_size", String(params.page_size));
    if (params?.sort) q.set("sort", params.sort);
    if (params?.unread_only != null) q.set("unread_only", String(params.unread_only));
    if (params?.read_only != null) q.set("read_only", String(params.read_only));
    if (params?.favorite_only != null) q.set("favorite_only", String(params.favorite_only));
    if (params?.later_only != null) q.set("later_only", String(params.later_only));
    const qs = q.toString();
    return apiFetch<ItemListResponse>(`/items${qs ? `?${qs}` : ""}`);
  },
  getItemSearchSuggestions: (params: { q: string; limit?: number }) => {
    const q = new URLSearchParams();
    q.set("q", params.q);
    if (params.limit != null) q.set("limit", String(params.limit));
    const qs = q.toString();
    return apiFetch<ItemSearchSuggestionResponse>(`/items/search-suggestions?${qs}`);
  },
  exportFavoritesMarkdown: async (params?: FavoritesMarkdownExportParams) => {
    const q = new URLSearchParams();
    if (params?.days != null) q.set("days", String(params.days));
    if (params?.limit != null) q.set("limit", String(params.limit));
    const authHeaders = await getAuthHeaders();
    const qs = q.toString();
    const res = await fetch(`/api/items/favorites/export-markdown${qs ? `?${qs}` : ""}`, {
      cache: "no-store",
      headers: {
        ...authHeaders,
      },
    });
    if (!res.ok) {
      const text = await res.text().catch(() => "");
      throw new Error(`${res.status}: ${text || res.statusText}`);
    }
    return res.text();
  },
  getReadingPlan: (params?: {
    window?: "24h" | "today_jst" | "7d";
    size?: number;
    diversify_topics?: boolean;
    exclude_read?: boolean;
  }) => {
    const q = new URLSearchParams();
    if (params?.window) q.set("window", params.window);
    if (params?.size) q.set("size", String(params.size));
    if (params?.diversify_topics != null) q.set("diversify_topics", String(params.diversify_topics));
    if (params?.exclude_read != null) q.set("exclude_read", String(params.exclude_read));
    const qs = q.toString();
    return apiFetch<ReadingPlanResponse>(`/items/reading-plan${qs ? `?${qs}` : ""}`);
  },
  getFocusQueue: (params?: {
    window?: "24h" | "today_jst" | "7d";
    size?: number;
    diversify_topics?: boolean;
    exclude_later?: boolean;
  }) => {
    const q = new URLSearchParams();
    if (params?.window) q.set("window", params.window);
    if (params?.size) q.set("size", String(params.size));
    if (params?.diversify_topics != null) q.set("diversify_topics", String(params.diversify_topics));
    if (params?.exclude_later != null) q.set("exclude_later", String(params.exclude_later));
    const qs = q.toString();
    return apiFetch<FocusQueueResponse>(`/items/focus-queue${qs ? `?${qs}` : ""}`);
  },
  getTriageQueue: (params?: {
    mode?: "quick" | "all";
    window?: "24h" | "today_jst" | "7d";
    size?: number;
    diversify_topics?: boolean;
    exclude_later?: boolean;
  }) => {
    const q = new URLSearchParams();
    if (params?.mode) q.set("mode", params.mode);
    if (params?.window) q.set("window", params.window);
    if (params?.size) q.set("size", String(params.size));
    if (params?.diversify_topics != null) q.set("diversify_topics", String(params.diversify_topics));
    if (params?.exclude_later != null) q.set("exclude_later", String(params.exclude_later));
    const qs = q.toString();
    return apiFetch<TriageQueueResponse>(`/items/triage-queue${qs ? `?${qs}` : ""}`);
  },
  getTodayQueue: (params?: { size?: number }) => {
    const q = new URLSearchParams();
    if (params?.size) q.set("size", String(params.size));
    const qs = q.toString();
    return apiFetch<TodayQueueResponse>(`/items/today-queue${qs ? `?${qs}` : ""}`);
  },
  getTriageAll: () => apiFetch<FocusQueueResponse>("/items/triage-all"),
  getItemStats: () => apiFetch<ItemStats>("/items/stats"),
  getItemUXMetrics: (params?: { days?: number }) => {
    const q = new URLSearchParams();
    if (params?.days) q.set("days", String(params.days));
    const qs = q.toString();
    return apiFetch<ItemUXMetrics>(`/items/ux-metrics${qs ? `?${qs}` : ""}`);
  },
  getDashboard: (params?: { llm_days?: number; topic_limit?: number; digest_limit?: number }) => {
    const q = new URLSearchParams();
    if (params?.llm_days) q.set("llm_days", String(params.llm_days));
    if (params?.topic_limit) q.set("topic_limit", String(params.topic_limit));
    if (params?.digest_limit) q.set("digest_limit", String(params.digest_limit));
    const qs = q.toString();
    return apiFetch<DashboardSnapshot>(`/dashboard${qs ? `?${qs}` : ""}`);
  },
  getBriefingToday: (params?: { size?: number; cache_bust?: boolean }) => {
    const q = new URLSearchParams();
    if (params?.size) q.set("size", String(params.size));
    if (params?.cache_bust) q.set("cache_bust", "1");
    const qs = q.toString();
    return apiFetch<BriefingTodayResponse>(`/briefing/today${qs ? `?${qs}` : ""}`);
  },
  getBriefingNavigator: (params?: { cache_bust?: boolean; navigator_preview?: boolean }) => {
    const q = new URLSearchParams();
    if (params?.cache_bust) q.set("cache_bust", "1");
    if (params?.navigator_preview) q.set("navigator_preview", "1");
    const qs = q.toString();
    return apiFetchStable<BriefingNavigatorResponse>(`/briefing/navigator${qs ? `?${qs}` : ""}`);
  },
  getItemNavigator: (id: string, params?: { cache_bust?: boolean; navigator_preview?: boolean }) => {
    const q = new URLSearchParams();
    if (params?.cache_bust) q.set("cache_bust", "1");
    if (params?.navigator_preview) q.set("navigator_preview", "1");
    const qs = q.toString();
    return apiFetch<ItemNavigatorResponse>(`/items/${id}/navigator${qs ? `?${qs}` : ""}`);
  },
  getReviewQueue: (params?: { size?: number }) => {
    const q = new URLSearchParams();
    if (params?.size) q.set("size", String(params.size));
    const qs = q.toString();
    return apiFetch<ReviewQueueResponse>(`/reviews/due${qs ? `?${qs}` : ""}`);
  },
  markReviewDone: (id: string) =>
    apiFetch<{ status: string; id: string }>(`/reviews/${id}/done`, { method: "POST" }),
  snoozeReview: (id: string, params?: { days?: number }) => {
    const q = new URLSearchParams();
    if (params?.days) q.set("days", String(params.days));
    const qs = q.toString();
    return apiFetch<{ status: string; id: string; days: number }>(`/reviews/${id}/snooze${qs ? `?${qs}` : ""}`, {
      method: "POST",
    });
  },
  getWeeklyReviewLatest: () => apiFetch<WeeklyReviewSnapshot>("/reviews/weekly/latest"),
  getPreferenceProfile: () => apiFetch<PreferenceProfile>("/settings/preference-profile"),
  getPreferenceProfileSummary: () => apiFetch<PreferenceProfileSummary>("/settings/preference-profile/summary"),
  resetPreferenceProfile: () => apiFetch<{ success: boolean }>("/settings/preference-profile", { method: "DELETE" }),
  ask: (body: {
    query: string;
    days?: number;
    unread_only?: boolean;
    limit?: number;
    source_ids?: string[];
  }) =>
    apiFetch<AskResponse>("/ask", {
      method: "POST",
      body: JSON.stringify(body),
    }).then((resp) => ({
      ...resp,
      bullets: Array.isArray(resp?.bullets) ? resp.bullets : [],
      citations: Array.isArray(resp?.citations) ? resp.citations : [],
      related_items: Array.isArray(resp?.related_items) ? resp.related_items : [],
    })),
  getAskNavigator: (body: {
    query: string;
    answer: string;
    bullets?: string[];
    citations?: AskCitation[];
    related_items?: AskCandidate[];
  }, params?: { cache_bust?: boolean }) => {
    const q = new URLSearchParams();
    if (params?.cache_bust) q.set("cache_bust", "1");
    const qs = q.toString();
    return apiFetch<AskNavigatorResponse>(`/ask/navigator${qs ? `?${qs}` : ""}`, {
      method: "POST",
      body: JSON.stringify(body),
    });
  },
  getAskInsights: (params?: { limit?: number }) => {
    const q = new URLSearchParams();
    if (params?.limit) q.set("limit", String(params.limit));
    const qs = q.toString();
    return apiFetch<{ insights: AskInsight[] }>(`/ask/insights${qs ? `?${qs}` : ""}`);
  },
  saveAskInsight: (body: {
    title: string;
    body: string;
    query?: string;
    goal_id?: string | null;
    tags?: string[];
    item_ids?: string[];
  }) =>
    apiFetch<AskInsight>("/ask/insights", {
      method: "POST",
      body: JSON.stringify(body),
    }),
  deleteAskInsight: (id: string) =>
    apiFetch<void>(`/ask/insights/${id}`, {
      method: "DELETE",
    }),
  getItemTopicTrends: (params?: { limit?: number }) => {
    const q = new URLSearchParams();
    if (params?.limit) q.set("limit", String(params.limit));
    const qs = q.toString();
    return apiFetch<{ items: TopicTrend[]; limit: number }>(`/items/topic-trends${qs ? `?${qs}` : ""}`);
  },
  getTopicPulse: (params?: { days?: number; limit?: number }) => {
    const q = new URLSearchParams();
    if (params?.days) q.set("days", String(params.days));
    if (params?.limit) q.set("limit", String(params.limit));
    const qs = q.toString();
    return apiFetch<{ days: number; limit: number; items: TopicPulseItem[] }>(`/topics/pulse${qs ? `?${qs}` : ""}`);
  },
  getItem: (id: string) => apiFetch<ItemDetail>(`/items/${id}`),
  updateItemGenre: (id: string, body: { user_genre: string | null; user_other_genre_label?: string | null }) =>
    apiFetch<ItemGenreUpdateResult>(`/items/${id}/genre`, {
      method: "PATCH",
      body: JSON.stringify(body),
    }),
  synthesizeSummaryAudio: (id: string) =>
    clientFetch<SummaryAudioSynthesisResponse>(`/summary-audio-proxy/items/${id}/synthesize`, {
      method: "POST",
    }, { apiPrefix: false }),
  saveItemNote: (id: string, body: { content: string; tags?: string[] }) =>
    apiFetch<ItemNote>(`/items/${id}/note`, {
      method: "PUT",
      body: JSON.stringify(body),
    }),
  getItemHighlights: (id: string) =>
    apiFetch<{ highlights: ItemHighlight[] }>(`/items/${id}/highlights`),
  createItemHighlight: (id: string, body: { quote_text: string; anchor_text?: string; section?: string }) =>
    apiFetch<ItemHighlight>(`/items/${id}/highlights`, {
      method: "POST",
      body: JSON.stringify(body),
    }),
  deleteItemHighlight: (id: string, highlightId: string) =>
    apiFetch<void>(`/items/${id}/highlights/${highlightId}`, {
      method: "DELETE",
    }),
  getRelatedItems: (id: string, params?: { limit?: number }) => {
    const q = new URLSearchParams();
    if (params?.limit) q.set("limit", String(params.limit));
    const qs = q.toString();
    return apiFetch<RelatedItemsResponse>(`/items/${id}/related${qs ? `?${qs}` : ""}`);
  },
  markItemRead: (id: string) =>
    apiFetch<ItemReadResult>(`/items/${id}/read`, { method: "POST" }),
  markItemUnread: (id: string) =>
    apiFetch<ItemReadResult>(`/items/${id}/read`, { method: "DELETE" }),
  markItemsReadBulk: (body: {
    item_ids?: string[];
    status?: string | null;
    source_id?: string | null;
    topic?: string | null;
    unread_only?: boolean;
    read_only?: boolean;
    favorite_only?: boolean;
    later_only?: boolean;
    older_than_days?: number | null;
  }) =>
    apiFetch<BulkMarkReadResult>("/items/mark-read-bulk", {
      method: "POST",
      body: JSON.stringify(body),
    }),
  markItemsReadByIDs: (itemIds: string[]) =>
    apiFetch<BulkMarkReadResult>("/items/mark-read-bulk", {
      method: "POST",
      body: JSON.stringify({ item_ids: itemIds }),
    }),
  markItemLater: (id: string) =>
    apiFetch<ItemLaterResult>(`/items/${id}/later`, { method: "POST" }),
  markItemsLaterBulk: (body: { item_ids: string[] }) =>
    apiFetch<BulkMarkLaterResult>("/items/mark-later-bulk", {
      method: "POST",
      body: JSON.stringify(body),
    }),
  unmarkItemLater: (id: string) =>
    apiFetch<ItemLaterResult>(`/items/${id}/later`, { method: "DELETE" }),
  deleteItem: (id: string) =>
    apiFetch<void>(`/items/${id}`, { method: "DELETE" }),
  deleteItemsBulk: (itemIds: string[]) =>
    apiFetch<BulkDeleteItemsResult>("/items/delete-bulk", {
      method: "POST",
      body: JSON.stringify({ item_ids: itemIds }),
    }),
  restoreItem: (id: string) =>
    apiFetch<ItemDetail>(`/items/${id}/restore`, { method: "POST" }),
  setItemFeedback: (id: string, body: { rating: number; is_favorite: boolean }) =>
    apiFetch<ItemFeedbackResult>(`/items/${id}/feedback`, {
      method: "PATCH",
      body: JSON.stringify(body),
    }),
  retryItem: (id: string) =>
    apiFetch<ItemRetryResult>(`/items/${id}/retry`, { method: "POST" }),
  retryItemsBulk: (itemIds: string[]) =>
    apiFetch<BulkRetryItemsResult>("/items/retry-bulk", {
      method: "POST",
      body: JSON.stringify({ item_ids: itemIds }),
    }),
  retryItemFromFacts: (id: string) =>
    apiFetch<ItemRetryResult>(`/items/${id}/retry-from-facts`, { method: "POST" }),
  retryItemsFromFactsBulk: (itemIds: string[]) =>
    apiFetch<BulkRetryItemsResult>("/items/retry-from-facts-bulk", {
      method: "POST",
      body: JSON.stringify({ item_ids: itemIds }),
    }),
  retryFailedItems: (params?: { source_id?: string }) => {
    const q = new URLSearchParams();
    if (params?.source_id) q.set("source_id", params.source_id);
    const qs = q.toString();
    return apiFetch<BulkRetryFailedResult>(`/items/retry-failed${qs ? `?${qs}` : ""}`, {
      method: "POST",
    });
  },

  // LLM Usage
  getLLMUsage: (params?: { limit?: number; month?: string }) => {
    const q = new URLSearchParams();
    if (params?.limit) q.set("limit", String(params.limit));
    if (params?.month) q.set("month", params.month);
    const qs = q.toString();
    return apiFetch<LLMUsageLog[]>(`/llm-usage${qs ? `?${qs}` : ""}`);
  },
  getLLMUsageSummary: (params?: { days?: number; month?: string }) => {
    const q = new URLSearchParams();
    if (params?.days) q.set("days", String(params.days));
    if (params?.month) q.set("month", params.month);
    const qs = q.toString();
    return apiFetch<LLMUsageDailySummary[]>(`/llm-usage/summary${qs ? `?${qs}` : ""}`);
  },
  getLLMUsageByModel: (params?: { days?: number; month?: string }) => {
    const q = new URLSearchParams();
    if (params?.days) q.set("days", String(params.days));
    if (params?.month) q.set("month", params.month);
    const qs = q.toString();
    return apiFetch<LLMUsageModelSummary[]>(`/llm-usage/by-model${qs ? `?${qs}` : ""}`);
  },
  getLLMUsageAnalysis: (params?: { days?: number }) => {
    const q = new URLSearchParams();
    if (params?.days) q.set("days", String(params.days));
    const qs = q.toString();
    return apiFetch<LLMUsageAnalysisSummary[]>(`/llm-usage/analysis${qs ? `?${qs}` : ""}`);
  },
  getLLMUsageCurrentMonthByProvider: (params?: { month?: string }) => {
    const q = new URLSearchParams();
    if (params?.month) q.set("month", params.month);
    const qs = q.toString();
    return apiFetch<LLMUsageProviderMonthSummary[]>(`/llm-usage/current-month/by-provider${qs ? `?${qs}` : ""}`);
  },
  getLLMUsageCurrentMonthByPurpose: (params?: { month?: string }) => {
    const q = new URLSearchParams();
    if (params?.month) q.set("month", params.month);
    const qs = q.toString();
    return apiFetch<LLMUsagePurposeMonthSummary[]>(`/llm-usage/current-month/by-purpose${qs ? `?${qs}` : ""}`);
  },
  getLLMExecutionCurrentMonthSummary: (params?: { days?: number; month?: string }) => {
    const q = new URLSearchParams();
    if (params?.days) q.set("days", String(params.days));
    if (params?.month) q.set("month", params.month);
    const qs = q.toString();
    return apiFetch<LLMExecutionCurrentMonthSummary[]>(`/llm-usage/current-month/execution-summary${qs ? `?${qs}` : ""}`);
  },
  getLLMValueMetricsCurrentMonth: (params?: { month?: string }) => {
    const q = new URLSearchParams();
    if (params?.month) q.set("month", params.month);
    const qs = q.toString();
    return apiFetch<LLMValueMetric[]>(`/llm-usage/current-month/value-metrics${qs ? `?${qs}` : ""}`);
  },
  getProviderModelUpdates: (params?: { days?: number; limit?: number }) => {
    const q = new URLSearchParams();
    if (params?.days) q.set("days", String(params.days));
    if (params?.limit) q.set("limit", String(params.limit));
    const qs = q.toString();
    return apiFetch<ProviderModelChangeEvent[]>(`/provider-model-updates${qs ? `?${qs}` : ""}`);
  },
  getProviderModelSnapshots: (params?: { providers?: string[]; q?: string; limit?: number; offset?: number }) => {
    const q = new URLSearchParams();
    for (const provider of params?.providers ?? []) {
      if (provider) q.append("provider", provider);
    }
    if (params?.q) q.set("q", params.q);
    if (params?.limit) q.set("limit", String(params.limit));
    if (params?.offset) q.set("offset", String(params.offset));
    const qs = q.toString();
    return apiFetch<ProviderModelSnapshotListResponse>(`/provider-model-snapshots${qs ? `?${qs}` : ""}`);
  },
  syncProviderModelSnapshots: () => {
    return apiFetch<ProviderModelSnapshotSyncSummary>("/provider-model-snapshots/sync", {
      method: "POST",
    });
  },
  getAudioBriefings: (params?: { limit?: number; tab?: "published" | "archived" | "pending" | "storage" }) => {
    const q = new URLSearchParams();
    if (params?.limit) q.set("limit", String(params.limit));
    if (params?.tab) q.set("tab", params.tab);
    const qs = q.toString();
    return apiFetch<{ items: AudioBriefingJob[] }>(`/audio-briefings${qs ? `?${qs}` : ""}`);
  },
  getAudioBriefing: (id: string) => apiFetch<AudioBriefingDetailResponse>(`/audio-briefings/${id}`),
  generateAudioBriefing: () =>
    apiFetch<AudioBriefingDetailResponse>("/audio-briefings/generate", {
      method: "POST",
    }),
  resumeAudioBriefing: (id: string) =>
    apiFetch<AudioBriefingDetailResponse>(`/audio-briefings/${id}/resume`, {
      method: "POST",
    }),
  archiveAudioBriefing: (id: string) =>
    apiFetch<AudioBriefingDetailResponse>(`/audio-briefings/${id}/archive`, {
      method: "POST",
    }),
  unarchiveAudioBriefing: (id: string) =>
    apiFetch<AudioBriefingDetailResponse>(`/audio-briefings/${id}/unarchive`, {
      method: "POST",
    }),
  deleteAudioBriefing: (id: string) =>
    apiFetch<void>(`/audio-briefings/${id}`, {
      method: "DELETE",
    }),
  startAudioBriefingConcat: (id: string) =>
    apiFetch<AudioBriefingDetailResponse>(`/audio-briefings/${id}/start-concat`, {
      method: "POST",
    }),
  startAudioBriefingVoicing: (id: string) =>
    apiFetch<AudioBriefingDetailResponse>(`/audio-briefings/${id}/start-voicing`, {
      method: "POST",
    }),
  getAINavigatorBriefs: (params?: { slot?: string; limit?: number }) => {
    const q = new URLSearchParams();
    if (params?.slot) q.set("slot", params.slot);
    if (params?.limit) q.set("limit", String(params.limit));
    const qs = q.toString();
    return apiFetch<AINavigatorBriefListResponse>(`/ai-navigator-briefs${qs ? `?${qs}` : ""}`);
  },
  getAINavigatorBrief: (id: string) => apiFetch<AINavigatorBriefDetailResponse>(`/ai-navigator-briefs/${id}`),
  generateAINavigatorBrief: () =>
    apiFetch<AINavigatorBriefDetailResponse>("/ai-navigator-briefs/generate", {
      method: "POST",
    }),
  deleteAINavigatorBrief: (id: string) =>
    apiFetch<void>(`/ai-navigator-briefs/${id}`, {
      method: "DELETE",
    }),
  appendAINavigatorBriefToSummaryAudioQueue: (id: string) =>
    apiFetch<AINavigatorBriefSummaryAudioQueueResponse>(`/ai-navigator-briefs/${id}/summary-audio-queue`, {
      method: "POST",
    }),
  getLatestPlaybackSessions: () =>
    apiFetch<LatestPlaybackSessionsResponse>("/playback-sessions/latest"),
  getPlaybackSessions: (params?: { mode?: PlaybackMode; status?: PlaybackSessionStatus; limit?: number }) => {
    const q = new URLSearchParams();
    if (params?.mode) q.set("mode", params.mode);
    if (params?.status) q.set("status", params.status);
    if (params?.limit) q.set("limit", String(params.limit));
    const qs = q.toString();
    return apiFetch<PlaybackSessionsResponse>(`/playback-sessions${qs ? `?${qs}` : ""}`);
  },
  createPlaybackSession: (body: CreatePlaybackSessionRequest) =>
    apiFetch<PlaybackSession>("/playback-sessions", {
      method: "POST",
      body: JSON.stringify(body),
    }),
  updatePlaybackSession: (id: string, body: UpdatePlaybackSessionRequest) =>
    apiFetch<PlaybackSession>(`/playback-sessions/${id}`, {
      method: "PATCH",
      body: JSON.stringify(body),
    }),
  completePlaybackSession: (id: string, body: UpdatePlaybackSessionRequest) =>
    apiFetch<PlaybackSession>(`/playback-sessions/${id}/complete`, {
      method: "POST",
      body: JSON.stringify(body),
    }),
  interruptPlaybackSession: (id: string, body: UpdatePlaybackSessionRequest) =>
    apiFetch<PlaybackSession>(`/playback-sessions/${id}/interrupt`, {
      method: "POST",
      body: JSON.stringify(body),
    }),

  // Settings
  getSettings: () => apiFetch<UserSettings>("/settings"),
  getNavigatorPersonas: () => apiFetch<Record<string, NavigatorPersonaDefinition>>("/settings/navigator-personas"),
  updateAudioBriefingSettings: (body: AudioBriefingSettings) =>
    apiFetch<{ user_id: string; audio_briefing: AudioBriefingSettings }>("/settings/audio-briefing", {
      method: "PATCH",
      body: JSON.stringify(body),
    }),
  listAudioBriefingPresets: () =>
    apiFetch<{ presets: AudioBriefingPreset[] }>("/audio-briefing-presets").then((resp) => resp.presets ?? []),
  createAudioBriefingPreset: (body: SaveAudioBriefingPresetRequest) =>
    apiFetch<{ preset: AudioBriefingPreset }>("/audio-briefing-presets", {
      method: "POST",
      body: JSON.stringify(body),
    }).then((resp) => resp.preset),
  updateAudioBriefingPreset: (id: string, body: SaveAudioBriefingPresetRequest) =>
    apiFetch<{ preset: AudioBriefingPreset }>(`/audio-briefing-presets/${id}`, {
      method: "PUT",
      body: JSON.stringify(body),
    }).then((resp) => resp.preset),
  deleteAudioBriefingPreset: (id: string) =>
    apiFetch<void>(`/audio-briefing-presets/${id}`, {
      method: "DELETE",
    }),
  getSummaryAudioSettings: () =>
    apiFetch<{ user_id: string; summary_audio: SummaryAudioVoiceSettings | null }>("/settings/summary-audio"),
  updatePodcastSettings: (body: PodcastSettings) =>
    apiFetch<{ user_id: string; podcast: PodcastSettings }>("/settings/podcast", {
      method: "PATCH",
      body: JSON.stringify(body),
    }),
  uploadPodcastArtwork: (body: { content_type: string; content_base64: string }) =>
    apiFetch<{ user_id: string; artwork_url: string | null }>("/settings/podcast-artwork", {
      method: "POST",
      body: JSON.stringify(body),
    }),
  updateAudioBriefingPersonaVoices: (voices: AudioBriefingPersonaVoice[]) =>
    apiFetch<{ user_id: string; audio_briefing_persona_voices: AudioBriefingPersonaVoice[] }>("/settings/audio-briefing/persona-voices", {
      method: "PATCH",
      body: JSON.stringify({ voices }),
    }),
  updateSummaryAudioSettings: (body: SummaryAudioVoiceSettings) =>
    apiFetch<{ user_id: string; summary_audio: SummaryAudioVoiceSettings | null }>("/settings/summary-audio", {
      method: "PUT",
      body: JSON.stringify(body),
    }),
  updateNotificationPriority: (body: NotificationPriorityRule) =>
    apiFetch<{ user_id: string; notification_priority: NotificationPriorityRule }>("/settings/notification-priority", {
      method: "PATCH",
      body: JSON.stringify(body),
    }),
  getLLMCatalog: () => apiFetch<LLMCatalog>("/settings/llm-catalog"),
  getUIFontCatalog: () => apiFetch<UIFontCatalogResponse>("/settings/ui-font-catalog"),
  updateSettings: (body: {
    monthly_budget_usd: number | null;
    budget_alert_enabled: boolean;
    budget_alert_threshold_pct: number;
    digest_email_enabled: boolean;
  }) =>
    apiFetch<UserSettings>("/settings", {
      method: "PATCH",
      body: JSON.stringify(body),
    }),
  updateUIFontSettings: (body: {
    ui_font_sans_key: string;
    ui_font_serif_key: string;
  }) =>
    apiFetch<{ user_id: string; ui_font_sans_key: string; ui_font_serif_key: string }>("/settings/ui-fonts", {
      method: "PATCH",
      body: JSON.stringify(body),
    }),
  updateReadingPlanSettings: (body: Pick<UserReadingPlanSettings, "window" | "size" | "diversify_topics">) =>
    apiFetch<{ user_id: string; reading_plan: UserReadingPlanSettings }>("/settings/reading-plan", {
      method: "PATCH",
      body: JSON.stringify(body),
    }),
  getReadingGoals: () =>
    apiFetch<{ active: ReadingGoal[]; archived: ReadingGoal[] }>("/settings/reading-goals"),
  createReadingGoal: (body: {
    title: string;
    description: string;
    priority: number;
    due_date?: string | null;
  }) =>
    apiFetch<ReadingGoal>("/settings/reading-goals", {
      method: "POST",
      body: JSON.stringify(body),
    }),
  updateReadingGoal: (id: string, body: {
    title: string;
    description: string;
    priority: number;
    due_date?: string | null;
  }) =>
    apiFetch<ReadingGoal>(`/settings/reading-goals/${id}`, {
      method: "PATCH",
      body: JSON.stringify(body),
    }),
  archiveReadingGoal: (id: string) =>
    apiFetch<ReadingGoal>(`/settings/reading-goals/${id}/archive`, {
      method: "POST",
    }),
  restoreReadingGoal: (id: string) =>
    apiFetch<ReadingGoal>(`/settings/reading-goals/${id}/restore`, {
      method: "POST",
    }),
  deleteReadingGoal: (id: string) =>
    apiFetch<void>(`/settings/reading-goals/${id}`, {
      method: "DELETE",
    }),
  updateObsidianExport: (body: {
    enabled: boolean;
    github_repo_owner?: string | null;
    github_repo_name?: string | null;
    github_repo_branch?: string | null;
    vault_root_path?: string | null;
    keyword_link_mode?: string | null;
  }) =>
    apiFetch<{
      user_id: string;
      obsidian_export: NonNullable<UserSettings["obsidian_export"]>;
    }>("/settings/obsidian-export", {
      method: "PATCH",
      body: JSON.stringify(body),
    }),
  runObsidianExportNow: () =>
    apiFetch<ObsidianExportRunResult>("/settings/obsidian-export/run", {
      method: "POST",
    }),
  updateLLMModelSettings: (body: {
    facts?: string | null;
    facts_secondary?: string | null;
    facts_secondary_rate_percent?: number;
    facts_fallback?: string | null;
    summary?: string | null;
    summary_secondary?: string | null;
    summary_secondary_rate_percent?: number;
    summary_fallback?: string | null;
    digest_cluster?: string | null;
    digest?: string | null;
    ask?: string | null;
    source_suggestion?: string | null;
    embedding?: string | null;
    facts_check?: string | null;
    facts_check_fallback?: string | null;
    faithfulness_check?: string | null;
    faithfulness_check_fallback?: string | null;
    navigator_enabled?: boolean;
    ai_navigator_brief_enabled?: boolean;
    navigator_persona_mode?: string | null;
    navigator_persona?: string | null;
    navigator?: string | null;
    navigator_fallback?: string | null;
    ai_navigator_brief?: string | null;
    ai_navigator_brief_fallback?: string | null;
    audio_briefing_script?: string | null;
    audio_briefing_script_fallback?: string | null;
    tts_markup_preprocess_model?: string | null;
  }) =>
    apiFetch<{ user_id: string; llm_models: UserSettings["llm_models"] }>("/settings/llm-models", {
      method: "PATCH",
      body: JSON.stringify(body),
    }),
  setAnthropicApiKey: (apiKey: string) =>
    apiFetch<{ user_id: string; has_anthropic_api_key: boolean; anthropic_api_key_last4: string | null }>(
      "/settings/anthropic-key",
      { method: "POST", body: JSON.stringify({ api_key: apiKey }) }
    ),
  deleteAnthropicApiKey: () =>
    apiFetch<{ user_id: string; has_anthropic_api_key: boolean; anthropic_api_key_last4: string | null }>(
      "/settings/anthropic-key",
      { method: "DELETE" }
    ),
  setOpenAIApiKey: (apiKey: string) =>
    apiFetch<{ user_id: string; has_openai_api_key: boolean; openai_api_key_last4: string | null }>(
      "/settings/openai-key",
      { method: "POST", body: JSON.stringify({ api_key: apiKey }) }
    ),
  deleteOpenAIApiKey: () =>
    apiFetch<{ user_id: string; has_openai_api_key: boolean; openai_api_key_last4: string | null }>(
      "/settings/openai-key",
      { method: "DELETE" }
    ),
  setGoogleApiKey: (apiKey: string) =>
    apiFetch<{ user_id: string; has_google_api_key: boolean; google_api_key_last4: string | null }>(
      "/settings/google-key",
      { method: "POST", body: JSON.stringify({ api_key: apiKey }) }
    ),
  deleteGoogleApiKey: () =>
    apiFetch<{ user_id: string; has_google_api_key: boolean; google_api_key_last4: string | null }>(
      "/settings/google-key",
      { method: "DELETE" }
    ),
  setGroqApiKey: (apiKey: string) =>
    apiFetch<{ user_id: string; has_groq_api_key: boolean; groq_api_key_last4: string | null }>(
      "/settings/groq-key",
      { method: "POST", body: JSON.stringify({ api_key: apiKey }) }
    ),
  deleteGroqApiKey: () =>
    apiFetch<{ user_id: string; has_groq_api_key: boolean; groq_api_key_last4: string | null }>(
      "/settings/groq-key",
      { method: "DELETE" }
    ),
  setDeepSeekApiKey: (apiKey: string) =>
    apiFetch<{ user_id: string; has_deepseek_api_key: boolean; deepseek_api_key_last4: string | null }>(
      "/settings/deepseek-key",
      { method: "POST", body: JSON.stringify({ api_key: apiKey }) }
    ),
  deleteDeepSeekApiKey: () =>
    apiFetch<{ user_id: string; has_deepseek_api_key: boolean; deepseek_api_key_last4: string | null }>(
      "/settings/deepseek-key",
      { method: "DELETE" }
    ),
  setAlibabaApiKey: (apiKey: string) =>
    apiFetch<{ user_id: string; has_alibaba_api_key: boolean; alibaba_api_key_last4: string | null }>(
      "/settings/alibaba-key",
      { method: "POST", body: JSON.stringify({ api_key: apiKey }) }
    ),
  deleteAlibabaApiKey: () =>
    apiFetch<{ user_id: string; has_alibaba_api_key: boolean; alibaba_api_key_last4: string | null }>(
      "/settings/alibaba-key",
      { method: "DELETE" }
    ),
  setMistralApiKey: (apiKey: string) =>
    apiFetch<{ user_id: string; has_mistral_api_key: boolean; mistral_api_key_last4: string | null }>(
      "/settings/mistral-key",
      { method: "POST", body: JSON.stringify({ api_key: apiKey }) }
    ),
  deleteMistralApiKey: () =>
    apiFetch<{ user_id: string; has_mistral_api_key: boolean; mistral_api_key_last4: string | null }>(
      "/settings/mistral-key",
      { method: "DELETE" }
    ),
  setCerebrasApiKey: (apiKey: string) =>
    apiFetch<{ user_id: string; has_cerebras_api_key: boolean; cerebras_api_key_last4: string | null }>(
      "/settings/cerebras-key",
      { method: "POST", body: JSON.stringify({ api_key: apiKey }) }
    ),
  deleteCerebrasApiKey: () =>
    apiFetch<{ user_id: string; has_cerebras_api_key: boolean; cerebras_api_key_last4: string | null }>(
      "/settings/cerebras-key",
      { method: "DELETE" }
    ),
  setMiniMaxApiKey: (apiKey: string) =>
    apiFetch<{ user_id: string; has_minimax_api_key: boolean; minimax_api_key_last4: string | null }>(
      "/settings/minimax-key",
      { method: "POST", body: JSON.stringify({ api_key: apiKey }) }
    ),
  deleteMiniMaxApiKey: () =>
    apiFetch<{ user_id: string; has_minimax_api_key: boolean; minimax_api_key_last4: string | null }>(
      "/settings/minimax-key",
      { method: "DELETE" }
    ),
  setXiaomiMiMoTokenPlanApiKey: (apiKey: string) =>
    apiFetch<{ user_id: string; has_xiaomi_mimo_token_plan_api_key: boolean; xiaomi_mimo_token_plan_api_key_last4: string | null }>(
      "/settings/xiaomi-mimo-token-plan-key",
      { method: "POST", body: JSON.stringify({ api_key: apiKey }) }
    ),
  deleteXiaomiMiMoTokenPlanApiKey: () =>
    apiFetch<{ user_id: string; has_xiaomi_mimo_token_plan_api_key: boolean; xiaomi_mimo_token_plan_api_key_last4: string | null }>(
      "/settings/xiaomi-mimo-token-plan-key",
      { method: "DELETE" }
    ),
  setMoonshotApiKey: (apiKey: string) =>
    apiFetch<{ user_id: string; has_moonshot_api_key: boolean; moonshot_api_key_last4: string | null }>(
      "/settings/moonshot-key",
      { method: "POST", body: JSON.stringify({ api_key: apiKey }) }
    ),
  deleteMoonshotApiKey: () =>
    apiFetch<{ user_id: string; has_moonshot_api_key: boolean; moonshot_api_key_last4: string | null }>(
      "/settings/moonshot-key",
      { method: "DELETE" }
    ),
  setXAIApiKey: (apiKey: string) =>
    apiFetch<{ user_id: string; has_xai_api_key: boolean; xai_api_key_last4: string | null }>(
      "/settings/xai-key",
      { method: "POST", body: JSON.stringify({ api_key: apiKey }) }
    ),
  deleteXAIApiKey: () =>
    apiFetch<{ user_id: string; has_xai_api_key: boolean; xai_api_key_last4: string | null }>(
      "/settings/xai-key",
      { method: "DELETE" }
    ),
  setZAIApiKey: (apiKey: string) =>
    apiFetch<{ user_id: string; has_zai_api_key: boolean; zai_api_key_last4: string | null }>(
      "/settings/zai-key",
      { method: "POST", body: JSON.stringify({ api_key: apiKey }) }
    ),
  deleteZAIApiKey: () =>
    apiFetch<{ user_id: string; has_zai_api_key: boolean; zai_api_key_last4: string | null }>(
      "/settings/zai-key",
      { method: "DELETE" }
    ),
  setFireworksApiKey: (apiKey: string) =>
    apiFetch<{ user_id: string; has_fireworks_api_key: boolean; fireworks_api_key_last4: string | null }>(
      "/settings/fireworks-key",
      { method: "POST", body: JSON.stringify({ api_key: apiKey }) }
    ),
  setTogetherApiKey: (apiKey: string) =>
    apiFetch<{ user_id: string; has_together_api_key: boolean; together_api_key_last4: string | null }>(
      "/settings/together-key",
      { method: "POST", body: JSON.stringify({ api_key: apiKey }) }
    ),
  deleteFireworksApiKey: () =>
    apiFetch<{ user_id: string; has_fireworks_api_key: boolean; fireworks_api_key_last4: string | null }>(
      "/settings/fireworks-key",
      { method: "DELETE" }
    ),
  deleteTogetherApiKey: () =>
    apiFetch<{ user_id: string; has_together_api_key: boolean; together_api_key_last4: string | null }>(
      "/settings/together-key",
      { method: "DELETE" }
    ),
  setPoeApiKey: (apiKey: string) =>
    apiFetch<{ user_id: string; has_poe_api_key: boolean; poe_api_key_last4: string | null }>(
      "/settings/poe-key",
      { method: "POST", body: JSON.stringify({ api_key: apiKey }) }
    ),
  deletePoeApiKey: () =>
    apiFetch<{ user_id: string; has_poe_api_key: boolean; poe_api_key_last4: string | null }>(
      "/settings/poe-key",
      { method: "DELETE" }
    ),
  setSiliconFlowApiKey: (apiKey: string) =>
    apiFetch<{ user_id: string; has_siliconflow_api_key: boolean; siliconflow_api_key_last4: string | null }>(
      "/settings/siliconflow-key",
      { method: "POST", body: JSON.stringify({ api_key: apiKey }) }
    ),
  deleteSiliconFlowApiKey: () =>
    apiFetch<{ user_id: string; has_siliconflow_api_key: boolean; siliconflow_api_key_last4: string | null }>(
      "/settings/siliconflow-key",
      { method: "DELETE" }
    ),
  setAzureSpeechConfig: (apiKey: string, region: string) =>
    apiFetch<{ user_id: string; has_azure_speech_api_key: boolean; azure_speech_api_key_last4: string | null; azure_speech_region: string | null }>(
      "/settings/azure-speech-config",
      { method: "POST", body: JSON.stringify({ api_key: apiKey, region }) }
    ),
  deleteAzureSpeechConfig: () =>
    apiFetch<{ user_id: string; has_azure_speech_api_key: boolean; azure_speech_api_key_last4: string | null; azure_speech_region: string | null }>(
      "/settings/azure-speech-config",
      { method: "DELETE" }
    ),
  setOpenRouterApiKey: (apiKey: string) =>
    apiFetch<{ user_id: string; has_openrouter_api_key: boolean; openrouter_api_key_last4: string | null }>(
      "/settings/openrouter-key",
      { method: "POST", body: JSON.stringify({ api_key: apiKey }) }
    ),
  deleteOpenRouterApiKey: () =>
    apiFetch<{ user_id: string; has_openrouter_api_key: boolean; openrouter_api_key_last4: string | null }>(
      "/settings/openrouter-key",
      { method: "DELETE" }
    ),
  setDeepInfraApiKey: (apiKey: string) =>
    apiFetch<{ user_id: string; has_deepinfra_api_key: boolean; deepinfra_api_key_last4: string | null }>(
      "/settings/deepinfra-key",
      { method: "POST", body: JSON.stringify({ api_key: apiKey }) }
    ),
  deleteDeepInfraApiKey: () =>
    apiFetch<{ user_id: string; has_deepinfra_api_key: boolean; deepinfra_api_key_last4: string | null }>(
      "/settings/deepinfra-key",
      { method: "DELETE" }
    ),
  setFeatherlessApiKey: (apiKey: string) =>
    apiFetch<{ user_id: string; has_featherless_api_key: boolean; featherless_api_key_last4: string | null }>(
      "/settings/featherless-key",
      { method: "POST", body: JSON.stringify({ api_key: apiKey }) }
    ),
  deleteFeatherlessApiKey: () =>
    apiFetch<{ user_id: string; has_featherless_api_key: boolean; featherless_api_key_last4: string | null }>(
      "/settings/featherless-key",
      { method: "DELETE" }
    ),
  setAivisApiKey: (apiKey: string) =>
    apiFetch<{ user_id: string; has_aivis_api_key: boolean; aivis_api_key_last4: string | null }>(
      "/settings/aivis-key",
      { method: "POST", body: JSON.stringify({ api_key: apiKey }) }
    ),
  deleteAivisApiKey: () =>
    apiFetch<{ user_id: string; has_aivis_api_key: boolean; aivis_api_key_last4: string | null }>(
      "/settings/aivis-key",
      { method: "DELETE" }
    ),
  setElevenLabsApiKey: (apiKey: string) =>
    apiFetch<{ user_id: string; has_elevenlabs_api_key: boolean; elevenlabs_api_key_last4: string | null }>(
      "/settings/elevenlabs-key",
      { method: "POST", body: JSON.stringify({ api_key: apiKey }) }
    ),
  deleteElevenLabsApiKey: () =>
    apiFetch<{ user_id: string; has_elevenlabs_api_key: boolean; elevenlabs_api_key_last4: string | null }>(
      "/settings/elevenlabs-key",
      { method: "DELETE" }
    ),
  setFishApiKey: (apiKey: string) =>
    apiFetch<{ user_id: string; has_fish_api_key: boolean; fish_api_key_last4: string | null }>(
      "/settings/fish-key",
      { method: "POST", body: JSON.stringify({ api_key: apiKey }) }
    ),
  deleteFishApiKey: () =>
    apiFetch<{ user_id: string; has_fish_api_key: boolean; fish_api_key_last4: string | null }>(
      "/settings/fish-key",
      { method: "DELETE" }
    ),
  getAivisUserDictionaries: () =>
    apiFetch<AivisUserDictionariesResponse>("/settings/aivis-user-dictionaries"),
  getPromptAdminCapabilities: () =>
    apiFetch<PromptAdminCapabilities>("/settings/prompt-admin/capabilities"),
  getPromptTemplates: () =>
    apiFetch<{ templates: PromptTemplate[] }>("/settings/prompt-admin/templates"),
  getPromptTemplateDetail: (id: string) =>
    apiFetch<PromptTemplateDetailResponse>(`/settings/prompt-admin/templates/${id}`),
  createPromptTemplateVersion: (
    id: string,
    body: {
      label: string;
      system_instruction: string;
      prompt_text: string;
      fallback_prompt_text?: string;
      variables_schema?: Record<string, unknown> | string | null;
      notes?: string;
    }
  ) =>
    apiFetch<PromptTemplateVersion>(`/settings/prompt-admin/templates/${id}/versions`, {
      method: "POST",
      body: JSON.stringify(body),
    }),
  activatePromptTemplateVersion: (id: string, versionId: string | null) =>
    apiFetch<{ ok: boolean }>(`/settings/prompt-admin/templates/${id}/activate`, {
      method: "POST",
      body: JSON.stringify({ version_id: versionId }),
    }),
  createPromptExperiment: (body: {
    template_id: string;
    name: string;
    status: string;
    assignment_unit: string;
    started_at?: string | null;
    ended_at?: string | null;
    arms: { version_id: string; weight: number }[];
  }) =>
    apiFetch<{ experiment: PromptExperiment; arms: PromptExperimentArm[] }>("/settings/prompt-admin/experiments", {
      method: "POST",
      body: JSON.stringify(body),
    }),
  updatePromptExperiment: (
    id: string,
    body: {
      status?: string;
      started_at?: string | null;
      ended_at?: string | null;
      arms?: { version_id: string; weight: number }[];
    }
  ) =>
    apiFetch<{ experiment: PromptExperiment; arms: PromptExperimentArm[] }>(`/settings/prompt-admin/experiments/${id}`, {
      method: "PATCH",
      body: JSON.stringify(body),
    }),
  setAivisUserDictionary: (uuid: string) =>
    apiFetch<{ user_id: string; aivis_user_dictionary_uuid: string | null }>(
      "/settings/aivis-user-dictionary",
      { method: "POST", body: JSON.stringify({ aivis_user_dictionary_uuid: uuid }) }
    ),
  deleteAivisUserDictionary: () =>
    apiFetch<{ user_id: string; aivis_user_dictionary_uuid: string | null }>(
      "/settings/aivis-user-dictionary",
      { method: "DELETE" }
    ),
  getOpenRouterModels: () =>
    apiFetch<OpenRouterModelsResponse>("/openrouter-models"),
  getOpenRouterSyncStatus: () =>
    apiFetch<OpenRouterSyncStatusResponse>("/openrouter-models/status"),
  syncOpenRouterModels: () =>
    apiFetch<OpenRouterModelsResponse>("/openrouter-models/sync", { method: "POST" }),
  getFeatherlessModels: () =>
    apiFetch<FeatherlessModelsResponse>("/featherless-models"),
  getFeatherlessSyncStatus: () =>
    apiFetch<FeatherlessSyncStatusResponse>("/featherless-models/status"),
  syncFeatherlessModels: () =>
    apiFetch<FeatherlessModelsResponse>("/featherless-models/sync", { method: "POST" }),
  getDeepInfraModels: () =>
    apiFetch<DeepInfraModelsResponse>("/deepinfra-models"),
  getDeepInfraSyncStatus: () =>
    apiFetch<DeepInfraSyncStatusResponse>("/deepinfra-models/status"),
  syncDeepInfraModels: () =>
    apiFetch<DeepInfraModelsResponse>("/deepinfra-models/sync", { method: "POST" }),
  setOpenRouterStructuredOutputOverride: (modelId: string, allowStructuredOutput: boolean) =>
    apiFetch<{ model_id: string; override_enabled: boolean; allow_structured_output: boolean }>(
      "/openrouter-models/overrides/structured-output",
      { method: "PUT", body: JSON.stringify({ model_id: modelId, allow_structured_output: allowStructuredOutput }) }
    ),
  getPoeModels: () =>
    apiFetch<PoeModelsResponse>("/poe-models"),
  getPoeUsage: (range = "30d", entriesLimit = 100) =>
    apiFetch<PoeUsageResponse>(`/poe-models/usage?range=${encodeURIComponent(range)}&entries_limit=${entriesLimit}`),
  syncPoeUsage: () =>
    apiFetch<{ run: PoeUsageResponse["last_sync_run"] }>("/poe-models/usage/sync", { method: "POST" }),
  getPoeSyncStatus: () =>
    apiFetch<PoeSyncStatusResponse>("/poe-models/status"),
  syncPoeModels: () =>
    apiFetch<PoeModelsResponse>("/poe-models/sync", { method: "POST" }),
  getAivisModels: () =>
    apiFetch<AivisModelsResponse>("/aivis-models"),
  getAivisSyncStatus: () =>
    apiFetch<AivisSyncStatusResponse>("/aivis-models/status"),
  syncAivisModels: () =>
    apiFetch<AivisModelsResponse>("/aivis-models/sync", { method: "POST" }),
  browseFishModels: (params?: { sort?: FishBrowseSort; query?: string; page?: number; pageSize?: number }) => {
    const search = new URLSearchParams();
    if (params?.sort) search.set("sort", params.sort);
    if (params?.query?.trim()) search.set("query", params.query.trim());
    if (params?.page) search.set("page", String(params.page));
    if (params?.pageSize) search.set("page_size", String(params.pageSize));
    const qs = search.toString();
    return apiFetch<FishBrowseResponse>(`/fish-models/browse${qs ? `?${qs}` : ""}`);
  },
  getXAIVoices: () =>
    apiFetch<XAIVoicesResponse>("/xai-voices"),
  getXAIVoiceSyncStatus: () =>
    apiFetch<{ run: XAIVoiceSyncRun | null }>("/xai-voices/status"),
  syncXAIVoices: () =>
    apiFetch<XAIVoicesResponse>("/xai-voices/sync", { method: "POST" }),
  getOpenAITTSVoices: () =>
    apiFetch<OpenAITTSVoicesResponse>("/openai-tts-voices"),
  getOpenAITTSSyncStatus: () =>
    apiFetch<{ run: OpenAITTSVoiceSyncRun | null }>("/openai-tts-voices/status"),
  syncOpenAITTSVoices: () =>
    apiFetch<OpenAITTSVoicesResponse>("/openai-tts-voices/sync", { method: "POST" }),
  getElevenLabsVoices: () =>
    apiFetch<ElevenLabsVoicesResponse>("/elevenlabs-voices"),
  getAzureSpeechVoices: () =>
    apiFetch<AzureSpeechVoicesResponse>("/azure-speech-voices"),
  getGeminiTTSVoices: () =>
    apiFetch<GeminiTTSVoicesResponse>("/gemini-tts-voices"),
  deleteInoreaderOAuth: () =>
    apiFetch<{ user_id: string; has_inoreader_oauth: boolean; inoreader_token_expires_at: string | null }>(
      "/settings/inoreader-oauth",
      { method: "DELETE" }
    ),

  // Digests
  getDigests: () => apiFetch<Digest[]>("/digests"),
  getDigest: (id: string) => apiFetch<DigestDetail>(`/digests/${id}`),
  getLatestDigest: () => apiFetch<DigestDetail>("/digests/latest"),
};
