# Prompt Versioning And A/B Test Design

## Goal

LLM prompt をコード外でバージョン管理できるようにし、将来的な A/B テストと品質比較の基盤を追加する。

ただし初期リリースでは、既定の実行 prompt は現行のコード内ハードコードを使い続ける。新機能は「切替可能な基盤」として先に入れ、既存の出力品質と運用を変えないことを優先する。

prompt 管理 UI は通常のアプリ機能として提供するが、利用できるのは許可された特定ユーザだけに限定する。

初回対象は次に限定する。

- `summary`
- `facts`
- `digest`
- `audio_briefing_script`

## Current State

- worker の各タスクが prompt をコード内で組み立てている
- prompt の修正には worker の deploy が必要
- どの prompt 文面で生成された結果かを後から追いにくい
- バージョン間比較や段階 rollout の仕組みがない

## Recommended Approach

prompt の正本は DB に置く。ただし保存・編集対象は「内部 template」ではなく、実際に LLM へ渡す完成 prompt とする。prompt の解決は worker ではなく API 側で行い、worker には「解決済み prompt」を渡す。

初期リリースでは resolver の既定値を現行コード prompt にする。DB 上に active version や experiment が存在しても、自動では使わない。明示的に opt-in された purpose / run だけが DB prompt を使う。

この構成にすると、

- deploy なしで prompt version を登録できる
- 実行ログに prompt version を紐付けられる
- worker を stateless のまま保てる
- 既定挙動を変えずに段階導入できる

## Scope

今回入れるもの:

- prompt template / version / experiment の DB schema
- API 側 `PromptResolver`
- 実行ごとの prompt metadata 保存
- deterministic な arm assignment の仕組み
- 特定ユーザ限定の prompt 管理 UI
- 特定ユーザ限定の管理 API

今回入れないもの:

- 全ユーザに見える prompt 管理 UI
- 自動 winner 判定
- 全 prompt 種別の一括移行
- 既存ハードコード prompt の即時削除

## Design Principle

既定値は常に現行コード prompt とする。

つまり解決優先順は初期状態では次のようにする。

1. 明示指定された override version
2. 明示的に有効化された experiment assignment
3. 現行コード内 default prompt

「DB に active version がある」だけでは既定値を置き換えない。これにより、schema と logging を先に本番投入しても生成品質の予期せぬ変化を避けられる。

ただし Prompt Admin で編集する実体は、現行コードと等価な完成 prompt そのものにする。`{{task_block}}` のような実装都合の中間変数は UI と DB の管理対象に含めない。

## Access Control

prompt 管理対象は global だが、操作権限は特定ユーザのみに制限する。

認可方式は環境変数でメールアドレス allowlist を持つ。

- 例: `PROMPT_ADMIN_EMAILS=alice@example.com,bob@example.com`

判定ルール:

- API は認証済みセッションユーザの email を取得し、allowlist に含まれる場合のみ管理 API を許可する
- Web は capability 情報に基づいて prompt 管理 UI を表示する
- 未許可ユーザが直接 API を叩いても `403` を返す

この機能は一般ユーザ向け設定ではなく、限定公開の運用者向け管理機能として扱う。

## Management UI

prompt 管理 UI は internal/debug ではなく通常アプリ内に置く。ただし表示対象は許可ユーザのみとする。

最低限必要な画面は次のとおり。

- template 一覧
- version 一覧
- version 詳細
- 新規 version 作成
- active override 状態の確認と切替
- experiment 作成・停止・配分変更

UI 上では対象 purpose を横断して扱えるようにし、`summary / facts / digest / audio_briefing_script` を同じ管理導線に載せる。

編集 UX は次を原則とする。

- `system_instruction` と `prompt_text` は全文を直接編集できる
- 変数一覧は別枠で確認できる
- 変数は `{{title}}`, `{{facts_text}}` のような実データ差し込みだけを残す
- `{{task_block}}` のような中間組み立て用変数は残さない
- プレビューはモーダルで、代表入力を差し込んだレンダリング結果を確認する

## API Surface

管理 UI 用に専用 API を追加する。

想定する最小 API:

- `GET /api/admin/prompts/capabilities`
- `GET /api/admin/prompts/templates`
- `GET /api/admin/prompts/templates/:id`
- `POST /api/admin/prompts/templates/:id/versions`
- `POST /api/admin/prompts/templates/:id/activate`
- `POST /api/admin/prompts/experiments`
- `PATCH /api/admin/prompts/experiments/:id`

`capabilities` では少なくとも `can_manage_prompts` を返し、Web 側で画面表示制御に使えるようにする。

## Data Model

### `prompt_templates`

prompt の論理単位。

- `id`
- `key`
- `purpose`
- `description`
- `status`
- `created_at`
- `updated_at`

`key` は `summary.default`, `facts.default` のような安定識別子にする。

### `prompt_template_versions`

template ごとの版管理。

- `id`
- `template_id`
- `version`
- `system_instruction`
- `prompt_text`
- `fallback_prompt_text`
- `variables_schema`
- `notes`
- `created_by`
- `created_at`

ここで完成 prompt 本文と変数仕様を管理する。`version` は template 内で単調増加にする。

`system_instruction` と `prompt_text` は、その version を activate したときにそのまま最終レンダリングへ使われる文面とする。内部部品の partial や section fragment は保存対象にしない。

### `prompt_experiments`

A/B テスト定義。

- `id`
- `template_id`
- `name`
- `status`
- `assignment_unit`
- `started_at`
- `ended_at`

### `prompt_experiment_arms`

experiment の arm 定義。

