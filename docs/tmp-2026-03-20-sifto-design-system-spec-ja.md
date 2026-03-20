# Sifto デザインシステム刷新仕様書

- 日付: 2026-03-20
- 対象: 画面横断のデザインシステム基盤
- 初回適用画面: 記事一覧
- 参照 Stitch 画面:
  - `ダッシュボード: 今日のブリーフィング (建築的精密)`
  - `記事一覧: インテリジェンス・フィード (建築的精密)`
  - `分析 & 設定: AIモデル管理 (建築的精密)`

## 目的

Sifto の Web UI を、編集面のような可読性と運用ツールとしての明快さを両立した共有デザインシステムで刷新する。

最初の適用先は記事一覧とする。その後、同じ土台をダッシュボードと分析・設定画面へ横展開できる状態を目指す。

## 今回の方針

### 採用方針

- トーン: ニュースルーム寄り
- 情報密度: 高め
- テーマ: 明るい紙面ベース
- 基本姿勢: エディトリアルな読みやすさに、運用導線を埋め込む
- 記事一覧の最優先要素: 記事タイトルと要点

### やらないこと

- マーケティング LP のような派手な演出への変更
- ダークテーマ前提の設計
- 3画面を別々の見た目で再設計すること
- 情報量を落として余白や装飾を増やすこと
- 既存の Next.js / Tailwind / flex ベース構成を崩すこと

## デザイン原則

- 装飾よりスキャン速度を優先する
- 階層は色よりも、タイポグラフィ・余白・面の強弱で作る
- 強い色は CTA、focus、semantic state に限定する
- 共有 primitive は少数に絞り、再利用性を優先する
- モバイルの使いやすさと既存レスポンシブ挙動を守る
- 表示ロジックと業務ロジック、i18n を混在させない

## 体験設計

対象画面は、同じプロダクトファミリーとして読める必要がある。

- ダッシュボード: 要約と状況把握
- 記事一覧: 選別とトリアージ
- 分析・設定: 管理と確認

各画面の読み順は、常に次の順序を基本とする。

1. 文脈
2. 状態
3. 判断材料
4. 操作

## 画面アーキテクチャ

### App Shell

- 上部ヘッダーを全画面の共通アンカーとする
- ページ全体のフレーミングは対象画面で揃える
- ナビゲーションは装飾ではなく運用導線としてコンパクトに保つ

### 共通ページ構成

対象画面では、必要に応じて次の順に構成する。

1. `PageHeader`
2. `SummaryStrip`
3. `FilterBar`
4. `PrimaryContent`
5. `SecondaryActions` または補助詳細領域

### Layout Region の責務

shared primitive と page 固有の composition を切り分けるため、layout region を定義する。

- `PageHeader`
  - 入力: page title、説明、主要 action、上位 meta
  - 責務: ページの文脈と最上位操作をまとめる
  - 所有: page container
- `SummaryStrip`
  - 入力: compact metric card の配列
  - 責務: 一覧や本文に入る前の短い状況把握
  - state responsibility: metric の成否判定は page container、表示は region が担当
  - 所有: page container
- `FilterBar`
  - 入力: search、sort、filter、補助 action
  - 責務: 一覧や表の絞り込みと並び替え
  - 所有: page container。ただし見た目は shared primitive
- `PrimaryContent`
  - 入力: ページごとの主要コンテンツ
  - 責務: 記事一覧、ダッシュボード panel 群、設定カード群などの本体表示
  - 所有: page container
- `SecondaryActions`
  - 入力: 補助 action、補助説明、補助 detail
  - 責務: 主目的を阻害しない補助導線の収容
  - 所有: page container
- `DenseArticleList`
  - 入力: item row の配列、loading、empty、error などの page-level state
  - 責務: 記事一覧画面における高密度 row 群の表示
  - state responsibility: list-level state の判定は page container、row 群の描画は region が担当
  - 所有: article list page container。row の見た目自体は shared primitive を利用
