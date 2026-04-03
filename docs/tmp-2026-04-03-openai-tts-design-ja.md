# OpenAI TTS追加 設計

## 背景

- 現在の Sifto の TTS は `aivis` と `xai` を provider 分岐で扱っており、Audio Briefing と Summary Audio の両方で同じ `tts_provider` 系設定を使っている。
- `aivis` は catalog sync、一覧画面、picker、設定画面内の導線までそろっている。
- `xai` も同様の体験を追加したが、実装は API / worker / web に provider 固有分岐が増えており、このまま 3 provider 目を足すと見通しが急速に悪くなる。
- ユーザー要望として、OpenAI TTS も `Aivis/xAI とそろう` 体験、かつ `model も選べる` 形で追加したい。

## 目的

- OpenAI TTS を Audio Briefing / Summary Audio の両方で利用可能にする。
- UI は `Aivis/xAI` と同じく catalog sync と picker を持ち、OpenAI では `model select + voice picker` の 2 段構成にする。
- 今後の provider 追加に備え、全面リライトではなく「薄い provider 抽象化」を先に入れる。

## スコープ

- API
  - OpenAI TTS voice catalog の同期 / 一覧 / status API
  - Audio Briefing persona voice 保存時の `tts_provider=openai` 対応
  - Summary Audio / Audio Briefing での OpenAI TTS 実行対応
  - provider capability / validation の薄い共通化
- DB
  - OpenAI TTS voice snapshots / sync runs
  - persona voice 設定で OpenAI 用 `tts model` を保持する最小変更
- Worker
  - OpenAI TTS synthesis adapter
  - provider dispatch の薄い抽象化
- Web
  - OpenAI voice catalog 一覧画面
  - OpenAI voice picker
  - Settings に `openai` provider と `model select` を追加
  - provider capability ベースの UI 出し分け

## 非スコープ

- TTS 設定テーブルの全面再設計
- Aivis / xAI catalog 基盤の完全統一
- Realtime API や会話型音声入出力
- provider ごとの細かな prosody パラメータ正規化
- OpenAI TTS 以外の新 provider 追加

## 現状の問題

### 1. provider 分岐が散っている

- API では key 解決、validation、voice completeness 判定が provider ごとに増えている。
- worker でも `aivis/xai/mock` 分岐が service に直書きされている。
- web でも `aivis` 専用 UI と `xai` 専用 UI が局所分岐で増えている。

### 2. provider ごとの能力差がコードに埋め込まれている

- `voice_style` が必要かどうか
- catalog picker があるか
- model と voice を分けて選ぶ必要があるか
- speed / pitch などの tuning を使うか

これらが構造化されていないため、OpenAI をそのまま追加すると保守性が落ちる。

## 要件

### 必須

- `tts_provider=openai` を保存できる。
- OpenAI voice catalog を同期 / 再表示できる。
- Settings で OpenAI の `model` を選べる。
- Settings で OpenAI voice picker を開き、voice を選べる。
- Audio Briefing で OpenAI voice を使える。
- Summary Audio でも同じ persona voice 設定を使える。
- OpenAI の認証は既存の OpenAI API key を使う。

### UX要件

- `Aivis/xAI` と同じく catalog 画面、同期、picker を持つ。
- ただし OpenAI は `model` と `voice` の役割が分かれているので、UI では
  - provider = openai
  - model select
  - voice picker
  の順に出す。
- 音声出力の既定は `mp3 / 192kbps / 44100Hz` とする。

## 推奨方針

- 全面リファクタはしない。
- 代わりに、provider capability と synthesis adapter の 2 点だけ薄く抽象化してから OpenAI を足す。

理由:

- 今の段階では DB / UI / worker を全部 provider-agnostic に組み直すほどではない。
- ただし `if provider == ...` を増やし続ける段階は超えた。
- OpenAI は `model` と `voice` が分かれるため、最小の整理なしでは UI と validation が崩れる。

## 設計方針

### 1. provider capability を導入する

Go / TS 双方で、以下のような capability を持つ provider descriptor を追加する。

- `requires_voice_style`
- `supports_catalog_picker`
- `supports_separate_tts_model`
- `supports_speech_tuning`
- `requires_user_api_key`

想定:

- `aivis`
  - style 必須
  - catalog あり
  - separate model なし
  - tuning あり
- `xai`
  - style 不要
  - catalog あり
  - separate model なし
  - tuning ほぼなし
- `openai`
  - style 不要
  - catalog あり
  - separate model あり
  - tuning は当初なし

これにより、保存 validation と UI 出し分けを条件分岐の羅列ではなく descriptor 参照で処理できる。

### 2. worker を provider adapter 方式に寄せる

`AudioBriefingTTSService` / `SummaryAudioPlayerService` から provider 固有処理を切り出す。

想定構成:

- `aivis_tts.py`
- `xai_tts.py`
- `openai_tts.py`
- `tts_provider_registry.py` または同等の dispatch helper

共通 service は

- provider 名
- text
- voice
- tts model
- 共通 output format

を受けて adapter に渡すだけにする。

### 3. OpenAI は `model` と `voice` を分離して扱う

OpenAI TTS では、同じ voice を複数 model で使える前提があるため、`voice` に model を埋め込まない。

