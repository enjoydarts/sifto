# AIナビ朝昼夜brief Implementation Plan

> **For agentic workers:** REQUIRED: Use superpowers:subagent-driven-development (if subagents available) or superpowers:executing-plans to implement this plan. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 朝昼夜の AI ナビ brief を自動生成・保存・通知し、専用一覧 / 詳細と要約読み上げキュー追加導線まで実装する。

**Architecture:** 既存の AI ナビ LLM 実行基盤を流用しつつ、`ai_navigator_briefs` と `ai_navigator_brief_items` を新設して独立した生成物として扱う。定時ジョブで slot ごとの候補記事を抽出し、brief を保存して push 通知を送り、web では専用一覧 / 詳細ページと summary audio 連携を提供する。

**Tech Stack:** Go API, PostgreSQL migration, existing worker/LLM pipeline, Next.js App Router, React Query, OneSignal notification flow

---

## File Structure

- Create: `db/migrations/000103_create_ai_navigator_briefs.up.sql`
- Create: `db/migrations/000103_create_ai_navigator_briefs.down.sql`
- Create: `api/internal/model/ai_navigator_brief.go`
- Create: `api/internal/repository/ai_navigator_briefs.go`
- Create: `api/internal/repository/ai_navigator_briefs_test.go`
- Create: `api/internal/service/ai_navigator_briefs.go`
- Create: `api/internal/service/ai_navigator_briefs_test.go`
- Create: `api/internal/handler/ai_navigator_briefs.go`
- Modify: `api/internal/repository/items.go`
- Modify: `api/internal/service/worker.go`
- Modify: `api/internal/service/push.go`
- Modify: `api/internal/service/push_test.go`
- Modify: `api/internal/inngest/...` or existing scheduled job entrypoints
- Modify: `api/cmd/server/main.go`
- Modify: `web/src/lib/api.ts`
- Create: `web/src/app/(main)/ai-navigator-briefs/page.tsx`
- Create: `web/src/app/(main)/ai-navigator-briefs/[id]/page.tsx`
- Modify: `web/src/app/(main)/settings/page.tsx`
- Modify: `web/src/components/nav.tsx`
- Modify: `web/src/i18n/dictionaries/ja.ts`
- Modify: `web/src/i18n/dictionaries/en.ts`

---

## Chunk 1: DB and Domain Model

### Task 1: Add migration for AIナビbrief tables

**Files:**
- Create: `db/migrations/000103_create_ai_navigator_briefs.up.sql`
- Create: `db/migrations/000103_create_ai_navigator_briefs.down.sql`

- [ ] **Step 1: Write the migration**

Create tables:
- `ai_navigator_briefs`
- `ai_navigator_brief_items`

Include:
- `slot`, `status`, `title`, `intro`, `summary`, `persona`, `model`
- `source_window_start`, `source_window_end`
- `generated_at`, `notification_sent_at`, `error_message`
- item snapshots and `comment`

- [ ] **Step 2: Apply migration locally**

Run: `make migrate-up`
Expected: version advances to `103`

- [ ] **Step 3: Verify migration version**

Run: `make migrate-version`
Expected: `103`

- [ ] **Step 4: Commit**

```bash
git add db/migrations/000103_create_ai_navigator_briefs.up.sql db/migrations/000103_create_ai_navigator_briefs.down.sql
git commit -m "AIナビbrief保存テーブルを追加する"
```

### Task 2: Add domain model types

**Files:**
- Create: `api/internal/model/ai_navigator_brief.go`

- [ ] **Step 1: Write the model definitions**

Define:
- brief slot constants `morning`, `noon`, `evening`
- brief status constants `queued`, `generated`, `failed`, `notified`
- `AINavigatorBrief`
- `AINavigatorBriefItem`
- list/detail response payloads as needed

- [ ] **Step 2: Run focused compile tests**

Run: `docker compose exec -T api go test ./internal/model ./internal/service ./internal/handler`
Expected: PASS

