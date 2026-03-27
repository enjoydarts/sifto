# AIナビ朝昼夜brief 設計

## 概要

朝・昼・夜の 3 回、AI ナビゲーターがその時間帯向けの専用 brief を自動生成し、push 通知で届ける。

brief は既存の `Today briefing` の派生ではなく、`AI ナビ brief` として独立した保存対象にする。通知タップ時は専用詳細ページを開き、導入文、総括コメント、コメント付き記事 10 本を読めるようにする。さらに、その 10 本を要約読み上げキューへ一括追加できるようにする。

## 目的

- Sifto を朝昼夜の定時接点で使う理由を作る
- AI ナビゲーターを単発の推薦ではなく、定時配信の体験へ広げる
- 通知から `読む / 聞く` までを一続きの導線にする
- 後から見返せる brief 履歴を持たせる

## スコープ

### 含む

- 朝昼夜 3 slot の AI ナビ brief 自動生成
- brief 保存
- push 通知送信
- AI ナビ brief 専用一覧ページ
- AI ナビ brief 専用詳細ページ
- 詳細ページから `10本を要約読み上げキューに追加`
- 機能全体の ON/OFF 設定

### 含まない

- 時間帯ごとの個別 ON/OFF
- 時間帯ごとの個別 model 設定
- brief の手動編集
- 記事単位の個別キュー追加
- 既存 briefing 一覧との統合

## 前提

- model 設定は既存 AI ナビゲーター設定をそのまま使う
- persona 設定も既存 AI ナビゲーター設定をそのまま使う
- brief の記事候補は、その時間帯の `直近未読` から取る
- 記事候補条件は AI ナビ寄りに揃える
  - `summarized`
  - `unread`
  - `later` 除外
  - `summary` あり

## ユーザー体験

### 通知

- 朝昼夜の定刻に AI ナビ brief を生成する
- push 通知のタイトルは AI が生成した brief タイトルを使う
- 本文は brief の導入文または総括コメントの短縮版を使う
- タップ先は AI ナビ brief の専用詳細ページ

### 詳細ページ

- AI 生成タイトル
- slot 表示 `朝 / 昼 / 夜`
- 生成時刻
- persona
- 導入文
- 総括コメント
- コメント付き記事 10 本
- `10本を要約読み上げキューに追加`

### 一覧ページ

- AI ナビ brief の配信履歴を一覧表示する
- 各カードに表示するもの
  - 朝 / 昼 / 夜
  - 生成時刻
  - タイトル
  - 要約 1 行

## アプローチ比較

### 案1: 定時バッチ生成 + 保存 + 通知

- 通知前に生成済みなので体験が安定する
- 一覧や履歴とも自然につながる
- push 再送制御や失敗管理もしやすい

### 案2: 通知タップ時にオンデマンド生成

- 事前ジョブは軽い
- ただし通知から開いた直後に待ちが発生する
- 一覧や履歴を持たせるには別途保存が必要

### 採用: 案1

- `通知体験`
- `あとから見返す履歴`
- `詳細ページから要約読み上げへ流す導線`

を全部成立させるには、最初から保存される生成物として持つのが一番素直。

## データ設計

### ai_navigator_briefs

用途:
- AI ナビ brief 一覧 / 詳細の本体
- 通知送信管理

想定フィールド:

- `id`
- `user_id`
- `slot`
  - `morning`
  - `noon`
  - `evening`
- `status`
  - `queued`
  - `generated`
  - `failed`
  - `notified`
- `title`
- `intro`
- `summary`
- `persona`
- `model`
- `source_window_start`
- `source_window_end`
- `generated_at`
- `notification_sent_at`
- `error_message`
- `created_at`
- `updated_at`

補助インデックス:

- `(user_id, generated_at desc)`
- `(user_id, slot, generated_at desc)`
- `(user_id, status, generated_at desc)`

### ai_navigator_brief_items

用途:
- brief に含まれる記事 10 本の snapshot

想定フィールド:

- `id`
- `brief_id`
- `rank`
- `item_id`
- `title_snapshot`
- `translated_title_snapshot`
- `source_title_snapshot`
- `comment`
- `created_at`

