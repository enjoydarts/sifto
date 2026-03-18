# Sifto Architecture

## Infrastructure Diagram

```mermaid
graph TB
    subgraph "ユーザー層"
        Browser["🌐 Browser / PWA"]
        Mobile["📱 Mobile"]
    end

    subgraph "フロントエンド"
        Vercel["☁️ Vercel\nNext.js 16 / React 19 / Tailwind v4\nRegion: hnd1 (東京)"]
    end

    subgraph "Fly.io (東京リージョン)"
        API["⚙️ Go API (chi)\nshared-cpu-1x / 256MB\nPort 8080"]
        Worker["🤖 Python Worker (FastAPI)\nshared-cpu-1x / 512MB\nPort 8000"]
        Inngest["⏰ Inngest\nワークフロー管理"]
    end

    subgraph "データベース"
        PostgreSQL["🐘 PostgreSQL (Neon)\nDB: sifto\n10+ migrations"]
        Redis["⚡ Redis 7\nキャッシュ / キュー"]
    end

    subgraph "LLM Providers"
        Anthropic["🟣 Anthropic\nClaude (要約/事実抽出)"]
        Google["🔵 Google Gemini\n翻訳/補助"]
        Groq["🟡 Groq\n高速推論"]
        DeepSeek["🟢 DeepSeek V3.2\nコスト最適化"]
        Zai["🔴 Z.ai (GLM)\nバックアップ"]
        OpenRouter["🔀 OpenRouter\nマルチモデルゲートウェイ"]
    end

    subgraph "外部サービス"
        Clerk["🔐 Clerk\n認証"]
        Inoreader["📖 Inoreader\nRSS連携"]
        OneSignal["🔔 OneSignal\nPush通知"]
        Sentry["🐛 Sentry\nエラー監視"]
    end

    subgraph "データソース"
        RSS["📰 RSS Feeds"]
        URLs["🔗 単発URL"]
        OPML["📋 OPML Import"]
    end

    %% 接続
    Browser --> Vercel
    Mobile --> Vercel
    Vercel --> API
    Vercel --> Clerk

    API --> PostgreSQL
    API --> Redis
    API --> Inngest
    API --> Worker

    Inngest --> API
    Inngest --> Worker

    Worker --> Anthropic
    Worker --> Google
    Worker --> Groq
    Worker --> DeepSeek
    Worker --> Zai
    Worker --> OpenRouter

    Worker --> PostgreSQL
    Worker --> Redis

    API --> Inoreader
    API --> OneSignal
    Vercel --> Sentry

    RSS --> API
    URLs --> API
    OPML --> API

    %% スタイル
    classDef frontend fill:#3b82f6,stroke:#1e40af,color:#fff
    classDef backend fill:#10b981,stroke:#065f46,color:#fff
    classDef data fill:#f59e0b,stroke:#92400e,color:#fff
    classDef llm fill:#8b5cf6,stroke:#5b21b6,color:#fff
    classDef external fill:#6b7280,stroke:#374151,color:#fff
    classDef source fill:#ec4899,stroke:#9d174d,color:#fff

    class Browser,Mobile,Vercel frontend
    class API,Worker,Inngest backend
    class PostgreSQL,Redis data
    class Anthropic,Google,Groq,DeepSeek,Zai,OpenRouter llm
    class Clerk,Inoreader,OneSignal,Sentry external
    class RSS,URLs,OPML source
```

## 処理フロー

```
1. 収集    RSS/URL/OPML → Go API → Ingestion
2. 抽出    Python Worker → 本文抽出 → 事実抽出
3. 要約    LLM (Anthropic/DeepSeek) → 要約生成 → 品質チェック
4. 配信    Inngest → ダイジェスト生成 → OneSignal Push / Email
5. 閲覧    Next.js → ブリーフィング / Today Queue / インラインリーダー
6. 問い合せ AI Ask (RAG) → 記事検索 → LLM回答
```

## LLM 使用戦略

| 用途 | プライマリ | バックアップ |
|---|---|---|
| 事実抽出 | Anthropic Claude | DeepSeek V3.2 |
| 要約 | Anthropic Claude | Google Gemini |
| ダイジェスト | Anthropic Claude | Groq |
| 翻訳 | OpenRouter | - |
| Ask (質問応答) | ユーザー選択 | - |

## デプロイ構成

| コンポーネント | プラットフォーム | Region | スペック |
|---|---|---|---|
| Web (Next.js) | Vercel | hnd1 (東京) | - |
| API (Go) | Fly.io | nrt (東京) | shared-cpu-1x / 256MB |
| Worker (Python) | Fly.io | nrt (東京) | shared-cpu-1x / 512MB |
| Inngest | Fly.io | nrt (東京) | - |
| PostgreSQL | Neon (Serverless) | - | - |
| Redis | - | - | - |

## 特徴

- **マルチLLM戦略**: コストと品質のバランスを用途別に最適化
- **Inngestベースのワークフロー**: 非同期タスクの信頼性高いオーケストレーション
- **東京リージョン偏重**: API・Worker・Web全て東京近辺（低レイテンシ）
- **Fly.io auto-stop**: 零時帯は機械を停止してコスト削減
