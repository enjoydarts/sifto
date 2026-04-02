# YouTube Item Extraction Implementation Plan

## Goal

YouTube URL を通常記事として誤要約しないようにしつつ、字幕が取得できる動画は既存の `facts -> summary` pipeline へ流す。

## Constraints

- 通常 URL の挙動は変えない
- UI 変更は入れない
- 字幕がない YouTube は deleted 扱い
- タイトルは YouTube 動画タイトルを使う

## Implementation Chunks

### Chunk 1: YouTube URL 判定と worker service 追加

対象:

- `worker/app/services/youtube_extract_service.py`
- `worker/app/routers/extract.py`

内容:

- YouTube URL 判定 helper を追加
- `yt-dlp` 実行で metadata を取得する service を追加
- タイトル、公開日時、thumbnail、subtitle candidate を抽出
- 日本語優先、英語 fallback の選択ロジックを実装
- 抽出結果を既存 `ExtractResponse` 互換の dict へ変換

テスト:

- YouTube URL 判定
- 字幕言語選択
- タイトル/字幕抽出
- 字幕なし判定

### Chunk 2: extract-body 分岐

対象:

- `worker/app/routers/extract.py`
- 必要なら既存 extract service の wrapper

内容:

- URL が YouTube のときは YouTube extractor を使う
- 通常 URL は既存 `trafilatura_service.extract_body()` を維持
- YouTube extractor failure を distinguish できる error message にする

テスト:

- YouTube URL は trafilatura を通らない
- 通常 URL は既存経路を通る

### Chunk 3: API/Inngest 側の deleted 扱い確認

対象:

- `api/internal/inngest/functions.go`
- `api/internal/inngest/process_item_fallback_test.go`

内容:

- YouTube 字幕なし時の worker error を `markProcessItemDeleted(...)` に流す
- retry ポリシーを確認し、不要な再試行を避ける
- `processing_error` に transcript unavailable を残す

テスト:

- YouTube transcript unavailable は deleted
- 通常 extract failure の既存挙動は維持

### Chunk 4: 依存と実行環境

対象:

- `worker` の Docker build / runtime 定義
- 必要な install script

内容:

- worker コンテナで `yt-dlp` を使えるようにする
- 実行バイナリパスを固定しすぎない
- command failure 時のログを明確化

テスト:

- `make check-worker`
- 必要なら lightweight integration test

### Chunk 5: 回帰確認

対象:

- worker unit tests
- API/Inngest tests

内容:

- YouTube 字幕ありで summary まで進めるケース
- 字幕なしで deleted になるケース
- 通常記事 URL が壊れていないことの確認

## Verification

- `make check-worker`
- `docker compose exec -T api go test ./internal/inngest`
- YouTube extractor unit tests
- extract-body router tests

## Rollout Notes

- `yt-dlp` 依存が追加されるため、worker image の build / deploy が必要
- 本番反映後、YouTube URL 既存 item の再取込は必要に応じて個別対応
