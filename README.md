# Sifto

RSS フィードと単発 URL を収集し、本文抽出、事実抽出、要約、品質チェック、Digest 配信まで自動化するパーソナル情報収集服务体系です。ブリーフィング、クイックトリアージ、インラインリーダー、トピックパルス、AI Ask、復習導線、LLM 使用量可視化、音声ブリーフィング、Podcast 配信、AI Navigator Briefs、Prompt Admin などで、日々のインプット整理を支援します。

## 現在の主な機能

- RSS / 単発 URL の登録と自動収集
- OPML インポート / エクスポート
- Inoreader 連携による購読フィード取り込み
- 記事ごとの本文抽出、事実抽出、事実チェック、要約、要約忠実性チェック
- Meilisearch による全文検索とサジェスト
- ブリーフィング画面でのハイライト、Today Queue、クラスタ、リーディングストリーク表示
- 読書ゴール管理、読書プラン
- クイックトリアージと「あとで読む」管理
- インラインリーダーでの要約 / 事実 / 原文確認
- 記事メモ、ハイライト、お気に入り Markdown / Obsidian エクスポート
- AI Ask による記事ベースの質問応答、Insight 保存
- AI Navigator（ブリーフィング、記事、ソース、Ask 各画面の文脈ナビ）
- AI Navigator Briefs（朝・昼・夜の自動生成ブリーフィング）
- Ask Insight 保存、再訪キュー、週次レビュー
- トピックパルスとトピッククラスタ表示
- ソース健全性と source optimization 提案、フィード推薦・発見
- 通知優先度ルール調整
- LLM 使用量、用途別コスト、モデル別使用量、value metrics の可視化
- LLM 分析（コスト vs 呼び出し散布図等）
- LLM 実行イベント追跡（成功/失敗/リトライ監視）
- OneSignal による Push 通知
- Obsidian GitHub エクスポート
- 用途別 LLM モデル選択と provider model updates の確認
- OpenRouter / Poe / Featherless / DeepInfra モデルカタログ管理
- SiliconFlow / Moonshot モデル対応
- 音声ブリーフィング（LLM スクリプト生成 + Aivis/Fish Speech/Gemini/xAI/ElevenLabs/Azure Speech 音声合成 + 連結 + R2 保管）
- 要約音声プレイヤー（記事単位の TTS 再生）
- 再生セッション・再生履歴管理
- Podcast フィード生成・配信
- 復習キューと Push 通知リマインド
- Prompt Admin（テンプレート管理・バージョン管理・A/B 実験）
- UI フォント設定（Google Fonts 日本語フォントカタログ）
- 記事ジャンル分類・パーソナルスコアフィードバック

## アーキテクチャ

```text
[Browser / PWA]
      |
      v
[Next.js 16 / React 19 / Tailwind v4]
      |
      v
[Go API / chi]
   |        \         \
   |         \         \--> [Meilisearch]
   |          \--> [Redis]
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
      +--> Cerebras
      +--> DeepSeek
      +--> Alibaba (Qwen)
      +--> Mistral
      +--> MiniMax
      +--> Xiaomi Mimo
      +--> Moonshot (Kimi)
      +--> xAI (Grok)
      +--> ZAI (GLM)
      +--> Fireworks
      +--> Together
      +--> Featherless
      +--> DeepInfra
      +--> OpenRouter
      +--> Poe
      +--> SiliconFlow
      +--> OpenAI

TTS providers:
      +--> Aivis
      +--> Fish Speech
      +--> OpenAI TTS
      +--> Gemini TTS
      +--> xAI TTS
      +--> ElevenLabs
      +--> Azure AI Speech

Other integrations:
- Clerk
- Resend
- OneSignal
- Langfuse
- GitHub App
- Sentry
- Cloudflare R2 (音声保管)
- GCP Cloud Run (音声連結)
```

## 技術スタック

