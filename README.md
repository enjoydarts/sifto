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
                     │
              [Resend（メール送信）]
```

### サービス構成

| サービス | 技術 | デプロイ先 |
|---|---|---|
| Web フロントエンド | Next.js + Tailwind CSS | Vercel |
| API サーバー | Go + chi ルーター | Fly.io (`sifto-api`) |
| 本文抽出・LLM 処理 | Python FastAPI + trafilatura | Fly.io (`sifto-worker`) |
| データベース | PostgreSQL (Neon) | Neon |
| 非同期ジョブ・cron | Inngest | Inngest Cloud |
| メール送信 | Resend | Resend |
| 認証 | NextAuth.js (JWT) | Vercel |

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
│   │   └── service/              # Resend・Inngest イベント送信
│   ├── go.mod
│   └── Dockerfile
├── worker/                       # Python Worker
│   ├── app/
│   │   ├── main.py
│   │   ├── routers/
│   │   │   ├── extract.py        # 本文抽出エンドポイント
│   │   │   ├── facts.py          # 事実抽出エンドポイント
│   │   │   ├── summarize.py      # 要約エンドポイント
│   │   │   └── compose_digest.py # Digest メール生成エンドポイント
│   │   └── services/
│   │       ├── trafilatura_service.py
│   │       └── claude_service.py
│   ├── requirements.txt
│   └── Dockerfile
├── web/                          # Next.js フロントエンド
│   ├── src/
│   │   ├── app/
│   │   │   ├── (main)/           # 認証後の画面群
│   │   │   │   ├── sources/      # ソース管理
│   │   │   │   ├── items/        # 記事一覧・詳細
│   │   │   │   ├── digests/      # Digest 一覧・詳細
│   │   │   │   └── debug/        # デバッグ用
│   │   │   └── (auth)/login/     # ログイン画面
│   │   └── lib/api.ts            # API クライアント
│   └── Dockerfile
├── db/
│   └── migrations/               # SQL マイグレーション（golang-migrate）
├── docker-compose.yml            # ローカル開発環境
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

全エンドポイントに JWT 認証を適用。

#### Sources
| メソッド | パス | 説明 |
|---|---|---|
| `GET` | `/api/sources` | ソース一覧 |
| `POST` | `/api/sources` | ソース登録 |
| `POST` | `/api/sources/discover` | URL からフィードを自動検出 |
| `PATCH` | `/api/sources/{id}` | 有効/無効の切り替え |
| `DELETE` | `/api/sources/{id}` | ソース削除 |

#### Items
| メソッド | パス | 説明 |
|---|---|---|
| `GET` | `/api/items` | 記事一覧（`status`, `source_id` でフィルタ可） |
| `GET` | `/api/items/{id}` | 記事詳細（facts・summary 含む） |
| `POST` | `/api/items/{id}/retry` | 失敗記事の個別リトライ |
| `POST` | `/api/items/retry-failed` | 失敗記事の一括リトライ |

#### Digests
| メソッド | パス | 説明 |
|---|---|---|
| `GET` | `/api/digests` | Digest 一覧 |
| `GET` | `/api/digests/latest` | 最新 Digest |
| `GET` | `/api/digests/{id}` | Digest 詳細（記事リスト含む） |

#### LLM 使用量
| メソッド | パス | 説明 |
|---|---|---|
| `GET` | `/api/llm-usage` | 使用ログ一覧 |
| `GET` | `/api/llm-usage/summary` | 日次サマリー |

### Python Worker

| メソッド | パス | 入力 | 出力 |
|---|---|---|---|
| `POST` | `/extract-body` | `{url}` | `{title, content, published_at}` |
| `POST` | `/extract-facts` | `{title, content}` | `{facts: string[]}` |
| `POST` | `/summarize` | `{title, facts}` | `{summary, topics, score}` |
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
  id, source_id → sources, url, title, content_text,
  status('new'|'fetched'|'facts_extracted'|'summarized'|'failed'),
  published_at, fetched_at, created_at, updated_at
  UNIQUE(source_id, url)

item_facts
  id, item_id → items (UNIQUE), facts(JSONB), extracted_at

item_summaries
  id, item_id → items (UNIQUE), summary, topics(TEXT[]), score, summarized_at

digests
  id, user_id → users, digest_date, email_subject, email_body,
  sent_at, created_at
  UNIQUE(user_id, digest_date)

digest_items
  id, digest_id → digests, item_id → items, rank
  UNIQUE(digest_id, item_id)

llm_usage_logs
  id, user_id, source_id, item_id, digest_id,
  provider, model, pricing_model_family, pricing_source, purpose('facts'|'summary'|'digest'),
  input_tokens, output_tokens, cache_creation_input_tokens, cache_read_input_tokens,
  estimated_cost_usd, idempotency_key(UNIQUE), created_at
```

## ローカル開発

### 前提条件

- Docker / Docker Compose
- Node.js 22+
- Go 1.23+（API を直接実行する場合）

