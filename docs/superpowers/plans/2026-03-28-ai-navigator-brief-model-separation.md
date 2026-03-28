# AIナビゲーターとAIナビブリーフのモデル分離 Implementation Plan

> **For agentic workers:** REQUIRED: Use superpowers:subagent-driven-development (if subagents available) or superpowers:executing-plans to implement this plan. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** AIナビゲーターとAIナビブリーフで primary / fallback の使用モデルを完全に分離する。

**Architecture:** 既存の `llm_models` 保存構造に brief 専用キーを 2 つ追加し、settings API と web settings UI を拡張する。AIナビブリーフ生成側の model resolve は `navigator` 系ではなく brief 専用キーだけを見るように差し替える。

**Tech Stack:** Go, Next.js App Router, PostgreSQL repository layer, make/docker compose

---

## Chunk 1: API Settings Surface

### Task 1: settings payload の failing test を追加

**Files:**
- Modify: `api/internal/service/settings_service_test.go`

- [ ] `LLMModelSettingsPayload` に `ai_navigator_brief` / `ai_navigator_brief_fallback` を期待する failing test を追加
- [ ] `docker compose exec -T api go test ./internal/service -run TestLLMModelSettingsPayloadIncludesFallbackModels -count=1` で RED を確認

### Task 2: settings model / service を最小実装

**Files:**
- Modify: `api/internal/model/model.go`
- Modify: `api/internal/service/settings_service.go`

- [ ] `UserSettings` と `SettingsGetPayload` 系の反映に必要なフィールドを追加
- [ ] `UpdateLLMModelsInput`、`modelSettingPurposes`、`modelSettingRequiredCapabilities`、`LLMModelSettingsPayload`、`UpdateLLMModels` の正規化対象を更新
- [ ] 同じテストを再実行して GREEN を確認

## Chunk 2: Repository / Handler Wiring

### Task 3: repository / handler の failing test を追加

**Files:**
- Modify: `api/internal/service/settings_service_test.go`
- Modify: `api/internal/handler/settings.go`

- [ ] brief 用 2 キーが update payload に通ることを検証する failing test を追加
- [ ] 該当テストだけ実行して RED を確認

### Task 4: repository / handler を実装

**Files:**
- Modify: `api/internal/repository/user_settings.go`
- Modify: `api/internal/handler/settings.go`

- [ ] `SELECT` / `Scan` / `UpsertLLMModelConfig` に brief 用 2 カラムを追加
- [ ] settings handler request struct と service 呼び出し配線を更新
- [ ] テストを再実行して GREEN を確認

## Chunk 3: AIナビブリーフ model resolve

### Task 5: failing test を追加

**Files:**
- Modify: `api/internal/service/settings_service_test.go` or create focused test file if needed
- Modify: `api/internal/service/ai_navigator_briefs.go`

- [ ] brief resolve が `navigator` ではなく `ai_navigator_brief` を優先する failing test を追加
- [ ] brief fallback も brief 専用キーを見る failing test を追加
- [ ] 該当テストを実行して RED を確認

### Task 6: resolve 実装を切り替える

**Files:**
- Modify: `api/internal/service/ai_navigator_briefs.go`

- [ ] `resolveAINavigatorBriefModel(...)` と関連 helper を brief 専用キー参照へ変更
- [ ] テストを再実行して GREEN を確認

## Chunk 4: Web Settings UI

### Task 7: web 型 / state 反映

**Files:**
- Modify: `web/src/lib/api.ts`
- Modify: `web/src/app/(main)/settings/page.tsx`

- [ ] API 型に brief 用 2 キーを追加
- [ ] settings state / load / save payload に brief 用 2 キーを追加

### Task 8: settings UI を追加

**Files:**
- Modify: `web/src/app/(main)/settings/page.tsx`
- Modify: `web/src/i18n/dictionaries/ja.ts`
- Modify: `web/src/i18n/dictionaries/en.ts`

- [ ] AIナビゲーター settings セクション内に brief 用 model select 2 つを追加
- [ ] i18n 文言を追加
- [ ] `make web-lint`
- [ ] `make web-build`

## Chunk 5: Verification

### Task 9: 全体確認

**Files:**
- Modify: none

- [ ] `make fmt-go`
- [ ] `docker compose exec -T api go test ./internal/service ./internal/handler ./internal/repository ./cmd/server`
- [ ] `make web-lint`
- [ ] `make web-build`
- [ ] `git status --short` で差分確認
