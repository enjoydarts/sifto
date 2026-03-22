# 2026-03-23 Sifto Poe Provider 設計メモ

## 結論

Poe は `OpenRouter と同じく動的カタログを永続化する provider` として導入する。

ただし UX は OpenRouter より単純にし、`ユーザーはモデルだけ選ぶ`、`OpenAI互換 / Anthropic互換の切り替えは裏側で自動判定する` 方式にする。

初手から実装対象にするのは以下。

- ユーザー別 Poe API key 保存
- Poe モデル一覧の同期
- モデルスナップショット永続化
- モデル検索用の一覧 UI
- 動的 catalog 合成
- 実行時 transport 自動切り替え
- Poe の価格 metadata 取り込み
- 日本語説明文のバックグラウンド翻訳
- provider model updates 連携

後回しでよいものは以下。

- OpenRouter と同等の `structured output override` UI
- Usage API を使った billed cost の完全照合
- transport 別 capability の高度な実測管理

## 要件整理

今回の前提は以下。

1. Poe でも OpenRouter 同様に `動的カタログ永続化` を行う
2. Poe でも `モデル検索用の一覧画面` を最初から提供する
3. ユーザーは `使用モデル` のみ選択する
4. OpenAI互換 / Anthropic互換の切り替えは UI に出さず、内部で自動判定する
5. 日本語説明文はバックグラウンド翻訳で順次反映する

## 背景

Sifto にはすでに OpenRouter 向けの運用レイヤーがある。

- `openrouter_model_sync_runs`
- `openrouter_model_snapshots`
- `openrouter_model_overrides`
- OpenRouter Models 一覧画面
- provider model updates 連携
- 動的 catalog 合成
- 説明文のバックグラウンド翻訳

参照実装:

- `api/internal/service/openrouter_catalog.go`
- `api/internal/handler/openrouter_models.go`
- `api/internal/repository/openrouter_models.go`
- `api/internal/repository/openrouter_model_overrides.go`
- `web/src/app/(main)/openrouter-models/page.tsx`
- `worker/app/services/openrouter_service.py`

Poe もモデル変動がある以上、固定 catalog よりこの方式の方が運用に合う。

## Poe の公式仕様から使えるもの

### 1. Models API

`GET https://api.poe.com/v1/models` でモデル一覧が取れる。

公開ドキュメント上、少なくとも以下の情報がある。

- `id`
- `description`
- `owned_by`
- `architecture`
- `pricing`
- その他の metadata

この `pricing` は一覧表示と概算価格に使える。

### 2. Usage API

`GET https://api.poe.com/usage/points_history` で利用履歴が取れ、`cost_usd` と `cost_points` を参照できる。

これは将来の billed cost 補完に使えるが、初期実装では snapshot pricing を優先する。

### 3. Anthropic 互換 API の制約

Poe の Anthropic 互換 API は `Claude models only` である。

重要なのは、`models API に互換 API の明示フラグがそのまま載っている前提ではない` こと。したがって transport 自動切り替えは、models API の metadata と公式仕様ルールの組み合わせで実装する。

## 基本方針

### 方針の要点

`OpenRouter と同じ永続化方式`  
`説明文翻訳も初手から入れる`  
`実行 transport は Poe 専用ルールで自動判定する`

具体的には以下。

1. provider id は `poe`
2. モデル一覧は API から同期し、DB にスナップショット保存する
3. catalog には `poe/<model_id>` 形式で動的注入する
4. ユーザーは model だけ選択する
5. 実行時は model metadata から transport を自動判定する
6. 一覧画面では検索・価格・provider・transport 情報を表示する
7. 説明文の日本語はバックグラウンドで反映する

## transport 自動切り替え設計

### ユーザー体験

ユーザーは `Claude-Sonnet-4.5` や `GPT-5` のようにモデルだけ選ぶ。`OpenAI互換で呼ぶか Anthropic互換で呼ぶか` は選ばせない。

### 内部判定ルール

