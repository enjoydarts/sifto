# Audio Briefing IA Move Implementation Plan

> **For agentic workers:** REQUIRED: Use superpowers:subagent-driven-development (if subagents available) or superpowers:executing-plans to implement this plan. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 生成から30日以上経過した音声ブリーフィング実ファイルを standard bucket から IA bucket へ移し、移動後も詳細画面の再生と削除を透過的に維持する。

**Architecture:** `audio_briefing_jobs` と `audio_briefing_script_chunks` に保存先 bucket を持たせ、生成時は standard bucket を保存する。worker は bucket-aware な upload / presign / delete / copy を提供し、API は job/chunk の bucket を使って再生 URL 発行と削除を行う。定期移送は Inngest cron で published job を走査し、`copy -> DB update -> source delete` の順で処理する。

**Tech Stack:** Go, PostgreSQL migrations, Inngest, FastAPI worker, boto3/R2, Next.js App Router, existing audio briefing pipeline

---

## File Map

### DB / model / repository

- Create: `db/migrations/000093_add_audio_briefing_storage_bucket_columns.up.sql`
- Create: `db/migrations/000093_add_audio_briefing_storage_bucket_columns.down.sql`
- Modify: `api/internal/model/model.go`
  - `AudioBriefingJob`
  - `AudioBriefingScriptChunk`
- Modify: `api/internal/repository/audio_briefings.go`
  - job/chunk scan
  - create/update paths
  - IA move candidate query
  - bucket update methods
- Create: `api/internal/repository/audio_briefings_archive_test.go`

### API services / handlers / scheduling

- Modify: `api/internal/service/worker.go`
  - bucket-aware presign / delete / copy client
- Modify: `api/internal/handler/audio_briefings.go`
  - detail の audio URL 発行を bucket-aware 化
- Create: `api/internal/service/audio_briefing_archive.go`
  - IA move orchestration
- Create: `api/internal/service/audio_briefing_archive_test.go`
- Modify: `api/internal/inngest/functions.go`
  - IA move cron function を追加
- Modify: `api/cmd/server/main.go`
  - archive service / inngest wiring

### Worker

- Modify: `worker/app/services/audio_briefing_tts.py`
  - bucket override upload/presign/delete
  - copy_objects
- Modify: `worker/app/routers/audio_briefing_tts.py`
  - new request/response models
  - copy endpoint
- Modify: `worker/app/services/test_audio_briefing_tts.py`

### Infra / env docs

- Modify: `.env.example`
- Modify: `infra/gcp/audio-concat-job.env.example`
- Modify: `infra/gcp/README.md`

### Verification

- Test: `make fmt-go`
- Test: `docker compose exec -T api go test ./...`
- Test: `make check-worker`
- Test: `make migrate-up`
- Test: `make migrate-version`

---

## Chunk 1: Persist Storage Bucket In DB

### Task 1: Add bucket columns

**Files:**
- Create: `db/migrations/000093_add_audio_briefing_storage_bucket_columns.up.sql`
- Create: `db/migrations/000093_add_audio_briefing_storage_bucket_columns.down.sql`

- [ ] **Step 1: Write the failing migration expectation in a repository test**

Add a new test file skeleton in `api/internal/repository/audio_briefings_archive_test.go` that assumes `AudioBriefingJob` / `AudioBriefingScriptChunk` can round-trip `r2_storage_bucket`.

- [ ] **Step 2: Run the focused repository test to confirm it fails**

Run: `docker compose exec -T api go test ./internal/repository -run 'TestAudioBriefing.*StorageBucket'`
Expected: FAIL because model/repository fields do not exist yet

- [ ] **Step 3: Create the migration**

Up migration:

```sql
ALTER TABLE audio_briefing_jobs
  ADD COLUMN IF NOT EXISTS r2_storage_bucket TEXT NOT NULL DEFAULT '';

ALTER TABLE audio_briefing_script_chunks
  ADD COLUMN IF NOT EXISTS r2_storage_bucket TEXT NOT NULL DEFAULT '';
```

Down migration:

```sql
ALTER TABLE audio_briefing_script_chunks
  DROP COLUMN IF EXISTS r2_storage_bucket;

ALTER TABLE audio_briefing_jobs
  DROP COLUMN IF EXISTS r2_storage_bucket;
```

- [ ] **Step 4: Apply migration**

Run: `make migrate-up`
Expected: migration `93/u` applies successfully

