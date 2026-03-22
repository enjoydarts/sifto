# Poe Provider Implementation Plan

> **For agentic workers:** REQUIRED: Use superpowers:subagent-driven-development (if subagents available) or superpowers:executing-plans to implement this plan. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add Poe as a first-class dynamic LLM provider with persisted model snapshots, a searchable model list UI, background Japanese description translation, OpenRouter-style model change notifications, and automatic OpenAI-compatible vs Anthropic-compatible transport selection based on the chosen model.

**Architecture:** The API owns Poe model sync, persistence, dynamic catalog injection, transport metadata, and notification diffs. The web app reuses the existing settings and OpenRouter-style model-list patterns for Poe-specific key management and searchable model browsing. The worker treats `poe` as one provider, resolves `poe/<model_id>` against the latest snapshot metadata, and automatically routes each request to either Poe's OpenAI-compatible or Anthropic-compatible transport.

**Tech Stack:** Go API, PostgreSQL, existing provider model update tables, Inngest/runtime patterns already used for OpenRouter, Python worker, Next.js App Router, React Query, existing i18n dictionaries, Docker Compose / make verification flow.

---

## File Map

### Persistence and Domain Model

- Modify: `db/migrations/*`
  - Add `poe_api_key_*` columns to `user_settings`
  - Add `poe_model_sync_runs`
  - Add `poe_model_snapshots`
- Modify: `api/internal/model/model.go`
  - Add Poe API key presence / last4 fields to `UserSettings`
- Modify: `api/internal/repository/user_settings.go`
  - Load and persist Poe API key data
- Create: `api/internal/repository/poe_models.go`
  - Persist sync runs, snapshots, translation progress, and history lookup
- Create: `api/internal/repository/poe_models_test.go`
  - Cover snapshot insert/list, diff comparison inputs, and translation progress

### Poe Catalog and Dynamic Model Injection

- Create: `api/internal/service/poe_catalog.go`
  - Fetch `/v1/models`, normalize snapshot rows, determine transport metadata, and convert snapshots into dynamic catalog entries
- Create: `api/internal/service/poe_catalog_test.go`
  - Cover response normalization, transport selection rules, and catalog conversion
- Modify: `api/internal/service/llm_catalog.go`
  - Add dynamic Poe model registration helpers mirroring OpenRouter
- Modify: `api/internal/service/model_provider.go`
  - Include `poe` in provider priority and provider resolution if needed
- Modify: `shared/llm_catalog.json`
  - Register `poe` provider metadata

### Settings and Provider Discovery

- Modify: `api/internal/handler/settings.go`
  - Add Set/Delete Poe API key handlers
- Modify: `api/internal/service/provider_model_discovery.go`
  - Optionally expose Poe model discovery in provider-discovery output
- Modify: `web/src/lib/api.ts`
  - Add Poe API key response types and Poe models endpoints
- Modify: `web/src/app/(main)/settings/page.tsx`
  - Add Poe access card and reuse existing provider-key UX
- Modify: `web/src/i18n/dictionaries/ja.ts`
  - Add Poe settings and Poe models copy
- Modify: `web/src/i18n/dictionaries/en.ts`
  - Add Poe settings and Poe models copy

### Poe Models Sync, Translation, and Notifications

- Create: `api/internal/handler/poe_models.go`
  - Sync/list/status endpoints, OpenRouter-style diff summary integration, and background description translation dispatch
- Create: `api/internal/handler/poe_models_test.go`
  - Cover sync/list/status behavior and stale-run handling
- Modify: `api/internal/handler/provider_model_updates.go`
  - Ensure Poe summaries appear through the shared provider updates system if wiring is needed
- Modify: `api/internal/repository/provider_model_updates.go`
  - Reuse existing OpenRouter-compatible diff insert path for `provider = "poe"` if new helper hooks are needed

### Web Poe Models UI

- Create: `web/src/app/(main)/poe-models/page.tsx`
  - Poe models page modeled after OpenRouter Models
- Modify: `web/src/components/nav.tsx`
  - Link to Poe Models if product wants it in nav
