# Playback History And Resume Implementation Plan

> **For agentic workers:** REQUIRED: Use superpowers:subagent-driven-development (if subagents available) or superpowers:executing-plans to implement this plan. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 共通音声プレイヤーの要約連続再生と音声ブリーフィング再生について、永続的な `続きから再生` と専用の `再生履歴` ページを追加する。

**Architecture:** API 側に `playback_sessions` と `playback_events` を追加し、共通プレイヤーが再生状態を継続保存する。復元は session snapshot から行い、履歴一覧は session を直接表示する。event は最小限の記録に留め、分析用途へ将来拡張できるようにする。

**Tech Stack:** Go API, PostgreSQL migration, Next.js App Router, React Query, shared audio player provider

---

## File Map

- Create: `db/migrations/000102_create_playback_sessions_and_events.up.sql`
- Create: `db/migrations/000102_create_playback_sessions_and_events.down.sql`
- Create: `api/internal/model/playback_session.go`
- Create: `api/internal/repository/playback_sessions.go`
- Create: `api/internal/repository/playback_sessions_test.go`
- Create: `api/internal/service/playback_sessions.go`
- Create: `api/internal/service/playback_sessions_test.go`
- Create: `api/internal/handler/playback_sessions.go`
- Modify: `api/cmd/server/main.go`
- Modify: `web/src/lib/api.ts`
- Modify: `web/src/components/shared-audio-player/types.ts`
- Modify: `web/src/components/shared-audio-player/provider.tsx`
- Create: `web/src/app/(main)/playback-history/page.tsx`
- Modify: `web/src/app/(main)/layout.tsx`
- Modify: `web/src/i18n/dictionaries/ja.ts`
- Modify: `web/src/i18n/dictionaries/en.ts`

## Chunk 1: Playback Session Persistence

### Task 1: Add playback session tables

**Files:**
- Create: `db/migrations/000102_create_playback_sessions_and_events.up.sql`
- Create: `db/migrations/000102_create_playback_sessions_and_events.down.sql`

- [ ] **Step 1: Write migration for `playback_sessions`**

Create table with:
- `id uuid primary key`
- `user_id uuid not null`
- `mode text not null`
- `status text not null`
- `title text not null default ''`
- `subtitle text not null default ''`
- `current_position_sec integer not null default 0`
- `duration_sec integer not null default 0`
- `progress_ratio double precision`
- `resume_payload jsonb not null default '{}'::jsonb`
- `started_at timestamptz not null`
- `updated_at timestamptz not null`
- `completed_at timestamptz`

Add indexes:
- `(user_id, mode, updated_at desc)`
- `(user_id, updated_at desc)`

- [ ] **Step 2: Write migration for `playback_events`**

Create table with:
- `id uuid primary key`
- `session_id uuid not null references playback_sessions(id) on delete cascade`
- `user_id uuid not null`
- `mode text not null`
- `event_type text not null`
- `position_sec integer not null default 0`
- `payload jsonb not null default '{}'::jsonb`
- `created_at timestamptz not null default now()`

Add indexes:
- `(session_id, created_at asc)`
- `(user_id, created_at desc)`

- [ ] **Step 3: Run migration**

Run: `make migrate-up`

Expected: migration version advances to `102`

- [ ] **Step 4: Verify migration version**

Run: `make migrate-version`

Expected: output includes `102`

- [ ] **Step 5: Commit**

```bash
git add db/migrations/000102_create_playback_sessions_and_events.up.sql db/migrations/000102_create_playback_sessions_and_events.down.sql
git commit -m "再生履歴のsessionとeventテーブルを追加する"
```

### Task 2: Add API models and repository

**Files:**
- Create: `api/internal/model/playback_session.go`
- Create: `api/internal/repository/playback_sessions.go`
- Create: `api/internal/repository/playback_sessions_test.go`

- [ ] **Step 1: Write failing repository tests**

Cover:
- session create
- latest by mode lookup
- list by user ordered by `updated_at desc`
- complete / interrupt update
- event insert

Example skeleton:

```go
func TestPlaybackSessionRepositoryLatestByMode(t *testing.T) {
    t.Parallel()
}
```

- [ ] **Step 2: Run repository tests to verify failure**

Run: `docker compose exec -T api go test ./internal/repository -run TestPlayback`

Expected: FAIL because repository/model do not exist yet

- [ ] **Step 3: Implement model types**

Add:
- `PlaybackSession`
- `PlaybackEvent`
- mode / status / event type constants

- [ ] **Step 4: Implement repository**

Include methods:
- `CreateSession`
- `UpdateSessionProgress`
- `CompleteSession`
- `InterruptSession`
- `LatestSessionByMode`
- `ListSessions`
- `CreateEvent`

