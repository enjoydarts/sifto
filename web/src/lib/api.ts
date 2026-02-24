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

export interface Item {
  id: string;
  source_id: string;
  url: string;
  title: string | null;
  content_text: string | null;
  status: "new" | "fetched" | "facts_extracted" | "summarized" | "failed";
  is_read: boolean;
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

export interface ItemDetail extends Item {
  facts: ItemFacts | null;
  summary: ItemSummary | null;
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
  published_at?: string | null;
  created_at: string;
}

export interface RelatedItemsResponse {
  item_id: string;
  limit: number;
  items: RelatedItem[];
}

export interface ItemRetryResult {
  item_id: string;
  status: "queued";
}

export interface ItemReadResult {
  item_id: string;
  is_read: boolean;
}

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
}

export interface ItemStats {
  total: number;
  read: number;
  unread: number;
  by_status: Record<string, number>;
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
}

export interface DigestDetail extends Digest {
  items: DigestItemDetail[];
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
  monthly_budget_usd: number | null;
  budget_alert_enabled: boolean;
  budget_alert_threshold_pct: number;
  reading_plan: UserReadingPlanSettings;
  current_month: UserSettingsCurrentMonth;
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
  getItems: (params?: { status?: string; source_id?: string; page?: number; page_size?: number; sort?: string; unread_only?: boolean }) => {
    const q = new URLSearchParams();
    if (params?.status) q.set("status", params.status);
    if (params?.source_id) q.set("source_id", params.source_id);
    if (params?.page) q.set("page", String(params.page));
    if (params?.page_size) q.set("page_size", String(params.page_size));
    if (params?.sort) q.set("sort", params.sort);
    if (params?.unread_only != null) q.set("unread_only", String(params.unread_only));
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
  getItemStats: () => apiFetch<ItemStats>("/items/stats"),
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

  // Settings
  getSettings: () => apiFetch<UserSettings>("/settings"),
  updateSettings: (body: {
    monthly_budget_usd: number | null;
    budget_alert_enabled: boolean;
    budget_alert_threshold_pct: number;
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

  // Digests
  getDigests: () => apiFetch<Digest[]>("/digests"),
  getDigest: (id: string) => apiFetch<DigestDetail>(`/digests/${id}`),
  getLatestDigest: () => apiFetch<DigestDetail>("/digests/latest"),
};
