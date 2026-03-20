# Sifto 記事一覧デザインシステム Phase 1 実装計画

> **For agentic workers:** REQUIRED: Use superpowers:subagent-driven-development (if subagents available) or superpowers:executing-plans to implement this plan. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** `/items` をニュースルーム寄りの高密度 UI に刷新しつつ、既存の FeedTabs、Filters、検索、Bulk Action、日付グルーピング、Pagination、InlineReader を落とさずに共有デザインシステムへ移行する。

**Architecture:** まず `web/src/app/globals.css` に token を追加し、記事一覧専用に閉じない shared primitive を `web/src/components/ui/` 配下へ導入する。その上で `/items` の page container を `PageHeader + SummaryStrip + FilterBar + DenseArticleList + InlineReaderLayer` に組み替え、既存の query / mutation / i18n ロジックは `web/src/app/(main)/items/page.tsx` に残したまま presentation を差し替える。

**Tech Stack:** Next.js App Router, React Query, Tailwind v4 `@theme inline`, existing i18n dictionaries, `make web-lint`, `make web-build`

---

## Chunk 1: デザイントークンと共有 Primitive の土台

### Task 1: 画面横断 token を `globals.css` に追加

**Files:**
- Modify: `web/src/app/globals.css`
- Test: `make web-build`

- [ ] 既存 token を壊さずに、記事一覧 Phase 1 で使う `background / panel / panel-muted / border-subtle / text-strong / text-muted / accent / semantic / radius / shadow / motion` を追加する
- [ ] 既存の `--background`, `--panel`, `--shadow-card` などと競合しないよう命名を整理する
- [ ] light editorial ベースに限定し、dark-first な token は追加しない
- [ ] 既存コンポーネントが即座に壊れないよう、段階的に参照できる token 層として置く
- [ ] Run: `make web-build`
- [ ] Expected: web build passes with no new CSS/token errors

### Task 2: 共通 surface / header primitive を追加

**Files:**
- Create: `web/src/components/ui/page-header.tsx`
- Create: `web/src/components/ui/section-card.tsx`
- Create: `web/src/components/ui/summary-metric-card.tsx`
- Create: `web/src/components/ui/summary-strip.tsx`
- Test: `make web-build`

- [ ] `PageHeader` を追加し、title、description、meta、actions slot を受け取れるようにする
- [ ] `PageHeader` の `default / compact` state を明示し、`/items` では compact に寄せても責務が崩れないようにする
- [ ] `SectionCard` を追加し、header / body / footer 構造を再利用できるようにする
- [ ] `SummaryMetricCard` を追加し、label / value / hint 程度の短い指標表示に責務を絞る
- [ ] `SummaryStrip` を追加し、metric card 群のレイアウトだけを担当させる
- [ ] presentation only に留め、query や i18n lookup は呼び出し元で行う
- [ ] Run: `make web-build`
- [ ] Expected: new primitive files compile and are importable

### Task 3: list / state primitive を追加

**Files:**
- Create: `web/src/components/ui/filter-bar.tsx`
- Create: `web/src/components/ui/status-pill.tsx`
- Create: `web/src/components/ui/score-badge.tsx`
- Create: `web/src/components/ui/tag.tsx`
- Create: `web/src/components/ui/action-row.tsx`
- Create: `web/src/components/ui/skeleton-list.tsx`
- Create: `web/src/components/ui/error-state.tsx`
- Modify: `web/src/components/empty-state.tsx`
- Modify: `web/src/components/skeleton.tsx`
- Test: `make web-build`

- [ ] `FilterBar` を `leading / filters / sort / actions` で組める shared shell として追加する
- [ ] `FilterBar` の sticky state と、active filter summary を bar 配下で扱える構造を明示する
- [ ] `StatusPill`, `ScoreBadge`, `Tag`, `ActionRow` を高密度 row 用 primitive として追加する
- [ ] `SummaryMetricCard`, `StatusPill`, `ScoreBadge`, `Tag`, `ActionRow`, `SkeletonList`, `ErrorState` それぞれの入力と state surface をファイル冒頭コメントか props で固定する
- [ ] `SkeletonList` と `ErrorState` は `components/ui/` 配下の shared primitive として追加し、既存 `skeleton.tsx` / `empty-state.tsx` は必要なら wrapper として薄く保つ
- [ ] `EmptyState` と `SkeletonItemRow` は必要なら editorial token に寄せて、新 primitive と重複責務を持たないよう整理する
- [ ] state 表現は color-only にせず、border / label / weight も使える構造にする
- [ ] Run: `make web-build`
- [ ] Expected: `/items` 未適用でも shared primitive 層がビルド可能

