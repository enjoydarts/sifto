import json
import re

from app.services.llm_text_utils import (
    clamp01,
    decode_json_string_fragment,
    extract_first_json_object,
    extract_json_string_value_loose,
    normalize_url_for_match,
    strip_code_fence,
)


ASK_SYSTEM_INSTRUCTION = """# Role
あなたはRSSキュレーションアシスタントです。

# Task
与えられた候補記事だけを根拠に、日本語で質問へ回答してください。

# Rules
- 根拠は候補記事だけに限定してください。
- 候補記事から判断できないことは「候補記事からは判断できない」と明記してください。
- 出力はJSONオブジェクトのみとし、余計な説明文は書かないでください。
- answer は2〜3文にしてください。
- bullets は2〜3件にしてください。
- citations は2〜3件に絞ってください。
- citations は同じ話題に偏らせず、回答の主要な論点を支える記事を優先してください。
- answer の各文末には対応する item_id を [[item_id]] 形式で付けてください。
- bullets には citation マーカーを付けないでください。
- answer で使う [[item_id]] は citations に含まれる item_id だけを使ってください。
- [[item_id]] を付けられない文は書かないでください。"""


ASK_SCHEMA = {
    "type": "object",
    "properties": {
        "answer": {"type": "string"},
        "bullets": {"type": "array", "items": {"type": "string"}},
        "citations": {
            "type": "array",
            "items": {
                "type": "object",
                "properties": {
                    "item_id": {"type": "string"},
                    "reason": {"type": "string"},
                },
                "required": ["item_id", "reason"],
                "additionalProperties": False,
            },
        },
    },
    "required": ["answer", "bullets", "citations"],
    "additionalProperties": False,
}


RANK_FEED_SCHEMA = {
    "type": "object",
    "properties": {
        "items": {
            "type": "array",
            "items": {
                "type": "object",
                "properties": {
                    "id": {"type": "string"},
                    "reason": {"type": "string"},
                    "confidence": {"type": "number"},
                },
                "required": ["id", "reason", "confidence"],
                "additionalProperties": False,
            },
        }
    },
    "required": ["items"],
    "additionalProperties": False,
}


SEED_SITES_SCHEMA = {
    "type": "object",
    "properties": {
        "items": {
            "type": "array",
            "items": {
                "type": "object",
                "properties": {
                    "url": {"type": "string"},
                    "reason": {"type": "string"},
                },
                "required": ["url", "reason"],
                "additionalProperties": False,
            },
        }
    },
    "required": ["items"],
    "additionalProperties": False,
}


def build_ask_task(query: str, candidates: list[dict]) -> dict:
    lines: list[str] = []
    for idx, item in enumerate(candidates, start=1):
        title = item.get("translated_title") or item.get("title") or "（タイトルなし）"
        facts = [str(v).strip() for v in (item.get("facts") or []) if str(v).strip()]
        lines.append(
            f"- item_id={item.get('item_id')} | rank={idx} | title={title} | published_at={item.get('published_at') or ''} | "
            f"topics={', '.join(item.get('topics') or [])} | similarity={item.get('similarity')} | "
            f"summary={str(item.get('summary') or '')[:500]} | facts={' / '.join(facts[:4])[:400]}"
        )
    prompt = f"""# Output
{{
  "answer": "2〜3文の回答 [[item_id]]",
  "bullets": ["補足ポイント1", "補足ポイント2"],
  "citations": [
    {{"item_id": "uuid", "reason": "この観点の根拠"}}
  ]
}}

# Input
question: {query}
candidates:
{chr(10).join(lines)}
"""
    return {
        "system_instruction": ASK_SYSTEM_INSTRUCTION,
        "prompt": prompt,
        "schema": ASK_SCHEMA,
    }


def parse_ask_result(text: str, candidates: list[dict], *, error_prefix: str) -> dict:
    data = extract_first_json_object(text) or {}
    answer = str(data.get("answer") or "").strip() or extract_json_string_value_loose(text, "answer")
    bullets = [str(v).strip() for v in (data.get("bullets") or []) if str(v).strip()]
    citations = []
    for raw in data.get("citations") or []:
        if isinstance(raw, dict) and str(raw.get("item_id") or "").strip():
            citations.append(
                {
                    "item_id": str(raw.get("item_id") or "").strip(),
                    "reason": str(raw.get("reason") or "").strip(),
                }
            )
    if not citations:
        s = strip_code_fence(text)
        for match in re.finditer(r'"item_id"\s*:\s*"([^"]+)"(?:[^}]*"reason"\s*:\s*"((?:\\.|[^"\\])*)")?', s, re.S):
            citations.append(
                {
                    "item_id": match.group(1).strip(),
                    "reason": decode_json_string_fragment(match.group(2)).strip() if match.group(2) else "",
                }
            )
    if not answer:
        raise RuntimeError(f"{error_prefix}: response_snippet={text[:500]}")
    if len(citations) < min(3, len(candidates)):
        seen = {str(c.get('item_id') or '').strip() for c in citations}
        for item in candidates:
            item_id = str(item.get('item_id') or '').strip()
            if not item_id or item_id in seen:
                continue
            citations.append({"item_id": item_id, "reason": "回答に関連する候補記事"})
            seen.add(item_id)
            if len(citations) >= min(5, len(candidates)):
                break
    return {"answer": answer, "bullets": bullets[:3], "citations": citations[:3]}


