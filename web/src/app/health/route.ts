import { NextResponse } from "next/server";

export const dynamic = "force-dynamic";

type CheckStatus = "ok" | "error";

type HealthCheck = {
  status: CheckStatus;
  latency_ms?: number;
  http_status?: number;
  detail?: string;
};

async function checkAPI(): Promise<HealthCheck> {
  const apiUrl = process.env.NEXT_PUBLIC_API_URL ?? "http://localhost:8080";
  const controller = new AbortController();
  const timeout = setTimeout(() => controller.abort(), 3000);
  const startedAt = Date.now();

  try {
    const res = await fetch(`${apiUrl}/health`, {
      method: "GET",
      cache: "no-store",
      signal: controller.signal,
    });
    const body = (await res.text()).trim();

    return {
      status: res.ok ? "ok" : "error",
      latency_ms: Date.now() - startedAt,
      http_status: res.status,
      detail: body || res.statusText,
    };
  } catch (error) {
    const detail =
      error instanceof Error ? error.message : "unknown fetch error";
    return {
      status: "error",
      latency_ms: Date.now() - startedAt,
      detail,
    };
  } finally {
    clearTimeout(timeout);
  }
}

export async function GET() {
  const api = await checkAPI();
  const ok = api.status === "ok";

  return NextResponse.json(
    {
      status: ok ? "ok" : "degraded",
      checked_at: new Date().toISOString(),
      checks: {
        web: {
          status: "ok",
        },
        api,
      },
    },
    {
      status: ok ? 200 : 503,
      headers: {
        "Cache-Control": "no-store",
      },
    }
  );
}

export async function HEAD() {
  const api = await checkAPI();
  return new NextResponse(null, {
    status: api.status === "ok" ? 200 : 503,
    headers: {
      "Cache-Control": "no-store",
    },
  });
}
