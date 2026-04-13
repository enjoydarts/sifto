# Sifto MiniMax Provider 追加設計

作成日: 2026-04-13

## 1. 目的

Sifto の LLM provider に `MiniMax` を独立 provider として追加する。

今回の対象はテキスト系 LLM のみとし、以下を一体で成立させる。

- ユーザー別 API key 管理
- 用途別モデル選択
- `shared/llm_catalog.json` への代表モデル登録
- API / worker での推論実行
- `llm_usage_logs` / LLM usage 画面での provider 集計
- `provider model updates` の snapshot / change tracking

TTS は今回のスコープ外とする。

## 2. スコープ

### 2.1 含むもの

- `minimax` を独立 provider として API / worker / web / usage / provider updates に追加
- `user_settings` への MiniMax API key 保存
- settings 画面での API key 保存 / 削除 / 表示
- `shared/llm_catalog.json` への MiniMax provider と代表モデル追加
- facts / summary / digest cluster draft / digest / ask / source suggestion など既存用途での利用
- `provider model snapshots` の同期対象への追加
- provider model update 通知・一覧・スナップショット表示への反映

### 2.2 含まないもの

- MiniMax の TTS / voice / audio 系
- MiniMax 専用の新規 UI 画面
- catalog に旧世代モデルを大量追加すること
- 未確認価格の推測登録

## 3. 前提

### 3.1 Provider と transport の分離

Sifto 内部では `minimax` を独立 provider として扱うが、実行 transport は既存の OpenAI 互換経路を再利用する。

理由:

- 既存 worker は OpenAI 互換 provider を複数抱えており、structured output / fallback / usage 記録の再利用価値が高い
- `provider=minimax` の集計・設定・snapshot を保ちながら、実装差分を限定できる
- 独自 endpoint 専用実装よりも既存の保守負荷に馴染む

ただし provider の意味論は `openai` に吸収しない。
settings、catalog、usage、snapshot、provider updates では常に `minimax` として扱う。

### 3.2 公式モデル採用方針

MiniMax の catalog には代表モデルのみを載せる。
日付付き alias や旧世代モデルは並べない。

今回採用するモデル:

- `MiniMax-M2.7`
- `MiniMax-M2.7-highspeed`
- `MiniMax-M2.5`
- `MiniMax-M2.5-highspeed`

今回採用しないモデル:

- `M2-her`
- `MiniMax-M2.1` 系
- `MiniMax-M2`

理由:

- Sifto の主用途は facts / summary / ask / digest であり、汎用テキストモデル中心の方がよい
- repo 方針上、代表モデルだけを catalog に置く
- 旧世代を増やすと settings の選択肢と運用コストだけが増える

## 4. catalog 設計

### 4.1 provider 定義

`shared/llm_catalog.json` に `minimax` provider を追加する。

- `id`: `minimax`
- `api_key_header`: `x-minimax-api-key`
- `match_exact`:
  - `MiniMax-M2.7`
  - `MiniMax-M2.7-highspeed`
  - `MiniMax-M2.5`
  - `MiniMax-M2.5-highspeed`

`match_prefixes` は今回追加しない。
正式な代表モデルだけを厳密一致で扱う。

### 4.2 用途別 default model

- `facts`: `MiniMax-M2.5-highspeed`
- `summary`: `MiniMax-M2.7`
- `digest_cluster_draft`: `MiniMax-M2.7`
- `digest`: `MiniMax-M2.7`
- `ask`: `MiniMax-M2.7-highspeed`
- `source_suggestion`: `MiniMax-M2.5-highspeed`

意図:

- `facts` と `source_suggestion` は速度重視
- `summary` / `digest` は品質重視
- `ask` は体感速度を優先しつつ一段上のモデルを使う

### 4.3 chat_models 定義

`chat_models` には上記 4 モデルを追加する。

各モデルに以下を記載する。

