# AGENTS.md

## 実行・検証ルール

- このリポジトリでは、実行・整形・検証はまず `docker compose` / `make` 経由で行う。
- 直接ローカルの `go`, `gofmt`, `node`, `npm` に依存しない（ローカル実行は明示指示がある場合のみ）。
- 代表例:
  - Go整形: `make fmt-go`
  - Web lint: `make web-lint`
  - Web build: `make web-build`
  - 一括チェック: `make check` / `make check-fast`
