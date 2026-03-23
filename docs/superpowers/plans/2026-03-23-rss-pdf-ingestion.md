# RSS PDF Ingestion Implementation Plan

> **For agentic workers:** REQUIRED: Use superpowers:subagent-driven-development (if subagents available) or superpowers:executing-plans to implement this plan. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** RSS item URL が PDF を指す場合でも、既存の本文抽出パイプラインで本文を取り込み、後続の facts / summary に流せるようにする。

**Architecture:** worker の `/extract-body` 契約は維持し、`trafilatura_service.py` を HTML/PDF の dispatcher にする。PDF 本文抽出は `pdf_service.py` に分離し、`PyMuPDF` でテキスト抽出した結果を既存の `ExtractResponse` 形式に合わせて返す。

**Tech Stack:** FastAPI, httpx, trafilatura, PyMuPDF, docker compose, make

---

## File Map

- Create: `worker/app/services/pdf_service.py`
  - PDF 判定補助、PDF バイナリ取得、PyMuPDF での本文抽出、title 解決
- Modify: `worker/app/services/trafilatura_service.py`
  - HTML/PDF dispatcher 化、既存 HTML 抽出の維持
- Modify: `worker/app/routers/extract.py`
  - 基本は契約維持。必要なら出力メタデータの観測項目を微調整
- Modify: `worker/requirements.txt` または依存定義ファイル
  - `PyMuPDF` 追加
- Create: `worker/app/services/test_pdf_service.py`
  - PDF 判定と PDF 本文抽出のユニットテスト
- Modify: `worker/app/services/test_trafilatura_service.py` または同等の新規 test file
  - HTML/PDF 分岐の回帰テスト

## Chunk 1: PDF Extractor

### Task 1: PDF 抽出サービスの失敗テストを書く

**Files:**
- Create: `worker/app/services/test_pdf_service.py`

- [ ] **Step 1: Write the failing test**

```python
def test_extract_pdf_body_returns_text_and_metadata_title():
    pdf_bytes = build_pdf_bytes("PDF title", ["first page", "second page"])
    result = extract_pdf_body_from_bytes(pdf_bytes, "https://example.com/file.pdf")
    assert result["title"] == "PDF title"
    assert "first page" in result["content"]
    assert "second page" in result["content"]
```

- [ ] **Step 2: Run test to verify it fails**

Run: `docker compose exec -T worker python -m unittest app.services.test_pdf_service -v`
Expected: FAIL with import or missing function error

- [ ] **Step 3: Write minimal implementation**

```python
def extract_pdf_body_from_bytes(pdf_bytes: bytes, url: str) -> dict | None:
    ...
```

- [ ] **Step 4: Run test to verify it passes**

Run: `docker compose exec -T worker python -m unittest app.services.test_pdf_service -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add worker/app/services/pdf_service.py worker/app/services/test_pdf_service.py
git commit -m "PDF本文抽出サービスを追加"
```

### Task 2: 空テキスト PDF の failure を固定する

**Files:**
- Modify: `worker/app/services/test_pdf_service.py`
- Modify: `worker/app/services/pdf_service.py`

- [ ] **Step 1: Write the failing test**

```python
def test_extract_pdf_body_returns_none_for_empty_text_pdf():
    pdf_bytes = build_pdf_bytes(None, [""])
    assert extract_pdf_body_from_bytes(pdf_bytes, "https://example.com/empty.pdf") is None
```

- [ ] **Step 2: Run test to verify it fails**

Run: `docker compose exec -T worker python -m unittest app.services.test_pdf_service -v`
Expected: FAIL because empty text is still returned

- [ ] **Step 3: Write minimal implementation**

```python
if not content.strip():
    return None
```

- [ ] **Step 4: Run test to verify it passes**

Run: `docker compose exec -T worker python -m unittest app.services.test_pdf_service -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add worker/app/services/pdf_service.py worker/app/services/test_pdf_service.py
git commit -m "空PDF抽出の失敗条件を追加"
```

## Chunk 2: Extract Dispatcher

### Task 3: PDF 判定の失敗テストを書く

**Files:**
- Create or Modify: `worker/app/services/test_trafilatura_service.py`
- Modify: `worker/app/services/trafilatura_service.py`

- [ ] **Step 1: Write the failing test**

```python
def test_is_pdf_response_accepts_content_type():
    assert is_pdf_response("https://example.com/file", "application/pdf", b"%PDF-1.7")

def test_is_pdf_response_accepts_pdf_extension():
    assert is_pdf_response("https://example.com/file.pdf", "application/octet-stream", b"")
```

