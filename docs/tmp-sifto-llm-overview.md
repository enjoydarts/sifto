# Sifto の LLM 実装まとめ

更新日: 2026-03-17

## 一言でいうと

Sifto の LLM 実装は、複数 provider と OpenRouter を横断して、用途別モデル選択・動的同期・コスト推定・Usage 分析まで一貫して扱える構成です。

---

## 1. 何を解いているのか

Sifto では、`facts`、`summary`、`ask`、`digest`、`check` のように用途ごとに別モデルを使い分けます。

ただし provider が増えると、すぐ次の問題が起きます。

- モデル選択 UI が肥大化する
- provider ごとに API key 管理が必要になる
- 同名モデルが provider をまたいで衝突する
- Usage とコスト比較を同じ軸で集計しにくい
- OpenRouter を入れるとモデル数が一気に増える

Sifto はこの問題を、次の4層で整理しています。

1. 各社 API に直接つなぐ静的 catalog
2. OpenRouter models API から同期する動的 catalog
3. 用途別モデル選択ロジック
4. API 側での usage / cost 正規化

これにより、実験モデルの導入と日常運用の分析を同じ仕組みの上で扱えるようにしています。

---

## 2. 全体アーキテクチャ

Sifto には2種類の LLM provider があります。

### 2-1. 各社 API に直接接続する provider

各社 API に直接接続する provider は、`shared/llm_catalog.json` を基準に静的 catalog として管理しています。

- `anthropic`
- `openai`
- `google`
- `groq`
- `deepseek`
- `alibaba`
- `mistral`
- `xai`
- `zai`

各 provider について、次を持ちます。

- API key header
- model ID と provider の対応
- 用途ごとの default model
- pricing
- structured output / tool calling などの capability

関連実装:

- `shared/llm_catalog.json`
- `api/internal/service/llm_catalog.go`
- `worker/app/services/llm_catalog.py`

### 2-2. OpenRouter

OpenRouter は単なる迂回ルートではなく、**独立 provider** として扱っています。

つまり Sifto では OpenRouter を、

- API key 管理
- 用途別モデル設定
- Usage 集計
- Analysis 画面
- モデル一覧同期

まで含めて、通常 provider と同じレイヤーに統合しています。

ただし OpenRouter は固定 catalog ではなく、OpenRouter models API から取得した結果を DB snapshot として保持し、そこから動的 catalog を組み立てます。

関連実装:

- `api/internal/service/openrouter_catalog.go`
- `api/internal/repository/openrouter_models.go`
- `api/internal/handler/openrouter_models.go`
- `api/cmd/server/main.go`

---

## 3. 静的 catalog と OpenRouter 動的 catalog

### 3-1. 静的 catalog

直接接続 provider のモデルは `shared/llm_catalog.json` に載っています。

主な属性:

- `id`
- `provider`
- `available_purposes`
- `recommendation`
- `best_for`
- `comment`
- `capabilities`
- `pricing`

これは「安定運用枠」の catalog です。

### 3-2. OpenRouter の動的 catalog

OpenRouter は `https://openrouter.ai/api/v1/models` から text generation 向けモデルを取得します。

除外対象:

- embedding 系
- moderation 系
- reranker 系
- speech / transcription 系

採用したモデルは DB snapshot に保存し、Sifto の chat model catalog に変換します。

特徴:

- provider は `openrouter`
- 元の提供元は `provider_slug` として保持
  - 例: `openai`, `google`, `anthropic`, `bytedance-seed`
- pricing は OpenRouter API の値をそのまま使う
- description は英語を保持し、日本語訳があればそちらを優先する

関連実装:

- `api/internal/service/openrouter_catalog.go`

---

## 4. OpenRouter alias と同名モデル問題

OpenRouter は、直接接続 provider と同じ model ID を持つことがあります。例:

- `openai/gpt-oss-120b`

このままだと、

- 直接接続 provider の `openai/gpt-oss-120b`
- OpenRouter 経由の `openai/gpt-oss-120b`

を区別できません。

そのため Sifto では、OpenRouter の動的 catalog には内部 alias を使います。

- OpenRouter 版 model ID
  - `openrouter::openai/gpt-oss-120b`

これにより、

- settings 上で別々に選べる
- Usage / Analysis で別 provider として集計できる
- OpenRouter snapshot 価格で再計算できる

という3つの利点が得られます。

ただし UI では alias を露出しません。画面上は元の model ID を表示します。

関連実装:

