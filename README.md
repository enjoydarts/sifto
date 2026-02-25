# Sifto

登録した RSS フィードと単発 URL を自動収集し、本文抽出・事実抽出・要約を行い、毎朝 Digest メールとして配信するパーソナル情報収集サービス。

## 概要

1. ユーザーが RSS フィードまたは単発 URL をソースとして登録する
2. RSS は 10 分ごとに自動取得し、新規記事を収集する
3. 各記事に対して **本文抽出 → 事実抽出 → 要約・スコアリング** の 3 段階処理を非同期実行する
4. 毎朝 6:00 JST に前日分の記事をスコア順に並べた Digest を生成し、メールで配信する

## アーキテクチャ

```
[Browser] ──→ [Next.js / Vercel]
                     │ JWT
                     ↓
              [Go API / Fly.io] ──→ [Neon PostgreSQL]
                     ↑
         [Inngest Cloud] ──→ [Go API] ──→ [Python Worker / Fly.io]
                                                  │
                                         [Anthropic Claude API]
                                         [OpenAI Embeddings API]
                     │
              [Resend（メール送信）]
```

### サービス構成

| サービス | 技術 | デプロイ先 |
|---|---|---|
| Web フロントエンド | Next.js 16 + React 19 + Tailwind CSS v4 | Vercel |
| API サーバー | Go 1.24 + chi ルーター | Fly.io (`sifto-api`) |
| 本文抽出・LLM 処理 | Python FastAPI + trafilatura | Fly.io (`sifto-worker`) |
| データベース | PostgreSQL (Neon) | Neon |
| 非同期ジョブ・cron | Inngest | Inngest Cloud |
| メール送信 | Resend | Resend |
| 認証 | NextAuth.js (JWT + Google OAuth) | Vercel |

## リポジトリ構成

```
sifto/
├── api/                          # Go API サーバー
│   ├── cmd/server/main.go        # エントリポイント・ルーティング
│   ├── internal/
│   │   ├── handler/              # HTTP ハンドラ
│   │   ├── inngest/              # Inngest 関数定義
│   │   ├── middleware/           # JWT 認証ミドルウェア
│   │   ├── model/                # データモデル
│   │   ├── repository/           # DB アクセス層
│   │   ├── service/              # Worker・Resend・Inngest・暗号化・OpenAI
│   │   └── timeutil/             # タイムゾーンユーティリティ
│   ├── go.mod
│   └── Dockerfile
├── worker/                       # Python Worker
│   ├── app/
│   │   ├── main.py
│   │   ├── routers/
│   │   │   ├── extract.py        # 本文抽出エンドポイント
│   │   │   ├── facts.py          # 事実抽出エンドポイント
│   │   │   ├── summarize.py      # 要約エンドポイント
│   │   │   └── digest.py         # Digest メール生成エンドポイント
│   │   └── services/
│   │       ├── trafilatura_service.py
│   │       └── claude_service.py
│   ├── requirements.txt
│   └── Dockerfile
├── web/                          # Next.js フロントエンド
│   ├── src/
│   │   ├── app/
│   │   │   ├── (main)/           # 認証後の画面群
│   │   │   │   ├── page.tsx      # ダッシュボード
│   │   │   │   ├── sources/      # ソース管理
│   │   │   │   ├── items/        # 記事一覧・詳細
│   │   │   │   ├── digests/      # Digest 一覧・詳細
│   │   │   │   ├── settings/     # ユーザー設定
│   │   │   │   ├── llm-usage/    # LLM 使用量・コスト
│   │   │   │   └── debug/        # デバッグ用
│   │   │   ├── (auth)/login/     # ログイン画面
│   │   │   └── api/              # NextAuth エンドポイント
│   │   ├── components/           # 共通 UI コンポーネント
│   │   └── lib/api.ts            # 型付き API クライアント
│   └── Dockerfile
├── db/
│   └── migrations/               # SQL マイグレーション（golang-migrate）
├── docker-compose.yml            # ローカル開発環境
├── Makefile                      # 開発・CI コマンド
└── .env.example
```