## Chunk 2: 記事一覧コンテナを新しい page structure に載せ替える

### Task 4: 記事一覧の上部構造を `PageHeader + SummaryStrip + FilterBar` に再編する

**Files:**
- Modify: `web/src/app/(main)/items/page.tsx`
- Modify: `web/src/components/items/feed-tabs.tsx`
- Modify: `web/src/components/items/filters-bar.tsx`
- Modify: `web/src/i18n/dictionaries/ja.ts`
- Modify: `web/src/i18n/dictionaries/en.ts`
- Test: `make web-build`

- [ ] 現在の `/items` ヘッダーを `PageHeader` ベースへ差し替える
- [ ] 既存データから `SummaryStrip` 用の compact metrics を計算し、page.tsx 直書きではなく page-local composition として出す
- [ ] `FeedTabs` は `FilterBar.leading` に入れて、grid 固定感を弱めつつ高密度に保つ
- [ ] `FiltersBar` は sort / favorite / topic / active filter summary を shared `FilterBar` に合わせて再配置する
- [ ] active filter summary は `FilterBar` surface 内の下段として扱い、ownership は page container に残す
- [ ] 検索ボタン、bulk action、active filter badges を `FilterBar.actions` または bar 下段へ整理する
- [ ] 新しい文言を追加した場合は `ja.ts` と `en.ts` の両方を更新する
- [ ] Run: `make web-build`
- [ ] Expected: `/items` が新しい header / summary / filter 構造で描画される

### Task 5: summary / list composition を page-local helper に分ける

**Files:**
- Create or Modify: `web/src/components/items/items-summary-strip.tsx`
- Create or Modify: `web/src/components/items/items-list-state.tsx`
- Create or Modify: `web/src/components/items/dense-article-list.tsx`
- Modify: `web/src/app/(main)/items/page.tsx`
- Test: `make web-build`

- [ ] `SummaryStrip` に流し込む記事一覧用 metrics を `items-summary-strip.tsx` へ分離する
- [ ] loading / empty / no-results / error 分岐を `items-list-state.tsx` のような page-local helper に寄せる
- [ ] 日付グルーピング、row 配列、pagination 直前までの list rendering は `dense-article-list.tsx` に寄せる
- [ ] `page.tsx` は query / mutation / navigation orchestration を主責務に戻す
- [ ] Run: `make web-build`
- [ ] Expected: `page.tsx` に summary / state 表示ロジックがべったり残らない

### Task 6: list-level state を spec 通りに分解する

**Files:**
- Modify: `web/src/app/(main)/items/page.tsx`
- Modify: `web/src/components/items/items-list-state.tsx`
- Modify: `web/src/components/ui/error-state.tsx`
- Modify: `web/src/components/ui/skeleton-list.tsx`
- Modify: `web/src/components/pagination.tsx`
- Test: `make web-build`

- [ ] initial loading は skeleton list を維持しつつ page framing を崩さない
- [ ] no-data と no-results を分け、no-results では filter reset 導線を必ず出す
- [ ] page error / permission denied / offline / rate limited / stale query / pagination failure の出し分けを page container に寄せる
- [ ] retryable な page-error には `ErrorState` を使い、`/items` 固有文言と retry action を page container から渡す
- [ ] date grouping header は `DenseArticleList` の一部として見た目を統一する
- [ ] pagination failure 時に current page list を即消ししない設計を反映する
- [ ] Pagination の見た目を editorial token に揃える
- [ ] Run: `make web-build`
- [ ] Expected: loading / empty / error / pagination states that already exist still render under the new layout

## Chunk 3: item row と inline reader の presentation を差し替える

### Task 7: `ItemCard` を shared primitive ベースへ寄せる