- `api/internal/service/llm_catalog.go`
- `worker/app/services/llm_catalog.py`
- `web/src/lib/model-display.ts`

---

## 5. 用途別モデル選択ロジック

Sifto では用途ごとにモデルを持てます。代表例:

- facts
- summary
- facts_check
- faithfulness_check
- digest_cluster_draft
- digest
- ask
- source_suggestion

基本原則は次の2層です。

1. ユーザー設定で明示的に選ばれた model
2. provider ごとの default model

ただし、実際の fallback 条件は用途によって違います。

### 5-1. provider 判定

model から provider を判定する時は API 側 catalog を使います。

- 通常 model
  - `CatalogProviderForModel(model)`
- OpenRouter alias
  - `openrouter::...` は必ず `openrouter`

関連実装:

- `api/internal/service/model_provider.go`
- `api/internal/service/llm_catalog.go`

### 5-2. Ask と process-item 系の違い

| 項目 | Ask | process-item 系 |
| --- | --- | --- |
| 実行場所 | API から worker を直接呼ぶ | Inngest / Worker |
| 優先設定 | `settings.ask_model` | 用途ごとの override |
| 未設定時 | 利用可能 provider の `ask` default | cost-efficient provider 順に default |
| 明示指定 model が使えない場合 | 別 provider へ自動退避しない | 別 provider へ自動退避しない |
| 未指定時の provider fallback | あり | あり |
| OpenRouter | 候補に含む | 候補に含む |

### 5-3. Ask の選択ロジック

Ask は専用設定を最優先にし、その次に provider ごとの `ask` default へ落とします。

流れ:

1. `settings.ask_model`
2. その model の provider に対応する API key があるか確認
3. 使えなければ、利用可能 provider の `ask` default model を選ぶ

補足:

- `settings.digest_model` や `settings.summary_model` には落ちません
- 実行時に選ばれた Ask model が失敗した場合、別 model へ自動で切り替える fallback はありません
- Anthropic service 内の個別 fallback はありますが、Ask 全体として他 provider へ自動退避する仕組みではありません

関連実装:

- `api/internal/handler/ask.go`

### 5-4. process-item 系の選択ロジック

facts / summary / check 系は Inngest 側で runtime を解決します。

流れ:

1. 用途ごとの override model を確認
2. override model が指定されている場合は、その model の provider を確定
3. その provider に対応する API key をそのユーザーが持っていれば実行
4. override model が未指定の場合だけ、cost-efficient provider 順に default model を選ぶ

つまり、

- 明示的に選んだ model がある
  - その model/provider を使う
  - key がなければその場で失敗し、別 provider へは自動で逃げません
- model が未指定
  - 実装側で定義した cost-efficient provider の優先順に、使える key を持つ provider の default model を選びます

この fallback 候補には OpenRouter も含まれます。

ここでいう cost-efficient provider 順は、単純なリアルタイム価格順ではなく、実装側で定義した固定の優先順です。

関連実装:

- `api/internal/inngest/functions.go`
- `api/internal/inngest/process_item_flow.go`
- `api/internal/inngest/check_flow.go`

---

## 6. Worker 実行と usage 収集

### 6-1. Worker の役割

Worker は stateless に寄せています。

- DB は持たない
- API から渡された model と API key で実行する
- usage は response から計算して返す

### 6-2. OpenRouter 実装

OpenRouter は OpenAI 互換 transport を使っています。

特徴:

- base URL: `https://openrouter.ai/api/v1/chat/completions`
- OpenRouter alias を実行直前に本来の model ID に解決
- ただし usage メタでは alias を維持して返す

この「alias を usage 側には残す」設計が、後段の価格正規化で重要になります。

関連実装:

- `worker/app/services/openrouter_service.py`
- `worker/app/services/llm_catalog.py`

---

## 7. API 側のコスト正規化

### 7-1. 基本原則

Sifto の LLM cost は、請求 API から取得した実額ではなく、usage token を基準に catalog 価格で再計算した推定値です。

保存する主な項目:

- provider
- model
- pricing_model_family
- pricing_source
- input_tokens
- output_tokens
- cache_creation_input_tokens
- cache_read_input_tokens
- estimated_cost_usd

### 7-2. なぜ API 側で再計算するのか

Worker も usage からコスト推定を返しますが、最終的には API 側 catalog を基準に正規化します。

理由:

- provider ごとの現在の catalog を 1 箇所で統一したい
- OpenRouter snapshot の動的価格を反映したい
- 同名モデルでも provider 差分を潰さず比較したい

