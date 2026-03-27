# 共通オーバーレイ音声プレイヤー Implementation Plan

> **For agentic workers:** REQUIRED: Use superpowers:subagent-driven-development (if subagents available) or superpowers:executing-plans to implement this plan. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 要約連続再生と音声ブリーフィング再生を共通プレイヤーへ統合し、Sifto 内のページ遷移後も再生を維持できるようにする

**Architecture:** `(main)` layout 配下に単一の shared audio provider と単一 `<audio>` 要素を配置し、summary queue と audio briefing を mode で切り替える。既存の page-local player state は provider へ移し、各ページは再生開始 request だけを送る形へ縮退させる。下部ミニプレイヤーと展開オーバーレイを共通 UI として実装し、summary queue の小さい再生 window も provider 側で保持する。

**Tech Stack:** Next.js App Router, React 19, TypeScript, TanStack Query, lucide-react, existing `api.ts`, existing i18n dictionaries, `make web-lint`, `make web-build`

---

## File Structure

### Create

- `web/src/components/shared-audio-player/provider.tsx`
  - 共通再生 state、単一 `<audio>`、summary/audio briefing の開始 API、queue 維持、prefetch、30 秒既読判定
- `web/src/components/shared-audio-player/mini-player.tsx`
  - 下部ミニプレイヤー UI
- `web/src/components/shared-audio-player/overlay.tsx`
  - 展開オーバーレイ UI
- `web/src/components/shared-audio-player/types.ts`
  - shared player の mode / state / payload 型定義

### Modify

- `web/src/app/(main)/layout.tsx`
  - provider と UI を常駐配置
- `web/src/components/providers.tsx`
  - provider の差し込み位置調整が必要なら対応
- `web/src/app/(main)/audio-player/page.tsx`
  - page-local 再生 state を撤去し、queue 開始画面寄りへ変更
- `web/src/app/(main)/audio-briefings/[id]/page.tsx`
  - page-local `<audio>` を共通プレイヤー起動に置換
- `web/src/app/(main)/items/page.tsx`
  - summary queue 起動導線を共通プレイヤー前提に調整
- `web/src/lib/api.ts`
  - 共通プレイヤーから使う summary synth / item detail / audio briefing detail 呼び出しの見直しが必要なら整理
- `web/src/i18n/dictionaries/ja.ts`
- `web/src/i18n/dictionaries/en.ts`

### Verification Targets

- `make web-lint`
- `make web-build`
- 手動確認:
  - `/items`
  - `/audio-player`
  - `/audio-briefings/[id]`
  - `/briefing`
  - `/sources`

## Chunk 1: Shared Player State Foundation

### Task 1: Define shared player types and state shape

**Files:**
- Create: `web/src/components/shared-audio-player/types.ts`
- Create: `web/src/components/shared-audio-player/provider.tsx`

- [ ] **Step 1: Define the shared state types**

Add explicit types for:

- `SharedAudioMode = "summary_queue" | "audio_briefing" | null`
- `SharedPlaybackState = "idle" | "preparing" | "playing" | "paused" | "error" | "finished"`
- summary queue payload
- audio briefing payload
- mini-player display metadata

- [ ] **Step 2: Create provider skeleton and public API**

Create:

- `SharedAudioPlayerProvider`
- `useSharedAudioPlayer()`

Expose functions for:

- `startSummaryQueuePlayback(queueKind, initialItems?)`
- `startAudioBriefingPlayback(payload)`
- `pausePlayback()`
- `resumePlayback()`
- `skipToNext()`
- `stopPlayback()`
- `expandPlayer()`
- `collapsePlayer()`

- [ ] **Step 3: Add single audio element ownership**

Inside provider, create a single hidden `<audio>` element and keep refs for:

- current prepared audio
- prefetched audio
- pending prefetch promise

Do not render any visible UI yet.

- [ ] **Step 4: Implement mode replacement behavior**

When either `startSummaryQueuePlayback` or `startAudioBriefingPlayback` runs:

- stop current playback
- release current object URLs
- clear previous mode-specific state
- replace with new mode state

- [ ] **Step 5: Run static verification**

Run: `make web-lint`
Expected: PASS with no new warnings/errors

- [ ] **Step 6: Commit**

```bash
git add web/src/components/shared-audio-player/types.ts web/src/components/shared-audio-player/provider.tsx
git commit -m "共通音声プレイヤーの基盤stateを追加する"
```

### Task 2: Move summary queue engine into provider

**Files:**
- Modify: `web/src/components/shared-audio-player/provider.tsx`
- Reference: `web/src/app/(main)/audio-player/page.tsx`
- Modify: `web/src/lib/api.ts` (only if helper shape cleanup is needed)

- [ ] **Step 1: Port summary queue constants and helpers**

Move the existing summary queue logic into provider:

- `PLAYBACK_QUEUE_BUFFER_SIZE = 24`
- `PLAYBACK_QUEUE_VISIBLE_COUNT = 12`
- `base64ToBlob`
- `synthesizeItem`

