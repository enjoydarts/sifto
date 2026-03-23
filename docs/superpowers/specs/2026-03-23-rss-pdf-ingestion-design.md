# RSS PDF Ingestion Design

## Goal

RSS item URL が PDF を指している場合でも、既存の `extract-body -> facts -> summary` パイプラインに載せられるようにする。

初期スコープは次に限定する。

- テキスト埋め込み済み PDF のみ対応
- OCR はしない
- 既存 UI / DB schema は極力変えない

## Current State

- API は item 処理中に worker の `/extract-body` を呼ぶ
- worker の `/extract-body` は `trafilatura_service.extract_body()` に委譲している
- `extract_body()` は HTML 前提で `trafilatura.fetch_url()` と `bare_extraction()` を使っている
- PDF URL は現在ほぼ抽出失敗になる

## Recommended Approach

`worker/app/services/pdf_service.py` を追加し、`trafilatura_service.py` は HTML/PDF の dispatcher にする。

この構成にすると、

- 既存の `/extract-body` 契約を壊さない
- HTML 抽出と PDF 抽出の責務を分けられる
- 将来 OCR や PDF 固有メタデータ処理を追加しやすい

## Flow

1. `/extract-body` は従来どおり `extract_body(url)` を呼ぶ
2. `extract_body(url)` で URL とレスポンスヘッダから PDF かどうかを判定する
3. PDF の場合は `pdf_service.extract_pdf_body(url)` を呼ぶ
4. HTML の場合は既存の trafilatura 抽出を継続する
5. 返り値は既存どおり `title / content / published_at / image_url`
6. API 側は返ってきた `content` をそのまま `items.content_text` に保存し、後続の facts / summary は既存処理を使う

## PDF Detection Rules

優先順は次のとおり。

1. `Content-Type` が `application/pdf`
2. 最終 URL の path が `.pdf` で終わる
3. レスポンス先頭バイトが `%PDF-`

単一の条件に依存せず、できるだけ誤判定を避ける。

## PDF Extraction

ライブラリは `PyMuPDF` を使う。

理由:

- テキスト抽出の安定性と速度のバランスが良い
- 依存追加が比較的素直
- ニュース系のテキスト PDF なら十分実用的

`extract_pdf_body(url)` の処理:

1. `httpx` で PDF バイナリを取得する
2. `fitz` で PDF を開く
3. 各ページからテキストを抽出する
4. ページごとのテキストを空行区切りで連結する
5. 過剰な空白を軽く正規化する
6. metadata title があれば title に使う
7. `published_at` と `image_url` は初期版では `None`

## Title Resolution

PDF の title は次の優先順で解決する。

1. PDF metadata title
2. 既存 item title / RSS item title
3. URL のファイル名

worker の `extract-body` は `title` を optional で返す既存契約なので、取れない場合は `None` でよい。

## Failure Handling

- PDF 判定後に取得失敗したら、通常の extract failure と同じ扱い
- PDF は開けたが本文テキストが空なら抽出失敗
- スキャン PDF でテキストが取れない場合も抽出失敗
- dev placeholder の挙動は既存ルールを踏襲する

## Non-Goals

- OCR 対応
- ページ画像サムネイル抽出
- 表や図の構造化抽出
- PDF 専用 UI 表示

## Testing

最低限の確認対象は次のとおり。

- PDF URL が PDF extractor に分岐する
- `application/pdf` ヘッダがなくても `.pdf` URL で拾える
- テキスト埋め込み PDF から本文が返る
- 空テキスト PDF は failure になる
- HTML URL の既存挙動を壊さない

## Rollout Notes

初期版は「PDF も本文テキストとして ingest できる」ことだけを目標にする。ニュース PDF は極端に長いものを想定しないため、初手では厳しいページ数制限や文字数制限は入れない。必要になった時点で、抽出時間や token コストを見ながら上限を追加する。
