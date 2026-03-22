# Meilisearch Full-Text Search Implementation Plan

> **For agentic workers:** REQUIRED: Use superpowers:subagent-driven-development (if subagents available) or superpowers:executing-plans to implement this plan. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace the current `ILIKE`-based Items search with Meilisearch-backed full-text search that supports Japanese queries, search modes, relevance ordering, and highlighted multi-snippet results inside the existing search modal.

**Architecture:** PostgreSQL remains the source of truth for items, summaries, and facts. Meilisearch becomes a derived, asynchronous search index fed by Inngest events and an initial backfill job. `GET /api/items` keeps its current role, but when `q` is present it queries Meilisearch for ranked item IDs and snippets, then hydrates those IDs from PostgreSQL and returns the usual item payload plus search metadata.

**Tech Stack:** Go API, PostgreSQL, Inngest, Redis, Meilisearch, Next.js App Router, React Query, existing Docker Compose local stack.

---

## File Map

### Infrastructure and Search Runtime

- Create: `api/internal/service/meilisearch.go`
  - Build a Meilisearch client from env and expose index helpers.
- Create: `api/internal/service/meilisearch_test.go`
  - Validate env parsing, client config, and mode parsing helpers.
- Modify: `api/cmd/server/main.go`
  - Initialize the search client/service and inject it into handlers and Inngest deps.
- Modify: `docker-compose.yml`
  - Add the Meilisearch service and wire API dependency.
- Modify: `.env.example`
  - Document Meilisearch URL and master key env vars.

### Search Document Build and Index Sync

- Create: `api/internal/repository/item_search_documents.go`
  - Load one or many summarized items as search documents from PostgreSQL.
- Create: `api/internal/repository/item_search_documents_test.go`
  - Cover query behavior, content shaping, and summarized-only selection.
- Create: `api/internal/inngest/item_search_index.go`
  - Upsert/delete/backfill functions for the search index.
- Create: `api/internal/inngest/item_search_index_test.go`
  - Cover event payload validation and service invocation.
- Modify: `api/internal/inngest/functions.go`
  - Register new search-index functions.
- Modify: `api/internal/inngest/process_item_flow.go`
  - Emit upsert events after summarize/facts/title-bearing stages complete.
- Modify: `api/internal/service/events.go`
  - Add publisher helpers for item search upsert/delete/backfill events.
- Modify: `api/internal/handler/internal.go`
  - Add a debug endpoint to trigger summarized-item backfill.

### API Search Integration

- Create: `api/internal/service/item_search.go`
  - Search Meilisearch, map search modes, and normalize snippets.
- Create: `api/internal/service/item_search_test.go`
  - Cover search mode mapping, snippet selection, and fallback behavior.
- Create: `api/internal/repository/items_search_results.go`
  - Hydrate item IDs in caller-provided order and merge search metadata.
- Create: `api/internal/repository/items_search_results_test.go`
  - Verify preserved ordering and hydration behavior.
- Modify: `api/internal/handler/items.go`
  - Branch `GET /api/items` into DB-only vs search-backed path.
- Modify: `api/internal/handler/cache_keys.go`
  - Include `search_mode` in item list cache keys.
- Modify: `api/internal/handler/items_cache_test.go`
  - Lock down the updated cache-key format.
- Modify: `api/internal/model/model.go`
  - Add backend response types for search snippets and counts.
- Modify: `api/internal/repository/items.go`
  - Remove direct `ILIKE` query behavior from the search branch only after Meilisearch path is wired.

### Frontend Integration

- Modify: `web/src/lib/api.ts`
  - Add `search_mode`, `search_match_count`, and `search_snippets` types/params.
- Create: `web/src/components/items/search-modal.tsx`
  - Own the expanded search modal UI and mode switching.
- Create: `web/src/components/items/search-snippets.tsx`
  - Render labeled, highlighted snippets under each search result.
