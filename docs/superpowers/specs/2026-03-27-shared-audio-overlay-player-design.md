# 共通オーバーレイ音声プレイヤー設計

## 概要

要約連続再生と音声ブリーフィング再生を、ページ固有の `<audio>` ではなく app 全体で共有する単一プレイヤーに統合する。

目的は次の 3 点。

- 再生中でも Sifto 内の別ページへ移動できること
- Google Meet や Spotify のように、下部ミニプレイヤーから再生面へ戻れること
- 要約連続再生と音声ブリーフィングで UI と再生制御を共通化すること

初回実装では、Sifto 内の SPA 遷移で再生を維持するところまでを対象にする。
desktop 別窓化と page reload 復元は今回の対象外とする。

## 要件

### 機能要件

- `(main)` 配下のどのページへ移動しても再生が維持される
- 下部ミニプレイヤーから再生・一時停止・次へ・終了・展開ができる
- 展開時はオーバーレイプレイヤーを表示する
- 要約連続再生と音声ブリーフィング再生は同じプレイヤー shell を使う
- 要約連続再生の再生キューはページ遷移後も維持される
- 要約連続再生は後続 1 件の先読みを維持する
- 30 秒既読判定は、実際に音声が流れている累積時間のみで判定する
- 音声ブリーフィングの詳細ページからも共通プレイヤーで再生を開始できる
- 既に何かを再生中に別モードを開始した場合、後から開始した方で現在再生を置き換える

### 非機能要件

- `<audio>` 要素は app 内で 1 つだけにする
- モバイルでは下部バー + 全画面シート、desktop では下部バー + オーバーレイを基本とする
- グローバル player 追加によって既存ページの初期表示を大きく重くしない
- i18n は既存辞書方式に揃える

## 対象範囲

### 今回やる

- 共通 player provider / store
- `(main)` layout 常駐のミニプレイヤー
- 展開オーバーレイ UI
- 要約連続再生 state の provider 移管
- 音声ブリーフィング detail page の page-local `<audio>` 廃止
- 再生開始 API の共通化

### 今回やらない

- desktop の別窓プレイヤー
- page reload 後の再生状態復元
- 他サイトへ同一タブで移動した際の再生維持
- Android / iOS の OS レベル Media Session 最適化の作り込み

## 採用案

### 単一プレイヤーエンジン + 単一セッション

app 全体で 1 つの player engine を持つ。
mode は `summary_queue` と `audio_briefing` の 2 種類だけにし、現在再生は常に 1 つに限定する。

理由:

- 再生競合と UI 競合を避けやすい
- `<audio>` を 1 つに固定できる
- 下部プレイヤー / オーバーレイ / 各ページが全て同じ state を見られる
- summary と audio briefing で「どちらが本物の再生面か」がぶれない

### 不採用案

#### モード別に独立 engine を持つ案

summary と audio briefing の state を別々に保持すると、切り替え時の resume や UI 同期が複雑になるため採用しない。

#### desktop 別窓を初回から入れる案

window 間同期、close 後の ownership、popup 制約が増えるため、初回実装では採用しない。

## アーキテクチャ

### 配置

- `web/src/app/(main)/layout.tsx`
  - `SharedAudioPlayerProvider`
  - `SharedAudioMiniPlayer`
  - `SharedAudioOverlay`

各ページは provider 配下で、`useSharedAudioPlayer()` のような hook を通じて再生開始だけを依頼する。

### 責務分離

- Provider
  - 再生状態、queue、prefetch、累積再生時間、expanded 状態を保持
- Shared audio UI
  - ミニプレイヤーと展開オーバーレイを描画
- Page components
  - 再生対象データを取得し、開始 request を送るだけ

### `<audio>` 要素

実際の `<audio>` は provider 内に 1 つだけ置く。
既存の page-local `<audio>` は summary player / audio briefing detail の両方から撤去する。

## 状態モデル

### 共通 state

- `mode: "summary_queue" | "audio_briefing" | null`
- `playback_state: "idle" | "preparing" | "playing" | "paused" | "error" | "finished"`
- `expanded: boolean`
- `current_time`
- `duration`
- `error_message`

### summary queue state

