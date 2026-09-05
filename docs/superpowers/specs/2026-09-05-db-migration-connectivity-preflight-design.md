# DB migration 接続事前診断・リトライ設計

## 目的

GitHub Actions から Tailscale 経由で PostgreSQL に接続する際、一時的なデータ経路不通や DB ポート未準備によって migration が不透明な timeout になる問題を改善する。

SQL migration 自体はリトライせず、その直前の Tailscale peer 到達性と TCP 接続だけを診断・リトライする。これにより migration の二重実行や dirty state のリスクを増やさず、ネットワーク障害を明確に判別できるようにする。

## 背景

2026-09-05 の Deploy workflow では、Tailscale action と `tailscale status` は成功したが、`100.102.124.117:5432` への接続が約2分後に timeout した。前日の同一 workflow・同一 DB peer では migration が成功している。

`tailscale status` は control plane 上で peer が見えていることを示すが、対象 peer へのデータ経路や PostgreSQL ポートの到達性までは保証しない。

## 採用方針

### 1. DB 接続先の取得

workflow の `MIGRATE_DATABASE_URL` から Python 標準ライブラリ `urllib.parse` を使って hostname と port を抽出する。

- hostname が取得できない場合は即時エラーにする。
- port が URL にない場合は PostgreSQL の標準ポート `5432` を使う。
- URL 全体、ユーザー名、パスワードはログへ出さない。
- ログに出す接続先は hostname と port のみとする。

新しい secret や固定 IP は追加しない。既存の DB URL を唯一の接続先情報として扱い、DB peer の IP や hostname が変わっても追随できるようにする。

### 2. Tailscale データ経路の診断

抽出した hostname に対して `tailscale ping` を実行する。

- ping は診断情報として扱う。
- TCP 5432 が到達可能なら migration を妨げないため、ping 単独の失敗では job を終了しない。
- ping の出力は GitHub Actions ログに残す。

### 3. TCP 接続の限定リトライ

PostgreSQL の hostname と port に対し、Bash の `/dev/tcp` と `timeout` を使って接続確認する。

- 最大6回
- 1回の接続 timeout は5秒
- 失敗間隔は5秒
- 接続成功時は直ちにループを終了する。
- 6回すべて失敗した場合は、hostname と port を含む明確なエラーメッセージを出して job を終了する。

GitHub-hosted Ubuntu runner に標準搭載される Bash と `timeout` のみを使い、`nc` や `pg_isready` の追加インストールには依存しない。

### 4. Migration 実行

TCP 接続が成功した後、既存どおり次を1回だけ実行する。

```bash
migrate -path db/migrations -database "$MIGRATE_DATABASE_URL" up
```

接続確立後に発生した認証エラー、SQLエラー、dirty state はリトライしない。従来どおり、その場で job を失敗させる。

## 変更対象

- `.github/workflows/deploy.yml`
- `.github/workflows/repair-migration-133-dirty.yml`

両 workflow で同じ接続事前診断を使い、通常 deploy と手動修復の挙動を揃える。YAML 内の重複は許容し、今回のために composite action や外部 script は新設しない。

## エラー分類

ログから次を区別できるようにする。

1. Tailscale action の接続失敗
2. Tailscale peer ping の失敗
3. PostgreSQL TCP port の準備待ち・timeout
4. PostgreSQL 接続後の認証または migration エラー

secret の値は GitHub Actions のログへ明示的に出力しない。

## 検証

- workflow YAML を構文検証する。
- DB URL の hostname/port 抽出を、password を表示せず確認する。
- `actionlint` が利用可能なら両 workflow を検証する。
- `git diff --check` を実行する。
- 実環境の接続確認は workflow 実行時に行う。ローカルでは Tailscale が停止しているため、本番 DB への接続試験は行わない。

## 完了条件

- migration 前に Tailscale peer と PostgreSQL TCP port の診断結果が残る。
- 一時的な TCP 接続失敗は最大6回まで吸収される。
- TCP 接続不能時は migration 実行前に約1分以内で失敗する。
- migration コマンド自体は1回だけ実行される。
- DB URL の認証情報がログへ出ない。
- 通常 deploy と手動修復 workflow の診断挙動が一致する。