- Reuse/Modify: `web/src/components/settings/provider-model-updates-panel.tsx`
  - Shared component already used; only change if provider labels or filters need adjustment

### Worker Poe Runtime

- Create: `worker/app/services/poe_service.py`
  - Single provider entrypoint that switches between Poe OpenAI-compatible and Anthropic-compatible calls
- Create: `worker/app/services/test_poe_service.py`
  - Cover transport selection, retry/fallback rules, and usage metadata normalization
- Modify: `worker/app/services/llm_catalog.py`
  - Resolve `poe/<model>` aliases and snapshot-backed metadata if needed
- Modify: `worker/app/services/llm_dispatch.py`
  - Route `provider == "poe"` into the new Poe service

### Verification

- Modify/Create as needed: focused API/handler/service tests
- Verification commands must use:
  - `docker compose exec -T api go test ./...`
  - `make web-lint`
  - `make web-build`
  - `make check-worker` or the repo’s current worker syntax check path

## Chunk 1: Persistence and Settings

### Task 1: Add Poe settings storage

**Files:**
- Modify: `db/migrations/*`
- Modify: `api/internal/model/model.go`
- Modify: `api/internal/repository/user_settings.go`
- Modify: `api/internal/handler/settings.go`

- [ ] **Step 1: Write the failing repository/model test**

Add or extend settings tests to assert:
- `HasPoeAPIKey`
- `PoeAPIKeyLast4`
- Set/Delete behavior mirrors OpenRouter

- [ ] **Step 2: Run the failing API repository test**

Run: `docker compose exec -T api go test ./internal/repository ./internal/handler -run Poe -v`
Expected: FAIL because Poe settings fields and handlers do not exist yet.

- [ ] **Step 3: Add the migration**

Add migration(s) that:
- append `poe_api_key_enc`
- append `poe_api_key_last4`

- [ ] **Step 4: Update Go model and repository**

Mirror the existing OpenRouter settings pattern for:
- loading encrypted values
- exposing `has_poe_api_key`
- exposing `poe_api_key_last4`

- [ ] **Step 5: Add settings handlers**

Add `SetPoeAPIKey` and `DeletePoeAPIKey` using the existing generic key helper path.

- [ ] **Step 6: Re-run the targeted tests**

Run: `docker compose exec -T api go test ./internal/repository ./internal/handler -run Poe -v`
Expected: PASS

- [ ] **Step 7: Commit**

```bash
git add db/migrations api/internal/model/model.go api/internal/repository/user_settings.go api/internal/handler/settings.go
git commit -m "Poe APIキー設定を追加"
```

### Task 2: Add Poe settings UI and API bindings

**Files:**
- Modify: `web/src/lib/api.ts`
- Modify: `web/src/app/(main)/settings/page.tsx`
- Modify: `web/src/i18n/dictionaries/ja.ts`
- Modify: `web/src/i18n/dictionaries/en.ts`

- [ ] **Step 1: Add the failing frontend/API types**

Add Poe fields to the settings API typing and page state wiring first so TypeScript fails until the UI is complete.

- [ ] **Step 2: Run the web build to capture the missing bindings**

Run: `make web-build`
Expected: FAIL with missing Poe settings props/handlers.

- [ ] **Step 3: Add Poe key actions to `web/src/lib/api.ts`**

Mirror OpenRouter’s API client methods for:
- save Poe API key
- delete Poe API key

- [ ] **Step 4: Add Poe settings card UI**

Reuse the existing provider access-card structure in settings and provide i18n-backed copy only.

- [ ] **Step 5: Add Japanese and English dictionary entries**

Add Poe-specific labels, descriptions, placeholders, delete confirmation strings, and toast strings.

- [ ] **Step 6: Re-run the web build**

Run: `make web-build`
Expected: PASS

- [ ] **Step 7: Commit**

```bash
git add web/src/lib/api.ts web/src/app/'(main)'/settings/page.tsx web/src/i18n/dictionaries/ja.ts web/src/i18n/dictionaries/en.ts
git commit -m "Poe APIキー設定UIを追加"
```

## Chunk 2: Poe Snapshot Persistence and Catalog Sync

### Task 3: Create Poe sync-run and snapshot repositories

