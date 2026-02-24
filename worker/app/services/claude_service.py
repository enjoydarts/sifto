import json
import logging
import os
import time
import anthropic

_client = None
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


def _split_text_chunks(text: str, chunk_chars: int = 8000, overlap_chars: int = 400) -> list[str]:
    text = (text or "").strip()
    if not text:
        return []
    if len(text) <= chunk_chars:
        return [text]

    chunks: list[str] = []
    start = 0
    n = len(text)
    while start < n:
        end = min(n, start + chunk_chars)
        chunks.append(text[start:end])
        if end >= n:
            break
        start = max(end - overlap_chars, start + 1)
    return chunks


def _parse_json_string_array(text: str) -> list[str]:
    start = text.find("[")
    end = text.rfind("]") + 1
    if start == -1 or end == 0:
        return []
    try:
        data = json.loads(text[start:end])
    except Exception:
        return []
    return [str(v) for v in data if isinstance(v, str)]


def _clamp01(v, default: float = 0.5) -> float:
    try:
        x = float(v)
    except Exception:
        return default
    if x < 0:
        return 0.0
    if x > 1:
        return 1.0
    return x


def _summary_composite_score(breakdown: dict) -> float:
    # Weighted for digest ranking / operations triage.
    weights = {
        "importance": 0.38,
        "novelty": 0.22,
        "actionability": 0.18,
        "reliability": 0.17,
        "relevance": 0.05,
    }
    total = 0.0
    for k, w in weights.items():
        total += _clamp01(breakdown.get(k, 0.5), 0.5) * w
    return round(total, 4)


def _merge_fact_lists(fact_lists: list[list[str]], max_items: int = 24) -> list[str]:
    # De-dup while preserving coverage across chunks by interleaving.
    normalized_seen: set[str] = set()
    merged: list[str] = []
    max_len = max((len(xs) for xs in fact_lists), default=0)
    for i in range(max_len):
        for facts in fact_lists:
            if i >= len(facts):
                continue
            fact = facts[i].strip()
            if not fact:
                continue
            key = " ".join(fact.lower().split())
            if key in normalized_seen:
                continue
            normalized_seen.add(key)
            merged.append(fact)
            if len(merged) >= max_items:
                return merged
    return merged