- [ ] **Step 3: Commit**

```bash
git add api/internal/model/ai_navigator_brief.go
git commit -m "AIナビbriefのモデル型を追加する"
```

---

## Chunk 2: Repository and Candidate Query

### Task 3: Add brief repository

**Files:**
- Create: `api/internal/repository/ai_navigator_briefs.go`
- Create: `api/internal/repository/ai_navigator_briefs_test.go`

- [ ] **Step 1: Write failing repository tests**

Cover:
- brief insert
- item insert
- list by user
- get detail by id
- slot ordering by `generated_at desc`

- [ ] **Step 2: Run repository tests to verify failure**

Run: `docker compose exec -T api go test ./internal/repository -run TestAINavigatorBrief`
Expected: FAIL with missing repo implementation

- [ ] **Step 3: Implement minimal repository**

Add methods:
- `CreateBrief`
- `AddBriefItems`
- `MarkBriefFailed`
- `MarkBriefNotified`
- `ListBriefsByUser`
- `GetBriefDetail`

- [ ] **Step 4: Re-run repository tests**

Run: `docker compose exec -T api go test ./internal/repository -run TestAINavigatorBrief`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add api/internal/repository/ai_navigator_briefs.go api/internal/repository/ai_navigator_briefs_test.go
git commit -m "AIナビbrief repositoryを追加する"
```

### Task 4: Add slot-based candidate query

**Files:**
- Modify: `api/internal/repository/items.go`
- Modify: `api/internal/repository/...tests...`

- [ ] **Step 1: Write failing tests for slot candidate windows**

Cover:
- summarized + unread + not later + summary present
- fetched/published window handling
- limit 24

- [ ] **Step 2: Run focused test**

Run: `docker compose exec -T api go test ./internal/repository -run TestAINavigatorBriefCandidates`
Expected: FAIL

- [ ] **Step 3: Implement candidate query**

Add repo method such as:
- `AINavigatorBriefCandidatesInWindow(ctx, userID, start, end, limit)`

Use existing AI navigator candidate style where possible.

- [ ] **Step 4: Re-run repository tests**

Run: `docker compose exec -T api go test ./internal/repository -run TestAINavigatorBriefCandidates`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add api/internal/repository/items.go api/internal/repository/*test.go
git commit -m "AIナビbrief候補記事抽出を追加する"
```

---

## Chunk 3: Brief Generation Service

### Task 5: Extend worker client / structured output contract

**Files:**
- Modify: `api/internal/service/worker.go`
- Worker-side files if needed for new task

- [ ] **Step 1: Write failing service contract test**

Cover:
- request includes persona, model, 24 candidates
- response expects `title`, `intro`, `summary`, `items[10]`

- [ ] **Step 2: Run focused service test**

Run: `docker compose exec -T api go test ./internal/service -run TestComposeAINavigatorBrief`
Expected: FAIL

- [ ] **Step 3: Add worker client method**

Add method such as:
- `ComposeAINavigatorBrief(...)`

Structured output must guarantee:
- exactly 10 items
- each item has `item_id` and `comment`

- [ ] **Step 4: Re-run service test**

