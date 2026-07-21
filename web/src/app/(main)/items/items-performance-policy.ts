export const ITEMS_FEED_STALE_TIME_MS = 30_000;
export const ITEM_DETAIL_STALE_TIME_MS = 5 * 60_000;
export const ITEM_RELATED_STALE_TIME_MS = 5 * 60_000;
export const itemsPrimaryContentRoute = "items";
export const itemDetailPrimaryContentRoute = "item-detail";

const ITEMS_FEED_PAGE_KEY_INDEX = 8;

export function isItemScopedStateCurrent(
  requestedItemId: string,
  stateItemId: string | null
): boolean {
  return stateItemId === requestedItemId;
}

export function isItemDetailPrimaryContentReady({
  requestedItemId,
  displayedItemId,
  loading,
  hasError,
}: {
  requestedItemId: string;
  displayedItemId: string | null | undefined;
  loading: boolean;
  hasError: boolean;
}): boolean {
  return displayedItemId === requestedItemId && !loading && !hasError;
}

export function isItemsFeedPrimaryLoading({
  hasData,
  isLoading,
  isFetching,
  isPlaceholderData,
}: {
  hasData: boolean;
  isLoading: boolean;
  isFetching: boolean;
  isPlaceholderData: boolean;
}): boolean {
  return isPlaceholderData || (!hasData && (isLoading || isFetching));
}

export function canReuseItemsFeedPlaceholder(
  previousKey: readonly unknown[] | undefined,
  currentKey: readonly unknown[]
): boolean {
  if (!previousKey || previousKey.length !== currentKey.length) return false;

  return currentKey.every(
    (value, index) => index === ITEMS_FEED_PAGE_KEY_INDEX || Object.is(previousKey[index], value)
  );
}
