"use client";

import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { useI18n } from "@/components/i18n-provider";
import { useToast } from "@/components/toast-provider";

export interface ModelCatalogConfig<TData> {
  fetchData: () => Promise<TData>;
  syncData?: () => Promise<TData>;
  syncSuccessKey?: string;
  isSyncRunning?: (data: TData | null) => boolean;
}

export function useModelCatalog<TData>(config: ModelCatalogConfig<TData>) {
  const { t } = useI18n();
  const { showToast } = useToast();
  const [loading, setLoading] = useState(true);
  const [syncing, setSyncing] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [data, setData] = useState<TData | null>(null);
  const [query, setQuery] = useState("");

  const fetchDataRef = useRef(config.fetchData);
  fetchDataRef.current = config.fetchData;
  const syncDataRef = useRef(config.syncData);
  syncDataRef.current = config.syncData;
  const syncSuccessKeyRef = useRef(config.syncSuccessKey);
  syncSuccessKeyRef.current = config.syncSuccessKey;

  const defaultIsSyncRunning = useMemo(() => (d: TData | null) => {
    const run = (d as Record<string, unknown> | null)?.latest_run as Record<string, unknown> | null | undefined;
    return run?.status === "running" && run?.trigger_type === "manual";
  }, []);

  const isRunning = config.isSyncRunning ?? defaultIsSyncRunning;

  const load = useCallback(async (options?: { silent?: boolean }) => {
    if (!options?.silent) setLoading(true);
    try {
      const next = await fetchDataRef.current();
      setData(next);
      setError(null);
    } catch (e) {
      setError(String(e));
    } finally {
      if (!options?.silent) setLoading(false);
    }
  }, []);

  useEffect(() => { load(); }, [load]);

  useEffect(() => {
    if (!isRunning(data)) return;
    const timer = window.setInterval(async () => {
      try {
        const next = await fetchDataRef.current();
        setData(next);
      } catch (e) {
        setError(String(e));
      }
    }, 3000);
    return () => window.clearInterval(timer);
  }, [data, isRunning]);

  const handleSync = useCallback(async () => {
    if (!syncDataRef.current) return;
    setSyncing(true);
    try {
      const next = await syncDataRef.current();
      setData(next);
      setError(null);
      const key = syncSuccessKeyRef.current;
      if (key) showToast(t(key), "success");
    } catch (e) {
      showToast(String(e), "error");
    } finally {
      setSyncing(false);
    }
  }, [showToast, t]);

  return { loading, syncing, error, data, setData, query, setQuery, load, handleSync };
}

export function useModelSort<S extends string>(defaultKey: S, descendingDefaults: S[] = []) {
  const [sortKey, setSortKey] = useState<S>(defaultKey);
  const [sortDirection, setSortDirection] = useState<"asc" | "desc">("asc");

  const setSort = useCallback(
    (nextKey: S) => {
      if (sortKey === nextKey) {
        setSortDirection((d) => (d === "asc" ? "desc" : "asc"));
        return;
      }
      setSortKey(nextKey);
      setSortDirection(descendingDefaults.includes(nextKey) ? "desc" : "asc");
    },
    [sortKey, descendingDefaults],
  );

  const sortMarker = useCallback(
    (key: S) => {
      if (sortKey !== key) return "";
      return sortDirection === "asc" ? " ↑" : " ↓";
    },
    [sortKey, sortDirection],
  );

  return { sortKey, sortDirection, setSort, sortMarker };
}

export function parseObject(raw: unknown): Record<string, unknown> {
  if (!raw) return {};
  if (typeof raw === "string") {
    try {
      return JSON.parse(raw) as Record<string, unknown>;
    } catch {
      return {};
    }
  }
  if (typeof raw === "object") return raw as Record<string, unknown>;
  return {};
}

export function formatPrice(value: unknown) {
  const num = typeof value === "number" ? value : typeof value === "string" ? Number(value) : NaN;
  if (!Number.isFinite(num)) return null;
  const perMillion = num * 1_000_000;
  if (perMillion === 0) return "free";
  if (perMillion >= 1) return `$${perMillion.toFixed(2)}`;
  if (perMillion >= 0.01) return `$${perMillion.toFixed(3)}`;
  return `$${perMillion.toFixed(4)}`;
}

export function limitSummaryModels(models: { model_id: string }[], limit = 5) {
  return {
    items: models.slice(0, limit).map((item) => item.model_id),
    remaining: Math.max(models.length - limit, 0),
  };
}

export function formatDateTime(value?: string | null) {
  if (!value) return "—";
  return new Date(value).toLocaleString();
}

export function formatUSD(value: number) {
  return new Intl.NumberFormat(undefined, {
    style: "currency",
    currency: "USD",
    minimumFractionDigits: value >= 1 ? 2 : 4,
    maximumFractionDigits: value >= 1 ? 2 : 4,
  }).format(value);
}

export function formatMetricNumber(value: number) {
  return new Intl.NumberFormat(undefined, {
    minimumFractionDigits: value >= 100 ? 0 : value >= 10 ? 1 : 2,
    maximumFractionDigits: value >= 100 ? 0 : value >= 10 ? 1 : 2,
  }).format(value);
}

export function formatNumber(value: number) {
  return new Intl.NumberFormat().format(value);
}