## 処理フロー

### 記事収集・処理パイプライン

```
① ソース登録
   ユーザーが RSS URL またはサイト URL を登録（フィード自動検出あり）

② RSS 定期取得（Inngest cron: */10 * * * *）
   enabled=true の全 RSS ソースをフェッチ
   → 新規 URL のみ items に INSERT（status='new'）
   → item/created イベントを送信

③ 記事処理（Inngest: item/created イベント駆動）
   Step 1 extract-body  : trafilatura で本文抽出  → status='fetched'
   Step 2 extract-facts : Claude で事実リスト生成 → status='facts_extracted'
   Step 3 summarize     : Claude で要約・スコア算出 → status='summarized'

④ Digest 生成（Inngest cron: 0 21 * * * = JST 06:00）
   全ユーザーの前日分 summarized 記事をスコア順にランク付け
   → digests + digest_items に INSERT
   → digest/created イベントを送信

⑤ メール配信（Inngest: digest/created イベント駆動）
   Claude で件名・本文を生成 → Resend で HTML メール送信
   → digests.sent_at を更新
```

### アイテムのステータス遷移

```
new → fetched → facts_extracted → summarized
  ↘         ↘               ↘
                          failed（いずれかのステップで失敗時）
```

## API エンドポイント

### Go API (`/api/*`)

JWT 認証を適用するエンドポイントと、内部用エンドポイントがある。

#### Sources（JWT 認証）

| メソッド | パス | 説明 |
|---|---|---|
| `GET` | `/api/sources` | ソース一覧 |
| `POST` | `/api/sources` | ソース登録 |
| `POST` | `/api/sources/discover` | URL からフィードを自動検出 |
| `PATCH` | `/api/sources/{id}` | 有効/無効の切り替え |
| `DELETE` | `/api/sources/{id}` | ソース削除 |

#### Items（JWT 認証）

| メソッド | パス | 説明 |
|---|---|---|
| `GET` | `/api/items` | 記事一覧（`status`, `source_id` でフィルタ可） |
| `GET` | `/api/items/stats` | ステータス別の記事数統計 |
| `GET` | `/api/items/reading-plan` | パーソナライズされたおすすめ記事リスト |
| `GET` | `/api/items/{id}` | 記事詳細（facts・summary 含む） |
| `GET` | `/api/items/{id}/related` | 類似記事（embedding ベース） |
| `PATCH` | `/api/items/{id}/feedback` | 記事のフィードバック（評価・お気に入り） |
| `POST` | `/api/items/{id}/read` | 既読にする |
| `DELETE` | `/api/items/{id}/read` | 未読に戻す |
| `POST` | `/api/items/{id}/retry` | 失敗記事の個別リトライ |
| `POST` | `/api/items/retry-failed` | 失敗記事の一括リトライ |

#### Digests（JWT 認証）

| メソッド | パス | 説明 |
|---|---|---|
| `GET` | `/api/digests` | Digest 一覧 |
| `GET` | `/api/digests/latest` | 最新 Digest |
| `GET` | `/api/digests/{id}` | Digest 詳細（記事リスト含む） |

#### LLM 使用量（JWT 認証）

| メソッド | パス | 説明 |
|---|---|---|
| `GET` | `/api/llm-usage` | 使用ログ一覧 |
| `GET` | `/api/llm-usage/summary` | 日次コストサマリー |

#### Settings（JWT 認証）

| メソッド | パス | 説明 |
|---|---|---|
| `GET` | `/api/settings` | ユーザー設定の取得 |
| `PATCH` | `/api/settings` | 予算・アラート設定の更新 |
| `PATCH` | `/api/settings/reading-plan` | おすすめ設定の更新 |
| `POST` | `/api/settings/anthropic-key` | Anthropic API キーの設定（暗号化保存） |
| `DELETE` | `/api/settings/anthropic-key` | Anthropic API キーの削除 |
| `POST` | `/api/settings/openai-key` | OpenAI API キーの設定（暗号化保存） |
| `DELETE` | `/api/settings/openai-key` | OpenAI API キーの削除 |