- Modify: `web/src/app/(main)/items/page.tsx`
  - Use the new search modal component and persist `search_mode` in the URL/query state.
- Modify: `web/src/components/items/item-card.tsx`
  - Show snippet blocks only when search metadata is present.
- Modify: `web/src/i18n/dictionaries/ja.ts`
  - Add Japanese copy for search modes, snippet labels, and search-unavailable states.
- Modify: `web/src/i18n/dictionaries/en.ts`
  - Add English copy for search modes, snippet labels, and search-unavailable states.

### Verification

- Modify: `api/internal/handler/items_test.go` or create `api/internal/handler/items_search_test.go`
  - Cover `GET /api/items` search mode handling and search failure responses.
- Optional later: `web` E2E coverage
  - No Playwright harness exists today; do not bootstrap E2E in the same implementation unless scope is explicitly expanded.

## Chunk 1: Search Runtime and Environment

### Task 1: Add Meilisearch Runtime Configuration

**Files:**
- Create: `api/internal/service/meilisearch.go`
- Create: `api/internal/service/meilisearch_test.go`
- Modify: `api/cmd/server/main.go`
- Modify: `docker-compose.yml`
- Modify: `.env.example`

- [ ] **Step 1: Write the failing service test**

Add tests for env parsing and default index naming in `api/internal/service/meilisearch_test.go`.

- [ ] **Step 2: Run the failing test**

Run: `docker compose exec -T api go test ./internal/service -run TestNewMeilisearchClientFromEnv -v`
Expected: FAIL because the client factory does not exist yet.

- [ ] **Step 3: Add Meilisearch env vars to `.env.example`**

Document:

```env
MEILISEARCH_URL=http://meilisearch:7700
MEILISEARCH_MASTER_KEY=change-me
MEILISEARCH_ITEMS_INDEX=items
```

- [ ] **Step 4: Add the `meilisearch` service to `docker-compose.yml`**

Use the official image, expose `7700`, persist data in a named volume, and inject the master key.

- [ ] **Step 5: Implement `api/internal/service/meilisearch.go`**

Include:

```go
type MeilisearchService struct {
    client *meilisearch.Client
    itemsIndex string
}
```

Expose:
- `NewMeilisearchServiceFromEnv()`
- `ItemsIndexName()`
- `Health(ctx context.Context) error`

- [ ] **Step 6: Wire the search service in `api/cmd/server/main.go`**

Construct the service near the other infrastructure clients and pass it into items/internal handlers and Inngest deps.

- [ ] **Step 7: Run the service test again**

Run: `docker compose exec -T api go test ./internal/service -run TestNewMeilisearchClientFromEnv -v`
Expected: PASS

- [ ] **Step 8: Commit**

```bash
git add .env.example docker-compose.yml api/cmd/server/main.go api/internal/service/meilisearch.go api/internal/service/meilisearch_test.go
git commit -m "Meilisearchランタイムを追加"
```

## Chunk 2: Search Documents and Async Indexing

### Task 2: Build Search Documents from PostgreSQL

**Files:**
- Create: `api/internal/repository/item_search_documents.go`
- Create: `api/internal/repository/item_search_documents_test.go`

- [ ] **Step 1: Write the failing repository test**

Cover:
- summarized items are returned
- `facts` are flattened into `facts_text`
- non-summarized items are excluded

- [ ] **Step 2: Run the failing test**

Run: `docker compose exec -T api go test ./internal/repository -run TestItemSearchDocuments -v`
Expected: FAIL because the repository does not exist yet.

- [ ] **Step 3: Implement the repository query**

Create a focused loader that returns:

```go
type ItemSearchDocument struct {
    ID              string
    UserID          string
    SourceID        string
    Status          string
    IsDeleted       bool
    Title           string
    TranslatedTitle string
    Summary         string
    FactsText       string
    ContentText     string
    PublishedAt     *time.Time
    CreatedAt       time.Time
}
```