- `InlineReaderLayer`
  - 入力: open state、active item、queue item ids、open / close handler
  - 責務: 一覧文脈を保ったまま記事内容を読むための補助閲覧レイヤー
  - state responsibility: open 対象、読了状態同期、queue 管理は page container、見た目と focus trap は layer が担当
  - 所有: article list page container。`ListRowCard` とは別 region / layer として扱う

### 記事一覧の基本構成

記事一覧は、今回のデザインシステムの基準実装とする。

1. `PageHeader`
   - ページタイトル
   - 短い説明文
   - 必要なら主要アクション
   - 件数や同期状況などの軽い補助メタ
2. `SummaryStrip`
   - `今日の流入`、`未読`、`要確認` などの短い指標カード
   - 主役ではなく、一覧前の文脈提示に限定する
3. `FilterBar`
   - 検索
   - ソート
   - feed / status 系フィルタ
   - 長い一覧でも使いやすい sticky 挙動
4. `DenseArticleList`
   - 高密度な 1 カラムリスト
   - 行構造は固定
   - ランクや推薦が必要な場合のみ featured variant を許可

### Phase 1 に含める既存 UI の対応表

記事一覧の初回導入で、既存画面の主要機能を落とさないことを明示する。

- `FeedTabs`
  - 新構造での配置先: `FilterBar.leading`
  - 役割: feed mode の主要切替
- `FiltersBar`
  - 新構造での配置先: `FilterBar.filters` と `FilterBar.sort`
  - 役割: topic、favorite、sort などの絞り込み
- `検索ボタン / 検索モーダル`
  - 新構造での配置先: `FilterBar.actions`
  - 役割: 高密度一覧を崩さず検索導線を持つ
- `bulk action`
  - 新構造での配置先: `FilterBar.actions`
  - 役割: 一括既読などの運用操作
- `filter badge 群`
  - 新構造での配置先: `FilterBar` 下部の active filter summary
  - 役割: 適用中条件の可視化と解除導線
- `date grouping`
  - 新構造での配置先: `DenseArticleList` 内の section header
  - 役割: 日付単位のスキャン補助
- `Pagination`
  - 新構造での配置先: `DenseArticleList` 下部
  - 役割: 長い一覧の移動
- `InlineReader`
  - 新構造での配置先: page-level overlay layer
  - 役割: 一覧文脈を維持したまま内容を読む

これらは Phase 1 の再配置対象であり、削除対象ではない。

## ビジュアルシステム

### 全体方向

見た目は、現代的なニュースルームの作業画面に寄せる。

- 少し温度のある明るい背景
- 静かな panel surface
- 細い境界線
- 控えめな影
- 強い見出し階層
- 限定的なアクセント色

柔らかさや遊びではなく、精密さと落ち着きを優先する。

### カラー設計

中立色を主軸にし、意味のある色だけを限定的に使う。

- `background`: 紙面を思わせるオフホワイト
- `panel`: 主コンテンツ面
- `panel-muted`: 補助面
- `border-subtle`: 薄い区切り線
- `text-strong`: 本文・主要見出し用
- `text-muted`: 補助情報用
- `accent`: 主要アクションと focus 用
- `success`, `warning`, `error`, `info`: semantic state 専用

ルール:

- 階層づけを色に頼らない
- semantic color を装飾に使わない
- 既読 / 未読や状態表現は色だけで分けない

### タイポグラフィ

- フォントファミリーは 1 系統の sans を中心に使う
- ページ見出しはダッシュボード風ではなく、編集面のように締まった見え方にする
- メタ情報と utility label は小さく、規則正しく揃える
- 記事タイトルは一覧の主役として最も強く見せる

役割定義:

- `display / page title`
- `section title`
- `article title`
- `body / supporting copy`
- `meta / utility label`

### 余白

- 小さく繰り返せる spacing scale を使う
- 高密度面の内側余白は詰める
- ページ外周はやや余裕を持たせる
- header、summary、filter、list の縦リズムを統一する

### Surface

