# Prompt Versioning And A/B Test Implementation Plan

> **For agentic workers:** REQUIRED: Use superpowers:subagent-driven-development (if subagents available) or superpowers:executing-plans to implement this plan. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add global prompt versioning and A/B test infrastructure for `summary`, `facts`, `digest`, and `audio_briefing_script`, with a prompt management UI that is available only to allowlisted users and with runtime defaults still using the current code-defined prompts. Operators must be able to edit the final prompt text directly rather than editing internal fragment placeholders such as `{{task_block}}`.

**Architecture:** The API owns prompt templates, versions, experiments, allowlist-based authorization, prompt resolution, and prompt metadata persistence. The web app adds a limited-access management UI plus capability-aware navigation. The worker remains stateless and only consumes resolved prompt payloads, while existing code-defined prompts stay as the default fallback until an override or experiment is explicitly activated. Both `default_code` and DB-backed versions must resolve through the same final-prompt strategy so that activating a version actually swaps the prompt being sent to the LLM.

**Tech Stack:** Go API, PostgreSQL migrations/repositories/handlers, existing auth/session patterns, Inngest runtime, Python worker, Next.js App Router, React Query, existing i18n dictionaries, Docker Compose / make verification flow.

---

## File Map

### Persistence and Domain Model

- Modify: `db/migrations/*`
  - Add prompt template/version/experiment tables
  - Add prompt metadata columns to existing LLM usage / run tables as needed
- Modify: `api/internal/model/model.go`
  - Add prompt template/version/experiment DTOs and capability fields if model structs are used by handlers
- Create: `api/internal/repository/prompt_templates.go`
  - CRUD for templates, versions, active overrides, experiments, and experiment arms
- Create: `api/internal/repository/prompt_templates_test.go`
  - Repository coverage for insert/list/activate/assignment lookup paths
- Modify: `api/internal/repository/*existing llm usage or run repositories*`
  - Persist resolved prompt metadata on generation runs

### Authorization and Prompt Resolution

- Create: `api/internal/service/prompt_admin_auth.go`
  - Parse `PROMPT_ADMIN_EMAILS` and answer `CanManagePrompts(email string) bool`
- Create: `api/internal/service/prompt_admin_auth_test.go`
  - Cover parsing, trimming, case normalization policy, and matching
- Create: `api/internal/service/prompt_resolver.go`
  - Resolve override / experiment / code fallback for each supported purpose
- Create: `api/internal/service/prompt_resolver_test.go`
  - Cover priority order, stable assignment, and DB failure fallback
- Modify: `api/internal/service/prompt_defaults.go`
  - Return editable final prompt defaults rather than internal fragment-oriented placeholders

### Prompt Management API

- Create: `api/internal/handler/prompt_admin.go`
  - Capabilities endpoint, template list/detail, version create, activate, experiment create/update
- Create: `api/internal/handler/prompt_admin_test.go`
  - Cover allowlist enforcement, response shapes, and validation errors
- Modify: `api/internal/handler/router` files as needed
  - Register prompt admin routes

### Runtime Integration

- Modify: `api/internal/inngest/functions.go`
  - Resolve prompts before dispatching summary/facts/digest/audio jobs
  - Persist prompt metadata with each run
- Modify: `worker/app/services/*task builders and dispatch payload handling*`
  - Accept resolved prompt payloads without changing default behavior
- Create/Modify: `worker/app/services/test_*`
  - Cover resolved prompt payload usage where task-level tests already exist
  - Prove that seeded template versions render equivalently to current hardcoded prompts

### Web Prompt Management UI

- Modify: `web/src/lib/api.ts`
  - Add prompt admin types and API methods
- Create: `web/src/app/(main)/prompt-admin/page.tsx`
  - Prompt template/version/experiment management page
- Modify: `web/src/components/nav.tsx`
  - Show link only when `can_manage_prompts` is true
- Modify: `web/src/lib/server-auth.ts` or nearby capability-loading code
  - Surface the current user email or prompt admin capability to the app layer if needed
- Modify: `web/src/i18n/dictionaries/ja.ts`
  - Add prompt admin copy
- Modify: `web/src/i18n/dictionaries/en.ts`
  - Add prompt admin copy

### Editing UX

- Modify: `web/src/app/(main)/prompt-admin/page.tsx`
  - Make `system_instruction` and `prompt_text` directly editable as full final prompt text
  - Show only real runtime variables in helper UI
  - Keep rendered preview in a modal as a confirmation aid, not as the primary editable source

