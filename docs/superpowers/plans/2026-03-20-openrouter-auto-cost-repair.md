# OpenRouter Auto Cost Repair Implementation Plan

> **For agentic workers:** REQUIRED: Use superpowers:subagent-driven-development (if subagents available) or superpowers:executing-plans to implement this plan. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Prevent `openrouter::auto` cost calculation from breaking when OpenRouter returns dated resolved model names, and provide a safe backfill path for already-corrupted historical usage rows.

**Architecture:** Treat OpenRouter-provided billing cost as the source of truth whenever available, and decouple canonical model resolution from cost accounting. Keep best-effort canonical resolution for analytics, then add an internal backfill endpoint that repairs negative or otherwise broken OpenRouter usage rows, zeroing unresolved historical rows by policy.

**Tech Stack:** Go API, Python worker, Postgres, Docker Compose, unittest, Go testing

---

## Chunk 1: Worker usage capture

### Task 1: Capture OpenRouter billed cost and canonical resolution inputs

**Files:**
- Modify: `worker/app/services/openai_compat_transport.py`
- Modify: `worker/app/services/openrouter_service.py`
- Test: `worker/app/services/test_openai_compat_transport.py`
- Test: `worker/app/services/test_openrouter_service.py`

- [ ] **Step 1: Write the failing worker tests**

Add tests that prove:
- OpenRouter chat response usage can include `usage.cost` and optional generation/request id metadata.
- OpenRouter LLM meta prefers billed cost over local model-derived pricing when billed cost is present.
- OpenRouter LLM meta keeps `requested_model`, `resolved_model`, and a best-effort canonical pricing family when resolved model normalization succeeds.

- [ ] **Step 2: Run worker tests to verify RED**

Run:
```bash
docker compose exec -T worker sh -lc 'cd /app && python -m unittest app.services.test_openai_compat_transport app.services.test_openrouter_service'
```

Expected:
- Failing assertions around missing billed cost / missing OpenRouter usage fields.

- [ ] **Step 3: Implement minimal worker changes**

Implement in `worker/app/services/openai_compat_transport.py`:
- Parse `usage.cost` from OpenRouter-style responses when present.
- Parse request or generation identifiers if exposed in response payload or headers and return them in usage metadata.

Implement in `worker/app/services/openrouter_service.py`:
- Add canonical model normalization helper(s) for OpenRouter resolved model names.
- Use billed cost when present before falling back to local pricing calculation.
- Preserve current token accounting and requested/resolved model tracking.

- [ ] **Step 4: Run worker tests to verify GREEN**

Run:
```bash
docker compose exec -T worker sh -lc 'cd /app && python -m unittest app.services.test_openai_compat_transport app.services.test_openrouter_service app.services.test_task_transport_common'
```

Expected:
- All listed worker tests pass.

- [ ] **Step 5: Run syntax verification**

Run:
```bash
make check-worker
```

Expected:
- Worker Python syntax check passes.

## Chunk 2: API normalization and storage

### Task 2: Store OpenRouter billed cost and stop recomputing it away

**Files:**
- Modify: `api/internal/service/worker.go`
- Modify: `api/internal/model/model.go`
- Modify: `api/internal/repository/llm_usage_logs.go`
- Modify: `api/internal/service/llm_usage_normalize.go`
- Modify: `api/internal/service/llm_usage_normalize_test.go`
- Modify: `api/internal/inngest/functions.go`
- Create: `db/migrations/000077_add_openrouter_billed_cost_to_llm_usage_logs.up.sql`
- Create: `db/migrations/000077_add_openrouter_billed_cost_to_llm_usage_logs.down.sql`

- [ ] **Step 1: Write the failing Go tests**

Add tests that prove:
- `NormalizeCatalogPricedUsage` keeps OpenRouter billed cost unchanged when present.
- OpenRouter resolved model normalization can map dated Anthropic-style resolved names onto canonical OpenRouter alias IDs for analytics.
- Rows without billed cost still use existing catalog-pricing fallback behavior.

- [ ] **Step 2: Run targeted Go tests to verify RED**

Run:
```bash
docker compose exec -T api go test ./internal/service -run 'TestNormalizeCatalogPricedUsage'
```

Expected:
- Failing tests for billed-cost preservation and dated-model normalization.

- [ ] **Step 3: Implement minimal API changes**

Implement:
- New usage fields for billed cost and any request/generation id needed later.
- DB migration and repository insert/read support for the new fields.
- OpenRouter normalization path that prefers billed cost over recomputed catalog pricing.
- Canonical OpenRouter model resolver that can absorb known dated resolved model formats.

- [ ] **Step 4: Run targeted Go tests to verify GREEN**

Run:
```bash
docker compose exec -T api go test ./internal/service -run 'TestNormalizeCatalogPricedUsage'
docker compose exec -T api go test ./internal/inngest -run 'TestLLM'
```

Expected:
- Targeted Go tests pass.

- [ ] **Step 5: Run migration and fast checks**

Run:
```bash
make migrate-up
make migrate-version
make check-fast
```

Expected:
- New migration applies and fast checks pass.

## Chunk 3: Historical repair endpoint

### Task 3: Add internal backfill for corrupted OpenRouter usage rows

**Files:**
- Modify: `api/internal/handler/internal.go`
- Modify: `api/cmd/server/main.go`
- Modify: `api/internal/repository/llm_usage_logs.go`
- Test: `api/internal/service/llm_usage_normalize_test.go`
- Create: `api/internal/repository/llm_usage_logs_backfill_test.go`

- [ ] **Step 1: Write the failing repair tests**

Add tests that prove:
- Backfill target selection picks negative-cost or explicitly broken OpenRouter rows.
- Repair recomputes cost when canonical resolution succeeds.
- Repair zeroes `estimated_cost_usd` when canonical resolution fails, per approved policy.

- [ ] **Step 2: Run targeted Go tests to verify RED**

Run:
```bash
docker compose exec -T api go test ./internal/repository -run 'Test.*LLMUsage.*Backfill'
```

Expected:
- Failing tests because repair selection/update methods do not exist yet.

- [ ] **Step 3: Implement minimal repair flow**

Implement:
- Repository methods to list and update candidate OpenRouter usage rows.
- Internal debug endpoint, following existing backfill endpoint patterns, with `dry_run`, `user_id`, `limit`, and optional date range filters.
- Response payload including matched, repaired, zeroed, unresolved sample rows.

- [ ] **Step 4: Run targeted Go tests to verify GREEN**

Run:
```bash
docker compose exec -T api go test ./internal/repository -run 'Test.*LLMUsage.*Backfill'
docker compose exec -T api go test ./internal/handler -run 'Test.*OpenRouter.*Backfill'
```

Expected:
- Repository and handler tests pass.

- [ ] **Step 5: Run final verification**

Run:
```bash
make fmt-go
make check-fast
docker compose exec -T api go test ./internal/service ./internal/repository ./internal/handler ./internal/inngest
docker compose exec -T worker sh -lc 'cd /app && python -m unittest app.services.test_openai_compat_transport app.services.test_openrouter_service app.services.test_task_transport_common'
```

Expected:
- Formatting is clean and all targeted Go/Python tests pass.
