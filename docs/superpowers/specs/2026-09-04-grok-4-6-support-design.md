# Grok 4.6 対応設計

## 目的

xAI の `grok-4.6` を Sifto の LLM モデル選択肢へ追加し、既存の xAI API キーと実行経路から利用可能にする。

## 公式仕様

- API model ID: `grok-4.6`
- context window: 500,000 tokens
- short-context pricing（1M tokens あたり）:
  - input: 2.00 USD
  - cached input: 0.50 USD
  - output: 6.00 USD
- 200,000 tokens 以上の prompt には long-context pricing が適用される。現行の Sifto catalog は入力長別価格を表現できないため、通常利用の概算に用いる short-context pricing を登録する。
- structured output、reasoning、tool calling に対応する。

参照:

- https://docs.x.ai/developers/models/grok-4.6
- https://docs.x.ai/developers/pricing

## 採用方針

既存の Grok 4.5 追加と同じ固定 catalog パターンを使う。xAI provider の通信処理、API キー管理、利用集計は catalog 駆動で共通化されているため、新しい専用実装は追加しない。

`grok-4.6` は全 LLM 用途で選択可能にするが、xAI の用途別既定モデルは変更しない。既存利用者の挙動と費用を暗黙に変えず、ユーザーが明示的に選択した場合だけ利用される構成とする。

## 変更内容

### Provider 判定

`shared/llm_catalog.json` の xAI provider にある `match_exact` へ `grok-4.6` を追加する。これにより API と worker の共通 provider 解決が xAI と判定する。

### 表示用モデル定義

同 catalog のモデル一覧へ次の定義を追加する。

- provider: `xai`
- available purposes: `facts`, `summary`, `digest_cluster_draft`, `digest`, `ask`, `source_suggestion`
- recommendation: `strong`
- best for: `balanced`
- highlights: `latest`
- capabilities:
  - structured output: supported
  - strict JSON schema: unsupported（既存 xAI モデルと同じ安全側の扱い）
  - reasoning: supported
  - tool calling: supported
  - cache read pricing: supported
  - cache write pricing: unsupported
- pricing source: `xai_docs_2026_08`
- input: 2.00 USD/MTok
- cached input: 0.50 USD/MTok
- output: 6.00 USD/MTok

最新モデル表示を一意に保つため、`grok-4.5` から `latest` highlight を外す。

### 既定モデル

xAI provider の `default_models` は変更しない。Grok 4.6 への自動移行や既存設定の書き換えも行わない。

### 対象外

- 価格が2倍となる Grok 4.6 fast variant
- 200,000 tokens を境界にした動的な long-context 価格計算
- DB migration
- UI 固有コンポーネントや i18n 文言の追加

## データフロー

モデル選択 UI は共通 catalog から `grok-4.6` を表示する。選択された model ID は既存経路で API または worker に渡り、xAI の `match_exact` により provider が解決され、保存済みの xAI API キーを使って既存の xAI service が呼び出される。利用料金の概算には追加した catalog pricing を用いる。

## エラー処理

Grok 4.6 専用のエラー処理は追加しない。API キー未設定、xAI API エラー、空応答、構造化出力失敗は既存 xAI 経路の処理を継承する。

## 検証

- worker catalog smoke test に `("xai", "grok-4.6")` を追加し、provider 解決と pricing 取得を確認する。
- API catalog test にモデル存在確認を追加する。
- catalog JSON の構文と整合性を worker テストで確認する。
- リポジトリルールに従い、Docker Compose / Make 経由の関連テストと `make check-fast` を実行する。

## 完了条件

- LLM モデル選択肢に Grok 4.6 が表示される。
- Grok 4.6 が xAI provider として解決される。
- 通常入力、cached input、出力の概算価格を取得できる。
- Grok 4.6 のみが xAI モデル群の `latest` 表示を持つ。
- xAI の既定モデルは変更されない。
- 関連テストが成功する。
