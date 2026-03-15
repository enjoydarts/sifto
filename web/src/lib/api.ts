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

export interface SourceOptimizationMetrics {
  unread_backlog: number;
  read_rate: number;
  favorite_rate: number;
  notification_open_rate: number;
  average_summary_score: number;
}

export interface SourceOptimizationItem {
  source_id: string;
  recommendation: "keep" | "prune" | "mute" | "promote" | string;
  reason: string;
  metrics: SourceOptimizationMetrics;
}

export interface ReadingGoal {
  id: string;
  user_id: string;
  title: string;
  description: string;
  priority: number;
  status: "active" | "archived" | string;
  due_date?: string | null;
  created_at?: string;
  updated_at?: string;
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
  facts_check_result?: "pass" | "warn" | "fail" | string | null;
  faithfulness_result?: "pass" | "warn" | "fail" | string | null;
  is_read: boolean;
  is_favorite: boolean;
  feedback_rating: -1 | 0 | 1 | number;
  summary_score?: number | null;
  summary_topics?: string[];
  recommendation_reason?: string | null;
  personal_score?: number;
  personal_score_reason?: string;
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

export interface SummaryFaithfulnessCheck {
  id: string;
  item_id: string;
  final_result: "pass" | "warn" | "fail" | string;
  retry_count: number;
  short_comment?: string | null;
  created_at: string;
  updated_at: string;
}

export interface FactsCheck {
  id: string;
  item_id: string;
  final_result: "pass" | "warn" | "fail" | string;
  retry_count: number;
  short_comment?: string | null;
  created_at: string;
  updated_at: string;
}

export interface ItemFeedback {
  user_id: string;
  item_id: string;
  rating: -1 | 0 | 1 | number;
  is_favorite: boolean;
  updated_at: string;
}

export interface ItemNote {
  id: string;
  user_id: string;
  item_id: string;
  content: string;
  tags?: string[];
  created_at: string;
  updated_at: string;
}

export interface ItemHighlight {
  id: string;
  user_id: string;
  item_id: string;
  quote_text: string;
  anchor_text?: string | null;
  section?: string | null;
  created_at: string;
}

export interface ItemDetail extends Item {
  processing_error?: string | null;
  facts: ItemFacts | null;
  facts_llm?: ItemSummaryLLM | null;
  facts_check?: FactsCheck | null;
  facts_check_llm?: ItemSummaryLLM | null;
  summary: ItemSummary | null;
  summary_llm?: ItemSummaryLLM | null;
  faithfulness?: SummaryFaithfulnessCheck | null;
  faithfulness_llm?: ItemSummaryLLM | null;
  feedback?: ItemFeedback | null;
  note?: ItemNote | null;
  highlights?: ItemHighlight[];
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

export interface BulkMarkReadResult {
  status: "ok";
  updated_count: number;
}

export interface BulkMarkLaterResult {
  status: "ok";
  updated_count: number;
}

export interface FavoritesMarkdownExportParams {
  days?: number;
  limit?: number;
}

export interface ItemLaterResult {
  item_id: string;
  is_later: boolean;
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

export interface TodayQueueItem {
  item: Item;
  estimated_reading_minutes: number;
  reason_labels: string[];
  matched_goals?: ReadingGoal[];
}

export interface TodayQueueResponse {
  items: TodayQueueItem[];
}

export interface ReviewQueueItem {
  id: string;
  user_id: string;
  item_id: string;
  source_signal: string;
  review_stage: string;
  status: string;
  review_due_at: string;
  last_surfaced_at?: string | null;
  completed_at?: string | null;
  snooze_count: number;
  created_at: string;
  updated_at: string;
  item: Item;
  reason_labels?: string[];
}

export interface ReviewQueueResponse {
  items: ReviewQueueItem[];
}

export interface AskInsightItemRef {
  item_id: string;
  title: string;
  url: string;
  topics?: string[];
}

export interface AskInsight {
  id: string;
  user_id: string;
  title: string;
  body: string;
  query?: string;
  goal_id?: string | null;
  tags?: string[];
  items?: AskInsightItemRef[];
  created_at: string;
  updated_at: string;
}

export interface WeeklyReviewTopic {
  topic: string;
  count: number;
}

export interface WeeklyReviewSnapshot {
  id: string;
  user_id: string;
  week_start: string;
  week_end: string;
  read_count: number;
  note_count: number;
  insight_count: number;
  favorite_count: number;
  dominant_topics?: WeeklyReviewTopic[];
  missed_high_value?: Item[];
  created_at: string;
}

export interface ItemStats {
  total: number;
  read: number;
  unread: number;
  by_status: Record<string, number>;
}

export interface ItemUXMetrics {
  days: number;
  today_date: string;
  today_new_items: number;
  today_read_items: number;
  today_consumption_rate?: number;
  period_read_items: number;
  period_active_read_days: number;
  period_average_reads_per_day: number;
  current_streak_days: number;
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
  digest_retry_count: number;
  cluster_draft_retry_count: number;
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
  digest_llm?: ItemSummaryLLM | null;
  cluster_draft_llm?: ItemSummaryLLM | null;
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

export interface LLMUsageProviderMonthSummary {
  month_jst: string;
  provider: string;
  calls: number;
  input_tokens: number;
  output_tokens: number;
  cache_creation_input_tokens: number;
  cache_read_input_tokens: number;
  estimated_cost_usd: number;
}

export interface LLMUsagePurposeMonthSummary {
  month_jst: string;
  purpose: string;
  calls: number;
  input_tokens: number;
  output_tokens: number;
  cache_creation_input_tokens: number;
  cache_read_input_tokens: number;
  estimated_cost_usd: number;
}

export interface LLMUsageAnalysisSummary {
  provider: string;
  model: string;
  purpose: string;
  pricing_source: string;
  calls: number;
  input_tokens: number;
  output_tokens: number;
  cache_creation_input_tokens: number;
  cache_read_input_tokens: number;
  estimated_cost_usd: number;
}

export interface LLMExecutionCurrentMonthSummary {
  month_jst: string;
  purpose: string;
  provider: string;
  model: string;
  attempts: number;
  successes: number;
  failures: number;
  retries: number;
  empty_responses: number;
  failure_rate_pct: number;
  retry_rate_pct: number;
  empty_rate_pct: number;
}

export interface LLMValueMetric {
  window_start: string;
  window_end: string;
  month_jst: string;
  purpose: string;
  provider: string;
  model: string;
  pricing_source: string;
  calls: number;
  total_cost_usd: number;
  item_count: number;
  read_count: number;
  favorite_count: number;
  insight_count: number;
  cost_to_read_usd?: number | null;
  cost_to_favorite_usd?: number | null;
  cost_to_insight_usd?: number | null;
  low_efficiency_flag: boolean;
  advisory_code: "ok" | "review_model" | "low_signal" | string;
  advisory_reason?: string | null;
  benchmark_provider?: string | null;
  benchmark_model?: string | null;
  benchmark_metric?: "read" | "favorite" | "insight" | string | null;
}

export interface ProviderModelChangeEvent {
  id: string;
  provider: string;
  change_type: "added" | "removed" | string;
  model_id: string;
  detected_at: string;
  metadata?: Record<string, unknown> | null;
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

export interface NotificationPriorityRule {
  id?: string;
  sensitivity: "low" | "medium" | "high" | string;
  daily_cap: number;
  theme_weight: number;
}

export interface LLMCatalogProvider {
  id: string;
  api_key_header?: string;
  match_exact?: string[];
  match_prefixes?: string[];
  default_models?: Record<string, string>;
}

export interface LLMCatalogModelCapabilities {
  supports_structured_output: boolean;
  supports_strict_json_schema: boolean;
  supports_reasoning: boolean;
  supports_tool_calling: boolean;
  supports_cache_read_pricing: boolean;
  supports_cache_write_pricing: boolean;
}

export interface LLMCatalogModelPricing {
  pricing_source: string;
  input_per_mtok_usd: number;
  output_per_mtok_usd: number;
  cache_write_per_mtok_usd: number;
  cache_read_per_mtok_usd: number;
}

export interface LLMCatalogModel {
  id: string;
  provider: "anthropic" | "google" | "groq" | "openai" | "deepseek" | "alibaba" | "mistral" | "xai" | string;
  available_purposes: string[];
  recommendation?: "recommended" | "strong" | "experimental" | string;
  best_for?: "facts" | "summary" | "ask" | "digest" | "embedding" | "balanced" | string;
  highlights?: Array<"lowestCost" | "fast" | "jsonStable" | string>;
  comment?: string;
  capabilities?: LLMCatalogModelCapabilities | null;
  pricing?: LLMCatalogModelPricing | null;
}

export interface LLMCatalog {
  providers: LLMCatalogProvider[];
  chat_models: LLMCatalogModel[];
  embedding_models: LLMCatalogModel[];
}

export interface UserSettings {
  user_id: string;
  has_anthropic_api_key: boolean;
  anthropic_api_key_last4: string | null;
  has_openai_api_key: boolean;
  openai_api_key_last4: string | null;
  has_google_api_key: boolean;
  google_api_key_last4: string | null;
  has_groq_api_key: boolean;
  groq_api_key_last4: string | null;
  has_deepseek_api_key: boolean;
  deepseek_api_key_last4: string | null;
  has_alibaba_api_key: boolean;
  alibaba_api_key_last4: string | null;
  has_mistral_api_key: boolean;
  mistral_api_key_last4: string | null;
  has_xai_api_key: boolean;
  xai_api_key_last4: string | null;
  has_inoreader_oauth?: boolean;
  inoreader_token_expires_at?: string | null;
  monthly_budget_usd: number | null;
  budget_alert_enabled: boolean;
  budget_alert_threshold_pct: number;
  digest_email_enabled: boolean;
  reading_plan: UserReadingPlanSettings;
  llm_models?: {
    facts?: string | null;
    summary?: string | null;
    digest_cluster?: string | null;
    digest?: string | null;
    ask?: string | null;
    source_suggestion?: string | null;
    embedding?: string | null;
    facts_check?: string | null;
    faithfulness_check?: string | null;
  };
  obsidian_export?: {
    enabled: boolean;
    github_app_enabled?: boolean;
    github_app_install_url?: string | null;
    github_installation_id?: number | null;
    github_repo_owner?: string | null;
    github_repo_name?: string | null;
    github_repo_branch?: string | null;
    vault_root_path?: string | null;
    keyword_link_mode?: string | null;
    last_run_at?: string | null;
    last_success_at?: string | null;
  };
  notification_priority?: NotificationPriorityRule;
  current_month: UserSettingsCurrentMonth;
}

export interface ObsidianExportRunResult {
  user_id: string;
  updated: number;
  skipped: number;
  failed: number;
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
  greeting_key?: "morning" | "afternoon" | "evening" | string;
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

export interface AskCitation {
  item_id: string;
  title: string;
  url: string;
  reason?: string;
  published_at?: string | null;
  topics?: string[];
}

export interface AskCandidate extends Item {
  summary: string;
  facts?: string[];
  similarity: number;
}

export interface AskResponse {
  query: string;
  answer: string;
  bullets?: string[];
  citations?: AskCitation[];
  related_items?: AskCandidate[];
}

async function apiFetch<T>(path: string, options?: RequestInit): Promise<T> {
  const requestPath = withCacheBust(path, options?.method);
  const authHeaders = await getAuthHeaders();
  let res = await fetch(`/api${requestPath}`, {
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
    res = await fetch(`/api${requestPath}`, {
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

const FORCE_FRESH_UNTIL_KEY = "sifto.forceFreshUntil";

function withCacheBust(path: string, method?: string): string {
  const upperMethod = (method ?? "GET").toUpperCase();
  if (upperMethod !== "GET" && upperMethod !== "HEAD") return path;
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
  getSourceHealth: () => apiFetch<{ items: SourceHealth[] }>("/sources/health"),
  getSourceOptimization: () => apiFetch<{ items: SourceOptimizationItem[] }>("/sources/optimization"),
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
  getItems: (params?: { status?: string; source_id?: string; topic?: string; page?: number; page_size?: number; sort?: string; unread_only?: boolean; read_only?: boolean; favorite_only?: boolean; later_only?: boolean }) => {
    const q = new URLSearchParams();
    if (params?.status) q.set("status", params.status);
    if (params?.source_id) q.set("source_id", params.source_id);
    if (params?.topic) q.set("topic", params.topic);
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
  setItemFeedback: (id: string, body: { rating: number; is_favorite: boolean }) =>
    apiFetch<ItemFeedbackResult>(`/items/${id}/feedback`, {
      method: "PATCH",
      body: JSON.stringify(body),
    }),
  retryItem: (id: string) =>
    apiFetch<ItemRetryResult>(`/items/${id}/retry`, { method: "POST" }),
  retryItemFromFacts: (id: string) =>
    apiFetch<ItemRetryResult>(`/items/${id}/retry-from-facts`, { method: "POST" }),
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
  getLLMUsageAnalysis: (params?: { days?: number }) => {
    const q = new URLSearchParams();
    if (params?.days) q.set("days", String(params.days));
    const qs = q.toString();
    return apiFetch<LLMUsageAnalysisSummary[]>(`/llm-usage/analysis${qs ? `?${qs}` : ""}`);
  },
  getLLMUsageCurrentMonthByProvider: () => {
    return apiFetch<LLMUsageProviderMonthSummary[]>("/llm-usage/current-month/by-provider");
  },
  getLLMUsageCurrentMonthByPurpose: () => {
    return apiFetch<LLMUsagePurposeMonthSummary[]>("/llm-usage/current-month/by-purpose");
  },
  getLLMExecutionCurrentMonthSummary: () => {
    return apiFetch<LLMExecutionCurrentMonthSummary[]>("/llm-usage/current-month/execution-summary");
  },
  getLLMValueMetricsCurrentMonth: () => {
    return apiFetch<LLMValueMetric[]>("/llm-usage/current-month/value-metrics");
  },
  getProviderModelUpdates: (params?: { days?: number; limit?: number }) => {
    const q = new URLSearchParams();
    if (params?.days) q.set("days", String(params.days));
    if (params?.limit) q.set("limit", String(params.limit));
    const qs = q.toString();
    return apiFetch<ProviderModelChangeEvent[]>(`/provider-model-updates${qs ? `?${qs}` : ""}`);
  },

  // Settings
  getSettings: () => apiFetch<UserSettings>("/settings"),
  updateNotificationPriority: (body: NotificationPriorityRule) =>
    apiFetch<{ user_id: string; notification_priority: NotificationPriorityRule }>("/settings/notification-priority", {
      method: "PATCH",
      body: JSON.stringify(body),
    }),
  getLLMCatalog: () => apiFetch<LLMCatalog>("/settings/llm-catalog"),
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
    summary?: string | null;
    digest_cluster?: string | null;
    digest?: string | null;
    ask?: string | null;
    source_suggestion?: string | null;
    embedding?: string | null;
    facts_check?: string | null;
    faithfulness_check?: string | null;
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