- `provider`: `minimax`
- `available_purposes`
- `recommendation`
- `best_for`
- `highlights`
- `comment`
- `capabilities`
- `pricing`

価格は公式 pricing docs で確認できた値だけを入れる。
未確認価格は入れない。

## 5. API / DB 設計

### 5.1 user_settings

`user_settings` に以下を追加する migration を入れる。

- `minimax_api_key_enc TEXT`
- `minimax_api_key_last4 TEXT`

既存 provider と同じく nullable で持つ。

### 5.2 Go model / repository / service

以下に `MiniMax` を追加する。

- `api/internal/model/model.go`
  - `HasMiniMaxAPIKey`
  - `MiniMaxAPIKeyLast4`
- `api/internal/repository/user_settings.go`
  - read path
  - `SetMiniMaxAPIKey`
  - `ClearMiniMaxAPIKey`
  - `GetMiniMaxAPIKeyEncrypted`
- `api/internal/service/settings_service.go`
  - DTO への反映
  - `SetAPIKey` / `DeleteAPIKey` switch 追加
- `api/internal/service/user_key_provider.go`
  - loader 追加

### 5.3 settings handler

`api/internal/handler/settings.go` に以下を追加する。

- `SetMiniMaxAPIKey`
- `DeleteMiniMaxAPIKey`

返却 payload は既存 provider と同様に以下を返す。

- `has_minimax_api_key`
- `minimax_api_key_last4`

### 5.4 key loading / runtime selection

以下に `minimax` を追加する。

- `api/internal/handler/key_loader.go`
- `api/internal/inngest/llm_keys.go`
- `api/internal/service/model_provider.go`
- `api/internal/handler/ask.go`
- `api/internal/handler/briefing.go`
- `api/internal/service/audio_briefing_pipeline.go`
- `api/internal/service/ai_navigator_briefs.go`
- `api/internal/service/source_suggestion.go`
- `api/internal/service/tts_markup_preprocess.go`

方針:

- `minimax` は provider 判定上は独立
- runtime key tuple では OpenAI 互換 transport に載せる
- ただし usage の `provider` は `minimax` を維持する

## 6. Worker 実行設計

### 6.1 API → worker header

`api/internal/service/worker.go` の header 構築に `X-Minimax-Api-Key` を追加する。

既存の `deepseek` / `alibaba` / `mistral` と同じく、全 LLM 呼び出しメソッドに MiniMax key を通す。

### 6.2 Python worker の provider 追加

worker 側では `minimax` を独立 provider として判定できるようにする。

必要な実装:

- model から `minimax` を識別する catalog 判定
- MiniMax API key header の受け取り
- OpenAI 互換 client を使う場合の MiniMax 専用 base URL / auth 設定
- structured output / JSON schema / retry / empty response handling を既存 OpenAI 互換 provider と同等に扱う

### 6.3 接続方式

実行 transport は OpenAI 互換 client を再利用する。

ただし内部で以下を切り替える。

- provider: `minimax`
- base URL: MiniMax 用 endpoint
- auth: MiniMax Bearer token
- model: `MiniMax-M2.7` など catalog に定義した model ID をそのまま送る

これにより `openai` provider と `minimax` provider を分離したまま、実装の重複を抑える。

## 7. Provider Model Updates 設計

### 7.1 discovery 対象追加

`api/internal/service/provider_model_discovery.go` の `ProviderModelDiscoveryKeys` と `DiscoverAll` に `minimax` を追加する。

`ProviderModelSnapshotSyncService` の user key 収集にも `MiniMax` を追加する。

### 7.2 discovery 方法

MiniMax の model discovery は、安定した公式 endpoint を使ってモデル一覧を取得し、snapshot 用 model ID 群へ正規化する。

要件:

- 公式ソースから取得する
- 一時失敗時は既存の retry 方針に従う
- 取得失敗時は snapshot status を `failed` にし、直前成功 snapshot を保持する

