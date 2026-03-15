# Sifto

RSS フィードと単発 URL を収集し、本文抽出、事実抽出、要約、品質チェック、Digest 配信まで自動化するパーソナル情報収集サービスです。ブリーフィング、クイックトリアージ、インラインリーダー、トピックパルス、AI Ask、復習導線、LLM 使用量可視化などで、日々のインプット整理を支援します。

## 現在の主な機能

- RSS / 単発 URL の登録と自動収集
- OPML インポート / エクスポート
- Inoreader 連携による購読フィード取り込み
- 記事ごとの本文抽出、事実抽出、事実チェック、要約、要約忠実性チェック
- ブリーフィング画面でのハイライト、Today Queue、クラスタ、リーディングストリーク表示
- 読書ゴール管理
- クイックトリアージと「あとで読む」管理
- インラインリーダーでの要約 / 事実 / 原文確認
- 記事メモ、ハイライト、お気に入り Markdown / Obsidian エクスポート
- AI Ask による記事ベースの質問応答
- Ask Insight 保存、再訪キュー、週次レビュー
- トピックパルスとトピッククラスタ表示
- ソース健全性と source optimization 提案
- 通知優先度ルール調整
- LLM 使用量、用途別コスト、モデル別使用量、value metrics の可視化
- OneSignal による Push 通知
- Obsidian GitHub エクスポート
- 用途別 LLM モデル選択と provider model updates の確認

## アーキテクチャ

```text
[Browser / PWA]
      |
      v
[Next.js 16 / React 19 / Tailwind v4]
      |
      v
[Go API / chi]
  |        \
  |         \--> [Redis]
  v
[PostgreSQL]
  ^
  |
[Inngest]
  |
  v
[Python Worker / FastAPI]
      |
      +--> Anthropic
      +--> Google Gemini
      +--> Groq
      +--> DeepSeek
      +--> Alibaba (Qwen)
      +--> Mistral
      +--> xAI (Grok)
      +--> OpenAI

Other integrations:
- Clerk
- Resend
- OneSignal
- Langfuse
- GitHub App
- Sentry
```

## 技術スタック

| レイヤ | 実装 |
|---|---|
| Web | Next.js 16, React 19, TypeScript, Tailwind CSS v4 |
| API | Go 1.24, chi, pgx, Inngest SDK |
| Worker | Python, FastAPI, trafilatura |
| DB | PostgreSQL |
| Cache | Redis |
| Job orchestration | Inngest |
| Auth | Clerk |
| Mail | Resend |
| Push | OneSignal |
| LLM observability | Langfuse |
| Error tracking | Sentry |

## リポジトリ構成

```text
sifto/
├── api/                # Go API
├── worker/             # Python worker
├── web/                # Next.js frontend
├── db/migrations/      # SQL migrations
├── shared/             # API / worker 共有定義
├── docker-compose.yml  # ローカル開発環境
├── Makefile            # 開発用コマンド
└── AGENTS.md           # リポジトリ固有ルール
```

補足:

- [shared/llm_catalog.json](/Users/minoru-kitayama/private/sifto/shared/llm_catalog.json) で利用可能モデルと既定モデルを管理します。
- [api/cmd/server/main.go](/Users/minoru-kitayama/private/sifto/api/cmd/server/main.go) に API ルーティングがまとまっています。
- [api/internal/inngest/functions.go](/Users/minoru-kitayama/private/sifto/api/internal/inngest/functions.go) と周辺ファイルに定期ジョブとイベント処理があります。

## 画面とユースケース

- ブリーフィング: 直近の注目記事、Today Queue、クラスタ、ストリーク、モデル更新通知を表示
- Items: 記事一覧、フィルタ、既読管理、関連記事表示
- Triage: フォーカスキューを中心に高速に仕分け
- Pulse: トピックの時系列推移を可視化
- Clusters: 関連記事をトピック単位で確認
- Digests: 生成済み Digest の一覧と詳細を確認
- Favorites: お気に入り記事の一覧、保存 Insight、Markdown エクスポート
- Sources: ソース管理、健全性、source optimization、AI 推薦、OPML、Inoreader 取り込み
- Ask: 記事内容に基づく質問応答と Insight 保存
- Settings: API キー、モデル選択、予算、通知優先度、読書ゴール、Obsidian、Inoreader 連携
- LLM Usage: 日次 / プロバイダ別 / モデル別 / 用途別コストと value metrics 確認

