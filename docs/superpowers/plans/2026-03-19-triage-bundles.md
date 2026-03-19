# Triage Bundles Implementation Plan

> **For agentic workers:** REQUIRED: Use superpowers:subagent-driven-development (if subagents available) or superpowers:executing-plans to implement this plan. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Rework Triage so users can process one news bundle at a time, collapsing duplicate coverage into a single decision while keeping unmatched items available as normal single-item triage cards.

**Architecture:** Extend the existing reading-plan / cluster pipeline instead of creating a separate product surface. Add a Triage-specific bundle contract in the API, generate high-confidence bundles from embeddings plus facts/time guards in the repository layer, then update the Triage page to render a mixed queue of `bundle | item` cards with bulk actions and bundle expansion.

**Tech Stack:** Go API, PostgreSQL, existing embeddings data, Next.js App Router, React Query, i18n dictionaries, docker compose / make verification

---

## Chunk 1: Define The Triage Bundle Contract

### Task 1: Add Triage bundle response models

**Files:**
- Modify: `api/internal/model/model.go`
- Modify: `web/src/lib/api.ts`
- Test: `docker compose exec -T api go test ./internal/model/... ./internal/handler/...`
- Test: `make web-build`

- [ ] Add API-side types for `TriageBundle`, `TriageQueueEntry`, and `TriageQueueResponse`
- [ ] Keep existing `FocusQueueResponse` untouched so non-triage callers do not regress
- [ ] Mirror the new contract in `web/src/lib/api.ts` with a discriminated union for `entry_type: "item" | "bundle"`
- [ ] Include only fields Triage needs for fast rendering: representative item, member items, size, similarity/confidence, shared facts/topics, and queue counters
- [ ] Run `docker compose exec -T api go test ./internal/model/... ./internal/handler/...`
- [ ] Run `make web-build`

### Task 2: Add Triage bundle API methods

**Files:**
- Modify: `web/src/lib/api.ts`
- Modify: `api/internal/handler/items.go`
- Modify: `api/cmd/server/main.go`
- Test: `docker compose exec -T api go test ./internal/handler/...`
- Test: `make web-build`

- [ ] Add a dedicated handler route for Triage queue retrieval, for example `/items/triage-queue`
- [ ] Keep `/items/focus-queue` and `/items/triage-all` stable until the UI migration is complete
- [ ] Add a new web API client method such as `api.getTriageQueue(...)`
- [ ] Reuse existing auth, cache, and invalidation patterns from `FocusQueue` / `TriageAll`
- [ ] Run `docker compose exec -T api go test ./internal/handler/...`
- [ ] Run `make web-build`

## Chunk 2: Generate High-Confidence Bundles

### Task 3: Add repository query for Triage candidates

**Files:**
- Modify: `api/internal/repository/items.go`
- Modify: `api/internal/repository/item_detail_queries.go` only if helper loading is shared there
- Test: `docker compose exec -T api go test ./internal/repository/...`

- [ ] Extract the candidate-loading portion of `ItemRepo.ReadingPlan` into a helper reusable by Triage
- [ ] Load the fields needed for bundle decisions: summary, summary topics, translated title, facts, timestamps, feedback/read/later state
- [ ] Preserve existing visibility filters: summarized only, unread only, exclude later by default, `deleted_at IS NULL`
- [ ] Keep candidate ordering score-first before bundle grouping so the best representative tends to surface first
- [ ] Run `docker compose exec -T api go test ./internal/repository/...`

### Task 4: Implement Triage-specific bundle rules

**Files:**
- Modify: `api/internal/repository/reading_plan_clusters.go`
- Modify: `api/internal/repository/items.go`
- Test: `docker compose exec -T api go test ./internal/repository/...`

- [ ] Add a dedicated clustering path such as `TriageBundlesByEmbeddings(...)` rather than reusing briefing thresholds as-is
- [ ] Use embeddings as candidate generation only
- [ ] Gate bundle membership with stricter checks based on publish-time proximity, topic overlap, and `item_facts.facts` overlap
- [ ] Keep conservative thresholds so uncertain candidates fall back to single-item entries
- [ ] Limit typical bundle size to a small range so a giant topic cluster does not become a single Triage card
- [ ] Reuse existing sort helpers where possible, but do not let analysis-oriented cluster ordering override Triage usability
- [ ] Run `docker compose exec -T api go test ./internal/repository/...`

### Task 5: Add focused tests for false-positive control

**Files:**
- Modify: `api/internal/repository/sources_daily_overview_test.go` only if shared fixtures live there
- Create or Modify: `api/internal/repository/reading_plan_clusters_test.go`
- Test: `docker compose exec -T api go test ./internal/repository/...`

- [ ] Add tests for “same event, multiple sources” becoming one bundle
- [ ] Add tests for “same company, different event” staying split
- [ ] Add tests for large topical clusters staying split into smaller bundles or singletons
- [ ] Add tests for missing embeddings or missing facts falling back safely
- [ ] Run `docker compose exec -T api go test ./internal/repository/...`

## Chunk 3: Serve A Mixed Triage Queue

### Task 6: Build queue assembly and caching in the handler

