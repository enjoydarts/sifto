# OpenRouter User Overrides Implementation Plan

> **For agentic workers:** REQUIRED: Use superpowers:subagent-driven-development (if subagents available) or superpowers:executing-plans to implement this plan. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Let each user explicitly override `constrained` OpenRouter models as structured-output-capable from the OpenRouter Models screen, while still keeping truly `removed` models unavailable.

**Architecture:** Add a user-scoped override table keyed by `user_id + model_id`, expose read/write APIs on the OpenRouter Models surface, and centralize capability resolution so UI selection and runtime validation both apply the same merge rule: `removed` always stays unavailable, `constrained` may become available only when the user has opted in.

**Tech Stack:** Go API, PostgreSQL migrations, existing OpenRouter snapshot sync, Next.js App Router, React Query, docker compose / make verification

---

## Chunk 1: Persist User Overrides

### Task 1: Add user override table

**Files:**
- Create: `db/migrations/000071_add_user_openrouter_model_overrides.up.sql`
- Create: `db/migrations/000071_add_user_openrouter_model_overrides.down.sql`
- Test: `make migrate-up`
- Test: `make migrate-version`

- [ ] Add `user_openrouter_model_overrides`
- [ ] Columns: `user_id`, `model_id`, `allow_structured_output`, `created_at`, `updated_at`
- [ ] Add unique constraint on `(user_id, model_id)`
- [ ] Do not reference snapshot rows directly; overrides must survive snapshot refreshes as long as the same `model_id` still exists
- [ ] Run `make migrate-up`
- [ ] Run `make migrate-version`

### Task 2: Add repository methods

**Files:**
- Create or Modify: `api/internal/repository/openrouter_model_overrides.go`
- Test: `docker compose exec -T api go test ./internal/repository/...`

- [ ] Add `ListByUser(ctx, userID)` returning model-id keyed overrides
- [ ] Add upsert for `allow_structured_output`
- [ ] Add delete / clear method for a model override
- [ ] Keep repository focused on persistence only
- [ ] Run repository tests

## Chunk 2: Resolve Effective Availability

### Task 3: Add effective OpenRouter availability resolver

**Files:**
- Modify: `api/internal/service/openrouter_catalog.go`
- Create or Modify: `api/internal/service/openrouter_catalog_test.go`
- Test: `docker compose exec -T api go test ./internal/service/...`

- [ ] Keep `OpenRouterSnapshotAvailability` as the raw snapshot truth
- [ ] Add a new resolver that merges raw availability with user override
- [ ] Rule 1: `removed` can never be overridden
- [ ] Rule 2: `available` stays available regardless of override
- [ ] Rule 3: `constrained` becomes effectively available only when `allow_structured_output=true`
- [ ] Add tests for `constrained -> available` and `removed -> removed`
- [ ] Run service tests

### Task 4: Use effective availability in OpenRouter Models API

**Files:**
- Modify: `api/internal/handler/openrouter_models.go`
- Modify: `api/internal/model/model.go` if response schema needs override metadata
- Test: `docker compose exec -T api go test ./internal/handler/...`

- [ ] Load current user ID in OpenRouter Models list handler
- [ ] Load user overrides once per request
- [ ] Return both raw reason and effective availability state
- [ ] Include enough metadata for the web UI to render a toggle state per model
- [ ] Keep removed models unavailable even if an override row exists
- [ ] Run handler tests

## Chunk 3: Edit Overrides From OpenRouter Models Screen

### Task 5: Add API endpoint for per-user override mutation

**Files:**
- Modify: `api/cmd/server/main.go`
- Modify: `api/internal/handler/openrouter_models.go`
- Modify: `web/src/lib/api.ts`
- Test: `docker compose exec -T api go test ./internal/handler/...`
- Test: `make web-build`

- [ ] Add authenticated endpoint to set or clear `allow_structured_output` for a `model_id`
- [ ] Accept only models currently present in latest snapshots
- [ ] Reject override writes for removed models
- [ ] Reuse latest snapshot lookup instead of trusting arbitrary model IDs
- [ ] Add client method in `web/src/lib/api.ts`
- [ ] Run handler tests
- [ ] Run `make web-build`

### Task 6: Add toggle UI on OpenRouter Models page

**Files:**
- Modify: `web/src/app/(main)/openrouter-models/page.tsx`
- Modify: `web/src/i18n/dictionaries/ja.ts`
- Modify: `web/src/i18n/dictionaries/en.ts`
- Test: `make web-build`

- [ ] Show override toggle only for `constrained` models
- [ ] Hide or disable toggle for `removed` models
- [ ] Label clearly that this is a user-local exception for structured output
- [ ] After toggle change, refresh model list and preserve current tab/filter if possible
- [ ] Surface current override state in the unavailable / available table row
- [ ] Run `make web-build`

## Chunk 4: Apply Override To Settings And Runtime

### Task 7: Apply effective capability in settings catalog

**Files:**
- Modify: `api/internal/service/openrouter_catalog.go`
- Modify: `api/internal/handler/settings.go`
- Modify: `web/src/app/(main)/settings/page.tsx`
- Test: `docker compose exec -T api go test ./internal/service/... ./internal/handler/...`
- Test: `make web-build`

- [ ] Ensure OpenRouter models merged into the effective catalog for a user reflect override-adjusted `supports_structured_output`
- [ ] Update settings API/catalog path so overridden models are selectable
- [ ] Keep removed models filtered out / unavailable in settings
- [ ] Run service and handler tests
- [ ] Run `make web-build`

### Task 8: Apply effective capability to runtime validation

**Files:**
- Modify: `api/internal/service/settings_service.go`
- Modify: `api/internal/handler/ask.go`
- Modify: any shared model-capability validation path used by facts/summary/digest/ask
- Test: `docker compose exec -T api go test ./internal/service/... ./internal/handler/... ./internal/inngest/...`

- [ ] Find the single validation path that rejects models missing structured output
- [ ] Change it to consult effective user-specific OpenRouter capability instead of raw snapshot capability
- [ ] Keep validation unchanged for non-OpenRouter providers
- [ ] Ensure removed models still fail validation
- [ ] Run focused API/runtime tests

## Chunk 5: Verification

### Task 9: Verify end-to-end behavior

**Files:**
- Modify: tests only if required by earlier tasks

- [ ] Run `make fmt-go`
- [ ] Run `make migrate-up`
- [ ] Run `docker compose exec -T api go test ./internal/repository/... ./internal/service/... ./internal/handler/... ./internal/inngest/...`
- [ ] Run `make web-build`
- [ ] Manual check: mark a constrained OpenRouter model as allowed, confirm it becomes selectable in settings
- [ ] Manual check: removed models still cannot be enabled
- [ ] Manual check: after a snapshot refresh, override still applies when the same `model_id` remains present

Plan complete and saved to `docs/superpowers/plans/2026-03-20-openrouter-user-overrides.md`. Ready to execute?
