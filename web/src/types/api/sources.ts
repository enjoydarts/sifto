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

export interface SourceItemStats {
  source_id: string;
  total_items: number;
  unread_items: number;
  read_items: number;
  avg_items_per_day_30d: number;
}

export interface SourceDailyCount {
  day: string;
  count: number;
}

export interface SourceDailyStats {
  source_id: string;
  today_count: number;
  yesterday_count: number;
  last_7d_total: number;
  last_30d_total: number;
  active_days_30d: number;
  avg_items_per_active_day_30d: number;
  daily_counts: SourceDailyCount[];
}

export interface SourcesDailyOverview {
  today_count: number;
  yesterday_count: number;
  last_7d_total: number;
  last_30d_total: number;
  active_days_30d: number;
  avg_items_per_active_day_30d: number;
  daily_counts: SourceDailyCount[];
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

export interface SourceNavigatorPick {
  source_id: string;
  title: string;
  comment: string;
}

export interface SourceNavigatorResponse {
  navigator?: {
    enabled: boolean;
    persona: string;
    character_name: string;
    character_title: string;
    avatar_style: string;
    speech_style: string;
    overview: string;
    keep: SourceNavigatorPick[];
    watch: SourceNavigatorPick[];
    standout: SourceNavigatorPick[];
    generated_at?: string | null;
  } | null;
}
