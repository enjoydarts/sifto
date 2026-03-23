# Briefing AI Navigator Intro Enhancement Design

## Goal

既存の `AIナビゲーター` を、単なる `おすすめ記事の紹介役` から、ブリーフィング冒頭で `2〜3文の自然な導入トーク` を話すキャラクターへ拡張する。

狙いは次の3点。

- キャラクター性を `記事コメント` だけでなく `登場時の話し方` にも反映する
- ブリーフィング開始時に、時間帯・曜日・季節感に沿った空気をつくる
- おすすめ記事リストに入る前の導線をなめらかにして、オーバーレイ全体の読み味を上げる

## Product Shape

- AIナビゲーターの冒頭吹き出しは `1文` ではなく `2〜3文` にする
- 内容は次の流れを基本にする
  - 1文目: 時間帯に合った挨拶
  - 2文目: 日付・曜日・季節・時刻に沿った自然な小話
  - 3文目: 今日のおすすめへの橋渡し
- `今日は何の日` は固定データや外部APIには依存しない
- 事実断定よりも、`時節に沿った自然な話題` を優先する

## Scope

今回の拡張は `navigator.intro` の中身を強化することが中心で、初手では API contract を細かく分割しない。

- `intro` は string のまま維持する
- UI は既存の吹き出し1枠を維持する
- LLM input を増やし、prompt 制約を強くする
- persona ごとの導入文 tone と文量差分を増やす

## Recommended Approach

推奨は `intro を 2〜3文の複合導入文にする` 方式。

理由:

- 現在の API / worker / web の契約を大きく崩さない
- snapshot / cache / usage の仕組みをそのまま使える
- persona ごとの違いを prompt だけでかなり出せる
- 将来必要なら `greeting / small_talk / bridge` に分割する余地も残る

今回は `structured output のフィールド追加` ではなく、`intro の品質と構造の強化` を先にやる。

## Prompt Input

記事候補に加えて、導入トーク用に次の文脈を LLM に渡す。

- `now_jst`
- `date_jst`
- `weekday_jst`
- `time_of_day`
  - `morning`
  - `afternoon`
  - `evening`
  - `late_night`
- `season_hint`
  - 例: `early_spring`, `rainy_season`, `mid_summer`, `late_autumn`

`season_hint` は API 側で JST 日付から軽く決め打ちしてよい。厳密な天候や祝日データは持ち込まない。

## Prompt Output Rules

`intro` には次の制約を入れる。

- 2〜3文で完結させる
- 1文目は挨拶として自然に始める
- 2文目で日付・曜日・季節・時間帯に沿った軽い小話を入れる
- 3文目でおすすめ記事への橋渡しをする
- 不確かな `今日は何の日` を断定しない
- 実在の祝日・記念日・イベントを断定的に言い切らない
- 雑学の正確性より、自然な読み味を優先する
- それでも記事紹介と無関係な長話にはしない

## Persona Differentiation

persona ごとの差分は `intro` にも強く反映する。

- `editor`
  - 落ち着いた書き出し
  - 季節感は控えめ
  - 橋渡しは簡潔
- `hype`
  - 挨拶の勢いを強める
  - 小話も前向きでテンポよく
  - 橋渡しは期待感を出す
- `analyst`
  - 小話に曜日やタイミングの意味づけを少し入れる
  - 少し長めでも許容
- `concierge`
  - やわらかく、気圧の低い案内
  - 季節感や生活感を自然に混ぜる
- `snark`
  - 軽口レベルの皮肉は可
  - 不快・攻撃・見下しは禁止
  - 時節の話題にも少し乾いたユーモアを混ぜる

## API / Data Contract

初手では `BriefingTodayResponse.navigator` の shape は変えない。

```json
{
  "navigator": {
    "intro": "こんばんは。週明けのこの時間帯は、頭を切り替えるきっかけになる記事から入ると流れをつかみやすいですね。今日はこの3本から見ていきましょう。"
  }
}
```

将来的に UI をより演出的にしたくなったら、次のような分割は可能。

- `greeting`
- `small_talk`
- `bridge`

ただし今回は見送る。

## UI Behavior

UI 構造は維持する。

- 既存の intro 吹き出しをそのまま使う
- 2〜3文になっても読みやすいように行間を少し優先する
- persona ごとの泡デザイン・色・表情差分はそのまま流用する

初手では導入トーク専用の別パネルは作らない。

## Failure Handling

- LLM が導入トークを 1文しか返さなくても受け入れる
- あまりに長い場合は post-process で文数または文字数を抑える
- `今日は何の日` 的な断定が入ったとしても、初手では hard fail しない
- ただし prompt で `不確かな記念日を断定しない` 制約を明示する

## Testing

最低限必要な確認は次。

- persona ごとに intro tone が切り替わる
- prompt に `2〜3文` と `時間帯/季節/橋渡し` の制約が入る
- `snark` に安全制約が維持される
- 既存の picks 生成を壊さない
- intro が空や極端に短い場合の fallback が効く

## Rollout

- 既存 navigator の上位互換として導入する
- settings や usage purpose は変えない
- snapshot / cache の更新頻度も変えない

## First Implementation Slice

1. worker の navigator prompt に `now_jst / weekday / season_hint / time_of_day` を追加
2. `intro` の制約を `2〜3文` に強化
3. persona ごとの intro tone 差分を増やす
4. web の intro 吹き出し表示を長文でも読みやすい状態に微調整する

