import json
import logging
import os
import anthropic

_api_key = os.getenv("ANTHROPIC_API_KEY")
_client = anthropic.Anthropic(api_key=_api_key) if _api_key else None
_facts_model = os.getenv("ANTHROPIC_FACTS_MODEL", "claude-haiku-4-5")
_summary_model = os.getenv("ANTHROPIC_SUMMARY_MODEL", "claude-sonnet-4-6")
_summary_model_fallback = os.getenv("ANTHROPIC_SUMMARY_MODEL_FALLBACK", "claude-sonnet-4-5-20250929")
_facts_model_fallback = os.getenv("ANTHROPIC_FACTS_MODEL_FALLBACK", "claude-3-5-haiku-20241022")
_digest_model = os.getenv("ANTHROPIC_DIGEST_MODEL", _summary_model)
_digest_model_fallback = os.getenv("ANTHROPIC_DIGEST_MODEL_FALLBACK", _summary_model_fallback)
_log = logging.getLogger(__name__)

_DEFAULT_MODEL_PRICING = {
    # USD per 1M tokens (Claude API pricing); cache write assumes 5m cache.
    "claude-haiku-4-5": {
        "input_per_mtok_usd": 1.0,
        "output_per_mtok_usd": 5.0,
        "cache_write_per_mtok_usd": 1.25,
        "cache_read_per_mtok_usd": 0.10,
    },
    "claude-sonnet-4-6": {
        "input_per_mtok_usd": 3.0,
        "output_per_mtok_usd": 15.0,
        "cache_write_per_mtok_usd": 3.75,
        "cache_read_per_mtok_usd": 0.30,
    },
}

def _env_optional_float(name: str) -> float | None:
    raw = os.getenv(name)
    if raw is None or raw == "":
        return None
    try:
        return float(raw)
    except Exception:
        return None


def _normalize_model_family(model: str) -> str:
    if not model:
        return ""
    for family in sorted(_DEFAULT_MODEL_PRICING.keys(), key=len, reverse=True):
        if model == family or model.startswith(family + "-"):
            return family
    return model


def _pricing_for_model(model: str, purpose: str) -> dict:
    family = _normalize_model_family(model)
    base = dict(
        _DEFAULT_MODEL_PRICING.get(
            family,
            {
                "input_per_mtok_usd": 0.0,
                "output_per_mtok_usd": 0.0,
                "cache_write_per_mtok_usd": 0.0,
                "cache_read_per_mtok_usd": 0.0,
            },
        )
    )
    source = "default"
    # Optional per-purpose overrides for temporary pricing changes without deploy.
    prefix = f"ANTHROPIC_{purpose.upper()}_"
    override_map = {
        "input_per_mtok_usd": _env_optional_float(prefix + "INPUT_PER_MTOK_USD"),
        "output_per_mtok_usd": _env_optional_float(prefix + "OUTPUT_PER_MTOK_USD"),
        "cache_write_per_mtok_usd": _env_optional_float(prefix + "CACHE_WRITE_PER_MTOK_USD"),
        "cache_read_per_mtok_usd": _env_optional_float(prefix + "CACHE_READ_PER_MTOK_USD"),
    }
    for k, v in override_map.items():
        if v is not None:
            base[k] = v
            source = "env_override"
    base["pricing_source"] = source
    base["pricing_model_family"] = family
    return base


def _message_usage(message) -> dict:
    usage = getattr(message, "usage", None)
    if usage is None:
        return {
            "input_tokens": 0,
            "output_tokens": 0,
            "cache_creation_input_tokens": 0,
            "cache_read_input_tokens": 0,
        }
    return {
        "input_tokens": int(getattr(usage, "input_tokens", 0) or 0),
        "output_tokens": int(getattr(usage, "output_tokens", 0) or 0),
        "cache_creation_input_tokens": int(getattr(usage, "cache_creation_input_tokens", 0) or 0),
        "cache_read_input_tokens": int(getattr(usage, "cache_read_input_tokens", 0) or 0),
    }


def _estimate_cost_usd(model: str, purpose: str, usage: dict) -> float:
    p = _pricing_for_model(model, purpose)
    total = 0.0
    total += usage["input_tokens"] / 1_000_000 * p["input_per_mtok_usd"]
    total += usage["output_tokens"] / 1_000_000 * p["output_per_mtok_usd"]
    total += usage["cache_creation_input_tokens"] / 1_000_000 * p["cache_write_per_mtok_usd"]
    total += usage["cache_read_input_tokens"] / 1_000_000 * p["cache_read_per_mtok_usd"]
    return round(total, 8)