**Files:**
- Create or Modify: `web/src/components/ui/list-row-card.tsx`
- Modify: `web/src/components/items/item-card.tsx`
- Modify: `web/src/components/score-indicator.tsx`
- Modify: `web/src/components/items/check-status-badges.tsx`
- Modify: `web/src/components/thumbnail.tsx`
- Test: `make web-build`

- [ ] `ListRowCard` を shared row primitive として追加し、`ItemCard` は Phase 1 ではその adapter / article-specific wrapper に寄せる
- [ ] `ItemCard` の `zinc` 直書き中心の class 構成を token / primitive ベースへ寄せる
- [ ] row anatomy を `thumbnail -> title / key point -> source/date/state -> score/reaction -> actions` に揃える
- [ ] read / unread / pending / failed / featured の state を shared primitive で一貫化する
- [ ] URL や補助情報は title より弱く見せる
- [ ] mobile で action row が横幅を壊さないことを優先する
- [ ] Run: `make web-build`
- [ ] Expected: card states compile and preserve current item actions / handlers

### Task 8: `InlineReader` を page-level overlay layer として揃える

**Files:**
- Modify: `web/src/components/inline-reader.tsx`
- Modify: `web/src/app/(main)/items/page.tsx`
- Test: `make web-build`

- [ ] `InlineReader` を row 内 panel ではなく page-level overlay layer として扱う前提で見た目を整理する
- [ ] open 時に reader 先頭へ focus、close 時に元 row または近傍 row へ戻す設計を崩さない
- [ ] loading / error / read toggle の見た目を item list と同じ token 言語へ寄せる
- [ ] `onOpenDetail`, `onReadToggled`, queue navigation の既存責務は維持する
- [ ] Run: `make web-build`
- [ ] Expected: inline reader opens/closes under the new visual system without breaking list navigation

## Chunk 4: 既存 mutation UX と i18n を仕上げる

### Task 9: retry / read toggle / bulk action / search modal の状態表現を揃える

**Files:**
- Modify: `web/src/app/(main)/items/page.tsx`
- Modify: `web/src/components/items/item-card.tsx`
- Modify: `web/src/components/inline-reader.tsx`
- Modify: `web/src/i18n/dictionaries/ja.ts`
- Modify: `web/src/i18n/dictionaries/en.ts`
- Test: `make web-build`

- [ ] retry 中の disabled state、success toast、error feedback を新 UI に合わせて崩れないようにする
- [ ] mark read / unread の optimistic update と rollback を視覚的に分かるようにする
- [ ] bulk action 実行中の disabled state を toolbar で統一する
- [ ] search modal の trigger / initial focus / close focus return を維持する
- [ ] 必要な追加文言を `ja.ts` と `en.ts` に揃えて入れる
- [ ] Run: `make web-build`
- [ ] Expected: mutation-related UI states remain understandable after the redesign

## Chunk 5: 検証と仕上げ

### Task 10: lint / build / manual checks で Phase 1 を確定する

**Files:**
- Modify: implementation files only if verification uncovered defects

- [ ] Run: `make web-lint`
- [ ] Expected: no new ESLint errors or warnings introduced by this work
- [ ] Run: `make web-build`
- [ ] Expected: production build passes
- [ ] Manual check: `/items` で feed tabs、filter、search、bulk action、date grouping、pagination が表示される
- [ ] Manual check: unread/read/pending/failed/featured card states が崩れない
- [ ] Manual check: no-results / page-error / offline 相当 state の見え方が新 layout で崩れない
- [ ] Manual check: `InlineReader` の open / close / detail 遷移 / read toggle が動く
- [ ] Manual check: mobile 幅で action row と filter bar が横にはみ出さない
- [ ] Manual check: keyboard だけで row navigation、主要 action、search modal、inline reader close まで辿れる
- [ ] Manual check: focus ring が light surface 上で十分見える
- [ ] Manual check: reduced motion 環境でも list / overlay の挙動が破綻しない
- [ ] Manual check: 追加した文言が `ja` / `en` の両方で表示できる

Plan complete and saved to `docs/superpowers/plans/2026-03-20-sifto-article-list-design-system-phase1.md`. Ready to execute?
