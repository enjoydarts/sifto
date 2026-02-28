// API client — proxied via Next.js rewrites to Go API
// Browser calls /api/* → Next.js rewrites → http://localhost:8080/api/*

export interface Source {
  id: string;
  user_id: string;
  url: string;
  type: "rss" | "manual";
  title: string | null;
  enabled: boolean;
  last_fetched_at: string | null;
  created_at: string;
  updated_at: string;
}

export interface SourceSuggestion {
  url: string;
  title: string | null;
  reasons: string[];
  matched_topics?: string[];
  ai_reason?: string | null;
  ai_confidence?: number | null;
  seed_source_ids: string[];
}

export interface RecommendedSource {
  source_id: string;
  url: string;
  title: string | null;
  affinity_score: number;
  read_count_30d: number;
  feedback_count_30d: number;
  favorite_count_30d: number;
  last_item_at?: string | null;
}

export interface SourceHealth {
  source_id: string;
  total_items: number;
  failed_items: number;
  summarized_items: number;
  failure_rate: number;
  last_item_at?: string | null;
  last_fetched_at?: string | null;
  status: "ok" | "stale" | "error" | "new" | "disabled" | string;
}

export interface Item {
  id: string;
  source_id: string;
  url: string;
  title: string | null;
  translated_title?: string | null;
  thumbnail_url?: string | null;
  content_text: string | null;
  status: "new" | "fetched" | "facts_extracted" | "summarized" | "failed";
  is_read: boolean;
  is_favorite: boolean;
  feedback_rating: -1 | 0 | 1 | number;
  summary_score?: number | null;
  summary_topics?: string[];
  published_at: string | null;
  fetched_at: string | null;
  created_at: string;
  updated_at: string;
}

export interface ItemFacts {
  id: string;
  item_id: string;
  facts: string[];
  extracted_at: string;
}

export interface ItemSummary {
  id: string;
  item_id: string;
  summary: string;
  topics: string[];
  translated_title?: string | null;
  score: number | null;
  score_breakdown?: {
    importance?: number;
    novelty?: number;
    actionability?: number;
    reliability?: number;
    relevance?: number;
  } | null;
  score_reason?: string | null;
  score_policy_version?: string | null;
  summarized_at: string;
}

export interface ItemSummaryLLM {
  provider: string;
  model: string;
  pricing_source: string;
  created_at: string;
}

export interface ItemFeedback {
  user_id: string;
  item_id: string;
  rating: -1 | 0 | 1 | number;
  is_favorite: boolean;
  updated_at: string;
}

export interface ItemDetail extends Item {
  processing_error?: string | null;
  facts: ItemFacts | null;
  summary: ItemSummary | null;
  summary_llm?: ItemSummaryLLM | null;
  feedback?: ItemFeedback | null;
}

export interface RelatedItem {
  id: string;
  source_id: string;
  url: string;
  title: string | null;
  summary?: string | null;
  topics?: string[];
  summary_score?: number | null;
  similarity: number;
  reason?: string | null;
  reason_topics?: string[];
  published_at?: string | null;
  created_at: string;
}

export interface RelatedItemsResponse {
  item_id: string;
  limit: number;
  items: RelatedItem[];
  clusters?: {
    id: string;
    label: string;
    size: number;
    max_similarity: number;
    representative: RelatedItem;
    items: RelatedItem[];
  }[];
}

export interface ItemRetryResult {
  item_id: string;
  status: "queued";
}

export interface ItemReadResult {
  item_id: string;
  is_read: boolean;
}

export type ItemFeedbackResult = ItemFeedback;

export interface ItemListResponse {
  items: Item[];
  page: number;
  page_size: number;
  total: number;
  has_next: boolean;
  sort: "newest" | "score" | string;
  status?: string | null;
  source_id?: string | null;
}

