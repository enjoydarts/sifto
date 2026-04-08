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

export interface OpenRouterSyncRun {
  id: string;
  started_at: string;
  finished_at?: string | null;
  status: string;
  trigger_type: string;
  fetched_count: number;
  accepted_count: number;
  translation_target_count: number;
  translation_completed_count: number;
  error_message?: string | null;
}

export interface OpenRouterSyncStatusResponse {
  run: OpenRouterSyncRun | null;
}

export interface PoeSyncRun {
  id: string;
  started_at: string;
  finished_at?: string | null;
  last_progress_at?: string | null;
  status: string;
  trigger_type: string;
  fetched_count: number;
  accepted_count: number;
  translation_target_count: number;
  translation_completed_count: number;
  translation_failed_count: number;
  error_message?: string | null;
}

export interface PoeSyncStatusResponse {
  run: PoeSyncRun | null;
}

export interface PromptAdminCapabilities {
  can_manage_prompts: boolean;
  user_email: string | null;
  purposes: string[];
}

export interface PromptTemplate {
  id: string;
  key: string;
  purpose: string;
  description: string;
  status: string;
  active_version_id?: string | null;
  created_at: string;
  updated_at: string;
}

export interface PromptTemplateVersion {
  id: string;
  template_id: string;
  version: number;
  label: string;
  system_instruction: string;
  prompt_text: string;
  fallback_prompt_text: string;
  variables_schema?: Record<string, unknown> | string | null;
  notes: string;
  created_by_user_id?: string | null;
  created_by_email: string;
  created_at: string;
}

export interface PromptExperiment {
  id: string;
  template_id: string;
  name: string;
  status: string;
  assignment_unit: string;
  started_at?: string | null;
  ended_at?: string | null;
  created_by_user_id?: string | null;
  created_by_email: string;
  created_at: string;
  updated_at: string;
}

export interface PromptExperimentArm {
  id: string;
  experiment_id: string;
  version_id: string;
  weight: number;
  created_at: string;
  updated_at: string;
}

export interface PromptTemplateDefault {
  label: string;
  system_instruction: string;
  prompt_text: string;
  fallback_prompt_text: string;
  variables_schema?: Record<string, unknown> | string | null;
  preview_variables?: Record<string, unknown> | string | null;
  notes: string;
}

export interface PromptTemplateDetailResponse {
  template: PromptTemplate;
  versions: PromptTemplateVersion[];
  experiments: PromptExperiment[];
  arms: PromptExperimentArm[];
  default_template: PromptTemplateDefault;
}

export interface AivisSyncRun {
  id: string;
  started_at: string;
  finished_at?: string | null;
  last_progress_at?: string | null;
  status: string;
  trigger_type: string;
  fetched_count: number;
  accepted_count: number;
  error_message?: string | null;
}

export interface AivisSyncStatusResponse {
  run: AivisSyncRun | null;
}

export interface AivisModelVoiceSample {
  audio_url: string;
  transcript: string;
}

export interface AivisModelSpeakerStyle {
  name: string;
  icon_url?: string | null;
  local_id: number;
  voice_samples: AivisModelVoiceSample[];
}

export interface AivisModelSpeaker {
  aivm_speaker_uuid: string;
  name: string;
  icon_url: string;
  supported_languages: string[];
  local_id: number;
  styles: AivisModelSpeakerStyle[];
}

export interface AivisModelTag {
  name: string;
}

export interface AivisModelFile {
  aivm_model_uuid: string;
  manifest_version: string;
  name: string;
  description: string;
  creators: string[];
  license_type: string;
  license_text?: string | null;
  model_type: string;
  model_architecture: string;
  model_format: string;
  training_epochs?: number | null;
  training_steps?: number | null;
  version: string;
  file_size: number;
  checksum: string;
  download_count: number;
  created_at: string;
  updated_at: string;
}

export interface AivisModelSnapshot {
  aivm_model_uuid: string;
  name: string;
  description: string;
  detailed_description: string;
  category: string;
  voice_timbre: string;
  visibility: string;
  is_tag_locked: boolean;
  total_download_count: number;
  like_count: number;
  is_liked: boolean;
  user_json: Record<string, unknown>;
  model_files_json: AivisModelFile[];
  tags_json: AivisModelTag[];
  speakers_json: AivisModelSpeaker[];
  model_file_count: number;
  speaker_count: number;
  style_count: number;
  created_at: string;
  updated_at: string;
  fetched_at: string;
}

