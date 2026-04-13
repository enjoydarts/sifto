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

export interface AskCitation {
  item_id: string;
  title: string;
  url: string;
  reason?: string;
  published_at?: string | null;
  topics?: string[];
}

import type { Item } from "./items";

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
