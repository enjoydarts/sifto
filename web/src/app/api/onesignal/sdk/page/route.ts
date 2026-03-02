const ONESIGNAL_PAGE_SDK_URL = "https://cdn.onesignal.com/sdks/web/v16/OneSignalSDK.page.js";

export const runtime = "nodejs";

export async function GET() {
  const controller = new AbortController();
  const timeout = setTimeout(() => controller.abort(), 10000);
  try {
    const res = await fetch(ONESIGNAL_PAGE_SDK_URL, {
      cache: "no-store",
      signal: controller.signal,
    });
    if (!res.ok) {
      return new Response("OneSignal SDK fetch failed", { status: 502 });
    }
    const body = await res.text();
    return new Response(body, {
      status: 200,
      headers: {
        "Content-Type": "application/javascript; charset=utf-8",
        "Cache-Control": "public, max-age=300, s-maxage=300, stale-while-revalidate=600",
      },
    });
  } catch {
    return new Response("OneSignal SDK fetch timeout", { status: 504 });
  } finally {
    clearTimeout(timeout);
  }
}