export interface ReadingPlanResponse {
  items: Item[];
  window: "24h" | "today_jst" | "7d" | string;
  size: number;
  diversify_topics: boolean;
  exclude_read: boolean;
  source_pool_count: number;
  topics: { topic: string; count: number; max_score?: number | null }[];
  clusters?: {
    id: string;
    label: string;
    size: number;
    max_similarity: number;
    representative: Item;
    items: Item[];
  }[];
}

export interface FocusQueueResponse {
  items: Item[];
  window: "24h" | "today_jst" | "7d" | string;
  size: number;
  completed: number;
  remaining: number;
  total: number;
  source_pool: number;
  diversify_topics: boolean;
}

export interface ItemStats {
  total: number;
  read: number;
  unread: number;
  by_status: Record<string, number>;
}

export interface TopicTrend {
  topic: string;
  count_24h: number;
  count_prev_24h: number;
  delta: number;
  max_score_24h?: number | null;
}

export interface TopicPulsePoint {
  date: string;
  count: number;
  max_score?: number | null;
}

export interface TopicPulseItem {
  topic: string;
  total: number;
  delta: number;
  max_score?: number | null;
  points: TopicPulsePoint[];
}

export interface BulkRetryFailedResult {
  status: "queued";
  source_id: string | null;
  matched: number;
  queued_count: number;
  failed_count: number;
}

export interface Digest {
  id: string;
  user_id: string;
  digest_date: string;
  email_subject: string | null;
  email_body: string | null;
  send_status?: string | null;
  send_error?: string | null;
  send_tried_at?: string | null;
  sent_at: string | null;
  created_at: string;
}

export interface DigestItemDetail {
  rank: number;
  item: Item;
  summary: ItemSummary;
  facts?: string[];
}

export interface DigestClusterDraft {
  id: string;
  digest_id: string;
  cluster_key: string;
  cluster_label: string;
  rank: number;
  item_count: number;
  topics: string[];
  max_score?: number | null;
  draft_summary: string;
  created_at: string;
  updated_at: string;
}

export interface DigestDetail extends Digest {
  items: DigestItemDetail[];
  cluster_drafts?: DigestClusterDraft[];
}

export interface LLMUsageLog {
  id: string;
  user_id?: string | null;
  source_id?: string | null;
  item_id?: string | null;
  digest_id?: string | null;
  provider: string;
  model: string;
  pricing_model_family?: string | null;
  pricing_source: string;
  purpose: "facts" | "summary" | "digest" | "embedding" | string;
  input_tokens: number;
  output_tokens: number;
  cache_creation_input_tokens: number;
  cache_read_input_tokens: number;
  estimated_cost_usd: number;
  created_at: string;
}

export interface LLMUsageDailySummary {
  date_jst: string;
  provider: string;
  purpose: string;
  pricing_source: string;
  calls: number;
  input_tokens: number;
  output_tokens: number;
  cache_creation_input_tokens: number;
  cache_read_input_tokens: number;
  estimated_cost_usd: number;
}

export interface LLMUsageModelSummary {
  provider: string;
  model: string;
  pricing_source: string;
  calls: number;
  input_tokens: number;
  output_tokens: number;
  cache_creation_input_tokens: number;
  cache_read_input_tokens: number;
  estimated_cost_usd: number;
}

export interface UserSettingsCurrentMonth {
  month_jst: string;
  period_start_jst: string;
  period_end_jst: string;
  estimated_cost_usd: number;
  remaining_budget_usd: number | null;
  remaining_budget_pct: number | null;
}

export interface UserReadingPlanSettings {
  window: "24h" | "today_jst" | "7d" | string;
  size: number;
  diversify_topics: boolean;
  exclude_read: boolean;
}