**Files:**
- Modify: `db/migrations/*`
- Create: `api/internal/repository/poe_models.go`
- Create: `api/internal/repository/poe_models_test.go`

- [ ] **Step 1: Write the failing repository tests**

Cover:
- starting/finishing a sync run
- inserting snapshots
- listing latest snapshots
- listing previous successful snapshots
- updating translation counters

- [ ] **Step 2: Run the failing repository test**

Run: `docker compose exec -T api go test ./internal/repository -run TestPoeModelRepo -v`
Expected: FAIL because the repo and tables do not exist yet.

- [ ] **Step 3: Add sync-run and snapshot migrations**

Create:
- `poe_model_sync_runs`
- `poe_model_snapshots`

Include translation progress columns from the start.

- [ ] **Step 4: Implement `PoeModelRepo`**

Expose:
- `StartSyncRun`
- `FinishSyncRun`
- `FailSyncRun`
- `InsertSnapshots`
- `ListLatestSnapshots`
- `ListPreviousSuccessfulSnapshots`
- `GetLatestManualRunningSyncRun`
- `UpdateTranslationProgress`
- `RecordTranslationFailure`
- `UpdateDescriptionsJA`

- [ ] **Step 5: Re-run the repository test**

Run: `docker compose exec -T api go test ./internal/repository -run TestPoeModelRepo -v`
Expected: PASS

- [ ] **Step 6: Commit**

```bash
git add db/migrations api/internal/repository/poe_models.go api/internal/repository/poe_models_test.go
git commit -m "Poeモデル同期の永続化を追加"
```

### Task 4: Implement Poe catalog service and dynamic catalog injection

**Files:**
- Create: `api/internal/service/poe_catalog.go`
- Create: `api/internal/service/poe_catalog_test.go`
- Modify: `api/internal/service/llm_catalog.go`
- Modify: `api/internal/service/model_provider.go`
- Modify: `shared/llm_catalog.json`

- [ ] **Step 1: Write the failing service test**

Cover:
- models API normalization
- `poe/<model>` alias conversion
- transport auto-selection for Claude vs non-Claude models
- pricing mapping into catalog models

- [ ] **Step 2: Run the failing service test**

Run: `docker compose exec -T api go test ./internal/service -run TestPoeCatalog -v`
Expected: FAIL because the service and helpers do not exist yet.

- [ ] **Step 3: Add provider metadata to `shared/llm_catalog.json`**

Register `poe` with empty `default_models` and the correct API key header behavior.

- [ ] **Step 4: Implement `poe_catalog.go`**

Include:
- Poe models fetch client
- snapshot normalizer
- transport rule helper
- dynamic catalog converter

- [ ] **Step 5: Wire dynamic catalog registration**

Add helper(s) parallel to the OpenRouter dynamic model path and ensure provider resolution works for `poe/<model_id>`.

- [ ] **Step 6: Re-run the service test**

Run: `docker compose exec -T api go test ./internal/service -run TestPoeCatalog -v`
Expected: PASS

- [ ] **Step 7: Commit**

```bash
git add api/internal/service/poe_catalog.go api/internal/service/poe_catalog_test.go api/internal/service/llm_catalog.go api/internal/service/model_provider.go shared/llm_catalog.json
git commit -m "Poe動的カタログを追加"
```

## Chunk 3: Poe Sync Handler, Translation, and Change Notifications

### Task 5: Add Poe sync/list/status handlers

**Files:**
- Create: `api/internal/handler/poe_models.go`
- Create: `api/internal/handler/poe_models_test.go`
- Modify: `api/cmd/server/main.go`

- [ ] **Step 1: Write the failing handler tests**

Cover:
- sync endpoint creates a run and stores snapshots
- list endpoint returns latest snapshots and run metadata
- status endpoint returns running sync state
- stale running sync is failed like the OpenRouter flow

- [ ] **Step 2: Run the failing handler test**

Run: `docker compose exec -T api go test ./internal/handler -run TestPoeModels -v`
Expected: FAIL because the handler and route wiring do not exist yet.

- [ ] **Step 3: Implement `poe_models.go`**

