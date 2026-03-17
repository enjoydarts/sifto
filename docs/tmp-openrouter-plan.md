# OpenRouter追加実装計画

> **For agentic workers:** REQUIRED: Use superpowers:subagent-driven-development (if subagents available) or superpowers:executing-plans to implement this plan. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** OpenRouter を独立 provider として追加し、DB スナップショット型の動的モデル棚、日次同期、手動更新、通知、専用画面まで含めて全用途で使えるようにする

**Architecture:** API 側が OpenRouter models API を同期して DB に最新スナップショットを保存し、settings / catalog / 通知 / 専用画面はそのスナップショットを参照する。worker 側は stateless を維持し、OpenRouter の推論実行だけを担当する。既存の静的 catalog は残しつつ、OpenRouter だけ動的 provider として合成表示する。

**Tech Stack:** Go API, PostgreSQL, Inngest, Python worker, Next.js App Router, OneSignal, Resend

---

## File Structure

### API / DB

- Create: `db/migrations/000067_add_openrouter_api_key_and_model_snapshots.up.sql`
- Create: `db/migrations/000067_add_openrouter_api_key_and_model_snapshots.down.sql`
- Modify: `api/internal/model/model.go`
- Modify: `api/internal/repository/user_settings.go`
- Create: `api/internal/repository/openrouter_models.go`
- Create: `api/internal/service/openrouter_catalog.go`
- Modify: `api/internal/service/settings_service.go`
- Modify: `api/internal/handler/settings.go`
- Modify: `api/internal/service/llm_catalog.go`
- Modify: `api/internal/service/llm_catalog_test.go`
- Modify: `api/internal/service/model_provider.go`
- Modify: `api/internal/handler/ask.go`
- Modify: `api/internal/handler/sources.go`
- Modify: `api/internal/service/worker.go`

### Inngest / Notifications

- Modify: `api/internal/inngest/functions.go`
- Create: `api/internal/repository/openrouter_model_notification_logs.go`
- Modify: `api/internal/service/resend.go`
- Modify: `api/internal/service/onesignal.go` (only if payload helper needed)

### Worker

- Create: `worker/app/services/openrouter_service.py`
- Modify: `worker/app/routers/facts.py`
- Modify: `worker/app/routers/summarize.py`
- Modify: `worker/app/routers/digest.py`
- Modify: `worker/app/routers/ask.py`
- Modify: `worker/app/routers/facts_check.py`
- Modify: `worker/app/routers/summary_faithfulness.py`
- Modify: `worker/app/routers/feed_seed_suggestions.py`
- Modify: `worker/app/routers/feed_suggestions.py`
- Modify: `worker/app/routers/translate_title.py`

### Web

- Modify: `web/src/lib/api.ts`
- Modify: `web/src/app/(main)/settings/page.tsx`
- Modify: `web/src/components/settings/model-select.tsx`
- Create: `web/src/app/(main)/openrouter-models/page.tsx`
- Modify: `web/src/components/nav.tsx`
- Modify: `web/src/app/(main)/llm-usage/page.tsx`
- Modify: `web/src/app/(main)/llm-analysis/page.tsx`
- Modify: `web/src/components/llm-usage/tables.tsx`
- Modify: `web/src/components/llm-usage/value-metrics-panel.tsx`
- Modify: `web/src/i18n/dictionaries/ja.ts`
- Modify: `web/src/i18n/dictionaries/en.ts`

## Chunk 1: DB と API key / snapshot 基盤

### Task 1: migration で OpenRouter 用カラムと snapshot テーブルを追加

**Files:**
- Create: `db/migrations/000067_add_openrouter_api_key_and_model_snapshots.up.sql`
- Create: `db/migrations/000067_add_openrouter_api_key_and_model_snapshots.down.sql`
- Test: `make migrate-up`

- [ ] **Step 1: migration の up SQL を作成**

追加対象:
- `user_settings.openrouter_api_key_enc`
- `user_settings.openrouter_api_key_last4`
- `openrouter_model_snapshots`
- `openrouter_model_sync_runs`
- `openrouter_model_notification_logs`

