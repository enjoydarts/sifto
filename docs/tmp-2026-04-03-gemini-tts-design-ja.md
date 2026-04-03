# Gemini TTS追加設計

## 目的

Audio Briefing に Gemini TTS を追加し、以下を実現する。

- single speaker では Gemini TTS を通常の TTS provider として使える
- duo では Gemini TTS の multi-speaker synthesis を使い、OP / 総括 / 各記事 / ED 単位でより自然な会話音声を生成する
- ペルソナ固有の tone / performance 指示は `shared/ai_navigator_personas.json` に持ち、Gemini TTS の prompt 入力に使う
- Gemini TTS は `voice` に加えて `model` 選択が必要なため、既存 provider 実装に無理に押し込まず専用 adapter を用意する

## 前提

- Google Cloud Text-to-Speech の Gemini-TTS を使う
- 使用モデル候補は少なくとも以下を対象にする
  - `gemini-2.5-flash-tts`
  - `gemini-2.5-pro-tts`
  - 必要なら `gemini-2.5-flash-lite-preview-tts`
- Gemini-TTS は single / multi-speaker の両方をサポートする
- multi-speaker では speaker alias は英数字のみで、prompt / dialogue はサイズ上限がある

参考:
- https://cloud.google.com/text-to-speech/docs/gemini-tts
- https://docs.cloud.google.com/text-to-speech/docs/create-dialogue-with-multispeakers

## 制約

Gemini-TTS は既存 provider より制約が強い。

- single speaker:
  - `text` と `prompt` は各 4,000 bytes まで
  - 合計 8,000 bytes まで
  - 出力音声は約 655 秒を超えると truncate されうる
- multi-speaker:
  - prompt と dialogue の各 field が 4,000 bytes まで
  - 合計 8,000 bytes まで
  - speaker alias は英数字のみ、空白不可

このため、duo 全体を 1 リクエストにせず、OP / 総括 / 各記事 / ED ごとに区切る。

## 全体方針

### 1. Gemini TTS は専用 provider / 専用 adapter とする

既存の Aivis / xAI / OpenAI と同じ `provider dispatch` の枠には乗せるが、実装は専用に分ける。

理由:

- `model + voice + prompt` の 3 要素が必要
- single と duo で API 入力構造が大きく異なる
- duo は multi-speaker を使うため、今の「1 chunk = 1 speaker = 1 request」実装と相性が悪い

実装イメージ:

- worker:
  - `gemini_tts.py` を新設
  - `synthesize_gemini_single_tts(...)`
  - `synthesize_gemini_multi_speaker_tts(...)`
- API:
  - provider capability に `gemini_tts` を追加
  - worker への request body に Gemini 専用フィールドを追加

### 2. ペルソナの tone / performance 指示は shared persona 定義に持つ

`shared/ai_navigator_personas.json` に `audio_briefing` セクションを追加する。

例:

```json
{
  "snark": {
    "name": "毒舌ガイド ジン",
    "voice": "...",
    "briefing": { "...": "..." },
    "audio_briefing": {
      "tone_prompt": "乾いたユーモアと軽い皮肉を混ぜるが、不快にしない",
      "speaking_style_prompt": "テンポよく、語尾は切りすぎず、会話の間を自然に取る",
      "duo_conversation_prompt": "相手の発言を軽く受けてから返す。被せすぎない"
    }
  }
}
```

このデータは:

- single では persona ごとの speech prompt 構築に使う
- duo では host / partner の話者指示と会話全体指示の構築に使う

### 3. duo の multi-speaker は section 単位で生成する

duo では以下の単位で multi-speaker synthesis を行う。

- OP
- 総括
- 各記事
- ED

理由:

- 記事単位で会話のまとまりを保てる
- Gemini の input size 制約に当たりにくい
- 失敗時の retry / 部分再生成がしやすい

この設計では、duo script の turn list から section ごとに会話ブロックを切り出し、各 section を 1 音声チャンクとして生成する。

## データモデル

### 1. 共有 persona 定義

`shared/ai_navigator_personas.json`

- 各 persona に `audio_briefing` を追加
  - `tone_prompt`
  - `speaking_style_prompt`
  - `duo_conversation_prompt`

これらは default 定義であり、ユーザー設定で上書きしない前提とする初期案とする。

理由:

- persona の正本を shared asset に一元化できる
- navigator / briefing / TTS の人格差分を同じ persona 定義に集約できる

### 2. ユーザー設定

既存の `audio_briefing_persona_voices` に以下を載せる。

- `tts_provider`
- `tts_model`
- `voice_model`
- `voice_style`

Gemini TTS では次のように解釈する。

- `tts_provider = "gemini_tts"`
- `tts_model = Gemini TTS model id`
- `voice_model = Gemini speaker / voice id`
- `voice_style` は原則未使用

つまり DB schema の大変更は避け、Gemini だけ `tts_model + voice_model` を主に使う。

## API / Worker 設計

### 1. single speaker

既存の `/audio-briefing/synthesize-upload` 相当の経路に `provider=gemini_tts` を追加する。

worker では:

