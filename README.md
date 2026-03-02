# Sifto

登録した RSS フィードと単発 URL を自動収集し、本文抽出・事実抽出・要約を行い、毎朝 Digest メールとして配信するパーソナル情報収集サービス。ブリーフィング、クイックトリアージ、トピックパルスなどの UX 機能でインプットの効率化を支援する。

## 概要

1. ユーザーが RSS フィードまたは単発 URL をソースとして登録する（OPML インポート・Inoreader 連携にも対応）
2. RSS は 10 分ごとに自動取得し、新規記事を収集する
3. 各記事に対して **本文抽出 → 事実抽出 → 要約・スコアリング** の 3 段階処理を非同期実行する
4. 毎朝 6:00 JST に前日分の記事をスコア順に並べた Digest を生成し、メールで配信する
5. ブリーフィング画面で今日のハイライトとリーディングストリークを確認できる
6. クイックトリアージでスワイプ操作により記事を効率的に仕分けできる
7. トピックパルスでトピックの人気度推移をヒートマップで可視化する

## アーキテクチャ

```
[Browser / PWA] ──→ [Next.js / Vercel]
                          │ JWT
                          ↓
                   [Go API / Fly.io] ──→ [Neon PostgreSQL]
                          │                    ↑
                          ├──→ [Redis（キャッシュ）]
                          ↑
              [Inngest Cloud] ──→ [Go API] ──→ [Python Worker / Fly.io]
                                                       │
                                              [Anthropic Claude API]
                                              [Google AI Studio (Gemini API)]
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
| キャッシュ | Redis | ローカル開発のみ |
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
│   │   ├── service/              # Worker・Resend・暗号化・OpenAI・キャッシュ
│   │   └── timeutil/             # タイムゾーンユーティリティ
│   ├── go.mod
│   └── Dockerfile
├── worker/                       # Python Worker
│   ├── app/
│   │   ├── main.py
│   │   ├── routers/
│   │   │   ├── extract.py        # 本文抽出
│   │   │   ├── facts.py          # 事実抽出
│   │   │   ├── summarize.py      # 要約・スコアリング
│   │   │   ├── translate_title.py # タイトル翻訳
│   │   │   ├── digest.py         # Digest メール・クラスタドラフト生成
│   │   │   ├── feed_suggestions.py      # フィード推薦ランキング
│   │   │   └── feed_seed_suggestions.py # フィードシード提案
│   │   └── services/
│   │       ├── trafilatura_service.py   # 本文抽出エンジン
│   │       ├── claude_service.py        # Anthropic Claude クライアント
│   │       ├── gemini_service.py        # Google Gemini クライアント
│   │       └── model_router.py          # モデル振り分け
│   ├── requirements.txt
│   └── Dockerfile
├── web/                          # Next.js フロントエンド
│   ├── src/
│   │   ├── app/
│   │   │   ├── (main)/           # 認証後の画面群
│   │   │   │   ├── page.tsx      # ブリーフィング（ホーム）
│   │   │   │   ├── triage/       # クイックトリアージ
│   │   │   │   ├── pulse/        # トピックパルス
│   │   │   │   ├── items/        # 記事一覧・詳細
│   │   │   │   ├── sources/      # ソース管理
│   │   │   │   ├── digests/      # Digest 一覧・詳細
│   │   │   │   ├── settings/     # ユーザー設定
│   │   │   │   ├── llm-usage/    # LLM 使用量・コスト
│   │   │   │   └── debug/        # デバッグ用
│   │   │   ├── (auth)/login/     # ログイン画面
│   │   │   └── api/              # NextAuth・デバッグエンドポイント
│   │   ├── components/
│   │   │   ├── nav.tsx           # ナビゲーション（デスクトップ＋モバイルボトムナビ）
│   │   │   ├── inline-reader.tsx # インラインリーダー（スワイプで閉じる）
│   │   │   ├── pagination.tsx    # ページネーション
│   │   │   ├── providers.tsx     # Context プロバイダ統合
│   │   │   ├── toast-provider.tsx    # トースト通知
│   │   │   ├── confirm-provider.tsx  # 確認ダイアログ
│   │   │   ├── i18n-provider.tsx     # 多言語対応プロバイダ
│   │   │   └── pwa-install.tsx       # PWA インストール促進
│   │   ├── i18n/
│   │   │   ├── types.ts              # 型定義
│   │   │   └── dictionaries/         # 翻訳辞書（ja / en）
│   │   └── lib/
│   │       ├── api.ts            # 型付き API クライアント
│   │       └── auth.ts           # 認証ユーティリティ
│   └── Dockerfile
├── db/
│   └── migrations/               # SQL マイグレーション（golang-migrate、34 ファイル）
├── docker-compose.yml            # ローカル開発環境
├── Makefile                      # 開発・CI コマンド
├── AGENTS.md                     # 開発ルール
└── .env.example
```

