# Sifto Meilisearch 全文検索 設計

作成日: 2026-03-22

## 1. 目的

Items 一覧の検索を、単純な `ILIKE` ベースの部分一致から、記事本文まで含む実用的な全文検索へ置き換える。個人利用を前提としつつ、今後のメモ検索や横断検索に拡張しやすい構造にする。

今回の初期スコープは記事中心に限定する。

- 対象画面: Items 一覧
- 対象データ: 記事
- 対象フィールド: `title`, `translated_title`, `summary`, `facts`, `content_text`
- UI: 既存の検索モーダルを拡張
- 検索結果: relevance 優先 + 複数抜粋 + ハイライト

## 2. 要件整理

### 2.1 機能要件

- 日本語にも効く全文検索を提供する
- 英語記事・日本語キーワードの両方で実用的に検索できる
- 検索結果には一致箇所のハイライトを表示する
- 1記事あたり最大 3 件の抜粋を返す
- 検索モードを UI で切り替えられる
  - `natural`
  - `and`
  - `or`
- 検索時の並び順は relevance 優先にする
- 検索対象の母集団は現在の一覧コンテキストに従う
  - 通常一覧
  - `pending`
  - `deleted`

### 2.2 非機能要件

- API や DB の責務を大きく壊さない
- 検索基盤障害時に Items 一覧全体を巻き込まない
- バックフィル可能で、導入時に既存記事を検索対象へ載せられる
- 今後の `/notes` 検索や横断検索に拡張可能

## 3. アプローチ比較

### 案 A: PostgreSQL 全文検索を強化

- `tsvector/tsquery` を DB に直接組み込む
- `pg_trgm` を補助的に併用する

利点:
- コンポーネント数が増えない
- 単一 DB で完結する

欠点:
- 日本語対応が中途半端になりやすい
- ハイライトや relevance 制御を強めるほど SQL が複雑になる
- 将来の横断検索で責務が肥大化しやすい

### 案 B: Meilisearch を検索基盤として分離

- DB を正本にし、Meilisearch を検索専用インデックスにする
- API は検索時だけ Meilisearch を使う

利点:
- 日本語を含む検索体験を作りやすい
- relevance とハイライト制御がしやすい
- 今後のメモ検索や横断検索に伸ばしやすい

欠点:
- 常駐コンポーネントが 1 つ増える
- インデックス同期設計が必要

### 採用

案 B を採用する。

理由:
- 今回は「検索品質」と「複数抜粋ハイライト」が重要で、PostgreSQL 直実装より Meilisearch の方が目的に合う
- 個人利用でも Meilisearch 常駐は許容できる
- DB クエリ肥大化を避けられる

## 4. 全体構成

### 4.1 責務分離

- PostgreSQL:
  - 記事本体の正本
  - 一覧フィルタの母集団判定
  - 最終的な Items レスポンス生成
- Meilisearch:
  - 全文検索
  - relevance 順位付け
  - ハイライト付き抜粋生成
- Inngest / worker:
  - 検索ドキュメントの非同期 upsert / delete

### 4.2 検索時のデータフロー

1. フロントが `GET /api/items?q=...&search_mode=...` を呼ぶ
2. API が現在の一覧コンテキストから検索母集団フィルタを構成する
3. API が Meilisearch へ検索を実行する
4. Meilisearch が `item_id`, relevance 順, formatted / matches を返す
5. API が該当 item IDs の記事データを DB から取得する
6. API が検索抜粋を Item に合成して返す

## 5. Meilisearch インデックス設計

### 5.1 インデックス名

- 初期案: `items`

必要に応じて将来 `notes`, `highlights` などを追加する。

### 5.2 ドキュメント構造

1 記事 1 ドキュメント。

```json
{
  "id": "item_id",
  "user_id": "user_uuid_text",
  "source_id": "source_uuid_text",
  "status": "summarized",
  "is_deleted": false,
  "title": "How Kubernetes restart behavior changed",
  "translated_title": "Kubernetes の再起動挙動の変更",
  "summary": "....",
  "facts_text": "fact 1\nfact 2\nfact 3",
  "content_text": "....",
  "published_at": "2026-03-22T00:00:00Z",
  "created_at": "2026-03-22T00:00:00Z"
}
```

### 5.3 searchableAttributes

優先順は以下とする。

1. `title`
2. `translated_title`
3. `summary`
4. `facts_text`
5. `content_text`

これにより title / summary / facts を強く、本文を弱く扱う。

### 5.4 filterableAttributes

初期版:

- `user_id`
- `source_id`
- `status`
- `is_deleted`

将来拡張候補:

- `topics`
- `is_favorite`
- `is_read`
- `has_later`

### 5.5 displayedAttributes

API がハイライト生成に使う属性のみ返せばよい。

- `id`
- `title`
- `translated_title`
- `summary`
- `facts_text`
- `content_text`
- `source_id`
- `status`
- `is_deleted`

## 6. 検索モード設計

### 6.1 モード

