# 記事詳細 AIナビゲーター設計

## 目的

記事詳細画面の右下に AI ナビゲーターのミニアイコンを常駐させ、クリック時にだけその記事の論評を生成して表示する。ブリーフィング用ナビゲーターとは用途もコスト特性も異なるため、API・キャッシュ・usage を分離する。

## 要件

- 記事詳細画面の右下に、persona に応じた顔アイコンの丸ボタンを表示する
- ページを開いただけでは生成しない
- ユーザーがアイコンをクリックしたときだけ初回生成する
- 生成後は右下オーバーレイで論評を表示する
- 閉じた後もミニアイコンは残し、再クリックで再表示できる
- 論評は `4〜7文` の中尺短評にする
- 使う材料は `title / translated_title / summary / facts / source / published_at`
- 本文全文は初期スコープでは使わない
- モデル設定と usage はブリーフィング用とは別に扱う

## アプローチ比較

### 案A: 記事詳細 API に同梱する

- 長所: endpoint が少ない
- 短所: ページを開くだけで生成コストが発生しやすい。記事詳細本体と責務が混ざる

### 案B: 記事詳細用の別 endpoint を作る

- 長所: クリック時だけ遅延生成できる。30分キャッシュを独立管理できる。usage も独立させやすい
- 短所: API と frontend query が 1 本増える

### 案C: frontend から worker を直接叩く前提に寄せる

- 長所: API 層の実装量は減る
- 短所: secret 管理、cache、usage 記録、settings 解決が崩れるので不適

推奨は `案B`。既存のブリーフィング AI ナビゲーターと同じ設計原則を保ちつつ、生成タイミングだけ `クリック時` に切り替えられる。

## API 設計

新規 endpoint:

- `GET /api/items/{id}/navigator`

責務:

- item detail をもとに論評生成候補を組み立てる
- settings から persona / model / fallback を解決する
- `item_id + persona + resolved model + preview` をキーに 30 分キャッシュする
- cache miss 時だけ worker に生成を依頼する

返却形式:

```json
{
  "navigator": {
    "enabled": true,
    "persona": "snark",
    "character_name": "ジン",
    "character_title": "Snark Guide",
    "avatar_style": "snark",
    "speech_style": "wry",
    "headline": "地味に大事な話",
    "commentary": "4〜7文の論評本文",
    "stance_tags": ["重要", "実務", "警戒"],
    "generated_at": "2026-03-23T21:00:00+09:00",
    "item_id": "..."
  }
}
```

初期版では `headline` と `stance_tags` を optional にしてもよいが、将来の UI 余地を考えると返す前提で揃える。

## 生成ロジック

入力材料:

- `title`
- `translated_title`
- `summary`
- `facts`
- `source.title`
- `published_at`
- `persona`

LLM の役割:

- 要約の言い直しではなく、その記事への短い論評を作る
- `なぜ気にする価値があるか`
- `どこが面白いか`
- `どこを少し警戒するか`
- persona ごとの口調差を出す

制約:

- 候補記事は 1 本だけなので、ブリーフィングのようなピック列挙はしない
- 事実の捏造は禁止
- summary / facts にないことは断定しない
- snark でも読者個人はいじらず、話題や状況への軽い皮肉に留める

## キャッシュ戦略

- cache key: `user_id + item_id + persona + resolved_model`
- TTL: `30分`
- 記事ごと・モデルごとに独立
- persona または model の設定変更時は key が変わるので自然に cache miss

これにより、同じ記事を開き直しても毎回は LLM を叩かない。

## Usage / コスト

新しい purpose:

- `item_navigator`

方針:

- `briefing_navigator` とは別 purpose にする
- `LLM Usage` で独立集計できるようにする
- cost attribution は既存の `recordAskLLMUsage` 経路を流用する

必要なら `llm_usage_logs_purpose_check` に `item_navigator` を追加する migration を入れる。

## Settings

初期版では新しい model 設定を増やさず、既存の AI ナビゲーター設定を共用する。

- `navigator_enabled`
- `navigator_persona`
- `navigator_model`
- `navigator_fallback_model`

理由:

- 設定を増やしすぎると操作負荷が上がる
- ブリーフィングと記事詳細で同じキャラが喋る方が体験として一貫する

将来必要なら `item_navigator_*` の別設定に分離できるよう、backend 内部では purpose を分ける。

## Frontend UI

記事詳細ページでの表示状態は 3 段階:

1. 初期状態
   右下にキャラ顔だけの丸ボタンを表示
2. 生成中
   ボタン押下後、同位置にローディング状態を表示
3. 生成後
   右下オーバーレイで論評を表示

オーバーレイの内容:

- ヘッダー: persona アイコン、名前、肩書き
- 本文: `headline` + `commentary`
- フッター: `stance_tags`
- 閉じるボタン: あり

閉じた後:

- 同一訪問中もミニアイコンは残す
- 再クリックで再表示
- 再表示時は cache hit があれば即表示

## エラーハンドリング

- 生成失敗時はプレーンな短いエラー文を表示
- item に summary / facts が不足している場合は生成せず、`論評に必要な材料がまだ揃っていません` 系の文言を返す
- deleted item では初期版はアイコン自体を出さないか、押下時に unavailable を返す

推奨は `deleted では非表示`。readonly 状態の記事に新しい遊び導線を足さないほうが自然。

## テスト方針

API:

- cache key が `item_id + persona + model` で分かれる
- クリック時生成 endpoint が `navigator` payload を返す
- summary / facts 不足時の fallback を確認する
- `item_navigator` usage が記録されることを確認する

Worker:

- prompt に `4〜7文`
- `summary / facts ベース`
- persona ごとの差分
- snark の安全制約

Web:

- 右下ミニアイコン表示
- クリックで loading -> overlay
- close 後の再表示
- 生成失敗時 UI

## 実装順

1. worker に記事詳細用 navigator task / parser / router を追加
2. API に `GET /api/items/{id}/navigator` を追加
3. `item_navigator` usage purpose を追加
4. article detail frontend に右下ミニアイコン + overlay を追加
5. cache / error / loading の仕上げ

