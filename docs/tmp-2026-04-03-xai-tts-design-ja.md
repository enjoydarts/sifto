# xAI TTS追加 設計

## 背景

- 現在の Sifto の TTS は、Audio Briefing のペルソナ音声設定と Summary Audio 再生の両方で `tts_provider` を通じて切り替える構造になっている。
- 実運用で十分な UX を持っているのは `aivis` だけで、専用のモデル同期、一覧画面、picker、設定画面内の選択導線がある。
- `mock` は開発用であり、実ユーザー向けの選択肢としては不十分。
- xAI は公式に Text to Speech API を提供しており、`POST /v1/tts` と `GET /v1/tts/voices` を使って音声合成と voice catalog の取得ができる。

## 目的

- xAI TTS を `aivis` と同等の体験で選択・同期・保存・再生できるようにする。
- 対象は Audio Briefing だけではなく、Summary Audio も含む全 TTS 経路とする。
- 既存の `aivis` 実装を壊さず、`xai` を横に追加する。

## スコープ

- API
  - xAI voice catalog の同期 API
  - xAI voice catalog の一覧 API
  - Audio Briefing persona voice 保存時の `tts_provider=xai` 対応
  - Summary Audio / Audio Briefing の xAI synthesis 対応
- DB
  - xAI voice catalog snapshots / sync runs の永続化
  - provider model updates への `xai` 反映
- Worker
  - xAI TTS の実音声生成
  - xAI voice list fetch
- Web
  - xAI voice catalog 一覧画面
  - xAI picker
  - Settings で `xai` を provider として選択可能にする
  - Summary Audio で保存済み xAI voice をそのまま利用できる状態にする

## 非スコープ

- xAI Voice Agent API の導入
- リアルタイム会話系の音声入出力
- provider 共通カタログ基盤への全面リファクタ
- Aivis の UX やデータ構造を xAI に合わせて作り直すこと

## 現状整理

### 1. Audio Briefing の音声設定

- `audio_briefing_persona_voices` には `tts_provider`, `voice_model`, `voice_style` などが保存される。
- Settings 画面では provider ごとに UI を分けており、`aivis` のときだけ picker と catalog refresh が出る。
- `mock` やその他 provider は自由入力 UI で済ませている。

### 2. TTS 実行

- Worker の `AudioBriefingTTSService` が provider ごとの synthesize を持つ。
- 現在の実装は `mock` と `aivis` のみ。
- Summary Audio も同じ provider パラメータを worker へ渡して synthesize するので、worker 側の provider 追加で流用可能。

### 3. Aivis 専用 UX

- API に `AivisModelsHandler` と `AivisCatalogService` がある。
- DB に sync run / snapshots がある。
- Web に Aivis models ページと picker がある。
- これが今回の `Aivisと同等` の基準になる。

## 要件

### 必須

- `tts_provider` で `xai` を選べる。
- xAI voice 一覧を API 経由で同期・保存・再表示できる。
- Settings で xAI voice picker を開き、選択結果を `voice_model` に保存できる。
- Audio Briefing の音声合成で xAI voice を使える。
- Summary Audio でも同じ xAI voice を使える。
- xAI の認証は既存の xAI API key を使う。新しい provider secret は増やさない。

### UX要件

- ユーザー体験は Aivis に寄せる。
  - 管理画面を開ける
  - 一覧更新できる
  - picker から選べる
  - 設定済み音声の状態が見える
- ただし xAI の実データ構造は Aivis と異なるため、内部保存形式は `voice_id` 中心でよい。

## 公式 API 前提

- xAI TTS:
  - `POST /v1/tts`
  - `GET /v1/tts/voices`
- xAI voice catalog は Aivis のような `model > speaker > style` 階層ではなく、実質 `voice_id` 一覧で扱う。
- よって Sifto の UI では Aivis 相当の見せ方をしつつ、内部表現は xAI の voice list に合わせる。

## 設計方針

### 推奨方針

- `aivis` の専用実装を一般化しようとせず、`xai` 用の専用 slice を横に追加する。
- 理由:
  - 既存の Aivis 実装が十分に安定している。
  - 今回必要なのは provider の追加であり、共通化そのものではない。
  - 共通化は工数の割に変更範囲が広く、既存 UX を崩すリスクが高い。

## データ設計

### 新規テーブル

- `xai_voice_sync_runs`
  - `id`
  - `status`
  - `trigger_type`
  - `started_at`
  - `finished_at`
  - `last_progress_at`
  - `fetched_count`
  - `saved_count`
  - `error_message`

- `xai_voice_snapshots`
  - `id`
  - `sync_run_id`
  - `voice_id`
  - `name`
  - `description`
  - `language`
  - `preview_url`
  - `metadata_json`
  - `fetched_at`

### 保存方針

- `voice_model` に `voice_id` を保存する。
- `voice_style` は空文字許容とし、xAI では基本未使用。
- Aivis の `voice_style` を前提にしている UI / payload は、provider ごとの条件分岐で扱う。

## API設計

### 新規 handler / service / repo

- `XAIVoicesHandler`
  - `GET /xai-voices`
  - `GET /xai-voices/status`
  - `POST /xai-voices/sync`

