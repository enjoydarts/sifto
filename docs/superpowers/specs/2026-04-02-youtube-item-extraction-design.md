# YouTube Item Extraction Design

## Goal

YouTube URL が通常記事として取り込まれ、本文のないまま `facts -> summary` まで進んでしまう問題を解消する。

狙いは次の 3 点。

- YouTube URL でも動画タイトルを正しく item title に反映する
- 字幕を取得できる動画は既存の要約 pipeline へ流す
- 字幕を取得できない動画は中身のない要約を作らず削除扱いにする

## Scope

対象:

- `youtube.com/watch`
- `youtube.com/shorts`
- `youtu.be`

非対象:

- 字幕の生成そのもの
- YouTube 動画向けの UI 特別表示
- YouTube Data API の導入
- 音声 ASR による fallback

## Proposed Approach

### 1. `extract-body` で YouTube URL を分岐

worker の本文抽出入口で YouTube URL を判定する。

- 通常 URL: 既存の `trafilatura_service.extract_body()` を利用
- YouTube URL: 新しい `youtube_extract_service.extract_body()` を利用

これにより後段の `facts -> summary -> embedding` は変更しない。

### 2. `yt-dlp` でタイトルと字幕を取得

YouTube 抽出は `yt-dlp` ベースで行う。

理由:

- タイトル取得と字幕取得を同じツールで扱える
- 手動字幕と自動字幕の両方を見に行ける
- 日本語優先、英語 fallback の実装がしやすい

取得方針:

- タイトルは常に `yt-dlp` のメタデータから取得する
- 字幕は `ja`, `ja-JP` を優先
- 日本語がなければ `en` 系へ fallback
- 手動字幕を優先し、なければ自動字幕を使う

### 3. 字幕を `content_text` として既存 pipeline に流す

字幕が取得できた場合は、連結した字幕テキストを `content_text` として扱う。

worker の戻り値は既存の `ExtractBodyResponse` のままとし、次を埋める。

- `title`: YouTube 動画タイトル
- `content`: 連結済み字幕テキスト
- `published_at`: `yt-dlp` から取れる場合は反映
- `image_url`: サムネイル URL を取れる場合は反映

これにより API/Inngest 側は通常記事と同じ処理で動く。

### 4. 字幕が取れない YouTube は削除扱い

YouTube 専用抽出で字幕を取得できなかった場合は、`process-item` で本文抽出失敗と同等に扱う。

期待動作:

- `content` を空にして先へ進めない
- item は `markProcessItemDeleted(...)` で削除扱い
- `processing_error` には「YouTube transcript unavailable」相当の理由を残す

これにより一覧上に中身のない要約済み item が残らない。

## Data Flow

1. item URL を受け取る
2. worker `extract-body` が URL を判定
3. YouTube なら `yt-dlp` で metadata と subtitle candidate を取得
4. 最適な字幕言語を選ぶ
5. 字幕あり:
   - `title` と `content_text` を返す
   - 既存の `facts -> summary` へ進む
6. 字幕なし:
   - API 側で item を deleted にする

## Failure Handling

### タイトル取得失敗

今回の前提では実質発生しないものとして扱う。
実装上は `yt-dlp` 呼び出し失敗としてまとめて扱い、削除扱いへ寄せる。

### 字幕取得失敗

- 字幕候補が 0 件
- 取得した字幕本文が空
- JSON parse / subtitle download 失敗

上記はいずれも「YouTube transcript unavailable」として削除扱いにする。

### `yt-dlp` 実行失敗

- worker ログに command failure を残す
- item は deleted にする

## Logging

最低限残すログ:

- `youtube extract start url=...`
- `youtube extract title resolved url=... title=...`
- `youtube extract transcript selected url=... lang=... auto=...`
- `youtube extract transcript unavailable url=...`

これにより「なぜ削除されたか」を後から追える。

## Testing

### Worker

- YouTube URL を判定できる
- 日本語字幕があるときは日本語を選ぶ
- 日本語がなく英語字幕があるときは英語を選ぶ
- 字幕がないときは unavailable 扱いになる
- 字幕テキスト連結が空文字を返さない

### API / Inngest

- YouTube 字幕あり item は通常通り `facts -> summary` へ進む
- YouTube 字幕なし item は deleted になる
- 通常 URL は既存の extract-body 挙動を維持する

## Trade-offs

### Pros

- 中身のない YouTube 要約を止められる
- 既存の要約 pipeline を再利用できる
- UI 変更なしで導入できる

### Cons

- worker 実行環境に `yt-dlp` 依存が増える
- 字幕がない動画は潔く捨てる設計になる
- 英語字幕 fallback では日本語記事ほど自然な事実抽出にならない可能性がある

## Rollout

段階導入は不要。
実装後は YouTube URL のみ専用経路へ入り、通常 URL は既存のまま維持する。

## Recommendation

`extract-body` の責務内で YouTube 専用抽出へ分岐し、`yt-dlp` でタイトルと字幕を取得する方式を採用する。

理由:

- 後段の `facts -> summary` を変えずに済む
- 「字幕ありなら通常処理、字幕なしなら削除」という要件に素直に一致する
- 実装の複雑さが最小で、運用時の挙動も説明しやすい