## 主要機能

### ブリーフィング（ホーム画面）

時間帯に応じた挨拶、今日のハイライト記事、トピッククラスタ、リーディングストリークを表示。ブリーフィングスナップショットは Inngest ジョブで事前生成される。

### クイックトリアージ

未読記事をカード形式で表示し、左右スワイプまたはボタン操作で「ブックマーク」「スキップ」「あとで読む」に仕分ける。フォーカスキュー（おすすめ順）を使用。

### インラインリーダー

記事一覧やトリアージ画面から記事の要約・事実・原文をオーバーレイで表示。下スワイプで閉じる。

### トピックパルス

トピックの人気度推移をヒートマップで可視化。期間（7日 / 14日 / 30日）を切り替え可能。

### リーディングストリーク

日次の読了数を追跡し、連続日数を記録。ブリーフィング画面で進捗バーとともに表示。

### ソース管理

- RSS フィードの登録・自動検出
- OPML インポート / エクスポート
- Inoreader 連携（OAuth）
- AI によるフィード推薦・シード提案
- ソースヘルスモニタリング

### 多言語対応

UI は日本語（ja）と英語（en）に対応。ブラウザの言語設定に応じて自動切替。

## 処理フロー

### 記事収集・処理パイプライン

```
① ソース登録
   ユーザーが RSS URL またはサイト URL を登録（フィード自動検出あり）
   OPML インポートや Inoreader 連携も可能

② RSS 定期取得（Inngest cron: */10 * * * *）
   enabled=true の全 RSS ソースをフェッチ
   → 新規 URL のみ items に INSERT（status='new'）
   → item/created イベントを送信

③ 記事処理（Inngest: item/created イベント駆動）
   Step 1 extract-body  : trafilatura で本文抽出       → status='fetched'
   Step 2 extract-facts : Claude/Gemini で事実リスト生成 → status='facts_extracted'
   Step 3 summarize     : Claude/Gemini で要約・スコア算出 → status='summarized'
   Step 4 embed-item    : OpenAI Embeddings で embedding 生成（キー設定時のみ）

④ Digest 生成（Inngest cron: 0 21 * * * = JST 06:00）
   全ユーザーの前日分 summarized 記事をスコア順にランク付け
   → digests + digest_items に INSERT
   → クラスタドラフトを LLM で生成
   → digest/created イベントを送信

⑤ メール配信（Inngest: digest/created イベント駆動）
   Claude/Gemini で件名・本文を生成 → Resend で HTML メール送信
   → digests.sent_at を更新

⑥ ブリーフィングスナップショット生成（Inngest: 定期）
   当日のハイライト・クラスタ・ストリーク情報を事前計算して保存
```

### アイテムのステータス遷移

```
new → fetched → facts_extracted → summarized
  ↘         ↘               ↘
                          failed（いずれかのステップで失敗時）
```

### Inngest 関数一覧

