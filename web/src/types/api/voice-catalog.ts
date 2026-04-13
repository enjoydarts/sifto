import type { ProviderModelChangeSummary } from "./common";

export interface AivisSyncRun {
  id: string;
  started_at: string;
  finished_at?: string | null;
  last_progress_at?: string | null;
  status: string;
  trigger_type: string;
  fetched_count: number;
  accepted_count: number;
  error_message?: string | null;
}

export interface AivisSyncStatusResponse {
  run: AivisSyncRun | null;
}

export interface AivisModelVoiceSample {
  audio_url: string;
  transcript: string;
}

export interface AivisModelSpeakerStyle {
  name: string;
  icon_url?: string | null;
  local_id: number;
  voice_samples: AivisModelVoiceSample[];
}

export interface AivisModelSpeaker {
  aivm_speaker_uuid: string;
  name: string;
  icon_url: string;
  supported_languages: string[];
  local_id: number;
  styles: AivisModelSpeakerStyle[];
}

export interface AivisModelTag {
  name: string;
}

export interface AivisModelFile {
  aivm_model_uuid: string;
  manifest_version: string;
  name: string;
  description: string;
  creators: string[];
  license_type: string;
  license_text?: string | null;
  model_type: string;
  model_architecture: string;
  model_format: string;
  training_epochs?: number | null;
  training_steps?: number | null;
  version: string;
  file_size: number;
  checksum: string;
  download_count: number;
  created_at: string;
  updated_at: string;
}

export interface AivisModelSnapshot {
  aivm_model_uuid: string;
  name: string;
  description: string;
  detailed_description: string;
  category: string;
  voice_timbre: string;
  visibility: string;
  is_tag_locked: boolean;
  total_download_count: number;
  like_count: number;
  is_liked: boolean;
  user_json: Record<string, unknown>;
  model_files_json: AivisModelFile[];
  tags_json: AivisModelTag[];
  speakers_json: AivisModelSpeaker[];
  model_file_count: number;
  speaker_count: number;
  style_count: number;
  created_at: string;
  updated_at: string;
  fetched_at: string;
}

export interface AivisModelsResponse {
  latest_run: AivisSyncRun | null;
  latest_change_summary?: ProviderModelChangeSummary | null;
  models: AivisModelSnapshot[];
  removed_models?: AivisModelSnapshot[];
}

export interface XAIVoiceSyncRun {
  id: number;
  started_at: string;
  finished_at?: string | null;
  last_progress_at?: string | null;
  status: string;
  trigger_type: string;
  fetched_count: number;
  saved_count: number;
  error_message?: string | null;
}

export interface XAIVoiceSnapshot {
  id: number;
  sync_run_id: number;
  voice_id: string;
  name: string;
  description: string;
  language: string;
  preview_url: string;
  metadata_json?: string | null;
  fetched_at: string;
}

export interface XAIVoicesResponse {
  latest_run: XAIVoiceSyncRun | null;
  latest_change_summary?: ProviderModelChangeSummary | null;
  voices: XAIVoiceSnapshot[];
}

export interface OpenAITTSVoiceSyncRun {
  id: number;
  started_at: string;
  finished_at?: string | null;
  last_progress_at?: string | null;
  status: string;
  trigger_type: string;
  fetched_count: number;
  saved_count: number;
  error_message?: string | null;
}

export interface OpenAITTSVoiceSnapshot {
  id: number;
  sync_run_id: number;
  voice_id: string;
  name: string;
  description: string;
  language: string;
  preview_url: string;
  metadata_json?: string | null;
  fetched_at: string;
}

export interface OpenAITTSVoicesResponse {
  latest_run: OpenAITTSVoiceSyncRun | null;
  latest_change_summary?: ProviderModelChangeSummary | null;
  voices: OpenAITTSVoiceSnapshot[];
}

export interface ElevenLabsVoiceCatalogEntry {
  voice_id: string;
  name: string;
  description: string;
  labels?: Record<string, unknown>;
  category?: string | null;
  preview_url: string;
  languages?: string[];
}

export interface ElevenLabsVoicesResponse {
  provider: string;
  source: string;
  voices: ElevenLabsVoiceCatalogEntry[];
}

export interface AzureSpeechVoiceCatalogEntry {
  voice_id: string;
  label: string;
  description: string;
  locale: string;
  gender: string;
  local_name: string;
  styles?: string[];
}

export interface AzureSpeechVoicesResponse {
  provider: string;
  source: string;
  region: string;
  voices: AzureSpeechVoiceCatalogEntry[];
}

export interface GeminiTTSVoiceCatalogEntry {
  voice_name: string;
  label: string;
  tone: string;
  description: string;
  sample_audio_path: string;
}

export interface GeminiTTSVoicesResponse {
  catalog_name: string;
  provider: string;
  source: string;
  voices: GeminiTTSVoiceCatalogEntry[];
}

export interface FishModelAuthor {
  id?: string | null;
  name?: string | null;
  username?: string | null;
  avatar_url?: string | null;
}

export interface FishModelTag {
  name: string;
}

export interface FishModelSample {
  audio_url: string;
  text?: string | null;
  transcript?: string | null;
  duration_seconds?: number | null;
}

export interface FishModelSnapshot {
  _id: string;
  title: string;
  description: string;
  cover_image?: string | null;
  tags: FishModelTag[];
  languages: string[];
  visibility: string;
  like_count: number;
  task_count: number;
  author?: FishModelAuthor | null;
  samples: FishModelSample[];
  fetched_at: string;
  updated_at?: string | null;
}

export type FishBrowseSort = "recommended" | "trending" | "latest";

export interface FishBrowseResponse {
  items: FishModelSnapshot[];
  page: number;
  page_size: number;
  total: number;
  has_more: boolean;
}

export interface AivisUserDictionary {
  uuid: string;
  name: string;
  description?: string | null;
  word_count: number;
  created_at: string;
  updated_at: string;
}

export interface AivisUserDictionariesResponse {
  user_dictionaries: AivisUserDictionary[];
}
