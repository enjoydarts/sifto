import json
import logging
import os
import re

import httpx

from app.services.gemini_service import (
    _clamp01,
    _extract_compose_digest_fields,
    _extract_first_json_object,
    _extract_json_string_value_loose,
    _needs_title_translation,
    _normalize_url_for_match,
    _parse_json_string_array,
    _strip_code_fence,
    _summary_composite_score,
    _summary_max_tokens,
    _target_summary_chars,
    _decode_json_string_fragment,
)

_log = logging.getLogger(__name__)
_GROQ_PRICING_SOURCE_VERSION = "groq_static_2026_03"
_DEFAULT_MODEL_PRICING = {
    "openai/gpt-oss-20b": {"input_per_mtok_usd": 0.075, "output_per_mtok_usd": 0.30, "cache_read_per_mtok_usd": 0.0375},
    "openai/gpt-oss-120b": {"input_per_mtok_usd": 0.15, "output_per_mtok_usd": 0.60, "cache_read_per_mtok_usd": 0.075},
    "llama-3.1-8b-instant": {"input_per_mtok_usd": 0.05, "output_per_mtok_usd": 0.08, "cache_read_per_mtok_usd": 0.0},
    "llama-3.3-70b-versatile": {"input_per_mtok_usd": 0.59, "output_per_mtok_usd": 0.79, "cache_read_per_mtok_usd": 0.0},
    "meta-llama/llama-4-scout-17b-16e-instruct": {"input_per_mtok_usd": 0.11, "output_per_mtok_usd": 0.34, "cache_read_per_mtok_usd": 0.0},
    "qwen/qwen3-32b": {"input_per_mtok_usd": 0.29, "output_per_mtok_usd": 0.59, "cache_read_per_mtok_usd": 0.0},
}


def _env_timeout_seconds(name: str, default: float) -> float:
    raw = os.getenv(name)
    if raw is None or raw == "":
        return default
    try:
        v = float(raw)
        return v if v > 0 else default
    except Exception:
        return default


def _env_optional_float(name: str) -> float | None:
    raw = os.getenv(name)
    if raw is None or raw == "":
        return None
    try:
        return float(raw)
    except Exception:
        return None


def _normalize_model_name(model: str) -> str:
    return str(model or "").strip()


def _normalize_model_family(model: str) -> str:
    m = _normalize_model_name(model)
    for family in sorted(_DEFAULT_MODEL_PRICING.keys(), key=len, reverse=True):
        if m == family or m.startswith(family + "-"):
            return family
    return m


def _pricing_for_model(model: str, purpose: str) -> dict:
    family = _normalize_model_family(model)
    base = dict(_DEFAULT_MODEL_PRICING.get(family, {"input_per_mtok_usd": 0.0, "output_per_mtok_usd": 0.0, "cache_read_per_mtok_usd": 0.0}))
    source = _GROQ_PRICING_SOURCE_VERSION
    prefix = f"GROQ_{purpose.upper()}_"
    override_map = {
        "input_per_mtok_usd": _env_optional_float(prefix + "INPUT_PER_MTOK_USD"),
        "output_per_mtok_usd": _env_optional_float(prefix + "OUTPUT_PER_MTOK_USD"),
        "cache_read_per_mtok_usd": _env_optional_float(prefix + "CACHE_READ_PER_MTOK_USD"),
    }
    for k, v in override_map.items():
        if v is not None:
            base[k] = v
            source = "env_override"
    base["pricing_source"] = source
    base["pricing_model_family"] = family
    return base


def _estimate_cost_usd(model: str, purpose: str, usage: dict) -> float:
    p = _pricing_for_model(model, purpose)
    non_cached_input_tokens = max(0, int(usage.get("input_tokens", 0) or 0) - int(usage.get("cache_read_input_tokens", 0) or 0))
    total = 0.0
    total += non_cached_input_tokens / 1_000_000 * p["input_per_mtok_usd"]
    total += int(usage.get("output_tokens", 0) or 0) / 1_000_000 * p["output_per_mtok_usd"]
    total += int(usage.get("cache_read_input_tokens", 0) or 0) / 1_000_000 * p.get("cache_read_per_mtok_usd", 0.0)
    return round(total, 8)


