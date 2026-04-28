# Sifto

A personal information curation system that collects RSS feeds and one-off URLs, then automates body extraction, fact extraction, summarization, quality checks, and digest delivery. It helps organize daily reading through briefings, quick triage, inline reader, topic pulse, AI Ask, review queues, LLM usage visualization, audio briefings, podcast delivery, AI Navigator Briefs, Prompt Admin, and more.

## Key Features

- RSS / single URL registration and automatic collection
- OPML import / export
- Inoreader integration for feed subscription import
- Per-article body extraction, fact extraction, fact-checking, summarization, and faithfulness checks
- Full-text search and suggestions via Meilisearch
- Briefing screen with highlights, Today Queue, clusters, and reading streaks
- Reading goal management and reading plans
- Quick triage and "read later" management
- Inline reader for summary / facts / original text
- Article notes, highlights, favorites with Markdown / Obsidian export
- AI Ask for article-based Q&A with insight saving
- AI Navigator (contextual navigation on briefing, article, source, and ask screens)
- AI Navigator Briefs (auto-generated morning / midday / evening briefings)
- Ask Insight saving, revisit queue, and weekly review
- Topic pulse and topic cluster visualization
- Source health monitoring, source optimization suggestions, feed recommendations and discovery
- Notification priority rule adjustment
- LLM usage, per-purpose cost, per-model usage, and value metrics visualization
- LLM analysis (cost vs. invocation scatter plots, etc.)
- LLM execution event tracking (success / failure / retry monitoring)
- Push notifications via OneSignal
- Obsidian GitHub export
- Per-purpose LLM model selection and provider model update tracking
- OpenRouter / Poe / Featherless / DeepInfra model catalog management
- SiliconFlow / Moonshot model support
- Audio briefings (LLM script generation + Aivis/Fish Speech/Gemini/xAI/ElevenLabs/Azure Speech TTS + concatenation + R2 storage)
- Summary audio player (per-article TTS playback)
- Playback session and history management
- Podcast feed generation and delivery
- Review queue with push notification reminders
- Prompt Admin (template management, versioning, A/B experiments)
- UI font settings (Google Fonts Japanese font catalog)
- Article genre classification and personal scoring feedback

