import { NextRequest } from "next/server";

export const revalidate = 300;
export const dynamicParams = true;

const PODCAST_FEED_CACHE_CONTROL = "public, max-age=300, s-maxage=600, stale-while-revalidate=1800";

function resolveAPIBaseURL(): string {
  const explicit = process.env.NEXT_PUBLIC_API_URL ?? process.env.API_URL;
  if (explicit && explicit.trim()) {
    return explicit.trim().replace(/\/+$/, "");
  }
  if (process.env.VERCEL === "1" || process.env.NODE_ENV === "production") {
    return "https://api.sifto.net";
  }
  return "http://api:8080";
}

async function fetchPodcastFeed(slug: string, method: "GET" | "HEAD"): Promise<Response> {
  const upstreamURL = `${resolveAPIBaseURL()}/podcasts/${encodeURIComponent(slug)}/feed.xml`;
  const upstream = await fetch(upstreamURL, {
    method,
    next: { revalidate },
    headers: {
      Accept: "application/rss+xml, application/xml;q=0.9, text/xml;q=0.8, */*;q=0.1",
    },
  });

  const headers = new Headers();
  headers.set("Content-Type", upstream.headers.get("Content-Type") || "application/rss+xml; charset=utf-8");
  const contentLength = upstream.headers.get("Content-Length");
  if (contentLength) {
    headers.set("Content-Length", contentLength);
  }
  const etag = upstream.headers.get("ETag");
  if (etag) {
    headers.set("ETag", etag);
  }
  const lastModified = upstream.headers.get("Last-Modified");
  if (lastModified) {
    headers.set("Last-Modified", lastModified);
  }
  headers.set("Cache-Control", upstream.headers.get("Cache-Control") || PODCAST_FEED_CACHE_CONTROL);

  return new Response(upstream.body, {
    status: upstream.status,
    headers,
  });
}

export async function GET(_req: NextRequest, context: { params: Promise<{ slug: string }> }) {
  const { slug } = await context.params;
  return fetchPodcastFeed(slug, "GET");
}

export async function HEAD(_req: NextRequest, context: { params: Promise<{ slug: string }> }) {
  const { slug } = await context.params;
  const response = await fetchPodcastFeed(slug, "HEAD");
  return new Response(null, {
    status: response.status,
    headers: response.headers,
  });
}