初期実装では以下で十分。

- `Claude 系 official model` は `poe_anthropic_compat` を優先
- それ以外は `poe_openai_compat`

判定材料:

- `owned_by`
- `id`
- `architecture`
- 公式仕様: Anthropic 互換は Claude のみ

推奨ロジック:

1. `id` が Claude 系か判定する
2. `owned_by == "Anthropic"` か、少なくとも Anthropic 系 official model と判定できるなら `anthropic_compat = true`
3. それ以外は `anthropic_compat = false`
4. `openai_compat = true` を既定にする
5. `preferred_transport = anthropic_compat ? "anthropic" : "openai"`

### snapshot に持たせる項目

- `transport_supports_openai_compat`
- `transport_supports_anthropic_compat`
- `preferred_transport`

これにより UI と worker の両方で同じ判定結果を使える。

## スコープ提案

### Phase 1: 初手から入れるもの

目的は「Poe を OpenRouter に近い完成度で載せること」。

- API key 管理
- モデル同期 run 永続化
- モデル snapshot 永続化
- 動的 catalog 合成
- モデル一覧 UI
- transport 自動切り替え
- snapshot pricing 表示
- provider model updates への統合
- 日本語説明文の非同期翻訳

### Phase 2: 精度改善

- billed cost を Usage API で補完
- capability 判定の精度向上
- Anthropic/OpenAI 互換別の structured output 実測反映
- 説明文キャッシュの改善

### Phase 3: 運用拡張

- override UI
- 実行不能モデルの通知強化
- provider 別の差分通知改善

## データモデル案

### 1. user_settings

`user_settings` に以下を追加する。

- `poe_api_key_enc TEXT`
- `poe_api_key_last4 TEXT`

OpenRouter と同じくユーザー別 API key 保存とする。

### 2. sync runs

OpenRouter と同様に `poe_model_sync_runs` を持つ。

カラム案:

- `id`
- `started_at`
- `finished_at`
- `last_progress_at`
- `status`
- `trigger_type`
- `fetched_count`
- `accepted_count`
- `translation_target_count`
- `translation_completed_count`
- `translation_failed_count`
- `last_error_message`
- `error_message`

OpenRouter と違って translation 列を削る理由はないため、初手から持つ。

### 3. snapshots

`poe_model_snapshots` を持つ。

カラム案:

- `id`
- `sync_run_id`
- `fetched_at`
- `model_id`
- `canonical_slug`
- `display_name`
- `owned_by`
- `description_en`
- `description_ja`
- `context_length`
- `pricing_json`
- `architecture_json`
- `modality_flags_json`
- `is_active`
- `transport_supports_openai_compat`
- `transport_supports_anthropic_compat`
- `preferred_transport`

OpenRouter にある `supported_parameters_json` は、Poe の models API で安定して取れないなら初期段階では持たなくてよい。

`description_ja` は同期直後は空でもよく、翻訳ジョブで順次埋める。

### 4. model updates

OpenRouter と同様に provider model updates に `provider = "poe"` で流す。

一覧画面で「追加」「削除」を見せる最低限の差分通知は初手から欲しい。

ここは `OpenRouter と同じ通知方式` に揃える。

- 差分種別は `added` / `removed` を基本とする
- 直前の成功 snapshot と比較して差分を作る
- `provider_model_updates` へ同じ形式で保存する
- Settings 画面の provider model updates panel に同じ見え方で出す
- dismiss / restore の UX も OpenRouter と同じにする

## catalog 設計

### provider 定義

`shared/llm_catalog.json` の `providers` に `poe` を追加する。

例:

- `id: "poe"`
- `api_key_header: "x-openai-api-key"`
- `default_models: {}`

`match_prefixes` は不要。動的モデル前提で扱う。

### model id

内部 catalog では `poe/<model_id>` の形式を使う。

理由:

- provider 判定が容易
- 他 provider と衝突しにくい
- 使用モデル設定の保存時に出自が明確