- [ ] **Step 5: Verify version**

Run: `make migrate-version`
Expected: `93`

- [ ] **Step 6: Commit**

```bash
git add db/migrations/000093_add_audio_briefing_storage_bucket_columns.*
git commit -m "音声ブリーフィングの保存先bucket列を追加"
```

### Task 2: Add model and scan support

**Files:**
- Modify: `api/internal/model/model.go`
- Modify: `api/internal/repository/audio_briefings.go`
- Test: `api/internal/repository/audio_briefings_archive_test.go`

- [ ] **Step 1: Extend the Go models**

Add:

```go
R2StorageBucket string `json:"r2_storage_bucket"`
```

to both:

- `AudioBriefingJob`
- `AudioBriefingScriptChunk`

- [ ] **Step 2: Update repository scans**

Include `r2_storage_bucket` in:

- `ListJobsByUser`
- `GetJobByID`
- `GetJobBySlotKey`
- `ListJobChunks`
- `scanAudioBriefingJob`

- [ ] **Step 3: Update insert/update paths**

Populate `r2_storage_bucket` for:

- initial job creation
- script chunk creation
- concat publish update path

with `standard bucket fallback` resolution.

- [ ] **Step 4: Add focused repository tests**

Test:

- empty DB value falls back to standard bucket in service layer later
- persisted value is scanned correctly

- [ ] **Step 5: Run repository tests**

Run: `docker compose exec -T api go test ./internal/repository -run 'TestAudioBriefing.*StorageBucket'`
Expected: PASS

- [ ] **Step 6: Commit**

```bash
git add api/internal/model/model.go api/internal/repository/audio_briefings.go api/internal/repository/audio_briefings_archive_test.go
git commit -m "音声ブリーフィングの保存先bucketをモデルとrepositoryに追加"
```

---

## Chunk 2: Make Worker Bucket-Aware

### Task 3: Add bucket-aware R2 operations in worker

**Files:**
- Modify: `worker/app/services/audio_briefing_tts.py`
- Modify: `worker/app/services/test_audio_briefing_tts.py`

- [ ] **Step 1: Write the failing worker tests**

Add tests for:

- `presign_audio_url(..., bucket_override=...)`
- `delete_objects([{bucket,key}, ...])`
- `copy_objects(source_bucket, target_bucket, keys)`

- [ ] **Step 2: Run the focused worker tests to verify they fail**

Run: `docker compose exec -T worker python -m unittest app.services.test_audio_briefing_tts`
Expected: FAIL on missing method signatures / bucket override behavior

- [ ] **Step 3: Implement standard/IA bucket helpers**

Add helpers:

```python
def standard_bucket(self) -> str: ...
def ia_bucket(self) -> str: ...
def resolve_bucket(self, bucket_override: str | None = None) -> str: ...
```

using:

- `AUDIO_BRIEFING_R2_STANDARD_BUCKET`
- fallback `AUDIO_BRIEFING_R2_BUCKET`
- `AUDIO_BRIEFING_R2_IA_BUCKET`

- [ ] **Step 4: Make presign/delete bucket-aware**

Support explicit bucket in:

- `presign_audio_url`
- `delete_objects`

- [ ] **Step 5: Add copy operation**

Implement:

```python
def copy_objects(self, source_bucket: str, target_bucket: str, object_keys: list[str]) -> int:
```

using S3 `copy_object`.

- [ ] **Step 6: Run worker tests**

Run: `docker compose exec -T worker python -m unittest app.services.test_audio_briefing_tts`
Expected: PASS

- [ ] **Step 7: Commit**

```bash
git add worker/app/services/audio_briefing_tts.py worker/app/services/test_audio_briefing_tts.py
git commit -m "音声ブリーフィングworkerのR2操作をbucket対応にする"
```

### Task 4: Expose worker endpoints

**Files:**
- Modify: `worker/app/routers/audio_briefing_tts.py`

- [ ] **Step 1: Write failing API-shape tests if router tests exist; otherwise use service-only verification**

If no router tests exist, document this and proceed with implementation plus `make check-worker`.

- [ ] **Step 2: Add request/response models**

Add endpoints for:

- bucket-aware presign
- bucket-aware delete
- object copy

- [ ] **Step 3: Wire router to service**

Ensure request payloads accept:

- `bucket`
- `source_bucket`
- `target_bucket`
- `object_keys`

- [ ] **Step 4: Run syntax verification**

