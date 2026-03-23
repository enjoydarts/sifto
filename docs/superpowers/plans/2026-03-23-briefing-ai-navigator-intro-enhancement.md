# Briefing AI Navigator Intro Enhancement Implementation Plan

> **For agentic workers:** REQUIRED: Use superpowers:subagent-driven-development (if subagents available) or superpowers:executing-plans to implement this plan. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** AIナビゲーターの導入文を 2〜3文の自然なトークに強化し、時間帯・曜日・季節感を踏まえたキャラらしい導入からおすすめ記事へつなげられるようにする。

**Architecture:** 既存の `navigator.intro` string は維持し、worker 側の prompt / schema 制約を強化して導入文の質を上げる。API 側で JST ベースの時間帯・曜日・季節ヒントを組み立てて worker に渡し、web は既存 UI を保ったまま長めの導入文を読みやすく見せる。

**Tech Stack:** Go, Next.js App Router, Python worker, existing structured-output LLM adapters, Docker Compose, make

---

## File Map

- Modify: `api/internal/service/worker.go`
  - navigator 生成リクエストに時間文脈を渡す
- Modify: `api/internal/handler/briefing.go`
  - JST 時刻から navigator intro 用の context を組み立てる
- Modify: `worker/app/routers/briefing_navigator.py`
  - 拡張された request schema を受ける
- Modify: `worker/app/services/feed_task_common.py`
  - 2〜3文 intro 制約、時間帯・季節ヒント、persona 別導入トーンを prompt に反映
- Modify: `worker/app/services/test_summary_task_common.py`
  - 既存共通 prompt テストに触れないことを確認する場合のみ
- Modify: `worker/app/services/test_feed_task_common.py`
  - navigator prompt と parse の回帰テストを追加
- Modify: `web/src/app/(main)/page.tsx`
  - 長め intro の見え方を微調整し、preview 表示と干渉しないことを確認
- Optional Modify: `web/src/i18n/dictionaries/ja.ts`
  - 追加 UI 文言が必要になった場合のみ
- Optional Modify: `web/src/i18n/dictionaries/en.ts`
  - 追加 UI 文言が必要になった場合のみ

## Chunk 1: API Context Plumbing

### Task 1: Add navigator intro context to worker request

**Files:**
- Modify: `api/internal/service/worker.go`
- Modify: `api/internal/handler/briefing.go`
- Test: `api/internal/handler/briefing_test.go`

- [ ] **Step 1: Write the failing test**

Add a test in `api/internal/handler/briefing_test.go` that builds navigator context for a fixed JST timestamp and asserts:

```go
func TestBuildBriefingNavigatorIntroContext(t *testing.T) {
    now := time.Date(2026, 3, 23, 19, 30, 0, 0, time.FixedZone("JST", 9*60*60))
    got := buildBriefingNavigatorIntroContext(now)

    if got.TimeOfDay != "evening" {
        t.Fatalf("time_of_day = %q", got.TimeOfDay)
    }
    if got.WeekdayJST != "Monday" && got.WeekdayJST != "月曜日" {
        t.Fatalf("weekday_jst = %q", got.WeekdayJST)
    }
    if got.SeasonHint == "" {
        t.Fatal("season_hint is empty")
    }
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `docker compose exec -T api go test ./internal/handler -run TestBuildBriefingNavigatorIntroContext -v`

Expected: FAIL because helper / struct does not exist yet.

- [ ] **Step 3: Write minimal implementation**

In `api/internal/handler/briefing.go`:

- add a small struct such as:

```go
type briefingNavigatorIntroContext struct {
    NowJST     string `json:"now_jst"`
    DateJST    string `json:"date_jst"`
    WeekdayJST string `json:"weekday_jst"`
    TimeOfDay  string `json:"time_of_day"`
    SeasonHint string `json:"season_hint"`
}
```

- add:

```go
func buildBriefingNavigatorIntroContext(now time.Time) briefingNavigatorIntroContext
```

- map hour ranges to:
  - `morning`
  - `afternoon`
  - `evening`
  - `late_night`

- map month/day roughly to season hints such as:
  - `early_spring`
  - `spring`
  - `rainy_season`
  - `mid_summer`
  - `late_summer`
  - `autumn`
  - `early_winter`
  - `mid_winter`

- extend `service.BriefingNavigatorCandidate` request path in `api/internal/service/worker.go` with a new `IntroContext` payload field used only by the navigator worker call.

- [ ] **Step 4: Thread the new context through navigator generation**

In `buildNavigator(...)` inside `api/internal/handler/briefing.go`, compute:

```go
introContext := buildBriefingNavigatorIntroContext(generatedAt)
```

and pass it into `GenerateBriefingNavigatorWithModel(...)`.

Do the same for `buildNavigatorPreview(...)` only if preview output needs persona-consistent intro generation. If preview keeps local strings, no change is needed there.

- [ ] **Step 5: Run tests to verify they pass**

Run: `docker compose exec -T api go test ./internal/handler ./internal/service -v`

Expected: PASS

- [ ] **Step 6: Commit**

```bash
git add api/internal/handler/briefing.go api/internal/service/worker.go api/internal/handler/briefing_test.go
git commit -m "AIナビゲーター導入トーク用の時間文脈を追加"
```

## Chunk 2: Worker Prompt Upgrade

### Task 2: Expand briefing navigator schema and prompt rules

**Files:**
- Modify: `worker/app/routers/briefing_navigator.py`
- Modify: `worker/app/services/feed_task_common.py`
- Test: `worker/app/services/test_feed_task_common.py`

- [ ] **Step 1: Write the failing test**

Add tests in `worker/app/services/test_feed_task_common.py` for:

```python
def test_build_briefing_navigator_task_includes_intro_structure_rules():
    task = build_briefing_navigator_task(
        persona="editor",
        candidates=[...],
        intro_context={
            "now_jst": "2026-03-23T19:30:00+09:00",
            "date_jst": "2026-03-23",
            "weekday_jst": "Monday",
            "time_of_day": "evening",
            "season_hint": "early_spring",
        },
    )
    prompt = task["prompt"]
    assert "2〜3文" in prompt or "2-3 sentences" in prompt
    assert "time_of_day" in prompt or "時間帯" in prompt
    assert "不確かな記念日を断定しない" in prompt