def _merge_llm_metas(metas: list[dict], purpose: str) -> dict:
    valid = [m for m in metas if isinstance(m, dict)]
    if not valid:
        return {
            "provider": "none",
            "model": "none",
            "pricing_model_family": "",
            "pricing_source": "default",
            "input_tokens": 0,
            "output_tokens": 0,
            "cache_creation_input_tokens": 0,
            "cache_read_input_tokens": 0,
            "estimated_cost_usd": 0.0,
        }

    providers = {str(m.get("provider", "")) for m in valid}
    models = [str(m.get("model", "")) for m in valid if m.get("model")]
    families = {str(m.get("pricing_model_family", "")) for m in valid if m.get("pricing_model_family") is not None}
    sources = {str(m.get("pricing_source", "default")) for m in valid}

    return {
        "provider": next(iter(providers)) if len(providers) == 1 else "mixed",
        "model": models[0] if len(set(models)) == 1 and models else "multiple",
        "pricing_model_family": next(iter(families)) if len(families) == 1 else "mixed",
        "pricing_source": next(iter(sources)) if len(sources) == 1 else "mixed",
        "input_tokens": sum(int(m.get("input_tokens", 0) or 0) for m in valid),
        "output_tokens": sum(int(m.get("output_tokens", 0) or 0) for m in valid),
        "cache_creation_input_tokens": sum(int(m.get("cache_creation_input_tokens", 0) or 0) for m in valid),
        "cache_read_input_tokens": sum(int(m.get("cache_read_input_tokens", 0) or 0) for m in valid),
        "estimated_cost_usd": round(sum(float(m.get("estimated_cost_usd", 0.0) or 0.0) for m in valid), 8),
        "calls": len(valid),
        "purpose": purpose,
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


def _client_for_api_key(api_key: str | None):
    if api_key:
        return anthropic.Anthropic(api_key=api_key)
    return None


def _messages_create(prompt: str, model: str, max_tokens: int = 1024, api_key: str | None = None):
    client = _client_for_api_key(api_key)
    if client is None:
        return None
    return client.messages.create(
        model=model,
        max_tokens=max_tokens,
        messages=[{"role": "user", "content": prompt}],
    )


def _is_rate_limit_error(err: Exception) -> bool:
    s = str(err).lower()
    return "429" in s or "rate_limit" in s


def _call_with_retries(prompt: str, model: str, max_tokens: int, retries: int = 2, api_key: str | None = None):
    last_err = None
    for attempt in range(retries + 1):
        try:
            return _messages_create(prompt, model, max_tokens=max_tokens, api_key=api_key)
        except Exception as e:
            last_err = e
            if attempt >= retries or not _is_rate_limit_error(e):
                raise
            # Small exponential backoff for rate limits.
            sleep_sec = 1.0 * (2 ** attempt)
            _log.warning(
                "anthropic rate-limited model=%s retry_in=%.1fs attempt=%d/%d",
                model,
                sleep_sec,
                attempt + 1,
                retries + 1,
            )
            time.sleep(sleep_sec)
    if last_err is not None:
        raise last_err
    return None


def _call_with_model_fallback(
    prompt: str, primary_model: str, fallback_model: str | None, max_tokens: int = 1024, api_key: str | None = None
):
    if _client_for_api_key(api_key) is None:
        return None, None
    try:
        return _call_with_retries(prompt, primary_model, max_tokens=max_tokens, api_key=api_key), primary_model
    except Exception as e:
        _log.warning("anthropic call failed model=%s err=%s", primary_model, e)
        if fallback_model and fallback_model != primary_model:
            try:
                return _call_with_retries(prompt, fallback_model, max_tokens=max_tokens, api_key=api_key), fallback_model
            except Exception as e2:
                _log.warning("anthropic fallback failed model=%s err=%s", fallback_model, e2)
        return None, None


def extract_facts(title: str | None, content: str, api_key: str | None = None) -> dict:
    if _client_for_api_key(api_key) is None:
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

    chunks = _split_text_chunks(content, chunk_chars=8000, overlap_chars=400)
    if not chunks:
        return {"facts": [], "llm": _merge_llm_metas([], "facts")}

    all_fact_lists: list[list[str]] = []
    llm_metas: list[dict] = []
    any_llm_success = False
    per_chunk_fact_target = 4 if len(chunks) <= 3 else 3 if len(chunks) <= 8 else 2

    for idx, chunk in enumerate(chunks, start=1):
        prompt = f"""以下の記事本文（{idx}/{len(chunks)}チャンク）から重要な事実を箇条書きで抽出してください。
事実は客観的かつ具体的に記述してください。
このチャンク内に明示されている内容だけを対象にしてください。
{per_chunk_fact_target}〜{per_chunk_fact_target + 2}個程度にまとめてください。
JSON配列として返してください。例: ["事実1", "事実2"]

タイトル: {title or "（不明）"}
チャンク: {idx}/{len(chunks)}

本文:
{chunk}
"""
        message, used_model = _call_with_model_fallback(
            prompt, _facts_model, _facts_model_fallback, max_tokens=1024, api_key=api_key
        )
        if message is None:
            continue
        any_llm_success = True
        text = message.content[0].text.strip()
        all_fact_lists.append(_parse_json_string_array(text))
        llm_metas.append(_llm_meta(message, "facts", used_model or _facts_model))

    if not any_llm_success:
        lines = [line.strip() for line in content.splitlines() if line.strip()]
        return {
            "facts": lines[:5],
            "llm": {
                "provider": "local-fallback",
                "model": _facts_model,
                "input_tokens": 0,
                "output_tokens": 0,
                "cache_creation_input_tokens": 0,
                "cache_read_input_tokens": 0,
                "estimated_cost_usd": 0.0,
            },
        }

    merged_facts = _merge_fact_lists(all_fact_lists, max_items=24)
    llm = _merge_llm_metas(llm_metas, "facts")
    llm["chunk_count"] = len(chunks)
    llm["chunk_success_count"] = len(llm_metas)
    return {
        "facts": merged_facts,
        "llm": llm,
    }


def summarize(title: str | None, facts: list[str], api_key: str | None = None) -> dict:
    if _client_for_api_key(api_key) is None:
        summary = " / ".join(facts[:5])[:420] if facts else (title or "")
        score_breakdown = {
            "importance": 0.4,
            "novelty": 0.4,
            "actionability": 0.4,
            "reliability": 0.5,
            "relevance": 0.5,
        }
        return {
            "summary": summary or "要約を生成できませんでした",
            "topics": ["local-dev"],
            "score": _summary_composite_score(score_breakdown),
            "score_breakdown": score_breakdown,
            "score_reason": "ローカルフォールバックのため簡易スコアです。",
            "score_policy_version": "v2",
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
  "score_breakdown": {{
    "importance": 0.0〜1.0,
    "novelty": 0.0〜1.0,
    "actionability": 0.0〜1.0,
    "reliability": 0.0〜1.0,
    "relevance": 0.0〜1.0
  }},
  "score_reason": "採点理由（1〜2文）"
}}

