# Sifto

登録した RSS フィードと単発 URL を自動収集し、本文抽出・事実抽出・要約を行い、Digest メールとブリーフィングで消化しやすく整えるパーソナル情報収集サービス。ブリーフィング、クイックトリアージ、インラインリーダー、トピックパルス、AI Ask などの UX 機能でインプットの効率化を支援する。

## 概要

1. ユーザーが RSS フィードまたは単発 URL をソースとして登録する（OPML インポート・Inoreader 連携にも対応）
2. RSS は 10 分ごとに自動取得し、新規記事を収集する
3. 各記事に対して **本文抽出 → 事実抽出 → 事実チェック → 要約・スコアリング → 忠実性チェック** の多段階処理を非同期実行する
4. 毎朝 6:00 JST に前日公開分の記事をもとに Digest を生成し、必要に応じてメール配信する
5. ブリーフィング画面で直近 24 時間のハイライトとリーディングストリークを確認できる
6. クイックトリアージでスワイプ操作により記事を効率的に仕分けできる
7. トピックパルスでトピックの人気度推移をヒートマップで可視化する
8. AI Ask で記事の内容に基づいた質問応答ができる

## アーキテクチャ

```
[Browser / PWA] ──→ [Next.js / Vercel]
                          │ Clerk token
                          ↓
                   [Go API / Fly.io] ──→ [PostgreSQL]
                          │                    ↑
                          ├──→ [Upstash Redis / Redis（キャッシュ）]
                          ↑
              [Inngest Cloud] ──→ [Go API] ──→ [Python Worker / Fly.io]
                                                       │
                                              [Anthropic Claude API]
                                              [Google AI Studio (Gemini API)]
                                              [Groq API]
                                              [DeepSeek API]
                                              [Alibaba (Qwen) API]
                                              [Mistral API]
                                              [OpenAI Embeddings API]
                          │
                   [Resend（メール送信）]
                          │
                   [OneSignal（Push 通知）]
                          │
                   [Langfuse（LLM オブザーバビリティ）]
                          │
                   [GitHub App（Obsidian エクスポート）]
```

### サービス構成

| サービス | 技術 | デプロイ先 |
|---|---|---|
| Web フロントエンド | Next.js 16 + React 19 + Tailwind CSS v4 | Vercel |
| API サーバー | Go 1.24 + chi ルーター | Fly.io (`sifto-api`) |
| 本文抽出・LLM 処理 | Python FastAPI + trafilatura | Fly.io (`sifto-worker`) |
| データベース | PostgreSQL | 本番 DB |
| キャッシュ | Redis / Upstash Redis | ローカル / Upstash |
| 非同期ジョブ・cron | Inngest | Inngest Cloud |
| メール送信 | Resend | Resend |
| Push 通知 | OneSignal | OneSignal |
| 認証 | Clerk (Email / OAuth) | Clerk |
| エラー監視 | Sentry | Vercel / Fly.io |
| LLM オブザーバビリティ | Langfuse | Langfuse |
| Obsidian エクスポート | GitHub App | GitHub |

## リポジトリ構成

