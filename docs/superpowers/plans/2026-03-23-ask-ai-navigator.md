# Ask AI Navigator Implementation Plan

> **For agentic workers:** REQUIRED: Use superpowers:subagent-driven-development (if subagents available) or superpowers:executing-plans to implement this plan. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Ask の回答直後に、AI ナビゲーターが前提のズレ・留保・次に掘る論点を 5〜8 文で返す機能を追加する。

**Architecture:** `POST /api/ask` はそのまま維持し、`POST /api/ask/navigator` を別 endpoint として追加する。API 側で `query + answer + persona + model` をキーに 30 分キャッシュし、worker には ask navigator 専用 task を追加する。web は Ask 回答取得後に自動で navigator を fetch し、回答直下のカードとして表示する。

**Tech Stack:** Go API, FastAPI worker, Next.js App Router, React Query, existing AI navigator persona/settings flow, PostgreSQL migration

---

## File Map

- Modify: `api/internal/model/model.go`
  - Ask navigator request/response model を追加する
- Modify: `api/internal/service/worker.go`
  - worker の `/ask-navigator` 呼び出しを追加する
- Modify: `api/internal/handler/cache_keys.go`
  - ask navigator 用 cache key を追加する
- Modify: `api/internal/handler/ask.go`
  - `POST /api/ask/navigator` と build/cache/usage 記録を追加する
- Modify: `api/cmd/server/main.go`
  - route 登録
- Create: `db/migrations/000085_allow_ask_navigator_purpose_in_llm_usage_logs.up.sql`
- Create: `db/migrations/000085_allow_ask_navigator_purpose_in_llm_usage_logs.down.sql`
- Modify: `worker/app/main.py`
  - router 登録
- Create: `worker/app/routers/ask_navigator.py`
  - request/response schema と provider dispatch
- Modify: `worker/app/services/feed_task_common.py`
  - ask navigator schema / task / parser 追加
- Modify: `worker/app/services/{claude,openai,gemini,groq,deepseek,alibaba,mistral,fireworks,openrouter,poe,xai,zai}_service.py`
  - `generate_ask_navigator()` 追加
- Modify: `worker/app/services/test_feed_task_common.py`
  - ask navigator prompt/schema テスト追加
- Modify: `web/src/lib/api.ts`
  - ask navigator 型と client 追加
- Modify: `web/src/app/(main)/ask/page.tsx`
  - 回答後の自動 fetch、loading/error、closeable card UI
- Modify: `web/src/i18n/dictionaries/ja.ts`
- Modify: `web/src/i18n/dictionaries/en.ts`

## Chunk 1: API Contract and Usage

### Task 1: Add ask navigator API models

**Files:**
- Modify: `api/internal/model/model.go`

- [ ] **Step 1: Add response envelope types**

Add:

```go
type AskNavigatorInput struct {
	Query        string         `json:"query"`
	Answer       string         `json:"answer"`
	Bullets      []string       `json:"bullets"`
	Citations    []AskCitation  `json:"citations"`
	RelatedItems []AskCandidate `json:"related_items"`
}

type AskNavigator struct {
	Enabled        bool      `json:"enabled"`
	Persona        string    `json:"persona"`
	CharacterName  string    `json:"character_name"`
	CharacterTitle string    `json:"character_title"`
	AvatarStyle    string    `json:"avatar_style"`
	SpeechStyle    string    `json:"speech_style"`
	Headline       string    `json:"headline"`
	Commentary     string    `json:"commentary"`
	NextAngles     []string  `json:"next_angles"`
	GeneratedAt    *time.Time `json:"generated_at,omitempty"`
}

type AskNavigatorEnvelope struct {
	Navigator *AskNavigator `json:"navigator,omitempty"`
}
```

- [ ] **Step 2: Verify field reuse**

Check that `AskCitation` / `AskCandidate` already match the worker input needs. Reuse them without introducing duplicate DTOs unless blocked.

- [ ] **Step 3: Commit**

```bash
git add api/internal/model/model.go
git commit -m "feat: add ask navigator API models"
```

### Task 2: Add worker client and cache key

**Files:**
- Modify: `api/internal/service/worker.go`
- Modify: `api/internal/handler/cache_keys.go`

- [ ] **Step 1: Add worker request/response structs**

In `worker.go`, add:

```go
type AskNavigatorInput struct {
	Query        string         `json:"query"`
	Answer       string         `json:"answer"`
	Bullets      []string       `json:"bullets"`
	Citations    []AskCitation  `json:"citations"`
	RelatedItems []AskCandidate `json:"related_items"`
}

type AskNavigatorResponse struct {
	Headline   string    `json:"headline"`
	Commentary string    `json:"commentary"`
	NextAngles []string  `json:"next_angles"`
	LLM        *LLMUsage `json:"llm,omitempty"`
}
```

- [ ] **Step 2: Add worker client method**

Add:

