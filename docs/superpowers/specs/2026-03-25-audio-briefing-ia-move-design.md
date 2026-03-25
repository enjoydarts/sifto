# 音声ブリーフィング IA バケット移動設計

## 概要

生成から 30 日以上経過した音声ブリーフィングの実ファイルを、通常バケットから Infrequent Access 用バケットへ移動する。
移動後も詳細画面の player から即時再生できることを維持する。

対象は次の実ファイル。

- 最終エピソード音声
- concat manifest
- chunk 音声

現在は object key しか DB に保存しておらず、保存先 bucket は `AUDIO_BRIEFING_R2_BUCKET` の単一前提で固定されている。
このままでは IA 移動後に presign / delete / concat 復旧が保存先を判別できないため、保存先 bucket を DB で保持するように拡張する。

## 要件

### 機能要件

- `published_at` から 30 日以上経過した published job を IA 移動対象にする
- IA 移動後も詳細画面の再生 URL は透過的に発行される
- 削除機能は、standard / IA のどちらにあるファイルでも正しく削除できる
- 移送は再実行可能で、途中失敗しても source 側ファイルを先に消さない

### 非機能要件

- object の移送は copy 完了確認後に source delete を行う
- DB 更新は object copy 成功後、source delete 前に行わない
- 移送 batch は冪等に近い挙動を持つ
- 実行コストを抑えるため、不要な existence check を API リクエストごとに増やさない

## 設計方針

### 採用案

保存先 bucket を DB に保持する。

理由:

- presign 時に探索ロジックを持たせずに済む
- delete / future archive / restore で保存先判定が明確になる
- API の read path を軽く保てる

### 不採用案

presign 時に standard bucket と IA bucket を順に探索する方式は採用しない。

理由:

- 読み取りのたびに existence check が必要になる
- delete 時の保存先判定が曖昧になる
- 今後 bucket/class が増えた時に分岐が増える

## データモデル変更

### 追加カラム

`audio_briefing_jobs`
- `r2_storage_bucket TEXT NOT NULL DEFAULT ''`

`audio_briefing_script_chunks`
- `r2_storage_bucket TEXT NOT NULL DEFAULT ''`

### backfill

migration で既存レコードの空値を現行 `AUDIO_BRIEFING_R2_BUCKET` 相当の standard bucket 名に埋める。

実装上は migration で固定 env を読めないため、次のどちらかで対応する。

1. migration は空文字許容のまま追加し、API/worker が空文字を standard bucket fallback として扱う
2. migration 後の deploy でバックフィル用 SQL を別途実行する

今回は安全性を優先し、`空文字は standard bucket fallback` とする。

## 環境変数

### API / Worker / Cloud Run Job 共通

- `AUDIO_BRIEFING_R2_STANDARD_BUCKET`
- `AUDIO_BRIEFING_R2_IA_BUCKET`

### 互換 fallback

- `AUDIO_BRIEFING_R2_BUCKET`
  - `AUDIO_BRIEFING_R2_STANDARD_BUCKET` 未設定時の fallback

### batch 用

- `AUDIO_BRIEFING_IA_MOVE_AFTER_DAYS=30`
- `AUDIO_BRIEFING_IA_MOVE_BATCH_LIMIT`

## API / Worker 責務変更

### Worker

`audio_briefing_tts` service に bucket override を追加する。

- upload
  - 通常生成時は standard bucket に保存
- presign
  - job/chunk が持つ bucket を使って URL 発行
- delete
  - object key だけでなく bucket も受け取れるようにする
- copy
  - standard -> IA への object copy API を追加する

### API

#### 生成

- draft/chunk 保存時に `r2_storage_bucket` を standard bucket 名で持つ
- concat publish 時に job の `r2_storage_bucket` も standard bucket 名で埋める

#### 詳細表示

- job の final audio presign は `job.r2_storage_bucket` を使う

#### 削除

- job/chunk ごとの保存 bucket を使って object delete を行う

## IA 移送 batch

### 対象条件

- `status = 'published'`
- `published_at < now() - interval '30 days'`
- `job.r2_storage_bucket = standard bucket`

### 実行フロー

1. 対象 job を `limit` 件取得
2. job 本体と chunks を取得
3. 移送対象 object 一覧を構築
4. worker に `copy_objects(source_bucket=standard, target_bucket=ia, objects=...)` を依頼
5. copy 成功後、DB の `r2_storage_bucket` を IA bucket に更新
   - job
   - chunks
6. DB 更新成功後に source bucket から object delete

### 失敗時

- copy 失敗:
  - DB 更新しない
  - source delete しない
- DB 更新失敗:
  - source delete しない
  - 次回 batch で再試行可能
- source delete 失敗:
  - DB は IA bucket を向いた状態になる
  - 再生は継続可能
  - standard 側に残骸が残るだけなので、後続 cleanup job で回収可能

## 一貫性戦略

最優先は「再生不能にしない」こととする。

そのため、失敗順序に対して次を守る。

- source delete を最も後ろに置く
- DB 更新は copy 成功後にのみ行う
- DB が IA を向いた後は presign が IA を使うので、source 側残骸は致命傷ではない

## 影響範囲

### DB

- `audio_briefing_jobs`
- `audio_briefing_script_chunks`

### API

- `audio_briefing` repository
- playable audio URL 解決
- delete service
- IA move batch service / scheduler

### Worker

- R2 upload / presign / delete
- object copy endpoint

### Infra

- standard bucket / IA bucket の env
- batch 実行方法
  - 既存 Inngest 定期 function 追加が第一候補

## テスト戦略

### repository / service

- job/chunk の bucket fallback が効く
- presign が DB 保存 bucket を使う
- IA 移送 batch が `copy -> DB update -> source delete` の順で動く
- copy 失敗時に DB を更新しない
- DB 更新失敗時に source delete しない

### worker

- bucket override presign
- copy_objects
- delete_objects with bucket

### E2E

- published 後 31 日相当の job を batch 対象にする
- 移送後も detail API の `audio_url` で再生可能
- 削除が IA bucket 上の object でも成功する

## 実装順

1. DB に bucket カラム追加
2. worker の bucket override / copy API 追加
3. API の upload/presign/delete を bucket aware に変更
4. IA move batch service を追加
5. Inngest か internal trigger で定期実行
6. テストと env example 更新

## 未決事項

- source delete 失敗時の再掃除を別 job に切るか、次回 IA move batch に含めるか
- chunk 音声まで IA へ移すか
  - 現時点では削除整合性のため移す前提