Follow the OpenRouter handler shape, but without override-specific logic.

- [ ] **Step 4: Wire the routes in `api/cmd/server/main.go`**

Register:
- `GET /api/poe-models`
- `POST /api/poe-models/sync`
- `GET /api/poe-models/status`

- [ ] **Step 5: Re-run the handler test**

Run: `docker compose exec -T api go test ./internal/handler -run TestPoeModels -v`
Expected: PASS

- [ ] **Step 6: Commit**

```bash
git add api/internal/handler/poe_models.go api/internal/handler/poe_models_test.go api/cmd/server/main.go
git commit -m "Poeモデル同期APIを追加"
```

### Task 6: Add background Japanese description translation

**Files:**
- Modify: `api/internal/service/poe_catalog.go`
- Modify: `api/internal/repository/poe_models.go`
- Modify: `api/internal/handler/poe_models.go`
- Add tests where the repo/service coverage belongs

- [ ] **Step 1: Write the failing translation tests**

Cover:
- descriptions are stored in English immediately
- background translation updates `description_ja`
- unchanged English descriptions are not retranslated
- translation failures increment failure counters without failing the sync

- [ ] **Step 2: Run the failing tests**

Run: `docker compose exec -T api go test ./internal/service ./internal/handler ./internal/repository -run Poe.*Translation -v`
Expected: FAIL because translation wiring does not exist yet.

- [ ] **Step 3: Reuse the OpenRouter translation pattern**

Implement:
- progress counting
- background goroutine dispatch
- cache-aware translation behavior
- per-model update of `description_ja`

- [ ] **Step 4: Add stale-safe progress updates**

Ensure `last_progress_at`, `translation_target_count`, `translation_completed_count`, and `translation_failed_count` stay consistent.

- [ ] **Step 5: Re-run the translation tests**

Run: `docker compose exec -T api go test ./internal/service ./internal/handler ./internal/repository -run Poe.*Translation -v`
Expected: PASS

- [ ] **Step 6: Commit**

```bash
git add api/internal/service/poe_catalog.go api/internal/repository/poe_models.go api/internal/handler/poe_models.go
git commit -m "Poeモデル説明の日本語翻訳を追加"
```

### Task 7: Integrate OpenRouter-style provider model diff notifications

**Files:**
- Modify: `api/internal/handler/poe_models.go`
- Modify: `api/internal/repository/provider_model_updates.go` if helper changes are needed
- Modify: `web/src/components/settings/provider-model-updates-panel.tsx` only if required for provider labeling

- [ ] **Step 1: Write the failing diff-notification test**

Cover:
- current successful Poe snapshots are compared to the previous successful sync
- `added` and `removed` events are inserted using the shared provider update path
- summaries are returned in the same format as OpenRouter

- [ ] **Step 2: Run the failing test**

Run: `docker compose exec -T api go test ./internal/handler ./internal/repository -run Poe.*ProviderModelUpdate -v`
Expected: FAIL because Poe diff insertion is not wired yet.

- [ ] **Step 3: Implement shared-format diff insertion**

Reuse the OpenRouter logic:
- compare last two successful snapshot sets
- insert `provider = "poe"` events
- expose summary on list/sync responses

- [ ] **Step 4: Re-run the tests**

Run: `docker compose exec -T api go test ./internal/handler ./internal/repository -run Poe.*ProviderModelUpdate -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add api/internal/handler/poe_models.go api/internal/repository/provider_model_updates.go web/src/components/settings/provider-model-updates-panel.tsx
git commit -m "Poeモデル差分通知を追加"
```

## Chunk 4: Poe Models Web UI

### Task 8: Add Poe models page

**Files:**
- Create: `web/src/app/(main)/poe-models/page.tsx`
- Modify: `web/src/lib/api.ts`
- Modify: `web/src/components/nav.tsx` if navigation entry is desired
- Modify: `web/src/i18n/dictionaries/ja.ts`
- Modify: `web/src/i18n/dictionaries/en.ts`

- [ ] **Step 1: Add the failing frontend/API typing**

Define Poe models response types including:
- sync run metadata
- translation progress
- snapshot transport fields
- provider change summary