def _llm_meta(model: str, purpose: str, usage: dict) -> dict:
    pricing = _pricing_for_model(model, purpose)
    actual_model = _normalize_model_name(model)
    return {
        "provider": "groq",
        "model": actual_model,
        "pricing_model_family": pricing.get("pricing_model_family", ""),
        "pricing_source": pricing.get("pricing_source", _GROQ_PRICING_SOURCE_VERSION),
        "input_tokens": int(usage.get("input_tokens", 0) or 0),
        "output_tokens": int(usage.get("output_tokens", 0) or 0),
        "cache_creation_input_tokens": int(usage.get("cache_creation_input_tokens", 0) or 0),
        "cache_read_input_tokens": int(usage.get("cache_read_input_tokens", 0) or 0),
        "estimated_cost_usd": _estimate_cost_usd(actual_model, purpose, usage),
    }


def _supports_strict_schema(model: str) -> bool:
    family = _normalize_model_family(model)
    return family in ("openai/gpt-oss-20b", "openai/gpt-oss-120b")


def _usage_from_response(data: dict) -> dict:
    usage = data.get("usage") or {}
    prompt_details = usage.get("prompt_tokens_details") or {}
    return {
        "input_tokens": int(usage.get("prompt_tokens", 0) or 0),
        "output_tokens": int(usage.get("completion_tokens", 0) or 0),
        "cache_creation_input_tokens": 0,
        "cache_read_input_tokens": int(prompt_details.get("cached_tokens", 0) or 0),
    }


def _chat_json(
    prompt: str,
    model: str,
    api_key: str,
    *,
    system_instruction: str | None = None,
    max_output_tokens: int = 1200,
    response_schema: dict | None = None,
    schema_name: str = "response",
    timeout_sec: float | None = None,
) -> tuple[str, dict]:
    api_key = (api_key or "").strip()
    if not api_key:
        raise RuntimeError("groq api key is required")
    body: dict = {
        "model": _normalize_model_name(model),
        "messages": [],
        "temperature": 0.2,
        "max_tokens": max_output_tokens,
    }
    if system_instruction:
        body["messages"].append({"role": "system", "content": system_instruction})
    body["messages"].append({"role": "user", "content": prompt})
    if response_schema is not None:
        if _supports_strict_schema(model):
            body["response_format"] = {
                "type": "json_schema",
                "json_schema": {
                    "name": schema_name,
                    "strict": True,
                    "schema": response_schema,
                },
            }
        else:
            body["response_format"] = {"type": "json_object"}
    url = "https://api.groq.com/openai/v1/chat/completions"
    headers = {
        "Authorization": f"Bearer {api_key}",
        "Content-Type": "application/json",
    }
    req_timeout = timeout_sec if timeout_sec and timeout_sec > 0 else _env_timeout_seconds("GROQ_TIMEOUT_SEC", 90.0)
    with httpx.Client(timeout=req_timeout) as client:
        resp = client.post(url, headers=headers, json=body)
    if resp.status_code >= 400:
        raise RuntimeError(f"groq chat.completions failed status={resp.status_code} body={resp.text[:1000]}")
    data = resp.json() if resp.content else {}
    choices = data.get("choices") or []
    if not choices:
        raise RuntimeError("groq chat.completions failed: empty choices")
    message = choices[0].get("message") or {}
    content = message.get("content")
    if isinstance(content, list):
        text = "\n".join(str(part.get("text") or "") for part in content if isinstance(part, dict))
    else:
        text = str(content or "")
    return text.strip(), _usage_from_response(data)


