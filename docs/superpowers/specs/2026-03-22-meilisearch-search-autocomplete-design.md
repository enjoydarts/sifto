# Sifto Meilisearch 検索オートコンプリート 設計

作成日: 2026-03-22

## 1. 目的

Items 一覧の全文検索モーダルに、検索候補を事前提示するオートコンプリートを追加する。候補提示は単なる query 補完ではなく、`article / source / topic` を横断した発見導線として機能させる。

今回の対象は Items 一覧の検索モーダルのみとする。

## 2. 体験目標

- 数文字入力した段階で、続きを打たずに候補から探し始められる
- 記事タイトルだけでなく、source や topic を直接入口にできる
- source / topic は free text ではなく filter として適用される
- 候補が多すぎてノイズにならない
- 既存の全文検索と矛盾しない

## 3. スコープ

### 3.1 含む

- 既存検索モーダルへの候補一覧追加
- 専用 `search_suggestions` index の追加
- `article / source / topic` 候補の生成
- source / topic 選択時の filter 即時適用
- 既存データの候補 index バックフィル

### 3.2 含まない

- Notes / Highlights の候補化
- Ask UI 側のオートコンプリート
- 入力履歴や人気候補の学習最適化
- typo correction の詳細 tuning

## 4. 採用方針

### 4.1 なぜ専用 index か

既存 `items` index をそのまま候補生成に使うと、`summary / facts / content_text` 由来の断片が候補に混ざりやすい。候補として自然に見せたいのは、短く正規化されたラベルであり、全文検索用 index とは責務が異なる。

そのため、`search_suggestions` 専用 index を追加する。

### 4.2 候補種別

- `article`
  - `title / translated_title` ベース
- `source`
  - source title ベース
- `topic`
  - summarized item の topics をユーザー単位で統合

## 5. 候補 index 設計

### 5.1 index 名

- `search_suggestions`

### 5.2 ドキュメント構造

```json
{
  "id": "article:item_id | source:source_id | topic:user_id:topic",
  "user_id": "user_uuid_text",
  "kind": "article | source | topic",
  "label": "Kubernetes の再起動挙動の変更",
  "normalized": "kubernetes の再起動挙動の変更",
  "score": 100,
  "item_id": "uuid-or-null",
  "source_id": "uuid-or-null",
  "topic": "AI",
  "article_count": 42,
  "updated_at": "2026-03-22T00:00:00Z"
}
```

### 5.3 kind ごとの意味

- `article`
  - `label`: title もしくは translated_title を優先表示
  - `item_id`: 必須
  - `source_id`: 付与
  - `topic`: なし
  - `article_count`: なし
- `source`
  - `label`: source title
  - `source_id`: 必須
  - `article_count`: その source の summarized 記事数
- `topic`
  - `label`: topic 名
  - `topic`: 必須
  - `article_count`: その topic を持つ summarized 記事数

### 5.4 searchableAttributes

優先順:

1. `label`
2. `normalized`

候補 index は短いラベル検索専用にし、本文断片は入れない。

### 5.5 filterableAttributes

- `user_id`
- `kind`
- `source_id`
- `topic`

## 6. ランキングと表示数

### 6.1 全体件数

- 候補合計: `10`

### 6.2 種別ごとの上限

- `article`: 最大 `6`
- `source`: 最大 `2`
- `topic`: 最大 `2`

### 6.3 空き枠ルール

- source/topic が少なければ、その空きは article に回す
- ただし source/topic は候補が存在する限り優先枠を試みる

### 6.4 事前 score

候補生成時に種別ごとの基礎 score を持つ。

例:

- article: 100
- source: 120
- topic: 110

最終的な並びは Meilisearch relevance を軸にしつつ、アプリ側で上限配分を適用する。

## 7. Topic の正規化

topic 候補は、同一 user 内で重複統合する。

例:

- `AI`
- `Ai`
- `ai`

は正規化後に 1 候補へまとめる。topic は filter 導線なので、記事文脈ごとに分けない。

## 8. UX 設計

### 8.1 入力トリガ

- 2 文字以上で候補取得開始
- 150ms〜250ms 程度の debounce

### 8.2 表示内容

各候補に以下を表示する。

- kind label
  - 記事
  - ソース
  - トピック
- main label
- 補助情報
  - source/topic は `article_count`
  - article は source 名か translated_title 補助を検討可能

### 8.3 キーボード操作

- ↑ / ↓ で候補移動
- Enter で選択
- Escape で候補リストを閉じる

### 8.4 選択時の挙動

- `article`
  - 検索語として確定
  - そのまま検索実行
- `source`
  - source filter を即時適用
  - 検索実行
- `topic`
  - topic filter を即時適用
  - 検索実行

source/topic は input text へ押し込まず、既存の filter state に直接反映する。

## 9. API 設計

### 9.1 追加 API

- `GET /api/items/search-suggestions`

クエリ:

- `q`
- `limit` (default 10, max 10)

レスポンス例:

```json
{
  "items": [
    {
      "kind": "source",
      "label": "OpenAI Blog",
      "source_id": "uuid",
      "article_count": 56
    },
    {
      "kind": "topic",
      "label": "AI",
      "topic": "AI",
      "article_count": 143
    },
    {
      "kind": "article",
      "label": "Kubernetes の再起動挙動の変更",
      "item_id": "uuid",
      "source_id": "uuid"
    }
  ]
}
```

### 9.2 API の役割

- Meilisearch から候補を取得
- user_id で filter
- kind ごとの上限配分を適用
- UI 向けに整形して返す

## 10. 更新設計

候補 index 更新は、`item_search_documents` 更新とは別イベント/別ジョブで扱う。

### 10.1 article 候補

- summarized 完了時に upsert
- title / translated_title 更新時に再 upsert
- item delete / restore に応じて整合を取る

### 10.2 source 候補

- source 作成 / 更新 / 削除時に upsert/delete
- `article_count` は summarized 記事数で再集計

### 10.3 topic 候補

- summarized item の topics 変化時に再集計
- user + topic 単位で upsert

## 11. バックフィル

初回導入時は候補 index もバックフィルする。

- article 候補:
  - summarized 記事から生成
- source 候補:
  - source 一覧と summarized 件数から生成
- topic 候補:
  - summarized item_summaries.topics を user 単位で集約

バックフィルは既存 search backfill と同じく debug 実行経路を用意してよいが、event と run 管理は別にする。

## 12. 障害時の挙動

- suggestion index が利用不可でも全文検索本体は維持する
- オートコンプリート API 失敗時は候補だけ閉じる
- モーダル全体を壊さない

## 13. 将来拡張

- recent searches
- notes / highlights 候補
- source/topic 候補へのアイコン追加
- article 候補への source 名サブラベル追加
- 利用頻度ベースの候補再ランキング