- [ ] **Step 2: Run test to verify it fails**

Run: `docker compose exec -T worker python -m unittest app.services.test_trafilatura_service -v`
Expected: FAIL with missing helper

- [ ] **Step 3: Write minimal implementation**

```python
def is_pdf_response(url: str, content_type: str | None, content: bytes | None) -> bool:
    ...
```

- [ ] **Step 4: Run test to verify it passes**

Run: `docker compose exec -T worker python -m unittest app.services.test_trafilatura_service -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add worker/app/services/trafilatura_service.py worker/app/services/test_trafilatura_service.py
git commit -m "PDF判定ヘルパーを追加"
```

### Task 4: `/extract-body` から PDF extractor に分岐させる

**Files:**
- Modify: `worker/app/services/trafilatura_service.py`
- Modify: `worker/app/routers/extract.py`
- Modify: `worker/app/services/test_trafilatura_service.py`

- [ ] **Step 1: Write the failing test**

```python
def test_extract_body_uses_pdf_extractor_for_pdf_url():
    with patch("app.services.trafilatura_service.extract_pdf_body") as extract_pdf_body:
        extract_pdf_body.return_value = {"title": "doc", "content": "text", "published_at": None, "image_url": None}
        result = extract_body("https://example.com/report.pdf")
    assert result["content"] == "text"
```

- [ ] **Step 2: Run test to verify it fails**

Run: `docker compose exec -T worker python -m unittest app.services.test_trafilatura_service -v`
Expected: FAIL because HTML path still runs

- [ ] **Step 3: Write minimal implementation**

```python
if detected_as_pdf:
    return extract_pdf_body(url)
```

- [ ] **Step 4: Run test to verify it passes**

Run: `docker compose exec -T worker python -m unittest app.services.test_trafilatura_service -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add worker/app/services/trafilatura_service.py worker/app/routers/extract.py worker/app/services/test_trafilatura_service.py
git commit -m "extract-bodyにPDF分岐を追加"
```

## Chunk 3: Dependency and End-to-End Verification

### Task 5: worker 依存に PyMuPDF を追加する

**Files:**
- Modify: `worker/requirements.txt` or active worker dependency file

- [ ] **Step 1: Write the failing environment check**

Run: `docker compose exec -T worker python -c "import fitz"`
Expected: FAIL with `ModuleNotFoundError`

- [ ] **Step 2: Add minimal dependency**

```txt
PyMuPDF==<repo-aligned-version>
```

- [ ] **Step 3: Rebuild or reinstall worker dependency layer**

Run: `docker compose build worker`
Expected: build succeeds with PyMuPDF installed

- [ ] **Step 4: Verify import passes**

Run: `docker compose exec -T worker python -c "import fitz; print('ok')"`
Expected: prints `ok`

- [ ] **Step 5: Commit**

```bash
git add worker/requirements.txt Dockerfile docker-compose.yml
git commit -m "workerにPyMuPDF依存を追加"
```

### Task 6: 回帰確認をまとめて通す

**Files:**
- Modify: none expected unless fixes are needed

- [ ] **Step 1: Run worker unit tests**

Run: `docker compose exec -T worker python -m unittest app.services.test_pdf_service app.services.test_trafilatura_service -v`
Expected: PASS

- [ ] **Step 2: Run worker wide validation**

Run: `make check-worker`
Expected: PASS

- [ ] **Step 3: Run API regression check for extract path contract**

Run: `docker compose exec -T api go test ./internal/service ./internal/inngest`
Expected: PASS

- [ ] **Step 4: Smoke test extract-body with a sample PDF URL**

Run: `docker compose exec -T worker python - <<'PY'\nfrom app.services.pdf_service import extract_pdf_body_from_bytes\nprint('ok')\nPY`
Expected: command succeeds

- [ ] **Step 5: Commit final integration**

```bash
git add worker api
git commit -m "RSS記事のPDF本文取り込みに対応"
```

## Notes for Implementation

- 既存の HTML 抽出経路を壊さないことを優先する
- 初期版では `published_at` と `image_url` は PDF で `None` のままでよい
- タイトルは PDF metadata を優先しつつ、取れない場合は既存 item title が残る前提でよい
- OCR は入れない
- ニュース PDF 前提なので、初手では厳しいページ数制限は追加しない
