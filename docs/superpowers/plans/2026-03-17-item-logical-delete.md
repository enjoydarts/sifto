# Item Logical Delete Implementation Plan

> **For agentic workers:** REQUIRED: Use superpowers:subagent-driven-development (if subagents available) or superpowers:executing-plans to implement this plan. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Prevent deleted RSS items from being re-imported while excluding deleted items from all product UI and recommendation logic, but keeping their historical LLM cost data intact.

**Architecture:** Add `items.deleted_at` and convert item deletion to a soft delete. Treat `deleted_at IS NULL` as the default visibility condition across item-facing queries, source/item stats, briefing/ask/recommendation paths, and personalization inputs. Keep LLM usage and execution logs unchanged so cost analytics still include deleted items.

**Tech Stack:** Go API, PostgreSQL migrations, Next.js web, docker compose / make verification

---

## Chunk 1: Data Model And Delete Behavior

### Task 1: Add item soft-delete field

**Files:**
- Create: `db/migrations/000070_add_items_deleted_at.up.sql`
- Create: `db/migrations/000070_add_items_deleted_at.down.sql`
- Test: `make migrate-up`

- [ ] Add migration to append nullable `deleted_at TIMESTAMPTZ` to `items`
- [ ] Add supporting index if query fanout needs it, scoped to visibility lookups
- [ ] Run `make migrate-up`
- [ ] Run `make migrate-version`

### Task 2: Convert delete endpoint to logical delete

**Files:**
- Modify: `api/internal/repository/items.go`
- Modify: `api/internal/handler/items.go`
- Test: `docker compose exec -T api go test ./internal/repository/... ./internal/handler/...`

- [ ] Change `ItemRepo.Delete` from `DELETE FROM items` to `UPDATE items SET deleted_at = NOW(), updated_at = NOW()`
- [ ] Keep ownership check intact
- [ ] Ensure delete remains idempotent enough for repeated calls
- [ ] Run repository and handler tests

### Task 3: Prevent re-import of logically deleted items

**Files:**
- Modify: `api/internal/repository/items.go`
- Modify: `api/internal/inngest/functions.go` (only if behavior contract changes)
- Test: `docker compose exec -T api go test ./internal/repository/... ./internal/inngest/...`

- [ ] Keep `UpsertFromFeed` using existing `(source_id, url)` uniqueness
- [ ] Treat rows with matching `(source_id, url)` as existing even when `deleted_at IS NOT NULL`
- [ ] Ensure logically deleted items return `created=false` and do not emit `item/created`
- [ ] Add or update tests for “deleted item is not recreated on feed upsert”

## Chunk 2: Visibility Filtering

### Task 4: Exclude deleted items from item-facing repository queries

**Files:**
- Modify: `api/internal/repository/items.go`
- Modify: `api/internal/repository/item_detail_queries.go`
- Modify: `api/internal/repository/review_queue.go`
- Test: `docker compose exec -T api go test ./internal/repository/...`

- [ ] Add `i.deleted_at IS NULL` to list/detail/related/reading-plan/highlight/topic/favorites/export queries
- [ ] Add the same visibility filter to review queue and any `items` joins used by item UX
- [ ] Keep LLM usage/value-metrics queries untouched unless they currently depend on visible items for cost totals
- [ ] Run repository tests

### Task 5: Exclude deleted items from source and personalization aggregates

**Files:**
- Modify: `api/internal/repository/sources.go`
- Modify: `api/internal/repository/preference_profile.go`
- Modify: `api/internal/repository/source_optimization.go`
- Modify: `api/internal/repository/feedback_ranking.go`
- Modify: `api/internal/repository/weekly_reviews.go`
- Test: `docker compose exec -T api go test ./internal/repository/...`

- [ ] Ensure source counts/daily stats/health-related item counts ignore deleted items
- [ ] Ensure favorite/read/rating-derived preference and ranking queries ignore deleted items
- [ ] Ensure weekly review and similar retrospective UX queries ignore deleted items
- [ ] Re-run repository tests

### Task 6: Exclude deleted items from Ask and Briefing

**Files:**
- Modify: `api/internal/repository/items.go`
- Modify: `api/internal/handler/briefing.go` (only if needed)
- Modify: `api/internal/service/*` only if hidden items leak through service composition
- Test: `docker compose exec -T api go test ./internal/repository/... ./internal/handler/... ./internal/service/...`

- [ ] Ensure `AskCandidatesByEmbedding` excludes deleted items
- [ ] Ensure briefing queues/highlights/clusters exclude deleted items
- [ ] Ensure deleted items do not influence recommendation reasons or queue totals
- [ ] Re-run focused API tests

## Chunk 3: API / Web Behavior

### Task 7: Keep deleted items out of web flows

**Files:**
- Modify: `web/src/app/(main)/items/page.tsx` if response shape needs handling
- Modify: `web/src/components/items/item-card.tsx` only if deleted state leaks through
- Test: `make web-build`

- [ ] Confirm list/detail/retry/unprocessed flows no longer surface deleted items
- [ ] Confirm source-linked item views do not show deleted rows
- [ ] Run `make web-build`

### Task 8: Preserve LLM analytics behavior

**Files:**
- Review: `api/internal/repository/llm_usage_logs.go`
- Review: `api/internal/repository/llm_value_metrics.go`
- Review: `api/internal/repository/ask_insights.go`
- Test: `docker compose exec -T api go test ./internal/repository/...`

- [ ] Verify deleted items do not get filtered out of LLM cost/usage totals
- [ ] If any usage query joins `items`, avoid adding `deleted_at IS NULL` there unless the metric is product-facing rather than cost-facing
- [ ] Add/update test coverage only where cost aggregation behavior could regress

## Chunk 4: Verification

### Task 9: Run end-to-end verification

**Files:**
- Modify: tests only if required by previous tasks

- [ ] Run `make fmt-go`
- [ ] Run `make migrate-up`
- [ ] Run `docker compose exec -T api go test ./internal/repository/... ./internal/handler/... ./internal/service/... ./internal/inngest/...`
- [ ] Run `make web-build`
- [ ] Sanity-check: delete an item, re-run feed ingestion, confirm it does not reappear