| レイヤ | 実装 |
|---|---|
| Web | Next.js 16, React 19, TypeScript, Tailwind CSS v4 |
| API | Go 1.24, chi, pgx, Inngest SDK |
| Worker | Python, FastAPI, trafilatura |
| DB | PostgreSQL 16 |
| Cache | Redis 7 |
| Search | Meilisearch v1.13 |
| Job orchestration | Inngest |
| Auth | Clerk |
| Mail | Resend |
| Push | OneSignal |
| TTS | Aivis, Fish Speech, OpenAI TTS, Gemini TTS, xAI TTS, ElevenLabs, Azure AI Speech |
| Object storage | Cloudflare R2 |
| Audio concat | GCP Cloud Run / ローカル |
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
│   ├── llm_catalog.json              # LLM モデルカタログ
│   ├── ai_navigator_personas.json    # AI Navigator ペルソナ定義
│   ├── gemini_tts_voices.json        # Gemini TTS 音声カタログ
│   ├── podcast_categories.json       # Podcast カテゴリ定義
│   ├── ui_font_catalog.json          # UI フォントカタログ
│   └── prompt_templates/             # プロンプトテンプレート (42 files)
├── infra/
│   ├── audio-concat/   # 音声連結 Cloud Run ジョブ / ローカルサーバー
│   └── gcp/            # GCP デプロイ設定
├── scripts/            # ビルド補助スクリプト
│   └── generate_ui_font_catalog.mjs  # Google Fonts カタログ生成
├── docs/               # 設計・プランドキュメント
├── docker-compose.yml  # ローカル開発環境
├── Makefile            # 開発用コマンド
└── AGENTS.md           # リポジトリ固有ルール
```

補足:

- [shared/llm_catalog.json](/Users/minoru-kitayama/private/sifto/shared/llm_catalog.json) で利用可能モデルと既定モデルを管理します。
- [shared/prompt_templates/](/Users/minoru-kitayama/private/sifto/shared/prompt_templates/) に用途別のプロンプトテンプレートが格納されています。Prompt Admin で管理・上書き可能です。
- [api/cmd/server/main.go](/Users/minoru-kitayama/private/sifto/api/cmd/server/main.go) に API ルーティングがまとまっています。
- [api/internal/inngest/](/Users/minoru-kitayama/private/sifto/api/internal/inngest/) 配下に定期ジョブとイベント処理があります。

## 画面とユースケース

- ブリーフィング: 直近の注目記事、Today Queue、クラスタ、ストリーク、モデル更新通知、AI Navigator を表示
- Items: 記事一覧、フィルタ、既読管理、関連記事表示、全文検索、ジャンル分類、フィードバック
- Triage: フォーカスキューを中心に高速に仕分け
- Pulse: トピックの時系列推移を可視化
- Clusters: 関連記事をトピック単位で確認
- Digests: 生成済み Digest の一覧と詳細を確認
- Favorites: お気に入り記事の一覧、保存 Insight、Markdown エクスポート
- Sources: ソース管理、健全性、source optimization、AI 推薦・発見、OPML、Inoreader 取り込み
- Ask: 記事内容に基づく質問応答、Insight 保存、AI Navigator
- AI Navigator Briefs: 朝・昼・夜の自動生成ブリーフィングの管理・確認
- Audio Briefings: 音声ブリーフィング生成・管理・再生
- Audio Player: 記事要約の音声再生
- Playback History: 再生履歴の管理
- Goals: 読書ゴール管理
- Settings: API キー、モデル選択、予算、通知優先度、読書ゴール、読書プラン、UI フォント、Obsidian、Inoreader、音声ブリーフィング、Podcast 連携
- LLM Usage: 日次 / プロバイダ別 / モデル別 / 用途別コストと value metrics 確認
- LLM Analysis: コスト vs 呼び出し散布図による分析
- OpenRouter Models: OpenRouter モデルカタログ同期・管理
- Poe Models: Poe モデルカタログ同期・使用量確認
- Featherless Models: Featherless モデルカタログ同期・管理
- DeepInfra Models: DeepInfra モデルカタログ同期・管理
- Aivis Models: Aivis TTS モデルカタログ同期
- SiliconFlow Models: SiliconFlow モデルカタログ管理
- Fish Models: Fish Speech モデルカタログ管理
- xAI Voices: xAI TTS 音声カタログ同期
- ElevenLabs Voices: ElevenLabs 音声カタログ同期
- Azure Speech Voices: Azure AI Speech 音声カタログ同期
- OpenAI TTS Voices: OpenAI TTS 音声カタログ同期
- Gemini TTS Voices: Gemini TTS 音声カタログ同期
- Prompt Admin: プロンプトテンプレート管理・バージョン管理・A/B 実験
- Provider Model Snapshots: プロバイダモデルスナップショット管理
- Debug: Digest 手動生成、embedding backfill、検索インデックス backfill、Push テスト等

## LLM / モデル運用

現行実装では 20 provider を扱います。

- Anthropic
- Google
- Groq
- Cerebras
- DeepSeek
- Alibaba
- Mistral
- MiniMax
- Xiaomi Mimo
- Moonshot
- xAI
- ZAI
- Fireworks
- Together
- Featherless
- DeepInfra
- OpenRouter
- Poe
- SiliconFlow
- OpenAI

ポイント:

- ユーザーごとに API キーを保存します。サーバー共通キー前提ではありません。
- Alibaba (Qwen) は現在 Virginia の Global endpoint を前提にしています。Singapore / International 用ではなく、Virginia 側で発行した API key を使ってください。
- OpenRouter、Poe、Featherless、DeepInfra はモデルカタログを定期同期し、動的にモデル一覧を更新します。
- SiliconFlow は固定モデル群で、静的カタログで管理します。
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
- Prompt Admin でテンプレート管理・バージョン管理・A/B 実験が行えます。

## バックグラウンド処理

主要な Inngest ジョブ (30 functions):

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
| `check-budget-alerts` | `0 0 * * *` | 月次予算アラート判定（メール + Push） |
| `generate-audio-briefings` | `0 * * * *` | 有効ユーザーの音声ブリーフィングを自動生成 |
| `run-audio-briefing-pipeline` | `audio-briefing/run` | 音声ブリーフィングのスクリプト→TTS→連結パイプライン |
| `move-audio-briefings-to-ia` | `17 3 * * *` | 古い音声を R2 IA バケットへ移送 |
| `fail-stale-audio-briefing-voicing` | `*/5 * * * *` | 停滞した音声合成ジョブを失敗扱いに |
| `notify-review-queue` | `0 * * * *` | 復習キューの Push 通知 |
| `sync-openrouter-models` | `0 3 * * *` | OpenRouter モデルカタログ同期 |
| `sync-poe-usage-history` | `0 */6 * * *` | Poe 使用量履歴の同期 |
| `generate-ai-navigator-briefs` | `0 * * * *` | AI Navigator Briefs の定期生成（8/12/18時） |
| `run-ai-navigator-brief-pipeline` | `ai-navigator-brief/run` | AI Navigator Brief 生成パイプライン実行 |
| `item-search-upsert` | `item/search.upsert` | Meilisearch へ記事ドキュメントを登録 |
| `item-search-delete` | `item/search.delete` | Meilisearch から記事ドキュメントを削除 |
| `item-search-backfill` | `item/search.backfill` | Meilisearch へ記事を一括投入 |
| `item-search-backfill-run` | `item/search.backfill.run` | 検索バックフィル実行のキューイング |
| `search-suggestion-article-upsert` | `search/suggestions.article.upsert` | 記事サジェストインデックス更新 |
| `search-suggestion-article-delete` | `search/suggestions.article.delete` | 記事サジェストインデックス削除 |
| `search-suggestion-source-upsert` | `search/suggestions.source.upsert` | ソースサジェストインデックス更新 |
| `search-suggestion-source-delete` | `search/suggestions.source.delete` | ソースサジェストインデックス削除 |
| `search-suggestion-topics-refresh` | `search/suggestions.topics.refresh` | トピックサジェスト更新 |

記事処理の大まかな流れ:

1. ソースを登録する
2. RSS を定期取得して `items` に追加する
3. Worker で本文抽出、事実抽出、事実チェック、要約、忠実性チェックを行う
4. 必要に応じて embedding を生成する
5. Meilisearch の検索インデックスを更新する
6. スコア条件を満たす記事は Push 通知対象になる
7. 日次で Digest と briefing snapshot を生成する
8. 設定に応じて音声ブリーフィングを自動生成・Podcast 配信する
9. 8/12/18時に AI Navigator Briefs を自動生成する

## API の概要

認証付き API は [api/cmd/server/main.go](/Users/minoru-kitayama/private/sifto/api/cmd/server/main.go) に定義されています。主なグループは以下です。

- `/api/items` — 記事 CRUD、検索、トリアージ、ハイライト、メモ、フィードバック、ジャンル
- `/api/sources` — ソース管理、OPML、Inoreader、健全性、推薦・発見
- `/api/topics` — トピックパルス
- `/api/ask` — 質問応答、Insight、Navigator
- `/api/digests` — Digest 一覧・詳細
- `/api/llm-usage` — 使用量、コスト、value metrics、分析
- `/api/provider-model-updates` — プロバイダモデル更新
- `/api/provider-model-snapshots` — プロバイダモデルスナップショット
- `/api/openrouter-models` — OpenRouter モデルカタログ
- `/api/poe-models` — Poe モデルカタログ・使用量
- `/api/featherless-models` — Featherless モデルカタログ
- `/api/deepinfra-models` — DeepInfra モデルカタログ
- `/api/aivis-models` — Aivis TTS モデルカタログ
- `/api/fish-models` — Fish Speech モデルカタログ
- `/api/xai-voices` — xAI TTS 音声カタログ
- `/api/elevenlabs-voices` — ElevenLabs 音声カタログ
- `/api/azure-speech-voices` — Azure AI Speech 音声カタログ
- `/api/openai-tts-voices` — OpenAI TTS 音声カタログ
- `/api/gemini-tts-voices` — Gemini TTS 音声カタログ
- `/api/briefing/today` — ブリーフィングスナップショット
- `/api/briefing/navigator` — ブリーフィング Navigator
- `/api/ai-navigator-briefs` — AI Navigator Briefs
- `/api/audio-briefings` — 音声ブリーフィング
- `/api/audio-briefing-presets` — 音声ブリーフィングプリセット
- `/api/summary-audio` — 要約音声合成
- `/api/playback-sessions` — 再生セッション
- `/api/reviews` — 復習キュー
- `/api/dashboard` — ダッシュボード
- `/api/settings` — 各種設定、API キー管理 (22 providers)、Prompt Admin

公開エンドポイント:

- `/podcasts/{slug}/feed.xml` — Podcast RSS フィード

内部向けエンドポイント:

- `/api/internal/users/*`
- `/api/internal/settings/obsidian-github/installation`
- `/api/internal/audio-briefings/{id}/concat-complete`
- `/api/internal/audio-briefings/chunks/{chunkID}/heartbeat`
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
- `/ask-rerank`
- `/ask-navigator`
- `/compose-digest`
- `/compose-digest-cluster-draft`
- `/rank-feed-suggestions`
- `/suggest-feed-seed-sites`
- `/audio-briefing/script`
- `/audio-briefing/synthesize-upload` (Aivis)
- `/audio-briefing/synthesize-upload-gemini-duo`
- `/audio-briefing/synthesize-upload-fish-duo`
- `/audio-briefing/synthesize-upload-elevenlabs-duo`
- `/audio-briefing/synthesize-upload-azure-speech-duo`
- `/audio-briefing/presign`
- `/audio-briefing/delete-objects`
- `/audio-briefing/copy-objects`
- `/summary-audio/synthesize`
- `/tts/preprocess-text`
- `/briefing-navigator`
- `/item-navigator`
- `/source-navigator`
- `/ai-navigator-brief/generate`

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
| Meilisearch | http://localhost:7700 |
| Inngest Dev Server | http://localhost:8288 |

### よく使うコマンド

```sh
make up
make up-core
make down
make build
make build-audio-concat-local
make restart
make ps
make logs-api
make logs-worker
make logs-web
make logs-audio-concat-local
make web-lint
make web-build
make fmt-go
make fmt-go-check
make check-worker
make test-worker
make check-worker-full
make check-fast
make check-web
make check
make migrate-up
make migrate-version
```

補足:

- Web を変更したら最低でも `make web-build` 相当まで確認してください。
- Go 整形は `make fmt-go` を使ってください。
- Worker の構文確認は `make check-worker`、テスト実行は `make test-worker` です。

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
| `INNGEST_BASE_URL` | self-host Inngest の base URL |
| `INNGEST_CF_ACCESS_CLIENT_ID` / `INNGEST_CF_ACCESS_CLIENT_SECRET` | Cloudflare Access 配下の self-host Inngest に API から接続するための Service Token |
| `USER_SECRET_ENCRYPTION_KEY` | ユーザー API キー暗号化 |
| `NEXT_PUBLIC_API_URL` | ブラウザから見る API ベース URL |
| `NEXT_PUBLIC_CLERK_PUBLISHABLE_KEY` | Clerk |
| `CLERK_SECRET_KEY` | Clerk |
| `CLERK_JWT_ISSUER` | Clerk JWT issuer |
| `CLERK_JWKS_URL` | Clerk JWKS URL |
| `MEILISEARCH_URL` | Meilisearch 接続 URL |
| `MEILISEARCH_MASTER_KEY` | Meilisearch マスターキー |
| `TZ` | タイムゾーン（`Asia/Tokyo`） |

### 任意だが機能に関わるもの

| 変数 | 用途 |
|---|---|
| `RESEND_API_KEY` / `RESEND_FROM_EMAIL` | Digest メール送信 |
| `ONESIGNAL_APP_ID` / `ONESIGNAL_REST_API_KEY` | Push 通知送信 |
| `NEXT_PUBLIC_ONESIGNAL_APP_ID` | Web Push 初期化 |
| `ONESIGNAL_PICK_SCORE_THRESHOLD` | 通知対象スコア閾値 |
| `ONESIGNAL_PICK_MAX_PER_DAY` | 1日最大通知件数 |
| `GITHUB_APP_ID` / `GITHUB_APP_PRIVATE_KEY` / `GITHUB_APP_INSTALL_URL` | Obsidian GitHub エクスポート |
| `GOOGLE_CLIENT_ID` / `GOOGLE_CLIENT_SECRET` | Google OAuth / Inoreader 周辺 |
| `YTDLP_COOKIES_B64` | YouTube 抽出用 cookies.txt を base64 で渡す |
| `YTDLP_EXTRACTOR_ARGS` | `yt-dlp --extractor-args` をそのまま渡す |
| `YTDLP_POT_PROVIDER_BASE_URL` | bgutil PO Token provider HTTP server の base URL |
| `YTDLP_POT_PROVIDER_DISABLE_INNERTUBE` | provider plugin に `disable_innertube=1` を渡す |
| `LANGFUSE_SECRET_KEY` / `LANGFUSE_PUBLIC_KEY` / `LANGFUSE_HOST` | LLM observability |
| `SENTRY_DSN` | API / Worker Sentry |
| `NEXT_PUBLIC_SENTRY_DSN` | Web Sentry |
| `APP_COMMIT_SHA` | Sentry リリース識別 |
| `AUDIO_BRIEFING_R2_*` | Cloudflare R2 音声保管 |
| `AUDIO_BRIEFING_CONCAT_MODE` | 音声連結モード (`cloud_run` / `local`) |
| `AUDIO_BRIEFING_IA_MOVE_AFTER_DAYS` | IA 移送までの日数 |
| `AUDIO_BRIEFING_STALE_DELETE_AFTER_MINUTES` | stale job 削除までの分数 |
| `AUDIO_BRIEFING_CHUNK_RETRY_AFTER_SEC` | chunk 再試行までの秒数 |
| `AIVIS_TTS_ENDPOINT` / `AIVIS_API_KEY` | Aivis TTS 音声合成 |
| `FISH_SPEECH_ENDPOINT` / `FISH_SPEECH_API_KEY` | Fish Speech 音声合成 |
| `PODCAST_FEED_BASE_URL` | Podcast RSS 公開 URL |
| `AUDIO_BRIEFING_PUBLIC_BASE_URL` | 音声公開用カスタムドメイン |
| `PYTHON_WORKER_COMPOSE_DIGEST_TIMEOUT_SEC` | Digest 合成タイムアウト |
| `PYTHON_WORKER_ASK_TIMEOUT_SEC` | Ask タイムアウト |
| `PYTHON_WORKER_AUDIO_BRIEFING_TIMEOUT_SEC` | 音声ブリーフィングタイムアウト |
| `BRIEFING_SNAPSHOT_MAX_AGE_SEC` | スナップショット新鲜判定秒数 |
| `ANTHROPIC_TIMEOUT_SEC` / `GEMINI_TIMEOUT_SEC` | LLM API タイムアウト |
| `ANTHROPIC_*_PER_MTOK_USD` | Anthropic 価格上書き |
| `GEMINI_*_CACHE*` | Gemini コンテキストキャッシュ設定 |

### ローカル認証

`.env.example` では `ALLOW_DEV_AUTH_BYPASS=true` が入っていますが、Clerk 前提の動作確認をする場合は Clerk 関連 env を埋めてください。

## データと集計の考え方

- 日付境界は JST (`Asia/Tokyo`) を基準に扱う箇所があります。
- LLM Usage では日次、モデル別、当月集計の母集団をそろえる実装があります。
- topic pulse と preference profile は定期ジョブで再計算されます。
- LLM 実行イベントで成功/失敗/リトライを追跡し、分析に活用します。

## 実装上のポイント

- Go API と Python Worker を分離し、本文抽出と LLM 処理を Worker 側へ寄せています。
- 中間成果物として facts、summary、checks、embedding を保持します。
- Redis は API の JSON キャッシュや Worker 側の一部キャッシュに利用します。
- Meilisearch は記事の全文検索とサジェストに利用します。
- 音声ブリーフィングは LLM でスクリプト生成 → Aivis/Fish Speech/Gemini/xAI/ElevenLabs/Azure Speech で TTS → Cloud Run / ローカルで連結 → R2 に保管の流れです。
- 古い音声は設定日数後に IA バケットへ自動移送されます。
- Podcast フィードはユーザーごとに slug ベースで公開されます。
- Obsidian エクスポートは GitHub App 経由です。
- OneSignal はアプリ内ページへの導線を前提にしています。
- AI Navigator Briefs は 8/12/18時に自動生成され、Push 通知で送达されます。
- Prompt Admin でプロンプトテンプレートをバージョン管理し、A/B 実験が行えます。
- TTS マークアップ前処理で provider ごとのテキスト正規化を行います（`/tts/preprocess-text`）。
- UI フォントカタログは `scripts/generate_ui_font_catalog.mjs` で Google Fonts から生成します。

## 検証の目安

変更内容に応じて次を使ってください。

```sh
make check-fast
make check-web
make check
```

フロントエンド変更時は `make web-build`、Worker 変更時は `make check-worker` と `make test-worker`、Go 変更時は `make fmt-go-check` と関連テストの実行を推奨します。
