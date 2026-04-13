export const queryKeys = {
  briefing: {
    today: (size: number) => ["briefing-today", size] as const,
    todayPrefix: ["briefing-today"] as const,
    clusters: (size: number) => ["briefing-clusters", size] as const,
    clustersPrefix: ["briefing-clusters"] as const,
    navigator: (preview: unknown, persona: string) =>
      ["briefing-navigator", preview, persona] as const,
  },
  items: {
    feedPrefix: ["items-feed"] as const,
    detail: (id: string) => ["item-detail", id] as const,
    related: (id: string, limit: number) =>
      ["item-related", id, limit] as const,
    searchSuggestions: (query: string) =>
      ["item-search-suggestions", query] as const,
  },
  settings: {
    all: () => ["settings"] as const,
    summaryAudioReadiness: () =>
      ["settings", "summary-audio-readiness"] as const,
  },
  dashboard: {
    all: () => ["dashboard"] as const,
  },
  queues: {
    focus: () => ["focus-queue"] as const,
    today: (size: number) => ["today-queue", size] as const,
    todayPrefix: ["today-queue"] as const,
    review: (size: number) => ["review-queue", size] as const,
    reviewPrefix: ["review-queue"] as const,
    triage: (
      mode: string,
      window: number,
      size: number,
      diversify: number
    ) => ["triage-queue", mode, window, size, diversify] as const,
    triagePrefix: ["triage-queue"] as const,
  },
  providerModelUpdates: (days: number) =>
    ["provider-model-updates", days] as const,
  readingGoals: () => ["reading-goals"] as const,
  weeklyReviewLatest: () => ["weekly-review-latest"] as const,
  preferenceProfile: () => ["preference-profile"] as const,
  favoritesPage: (pageSize: number) => ["favorites-page", pageSize] as const,
  favoritesPagePrefix: ["favorites-page"] as const,
  askInsights: (limit: number) => ["ask-insights", limit] as const,
  askInsightsPrefix: ["ask-insights"] as const,
  topicsPulse: (days: number, limit: number) =>
    ["topics-pulse", days, limit] as const,
  audio: {
    sharedSettings: () => ["shared-audio-player-settings"] as const,
    sharedQueue: (kind: string, query: string) =>
      ["shared-summary-audio-queue", kind, query] as const,
    sharedQueuePrefix: ["shared-summary-audio-queue"] as const,
    sharedItem: (id: string) => ["shared-summary-audio-item", id] as const,
    summaryItem: (id: string) => ["summary-audio-item", id] as const,
    summaryQueue: () => ["summary-audio-queue"] as const,
    navigatorPersonas: () => ["navigator-personas"] as const,
    latestPlaybackSessions: () => ["latest-playback-sessions"] as const,
    playbackHistory: (filter: string) =>
      ["playback-history", filter] as const,
  },
  navigatorBriefs: {
    list: () => ["ai-navigator-briefs"] as const,
    detail: (id: string) => ["ai-navigator-brief", id] as const,
  },
} as const;