## LLM / モデル運用

現行実装では 8 provider を扱います。

- Anthropic
- Google
- Groq
- DeepSeek
- Alibaba
- Mistral
- xAI
- OpenAI

ポイント:

- ユーザーごとに API キーを保存します。サーバー共通キー前提ではありません。
- Alibaba (Qwen) は現在 Virginia の Global endpoint を前提にしています。Singapore / International 用ではなく、Virginia 側で発行した API key を使ってください。
- 用途別にモデルを選択できます。
  - facts
  - summary
  - digest cluster draft
  - digest
  - ask
  - source suggestion
  - facts check
  - faithfulness check
  - embedding
- モデル定義は [shared/llm_catalog.json](/Users/minoru-kitayama/private/sifto/shared/llm_catalog.json) を API / Worker で共有します。
- Settings 画面で recent provider model updates を確認できます。

## バックグラウンド処理

主要な Inngest ジョブ:

| ID | トリガー | 役割 |
|---|---|---|
| `fetch-rss` | `*/10 * * * *` | RSS を定期取得して新規記事を登録 |
| `process-item` | `item/created` | 本文抽出、事実抽出、チェック、要約、通知まで実行 |
| `embed-item` | `item/embed` | 埋め込み生成 |
| `generate-digest` | `0 21 * * *` | JST 06:00 向け Digest 作成 |
| `compose-digest-copy` | `digest/created` | Digest 件名・本文・クラスタドラフト生成 |
| `send-digest` | `digest/copy-composed` | Resend で配信 |
| `generate-briefing-snapshots` | `*/30 * * * *` | ブリーフィング用スナップショット生成 |
| `compute-topic-pulse-daily` | `10 * * * *` | topic pulse 集計更新 |
| `compute-preference-profiles` | `0 20 * * *` | 最近の読了 / フィードバックから嗜好プロファイル更新 |
| `export-obsidian-favorites` | `0 * * * *` | お気に入り記事を Obsidian 向けにエクスポート |
| `track-provider-model-updates` | `0 */6 * * *` | provider のモデル差分を検出 |
| `check-budget-alerts` | `0 0 * * *` | 月次予算アラート判定 |

記事処理の大まかな流れ:

1. ソースを登録する
2. RSS を定期取得して `items` に追加する
3. Worker で本文抽出、事実抽出、事実チェック、要約、忠実性チェックを行う
4. 必要に応じて embedding を生成する
5. スコア条件を満たす記事は Push 通知対象になる
6. 日次で Digest と briefing snapshot を生成する

## API の概要

認証付き API は [api/cmd/server/main.go](/Users/minoru-kitayama/private/sifto/api/cmd/server/main.go) に定義されています。主なグループは以下です。

- `/api/sources`
- `/api/items`
- `/api/topics`
- `/api/ask`
- `/api/digests`
- `/api/llm-usage`
- `/api/provider-model-updates`
- `/api/briefing/today`
- `/api/dashboard`
- `/api/settings`

内部向けエンドポイント:

- `/api/internal/users/*`
- `/api/internal/settings/obsidian-github/installation`
- `/api/internal/debug/*`
- `/api/inngest`

Worker の主なエンドポイント:

- `/extract-body`
- `/extract-facts`
- `/check-facts`
- `/summarize`
- `/check-summary-faithfulness`
- `/translate-title`
- `/ask`
- `/compose-digest`
- `/compose-digest-cluster-draft`
- `/rank-feed-suggestions`
- `/suggest-feed-seed-sites`

## ローカル開発

### 前提

- Docker / Docker Compose
- `migrate` CLI

このリポジトリでは、実行・整形・検証は基本的に `docker compose` / `make` 経由で行います。

### 初期セットアップ

```sh
cp .env.example .env
make up
make migrate-up
```

ローカル起動後の主な URL:

| サービス | URL |
|---|---|
| Web | http://localhost:3000 |
| API | http://localhost:8081 |
| Worker | http://localhost:8000 |
| Inngest Dev Server | http://localhost:8288 |