#### 内部エンドポイント（認証なし / X-Internal-Secret で保護）

| メソッド | パス | 説明 |
|---|---|---|
| `POST` | `/api/internal/users/upsert` | NextAuth コールバック用ユーザー作成・更新 |
| `POST` | `/api/internal/debug/digests/generate` | Digest 手動生成 |
| `POST` | `/api/internal/debug/digests/send` | Digest 手動送信 |
| `POST` | `/api/internal/debug/embeddings/backfill` | embedding 一括生成 |

#### その他

| メソッド | パス | 説明 |
|---|---|---|
| `GET` | `/health` | ヘルスチェック（commit SHA 付き） |
| `POST` | `/api/inngest` | Inngest Webhook ハンドラ |

### Python Worker

| メソッド | パス | 入力 | 出力 |
|---|---|---|---|
| `POST` | `/extract-body` | `{url}` | `{title, content, published_at}` |
| `POST` | `/extract-facts` | `{title, content}` | `{facts: string[]}` |
| `POST` | `/summarize` | `{title, facts}` | `{summary, topics, score, score_breakdown, score_reason}` |
| `POST` | `/compose-digest` | `{digest_date, items[]}` | `{subject, body}` |
| `GET` | `/health` | — | `{status: "ok"}` |

## データベーススキーマ

```
users
  id, email, name, email_verified_at, created_at, updated_at

sources
  id, user_id → users, url, type('rss'|'manual'), title,
  enabled, last_fetched_at, created_at, updated_at
  UNIQUE(user_id, url)

items
  id, source_id → sources, url, title, content_text, thumbnail_url,
  status('new'|'fetched'|'facts_extracted'|'summarized'|'failed'),
  published_at, fetched_at, created_at, updated_at
  UNIQUE(source_id, url)

item_facts
  id, item_id → items (UNIQUE), facts(JSONB), extracted_at

item_summaries
  id, item_id → items (UNIQUE), summary, topics(TEXT[]),
  score, score_breakdown(JSONB), score_reason, score_policy_version,
  summarized_at

item_reads
  user_id → users, item_id → items, read_at
  PK(user_id, item_id)

item_embeddings
  item_id → items (PK), model, dimensions, embedding(DOUBLE PRECISION[]),
  created_at, updated_at

item_feedbacks
  user_id → users, item_id → items,
  rating(-1|0|1), is_favorite, updated_at, created_at
  PK(user_id, item_id)

digests
  id, user_id → users, digest_date, email_subject, email_body,
  send_status, send_error, send_tried_at, sent_at, created_at
  UNIQUE(user_id, digest_date)

digest_items
  id, digest_id → digests, item_id → items, rank
  UNIQUE(digest_id, item_id)

llm_usage_logs
  id, user_id, source_id, item_id, digest_id,
  provider, model, pricing_model_family, pricing_source,
  purpose('facts'|'summary'|'digest'|'embedding'),
  input_tokens, output_tokens, cache_creation_input_tokens, cache_read_input_tokens,
  estimated_cost_usd, idempotency_key(UNIQUE), created_at

user_settings
  user_id → users (PK),
  anthropic_api_key_enc, anthropic_api_key_last4,
  openai_api_key_enc, openai_api_key_last4,
  monthly_budget_usd, budget_alert_enabled, budget_alert_threshold_pct,
  reading_plan_window, reading_plan_size,
  reading_plan_diversify_topics, reading_plan_exclude_read,
  digest_email_enabled,
  created_at, updated_at

budget_alert_logs
  id, user_id, month_jst, threshold_pct, budget_usd,
  used_cost_usd, remaining_ratio, sent_at, created_at
```

## ローカル開発

### 前提条件

- Docker / Docker Compose
- Node.js 22+
- Go 1.24+（API を直接実行する場合）

### セットアップ