- persona の `audio_briefing.tone_prompt`
- `audio_briefing.speaking_style_prompt`
- script chunk text
- `tts_model`
- `voice_model`

から Gemini single-speaker request を組み立てる。

prompt の構造は概ね:

- role / style 指示
- persona 固有 tone
- speaking style
- chunk のテキスト

### 2. duo multi-speaker

既存 single-speaker synth route とは別に、Gemini duo 専用 route を新設する。

例:

- `/audio-briefing/synthesize-upload-gemini-duo`

request には以下を含める。

- section type
- turn list
- host persona
- partner persona
- host model / voice
- partner model / voice
- output object key

worker では:

- section 内 turn list を MultiSpeakerMarkup または freeform dialogue へ変換
- speaker alias は固定で `Host1`, `Partner1` のような英数字のみを使う
- host / partner の persona prompt を組み立てる
- section 全体 prompt を追加する
- 1 section = 1 audio object として出力する

### 3. capability / validation

API 側に `gemini_tts` の capability を追加する。

- `supportsCatalogPicker = true`
- `supportsSeparateTTSModel = true`
- `requiresVoiceStyle = false`
- `supportsGeminiPerformancePrompt = true`
- `supportsMultiSpeaker = true`

また duo + Gemini multi-speaker を有効にする条件を追加する。

- host voice: `tts_provider == gemini_tts`
- partner voice: `tts_provider == gemini_tts`
- host `tts_model` / `voice_model` が設定済み
- partner `tts_model` / `voice_model` が設定済み

条件未達時は fallback しない。
UI でも API でも明示的に unavailable 扱いにする。

理由:

- 片方だけ Gemini で multi-speaker を組むと仕様が歪む
- 期待と異なる fallback はバグに見えやすい

## Web / UX

### 1. persona voice card

Audio Briefing の各 persona card に Gemini TTS を追加する。

Gemini 選択時に表示する項目:

- Provider
- Gemini TTS model
- Gemini TTS voice

voice picker は xAI / OpenAI と同様に catalog ページへつなぐ。

### 2. duo synthesis mode

Audio Briefing 全体設定に次を追加する。

- `Standard`
- `Gemini Multi-Speaker`

`Gemini Multi-Speaker` 選択時は readiness を表示する。

- host: ready / missing
- partner: ready / missing
- unavailable reason

### 3. persona prompt 編集

初期段階では shared persona 定義を直接 user-editable にしない。

理由:

- `shared/ai_navigator_personas.json` はプロダクト既定値であり、per-user mutable data とは性格が違う
- まずは固定 persona prompt で導入し、必要なら次段階で override UI を検討する

つまり初期実装では:

- shared JSON に default prompt を持つ
- UI では読み取り専用サマリ表示までに留めてもよい

## Catalog

Gemini TTS では `model` と `voice` の両方を扱う。

### モデル

初期は固定候補でよい。

- `gemini-2.5-flash-tts`
- `gemini-2.5-pro-tts`
- 必要なら `gemini-2.5-flash-lite-preview-tts`

理由:

- モデルは少数で、動的同期の必要性が低い
- UI でも select で十分

### voice

voice は catalog sync 対応が望ましいが、Gemini の voice 一覧 API の扱いが限定的なら初期は固定カタログでもよい。

推奨:

- 初期は curated voice catalog を shared asset or DB snapshot で管理
- 後で自動同期に拡張

## concat / postprocess

Gemini multi-speaker section も既存の audio-concat に流す。

既に:

- チャンク間 1 秒 gap
- 48kHz / stereo 正規化
- 最終成果物は VBR

になっているため、Gemini TTS 追加後も concat 層の基本設計は変えない。

## エラーハンドリング

- Gemini TTS single / duo で input size 上限超過時は明示エラー
- duo section が長すぎる場合は section をさらに再分割する fallback を将来余地として残すが、初期は error にする
- multi-speaker unavailable 時は silent fallback しない
- API / worker のログには `provider=gemini_tts`, `mode=single|multi_speaker`, `section`, `model`, `voice ids` を残す

## テスト

最低限追加する。

- shared persona loader が `audio_briefing` prompt を読む
- provider capability に `gemini_tts` が入る
- single speaker request で Gemini prompt が組み立つ
- duo で section 単位の multi-speaker request に変換される
- host / partner の Gemini readiness validation
- settings UI で Gemini provider 選択時に model + voice picker が出る

## 段階導入

### Phase 1

- Gemini TTS single speaker
- provider/model/voice 設定
- shared persona prompt 読み込み

### Phase 2

- duo multi-speaker
- section 単位生成
- readiness validation

### Phase 3

- Gemini voice catalog の改善
- per-user persona prompt override の検討

## 推奨実装方針

Gemini TTS は既存 provider 実装に継ぎ足すのではなく、

- provider dispatch には追加する
- ただし worker 実装は Gemini 専用 adapter に切り出す
- duo は Gemini 専用 synthesis path を新設する

を推奨する。

これが最も安全で、single / duo の違いと Gemini 固有の prompt-driven TTS を素直に表現できる。