- radius は控えめに統一する
- 面の分離は border を主役にする
- shadow は card では薄く、popover や overlay でのみ少し強める
- glassmorphism や強い blur は使わない

### アイコン

- 線の細いシンプルなアイコンを使う
- サイズは小さめを基本とする
- 意味を補強する場合のみ使う
- 高密度リストでは装飾アイコンを増やさない

### モーション

- 150-200ms の fade / lift を基本とする
- 一覧表示時の軽い stagger は許容する
- hover は主役にせず、操作可能性の補助に留める
- reduced motion を阻害しない

## 共有 UI Primitive

導入するデザインシステムは、少数の shared primitive で構成する。

- `PageHeader`
- `SectionCard`
- `SummaryMetricCard`
- `FilterBar`
- `ListRowCard`
- `StatusPill`
- `ScoreBadge`
- `Tag`
- `ActionRow`
- `EmptyState`
- `SkeletonList`
- `ErrorState`
- `InlineReaderLayer`

### 各 Primitive の責務

- `PageHeader`: タイトル、説明、主要アクション、上位メタ情報
- `SectionCard`: 共通 surface。header / body / footer slot を持つ
- `SummaryMetricCard`: 高信号な数値や状態の短い表示
- `FilterBar`: 検索、ソート、状態切替、補助操作
- `ListRowCard`: 高密度行の標準構造
- `StatusPill`: 状態表示を row layout から分離して扱う
- `ScoreBadge`: スコアや優先度の圧縮表現
- `Tag`: topic や補助分類の軽量ラベル表現
- `ActionRow`: 項目ごとの繰り返し操作の見た目を揃える

### Primitive の最小契約

- `SectionCard`
  - slots: `header`, `body`, `footer`
  - states: default
  - ownership: shared primitive
- `PageHeader`
  - inputs: title、description、primary actions、optional meta
  - slots: `meta`, `actions`
  - states: default、compact
  - ownership: shared primitive。配置順や表示有無の決定は page container が持つ
- `SummaryMetricCard`
  - inputs: label、value、optional delta / hint
  - states: default、muted
  - ownership: shared primitive
- `FilterBar`
  - slots: `leading`, `filters`, `sort`, `actions`
  - states: default、sticky
  - ownership: shared primitive。どの filter を表示するかは page container が持つ
- `ListRowCard`
  - slots: `media`, `main`, `meta`, `status`, `score`, `actions`
  - states: default、read、unread、featured、pending、failed
  - ownership: shared primitive
  - interaction responsibility: row click の見た目と focus ring は primitive、遷移先と action handler は page container
- `StatusPill`
  - inputs: semantic kind、label、optional icon
  - states: info、success、warning、error、neutral
  - ownership: shared primitive
- `ScoreBadge`
  - inputs: score、kind、optional reason
  - states: compact、emphasized
  - ownership: shared primitive
- `Tag`
  - inputs: label、optional removable、optional tone
  - states: neutral、accent、semantic-subtle
  - ownership: shared primitive
- `ActionRow`
  - inputs: primary actions、optional secondary actions
  - states: desktop、mobile-wrap
  - ownership: shared primitive
- `EmptyState`
  - inputs: title、description、optional action
  - states: no-data、no-results
  - ownership: shared primitive
- `SkeletonList`
  - inputs: row count、variant
  - states: initial-load、incremental-load
  - ownership: shared primitive
- `ErrorState`
  - inputs: title、description、retry action
  - states: page-error、inline-error
  - ownership: shared primitive
- `InlineReaderLayer`
  - inputs: active item、queue item ids、open、onClose、onOpenDetail、onReadToggled
  - states: closed、open、loading、error
  - ownership: page-scoped composite。記事一覧専用 layer として Phase 1 で扱う

## 記事一覧の行設計

### 行の基本順序

すべての行で、視線移動を次の順に揃える。

1. Thumbnail
2. Title / key point
3. Source / date / state
4. Score / reaction
5. Primary actions

### 密度ルール