- [ ] **Step 2: Port queue window behavior**

Implement provider-owned queue state:

- keep at most 24 items
- visible list is first 12
- after one item finishes, drop the head
- clicking item `n` later should replace queue with `slice(n)`

- [ ] **Step 3: Port prefetch behavior**

Move current `pendingPrefetchRef`, `prefetchedAudioRef`, and prefetch fallback behavior into provider.

Preserve current rule:

- current item is generated now
- next item only is prefetched
- if prefetch fails, next playback falls back to normal synth

- [ ] **Step 4: Port cumulative 30-second read logic**

Move current summary queue read tracking into provider:

- count only while actual audio is playing
- pause / preparing / waiting time does not count
- at 30 seconds, call `api.markItemRead(itemID)`

- [ ] **Step 5: Validate summary queue compiles inside provider**

Run: `make web-build`
Expected: PASS

- [ ] **Step 6: Commit**

```bash
git add web/src/components/shared-audio-player/provider.tsx web/src/lib/api.ts
git commit -m "要約連続再生のqueue engineを共通プレイヤーへ移す"
```

## Chunk 2: Shared UI Shell

### Task 3: Add the persistent mini-player to main layout

**Files:**
- Create: `web/src/components/shared-audio-player/mini-player.tsx`
- Modify: `web/src/app/(main)/layout.tsx`
- Modify: `web/src/i18n/dictionaries/ja.ts`
- Modify: `web/src/i18n/dictionaries/en.ts`

- [ ] **Step 1: Build the mini-player component**

Render:

- mode label
- title
- source label for summary queue
- play / pause
- next
- stop
- queue count
- expand button

Match existing editorial look and keep it fixed at the bottom.

- [ ] **Step 2: Mount provider and mini-player in main layout**

Wrap `(main)` children with `SharedAudioPlayerProvider` and render `SharedAudioMiniPlayer` after `<main>`.

Keep existing bottom padding behavior in mind so content is not hidden behind the player.

- [ ] **Step 3: Add i18n labels**

Add both ja/en dictionary keys for:

- shared audio mode names
- shared player actions
- queue count / preparing / prefetching / paused states

- [ ] **Step 4: Verify mobile and desktop layout**

Run: `make web-build`
Expected: PASS

Then manually verify:

- mini-player is hidden when idle
- mini-player appears when active
- bottom padding remains usable on mobile

- [ ] **Step 5: Commit**

```bash
git add web/src/components/shared-audio-player/mini-player.tsx web/src/app/(main)/layout.tsx web/src/i18n/dictionaries/ja.ts web/src/i18n/dictionaries/en.ts
git commit -m "共通音声ミニプレイヤーをmain layoutに常駐させる"
```

### Task 4: Add the expanded overlay player

**Files:**
- Create: `web/src/components/shared-audio-player/overlay.tsx`
- Modify: `web/src/app/(main)/layout.tsx`
- Modify: `web/src/i18n/dictionaries/ja.ts`
- Modify: `web/src/i18n/dictionaries/en.ts`

- [ ] **Step 1: Build overlay shell**

Implement:

- desktop centered overlay
- mobile full-screen sheet
- close / collapse action

- [ ] **Step 2: Render summary queue detail view**

Display:

- translated title
- original title
- summary body
- source title
- original link
- visible playback queue

- [ ] **Step 3: Render audio briefing detail view**

Display:

- briefing title
- overview/summary
- detail page link
- basic playback meta

- [ ] **Step 4: Reuse provider state only**

Do not create any playback state inside overlay.
Overlay is a pure view over provider state.

- [ ] **Step 5: Verify**

Run: `make web-lint`
Expected: PASS with no new warnings/errors

- [ ] **Step 6: Commit**

```bash
git add web/src/components/shared-audio-player/overlay.tsx web/src/app/(main)/layout.tsx web/src/i18n/dictionaries/ja.ts web/src/i18n/dictionaries/en.ts
git commit -m "共通音声プレイヤーの展開オーバーレイを追加する"
```

## Chunk 3: Summary Queue Integration

### Task 5: Convert the summary audio page into a shared-player entry surface

**Files:**
- Modify: `web/src/app/(main)/audio-player/page.tsx`
- Modify: `web/src/components/shared-audio-player/provider.tsx`

- [ ] **Step 1: Remove page-local audio ownership**

Delete page-owned:

- `<audio>`
- playback refs
- queue refs
- read progress refs

Use provider state and actions instead.

- [ ] **Step 2: Keep page as queue launcher / viewer**

The page should:

- read `queue` from URL
- request queue start through provider
- show current queue mode / current item / guidance
- not own actual playback

- [ ] **Step 3: Prevent accidental restart loops**

Ensure entering `/audio-player?queue=x` while same mode is already active does not unnecessarily reset queue unless the queue kind changed.

- [ ] **Step 4: Verify natural transitions**

