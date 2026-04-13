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
