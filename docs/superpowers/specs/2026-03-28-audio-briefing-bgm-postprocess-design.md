# Audio Briefing BGM Postprocess Design

## Goal

音声ブリーフィングの最終完成音声に、R2 上の BGM をランダム選択で薄く重ね、フェードとラウドネス正規化をまとめて適用する。

## Scope

- 既存のチャンク結合 Cloud Run Job を拡張し、BGM 合成と loudness normalize を同じ後処理で行う
- BGM の有効/無効と R2 prefix を音声ブリーフィング設定に追加する
- 完成音声に使った BGM ファイル名を job メタデータとして保存する
- BGM 処理に失敗しても、本編のみで publish を継続する

## Decisions

### 1. Postprocess location

後処理は既存の `infra/audio-concat` Cloud Run Job に寄せる。`ffmpeg` 依存を API / worker から切り離しつつ、チャンク結合後の最終音声処理を 1 箇所に集約する。

### 2. BGM selection

- BGM 素材は R2 の特定 prefix 配下に配置する
- 配信生成ごとに毎回ランダムで 1 曲選ぶ
- 前後編で同じ曲に固定しない
- BGM は本編長に合わせてループし、先頭 2 秒 fade-in、末尾 4 秒 fade-out をかける
- 音量は固定の薄いゲインにする

### 3. Loudness normalize

最終音声は `-16 LUFS / true peak -1.5 dB` を目安に正規化する。BGM の有無に関係なく常時適用する。

### 4. Failure handling

- BGM 候補が空
- BGM ダウンロード失敗
- BGM ミックス失敗

上記はいずれも soft failure とし、チャンク結合済みの本編だけを完成音声として publish する。concat 自体が失敗した場合のみ job を failed にする。

## Data changes

### audio_briefing_settings

- `bgm_enabled boolean not null default false`
- `bgm_r2_prefix text null`

### audio_briefing_jobs

- `bgm_object_key text null`

## Flow

1. API が音声ブリーフィング設定から `bgm_enabled` / `bgm_r2_prefix` を取得する
2. concat 起動時に Cloud Run Job override env へ設定を渡す
3. Cloud Run Job がチャンクを結合する
4. `bgm_enabled=true` かつ prefix があれば R2 prefix 配下の音声ファイルから 1 曲選ぶ
5. `ffmpeg` で loop/trim/fade/amix を適用する
6. `loudnorm` をかけて完成音声を upload する
7. internal callback に `audio_object_key`, `audio_duration_sec`, `bgm_object_key` を返す
8. API が job に最終音声と BGM メタデータを保存する

## UI

設定画面の音声ブリーフィング基本設定に以下を追加する。

- BGM を有効にする
- BGM R2 prefix

初期リリースでは曲プレビュー、重み付け、音量微調整 UI は入れない。

## Tests

- Go:
  - settings payload / validation
  - concat starter が BGM 設定を runner request に渡す
  - concat complete callback が `bgm_object_key` を保存する
- Python:
  - BGM 無効時は concat + normalize のみ
  - BGM 有効時は候補から 1 曲選んで mix を行う
  - BGM 失敗時は本編だけで継続する