必要関数:

- `PoeAliasModelID(modelID string) string`
- `ResolvePoeModelID(modelID string) string`

## モデル同期サービス

### API

`api/internal/service/poe_catalog.go` を新設するのが素直。

責務:

- `GET /v1/models` を呼ぶ
- text generation 向けモデルに絞る
- snapshot 形式へ正規化する
- transport 自動判定 metadata を付与する
- pricing を JSON のまま保存する

### 正規化ルール

- `model_id`: API の `id`
- `display_name`: まず `id` をそのまま使い、必要なら alias を後で足す
- `owned_by`: API の `owned_by`
- `description_en`: API の `description`
- `pricing_json`: API の `pricing`
- `architecture_json`: API の `architecture`
- `modality_flags_json`: 必要な modality を JSON 化
- `preferred_transport`: 自動判定結果

### 説明文翻訳

Poe でも OpenRouter と同じパターンを採用する。

- sync ではまず `description_en` を保存する
- sync API の応答は翻訳完了を待たずに返す
- その後バックグラウンドで `description_en -> description_ja` を翻訳する
- 翻訳済みのものから順次一覧に反映する

翻訳方針:

- `OPENAI_API_KEY` がある場合のみ実行
- 前回キャッシュと同一の英語説明は再翻訳しない
- 翻訳失敗は `translation_failed_count` に積み、モデル同期全体は失敗扱いにしない

## リポジトリ層

OpenRouter の `OpenRouterModelRepo` に近い `PoeModelRepo` を追加する。

必要機能:

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

translation 系メソッドも初手から必要とする。

## handler 設計

`api/internal/handler/poe_models.go` を追加する。

必要 endpoint:

- `GET /api/poe-models`
- `POST /api/poe-models/sync`
- `GET /api/poe-models/status`

返却内容は OpenRouter Models 画面に近づける。

返す項目:

- `models`
- `latest_run`
- `latest_change_summary`

モデル行には以下を含める。

- `model_id`
- `display_name`
- `owned_by`
- `description_en`
- `description_ja`
- `context_length`
- `pricing_json`
- `preferred_transport`
- `transport_supports_openai_compat`
- `transport_supports_anthropic_compat`

## Web UI 設計

### Settings

OpenRouter と同様に Settings に Poe API key カードを追加する。

必要変更:

- `web/src/app/(main)/settings/page.tsx`
- `web/src/lib/api.ts`
- `web/src/i18n/dictionaries/ja.ts`
- `web/src/i18n/dictionaries/en.ts`

### Poe Models 画面

OpenRouter Models と同系統の画面を作る。

推奨 URL:

- `/poe-models`

表示項目:

- モデル ID
- 提供元 `owned_by`
- 説明
- 日本語説明
- コンテキスト長
- 価格
- preferred transport
- OpenAI互換対応
- Anthropic互換対応

検索対象:

- model id
- description
- owned_by

フィルタ:

- provider/owned_by
- preferred transport
- Claude のみ
- 利用可能状態

### 一覧 UI の注意

transport 選択 UI は出さず、`このモデルは内部的に Anthropic互換で実行` のような表示に留める。

翻訳進捗も表示対象にする。

- 翻訳対象数
- 翻訳完了数
- 翻訳失敗数
- 実行中ステータス

## worker 設計

### service 分割

`worker/app/services/poe_service.py` を追加する。

内部では transport を2つ持つ。

- `poe_openai_compat`
- `poe_anthropic_compat`

ただし呼び出し側には 1 provider として見せる。

### 実行フロー

1. `poe/<model_id>` を解決
2. 最新 snapshot から `preferred_transport` を引く
3. `preferred_transport == anthropic` なら Anthropic 互換 endpoint を呼ぶ
4. それ以外は OpenAI 互換 endpoint を呼ぶ

### fallback

実行時に transport 不整合で失敗した場合は、限定的に片系 fallback を検討してよい。

例:

