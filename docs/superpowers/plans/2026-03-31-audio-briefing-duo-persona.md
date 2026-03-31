# Audio Briefing Duo Persona Implementation Plan

> **For agentic workers:** REQUIRED: Use superpowers:subagent-driven-development (if subagents available) or superpowers:executing-plans to implement this plan. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 音声ブリーフィングに `single / duo` の会話モード切替を追加し、`single` は既存実装を完全温存したまま、`duo` で `host + partner` の二人会話配信を生成できるようにする。

**Architecture:** 上位 pipeline は `conversation_mode` を見て strategy を選ぶだけに留め、既存 `single` の script / narration / voicing のコードパスはそのまま残す。`duo` は別の script schema、turn ベース narration、speaker ごとの TTS chunking を追加し、設定を `single` に戻すだけで即時 rollback できる構成にする。

**Tech Stack:** Go, PostgreSQL migrations, existing audio briefing repository/service flow, FastAPI worker, structured LLM output, Aivis-based TTS upload flow, Next.js App Router, React Query, i18n dictionaries

---

## File Map

### DB / backend model / repository

- Create: `db/migrations/000113_add_audio_briefing_conversation_mode.up.sql`
  - `audio_briefing_settings` と `audio_briefing_jobs` に `conversation_mode` と `partner_persona`、必要な進行管理列を追加する
- Create: `db/migrations/000113_add_audio_briefing_conversation_mode.down.sql`
  - 追加列の rollback
- Modify: `api/internal/model/model.go`
  - `AudioBriefingSettings`, `AudioBriefingJob`, narration/turn 系 DTO を追加する
- Modify: `api/internal/repository/audio_briefings.go`
  - settings / jobs の scan, upsert, insert, update に新列を追加する
- Modify: `api/internal/repository/audio_briefings_test.go`
  - settings/job の保存・取得を固定する

### Backend service / pipeline / worker client

- Modify: `api/internal/service/audio_briefing_pipeline.go`
  - `conversation_mode` ごとの strategy 選択、host / partner 選出、mode 別 stage 管理を追加する
- Create: `api/internal/service/audio_briefing_strategy.go`
  - `single` と `duo` の script / narration / voicing strategy interface を定義する
- Create: `api/internal/service/audio_briefing_strategy_single.go`
  - 既存 single の wrapper。既存コードを直接温存しつつ strategy 実装へ載せる
- Create: `api/internal/service/audio_briefing_strategy_duo.go`
  - duo 用 host/partner 決定、worker payload 構築、turn narration 生成を担当する
- Modify: `api/internal/service/audio_briefing_generation.go`
  - `single` の draft/chunk 組み立てを維持しつつ、`duo` 用 turn-based draft builder を追加する
- Modify: `api/internal/service/audio_briefing_voice.go`
  - chunk 単位で persona voice を引く現状を維持しつつ、duo chunk の speaker/partner voice を使えるようにする
- Modify: `api/internal/service/worker.go`
  - worker の `/audio-briefing-script` request/response DTO に `conversation_mode`, `host_persona`, `partner_persona`, `turns` 系 schema を追加する
- Modify: `api/internal/service/persona_mode.go`
  - `host` の random 選出を single と同じロジックで使い回し、partner 用再抽選 helper を追加する
- Create: `api/internal/service/audio_briefing_duo_test.go`
  - duo strategy の host / partner 選出、fallback、turn chunk 化を固定する

### Worker / prompt / router

- Modify: `worker/app/routers/audio_briefing_script.py`
  - request に `conversation_mode`, `host_persona`, `partner_persona` を追加し、response に `turns` を含める
- Modify: `worker/app/services/feed_task_common.py`
  - `single` 用既存 prompt を温存しつつ、`duo` 用 prompt builder / schema / parse を追加する
- Modify: `worker/app/services/test_feed_task_common.py`
  - duo prompt / parse / validation テストを追加する
- Modify: provider service files that call `build_audio_briefing_script_task(...)`
  - `worker/app/services/claude_service.py`
  - `worker/app/services/openai_service.py`
  - `worker/app/services/openrouter_service.py`
  - `worker/app/services/moonshot_service.py`
  - `worker/app/services/gemini_service.py`
  - 他の同系 service
  - 既存 single を壊さず、duo request payload を渡せるようにする

### Settings API / frontend

- Modify: `api/internal/handler/settings.go`
  - audio briefing settings payload に `conversation_mode` を追加する
- Modify: `api/internal/service/settings_service.go`
  - settings レスポンス整形に `conversation_mode` を追加する
