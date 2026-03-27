# 再生履歴 / 続きから再生 設計

## 概要

共通音声プレイヤーで扱う `要約連続再生` と `音声ブリーフィング` について、再生セッションを永続化し、ページ再読み込みや日をまたいでも `続きから再生` できるようにする。

同時に、専用の `再生履歴` ページを追加し、最近の再生セッションを `途中 / 完了 / 中断` の状態つきで確認できるようにする。

今回は分析基盤まで大きく広げず、`復元に必要な session snapshot` を中核にしつつ、将来の分析に使える最低限の `playback event` も保存する。

## 目的

- 要約連続再生の途中離脱後に、そのまま再開できるようにする
- 音声ブリーフィングも途中位置から再生再開できるようにする
- 共通プレイヤー導入後の再生体験を、SPA 内だけでなく reload / 翌日利用にも広げる
- ユーザーが最近聞いていた内容を一覧で確認できるようにする
- 将来的な完聴率や離脱地点分析に備え、再生イベントの足場を作る

## スコープ

### 含む

- `要約連続再生` の再生セッション永続化
- `音声ブリーフィング` の再生セッション永続化
- `続きから再生` の復元 API / UI
- `再生履歴` 専用ページ
- 最低限の再生イベント保存

### 含まない

- 複数端末間のリアルタイム同期
- プレイヤーの別ウィンドウ化
- セッション履歴の削除 UI
- 詳細な分析ダッシュボード
- 再生イベントを直接見る管理画面

## 前提

- 共通プレイヤーは `web/src/components/shared-audio-player/provider.tsx` の単一 `<audio>` を使う
- `要約連続再生` はローカル queue window を持っている
- `音声ブリーフィング` は briefing id と audio url を共通プレイヤーへ渡して再生している
- 既読判定や先読みの現在仕様は維持する

## ユーザー体験

### 続きから再生

- `要約連続再生` は、再生開始時点の queue 順を保存する
- 再開時は、その保存済み queue を使って同じ続きから再生する
- 再開時に live の未読一覧へ差し戻さない
- `音声ブリーフィング` は、最後に聞いていた briefing 回と再生位置から再開する

### 再生履歴ページ

- 専用ページを追加する
- フィルタは `すべて / 要約読み上げ / 音声ブリーフィング`
- 各カードに表示するもの
  - タイトル
  - モード
  - 状態 `途中 / 完了 / 中断`
  - 最終再生時刻
  - 進捗率
  - summary の場合は残り件数と現在記事
  - audio briefing の場合は再生位置と総時間
- 各カードから `続きから再生` を実行できる
- 詳細ページへの導線も必要に応じて表示する

## アプローチ比較

### 案1: session snapshot のみ

- 復元と履歴一覧には十分
- 実装は軽い
- ただし将来分析の材料が弱い

### 案2: event log のみ

- 分析には強い
- ただし `続きから再生` の復元に再構築ロジックが必要
- 初回実装としては重い

### 採用: session snapshot + 最低限の event log

- 復元は snapshot で素直に実現する
- 履歴一覧も snapshot ベースで構築する
- event log は将来の分析用に残す
- 初回機能としての複雑さを抑えながら、後の拡張余地を残す

## データ設計

### playback_sessions

用途:
- `続きから再生` の復元元
- 再生履歴一覧の本体

想定フィールド:

- `id`
- `user_id`
- `mode`
  - `summary_queue`
  - `audio_briefing`
- `status`
  - `in_progress`
  - `completed`
  - `interrupted`
- `title`
- `subtitle`
- `current_position_sec`
- `duration_sec`
- `progress_ratio`
- `started_at`
- `updated_at`
- `completed_at`
- `resume_payload` JSONB

補助インデックス:
- `(user_id, mode, updated_at desc)`
- `(user_id, status, updated_at desc)`

### resume_payload

#### summary_queue

- `queue_kind`
  - `unread | later | favorite`
- `queue_item_ids`
  - 保存時点の queue 順
- `current_item_id`
- `current_queue_index`
- `current_item_offset_sec`
- `excluded_item_ids`
  - 必要なら保持し、再構築の一貫性を高める

#### audio_briefing

- `briefing_id`
- `current_offset_sec`

### playback_events

用途:
- 将来の分析の最低限の足場