補助インデックス:

- `(brief_id, rank)`

### snapshot の考え方

- `item_id` は参照として保持する
- ただしタイトルやコメントは brief 生成時の snapshot を持つ
- item 側の内容が後で変わっても、brief 自体は生成当時の見え方を保つ

## 候補抽出

### slot と window

- `morning`
  - 前回 evening 以降、または日跨ぎ後の基準時刻以降
- `noon`
  - morning 以降
- `evening`
  - noon 以降

実装では、各 brief に `source_window_start / source_window_end` を保存し、どの範囲から選んだかを明示する。

### 候補条件

- `summarized`
- `unread`
- `later` 除外
- `summary` あり
- 時間帯 window 内

### 候補数

- LLM に渡す候補は 24 件程度
- そこからコメント付き 10 件を選ばせる

## 生成フォーマット

brief 本文は最低限次を返す:

- `title`
- `intro`
- `summary`
- `items`
  - `item_id`
  - `comment`

制約:

- 記事数は常に 10
- 10 本すべてにコメントを付ける
- タイトルは毎回 AI が自由につける

## API 設計

### 読み取り

- `GET /ai-navigator-briefs`
  - 一覧
  - filter: slot, limit, cursor
- `GET /ai-navigator-briefs/{id}`
  - 詳細

### 書き込み / 補助

- `POST /ai-navigator-briefs/{id}/summary-audio-queue`
  - brief の 10 本を要約読み上げキューへ一括追加

初回は管理系 API を増やしすぎず、生成ジョブは server / worker から直接 repository / service を叩く構成でよい。

## 通知設計

- `kind`: `ai_navigator_brief`
- クリック先: `/ai-navigator-briefs/{id}`
- タイトル: brief の `title`
- 本文: `intro` または `summary` の短縮版
- 重複抑制:
  - slot ごとに 1 回
  - 同一 brief へ二重送信しない

## UI 設計

### 一覧ページ

- ページ名: `AIナビ brief`
- カード情報
  - slot tag
  - 生成時刻
  - タイトル
  - summary 1 行
- 並び順は `generated_at desc`

### 詳細ページ

- hero:
  - タイトル
  - slot
  - 生成時刻
  - persona
- 本文:
  - 導入文
  - 総括コメント
- 記事リスト:
  - 1〜10
  - タイトル
  - ソース名
  - コメント
  - 記事詳細への導線
- 操作:
  - `10本を要約読み上げキューに追加`

## 設定

- 既存設定に `AIナビ朝昼夜brief` の機能 ON/OFF を追加
- persona / model は既存 AI ナビ設定を使う
- 初回は時間帯別 persona 設定や個別時刻設定は入れない

## 実行方式

- 定時ジョブで `morning / noon / evening` を判定
- user ごとに feature ON を確認
- 候補を抽出
- AI ナビ brief を生成して保存
- push 通知を送る
- 成功 / 失敗を status に反映

## エラー処理

- 候補不足:
  - 10 本未満なら生成をスキップ、または short brief に落とす選択が必要
  - 初回は `10 本揃わない時は生成しない` が安全
- LLM 失敗:
  - `failed`
  - `error_message` 保存
- 通知失敗:
  - brief 自体は `generated`
  - `notification_sent_at` は空
  - 再送対象として扱えるようにする

## テスト観点

- slot ごとの候補抽出 window が正しい
- `summarized / unread / later 除外 / summary あり` 条件が効く
- 10 本の item snapshot が保存される
- 一覧 / 詳細 API が正しく返る
- `10本を要約読み上げキューに追加` が順序通りに積まれる
- push payload の click URL が専用詳細ページになる
- feature OFF ユーザーには配信しない

## 段階導入

### Phase 1

- DB
- service / repository
- 一覧 / 詳細 API
- 詳細ページ
- 一括キュー追加

### Phase 2

- 定時ジョブ生成
- push 通知

### Phase 3

- 一覧ページ polish
- 失敗管理 / 再送改善

