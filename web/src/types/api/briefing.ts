import type { Item } from "./items";

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