- `queue_kind: "unread" | "later" | "favorite"`
- `queue_window`
  - 最大 24 件
- `queue_visible`
  - 先頭 12 件表示
- `current_item`
- `prefetched_item`
- `prefetch_state`
- `read_progress_seconds`
  - current item の累積再生秒数
- `marked_read_ids`

### audio briefing state

- `job_id`
- `title`
- `summary`
- `audio_url`
- `detail_href`

### 置換ルール

新しい再生開始 request を受けたら、現在再生を stop して state を全置換する。
summary -> audio briefing、audio briefing -> summary のどちらでも後勝ちにする。

## UI 設計

### 下部ミニプレイヤー

常に画面下へ固定表示する。

- 左
  - モード label
  - タイトル
  - summary の場合はソース名
- 中央
  - 再生 / 一時停止
  - 次へ
  - 終了
- 右
  - 残りキュー件数
  - 展開ボタン

状態に応じて `preparing` / `prefetching` / `paused` を表示する。

### 展開プレイヤー

#### summary queue

- 邦題
- 原題
- 要約
- ソース名
- 元記事リンク
- 再生キュー

#### audio briefing

- 回タイトル
- 概要
- 詳細ページリンク
- 必要なら採用記事数などの補助情報

### レスポンシブ

- mobile
  - 下部バー
  - 展開時は全画面シート
- desktop
  - 下部バー
  - 展開時は中央寄せオーバーレイ

## 要約連続再生の扱い

### キュー維持

summary queue は provider に持ち上げる。
そのため Sifto 内のページ遷移では queue を維持できる。

### キューサイズ

全未読を保持しない。
ローカルで持つのは再生用 window 24 件までとし、表示は 12 件までにする。

1 件再生が終わると queue の先頭を落とし、13 件目が自動で繰り上がる。
途中の item を選んだ場合は、その item から先を新しい再生 queue とみなす。

### 既読判定

既読化は即時ではなく、current item の累積再生時間が 30 秒に達した時点で行う。

次の時間はカウントしない。

- preparing 中
- prefetch 待ち
- pause 中
- stop 後

## 音声ブリーフィングの扱い

詳細ページの `<audio controls>` はやめて、共通 player を起動するボタンに置き換える。

再生開始時に provider へ次を渡す。

- `job_id`
- `title`
- `summary`
- `audio_url`
- `detail_href`

展開プレイヤーから detail へ戻れる導線は残す。

## ページ別の変更

### `audio-player/page.tsx`

- 再生 state を provider に移す
- ページ自体は queue の開始画面、または展開プレイヤーへの導線に寄せる
- queue tab と article metadata 表示は shared overlay 側へ寄せる

### `audio-briefings/[id]/page.tsx`

- page-local `<audio>` を廃止
- 再生ボタンは共通 player の起動に置換

### `items/page.tsx`

- `音声` 導線はそのまま使う
- 遷移後に dedicated page で閉じず、共通 player 開始へつながる形に調整する

## エラー処理

- summary synth 失敗
  - ミニプレイヤー / overlay に失敗表示
  - 再試行導線を出す
- prefetch 失敗
  - 現在再生は継続
  - 次 item 開始時に通常 synth へフォールバック
- audio briefing URL 不正
  - 再生開始失敗として扱い、detail page から retry 可能にする

## テスト方針

### Web

- layout をまたいだページ遷移でも再生 state が落ちない
- summary queue の 1 件終了で queue が繰り上がる
- 30 秒累積再生でのみ既読化される
- pause 中と preparing 中は既読カウントされない
- audio briefing 再生開始で summary 再生が置換される
- overlay の summary / audio briefing 表示切替
- mobile / desktop の表示分岐

### 回帰確認

- 既存の要約連続再生の先読みが維持される
- 音声ブリーフィング詳細から再生できる
- 別ページへ移動しても下部プレイヤーが残る

## 段階導入

1. shared provider と下部ミニプレイヤーを追加
2. summary queue state を provider に移す
3. overlay UI を追加
4. audio briefing detail を共通 player 起動へ切り替える
5. 既存専用ページの役割を整理する

## 将来拡張

- desktop 別窓プレイヤー
- reload 復元
- Media Session / lock screen controls
- 再生履歴や last queue snapshot の保存
