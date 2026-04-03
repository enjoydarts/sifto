# xAI TTS追加 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add xAI TTS as a first-class provider with Aivis-equivalent sync, picker, settings, and playback coverage across Audio Briefing and Summary Audio.

**Architecture:** Keep the existing Aivis slice intact and add a parallel xAI slice for catalog sync, picker UX, and synthesis. Reuse the existing `tts_provider` flow for persona voice persistence and summary audio playback, while introducing xAI-specific repository, handler, worker service, and UI components.

**Tech Stack:** Go API, PostgreSQL migrations, Python worker, Next.js App Router, TypeScript, i18n dictionaries, Docker Compose / make

---

### Task 1: Add xAI voice catalog persistence

**Files:**
- Create: `db/migrations/000121_add_xai_voice_catalog.up.sql`
- Create: `db/migrations/000121_add_xai_voice_catalog.down.sql`
- Create: `api/internal/repository/xai_voices.go`
- Test: `api/internal/repository/xai_voices_test.go`

- [ ] **Step 1: Write the failing repository test**

```go
func TestXAIVoiceRepoInsertAndListLatestSnapshots(t *testing.T) {
	db := testDB(t)
	repo := NewXAIVoiceRepo(db)

	runID, err := repo.StartSyncRun(context.Background(), "manual")
	if err != nil {
		t.Fatalf("StartSyncRun() error = %v", err)
	}
	fetchedAt := time.Date(2026, 4, 3, 10, 0, 0, 0, time.UTC)
	err = repo.InsertSnapshots(context.Background(), runID, fetchedAt, []XAIVoiceSnapshot{
		{VoiceID: "voice-1", Name: "Grok Voice 1", Description: "Warm", Language: "en", PreviewURL: "https://example.com/1.mp3"},
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
		t.Fatalf("latestRun = %#v, want succeeded run", latestRun)
	}
	if len(rows) != 1 || rows[0].VoiceID != "voice-1" {
		t.Fatalf("rows = %#v, want voice-1", rows)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `docker compose exec -T api go test ./internal/repository -run TestXAIVoiceRepoInsertAndListLatestSnapshots`
Expected: FAIL with missing migration / repo / types

- [ ] **Step 3: Add migration and repo**

```sql
CREATE TABLE xai_voice_sync_runs (
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

CREATE TABLE xai_voice_snapshots (
  id bigserial PRIMARY KEY,
  sync_run_id bigint NOT NULL REFERENCES xai_voice_sync_runs(id) ON DELETE CASCADE,
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

```go
type XAIVoiceSnapshot struct {
	ID           int64
	SyncRunID    int64
	VoiceID      string
	Name         string
	Description  string
	Language     string
	PreviewURL   string
	MetadataJSON []byte
	FetchedAt    time.Time
}
```

- [ ] **Step 4: Run repository test to verify it passes**

Run: `docker compose exec -T api go test ./internal/repository -run TestXAIVoiceRepoInsertAndListLatestSnapshots`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add db/migrations/000121_add_xai_voice_catalog.up.sql db/migrations/000121_add_xai_voice_catalog.down.sql api/internal/repository/xai_voices.go api/internal/repository/xai_voices_test.go
git commit -m "feat: add xai voice catalog persistence"
```

### Task 2: Add xAI catalog fetch service and HTTP endpoints

**Files:**
- Create: `api/internal/service/xai_voice_catalog.go`
- Create: `api/internal/handler/xai_voices.go`
- Modify: `api/cmd/server/main.go`
- Modify: `api/internal/repository/provider_model_updates.go`
- Test: `api/internal/service/xai_voice_catalog_test.go`
- Test: `api/internal/handler/xai_voices_test.go`

- [ ] **Step 1: Write the failing service test**

```go
func TestXAIVoiceCatalogServiceFetchVoices(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/tts/voices" {
			t.Fatalf("path = %s, want /v1/tts/voices", r.URL.Path)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"voices": []map[string]any{
				{"voice_id": "voice-1", "name": "Calm", "description": "Warm", "language": "en"},
			},
		})
	}))
	defer srv.Close()

	svc := NewXAIVoiceCatalogServiceWithBaseURL(srv.URL)
	rows, err := svc.FetchVoices(context.Background(), "test-key")
	if err != nil {
		t.Fatalf("FetchVoices() error = %v", err)
	}
	if len(rows) != 1 || rows[0].VoiceID != "voice-1" {
		t.Fatalf("rows = %#v, want voice-1", rows)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `docker compose exec -T api go test ./internal/service -run TestXAIVoiceCatalogServiceFetchVoices`
Expected: FAIL with missing service

- [ ] **Step 3: Implement service and handler**

```go
func (s *XAIVoiceCatalogService) FetchVoices(ctx context.Context, apiKey string) ([]repository.XAIVoiceSnapshot, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, s.baseURL+"/v1/tts/voices", nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+strings.TrimSpace(apiKey))
	// decode payload -> []repository.XAIVoiceSnapshot
}
```

```go
func (h *XAIVoicesHandler) Sync(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r)
	apiKey, err := h.settingsRepo.GetXAIAPIKeyDecrypted(r.Context(), userID)
	if err != nil {
		http.Error(w, "xai api key is not configured", http.StatusBadRequest)
		return
	}
	// start run -> fetch -> insert snapshots -> provider updates -> respond with list
}
```

- [ ] **Step 4: Add route wiring**

```go
xaiVoiceRepo := repository.NewXAIVoiceRepo(db)
xaiVoiceCatalogSvc := service.NewXAIVoiceCatalogService()
xaiVoicesH := handler.NewXAIVoicesHandler(xaiVoiceRepo, userSettingsRepo, providerModelUpdateRepo, secretCipher, xaiVoiceCatalogSvc)

r.Route("/xai-voices", func(r chi.Router) {
	r.Get("/", xaiVoicesH.List)
	r.Get("/status", xaiVoicesH.Status)
	r.Post("/sync", xaiVoicesH.Sync)
})
```

- [ ] **Step 5: Run tests to verify they pass**

Run: `docker compose exec -T api go test ./internal/service ./internal/handler -run 'Test(XAIVoiceCatalogServiceFetchVoices|XAIVoicesHandler)'`
Expected: PASS

- [ ] **Step 6: Commit**

```bash
git add api/internal/service/xai_voice_catalog.go api/internal/service/xai_voice_catalog_test.go api/internal/handler/xai_voices.go api/internal/handler/xai_voices_test.go api/cmd/server/main.go api/internal/repository/provider_model_updates.go
git commit -m "feat: add xai voice sync endpoints"
```

### Task 3: Accept `tts_provider=xai` in saved persona voices

**Files:**
- Modify: `api/internal/service/settings_service.go`
- Modify: `api/internal/handler/settings.go`
- Modify: `api/internal/model/model.go`
- Modify: `api/internal/repository/audio_briefings.go`
- Test: `api/internal/service/settings_service_test.go`

- [ ] **Step 1: Write the failing test**

```go
func TestUpdateAudioBriefingPersonaVoicesAllowsXAI(t *testing.T) {
	svc := newSettingsServiceForTest(t)
	rows, err := svc.UpdateAudioBriefingPersonaVoices(context.Background(), "user-1", []UpdateAudioBriefingPersonaVoiceInput{
		{Persona: "editor", TTSProvider: "xai", VoiceModel: "voice-1", VoiceStyle: ""},
	})
	if err != nil {
		t.Fatalf("UpdateAudioBriefingPersonaVoices() error = %v", err)
	}
	if len(rows) != 1 || rows[0].TTSProvider != "xai" {
		t.Fatalf("rows = %#v, want xai", rows)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `docker compose exec -T api go test ./internal/service -run TestUpdateAudioBriefingPersonaVoicesAllowsXAI`
Expected: FAIL with invalid provider validation

- [ ] **Step 3: Implement minimal provider acceptance**

```go
switch strings.TrimSpace(strings.ToLower(in.TTSProvider)) {
case "aivis", "mock", "xai":
	// allowed
default:
	return nil, fmt.Errorf("invalid tts_provider")
}
```

- [ ] **Step 4: Run targeted test**

Run: `docker compose exec -T api go test ./internal/service -run TestUpdateAudioBriefingPersonaVoicesAllowsXAI`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add api/internal/service/settings_service.go api/internal/handler/settings.go api/internal/model/model.go api/internal/repository/audio_briefings.go api/internal/service/settings_service_test.go
git commit -m "feat: allow xai persona voice provider"
```

### Task 4: Add xAI synthesis to worker for Audio Briefing

**Files:**
- Modify: `worker/app/services/audio_briefing_tts.py`
- Create: `worker/app/services/xai_tts.py`
- Test: `worker/app/services/test_audio_briefing_tts.py`

- [ ] **Step 1: Write the failing worker test**

```python
def test_synthesize_and_upload_uses_xai_provider(self):
    service = AudioBriefingTTSService()
    with patch.object(service, "synthesize_xai_audio", return_value=(b"mp3", "audio/mpeg", ".mp3", 12)) as synth:
        with patch.object(service, "upload_bytes") as upload:
            object_key, duration_sec = service.synthesize_and_upload(
                provider="xai",
                voice_model="voice-1",
                voice_style="",
                text="hello",
                speech_rate=1.0,
                emotional_intensity=1.0,
                tempo_dynamics=1.0,
                line_break_silence_seconds=0.0,
                chunk_trailing_silence_seconds=0.0,
                pitch=0.0,
                volume_gain=0.0,
                output_object_key="audio/test",
            )
    synth.assert_called_once()
    upload.assert_called_once()
    self.assertEqual(duration_sec, 12)
    self.assertTrue(object_key.endswith(".mp3"))
```

- [ ] **Step 2: Run test to verify it fails**

Run: `docker compose exec -T worker python -m unittest app.services.test_audio_briefing_tts.AudioBriefingTTSServiceTests.test_synthesize_and_upload_uses_xai_provider`
Expected: FAIL with unsupported provider

- [ ] **Step 3: Implement xAI synthesize path**

```python
elif provider == "xai":
    payload, content_type, suffix, duration_sec = self.synthesize_xai_audio(
        voice_id=voice_model,
        text=text,
        speech_rate=speech_rate,
    )
```

```python
def synthesize_xai_audio(self, voice_id: str, text: str, speech_rate: float) -> tuple[bytes, str, str, int]:
    response = httpx.post(
        f"{self.xai_tts_endpoint}/v1/tts",
        headers={"Authorization": f"Bearer {self.xai_api_key}"},
        json={"input": text, "voice_id": voice_id, "format": "mp3"},
        timeout=self.xai_timeout_sec,
    )
    response.raise_for_status()
    return response.content, "audio/mpeg", ".mp3", estimate_audio_duration_sec(text, speech_rate)
```

- [ ] **Step 4: Run targeted worker tests**

Run: `docker compose exec -T worker python -m unittest app.services.test_audio_briefing_tts`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add worker/app/services/audio_briefing_tts.py worker/app/services/xai_tts.py worker/app/services/test_audio_briefing_tts.py
git commit -m "feat: add xai audio briefing synthesis"
```

### Task 5: Add xAI synthesis to Summary Audio path

**Files:**
- Modify: `api/internal/service/summary_audio_player.go`
- Modify: `api/internal/service/worker.go`
- Modify: `api/internal/service/worker_test.go`
- Modify: `worker/app/services/summary_audio_player.py`
- Modify: `worker/app/routers/summary_audio_player.py`
- Test: `worker/app/services/test_summary_audio_player.py`

- [ ] **Step 1: Write the failing tests**

```go
func TestSynthesizeSummaryAudioIncludesXAIKeyHeader(t *testing.T) {
	// assert worker client forwards xai api key for summary audio calls
}
```

```python
def test_synthesize_supports_xai_provider(self):
    service = SummaryAudioPlayerService()
    with patch("app.services.summary_audio_player.AudioBriefingTTSService.synthesize_xai_audio", return_value=(b"mp3", "audio/mpeg", ".mp3", 9)):
        audio_base64, content_type, duration_sec, resolved_text = service.synthesize(
            provider="xai",
            voice_model="voice-1",
            voice_style="",
            text="title\n\nsummary",
            speech_rate=1.0,
            emotional_intensity=1.0,
            tempo_dynamics=1.0,
            line_break_silence_seconds=0.0,
            chunk_trailing_silence_seconds=0.0,
            pitch=0.0,
            volume_gain=0.0,
        )
    self.assertEqual(content_type, "audio/mpeg")
```

- [ ] **Step 2: Run failing tests**

Run: `docker compose exec -T api go test ./internal/service -run TestSynthesizeSummaryAudioIncludesXAIKeyHeader`
Expected: FAIL

Run: `docker compose exec -T worker python -m unittest app.services.test_summary_audio_player.SummaryAudioPlayerTests.test_synthesize_supports_xai_provider`
Expected: FAIL

- [ ] **Step 3: Implement minimal wiring**

```go
var xaiAPIKey *string
if strings.EqualFold(strings.TrimSpace(voice.TTSProvider), "xai") {
	xaiAPIKey, err = s.loadXAIAPIKey(ctx, userID)
	if err != nil {
		return nil, err
	}
}
```

```go
return postWithHeaders[SummaryAudioSynthesizeResponse](ctx, w, "/summary-audio/synthesize", requestBody, workerHeaders(nil, nil, nil, nil, nil, nil, xaiAPIKey, nil, nil, nil, aivisAPIKey, w.internalSecret))
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `docker compose exec -T api go test ./internal/service -run TestSynthesizeSummaryAudioIncludesXAIKeyHeader`
Expected: PASS

Run: `docker compose exec -T worker python -m unittest app.services.test_summary_audio_player`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add api/internal/service/summary_audio_player.go api/internal/service/worker.go api/internal/service/worker_test.go worker/app/services/summary_audio_player.py worker/app/routers/summary_audio_player.py worker/app/services/test_summary_audio_player.py
git commit -m "feat: add xai summary audio synthesis"
```

### Task 6: Build xAI voices UI and picker

**Files:**
- Create: `web/src/app/(main)/xai-voices/page.tsx`
- Create: `web/src/components/xai-voice-picker.tsx`
- Modify: `web/src/lib/api.ts`
- Modify: `web/src/app/(main)/settings/page.tsx`
- Modify: `web/src/i18n/dictionaries/ja.ts`
- Modify: `web/src/i18n/dictionaries/en.ts`

- [ ] **Step 1: Write the failing type-level/UI assertions**

```ts
type XAIVoice = {
  voice_id: string;
  name: string;
  description: string;
  language: string | null;
  preview_url: string | null;
};
```

Add tests or compile targets so these new API shapes are required by the page and picker props.

- [ ] **Step 2: Run typecheck to verify it fails**

Run: `docker compose exec -T web npm exec tsc --noEmit`
Expected: FAIL with missing xAI API types / component props

- [ ] **Step 3: Implement page and picker**

```tsx
{voice.tts_provider === "xai" ? (
  <button type="button" onClick={() => void openXAIPicker(voice.persona)}>
    {t("settings.audioBriefing.pickXAIVoice")}
  </button>
) : null}
```

```tsx
<select
  value={voice.tts_provider}
  onChange={(e) => updateAudioBriefingVoice(voice.persona, { tts_provider: e.target.value })}
>
  {Array.from(new Set([voice.tts_provider, "aivis", "xai", "mock"])).map((provider) => (
    <option key={provider} value={provider}>{provider}</option>
  ))}
</select>
```

- [ ] **Step 4: Add i18n strings**

```ts
"settings.audioBriefing.pickXAIVoice": "xAI voice を選ぶ",
"xaiVoices.title": "xAI Voices",
"xaiVoices.sync": "音声一覧を更新",
```

Add matching English keys in `web/src/i18n/dictionaries/en.ts`.

- [ ] **Step 5: Run web checks**

Run: `docker compose exec -T web npm exec tsc --noEmit`
Expected: PASS

Run: `make web-lint`
Expected: no new warnings/errors beyond baseline

- [ ] **Step 6: Commit**

```bash
git add web/src/app/'(main)'/xai-voices/page.tsx web/src/components/xai-voice-picker.tsx web/src/lib/api.ts web/src/app/'(main)'/settings/page.tsx web/src/i18n/dictionaries/ja.ts web/src/i18n/dictionaries/en.ts
git commit -m "feat: add xai voice catalog UI"
```

### Task 7: End-to-end validation and cleanup

**Files:**
- Modify: any touched files for small fixes only

- [ ] **Step 1: Apply migrations**

Run: `make migrate-up`
Expected: new xAI voice catalog migration applies successfully

- [ ] **Step 2: Verify migration version**

Run: `make migrate-version`
Expected: latest version includes the new migration number

- [ ] **Step 3: Run Go tests**

Run: `docker compose exec -T api go test ./internal/handler ./internal/repository ./internal/service`
Expected: PASS

- [ ] **Step 4: Run worker checks**

Run: `make check-worker`
Expected: PASS

- [ ] **Step 5: Run web checks**

Run: `docker compose exec -T web npm exec tsc --noEmit`
Expected: PASS

Run: `make web-build`
Expected: PASS, unless blocked by known external font/network issue

- [ ] **Step 6: Manual smoke checks**

1. Open Settings > Audio Briefing.
2. Change one persona to `tts_provider=xai`.
3. Open xAI picker and select a voice.
4. Save voices and reload the page.
5. Confirm the selected `voice_id` persists.
6. Trigger Summary Audio synthesis on an item and confirm audio returns.
7. Trigger an Audio Briefing job and confirm chunks synthesize with `tts_provider=xai`.

- [ ] **Step 7: Final commit**

```bash
git add .
git commit -m "feat: add xai tts provider"
```

## Self-Review

- Spec coverage:
  - DB sync/snapshots: Task 1
  - API sync/list/status: Task 2
  - `tts_provider=xai` persistence: Task 3
  - Audio Briefing synthesis: Task 4
  - Summary Audio synthesis: Task 5
  - Web list/picker/settings/i18n: Task 6
  - Verification: Task 7
- Placeholder scan:
  - No `TODO` / `TBD`
  - Commands and target files are concrete
- Type consistency:
  - `tts_provider="xai"`
  - `voice_model` stores `voice_id`
  - `voice_style` remains empty string for xAI
