# Meilisearch Search Autocomplete Implementation Plan

> **For agentic workers:** REQUIRED: Use superpowers:executing-plans or superpowers:subagent-driven-development if implementation is delegated. Keep verification evidence before claiming completion.

**Goal:** Add autocomplete to the existing Items search modal using a dedicated `search_suggestions` Meilisearch index that mixes `article / source / topic` candidates and applies source/topic selections as filters instead of free text.

**Architecture:** PostgreSQL remains the source of truth. Meilisearch keeps two derived indexes: the existing full-text `items` index and a new `search_suggestions` index. Suggestion updates are handled through dedicated async events/jobs rather than piggybacking on the full-text indexing flow. The UI calls a separate suggestions API while the user types, then maps article selections to query text and source/topic selections to filter state.

**Tech Stack:** Go API, PostgreSQL, Inngest, Meilisearch, Next.js App Router, existing Items modal and filter state.

---

## File Map

### Suggestion Runtime and Domain Types

- Modify: `api/internal/service/meilisearch.go`
  - Add helpers for suggestion index settings and query methods.
- Modify: `api/internal/model/model.go`
  - Add API types for suggestion responses.
- Create: `api/internal/service/search_suggestions.go`
  - Query Meilisearch and apply per-kind distribution rules.

### Suggestion Source Data and Sync

- Create: `api/internal/repository/search_suggestion_documents.go`
  - Build `article / source / topic` candidate documents from PostgreSQL.
- Create: `api/internal/inngest/search_suggestions_index.go`
  - Upsert/delete/backfill functions for the dedicated suggestions index.
- Modify: `api/internal/inngest/functions.go`
  - Register suggestion index functions.
- Modify: `api/internal/service/events.go`
  - Add publisher helpers for suggestion upsert/delete/backfill.
- Modify: `api/internal/inngest/process_item_flow.go`
  - Emit article/topic suggestion updates after summarize.
- Modify: `api/internal/handler/internal.go`
  - Add debug backfill endpoint for suggestions if needed.

### Suggestion API

- Create or modify: `api/internal/handler/items_search_suggestions.go`
  - Add `GET /api/items/search-suggestions`.
- Modify: `api/cmd/server/main.go`
  - Register the route and wire the service.

### Frontend Modal UX

- Modify: `web/src/lib/api.ts`
  - Add suggestion request/response types.
- Modify: `web/src/app/(main)/items/page.tsx`
  - Request suggestions and apply source/topic filters.
- Create: `web/src/components/items/search-autocomplete.tsx`
  - Render grouped suggestion items with keyboard navigation.
- Modify: existing search modal component or inline modal UI
  - Attach debounce, active index, and selection behavior.
- Modify: `web/src/i18n/dictionaries/ja.ts`
  - Add suggestion copy.
- Modify: `web/src/i18n/dictionaries/en.ts`
  - Add suggestion copy.

## Chunk 1: Suggestion Index Domain

### Task 1: Define the suggestion document model

- [ ] Add document types for `article / source / topic`
- [ ] Encode `id`, `user_id`, `kind`, `label`, `normalized`, `score`, and optional references
- [ ] Keep topic documents user-scoped and normalized

### Task 2: Build repository loaders

- [ ] Create repository queries for article suggestion docs
- [ ] Create repository queries for source suggestion docs with summarized article counts
- [ ] Create repository queries for topic suggestion docs aggregated per user/topic
- [ ] Add repository tests for:
  - topic deduplication
  - source counts
  - summarized-only article candidates

## Chunk 2: Async Indexing

### Task 3: Add suggestion index events and Inngest functions

- [ ] Add publisher helpers:
  - `SendSearchSuggestionArticleUpsertE`
  - `SendSearchSuggestionSourceUpsertE`
  - `SendSearchSuggestionTopicRefreshE`
  - `SendSearchSuggestionBackfillE`
- [ ] Create Inngest functions for:
  - article upsert
  - source upsert/delete
  - topic refresh
  - backfill
- [ ] Register functions in `api/internal/inngest/functions.go`

### Task 4: Hook updates into existing write paths

- [ ] On summarized completion, refresh article + topic suggestions
- [ ] On source create/update/delete, refresh source suggestions
- [ ] Decide whether item delete/restore should soft-adjust article candidates or be deferred to refresh jobs

## Chunk 3: Suggestion Query API

### Task 5: Implement suggestion service

- [ ] Query Meilisearch `search_suggestions`
- [ ] Filter by `user_id`
- [ ] Apply total limit `10`
- [ ] Apply per-kind caps:
  - article max `6`
  - source max `2`
  - topic max `2`
- [ ] Spill unused source/topic slots into article

### Task 6: Add API endpoint

- [ ] Add `GET /api/items/search-suggestions?q=...&limit=10`
- [ ] Validate `q` length and limit bounds
- [ ] Return UI-friendly suggestion payloads

## Chunk 4: Search Modal UX

### Task 7: Add frontend suggestion fetching

- [ ] Trigger only after 2+ chars
- [ ] Debounce 150ms–250ms
- [ ] Cancel or ignore stale responses

### Task 8: Add autocomplete list UI

- [ ] Render kind labels for article/source/topic
- [ ] Show `article_count` for source/topic
- [ ] Support arrow keys, Enter, Escape

### Task 9: Wire selection behavior

- [ ] Article:
  - set query text
  - run search
- [ ] Source:
  - apply source filter
  - run search immediately
- [ ] Topic:
  - apply topic filter
  - run search immediately

## Chunk 5: Backfill and Verification

### Task 10: Add suggestion backfill

- [ ] Add debug path to enqueue suggestion index backfill
- [ ] Backfill article/source/topic suggestion docs
- [ ] Keep this run tracking separate from full-text backfill

### Task 11: Verification

- [ ] `docker compose exec -T api go test ./...`
- [ ] `make web-lint`
- [ ] `make web-build`
- [ ] Manual check:
  - article suggestions appear first when relevant
  - source/topic candidates appear with labels
  - source/topic selection applies filters, not free text
  - suggestions fail closed when API is unavailable

## Suggested Commit Sequence

1. `検索候補ドキュメントとrepositoryを追加`
2. `検索候補indexのイベントとInngest処理を追加`
3. `検索候補APIを追加`
4. `検索モーダルにオートコンプリートを追加`
5. `検索候補バックフィルを追加`
