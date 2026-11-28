import type { ItemListResponse } from "@/types/api/items";

export type ItemsInitialFeed = {
  userId: string;
  query: string;
  dataUpdatedAt: number;
  response: ItemListResponse;
};

export function matchesItemsInitialFeed(
  feed: Pick<ItemsInitialFeed, "userId" | "query"> | undefined,
  userId: string | null | undefined,
  query: string
): boolean {
  return feed !== undefined && feed.userId === userId && feed.query === query;
}
