export const ITEMS_DOCUMENT_HEADER = "x-sifto-items-document";

export function itemsSSREnabled(): boolean {
  return process.env.ITEMS_SSR_PILOT !== "0";
}

export function itemsSSRRequested(headers: Headers): boolean {
  return headers.get(ITEMS_DOCUMENT_HEADER) === "1";
}
