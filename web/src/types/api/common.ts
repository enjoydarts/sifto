import type { Digest } from "./digests";
import type { ItemStats, ItemListResponse, TopicTrend } from "./items";
import type { LLMUsageDailySummary } from "./llm-usage";

export interface ProviderModelChangeEvent {
  id: string;
  provider: string;
  change_type: "added" | "constrained" | "removed" | string;
  model_id: string;
  detected_at: string;
  metadata?: Record<string, unknown> | null;
}

export interface ProviderModelChangeSummary {
  provider: string;
  detected_at: string;
  trigger: string;
  added: ProviderModelChangeEvent[];
  constrained: ProviderModelChangeEvent[];
  removed: ProviderModelChangeEvent[];
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