```sh
# 1. 環境変数を設定
cp .env.example .env
# .env を編集: NEXTAUTH_SECRET, USER_SECRET_ENCRYPTION_KEY を設定

# 2. バックエンドサービスを起動
docker compose up -d postgres api worker inngest

# 3. フロントエンドを起動（HMR 有効）
cd web && npm install && npm run dev
```

起動後のアクセス先:

| サービス | URL |
|---|---|
| Web | http://localhost:3000 |
| Go API | http://localhost:8081 |
| Python Worker | http://localhost:8000 |
| Inngest Dev Server | http://localhost:8288 |

### 認証バイパス（ローカル開発用）

`.env` に以下を設定すると、ログインなしで開発用ユーザーとして動作する:

```env
ALLOW_DEV_AUTH_BYPASS=true
DEV_AUTH_USER_ID=00000000-0000-0000-0000-000000000001
```

### Inngest 関数の手動トリガー

Inngest Dev Server（http://localhost:8288）から各関数を手動実行できる。

- `fetch-rss` — RSS 取得をその場で実行
- `generate-digest` — Digest 生成をその場で実行

### DB マイグレーション

```sh
# ローカル DB にマイグレーション適用
make migrate-up

# 1 つ戻す
make migrate-down

# 現在のバージョン確認
make migrate-version
```

### Make コマンド

```sh
make up            # 全サービス起動（postgres, api, worker, inngest, web）
make up-core       # Web 以外を起動
make down          # 全サービス停止
make build         # api/worker/web イメージをビルド
make logs-api      # API ログを tail
make fmt-go        # Go コード整形
make fmt-go-check  # gofmt チェック（CI 相当）
make check-worker  # Python 構文チェック
make check-fast    # gofmt + worker 構文チェック
make check-web     # ESLint + Next.js ビルド
make check         # PR 前チェック一式
make psql          # ローカル DB に接続
```

## 環境変数

### 必須

| 変数名 | 説明 |
|---|---|
| `DATABASE_URL` | PostgreSQL 接続文字列 |
| `PYTHON_WORKER_URL` | Python Worker の URL |
| `INNGEST_EVENT_KEY` | Inngest イベントキー |
| `INNGEST_SIGNING_KEY` | Inngest 署名キー |
| `NEXTAUTH_SECRET` | NextAuth.js 署名シークレット（32 文字以上） |
| `NEXTAUTH_URL` | NextAuth.js のベース URL |
| `USER_SECRET_ENCRYPTION_KEY` | ユーザー API キー暗号化用シークレット |
| `RESEND_API_KEY` | Resend API キー |
| `RESEND_FROM_EMAIL` | 送信元メールアドレス |
| `NEXT_PUBLIC_API_URL` | フロントエンドから API への URL |

### オプション

| 変数名 | 説明 | デフォルト |
|---|---|---|
| `PORT` | Go API のリッスンポート | `8080` |
| `GOOGLE_CLIENT_ID` | Google OAuth クライアント ID | — |
| `GOOGLE_CLIENT_SECRET` | Google OAuth クライアントシークレット | — |
| `ANTHROPIC_FACTS_MODEL` | 事実抽出モデル | `claude-haiku-4-5` |
| `ANTHROPIC_FACTS_MODEL_FALLBACK` | 事実抽出フォールバックモデル | `claude-3-5-haiku-20241022` |
| `ANTHROPIC_SUMMARY_MODEL` | 要約モデル | `claude-sonnet-4-6` |
| `ANTHROPIC_SUMMARY_MODEL_FALLBACK` | 要約フォールバックモデル | `claude-sonnet-4-5-20250929` |
| `ANTHROPIC_DIGEST_MODEL` | Digest 生成モデル | `claude-sonnet-4-6` |
| `ANTHROPIC_DIGEST_MODEL_FALLBACK` | Digest 生成フォールバックモデル | `claude-sonnet-4-5-20250929` |
| `ALLOW_DEV_AUTH_BYPASS` | ローカル開発用認証バイパス | `false` |
| `ALLOW_DEV_EXTRACT_PLACEHOLDER` | 本文抽出のプレースホルダーモード | `false` |
| `INNGEST_DEV` | Inngest Dev Server モード | `false` |

