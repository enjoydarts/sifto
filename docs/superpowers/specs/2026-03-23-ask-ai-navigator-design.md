# Ask AI Navigator Design

Date: 2026-03-23

## Goal

`Ask` 画面の回答の直後に、AI ナビゲーターが `前提のズレ / 留保 / 次に掘る論点` をキャラ付きで返す機能を追加する。

この機能は、回答を言い換える補足ではなく、`問いの見方を一段ずらす案内役` として振る舞う。

## Product Shape

- `Ask` の本体回答は現状維持
- 回答の取得後、AI ナビゲーターを自動表示する
- ナビゲーターは閉じられる
- 表示内容は `headline + commentary + next_angles[]`
- `commentary` は `5〜8文`
- `next_angles[]` は `2〜4件`

## Why Separate It

`Ask` 本体と AI ナビゲーターは役割もコスト特性も違う。

- `Ask` 本体: 質問に答える
- `Ask Navigator`: 問いの前提、留保、次の論点を刺す

そのため、同じ endpoint に混ぜず、別 endpoint / 別 cache / 別 usage purpose に分離する。

## API Design

### Existing

- `POST /api/ask`
  - そのまま維持

### New

- `POST /api/ask/navigator`

Request:

```json
{
  "query": "質問文",
  "answer": "Ask本体の回答",
  "bullets": ["補足1", "補足2"],
  "citations": [
    {
      "item_id": "uuid",
      "title": "記事タイトル",
      "url": "https://...",
      "reason": "この論点を支える理由"
    }
  ],
  "related_items": [
    {
      "id": "uuid",
      "title": "記事タイトル",
      "translated_title": "翻訳タイトル",
      "summary": "summary",
      "facts": ["fact1", "fact2"]
    }
  ]
}
```

Response:

```json
{
  "navigator": {
    "enabled": true,
    "persona": "native",
    "character_name": "ネイティブ 春香",
    "character_title": "AIネイティブ",
    "avatar_style": "native",
    "speech_style": "light",
    "headline": "この問いで見落としやすいところ",
    "commentary": "5〜8文の論評",
    "next_angles": [
      "次に掘る論点1",
      "次に掘る論点2"
    ],
    "generated_at": "2026-03-23T..."
  }
}
```

## Cache Strategy

- cache key:
  - `query + answer + persona + resolved model`
- TTL:
  - `30分`

これにより、同じ問いと同じ回答を開き直しても毎回は LLM を叩かない。

`Ask` 本体の cache とは分離する。

## Generation Inputs

Worker に渡す入力は以下。

- `query`
- `answer`
- `bullets`
- `citations`
- `related_items`
- `persona`
- `model`

この構成により、ナビゲーターは `Ask の回答テキストだけ` をなぞるのではなく、`その回答が何を前提にしているか`、`どこが掘りどころか` を判断できる。

## Prompt Direction

AI ナビゲーターは以下を強く守る。

- 回答の要約は禁止
- 客観的レビューではなく、そのペルソナの主観で語る
- 質問に潜む前提のズレや、引用候補から見える留保を拾う
- 次に掘ると面白い論点を `next_angles[]` に落とす
- `commentary` は 5〜8 文
- `next_angles[]` は 2〜4 件

期待する論調:

- `この質問、そこを一足飛びに結ぶと雑になる`
- `いま答えは出ているが、実は次に見るべき論点はこっち`
- `候補記事だけでは断定しにくいところがある`

## UI Design

`Ask` 画面で回答が表示された後、AI ナビゲーターを自動表示する。

### Behavior

- 回答表示後に自動表示
- 閉じるボタンあり
- 同じ回答セッション中は、閉じたら自動再表示しない
- 再度同じ質問を送るか、ページを開き直したら再び表示対象になる

### Visual Placement

- 回答カードの直後、または右下固定のどちらでも成立する
- 初期版は `回答直下の独立カード` が自然

理由:

- `Ask` は質問と回答の文脈が一本なので、右下オーバーレイより本文の流れに接続した方が読みやすい
- `ブリーフィング / 記事詳細` のような常駐キャラより、`回答後の一言` の性格が強い

### Card Content

- avatar
- character_name / character_title
- headline
- commentary
- next_angles chips or short list
- close button

## Settings

初期版では既存の AI ナビゲーター設定を流用する。

- `navigator_enabled`
- `navigator_persona`
- `navigator_model`
- `navigator_fallback_model`

将来的に `briefing / item / ask / sources` を別々に分ける余地は残すが、初手では増やしすぎない。

## Usage / Cost

LLM Usage では独立 purpose を追加する。

- `ask_navigator`

これにより、

- `ask`
- `briefing_navigator`
- `item_navigator`
- `source_navigator`
- `ask_navigator`

を別々に集計できる。

DB 制約の `purpose check` にも追加が必要。

## Error Handling

- navigator 生成失敗時でも `Ask` 本体の回答は壊さない
- UI はナビゲーター領域だけを silent fail か、軽いエラーメッセージにする
- 空キャッシュを長時間保持しない
- `navigator == nil` は cache しない

## Testing

### API

- `POST /api/ask/navigator` の正常系
- cache hit / miss
- disabled 時に空 envelope
- `navigator == nil` を cache しない

### Worker

- schema が strict JSON mode で成立する
- prompt に `再要約禁止 / 前提 / 留保 / 次の論点` が入っている
- `next_angles[]` を返す前提が prompt に入っている

### Web

- 回答後に自動表示される
- close できる
- 再送時は新しい結果に差し替わる
- loading / error 表示

## Recommended Implementation Order

1. API model / cache key / endpoint 追加
2. worker ask_navigator router と task 追加
3. `ask_navigator` purpose migration 追加
4. web で回答後の自動表示カード追加
5. usage / cache / error handling 確認

## Recommendation

この機能は `Ask の答えをもう一回説明する機能` にしないことが重要。

価値が出るのは、`答えたあとに、キャラがその問いの見方を少しずらす` ときである。
そのため、設計の重心は常に `前提 / 留保 / 次の論点` に置く。