| 関数名 | トリガー | 説明 |
|---|---|---|
| `fetch-rss` | cron `*/10 * * * *` | 全 RSS ソースをフェッチし新規記事を登録 |
| `process-item` | イベント `item/created` | 3 段階処理（本文抽出→事実抽出→要約） |
| `embed-item` | イベント | OpenAI Embeddings で embedding 生成 |
| `generate-digest` | cron `0 21 * * *` | 前日分の Digest を生成 |
| `generate-digest-cluster-drafts` | イベント `digest/created` | クラスタ要約ドラフトを生成 |
| `compose-digest-copy` | イベント | Digest メール本文を LLM で生成 |
| `send-digest` | イベント | Resend で Digest メールを送信 |
| `generate-briefing-snapshots` | 定期 | ブリーフィングスナップショットを事前生成 |

## API エンドポイント

### Go API (`/api/*`)

JWT 認証を適用するエンドポイントと、内部用エンドポイントがある。

#### Sources（JWT 認証）

| メソッド | パス | 説明 |
|---|---|---|
| `GET` | `/api/sources` | ソース一覧 |
| `POST` | `/api/sources` | ソース登録 |
| `POST` | `/api/sources/discover` | URL からフィードを自動検出 |
| `GET` | `/api/sources/health` | ソースヘルス状況 |
| `GET` | `/api/sources/opml` | OPML エクスポート |
| `POST` | `/api/sources/opml/import` | OPML インポート |
| `POST` | `/api/sources/inoreader/import` | Inoreader からインポート |
| `GET` | `/api/sources/recommended` | おすすめソース |
| `GET` | `/api/sources/suggestions` | AI によるソース提案 |
| `PATCH` | `/api/sources/{id}` | ソース更新 |
| `DELETE` | `/api/sources/{id}` | ソース削除 |

#### Items（JWT 認証）

| メソッド | パス | 説明 |
|---|---|---|
| `GET` | `/api/items` | 記事一覧（`status`, `source_id` 等でフィルタ可） |
| `GET` | `/api/items/stats` | ステータス別の記事数統計 |
| `GET` | `/api/items/ux-metrics` | リーディングストリーク・消化率 |
| `GET` | `/api/items/topic-trends` | トピック人気度推移 |
| `GET` | `/api/items/reading-plan` | パーソナライズされたおすすめ記事リスト |
| `GET` | `/api/items/focus-queue` | トリアージ用フォーカスキュー |
| `GET` | `/api/items/{id}` | 記事詳細（facts・summary 含む） |
| `GET` | `/api/items/{id}/related` | 類似記事（embedding ベース） |
| `PATCH` | `/api/items/{id}/feedback` | 記事のフィードバック（評価・お気に入り） |
| `POST` | `/api/items/{id}/read` | 既読にする |
| `DELETE` | `/api/items/{id}/read` | 未読に戻す |
| `POST` | `/api/items/{id}/later` | あとで読むに追加 |
| `DELETE` | `/api/items/{id}/later` | あとで読むを解除 |
| `POST` | `/api/items/{id}/retry` | 失敗記事の個別リトライ |
| `POST` | `/api/items/retry-failed` | 失敗記事の一括リトライ |
| `DELETE` | `/api/items/{id}` | 記事削除 |

#### Topics（JWT 認証）

| メソッド | パス | 説明 |
|---|---|---|
| `GET` | `/api/topics/pulse` | トピックパルス（ヒートマップ用データ） |

#### Briefing（JWT 認証）

| メソッド | パス | 説明 |
|---|---|---|
| `GET` | `/api/briefing/today` | 今日のブリーフィング |

#### Dashboard（JWT 認証）

| メソッド | パス | 説明 |
|---|---|---|
| `GET` | `/api/dashboard` | ダッシュボードデータ |

#### Digests（JWT 認証）

| メソッド | パス | 説明 |
|---|---|---|
| `GET` | `/api/digests` | Digest 一覧 |
| `GET` | `/api/digests/latest` | 最新 Digest |
| `GET` | `/api/digests/{id}` | Digest 詳細（記事リスト・クラスタドラフト含む） |

#### LLM 使用量（JWT 認証）