```

Also add a test that persona `snark` keeps safety language while still allowing light humor.

- [ ] **Step 2: Run test to verify it fails**

Run: `docker compose exec -T worker python -m unittest app.services.test_feed_task_common -v`

Expected: FAIL because `intro_context` is not accepted or prompt text is missing.

- [ ] **Step 3: Implement minimal schema changes**

In `worker/app/routers/briefing_navigator.py`:

- extend request model with:

```python
class BriefingNavigatorIntroContext(BaseModel):
    now_jst: str
    date_jst: str
    weekday_jst: str
    time_of_day: str
    season_hint: str
```

- include `intro_context` in `BriefingNavigatorRequest`

In `worker/app/services/feed_task_common.py`:

- change `build_briefing_navigator_task(...)` signature to accept `intro_context`
- inject `intro_context` into the prompt
- strengthen intro rules:
  - 2〜3文
  - first sentence greeting
  - second sentence seasonal/date/time small talk
  - final sentence bridge to picks
  - do not assert uncertain commemorative days or holidays

- [ ] **Step 4: Add persona-specific intro guidance**

In the same prompt builder:

- `editor`: calm, concise bridge
- `hype`: lively greeting and momentum
- `analyst`: slightly more contextual framing
- `concierge`: soft lifestyle-like small talk
- `snark`: dry humor allowed, contempt forbidden

Keep picks generation rules unchanged except where intro framing references them.

- [ ] **Step 5: Run tests to verify they pass**

Run:

- `docker compose exec -T worker python -m unittest app.services.test_feed_task_common -v`
- `make check-worker`

Expected: PASS

- [ ] **Step 6: Commit**

```bash
git add worker/app/routers/briefing_navigator.py worker/app/services/feed_task_common.py worker/app/services/test_feed_task_common.py
git commit -m "AIナビゲーターの導入トーク prompt を強化"
```

## Chunk 3: Integration Verification and UI Readability

### Task 3: Verify intro rendering and adjust briefing overlay if needed

**Files:**
- Modify: `web/src/app/(main)/page.tsx`
- Test: none or existing build verification only

- [ ] **Step 1: Inspect current intro rendering**

Check the current intro bubble in `web/src/app/(main)/page.tsx` and verify whether 2〜3文でも読みにくくならないか確認する.

- [ ] **Step 2: Apply minimal UI adjustment only if needed**

If readability is weak, make minimal changes such as:

- slightly larger line-height for intro paragraph
- stronger max width usage within the bubble
- small spacing adjustment between intro bubble and picks list

Do not add a new visual sub-panel in this task.

- [ ] **Step 3: Verify preview mode still works**

Run:

- `docker compose exec -T web sh -lc "wget -qO- 'http://api:8080/api/briefing/today?size=18&navigator_preview=1&cache_bust=1' | head -c 4000"`

Expected: JSON contains `navigator.intro` and `navigator.picks`.

- [ ] **Step 4: Run build verification**

Run:

- `docker compose exec -T api go test ./internal/handler ./internal/service ./internal/repository`
- `make web-build`

Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add web/src/app/(main)/page.tsx api/internal/handler/briefing.go api/internal/service/worker.go worker/app/routers/briefing_navigator.py worker/app/services/feed_task_common.py worker/app/services/test_feed_task_common.py
git commit -m "AIナビゲーターの導入トーク表示を強化"
```

## Final Verification

- [ ] Run: `docker compose exec -T api go test ./internal/handler ./internal/repository ./internal/service`
- [ ] Run: `make check-worker`
- [ ] Run: `make web-build`
- [ ] Confirm:
  - `navigator_preview=1&cache_bust=1` で常に preview が返る
  - 実生成時の `navigator.intro` が 2〜3文になっている
  - persona 切替で導入トークの tone が変わる
  - 既存の picks 表示と click 動作を壊していない