- Modify: `web/src/lib/api.ts`
  - `UserSettings["audio_briefing"]` と audio briefing detail types に新フィールドを追加する
- Modify: `web/src/app/(main)/settings/page.tsx`
  - audio briefing 設定 UI に `single / duo` 切替を追加する
- Modify: `web/src/i18n/dictionaries/ja.ts`
- Modify: `web/src/i18n/dictionaries/en.ts`
  - ラベルと説明文を追加する

### Docs / verification

- Modify: `docs/superpowers/specs/2026-03-31-audio-briefing-duo-persona-design.md`
  - 今回追加した `single温存 / host random 踏襲` の追記を commit へ載せる
- Create: `docs/superpowers/plans/2026-03-31-audio-briefing-duo-persona.md`
  - 本計画書
- Copy to Obsidian: `/Users/minoru-kitayama/private/obsidian/100_private/114_Sifto/2026-03-31_音声ブリーフィング二人会話実装計画.md`

### Verification commands

- `make migrate-up`
- `make migrate-version`
- `make fmt-go`
- `docker compose exec -T api go test ./internal/repository ./internal/service ./internal/handler`
- `docker compose exec -T worker python -m unittest app.services.test_feed_task_common`
- `make check-worker`
- `make web-build`

---

## Chunk 1: Settings And Persistence

### Task 1: Add conversation mode columns

**Files:**
- Create: `db/migrations/000113_add_audio_briefing_conversation_mode.up.sql`
- Create: `db/migrations/000113_add_audio_briefing_conversation_mode.down.sql`
- Test: `make migrate-up`, `make migrate-version`

- [ ] **Step 1: Write the failing migration plan**

Add columns:

```sql
ALTER TABLE audio_briefing_settings
  ADD COLUMN IF NOT EXISTS conversation_mode TEXT NOT NULL DEFAULT 'single';

ALTER TABLE audio_briefing_jobs
  ADD COLUMN IF NOT EXISTS conversation_mode TEXT NOT NULL DEFAULT 'single',
  ADD COLUMN IF NOT EXISTS partner_persona TEXT,
  ADD COLUMN IF NOT EXISTS pipeline_stage TEXT;
```

Add `CHECK (conversation_mode IN ('single', 'duo'))`.

- [ ] **Step 2: Apply migration locally**

Run: `make migrate-up`  
Expected: migration `113/u` applies successfully

- [ ] **Step 3: Verify migration version**

Run: `make migrate-version`  
Expected: `113`

- [ ] **Step 4: Commit**

```bash
git add db/migrations/000113_add_audio_briefing_conversation_mode.*
git commit -m "音声ブリーフィング会話モード列を追加"
```

### Task 2: Extend audio briefing settings and jobs

**Files:**
- Modify: `api/internal/model/model.go`
- Modify: `api/internal/repository/audio_briefings.go`
- Test: `docker compose exec -T api go test ./internal/repository`

- [ ] **Step 1: Extend settings/job structs**

Add:

```go
ConversationMode string `json:"conversation_mode"`
PartnerPersona   *string `json:"partner_persona,omitempty"`
PipelineStage    *string `json:"pipeline_stage,omitempty"`
```

to the appropriate structs.

- [ ] **Step 2: Update settings repository scans and upserts**

Include `conversation_mode` in:

- `EnsureSettingsDefaults`
- `GetSettings`
- `ListEnabledSettings`
- `UpsertSettings`

- [ ] **Step 3: Update job scans and inserts**

Include new fields in:

- `CreatePendingJob`
- `GetJobByID`
- `GetJobBySlotKey`
- `ListJobsByUser`
- stage/status updates that read/write `pipeline_stage`

- [ ] **Step 4: Run focused repository tests**

Run: `docker compose exec -T api go test ./internal/repository`  
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add api/internal/model/model.go api/internal/repository/audio_briefings.go
git commit -m "音声ブリーフィング設定とjobに会話モードを追加"
```

---

## Chunk 2: Strategy Scaffold And Persona Selection

### Task 3: Introduce strategy interfaces without changing single behavior

**Files:**
- Create: `api/internal/service/audio_briefing_strategy.go`
- Create: `api/internal/service/audio_briefing_strategy_single.go`
- Modify: `api/internal/service/audio_briefing_pipeline.go`
- Test: `docker compose exec -T api go test ./internal/service`

- [ ] **Step 1: Define strategy interfaces**

Create interfaces similar to:

```go
type audioBriefingScriptStrategy interface {
    BuildScript(ctx context.Context, job *model.AudioBriefingJob, settings *model.AudioBriefingSettings, items []model.AudioBriefingJobItem) (audioBriefingScriptResult, error)
}