def _llm_meta(message, purpose: str, model: str, provider: str = "anthropic") -> dict:
    usage = _message_usage(message) if message is not None else _message_usage(None)
    actual_model = str(getattr(message, "model", None) or model)
    pricing = _pricing_for_model(actual_model, purpose)
    return {
        "provider": provider,
        "model": actual_model,
        "pricing_model_family": pricing.get("pricing_model_family", ""),
        "pricing_source": pricing.get("pricing_source", "default"),
        **usage,
        "estimated_cost_usd": _estimate_cost_usd(actual_model, purpose, usage),
    }


def _messages_create(prompt: str, model: str, max_tokens: int = 1024):
    if _client is None:
        return None
    return _client.messages.create(
        model=model,
        max_tokens=max_tokens,
        messages=[{"role": "user", "content": prompt}],
    )


def _call_with_model_fallback(
    prompt: str, primary_model: str, fallback_model: str | None, max_tokens: int = 1024
):
    if _client is None:
        return None, None
    try:
        return _messages_create(prompt, primary_model, max_tokens=max_tokens), primary_model
    except Exception as e:
        _log.warning("anthropic call failed model=%s err=%s", primary_model, e)
        if fallback_model and fallback_model != primary_model:
            try:
                return _messages_create(prompt, fallback_model, max_tokens=max_tokens), fallback_model
            except Exception as e2:
                _log.warning("anthropic fallback failed model=%s err=%s", fallback_model, e2)
        return None, None


def extract_facts(title: str | None, content: str) -> dict:
    if _client is None:
        lines = [line.strip() for line in content.splitlines() if line.strip()]
        facts = lines[:5]
        if not facts and title:
            facts = [f"タイトル: {title}"]
        return {
            "facts": facts,
            "llm": {
                "provider": "local-dev",
                "model": "local-fallback",
                "input_tokens": 0,
                "output_tokens": 0,
                "cache_creation_input_tokens": 0,
                "cache_read_input_tokens": 0,
                "estimated_cost_usd": 0.0,
            },
        }

    prompt = f"""以下の記事から重要な事実を箇条書きで抽出してください。
事実は客観的かつ具体的に記述し、5〜10個程度にまとめてください。
JSON配列として返してください。例: ["事実1", "事実2"]

タイトル: {title or "（不明）"}

本文:
{content[:4000]}
"""
    message, used_model = _call_with_model_fallback(prompt, _facts_model, _facts_model_fallback, max_tokens=1024)
    if message is None:
        lines = [line.strip() for line in content.splitlines() if line.strip()]
        return {
            "facts": lines[:5],
            "llm": {
                "provider": "local-fallback",
                "model": used_model or _facts_model,
                "input_tokens": 0,
                "output_tokens": 0,
                "cache_creation_input_tokens": 0,
                "cache_read_input_tokens": 0,
                "estimated_cost_usd": 0.0,
            },
        }
    text = message.content[0].text.strip()
    # Extract JSON array from response
    start = text.find("[")
    end = text.rfind("]") + 1
    if start == -1 or end == 0:
        return {"facts": [], "llm": _llm_meta(message, "facts", used_model or _facts_model)}
    try:
        data = json.loads(text[start:end])
    except Exception:
        data = []
    return {
        "facts": [str(v) for v in data if isinstance(v, str)],
        "llm": _llm_meta(message, "facts", used_model or _facts_model),
    }