- [ ] **Step 5: Run repository tests**

Run: `docker compose exec -T api go test ./internal/repository -run TestPlayback`

Expected: PASS

- [ ] **Step 6: Commit**

```bash
git add api/internal/model/playback_session.go api/internal/repository/playback_sessions.go api/internal/repository/playback_sessions_test.go
git commit -m "再生履歴repositoryを追加する"
```

### Task 3: Add playback session service and handler

**Files:**
- Create: `api/internal/service/playback_sessions.go`
- Create: `api/internal/service/playback_sessions_test.go`
- Create: `api/internal/handler/playback_sessions.go`
- Modify: `api/cmd/server/main.go`

- [ ] **Step 1: Write failing service tests**

Cover:
- summary session start builds correct snapshot
- audio briefing session start builds correct snapshot
- replacing active session interrupts previous one
- complete updates status and completed time

- [ ] **Step 2: Run service tests to verify failure**

Run: `docker compose exec -T api go test ./internal/service -run TestPlaybackSession`

Expected: FAIL

- [ ] **Step 3: Implement service**

Add methods such as:
- `StartSummarySession`
- `StartAudioBriefingSession`
- `UpdateProgress`
- `CompleteSession`
- `InterruptSession`
- `LatestSessions`
- `ListHistory`

Service rules:
- one latest active session per mode
- new session interrupts previous in-progress session for same mode
- cross-mode replacement interrupts old active session when the shared player switches mode

- [ ] **Step 4: Implement handler and routes**

Add endpoints:
- `GET /playback-sessions/latest`
- `GET /playback-sessions`
- `POST /playback-sessions`
- `PATCH /playback-sessions/{id}`
- `POST /playback-sessions/{id}/complete`
- `POST /playback-sessions/{id}/interrupt`

- [ ] **Step 5: Run API tests**

Run: `docker compose exec -T api go test ./internal/service ./internal/handler ./cmd/server`

Expected: PASS

- [ ] **Step 6: Commit**

```bash
git add api/internal/service/playback_sessions.go api/internal/service/playback_sessions_test.go api/internal/handler/playback_sessions.go api/cmd/server/main.go
git commit -m "再生履歴APIを追加する"
```

## Chunk 2: Shared Player Save And Resume

### Task 4: Extend web API client

**Files:**
- Modify: `web/src/lib/api.ts`

- [ ] **Step 1: Add failing type usage locally**

Define client types for:
- playback mode
- playback status
- latest sessions response
- history list response
- create / update session payloads

- [ ] **Step 2: Implement API client methods**

Add methods:
- `getLatestPlaybackSessions`
- `getPlaybackSessions`
- `createPlaybackSession`
- `updatePlaybackSession`
- `completePlaybackSession`
- `interruptPlaybackSession`

- [ ] **Step 3: Run web lint**

Run: `make web-lint`

Expected: PASS

- [ ] **Step 4: Commit**

```bash
git add web/src/lib/api.ts
git commit -m "再生履歴APIクライアントを追加する"
```

### Task 5: Add provider-side session persistence

**Files:**
- Modify: `web/src/components/shared-audio-player/types.ts`
- Modify: `web/src/components/shared-audio-player/provider.tsx`

- [ ] **Step 1: Add failing provider tests or local assertions**

If there is no existing provider test harness, document state transitions inline and implement incrementally.

Required behaviors:
- start summary session creates remote session
- start audio briefing session creates remote session
- pause / stop / complete update remote session
- replacing current mode interrupts previous session
- summary resume payload stores saved queue order and current index
- audio briefing resume payload stores briefing id and offset

- [ ] **Step 2: Implement provider session state**

Add:
- current `remoteSessionID`
- throttled progress save
- lifecycle hooks for `started / paused / resumed / stopped / completed / replaced`

- [ ] **Step 3: Implement summary resume snapshot**

Save:
- queue kind
- queue item ids
- current item id
- current queue index
- current item offset

- [ ] **Step 4: Implement audio briefing resume snapshot**

Save:
- briefing id
- current offset

- [ ] **Step 5: Add recovery helpers**

Provider should expose:
- `resumePlaybackSession(session)`

It should:
- rebuild summary queue from saved ids
- re-fetch current item detail
- restart briefing playback by id
- seek after audio source is ready

- [ ] **Step 6: Run web lint and build**

Run:
- `make web-lint`
- `make web-build`

Expected: PASS

- [ ] **Step 7: Commit**

```bash
git add web/src/components/shared-audio-player/types.ts web/src/components/shared-audio-player/provider.tsx
git commit -m "共通プレイヤーに再生session保存を追加する"
```