想定フィールド:

- `id`
- `session_id`
- `user_id`
- `mode`
- `event_type`
  - `started`
  - `paused`
  - `resumed`
  - `stopped`
  - `completed`
  - `replaced`
  - `progressed`
- `position_sec`
- `payload` JSONB
- `created_at`

初回は保管中心で、複雑な集約はまだ作らない。

## 状態遷移

### in_progress

- 再生開始時に作成する
- pause / resume 中は継続

### completed

- `要約連続再生`: 保存済み queue を最後まで再生し切った時
- `音声ブリーフィング`: 末尾まで再生した時

### interrupted

- 明示的に停止した時
- 別の再生を始めて上書きした時
- 完了前に離脱して、次回開始時に旧セッションを閉じる時

### replaced

- event としては残す
- session status 自体は `interrupted` に寄せる

## 保存タイミング

### session 更新

- 再生開始
- pause
- stop
- ended
- 一定間隔
- page unload / visibility change

### event 記録

- started
- paused
- resumed
- stopped
- completed
- replaced
- progressed

`progressed` は高頻度にしすぎず、例えば 15 秒ごと、または 30 秒ごとに間引く。

## API 設計

### 読み取り

- `GET /playback-sessions/latest`
  - mode ごとの最新 session を返す
- `GET /playback-sessions`
  - 履歴一覧
  - filter: mode, status, limit, cursor
- `GET /playback-sessions/{id}`
  - 履歴カード詳細が必要なら追加

### 書き込み

- `POST /playback-sessions`
  - 新規 session 開始
- `PATCH /playback-sessions/{id}`
  - 進捗更新
- `POST /playback-sessions/{id}/complete`
  - 完了化
- `POST /playback-sessions/{id}/interrupt`
  - 中断化
- `POST /playback-events`
  - event 追加

初回は API 数を増やしすぎず、session 更新 API に event 記録を内包してもよい。

## フロントエンド設計

### 共通プレイヤー

`SharedAudioPlayerProvider` に以下を追加する。

- 現在の remote session id
- session 保存ロジック
- resume payload の組み立て
- mode 切替時の旧 session interrupt 処理
- 起動時の latest session 復元導線用 query

### 続きから再生の復元

#### summary_queue

- `resume_payload.queue_item_ids` から queue を再構築する
- 必要な item 詳細は再取得する
- 音声そのものの prefetch は復元しない
- 復元後に current item の offset まで seek する

#### audio_briefing

- `briefing_id` を再取得する
- 共通プレイヤー起動後に offset へ seek する

### 再生履歴ページ

ページ候補:
- `/playback-history`

構成:
- page header
- mode filter tabs
- status badge
- progress bar / progress ratio
- `続きから再生`
- `詳細を開く`

## エラーハンドリング

- 保存済み summary queue の item が削除済み
  - スキップ可能な item は飛ばす
  - 全件消えていれば再開不可として履歴カード上で案内
- audio briefing が削除済み or 非公開化
  - 再開不可として履歴カード上で案内
- duration 不明
  - 進捗率は nullable にする
- resume payload 不整合
  - session を `interrupted` として扱い、再生は開始しない

## テスト

### API / repository

- session 作成
- latest session 取得
- mode 別最新取得
- completed / interrupted 更新
- resume payload 保存と取得
- event 保存

### フロントエンド

- summary queue を途中位置から復元
- audio briefing を途中位置から復元
- 別再生開始時に旧 session が interrupted になる
- 完了時に completed になる
- 履歴一覧の mode filter
- 進捗率と残り件数表示

### 手動確認

- 要約連続再生を途中で止めて reload 後に再開
- 翌日でも続きから再開
- 音声ブリーフィングを途中で止めて再開
- 別モード再生で旧 session が中断になる
- 履歴一覧に `途中 / 完了 / 中断` が意図どおり並ぶ

## 導入順

1. DB schema と API モデル追加
2. session 保存 API 実装
3. 共通プレイヤーへの保存統合
4. `続きから再生` 復元
5. 履歴一覧ページ
6. 最低限の event 保存

## 将来拡張

- 完聴率 / 離脱率の可視化
- summary queue ごとの完走率比較
- プレイヤー上の `前回の続きを再生`
- 履歴削除
- デバイス別再生傾向