Manual check:

1. Open `/audio-player?queue=unread`
2. Let one item finish
3. Confirm next item starts
4. Confirm queue list shifts upward

- [ ] **Step 5: Run verification**

Run:

- `make web-lint`
- `make web-build`

Expected: both PASS

- [ ] **Step 6: Commit**

```bash
git add web/src/app/(main)/audio-player/page.tsx web/src/components/shared-audio-player/provider.tsx
git commit -m "要約再生ページを共通プレイヤーの起動面へ切り替える"
```

### Task 6: Preserve queue across app navigation

**Files:**
- Modify: `web/src/components/shared-audio-player/provider.tsx`
- Reference: `web/src/components/providers.tsx`

- [ ] **Step 1: Keep provider state under `(main)` layout lifetime**

Confirm provider is mounted once for all `(main)` routes and that route changes do not reset queue.

- [ ] **Step 2: Add queue refresh behavior without destroying current playback**

When item queries refresh:

- append newly available items only
- keep current queue head intact
- do not drop current item mid-playback

- [ ] **Step 3: Re-check 30-second read behavior during navigation**

Manual check:

1. Start summary playback
2. Navigate to another `(main)` page before 30 seconds
3. Keep listening
4. Confirm read marking still happens after cumulative 30 seconds

- [ ] **Step 4: Run verification**

Run: `make web-build`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add web/src/components/shared-audio-player/provider.tsx web/src/app/(main)/layout.tsx
git commit -m "要約再生キューをページ遷移後も維持する"
```

## Chunk 4: Audio Briefing Integration

### Task 7: Replace the audio briefing detail page local player

**Files:**
- Modify: `web/src/app/(main)/audio-briefings/[id]/page.tsx`
- Modify: `web/src/components/shared-audio-player/provider.tsx`
- Modify: `web/src/i18n/dictionaries/ja.ts`
- Modify: `web/src/i18n/dictionaries/en.ts`

- [ ] **Step 1: Add audio briefing start action to provider**

Accept:

- `job_id`
- `title`
- `summary`
- `audio_url`
- `detail_href`

and set mode to `audio_briefing`.

- [ ] **Step 2: Replace page-local `<audio>` with shared-player button**

If `detail.audio_url` exists:

- show start playback button
- optionally show "open in shared player" style secondary copy

If missing:

- keep pending state message

- [ ] **Step 3: Add detail link support in overlay**

Overlay audio briefing mode should expose a link back to `/audio-briefings/[id]`.

- [ ] **Step 4: Verify mode replacement**

Manual check:

1. Start summary queue playback
2. Open an audio briefing detail page
3. Start audio briefing playback
4. Confirm summary playback stops and briefing starts

- [ ] **Step 5: Run verification**

Run:

- `make web-lint`
- `make web-build`

Expected: both PASS

- [ ] **Step 6: Commit**

```bash
git add web/src/app/(main)/audio-briefings/[id]/page.tsx web/src/components/shared-audio-player/provider.tsx web/src/i18n/dictionaries/ja.ts web/src/i18n/dictionaries/en.ts
git commit -m "音声ブリーフィング再生を共通プレイヤーへ統合する"
```

## Chunk 5: Final Pass and UX Hardening

### Task 8: Make item/audio entry points consistent

**Files:**
- Modify: `web/src/app/(main)/items/page.tsx`
- Modify: `web/src/app/(main)/audio-player/page.tsx`
- Modify: `web/src/app/(main)/audio-briefings/[id]/page.tsx`

- [ ] **Step 1: Normalize launch flows**

Ensure the three main entry points feel consistent:

- items page audio launch
- audio-player page
- audio briefing detail page

- [ ] **Step 2: Check hover, disabled, and loading states**

Mini-player and overlay controls must have:

- hover state
- disabled handling
- preparing / prefetching visibility

- [ ] **Step 3: Confirm no stale duplicated player remains**

Search for any remaining page-local `<audio>` related to summary queue or audio briefing detail and remove/replace it if it duplicates shared playback.

Run:

```bash
rg -n "<audio|audioRef|currentAudioRef|prefetchedAudioRef" web/src/app web/src/components
```

Expected: only shared provider owns the persistent player for these two features.

- [ ] **Step 4: Run final verification**

Run:

- `make web-lint`
- `make web-build`

Manual verification:

1. Start summary playback on `/items`
2. Navigate to `/briefing`, `/sources`, `/ask`
3. Confirm mini-player remains and playback continues
4. Expand overlay and inspect queue
5. Switch to audio briefing playback and confirm replacement
6. Check mobile width in responsive mode

- [ ] **Step 5: Commit**

```bash
git add web/src/app/(main)/items/page.tsx web/src/app/(main)/audio-player/page.tsx web/src/app/(main)/audio-briefings/[id]/page.tsx web/src/components/shared-audio-player
git commit -m "共通オーバーレイ音声プレイヤーの導線と挙動を仕上げる"
```