- [ ] **Step 2: migration の down SQL を作成**

削除対象を up の逆順で定義する。

- [ ] **Step 3: migration を適用**

Run: `make migrate-up`
Expected: migration version が 67 になる

- [ ] **Step 4: migration version を確認**

Run: `make migrate-version`
Expected: `67`

- [ ] **Step 5: Commit**

```bash
git add db/migrations
git commit -m "OpenRouter用のAPIキーとモデルスナップショット基盤を追加"
```

### Task 2: OpenRouter API key を settings に通す

**Files:**
- Modify: `api/internal/model/model.go`
- Modify: `api/internal/repository/user_settings.go`
- Modify: `api/internal/service/settings_service.go`
- Modify: `api/internal/handler/settings.go`
- Modify: `web/src/lib/api.ts`

- [ ] **Step 1: model.UserSettings に OpenRouter API key 状態を追加**

Z.ai と同じパターンで:
- `HasOpenRouterAPIKey`
- `OpenRouterAPIKeyLast4`

- [ ] **Step 2: repository に OpenRouter API key CRUD を追加**

Z.ai の以下を雛形にする:
- get encrypted
- set
- delete

- [ ] **Step 3: settings service payload に OpenRouter を追加**

`SettingsGetPayload` に
- `has_openrouter_api_key`
- `openrouter_api_key_last4`
を追加する。

- [ ] **Step 4: settings handler に endpoint を追加**

追加:
- `POST /settings/openrouter-key`
- `DELETE /settings/openrouter-key`

- [ ] **Step 5: web API 型と client を追加**

`web/src/lib/api.ts` に:
- settings payload フィールド
- `setOpenRouterApiKey`
- `deleteOpenRouterApiKey`

- [ ] **Step 6: API テストを通す**

Run: `docker compose exec -T api go test ./internal/handler/... ./internal/service/... ./internal/repository/...`
Expected: PASS

- [ ] **Step 7: Commit**

```bash
git add api/internal/model/model.go api/internal/repository/user_settings.go api/internal/service/settings_service.go api/internal/handler/settings.go web/src/lib/api.ts
git commit -m "OpenRouter APIキー設定をユーザー別に追加"
```

## Chunk 2: OpenRouter snapshot 同期と通知

### Task 3: OpenRouter models API client と snapshot repository を追加

**Files:**
- Create: `api/internal/repository/openrouter_models.go`
- Create: `api/internal/service/openrouter_catalog.go`
- Modify: `api/internal/service/llm_catalog.go`

- [ ] **Step 1: OpenRouter snapshot repository を作成**

責務:
- sync run の開始/完了記録
- snapshot batch 保存
- 最新 snapshot 取得
- 前回 snapshot との差分取得

- [ ] **Step 2: OpenRouter models API client を作成**

責務:
- OpenRouter models API 呼び出し
- テキスト用途モデルのフィルタ
- provider slug / pricing / supported parameters / description の正規化

- [ ] **Step 3: description の日本語機械翻訳方針を実装**

最初は API 側で既存 worker translation を使わず、軽い LLM 翻訳呼び出し方針を決める。
ここは最小実装として:
- 英語 description を保存
- 日本語は同期待ちに翻訳実行
- 失敗時は英語 fallback

- [ ] **Step 4: llm catalog 合成読み出しを追加**

`LLMCatalogData()` 相当の静的 catalog に OpenRouter 最新 snapshot 群を合成する read path を追加する。

- [ ] **Step 5: API テストを追加**

追加する観点:
- snapshot insert / latest read
- filter が embedding-only 等を除外する
- dynamic models が llm catalog 合成に入る

- [ ] **Step 6: テスト実行**

Run: `docker compose exec -T api go test ./internal/service/... ./internal/repository/...`
Expected: PASS

- [ ] **Step 7: Commit**