### Seed and Audit

- Create: `api/internal/service/prompt_seed.go`
  - Seed DB versions from current code-defined prompts
- Create: `api/internal/service/prompt_seed_test.go`
  - Cover seed idempotency and version mapping
- Extend existing audit/logging location or create a focused persistence helper if needed
  - Record who created or activated prompt versions and experiments

### Verification

- Verification commands must use:
  - `docker compose exec -T api go test ./internal/service ./internal/repository ./internal/handler ./internal/inngest`
  - `make web-build`
  - `make web-lint`
  - `make check-fast`
  - `make check-worker`

## Chunk 1: Schema, Auth, and Repository Foundation

### Task 1: Add prompt management schema

**Files:**
- Modify: `db/migrations/*`
- Create: `api/internal/repository/prompt_templates_test.go`

- [ ] **Step 1: Write the failing repository test**

Add repository tests that expect:
- template creation
- version creation
- active override storage
- experiment and arm storage
- deterministic assignment lookup inputs

- [ ] **Step 2: Run the failing repository test**

Run: `docker compose exec -T api go test ./internal/repository -run Prompt -v`
Expected: FAIL because prompt tables and repository do not exist yet.

- [ ] **Step 3: Add the migration**

Create migration(s) for:
- `prompt_templates`
- `prompt_template_versions`
- `prompt_experiments`
- `prompt_experiment_arms`
- prompt metadata columns on existing run/usage tables if needed for traceability

- [ ] **Step 4: Re-run migration status**

Run: `make migrate-version`
Expected: new migration is visible and ordered correctly.

- [ ] **Step 5: Commit**

```bash
git add db/migrations api/internal/repository/prompt_templates_test.go
git commit -m "プロンプト管理基盤のDBスキーマを追加"
```

### Task 2: Implement allowlist authorization service

**Files:**
- Create: `api/internal/service/prompt_admin_auth.go`
- Create: `api/internal/service/prompt_admin_auth_test.go`

- [ ] **Step 1: Write the failing auth service test**

Cover:
- comma-separated email parsing
- surrounding whitespace trimming
- empty entries ignored
- allowed email match
- disallowed email mismatch

- [ ] **Step 2: Run the failing auth service test**

Run: `docker compose exec -T api go test ./internal/service -run PromptAdminAuth -v`
Expected: FAIL because the auth service does not exist yet.

- [ ] **Step 3: Implement the auth service**

Add a small service that:
- reads `PROMPT_ADMIN_EMAILS`
- normalizes configured values
- answers `CanManagePrompts(email string) bool`

- [ ] **Step 4: Re-run the auth service test**

Run: `docker compose exec -T api go test ./internal/service -run PromptAdminAuth -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add api/internal/service/prompt_admin_auth.go api/internal/service/prompt_admin_auth_test.go
git commit -m "プロンプト管理者メール許可リスト判定を追加"
```

### Task 3: Implement prompt template repository

**Files:**
- Create: `api/internal/repository/prompt_templates.go`
- Modify: `api/internal/model/model.go`
- Modify: `api/internal/repository/prompt_templates_test.go`

- [ ] **Step 1: Run the existing failing repository test**

Run: `docker compose exec -T api go test ./internal/repository -run Prompt -v`
Expected: FAIL because repository methods are missing.

- [ ] **Step 2: Implement minimal repository methods**

Add methods for:
- create/list templates
- create/list versions
- activate/deactivate override version
- create/update experiments
- list active experiments and arms

- [ ] **Step 3: Re-run the repository test**

Run: `docker compose exec -T api go test ./internal/repository -run Prompt -v`
Expected: PASS

- [ ] **Step 4: Commit**

```bash
git add api/internal/model/model.go api/internal/repository/prompt_templates.go api/internal/repository/prompt_templates_test.go
git commit -m "プロンプトテンプレート永続化リポジトリを追加"
```

## Chunk 2: Resolver and Seed

### Task 4: Add seed service for current code prompts

**Files:**
- Create: `api/internal/service/prompt_seed.go`
- Create: `api/internal/service/prompt_seed_test.go`

- [ ] **Step 1: Write the failing seed test**

