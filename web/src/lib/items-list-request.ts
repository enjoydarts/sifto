export type ItemsListParams = {
  status?: string;
  source_id?: string;
  topic?: string;
  genre?: string;
  q?: string;
  search_mode?: string;
  page?: number;
  page_size?: number;
  sort?: string;
  unread_only?: boolean;
  read_only?: boolean;
  favorite_only?: boolean;
  later_only?: boolean;
};

export function itemsListPath(params?: ItemsListParams): string {
  const query = new URLSearchParams();
  for (const [key, value] of Object.entries(params ?? {})) {
    if (value || typeof value === "boolean") query.set(key, String(value));
  }
  const encoded = query.toString();
  return encoded ? `/items?${encoded}` : "/items";
}