LLM トークン単価は `ANTHROPIC_*_PER_MTOK_USD` 系の変数で上書き可能（`.env.example` 参照）。

### Docker Compose 用

| 変数名 | 説明 |
|---|---|
| `DOCKER_DATABASE_URL` | コンテナ内 DB 接続文字列 |
| `DOCKER_PYTHON_WORKER_URL` | コンテナ内 Worker URL |
| `DOCKER_INNGEST_BASE_URL` | コンテナ内 Inngest URL |
| `INNGEST_DEV_UPSTREAM_URL` | Inngest → API のコールバック URL |
| `POSTGRES_DB` / `POSTGRES_USER` / `POSTGRES_PASSWORD` | PostgreSQL コンテナ設定 |
| `TZ` | タイムゾーン |

## デプロイ

main ブランチへの push で GitHub Actions が自動実行される:

1. **DB マイグレーション** — Neon に対して `migrate up`
2. **API デプロイ** — `flyctl deploy` (`sifto-api`)
3. **Worker デプロイ** — `flyctl deploy` (`sifto-worker`)

Web（Vercel）は Vercel の GitHub 連携による自動デプロイ。

### 必要な GitHub Secrets

| シークレット | 用途 |
|---|---|
| `MIGRATE_DATABASE_URL` | マイグレーション用 DB 接続文字列 |
| `FLY_API_TOKEN` | Fly.io デプロイトークン |

### Fly.io シークレットの設定

```sh
# Go API に環境変数を設定
cd api
flyctl secrets set \
  DATABASE_URL="..." \
  INNGEST_EVENT_KEY="..." \
  INNGEST_SIGNING_KEY="..." \
  NEXTAUTH_SECRET="..." \
  USER_SECRET_ENCRYPTION_KEY="..." \
  RESEND_API_KEY="..." \
  RESEND_FROM_EMAIL="digest@yourdomain.com"

# Python Worker に環境変数を設定
cd worker
flyctl secrets set \
  ANTHROPIC_FACTS_MODEL="..." \
  ANTHROPIC_SUMMARY_MODEL="..." \
  ANTHROPIC_DIGEST_MODEL="..."
```

Anthropic / OpenAI API キーはサーバー共通ではなく、ユーザーごとに設定画面から登録する（暗号化して `user_settings` に保存）。

## 設計上の判断

**Python マイクロサービス分離**
trafilatura（本文抽出）は Python ライブラリのため、Go API とは独立した FastAPI サービスとして分離。Inngest の step 関数から個別に呼ぶことで、各ステップ単位でリトライが可能。

**Fly.io の採用**
Inngest が 10 分ごとに cron でサービスを叩くため、コールドスタート遅延が問題になる。Fly.io の Machine auto-stop/start は数百 ms で起動するため採用。内部ネットワーク（`.internal`）で API → Worker を低レイテンシで呼び出せる点も利点。

**3 段階処理パイプライン**
本文抽出 → 事実抽出 → 要約の 3 段階に分離することで、LLM 呼び出し失敗時に該当ステップだけリトライできる。事実リストは中間成果物として保存し、UI でも確認可能。

**ユーザー別 API キー管理**
サーバー共通の Anthropic キーではなく、ユーザーごとに自分の API キーを設定する方式。キーは `USER_SECRET_ENCRYPTION_KEY` で暗号化して DB に保存し、Worker 呼び出し時にリクエストヘッダで渡す。

**Neon の接続戦略**
Go API は direct 接続（長時間接続）、Next.js は pooled 接続（サーバレス環境での接続爆発防止）を使い分ける。

**OpenAI Embeddings による関連記事**
記事間の類似度計算には OpenAI の embedding API を使用。ユーザーが OpenAI API キーを設定すると、要約済み記事に対して embedding を生成し、関連記事の検索に利用する。