| メソッド | パス | 説明 |
|---|---|---|
| `GET` | `/api/llm-usage` | 使用ログ一覧 |
| `GET` | `/api/llm-usage/summary` | 日次コストサマリー |
| `GET` | `/api/llm-usage/by-model` | モデル別使用量 |

#### Settings（JWT 認証）

| メソッド | パス | 説明 |
|---|---|---|
| `GET` | `/api/settings` | ユーザー設定の取得 |
| `PATCH` | `/api/settings` | 予算・アラート設定の更新 |
| `PATCH` | `/api/settings/reading-plan` | おすすめ設定の更新 |
| `PATCH` | `/api/settings/llm-models` | LLM モデル選択の更新 |
| `POST` | `/api/settings/anthropic-key` | Anthropic API キーの設定（暗号化保存） |
| `DELETE` | `/api/settings/anthropic-key` | Anthropic API キーの削除 |
| `POST` | `/api/settings/openai-key` | OpenAI API キーの設定（暗号化保存） |
| `DELETE` | `/api/settings/openai-key` | OpenAI API キーの削除 |
| `POST` | `/api/settings/google-key` | Google API キーの設定（暗号化保存） |
| `DELETE` | `/api/settings/google-key` | Google API キーの削除 |
| `GET` | `/api/settings/inoreader/connect` | Inoreader OAuth 開始 |
| `GET` | `/api/settings/inoreader/callback` | Inoreader OAuth コールバック |
| `DELETE` | `/api/settings/inoreader-oauth` | Inoreader 連携解除 |

#### 内部エンドポイント（X-Internal-Secret で保護）

| メソッド | パス | 説明 |
|---|---|---|
| `POST` | `/api/internal/users/upsert` | NextAuth コールバック用ユーザー作成・更新 |
| `POST` | `/api/internal/debug/digests/generate` | Digest 手動生成 |
| `POST` | `/api/internal/debug/digests/send` | Digest 手動送信 |
| `POST` | `/api/internal/debug/embeddings/backfill` | embedding 一括生成 |
| `POST` | `/api/internal/debug/titles/backfill` | タイトル翻訳の一括バックフィル |
| `GET` | `/api/internal/debug/system-status` | システム診断情報 |

#### その他

| メソッド | パス | 説明 |
|---|---|---|
| `GET` | `/health` | ヘルスチェック（commit SHA 付き） |
| `POST` | `/api/inngest` | Inngest Webhook ハンドラ |

### Python Worker

全エンドポイントは `x-internal-worker-secret` ヘッダで保護。Claude と Gemini の両方に対応し、`model` パラメータで振り分ける。

| メソッド | パス | 説明 |
|---|---|---|
| `POST` | `/extract-body` | trafilatura による本文抽出 |
| `POST` | `/extract-facts` | 事実リスト生成 |
| `POST` | `/summarize` | 要約・スコアリング・タイトル翻訳 |
| `POST` | `/translate-title` | タイトル翻訳（単独） |
| `POST` | `/compose-digest` | Digest メール本文生成 |
| `POST` | `/compose-digest-cluster-draft` | クラスタドラフト要約生成 |
| `POST` | `/rank-feed-suggestions` | フィード候補のランキング |
| `POST` | `/suggest-feed-seed-sites` | フィードシードサイト提案 |
| `GET` | `/health` | ヘルスチェック |

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
  processing_error, published_at, fetched_at, created_at, updated_at
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

item_laters
  user_id → users, item_id → items, created_at, updated_at
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

digest_cluster_drafts
  id, digest_id → digests, cluster_key, cluster_label, rank,
  item_count, topics(TEXT[]), max_score, draft_summary,
  created_at, updated_at
  UNIQUE(digest_id, cluster_key)

briefing_snapshots
  id, user_id → users, briefing_date, status('pending'|'ready'|'stale'),
  payload_json(JSONB), generated_at, created_at, updated_at
  UNIQUE(user_id, briefing_date)

