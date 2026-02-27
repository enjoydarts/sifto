import json
import logging
import os

import httpx

_log = logging.getLogger(__name__)
_GEMINI_PRICING_SOURCE_VERSION = "google_aistudio_static_2026_02"

_DEFAULT_MODEL_PRICING = {
    "gemini-3-flash-preview": {"input_per_mtok_usd": 0.5, "output_per_mtok_usd": 3.0},
    # <=200k prompt tokens. >200k tier is handled in _estimate_cost_usd.
    "gemini-3.1-pro-preview": {"input_per_mtok_usd": 2.0, "output_per_mtok_usd": 12.0},
    # Deprecated model kept for backward-compat pricing on existing user settings.
    "gemini-3-pro-preview": {"input_per_mtok_usd": 2.0, "output_per_mtok_usd": 12.0},
    # USD per 1M tokens (input/output).
    "gemini-2.5-flash": {"input_per_mtok_usd": 0.3, "output_per_mtok_usd": 2.5},
    "gemini-2.5-flash-lite": {"input_per_mtok_usd": 0.1, "output_per_mtok_usd": 0.4},
    "gemini-2.5-pro": {"input_per_mtok_usd": 1.25, "output_per_mtok_usd": 10.0},
    # Legacy/deprecated families kept for backward compatibility in historical logs/user settings.
    "gemini-2.0-flash": {"input_per_mtok_usd": 0.1, "output_per_mtok_usd": 0.4},
    "gemini-2.0-flash-lite": {"input_per_mtok_usd": 0.075, "output_per_mtok_usd": 0.3},
    "gemini-1.5-flash": {"input_per_mtok_usd": 0.075, "output_per_mtok_usd": 0.3},
    "gemini-1.5-pro": {"input_per_mtok_usd": 1.25, "output_per_mtok_usd": 5.0},
}


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


def _clamp_int(v: int, lo: int, hi: int) -> int:
    return max(lo, min(hi, int(v)))


def _target_summary_chars(source_text_chars: int | None, facts: list[str]) -> int:
    if isinstance(source_text_chars, int) and source_text_chars > 0:
        return _clamp_int(round(source_text_chars * 0.16), 220, 1200)
    facts_chars = sum(len(str(f)) for f in (facts or []))
    if facts_chars > 0:
        return _clamp_int(round(facts_chars * 0.9), 220, 900)
    return 300


def _summary_max_tokens(target_chars: int) -> int:
    return _clamp_int(round(target_chars * 1.2), 700, 2600)


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


def _normalize_model_name(model: str) -> str:
    m = str(model or "").strip()
    if m.startswith("models/"):
        return m[7:]
    if "/models/" in m:
        return m.split("/models/", 1)[1]
    return m


def _normalize_model_family(model: str) -> str:
    m = _normalize_model_name(model)
    for family in sorted(_DEFAULT_MODEL_PRICING.keys(), key=len, reverse=True):
        if m == family or m.startswith(family + "-"):
            return family
    return m


def _env_optional_float(name: str) -> float | None:
    raw = os.getenv(name)
    if raw is None or raw == "":
        return None
    try:
        return float(raw)
    except Exception:
        return None