### Task 6: Add lightweight resume entry points

**Files:**
- Modify: `web/src/app/(main)/audio-player/page.tsx`
- Modify: `web/src/app/(main)/audio-briefings/[id]/page.tsx`
- Modify: `web/src/i18n/dictionaries/ja.ts`
- Modify: `web/src/i18n/dictionaries/en.ts`

- [ ] **Step 1: Add i18n keys**

Add keys for:
- `playbackHistory.resumeLatest`
- `playbackHistory.noResume`
- optional summary / briefing resume labels

- [ ] **Step 2: Add latest session query usage**

On summary page and audio briefing detail page:
- fetch latest session for relevant mode
- show `前回の続きを再生` only when resumable

- [ ] **Step 3: Wire to provider resume**

Use provider helper instead of starting fresh playback.

- [ ] **Step 4: Run web lint and build**

Run:
- `make web-lint`
- `make web-build`

Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add web/src/app/(main)/audio-player/page.tsx web/src/app/(main)/audio-briefings/[id]/page.tsx web/src/i18n/dictionaries/ja.ts web/src/i18n/dictionaries/en.ts
git commit -m "各音声画面に前回の続きを再生を追加する"
```

## Chunk 3: Playback History Page

### Task 7: Build playback history page

**Files:**
- Create: `web/src/app/(main)/playback-history/page.tsx`
- Modify: `web/src/app/(main)/layout.tsx`
- Modify: `web/src/i18n/dictionaries/ja.ts`
- Modify: `web/src/i18n/dictionaries/en.ts`

- [ ] **Step 1: Add i18n keys for history page**

Add:
- page title
- filter tabs
- status labels
- progress labels
- empty state
- continue / open detail actions

- [ ] **Step 2: Build history page query and filters**

Implement:
- `all / summary / audio briefing`
- cards ordered by latest first
- status badge rendering

- [ ] **Step 3: Render progress details**

Summary cards:
- current item
- remaining item count
- progress ratio

Audio briefing cards:
- current position
- total duration
- progress ratio

- [ ] **Step 4: Add actions**

Buttons:
- `続きから再生`
- `詳細を開く`

- [ ] **Step 5: Add navigation entry if appropriate**

Expose page from existing nav pattern without overcrowding main navigation.

- [ ] **Step 6: Run web lint and build**

Run:
- `make web-lint`
- `make web-build`

Expected: PASS

- [ ] **Step 7: Commit**

```bash
git add web/src/app/(main)/playback-history/page.tsx web/src/app/(main)/layout.tsx web/src/i18n/dictionaries/ja.ts web/src/i18n/dictionaries/en.ts
git commit -m "再生履歴ページを追加する"
```

### Task 8: Add event logging

**Files:**
- Modify: `api/internal/service/playback_sessions.go`
- Modify: `web/src/components/shared-audio-player/provider.tsx`
- Modify: `api/internal/service/playback_sessions_test.go`

- [ ] **Step 1: Add failing tests for event creation**

Cover:
- started event on session start
- paused / resumed on player interactions
- completed / replaced / stopped on terminal transitions

- [ ] **Step 2: Implement event writes in service**

Event creation should remain internal to the service layer where possible.

- [ ] **Step 3: Wire provider lifecycle calls**

Ensure event spam is controlled:
- `progressed` should be throttled
- duplicate pause / resume should not emit repeatedly

- [ ] **Step 4: Run API and web verification**

Run:
- `docker compose exec -T api go test ./internal/service ./internal/handler`
- `make web-lint`
- `make web-build`

Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add api/internal/service/playback_sessions.go api/internal/service/playback_sessions_test.go web/src/components/shared-audio-player/provider.tsx
git commit -m "再生履歴event記録を追加する"
```

## Final Verification

### Task 9: Run full verification and manual checks

**Files:**
- Modify: none

- [ ] **Step 1: Run formatting and tests**

Run:
- `make fmt-go`
- `docker compose exec -T api go test ./...`
- `make web-lint`
- `make web-build`

Expected: PASS

- [ ] **Step 2: Manual summary resume check**

Verify:
- start summary playback
- move to middle of queue
- stop
- reload page
- resume from saved queue and position

- [ ] **Step 3: Manual audio briefing resume check**

Verify:
- start briefing playback
- seek to middle
- stop
- reload page
- resume from saved position

- [ ] **Step 4: Manual history page check**

Verify:
- history cards render
- mode filters work
- statuses become `途中 / 完了 / 中断`
- `続きから再生` resumes correctly

- [ ] **Step 5: Commit final integration adjustments if needed**

```bash
git add .
git commit -m "再生履歴と続きから再生を仕上げる"
```