```bash
git add api/internal/repository/openrouter_models.go api/internal/service/openrouter_catalog.go api/internal/service/llm_catalog.go api/internal/service/llm_catalog_test.go
git commit -m "OpenRouterモデルの動的スナップショット取得とcatalog合成を追加"
```

### Task 4: 日次同期 Inngest と新規モデル通知を追加

**Files:**
- Modify: `api/internal/inngest/functions.go`
- Create: `api/internal/repository/openrouter_model_notification_logs.go`
- Modify: `api/internal/service/resend.go`

- [ ] **Step 1: OpenRouter 同期 Inngest function を追加**

責務:
- models API 取得
- snapshot 保存
- 追加モデル差分の抽出

- [ ] **Step 2: 手動同期用 service 呼び出しを共通化**

日次同期と手動同期で同じ同期ロジックを使う。

- [ ] **Step 3: notification log repository を追加**

同じ追加モデル群に対する重複通知を防ぐ。

- [ ] **Step 4: push 通知文面を追加**

内容:
- 件名: 新規追加モデル件数
- 本文: provider ごとに数件列挙
- target URL: `/openrouter-models`

- [ ] **Step 5: メール通知文面を追加**

Resend で送る summary を追加する。

- [ ] **Step 6: Inngest function テスト**

Run: `docker compose exec -T api go test ./internal/inngest/... ./internal/service/... ./internal/repository/...`
Expected: PASS

- [ ] **Step 7: Commit**

```bash
git add api/internal/inngest/functions.go api/internal/repository/openrouter_model_notification_logs.go api/internal/service/resend.go
git commit -m "OpenRouterモデルの日次同期と新規追加通知を追加"
```

## Chunk 3: Worker と全用途 routing

### Task 5: OpenRouter provider を worker に追加

**Files:**
- Create: `worker/app/services/openrouter_service.py`
- Modify: `worker/app/routers/facts.py`
- Modify: `worker/app/routers/summarize.py`
- Modify: `worker/app/routers/digest.py`
- Modify: `worker/app/routers/ask.py`
- Modify: `worker/app/routers/facts_check.py`
- Modify: `worker/app/routers/summary_faithfulness.py`
- Modify: `worker/app/routers/feed_seed_suggestions.py`
- Modify: `worker/app/routers/feed_suggestions.py`
- Modify: `worker/app/routers/translate_title.py`
- Modify: `api/internal/service/worker.go`
- Modify: `api/internal/handler/ask.go`
- Modify: `api/internal/handler/sources.go`

- [ ] **Step 1: OpenRouter service を作成**

`zai_service.py` を雛形にして:
- provider 名 `openrouter`
- base URL を env で切り替え
- pricing は API snapshot 側を使う前提なので最小メタだけ返す

- [ ] **Step 2: 全 router に openrouter 分岐を追加**

対象用途:
- facts
- summary
- digest
- ask
- facts_check
- faithfulness_check
- source suggestion
- translate title

- [ ] **Step 3: worker headers に OpenRouter API key を追加**

Go 側の `workerHeaders(...)` と各 worker 呼び出しを拡張する。

- [ ] **Step 4: ask / sources の provider 選択ロジックに OpenRouter を追加**

OpenRouter key を持つ場合に provider 選択候補へ入るようにする。

- [ ] **Step 5: worker 構文チェック**

Run: `make check-worker`
Expected: PASS

- [ ] **Step 6: API テスト**

Run: `docker compose exec -T api go test ./internal/handler/... ./internal/service/...`
Expected: PASS

- [ ] **Step 7: Commit**

```bash
git add worker/app/services/openrouter_service.py worker/app/routers api/internal/service/worker.go api/internal/handler/ask.go api/internal/handler/sources.go
git commit -m "OpenRouter providerをworkerと全用途のAPI routingに追加"
```

## Chunk 4: Web / OpenRouter Models 画面 / settings 統合

### Task 6: settings に OpenRouter API key と model grouping を追加

**Files:**
- Modify: `web/src/app/(main)/settings/page.tsx`
- Modify: `web/src/components/settings/model-select.tsx`
- Modify: `web/src/i18n/dictionaries/ja.ts`
- Modify: `web/src/i18n/dictionaries/en.ts`

