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
  summarized_at: string;
}

export interface ItemDetail extends Item {
  facts: ItemFacts | null;
  summary: ItemSummary | null;
}

export interface ItemRetryResult {
  item_id: string;
  status: "queued";
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
  purpose: "facts" | "summary" | "digest" | string;
  input_tokens: number;
  output_tokens: number;
  cache_creation_input_tokens: number;
  cache_read_input_tokens: number;
  estimated_cost_usd: number;
  created_at: string;
}

export interface LLMUsageDailySummary {
  date_jst: string;
  purpose: string;
  pricing_source: string;
  calls: number;
  input_tokens: number;
  output_tokens: number;
  cache_creation_input_tokens: number;
  cache_read_input_tokens: number;
  estimated_cost_usd: number;
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
  getItems: (params?: { status?: string; source_id?: string }) => {
    const q = new URLSearchParams();
    if (params?.status) q.set("status", params.status);
    if (params?.source_id) q.set("source_id", params.source_id);
    const qs = q.toString();
    return apiFetch<Item[]>(`/items${qs ? `?${qs}` : ""}`);
  },
  getItem: (id: string) => apiFetch<ItemDetail>(`/items/${id}`),
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

  // Digests
  getDigests: () => apiFetch<Digest[]>("/digests"),
  getDigest: (id: string) => apiFetch<DigestDetail>(`/digests/${id}`),
  getLatestDigest: () => apiFetch<DigestDetail>("/digests/latest"),
};