- Anthropic 互換で 400/unsupported model
- そのモデルの `transport_supports_openai_compat = true`
- そのときのみ OpenAI 互換へ再試行

ただし無限 fallback は避ける。

## capability 方針

初期段階では保守的に扱う。

- `supports_structured_output`: false 既定
- `supports_tool_calling`: false 既定
- `supports_reasoning`: false 既定

将来的に transport 別に capability を持たせる。

理由:

- Claude を Anthropic互換で呼ぶ場合と OpenAI互換で呼ぶ場合で挙動が違い得る
- models API だけでは strict schema の可否を安全に判定しにくい

## pricing 方針

### 初期段階

`/v1/models` の `pricing` を snapshot に保存し、一覧画面と catalog の概算価格に使う。

`pricing_source = "poe_snapshot"` の扱いでよい。

### 将来

`/usage/points_history` の `cost_usd` / `cost_points` を使って billed cost 補完を行う。

このときは OpenRouter の generation API 補完とは別実装にする。

## OpenRouter 実装から再利用できる部分

- Settings の API key 管理 UX
- sync run / snapshot の永続化パターン
- description 翻訳のバックグラウンド処理
- provider model updates 連携
- 動的 model catalog 合成
- モデル一覧ページの UI 骨格
- `requested_model` / `resolved_model` を usage に残す考え方

## OpenRouter 実装をそのまま持ち込まない部分

- `structured output override` の運用
- OpenRouter generation API に依存した billed cost 補完
- `supported_parameters_json` 前提の capability 判定

## リスク

### 1. transport 自動判定の誤り

models API に明示的な互換 API フラグがないため、Claude 判定を実装で吸収する必要がある。

対策:

- snapshot に自動判定結果を保存して可視化する
- Claude official model 判定ロジックをテストする
- 失敗時の片系 fallback を限定的に持つ

### 2. モデル変動

公開モデルの入れ替わりで保存済みモデルが使えなくなる可能性がある。

対策:

- 定期同期
- OpenRouter と同じ provider model updates 通知
- 実行時の invalid model 検出

### 3. 価格の差分

snapshot pricing と usage の実績請求が完全一致しない可能性がある。

対策:

- 一覧は snapshot pricing
- usage 集計は将来的に billed cost 補完

### 4. 翻訳ジョブの停滞

モデル数が多い場合、翻訳ジョブが完了まで時間を要する可能性がある。

対策:

- sync run に翻訳進捗を持つ
- stale 判定を入れる
- 説明文未翻訳でも一覧自体は利用可能にする

## 推奨実装順

1. `user_settings` に Poe API key を追加
2. `poe_model_sync_runs` / `poe_model_snapshots` migration を追加
3. `PoeModelRepo` と `PoeCatalogService` を追加
4. `GET /v1/models` 同期と snapshot 永続化を実装
5. description のバックグラウンド翻訳を実装
6. 動的 catalog に `poe/<model>` を注入
7. Settings に Poe API key UI を追加
8. `Poe Models` 一覧画面を追加
9. worker に `poe_service.py` を追加
10. transport 自動切り替えを実装
11. provider model updates へ Poe を統合

## 最終提案

Poe は `OpenRouter と同じく動的カタログを永続化する provider` として扱う。

ただし UX はよりシンプルにし、`モデルだけ選ぶ`、`transport は内部で自動選択する` に寄せる。

この方式なら以下を両立できる。

- モデル変動への追従
- 一覧検索の使いやすさ
- Claude 系は Anthropic 互換の恩恵を受ける
- それ以外の Poe モデルも OpenAI 互換で広く扱える
- 日本語説明文を後追いで自然に反映できる

初回実装の完成条件は以下。

- Poe API key を保存できる
- Poe モデルを同期・永続化できる
- Poe Models 画面で検索できる
- 日本語説明文がバックグラウンドで反映される
- `poe/<model>` を選ぶだけで worker が適切な transport を自動選択して実行できる

この形で進めるのが妥当。
