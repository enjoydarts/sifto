# Poe Usage Persistence Implementation Plan

> **For agentic workers:** REQUIRED: Use superpowers:subagent-driven-development (if subagents available) or superpowers:executing-plans to implement this plan. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Poe Usage を Sifto 側に永続化し、`今日 / 昨日 / 7日 / 14日 / 30日 / 今月 / 先月` で絞り込める Usage 画面にする。

**Architecture:** Poe Usage API の `points_history` を `poe_usage_entries` に upsert し、画面の履歴・集計は DB ベースへ切り替える。`current_balance` だけは live fetch を維持し、`残高は live / 履歴と集計は DB` に責務分離する。

**Tech Stack:** Go, PostgreSQL, chi, pgx, Next.js App Router, React Query, docker compose, make

---

## File Map

- Create: `db/migrations/0000xx_add_poe_usage_entries.up.sql`
- Create: `db/migrations/0000xx_add_poe_usage_entries.down.sql`
- Create: `api/internal/repository/poe_usage.go`
- Create: `api/internal/repository/poe_usage_test.go`
- Modify: `api/internal/service/poe_usage.go`
- Modify: `api/internal/service/poe_usage_test.go`
- Modify: `api/internal/handler/poe_models.go`
- Modify: `api/cmd/server/main.go`
- Modify: `web/src/lib/api.ts`
- Modify: `web/src/app/(main)/poe-models/page.tsx`
- Modify: `web/src/i18n/dictionaries/ja.ts`
- Modify: `web/src/i18n/dictionaries/en.ts`

## Chunk 1: DB Persistence

### Task 1: Poe Usage テーブル migration を追加する

**Files:**
- Create: `db/migrations/0000xx_add_poe_usage_entries.up.sql`
- Create: `db/migrations/0000xx_add_poe_usage_entries.down.sql`

- [ ] **Step 1: Write the failing migration expectation**

確認内容:

- `poe_usage_sync_runs`
- `poe_usage_entries`
- unique `(user_id, query_id)`
- `created_at` 系 index

- [ ] **Step 2: Run migration status to verify table is absent**

Run: `make migrate-version`
Expected: current version is older than the new migration

- [ ] **Step 3: Write minimal migration**

```sql
CREATE TABLE poe_usage_sync_runs (...);
CREATE TABLE poe_usage_entries (...);
CREATE UNIQUE INDEX ... ON poe_usage_entries (user_id, query_id);
```

- [ ] **Step 4: Apply migration and verify**

Run: `make migrate-up`
Expected: migration applies successfully

Run: `make migrate-version`
Expected: version advances to the new migration

- [ ] **Step 5: Commit**

```bash
git add db/migrations/0000xx_add_poe_usage_entries.*
git commit -m "Poe Usage永続化テーブルを追加"
```

### Task 2: repository の failing test を書く

**Files:**
- Create: `api/internal/repository/poe_usage.go`
- Create: `api/internal/repository/poe_usage_test.go`

- [ ] **Step 1: Write the failing test**

