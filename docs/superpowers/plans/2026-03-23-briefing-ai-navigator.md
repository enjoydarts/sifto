# Briefing AI Navigator Implementation Plan

> **For agentic workers:** REQUIRED: Use superpowers:subagent-driven-development (if subagents available) or superpowers:executing-plans to implement this plan. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** ブリーフィング画面に、persona 選択可能な AI ナビゲーターの右下オーバーレイを追加し、未読の直近記事から LLM が 3 本前後を選んでコメント付きで紹介できるようにする。

**Architecture:** AI ナビゲーターは `BuildBriefingToday` の生成フローに組み込み、briefing snapshot / cache に同梱する。候補記事収集は repository、選定とコメント生成は worker 経由の LLM、表示は briefing レスポンスの `navigator` payload を描画する共通 UI コンポーネントで行う。モデル設定と usage は `briefing_navigator` として summary/facts と分離する。

**Tech Stack:** Go, PostgreSQL migrations, Inngest-compatible server flow, Next.js App Router, React Query, existing worker LLM endpoints, i18n dictionaries

---

## File Map

### Backend / model / repository

- Modify: `api/internal/model/model.go`
  - `BriefingTodayResponse` に `navigator` payload を追加
  - `UserSettings` に navigator 用設定を追加
- Modify: `api/internal/repository/user_settings.go`
  - navigator の設定読み書き対応
- Modify: `api/internal/repository/items.go`
  - AI ナビゲーター候補記事取得 query を追加
- Modify: `api/internal/repository/briefing_snapshots.go`
  - payload 互換確認のみ。構造変更への影響をテストで固定
- Create: `db/migrations/000081_add_briefing_navigator_settings.up.sql`
- Create: `db/migrations/000081_add_briefing_navigator_settings.down.sql`

### Service / handler / worker integration

- Modify: `api/internal/service/briefing.go`
  - navigator 候補収集と payload 注入
- Create: `api/internal/service/briefing_navigator.go`
  - persona 定義、候補正規化、prompt 入出力 schema、fallback 制御
- Create: `api/internal/service/briefing_navigator_test.go`
  - persona / fallback / candidate shaping / parse fallback テスト
- Modify: `api/internal/handler/briefing.go`
  - briefing レスポンスの navigator 同梱互換確認
- Modify: `api/internal/handler/settings.go`
  - navigator 設定の保存・取得 API 対応
- Modify: `api/internal/inngest/functions.go`
  - usage purpose `briefing_navigator` 追加で既存 logging が流れることを確認

### Worker / prompt

- Modify: `worker/app/routers/summarize.py` or existing relevant router used for structured generation
  - 新しい navigator 用 endpoint を追加するか、既存共通 LLM endpoint を拡張
- Create: `worker/app/services/briefing_navigator_service.py`
  - persona ごとの prompt profile と structured output 呼び出し
- Create: `worker/app/services/test_briefing_navigator_service.py`
  - persona prompt / JSON parse / safety constraints のテスト

### Frontend

- Modify: `web/src/lib/api.ts`
  - `BriefingTodayResponse` と settings payload に navigator 型を追加
- Modify: `web/src/app/(main)/page.tsx`
  - navigator overlay の表示・close 状態管理
- Create: `web/src/components/briefing/ai-navigator-overlay.tsx`
  - 共通オーバーレイ本体
- Create: `web/src/components/briefing/ai-navigator-persona.ts`
  - persona ごとの UI token 定義
- Modify: `web/src/app/(main)/settings/page.tsx`
  - navigator enable / persona / model / fallback UI
- Modify: `web/src/i18n/dictionaries/ja.ts`
- Modify: `web/src/i18n/dictionaries/en.ts`

### Verification

- Test: `docker compose exec -T api go test ./internal/service ./internal/handler ./internal/repository ./internal/inngest`
- Test: `make check-worker`
- Test: `make web-build`
- Test: `make migrate-up`
- Test: `make migrate-version`

---

## Chunk 1: Settings And Data Contract

### Task 1: Add navigator settings columns

**Files:**
- Create: `db/migrations/000081_add_briefing_navigator_settings.up.sql`
- Create: `db/migrations/000081_add_briefing_navigator_settings.down.sql`
- Test: migration apply via `make migrate-up`

