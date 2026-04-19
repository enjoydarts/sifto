import type { PodcastSettings } from "./podcasts";
import type { AudioBriefingSettings, AudioBriefingPersonaVoice, SummaryAudioVoiceSettings } from "./audio-briefing";

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
  has_minimax_api_key: boolean;
  minimax_api_key_last4: string | null;
  has_xiaomi_mimo_token_plan_api_key: boolean;
  xiaomi_mimo_token_plan_api_key_last4: string | null;
  has_xai_api_key: boolean;
  xai_api_key_last4: string | null;
  has_zai_api_key: boolean;
  zai_api_key_last4: string | null;
  has_fireworks_api_key: boolean;
  fireworks_api_key_last4: string | null;
  has_together_api_key: boolean;
  together_api_key_last4: string | null;
  has_poe_api_key: boolean;
  poe_api_key_last4: string | null;
  has_siliconflow_api_key: boolean;
  siliconflow_api_key_last4: string | null;
  has_azure_speech_api_key?: boolean;
  azure_speech_api_key_last4?: string | null;
  azure_speech_region?: string | null;
  has_openrouter_api_key: boolean;
  openrouter_api_key_last4: string | null;
  has_featherless_api_key?: boolean;
  featherless_api_key_last4?: string | null;
  has_aivis_api_key: boolean;
  aivis_api_key_last4: string | null;
  has_elevenlabs_api_key?: boolean;
  elevenlabs_api_key_last4?: string | null;
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
    tts_markup_preprocess_model?: string | null;
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

export interface ObsidianExportRunResult {
  user_id: string;
  updated: number;
  skipped: number;
  failed: number;
}