type audioBriefingNarrationStrategy interface {
    BuildDraft(job *model.AudioBriefingJob, items []model.AudioBriefingJobItem, voice *model.AudioBriefingPersonaVoice, targetChars int, script audioBriefingScriptResult) (AudioBriefingDraft, error)
}
```

- [ ] **Step 2: Wrap existing single flow**

Move existing single logic behind `single` strategy wrappers without changing output.

- [ ] **Step 3: Switch pipeline to mode dispatch**

In `continuePipeline` / `runScriptingStage`, select strategy from `job.ConversationMode`, defaulting to `single`.

- [ ] **Step 4: Run service tests**

Run: `docker compose exec -T api go test ./internal/service`  
Expected: PASS with no single regression

- [ ] **Step 5: Commit**

```bash
git add api/internal/service/audio_briefing_strategy.go api/internal/service/audio_briefing_strategy_single.go api/internal/service/audio_briefing_pipeline.go
git commit -m "音声ブリーフィングにsingle strategyを導入"
```

### Task 4: Add duo persona selection helpers

**Files:**
- Modify: `api/internal/service/persona_mode.go`
- Create: `api/internal/service/audio_briefing_duo_test.go`
- Modify: `api/internal/service/audio_briefing_pipeline.go`

- [ ] **Step 1: Add partner selection helper**

Implement helper like:

```go
func ResolvePartnerPersonaRandom(host string, voices []model.AudioBriefingPersonaVoice) (string, bool)
```

Requirements:

- exclude `host`
- only choose personas with configured voices
- return `false` when no partner is available

- [ ] **Step 2: Preserve host random behavior**

Ensure duo host selection still calls `ResolvePersonaAvoidRecent(...)` with the same inputs used by single.

- [ ] **Step 3: Add tests**

Cover:

- fixed host + random partner
- random host follows existing single behavior
- partner excludes host
- no partner available => fallback indicator

- [ ] **Step 4: Run focused tests**

Run: `docker compose exec -T api go test ./internal/service -run 'TestResolvePersonaAvoidRecent|TestAudioBriefingDuo'`  
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add api/internal/service/persona_mode.go api/internal/service/audio_briefing_duo_test.go api/internal/service/audio_briefing_pipeline.go
git commit -m "音声ブリーフィングduoのpersona選出を追加"
```

---

## Chunk 3: Worker Script Contract For Duo

### Task 5: Add duo request/response schema to worker router

**Files:**
- Modify: `worker/app/routers/audio_briefing_script.py`
- Modify: `api/internal/service/worker.go`
- Test: `make check-worker`, `docker compose exec -T api go test ./internal/service`

- [ ] **Step 1: Extend worker request schema**

Add request fields:

```python
conversation_mode: str = "single"
host_persona: str | None = None
partner_persona: str | None = None
```

- [ ] **Step 2: Add duo turn models**

Add response models like:

```python
class AudioBriefingScriptTurn(BaseModel):
    speaker: str
    section: str
    item_id: str | None = None
    text: str
```

and make duo response carry `turns`.

- [ ] **Step 3: Mirror the contract in Go worker client**

Update `WorkerClient` request/response types so API can send/receive duo schema.

- [ ] **Step 4: Run syntax and compile checks**

Run:

- `make check-worker`
- `docker compose exec -T api go test ./internal/service`

Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add worker/app/routers/audio_briefing_script.py api/internal/service/worker.go
git commit -m "音声ブリーフィングduoのworker契約を追加"
```

### Task 6: Add duo prompt builder and parser

**Files:**
- Modify: `worker/app/services/feed_task_common.py`
- Modify: `worker/app/services/test_feed_task_common.py`
- Modify: provider service files that call `build_audio_briefing_script_task(...)`

- [ ] **Step 1: Split prompt builder by mode**

Keep `single` prompt path unchanged. Add `duo` path:

- `host` explains article core
- `partner` reacts with comparison / question / angle
- fixed 5-turn article pattern

- [ ] **Step 2: Add duo parse validation**

Validate:

- valid `speaker in {"host","partner"}`
- valid `section`
- article turns keep input order
- partner does not disappear from any article

- [ ] **Step 3: Thread new args through provider services**

Update all `generate_audio_briefing_script_*` entrypoints so they pass `conversation_mode`, `host_persona`, and `partner_persona`.

- [ ] **Step 4: Run worker tests**

Run: `docker compose exec -T worker python -m unittest app.services.test_feed_task_common`  
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add worker/app/services/feed_task_common.py worker/app/services/test_feed_task_common.py worker/app/services/*_service.py
git commit -m "音声ブリーフィングduoのpromptとparseを追加"
```