- [ ] **Step 2: Run the web build to capture missing UI pieces**

Run: `make web-build`
Expected: FAIL until the page and i18n strings are present.

- [ ] **Step 3: Build the page using the OpenRouter page as the reference**

Include:
- sync button
- sync progress/status
- translation progress
- search box
- owned-by filter
- preferred transport filter
- list of models with price and descriptions

- [ ] **Step 4: Add navigation and copy**

Expose the page in the same way OpenRouter Models is exposed, unless product explicitly wants it hidden.

- [ ] **Step 5: Re-run the web build**

Run: `make web-build`
Expected: PASS

- [ ] **Step 6: Commit**

```bash
git add web/src/app/'(main)'/poe-models/page.tsx web/src/lib/api.ts web/src/components/nav.tsx web/src/i18n/dictionaries/ja.ts web/src/i18n/dictionaries/en.ts
git commit -m "Poeモデル一覧画面を追加"
```

## Chunk 5: Worker Runtime and Automatic Transport Selection

### Task 9: Add the Poe worker service

**Files:**
- Create: `worker/app/services/poe_service.py`
- Create: `worker/app/services/test_poe_service.py`
- Modify: `worker/app/services/llm_dispatch.py`
- Modify: `worker/app/services/llm_catalog.py` if runtime metadata lookup is needed

- [ ] **Step 1: Write the failing worker tests**

Cover:
- `poe/<model>` resolves to provider `poe`
- Claude official models select Anthropic-compatible transport
- non-Claude models select OpenAI-compatible transport
- unsupported Anthropic-compatible call can fall back once to OpenAI-compatible when allowed
- usage metadata preserves requested and resolved model IDs

- [ ] **Step 2: Run the failing worker test**

Run: `make check-worker`
Expected: FAIL because Poe dispatch and service code do not exist yet.

- [ ] **Step 3: Implement `poe_service.py`**

Build:
- shared Poe auth handling
- OpenAI-compatible request path
- Anthropic-compatible request path
- transport auto-selection helper
- bounded fallback logic

- [ ] **Step 4: Wire worker dispatch**

Route `provider == "poe"` into the new service and ensure alias resolution works.

- [ ] **Step 5: Re-run worker verification**

Run: `make check-worker`
Expected: PASS

- [ ] **Step 6: Commit**

```bash
git add worker/app/services/poe_service.py worker/app/services/test_poe_service.py worker/app/services/llm_dispatch.py worker/app/services/llm_catalog.py
git commit -m "Poe実行transport自動切替を追加"
```

## Chunk 6: End-to-End Verification

### Task 10: Run full verification and document manual checks

**Files:**
- Modify as needed: touched files only

- [ ] **Step 1: Run API tests**

Run: `docker compose exec -T api go test ./...`
Expected: PASS

- [ ] **Step 2: Run web lint**

Run: `make web-lint`
Expected: PASS with no new warnings/errors introduced by this work.

- [ ] **Step 3: Run web build**

Run: `make web-build`
Expected: PASS

- [ ] **Step 4: Run worker checks**

Run: `make check-worker`
Expected: PASS

- [ ] **Step 5: Manual verification**

Confirm:
- Poe API key can be saved and deleted
- Poe sync stores models and updates the list page
- Japanese descriptions appear after background translation completes
- provider model updates show Poe changes in the same panel style as OpenRouter
- selecting a Claude Poe model routes through Anthropic-compatible transport
- selecting a non-Claude Poe model routes through OpenAI-compatible transport

- [ ] **Step 6: Commit**

```bash
git add .
git commit -m "Poeプロバイダを実装"
```

## Suggested Commit Sequence

1. `Poe APIキー設定を追加`
2. `Poe APIキー設定UIを追加`
3. `Poeモデル同期の永続化を追加`
4. `Poe動的カタログを追加`
5. `Poeモデル同期APIを追加`
6. `Poeモデル説明の日本語翻訳を追加`
7. `Poeモデル差分通知を追加`
8. `Poeモデル一覧画面を追加`
9. `Poe実行transport自動切替を追加`
10. `Poeプロバイダを実装`