reading_streaks
  id, user_id → users, streak_date, read_count, streak_days,
  is_completed, created_at, updated_at
  UNIQUE(user_id, streak_date)

source_health_snapshots
  source_id → sources (PK), total_items, failed_items, summarized_items,
  failure_rate, last_item_at, last_fetched_at, status, reason,
  checked_at, updated_at

llm_usage_logs
  id, user_id, source_id, item_id, digest_id,
  provider, model, pricing_model_family, pricing_source,
  purpose('facts'|'summary'|'digest'|'embedding'|'source_suggestion'|'digest_cluster_draft'),
  input_tokens, output_tokens, cache_creation_input_tokens, cache_read_input_tokens,
  estimated_cost_usd, idempotency_key(UNIQUE), created_at

user_settings
  user_id → users (PK),
  anthropic_api_key_enc, anthropic_api_key_last4,
  openai_api_key_enc, openai_api_key_last4,
  google_api_key_enc, google_api_key_last4,
  inoreader_access_token_enc, inoreader_refresh_token_enc, inoreader_token_expires_at,
  monthly_budget_usd, budget_alert_enabled, budget_alert_threshold_pct,
  reading_plan_window, reading_plan_size,
  reading_plan_diversify_topics, reading_plan_exclude_read,
  digest_email_enabled,
  anthropic_facts_model, anthropic_summary_model, anthropic_digest_model,
  anthropic_digest_cluster_model, anthropic_source_suggestion_model,
  openai_embedding_model,
  created_at, updated_at

budget_alert_logs
  id, user_id, month_jst, threshold_pct, budget_usd,
  used_cost_usd, remaining_ratio, sent_at, created_at
```

## ローカル開発

### 前提条件

- Docker / Docker Compose
- Node.js 22+（フロントエンドをローカル実行する場合）
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
| Redis | localhost:6379 |

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
- `generate-briefing-snapshots` — ブリーフィングスナップショット生成

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
make restart       # api/worker/web を再作成
make ps            # compose ステータス表示
make logs-api      # API ログを tail
make logs-worker   # Worker ログを tail
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
| `INTERNAL_WORKER_SECRET` | API → Worker 間の認証シークレット |
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
| `BRIEFING_SNAPSHOT_MAX_AGE_SEC` | ブリーフィングスナップショットの有効秒数 | `2700` |
| `ANTHROPIC_FACTS_MODEL` | 事実抽出モデル | `claude-haiku-4-5` |
| `ANTHROPIC_FACTS_MODEL_FALLBACK` | 事実抽出フォールバックモデル | `claude-3-5-haiku-20241022` |
| `ANTHROPIC_SUMMARY_MODEL` | 要約モデル | `claude-sonnet-4-6` |
| `ANTHROPIC_SUMMARY_MODEL_FALLBACK` | 要約フォールバックモデル | `claude-sonnet-4-5-20250929` |
| `ANTHROPIC_DIGEST_MODEL` | Digest 生成モデル | `claude-sonnet-4-6` |
| `ANTHROPIC_DIGEST_MODEL_FALLBACK` | Digest 生成フォールバックモデル | `claude-sonnet-4-5-20250929` |
| `ANTHROPIC_TIMEOUT_SEC` | Anthropic API タイムアウト | `90` |
| `ANTHROPIC_COMPOSE_DIGEST_TIMEOUT_SEC` | Digest 生成タイムアウト | `300` |
| `GEMINI_TIMEOUT_SEC` | Gemini API タイムアウト | `90` |
| `GEMINI_COMPOSE_DIGEST_TIMEOUT_SEC` | Gemini Digest 生成タイムアウト | `240` |
| `PYTHON_WORKER_COMPOSE_DIGEST_TIMEOUT_SEC` | API → Worker Digest タイムアウト | `420` |
| `ALLOW_DEV_AUTH_BYPASS` | ローカル開発用認証バイパス | `false` |
| `ALLOW_DEV_EXTRACT_PLACEHOLDER` | 本文抽出のプレースホルダーモード | `false` |
| `INNGEST_DEV` | Inngest Dev Server モード | `false` |

LLM トークン単価は `ANTHROPIC_*_PER_MTOK_USD` 系の変数で上書き可能（`.env.example` 参照）。

ユーザーは設定画面から用途別の LLM モデルを選択できる（Anthropic Claude / Google Gemini）。

### Docker Compose 用

| 変数名 | 説明 |
|---|---|
| `DOCKER_DATABASE_URL` | コンテナ内 DB 接続文字列 |
| `DOCKER_PYTHON_WORKER_URL` | コンテナ内 Worker URL |
| `DOCKER_INNGEST_BASE_URL` | コンテナ内 Inngest URL |
| `DOCKER_REDIS_URL` | コンテナ内 Redis URL |
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
  PYTHON_WORKER_URL="..." \
  INTERNAL_WORKER_SECRET="..." \
  INNGEST_EVENT_KEY="..." \
  INNGEST_SIGNING_KEY="..." \
  NEXTAUTH_SECRET="..." \
  USER_SECRET_ENCRYPTION_KEY="..." \
  RESEND_API_KEY="..." \
  RESEND_FROM_EMAIL="digest@yourdomain.com"

# Python Worker に環境変数を設定
cd worker
flyctl secrets set \
  INTERNAL_WORKER_SECRET="..." \
  ANTHROPIC_FACTS_MODEL="..." \
  ANTHROPIC_SUMMARY_MODEL="..." \
  ANTHROPIC_DIGEST_MODEL="..."
```