```go
func (w *WorkerClient) GenerateAskNavigatorWithModel(
	ctx context.Context,
	persona string,
	input AskNavigatorInput,
	anthropicAPIKey, googleAPIKey, groqAPIKey, deepseekAPIKey, alibabaAPIKey, mistralAPIKey, xaiAPIKey, zaiAPIKey, fireworksAPIKey, openAIAPIKey string,
	model *string,
) (*AskNavigatorResponse, error)
```

Use `/ask-navigator` and include internal secret plus provider/model headers exactly like source/item navigator.

- [ ] **Step 3: Add ask navigator cache key**

In `cache_keys.go`, add:

```go
func cacheKeyAskNavigator(userID, query, answer, persona, model string) string
```

Hash `query + answer` and include `persona` and `model`.

- [ ] **Step 4: Run focused Go tests**

Run:

```bash
docker compose exec -T api go test ./internal/service ./internal/handler ./internal/repository
```

Expected: build passes even before handler wiring.

- [ ] **Step 5: Commit**

```bash
git add api/internal/service/worker.go api/internal/handler/cache_keys.go
git commit -m "feat: add ask navigator worker contract"
```

### Task 3: Add ask navigator endpoint and usage accounting

**Files:**
- Modify: `api/internal/handler/ask.go`
- Modify: `api/cmd/server/main.go`
- Create: `db/migrations/000085_allow_ask_navigator_purpose_in_llm_usage_logs.up.sql`
- Create: `db/migrations/000085_allow_ask_navigator_purpose_in_llm_usage_logs.down.sql`

- [ ] **Step 1: Add migration for new purpose**

Add `ask_navigator` to `llm_usage_logs_purpose_check`.

- [ ] **Step 2: Add request parsing and empty guard**

In `ask.go`, add a new handler:

```go
func (h *AskHandler) Navigator(w http.ResponseWriter, r *http.Request)
```

Behavior:
- require auth
- load settings
- if navigator disabled: return empty envelope
- parse body into `model.AskNavigatorInput`
- if query/answer empty: return empty envelope

- [ ] **Step 3: Add cache behavior**

Use:
- persona from existing navigator settings
- model from existing navigator model resolution
- `cacheKeyAskNavigator(...)`
- 30 minute TTL
- do not cache `navigator == nil`

- [ ] **Step 4: Add worker call**

Map body into `service.AskNavigatorInput`, decrypt provider keys exactly like other navigator endpoints, then call `GenerateAskNavigatorWithModel(...)`.

On success:
- map `headline/commentary/next_angles`
- attach persona meta from existing helper
- call `recordAskLLMUsage(..., "ask_navigator", ...)`

- [ ] **Step 5: Register route**

In `main.go`:

```go
r.Post("/ask/navigator", askH.Navigator)
```

- [ ] **Step 6: Run migration and API tests**

Run:

```bash
make migrate-up
docker compose exec -T api go test ./internal/handler ./internal/service ./internal/repository
make migrate-version
```

Expected:
- tests pass
- migration version increments to `85`

- [ ] **Step 7: Commit**

```bash
git add api/internal/handler/ask.go api/cmd/server/main.go db/migrations/000085_allow_ask_navigator_purpose_in_llm_usage_logs.up.sql db/migrations/000085_allow_ask_navigator_purpose_in_llm_usage_logs.down.sql
git commit -m "feat: add ask navigator API endpoint"
```

## Chunk 2: Worker Prompt and Routing

### Task 4: Add ask navigator schema, task, and parser

**Files:**
- Modify: `worker/app/services/feed_task_common.py`
- Test: `worker/app/services/test_feed_task_common.py`

- [ ] **Step 1: Add strict schema**

Add:

```python
ASK_NAVIGATOR_SCHEMA = {
    "type": "object",
    "properties": {
        "headline": {"type": "string"},
        "commentary": {"type": "string"},
        "next_angles": {"type": "array", "items": {"type": "string"}},
    },
    "required": ["headline", "commentary", "next_angles"],
    "additionalProperties": False,
}
```

- [ ] **Step 2: Add prompt builder**

Add:

```python
def build_ask_navigator_task(persona: str, ask_input: dict) -> dict:
```

Prompt rules:
- commentary 5〜8文
- answer の再要約禁止
- 前提のズレ / 留保 / 次の論点 を重視
- `next_angles` は 2〜4件
- persona の主観で語る

- [ ] **Step 3: Add parser**

Add:

```python
def parse_ask_navigator_result(text: str, ask_input: dict) -> dict:
```

Fallback should build:
- `headline`
- `commentary`
- `next_angles`

without hallucinating unsupported claims.

- [ ] **Step 4: Add tests**

Add tests asserting:
- schema required fields
- prompt includes `再要約禁止`
- prompt includes `前提`, `留保`, `次に掘る論点`
- prompt includes `next_angles`

- [ ] **Step 5: Run worker unit tests**

Run:

```bash
docker compose exec -T worker python -m unittest app.services.test_feed_task_common
make check-worker
```

- [ ] **Step 6: Commit**

```bash
git add worker/app/services/feed_task_common.py worker/app/services/test_feed_task_common.py
git commit -m "feat: add ask navigator worker task"
```