### 7.3 snapshot / change event

既存実装をそのまま使う。

`provider=minimax` の snapshot が追加されれば、以下へ自然に反映される。

- provider model updates panel
- main page の update notification
- provider model snapshots 一覧
- Push 通知

## 8. Web / Settings 設計

### 8.1 型

以下に `minimax` を追加する。

- `web/src/types/api/settings.ts`
- provider 関連の UI 型
- model guide / usage 色分けの provider 名一覧

### 8.2 API key card

`web/src/components/settings/system-access-cards.ts` に `minimax` card を追加する。

必要項目:

- `settings.minimaxTitle`
- `settings.minimaxDescription`
- `settings.minimaxNotSet`
- API key 入力例

### 8.3 settings page state

`web/src/app/(main)/settings/use-settings-page-data.ts` に以下を追加する。

- input state
- saving / deleting state
- submit / delete handler
- toast 文言
- access card runtime

### 8.4 i18n

以下を必ず両方更新する。

- `web/src/i18n/dictionaries/ja.ts`
- `web/src/i18n/dictionaries/en.ts`

追加対象:

- API key card title / description / not set
- save / delete toast
- delete confirm title / message
- model guide provider label
- 必要なら settings subtitle / pricing description の provider 列挙文

英語直書きは行わない。

## 9. Usage / 分析表示設計

`provider=minimax` の usage は既存の `llm_usage_logs` 集計で扱える前提とする。

追加対象:

- provider 表示名
- usage / analysis の provider color mapping
- provider filter 一覧

集計期間条件や JST 境界は既存ロジックを変えない。

## 10. テスト方針

最低限、以下を追加または更新する。

- `user_settings` repository の MiniMax key 保存 / 削除 / 取得
- `settings_service` の provider switch
- `settings handler` の save / delete response
- `llm_catalog` の provider 判定と default model 解決
- `provider_model_discovery` の MiniMax discovery
- `provider_model_snapshots_sync` の MiniMax key 収集
- worker header に `X-Minimax-Api-Key` が入ること
- `ask` など provider availability 判定に `HasMiniMaxAPIKey` が効くこと

Web 側は少なくとも型整合と build を通す。

## 11. 実装順

1. migration で `user_settings` に MiniMax key 列を追加
2. Go model / repository / settings service / handler / key provider を追加
3. `shared/llm_catalog.json` に MiniMax provider / models / defaults / pricing を追加
4. provider 判定と runtime key loading を `minimax` 対応に広げる
5. `api/internal/service/worker.go` と Python worker に MiniMax transport を追加
6. provider model discovery / snapshot sync / updates UI を MiniMax 対応にする
7. settings UI / i18n / usage color mapping を追加
8. Go test / web build / worker 構文確認で検証する

## 12. リスク

### 12.1 MiniMax 互換 API の差異

OpenAI 互換でも strict schema や finish reason の細部が異なる可能性がある。
そのため worker では以下を重点確認する。

- 構造化出力の schema 制約
- 空文字応答
- retry 可能 error
- usage fields の有無

### 12.2 model discovery の安定性

MiniMax の公開 API / docs 側で一覧 endpoint が不安定な場合、snapshot sync が失敗する可能性がある。
この場合でも UI を壊さず、既存 snapshot を保持する実装にする。

### 12.3 pricing 更新

価格は静的 catalog 管理なので、価格改定時は手動更新が必要。
不明価格を推測で入れない。

## 13. 採用判断

今回の MiniMax 追加は以下の条件を満たせば完了とする。

- settings で MiniMax API key を保存 / 削除できる
- MiniMax モデルを各 LLM 用途で選べる
- worker が MiniMax を使って既存 LLM 処理を実行できる
- usage / analysis で `provider=minimax` が見える
- provider model snapshots / updates に MiniMax が出る
- `make check-fast`、`make web-build`、`make check-worker`、必要な Go test が通る