つまりこれは単なる見積もりではなく、**比較可能な Usage データを作るための正規化層**です。

### 7-3. 推定方法

API 側では次を catalog の価格で再計算します。

- non-cached input
- output
- cache read
- cache write

そのため `LLM Usage` / `LLM Analysis` に出るコストは、基本的に `estimated cost` です。

関連実装:

- `api/internal/service/llm_usage_normalize.go`

### 7-4. OpenRouter と重複 model のコスト

OpenRouter と直接接続 provider の model ID が重複している場合でも、

- worker は alias を usage の `model` に残す
- API は `openrouter::...` を OpenRouter model として catalog lookup する
- OpenRouter snapshot 価格で `estimated_cost_usd` を再計算する

これにより、同じ表示名のモデルでも

- 直接接続 provider 版
- OpenRouter 版

を Usage / Analysis 上で分離できます。

関連実装:

- `worker/app/services/openrouter_service.py`
- `api/internal/service/llm_usage_normalize.go`
- `api/internal/service/llm_usage_normalize_test.go`

---

## 8. 運用 UI への反映

### 8-1. OpenRouter Models 画面

OpenRouter Models は、OpenRouter 同期結果を一覧表で見る専用画面です。

主な機能:

- 最終同期時刻
- fetched / accepted 件数
- 手動同期
- provider フィルタ
- フリーテキスト検索
- 全モデル一覧表
- provider / model / context / pricing / params の列ソート
- 行クリックで詳細モーダル
- 日本語訳説明文の確認

設計意図:

- モデル数が多いのでカード UI は使わない
- 1 モデル 1 行で一覧性を優先
- 詳細はモーダルへ逃がす

関連実装:

- `web/src/app/(main)/openrouter-models/page.tsx`

### 8-2. Settings 画面のモデル選択 UI

OpenRouter 導入後、モデル数が大きく増えたため、従来の select だけでは扱いにくくなりました。

そのため Settings のモデル選択はモーダル方式に変えています。

機能:

- 用途ごとの「選択」ボタン
- provider フィルタ
- フリーテキスト検索
- モデル行クリック
- 確認ステップ
- `はい` で確定して閉じる
- `いいえ` で戻る

これにより OpenRouter の大量モデルも選びやすくしています。

関連実装:

- `web/src/components/settings/model-select.tsx`
- `web/src/app/(main)/settings/page.tsx`

### 8-3. Usage / Analysis 画面

すべての用途で保存された usage は共通の `llm_usage_logs` に入ります。

OpenRouter も通常 provider と同じように集計されます。

画面:

- `LLM Usage`
- `LLM Analysis`

ここでは provider 表示も `OpenRouter` として出ます。

関連実装:

- `api/internal/repository/llm_usage_logs.go`
- `web/src/app/(main)/llm-usage/page.tsx`
- `web/src/app/(main)/llm-analysis/page.tsx`

---

## 9. OpenRouter 同期と通知

### 9-1. 手動同期

OpenRouter Models 画面から手動同期できます。

流れ:

1. `POST /openrouter-models/sync`
2. OpenRouter models API 取得
3. DB に snapshot 保存
4. 動的 catalog 更新
5. 説明文の日本語訳は非同期で進行

補足:

- 説明文の日本語訳には `api` 側の `OPENAI_API_KEY` が必要です
- key がない場合は英語 description のまま使います
- 翻訳処理は OpenAI 系サービス層の既存実装を流用しています

関連実装:

- `api/internal/handler/openrouter_models.go`
- `api/internal/service/openrouter_catalog.go`
- `api/internal/service/openai_embeddings.go`

### 9-2. 日次同期

日次同期も持っています。

流れ:

1. Inngest の `sync-openrouter-models`
2. 最新モデル一覧取得
3. snapshot 保存
4. 日本語 description の補完
5. 動的 catalog 更新
6. 新規モデル追加差分を通知

関連実装:

- `api/internal/inngest/functions.go`

### 9-3. 新規モデル通知

前回の成功 snapshot と比較して、新規 model ID が増えていたら通知します。

通知手段:

- push
- email

関連実装:

- `api/internal/inngest/functions.go`
- `api/internal/service/resend.go`

---

## 10. まとめ

つまり Sifto の LLM 基盤は、複数 provider と OpenRouter を横断しながら、実験モデルの導入しやすさと、運用時の比較・分析しやすさを両立させるための設計になっています。