### Task 5: Add ask navigator router and provider wrappers

**Files:**
- Create: `worker/app/routers/ask_navigator.py`
- Modify: `worker/app/main.py`
- Modify: `worker/app/services/{claude,openai,gemini,groq,deepseek,alibaba,mistral,fireworks,openrouter,poe,xai,zai}_service.py`

- [ ] **Step 1: Add router**

Create `ask_navigator.py` mirroring item/source navigator:
- request model with `persona`, `query`, `answer`, `bullets`, `citations`, `related_items`, `model`
- response model with `headline`, `commentary`, `next_angles`, `llm`
- `POST /ask-navigator`

- [ ] **Step 2: Register router**

In `worker/app/main.py`:

```python
from app.routers import ..., ask_navigator, ...
app.include_router(ask_navigator.router)
```

- [ ] **Step 3: Add provider wrappers**

For each provider service, add:

```python
def generate_ask_navigator(persona: str, ask_input: dict, model: str, api_key: str) -> dict:
```

or the anthropic equivalent with optional key/model signature.

Reuse:
- `build_ask_navigator_task`
- `parse_ask_navigator_result`
- provider-specific `_chat_json` / `_generate_content`

- [ ] **Step 4: Run worker verification**

Run:

```bash
make check-worker
docker compose exec -T worker python -m unittest app.services.test_feed_task_common
```

- [ ] **Step 5: Commit**

```bash
git add worker/app/routers/ask_navigator.py worker/app/main.py worker/app/services/*.py
git commit -m "feat: add ask navigator worker endpoint"
```

## Chunk 3: Web UI

### Task 6: Add API client types and method

**Files:**
- Modify: `web/src/lib/api.ts`

- [ ] **Step 1: Add response types**

Add:

```ts
export interface AskNavigatorResponse {
  navigator?: {
    enabled: boolean;
    persona: string;
    character_name: string;
    character_title: string;
    avatar_style: string;
    speech_style: string;
    headline: string;
    commentary: string;
    next_angles: string[];
    generated_at?: string | null;
  } | null;
}
```

- [ ] **Step 2: Add API method**

Add:

```ts
getAskNavigator: (body: {
  query: string;
  answer: string;
  bullets?: string[];
  citations?: AskCitation[];
  related_items?: AskCandidate[];
}) => apiFetch<AskNavigatorResponse>("/ask/navigator", { method: "POST", body: JSON.stringify(body) })
```

- [ ] **Step 3: Commit**

```bash
git add web/src/lib/api.ts
git commit -m "feat: add ask navigator web client"
```

### Task 7: Render ask navigator below answer

**Files:**
- Modify: `web/src/app/(main)/ask/page.tsx`
- Modify: `web/src/i18n/dictionaries/ja.ts`
- Modify: `web/src/i18n/dictionaries/en.ts`

- [ ] **Step 1: Add state**

Add state for:
- `askNavigator`
- `askNavigatorLoading`
- `askNavigatorError`
- `askNavigatorDismissed`

- [ ] **Step 2: Trigger auto-generation**

After a successful Ask response:
- reset navigator state
- call `api.getAskNavigator(...)`
- if navigator returns, show it automatically

Do not block Ask answer rendering on navigator fetch.

- [ ] **Step 3: Render card**

Below the Ask answer card, render:
- avatar
- name/title
- headline
- commentary
- `next_angles[]`
- close button

The card should visually reuse the existing navigator tone, but live inline with Ask output instead of bottom-right fixed positioning.

- [ ] **Step 4: Add loading and error states**

While fetching:
- show avatar + loading text

On failure:
- show lightweight error copy, not a blocking modal

- [ ] **Step 5: Add i18n strings**

Add both `ja.ts` and `en.ts` keys for:
- `ask.navigator.loading`
- `ask.navigator.error`
- `ask.navigator.close`
- `ask.navigator.nextAngles`
- any heading/subtitle text

- [ ] **Step 6: Run web build**

Run:

```bash
make web-build
```

- [ ] **Step 7: Commit**

```bash
git add web/src/app/(main)/ask/page.tsx web/src/i18n/dictionaries/ja.ts web/src/i18n/dictionaries/en.ts
git commit -m "feat: add ask navigator UI"
```

## Chunk 4: Final Verification

### Task 8: Verify end to end

**Files:**
- No new files

- [ ] **Step 1: Run full relevant checks**

Run:

```bash
make fmt-go
docker compose exec -T api go test ./internal/handler ./internal/service ./internal/repository
docker compose exec -T worker python -m unittest app.services.test_feed_task_common app.services.test_item_navigator_task_common
make check-worker
make web-build
make migrate-version
```

Expected:
- all pass
- migration version is `85`

- [ ] **Step 2: Manual behavior check**

Verify in browser:
- Ask answer appears first
- ask navigator auto-appears after answer
- close button hides it
- repeated identical query within 30 minutes does not trigger a visible long delay

- [ ] **Step 3: Commit final cleanup**

```bash
git add -A
git commit -m "feat: ship ask AI navigator"
```
