# Source AI Navigator Implementation Plan

> **For agentic workers:** REQUIRED: Use superpowers:subagent-driven-development (if subagents available) or superpowers:executing-plans to implement this plan. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add a lazy, persona-driven AI navigator on the Sources page that reviews the user's source lineup and suggests what to keep, watch, or revisit.

**Architecture:** Reuse the existing navigator pattern: add a dedicated `/api/sources/navigator` endpoint with 30-minute caching, aggregate 30-day source metrics in the API, generate structured commentary in the worker, and render the result in a bottom-right overlay on the Sources page. Keep settings/persona/model reuse aligned with existing navigator configuration while recording usage under a new `source_navigator` purpose.

**Tech Stack:** Go API, PostgreSQL repositories, Python worker, Next.js App Router, React Query, shared AI navigator persona definitions.

---

## Chunk 1: API contract and data aggregation

**Files:**
- Modify: `api/internal/model/model.go`
- Modify: `api/internal/repository/sources.go`
- Modify: `api/internal/service/worker.go`
- Modify: `api/internal/handler/cache_keys.go`
- Modify: `api/internal/handler/sources.go`
- Test: `api/internal/handler/sources_test.go`

- [ ] Add source navigator response models in `api/internal/model/model.go`
- [ ] Add repository query for 30-day source navigator metrics in `api/internal/repository/sources.go`
- [ ] Add worker request/response structs for source navigator in `api/internal/service/worker.go`
- [ ] Add cache key helper for source navigator in `api/internal/handler/cache_keys.go`
- [ ] Add `/api/sources/navigator` handler with persona/model resolution, 30-minute cache, and usage recording in `api/internal/handler/sources.go`
- [ ] Add handler/repository tests for cache key and empty-data behavior

## Chunk 2: Worker generation flow

**Files:**
- Modify: `worker/app/services/feed_task_common.py`
- Modify: `worker/app/routers/briefing_navigator.py` or add `worker/app/routers/source_navigator.py`
- Modify: provider service files that dispatch navigator generation
- Test: `worker/app/services/test_feed_task_common.py`

- [ ] Define source navigator schema and task builder in `feed_task_common.py`
- [ ] Add parser for source navigator output with sane fallbacks
- [ ] Add worker router endpoint for source navigator
- [ ] Wire provider-specific dispatch functions to call the new task/parser
- [ ] Add prompt tests covering overview length and keep/watch/standout structure

## Chunk 3: Usage purpose and migrations

**Files:**
- Create: `db/migrations/0000xx_allow_source_navigator_purpose_in_llm_usage_logs.up.sql`
- Modify: `db/migrations/0000xx_allow_source_navigator_purpose_in_llm_usage_logs.down.sql` if needed by project pattern
- Test: existing API handler tests

- [ ] Add `source_navigator` to `llm_usage_logs` purpose check
- [ ] Ensure API records usage with purpose `source_navigator`
- [ ] Run migrations locally and verify schema version if a migration is added

## Chunk 4: Sources page UI

**Files:**
- Modify: `web/src/lib/api.ts`
- Modify: `web/src/app/(main)/sources/page.tsx`
- Modify: `web/src/i18n/dictionaries/ja.ts`
- Modify: `web/src/i18n/dictionaries/en.ts`
- Reuse: `web/src/components/briefing/ai-navigator-avatar.tsx`

- [ ] Add Source navigator types and client method in `web/src/lib/api.ts`
- [ ] Add right-bottom mini button, lazy fetch, loading bubble, overlay, and close/reopen state in `sources/page.tsx`
- [ ] Render long `overview` plus `keep/watch/standout` sections with source links
- [ ] Add i18n strings for loading, error, empty state, and section labels
- [ ] Ensure mobile bottom offset matches existing navigator behavior

## Chunk 5: Verification

**Files:**
- No code changes expected

- [ ] Run `docker compose exec -T api go test ./internal/handler ./internal/service ./internal/repository`
- [ ] Run `docker compose exec -T worker python -m unittest app.services.test_feed_task_common`
- [ ] Run `make check-worker`
- [ ] Run `make web-build`
- [ ] Commit the feature once all checks pass