- `XAIVoiceCatalogService`
  - xAI 公式 `GET /v1/tts/voices` 呼び出し
  - レスポンスの正規化

- `XAIVoiceRepo`
  - sync run の開始・完了・失敗
  - snapshots の insert / list
  - 前回 snapshot との差分参照

### provider model updates

- `provider_model_updates` に `provider=xai` の snapshot / change event を追加する。
- 現状の `aivis` 同様、voice_id 単位で added / removed を記録する。

## Worker設計

### xAI synthesis

- `AudioBriefingTTSService` に `provider == "xai"` 分岐を追加する。
- 新規 helper 例:
  - `synthesize_xai_audio(...)`
  - `fetch_xai_voices(...)`

### 認証

- worker header で既存の xAI API key を渡す。
- API サーバ側では、ユーザーの xAI key を復号して worker に送る。
- `aivis_api_key` 専用パラメータのような provider 固定名は、xAI 追加時に見直す。
  - ただし今回の変更では、最小差分で `xai_api_key` 追加でもよい。

### 音声パラメータ

- xAI が明示的に受けるものだけ送る。
- `speech_rate` は可能ならマップする。
- `tempo_dynamics`, `emotional_intensity`, `line_break_silence_seconds`, `user_dictionary_uuid` は xAI では無効。
- provider ごとに利用しないパラメータは黙って捨てるか、worker 内で無視する。

### 出力形式

- Summary Audio は既存通り base64 音声を返す。
- Audio Briefing は既存通り R2 upload まで行う。
- xAI が返す音声 format は、既存の downstream が扱いやすい `mp3` を基本にする。

## Web設計

### 新規画面

- `/xai-voices`
  - Aivis 一覧画面と同等の役割
  - voice name, voice_id, description, preview の有無, 最終同期などを表示
  - 検索・同期ボタンを持つ

### Settings 画面

- Audio Briefing persona voice の provider select に `xai` を追加
- `xai` 選択時は:
  - xAI picker を開くボタン
  - 現在選択中の voice name / voice_id を表示
  - provider 固有の注意書きを表示
- `aivis` 専用 UI はそのまま維持
- `xai` では不要な Aivis 固有パラメータ UI は出さない

### picker

- `Aivis` picker を参考に `XAI` picker を追加
- 選択結果:
  - `tts_provider = "xai"`
  - `voice_model = selected.voice_id`
  - `voice_style = ""`

### i18n

- `ja.ts` / `en.ts` の両方に以下を追加
  - xAI voice catalog 画面文言
  - picker 文言
  - provider status / warning 文言

## Summary Audio への反映

- Summary Audio はすでに persona voice の `tts_provider` を読んで worker に渡す構造。
- xAI provider を worker と API で扱えるようにすれば、そのまま対象になる。
- 追加で必要なのは:
  - xAI key 解決
  - xAI provider の synthesize 分岐
  - 必要なら provider 固有のエラーメッセージ整備

## エラーハンドリング

- xAI voice sync API 失敗時:
  - sync run を `failed`
  - provider update snapshot も `failed`
  - HTTP 502 を返す

- xAI key 未設定時:
  - Settings に警告を出す
  - synthesize は conflict / internal error にせず、既存の provider 警告 UX に合わせて明示メッセージを返す

- voice_id 消滅時:
  - 設定済み voice は「removed/unknown」と表示
  - picker で再選択を促す

## テスト設計

### API / Go

- xAI catalog service のレスポンス正規化
- xAI sync handler の成功 / 失敗
- settings save で `tts_provider=xai` を受理すること
- Summary Audio が xAI key を解決して worker に渡すこと

### Worker / Python

- xAI voice list fetch の正規化
- xAI TTS リクエスト body / header
- unsupported provider エラーの回避
- Summary Audio / Audio Briefing 両方で xAI synthesize が動くこと

### Web

- provider select で `xai` が見える
- xAI picker で voice_id が保存される
- provider ごとの入力 UI 切り替え
- 型定義と build 通過

## 実装順

1. DB migration 追加
2. API 側 xAI voice repo / service / handler 追加
3. worker 側 xAI voice fetch / synthesize 追加
4. Audio Briefing / Summary Audio の xAI key 配線
5. Web 一覧画面と picker 追加
6. Settings 画面へ provider `xai` 追加
7. テスト、lint、build、動作確認

## リスク

- xAI の voice catalog 形状が Aivis より単純なので、UI を Aivis に寄せすぎると不自然になる。
- xAI TTS で受けられる音声制御パラメータが Aivis より少ない可能性が高い。
- provider 固有 secret の扱いが `aivis_api_key` 前提で散っている箇所は、xAI 追加時に露呈しやすい。
- Summary Audio と Audio Briefing の両方に効かせるため、worker route の差分漏れがあると片側だけ動かない。

## 採用判断

- 今回は「Aivis と同等の UX を持つ xAI TTS provider を横追加する」方針を採用する。
- カタログの内部データ構造は xAI の `voice_id` ベースに合わせる。
- provider 共通基盤への全面抽象化は後回しにする。
