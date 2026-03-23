# Sources AI Navigator Design

## Goal

`Sources` 画面で、AIナビゲーターが購読ソース全体を棚卸ししてくれる機能を追加する。
右下のナビゲーターボタンを押したときだけ生成し、ユーザーに「何を残すか / 何を見直すか」の判断材料を返す。

記事単位ではなく、`ソース一覧全体` に対する論評であることを明確にする。

## User Experience

- `Sources` 画面の右下に AIナビゲーターボタンを置く
- 初期状態はミニアイコンのみ
- クリック時に初回生成する
- 生成中はアイコン + 横長の loading バブルを出す
- 成功後は右下オーバーレイで結果を表示する
- 閉じたらミニアイコンだけ残し、再クリックで再表示できる
- 自動表示はしない

## Output Shape

AIナビゲーターは次の構造で返す。

- `overview`
  - 6〜10文程度の、かなりしっかりした総評
  - 全体の購読バランス、ノイズ、強い領域、弱い領域、見直しポイントまで話す
- `keep`
  - 残す価値が高いソースの候補
- `watch`
  - 最近少し怪しい、または見直し候補のソース
- `standout`
  - 最近効いている、読書体験に寄与しているソース

各リスト項目には次を含める。

- `source_id`
- `title`
- `comment`

リストはそれぞれ 0〜3 件でよい。

## Data Input

LLM に渡す材料は `直近30日` を基本とする。

各ソースについて次を集計する。

- `source_id`
- `title`
- `url`
- `enabled`
- `last_fetched_at`
- `total_items_30d`
- `unread_items_30d`
- `read_items_30d`
- `favorite_count_30d`
- `avg_items_per_day_30d`
- `failure_rate`
- `status`

必要に応じて追加する候補:

- `last_item_at`
- `active_days_30d`
- `avg_items_per_active_day_30d`

この機能の主眼は「ソース運用の健全性」と「読書価値」なので、記事本文や記事要約までは渡さない。

## Generation Rules

ナビゲーター prompt では次を強く指示する。

- ソース一覧全体の棚卸しとして話す
- 個別記事の推薦に逃げない
- 実データに基づいて論評する
- 単なる件数の読み上げにしない
- `overview` では全体傾向、偏り、ノイズ、維持価値、見直しポイントを自然文でまとめる
- `keep/watch/standout` は観点を変えて選ぶ
- ペルソナごとの価値観で評価する
- 他キャラ名を名乗らない

## API Design

新規 endpoint:

- `GET /api/sources/navigator`

レスポンス:

- `navigator`
  - `enabled`
  - `persona`
  - `character_name`
  - `character_title`
  - `avatar_style`
  - `speech_style`
  - `overview`
  - `keep[]`
  - `watch[]`
  - `standout[]`
  - `generated_at`

## Caching

- 生成はボタンクリック時のみ
- `user_id + persona + resolved model` をキーに `30分` キャッシュ
- `cache_bust=1` を debug 用に許可してよい
- persona や model が変われば別キーになる

## Cost / Usage

- usage purpose は `source_navigator`
- `llm_usage_logs` に新しい purpose を追加する migration が必要
- `briefing_navigator` / `item_navigator` とは分離して計上する

## Settings

初期版では既存の AIナビゲーター設定を流用する。

- `navigator_enabled`
- `navigator_persona`
- `navigator_model`
- `navigator_fallback_model`

将来的に `source navigator` 専用設定を分ける余地は残すが、初期版では追加しない。

## UI Notes

- 記事詳細やブリーフィングと同じアバター基盤を流用する
- モバイルでは下部ナビに被らない高さに置く
- オーバーレイは縦長になりすぎないよう `max-height + inner scroll`
- `overview` は読み物として十分な長さを確保する
- 下の3セクションはカード分割して視認性を上げる

## Failure Handling

- 生成失敗時はオーバーレイ内にプレーンなエラー文を出す
- 利用可能なソースが少なすぎる場合も、その旨を分かる文面で返す
- ソース0件なら `overview` は「まだ棚卸しする材料が少ない」系の文面にする

## Recommended Implementation Order

1. `source navigator` 用 response model / worker contract 追加
2. `sources` 集計データの repository/service 実装
3. `/api/sources/navigator` endpoint 追加
4. worker prompt / parser 実装
5. usage purpose 追加
6. `Sources` 画面の右下 UI 実装
7. cache / loading / error handling