- [ ] **Step 4: Re-run repository tests**

Run: `docker compose exec -T api go test ./internal/repository -run TestItemSearchDocuments -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add api/internal/repository/item_search_documents.go api/internal/repository/item_search_documents_test.go
git commit -m "検索ドキュメント生成用のrepositoryを追加"
```

### Task 3: Add Async Search Index Events and Inngest Functions

**Files:**
- Create: `api/internal/inngest/item_search_index.go`
- Create: `api/internal/inngest/item_search_index_test.go`
- Modify: `api/internal/inngest/functions.go`
- Modify: `api/internal/inngest/process_item_flow.go`
- Modify: `api/internal/service/events.go`

- [ ] **Step 1: Write the failing Inngest test**

Add tests for:
- upsert event accepts an item ID
- delete event deletes by item ID
- backfill walks summarized items in batches

- [ ] **Step 2: Run the failing test**

Run: `docker compose exec -T api go test ./internal/inngest -run TestItemSearchIndex -v`
Expected: FAIL because the functions are not registered yet.

- [ ] **Step 3: Extend the event publisher**

Add publisher helpers such as:
- `SendItemSearchUpsertE`
- `SendItemSearchDeleteE`
- `SendItemSearchBackfillE`

- [ ] **Step 4: Implement the Inngest search-index functions**

Responsibilities:
- load the latest search document from PostgreSQL
- create/update the Meilisearch document
- delete from Meilisearch on delete events
- page through summarized items for backfill

- [ ] **Step 5: Emit upsert events from summarize flow**

Emit search upsert after successful summarize persistence in `api/internal/inngest/process_item_flow.go`.

- [ ] **Step 6: Register the functions**

Update `api/internal/inngest/functions.go` so the handlers are mounted with the rest of the flow.

- [ ] **Step 7: Re-run Inngest tests**

Run: `docker compose exec -T api go test ./internal/inngest -run TestItemSearchIndex -v`
Expected: PASS

- [ ] **Step 8: Commit**

```bash
git add api/internal/service/events.go api/internal/inngest/functions.go api/internal/inngest/process_item_flow.go api/internal/inngest/item_search_index.go api/internal/inngest/item_search_index_test.go
git commit -m "検索インデックス同期のInngest処理を追加"
```

### Task 4: Add a Summarized-Only Backfill Trigger

**Files:**
- Modify: `api/internal/handler/internal.go`
- Modify: `api/cmd/server/main.go`

- [ ] **Step 1: Add a debug endpoint handler**

Add a handler that queues the summarized-only backfill event.

- [ ] **Step 2: Mount the route**

Expose a new internal debug route alongside the other backfill endpoints.

- [ ] **Step 3: Smoke-test the route**

Run: `docker compose exec -T api go test ./internal/handler -run TestInternal -v`
Expected: PASS or no relevant test files; if no test exists, add a targeted handler test before proceeding.

- [ ] **Step 4: Commit**

```bash
git add api/internal/handler/internal.go api/cmd/server/main.go
git commit -m "検索インデックスのバックフィル起動APIを追加"
```

## Chunk 3: API Search Path

### Task 5: Implement Search Service and Search-Aware Hydration

**Files:**
- Create: `api/internal/service/item_search.go`
- Create: `api/internal/service/item_search_test.go`
- Create: `api/internal/repository/items_search_results.go`
- Create: `api/internal/repository/items_search_results_test.go`
- Modify: `api/internal/model/model.go`

- [ ] **Step 1: Write the failing tests**

Cover:
- mode parsing for `natural`, `and`, `or`
- snippet capping at 3
- preservation of Meilisearch result order when hydrating from PostgreSQL

- [ ] **Step 2: Run the failing tests**

Run: `docker compose exec -T api go test ./internal/service ./internal/repository -run 'TestItemSearch|TestHydrateSearchResults' -v`
Expected: FAIL because the search service and hydration helpers do not exist yet.

