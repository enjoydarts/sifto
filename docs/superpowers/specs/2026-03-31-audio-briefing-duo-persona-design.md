# Audio Briefing Duo Persona Design

## Goal

音声ブリーフィングを、既存の単独パーソナリティ読み上げに加えて、`host + partner` の二人会話形式でも生成できるようにする。

## Scope

- 音声ブリーフィング設定に `single / duo` の会話モード切替を追加する
- `duo` では既定 persona を `host`、ランダム選出 persona を `partner` として使う
- 台本 schema を単独 narration と会話 turns の両方に対応させる
- TTS を turn 単位で persona ごとに合成し、最終音声へ結合する
- `single` を既定として残し、`duo` が不安定でも即座に戻せるようにする

## Out of scope

- 二人ともランダムにするモード
- 相性ルール付きの固定コンビ生成
- 回ごとの一時上書き UI
- 三人以上の会話
- 会話量やキャラ相性の細かいチューニング UI

## Decisions

### 1. Conversation mode

設定は音声ブリーフィング全体の既定値として 1 つだけ持つ。

- `single`
- `duo`

初期値は `single` にする。既存の単独読み上げを壊さず、`duo` の品質検証を段階的に進めやすくするため。

### 2. Duo role model

`duo` は対等会話ではなく、役割を固定した `host + partner` にする。

- `host`: 既定 persona。進行、要点説明、記事本文の芯を担う
- `partner`: 全 persona からランダム選出。反応、比較、問い返し、補足視点を担う

`partner` は毎記事で必ず登場させる。会話感を明確に出しつつ、本文の軸は `host` に固定して冗長化を抑える。

### 3. Script structure

台本は section 単位の自由文ではなく、turn の列で表現する。

各 turn は少なくとも以下を持つ。

- `speaker`: `host` or `partner`
- `section`: `opening | overall_summary | article | ending`
- `item_id`: 記事 turn の場合のみ設定
- `text`: 読み上げ本文

`single` では既存の narration 構造を維持し、`duo` のみ turn 配列を生成する。既存 draft や再生成処理への影響面を最小化するため、両 mode を同一 schema に無理に寄せない。

### 4. Article turn pattern

各記事の会話パターンは固定する。

1. `host`: 記事導入
2. `partner`: 反応
3. `host`: 本文説明
4. `partner`: 比較や違和感の補足
5. `host`: 記事締め

`partner` は 2〜4 文程度で会話感を作るが、記事の一次説明責務は持たない。`host` の言い換えを避け、必ず新しい視点を足す。

### 5. Prompt constraints

`duo` 用 prompt では以下を強く固定する。

- `host` は進行役として記事の芯を説明する
- `partner` は相槌だけで終わらず、比較、違和感、理由、見方を足す
- `partner` は `host` が言った情報の言い換えをしない
- 同じ導入句、同じ締め句を turn をまたいで繰り返さない
- 文字数が不足する場合は `host` の本文と `partner` の補足を厚くする

### 6. Voice mapping

音声設定は既存の persona ごとの voice mapping をそのまま使う。

`duo` の追加設定は持たず、

- `host` は既定 persona の voice mapping
- `partner` はランダム選出された persona の voice mapping

をそのまま参照する。

voice 未設定 persona が `partner` に選ばれた場合は、その回は `single` にフォールバックするか、`partner` 候補を再抽選する。初期実装では再抽選を優先する。

## Data changes

### audio_briefing_settings

- `conversation_mode text not null default 'single'`

許可値は `single`, `duo`。

### audio_briefing_jobs

- `conversation_mode text not null default 'single'`
- `partner_persona text null`

生成時点の mode と `partner` を job 側へ固定して、再生成やデバッグで同一条件を追えるようにする。

### narration payload

`duo` 用に `turns[]` を持つ新しい narration 形式を追加する。

## Flow

1. job 作成時に `conversation_mode` を settings から固定する
2. `single` なら既存フローをそのまま使う
3. `duo` なら既定 persona を `host` とし、voice 設定済み persona から `partner` をランダム選出する
4. worker が `duo` 用 script prompt で turn 配列を生成する
5. API が turn 配列を保存し、voicing 対象 chunk を speaker 単位で分割する
6. worker TTS が turn ごとに対応 persona の voice で音声を生成する
7. concat が turn 順に結合し、完成音声を publish する

## Failure handling

- `partner` 選出不可
  - `single` にフォールバックして継続する
- `duo` script parse 失敗
  - 既存の batch retry を維持し、失敗時は job を failed
- `partner` 側 voice 設定不備
  - 別 persona を再抽選し、それでも解決しなければ `single` にフォールバック
- turn 単位 TTS 失敗
  - 既存の chunk retry / stale recovery を流用する

## UI

設定画面の音声ブリーフィング基本設定に会話モードを追加する。

- `single`: 一人読み上げ
- `duo`: 二人会話

説明文は「`duo` では既定 persona が司会役になり、相棒 persona がランダムで会話参加する」とする。

初期リリースでは、`partner` 候補の固定、会話量調整、相性プリセット UI は追加しない。

## Testing

- Go:
  - settings の `conversation_mode` 保存と取得
  - job 作成時の `conversation_mode` / `partner_persona` 固定
  - `duo` 時の `partner` 選出と fallback
  - narration turn から TTS chunk 生成への変換
- Python:
  - `duo` script prompt が `host + partner` の turn 配列を返す
  - `partner` が `host` の単純言い換えにならない validation
  - turn ごとの persona voice ルーティング
- Integration:
  - `single` 既存フローが退行しない
  - `duo` で完成音声まで通る
  - voice 未設定 partner を引いたときの fallback

## Rollout

1. schema と settings を追加する
2. `single` / `duo` の mode 固定を job 作成へ入れる
3. `duo` script schema と prompt を追加する
4. turn 単位 TTS と concat を入れる
5. 内部検証後に `duo` を UI から有効化する