- [ ] **Step 1: Write the migration**

Add nullable/defaulted columns on `user_settings`:

```sql
ALTER TABLE user_settings
  ADD COLUMN IF NOT EXISTS navigator_enabled BOOLEAN NOT NULL DEFAULT FALSE,
  ADD COLUMN IF NOT EXISTS navigator_persona TEXT NOT NULL DEFAULT 'editor',
  ADD COLUMN IF NOT EXISTS navigator_model TEXT,
  ADD COLUMN IF NOT EXISTS navigator_fallback_model TEXT;
```

- [ ] **Step 2: Apply migration locally**

Run: `make migrate-up`  
Expected: migration `81/u` applies successfully

- [ ] **Step 3: Verify migration version**

Run: `make migrate-version`  
Expected: `81`

- [ ] **Step 4: Commit**

```bash
git add db/migrations/000081_add_briefing_navigator_settings.*
git commit -m "AIナビゲーター設定カラムを追加"
```

### Task 2: Extend user settings model and repository

**Files:**
- Modify: `api/internal/model/model.go`
- Modify: `api/internal/repository/user_settings.go`
- Modify: `api/internal/handler/settings.go`
- Test: existing settings tests plus targeted API test if present

- [ ] **Step 1: Add fields to `UserSettings`**

Add:

```go
NavigatorEnabled       bool    `json:"navigator_enabled"`
NavigatorPersona       string  `json:"navigator_persona"`
NavigatorModel         *string `json:"navigator_model,omitempty"`
NavigatorFallbackModel *string `json:"navigator_fallback_model,omitempty"`
```

- [ ] **Step 2: Update repository scans and upserts**

Include the new columns in:

- `GetByUserID`
- `EnsureDefaults` compatibility
- LLM model config upsert path or a new dedicated navigator config upsert method

- [ ] **Step 3: Expose settings in handler payload**

Return/save navigator settings from settings API with the rest of LLM model config.

- [ ] **Step 4: Run focused API tests**

Run: `docker compose exec -T api go test ./internal/handler ./internal/repository`  
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add api/internal/model/model.go api/internal/repository/user_settings.go api/internal/handler/settings.go
git commit -m "AIナビゲーター設定をAPIに追加"
```

### Task 3: Extend briefing response contract

**Files:**
- Modify: `api/internal/model/model.go`
- Modify: `web/src/lib/api.ts`

- [ ] **Step 1: Add navigator payload structs in Go**

Add:

```go
type BriefingNavigatorPick struct {
    ItemID string   `json:"item_id"`
    Rank   int      `json:"rank"`
    Comment string  `json:"comment"`
    ReasonTags []string `json:"reason_tags,omitempty"`
}

type BriefingNavigator struct {
    Enabled        bool                    `json:"enabled"`
    Persona        string                  `json:"persona"`
    CharacterName  string                  `json:"character_name"`
    CharacterTitle string                  `json:"character_title"`
    AvatarStyle    string                  `json:"avatar_style"`
    SpeechStyle    string                  `json:"speech_style"`
    Intro          string                  `json:"intro"`
    GeneratedAt    *time.Time              `json:"generated_at,omitempty"`
    Picks          []BriefingNavigatorPick `json:"picks"`
}
```

Then add:

```go
Navigator *BriefingNavigator `json:"navigator,omitempty"`
```

to `BriefingTodayResponse`.

- [ ] **Step 2: Mirror the contract in web API types**

Update `web/src/lib/api.ts` with matching `BriefingNavigator` and `BriefingNavigatorPick`.

- [ ] **Step 3: Run build-free type checks through app build later**

No separate command here; this will be validated in Chunk 3 by `make web-build`.

- [ ] **Step 4: Commit**

```bash
git add api/internal/model/model.go web/src/lib/api.ts
git commit -m "AIナビゲーターのbriefing契約を追加"
```

---

## Chunk 2: Candidate Selection And LLM Generation

### Task 4: Add candidate article query

**Files:**
- Modify: `api/internal/repository/items.go`
- Test: new repository/service test file if simpler than DB-level repo test

- [ ] **Step 1: Add a focused repository method**

Create a query like:

```go
func (r *ItemRepo) BriefingNavigatorCandidates24h(ctx context.Context, userID string, limit int) ([]model.Item, error)
```

Query requirements:

- unread only
- exclude deleted
- exclude later
- prefer recent items
- keep limit in `10-15`

- [ ] **Step 2: Write a failing test around candidate filtering or selection helper**

If DB integration is too heavy, write a service-level test that validates post-query shaping and document the SQL assumptions.

- [ ] **Step 3: Implement the query**

Use existing briefing and reading plan queries as references instead of inventing a new article shape.

- [ ] **Step 4: Run tests**

Run: `docker compose exec -T api go test ./internal/repository ./internal/service`  
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add api/internal/repository/items.go api/internal/service/*briefing*test*.go
git commit -m "AIナビゲーター候補記事取得を追加"
```