export interface UserSettings {
  user_id: string;
  has_anthropic_api_key: boolean;
  anthropic_api_key_last4: string | null;
  has_openai_api_key: boolean;
  openai_api_key_last4: string | null;
  has_google_api_key: boolean;
  google_api_key_last4: string | null;
  has_inoreader_oauth?: boolean;
  inoreader_token_expires_at?: string | null;
  monthly_budget_usd: number | null;
  budget_alert_enabled: boolean;
  budget_alert_threshold_pct: number;
  digest_email_enabled: boolean;
  reading_plan: UserReadingPlanSettings;
  llm_models?: {
    anthropic_facts?: string | null;
    anthropic_summary?: string | null;
    anthropic_digest_cluster?: string | null;
    anthropic_digest?: string | null;
    anthropic_source_suggestion?: string | null;
    openai_embedding?: string | null;
  };
  current_month: UserSettingsCurrentMonth;
}

export interface DashboardSnapshot {
  sources_count: number;
  item_stats: ItemStats | null;
  digests: Digest[];
  llm_summary: LLMUsageDailySummary[];
  topic_trends: { items: TopicTrend[]; limit: number };
  failed_items_preview?: ItemListResponse | null;
  llm_days: number;
}

export interface BriefingCluster {
  id: string;
  label: string;
  summary?: string;
  max_score?: number | null;
  topics?: string[];
  items: Item[];
}

export interface BriefingTodayResponse {
  date: string;
  greeting: string;
  status: "pending" | "ready" | "stale" | string;
  generated_at?: string | null;
  highlight_items: Item[];
  clusters: BriefingCluster[];
  stats: {
    total_unread: number;
    today_highlight_count: number;
    yesterday_read: number;
    yesterday_skipped: number;
    streak_days: number;
    today_read_count?: number;
    streak_target?: number;
    streak_remaining?: number;
    streak_at_risk?: boolean;
  };
}

async function apiFetch<T>(path: string, options?: RequestInit): Promise<T> {
  const res = await fetch(`/api${path}`, {
    ...options,
    headers: {
      "Content-Type": "application/json",
      ...options?.headers,
    },
  });
  if (!res.ok) {
    const text = await res.text().catch(() => "");
    throw new Error(`${res.status}: ${text || res.statusText}`);
  }
  if (res.status === 204) return undefined as T;
  return res.json();
}