def _pricing_for_model(model: str, purpose: str) -> dict:
    family = _normalize_model_family(model)
    base = dict(
        _DEFAULT_MODEL_PRICING.get(
            family,
            {
                "input_per_mtok_usd": 0.0,
                "output_per_mtok_usd": 0.0,
            },
        )
    )
    source = _GEMINI_PRICING_SOURCE_VERSION
    prefix = f"GEMINI_{purpose.upper()}_"
    override_map = {
        "input_per_mtok_usd": _env_optional_float(prefix + "INPUT_PER_MTOK_USD"),
        "output_per_mtok_usd": _env_optional_float(prefix + "OUTPUT_PER_MTOK_USD"),
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
    family = _normalize_model_family(model)
    input_rate = p["input_per_mtok_usd"]
    output_rate = p["output_per_mtok_usd"]
    # Gemini Pro preview families have two pricing tiers by prompt length.
    if family in ("gemini-3.1-pro-preview", "gemini-3-pro-preview") and usage.get("input_tokens", 0) > 200_000:
        input_rate = 4.0
        output_rate = 18.0
    if family == "gemini-2.5-pro" and usage.get("input_tokens", 0) > 200_000:
        input_rate = 2.5
        output_rate = 15.0
    total = 0.0
    total += usage.get("input_tokens", 0) / 1_000_000 * input_rate
    total += usage.get("output_tokens", 0) / 1_000_000 * output_rate
    return round(total, 8)


def _llm_meta(model: str, purpose: str, usage: dict) -> dict:
    pricing = _pricing_for_model(model, purpose)
    actual_model = _normalize_model_name(model)
    return {
        "provider": "google",
        "model": actual_model,
        "pricing_model_family": pricing.get("pricing_model_family", ""),
        "pricing_source": pricing.get("pricing_source", _GEMINI_PRICING_SOURCE_VERSION),
        "input_tokens": usage.get("input_tokens", 0),
        "output_tokens": usage.get("output_tokens", 0),
        "cache_creation_input_tokens": 0,
        "cache_read_input_tokens": 0,
        "estimated_cost_usd": _estimate_cost_usd(actual_model, purpose, usage),
    }


def _generate_content(prompt: str, model: str, api_key: str, max_output_tokens: int = 1024) -> tuple[str, dict]:
    if not api_key:
        raise RuntimeError("google api key is required")
    model_name = _normalize_model_name(model)
    url = f"https://generativelanguage.googleapis.com/v1beta/models/{model_name}:generateContent"
    body = {
        "contents": [{"role": "user", "parts": [{"text": prompt}]}],
        "generationConfig": {
            "temperature": 0.2,
            "maxOutputTokens": max_output_tokens,
            "responseMimeType": "application/json",
        },
    }
    with httpx.Client(timeout=60.0) as client:
        resp = client.post(url, json=body, params={"key": api_key})
    if resp.status_code >= 400:
        raise RuntimeError(f"gemini generateContent failed status={resp.status_code} body={resp.text[:1000]}")
    data = resp.json()
    usage_meta = data.get("usageMetadata", {}) if isinstance(data, dict) else {}
    usage = {
        "input_tokens": int(usage_meta.get("promptTokenCount", 0) or 0),
        "output_tokens": int(usage_meta.get("candidatesTokenCount", 0) or 0),
    }
    candidates = data.get("candidates", []) if isinstance(data, dict) else []
    if not candidates:
        return "", usage
    parts = candidates[0].get("content", {}).get("parts", [])
    text = ""
    for p in parts:
        t = p.get("text")
        if isinstance(t, str):
            text += t
    return text.strip(), usage


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


def extract_facts(title: str | None, content: str, model: str, api_key: str) -> dict:
    prompt = f"""以下の記事本文から重要な事実を箇条書きで抽出してください。
事実は客観的かつ具体的に記述してください。
8〜18個程度にまとめてください。
記事本文が英語でも、出力は必ず自然な日本語にしてください。
固有名詞・製品名・会社名・API名は原文を尊重して必要に応じて英字のまま残して構いません。
JSON配列として返してください。例: ["事実1", "事実2"]

タイトル: {title or "（不明）"}

本文:
{content}
"""
    text, usage = _generate_content(prompt, model=model, api_key=api_key, max_output_tokens=1500)
    facts = _parse_json_string_array(text)
    return {"facts": facts, "llm": _llm_meta(model, "facts", usage)}


def summarize(
    title: str | None,
    facts: list[str],
    source_text_chars: int | None = None,
    model: str = "gemini-2.5-flash",
    api_key: str = "",
) -> dict:
    target_chars = _target_summary_chars(source_text_chars, facts)
    min_chars = _clamp_int(round(target_chars * 0.8), 160, 1000)
    max_chars = _clamp_int(round(target_chars * 1.2), 260, 1400)
    max_tokens = _summary_max_tokens(target_chars)
    facts_text = "\n".join(f"- {f}" for f in facts)
    prompt = f"""以下の事実リストをもとに、記事の要約を作成してください。
以下のJSON形式で返してください:
{{
  "summary": "{min_chars}〜{max_chars}字程度の要約",
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
- 箇条書きではなく、2〜4段落の文章でまとめる
- 要約の目標文字数は約{target_chars}字。短すぎる要約を避ける
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
    text, usage = _generate_content(prompt, model=model, api_key=api_key, max_output_tokens=max_tokens)
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
    return {
        "summary": str(data.get("summary", "")).strip(),
        "topics": [str(t) for t in topics],
        "score": _summary_composite_score(score_breakdown),
        "score_breakdown": score_breakdown,
        "score_reason": score_reason[:400],
        "score_policy_version": "v2",
        "llm": _llm_meta(model, "summary", usage),
    }


def compose_digest(digest_date: str, items: list[dict], model: str, api_key: str) -> dict:
    if not items:
        return {
            "subject": f"Sifto Digest - {digest_date}",
            "body": "本日のダイジェスト対象記事はありませんでした。",
            "llm": _llm_meta(model, "digest", {"input_tokens": 0, "output_tokens": 0}),
        }
    input_mode, digest_input = _build_digest_input_sections(items)
    prompt = f"""あなたはニュースダイジェスト編集者です。
以下の記事一覧をもとに、メール用のダイジェスト本文を日本語で作成してください。

要件:
- 当日分の全記事要約を踏まえて全体像をまとめる（記事を取りこぼさない）
- 読みやすく整理されていれば、本文は長めでもよい（目安 900〜2200字、必要なら超えて可）
- 本文は次の順序・構成で必ず作る:
  1) 全体サマリ（1〜3段落）
  2) 注目ポイント（5〜10個。各ポイントは1〜2文）
  3) その他のポイント（個数指定なし。箇条書き）
  4) 明日以降のフォローポイント（1段落）
  5) 締めの1文