要約スタイル:
- 客観的・中立的に書く（意見や煽りを入れない）
- 読みやすい自然な日本語にする（硬すぎる定型文を避ける）
- 記事の主題、何が起きたか、重要なポイントを過不足なく含める
- 箇条書きではなく、1〜2段落の文章でまとめる
- score_breakdown は以下の観点で付与する
  - importance: 一般読者にとっての重要度
  - novelty: 新規性・変化の大きさ
  - actionability: 実務で行動に繋がる度合い
  - reliability: 具体性・確度（数値/固有名詞/条件の明確さ）
  - relevance: 幅広い読者への関連性（個別ユーザー最適化ではない）

タイトル: {title or "（不明）"}

事実:
{facts_text}
"""
    message, used_model = _call_with_model_fallback(
        prompt, _summary_model, _summary_model_fallback, max_tokens=1800, api_key=api_key
    )
    if message is None:
        summary = " / ".join(facts[:5])[:420] if facts else (title or "")
        score_breakdown = {
            "importance": 0.4,
            "novelty": 0.4,
            "actionability": 0.4,
            "reliability": 0.5,
            "relevance": 0.5,
        }
        return {
            "summary": summary or "要約を生成できませんでした",
            "topics": ["local-dev"],
            "score": _summary_composite_score(score_breakdown),
            "score_breakdown": score_breakdown,
            "score_reason": "Anthropic応答を取得できなかったため簡易スコアです。",
            "score_policy_version": "v2",
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
    topics = data.get("topics", [])
    if not isinstance(topics, list):
        topics = []
    score_breakdown = data.get("score_breakdown", {})
    if not isinstance(score_breakdown, dict):
        score_breakdown = {}
    score_breakdown = {
        "importance": _clamp01(score_breakdown.get("importance", 0.5)),
        "novelty": _clamp01(score_breakdown.get("novelty", 0.5)),
        "actionability": _clamp01(score_breakdown.get("actionability", 0.5)),
        "reliability": _clamp01(score_breakdown.get("reliability", 0.5)),
        "relevance": _clamp01(score_breakdown.get("relevance", 0.5)),
    }
    score_reason = str(data.get("score_reason") or "").strip()
    if not score_reason:
        score_reason = "総合的な重要度・新規性・実用性を基に採点。"
    score = _summary_composite_score(score_breakdown)

    return {
        "summary": data.get("summary", ""),
        "topics": [str(t) for t in topics],
        "score": score,
        "score_breakdown": score_breakdown,
        "score_reason": score_reason[:400],
        "score_policy_version": "v2",
        "llm": _llm_meta(message, "summary", used_model or _summary_model),
    }


def _digest_primary_topic(item: dict) -> str:
    topics = item.get("topics") or []
    if isinstance(topics, list):
        for t in topics:
            s = str(t).strip()
            if s:
                return s[:40]
    return "その他"


def _digest_item_score(item: dict) -> float:
    try:
        return float(item.get("score", 0.0) or 0.0)
    except Exception:
        return 0.0


def _build_digest_input_sections(items: list[dict]) -> tuple[str, str]:
    # Small/medium days: preserve per-item details.
    if len(items) <= 80:
        summary_limit = 450 if len(items) <= 20 else 240 if len(items) <= 50 else 120
        lines = []
        for idx, item in enumerate(items, start=1):
            rank = item.get("rank")
            title = item.get("title") or "（タイトルなし）"
            summary = str(item.get("summary") or "")[:summary_limit]
            topics = ", ".join(item.get("topics") or [])
            score = item.get("score")
            lines.append(
                f"- item={idx} rank={rank} | title={title} | topics={topics} | score={score} | summary={summary}"
            )
        return "items", "\n".join(lines)

    # Large days: topic-based compression + top item highlights.
    sorted_items = sorted(
        items,
        key=lambda x: (
            int(x.get("rank") or 10**9),
            -_digest_item_score(x),
        ),
    )
    highlights = sorted_items[: min(24, len(sorted_items))]

    groups: dict[str, list[dict]] = {}
    for item in items:
        groups.setdefault(_digest_primary_topic(item), []).append(item)

    ordered_groups = sorted(
        groups.items(),
        key=lambda kv: (-len(kv[1]), -max((_digest_item_score(i) for i in kv[1]), default=0.0), kv[0]),
    )

    lines: list[str] = []
    lines.append("[top_items]")
    for idx, item in enumerate(highlights, start=1):
        title = item.get("title") or "（タイトルなし）"
        summary = str(item.get("summary") or "")[:140]
        topics = ", ".join(item.get("topics") or [])
        rank = item.get("rank")
        score = item.get("score")
        lines.append(
            f"- top={idx} rank={rank} | title={title} | topics={topics} | score={score} | summary={summary}"
        )

    lines.append("")
    lines.append("[topic_groups]")
    for topic, topic_items in ordered_groups[:40]:
        sorted_topic_items = sorted(
            topic_items,
            key=lambda x: (
                int(x.get("rank") or 10**9),
                -_digest_item_score(x),
            ),
        )
        sample_titles = [str(i.get("title") or "（タイトルなし）")[:60] for i in sorted_topic_items[:4]]
        sample_summaries = [str(i.get("summary") or "")[:90] for i in sorted_topic_items[:3]]
        avg_score = round(
            sum(_digest_item_score(i) for i in topic_items) / max(1, len(topic_items)),
            3,
        )
        lines.append(
            f"- topic={topic} | count={len(topic_items)} | avg_score={avg_score} | "
            f"sample_titles={' / '.join(sample_titles)} | sample_summaries={' / '.join(sample_summaries)}"
        )

    return "topic_grouped", "\n".join(lines)


def compose_digest(digest_date: str, items: list[dict], api_key: str | None = None) -> dict:
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

    input_mode, digest_input = _build_digest_input_sections(items)

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
input_mode: {input_mode}
items:
{digest_input}
"""

    message, used_model = _call_with_model_fallback(
        prompt,
        _digest_model,
        _digest_model_fallback,
        max_tokens=4000,
        api_key=api_key,
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
    llm = _llm_meta(message, "digest", used_model or _digest_model)
    llm["input_mode"] = input_mode
    llm["items_count"] = len(items)
    return {
        "subject": subject,
        "body": body,
        "llm": llm,
    }