### Task 5: Build navigator service and persona profiles

**Files:**
- Create: `api/internal/service/briefing_navigator.go`
- Create: `api/internal/service/briefing_navigator_test.go`

- [ ] **Step 1: Write the failing tests first**

Cover:

- persona token lookup
- `snark` safety clamp
- candidate payload truncation
- empty candidate fallback

Example:

```go
func TestNavigatorPersonaProfileSnarkStaysLightweight(t *testing.T) {
    profile := briefingNavigatorPersonaProfile("snark")
    if profile.HumorLevel != "light" {
        t.Fatalf("HumorLevel = %q, want light", profile.HumorLevel)
    }
}
```

- [ ] **Step 2: Implement persona config**

Keep all persona definitions in one place:

- display name
- title
- avatar style
- speech style
- intro/comment length target
- prompt profile knobs

- [ ] **Step 3: Implement candidate normalization**

Map `model.Item` into a smaller LLM input payload:

- title
- source
- published_at
- summary
- item_id

Clamp long text aggressively.

- [ ] **Step 4: Implement parse-safe navigator result shaping**

Add helpers that accept structured LLM output and drop invalid picks cleanly.

- [ ] **Step 5: Run service tests**

Run: `docker compose exec -T api go test ./internal/service`  
Expected: PASS

- [ ] **Step 6: Commit**

```bash
git add api/internal/service/briefing_navigator.go api/internal/service/briefing_navigator_test.go
git commit -m "AIナビゲーターpersonaと整形ロジックを追加"
```

### Task 6: Add worker-side generation

**Files:**
- Create: `worker/app/services/briefing_navigator_service.py`
- Create: `worker/app/services/test_briefing_navigator_service.py`
- Modify: relevant worker router file used for structured LLM tasks

- [ ] **Step 1: Write failing worker tests**

Cover:

- persona prompt profile selection
- JSON schema parse success
- invalid output fallback to empty navigator

- [ ] **Step 2: Add a structured output schema**

Expected output shape:

```json
{
  "intro": "...",
  "picks": [
    { "item_id": "id", "rank": 1, "comment": "...", "reason_tags": ["fresh"] }
  ]
}
```

- [ ] **Step 3: Implement prompt assembly**

Prompt must include:

- unread-only context
- choose around 3 picks
- no fabricated facts
- persona tone rules
- special snark safety rules

- [ ] **Step 4: Expose worker endpoint or extend existing endpoint**

Prefer the smallest addition that matches existing worker patterns.

- [ ] **Step 5: Run worker checks**

Run: `make check-worker`  
Expected: PASS

- [ ] **Step 6: Commit**

```bash
git add worker/app/services/briefing_navigator_service.py worker/app/services/test_briefing_navigator_service.py worker/app/routers/*.py
git commit -m "AIナビゲーター生成をworkerに追加"
```

### Task 7: Integrate navigator generation into briefing build

**Files:**
- Modify: `api/internal/service/briefing.go`
- Modify: `api/internal/handler/briefing.go`
- Modify: `api/internal/inngest/functions.go`

- [ ] **Step 1: Write a failing service test for briefing payload inclusion**

Assert that when candidates and navigator generation succeed, `BriefingTodayResponse.Navigator` is populated.

- [ ] **Step 2: Extend `BuildBriefingToday` dependencies**

Refactor carefully so the function can access:

- item candidates
- user navigator settings
- worker client or abstraction for generation

If this makes the signature too wide, introduce a small builder/deps struct rather than stuffing more params into one function.

- [ ] **Step 3: Add graceful failure behavior**

If navigator generation fails:

- keep briefing response valid
- set `navigator=nil`
- do not fail the whole page

- [ ] **Step 4: Record LLM usage under `briefing_navigator`**

Use the same accounting path used by other LLM features so model/cost are separated in usage views.

- [ ] **Step 5: Run backend tests**

Run: `docker compose exec -T api go test ./internal/service ./internal/handler ./internal/inngest`  
Expected: PASS

- [ ] **Step 6: Commit**

```bash
git add api/internal/service/briefing.go api/internal/handler/briefing.go api/internal/inngest/functions.go
git commit -m "AIナビゲーターをbriefing生成に統合"
```

---

## Chunk 3: Settings UI And Briefing Overlay

### Task 8: Add settings controls

**Files:**
- Modify: `web/src/app/(main)/settings/page.tsx`
- Modify: `web/src/i18n/dictionaries/ja.ts`
- Modify: `web/src/i18n/dictionaries/en.ts`

- [ ] **Step 1: Add navigator settings state bindings**

Bind:

- enabled
- persona
- model
- fallback model

to existing settings fetch/save flows.

- [ ] **Step 2: Add persona option list**

Show 5 persona choices with short descriptions:

- editor
- hype
- analyst
- concierge
- snark

- [ ] **Step 3: Add i18n labels**

Add all settings labels and persona descriptions in both dictionaries.

- [ ] **Step 4: Run web build**

Run: `make web-build`  
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add web/src/app/(main)/settings/page.tsx web/src/i18n/dictionaries/ja.ts web/src/i18n/dictionaries/en.ts
git commit -m "AIナビゲーター設定UIを追加"
```

### Task 9: Build the overlay component

**Files:**
- Create: `web/src/components/briefing/ai-navigator-overlay.tsx`
- Create: `web/src/components/briefing/ai-navigator-persona.ts`
- Modify: `web/src/app/(main)/page.tsx`

- [ ] **Step 1: Write the component against the new API shape**

Props should be close to:

```ts
type Props = {
  navigator: BriefingNavigator;
  onClose: () => void;
  onOpenItem: (itemId: string) => void;
};
```

- [ ] **Step 2: Implement persona UI tokens**

Keep per-persona tokens in one file:

- accent colors
- avatar emoji/shape/illustration token
- bubble class names
- label copy

- [ ] **Step 3: Wire auto-open behavior in briefing page**

Rules:

- auto-open on each page visit if navigator exists
- remain visible until closed
- after close, stay hidden for that visit only

- [ ] **Step 4: Wire CTA actions**

Each recommendation should let the user:

- open the article
- optionally open inline reader directly if that matches current page behavior best

- [ ] **Step 5: Run web build**

Run: `make web-build`  
Expected: PASS

- [ ] **Step 6: Commit**

```bash
git add web/src/components/briefing/ai-navigator-overlay.tsx web/src/components/briefing/ai-navigator-persona.ts web/src/app/(main)/page.tsx
git commit -m "AIナビゲーターオーバーレイをbriefingに追加"
```

### Task 10: End-to-end verification

**Files:**
- Verify only

- [ ] **Step 1: Run API test suite for touched areas**

Run: `docker compose exec -T api go test ./internal/service ./internal/handler ./internal/repository ./internal/inngest`

- [ ] **Step 2: Run worker verification**

Run: `make check-worker`

- [ ] **Step 3: Run frontend production build**

Run: `make web-build`

- [ ] **Step 4: Ensure migrations are applied**

Run: `make migrate-up`

- [ ] **Step 5: Confirm migration version**

Run: `make migrate-version`
Expected: `81`

- [ ] **Step 6: Smoke-check behavior manually**

Check:

- briefing opens with overlay
- close hides for this visit
- next full revisit shows it again
- persona choice changes UI and tone
- missing/failed navigator generation does not break briefing page

- [ ] **Step 7: Final commit**

```bash
git add .
git commit -m "ブリーフィングAIナビゲーターを実装"
```
