import type { SummaryAudioVoiceSettings, UserSettings } from "@/lib/api";

export type SummaryAudioReadiness = {
  ready: boolean;
  reasonKey: string | null;
};

function hasConfiguredVoice(settings: SummaryAudioVoiceSettings | null | undefined): boolean {
  if (!settings) {
    return false;
  }
  if (!settings.tts_provider.trim() || !settings.voice_model.trim()) {
    return false;
  }
  if ((settings.tts_provider === "openai" || settings.tts_provider === "gemini_tts" || settings.tts_provider === "elevenlabs") && !settings.tts_model.trim()) {
    return false;
  }
  if (settings.tts_provider === "aivis" && !settings.voice_style.trim()) {
    return false;
  }
  return true;
}

export function getSummaryAudioReadiness(settings: UserSettings | null | undefined): SummaryAudioReadiness {
  const voice = settings?.summary_audio;
  if (!hasConfiguredVoice(voice)) {
    return { ready: false, reasonKey: "summaryAudio.playbackBlocked.notConfigured" };
  }
  switch (voice?.tts_provider) {
    case "aivis":
      if (!settings?.has_aivis_api_key) {
        return { ready: false, reasonKey: "summaryAudio.playbackBlocked.aivisApiKeyMissing" };
      }
      return { ready: true, reasonKey: null };
    case "xai":
      return settings?.has_xai_api_key
        ? { ready: true, reasonKey: null }
        : { ready: false, reasonKey: "summaryAudio.playbackBlocked.xaiApiKeyMissing" };
    case "fish":
      return settings?.has_fish_api_key
        ? { ready: true, reasonKey: null }
        : { ready: false, reasonKey: "summaryAudio.playbackBlocked.fishApiKeyMissing" };
    case "elevenlabs":
      return settings?.has_elevenlabs_api_key
        ? { ready: true, reasonKey: null }
        : { ready: false, reasonKey: "summaryAudio.playbackBlocked.elevenlabsApiKeyMissing" };
    case "openai":
      return settings?.has_openai_api_key
        ? { ready: true, reasonKey: null }
        : { ready: false, reasonKey: "summaryAudio.playbackBlocked.openaiApiKeyMissing" };
    case "azure_speech":
      if (!settings?.has_azure_speech_api_key) {
        return { ready: false, reasonKey: "summaryAudio.playbackBlocked.azureSpeechApiKeyMissing" };
      }
      return settings?.azure_speech_region?.trim()
        ? { ready: true, reasonKey: null }
        : { ready: false, reasonKey: "summaryAudio.playbackBlocked.azureSpeechRegionMissing" };
    case "gemini_tts":
      return settings?.gemini_tts_enabled
        ? { ready: true, reasonKey: null }
        : { ready: false, reasonKey: "summaryAudio.playbackBlocked.geminiNotAllowed" };
    default:
      return { ready: true, reasonKey: null };
  }
}