---

## Chunk 4: Duo Draft Generation And Turn Chunking

### Task 7: Add duo narration and draft builder

**Files:**
- Modify: `api/internal/service/audio_briefing_generation.go`
- Create: `api/internal/service/audio_briefing_generation_duo_test.go`
- Modify: `api/internal/service/audio_briefing_pipeline.go`

- [ ] **Step 1: Add duo narration DTOs**

Define:

```go
type AudioBriefingNarrationTurn struct {
    Speaker string
    Section string
    ItemID  *string
    Text    string
}
```

Keep existing `AudioBriefingNarration` for single untouched.

- [ ] **Step 2: Add duo draft builder**

Create a new builder:

```go
func BuildAudioBriefingDraftFromTurns(...)
```

that emits chunks from ordered turns instead of monolithic article text.

- [ ] **Step 3: Keep single builder intact**

Do not change `BuildAudioBriefingDraftFromNarration(...)` behavior.

- [ ] **Step 4: Run service tests**

Run: `docker compose exec -T api go test ./internal/service -run 'TestBuildAudioBriefingDraft'`  
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add api/internal/service/audio_briefing_generation.go api/internal/service/audio_briefing_generation_duo_test.go api/internal/service/audio_briefing_pipeline.go
git commit -m "音声ブリーフィングduoのturn draft生成を追加"
```

### Task 8: Add speaker-aware chunk voicing

**Files:**
- Modify: `api/internal/model/model.go`
- Modify: `api/internal/service/audio_briefing_voice.go`
- Modify: `api/internal/repository/audio_briefings.go`
- Test: `docker compose exec -T api go test ./internal/service ./internal/repository`

- [ ] **Step 1: Add chunk speaker/persona fields if needed**

If current chunk row is insufficient, add fields like:

```go
Speaker *string `json:"speaker,omitempty"`
Persona *string `json:"persona,omitempty"`
```

and corresponding DB columns in a follow-up migration if required.

- [ ] **Step 2: Resolve voice per chunk persona**

Update voicing so `single` still uses `job.Persona`, while `duo` uses the chunk persona (`host` or `partner`) mapped to the right voice row.

- [ ] **Step 3: Add fallback behavior**

If a stored partner persona loses voice config before voicing, retry partner reselection only before first duo chunk creation; after draft creation, fail fast instead of silently mutating recorded narration.

- [ ] **Step 4: Run focused tests**

Run: `docker compose exec -T api go test ./internal/service ./internal/repository`  
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add api/internal/model/model.go api/internal/service/audio_briefing_voice.go api/internal/repository/audio_briefings.go
git commit -m "音声ブリーフィングduoのspeaker別voiceを追加"
```

---

## Chunk 5: Pipeline Wiring And Fallback

### Task 9: Implement duo pipeline stage handling

**Files:**
- Modify: `api/internal/service/audio_briefing_pipeline.go`
- Modify: `api/internal/repository/audio_briefings.go`
- Modify: `api/internal/service/audio_briefing_pipeline_test.go`

- [ ] **Step 1: Add mode-specific stage names**

Use:

- `single_script`, `single_voice`, `single_concat`
- `duo_script`, `duo_voice`, `duo_concat`

while preserving the existing external status model.

- [ ] **Step 2: Fix job creation to persist mode and partner**

When creating jobs:

- save `conversation_mode`
- save `partner_persona` for duo
- save mode-specific `pipeline_stage`

- [ ] **Step 3: Add duo fallback to single**

If duo preconditions fail before script generation:

- no partner voice candidate
- invalid duo worker response

then explicitly fall back to single and log it.

- [ ] **Step 4: Add tests**

Cover:

- single job path unchanged
- duo job path stores partner
- duo preflight failure falls back to single

- [ ] **Step 5: Run pipeline tests**

Run: `docker compose exec -T api go test ./internal/service -run 'TestAudioBriefing'`  
Expected: PASS

- [ ] **Step 6: Commit**

```bash
git add api/internal/service/audio_briefing_pipeline.go api/internal/repository/audio_briefings.go api/internal/service/audio_briefing_pipeline_test.go
git commit -m "音声ブリーフィングduo pipelineを接続する"
```