- 通常行はおおむね 2 層以内に収める
- 補助メタ情報は折りたたむか、明確に格下げする
- URL などの補助情報はタイトルより強く見せない
- 行ごとのレイアウト変化は最小限にする

### タイトルと要点

- 記事タイトルを最優先で見せる
- 要点や短い補足文がある場合はタイトル直下または直近に置く
- タイトルとメタ情報の階層差を明確にする

### 状態と既読管理

- 未読はウェイト、コントラスト、ラベルで強く見せる
- 既読は少し沈めるが、読めなくしない
- 失敗や処理待ちはコンパクトな semantic pill と短い補足で表現する
- check result や quality badge は可視性を保ちつつ脇役にする

### 操作

- 高頻度操作だけを表に出す
- 低頻度操作は将来的に overflow へ逃がせる形を保つ
- モバイルでは横幅を守るため、操作列を圧縮しても良い

### Featured Variant

推薦やランキング用に、強い variant を 1 種だけ許可する。

条件:

- 基本構造は通常行と同じ
- surface の強調だけで差を付ける
- 別言語のような見え方にしない
- 一覧全体のリズムを壊さない

## レスポンシブ方針

### Desktop

- 高密度な 1 カラムリストを維持する
- summary card は一覧の上に置き、横に並べない
- 行内では thumbnail、テキスト、状態、操作を横組みで処理する

### Mobile

- 固定幅前提にしない
- thumbnail は小さめにする
- メタ情報や操作は自然に wrap させる
- 既存の flex 前提挙動を壊す grid 化は避ける

## アクセシビリティ

- 状態は色だけに依存しない
- キーボードで row 遷移と主要アクションに到達できる
- 明るい surface 上でも focus が十分見える
- 長時間閲覧に耐えるコントラストを確保する
- モーションは控えめで reduced motion と両立する

## 技術方針

### スタイリング戦略

- 既存の Next.js App Router と Tailwind ベース構成を維持する
- `web/src/app/globals.css` の token 層を拡張する
- CSS custom properties と `@theme inline` を共通ソースにする
- utility-first と flex-first の構成を崩さない

### トークンカテゴリ

少なくとも次を定義する。

- page / surface color
- text tier
- semantic state color
- radius
- shadow level
- spacing scale
- typography role
- motion timing

### コンポーネント境界

デザインシステムと業務ロジックを混ぜない。

- visual primitive は presentation だけを持つ
- page container は composition を持つ
- status 判定、API data formatting、i18n は外側に残す

## 展開順序

### Phase 1

記事一覧へ先行適用する。

対象 route:

- `/items`

対象 control:

- page header
- summary strip
- feed tabs
- filters bar
- search modal trigger
- bulk action toolbar
- active filter badges
- date grouping header
- item row card
- pagination
- inline reader の見た目境界

非対象:

- item detail 画面そのものの再設計
- triage 画面の再設計
- API や query parameter の変更

### Phase 2

同じ primitive をダッシュボードへ広げる。

対象 route:

- `/`

対象:

- `PageHeader` の統一
- `SectionCard` による briefing panel の surface 統一
- `SummaryMetricCard` による上位指標の見せ方統一
- status / score 系 badge の見え方統一

非対象:

- briefing の機能追加
- recommendation logic の変更
- briefing data source や selection rule の変更

受け入れ条件:

- `/` の page header が共通 token / primitive に揃っている
- 主要 briefing panel の surface、見出し、余白、badge 表現が共通ルールに揃っている
- loading / empty / error の見え方が article list と同じ設計言語になっている
- 既存の briefing ロジックと表示対象は変わっていない

### Phase 3

同じ primitive を分析・設定画面へ広げる。

対象 route:

- `/settings`

対象 control:

- page header
- provider / API key 系 card
- model guide / model select 周辺 panel
- provider model updates panel
- settings metric card
- modal / inline helper の surface と status 表現

対象:

- `PageHeader` の統一
- `SectionCard` による panel 群の共通 surface 化
- `FilterBar` や table 周辺 control の見た目統一
- status / metric 表現の統一

非対象:

- provider / model 設定フローの変更
- 新規 analytics 機能の追加
- `/llm-usage`, `/llm-analysis`, `/openrouter-models` の再設計

受け入れ条件:

- `/settings` の主要 card / panel が共通 surface と見出しルールに揃っている
- helper modal、inline helper、status / metric 表現が共通 token に揃っている
- field error、save 中、save error の見え方が card / modal 単位で統一されている
- settings の入力規則、API、保存フロー自体は変わっていない

## リスクと制約

- 圧縮しすぎると一覧の視認性が落ちる
- component variant を増やしすぎると一貫性が崩れる
- flex を rigid な grid に置き換えるとモバイル崩れを起こしやすい
- state chip や badge に色を使いすぎるとノイズになる
- card のリファクタで presentation と logic が混ざる恐れがある
- 文言追加は必ず i18n 辞書経由にする

## エラーハンドリングと境界条件

- pending / failed / partially processed な item も同じ行構造で読めること
- 長いタイトルは予測可能に clamp され、メタや操作を壊さないこと
- thumbnail 欠損時は一貫した fallback surface を使うこと
- score や reaction がない場合も行揃えが崩れないこと
- フィルタが増えても狭い幅で読めること

### Page-level State Rules

- `initial loading`
  - 一覧全体の初回取得中は `SkeletonList` を使う
  - summary strip も必要なら skeleton 化するが、page layout は維持する
- `incremental loading`
  - フィルタ変更や再取得時は既存一覧を残したまま局所 loading を出す
  - 画面全体の骨格を揺らさない
- `empty`
  - データが 0 件で、フィルタ条件も通常状態なら `EmptyState(no-data)` を使う
  - 次の行動が分かる短い説明を置く
- `no results`
  - データ自体は存在するがフィルタ条件に一致しない場合は `EmptyState(no-results)` を使う
  - filter reset 導線を出す
- `page error`
  - 一覧全体の取得失敗は `ErrorState(page-error)` を使う
  - retry action を必ず付ける
- `permission denied`
  - 権限不足や認証切れで一覧が取得できない場合は `ErrorState(page-error)` を使う
  - 再試行だけでなく、再認証または権限不足を示す説明を出す
- `offline / network failure`
  - 一時的な通信失敗は page-level error として扱う
  - retry action を付け、直前まで表示できていた一覧がある場合はそれを即座に消さない
- `rate limited`
  - rate limit は generic error に潰さず、少し待って再試行する旨を出す
- `unsupported / stale query`
  - 既存 query param が無効な場合は安全な既定値へ戻すか、filter reset 導線を出す
- `pagination failure`
  - ページ切替失敗時は現在ページの一覧を保持したまま error feedback を出す
- `inline error`
  - item 単位の処理失敗は行内で `StatusPill(error)` と短い補足を出す
  - page 全体の表示は壊さない
- `partial data`
  - thumbnail や score など一部データ欠損は row fallback で吸収し、page error にしない

### Mutation UX Rules

- `retry`
  - retry 実行中は対象 row の retry action を disabled にする
  - 成功時は success feedback を出し、一覧再取得で最新状態へ揃える
  - 失敗時は row を消さず、error feedback を出す
- `mark read / unread`
  - optimistic update を許可する
  - 更新中は対象 row の read action を disabled にする
  - 失敗時は一覧データを server state に戻す
  - unread-only 系一覧では、既読化により row が消える可能性を許容する
- `bulk mark read`
  - confirm を必須にする
  - 実行中は toolbar action を disabled にする
  - 成功時は件数 feedback を出し、一覧と summary を再同期する
  - 失敗時は既存一覧を維持し、error feedback を出す
- `inline reader read toggle`
  - inline reader 内で既読変更した場合も一覧側と状態同期する
  - 現在のフィルタ条件により row が消える場合があることを許容する
  - focus は reader close 後に失われないよう、元の row か近い位置へ戻す
