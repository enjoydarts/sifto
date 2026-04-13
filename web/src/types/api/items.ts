export interface Item {
  id: string;
  source_id: string;
  source_title?: string | null;
  url: string;
  title: string | null;
  translated_title?: string | null;
  thumbnail_url?: string | null;
  content_text: string | null;
  status: "new" | "fetched" | "facts_extracted" | "summarized" | "failed" | "deleted";
  processing_error?: string | null;
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
  personal_score_breakdown?: PersonalScoreBreakdown | null;
  search_match_count?: number;
  search_snippets?: ItemSearchSnippet[];
  published_at: string | null;
  fetched_at: string | null;
  created_at: string;
  updated_at: string;
}

export interface ItemSearchSnippet {
  field: "title" | "summary" | "facts" | "note" | "highlight" | "content" | string;
  snippet_html: string;
}

export interface ItemSearchSuggestion {
  kind: "article" | "source" | "topic" | string;
  label: string;
  item_id?: string | null;
  source_id?: string | null;
  topic?: string | null;
  article_count?: number | null;
}

export interface ItemSearchSuggestionResponse {
  items: ItemSearchSuggestion[];
}

export interface PersonalScoreComponent {
  value: number;
  weight: number;
}

export interface PersonalScoreBreakdown {
  learned_weight_score: PersonalScoreComponent;
  topic_relevance: PersonalScoreComponent;
  embedding_similarity: PersonalScoreComponent;
  source_affinity: PersonalScoreComponent;
  recency_decay: PersonalScoreComponent;
  matched_topics?: string[];
  dominant_dimension?: string | null;
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
  requested_model?: string | null;
  resolved_model?: string | null;
  pricing_source: string;
  prompt_key?: string | null;
  prompt_source?: string | null;
  prompt_version_id?: string | null;
  prompt_version_number?: number | null;
  prompt_experiment_id?: string | null;
  prompt_experiment_arm_id?: string | null;
  created_at: string;
}

export interface ItemLLMExecutionAttempt {
  provider: string;
  model: string;
  purpose: string;
  prompt_key?: string;
  prompt_source?: string;
  prompt_version_id?: string | null;
  prompt_version_number?: number | null;
  prompt_experiment_id?: string | null;
  prompt_experiment_arm_id?: string | null;
  status: string;
  attempt_index: number;
  error_kind?: string | null;
  error_message?: string | null;
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
  facts_executions?: ItemLLMExecutionAttempt[];
  facts_check?: FactsCheck | null;
  facts_check_llm?: ItemSummaryLLM | null;
  summary: ItemSummary | null;
  summary_llm?: ItemSummaryLLM | null;
  summary_executions?: ItemLLMExecutionAttempt[];
  faithfulness?: SummaryFaithfulnessCheck | null;
  faithfulness_llm?: ItemSummaryLLM | null;
  feedback?: ItemFeedback | null;
  note?: ItemNote | null;
  highlights?: ItemHighlight[];
}

export interface PreferenceProfileWeight {
  value: number;
  default: number;
  delta: number;
}

export interface PreferenceProfileTopic {
  topic: string;
  score: number;
  signal_count: number;
}

export interface PreferenceProfileSource {
  source_id: string;
  source_title: string;
  score: number;
}

export interface PreferenceProfileReadingPattern {
  avg_score_read: number;
  avg_score_skipped: number;
  favorite_rate: number;
  note_rate: number;
}

export interface PreferenceProfile {
  status: "cold_start" | "learning" | "active" | string;
  confidence: number;
  feedback_count: number;
  read_count: number;
  computed_at?: string | null;
  learned_weights: Record<string, PreferenceProfileWeight>;
  top_topics: PreferenceProfileTopic[];
  top_sources: PreferenceProfileSource[];
  reading_pattern: PreferenceProfileReadingPattern;
}

export interface PreferenceProfileSummary {
  status: "cold_start" | "learning" | "active" | string;
  confidence: number;
  feedback_count: number;
  top_topics: string[];
  strongest_weight: string;
  computed_at?: string | null;
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

export interface SummaryAudioSynthesisResponse {
  item?: ItemDetail | null;
  persona: string;
  audio_base64: string;
  content_type: string;
  duration_sec: number;
  resolved_text: string;
  preprocessed_text?: string | null;
}

export interface BulkMarkReadResult {
  status: "ok";
  updated_count: number;
}

export interface BulkMarkLaterResult {
  status: "ok";
  updated_count: number;
}

export interface BulkDeleteItemsResult {
  status: "ok";
  item_ids: string[];
  updated_count: number;
  skipped_count: number;
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
  search_mode?: "natural" | "and" | "or" | string | null;
  search_unavailable?: boolean;
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

export interface TriageBundle {
  id: string;
  label: string;
  size: number;
  max_similarity: number;
  representative: Item;
  items: Item[];
  summary?: string | null;
  shared_topics?: string[];
}

export interface TriageQueueEntry {
  entry_type: "item" | "bundle";
  item?: Item | null;
  bundle?: TriageBundle | null;
}

export interface TriageQueueResponse {
  entries: TriageQueueEntry[];
  window: "24h" | "today_jst" | "7d" | string;
  size: number;
  completed: number;
  remaining: number;
  total: number;
  underlying_items: number;
  bundle_count: number;
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

import type { ReadingGoal } from "./reading-goals";

export interface BulkRetryItemsResult {
  status: "queued";
  item_ids: string[];
  queued_count: number;
  skipped_count: number;
}