- [ ] **Step 3: Add search response types to `model.go`**

Add:

```go
type ItemSearchSnippet struct {
    Field       string `json:"field"`
    SnippetHTML string `json:"snippet_html"`
}
```

and attach:
- `SearchMatchCount *int`
- `SearchSnippets []ItemSearchSnippet`

to `model.Item`.

- [ ] **Step 4: Implement the Meilisearch query service**

Support:
- current-list filter construction
- search modes
- normalized snippet extraction
- `search unavailable` sentinel errors

- [ ] **Step 5: Implement ordered DB hydration**

Load item rows by ID and preserve Meilisearch rank order when returning the hydrated slice.

- [ ] **Step 6: Re-run the focused tests**

Run: `docker compose exec -T api go test ./internal/service ./internal/repository -run 'TestItemSearch|TestHydrateSearchResults' -v`
Expected: PASS

- [ ] **Step 7: Commit**

```bash
git add api/internal/model/model.go api/internal/service/item_search.go api/internal/service/item_search_test.go api/internal/repository/items_search_results.go api/internal/repository/items_search_results_test.go
git commit -m "検索サービスと検索結果ハイドレーションを追加"
```

### Task 6: Integrate `GET /api/items` with Search Mode and Cache Keys

**Files:**
- Modify: `api/internal/handler/items.go`
- Modify: `api/internal/handler/cache_keys.go`
- Modify: `api/internal/handler/items_cache_test.go`
- Modify: `api/internal/repository/items.go`

- [ ] **Step 1: Write the failing handler test**

Cover:
- `q` + `search_mode` uses the search service
- `q` absent uses the legacy DB path
- cache key changes when `search_mode` changes

- [ ] **Step 2: Run the failing handler test**

Run: `docker compose exec -T api go test ./internal/handler -run 'TestItemsListSearch' -v`
Expected: FAIL because `search_mode` is not wired yet.

- [ ] **Step 3: Thread `search_mode` through the request path**

Update:
- request parsing
- cache-key generation
- handler branching
- response metadata merge

- [ ] **Step 4: Remove direct `ILIKE` dependency from the search branch**

Leave non-search list queries unchanged, but stop relying on `ILIKE` for `q` requests.

- [ ] **Step 5: Re-run handler tests**

Run: `docker compose exec -T api go test ./internal/handler -run 'TestItemsListSearch' -v`
Expected: PASS

- [ ] **Step 6: Commit**

```bash
git add api/internal/handler/items.go api/internal/handler/cache_keys.go api/internal/handler/items_cache_test.go api/internal/repository/items.go
git commit -m "Items APIにMeilisearch検索を統合"
```

## Chunk 4: Web Search Experience

### Task 7: Extend the Web API Client and Query State

**Files:**
- Modify: `web/src/lib/api.ts`
- Modify: `web/src/app/(main)/items/page.tsx`

- [ ] **Step 1: Write the TypeScript shape changes**

Add:
- `search_mode` request param
- `search_match_count`
- `search_snippets`

- [ ] **Step 2: Thread `search_mode` through query-state management**

Persist it in:
- URL params
- React Query cache key
- modal draft/apply flow

- [ ] **Step 3: Run lint**

Run: `make web-lint`
Expected: PASS with only the existing `layout.tsx` warning.

- [ ] **Step 4: Commit**

```bash
git add web/src/lib/api.ts web/src/app/(main)/items/page.tsx
git commit -m "Items検索のquery stateを拡張"
```

### Task 8: Extract and Build the Expanded Search Modal

**Files:**
- Create: `web/src/components/items/search-modal.tsx`
- Modify: `web/src/app/(main)/items/page.tsx`
- Modify: `web/src/i18n/dictionaries/ja.ts`
- Modify: `web/src/i18n/dictionaries/en.ts`

