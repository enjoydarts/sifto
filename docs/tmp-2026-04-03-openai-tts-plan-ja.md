# OpenAI TTS追加 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add OpenAI TTS as a first-class provider across Audio Briefing and Summary Audio, with synced voice catalog, model selection, and picker UX aligned with Aivis/xAI.

**Architecture:** First introduce a thin provider-capability layer and provider-specific synthesis adapters, then add OpenAI-specific persistence, sync APIs, worker synthesis, and settings UI. Keep the existing `tts_provider / voice_model / voice_style` flow intact while adding `tts_model` only where OpenAI requires a separate model choice.

**Tech Stack:** Go API, PostgreSQL migrations, Python worker, Next.js App Router, TypeScript, i18n dictionaries, Docker Compose / make

---

### Task 1: Add provider capability plumbing and `tts_model` persistence

**Files:**
- Create: `db/migrations/000122_add_audio_briefing_tts_model.up.sql`
- Create: `db/migrations/000122_add_audio_briefing_tts_model.down.sql`
- Create: `api/internal/service/tts_provider_capabilities.go`
- Modify: `api/internal/model/model.go`
- Modify: `api/internal/repository/audio_briefings.go`
- Modify: `api/internal/service/settings_service.go`
- Test: `api/internal/service/settings_service_test.go`

- [ ] **Step 1: Write the failing test**

```go
func TestUpdateAudioBriefingPersonaVoicesRequiresTTSModelForOpenAI(t *testing.T) {
	svc := newSettingsServiceForTest(t)

	_, err := svc.UpdateAudioBriefingPersonaVoices(context.Background(), "user-1", []UpdateAudioBriefingPersonaVoiceInput{
		{
			Persona:     "editor",
			TTSProvider: "openai",
			TTSModel:    "",
			VoiceModel:  "alloy",
			VoiceStyle:  "",
		},
	})
	if err == nil || !strings.Contains(err.Error(), "tts_model") {
		t.Fatalf("err = %v, want tts_model validation error", err)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `docker compose exec -T api go test ./internal/service -run TestUpdateAudioBriefingPersonaVoicesRequiresTTSModelForOpenAI`
Expected: FAIL because `TTSModel` does not exist yet and OpenAI validation is not implemented

- [ ] **Step 3: Add migration and model/repository fields**

```sql
ALTER TABLE audio_briefing_persona_voices
ADD COLUMN tts_model text NOT NULL DEFAULT '';
```

```go
type AudioBriefingPersonaVoice struct {
	Persona     string  `json:"persona"`
	TTSProvider string  `json:"tts_provider"`
	TTSModel    string  `json:"tts_model"`
	VoiceModel  string  `json:"voice_model"`
	VoiceStyle  string  `json:"voice_style"`
	// ...
}
```

- [ ] **Step 4: Add capability helper and minimal validation**

```go
type TTSProviderCapabilities struct {
	RequiresVoiceStyle      bool
	SupportsCatalogPicker   bool
	SupportsSeparateTTSModel bool
	SupportsSpeechTuning    bool
	RequiresUserAPIKey      bool
}