def build_rank_feed_task(
    existing_sources: list[dict],
    preferred_topics: list[str],
    candidates: list[dict],
    positive_examples: list[dict] | None,
    negative_examples: list[dict] | None,
) -> dict:
    existing_sources = existing_sources[:40]
    preferred_topics = [str(t).strip() for t in preferred_topics if str(t).strip()][:20]
    candidates = candidates[:80]
    positive_examples = (positive_examples or [])[:8]
    negative_examples = (negative_examples or [])[:5]
    prompt = f"""あなたはRSSフィードの推薦アシスタントです。
既存の購読ソース・興味トピック・候補フィードを見て、ユーザーに合いそうな候補を順位付けしてください。

要件:
- 候補は必ず id で指定する（urlは補助情報で、新規URLを作らない）
- 既存ソースと重複しすぎる候補は下げる
- 興味トピックに近い候補を優先
- 理由は日本語で短く（40〜100字）
- JSONのみで返す

返却形式:
{{
  "items": [
    {{"id":"c001", "reason":"...", "confidence":0.0-1.0}}
  ]
}}

Few-shot（好みの既存Feed例）:
{json.dumps(positive_examples, ensure_ascii=False)}

Few-shot（避けたい傾向の既存Feed例）:
{json.dumps(negative_examples, ensure_ascii=False)}

既存ソース:
{json.dumps(existing_sources, ensure_ascii=False)}

興味トピック:
{json.dumps(preferred_topics, ensure_ascii=False)}

候補フィード:
{json.dumps(candidates, ensure_ascii=False)}
"""
    return {
        "prompt": prompt,
        "schema": RANK_FEED_SCHEMA,
        "existing_sources": existing_sources,
        "preferred_topics": preferred_topics,
        "candidates": candidates,
    }


def parse_rank_feed_result(text: str, candidates: list[dict]) -> list[dict]:
    data = extract_first_json_object(text) or {}
    rows = data.get("items", []) if isinstance(data.get("items"), list) else []
    allowed_ids = {str(c.get("id") or "").strip() for c in candidates if str(c.get("id") or "").strip()}
    out = []
    for row in rows:
        if not isinstance(row, dict):
            continue
        cid = str(row.get("id") or "").strip()
        if not cid or cid not in allowed_ids:
            continue
        out.append(
            {
                "id": cid,
                "url": "",
                "reason": str(row.get("reason") or "").strip()[:180],
                "confidence": clamp01(row.get("confidence", 0.5), 0.5),
            }
        )
    return out


def build_seed_sites_task(
    existing_sources: list[dict],
    preferred_topics: list[str],
    positive_examples: list[dict] | None,
    negative_examples: list[dict] | None,
) -> dict:
    existing_sources = existing_sources[:40]
    preferred_topics = [str(t).strip() for t in preferred_topics if str(t).strip()][:20]
    positive_examples = (positive_examples or [])[:8]
    negative_examples = (negative_examples or [])[:5]
    prompt = f"""あなたはRSSフィード探索アシスタントです。
既存の購読ソースと興味トピックを元に、「まだ登録していない可能性が高い」ニュース/技術メディアのサイトURL候補を提案してください。

要件:
- URLは実在しそうなサイトのトップURLを優先（https://example.com/ 形式）
- RSS URLを直接知らない場合はサイトトップURLでよい（後段でRSS探索する）
- 既存ソースと同じURLは除外
- 日本語で短い理由を付ける
- 最大30件
- JSONのみで返す

返却形式（必須）:
{{
  "items": [
    {{"url":"https://...", "reason":"..."}}
  ]
}}

Few-shot（好みの既存Feed例）:
{json.dumps(positive_examples, ensure_ascii=False)}

Few-shot（避けたい傾向の既存Feed例）:
{json.dumps(negative_examples, ensure_ascii=False)}

既存ソース:
{json.dumps(existing_sources, ensure_ascii=False)}

興味トピック:
{json.dumps(preferred_topics, ensure_ascii=False)}
"""
    return {
        "prompt": prompt,
        "schema": SEED_SITES_SCHEMA,
        "existing_sources": existing_sources,
        "preferred_topics": preferred_topics,
    }


def parse_seed_sites_result(text: str, existing_sources: list[dict]) -> list[dict]:
    data = extract_first_json_object(text) or {}
    rows = data.get("items", []) if isinstance(data.get("items"), list) else []
    existing_set = {normalize_url_for_match(str(s.get("url") or "").strip()) for s in existing_sources}
    out = []
    for row in rows[:30]:
        if not isinstance(row, dict):
            continue
        url = str(row.get("url") or "").strip()
        if not url or normalize_url_for_match(url) in existing_set:
            continue
        out.append({"url": url, "reason": str(row.get("reason") or "").strip()[:180]})
    return out
