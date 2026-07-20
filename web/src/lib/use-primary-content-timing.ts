"use client";

import * as Sentry from "@sentry/nextjs";
import { useEffect, useRef } from "react";

const routeTransitionMark = "sifto:route-transition-start";
const recentRouteTransitionWindowMs = 5_000;

export function usePrimaryContentTiming({ route, ready }: { route: string; ready: boolean }) {
  const mountStartedAtRef = useRef<number | null>(null);
  const recordedRef = useRef(false);

  useEffect(() => {
    if (mountStartedAtRef.current === null) {
      mountStartedAtRef.current = performance.now();
    }
    if (!ready || recordedRef.current) return;

    const frameID = requestAnimationFrame(() => {
      if (recordedRef.current) return;

      const endedAt = performance.now();
      const mountStartedAt = mountStartedAtRef.current ?? endedAt;
      const routeMark = performance.getEntriesByName(routeTransitionMark, "mark").at(-1);
      const routeStartedAt = routeMark?.startTime;
      const startedAt =
        routeStartedAt !== undefined &&
        routeStartedAt <= endedAt &&
        routeStartedAt >= mountStartedAt - recentRouteTransitionWindowMs
          ? routeStartedAt
          : mountStartedAt;
      const durationMs = Math.max(0, endedAt - startedAt);

      try {
        performance.measure(`sifto:${route}:primary-content`, {
          start: startedAt,
          end: endedAt,
          detail: { route },
        });
        Sentry.getActiveSpan()?.setAttributes({
          "ui.route": route,
          "ui.primary_content_ms": durationMs,
        });
      } finally {
        recordedRef.current = true;
      }
    });

    return () => cancelAnimationFrame(frameID);
  }, [ready, route]);
}
