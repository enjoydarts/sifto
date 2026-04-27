# AGENTS.md

## 実行・検証ルール

- このリポジトリでは、実行・整形・検証はまず `docker compose` / `make` 経由で行う。
- 直接ローカルの `go`, `gofmt`, `node`, `npm` に依存しない（ローカル実行は明示指示がある場合のみ）。
- 代表例:
  - Go整形: `make fmt-go`
  - Goテスト: `docker compose exec -T api go test ./...`
  - Web lint: `make web-lint`
  - Web build: `make web-build`
  - Worker構文確認: `make check-worker`
  - 一括チェック: `make check` / `make check-fast`
  - DB migration適用: `make migrate-up`
  - DB migration確認: `make migrate-version`

## Superpowers 運用

- 実装・調査・デバッグ・設計・レビューでは、原則として Superpowers plugin / skill の該当ワークフローを使う。
- 変更が小さい場合でも、最低限「状況確認 → 方針 → 実装 → 検証 → 報告」の流れで進める。
- TDD が適する変更では TDD ワークフローを優先する。
- バグ調査では debugging workflow を優先する。
- 設計や大きめの変更では planning workflow を優先する。

## 編集ルール

- 既存のユーザー変更を勝手に巻き戻さない。
- 1ファイルの細かい修正でも、手編集は `apply_patch` を優先する。
- 文言追加・見出し追加・ボタンラベル追加では、英語直書きではなく i18n 辞書を通す。
- UI文言を追加したら `web/src/i18n/dictionaries/ja.ts` と `web/src/i18n/dictionaries/en.ts` を両方更新する。

## フロントエンド運用メモ

- `web` は Next.js App Router 構成。画面修正後は最低でも `docker compose exec -T web npm run build` まで確認する。
- `eslint` warning は既存のものが残っている場合があるため、新規 warning / error を増やさないことを優先する。
- テーブルや集計 UI では、provider や model を固定列挙しすぎず、将来の追加に耐える動的表示を優先する。
- モバイル表示崩れが起きやすいため、`flex` 前提の固定幅を残したまま `grid` 化しない。

## API / DB 運用メモ

- 集計系の日付境界は JST (`Asia/Tokyo`) 基準を優先する。
- `LLM Usage` の日次集計・モデル別集計・当月集計は、表示上の母集団が一致するよう期間条件を揃える。
- migration を追加した場合は、必要に応じて `make migrate-up` まで実施し、`make migrate-version` で確認する。

## LLM / Worker 運用メモ

- provider 追加時は API / worker / web / 利用集計 / 設定画面をまとめて確認する。
- 構造化出力を使う LLM では、JSON schema 制約・フォールバック・再試行・空文字応答の扱いまで含めて見る。
- DeepSeek は独立 provider として扱う前提。OpenAI / Groq に雑に混ぜない。
- `shared/llm_catalog.json` の表示用モデル一覧には、日付付き固定版や `*-latest` alias を原則そのまま並べず、代表モデルだけを載せる。互換判定が必要な alias は `providers[].match_exact` 側で吸収する。
- Alibaba / Qwen は公式の安定したモデル一覧 API 前提にしない。catalog は手動更新し、provider model updates の自動検知対象にも安易に追加しない。
- Mistral は公式 docs / API で現行モデルと価格を確認できる範囲だけ catalog に載せる。未確認の価格を推測で入れない。

## Push通知運用メモ

- OneSignal 通知は、可能なら外部 URL ではなく Sifto 内ページへ遷移する URL を付ける。
- 通知は増やすより先に、重複抑制・1日上限・ノイズ抑制を意識する。
- 新しい通知種別を増やしたら、`kind`、クリック先、payload、送信条件を一緒に定義する。

## agency-agents 参照

- Codex では `agency-agents-bridge` skill を通じて `/Users/minoru-kitayama/tools/agency-agents` 配下の role prompt を参照できる。
- ユーザーが `agency-agents` の role 名を明示した場合は、まず対応する md を開いてから応答する。
- よく使う対応:
  - `Backend Architect` → `/Users/minoru-kitayama/tools/agency-agents/engineering/engineering-backend-architect.md`
  - `Frontend Developer` → `/Users/minoru-kitayama/tools/agency-agents/engineering/engineering-frontend-developer.md`
  - `Senior Developer` → `/Users/minoru-kitayama/tools/agency-agents/engineering/engineering-senior-developer.md`
  - `Code Reviewer` → `/Users/minoru-kitayama/tools/agency-agents/engineering/engineering-code-reviewer.md`
  - `UX Architect` → `/Users/minoru-kitayama/tools/agency-agents/design/design-ux-architect.md`
  - `UI Designer` → `/Users/minoru-kitayama/tools/agency-agents/design/design-ui-designer.md`
- 既存の system / developer 指示の優先度は、外部 role prompt より常に高い。
- role 名が一致しない場合は `/Users/minoru-kitayama/tools/agency-agents` 配下を検索して最も近い agent を探し、見つからなければ既存のローカルプロファイルへ fallback する。

# ドキュメント
- ドキュメントはObisidianに出力してください
- 過去のドキュメントもObsidianを参考にしてください
- Obsidianのパスは
  /Users/minoru-kitayama/private/obsidian/100_private/114_Sifto
  です