### よく使うコマンド

```sh
make up
make up-core
make down
make build
make restart
make ps
make logs-api
make logs-worker
make logs-web
make web-lint
make web-build
make fmt-go
make fmt-go-check
make check-worker
make check-fast
make check-web
make check
make migrate-up
make migrate-version
```

補足:

- Web を変更したら最低でも `make web-build` 相当まで確認してください。
- Go 整形は `make fmt-go` を使ってください。
- Worker の構文確認は `make check-worker` です。

## 環境変数

詳細は [.env.example](/Users/minoru-kitayama/private/sifto/.env.example) を参照してください。ここでは重要なものだけ挙げます。

### 開発で最低限必要なもの

| 変数 | 用途 |
|---|---|
| `DATABASE_URL` | ローカル PostgreSQL 接続 |
| `DOCKER_DATABASE_URL` | compose 内 API 用 DB 接続 |
| `PYTHON_WORKER_URL` | API から Worker を呼ぶ URL |
| `DOCKER_PYTHON_WORKER_URL` | compose 内 API から Worker を呼ぶ URL |
| `INTERNAL_WORKER_SECRET` | API -> Worker 認証 |
| `INTERNAL_API_SECRET` | Web internal route -> API 認証 |
| `INNGEST_EVENT_KEY` | Inngest イベントキー |
| `INNGEST_SIGNING_KEY` | Inngest 署名検証キー |
| `USER_SECRET_ENCRYPTION_KEY` | ユーザー API キー暗号化 |
| `NEXT_PUBLIC_API_URL` | ブラウザから見る API ベース URL |
| `NEXT_PUBLIC_CLERK_PUBLISHABLE_KEY` | Clerk |
| `CLERK_SECRET_KEY` | Clerk |
| `CLERK_JWT_ISSUER` | Clerk JWT issuer |
| `CLERK_JWKS_URL` | Clerk JWKS URL |

### 任意だが機能に関わるもの

| 変数 | 用途 |
|---|---|
| `RESEND_API_KEY` / `RESEND_FROM_EMAIL` | Digest メール送信 |
| `ONESIGNAL_APP_ID` / `ONESIGNAL_REST_API_KEY` | Push 通知送信 |
| `NEXT_PUBLIC_ONESIGNAL_APP_ID` | Web Push 初期化 |
| `GITHUB_APP_ID` / `GITHUB_APP_PRIVATE_KEY` / `GITHUB_APP_INSTALL_URL` | Obsidian GitHub エクスポート |
| `GOOGLE_CLIENT_ID` / `GOOGLE_CLIENT_SECRET` | Google OAuth / Inoreader 周辺 |
| `LANGFUSE_SECRET_KEY` / `LANGFUSE_PUBLIC_KEY` / `LANGFUSE_HOST` | LLM observability |
| `SENTRY_DSN` | API / Worker Sentry |
| `NEXT_PUBLIC_SENTRY_DSN` | Web Sentry |

### ローカル認証

`.env.example` では `ALLOW_DEV_AUTH_BYPASS=true` が入っていますが、Clerk 前提の動作確認をする場合は Clerk 関連 env を埋めてください。

## データと集計の考え方

- 日付境界は JST (`Asia/Tokyo`) を基準に扱う箇所があります。
- LLM Usage では日次、モデル別、当月集計の母集団をそろえる実装があります。
- topic pulse と preference profile は定期ジョブで再計算されます。

## 実装上のポイント

- Go API と Python Worker を分離し、本文抽出と LLM 処理を Worker 側へ寄せています。
- 中間成果物として facts、summary、checks、embedding を保持します。
- Redis は API の JSON キャッシュや Worker 側の一部キャッシュに利用します。
- Obsidian エクスポートは GitHub App 経由です。
- OneSignal はアプリ内ページへの導線を前提にしています。

## 検証の目安

変更内容に応じて次を使ってください。

```sh
make check-fast
make check-web
make check
```

フロントエンド変更時は `make web-build`、Worker 変更時は `make check-worker`、Go 変更時は `make fmt-go-check` と関連テストの実行を推奨します。