Cover:
- current code prompt mapping for all four purposes
- idempotent re-run behavior
- stable version labeling for seeded defaults
- seeded versions store human-editable final prompt text rather than internal fragment placeholders

- [ ] **Step 2: Run the failing seed test**

Run: `docker compose exec -T api go test ./internal/service -run PromptSeed -v`
Expected: FAIL because the seed service does not exist yet.

- [ ] **Step 3: Implement the seed service**

Map each supported purpose to the existing code-defined prompt builders and insert missing template/version rows only. The seeded version must be directly editable and equivalent to the current hardcoded final prompt.

- [ ] **Step 4: Re-run the seed test**

Run: `docker compose exec -T api go test ./internal/service -run PromptSeed -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add api/internal/service/prompt_seed.go api/internal/service/prompt_seed_test.go
git commit -m "現行コードのプロンプトを初期投入するseed処理を追加"
```

### Task 5: Implement prompt resolver with code fallback

**Files:**
- Create: `api/internal/service/prompt_resolver.go`
- Create: `api/internal/service/prompt_resolver_test.go`

- [ ] **Step 1: Write the failing resolver test**

Cover priority order:
- explicit override version wins
- explicit active experiment arm wins when enabled
- otherwise code fallback wins
- repository/read failure falls back to code prompt
- assignment is stable for the same `item_id`, `digest_id`, or `job_id`
- `default_code` and seeded `template_version` produce equivalent final prompt output for the same sample inputs

- [ ] **Step 2: Run the failing resolver test**

Run: `docker compose exec -T api go test ./internal/service -run PromptResolver -v`
Expected: FAIL because the resolver does not exist yet.

- [ ] **Step 3: Implement the resolver**

Return a resolved payload with:
- `prompt_key`
- `prompt_source`
- `resolved_prompt_text`
- `resolved_system_instruction`
- version / experiment metadata when applicable

- [ ] **Step 4: Re-run the resolver test**

Run: `docker compose exec -T api go test ./internal/service -run PromptResolver -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add api/internal/service/prompt_resolver.go api/internal/service/prompt_resolver_test.go
git commit -m "コードfallback付きのプロンプトresolverを追加"
```

## Chunk 3: Admin API and Auditability

### Task 6: Add prompt admin capabilities endpoint

**Files:**
- Create: `api/internal/handler/prompt_admin.go`
- Create: `api/internal/handler/prompt_admin_test.go`
- Modify: router registration files under `api/internal/handler` or API bootstrap

- [ ] **Step 1: Write the failing handler test**

Cover:
- allowlisted email gets `can_manage_prompts=true`
- non-allowlisted email gets `false`
- unauthenticated request is rejected according to existing auth behavior

- [ ] **Step 2: Run the failing handler test**

Run: `docker compose exec -T api go test ./internal/handler -run PromptAdmin -v`
Expected: FAIL because the handler and route do not exist yet.

- [ ] **Step 3: Implement the capabilities endpoint**

Return:
- `can_manage_prompts`
- optional supported purposes/template keys if convenient for the UI

- [ ] **Step 4: Re-run the handler test**

Run: `docker compose exec -T api go test ./internal/handler -run PromptAdmin -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add api/internal/handler/prompt_admin.go api/internal/handler/prompt_admin_test.go
git commit -m "プロンプト管理機能の権限判定APIを追加"
```

### Task 7: Add prompt admin CRUD API

**Files:**
- Modify: `api/internal/handler/prompt_admin.go`
- Modify: `api/internal/handler/prompt_admin_test.go`
- Modify: route registration files
- Modify or create audit persistence helper files

- [ ] **Step 1: Extend the failing handler test**

Cover:
- list templates
- template detail with versions/experiments
- create version
- activate override
- create/update experiment
- non-allowlisted caller gets `403`

- [ ] **Step 2: Run the failing handler test**

Run: `docker compose exec -T api go test ./internal/handler -run PromptAdmin -v`
Expected: FAIL because CRUD behavior is still missing.

- [ ] **Step 3: Implement the CRUD endpoints**

Keep behavior minimal:
- create versions
- toggle active override
- create/update experiments and arm weights
- return current effective state for the UI

- [ ] **Step 4: Add audit logging**

Record at least:
- operator user id
- operator email
- action type
- target template/version/experiment
- timestamp

- [ ] **Step 5: Re-run the handler test**

Run: `docker compose exec -T api go test ./internal/handler -run PromptAdmin -v`
Expected: PASS