- `search modal`
  - open 中は検索入力へ初期 focus を移す
  - close 時は trigger に focus を戻す
  - submit 失敗時はモーダルを維持し、再入力可能にする
- `row click target`
  - row 全体を primary navigation として扱う場合でも、内部 button の操作を阻害しない
  - action button の focus order は row 内で予測可能に保つ
  - inline reader close 後は元 row か近い要素へ focus を戻す

### Region / Primitive Boundary Notes

- `SummaryStrip`
  - page container が metric データと表示条件を決める
  - region は並び方と surface だけを担当する
- `DenseArticleList`
  - page container が section grouping、pagination、empty/error/loading の分岐を決める
  - region は section header と row 配列の表示を担当する
- `ListRowCard`
  - primitive は row 内レイアウト、hover、focus、状態面の見せ方を担当する
  - page container は click 遷移、button action、disabled 条件、toast や再取得を担当する
- `InlineReaderLayer`
  - page container は open / close、対象 item、queue、detail 遷移、read 同期を担当する
  - layer は overlay 表示、focus trap、close affordance、loading / error surface を担当する

### InlineReader Contract

- 形式
  - modal ではなく、記事一覧の上に重なる page-level overlay layer として扱う
  - row に埋め込む inline panel ではない
- open trigger
  - `ListRowCard` の primary click で開く
  - detail 導線は別 action として残す
- close behavior
  - close button、overlay 内の明示 action、必要なら escape で閉じる
- focus behavior
  - open 時は reader 内の先頭 focusable 要素へ移す
  - close 時は元の row、またはその row が消えた場合は近傍 row へ戻す
- reuse boundary
  - `ListRowCard` を再利用しない
  - article list page に従属する別レイヤーとして扱う
- state behavior
  - loading 中は reader 専用 loading surface
  - item fetch failure は reader 内 error surface
  - 読了状態変更は一覧側と同期する

### Phase 3 Form Rules

Phase 3 で対象にするのは settings 画面の surface と状態表現であり、フォーム仕様の全面変更ではない。その前提で、最低限の UX ルールだけ定義する。

- field validation
  - 既存の validation rule は変更しない
  - field error は field 直下の compact message として出す
- save action
  - 保存中は対象 form または action button を disabled にする
  - 成功時は page-level toast か inline success feedback を使う
- retry
  - 再試行可能な保存失敗は、同じ card または modal の文脈内で retry できるようにする
- field error / save error
  - 入力エラーは field に近い場所で出す
  - 保存失敗は card 単位または modal 単位で出し、page 全体の error にしない
- out of scope
  - settings form の項目追加、validation ルール変更、送信先 API 変更はこの仕様に含めない

### Later Phase State Rules

- `Phase 2 loading`
  - briefing panel 単位で skeleton または loading surface を出す
  - header と page framing は維持する
- `Phase 2 empty`
  - briefing データがない場合は panel 単位の empty state を使う
- `Phase 2 error`
  - page 全体を壊さず、panel 単位または section 単位の error surface を出す
- `Phase 3 loading`
  - settings card 単位で loading / saving を表現する
- `Phase 3 validation error`
  - field 直下または card 内 summary で出し、page 全体の error にしない
- `Phase 3 save error`
  - card / modal 単位で retry 可能にする

## 検証観点

以下を満たした時点で、実装計画へ進める。

- 記事一覧の構造が具体化されている
- shared primitive の責務が分離されている
- 現在のコードベースに乗る styling 方針になっている
- responsive と accessibility の制約が明文化されている
- 実装計画を 1 本にまとめられるスコープに収まっている

## 実装準備サマリ

この仕様は、共有デザインシステム基盤と、記事一覧を起点にした導入順序に限定している。実装計画は Phase 1 を中心に立て、Phase 2 と Phase 3 は shared primitive の再利用境界を定義するために含める。ダッシュボードの機能追加、データモデル変更、ナビゲーション再編などの別件は含めない。
