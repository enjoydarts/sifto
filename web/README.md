## Web App

Sifto のフロントエンドです。Next.js App Router を使い、収集、選別、読書、保存、復習までをつなぐ UI を提供します。

主な画面:

- Briefing / Today Queue
- Items / Inline Reader / Related
- Triage
- Favorites / Notes / Highlights / Insights
- Sources / Source optimization
- Ask
- Reviews / Weekly review
- Settings / Notification priority / Reading goals
- LLM Usage / Value metrics

## Development

基本の実行と検証はリポジトリルートの `Makefile` / `docker compose` を使います。

```bash
make up
make web-lint
make web-build
```

Web コンテナ内で開発サーバーを直接起動する場合:

```bash
docker compose exec -T web npm run dev
```

## Notes

- App Router 構成です。
- 文言追加時は `src/i18n/dictionaries/ja.ts` と `src/i18n/dictionaries/en.ts` を両方更新します。
- 画面変更後は最低でも `make web-build` を通します。
