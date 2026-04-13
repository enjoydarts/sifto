import type { Item } from "./items";

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
