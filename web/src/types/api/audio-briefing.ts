export type PlaybackMode = "summary_queue" | "audio_briefing";
export type PlaybackSessionStatus = "in_progress" | "completed" | "interrupted";

export interface PlaybackSession {
  id: string;
  user_id: string;
  mode: PlaybackMode;
  status: PlaybackSessionStatus;
  title: string;
  subtitle: string;
  current_position_sec: number;
  duration_sec: number;
  progress_ratio?: number | null;
  resume_payload: Record<string, unknown>;
  started_at: string;
  updated_at: string;
  completed_at?: string | null;
}

export interface LatestPlaybackSessionsResponse {
  summary_queue: PlaybackSession | null;
  audio_briefing: PlaybackSession | null;
}

export interface PlaybackSessionsResponse {
  items: PlaybackSession[];
}

export interface CreatePlaybackSessionRequest {
  mode: PlaybackMode;
  title: string;
  subtitle: string;
  current_position_sec: number;
  duration_sec: number;
  progress_ratio?: number | null;
  resume_payload: Record<string, unknown>;
}

export interface UpdatePlaybackSessionRequest {
  title: string;
  subtitle: string;
  current_position_sec: number;
  duration_sec: number;
  progress_ratio?: number | null;
  resume_payload: Record<string, unknown>;
}

export interface AudioBriefingSettings {
  enabled: boolean;
  schedule_mode?: "interval" | "fixed_slots_3x" | string | null;
  interval_hours: number;
  articles_per_episode: number;
  target_duration_minutes: number;
  chunk_trailing_silence_seconds: number;
  program_name?: string | null;
  default_persona_mode: string;
  default_persona: string;
  conversation_mode: "single" | "duo" | string;
  bgm_enabled: boolean;
  bgm_r2_prefix?: string | null;
}

export interface AudioBriefingPersonaVoice {
  persona: string;
  tts_provider: string;
  tts_model: string;
  voice_model: string;
  voice_style: string;
  provider_voice_label?: string;
  provider_voice_description?: string;
  speech_rate: number;
  emotional_intensity: number;
  tempo_dynamics: number;
  line_break_silence_seconds: number;
  pitch: number;
  volume_gain: number;
}

export interface AudioBriefingPreset {
  id: string;
  user_id: string;
  name: string;
  default_persona_mode: "fixed" | "random" | string;
  default_persona: string;
  conversation_mode: "single" | "duo" | string;
  voices: AudioBriefingPersonaVoice[];
  created_at: string;
  updated_at: string;
}

export interface SaveAudioBriefingPresetRequest {
  name: string;
  default_persona_mode: "fixed" | "random" | string;
  default_persona: string;
  conversation_mode: "single" | "duo" | string;
  voices: AudioBriefingPersonaVoice[];
}

export interface SummaryAudioVoiceSettings {
  tts_provider: string;
  tts_model: string;
  voice_model: string;
  voice_style: string;
  provider_voice_label?: string;
  provider_voice_description?: string;
  speech_rate: number;
  emotional_intensity: number;
  tempo_dynamics: number;
  line_break_silence_seconds: number;
  pitch: number;
  volume_gain: number;
  aivis_user_dictionary_uuid?: string | null;
}

export interface AudioBriefingJob {
  id: string;
  user_id: string;
  slot_started_at_jst: string;
  slot_key: string;
  persona: string;
  conversation_mode?: "single" | "duo" | string;
  partner_persona?: string | null;
  pipeline_stage?: string | null;
  status: string;
  archive_status: "active" | "archived" | string;
  source_item_count: number;
  reused_item_count: number;
  script_char_count: number;
  script_llm_models?: string | null;
  prompt_key?: string | null;
  prompt_source?: string | null;
  prompt_version_id?: string | null;
  prompt_version_number?: number | null;
  prompt_experiment_id?: string | null;
  prompt_experiment_arm_id?: string | null;
  audio_duration_sec?: number | null;
  title?: string | null;
  r2_audio_object_key?: string | null;
  r2_manifest_object_key?: string | null;
  bgm_object_key?: string | null;
  provider_job_id?: string | null;
  idempotency_key?: string | null;
  error_code?: string | null;
  error_message?: string | null;
  published_at?: string | null;
  failed_at?: string | null;
  created_at: string;
  updated_at: string;
}

export interface AudioBriefingJobItem {
  item_id: string;
  rank: number;
  segment_title?: string | null;
  summary_snapshot?: string | null;
  title?: string | null;
  translated_title?: string | null;
  source_title?: string | null;
  published_at?: string | null;
}

export interface AudioBriefingScriptChunk {
  seq: number;
  part_type: string;
  speaker?: "host" | "partner" | string | null;
  text: string;
  preprocessed_text?: string | null;
  char_count: number;
  tts_status: string;
  tts_provider?: string | null;
  voice_model?: string | null;
  provider_voice_label?: string | null;
  voice_style?: string | null;
  r2_audio_object_key?: string | null;
  duration_sec?: number | null;
  error_message?: string | null;
}

export interface AudioBriefingUsedTTS {
  provider: string;
  tts_model?: string | null;
  host_voice_model?: string | null;
  host_voice_label?: string | null;
  partner_voice_model?: string | null;
  partner_voice_label?: string | null;
}

export interface AudioBriefingDetailResponse {
  job: AudioBriefingJob;
  items: AudioBriefingJobItem[];
  chunks: AudioBriefingScriptChunk[];
  used_tts?: AudioBriefingUsedTTS | null;
  audio_url?: string | null;
  delete_allowed?: boolean;
  resume_allowed?: boolean;
  archive_allowed?: boolean;
  unarchive_allowed?: boolean;
}
