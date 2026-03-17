# 2026-03-17_OpenRouter追加設計

## 目的

Sifto に `OpenRouter` を独立 provider として追加し、単一の API キーで広めの実験モデル群を全用途に使えるようにする。

狙いは次の 3 点。

- 新モデル試験の導入コストを下げる
- 直 provider を増やし続ける運用負荷を下げる
- `facts / summary / digest_cluster_draft / digest / ask / source_suggestion / checks` を横断して同じ provider で比較できるようにする

## 方針

### 1. OpenRouter は独立 provider として扱う

- provider id は `openrouter`
- API key はユーザー別に 1 つだけ保存する
- `LLM Usage` と `LLM Analysis` では provider は `OpenRouter` として集計する
- model は `google/gemini-2.5-flash` のような OpenRouter 上のフル ID をそのまま持つ

### 2. モデル一覧は OpenRouter models API 由来にする

- 他 provider のような完全手動 catalog 運用にはしない
- OpenRouter はモデル数と変動が大きいので、`models API` を供給源とする
- ただし実行時に毎回 API を叩かず、Sifto 内に日次スナップショットを保持する

### 3. 採用基準は広めにする

- 実験枠なので厳しく絞りすぎない
- 基本方針は「テキスト生成に使えるモデルは採用」
- 除外対象は次を想定
  - embedding 専用
  - moderation 専用
  - reranker 専用
  - 画像/音声専用で chat/completions 互換がないもの

### 4. UI は provider ごとにグループ化する

- `ModelSelect` では OpenRouter 配下のモデルを上位 provider ごとにグループ化する
- 例:
  - OpenAI
  - Google
  - Anthropic
  - Meta
  - Qwen
  - DeepSeek
  - Mistral
  - Nvidia
  - Bytedance Seed
  - Liquid
- 大量のモデルを載せても検索しやすい構成を維持する

## データ設計

### 1. ユーザー設定

既存の provider 別 API key と同じ方式で、`user_settings` に OpenRouter 用カラムを追加する。

- `openrouter_api_key_enc`
- `openrouter_api_key_last4`

### 2. OpenRouter モデルスナップショット

ファイル更新ではなく DB に保存する。理由は本番環境での運用性と multi-instance 互換性のため。

新規テーブル案:

- `openrouter_model_snapshots`
  - `id`
  - `fetched_at`
  - `model_id`
  - `canonical_slug`
  - `provider_slug`
  - `display_name`
  - `description`
  - `context_length`
  - `pricing_json`
  - `supported_parameters_json`
  - `architecture_json`
  - `top_provider_json`
  - `modality_flags_json`
  - `is_text_generation`
  - `is_active`

実運用では「最新 fetched_at の集合」を catalog 相当として扱う。

### 3. 実行時の pricing

- `LLM Usage` では OpenRouter 同期結果の pricing を使う
- pricing は snapshot 保存時にそのまま正規化して持つ
- 実行ログには `pricing_source=openrouter_snapshot_YYYY_MM_DD` のような形で残す

## 同期設計

### 1. 日次同期

- Inngest か cron で 1 日 1 回実行
- OpenRouter models API を取得
- 対象モデルをフィルタ
- 正規化して DB に最新スナップショットとして保存

### 2. 手動同期

- 設定画面に `OpenRouter モデル更新` ボタンを追加
- 管理者または現在ユーザー起点で同期を実行できるようにする
- 失敗時は toast と server log に理由を出す

### 3. フォールバック

- 同期失敗時も前回のスナップショットを使い続ける
- 実行時に OpenRouter models API が落ちていても、既存モデル選択や過去設定は壊さない

## API / Worker 設計

### 1. provider 実装

- worker 側は OpenAI 互換 transport を流用する
- base URL は OpenRouter の chat/completions endpoint を使う
- provider 名は `openrouter`

### 2. 全用途対応

OpenRouter は次の全用途で選択可能にする。

- `facts`
- `summary`
- `digest_cluster_draft`
- `digest`
- `ask`
- `source_suggestion`
- `facts_check`
- `faithfulness_check`

### 3. capability の扱い

OpenRouter の `supported_parameters` は snapshot に保持するが、初期段階では capability 判定を過度に厳しくしない。

初期ルール:

- chat/completions 互換なら選択候補に出す
- `structured_outputs` や `tools` の有無は補助情報として表示
- ただし Sifto 側で必須 capability を要求する用途では、既存 capability チェックと整合するよう最小限の判定を追加する

## UI 設計

### 1. Settings

- API key セクションに `OpenRouter API Key (Per User)` を追加
- モデル選択では OpenRouter 群を provider ごとにグループ化して表示
- 既存の catalog provider と同様に全用途で選択可能

### 2. LLM Usage / LLM Analysis

- provider 表示は `OpenRouter`
- model 表示はフル model id のまま
- 集計は直 provider と同じ形式で扱う

### 3. モデル比較表

- OpenRouter モデルは動的スナップショットを使う
- 既存の静的 `shared/llm_catalog.json` と完全に同じ扱いにはしない
- 比較 UI では「静的 provider 群 + OpenRouter 動的群」の合成結果を見せる

## 実装ステップ

1. DB migration
   - `user_settings` に OpenRouter API key カラム追加
   - OpenRouter snapshot テーブル追加

2. API / repository
   - OpenRouter API key CRUD
   - snapshot 保存/取得 repo
   - settings payload への反映

3. 同期処理
   - OpenRouter models API 取得 client
   - 正規化/フィルタ
   - 日次同期ジョブ
   - 手動同期 endpoint

4. Worker / routing
   - `openrouter` provider 追加
   - 全用途で key を渡せるようにする

5. Web
   - settings の API key UI
   - ModelSelect の OpenRouter グルーピング
   - モデル比較表への OpenRouter 動的統合

6. Usage / Analysis
   - provider 表示
   - pricing source の表示整合

## リスク

### 1. モデル数が多すぎる

- 対策:
  - provider ごとにグループ化
  - 検索前提 UI
  - activity や text generation 判定で最低限の除外

### 2. capability 情報が不完全

- 対策:
  - 初期は補助情報として扱う
  - strict schema 前提の hard requirement を OpenRouter 全体には課しすぎない

### 3. pricing の変動

- 対策:
  - snapshot 時点の pricing を実行時に使う
  - pricing source をログに残す

### 4. OpenRouter API 障害

- 対策:
  - snapshot キャッシュ継続利用
  - 同期失敗でも既存 catalog は維持

## 今回やらないこと

- OpenRouter 全モデルの完全自動推薦
- モデル品質の自動スコアリング
- OpenRouter の realtime / multimodal 専用機能の対応
- 既存 static catalog 全体の dynamic catalog 置き換え

## 推奨方針

まずは OpenRouter だけを「動的モデル棚を持つ独立 provider」として追加する。  
これにより、Sifto は既存 provider の安定運用を保ったまま、新モデル試験を 1 API key で高速に回せるようになる。
