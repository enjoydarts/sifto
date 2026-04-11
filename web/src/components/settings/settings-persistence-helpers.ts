"use client";

import type {
  AudioBriefingPersonaVoice,
  AudioBriefingSettings,
  PodcastCategoryOption,
  PodcastSettings,
  SummaryAudioVoiceSettings,
  UserSettings,
} from "@/lib/api";

type AudioBriefingScheduleSelection = "interval3h" | "interval6h" | "fixed3x";

export function buildAudioBriefingSchedulePayload(selection: AudioBriefingScheduleSelection): {
  schedule_mode: "interval" | "fixed_slots_3x";
  interval_hours: number;
} {
  if (selection === "fixed3x") {
    return { schedule_mode: "fixed_slots_3x", interval_hours: 6 };
  }
  if (selection === "interval3h") {
    return { schedule_mode: "interval", interval_hours: 3 };
  }
  return { schedule_mode: "interval", interval_hours: 6 };
}

export function buildAudioBriefingSettingsPayload(params: {
  scheduleSelection: AudioBriefingScheduleSelection;
  enabled: boolean;
  articlesPerEpisode: string;
  targetDurationMinutes: string;
  chunkTrailingSilenceSeconds: string;
  programName: string;
  defaultPersonaMode: string;
  defaultPersona: string;
  conversationMode: "single" | "duo";
  bgmEnabled: boolean;
  bgmPrefix: string;
}): AudioBriefingSettings {
  const schedulePayload = buildAudioBriefingSchedulePayload(params.scheduleSelection);
  return {
    enabled: params.enabled,
    ...schedulePayload,
    articles_per_episode: Number(params.articlesPerEpisode),
    target_duration_minutes: Number(params.targetDurationMinutes),
    chunk_trailing_silence_seconds: Number(params.chunkTrailingSilenceSeconds),
    program_name: params.programName.trim() || null,
    default_persona_mode: params.defaultPersonaMode,
    default_persona: params.defaultPersona,
    conversation_mode: params.conversationMode,
    bgm_enabled: params.bgmEnabled,
    bgm_r2_prefix: params.bgmPrefix.trim() || null,
  };
}

export function buildPodcastSettingsPayload(params: {
  enabled: boolean;
  feedSlug: string;
  rssURL: string;
  title: string;
  description: string;
  author: string;
  language: string;
  category: string;
  subcategory: string;
  explicit: boolean;
  artworkURL: string;
}): PodcastSettings {
  return {
    enabled: params.enabled,
    feed_slug: params.feedSlug || null,
    rss_url: params.rssURL || null,
    title: params.title || null,
    description: params.description || null,
    author: params.author || null,
    language: params.language || "ja",
    category: params.category || null,
    subcategory: params.subcategory || null,
    explicit: params.explicit,
    artwork_url: params.artworkURL || null,
  };
}

export function buildSummaryAudioSettingsPayload(params: {
  provider: string;
  ttsModel: string;
  voiceModel: string;
  voiceStyle: string;
  providerVoiceLabel: string;
  providerVoiceDescription: string;
  speechRate: string;
  emotionalIntensity: string;
  tempoDynamics: string;
  lineBreakSilenceSeconds: string;
  pitch: string;
  volumeGain: string;
  aivisUserDictionaryUUID: string;
}): SummaryAudioVoiceSettings {
  return {
    tts_provider: params.provider.trim() || "aivis",
    tts_model: params.ttsModel.trim(),
    voice_model: params.voiceModel.trim(),
    voice_style: params.voiceStyle.trim(),
    provider_voice_label: params.providerVoiceLabel.trim(),
    provider_voice_description: params.providerVoiceDescription.trim(),
    speech_rate: Number(params.speechRate),
    emotional_intensity: Number(params.emotionalIntensity),
    tempo_dynamics: Number(params.tempoDynamics),
    line_break_silence_seconds: Number(params.lineBreakSilenceSeconds),
    pitch: Number(params.pitch),
    volume_gain: Number(params.volumeGain),
    aivis_user_dictionary_uuid: params.aivisUserDictionaryUUID.trim() || null,
  };
}

export function buildPodcastSettingsAfterArtworkUpload(params: {
  previousSettings: UserSettings | null;
  artworkURL: string | null;
  enabled: boolean;
  feedSlug: string;
  rssURL: string;
  title: string;
  description: string;
  author: string;
  language: string;
  category: string;
  subcategory: string;
  availableCategories: PodcastCategoryOption[];
  explicit: boolean;
}): UserSettings | null {
  const prev = params.previousSettings;
  if (!prev) return prev;
  return {
    ...prev,
    podcast: {
      enabled: prev.podcast?.enabled ?? params.enabled,
      feed_slug: prev.podcast?.feed_slug ?? (params.feedSlug || null),
      rss_url: prev.podcast?.rss_url ?? (params.rssURL || null),
      title: prev.podcast?.title ?? (params.title || null),
      description: prev.podcast?.description ?? (params.description || null),
      author: prev.podcast?.author ?? (params.author || null),
      language: prev.podcast?.language ?? params.language,
      category: prev.podcast?.category ?? (params.category || null),
      subcategory: prev.podcast?.subcategory ?? (params.subcategory || null),
      available_categories: prev.podcast?.available_categories ?? params.availableCategories,
      explicit: prev.podcast?.explicit ?? params.explicit,
      artwork_url: params.artworkURL,
    },
  };
}

export function mergeSummaryAudioIntoSettings(settings: UserSettings | null, summaryAudio: SummaryAudioVoiceSettings | null): UserSettings | null {
  return settings ? { ...settings, summary_audio: summaryAudio } : null;
}

export function mergeAudioBriefingIntoSettings(settings: UserSettings | null, audioBriefing: UserSettings["audio_briefing"] | null | undefined): UserSettings | null {
  return settings ? { ...settings, audio_briefing: audioBriefing ?? undefined } : settings;
}

export function mergeAudioBriefingVoicesIntoSettings(settings: UserSettings | null, voices: AudioBriefingPersonaVoice[]): UserSettings | null {
  return settings ? { ...settings, audio_briefing_persona_voices: voices } : settings;
}

export function mergePodcastIntoSettings(settings: UserSettings | null, podcast: UserSettings["podcast"] | null | undefined): UserSettings | null {
  return settings ? { ...settings, podcast: podcast ?? undefined } : settings;
}