export interface AivisModelsResponse {
  latest_run: AivisSyncRun | null;
  latest_change_summary?: ProviderModelChangeSummary | null;
  models: AivisModelSnapshot[];
  removed_models?: AivisModelSnapshot[];
}

export interface XAIVoiceSyncRun {
  id: number;
  started_at: string;
  finished_at?: string | null;
  last_progress_at?: string | null;
  status: string;
  trigger_type: string;
  fetched_count: number;
  saved_count: number;
  error_message?: string | null;
}

export interface XAIVoiceSnapshot {
  id: number;
  sync_run_id: number;
  voice_id: string;
  name: string;
  description: string;
  language: string;
  preview_url: string;
  metadata_json?: string | null;
  fetched_at: string;
}

export interface XAIVoicesResponse {
  latest_run: XAIVoiceSyncRun | null;
  latest_change_summary?: ProviderModelChangeSummary | null;
  voices: XAIVoiceSnapshot[];
}

export interface OpenAITTSVoiceSyncRun {
  id: number;
  started_at: string;
  finished_at?: string | null;
  last_progress_at?: string | null;
  status: string;
  trigger_type: string;
  fetched_count: number;
  saved_count: number;
  error_message?: string | null;
}

export interface OpenAITTSVoiceSnapshot {
  id: number;
  sync_run_id: number;
  voice_id: string;
  name: string;
  description: string;
  language: string;
  preview_url: string;
  metadata_json?: string | null;
  fetched_at: string;
}

export interface OpenAITTSVoicesResponse {
  latest_run: OpenAITTSVoiceSyncRun | null;
  latest_change_summary?: ProviderModelChangeSummary | null;
  voices: OpenAITTSVoiceSnapshot[];
}

export interface GeminiTTSVoiceCatalogEntry {
  voice_name: string;
  label: string;
  tone: string;
  description: string;
  sample_audio_path: string;
}

export interface GeminiTTSVoicesResponse {
  catalog_name: string;
  provider: string;
  source: string;
  voices: GeminiTTSVoiceCatalogEntry[];
}

export interface FishModelAuthor {
  id?: string | null;
  name?: string | null;
  username?: string | null;
  avatar_url?: string | null;
}

export interface FishModelTag {
  name: string;
}

export interface FishModelSample {
  audio_url: string;
  text?: string | null;
  transcript?: string | null;
  duration_seconds?: number | null;
}

export interface FishModelSnapshot {
  _id: string;
  title: string;
  description: string;
  cover_image?: string | null;
  tags: FishModelTag[];
  languages: string[];
  visibility: string;
  like_count: number;
  task_count: number;
  author?: FishModelAuthor | null;
  samples: FishModelSample[];
  fetched_at: string;
  updated_at?: string | null;
}

export type FishBrowseSort = "recommended" | "trending" | "latest";

export interface FishBrowseResponse {
  items: FishModelSnapshot[];
  page: number;
  page_size: number;
  total: number;
  has_more: boolean;
}

export interface ProviderModelSnapshotEntry {
  provider: string;
  model_id: string;
  fetched_at: string;
  status: string;
  error?: string | null;
}

export interface ProviderModelSnapshotListResponse {
  items: ProviderModelSnapshotEntry[];
  providers: string[];
  total: number;
  limit: number;
  offset: number;
}

export interface ProviderModelSnapshotSyncSummary {
  providers: number;
  changes: number;
}

export interface AivisUserDictionary {
  uuid: string;
  name: string;
  description?: string | null;
  word_count: number;
  created_at: string;
  updated_at: string;
}

export interface AivisUserDictionariesResponse {
  user_dictionaries: AivisUserDictionary[];
}

export interface PoeModelSnapshot {
  model_id: string;
  canonical_slug?: string | null;
  display_name: string;
  owned_by: string;
  description_en?: string | null;
  description_ja?: string | null;
  context_length?: number | null;
  pricing_json: Record<string, unknown> | string;
  architecture_json: Record<string, unknown> | string;
  modality_flags_json: Record<string, unknown> | string;
  is_active: boolean;
  transport_supports_openai_compat: boolean;
  transport_supports_anthropic_compat: boolean;
  preferred_transport: "openai" | "anthropic" | string;
  fetched_at: string;
}

