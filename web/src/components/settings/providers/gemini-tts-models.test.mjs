import test from "node:test";
import assert from "node:assert/strict";

import { buildGeminiTTSModelOptions } from "./tts-model-options.ts";
import { getAudioBriefingTTSProviderDefaultModel, getTTSProviderDefaultModel } from "./tts-provider-metadata.ts";

test("Gemini TTS options include gemini-3.1-flash-tts-preview", () => {
  const options = buildGeminiTTSModelOptions("");

  assert.equal(options[0]?.value, "gemini-3.1-flash-tts-preview");
  assert.ok(options.some((option) => option.value === "gemini-2.5-flash-tts"));
});

test("Gemini TTS default model resolves to gemini-3.1-flash-tts-preview", () => {
  assert.equal(getTTSProviderDefaultModel("gemini_tts"), "gemini-3.1-flash-tts-preview");
  assert.equal(
    getAudioBriefingTTSProviderDefaultModel("gemini_tts", "single"),
    "gemini-3.1-flash-tts-preview",
  );
});

test("Gemini TTS model options preserve an existing saved value", () => {
  const options = buildGeminiTTSModelOptions("gemini-2.5-pro-tts");

  assert.ok(options.some((option) => option.value === "gemini-2.5-pro-tts"));
});
