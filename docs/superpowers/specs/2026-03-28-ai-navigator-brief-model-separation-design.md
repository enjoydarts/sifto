# AIナビゲーターとAIナビブリーフのモデル分離 設計

## 概要

`AIナビゲーター` と `AIナビブリーフ` が同じ `navigator` / `navigator_fallback` 設定を共有している状態をやめ、`AIナビブリーフ` 用に独立した `通常モデル` と `fallbackモデル` を持てるようにする。

設定画面では、既存の `AIナビゲーター` セクション内に `AIナビブリーフモデル` と `AIナビブリーフ fallback` を追加する。定時生成と `今すぐ生成` はどちらも同じ brief 用設定を参照する。

## 目的

- `AIナビゲーター` と `AIナビブリーフ` のモデル選定を独立させる
- AIナビブリーフだけ、重いモデルや別 provider を使えるようにする
- settings 保存構造を既存の `llm_models` パターンに揃えたまま拡張する

## スコープ

### 含む

- settings 保存キーの追加
  - `ai_navigator_brief`
  - `ai_navigator_brief_fallback`
- settings API request / response への追加
- settings 画面への model select 追加
- AIナビブリーフ生成時の model resolve の切り替え
- 定時生成 / 手動生成の両方への反映
- 回帰テストの追加

### 含まない

- AIナビゲーター本体の model resolve 変更
- AIナビブリーフの persona 設定変更
- 時間帯ごとの個別 model 設定
- migration による既存 `navigator` 値のコピー

## 現状

- `navigator` と `navigator_fallback` は settings に保存されている
- `AIナビブリーフ` は `resolveAINavigatorBriefModel(...)` で
  - `settings.NavigatorModel`
  - `settings.NavigatorFallbackModel`
  - default summary model
  の順に参照している
- そのため、AIナビゲーターの model を変えると AIナビブリーフも同時に変わる

## アプローチ比較

### 案1: brief 用 settings キーを追加して完全分離

- `ai_navigator_brief`
- `ai_navigator_brief_fallback`

長所:
- `navigator` と対称で分かりやすい
- API / web / service の差分が素直
- 将来の usage / 障害切り分けでも追いやすい

短所:
- settings 項目が 2 つ増える

### 案2: navigator 設定のネスト配下に brief 設定を持つ

長所:
- 概念上はまとまりがある

短所:
- 現在の `llm_models` 保存構造から外れる
- API と web の差分が大きくなる

### 案3: brief 未設定時だけ navigator を暗黙利用

長所:
- 後方互換は強い

短所:
- 「分離したつもり」で実際には共有が残る
- 挙動が分かりづらい

### 採用: 案1

brief 用 settings キーを独立追加し、`AIナビブリーフ` は brief 用設定だけを見る。

## 設定設計

### 保存キー

- `navigator`
- `navigator_fallback`
- `ai_navigator_brief`
- `ai_navigator_brief_fallback`

`navigator` 系は AIナビゲーター本体専用、`ai_navigator_brief` 系は AIナビブリーフ専用とする。

### 未設定時の扱い

- `ai_navigator_brief` が未設定なら、既存の default summary model 解決へ落とす
- `ai_navigator_brief_fallback` が未設定なら、brief 側 fallback なしとして扱う
- `navigator` / `navigator_fallback` への暗黙 fallback はしない

## API / モデル変更

### API request

settings 更新 request に追加:

- `ai_navigator_brief`
- `ai_navigator_brief_fallback`

### API response

settings 取得 response の `llm_models` に追加:

- `ai_navigator_brief`
- `ai_navigator_brief_fallback`

### サーバー内部モデル

`UserSettings` 相当の設定構造体に追加:

- `AINavigatorBriefModel`
- `AINavigatorBriefFallbackModel`

`SettingsService.UpdateLLMModels(...)` の正規化対象にも同じ 2 キーを追加する。

## AIナビブリーフ生成側

### 参照順

`resolveAINavigatorBriefModel(...)` は次の順で model を決める。

1. `settings.AINavigatorBriefModel`
2. `settings.AINavigatorBriefFallbackModel`
3. default summary model

fallback 実行時の別解決関数も、`navigator` 系ではなく `ai_navigator_brief` 系だけを見る。

### 適用箇所

- 定時生成
- `今すぐ生成`
- queued brief の run 実行

どの経路でも同じ resolve ロジックを使う。

## UI 設計

settings の `AIナビゲーター` セクション内に追加する。

- `AIナビゲーターモデル`
- `AIナビゲーター fallback`
- `AIナビブリーフモデル`
- `AIナビブリーフ fallback`

UI 上は別セクション化せず、AIナビゲーター設定の下に brief 用 2 項目を並べる。

## エラーハンドリング

- brief 用 model が catalog にない場合は保存時の既存正規化に従う
- brief 用 model が未設定でも `navigator` 系へは落とさない
- brief 用 fallback 未設定時は primary だけで生成する

## テスト

### API / service

- settings load で `ai_navigator_brief` 系が返る
- settings update で `ai_navigator_brief` 系が保存される
- brief model resolve が `navigator` 系ではなく brief 系を優先する
- brief fallback resolve が brief 専用 key を使う

### web

- settings 初期表示で brief 用 model 値が反映される
- save payload に brief 用 2 キーが含まれる

## 影響範囲

- `api/internal/model/model.go`
- `api/internal/repository/user_settings.go`
- `api/internal/service/settings_service.go`
- `api/internal/service/ai_navigator_briefs.go`
- `api/internal/handler/settings.go`
- `web/src/lib/api.ts`
- `web/src/app/(main)/settings/page.tsx`

## 期待される結果

- AIナビゲーターは従来どおり独自の model / fallback を使う
- AIナビブリーフは別の model / fallback を使える
- AIナビゲーター設定を変えても AIナビブリーフの生成モデルは自動では変わらない