**Files:**
- Modify: `api/internal/handler/items.go`
- Modify: `api/internal/handler/cache_keys.go`
- Modify: `api/internal/handler/cache_keys_test.go`
- Test: `docker compose exec -T api go test ./internal/handler/...`

- [ ] Add a handler that assembles a mixed queue of bundle entries followed by singleton entries not consumed into any bundle
- [ ] Preserve existing `mode=quick|all`, window, size, and diversification settings where they still make sense
- [ ] Add a separate cache key namespace for the new triage queue so the old `focus-queue` cache remains valid during rollout
- [ ] Return queue counters for total entries, total underlying items, bundle count, and remaining count
- [ ] Run `docker compose exec -T api go test ./internal/handler/...`

### Task 7: Add bulk bundle action endpoints or reuse existing bulk item actions cleanly

**Files:**
- Modify: `api/internal/handler/items.go`
- Modify: `web/src/lib/api.ts`
- Test: `docker compose exec -T api go test ./internal/handler/...`

- [ ] Decide explicitly whether the web client will call existing bulk endpoints with member IDs or a new bundle action endpoint
- [ ] Prefer reusing `mark-read-bulk` and `mark-later-bulk` if the UI already has member IDs
- [ ] Add a bundle-level “exclude” behavior definition before implementation; if there is no current state for it, keep it out of MVP rather than inventing a weak placeholder
- [ ] Ensure all bulk actions bump the same cache versions the current item actions rely on
- [ ] Run `docker compose exec -T api go test ./internal/handler/...`

## Chunk 4: Rework The Triage UI Around Bundles

### Task 8: Replace item-only card flow with mixed bundle/item rendering

**Files:**
- Modify: `web/src/app/(main)/triage/page.tsx`
- Modify: `web/src/components/thumbnail.tsx` only if bundle cards need a safe representative image fallback
- Test: `make web-build`

- [ ] Switch Triage data loading from `getFocusQueue/getTriageAll` to the new Triage queue API
- [ ] Render bundle entries as the primary card type, with singleton items using the existing fast-path card
- [ ] Make the bundle card summary-first: representative title, concise summary, shared facts/topics, source count, member count
- [ ] Keep the current keyboard / swipe interaction model, but map actions onto bundle member IDs when the current card is a bundle
- [ ] Preserve inline reader access for singleton items and representative items
- [ ] Run `make web-build`

### Task 9: Add bundle expansion and exception handling

**Files:**
- Modify: `web/src/app/(main)/triage/page.tsx`
- Modify: `web/src/i18n/dictionaries/ja.ts`
- Modify: `web/src/i18n/dictionaries/en.ts`
- Test: `make web-build`

- [ ] Add a lightweight “show bundle members” panel listing member titles and sources
- [ ] Add MVP exception handling for “keep just one item visible” or “open representative detail before acting”
- [ ] Keep the initial UI narrow: no bundle editing, no drag-and-drop member management, no custom merge/split controls
- [ ] Add i18n keys for bundle labels, counts, expansion, and action copy
- [ ] Run `make web-build`

### Task 10: Retire or demote the separate Clusters page path

**Files:**
- Modify: `web/src/app/(main)/clusters/page.tsx`
- Modify: `web/src/app/(main)/page.tsx`
- Modify: `web/src/components/nav.tsx`
- Modify: `web/src/i18n/dictionaries/ja.ts`
- Modify: `web/src/i18n/dictionaries/en.ts`
- Test: `make web-build`

- [ ] Decide whether `/clusters` becomes a secondary inspection page or is linked less prominently after Triage absorbs bundle handling
- [ ] Remove duplicated CTA language that sends users to both cluster triage and item triage for the same job
- [ ] Keep any remaining Clusters surface clearly positioned as “browse by topic”, not “daily cleanup”
- [ ] Run `make web-build`

## Chunk 5: Instrumentation And Verification

### Task 11: Add triage bundle instrumentation

**Files:**
- Modify: `web/src/app/(main)/triage/page.tsx`
- Modify: `api/internal/handler/items.go` only if server-returned counters need expansion
- Test: `make web-build`

- [ ] Extend local triage metrics to capture bundle actions, member counts consumed, and bundle expansion rate
- [ ] Keep existing item-level speed metrics so before/after comparisons remain possible
- [ ] Surface one or two bundle-specific stats in the existing metrics panel rather than adding a new dashboard
- [ ] Run `make web-build`

### Task 12: Run end-to-end verification

**Files:**
- Modify: tests only if required by earlier tasks

- [ ] Run `make fmt-go`
- [ ] Run `docker compose exec -T api go test ./internal/repository/... ./internal/handler/... ./internal/service/...`
- [ ] Run `make web-build`
- [ ] Manually verify in Triage that one same-news bundle can be marked read in a single action
- [ ] Manually verify that an unrelated but embedding-near story still appears as a separate entry
- [ ] Manually verify that `あとで読む` / `既読` invalidates briefing, items feed, and Triage queue correctly

Plan complete and saved to `docs/superpowers/plans/2026-03-19-triage-bundles.md`. Ready to execute?
