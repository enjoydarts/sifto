"use client";

type ToastTone = "success" | "error";

type LoadResourceActionArgs<T> = {
  setLoading: (loading: boolean) => void;
  fetch: () => Promise<T>;
  setData: (data: T) => void;
  setError: (message: string | null) => void;
};

export async function loadResourceAction<T>({
  setLoading,
  fetch,
  setData,
  setError,
}: LoadResourceActionArgs<T>): Promise<T> {
  setLoading(true);
  try {
    const next = await fetch();
    setData(next);
    setError(null);
    return next;
  } catch (error) {
    const message = String(error);
    setError(message);
    throw error;
  } finally {
    setLoading(false);
  }
}

type SyncResourceActionArgs<T> = {
  setSyncing: (syncing: boolean) => void;
  sync: () => Promise<T>;
  setData: (data: T) => void;
  setError: (message: string | null) => void;
  showToast: (message: string, tone: ToastTone) => void;
  successMessage: string;
};

export async function syncResourceAction<T>({
  setSyncing,
  sync,
  setData,
  setError,
  showToast,
  successMessage,
}: SyncResourceActionArgs<T>): Promise<T> {
  setSyncing(true);
  try {
    const next = await sync();
    setData(next);
    setError(null);
    showToast(successMessage, "success");
    return next;
  } catch (error) {
    const message = String(error);
    setError(message);
    showToast(message, "error");
    throw error;
  } finally {
    setSyncing(false);
  }
}