```go
func TestPoeUsageRepo_UpsertEntriesAndRangeSummary(t *testing.T) {
    // insert same query_id twice
    // expect one row after upsert
    // expect summary for selected range
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `docker compose exec -T api go test ./internal/repository -run TestPoeUsageRepo -v`
Expected: FAIL with missing repo or method

- [ ] **Step 3: Write minimal implementation**

実装対象:

- `StartSyncRun`
- `FinishSyncRun`
- `UpsertEntries`
- `GetLatestSyncRun`
- `SummarizeRange`
- `ListEntriesRange`
- `ListModelSummariesRange`

- [ ] **Step 4: Run test to verify it passes**

Run: `docker compose exec -T api go test ./internal/repository -run TestPoeUsageRepo -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add api/internal/repository/poe_usage.go api/internal/repository/poe_usage_test.go
git commit -m "Poe Usage repositoryを追加"
```

## Chunk 2: Sync and API

### Task 3: Poe Usage service を DB sync 対応にする

**Files:**
- Modify: `api/internal/service/poe_usage.go`
- Modify: `api/internal/service/poe_usage_test.go`

- [ ] **Step 1: Write the failing test**

```go
func TestPoeUsageService_SyncPointsHistoryPersistsEntries(t *testing.T) {
    // mocked Poe API responses
    // sync should insert rows and return counts
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `docker compose exec -T api go test ./internal/service -run TestPoeUsageService -v`
Expected: FAIL with missing sync path

- [ ] **Step 3: Write minimal implementation**

追加内容:

- `SyncUsage(ctx, userID, apiKey, triggerType)`
- `FetchCurrentBalance(ctx, apiKey)` は live のまま分離
- `points_history` paging + upsert
- `today / yesterday / 7d / 14d / 30d / mtd / prev_month` の range helper

- [ ] **Step 4: Run test to verify it passes**

Run: `docker compose exec -T api go test ./internal/service -run TestPoeUsageService -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add api/internal/service/poe_usage.go api/internal/service/poe_usage_test.go
git commit -m "Poe Usage同期サービスを追加"
```

### Task 4: Poe Models handler を DB 集計ベースへ切り替える

**Files:**
- Modify: `api/internal/handler/poe_models.go`
- Modify: `api/cmd/server/main.go`

- [ ] **Step 1: Write the failing handler test**

```go
func TestPoeModelsHandler_UsageReturnsPersistedRangeSummary(t *testing.T) {
    // seed repo rows
    // GET /api/poe-models/usage?range=14d&entries_limit=100
    // expect selected range summary + latest sync metadata
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `docker compose exec -T api go test ./internal/handler -run TestPoeModelsHandler_Usage -v`
Expected: FAIL because handler still live-fetches everything

- [ ] **Step 3: Write minimal implementation**

変更内容:

- `GET /api/poe-models/usage`
  - DB summary
  - DB model rollups
  - DB recent entries
  - live `current_balance`
- `POST /api/poe-models/usage/sync`
  - manual sync endpoint

- [ ] **Step 4: Run test to verify it passes**

Run: `docker compose exec -T api go test ./internal/handler -run TestPoeModelsHandler_Usage -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add api/internal/handler/poe_models.go api/cmd/server/main.go
git commit -m "Poe Usage APIを永続化ベースへ切り替え"
```

## Chunk 3: UI and Verification

### Task 5: web API 型と取得関数を期間対応にする

**Files:**
- Modify: `web/src/lib/api.ts`

- [ ] **Step 1: Write the failing type usage**

想定変更:

- `range`
- `usage_type`
- `last_synced_at`
- `sync_status`
- `entry_count`
- `avg points/usd per call`

- [ ] **Step 2: Run build to verify it fails after UI references new fields**

Run: `make web-build`
Expected: FAIL until API client types are updated

- [ ] **Step 3: Write minimal implementation**

```ts
getPoeUsage(params?: { range?: string; entriesLimit?: number; usageType?: string })
```

- [ ] **Step 4: Run build to verify API client compiles**

Run: `make web-build`
Expected: PASS for this layer

- [ ] **Step 5: Commit**

```bash
git add web/src/lib/api.ts
git commit -m "Poe Usage APIクライアントを期間対応に更新"
```

### Task 6: Poe Models Usage タブに期間切替を追加する

**Files:**
- Modify: `web/src/app/(main)/poe-models/page.tsx`
- Modify: `web/src/i18n/dictionaries/ja.ts`
- Modify: `web/src/i18n/dictionaries/en.ts`

- [ ] **Step 1: Write the failing UI behavior expectation**

確認内容:

- `今日 / 昨日 / 7日 / 14日 / 30日 / 今月 / 先月`
- `50 / 100 / 200` 件切替
- KPI とモデル別表と履歴表が同じ期間に追従

- [ ] **Step 2: Run build to verify current UI lacks the new state**

Run: `make web-build`
Expected: UI implementation not yet present

- [ ] **Step 3: Write minimal implementation**

追加内容:

- range segmented control
- latest sync status line
- sync button
- current balance fixed card
- existing KPI / model rollups / recent entries wired to selected range

- [ ] **Step 4: Run build to verify it passes**

Run: `make web-build`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add web/src/app/(main)/poe-models/page.tsx web/src/i18n/dictionaries/ja.ts web/src/i18n/dictionaries/en.ts
git commit -m "Poe Usage画面に期間切替を追加"
```

### Task 7: End-to-end verification

**Files:**
- Modify: none expected unless fixes are needed

- [ ] **Step 1: Run repository and service tests**

Run: `docker compose exec -T api go test ./internal/repository ./internal/service -v`
Expected: PASS

- [ ] **Step 2: Run handler tests**

Run: `docker compose exec -T api go test ./internal/handler -v`
Expected: PASS

- [ ] **Step 3: Run migration verification**

Run: `make migrate-version`
Expected: latest migration version is applied

- [ ] **Step 4: Run web build**

Run: `make web-build`
Expected: PASS

- [ ] **Step 5: Commit final integration**

```bash
git add api web db
git commit -m "Poe Usage永続化と期間集計を実装"
```

## Notes for Implementation

- `current_balance` は live fetch のまま維持する
- `points_history` は最大 30 日なので、初期版でも page load sync と manual sync は入れる
- `query_id` は request log と厳密に結べないので、`poe_usage_entries` は billing/consumption 用として扱う
- `bot_name` と catalog model の完全一致を前提にしない
- 期間境界は JST (`Asia/Tokyo`) で揃える