def _translate_title_to_ja(title: str, model: str, api_key: str) -> str:
    system_instruction = """# Role
あなたは見出し翻訳の専門家です。

# Task
英語やその他外国語のニュース記事タイトルを自然な日本語に翻訳してください。

# Rules
- 出力は必ず有効なJSONオブジェクト1つのみ
- translated_title が翻訳結果
- 既に日本語タイトルなら translated_title は空文字
- 固有名詞・製品名・企業名は必要に応じて原語を維持"""
    prompt = f"""# Output
{{
  \"translated_title\": \"日本語訳\"
}}

# Input
タイトル: {title}
"""
    text, _ = _chat_json(prompt, model, api_key, system_instruction=system_instruction, max_output_tokens=180, response_schema={
        "type": "object",
        "properties": {"translated_title": {"type": "string"}},
        "required": ["translated_title"],
        "additionalProperties": False,
    }, schema_name="translated_title")
    data = _extract_first_json_object(text) or {}
    translated = str(data.get("translated_title") or "").strip()
    if translated:
        return translated[:300]
    plain_prompt = f"""# Input
次のタイトルが外国語なら自然な日本語に翻訳し、日本語なら空文字を返してください。
タイトル: {title}
"""
    plain_text, _ = _chat_json(plain_prompt, model, api_key, max_output_tokens=120)
    candidate = _strip_code_fence(plain_text).strip().strip('"').strip("'")
    return candidate[:300]


def extract_facts(title: str | None, content: str, model: str, api_key: str) -> dict:
    system_instruction = """# Role
あなたは正確かつ客観的なニュース要約の専門家です。

# Task
提供される記事から重要な事実を8〜18個の箇条書きで抽出してください。

# Rules
- 出力は必ず [\"事実1\", \"事実2\", ...] のJSON形式の配列のみとしてください。
- 余計な挨拶や解説は一切不要です。
- 事実は客観的かつ具体的に記述してください。
- 記事が英語の場合も、出力は自然な日本語にしてください。
- 固有名詞は原文を尊重し、適宜英字を維持してください。"""
    prompt = f"""# Input
タイトル: {title or '（不明）'}

本文:
{content}
"""
    text, usage = _chat_json(prompt, model, api_key, system_instruction=system_instruction, max_output_tokens=1500)
    return {"facts": _parse_json_string_array(text), "llm": _llm_meta(model, "facts", usage)}