Run: `make check-worker`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add worker/app/routers/audio_briefing_tts.py
git commit -m "音声ブリーフィングworkerのbucket操作APIを追加"
```

---

## Chunk 3: Make API Read/Delete Paths Respect Bucket

### Task 5: Extend worker client

**Files:**
- Modify: `api/internal/service/worker.go`
- Test: `api/internal/service/audio_briefing_archive_test.go`

- [ ] **Step 1: Write failing Go tests**

Add tests that expect `WorkerClient` request bodies to include bucket fields for:

- presign
- delete
- copy

- [ ] **Step 2: Run focused service tests to verify failure**

Run: `docker compose exec -T api go test ./internal/service -run 'TestAudioBriefing.*Bucket'`
Expected: FAIL due to missing methods / payload fields

- [ ] **Step 3: Implement bucket-aware client methods**

Add:

- `PresignAudioBriefingObjectInBucket`
- `DeleteAudioBriefingObjectsInBuckets`
- `CopyAudioBriefingObjects`

- [ ] **Step 4: Keep compatibility wrappers**

Preserve existing methods as wrappers using standard bucket fallback where possible.

- [ ] **Step 5: Re-run focused tests**

Run: `docker compose exec -T api go test ./internal/service -run 'TestAudioBriefing.*Bucket'`
Expected: PASS

- [ ] **Step 6: Commit**

```bash
git add api/internal/service/worker.go api/internal/service/audio_briefing_archive_test.go
git commit -m "音声ブリーフィングAPIのworker clientをbucket対応にする"
```

### Task 6: Update detail and delete flows

**Files:**
- Modify: `api/internal/handler/audio_briefings.go`
- Modify: `api/internal/service/audio_briefing_delete.go`
- Modify: `api/internal/repository/audio_briefings.go`
- Test: `api/internal/service/audio_briefing_delete_test.go`

- [ ] **Step 1: Write failing delete-flow tests**

Add tests for:

- delete uses per-object bucket
- detail presign uses `job.r2_storage_bucket`
- empty bucket falls back to standard bucket

- [ ] **Step 2: Run focused tests to verify they fail**

Run: `docker compose exec -T api go test ./internal/service -run 'TestAudioBriefingDeleteService|TestAudioBriefing.*Presign'`
Expected: FAIL

- [ ] **Step 3: Update playable URL resolution**

`resolvePlayableAudioURL` should read:

- `job.R2StorageBucket`
- fallback to standard bucket env if empty

- [ ] **Step 4: Update delete service**

Build delete payloads as `(bucket, object_key)` pairs for:

- episode audio
- manifest
- chunks

- [ ] **Step 5: Re-run focused tests**

Run: `docker compose exec -T api go test ./internal/service -run 'TestAudioBriefingDeleteService|TestAudioBriefing.*Presign'`
Expected: PASS

- [ ] **Step 6: Commit**

```bash
git add api/internal/handler/audio_briefings.go api/internal/service/audio_briefing_delete.go api/internal/service/audio_briefing_delete_test.go api/internal/repository/audio_briefings.go
git commit -m "音声ブリーフィングの再生URLと削除をbucket対応にする"
```

---

## Chunk 4: Add IA Move Batch

### Task 7: Add archive candidate query and bucket update methods

**Files:**
- Modify: `api/internal/repository/audio_briefings.go`
- Modify: `api/internal/repository/audio_briefings_archive_test.go`

- [ ] **Step 1: Write failing repository tests**

Test methods for:

- `ListIAMoveCandidates(before, limit)`
- `UpdateStorageBucketForJobAndChunks(jobID, bucket)`

- [ ] **Step 2: Run repository tests to confirm failure**

Run: `docker compose exec -T api go test ./internal/repository -run 'TestAudioBriefing.*Archive'`
Expected: FAIL

- [ ] **Step 3: Implement candidate query**

Select jobs where:

- `status = 'published'`
- `published_at < cutoff`
- `COALESCE(r2_storage_bucket, '') IN ('', standard_bucket)`

- [ ] **Step 4: Implement atomic bucket update**

Inside one transaction:

- update job bucket
- update all chunk buckets

- [ ] **Step 5: Re-run repository tests**

Run: `docker compose exec -T api go test ./internal/repository -run 'TestAudioBriefing.*Archive'`
Expected: PASS

- [ ] **Step 6: Commit**

```bash
git add api/internal/repository/audio_briefings.go api/internal/repository/audio_briefings_archive_test.go
git commit -m "音声ブリーフィングIA移送用のrepository操作を追加"
```

### Task 8: Implement archive orchestration service

**Files:**
- Create: `api/internal/service/audio_briefing_archive.go`
- Create: `api/internal/service/audio_briefing_archive_test.go`

- [ ] **Step 1: Write failing service tests**

Cover:

- `copy -> DB update -> source delete` happy path
- copy failure stops before DB update
- DB update failure stops before source delete
- source delete failure leaves DB updated and returns error/loggable state

- [ ] **Step 2: Run focused service tests to verify failure**

Run: `docker compose exec -T api go test ./internal/service -run 'TestAudioBriefingArchive'`
Expected: FAIL

- [ ] **Step 3: Implement service**

Core method:

```go
func (s *AudioBriefingArchiveService) MovePublishedToIA(ctx context.Context) error
```

with env:

- `AUDIO_BRIEFING_IA_MOVE_AFTER_DAYS`
- `AUDIO_BRIEFING_IA_MOVE_BATCH_LIMIT`

- [ ] **Step 4: Re-run focused service tests**

Run: `docker compose exec -T api go test ./internal/service -run 'TestAudioBriefingArchive'`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add api/internal/service/audio_briefing_archive.go api/internal/service/audio_briefing_archive_test.go
git commit -m "音声ブリーフィングのIA移送serviceを追加"
```