- [ ] **Step 1: Extract the inline modal into a component**

Move the existing modal out of `page.tsx` before adding new controls.

- [ ] **Step 2: Add mode switching UI**

Include:
- natural
- and
- or

and concise search syntax hints.

- [ ] **Step 3: Add i18n strings**

Update both dictionaries with:
- mode labels
- helper text
- search unavailable copy

- [ ] **Step 4: Run lint**

Run: `make web-lint`
Expected: PASS with only the existing `layout.tsx` warning.

- [ ] **Step 5: Commit**

```bash
git add web/src/components/items/search-modal.tsx web/src/app/(main)/items/page.tsx web/src/i18n/dictionaries/ja.ts web/src/i18n/dictionaries/en.ts
git commit -m "Items検索モーダルを全文検索向けに拡張"
```

### Task 9: Render Highlighted Multi-Snippet Search Results

**Files:**
- Create: `web/src/components/items/search-snippets.tsx`
- Modify: `web/src/components/items/item-card.tsx`

- [ ] **Step 1: Add a focused snippet renderer**

Render:
- field label
- trusted highlighted snippet HTML
- max 3 snippets

- [ ] **Step 2: Show snippets only for search-backed results**

Do not disturb the non-search list layout.

- [ ] **Step 3: Build and verify**

Run: `make web-build`
Expected: PASS

- [ ] **Step 4: Commit**

```bash
git add web/src/components/items/search-snippets.tsx web/src/components/items/item-card.tsx
git commit -m "検索結果にハイライト抜粋を表示"
```

## Chunk 5: End-to-End Verification and Rollout

### Task 10: Run Backend Verification

**Files:**
- Modify: any tests needed while fixing regressions

- [ ] **Step 1: Run the full API test suite**

Run: `docker compose exec -T api go test ./...`
Expected: PASS

- [ ] **Step 2: Run targeted backfill/search smoke checks**

Suggested commands:

```bash
docker compose exec -T api go test ./internal/inngest -run TestItemSearchIndex -v
docker compose exec -T api go test ./internal/handler -run TestItemsListSearch -v
```

Expected: PASS

- [ ] **Step 3: Commit any last backend fixes**

```bash
git add api
git commit -m "全文検索バックエンドの最終調整"
```

### Task 11: Run Web Verification and Manual Search Checklist

**Files:**
- Modify: web files only if verification finds issues

- [ ] **Step 1: Run lint and production build**

Run:

```bash
make web-lint
make web-build
```

Expected:
- lint: PASS with only the existing `layout.tsx` warning
- build: PASS

- [ ] **Step 2: Manual verification in the browser**

Check:
- regular list search in summarized feed
- `pending` feed search stays inside pending items
- `deleted` feed search stays inside deleted items
- mode switching changes result breadth
- snippets show labels and `<mark>` highlights
- search unavailable state renders without breaking the page

- [ ] **Step 3: Commit any last web fixes**

```bash
git add web
git commit -m "全文検索UIの最終調整"
```

- [ ] **Step 4: Final integration commit**

```bash
git add .
git commit -m "Meilisearch全文検索を実装"
```

## Notes for Execution

- Do not bootstrap Playwright in this plan unless the user explicitly expands scope; there is no existing E2E harness in this repository today.
- Keep search-index synchronization best-effort and non-blocking for the main item-processing path.
- Prefer small, isolated commits that match the tasks above.
- If Meilisearch is unavailable during development, keep the Items page usable and surface a search-specific error instead of breaking the full feed.

## Manual Rollout Checklist

- Start the updated stack with Meilisearch enabled
- Run the summarized-item backfill once
- Verify a known Japanese query and a known English query
- Verify delete/restore removes and re-adds the item from search results
- Verify newly summarized items become searchable after async indexing completes

Plan complete and saved to `docs/superpowers/plans/2026-03-22-meilisearch-fulltext-search.md`. Ready to execute?
