import type { Item, ItemSummary, ItemSummaryLLM } from "./items";

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