export const api = {
  // Sources
  getSources: () => apiFetch<Source[]>("/sources"),
  getSourceHealth: () => apiFetch<{ items: SourceHealth[] }>("/sources/health"),
  exportSourcesOPML: async () => {
    const res = await fetch("/api/sources/opml");
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
    return apiFetch<{ items: SourceSuggestion[]; limit: number; llm?: { provider?: string; model?: string; estimated_cost_usd?: number } | null }>(`/sources/suggestions${qs ? `?${qs}` : ""}`);
  },
  getRecommendedSources: (params?: { limit?: number }) => {
    const q = new URLSearchParams();
    if (params?.limit) q.set("limit", String(params.limit));
    const qs = q.toString();
    return apiFetch<{ items: RecommendedSource[]; limit: number }>(`/sources/recommended${qs ? `?${qs}` : ""}`);
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
  getItems: (params?: { status?: string; source_id?: string; topic?: string; page?: number; page_size?: number; sort?: string; unread_only?: boolean; favorite_only?: boolean }) => {
    const q = new URLSearchParams();
    if (params?.status) q.set("status", params.status);
    if (params?.source_id) q.set("source_id", params.source_id);
    if (params?.topic) q.set("topic", params.topic);
    if (params?.page) q.set("page", String(params.page));
    if (params?.page_size) q.set("page_size", String(params.page_size));
    if (params?.sort) q.set("sort", params.sort);
    if (params?.unread_only != null) q.set("unread_only", String(params.unread_only));
    if (params?.favorite_only != null) q.set("favorite_only", String(params.favorite_only));
    const qs = q.toString();
    return apiFetch<ItemListResponse>(`/items${qs ? `?${qs}` : ""}`);
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
  }) => {
    const q = new URLSearchParams();
    if (params?.window) q.set("window", params.window);
    if (params?.size) q.set("size", String(params.size));
    if (params?.diversify_topics != null) q.set("diversify_topics", String(params.diversify_topics));
    const qs = q.toString();
    return apiFetch<FocusQueueResponse>(`/items/focus-queue${qs ? `?${qs}` : ""}`);
  },
  getItemStats: () => apiFetch<ItemStats>("/items/stats"),
  getDashboard: (params?: { llm_days?: number; topic_limit?: number; digest_limit?: number }) => {
    const q = new URLSearchParams();
    if (params?.llm_days) q.set("llm_days", String(params.llm_days));
    if (params?.topic_limit) q.set("topic_limit", String(params.topic_limit));
    if (params?.digest_limit) q.set("digest_limit", String(params.digest_limit));
    const qs = q.toString();
    return apiFetch<DashboardSnapshot>(`/dashboard${qs ? `?${qs}` : ""}`);
  },
  getBriefingToday: (params?: { size?: number }) => {
    const q = new URLSearchParams();
    if (params?.size) q.set("size", String(params.size));
    const qs = q.toString();
    return apiFetch<BriefingTodayResponse>(`/briefing/today${qs ? `?${qs}` : ""}`);
  },
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
  deleteItem: (id: string) =>
    apiFetch<void>(`/items/${id}`, { method: "DELETE" }),
  setItemFeedback: (id: string, body: { rating: number; is_favorite: boolean }) =>
    apiFetch<ItemFeedbackResult>(`/items/${id}/feedback`, {
      method: "PATCH",
      body: JSON.stringify(body),
    }),
  retryItem: (id: string) =>
    apiFetch<ItemRetryResult>(`/items/${id}/retry`, { method: "POST" }),
  retryFailedItems: (params?: { source_id?: string }) => {
    const q = new URLSearchParams();
    if (params?.source_id) q.set("source_id", params.source_id);
    const qs = q.toString();
    return apiFetch<BulkRetryFailedResult>(`/items/retry-failed${qs ? `?${qs}` : ""}`, {
      method: "POST",
    });
  },

  // LLM Usage
  getLLMUsage: (params?: { limit?: number }) => {
    const q = new URLSearchParams();
    if (params?.limit) q.set("limit", String(params.limit));
    const qs = q.toString();
    return apiFetch<LLMUsageLog[]>(`/llm-usage${qs ? `?${qs}` : ""}`);
  },
  getLLMUsageSummary: (params?: { days?: number }) => {
    const q = new URLSearchParams();
    if (params?.days) q.set("days", String(params.days));
    const qs = q.toString();
    return apiFetch<LLMUsageDailySummary[]>(`/llm-usage/summary${qs ? `?${qs}` : ""}`);
  },
  getLLMUsageByModel: (params?: { days?: number }) => {
    const q = new URLSearchParams();
    if (params?.days) q.set("days", String(params.days));
    const qs = q.toString();
    return apiFetch<LLMUsageModelSummary[]>(`/llm-usage/by-model${qs ? `?${qs}` : ""}`);
  },

  // Settings
  getSettings: () => apiFetch<UserSettings>("/settings"),
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
  updateReadingPlanSettings: (body: Pick<UserReadingPlanSettings, "window" | "size" | "diversify_topics">) =>
    apiFetch<{ user_id: string; reading_plan: UserReadingPlanSettings }>("/settings/reading-plan", {
      method: "PATCH",
      body: JSON.stringify(body),
    }),
  updateLLMModelSettings: (body: {
    anthropic_facts?: string | null;
    anthropic_summary?: string | null;
    anthropic_digest_cluster?: string | null;
    anthropic_digest?: string | null;
    anthropic_source_suggestion?: string | null;
    openai_embedding?: string | null;
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
  deleteInoreaderOAuth: () =>
    apiFetch<{ user_id: string; has_inoreader_oauth: boolean; inoreader_token_expires: string | null }>(
      "/settings/inoreader-oauth",
      { method: "DELETE" }
    ),

  // Digests
  getDigests: () => apiFetch<Digest[]>("/digests"),
  getDigest: (id: string) => apiFetch<DigestDetail>(`/digests/${id}`),
  getLatestDigest: () => apiFetch<DigestDetail>("/digests/latest"),
};
