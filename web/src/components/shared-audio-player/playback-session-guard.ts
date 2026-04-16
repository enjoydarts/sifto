"use client";

export async function swallowPlaybackSessionError<T>(
  operation: () => Promise<T>,
  onError?: (error: unknown) => void,
): Promise<T | null> {
  try {
    return await operation();
  } catch (error) {
    if (onError) onError(error);
    return null;
  }
}