---

## Chunk 6: Settings API And UI Toggle

### Task 10: Expose conversation mode through settings API

**Files:**
- Modify: `api/internal/handler/settings.go`
- Modify: `api/internal/service/settings_service.go`
- Modify: `api/internal/service/settings_service_test.go`

- [ ] **Step 1: Add `conversation_mode` to API payload**

Include it in:

- audio briefing settings response
- audio briefing settings save request
- normalized defaults

- [ ] **Step 2: Validate allowed values**

Reject anything except `single` / `duo`.

- [ ] **Step 3: Run backend tests**

Run: `docker compose exec -T api go test ./internal/service ./internal/handler`  
Expected: PASS

- [ ] **Step 4: Commit**

```bash
git add api/internal/handler/settings.go api/internal/service/settings_service.go api/internal/service/settings_service_test.go
git commit -m "音声ブリーフィング会話モード設定をAPIに追加"
```

### Task 11: Add settings UI toggle and copy

**Files:**
- Modify: `web/src/lib/api.ts`
- Modify: `web/src/app/(main)/settings/page.tsx`
- Modify: `web/src/i18n/dictionaries/ja.ts`
- Modify: `web/src/i18n/dictionaries/en.ts`
- Test: `make web-build`

- [ ] **Step 1: Extend frontend types**

Add `conversation_mode: "single" | "duo"` to audio briefing settings type.

- [ ] **Step 2: Add form state and save payload**

Mirror the existing `default_persona_mode` form handling pattern.

- [ ] **Step 3: Add UI controls and help text**

Show:

- `single`: 一人読み上げ
- `duo`: 二人会話

Help text should clarify:

- fixed persona => configured persona is host
- random persona => host uses the current random logic and partner is another random persona

- [ ] **Step 4: Update i18n**

Add both Japanese and English strings.

- [ ] **Step 5: Run frontend build**

Run: `make web-build`  
Expected: PASS

- [ ] **Step 6: Commit**

```bash
git add web/src/lib/api.ts web/src/app/(main)/settings/page.tsx web/src/i18n/dictionaries/ja.ts web/src/i18n/dictionaries/en.ts
git commit -m "音声ブリーフィング会話モード設定UIを追加"
```

---

## Chunk 7: Verification, Docs, And Rollout

### Task 12: Verify single regression and duo happy path

**Files:**
- Modify as needed: tests touched above

- [ ] **Step 1: Run full relevant verification**

Run:

- `make migrate-up`
- `make migrate-version`
- `make fmt-go`
- `docker compose exec -T api go test ./internal/repository ./internal/service ./internal/handler`
- `docker compose exec -T worker python -m unittest app.services.test_feed_task_common`
- `make check-worker`
- `make web-build`

Expected: all PASS

- [ ] **Step 2: Smoke-check fallback behavior**

Manually verify in tests or fixtures:

- `single` still uses the exact existing path
- `duo` with no partner voice falls back to `single`

- [ ] **Step 3: Commit**

```bash
git add .
git commit -m "音声ブリーフィングduo実装を検証する"
```

### Task 13: Save docs and sync Obsidian copy

**Files:**
- Modify: `docs/superpowers/specs/2026-03-31-audio-briefing-duo-persona-design.md`
- Create: `docs/superpowers/plans/2026-03-31-audio-briefing-duo-persona.md`
- Copy: `/Users/minoru-kitayama/private/obsidian/100_private/114_Sifto/2026-03-31_音声ブリーフィング二人会話設計.md`
- Copy: `/Users/minoru-kitayama/private/obsidian/100_private/114_Sifto/2026-03-31_音声ブリーフィング二人会話実装計画.md`

- [ ] **Step 1: Commit the updated spec text**

Include the already-reviewed refinements:

- `single` path remains intact
- `duo` is a separate strategy
- `duo + random` means host follows current random logic and partner is another random persona

- [ ] **Step 2: Copy plan and spec to Obsidian**

Use the existing Obsidian note path conventions.

- [ ] **Step 3: Commit docs**

```bash
git add docs/superpowers/specs/2026-03-31-audio-briefing-duo-persona-design.md docs/superpowers/plans/2026-03-31-audio-briefing-duo-persona.md
git commit -m "音声ブリーフィングduoの設計と計画を整理する"
```

---

Plan complete and saved to `docs/superpowers/plans/2026-03-31-audio-briefing-duo-persona.md`. Ready to execute?