### Task 9: Schedule archive batch

**Files:**
- Modify: `api/internal/inngest/functions.go`
- Modify: `api/cmd/server/main.go`

- [ ] **Step 1: Write a failing inngest/service wiring test if coverage exists**

If direct test coverage is thin, add a focused service constructor/wiring test instead.

- [ ] **Step 2: Add a cron function**

Example cadence:

```go
inngestgo.CronTrigger("17 3 * * *")
```

The function should call `MovePublishedToIA`.

- [ ] **Step 3: Wire the service in server startup**

Create the archive service with:

- audio briefing repo
- worker client
- env-derived standard/IA bucket config

- [ ] **Step 4: Run inngest + service tests**

Run: `docker compose exec -T api go test ./internal/inngest ./internal/service`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add api/internal/inngest/functions.go api/cmd/server/main.go
git commit -m "音声ブリーフィングIA移送の定期実行を追加"
```

---

## Chunk 5: Env And Full Verification

### Task 10: Update env examples and docs

**Files:**
- Modify: `.env.example`
- Modify: `infra/gcp/audio-concat-job.env.example`
- Modify: `infra/gcp/README.md`

- [ ] **Step 1: Add new env keys**

Document:

- `AUDIO_BRIEFING_R2_STANDARD_BUCKET`
- `AUDIO_BRIEFING_R2_IA_BUCKET`
- `AUDIO_BRIEFING_IA_MOVE_AFTER_DAYS`
- `AUDIO_BRIEFING_IA_MOVE_BATCH_LIMIT`

- [ ] **Step 2: Note fallback behavior**

Clarify that:

- `AUDIO_BRIEFING_R2_BUCKET` is legacy fallback for standard bucket

- [ ] **Step 3: Commit**

```bash
git add .env.example infra/gcp/audio-concat-job.env.example infra/gcp/README.md
git commit -m "音声ブリーフィングIA移送の環境変数を文書化"
```

### Task 11: Run full verification

**Files:**
- Verify all modified files above

- [ ] **Step 1: Run Go formatting**

Run: `make fmt-go`
Expected: no diff after formatting

- [ ] **Step 2: Run full API tests**

Run: `docker compose exec -T api go test ./...`
Expected: PASS

- [ ] **Step 3: Run worker checks**

Run: `make check-worker`
Expected: PASS

- [ ] **Step 4: Run migration verification**

Run: `make migrate-up`
Expected: latest migration applied successfully

Run: `make migrate-version`
Expected: `93`

- [ ] **Step 5: Commit verification-safe cleanup**

```bash
git add .
git commit -m "音声ブリーフィングIA移送の実装を仕上げる"
```

---

## Notes For Execution

- `audio_briefing_jobs` と `audio_briefing_script_chunks` の `r2_storage_bucket` が空文字の既存行は、API / worker で standard bucket fallback に寄せる
- source delete failure は「再生不能」より優先度が低い。DB を先に巻き戻さない
- `shared/ai_navigator_personas.json` はこの作業に無関係なので触らない
