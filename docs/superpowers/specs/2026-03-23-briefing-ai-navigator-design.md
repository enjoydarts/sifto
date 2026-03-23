# Briefing AI Navigator Design

## Goal

ブリーフィング画面に、固定キャラの `AIナビゲーター` が直近取り込みの未読記事からおすすめを数本紹介する遊び機能を追加する。

狙いは次の3点。

- ブリーフィング体験に軽い人格と発見感を加える
- 未読記事の中から「今読むと面白い」候補を自然に提示する
- 既存の summary / digest とは別の LLM 用途として、モデル設定と usage を独立管理できるようにする

## Product Shape

- ブリーフィングを開くたびに、右下オーバーレイで `AIナビゲーター` を自動表示する
- オーバーレイはユーザーが閉じるまで残る
- そのページ滞在中に閉じたら再表示しない
- 次回訪問時には再び自動表示する
- 対象は `直近取り込みの未読記事` のみ
- ナビゲーターは `3本前後` の記事を短いコメント付きで紹介する

## Persona Model

初期バージョンでは 5 persona を持つ。

- `editor`
  - 落ち着いた編集者。要点整理がうまく、簡潔で信頼感のある文体
- `hype`
  - 熱量高めの案内役。勢いと期待感で読ませにいく
- `analyst`
  - 背景や含意を短く添える。やや理詰めで文量は少し長め
- `concierge`
  - やわらかく親しみのある案内。押しつけずに勧める
- `snark`
  - 軽口レベルで面白い毒舌。皮肉は入れるが、不快・攻撃的・見下しは禁止

persona 差分は `prompt + UI表情 + 文量` で表現する。初期実装から persona ごとに以下を分ける。

- 表示名
- 肩書き
- 導入文 tone
- 吹き出し/パネル配色
- アバター表情
- 1コメントの目標文量
- 口調制約

## Architecture

推奨方式は `briefing 生成時に navigator payload を一緒に作る` 形。

理由:

- ブリーフィング API / snapshot / cache と整合する
- ページ表示時の追加 LLM 待ちを避けられる
- 毎回の表示ごとに余分なコストが発生しない
- UI は payload を描画するだけでよく、フロントを薄く保てる

構成:

1. briefing build 時に未読候補記事を収集
2. navigator 用 prompt で LLM に `選定 + コメント生成` を依頼
3. `BriefingTodayResponse.navigator` に格納
4. snapshot / cache に briefing 本体と一緒に保存
5. web は右下オーバーレイで描画

## Data Flow

### Candidate Selection

候補記事は repository から次の条件で取得する。

- 対象 user の記事
- `is_read = false`
- `deleted / later` など通常の briefing 体験を壊す状態は除外
- 新しさを優先しつつ、source 偏りを抑える
- 最大 `10〜15件` を LLM 候補として渡す

候補に渡すデータ:

- `item_id`
- `title`
- `source_title`
- `published_at`
- `summary`
- 必要なら `score` や `cluster label`

### LLM Output

LLM には次を返させる。

- `intro`
- `picks[]`

`picks[]` の各要素:

- `item_id`
- `rank`
- `comment`
- `reason_tags[]`

想定表示本数は `3件`。候補不足時は `1〜2件` でも成立させる。

## API Contract

`BriefingTodayResponse` に `navigator` を追加する。

```json
{
  "navigator": {
    "enabled": true,
    "persona": "editor",
    "character_name": "ミナト",
    "character_title": "Briefing Navigator",
    "avatar_style": "editorial_calm",
    "speech_style": "calm_short",
    "intro": "今朝はこの3本から入ると流れがつかみやすいです。",
    "generated_at": "2026-03-23T00:10:00Z",
    "picks": [
      {
        "item_id": "xxx",
        "rank": 1,
        "comment": "まずこれ。今日の流れを一番きれいに掴めます。",
        "reason_tags": ["fresh", "high-signal"]
      }
    ]
  }
}
```

`navigator.enabled=false` または `navigator=null` のケースも許容する。

## Settings

`user_settings` に次を追加する。

- `navigator_enabled`
- `navigator_persona`
- `navigator_model`
- `navigator_fallback_model`

設定画面では少なくとも次を扱う。

- ON/OFF
- persona 選択
- model
- fallback model

persona 選択 UI はラベルだけでなく短い説明を付ける。

## Prompt Design

prompt は共通骨格 + persona profile の 2 層にする。

共通制約:

- 候補記事の中から未読ユーザーに今すすめる価値が高いものを選ぶ
- 3件前後まで
- コメントはネタバレしすぎず、読む理由を短く伝える
- 箇条書きではなく、オーバーレイ向けの短い話し言葉にする
- 事実にないことを足さない
- source の偏りが強すぎるときは分散を意識する

persona profile 差分:

- `tone`
- `max_comment_length`
- `intro_length`
- `allowed_humor_level`
- `forbidden_style`

`snark` persona には追加制約を入れる。

- 軽い皮肉は可
- 攻撃的、侮辱的、人格否定、煽りは禁止
- 読者や記事を下に見る表現は禁止

## UI Design

### Placement

- ブリーフィング画面の右下固定
- desktop ではフローティングオーバーレイ
- mobile では下端寄りのコンパクトシートに近い表示へ寄せる

### Structure

- persona avatar / 表情
- 名前
- 肩書き
- intro
- picks list
- 各 pick に `開く` アクション
- close button

### Visual System

共通レイアウトは維持しつつ、persona ごとに以下を変える。

- avatar illustration style token
- accent color
- bubble border / glow
- intro text rhythm
- comment line clamp

差分は theme token で表現し、5個の別コンポーネントには分けない。

## Usage and Billing

AI ナビゲーターは既存 summary 系とは別 purpose として扱う。

- purpose: `briefing_navigator`

これにより:

- モデル別 usage で切り出せる
- 将来 navigator だけ model を変えても追跡できる
- 月次 budget 上の内訳が見える

## Failure Handling

- LLM が失敗したら briefing 全体は失敗させない
- `navigator=null` で通常 briefing を返す
- fallback model がある場合は 1 回だけ切替
- 候補記事不足時は picks 数を減らす
- JSON parse 失敗時は navigator を落として briefing を継続

## Caching and Lifetime

- navigator payload は briefing snapshot に含める
- briefing cache と同じ寿命で扱う
- user がオーバーレイを閉じる状態は server には保存しない
- close 状態は page local state のみ

## Testing

最低限必要な検証:

- candidate selection が未読のみを返す
- persona ごとの prompt profile 切替
- navigator parse の正常系 / failure fallback
- briefing API が navigator 付きでも既存表示を壊さない
- usage purpose が `briefing_navigator` で記録される
- settings の persona/model 保存
- web で auto-open / close / reopen-on-next-visit が成立する

## Rollout

段階導入を推奨する。

1. `navigator_enabled` は default off で実装
2. 開発環境で persona と prompt を調整
3. 必要なら internal users のみ on
4. 安定後に default on を検討

毎回自動表示という UX は強いので、初期は設定で簡単に切れるようにする。

## Recommended First Scope

初手スコープは次。

- persona 5種
- briefing build 時生成
- 3 picks
- Settings で persona / model / fallback / enable
- right-bottom overlay
- usage purpose 分離

次フェーズ以降でよいもの:

- persona ごとの細かなモーション差
- avatar の完全なイラスト差し替え
- 過去のクリック率からの persona 最適化
- A/B testing