def summarize(title: str | None, facts: list[str], source_text_chars: int | None = None, model: str = "openai/gpt-oss-120b", api_key: str = "") -> dict:
    target_chars = _target_summary_chars(source_text_chars, facts)
    min_chars = max(160, round(target_chars * 0.8))
    max_chars = min(1400, max(260, round(target_chars * 1.2)))
    facts_text = "\n".join(f"- {f}" for f in facts)
    system_instruction = """# Role
あなたは正確かつ客観的なニュース要約の専門家です。

# Task
与えられた事実リストから記事要約を作成してください。

# Rules
- 出力は必ず有効なJSONオブジェクト1つのみにしてください。
- 前置き・後置き・コードフェンス・注釈は不要です。
- 要約は客観的・中立的な自然な日本語で書いてください。
- 記事の主題、何が起きたか、重要なポイントを過不足なく含めてください。
- 箇条書きではなく2〜4段落の文章でまとめてください。
- タイトルが主に英語の場合のみ translated_title に自然な日本語訳を入れてください。
- タイトルが日本語の場合は translated_title を空文字にしてください。
- 事実リストにない推測の断定、誇張表現、主観的評価は禁止です。
- topics は重複を避け、粒度を揃えてください。
- score_reason は採点の根拠を1〜2文で簡潔に述べてください。"""
    prompt = f"""# Output
{{
  \"summary\": \"要約\",
  \"topics\": [\"トピック1\", \"トピック2\"],
  \"translated_title\": \"英語タイトルの場合のみ日本語訳（日本語記事は空文字）\",
  \"score_breakdown\": {{
    \"importance\": 0.0,
    \"novelty\": 0.0,
    \"actionability\": 0.0,
    \"reliability\": 0.0,
    \"relevance\": 0.0
  }},
  \"score_reason\": \"採点理由（1〜2文）\"
}}

# Input
summary は {min_chars}〜{max_chars}字程度で作成し、目標は約{target_chars}字にしてください。

タイトル: {title or '（不明）'}
事実:
{facts_text}
"""
    schema = {
        "type": "object",
        "properties": {
            "summary": {"type": "string"},
            "topics": {"type": "array", "items": {"type": "string"}},
            "translated_title": {"type": "string"},
            "score_breakdown": {
                "type": "object",
                "properties": {
                    "importance": {"type": "number"},
                    "novelty": {"type": "number"},
                    "actionability": {"type": "number"},
                    "reliability": {"type": "number"},
                    "relevance": {"type": "number"},
                },
                "required": ["importance", "novelty", "actionability", "reliability", "relevance"],
                "additionalProperties": False,
            },
            "score_reason": {"type": "string"},
        },
        "required": ["summary", "topics", "translated_title", "score_breakdown", "score_reason"],
        "additionalProperties": False,
    }
    text, usage = _chat_json(prompt, model, api_key, system_instruction=system_instruction, max_output_tokens=_summary_max_tokens(target_chars), response_schema=schema, schema_name="summary")
    data = _extract_first_json_object(text) or {}
    topics = data.get("topics", []) if isinstance(data.get("topics"), list) else []
    raw_breakdown = data.get("score_breakdown") if isinstance(data.get("score_breakdown"), dict) else {}
    score_breakdown = {
        "importance": _clamp01(raw_breakdown.get("importance", 0.5)),
        "novelty": _clamp01(raw_breakdown.get("novelty", 0.5)),
        "actionability": _clamp01(raw_breakdown.get("actionability", 0.5)),
        "reliability": _clamp01(raw_breakdown.get("reliability", 0.5)),
        "relevance": _clamp01(raw_breakdown.get("relevance", 0.5)),
    }
    score_reason = str(data.get("score_reason") or "").strip() or "総合的な重要度・新規性・実用性を基に採点。"
    translated_title = str(data.get("translated_title") or "").strip()
    if _needs_title_translation(title, translated_title):
        translated_title = _translate_title_to_ja(title or "", model, api_key)
    return {
        "summary": str(data.get("summary") or "").strip(),
        "topics": [str(t) for t in topics if str(t).strip()],
        "translated_title": translated_title[:300],
        "score": _summary_composite_score(score_breakdown),
        "score_breakdown": score_breakdown,
        "score_reason": score_reason[:400],
        "score_policy_version": "v2",
        "llm": _llm_meta(model, "summary", usage),
    }


def translate_title(title: str, model: str = "openai/gpt-oss-20b", api_key: str = "") -> dict:
    src = (title or "").strip()
    if not src:
        return {"translated_title": "", "llm": None}
    translated = _translate_title_to_ja(src, model, api_key)
    return {"translated_title": translated[:300], "llm": None}


def compose_digest(digest_date: str, items: list[dict], model: str, api_key: str) -> dict:
    if not items:
        return {
            "subject": f"Sifto Digest - {digest_date}",
            "body": "本日のダイジェスト対象記事はありませんでした。",
            "llm": _llm_meta(model, "digest", {"input_tokens": 0, "output_tokens": 0}),
        }
    lines = []
    for idx, item in enumerate(items, start=1):
        lines.append(
            f"- item={idx} rank={item.get('rank')} | title={item.get('title') or '（タイトルなし）'} | topics={', '.join(item.get('topics') or [])} | score={item.get('score')} | summary={str(item.get('summary') or '')[:260]}"
        )
    system_instruction = """# Role
あなたはニュースダイジェスト編集者です。

# Task
与えられた記事一覧をもとに、メール用のダイジェスト本文を日本語で作成してください。

# Rules
- 当日分の全記事要約を踏まえて全体像をまとめてください。記事を取りこぼさないでください。
- 本文は900〜2200字程度を目安とし、必要なら超えて構いません。
- 本文は必ず次の順序で構成してください:
  1) 全体サマリ（1〜3段落）
  2) 注目ポイント（5〜10個。各ポイントは1〜2段落）
  3) その他のポイント（個数指定なし。箇条書き）
  4) 明日以降のフォローポイント（1段落）
  5) 締めの1文
- body は可読性を最優先し、各セクションの間に必ず空行1行（\\n\\n）を入れてください。
- 段落同士も必要に応じて空行（\\n\\n）で分けてください。
- 誇張せず、与えられた情報だけで書いてください。
- 出力はJSONオブジェクトのみとしてください。"""
    prompt = f"""# Output
{{
  \"subject\": \"件名（40字程度）\",
  \"body\": \"メール本文（プレーンテキスト。改行を含めてよい）\",
  \"sections\": {{
    \"overall_summary\": \"1〜3段落\",
    \"highlights\": [\"注目ポイント1（1〜2段落）\", \"注目ポイント2（1〜2段落）\"],
    \"other_points\": [\"補足1\", \"補足2\"],
    \"follow_up\": \"明日以降のフォローポイント（1段落）\",
    \"closing\": \"締めの1文\"
  }}
}}

# Input
digest_date: {digest_date}
items_count: {len(items)}
items:
{chr(10).join(lines)}
"""
    schema = {
        "type": "object",
        "properties": {
            "subject": {"type": "string"},
            "body": {"type": "string"},
        },
        "required": ["subject", "body"],
        "additionalProperties": True,
    }
    text, usage = _chat_json(prompt, model, api_key, system_instruction=system_instruction, max_output_tokens=8000, response_schema=schema, schema_name="digest", timeout_sec=_env_timeout_seconds("GROQ_COMPOSE_DIGEST_TIMEOUT_SEC", 240.0))
    subject, body = _extract_compose_digest_fields(text)
    if not subject or not body:
        raise RuntimeError(f"groq compose_digest parse failed: response_snippet={text[:500]}")
    return {"subject": subject, "body": body, "llm": _llm_meta(model, "digest", usage)}