func LookupTTSProviderCapabilities(provider string) TTSProviderCapabilities {
	switch strings.TrimSpace(strings.ToLower(provider)) {
	case "aivis":
		return TTSProviderCapabilities{RequiresVoiceStyle: true, SupportsCatalogPicker: true, SupportsSpeechTuning: true, RequiresUserAPIKey: true}
	case "xai":
		return TTSProviderCapabilities{SupportsCatalogPicker: true, RequiresUserAPIKey: true}
	case "openai":
		return TTSProviderCapabilities{SupportsCatalogPicker: true, SupportsSeparateTTSModel: true, RequiresUserAPIKey: true}
	case "mock":
		return TTSProviderCapabilities{}
	default:
		return TTSProviderCapabilities{}
	}
}
```

```go
caps := LookupTTSProviderCapabilities(in.TTSProvider)
if caps.SupportsSeparateTTSModel && strings.TrimSpace(in.TTSModel) == "" {
	return nil, fmt.Errorf("tts_model is required for provider %s", in.TTSProvider)
}
if caps.RequiresVoiceStyle && strings.TrimSpace(in.VoiceStyle) == "" {
	return nil, fmt.Errorf("voice_style is required for provider %s", in.TTSProvider)
}
```

- [ ] **Step 5: Run targeted test and repo tests**

Run: `docker compose exec -T api go test ./internal/service ./internal/repository -run 'Test(UpdateAudioBriefingPersonaVoicesRequiresTTSModelForOpenAI|AudioBriefing)'`
Expected: PASS

- [ ] **Step 6: Commit**

```bash
git add db/migrations/000122_add_audio_briefing_tts_model.up.sql db/migrations/000122_add_audio_briefing_tts_model.down.sql api/internal/service/tts_provider_capabilities.go api/internal/model/model.go api/internal/repository/audio_briefings.go api/internal/service/settings_service.go api/internal/service/settings_service_test.go
git commit -m "feat: add tts model support for openai voices"
```

### Task 2: Add OpenAI TTS voice catalog persistence and sync APIs

**Files:**
- Create: `db/migrations/000123_add_openai_tts_voice_catalog.up.sql`
- Create: `db/migrations/000123_add_openai_tts_voice_catalog.down.sql`
- Create: `api/internal/repository/openai_tts_voices.go`
- Create: `api/internal/service/openai_tts_voice_catalog.go`
- Create: `api/internal/handler/openai_tts_voices.go`
- Modify: `api/cmd/server/main.go`
- Test: `api/internal/repository/openai_tts_voices_test.go`
- Test: `api/internal/service/openai_tts_voice_catalog_test.go`
- Test: `api/internal/handler/openai_tts_voices_test.go`

- [ ] **Step 1: Write the failing repository test**

```go
func TestOpenAITTSVoiceRepoInsertAndListLatestSnapshots(t *testing.T) {
	db := testDB(t)
	repo := NewOpenAITTSVoiceRepo(db)

	runID, err := repo.StartSyncRun(context.Background(), "manual")
	if err != nil {
		t.Fatalf("StartSyncRun() error = %v", err)
	}
	fetchedAt := time.Date(2026, 4, 3, 12, 0, 0, 0, time.UTC)
	err = repo.InsertSnapshots(context.Background(), runID, fetchedAt, []OpenAITTSVoiceSnapshot{
		{VoiceID: "alloy", Name: "Alloy", Description: "Neutral", Language: "multilingual"},
	})
	if err != nil {
		t.Fatalf("InsertSnapshots() error = %v", err)
	}
	if err := repo.FinishSyncRun(context.Background(), runID, 1, 1, nil); err != nil {
		t.Fatalf("FinishSyncRun() error = %v", err)
	}

	rows, latestRun, err := repo.ListLatestSnapshots(context.Background())
	if err != nil {
		t.Fatalf("ListLatestSnapshots() error = %v", err)
	}
	if latestRun == nil || latestRun.Status != "succeeded" {
		t.Fatalf("latestRun = %#v, want succeeded", latestRun)
	}
	if len(rows) != 1 || rows[0].VoiceID != "alloy" {
		t.Fatalf("rows = %#v, want alloy", rows)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `docker compose exec -T api go test ./internal/repository -run TestOpenAITTSVoiceRepoInsertAndListLatestSnapshots`
Expected: FAIL with missing tables/repo

- [ ] **Step 3: Add migration and repo**

```sql
CREATE TABLE openai_tts_voice_sync_runs (
  id bigserial PRIMARY KEY,
  status text NOT NULL DEFAULT 'running',
  trigger_type text NOT NULL,
  started_at timestamptz NOT NULL DEFAULT now(),
  finished_at timestamptz,
  last_progress_at timestamptz,
  fetched_count integer NOT NULL DEFAULT 0,
  saved_count integer NOT NULL DEFAULT 0,
  error_message text
);

CREATE TABLE openai_tts_voice_snapshots (
  id bigserial PRIMARY KEY,
  sync_run_id bigint NOT NULL REFERENCES openai_tts_voice_sync_runs(id) ON DELETE CASCADE,
  voice_id text NOT NULL,
  name text NOT NULL,
  description text NOT NULL DEFAULT '',
  language text NOT NULL DEFAULT '',
  preview_url text NOT NULL DEFAULT '',
  metadata_json jsonb NOT NULL DEFAULT '{}'::jsonb,
  fetched_at timestamptz NOT NULL,
  UNIQUE (sync_run_id, voice_id)
);
```

- [ ] **Step 4: Add failing service/handler test**

```go
func TestOpenAITTSVoiceCatalogServiceFetchVoices(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"data": []map[string]any{
				{"voice": "alloy", "name": "Alloy", "description": "Neutral", "language": "multilingual"},
			},
		})
	}))
	defer srv.Close()

	svc := NewOpenAITTSVoiceCatalogServiceWithBaseURL(srv.URL)
	rows, err := svc.FetchVoices(context.Background(), "openai-key")
	if err != nil {
		t.Fatalf("FetchVoices() error = %v", err)
	}
	if len(rows) != 1 || rows[0].VoiceID != "alloy" {
		t.Fatalf("rows = %#v, want alloy", rows)
	}
}
```

- [ ] **Step 5: Implement service and routes**

```go
r.Route("/openai-tts-voices", func(r chi.Router) {
	r.Get("/", openAITTSVoicesH.List)
	r.Get("/status", openAITTSVoicesH.Status)
	r.Post("/sync", openAITTSVoicesH.Sync)
})
```

- [ ] **Step 6: Run targeted tests**

Run: `docker compose exec -T api go test ./internal/repository ./internal/service ./internal/handler -run 'Test(OpenAITTSVoiceRepoInsertAndListLatestSnapshots|OpenAITTSVoiceCatalogServiceFetchVoices|OpenAITTSVoicesHandler)'`
Expected: PASS

- [ ] **Step 7: Commit**

```bash
git add db/migrations/000123_add_openai_tts_voice_catalog.up.sql db/migrations/000123_add_openai_tts_voice_catalog.down.sql api/internal/repository/openai_tts_voices.go api/internal/repository/openai_tts_voices_test.go api/internal/service/openai_tts_voice_catalog.go api/internal/service/openai_tts_voice_catalog_test.go api/internal/handler/openai_tts_voices.go api/internal/handler/openai_tts_voices_test.go api/cmd/server/main.go
git commit -m "feat: add openai tts voice sync endpoints"
```

### Task 3: Wire OpenAI provider through API synthesis flows

**Files:**
- Modify: `api/internal/service/audio_briefing_voice.go`
- Modify: `api/internal/service/summary_audio_player.go`
- Modify: `api/internal/service/worker.go`
- Modify: `api/internal/service/worker_test.go`
- Test: `api/internal/service/audio_briefing_voice_test.go`
- Test: `api/internal/service/summary_audio_player_test.go`

- [ ] **Step 1: Write the failing worker request test**

```go
func TestSynthesizeSummaryAudioIncludesOpenAIHeadersAndModel(t *testing.T) {
	var capturedBody map[string]any
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("x-openai-api-key"); got != "openai-key" {
			t.Fatalf("x-openai-api-key = %q, want openai-key", got)
		}
		if err := json.NewDecoder(r.Body).Decode(&capturedBody); err != nil {
			t.Fatalf("Decode() error = %v", err)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"audio_base64": "YQ==",
			"content_type": "audio/mpeg",
			"duration_sec": 1,
			"resolved_text": "text",
		})
	}))
	defer ts.Close()

	client := NewWorkerClient(ts.URL, "secret")
	_, err := client.SynthesizeSummaryAudio(context.Background(), "openai", "alloy", "", "gpt-4o-mini-tts", "text", 1, 1, 1, 0.4, 1, 0, 0, nil, nil, nil, ptr("openai-key"))
	if err != nil {
		t.Fatalf("SynthesizeSummaryAudio() error = %v", err)
	}
	if capturedBody["tts_model"] != "gpt-4o-mini-tts" {
		t.Fatalf("tts_model = %#v, want gpt-4o-mini-tts", capturedBody["tts_model"])
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `docker compose exec -T api go test ./internal/service -run TestSynthesizeSummaryAudioIncludesOpenAIHeadersAndModel`
Expected: FAIL because worker request does not carry `tts_model` or OpenAI key

- [ ] **Step 3: Extend worker client and service call sites**

```go
func (w *WorkerClient) SynthesizeSummaryAudio(
	ctx context.Context,
	provider string,
	voiceModel string,
	voiceStyle string,
	ttsModel string,
	text string,
	// ...
	openAIAPIKey *string,
) (*SummaryAudioSynthesizeResponse, error)
```

```go
requestBody := map[string]any{
	"provider":    provider,
	"voice_model": voiceModel,
	"voice_style": voiceStyle,
	"tts_model":   ttsModel,
	"text":        text,
}
```

- [ ] **Step 4: Load OpenAI API key in Audio Briefing and Summary Audio**

```go
} else if provider == "openai" {
	openAIAPIKey, err = r.loadOpenAIAPIKey(ctx, userID)
	if err != nil {
		return r.handleChunkGenerationFailure(ctx, jobID, chunk, "tts_failed", err)
	}
}
```

- [ ] **Step 5: Run targeted API tests**

Run: `docker compose exec -T api go test ./internal/service -run 'Test(SynthesizeSummaryAudioIncludesOpenAIHeadersAndModel|AudioBriefingVoice|SummaryAudio)'`
Expected: PASS

- [ ] **Step 6: Commit**

```bash
git add api/internal/service/audio_briefing_voice.go api/internal/service/audio_briefing_voice_test.go api/internal/service/summary_audio_player.go api/internal/service/summary_audio_player_test.go api/internal/service/worker.go api/internal/service/worker_test.go
git commit -m "feat: wire openai tts through api synthesis flows"
```

### Task 4: Add OpenAI synthesis adapter and worker dispatch

**Files:**
- Create: `worker/app/services/openai_tts.py`
- Create: `worker/app/services/tts_provider_registry.py`
- Modify: `worker/app/services/audio_briefing_tts.py`
- Modify: `worker/app/services/summary_audio_player.py`
- Modify: `worker/app/routers/audio_briefing_tts.py`
- Modify: `worker/app/routers/summary_audio_player.py`
- Test: `worker/app/services/test_audio_briefing_tts.py`
- Test: `worker/app/services/test_summary_audio_player.py`

- [ ] **Step 1: Write the failing worker test**

```python
def test_synthesize_openai_tts_uses_mp3_192kbps_44khz():
    captured = {}

    def fake_post(url, headers=None, json=None, timeout=None):
        captured["url"] = url
        captured["headers"] = headers
        captured["json"] = json
        request = httpx.Request("POST", url)
        return httpx.Response(200, content=b"audio", request=request)

    with patch("app.services.openai_tts.httpx.post", side_effect=fake_post):
        audio_bytes, content_type, suffix, duration_sec = synthesize_openai_tts(
            endpoint="https://api.openai.com/v1",
            api_key="openai-key",
            model="gpt-4o-mini-tts",
            voice="alloy",
            text="summary text",
            timeout_sec=30.0,
        )

    assert captured["json"]["model"] == "gpt-4o-mini-tts"
    assert captured["json"]["voice"] == "alloy"
    assert captured["json"]["format"] == "mp3"
    assert captured["json"]["bitrate"] == 192000
    assert captured["json"]["sample_rate"] == 44100
```

- [ ] **Step 2: Run test to verify it fails**

Run: `docker compose exec -T worker python -m unittest app.services.test_audio_briefing_tts app.services.test_summary_audio_player`
Expected: FAIL with missing OpenAI adapter and request fields

- [ ] **Step 3: Add adapter and dispatch helper**

```python
def synthesize_openai_tts(*, endpoint: str, api_key: str, model: str, voice: str, text: str, timeout_sec: float):
    response = httpx.post(
        f"{endpoint.rstrip('/')}/audio/speech",
        headers={"Authorization": f"Bearer {api_key}"},
        json={
            "model": model,
            "voice": voice,
            "input": text,
            "format": "mp3",
            "bitrate": 192000,
            "sample_rate": 44100,
        },
        timeout=timeout_sec,
    )
    response.raise_for_status()
    return response.content, "audio/mpeg", ".mp3", estimate_audio_duration_sec(text, 1.0)
```

- [ ] **Step 4: Thread `tts_model` through router and service requests**

```python
class SummaryAudioSynthesizeRequest(BaseModel):
    provider: str
    voice_model: str
    voice_style: str
    tts_model: str = ""
    text: str
```

- [ ] **Step 5: Run worker tests**

Run: `docker compose exec -T worker python -m unittest app.services.test_audio_briefing_tts app.services.test_summary_audio_player`
Expected: PASS

- [ ] **Step 6: Commit**

```bash
git add worker/app/services/openai_tts.py worker/app/services/tts_provider_registry.py worker/app/services/audio_briefing_tts.py worker/app/services/summary_audio_player.py worker/app/routers/audio_briefing_tts.py worker/app/routers/summary_audio_player.py worker/app/services/test_audio_briefing_tts.py worker/app/services/test_summary_audio_player.py
git commit -m "feat: add openai tts worker adapter"
```

### Task 5: Add OpenAI settings UI, picker, and dictionaries

**Files:**
- Create: `web/src/app/(main)/openai-tts-voices/page.tsx`
- Modify: `web/src/app/(main)/settings/page.tsx`
- Modify: `web/src/lib/api.ts`
- Modify: `web/src/i18n/dictionaries/ja.ts`
- Modify: `web/src/i18n/dictionaries/en.ts`

- [ ] **Step 1: Write the failing UI type check by using missing fields**

```tsx
const isOpenAI = draft.tts_provider === "openai";
{isOpenAI ? (
  <ModelSelect
    label={t("settings.audioBriefing.openaiTTSModel")}
    value={draft.tts_model}
    onChange={(value) => updateDraft(persona, { tts_model: value })}
    options={OPENAI_TTS_MODEL_OPTIONS}
    labels={modelSelectLabels}
    variant="modal"
  />
) : null}
```

- [ ] **Step 2: Run typecheck to verify it fails**

Run: `docker compose exec -T web npm exec tsc --noEmit`
Expected: FAIL because API types / draft state / dictionary keys are missing

- [ ] **Step 3: Add API types and capability-based UI branching**

```ts
export type AudioBriefingPersonaVoice = {
  persona: string;
  tts_provider: string;
  tts_model: string;
  voice_model: string;
  voice_style: string;
  // ...
};
```

```ts
const TTS_PROVIDER_CAPABILITIES: Record<string, {
  requiresVoiceStyle: boolean;
  supportsCatalogPicker: boolean;
  supportsSeparateTTSModel: boolean;
  supportsSpeechTuning: boolean;
}> = {
  aivis: { requiresVoiceStyle: true, supportsCatalogPicker: true, supportsSeparateTTSModel: false, supportsSpeechTuning: true },
  xai: { requiresVoiceStyle: false, supportsCatalogPicker: true, supportsSeparateTTSModel: false, supportsSpeechTuning: false },
  openai: { requiresVoiceStyle: false, supportsCatalogPicker: true, supportsSeparateTTSModel: true, supportsSpeechTuning: false },
  mock: { requiresVoiceStyle: false, supportsCatalogPicker: false, supportsSeparateTTSModel: false, supportsSpeechTuning: false },
};
```

- [ ] **Step 4: Add OpenAI picker page and dictionaries**

```tsx
{openAITTSVoicesSyncing ? t("settings.audioBriefing.syncingOpenAITTSCatalog") : t("settings.audioBriefing.refreshOpenAITTSCatalog")}
```

- [ ] **Step 5: Run typecheck**

Run: `docker compose exec -T web npm exec tsc --noEmit`
Expected: PASS

- [ ] **Step 6: Commit**

```bash
git add web/src/app/\(main\)/openai-tts-voices/page.tsx web/src/app/\(main\)/settings/page.tsx web/src/lib/api.ts web/src/i18n/dictionaries/ja.ts web/src/i18n/dictionaries/en.ts
git commit -m "feat: add openai tts settings ui"
```

### Task 6: Final integration and verification

**Files:**
- Modify: `api/internal/service/summary_audio_player.go`
- Modify: `api/internal/service/audio_briefing_voice.go`
- Modify: `web/src/app/(main)/settings/page.tsx`
- Modify: `worker/app/services/audio_briefing_tts.py`
- Modify: `worker/app/services/summary_audio_player.py`

- [ ] **Step 1: Apply migrations**

Run: `make migrate-up`
Expected: migration `123/u` (and `122/u` before it) are applied successfully

- [ ] **Step 2: Verify migration version**

Run: `make migrate-version`
Expected: latest version matches `123`

- [ ] **Step 3: Run Go formatting and tests**

Run: `make fmt-go`
Expected: Go files formatted

Run: `docker compose exec -T api go test ./internal/repository ./internal/handler ./internal/service`
Expected: PASS

- [ ] **Step 4: Run worker verification**

Run: `make check-worker`
Expected: PASS

- [ ] **Step 5: Run web verification**

Run: `docker compose exec -T web npm exec tsc --noEmit`
Expected: PASS

Run: `make web-build`
Expected: PASS, or if an existing external-font/network issue remains, capture the exact failure and confirm no new OpenAI TTS regression is involved

- [ ] **Step 6: Commit**

```bash
git add api/internal/service/summary_audio_player.go api/internal/service/audio_briefing_voice.go web/src/app/\(main\)/settings/page.tsx worker/app/services/audio_briefing_tts.py worker/app/services/summary_audio_player.py
git commit -m "feat: finish openai tts integration"
```