```
sifto/
├── api/                          # Go API サーバー
│   ├── cmd/server/main.go        # エントリポイント・ルーティング
│   ├── internal/
│   │   ├── handler/              # HTTP ハンドラ
│   │   ├── inngest/              # Inngest 関数定義
│   │   ├── middleware/           # Clerk Bearer token 認証ミドルウェア
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
│   │   │   ├── facts_check.py    # 事実チェック
│   │   │   ├── summarize.py      # 要約・スコアリング
│   │   │   ├── summary_faithfulness.py # 要約忠実性チェック
│   │   │   ├── translate_title.py # タイトル翻訳
│   │   │   ├── ask.py            # AI Ask（質問応答）
│   │   │   ├── digest.py         # Digest メール・クラスタドラフト生成
│   │   │   ├── feed_suggestions.py      # フィード推薦ランキング
│   │   │   └── feed_seed_suggestions.py # フィードシード提案
│   │   └── services/
│   │       ├── trafilatura_service.py       # 本文抽出エンジン
│   │       ├── anthropic_transport.py       # Anthropic Claude トランスポート
│   │       ├── gemini_transport.py          # Google Gemini トランスポート
│   │       ├── openai_compat_transport.py   # OpenAI 互換トランスポート（Groq / DeepSeek / Alibaba / Mistral）
│   │       ├── openai_responses_transport.py # OpenAI Responses API トランスポート
│   │       ├── openai_service.py            # OpenAI Embeddings
│   │       ├── llm_dispatch.py              # LLM プロバイダディスパッチ
│   │       ├── llm_catalog.py               # LLM カタログ読み込み
│   │       ├── model_router.py              # モデル振り分け
│   │       ├── langfuse_client.py           # Langfuse クライアント
│   │       ├── facts_check_runner.py        # 事実チェック実行
│   │       └── summary_faithfulness_runner.py # 忠実性チェック実行
│   ├── requirements.txt
│   └── Dockerfile
├── web/                          # Next.js フロントエンド
│   ├── src/
│   │   ├── app/
│   │   │   ├── (main)/           # 認証後の画面群
│   │   │   │   ├── page.tsx      # ブリーフィング（ホーム）
│   │   │   │   ├── ask/          # AI Ask
│   │   │   │   ├── triage/       # クイックトリアージ
│   │   │   │   ├── pulse/        # トピックパルス
│   │   │   │   ├── clusters/     # トピッククラスタ
│   │   │   │   ├── items/        # 記事一覧・詳細
│   │   │   │   ├── favorites/    # お気に入り
│   │   │   │   ├── sources/      # ソース管理
│   │   │   │   ├── digests/      # Digest 一覧・詳細
│   │   │   │   ├── settings/     # ユーザー設定
│   │   │   │   ├── llm-usage/    # LLM 使用量・コスト
│   │   │   │   └── debug/        # デバッグ用
│   │   │   ├── (auth)/login/     # ログイン画面
│   │   │   └── api/              # Clerk bridge・デバッグエンドポイント
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
│   │       ├── internal-secret.ts # internal route 用シークレット helper
│   │       └── server-auth.ts    # Clerk server auth helper
│   └── Dockerfile
├── shared/
│   └── llm_catalog.json          # LLM プロバイダ・モデル定義（API / Worker 共有）
├── db/
│   └── migrations/               # SQL マイグレーション（golang-migrate）
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

### AI Ask

記事の内容に基づいた質問応答。選択した LLM モデルで記事コンテンツを参照しながら回答を生成する。

### トピックパルス

トピックの人気度推移をヒートマップで可視化。期間（7日 / 14日 / 30日）を切り替え可能。

### トピッククラスタ

関連する記事をトピック単位でグルーピングして表示。

### リーディングストリーク

日次の読了数を追跡し、連続日数を記録。ブリーフィング画面で進捗バーとともに表示。

### ソース管理

- RSS フィードの登録・自動検出
- OPML インポート / エクスポート
- Inoreader 連携（OAuth）
- AI によるフィード推薦・シード提案
- ソースヘルスモニタリング

### 通知・配信

- Digest メール生成と送信制御
- OneSignal による Push 通知（高スコア記事の自動通知）
- ダイジェスト送信を無効化した生成のみモード
- 月次予算アラート（メール・Push 通知）

### お気に入り・Obsidian エクスポート

- お気に入り記事の一覧表示・Markdown エクスポート
- GitHub App 連携による Obsidian Vault への自動エクスポート（1 時間ごと）

### 多言語対応

UI は日本語（ja）と英語（en）に対応。ブラウザの言語設定に応じて自動切替。

### マルチ LLM プロバイダ

7 つの LLM プロバイダに対応。ユーザーが用途ごとにモデルを選択できる。

| プロバイダ | 主なモデル |
|---|---|
| Anthropic | Claude Haiku 4.5, Claude Sonnet 4.6 |
| Google | Gemini 2.5 Flash, Gemini Flash Lite |
| Groq | Llama, Qwen（OpenAI 互換） |
| DeepSeek | deepseek-chat, deepseek-reasoner |
| Alibaba | Qwen3 Max, Qwen Plus, Qwen Flash |
| Mistral | Mistral Small / Medium / Large, Ministral-8B |
| OpenAI | Embeddings API |

モデル定義は `shared/llm_catalog.json` で一元管理し、API・Worker の両方で参照する。

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
   Step 1 extract-body       : trafilatura で本文抽出           → status='fetched'
   Step 2 extract-facts      : LLM で事実リスト生成             → status='facts_extracted'
   Step 3 check-facts        : LLM で事実チェック（品質検証）
   Step 4 summarize          : LLM で要約・スコア算出           → status='summarized'
   Step 5 check-faithfulness : LLM で要約の忠実性チェック
   Step 6 embed-item         : OpenAI Embeddings で embedding 生成（キー設定時のみ）
   Step 7 push-notification  : 高スコア記事の Push 通知

④ Digest 生成（Inngest cron: 0 21 * * * = JST 06:00）
   全ユーザーの前日公開分 summarized 記事をランク付け
   → digests + digest_items に INSERT
   → digest/created イベントを送信

⑤ メール本文生成（Inngest: digest/created イベント駆動）
   LLM でクラスタドラフト・件名・本文を生成
   → digest/copy-composed イベントを送信

⑥ メール配信（Inngest: digest/copy-composed イベント駆動）
   Resend で HTML メール送信
   → digests.sent_at を更新

⑦ ブリーフィングスナップショット生成（Inngest cron: */30 * * * *）
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
| `process-item` | イベント `item/created` | 多段階処理（本文抽出→事実抽出→チェック→要約→忠実性チェック→Push 通知） |
| `embed-item` | イベント `item/embed` | OpenAI Embeddings で embedding 生成 |
| `generate-digest` | cron `0 21 * * *` | 前日分の Digest を生成 |
| `compose-digest-copy` | イベント `digest/created` | クラスタドラフト・Digest メール本文を LLM で生成 |
| `send-digest` | イベント `digest/copy-composed` | Resend で Digest メールを送信 |
| `generate-briefing-snapshots` | cron `*/30 * * * *` | ブリーフィングスナップショットを事前生成 |
| `export-obsidian-favorites` | cron `0 * * * *` | お気に入り記事を Obsidian（GitHub）にエクスポート |
| `track-provider-model-updates` | cron `0 */6 * * *` | LLM プロバイダのモデル更新を検出・記録 |
| `check-budget-alerts` | cron `0 0 * * *` (JST 09:00) | 月次予算アラートの確認・通知 |

## API エンドポイント

### Go API (`/api/*`)

Clerk Bearer token 認証を適用するエンドポイントと、内部用エンドポイントがある。

#### Sources（Clerk 認証）

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

#### Items（Clerk 認証）

| メソッド | パス | 説明 |
|---|---|---|
| `GET` | `/api/items` | 記事一覧（`status`, `source_id` 等でフィルタ可） |
| `GET` | `/api/items/favorites/export-markdown` | お気に入り記事の Markdown エクスポート |
| `GET` | `/api/items/stats` | ステータス別の記事数統計 |
| `GET` | `/api/items/ux-metrics` | リーディングストリーク・消化率 |
| `GET` | `/api/items/topic-trends` | トピック人気度推移 |
| `GET` | `/api/items/reading-plan` | パーソナライズされたおすすめ記事リスト |
| `GET` | `/api/items/focus-queue` | トリアージ用フォーカスキュー |
| `GET` | `/api/items/triage-all` | 全記事トリアージ |
| `GET` | `/api/items/{id}` | 記事詳細（facts・summary 含む） |
| `GET` | `/api/items/{id}/related` | 類似記事（embedding ベース） |
| `PATCH` | `/api/items/{id}/feedback` | 記事のフィードバック（評価・お気に入り） |
| `POST` | `/api/items/{id}/read` | 既読にする |
| `POST` | `/api/items/mark-read-bulk` | 一括既読 |
| `POST` | `/api/items/mark-later-bulk` | 一括あとで読む |
| `DELETE` | `/api/items/{id}/read` | 未読に戻す |
| `POST` | `/api/items/{id}/later` | あとで読むに追加 |
| `DELETE` | `/api/items/{id}/later` | あとで読むを解除 |
| `POST` | `/api/items/{id}/retry` | 失敗記事の個別リトライ |
| `POST` | `/api/items/{id}/retry-from-facts` | 事実抽出からの再処理リトライ |
| `POST` | `/api/items/retry-failed` | 失敗記事の一括リトライ |
| `DELETE` | `/api/items/{id}` | 記事削除 |

#### Topics（Clerk 認証）

| メソッド | パス | 説明 |
|---|---|---|
| `GET` | `/api/topics/pulse` | トピックパルス（ヒートマップ用データ） |

#### Ask（Clerk 認証）

| メソッド | パス | 説明 |
|---|---|---|
| `POST` | `/api/ask` | 記事に基づく AI 質問応答 |

#### Briefing（Clerk 認証）

| メソッド | パス | 説明 |
|---|---|---|
| `GET` | `/api/briefing/today` | 今日のブリーフィング |

#### Dashboard（Clerk 認証）

| メソッド | パス | 説明 |
|---|---|---|
| `GET` | `/api/dashboard` | ダッシュボードデータ |

#### Digests（Clerk 認証）

| メソッド | パス | 説明 |
|---|---|---|
| `GET` | `/api/digests` | Digest 一覧 |
| `GET` | `/api/digests/latest` | 最新 Digest |
| `GET` | `/api/digests/{id}` | Digest 詳細（記事リスト・クラスタドラフト含む） |

#### LLM 使用量（Clerk 認証）

| メソッド | パス | 説明 |
|---|---|---|
| `GET` | `/api/llm-usage` | 使用ログ一覧 |
| `GET` | `/api/llm-usage/summary` | 日次コストサマリー |
| `GET` | `/api/llm-usage/by-model` | モデル別使用量 |
| `GET` | `/api/llm-usage/current-month/by-provider` | 当月プロバイダ別使用量 |
| `GET` | `/api/llm-usage/current-month/by-purpose` | 当月用途別使用量 |
| `GET` | `/api/llm-usage/current-month/execution-summary` | 当月実行サマリー |

#### Provider Model Updates（Clerk 認証）

| メソッド | パス | 説明 |
|---|---|---|
| `GET` | `/api/provider-model-updates` | LLM プロバイダのモデル更新履歴 |

#### Settings（Clerk 認証）

| メソッド | パス | 説明 |
|---|---|---|
| `GET` | `/api/settings` | ユーザー設定の取得 |
| `GET` | `/api/settings/llm-catalog` | LLM カタログ（利用可能なモデル一覧） |
| `PATCH` | `/api/settings` | 予算・アラート・Digest 送信設定の更新 |
| `PATCH` | `/api/settings/reading-plan` | おすすめ設定の更新 |
| `PATCH` | `/api/settings/llm-models` | LLM モデル選択の更新 |
| `PATCH` | `/api/settings/obsidian-export` | Obsidian エクスポート設定の更新 |
| `POST` | `/api/settings/obsidian-export/run` | Obsidian エクスポートの手動実行 |
| `POST` | `/api/settings/anthropic-key` | Anthropic API キーの設定（暗号化保存） |
| `DELETE` | `/api/settings/anthropic-key` | Anthropic API キーの削除 |
| `POST` | `/api/settings/openai-key` | OpenAI API キーの設定（暗号化保存） |
| `DELETE` | `/api/settings/openai-key` | OpenAI API キーの削除 |
| `POST` | `/api/settings/google-key` | Google API キーの設定（暗号化保存） |
| `DELETE` | `/api/settings/google-key` | Google API キーの削除 |
| `POST` | `/api/settings/groq-key` | Groq API キーの設定（暗号化保存） |
| `DELETE` | `/api/settings/groq-key` | Groq API キーの削除 |
| `POST` | `/api/settings/deepseek-key` | DeepSeek API キーの設定（暗号化保存） |
| `DELETE` | `/api/settings/deepseek-key` | DeepSeek API キーの削除 |
| `POST` | `/api/settings/alibaba-key` | Alibaba API キーの設定（暗号化保存） |
| `DELETE` | `/api/settings/alibaba-key` | Alibaba API キーの削除 |
| `POST` | `/api/settings/mistral-key` | Mistral API キーの設定（暗号化保存） |
| `DELETE` | `/api/settings/mistral-key` | Mistral API キーの削除 |
| `GET` | `/api/settings/inoreader/connect` | Inoreader OAuth 開始 |
| `GET` | `/api/settings/inoreader/callback` | Inoreader OAuth コールバック |
| `DELETE` | `/api/settings/inoreader-oauth` | Inoreader 連携解除 |

#### 内部エンドポイント（X-Internal-Secret で保護）

| メソッド | パス | 説明 |
|---|---|---|
| `POST` | `/api/internal/users/upsert` | ユーザー upsert |
| `POST` | `/api/internal/users/resolve-identity` | Clerk user を internal user_id へ解決・紐付け |
| `POST` | `/api/internal/settings/obsidian-github/installation` | Obsidian GitHub App インストール登録 |
| `POST` | `/api/internal/debug/digests/generate` | Digest 手動生成 |
| `POST` | `/api/internal/debug/digests/send` | Digest 手動送信 |
| `POST` | `/api/internal/debug/embeddings/backfill` | embedding 一括生成 |
| `POST` | `/api/internal/debug/titles/backfill` | タイトル翻訳の一括バックフィル |
| `POST` | `/api/internal/debug/push/test` | Push 通知テスト送信 |
| `GET` | `/api/internal/debug/system-status` | システム診断情報 |

#### その他

| メソッド | パス | 説明 |
|---|---|---|
| `GET` | `/health` | ヘルスチェック（commit SHA 付き） |
| `POST` | `/api/inngest` | Inngest Webhook ハンドラ |

### Python Worker

全エンドポイントは `x-internal-worker-secret` ヘッダで保護。7 つの LLM プロバイダに対応し、リクエストヘッダの API キーとモデル名に基づいてディスパッチする。

| メソッド | パス | 説明 |
|---|---|---|
| `POST` | `/extract-body` | trafilatura による本文抽出 |
| `POST` | `/extract-facts` | 事実リスト生成 |
| `POST` | `/check-facts` | 事実チェック（品質検証） |
| `POST` | `/summarize` | 要約・スコアリング・タイトル翻訳 |
| `POST` | `/check-summary-faithfulness` | 要約の忠実性チェック |
| `POST` | `/translate-title` | タイトル翻訳（単独） |
| `POST` | `/ask` | 記事に基づく質問応答 |
| `POST` | `/compose-digest` | Digest メール本文生成 |
| `POST` | `/compose-digest-cluster-draft` | クラスタドラフト要約生成 |
| `POST` | `/rank-feed-suggestions` | フィード候補のランキング |
| `POST` | `/suggest-feed-seed-sites` | フィードシードサイト提案 |
| `GET` | `/health` | ヘルスチェック |

## データベーススキーマ

```
users
  id, email, name, email_verified_at, created_at, updated_at

user_identities
  id, user_id → users, provider, provider_user_id, email,
  created_at, updated_at
  UNIQUE(provider, provider_user_id)

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

item_export_records
  id, user_id, item_id, target, github_path, github_sha,
  content_hash, status, exported_at, last_error,
  created_at, updated_at

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
  purpose('facts'|'summary'|'digest'|'embedding'|'source_suggestion'|
          'digest_cluster_draft'|'ask'|'facts_check'|'faithfulness_check'),
  input_tokens, output_tokens, cache_creation_input_tokens, cache_read_input_tokens,
  estimated_cost_usd, idempotency_key(UNIQUE), created_at

llm_execution_events
  id, user_id, item_id, digest_id, source_id,
  purpose, provider, model, status, error_message,
  duration_ms, created_at

push_notification_logs
  id, user_id, kind, item_id, external_push_id,
  recipients, created_at

provider_model_updates
  id, provider, detected_at, models_added, models_removed,
  snapshot_json, created_at

obsidian_export_settings
  user_id → users (PK), enabled,
  github_installation_id, github_repo_owner, github_repo_name,
  github_repo_branch, vault_root_path, keyword_link_mode,
  last_run_at, last_success_at, created_at, updated_at

user_settings
  user_id → users (PK),
  anthropic_api_key_enc, anthropic_api_key_last4,
  openai_api_key_enc, openai_api_key_last4,
  google_api_key_enc, google_api_key_last4,
  groq_api_key_enc, groq_api_key_last4,
  deepseek_api_key_enc, deepseek_api_key_last4,
  alibaba_api_key_enc, alibaba_api_key_last4,
  mistral_api_key_enc, mistral_api_key_last4,
  inoreader_access_token_enc, inoreader_refresh_token_enc, inoreader_token_expires_at,
  monthly_budget_usd, budget_alert_enabled, budget_alert_threshold_pct,
  reading_plan_window, reading_plan_size,
  reading_plan_diversify_topics, reading_plan_exclude_read,
  digest_email_enabled,
  facts_model, summary_model, digest_model,
  digest_cluster_model, ask_model, source_suggestion_model,
  embedding_model, facts_check_model, faithfulness_check_model,
  created_at, updated_at

budget_alert_logs
  id, user_id, month_jst, threshold_pct, budget_usd,
  used_cost_usd, remaining_ratio, sent_at, created_at
```

## ローカル開発

### 前提条件

- Docker / Docker Compose
- `migrate` CLI（`make migrate-up` を使う場合）

### セットアップ

```sh
# 1. 環境変数を設定
cp .env.example .env
# .env を編集: Clerk / INTERNAL_API_SECRET / USER_SECRET_ENCRYPTION_KEY を設定

# 2. 全サービスを Docker で起動
make up

# 3. ローカル DB にマイグレーション適用
make migrate-up
```

起動後のアクセス先:

| サービス | URL |
|---|---|
| Web | http://localhost:3000 |
| Go API | http://localhost:8081 |
| Python Worker | http://localhost:8000 |
| Inngest Dev Server | http://localhost:8288 |
| Redis | redis://localhost:6379 |

### 認証（ローカル開発）

ローカルでも Clerk を使う前提。最低でも以下を `.env` に設定する:

```env
NEXT_PUBLIC_CLERK_PUBLISHABLE_KEY=pk_test_xxx
CLERK_SECRET_KEY=sk_test_xxx
CLERK_JWT_ISSUER=https://your-instance.clerk.accounts.dev
CLERK_JWKS_URL=https://your-instance.clerk.accounts.dev/.well-known/jwks.json
INTERNAL_API_SECRET=your-random-internal-secret
USER_SECRET_ENCRYPTION_KEY=your-user-secret-encryption-key
```

Google ログインを使う場合も、Sifto 側の `GOOGLE_CLIENT_ID` / `GOOGLE_CLIENT_SECRET` ではなく Clerk 側の Google provider 設定を使う。

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
| `INTERNAL_API_SECRET` | Web → API internal route 間の認証シークレット |
| `INNGEST_EVENT_KEY` | Inngest イベントキー |
| `INNGEST_SIGNING_KEY` | Inngest 署名キー |
| `NEXT_PUBLIC_CLERK_PUBLISHABLE_KEY` | Clerk publishable key |
| `CLERK_SECRET_KEY` | Clerk secret key |
| `CLERK_JWT_ISSUER` | Clerk JWT issuer |
| `CLERK_JWKS_URL` | Clerk JWKS URL |
| `USER_SECRET_ENCRYPTION_KEY` | ユーザー API キー暗号化用シークレット |
| `NEXT_PUBLIC_API_URL` | フロントエンドから API への URL |

### オプション

| 変数名 | 説明 | デフォルト |
|---|---|---|
| `PORT` | Go API のリッスンポート | `8080` |
| `UPSTASH_REDIS_URL` / `REDIS_URL` | API / Worker が使う Redis 接続文字列 | -- |
| `REDIS_CACHE_PREFIX` | Redis キャッシュの key prefix | `sifto` 相当 |
| `CLERK_JWT_AUDIENCE` | Clerk JWT audience | -- |
| `RESEND_API_KEY` | Resend API キー | -- |
| `RESEND_FROM_EMAIL` | 送信元メールアドレス | -- |
| `ONESIGNAL_APP_ID` | OneSignal App ID | -- |
| `ONESIGNAL_REST_API_KEY` | OneSignal REST API Key | -- |
| `NEXT_PUBLIC_ONESIGNAL_APP_ID` | Web Push 用 OneSignal App ID | -- |
| `ONESIGNAL_PICK_SCORE_THRESHOLD` | 高スコア記事 push の閾値 | `0.90` |
| `ONESIGNAL_PICK_MAX_PER_DAY` | 1 日あたりの push 上限 | `2` |
| `GITHUB_APP_ID` | GitHub App ID（Obsidian エクスポート用） | -- |
| `GITHUB_APP_PRIVATE_KEY` | GitHub App 秘密鍵 | -- |
| `GITHUB_APP_INSTALL_URL` | GitHub App インストール URL | -- |
| `LANGFUSE_SECRET_KEY` | Langfuse シークレットキー | -- |
| `LANGFUSE_PUBLIC_KEY` | Langfuse パブリックキー | -- |
| `LANGFUSE_HOST` | Langfuse ホスト URL | -- |
| `SENTRY_DSN` | API / Worker 用 Sentry DSN | -- |
| `NEXT_PUBLIC_SENTRY_DSN` | Web 用 Sentry DSN | -- |
| `SENTRY_ENVIRONMENT` | Sentry environment | -- |
| `SENTRY_TRACES_SAMPLE_RATE` | サーバー側 tracing rate | `0` |
| `NEXT_PUBLIC_SENTRY_TRACES_SAMPLE_RATE` | Web tracing rate | `0` |
| `NEXT_PUBLIC_SENTRY_REPLAYS_SESSION_SAMPLE_RATE` | Web replay rate | `0` |
| `NEXT_PUBLIC_SENTRY_REPLAYS_ON_ERROR_SAMPLE_RATE` | Web replay on error rate | `0` |
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
| `NEXTAUTH_URL` | アプリのベース URL（Digest 内リンク生成用） | -- |

LLM トークン単価は `ANTHROPIC_*_PER_MTOK_USD` 系の変数で上書き可能（`.env.example` 参照）。

ユーザーは設定画面から用途別の LLM モデルを選択できる（Anthropic / Google / Groq / DeepSeek / Alibaba / Mistral）。

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

1. **DB マイグレーション** -- Tailscale 経由で private DB に `migrate up`
2. **API デプロイ** -- `flyctl deploy` (`sifto-api`)
3. **Worker デプロイ** -- `flyctl deploy` (`sifto-worker`)

Web（Vercel）は Vercel の GitHub 連携による自動デプロイ。

Vercel 側では少なくとも以下の env を設定する:

- `NEXT_PUBLIC_CLERK_PUBLISHABLE_KEY`
- `CLERK_SECRET_KEY`
- `INTERNAL_API_SECRET`
- `NEXT_PUBLIC_API_URL`
- `NEXT_PUBLIC_ONESIGNAL_APP_ID`（使う場合）

### 必要な GitHub Secrets

| シークレット | 用途 |
|---|---|
| `MIGRATE_DATABASE_URL` | マイグレーション用 DB 接続文字列 |
| `TS_OAUTH_CLIENT_ID` | GitHub Actions から Tailscale に参加する OAuth Client ID |
| `TS_OAUTH_SECRET` | GitHub Actions から Tailscale に参加する OAuth Client secret |
| `FLY_API_TOKEN` | Fly.io デプロイトークン |

### Fly.io シークレットの設定

```sh
# Go API に環境変数を設定
cd api
flyctl secrets set \
  DATABASE_URL="..." \
  PYTHON_WORKER_URL="..." \
  INTERNAL_WORKER_SECRET="..." \
  INTERNAL_API_SECRET="..." \
  UPSTASH_REDIS_URL="..." \
  INNGEST_EVENT_KEY="..." \
  INNGEST_SIGNING_KEY="..." \
  CLERK_JWT_ISSUER="..." \
  CLERK_JWKS_URL="..." \
  CLERK_JWT_AUDIENCE="..." \
  USER_SECRET_ENCRYPTION_KEY="..." \
  RESEND_API_KEY="..." \
  RESEND_FROM_EMAIL="digest@yourdomain.com" \
  ONESIGNAL_APP_ID="..." \
  ONESIGNAL_REST_API_KEY="..." \
  GITHUB_APP_ID="..." \
  GITHUB_APP_PRIVATE_KEY="..." \
  SENTRY_DSN="..."

# Python Worker に環境変数を設定
cd worker
flyctl secrets set \
  INTERNAL_WORKER_SECRET="..." \
  REDIS_URL="..." \
  LANGFUSE_SECRET_KEY="..." \
  LANGFUSE_PUBLIC_KEY="..." \
  LANGFUSE_HOST="..." \
  SENTRY_DSN="..."
```

Anthropic / OpenAI / Google / Groq / DeepSeek / Alibaba / Mistral API キーはサーバー共通ではなく、ユーザーごとに設定画面から登録する（暗号化して `user_settings` に保存）。

### Tailscale を使った migration

GitHub Actions の `migrate` job は `tailscale/github-action@v4` を使って tailnet に参加し、private DB へ接続する。`MIGRATE_DATABASE_URL` は Tailscale 到達可能な DB ホスト / IP を向ける。

Tailscale 側で必要な準備:

1. GitHub Actions 用 OAuth Client を作成
2. `TS_OAUTH_CLIENT_ID` / `TS_OAUTH_SECRET` を GitHub Secrets に登録
3. `tag:ci` を作成し、`tagOwners` で許可する
4. DB 側は Tailscale からの 5432 のみ許可し、public 5432 は閉じる

## 設計上の判断

**Python マイクロサービス分離**
trafilatura（本文抽出）は Python ライブラリのため、Go API とは独立した FastAPI サービスとして分離。Inngest の step 関数から個別に呼ぶことで、各ステップ単位でリトライが可能。

**Clerk 認証の採用**
認証は Clerk を使用し、Web は Clerk セッション、API は Clerk Bearer token を受ける。external auth user id は `user_identities` を介して既存の `internal user_id` に解決するため、既存 DB スキーマを壊さずに移行できる。

**Fly.io の採用**
Inngest が 10 分ごとに cron でサービスを叩くため、コールドスタート遅延が問題になる。Fly.io の Machine auto-stop/start は数百 ms で起動するため採用。内部ネットワーク（`.internal`）で API → Worker を低レイテンシで呼び出せる点も利点。

**多段階処理パイプライン**
本文抽出 → 事実抽出 → 事実チェック → 要約 → 忠実性チェックの多段階に分離することで、LLM 呼び出し失敗時に該当ステップだけリトライできる。事実リストは中間成果物として保存し、UI でも確認可能。事実チェック・忠実性チェックのステップで品質を自動検証する。

**マルチ LLM 対応**
7 つのプロバイダ（Anthropic / Google / Groq / DeepSeek / Alibaba / Mistral / OpenAI）に対応。ユーザーが設定画面から用途別（事実抽出・要約・Digest 生成・Ask・事実チェック・忠実性チェック等）に使用するモデルを選択できる。Worker 側ではトランスポート層（`anthropic_transport`、`gemini_transport`、`openai_compat_transport`）でプロバイダの差異を吸収し、`llm_dispatch` で統一的にディスパッチする。モデル定義は `shared/llm_catalog.json` で一元管理する。

**ユーザー別 API キー管理**
サーバー共通の LLM キーではなく、ユーザーごとに自分の API キー（Anthropic / OpenAI / Google / Groq / DeepSeek / Alibaba / Mistral）を設定する方式。キーは `USER_SECRET_ENCRYPTION_KEY` で暗号化して DB に保存し、Worker 呼び出し時にリクエストヘッダで渡す。

**接続戦略**
Go API / Worker は DB へ常時接続し、Web は API を通じてアクセスする。migration は GitHub Actions から Tailscale 経由で private DB に対して実行する。

**OpenAI Embeddings による関連記事**
記事間の類似度計算には OpenAI の embedding API を使用。ユーザーが OpenAI API キーを設定すると、要約済み記事に対して embedding を生成し、関連記事の検索に利用する。

**多言語対応（i18n）**
フロントエンドは日本語・英語の 2 言語に対応。翻訳辞書は `web/src/i18n/dictionaries/` で管理し、`I18nProvider` でブラウザの言語設定に応じて自動切替する。API からは言語非依存のキーを返し、フロント側で翻訳する方針。

**PWA 対応**
モバイルでのホーム画面追加を促進するため、PWA インストールコンポーネントを搭載。

**Obsidian エクスポート**
お気に入り記事を GitHub App 経由で Obsidian Vault に Markdown ファイルとしてエクスポート。GitHub App のインストールで連携し、1 時間ごとに自動実行する。キーワードリンクモード（wiki-link / markdown-link / none）を選択可能。

**Langfuse による LLM オブザーバビリティ**
Worker の全 LLM 呼び出しを Langfuse でトレースし、リクエスト単位でのレイテンシ・トークン使用量・エラーを可視化する。

**プロバイダモデル自動検出**
6 時間ごとに各 LLM プロバイダの利用可能モデルをスキャンし、追加・削除されたモデルを `provider_model_updates` に記録する。

**Sentry / OneSignal / Redis**
Web・API・Worker に Sentry を導入し、Redis は API の JSON キャッシュと Worker の Gemini コンテキストキャッシュに利用する。Push 通知は OneSignal を利用し、高スコア記事を自動通知する。月次予算アラートも Push 通知で配信可能。