保存方針:

- `tts_provider = openai`
- `voice_model = selected voice`
- `voice_style = ""`
- 新たに `tts_model` を persona voice に保持する

`tts_model` は将来的に他 provider でも使える汎用名だが、今回利用するのは OpenAI のみとする。

## データ設計

### 1. persona voice に `tts_model` を追加

対象:

- `audio_briefing_persona_voices`

追加列:

- `tts_model text not null default ''`

利用ルール:

- `openai` のとき必須
- `aivis/xai/mock` は空でよい

### 2. OpenAI voice catalog テーブルを追加

- `openai_tts_voice_sync_runs`
  - `id`
  - `status`
  - `trigger_type`
  - `started_at`
  - `finished_at`
  - `last_progress_at`
  - `fetched_count`
  - `saved_count`
  - `error_message`

- `openai_tts_voice_snapshots`
  - `id`
  - `sync_run_id`
  - `voice_id`
  - `name`
  - `description`
  - `language`
  - `preview_url`
  - `metadata_json`
  - `fetched_at`

### 3. OpenAI model 候補

- 音声 `model` は catalog sync 対象ではなく、当面は手動管理の候補リストにする。
- 理由:
  - voice list と違い、OpenAI TTS model 一覧は catalog 連携の必要性が低い。
  - LLM model catalog と混ぜると責務がぶれる。

初期候補は repo 内の定数または shared catalog で管理する。

## API設計

### 新規

- `GET /openai-tts-voices`
- `GET /openai-tts-voices/status`
- `POST /openai-tts-voices/sync`

### 変更

- Audio Briefing persona voice 保存 API
  - `tts_model` を受ける
  - `openai` の場合は `tts_model` 必須
  - capability に基づいて `voice_style` 必須条件を決める

- Summary Audio / Audio Briefing synthesis
  - provider ごとの key 解決を descriptor 経由に寄せる
  - OpenAI の場合は OpenAI API key を復号して worker に渡す

## Worker設計

### OpenAI synthesis adapter

- `openai_tts.py` を追加
- 受け取る値:
  - `api_key`
  - `tts_model`
  - `voice`
  - `text`
  - output format 指定

### 出力形式

- 既定:
  - `mp3`
  - `192kbps`
  - `44100Hz`

### tuning

- OpenAI は当初 `speech_rate / pitch / tempo_dynamics / emotional_intensity` を UI 上では実質無効扱いにする。
- provider capability で tuning 非対応にし、Aivis 固有 UI を出さない。

## Web設計

### 新規画面

- `/openai-tts-voices`
  - xAI voice catalog 画面と同等
  - voice name / voice_id / description / language / preview を表示

### Settings 画面

- provider select に `openai` を追加
- `openai` 選択時は以下を表示
  - `tts model` select
  - OpenAI voice picker ボタン
  - 選択済み voice の表示
  - tuning 非対応の説明

### capability ベース出し分け

- `requires_voice_style`
  - true のときだけ style 入力を出す
- `supports_separate_tts_model`
  - true のときだけ model select を出す
- `supports_catalog_picker`
  - true のときだけ picker ボタンと同期導線を出す
- `supports_speech_tuning`
  - false のときは tuning セクションを隠すか disabled にする

## Summary Audio への反映

- Summary Audio は既存の persona voice 設定を読む構造を維持する。
- provider が `openai` の場合:
  - `tts_model`
  - `voice_model` as voice
  - OpenAI API key
  を worker に渡す。

## エラーハンドリング

- OpenAI API key 未設定
  - Settings に警告
  - synthesize は明示エラーを返す

- voice が catalog から消えた
  - 保存済み設定を warning 状態で表示
  - picker で再選択を促す

- model 未設定
  - `openai` のとき保存時 validation error

## テスト設計

### Go

- `tts_provider=openai` の保存 validation
- `tts_model` 必須条件
- OpenAI API key を worker header に渡すこと
- Summary Audio / Audio Briefing の request body が OpenAI で正しく組まれること

### Python

- OpenAI TTS request body / header / output format
- provider dispatch が `openai` を通すこと
- Summary Audio / Audio Briefing の両方で OpenAI adapter が使われること

### Web

- provider=openai 時に `model select + voice picker` が出る
- style 入力が不要になる
- 同期中文言と picker 文言が辞書経由で表示される

## 実装順

1. provider capability の薄い共通化
2. `tts_model` 列追加と保存 API 対応
3. OpenAI voice catalog persistence / sync API
4. worker OpenAI TTS adapter
5. Settings の `openai` provider + model select + picker
6. Summary Audio / Audio Briefing 全経路の結線

## リスク

- OpenAI TTS の入力 / 出力パラメータが xAI / Aivis と異なるため、共通パラメータの押し込みは危険
- `tts_model` をどう保存するかを曖昧にすると UI と API がすぐ不整合になる
- provider capability を薄く入れずに進めると、4 provider 目以降の追加コストがさらに上がる

## 結論

- OpenAI TTS 追加は可能だが、その前に小さなリファクタを入れるべき段階に来ている。
- ただし全面再設計は不要で、今回は
  - provider capability
  - worker adapter
  - OpenAI 用 `tts_model`
 だけを追加するのが最も現実的で安全。