## Architecture

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
- Cloudflare R2 (audio storage)
- GCP Cloud Run (audio concatenation)
```

## Tech Stack

| Layer | Implementation |
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
| Audio concat | GCP Cloud Run / local |
| LLM observability | Langfuse |
| Error tracking | Sentry |

## Repository Structure

```text
sifto/
├── api/                # Go API
├── worker/             # Python worker
├── web/                # Next.js frontend
├── db/migrations/      # SQL migrations
├── shared/             # API / worker shared definitions
│   ├── llm_catalog.json              # LLM model catalog
│   ├── ai_navigator_personas.json    # AI Navigator persona definitions
│   ├── gemini_tts_voices.json        # Gemini TTS voice catalog
│   ├── podcast_categories.json       # Podcast category definitions
│   ├── ui_font_catalog.json          # UI font catalog
│   └── prompt_templates/             # Prompt templates (42 files)
├── infra/
│   ├── audio-concat/   # Audio concat Cloud Run job / local server
│   └── gcp/            # GCP deployment config
├── scripts/            # Build helper scripts
│   └── generate_ui_font_catalog.mjs  # Google Fonts catalog generator
├── docs/               # Design and plan documents
├── docker-compose.yml  # Local development environment
├── Makefile            # Development commands
└── AGENTS.md           # Repository-specific rules
```

Notes:

- [shared/llm_catalog.json](shared/llm_catalog.json) manages available and default models.
- [shared/prompt_templates/](shared/prompt_templates/) contains per-purpose prompt templates, manageable via Prompt Admin.
- [api/cmd/server/main.go](api/cmd/server/main.go) contains the API routing.
- [api/internal/inngest/](api/internal/inngest/) contains scheduled jobs and event handlers.

## Screens and Use Cases

- **Briefing**: Recent notable articles, Today Queue, clusters, streaks, model update notifications, AI Navigator
- **Items**: Article list, filters, read management, related articles, full-text search, genre classification, feedback
- **Triage**: Fast triage centered on the focus queue
- **Pulse**: Time-series topic trend visualization
- **Clusters**: Related articles grouped by topic
- **Digests**: Generated digest list and details
- **Favorites**: Favorite articles, saved insights, Markdown export
- **Sources**: Source management, health, source optimization, AI recommendations and discovery, OPML, Inoreader import
- **Ask**: Article-based Q&A, insight saving, AI Navigator
- **AI Navigator Briefs**: Management and viewing of auto-generated morning / midday / evening briefings
- **Audio Briefings**: Audio briefing generation, management, and playback
- **Audio Player**: Per-article summary audio playback
- **Playback History**: Playback history management
- **Goals**: Reading goal management
- **Settings**: API keys, model selection, budget, notification priority, reading goals, reading plan, UI fonts, Obsidian, Inoreader, audio briefing, podcast integration
- **LLM Usage**: Daily / per-provider / per-model / per-purpose cost and value metrics
- **LLM Analysis**: Cost vs. invocation scatter plot analysis
- **OpenRouter Models**: OpenRouter model catalog sync and management
- **Poe Models**: Poe model catalog sync and usage
- **Featherless Models**: Featherless model catalog sync and management
- **DeepInfra Models**: DeepInfra model catalog sync and management
- **Aivis Models**: Aivis TTS model catalog sync
- **SiliconFlow Models**: SiliconFlow model catalog management
- **Fish Models**: Fish Speech model catalog management
- **xAI Voices**: xAI TTS voice catalog sync
- **ElevenLabs Voices**: ElevenLabs voice catalog sync
- **Azure Speech Voices**: Azure AI Speech voice catalog sync
- **OpenAI TTS Voices**: OpenAI TTS voice catalog sync
- **Gemini TTS Voices**: Gemini TTS voice catalog sync
- **Prompt Admin**: Prompt template management, versioning, A/B experiments
- **Provider Model Snapshots**: Provider model snapshot management
- **Debug**: Manual digest generation, embedding backfill, search index backfill, push test, etc.

## LLM / Model Operations

The current implementation supports 20 providers:

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

Key points:

- API keys are stored per user. Server-wide keys are not assumed.
- Alibaba (Qwen) currently uses the Virginia Global endpoint. Use API keys issued from the Virginia side, not Singapore / International.
- OpenRouter, Poe, Featherless, and DeepInfra periodically sync their model catalogs and update the model list dynamically.
- SiliconFlow uses a fixed set of models managed via a static catalog.
- Models can be selected per purpose:
  - facts
  - summary
  - digest cluster draft
  - digest
  - ask
  - source suggestion
  - facts check
  - faithfulness check
  - embedding
- Model definitions are shared between API and Worker via [shared/llm_catalog.json](shared/llm_catalog.json).
- Recent provider model updates can be checked on the Settings screen.
- Prompt Admin supports template management, versioning, and A/B experiments.

## Background Processing

Main Inngest jobs (30 functions):

| ID | Trigger | Description |
|---|---|---|
| `fetch-rss` | `*/10 * * * *` | Periodically fetch RSS and register new articles |
| `process-item` | `item/created` | Body extraction, fact extraction, checks, summarization, and notification |
| `embed-item` | `item/embed` | Generate embeddings |
| `generate-digest` | `0 21 * * *` | Create digest for JST 06:00 delivery |
| `compose-digest-copy` | `digest/created` | Generate digest subject, body, and cluster drafts |
| `send-digest` | `digest/copy-composed` | Deliver via Resend |
| `generate-briefing-snapshots` | `*/30 * * * *` | Generate briefing snapshots |
| `compute-topic-pulse-daily` | `10 * * * *` | Update topic pulse aggregations |
| `compute-preference-profiles` | `0 20 * * *` | Update preference profiles from recent reads / feedback |
| `export-obsidian-favorites` | `0 * * * *` | Export favorite articles to Obsidian |
| `track-provider-model-updates` | `0 */6 * * *` | Detect provider model diffs |
| `check-budget-alerts` | `0 0 * * *` | Monthly budget alert evaluation (email + push) |
| `generate-audio-briefings` | `0 * * * *` | Auto-generate audio briefings for enabled users |
| `run-audio-briefing-pipeline` | `audio-briefing/run` | Audio briefing script → TTS → concat pipeline |
| `move-audio-briefings-to-ia` | `17 3 * * *` | Move old audio to R2 IA bucket |
| `fail-stale-audio-briefing-voicing` | `*/5 * * * *` | Mark stalled voicing jobs as failed |
| `notify-review-queue` | `0 * * * *` | Push notifications for review queue |
| `sync-openrouter-models` | `0 3 * * *` | Sync OpenRouter model catalog |
| `sync-poe-usage-history` | `0 */6 * * *` | Sync Poe usage history |
| `generate-ai-navigator-briefs` | `0 * * * *` | Periodic AI Navigator Briefs generation (8/12/18 JST) |
| `run-ai-navigator-brief-pipeline` | `ai-navigator-brief/run` | AI Navigator Brief generation pipeline |
| `item-search-upsert` | `item/search.upsert` | Upsert article document to Meilisearch |
| `item-search-delete` | `item/search.delete` | Delete article document from Meilisearch |
| `item-search-backfill` | `item/search.backfill` | Bulk import articles to Meilisearch |
| `item-search-backfill-run` | `item/search.backfill.run` | Queue search backfill run |
| `search-suggestion-article-upsert` | `search/suggestions.article.upsert` | Update article suggestion index |
| `search-suggestion-article-delete` | `search/suggestions.article.delete` | Delete article suggestion index |
| `search-suggestion-source-upsert` | `search/suggestions.source.upsert` | Update source suggestion index |
| `search-suggestion-source-delete` | `search/suggestions.source.delete` | Delete source suggestion index |
| `search-suggestion-topics-refresh` | `search/suggestions.topics.refresh` | Refresh topic suggestions |

Article processing flow:

1. Register sources
2. Periodically fetch RSS and add to `items`
3. Worker performs body extraction, fact extraction, fact-checking, summarization, and faithfulness checks
4. Generate embeddings as needed
5. Update Meilisearch search index
6. Articles meeting score thresholds become push notification targets
7. Generate daily digests and briefing snapshots
8. Auto-generate audio briefings and deliver podcasts based on settings
9. Auto-generate AI Navigator Briefs at 8/12/18 JST

## API Overview

Authenticated API routes are defined in [api/cmd/server/main.go](api/cmd/server/main.go). Main route groups:

- `/api/items` — Article CRUD, search, triage, highlights, notes, feedback, genre
- `/api/sources` — Source management, OPML, Inoreader, health, recommendations and discovery
- `/api/topics` — Topic pulse
- `/api/ask` — Q&A, insights, Navigator
- `/api/digests` — Digest list and details
- `/api/llm-usage` — Usage, cost, value metrics, analysis
- `/api/provider-model-updates` — Provider model updates
- `/api/provider-model-snapshots` — Provider model snapshots
- `/api/openrouter-models` — OpenRouter model catalog
- `/api/poe-models` — Poe model catalog and usage
- `/api/featherless-models` — Featherless model catalog
- `/api/deepinfra-models` — DeepInfra model catalog
- `/api/aivis-models` — Aivis TTS model catalog
- `/api/fish-models` — Fish Speech model catalog
- `/api/xai-voices` — xAI TTS voice catalog
- `/api/elevenlabs-voices` — ElevenLabs voice catalog
- `/api/azure-speech-voices` — Azure AI Speech voice catalog
- `/api/openai-tts-voices` — OpenAI TTS voice catalog
- `/api/gemini-tts-voices` — Gemini TTS voice catalog
- `/api/briefing/today` — Briefing snapshot
- `/api/briefing/navigator` — Briefing Navigator
- `/api/ai-navigator-briefs` — AI Navigator Briefs
- `/api/audio-briefings` — Audio briefings
- `/api/audio-briefing-presets` — Audio briefing presets
- `/api/summary-audio` — Summary audio synthesis
- `/api/playback-sessions` — Playback sessions
- `/api/reviews` — Review queue
- `/api/dashboard` — Dashboard
- `/api/settings` — Various settings, API key management (22 providers), Prompt Admin

Public endpoints:

- `/podcasts/{slug}/feed.xml` — Podcast RSS feed

Internal endpoints:

- `/api/internal/users/*`
- `/api/internal/settings/obsidian-github/installation`
- `/api/internal/audio-briefings/{id}/concat-complete`
- `/api/internal/audio-briefings/chunks/{chunkID}/heartbeat`
- `/api/internal/debug/*`
- `/api/inngest`

Main Worker endpoints:

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

## Local Development

### Prerequisites

- Docker / Docker Compose
- `migrate` CLI

This repository runs all execution, formatting, and verification through `docker compose` / `make`.

### Initial Setup

```sh
cp .env.example .env
make up
make migrate-up
```

Local service URLs:

| Service | URL |
|---|---|
| Web | http://localhost:3000 |
| API | http://localhost:8081 |
| Worker | http://localhost:8000 |
| Meilisearch | http://localhost:7700 |
| Inngest Dev Server | http://localhost:8288 |

### Common Commands

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

Notes:

- After changing the web frontend, verify at least through `make web-build`.
- Use `make fmt-go` for Go formatting.
- Use `make check-worker` for Worker syntax checks and `make test-worker` for tests.

## Environment Variables

See [.env.example](.env.example) for details. Only the important ones are listed here.

### Required for Development

| Variable | Purpose |
|---|---|
| `DATABASE_URL` | Local PostgreSQL connection |
| `DOCKER_DATABASE_URL` | Compose-internal API DB connection |
| `PYTHON_WORKER_URL` | API → Worker URL |
| `DOCKER_PYTHON_WORKER_URL` | Compose-internal API → Worker URL |
| `INTERNAL_WORKER_SECRET` | API → Worker authentication |
| `INTERNAL_API_SECRET` | Web internal route → API authentication |
| `INNGEST_EVENT_KEY` | Inngest event key |
| `INNGEST_SIGNING_KEY` | Inngest signing key |
| `INNGEST_BASE_URL` | Self-host Inngest base URL |
| `INNGEST_CF_ACCESS_CLIENT_ID` / `INNGEST_CF_ACCESS_CLIENT_SECRET` | Cloudflare Access service token for self-host Inngest |
| `USER_SECRET_ENCRYPTION_KEY` | User API key encryption key |
| `NEXT_PUBLIC_API_URL` | Browser-facing API base URL |
| `NEXT_PUBLIC_CLERK_PUBLISHABLE_KEY` | Clerk publishable key |
| `CLERK_SECRET_KEY` | Clerk secret key |
| `CLERK_JWT_ISSUER` | Clerk JWT issuer |
| `CLERK_JWKS_URL` | Clerk JWKS URL |
| `MEILISEARCH_URL` | Meilisearch connection URL |
| `MEILISEARCH_MASTER_KEY` | Meilisearch master key |
| `TZ` | Timezone (`Asia/Tokyo`) |

### Optional but Feature-Related

| Variable | Purpose |
|---|---|
| `RESEND_API_KEY` / `RESEND_FROM_EMAIL` | Digest email delivery |
| `ONESIGNAL_APP_ID` / `ONESIGNAL_REST_API_KEY` | Push notification sending |
| `NEXT_PUBLIC_ONESIGNAL_APP_ID` | Web Push initialization |
| `ONESIGNAL_PICK_SCORE_THRESHOLD` | Notification score threshold |
| `ONESIGNAL_PICK_MAX_PER_DAY` | Max notifications per day |
| `GITHUB_APP_ID` / `GITHUB_APP_PRIVATE_KEY` / `GITHUB_APP_INSTALL_URL` | Obsidian GitHub export |
| `GOOGLE_CLIENT_ID` / `GOOGLE_CLIENT_SECRET` | Google OAuth / Inoreader |
| `YTDLP_COOKIES_B64` | YouTube extraction cookies.txt (base64) |
| `YTDLP_EXTRACTOR_ARGS` | `yt-dlp --extractor-args` passthrough |
| `YTDLP_POT_PROVIDER_BASE_URL` | bgutil PO Token provider HTTP server base URL |
| `YTDLP_POT_PROVIDER_DISABLE_INNERTUBE` | Pass `disable_innertube=1` to provider plugin |
| `LANGFUSE_SECRET_KEY` / `LANGFUSE_PUBLIC_KEY` / `LANGFUSE_HOST` | LLM observability |
| `SENTRY_DSN` | API / Worker Sentry |
| `NEXT_PUBLIC_SENTRY_DSN` | Web Sentry |
| `APP_COMMIT_SHA` | Sentry release identification |
| `AUDIO_BRIEFING_R2_*` | Cloudflare R2 audio storage |
| `AUDIO_BRIEFING_CONCAT_MODE` | Audio concat mode (`cloud_run` / `local`) |
| `AUDIO_BRIEFING_IA_MOVE_AFTER_DAYS` | Days before IA move |
| `AUDIO_BRIEFING_STALE_DELETE_AFTER_MINUTES` | Minutes before stale job deletion |
| `AUDIO_BRIEFING_CHUNK_RETRY_AFTER_SEC` | Seconds before chunk retry |
| `AIVIS_TTS_ENDPOINT` / `AIVIS_API_KEY` | Aivis TTS synthesis |
| `FISH_SPEECH_ENDPOINT` / `FISH_SPEECH_API_KEY` | Fish Speech synthesis |
| `PODCAST_FEED_BASE_URL` | Podcast RSS public URL |
| `AUDIO_BRIEFING_PUBLIC_BASE_URL` | Audio public custom domain |
| `PYTHON_WORKER_COMPOSE_DIGEST_TIMEOUT_SEC` | Digest composition timeout |
| `PYTHON_WORKER_ASK_TIMEOUT_SEC` | Ask timeout |
| `PYTHON_WORKER_AUDIO_BRIEFING_TIMEOUT_SEC` | Audio briefing timeout |
| `BRIEFING_SNAPSHOT_MAX_AGE_SEC` | Snapshot freshness threshold (seconds) |
| `ANTHROPIC_TIMEOUT_SEC` / `GEMINI_TIMEOUT_SEC` | LLM API timeouts |
| `ANTHROPIC_*_PER_MTOK_USD` | Anthropic price overrides |
| `GEMINI_*_CACHE*` | Gemini context cache settings |

### Local Authentication

`.env.example` ships with `ALLOW_DEV_AUTH_BYPASS=true`. To test Clerk-based authentication, fill in the Clerk-related environment variables.

## Data and Aggregation

- Date boundaries use JST (`Asia/Tokyo`) in relevant places.
- LLM Usage ensures consistent population across daily, per-model, and current-month aggregations.
- Topic pulse and preference profiles are recalculated by scheduled jobs.
- LLM execution events track success / failure / retries for analysis.

## Implementation Notes

- The Go API and Python Worker are separated, with body extraction and LLM processing handled on the Worker side.
- Intermediate artifacts (facts, summaries, checks, embeddings) are persisted.
- Redis is used for API JSON caching and some Worker-side caching.
- Meilisearch powers full-text search and suggestions for articles.
- Audio briefings follow the flow: LLM script generation → TTS via Aivis/Fish Speech/Gemini/xAI/ElevenLabs/Azure Speech → concatenation via Cloud Run / local → R2 storage.
- Older audio is automatically moved to the IA bucket after a configurable number of days.
- Podcast feeds are published per-user with slug-based URLs.
- Obsidian export goes through a GitHub App integration.
- OneSignal notifications link to in-app pages by default.
- AI Navigator Briefs are auto-generated at 8/12/18 JST and delivered via push notification.
- Prompt Admin enables prompt template versioning and A/B experiments.
- TTS markup preprocessing performs per-provider text normalization (`/tts/preprocess-text`).
- The UI font catalog is generated from Google Fonts via `scripts/generate_ui_font_catalog.mjs`.

## Verification Guide

Use the following depending on what was changed:

```sh
make check-fast
make check-web
make check
```

For frontend changes, run `make web-build`. For Worker changes, run `make check-worker` and `make test-worker`. For Go changes, run `make fmt-go-check` and related tests.