- `natural`
  - デフォルト
  - 自然な入力を想定
  - スペース区切り、引用符、除外語を扱えるようアプリ側で整形する
- `and`
  - 入力した語をすべて含む結果を優先する
- `or`
  - より広く拾う

### 6.2 Meilisearch への変換方針

Meilisearch の query string と filter を組み合わせる。

- `natural`:
  - 入力文字列をほぼそのまま query に渡す
  - quoted phrase や除外語はアプリ側パーサで補助
- `and`:
  - すべてのトークンを必須条件として構成
- `or`:
  - いずれかに一致すればヒットする構成

具体的な文字列パースは API 側の小さな検索構文パーサに閉じ込める。

## 7. API 設計

### 7.1 対象 API

既存の `GET /api/items` を拡張する。

追加パラメータ:

- `q`
- `search_mode=natural|and|or`

### 7.2 検索時のソート

- `q` が空: 既存 sort を維持
- `q` がある: `relevance` 優先

検索中は `newest / score / personal_score` より検索 relevance を優先する。

### 7.3 レスポンス拡張

`items[]` に以下を追加する。

```json
{
  "search_match_count": 3,
  "search_snippets": [
    { "field": "summary", "snippet_html": "kubectl <mark>rollout restart</mark> now ..." },
    { "field": "facts", "snippet_html": "The change affects <mark>deployment</mark> ..." },
    { "field": "content", "snippet_html": "... <mark>kubernetes</mark> automation saw ..." }
  ]
}
```

制約:

- 最大 3 件
- 異なる field を優先
- `field` は `title | summary | facts | content`

## 8. 一覧コンテキストとの整合

検索対象の母集団は常に現在の一覧条件に追従する。

例:

- 通常 Items 一覧:
  - 通常一覧の対象に限定
- `pending` 一覧:
  - `pending` 母集団内で検索
- `deleted` 一覧:
  - `deleted` 母集団内で検索

これにより、検索だけ別の世界を見ている状態を避ける。

## 9. インデックス同期

### 9.1 基本方針

- DB を正本
- Meilisearch は派生インデックス
- 更新は非同期

### 9.2 upsert を発火するイベント

- 記事が `summarized` になった
- summary が更新された
- facts が更新された
- title / translated_title / content_text が更新された
- item が restore された

### 9.3 delete を発火するイベント

- item が deleted になった

### 9.4 実装責務

- API / repository:
  - 検索更新イベント発火
- Inngest function:
  - 対象記事を DB から読み直し
  - 検索ドキュメント組み立て
  - Meilisearch に upsert / delete

## 10. バックフィル

### 10.1 対象

- `summarized` のみ

### 10.2 実行方式

- debug エンドポイントまたは Inngest 関数で起動
- ページングで順次投入
- 冪等に再実行可能

### 10.3 期待動作

- 既存記事が初回導入後すぐ検索可能になる
- 途中失敗しても再開可能

## 11. UI 設計

### 11.1 検索モーダル

既存の Items 検索モーダルを拡張する。

追加するもの:

- 検索モード切替
  - 自然文寄り
  - 厳密AND
  - ゆるめOR
- 検索ヒント
  - `"quoted phrase"`
  - `-exclude`

### 11.2 検索結果カード

一覧カードに検索時だけ追加表示する。

- relevance 順で表示
- 最大 3 件の抜粋
- `Title / Summary / Facts / Content` ラベル付き
- 一致語を `<mark>` で強調

初期版では検索結果の見た目は今の一覧に寄せ、検索体験だけを強化する。

## 12. 障害時の扱い

### 12.1 Meilisearch 障害

- `GET /api/items?q=...` は検索専用エラーを返す
- 一覧画面全体は壊さない
- UI では「検索インデックスが利用できない」と再試行導線を出す

### 12.2 インデックス遅延

- 数秒から数十秒の反映遅延は許容する
- DB が正本であることを優先する

### 12.3 delete / restore

- delete: Meilisearch でも delete
- restore: 再 upsert

## 13. テスト方針

### 13.1 API / repository

- `q` なしは従来一覧と同じ
- `q` ありで relevance 順になる
- `search_mode` ごとの差分
- `pending` / `deleted` 文脈の母集団切替
- snippets と match count が返る

### 13.2 インデックス同期

- summarized 作成時 upsert
- summary/facts 更新時再 upsert
- delete / restore
- バックフィルの冪等性

### 13.3 E2E

- Items 画面で検索モーダルを開く
- 検索モード切替
- 検索結果に複数抜粋が出る
- `pending` / `deleted` 一覧でも検索できる

## 14. 実装順

1. Meilisearch 導入と設定管理
2. 検索ドキュメント組み立てロジック
3. Inngest による upsert / delete
4. 既存記事バックフィル
5. `GET /api/items` の検索統合
6. Items 検索モーダル拡張
7. snippets / highlights 表示
8. テスト追加

## 15. 将来拡張

- `/notes` 検索
- メモ / ハイライト横断検索
- ソース横断検索
- 検索履歴 / 保存検索
- 絞り込み条件の高度化