export interface PoeModelsResponse {
  latest_run: PoeSyncRun | null;
  latest_change_summary?: ProviderModelChangeSummary | null;
  models: PoeModelSnapshot[];
  removed_models?: PoeModelSnapshot[];
}

export interface PoeUsageSummary {
  entry_count: number;
  api_entry_count: number;
  chat_entry_count: number;
  total_cost_points: number;
  total_cost_usd: number;
  average_cost_points: number;
  average_cost_usd: number;
  latest_entry_at?: string | null;
}

export interface PoeUsageModelSummary {
  bot_name: string;
  entry_count: number;
  total_cost_points: number;
  total_cost_usd: number;
  average_cost_points: number;
  average_cost_usd: number;
  latest_entry_at?: string | null;
}

export interface PoeUsageEntry {
  query_id: string;
  bot_name: string;
  created_at: string;
  cost_usd: number;
  raw_cost_usd: string;
  cost_points: number;
  cost_breakdown_in_points: Record<string, string>;
  usage_type: string;
  chat_name?: string | null;
}

export interface PoeUsageResponse {
  configured: boolean;
  selected_range: string;
  range_started_at?: string | null;
  range_ended_at?: string | null;
  current_point_balance?: number | null;
  summary: PoeUsageSummary;
  model_summaries: PoeUsageModelSummary[];
  entries: PoeUsageEntry[];
  entry_limit: number;
  available_ranges: { key: string }[];
  last_sync_run?: {
    id: string;
    user_id: string;
    started_at: string;
    finished_at?: string | null;
    status: string;
    sync_source: string;
    fetched_count: number;
    inserted_count: number;
    updated_count: number;
    latest_entry_at?: string | null;
    oldest_entry_at?: string | null;
    error_message?: string | null;
  } | null;
}

export interface OpenRouterModelSnapshot {
  model_id: string;
  canonical_slug?: string | null;
  provider_slug: string;
  display_name: string;
  description_en?: string | null;
  description_ja?: string | null;
  context_length?: number | null;
  pricing_json: Record<string, unknown> | string;
  supported_parameters_json: string[] | string;
  architecture_json: Record<string, unknown> | string;
  top_provider_json: Record<string, unknown> | string;
  modality_flags_json: Record<string, unknown> | string;
  is_text_generation: boolean;
  is_active: boolean;
  fetched_at: string;
}

export interface OpenRouterModelListEntry extends OpenRouterModelSnapshot {
  availability: "available" | "constrained" | "removed";
  raw_availability: "available" | "constrained" | "removed";
  reason?: string | null;
  recent_change?: "available" | "constrained" | "removed" | null;
  override_enabled: boolean;
}

