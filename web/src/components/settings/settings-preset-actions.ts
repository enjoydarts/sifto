"use client";

import type { AudioBriefingPreset, SaveAudioBriefingPresetRequest } from "@/lib/api";

type ToastTone = "success" | "error";

export async function saveAudioBriefingPresetAction({
  name,
  presetsLoaded,
  presets,
  loadPresets,
  setPresets,
  setPresetsLoaded,
  setPresetsError,
  buildPayload,
  createPreset,
  updatePreset,
  confirmOverwrite,
  onSaved,
  showToast,
  requiredNameMessage,
  updatedMessage,
  savedMessage,
  setSaving,
}: {
  name: string;
  presetsLoaded: boolean;
  presets: AudioBriefingPreset[];
  loadPresets: () => Promise<AudioBriefingPreset[]>;
  setPresets: (updater: AudioBriefingPreset[] | ((prev: AudioBriefingPreset[]) => AudioBriefingPreset[])) => void;
  setPresetsLoaded: (value: boolean) => void;
  setPresetsError: (value: string | null) => void;
  buildPayload: () => SaveAudioBriefingPresetRequest;
  createPreset: (payload: SaveAudioBriefingPresetRequest) => Promise<AudioBriefingPreset>;
  updatePreset: (id: string, payload: SaveAudioBriefingPresetRequest) => Promise<AudioBriefingPreset>;
  confirmOverwrite: (name: string) => Promise<boolean>;
  onSaved: () => void;
  showToast: (message: string, tone: ToastTone) => void;
  requiredNameMessage: string;
  updatedMessage: string;
  savedMessage: string;
  setSaving: (value: boolean) => void;
}) {
  const trimmedName = name.trim();
  if (!trimmedName) {
    showToast(requiredNameMessage, "error");
    return;
  }
  setSaving(true);
  try {
    const presetList = presetsLoaded ? presets : await loadPresets();
    if (!presetsLoaded) {
      setPresets(presetList);
      setPresetsLoaded(true);
      setPresetsError(null);
    }
    const existing = presetList.find((preset) => preset.name.trim() === trimmedName);
    const payload = buildPayload();
    let saved: AudioBriefingPreset | null = null;
    if (existing) {
      const ok = await confirmOverwrite(trimmedName);
      if (!ok) return;
      saved = await updatePreset(existing.id, payload);
    } else {
      saved = await createPreset(payload);
    }
    setPresets((prev) => [saved, ...prev.filter((preset) => preset.id !== saved.id)]);
    onSaved();
    showToast(existing ? updatedMessage : savedMessage, "success");
  } catch (error) {
    showToast(String(error), "error");
  } finally {
    setSaving(false);
  }
}

export async function openAudioBriefingPresetApplyAction({
  presetsCount,
  setSelection,
  setOpen,
  loadPresets,
}: {
  presetsCount: number;
  setSelection: (value: string | null) => void;
  setOpen: (value: boolean) => void;
  loadPresets: () => Promise<unknown>;
}) {
  setSelection(null);
  setOpen(true);
  if (presetsCount === 0) {
    await loadPresets();
  }
}
