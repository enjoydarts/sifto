# Poe Usage永続化設計メモ

## 結論

Poe Usage は live fetch ベースのままではなく、`points_history` を Sifto 側に永続化して、画面は DB 集計を主に使う設計へ切り替える。

`current_balance` だけはリアルタイム性が高いので live fetch 維持でもよいが、`利用履歴` と `期間集計` は永続化前提にする。

## 背景

現状の `Poe Models > Usage` は、Poe Usage API を毎回その場で叩いて集計している。

この方式の問題は次のとおり。

- `今月` や `先月` のような期間切替を増やしにくい
- 毎回 30 日分の remote fetch と集計が走る
- 直近100件表示と期間集計の責務が混ざる
- Poe API の `points_history` は最大 30 日なので、長期保持にはならない

## 方針

### 1. 永続化を追加する

Poe Usage API の `points_history` を Sifto DB に保存する。

保存対象:

- `query_id`
- `user_id`
- `bot_name`
- `created_at`
- `cost_usd`
- `raw_cost_usd`
- `cost_points`
- `cost_breakdown_in_points`
- `usage_type`
- `chat_name`
- `synced_at`

一意制約は `user_id + query_id` にする。

### 2. 画面は DB 集計ベースにする

`Poe Models > Usage` は DB から集計して返す。

初期の期間プリセットは次で十分。

- 今日
- 昨日
- 7日
- 14日
- 30日
- 今月
- 先月

### 3. current_balance は live fetch 併用

`current_balance` は「いま何ポイント残っているか」を見る用途なので、毎回 Poe API を叩いてよい。

つまり:

- `残高`: live
- `履歴/集計`: DB

の二層に分ける。

## 推奨テーブル

### poe_usage_sync_runs

用途:

- 同期ジョブの状態管理
- エラー確認
- 最終成功時刻の把握

想定カラム:

- `id`
- `user_id`
- `trigger_type`
  - `manual`
  - `page_load`
  - `scheduled`
- `status`
  - `running`
  - `success`
  - `failed`
- `started_at`
- `finished_at`
- `last_progress_at`
- `fetched_count`
- `inserted_count`
- `updated_count`
- `error_message`
- `cursor_starting_after`
- `oldest_entry_at`
- `newest_entry_at`

### poe_usage_entries

用途:

- Poe Usage API の実データ保持
- 期間集計
- モデル別集計
- 履歴テーブル表示

想定カラム:

- `user_id`
- `query_id`
- `bot_name`
- `created_at`
- `cost_usd`
- `raw_cost_usd`
- `cost_points`
- `cost_breakdown_in_points_json`
- `usage_type`
- `chat_name`
- `synced_at`
- `created_row_at`
- `updated_row_at`

インデックス:

- unique `(user_id, query_id)`
- index `(user_id, created_at desc)`
- index `(user_id, bot_name, created_at desc)`
- index `(user_id, usage_type, created_at desc)`

## 同期戦略

### 基本方針

Poe Usage API は 30 日までしか取れないので、`定期同期` が前提になる。

推奨:

- 画面を開いた時に軽く同期
- バックグラウンドで定期同期
- 少なくとも 1 日 1 回は全ユーザー分を更新

### 初期スコープ

まずは次で十分。

- 手動 sync endpoint
- 画面表示時 sync
- 直近 30 日を pageing しながら upsert

その後に scheduled sync を追加する。

### upsert ルール

`query_id` は同一リクエストを識別できるので、同じ `user_id + query_id` が来たら upsert でよい。

これで重複取得に強くなる。

## API 設計

### GET /api/poe-models/usage

live fetch ではなく DB 集計結果を返す。

クエリ案:

- `range=today|yesterday|7d|14d|30d|mtd|prev_month`
- `entries_limit=50|100|200`
- `usage_type=all|api|chat`

返却案:

- `configured`
- `current_point_balance`
- `last_synced_at`
- `sync_status`
- `summary`
- `model_summaries`
- `entries`
- `truncated`

### POST /api/poe-models/usage/sync

Poe Usage の手動同期を開始する。

返却案:

- `sync_run`
- `inserted_count`
- `updated_count`

## 集計仕様

### summary

- 合計 calls
- 合計 points
- 合計 USD
- avg points / call
- avg USD / call
- 最新利用時刻

### model_summaries

- `bot_name`
- `entry_count`
- `total_cost_points`
- `total_cost_usd`
- `avg_cost_points_per_call`
- `avg_cost_usd_per_call`
- `latest_entry_at`

### entries

初期表示は `100件`。

表示件数切替:

- 50
- 100
- 200

## UI 方針

`Poe Models > Usage` に期間切替 UI を追加する。

初期表示:

- 今日
- 昨日
- 7日
- 14日
- 30日
- 今月
- 先月

表示の考え方:

- KPI は選択期間に追従
- モデル別消費上位も選択期間に追従
- 最近の利用履歴も選択期間内で新しい順

`current_balance` だけは期間に依存しない live 値として上部に固定表示する。

## LLM Usage との関係

Poe Usage の永続化は `billing truth` に近いが、`個別の LLM request` との厳密な 1:1 紐付けには使いにくい。

理由:

- Poe の通常 API response に `query_id` が返らない
- Usage API 側の `query_id` と request response を厳密には結べない

したがって位置づけは次のとおり。

- `llm_usage_logs`: Sifto 内の request 実行ログ
- `poe_usage_entries`: Poe 側の billing / consumption 実績

この2つは用途を分ける。

## 実装順

1. migration 追加
2. repository 追加
3. Poe Usage sync service 追加
4. handler を DB 集計ベースへ変更
5. `current_balance` の live fetch 併用
6. UI に期間切替追加
7. 手動 sync ボタン追加
8. scheduled sync は後段

## 注意点

- Poe Usage API は 30 日上限なので、同期失敗を長く放置しない
- `cost_usd` は string 原本も保持しておく
- `bot_name` は Poe 名なので、Sifto catalog の `poe::<model>` との完全一致は期待しすぎない
- 同一期間でも `current_balance` は変動するので、履歴集計とは別物として扱う

## 結論の再整理

Poe Usage は live 表示だけだと拡張性が低い。`points_history` の永続化を入れて、`今日 / 昨日 / 7日 / 14日 / 30日 / 今月 / 先月` で集計できるようにするのが正しい。

初手では `履歴/集計は DB`、`残高は live` の分離が最も実装効率と使い勝手のバランスが良い。