def summarize(title: str | None, facts: list[str]) -> dict:
    if _client is None:
        summary = " / ".join(facts[:5])[:420] if facts else (title or "")
        return {
            "summary": summary or "要約を生成できませんでした",
            "topics": ["local-dev"],
            "score": 0.5,
            "llm": {
                "provider": "local-dev",
                "model": "local-fallback",
                "input_tokens": 0,
                "output_tokens": 0,
                "cache_creation_input_tokens": 0,
                "cache_read_input_tokens": 0,
                "estimated_cost_usd": 0.0,
            },
        }

    facts_text = "\n".join(f"- {f}" for f in facts)
    prompt = f"""以下の事実リストをもとに、記事の要約を作成してください。
以下のJSON形式で返してください:
{{
  "summary": "350〜600字程度の要約",
  "topics": ["トピック1", "トピック2"],
  "score": 0.0〜1.0の関連度スコア（一般的な読者にとっての重要度）
}}

要約スタイル:
- 客観的・中立的に書く（意見や煽りを入れない）
- 読みやすい自然な日本語にする（硬すぎる定型文を避ける）
- 記事の主題、何が起きたか、重要なポイントを過不足なく含める
- 箇条書きではなく、1〜2段落の文章でまとめる

タイトル: {title or "（不明）"}

事実:
{facts_text}
"""
    message, used_model = _call_with_model_fallback(prompt, _summary_model, _summary_model_fallback, max_tokens=1800)
    if message is None:
        summary = " / ".join(facts[:5])[:420] if facts else (title or "")
        return {
            "summary": summary or "要約を生成できませんでした",
            "topics": ["local-dev"],
            "score": 0.5,
            "llm": {
                "provider": "local-fallback",
                "model": used_model or _summary_model,
                "input_tokens": 0,
                "output_tokens": 0,
                "cache_creation_input_tokens": 0,
                "cache_read_input_tokens": 0,
                "estimated_cost_usd": 0.0,
            },
        }
    text = message.content[0].text.strip()
    start = text.find("{")
    end = text.rfind("}") + 1
    try:
        data = json.loads(text[start:end])
    except Exception:
        data = {}
    try:
        score = float(data.get("score", 0.5))
    except Exception:
        score = 0.5

    topics = data.get("topics", [])
    if not isinstance(topics, list):
        topics = []

    return {
        "summary": data.get("summary", ""),
        "topics": [str(t) for t in topics],
        "score": score,
        "llm": _llm_meta(message, "summary", used_model or _summary_model),
    }


def compose_digest(digest_date: str, items: list[dict]) -> dict:
    if not items:
        return {
            "subject": f"Sifto Digest - {digest_date}",
            "body": "本日のダイジェスト対象記事はありませんでした。",
            "llm": {
                "provider": "none",
                "model": "none",
                "input_tokens": 0,
                "output_tokens": 0,
                "cache_creation_input_tokens": 0,
                "cache_read_input_tokens": 0,
                "estimated_cost_usd": 0.0,
            },
        }

    item_lines = []
    for idx, item in enumerate(items, start=1):
        rank = item.get("rank")
        title = item.get("title") or "（タイトルなし）"
        summary_limit = 450 if len(items) <= 20 else 240 if len(items) <= 50 else 120
        summary = str(item.get("summary") or "")[:summary_limit]
        topics = ", ".join(item.get("topics") or [])
        score = item.get("score")
        item_lines.append(
            f"- item={idx} rank={rank} | title={title} | topics={topics} | score={score} | summary={summary}"
        )

    prompt = f"""あなたはニュースダイジェスト編集者です。
以下の記事一覧をもとに、メール用のダイジェスト本文を日本語で作成してください。

要件:
- 当日分の全記事要約を踏まえて全体像をまとめる（記事を取りこぼさない）
- 読みやすく整理されていれば、本文は長めでもよい（目安 800〜2000字、必要なら超えて可）
- まず全体の流れを1〜2段落で要約
- 次に「注目ポイント」を箇条書きで5〜10点
- 誇張しない。与えられた情報だけで書く
- 最後に短い締めの一文
- JSONで返す

形式:
{{
  "subject": "件名（40字程度）",
  "body": "メール本文（プレーンテキスト。改行を含めてよい）"
}}

digest_date: {digest_date}
items_count: {len(items)}
items:
{chr(10).join(item_lines)}
"""

    message, used_model = _call_with_model_fallback(
        prompt,
        _digest_model,
        _digest_model_fallback,
        max_tokens=4000,
    )
    if message is None:
        top_topics = []
        for item in items:
            top_topics.extend(item.get("topics") or [])
        body = "本日のダイジェスト（当日分の全記事要約ベース）をお届けします。\n\n"
        body += "\n".join(
            f"- #{item.get('rank')} {item.get('title') or '（タイトルなし）'}"
            for item in items
        )
        if top_topics:
            body += "\n\n主なトピック: " + ", ".join(dict.fromkeys(top_topics))
        return {
            "subject": f"Sifto Digest {digest_date}",
            "body": body,
            "llm": {
                "provider": "local-fallback",
                "model": used_model or _digest_model,
                "input_tokens": 0,
                "output_tokens": 0,
                "cache_creation_input_tokens": 0,
                "cache_read_input_tokens": 0,
                "estimated_cost_usd": 0.0,
            },
        }

    text = message.content[0].text.strip()
    start = text.find("{")
    end = text.rfind("}") + 1
    try:
        data = json.loads(text[start:end])
    except Exception:
        data = {}

    subject = str(data.get("subject") or f"Sifto Digest {digest_date}")
    body = str(data.get("body") or "本日のダイジェストをお送りします。")
    return {
        "subject": subject,
        "body": body,
        "llm": _llm_meta(message, "digest", used_model or _digest_model),
    }