export interface OpenRouterModelsResponse {
  latest_run: OpenRouterSyncRun | null;
  latest_change_summary?: ProviderModelChangeSummary | null;
  models: OpenRouterModelListEntry[];
  unavailable_models: OpenRouterModelListEntry[];
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

export interface AINavigatorBriefItem {
  id: string;
  brief_id: string;
  rank: number;
  item_id: string;
  title_snapshot: string;
  translated_title_snapshot: string;
  source_title_snapshot: string;
  comment: string;
  created_at: string;
}

export interface AINavigatorBrief {
  id: string;
  user_id: string;
  slot: "morning" | "noon" | "evening" | string;
  status: "queued" | "generated" | "failed" | "notified" | string;
  title: string;
  intro: string;
  summary: string;
  ending: string;
  persona: string;
  model: string;
  source_window_start?: string | null;
  source_window_end?: string | null;
  generated_at?: string | null;
  notification_sent_at?: string | null;
  error_message?: string | null;
  created_at: string;
  updated_at: string;
  items?: AINavigatorBriefItem[];
}

export interface AINavigatorBriefListResponse {
  items: AINavigatorBrief[];
}

export interface AINavigatorBriefDetailResponse {
  brief?: AINavigatorBrief | null;
}

export interface AINavigatorBriefSummaryAudioQueueResponse {
  brief_id: string;
  count: number;
  items: Item[];
}

export type PlaybackMode = "summary_queue" | "audio_briefing";
export type PlaybackSessionStatus = "in_progress" | "completed" | "interrupted";

export interface PlaybackSession {
  id: string;
  user_id: string;
  mode: PlaybackMode;
  status: PlaybackSessionStatus;
  title: string;
  subtitle: string;
  current_position_sec: number;
  duration_sec: number;
  progress_ratio?: number | null;
  resume_payload: Record<string, unknown>;
  started_at: string;
  updated_at: string;
  completed_at?: string | null;
}

export interface LatestPlaybackSessionsResponse {
  summary_queue: PlaybackSession | null;
  audio_briefing: PlaybackSession | null;
}

export interface PlaybackSessionsResponse {
  items: PlaybackSession[];
}

export interface CreatePlaybackSessionRequest {
  mode: PlaybackMode;
  title: string;
  subtitle: string;
  current_position_sec: number;
  duration_sec: number;
  progress_ratio?: number | null;
  resume_payload: Record<string, unknown>;
}

export interface UpdatePlaybackSessionRequest {
  title: string;
  subtitle: string;
  current_position_sec: number;
  duration_sec: number;
  progress_ratio?: number | null;
  resume_payload: Record<string, unknown>;
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

export interface BulkRetryItemsResult {
  status: "queued";
  item_ids: string[];
  queued_count: number;
  skipped_count: number;
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
  immediate_enabled: boolean;
  briefing_enabled: boolean;
  review_enabled: boolean;
  goal_match_enabled: boolean;
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
  provider: "anthropic" | "google" | "groq" | "openai" | "deepseek" | "alibaba" | "mistral" | "xai" | "zai" | string;
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

export interface NavigatorPersonaTaskHints {
  comment_range?: string;
  intro_range?: string;
  intro_style?: string;
  style?: string;
}

export interface NavigatorPersonaSamplingProfile {
  temperature_hint?: "low" | "medium" | "medium_high" | string;
  top_p_hint?: "narrow" | "balanced" | "wide" | string;
  verbosity_hint?: "tight" | "balanced" | "expansive" | string;
}

export interface NavigatorPersonaDefinition {
  name: string;
  gender: string;
  age_vibe: string;
  first_person: string;
  speech_style: string;
  occupation: string;
  experience: string;
  personality: string;
  values: string;
  interests: string;
  dislikes: string;
  voice: string;
  sampling_profile?: NavigatorPersonaSamplingProfile;
  briefing?: NavigatorPersonaTaskHints;
  item?: NavigatorPersonaTaskHints;
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
  has_moonshot_api_key: boolean;
  moonshot_api_key_last4: string | null;
  has_xai_api_key: boolean;
  xai_api_key_last4: string | null;
  has_zai_api_key: boolean;
  zai_api_key_last4: string | null;
  has_fireworks_api_key: boolean;
  fireworks_api_key_last4: string | null;
  has_poe_api_key: boolean;
  poe_api_key_last4: string | null;
  has_siliconflow_api_key: boolean;
  siliconflow_api_key_last4: string | null;
  has_openrouter_api_key: boolean;
  openrouter_api_key_last4: string | null;
  has_aivis_api_key: boolean;
  aivis_api_key_last4: string | null;
  has_fish_api_key?: boolean;
  fish_api_key_last4?: string | null;
  ui_font_sans_key?: string;
  ui_font_serif_key?: string;
  aivis_user_dictionary_uuid?: string | null;
  gemini_tts_enabled?: boolean;
  podcast?: PodcastSettings;
  has_inoreader_oauth?: boolean;
  inoreader_token_expires_at?: string | null;
  monthly_budget_usd: number | null;
  budget_alert_enabled: boolean;
  budget_alert_threshold_pct: number;
  digest_email_enabled: boolean;
  reading_plan: UserReadingPlanSettings;
  llm_models?: {
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
    faithfulness_check?: string | null;
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
    fish_preprocess_model?: string | null;
  };
  audio_briefing?: AudioBriefingSettings;
  audio_briefing_persona_voices?: AudioBriefingPersonaVoice[];
  summary_audio?: SummaryAudioVoiceSettings | null;
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

export interface UIFontCatalogEntry {
  key: string;
  label: string;
  family: string;
  category: "sans" | "serif" | "display";
  selectable_for_sans: boolean;
  selectable_for_serif: boolean;
  preview_ui: string;
  preview_body: string;
}

export interface UIFontCatalogResponse {
  catalog_name: string;
  source: string;
  source_reference: string;
  fonts: UIFontCatalogEntry[];
}

export interface AudioBriefingSettings {
  enabled: boolean;
  schedule_mode?: "interval" | "fixed_slots_3x" | string | null;
  interval_hours: number;
  articles_per_episode: number;
  target_duration_minutes: number;
  chunk_trailing_silence_seconds: number;
  program_name?: string | null;
  default_persona_mode: string;
  default_persona: string;
  conversation_mode: "single" | "duo" | string;
  bgm_enabled: boolean;
  bgm_r2_prefix?: string | null;
}

export interface PodcastCategoryOption {
  category: string;
  subcategories: string[];
}

export interface PodcastSettings {
  enabled: boolean;
  feed_slug?: string | null;
  rss_url?: string | null;
  title?: string | null;
  description?: string | null;
  author?: string | null;
  language: string;
  category?: string | null;
  subcategory?: string | null;
  available_categories?: PodcastCategoryOption[];
  explicit: boolean;
  artwork_url?: string | null;
}

export interface AudioBriefingPersonaVoice {
  persona: string;
  tts_provider: string;
  tts_model: string;
  voice_model: string;
  voice_style: string;
  provider_voice_label?: string;
  provider_voice_description?: string;
  speech_rate: number;
  emotional_intensity: number;
  tempo_dynamics: number;
  line_break_silence_seconds: number;
  pitch: number;
  volume_gain: number;
}

export interface AudioBriefingPreset {
  id: string;
  user_id: string;
  name: string;
  default_persona_mode: "fixed" | "random" | string;
  default_persona: string;
  conversation_mode: "single" | "duo" | string;
  voices: AudioBriefingPersonaVoice[];
  created_at: string;
  updated_at: string;
}

export interface SaveAudioBriefingPresetRequest {
  name: string;
  default_persona_mode: "fixed" | "random" | string;
  default_persona: string;
  conversation_mode: "single" | "duo" | string;
  voices: AudioBriefingPersonaVoice[];
}

export interface SummaryAudioVoiceSettings {
  tts_provider: string;
  tts_model: string;
  voice_model: string;
  voice_style: string;
  provider_voice_label?: string;
  provider_voice_description?: string;
  speech_rate: number;
  emotional_intensity: number;
  tempo_dynamics: number;
  line_break_silence_seconds: number;
  pitch: number;
  volume_gain: number;
  aivis_user_dictionary_uuid?: string | null;
}

export interface AudioBriefingJob {
  id: string;
  user_id: string;
  slot_started_at_jst: string;
  slot_key: string;
  persona: string;
  conversation_mode?: "single" | "duo" | string;
  partner_persona?: string | null;
  pipeline_stage?: string | null;
  status: string;
  archive_status: "active" | "archived" | string;
  source_item_count: number;
  reused_item_count: number;
  script_char_count: number;
  script_llm_models?: string | null;
  prompt_key?: string | null;
  prompt_source?: string | null;
  prompt_version_id?: string | null;
  prompt_version_number?: number | null;
  prompt_experiment_id?: string | null;
  prompt_experiment_arm_id?: string | null;
  audio_duration_sec?: number | null;
  title?: string | null;
  r2_audio_object_key?: string | null;
  r2_manifest_object_key?: string | null;
  bgm_object_key?: string | null;
  provider_job_id?: string | null;
  idempotency_key?: string | null;
  error_code?: string | null;
  error_message?: string | null;
  published_at?: string | null;
  failed_at?: string | null;
  created_at: string;
  updated_at: string;
}

export interface AudioBriefingJobItem {
  item_id: string;
  rank: number;
  segment_title?: string | null;
  summary_snapshot?: string | null;
  title?: string | null;
  translated_title?: string | null;
  source_title?: string | null;
  published_at?: string | null;
}

export interface AudioBriefingScriptChunk {
  seq: number;
  part_type: string;
  speaker?: "host" | "partner" | string | null;
  text: string;
  preprocessed_text?: string | null;
  char_count: number;
  tts_status: string;
  tts_provider?: string | null;
  voice_model?: string | null;
  provider_voice_label?: string | null;
  voice_style?: string | null;
  r2_audio_object_key?: string | null;
  duration_sec?: number | null;
  error_message?: string | null;
}

export interface AudioBriefingUsedTTS {
  provider: string;
  tts_model?: string | null;
  host_voice_model?: string | null;
  host_voice_label?: string | null;
  partner_voice_model?: string | null;
  partner_voice_label?: string | null;
}

export interface AudioBriefingDetailResponse {
  job: AudioBriefingJob;
  items: AudioBriefingJobItem[];
  chunks: AudioBriefingScriptChunk[];
  used_tts?: AudioBriefingUsedTTS | null;
  audio_url?: string | null;
  delete_allowed?: boolean;
  resume_allowed?: boolean;
  archive_allowed?: boolean;
  unarchive_allowed?: boolean;
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
  navigator?: {
    enabled: boolean;
    persona: string;
    character_name: string;
    character_title: string;
    avatar_style: string;
    speech_style: string;
    intro: string;
    generated_at?: string | null;
    picks: Array<{
      item_id: string;
      rank: number;
      title: string;
      source_title?: string | null;
      comment: string;
      reason_tags?: string[];
    }>;
    llm?: NavigatorLLM | null;
  } | null;
}

export interface BriefingNavigatorResponse {
  navigator?: BriefingTodayResponse["navigator"];
}

export interface NavigatorLLM {
  provider: string;
  model: string;
  requested_model?: string | null;
  resolved_model?: string | null;
}

export interface ItemNavigatorResponse {
  navigator?: {
    enabled: boolean;
    item_id: string;
    persona: string;
    character_name: string;
    character_title: string;
    avatar_style: string;
    speech_style: string;
    headline: string;
    commentary: string;
    stance_tags?: string[];
    generated_at?: string | null;
    llm?: NavigatorLLM | null;
  } | null;
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

export interface AskLLM {
  provider: string;
  model: string;
  pricing_source?: string;
}

export interface AskResponse {
  query: string;
  answer: string;
  bullets?: string[];
  citations?: AskCitation[];
  related_items?: AskCandidate[];
  ask_llm?: AskLLM | null;
}

export interface AskNavigator {
  enabled: boolean;
  persona: string;
  character_name: string;
  character_title: string;
  avatar_style: string;
  speech_style: string;
  headline: string;
  commentary: string;
  next_angles?: string[];
  generated_at?: string | null;
}

export interface AskNavigatorResponse {
  navigator?: AskNavigator | null;
}

async function clientFetch<T>(path: string, options?: RequestInit, opts?: { apiPrefix?: boolean }): Promise<T> {
  const requestPath = withCacheBust(path, options?.method);
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
  getItems: (params?: { status?: string; source_id?: string; topic?: string; q?: string; search_mode?: string; page?: number; page_size?: number; sort?: string; unread_only?: boolean; read_only?: boolean; favorite_only?: boolean; later_only?: boolean }) => {
    const q = new URLSearchParams();
    if (params?.status) q.set("status", params.status);
    if (params?.source_id) q.set("source_id", params.source_id);
    if (params?.topic) q.set("topic", params.topic);
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
    return apiFetch<BriefingNavigatorResponse>(`/briefing/navigator${qs ? `?${qs}` : ""}`);
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
  getLLMExecutionCurrentMonthSummary: (params?: { days?: number }) => {
    const q = new URLSearchParams();
    if (params?.days) q.set("days", String(params.days));
    const qs = q.toString();
    return apiFetch<LLMExecutionCurrentMonthSummary[]>(`/llm-usage/current-month/execution-summary${qs ? `?${qs}` : ""}`);
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
    faithfulness_check?: string | null;
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
    fish_preprocess_model?: string | null;
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
  deleteFireworksApiKey: () =>
    apiFetch<{ user_id: string; has_fireworks_api_key: boolean; fireworks_api_key_last4: string | null }>(
      "/settings/fireworks-key",
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
  getGeminiTTSVoices: () =>
    apiFetch<GeminiTTSVoicesResponse>("/gemini-tts-voices"),
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