- [ ] **Step 6: Commit**

```bash
git add api/internal/handler/prompt_admin.go api/internal/handler/prompt_admin_test.go
git commit -m "プロンプト管理APIと監査ログを追加"
```

## Chunk 4: Runtime Integration

### Task 8: Persist resolved prompt metadata on generation runs

**Files:**
- Modify: existing LLM usage/run repositories and model structs
- Modify: `api/internal/inngest/functions.go`

- [ ] **Step 1: Write the failing integration-style API test**

Cover one representative flow that asserts:
- resolved prompt metadata is passed through
- the persisted run/usage row stores prompt source and version/experiment identifiers

- [ ] **Step 2: Run the failing integration test**

Run: `docker compose exec -T api go test ./internal/inngest -run Prompt -v`
Expected: FAIL because prompt metadata is not stored yet.

- [ ] **Step 3: Implement prompt metadata persistence**

Thread resolved prompt metadata through the same places that already persist model/provider/usage details.

- [ ] **Step 4: Re-run the integration test**

Run: `docker compose exec -T api go test ./internal/inngest -run Prompt -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add api/internal/inngest/functions.go api/internal/model/model.go api/internal/repository
git commit -m "生成実行ログへプロンプト解決メタデータを保存する"
```

### Task 9: Resolve prompts before dispatching summary, facts, digest, and audio jobs

**Files:**
- Modify: `api/internal/inngest/functions.go`
- Modify: closely related service/helper files if prompt builders live elsewhere

- [ ] **Step 1: Extend the failing Inngest test**

Cover:
- summary flow uses code fallback by default
- facts flow uses code fallback by default
- digest flow can use active override or experiment
- audio flow can use active override or experiment

- [ ] **Step 2: Run the failing Inngest test**

Run: `docker compose exec -T api go test ./internal/inngest -run PromptResolver -v`
Expected: FAIL because the task dispatchers do not call the resolver yet.

- [ ] **Step 3: Integrate the resolver**

For each supported flow:
- compute context id
- resolve prompt
- include resolved payload in downstream task params
- keep current behavior untouched when no override/experiment applies
- use the same final-prompt contract for both `default_code` and DB-backed template versions

- [ ] **Step 4: Re-run the Inngest test**

Run: `docker compose exec -T api go test ./internal/inngest -run PromptResolver -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add api/internal/inngest/functions.go api/internal/service/prompt_resolver.go
git commit -m "主要LLMタスクでプロンプトresolverを利用する"
```

### Task 10: Accept resolved prompt payloads in the worker

**Files:**
- Modify: `worker/app/services/*` files that receive task payloads
- Modify/Create: related worker tests

- [ ] **Step 1: Write the failing worker test**

Cover:
- when resolved prompt payload is present, the worker uses it
- when absent, current behavior still works
- only real runtime variables are rendered into prompt text
- seeded template versions render the same final prompt as `default_code`

- [ ] **Step 2: Run the failing worker test**

Run: `make check-worker`
Expected: FAIL because worker payload handling does not understand resolved prompt fields yet.

- [ ] **Step 3: Implement minimal worker payload support**

Do not move prompt resolution into the worker. Only read resolved fields if provided, and render final prompt text from real runtime variables only.

- [ ] **Step 4: Re-run the worker verification**