Anthropic / OpenAI / Google API キーはサーバー共通ではなく、ユーザーごとに設定画面から登録する（暗号化して `user_settings` に保存）。

## 設計上の判断

**Python マイクロサービス分離**
trafilatura（本文抽出）は Python ライブラリのため、Go API とは独立した FastAPI サービスとして分離。Inngest の step 関数から個別に呼ぶことで、各ステップ単位でリトライが可能。

**Fly.io の採用**
Inngest が 10 分ごとに cron でサービスを叩くため、コールドスタート遅延が問題になる。Fly.io の Machine auto-stop/start は数百 ms で起動するため採用。内部ネットワーク（`.internal`）で API → Worker を低レイテンシで呼び出せる点も利点。

**3 段階処理パイプライン**
本文抽出 → 事実抽出 → 要約の 3 段階に分離することで、LLM 呼び出し失敗時に該当ステップだけリトライできる。事実リストは中間成果物として保存し、UI でも確認可能。

**マルチ LLM 対応**
Anthropic Claude と Google Gemini の両方に対応。ユーザーが設定画面から用途別（事実抽出・要約・Digest 生成等）に使用するモデルを選択できる。Worker 側で `model_router` がモデル名に基づいて適切なサービスに振り分ける。

**ユーザー別 API キー管理**
サーバー共通の LLM キーではなく、ユーザーごとに自分の API キー（Anthropic / OpenAI / Google）を設定する方式。キーは `USER_SECRET_ENCRYPTION_KEY` で暗号化して DB に保存し、Worker 呼び出し時にリクエストヘッダで渡す。

**Neon の接続戦略**
Go API は direct 接続（長時間接続）、Next.js は pooled 接続（サーバレス環境での接続爆発防止）を使い分ける。

**OpenAI Embeddings による関連記事**
記事間の類似度計算には OpenAI の embedding API を使用。ユーザーが OpenAI API キーを設定すると、要約済み記事に対して embedding を生成し、関連記事の検索に利用する。

**多言語対応（i18n）**
フロントエンドは日本語・英語の 2 言語に対応。翻訳辞書は `web/src/i18n/dictionaries/` で管理し、`I18nProvider` でブラウザの言語設定に応じて自動切替する。API からは言語非依存のキーを返し、フロント側で翻訳する方針。

**PWA 対応**
モバイルでのホーム画面追加を促進するため、PWA インストールコンポーネントを搭載。
