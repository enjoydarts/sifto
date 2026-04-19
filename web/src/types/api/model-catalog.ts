import type { ProviderModelChangeSummary } from "./common";

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

export interface FeatherlessSyncRun {
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

export interface FeatherlessSyncStatusResponse {
  run: FeatherlessSyncRun | null;
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

export interface FeatherlessModelSnapshot {
  provider_slug: string;
  display_name: string;
  model_id: string;
  model_class?: string | null;
  context_length?: number | null;
  max_completion_tokens?: number | null;
  is_gated: boolean;
  available_on_current_plan: boolean;
  fetched_at: string;
}

export interface FeatherlessModelListEntry extends FeatherlessModelSnapshot {
  availability: "available" | "unavailable" | "removed" | string;
  raw_availability?: "available" | "unavailable" | "removed" | "not_on_plan" | string;
  reason?: "not_on_plan" | "removed" | string | null;
  recent_change?: "added" | "availability_changed" | "gated_changed" | "removed" | string | null;
  gated?: boolean;
  available_on_current_plan: boolean;
}

export interface FeatherlessModelsResponse {
  latest_run: FeatherlessSyncRun | null;
  latest_change_summary?: ProviderModelChangeSummary | null;
  models: FeatherlessModelListEntry[];
  unavailable_models: FeatherlessModelListEntry[];
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
  source_provider?: string;
  available_purposes: string[];
  recommendation?: "recommended" | "strong" | "experimental" | string;
  best_for?: "facts" | "summary" | "ask" | "digest" | "embedding" | "balanced" | string;
  highlights?: Array<"lowestCost" | "fast" | "jsonStable" | string>;
  comment?: string;
  availability?: "available" | "unavailable" | "removed" | string;
  raw_availability?: "available" | "unavailable" | "removed" | "not_on_plan" | string;
  reason?: "not_on_plan" | "removed" | string | null;
  gated?: boolean;
  available_on_current_plan?: boolean;
  capabilities?: LLMCatalogModelCapabilities | null;
  pricing?: LLMCatalogModelPricing | null;
}

export interface LLMCatalog {
  providers: LLMCatalogProvider[];
  chat_models: LLMCatalogModel[];
  embedding_models: LLMCatalogModel[];
}