Run: `make check-worker`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add worker/app/services
git commit -m "workerで解決済みプロンプトpayloadを扱えるようにする"
```

## Chunk 5: Web UI

### Task 11: Add prompt admin API client and capability loading

**Files:**
- Modify: `web/src/lib/api.ts`
- Modify: `web/src/lib/server-auth.ts` or nearby capability bootstrapping code

- [ ] **Step 1: Add failing TypeScript references**

Introduce prompt admin API types and page expectations so the build fails until methods exist.

- [ ] **Step 2: Run the web build**

Run: `make web-build`
Expected: FAIL with missing prompt admin API bindings.

- [ ] **Step 3: Implement API client methods**

Add methods for:
- fetch capabilities
- list templates
- fetch template detail
- create version
- activate override
- create/update experiment

- [ ] **Step 4: Re-run the web build**

Run: `make web-build`
Expected: PASS or fail later on the missing page component.

- [ ] **Step 5: Commit**

```bash
git add web/src/lib/api.ts web/src/lib/server-auth.ts
git commit -m "プロンプト管理画面向けのAPIクライアントを追加"
```

### Task 12: Build the prompt admin page

**Files:**
- Create: `web/src/app/(main)/prompt-admin/page.tsx`
- Modify: `web/src/i18n/dictionaries/ja.ts`
- Modify: `web/src/i18n/dictionaries/en.ts`

- [ ] **Step 1: Create the page with failing imports/state usage**

Set up the page skeleton with:
- template list
- selected template detail
- version list
- version creation form
- override activate controls
- experiment create/update controls
- direct editing for full `system_instruction` and `prompt_text`

- [ ] **Step 2: Run the web build**

Run: `make web-build`
Expected: FAIL until the page and i18n strings are fully wired.

- [ ] **Step 3: Implement the page**

Keep the first version pragmatic:
- one screen
- no drag-and-drop
- simple textareas/forms
- clear display of current effective state
- rendered preview modal
- variables helper limited to real runtime inputs only

- [ ] **Step 4: Add i18n strings**

Update both:
- `web/src/i18n/dictionaries/ja.ts`
- `web/src/i18n/dictionaries/en.ts`

- [ ] **Step 5: Re-run the web build**

Run: `make web-build`
Expected: PASS

- [ ] **Step 6: Commit**

```bash
git add web/src/app/'(main)'/prompt-admin/page.tsx web/src/i18n/dictionaries/ja.ts web/src/i18n/dictionaries/en.ts
git commit -m "特定ユーザ向けプロンプト管理画面を追加"
```

### Task 13: Gate navigation and page access by capability

**Files:**
- Modify: `web/src/components/nav.tsx`
- Modify: `web/src/app/(main)/prompt-admin/page.tsx`

- [ ] **Step 1: Write the failing gating behavior**

Add or adapt tests if this area already has them. If not, use build-level verification and explicit manual assertions:
- allowed user sees the nav link and page content
- disallowed user does not see the link and cannot use the page

- [ ] **Step 2: Run verification**

Run: `make web-build`
Expected: FAIL or remain incomplete until capability gating is wired.

- [ ] **Step 3: Implement gating**

Hide the nav link unless `can_manage_prompts` is true and block the page with a clear forbidden state or redirect according to existing app patterns.

- [ ] **Step 4: Re-run web checks**

Run:
- `make web-build`
- `make web-lint`

Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add web/src/components/nav.tsx web/src/app/'(main)'/prompt-admin/page.tsx
git commit -m "プロンプト管理画面の表示と導線を権限で制御する"
```

## Chunk 6: Full Verification and Rollout Safety

### Task 14: End-to-end verification

**Files:**
- Modify only if verification reveals issues

- [ ] **Step 1: Run focused API suites**

Run: `docker compose exec -T api go test ./internal/service ./internal/repository ./internal/handler ./internal/inngest`
Expected: PASS

- [ ] **Step 2: Run worker verification**

Run: `make check-worker`
Expected: PASS

- [ ] **Step 3: Run web verification**

Run:
- `make web-build`
- `make web-lint`

Expected: PASS

- [ ] **Step 4: Run fast repo-wide checks**

Run: `make check-fast`
Expected: PASS

- [ ] **Step 5: Manual verification notes**

Verify manually:
- allowlisted user can open prompt admin page
- non-allowlisted user cannot
- no override active means outputs still record `prompt_source=default_code`
- activating an override changes only the intended purpose
- experiment assignment is stable on repeated runs for the same target id

- [ ] **Step 6: Commit**

```bash
git add .
git commit -m "プロンプト管理基盤の検証と仕上げを完了する"
```

## Rollout Notes

- Deploy schema and code together so prompt metadata persistence and prompt admin UI stay in sync.
- Set `PROMPT_ADMIN_EMAILS` before exposing the page in production.
- Seed current prompts before any operator starts creating new versions.
- Leave all overrides disabled initially so production behavior remains unchanged.
- Enable one purpose at a time when testing override or experiment behavior in production.

## Done Criteria

- Prompt templates, versions, and experiments can be stored in DB
- Only allowlisted emails can access prompt management UI and API
- `summary`, `facts`, `digest`, and `audio_briefing_script` all resolve prompts through the API resolver
- Default runtime behavior still uses current code prompts unless an override or experiment is explicitly enabled
- Prompt source/version/experiment metadata is saved with generation runs
- Web build, worker checks, and API tests pass
