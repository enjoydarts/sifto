# Audio Briefing BGM Postprocess Implementation Plan

> **For agentic workers:** REQUIRED: Use superpowers:subagent-driven-development (if subagents available) or superpowers:executing-plans to implement this plan. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 音声ブリーフィングの完成音声に BGM と loudness normalize を追加し、設定画面から一括制御できるようにする

**Architecture:** 既存の `infra/audio-concat` Cloud Run Job を最終後処理ジョブに拡張する。API は設定値と callback メタデータを持ち、Job は concat / BGM mix / loudnorm を担当する。

**Tech Stack:** Go, PostgreSQL migrations, Next.js App Router, Python, ffmpeg, Cloud Run Jobs, R2

---

## Chunk 1: Schema And API

### Task 1: Add schema for BGM settings and job metadata

**Files:**
- Create: `db/migrations/000108_add_audio_briefing_bgm_settings_and_job_metadata.up.sql`
- Create: `db/migrations/000108_add_audio_briefing_bgm_settings_and_job_metadata.down.sql`
- Modify: `api/internal/model/model.go`
- Modify: `api/internal/repository/audio_briefings.go`

- [ ] Step 1: Write failing Go tests for settings / finalize callback fields
- [ ] Step 2: Run the focused tests and verify they fail for missing columns/fields
- [ ] Step 3: Add migration and model/repository fields
- [ ] Step 4: Re-run focused tests and verify they pass

### Task 2: Accept and return BGM settings through API

**Files:**
- Modify: `api/internal/service/settings_service.go`
- Modify: `api/internal/handler/settings.go`
- Modify: `web/src/lib/api.ts`

- [ ] Step 1: Write failing tests for audio briefing settings payload and validation
- [ ] Step 2: Run the focused tests and verify they fail
- [ ] Step 3: Implement minimal payload / validation / request body changes
- [ ] Step 4: Re-run focused tests and verify they pass

## Chunk 2: Concat Runner Contract

### Task 3: Pass BGM settings into concat job runs

**Files:**
- Modify: `api/internal/service/audio_briefing_concat_runner.go`
- Modify: `api/internal/service/audio_briefing_concat.go`
- Modify: `api/internal/service/cloud_run_jobs.go`
- Modify: `api/internal/service/audio_briefing_concat_test.go`

- [ ] Step 1: Write failing tests asserting BGM env/payload is forwarded
- [ ] Step 2: Run the focused tests and verify they fail
- [ ] Step 3: Extend run request and Cloud Run env overrides
- [ ] Step 4: Re-run focused tests and verify they pass

### Task 4: Save `bgm_object_key` from concat callback

**Files:**
- Modify: `api/internal/handler/internal_audio_briefings.go`
- Modify: `api/internal/repository/audio_briefings.go`

- [ ] Step 1: Write failing test for callback body containing `bgm_object_key`
- [ ] Step 2: Run the focused test and verify it fails
- [ ] Step 3: Implement callback parsing and persistence
- [ ] Step 4: Re-run the focused test and verify it passes

## Chunk 3: Audio Postprocess Job

### Task 5: Add BGM selection and mixing logic

**Files:**
- Modify: `infra/audio-concat/app/concat_job.py`
- Modify: `infra/audio-concat/app/local_server.py`
- Modify: `infra/audio-concat/app/test_concat_job.py`
- Modify: `infra/gcp/README.md`

- [ ] Step 1: Write failing Python tests for BGM-enabled and BGM-fallback flows
- [ ] Step 2: Run the focused tests and verify they fail
- [ ] Step 3: Add env parsing, R2 prefix listing, random selection, mix pipeline, fallback behavior
- [ ] Step 4: Re-run the focused tests and verify they pass

### Task 6: Add normalize step to final audio output

**Files:**
- Modify: `infra/audio-concat/app/concat_job.py`
- Modify: `infra/audio-concat/app/test_concat_job.py`

- [ ] Step 1: Write failing Python test that expects normalize to be invoked for final output
- [ ] Step 2: Run the focused test and verify it fails
- [ ] Step 3: Implement final normalize step in the ffmpeg pipeline
- [ ] Step 4: Re-run the focused test and verify it passes

## Chunk 4: Settings UI

### Task 7: Expose BGM controls in settings page

**Files:**
- Modify: `web/src/app/(main)/settings/page.tsx`
- Modify: `web/src/i18n/dictionaries/ja.ts`
- Modify: `web/src/i18n/dictionaries/en.ts`

- [ ] Step 1: Add form state and submit payload for `bgm_enabled` / `bgm_r2_prefix`
- [ ] Step 2: Add localized labels/help text in both dictionaries
- [ ] Step 3: Render controls in the audio briefing summary section
- [ ] Step 4: Run web build and verify the page compiles

## Chunk 5: Verification

### Task 8: End-to-end verification

**Files:**
- Modify as needed from previous tasks only

- [ ] Step 1: Run `make fmt-go`
- [ ] Step 2: Run focused Go tests for `api/internal/service`, `api/internal/repository`, `api/internal/handler`
- [ ] Step 3: Run `make migrate-up` and `make migrate-version`
- [ ] Step 4: Run Python tests for `infra/audio-concat/app/test_concat_job.py`
- [ ] Step 5: Run `make web-build`