- 誇張しない。与えられた情報だけで書く
- JSONで返す

形式:
{{
  "subject": "件名（40字程度）",
  "body": "メール本文（プレーンテキスト。改行を含めてよい）",
  "sections": {{
    "overall_summary": "1〜3段落",
    "highlights": ["ポイント1", "ポイント2"],
    "other_points": ["補足1", "補足2"],
    "follow_up": "明日以降のフォローポイント（1段落）",
    "closing": "締めの1文"
  }}
}}

digest_date: {digest_date}
items_count: {len(items)}
input_mode: {input_mode}
items:
{digest_input}
"""
    text, usage = _generate_content(prompt, model=model, api_key=api_key, max_output_tokens=2400)
    start = text.find("{")
    end = text.rfind("}") + 1
    try:
        data = json.loads(text[start:end])
    except Exception as e:
        snippet = text[:500].replace("\n", "\\n")
        raise RuntimeError(f"gemini compose_digest json parse failed: {e}; response_snippet={snippet}")
    subject = str(data.get("subject") or "").strip()
    body = str(data.get("body") or "").strip()
    if not subject or not body:
        snippet = text[:500].replace("\n", "\\n")
        raise RuntimeError(f"gemini compose_digest missing subject/body; response_snippet={snippet}")
    if len(body) < 80:
        raise RuntimeError(f"gemini compose_digest body too short: len={len(body)}")
    return {
        "subject": subject,
        "body": body,
        "llm": _llm_meta(model, "digest", usage),
    }


def compose_digest_cluster_draft(
    cluster_label: str,
    item_count: int,
    topics: list[str],
    source_lines: list[str],
    model: str,
    api_key: str,
) -> dict:
    prompt = f"""あなたはニュースダイジェストの下書き編集者です。
以下は同じ話題（クラスタ）に属する複数記事の要点メモです。重複をまとめ、事実ベースで読みやすいクラスタ下書きに整理してください。

要件:
- 与えられた内容のみ使う（推測しない）
- 重複をまとめる
- 重要な相違点があれば残す
- プレーンテキストで返す
- 箇条書き 3〜8 行程度
- JSONのみで返す

返却形式:
{{
  "draft_summary": "- ...\\n- ..."
}}

cluster_label: {cluster_label}
item_count: {item_count}
topics: {json.dumps(topics or [], ensure_ascii=False)}
source_lines:
{json.dumps(source_lines or [], ensure_ascii=False)}
"""
    text, usage = _generate_content(prompt, model=model, api_key=api_key, max_output_tokens=900)
    start = text.find("{")
    end = text.rfind("}") + 1
    try:
        data = json.loads(text[start:end])
    except Exception:
        data = {}
    summary = str(data.get("draft_summary") or "").strip()
    if not summary:
        summary = "\n".join(source_lines[:5])
    return {
        "draft_summary": summary,
        "llm": _llm_meta(model, "digest_cluster_draft", usage),
    }
