# Gemini TTS追加実装計画

## 方針

Gemini TTS は既存 provider に単純追加せず、次の 2 本立てで進める。

- single speaker: 既存 TTS dispatch に `gemini_tts` を追加
- duo multi-speaker: Gemini 専用 synthesis path を新設

persona の tone / performance は `shared/ai_navigator_personas.json` の `audio_briefing` セクションから読む。
voice は API sync ではなく curated catalog で扱う。

## Task 1: shared persona 定義と curated voice catalog の追加

### 目的

- persona 定義に `audio_briefing` prompt を持たせる
- Gemini voice 一覧を shared asset として追加する

### 実装

- `shared/ai_navigator_personas.json`
  - 各 persona に `audio_briefing`
    - `tone_prompt`
    - `speaking_style_prompt`
    - `duo_conversation_prompt`
- `shared/gemini_tts_voices.json`
  - curated voice list
  - `voice_id`
  - `name`
  - `description`
  - 必要なら `recommended_for`

### 検証

- persona loader テスト
- catalog JSON の load テスト

## Task 2: API / model / capability への Gemini TTS 追加

### 目的

- `gemini_tts` を Audio Briefing voice provider として認識させる

### 実装

- `api/internal/model/model.go`
- `api/internal/service/tts_provider_capabilities.go`
- `api/internal/service/settings_service.go`
- `api/internal/handler/settings.go`

追加内容:

- `provider=gemini_tts`
- `tts_model` 必須
- `voice_model` 必須
- `voice_style` 不要
- Duo multi-speaker readiness 用 validation

### 検証

- settings save/load テスト
- validation テスト

## Task 3: Gemini curated voice catalog API

### 目的

- Web から Gemini voice picker 用データを取得できるようにする

### 実装

- API に read-only endpoint を追加
  - `GET /gemini-tts-voices`
- shared JSON を読み込んで返す service / handler を追加

補足:

- xAI / OpenAI のような `/sync` と `/status` は作らない
- catalog 更新は repo asset 更新で行う

### 検証

- handler test
- catalog parse test

## Task 4: Worker に Gemini single-speaker adapter を追加

### 目的

- single speaker で Gemini TTS を生成できるようにする

### 実装

- `worker/app/services/gemini_tts.py` 新設
  - Gemini API single-speaker request
  - model / voice / prompt 構築
  - response audio decode
- `worker/app/services/tts_provider_registry.py`
  - `gemini_tts` dispatch 追加
- `worker/app/services/audio_briefing_tts.py`
  - single speaker synthesis 経路に Gemini 追加
- `worker/app/services/summary_audio_player.py`
  - Summary Audio に Gemini を通すかは別判断
  - 初期は Audio Briefing 優先

### prompt 構築

- persona `tone_prompt`
- persona `speaking_style_prompt`
- chunk text

### 検証

- worker unittest
- request payload テスト

## Task 5: Worker に Gemini duo multi-speaker adapter を追加

### 目的

- duo で `OP / 総括 / 各記事 / ED` 単位の multi-speaker synthesis を行う

### 実装

- `worker/app/services/gemini_tts.py`
  - `synthesize_gemini_multi_speaker_tts(...)`
- Gemini 専用 router 追加
  - 例: `/audio-briefing/synthesize-upload-gemini-duo`
- request:
  - section type
  - turn list
  - host / partner model
  - host / partner voice
  - persona prompts

### section 分割

- OP
- 総括
- 各記事
- ED

### 検証

- turn list から Gemini multi-speaker request 変換テスト
- alias validation テスト
- section 単位生成テスト

## Task 6: API の duo Gemini synthesis orchestration

### 目的

- duo かつ Gemini ready のときだけ Gemini multi-speaker 経路を使う

### 実装

- `api/internal/service/audio_briefing_voice.go`
- 必要なら `audio_briefing_pipeline.go`

追加内容:

- job / chunk 単位ではなく section 単位の Gemini duo dispatch
- readiness check
- fallback なしの explicit unavailable

### 検証

- duo readiness テスト
- section dispatch テスト

## Task 7: Web settings UI

### 目的

- persona ごとに Gemini `provider / model / voice` を選べるようにする
- Duo multi-speaker availability を見える化する

### 実装

- `web/src/app/(main)/settings/page.tsx`
- `web/src/lib/api.ts`
- i18n dictionaries

追加内容:

- provider select に `gemini_tts`
- Gemini model select
- curated voice picker
- duo synthesis mode
- Gemini multi-speaker readiness UI

### 検証

- `docker compose exec -T web npm exec tsc --noEmit`

## Task 8: Prompt / summary / logging 整備

### 目的

- Gemini TTS の observability を確保する

### 実装

- provider / mode / model / voice ids / section type をログに残す
- 必要なら usage 集計の provider 追加

### 検証

- worker / API test

## Task 9: 総合検証

### 確認項目

- single + Gemini TTS
- duo + Gemini multi-speaker
- provider 切替時の settings 保存
- curated voice picker
- concat 後の最終音声出力

### 実行

- `make fmt-go`
- `docker compose exec -T api go test ./internal/handler ./internal/repository ./internal/service`
- `make check-worker`
- `docker compose exec -T web npm exec tsc --noEmit`
- 必要なら `make web-build`

## 実行順

1. shared persona / curated catalog
2. API capability / settings
3. Gemini voice catalog read API
4. worker single-speaker adapter
5. worker duo multi-speaker adapter
6. API orchestration
7. Web settings UI
8. observability
9. 総合検証

## リスク

- Gemini API の multi-speaker 制約が想定より厳しい可能性
- section text がサイズ上限を超えるケース
- voice catalog に sync API がないため curated list メンテが必要
- 既存 chunk 単位 pipeline と section 単位 Gemini duo の整合

## 推奨進め方

subagent-driven で分ける。

- Worker single / duo
- API capability / orchestration
- Web settings UI

を書き込み範囲を分けて並列に進めるのがよい。
