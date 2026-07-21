"use client";

import * as Sentry from "@sentry/nextjs";
import { useEffect, useRef } from "react";

const routeTransitionMark = "sifto:route-transition-start";
const recentRouteTransitionWindowMs = 5_000;

export function usePrimaryContentTiming({
  route,
  ready,
  transitionKey = route,
}: {
  route: string;
  ready: boolean;
  transitionKey?: string;
}) {
  const mountStartedAtRef = useRef<number | null>(null);
  const recordedRef = useRef(false);
  const previousTransitionKeyRef = useRef(transitionKey);

  useEffect(() => {
    const hasPerformanceNow = typeof performance !== "undefined" && typeof performance.now === "function";
    if (previousTransitionKeyRef.current !== transitionKey) {
      previousTransitionKeyRef.current = transitionKey;
      recordedRef.current = false;
      mountStartedAtRef.current = hasPerformanceNow ? performance.now() : null;
    }
    if (!hasPerformanceNow || typeof requestAnimationFrame !== "function") {
      recordedRef.current = true;
      return;
    }
    if (mountStartedAtRef.current === null) {
      mountStartedAtRef.current = performance.now();
    }
    if (!ready || recordedRef.current) return;

    const frameID = requestAnimationFrame(() => {
      if (recordedRef.current) return;

      try {
        const endedAt = performance.now();
        const mountStartedAt = mountStartedAtRef.current ?? endedAt;
        const routeMark =
          typeof performance.getEntriesByName === "function"
            ? performance.getEntriesByName(routeTransitionMark, "mark").at(-1)
            : undefined;
        const routeStartedAt = routeMark?.startTime;
        const startedAt =
          routeStartedAt !== undefined &&
          routeStartedAt <= endedAt &&
          routeStartedAt >= mountStartedAt - recentRouteTransitionWindowMs
            ? routeStartedAt
            : mountStartedAt;
        const durationMs = Math.max(0, endedAt - startedAt);

        try {
          if (typeof performance.measure === "function") {
            performance.measure(`sifto:${route}:primary-content`, {
              start: startedAt,
              end: endedAt,
              detail: { route },
            });
          }
        } catch {
          // Measurement support differs across runtimes and must not affect rendering.
        }
        try {
          Sentry.getActiveSpan()?.setAttributes({
            "ui.route": route,
            "ui.primary_content_ms": durationMs,
          });
        } catch {
          // Telemetry is best-effort.
        }
      } finally {
        recordedRef.current = true;
      }
    });

    return () => {
      if (typeof cancelAnimationFrame === "function") cancelAnimationFrame(frameID);
    };
  }, [ready, route, transitionKey]);
}