def ask_question(query: str, candidates: list[dict], model: str, api_key: str) -> dict:
    if not candidates:
        return {
            "answer": "該当する記事は見つかりませんでした。",
            "bullets": [],
            "citations": [],
            "llm": _llm_meta(model, "ask", {"input_tokens": 0, "output_tokens": 0}),
        }
    lines = []
    for idx, item in enumerate(candidates, start=1):
        title = item.get("translated_title") or item.get("title") or "（タイトルなし）"
        facts = [str(v).strip() for v in (item.get("facts") or []) if str(v).strip()]
        lines.append(
            f"- item_id={item.get('item_id')} | rank={idx} | title={title} | published_at={item.get('published_at') or ''} | topics={', '.join(item.get('topics') or [])} | similarity={item.get('similarity')} | summary={str(item.get('summary') or '')[:500]} | facts={' / '.join(facts[:4])[:400]}"
        )
    system_instruction = """# Role
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
- answer で使う [[item_id]] は citations に含まれる item_id だけを使ってください。"""
    prompt = f"""# Output
{{
  \"answer\": \"2〜3文の回答 [[item_id]]\",
  \"bullets\": [\"補足ポイント1\", \"補足ポイント2\"],
  \"citations\": [
    {{\"item_id\": \"uuid\", \"reason\": \"この観点の根拠\"}}
  ]
}}

# Input
question: {query}
candidates:
{chr(10).join(lines)}
"""
    schema = {
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
    text, usage = _chat_json(prompt, model, api_key, system_instruction=system_instruction, max_output_tokens=3200, response_schema=schema, schema_name="ask")
    data = _extract_first_json_object(text) or {}
    answer = str(data.get("answer") or "").strip() or _extract_json_string_value_loose(text, "answer")
    bullets = [str(v).strip() for v in (data.get("bullets") or []) if str(v).strip()]
    citations = []
    for raw in data.get("citations") or []:
        if isinstance(raw, dict) and str(raw.get("item_id") or "").strip():
            citations.append({"item_id": str(raw.get("item_id") or "").strip(), "reason": str(raw.get("reason") or "").strip()})
    if not citations:
        s = _strip_code_fence(text)
        for match in re.finditer(r'"item_id"\s*:\s*"([^"]+)"(?:[^}]*"reason"\s*:\s*"((?:\\.|[^"\\])*)")?', s, re.S):
            citations.append({
                "item_id": match.group(1).strip(),
                "reason": _decode_json_string_fragment(match.group(2)).strip() if match.group(2) else "",
            })
    if not answer:
        raise RuntimeError(f"groq ask missing answer; response_snippet={text[:500]}")
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
    return {"answer": answer, "bullets": bullets[:3], "citations": citations[:3], "llm": _llm_meta(model, "ask", usage)}


def compose_digest_cluster_draft(cluster_label: str, item_count: int, topics: list[str], source_lines: list[str], model: str, api_key: str) -> dict:
    system_instruction = """# Role
あなたはニュースダイジェストの下書き編集者です。

# Task
同じ話題に属する複数記事の要点メモから、重複をまとめたクラスタ下書きを作成してください。

# Rules
- 与えられた内容のみ使う
- 重複をまとめる
- 重要な相違点があれば残す
- プレーンテキストの箇条書き 3〜8 行程度
- JSONのみで返す"""
    prompt = f"""# Output
{{
  \"draft_summary\": \"- ...\\n- ...\"
}}

# Input
cluster_label: {cluster_label}
item_count: {item_count}
topics: {json.dumps(topics or [], ensure_ascii=False)}
source_lines:
{json.dumps(source_lines or [], ensure_ascii=False)}
"""
    schema = {
        "type": "object",
        "properties": {"draft_summary": {"type": "string"}},
        "required": ["draft_summary"],
        "additionalProperties": False,
    }
    text, usage = _chat_json(prompt, model, api_key, system_instruction=system_instruction, max_output_tokens=1200, response_schema=schema, schema_name="digest_cluster_draft")
    data = _extract_first_json_object(text) or {}
    draft = str(data.get("draft_summary") or "").strip()
    if not draft:
        raise RuntimeError(f"groq compose_digest_cluster_draft parse failed: response_snippet={text[:500]}")
    return {"draft_summary": draft, "llm": _llm_meta(model, "digest_cluster_draft", usage)}


def rank_feed_suggestions(existing_sources: list[dict], preferred_topics: list[str], candidates: list[dict], positive_examples: list[dict] | None, negative_examples: list[dict] | None, model: str, api_key: str) -> dict:
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
  \"items\": [
    {{\"id\":\"c001\", \"reason\":\"...\", \"confidence\":0.0}}
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
    schema = {
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
    text, usage = _chat_json(prompt, model, api_key, max_output_tokens=2800, response_schema=schema, schema_name="rank_feed_suggestions")
    data = _extract_first_json_object(text) or {}
    rows = data.get("items", []) if isinstance(data.get("items"), list) else []
    allowed_ids = {str(c.get("id") or "").strip() for c in candidates if str(c.get("id") or "").strip()}
    out = []
    for row in rows:
        if not isinstance(row, dict):
            continue
        cid = str(row.get("id") or "").strip()
        if not cid or cid not in allowed_ids:
            continue
        out.append({
            "id": cid,
            "url": "",
            "reason": str(row.get("reason") or "").strip()[:180],
            "confidence": _clamp01(row.get("confidence", 0.5), 0.5),
        })
    return {"items": out, "llm": _llm_meta(model, "source_suggestion", usage)}


def suggest_feed_seed_sites(existing_sources: list[dict], preferred_topics: list[str], positive_examples: list[dict] | None, negative_examples: list[dict] | None, model: str, api_key: str) -> dict:
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
  \"items\": [
    {{\"url\":\"https://...\", \"reason\":\"...\"}}
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
    schema = {
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
    text, usage = _chat_json(prompt, model, api_key, max_output_tokens=2200, response_schema=schema, schema_name="suggest_feed_seed_sites")
    data = _extract_first_json_object(text) or {}
    rows = data.get("items", []) if isinstance(data.get("items"), list) else []
    existing_set = {_normalize_url_for_match(str(s.get("url") or "").strip()) for s in existing_sources}
    out = []
    for row in rows[:30]:
        if not isinstance(row, dict):
            continue
        url = str(row.get("url") or "").strip()
        if not url or _normalize_url_for_match(url) in existing_set:
            continue
        out.append({"url": url, "reason": str(row.get("reason") or "").strip()[:180]})
    return {"items": out, "llm": _llm_meta(model, "source_suggestion", usage)}