- [ ] **Step 1: settings に OpenRouter API key セクションを追加**

Z.ai と同じ密度で:
- 保存
- 削除
- last4 表示

- [ ] **Step 2: ModelSelect の OpenRouter grouping に対応**

OpenRouter snapshot の provider slug を group label に使う。

- [ ] **Step 3: i18n 文言を追加**

追加対象:
- OpenRouter title
- description
- save/delete toast
- sync button

- [ ] **Step 4: web build**

Run: `make web-build`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add web/src/app/(main)/settings/page.tsx web/src/components/settings/model-select.tsx web/src/i18n/dictionaries/ja.ts web/src/i18n/dictionaries/en.ts
git commit -m "設定画面にOpenRouter APIキーとモデル選択UIを追加"
```

### Task 7: OpenRouter Models 画面を追加

**Files:**
- Create: `web/src/app/(main)/openrouter-models/page.tsx`
- Modify: `web/src/components/nav.tsx`
- Modify: `web/src/lib/api.ts`
- Modify: `web/src/i18n/dictionaries/ja.ts`
- Modify: `web/src/i18n/dictionaries/en.ts`

- [ ] **Step 1: OpenRouter models API client を web に追加**

取得対象:
- latest snapshot
- last sync metadata
- new model list
- manual sync action

- [ ] **Step 2: 一覧画面を作る**

構成:
- 上段: 新規追加モデル
- 右上: 手動更新 / 最終同期時刻
- 下段: provider group ごとの全モデル一覧
- 説明文は `description_ja` 優先

- [ ] **Step 3: nav に導線を追加**

`More` 配下に `OpenRouter Models` を追加する。

- [ ] **Step 4: web build**

Run: `make web-build`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add web/src/app/(main)/openrouter-models/page.tsx web/src/components/nav.tsx web/src/lib/api.ts web/src/i18n/dictionaries/ja.ts web/src/i18n/dictionaries/en.ts
git commit -m "OpenRouter Models画面と手動同期導線を追加"
```

### Task 8: Usage / Analysis / 比較表に OpenRouter を統合

**Files:**
- Modify: `web/src/app/(main)/llm-usage/page.tsx`
- Modify: `web/src/app/(main)/llm-analysis/page.tsx`
- Modify: `web/src/components/llm-usage/tables.tsx`
- Modify: `web/src/components/llm-usage/value-metrics-panel.tsx`
- Modify: `api/internal/service/model_provider.go`

- [ ] **Step 1: provider 表示に OpenRouter を追加**

`providerLabel` に `OpenRouter` を追加する。

- [ ] **Step 2: model 表示はフル model id を維持**

OpenRouter は provider 内 sub-provider を model id に含めるため、短縮しすぎない。

- [ ] **Step 3: cost efficient provider priority に OpenRouter を追加**

既存 provider 並びとの整合を取る。

- [ ] **Step 4: web build と API test**

Run:
- `make web-build`
- `docker compose exec -T api go test ./internal/service/...`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add web/src/app/(main)/llm-usage/page.tsx web/src/app/(main)/llm-analysis/page.tsx web/src/components/llm-usage/tables.tsx web/src/components/llm-usage/value-metrics-panel.tsx api/internal/service/model_provider.go
git commit -m "LLM UsageとAnalysisにOpenRouter providerを統合"
```

## Final Verification

- [ ] **Step 1: Run API tests**

Run: `docker compose exec -T api go test ./...`
Expected: PASS

- [ ] **Step 2: Run worker checks**

Run: `make check-worker`
Expected: PASS

- [ ] **Step 3: Run web build**

Run: `make web-build`
Expected: PASS

- [ ] **Step 4: Run migration**

Run: `make migrate-up`
Expected: version `67`

- [ ] **Step 5: Commit final verification-only fixes if needed**

```bash
git add -A
git commit -m "OpenRouter追加の最終整合を確認"
```