Run: `docker compose exec -T api go test ./internal/service -run TestComposeAINavigatorBrief`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add api/internal/service/worker.go worker/app/services/* 
git commit -m "AIナビbrief生成のworker連携を追加する"
```

### Task 6: Implement brief service

**Files:**
- Create: `api/internal/service/ai_navigator_briefs.go`
- Create: `api/internal/service/ai_navigator_briefs_test.go`

- [ ] **Step 1: Write failing service tests**

Cover:
- create brief for one slot
- persist brief + 10 item snapshots
- fail when candidates < 10
- save `failed` status on LLM failure

- [ ] **Step 2: Run service tests**

Run: `docker compose exec -T api go test ./internal/service -run TestAINavigatorBriefService`
Expected: FAIL

- [ ] **Step 3: Implement service**

Add:
- slot window resolution
- candidate loading
- compose request
- brief persistence
- item snapshot persistence

- [ ] **Step 4: Re-run service tests**

Run: `docker compose exec -T api go test ./internal/service -run TestAINavigatorBriefService`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add api/internal/service/ai_navigator_briefs.go api/internal/service/ai_navigator_briefs_test.go
git commit -m "AIナビbrief生成サービスを追加する"
```

---

## Chunk 4: API and Notification

### Task 7: Add API handler for list/detail and queue append

**Files:**
- Create: `api/internal/handler/ai_navigator_briefs.go`
- Modify: `api/cmd/server/main.go`

- [ ] **Step 1: Write failing handler tests**

Cover:
- `GET /ai-navigator-briefs`
- `GET /ai-navigator-briefs/{id}`
- `POST /ai-navigator-briefs/{id}/summary-audio-queue`

- [ ] **Step 2: Run handler tests**

Run: `docker compose exec -T api go test ./internal/handler -run TestAINavigatorBriefHandler`
Expected: FAIL

- [ ] **Step 3: Implement handler and routes**

Add list/detail endpoints and queue append endpoint using saved item ids in rank order.

- [ ] **Step 4: Re-run handler tests**

Run: `docker compose exec -T api go test ./internal/handler -run TestAINavigatorBriefHandler`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add api/internal/handler/ai_navigator_briefs.go api/cmd/server/main.go
git commit -m "AIナビbrief APIを追加する"
```

### Task 8: Add push notification wiring

**Files:**
- Modify: `api/internal/service/push.go`
- Modify: `api/internal/service/push_test.go`

- [ ] **Step 1: Write failing push tests**

Cover:
- new `kind`
- internal URL to `/ai-navigator-briefs/{id}`
- title/body selection

- [ ] **Step 2: Run push tests**

Run: `docker compose exec -T api go test ./internal/service -run TestAINavigatorBriefPush`
Expected: FAIL

- [ ] **Step 3: Implement notification builder**

Add:
- `kind = ai_navigator_brief`
- click URL to internal page
- body shortening logic

- [ ] **Step 4: Re-run push tests**

Run: `docker compose exec -T api go test ./internal/service -run TestAINavigatorBriefPush`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add api/internal/service/push.go api/internal/service/push_test.go
git commit -m "AIナビbrief通知を追加する"
```

---

## Chunk 5: Scheduler and Feature Toggle

### Task 9: Add settings flag

**Files:**
- Modify: settings model/repository/handler as needed
- Modify: `web/src/app/(main)/settings/page.tsx`
- Modify: `web/src/i18n/dictionaries/ja.ts`
- Modify: `web/src/i18n/dictionaries/en.ts`

- [ ] **Step 1: Write failing API/settings tests**

Cover:
- load/save `ai_navigator_brief_enabled`

- [ ] **Step 2: Run focused tests**

Run: `docker compose exec -T api go test ./internal/handler ./internal/service -run TestAINavigatorBriefSetting`
Expected: FAIL

- [ ] **Step 3: Implement settings field**

Add one feature-level ON/OFF only. Reuse existing AI navigator model/persona settings.

- [ ] **Step 4: Add settings UI**

Add a simple toggle in settings page with i18n labels.

- [ ] **Step 5: Verify tests and web build**

Run:
- `docker compose exec -T api go test ./internal/handler ./internal/service -run TestAINavigatorBriefSetting`
- `make web-lint`
- `make web-build`

- [ ] **Step 6: Commit**

```bash
git add api/internal/... web/src/app/(main)/settings/page.tsx web/src/i18n/dictionaries/ja.ts web/src/i18n/dictionaries/en.ts
git commit -m "AIナビbriefの設定トグルを追加する"
```

### Task 10: Add scheduled generation job

**Files:**
- Modify: scheduler / inngest entrypoints
- Modify: relevant tests

- [ ] **Step 1: Write failing job tests**

Cover:
- morning / noon / evening slot dispatch
- feature OFF users skipped
- generated brief marked notified on success

- [ ] **Step 2: Run focused job tests**

Run: `docker compose exec -T api go test ./internal/inngest ./internal/service -run TestAINavigatorBriefSchedule`
Expected: FAIL

- [ ] **Step 3: Implement scheduler**

Add:
- slot resolution
- per-user generation
- notification send
- status updates

- [ ] **Step 4: Re-run job tests**

Run: `docker compose exec -T api go test ./internal/inngest ./internal/service -run TestAINavigatorBriefSchedule`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add api/internal/inngest/... api/internal/service/...
git commit -m "AIナビbrief定時生成ジョブを追加する"
```

---

## Chunk 6: Web Pages

### Task 11: Add API client support

**Files:**
- Modify: `web/src/lib/api.ts`

- [ ] **Step 1: Add client types**

Define:
- `AINavigatorBrief`
- `AINavigatorBriefItem`
- list/detail responses

- [ ] **Step 2: Add client methods**

Add:
- `getAINavigatorBriefs`
- `getAINavigatorBrief`
- `appendAINavigatorBriefToSummaryQueue`

- [ ] **Step 3: Verify web lint/build**

Run:
- `make web-lint`
- `make web-build`

- [ ] **Step 4: Commit**

```bash
git add web/src/lib/api.ts
git commit -m "AIナビbrief API clientを追加する"
```

### Task 12: Add AIナビbrief list page

**Files:**
- Create: `web/src/app/(main)/ai-navigator-briefs/page.tsx`
- Modify: `web/src/components/nav.tsx`
- Modify: dictionaries

- [ ] **Step 1: Build list page**

Render:
- slot tag
- generated_at
- title
- one-line summary

- [ ] **Step 2: Add nav entry**

Put it in the content/navigation area consistent with existing nav structure.

- [ ] **Step 3: Add i18n labels**

Both `ja.ts` and `en.ts`.

- [ ] **Step 4: Verify**

Run:
- `make web-lint`
- `make web-build`

- [ ] **Step 5: Commit**

```bash
git add web/src/app/(main)/ai-navigator-briefs/page.tsx web/src/components/nav.tsx web/src/i18n/dictionaries/ja.ts web/src/i18n/dictionaries/en.ts
git commit -m "AIナビbrief一覧ページを追加する"
```

### Task 13: Add AIナビbrief detail page

**Files:**
- Create: `web/src/app/(main)/ai-navigator-briefs/[id]/page.tsx`
- Modify: dictionaries if needed

- [ ] **Step 1: Build detail page**

Render:
- title
- slot
- generated_at
- persona
- intro
- summary
- 10 commented items

- [ ] **Step 2: Add queue append action**

Button:
- `10本を要約読み上げキューに追加`

Success behavior:
- toast
- optional link to `/audio-player`

- [ ] **Step 3: Verify**

Run:
- `make web-lint`
- `make web-build`

- [ ] **Step 4: Commit**

```bash
git add web/src/app/(main)/ai-navigator-briefs/[id]/page.tsx web/src/i18n/dictionaries/ja.ts web/src/i18n/dictionaries/en.ts
git commit -m "AIナビbrief詳細ページを追加する"
```

---

## Chunk 7: Final Verification

### Task 14: Run full verification

**Files:**
- No code changes expected

- [ ] **Step 1: Run backend verification**

Run:
- `make fmt-go`
- `docker compose exec -T api go test ./...`

Expected: PASS

- [ ] **Step 2: Run web verification**

Run:
- `make web-lint`
- `make web-build`

Expected: PASS

- [ ] **Step 3: Run migration verification**

Run:
- `make migrate-up`
- `make migrate-version`

Expected: `103`

- [ ] **Step 4: Manual verification checklist**

Check:
- setting ON/OFF works
- list page renders
- detail page renders
- queue append preserves order
- notification click opens detail page

- [ ] **Step 5: Final commit if needed**

```bash
git status
git add ...
git commit -m "AIナビ朝昼夜briefを完成させる"
```