### セットアップ

```sh
# 1. 環境変数を設定
cp .env.example .env
# .env を編集: ANTHROPIC_API_KEY, NEXTAUTH_SECRET を設定

# 2. バックエンドサービスを起動
docker compose up -d postgres api worker inngest

# 3. フロントエンドを起動（HMR 有効）
cd web && npm install && npm run dev
```

起動後のアクセス先:

| サービス | URL |
|---|---|
| Web | http://localhost:3000 |
| Go API | http://localhost:8080 |
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
migrate -path db/migrations \
  -database "postgres://sifto:sifto@localhost:5432/sifto?sslmode=disable" \
  up
```

### Go コード整形（gofmt）

```sh
# Go API の Go ファイルを整形
make fmt-go

# gofmt 済みかチェック（CI相当）
make fmt-go-check
```

## 環境変数

| 変数名 | 説明 | 必須 |
|---|---|---|
| `DATABASE_URL` | PostgreSQL 接続文字列 | ✓ |
| `PYTHON_WORKER_URL` | Python Worker の URL | ✓ |
| `ANTHROPIC_API_KEY` | Anthropic API キー | ✓ |
| `INNGEST_EVENT_KEY` | Inngest イベントキー | ✓ |
| `INNGEST_SIGNING_KEY` | Inngest 署名キー | ✓ |
| `NEXTAUTH_SECRET` | NextAuth.js 署名シークレット（32 文字以上） | ✓ |
| `NEXTAUTH_URL` | NextAuth.js のベース URL | ✓ |
| `RESEND_API_KEY` | Resend API キー | ✓ |
| `RESEND_FROM_EMAIL` | 送信元メールアドレス | ✓ |
| `GOOGLE_CLIENT_ID` | Google OAuth クライアント ID | — |
| `GOOGLE_CLIENT_SECRET` | Google OAuth クライアントシークレット | — |
| `ANTHROPIC_FACTS_MODEL` | 事実抽出モデル（デフォルト: claude-haiku-4-5） | — |
| `ANTHROPIC_SUMMARY_MODEL` | 要約モデル（デフォルト: claude-sonnet-4-6） | — |
| `ANTHROPIC_DIGEST_MODEL` | Digest 生成モデル（デフォルト: claude-sonnet-4-6） | — |
| `ALLOW_DEV_AUTH_BYPASS` | ローカル開発用認証バイパス | — |
| `INNGEST_DEV` | Inngest Dev Server モード | — |

LLM トークン単価は `ANTHROPIC_*_PER_MTOK_USD` 系の変数で上書き可能（`.env.example` 参照）。

## デプロイ

main ブランチへの push で GitHub Actions が自動実行される:

1. **DB マイグレーション** — Neon に対して `migrate up`
2. **API デプロイ** — `flyctl deploy` (`sifto-api`)
3. **Worker デプロイ** — `flyctl deploy` (`sifto-worker`)
4. **Web デプロイ** — `vercel deploy --prod`

### 必要な GitHub Secrets

| シークレット | 用途 |
|---|---|
| `MIGRATE_DATABASE_URL` | マイグレーション用 DB 接続文字列 |
| `FLY_API_TOKEN` | Fly.io デプロイトークン |
| `VERCEL_TOKEN` | Vercel デプロイトークン |
| `VERCEL_ORG_ID` | Vercel 組織 ID |
| `VERCEL_PROJECT_ID` | Vercel プロジェクト ID |

### Fly.io シークレットの設定

```sh
# Go API に環境変数を設定
cd api
flyctl secrets set \
  DATABASE_URL="..." \
  INNGEST_EVENT_KEY="..." \
  INNGEST_SIGNING_KEY="..." \
  NEXTAUTH_SECRET="..." \
  RESEND_API_KEY="..." \
  RESEND_FROM_EMAIL="digest@yourdomain.com"

# Python Worker に環境変数を設定
cd worker
flyctl secrets set \
  ANTHROPIC_API_KEY="..."
```

## 設計上の判断

**Python マイクロサービス分離**
trafilatura（本文抽出）は Python ライブラリのため、Go API とは独立した FastAPI サービスとして分離。Inngest の step 関数から個別に呼ぶことで、各ステップ単位でリトライが可能。

**Fly.io の採用**
Inngest が 10 分ごとに cron でサービスを叩くため、コールドスタート遅延が問題になる。Fly.io の Machine auto-stop/start は数百 ms で起動するため採用。内部ネットワーク（`.internal`）で API → Worker を低レイテンシで呼び出せる点も利点。

**3 段階処理パイプライン**
本文抽出 → 事実抽出 → 要約の 3 段階に分離することで、LLM 呼び出し失敗時に該当ステップだけリトライできる。事実リストは中間成果物として保存し、UI でも確認可能。

**Neon の接続戦略**
Go API は direct 接続（長時間接続）、Next.js は pooled 接続（サーバレス環境での接続爆発防止）を使い分ける。