- `id`
- `experiment_id`
- `version_id`
- `weight`

## Assignment Unit

再試行や同一対象の再生成でも arm がぶれないよう、対象ごとに deterministic assignment を行う。

- `summary`, `facts`: `item_id`
- `digest`: `digest_id`
- `audio_briefing_script`: `job_id` 相当の stable id

初期版では `digest_cluster_draft` のような補助タスクは `digest` に含めて扱う。必要なら後で key を分離する。

## Resolver Flow

1. API/Inngest が task 実行前に `purpose`, `prompt_key`, `context` を集める
2. `PromptResolver` が global override 指定の有無を確認する
3. opt-in 対象の experiment が有効なら deterministic に arm を選ぶ
4. 該当がなければ現行コードの default prompt strategy を使う
5. 解決結果を worker リクエストへ含める
6. 実行結果の usage / run log に prompt metadata を保存する

worker は DB や assignment を知らず、受け取った prompt を使うだけにする。

## Prompt Strategy

`default_code` と `template_version` は別々の特殊処理にせず、同じ interface に寄せる。

- `default_code`
  現行コードと等価な完成 prompt を返す strategy
- `template_version`
  DB に保存された完成 prompt を返す strategy

どちらの strategy でも、最終的に worker へ渡すのは次の 3 点に揃える。

- `system_instruction`
- `prompt_text`
- `variables`

これにより、既定 prompt と DB prompt が同じ renderer を通り、比較や置換が自然になる。

## Prompt Resolution Contract

API から worker に渡す構造は次を想定する。

- `prompt_key`
- `prompt_source` (`default_code`, `template_version`, `experiment_arm`, `override`)
- `resolved_prompt_text`
- `resolved_system_instruction`
- `prompt_version_id` nullable
- `prompt_version_number` nullable
- `prompt_experiment_id` nullable
- `prompt_experiment_arm_id` nullable

`default_code` の場合でも `prompt_key` と source を残すことで、後から「どの runs がまだコード prompt を使っているか」を追える。

## Editing Semantics

Prompt Admin で編集した version は、「既存 prompt への追記」ではなく「完成 prompt の全文置換」として扱う。

このため次が可能になる。

- 既存の一節を削除する
- 構成順を入れ替える
- 文体や制約文を丸ごと差し替える

逆に、section fragment を個別合成する設計には戻さない。削除不能な編集体験になるためである。

## Logging And Evaluation

既存の LLM usage / execution log には最低限次を保存する。

- `prompt_key`
- `prompt_source`
- `prompt_version_id`
- `prompt_version_number`
- `prompt_experiment_id`
- `prompt_experiment_arm_id`

また、template version 作成や active 切替などの管理操作は監査ログに残す。

最低限残す項目:

- 操作者 user id
- 操作者 email
- 操作種別
- 対象 template / version / experiment
- 実行時刻

比較指標としては次を優先する。

- `run_count`
- `success_rate`
- `retry_rate`
- `parse_error_rate`
- `empty_output_rate`
- 既存 quality check の warn / fail rate
- `avg_input_tokens`
- `avg_output_tokens`
- `avg_cost_usd`

purpose ごとの業務指標は phase 2 以降で足す。初期版はまず生成品質とコストの比較を成立させる。

## Rollout Plan

### Phase 1: Foundation

- schema と repository を追加
- `PromptResolver` を追加
- 各対象 task に prompt metadata を通す
- 既存 prompt と等価な完成 prompt を seed で DB に投入できるようにする
- 環境変数 allowlist による認可 service を追加する
- 許可ユーザ限定の管理 UI を追加する
- ただし runtime default は現行コード prompt のままにする

### Phase 2: Controlled Opt-In

- 管理 UI 経由で template version を指定できるようにする
- experiment を purpose 単位で明示有効化できるようにする
- assignment と logging を有効にする

### Phase 3: Comparison And Promotion

- version / arm ごとの集計画面を追加
- winner 候補の比較を可能にする
- 必要なら既定値を DB 管理へ寄せる

## Failure Handling

- resolver が DB 読み取りに失敗しても、既定ではコード prompt に fallback する
- experiment が壊れていても run 自体は止めない
- version に必要変数が足りない場合は run 前に validation error として扱う
- worker 側は prompt source に依存した特別処理を持たない

「prompt 管理基盤の障害で本来の生成が止まる」を避けるのが優先である。

## Migration Strategy

既存の prompt builder は削除しない。初期導入では fallback として残す。

移行手順は次を基本とする。

1. 現行 prompt を DB へ seed
2. metadata 保存と認可付き管理 UI を本番投入
3. 許可ユーザが UI から限定的に override 実行
4. 問題ない purpose から experiment を有効化
5. 運用が安定してから default source の見直しを検討

## Testing

最低限の確認対象は次のとおり。

- resolver が override を最優先する
- experiment 無効時は必ずコード prompt を返す
- experiment 有効時に assignment が stable である
- DB 障害時もコード prompt に fallback する
- run log に prompt metadata が保存される
- 未許可ユーザは管理 UI を見られず API でも `403` になる
- allowlist 環境変数の変更で認可結果が反映される
- 対象 4 系統で既存生成フローを壊さない

## Open Questions

- seed と code fallback の差分検知をどこまで自動化するか
- 評価指標に product analytics をどの段階で結合するか

## Decision

J1 の初回実装は「prompt versioning / A/B test 基盤を入れるが、既定 prompt は現行コードのまま」にする。これにより、運用上の安全性を保ったまま、将来の prompt 改善サイクルを高速化するための土台を先に整備する。
