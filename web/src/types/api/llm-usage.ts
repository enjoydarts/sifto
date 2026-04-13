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
  estimated_cost_usd: number;
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
