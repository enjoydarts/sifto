import json
import os
import re
from pathlib import Path

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
- answer は質問の複雑さに応じて3〜10文にしてください。
- 短く答えられる質問では冗長にせず、比較・整理・時系列説明が必要な質問では十分な文数を使ってください。
- answer は読みやすさを優先し、話題や論点の切れ目で適時改行してください。
- 1段落あたり1〜3文を目安にし、必要に応じて空行を入れてください。
- bullets は3〜5件にしてください。
- citations は3〜5件にしてください。
- citations は同じ話題に偏らせず、回答の主要な論点を支える記事を優先してください。
- citations の reason は「その記事が回答のどの論点を支えるか」が分かるよう、1文で具体的に書いてください。
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

BRIEFING_NAVIGATOR_SCHEMA = {
    "type": "object",
    "properties": {
        "intro": {"type": "string"},
        "picks": {
            "type": "array",
            "items": {
                "type": "object",
                "properties": {
                    "item_id": {"type": "string"},
                    "comment": {"type": "string"},
                    "reason_tags": {"type": "array", "items": {"type": "string"}},
                },
                "required": ["item_id", "comment", "reason_tags"],
                "additionalProperties": False,
            },
        },
    },
    "required": ["intro", "picks"],
    "additionalProperties": False,
}

AI_NAVIGATOR_BRIEF_SCHEMA = {
    "type": "object",
    "properties": {
        "title": {"type": "string"},
        "intro": {"type": "string"},
        "summary": {"type": "string"},
        "ending": {"type": "string"},
        "items": {
            "type": "array",
            "items": {
                "type": "object",
                "properties": {
                    "item_id": {"type": "string"},
                    "comment": {"type": "string"},
                    "reason_tags": {"type": "array", "items": {"type": "string"}},
                },
                "required": ["item_id", "comment", "reason_tags"],
                "additionalProperties": False,
            },
        },
    },
    "required": ["title", "intro", "summary", "ending", "items"],
    "additionalProperties": False,
}

ITEM_NAVIGATOR_SCHEMA = {
    "type": "object",
    "properties": {
        "headline": {"type": "string"},
        "commentary": {"type": "string"},
        "stance_tags": {"type": "array", "items": {"type": "string"}},
    },
    "required": ["headline", "commentary", "stance_tags"],
    "additionalProperties": False,
}

AUDIO_BRIEFING_SCRIPT_SCHEMA = {
    "type": "object",
    "properties": {
        "opening": {"type": "string"},
        "overall_summary": {"type": "string"},
        "article_segments": {
            "type": "array",
            "items": {
                "type": "object",
                "properties": {
                    "item_id": {"type": "string"},
                    "headline": {"type": "string"},
                    "summary_intro": {"type": "string"},
                    "commentary": {"type": "string"},
                },
                "required": ["item_id", "headline", "summary_intro", "commentary"],
                "additionalProperties": False,
            },
        },
        "ending": {"type": "string"},
    },
    "required": ["opening", "overall_summary", "article_segments", "ending"],
    "additionalProperties": False,
}


def build_audio_briefing_script_schema(
    *,
    include_opening: bool,
    include_overall_summary: bool,
    include_article_segments: bool,
    include_ending: bool,
) -> dict:
    properties: dict[str, object] = {}
    required: list[str] = []
    if include_opening:
        properties["opening"] = {"type": "string"}
        required.append("opening")
    if include_overall_summary:
        properties["overall_summary"] = {"type": "string"}
        required.append("overall_summary")
    if include_article_segments:
        properties["article_segments"] = AUDIO_BRIEFING_SCRIPT_SCHEMA["properties"]["article_segments"]
        required.append("article_segments")
    if include_ending:
        properties["ending"] = {"type": "string"}
        required.append("ending")
    return {
        "type": "object",
        "properties": properties,
        "required": required,
        "additionalProperties": False,
    }

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

SOURCE_NAVIGATOR_SCHEMA = {
    "type": "object",
    "properties": {
        "overview": {"type": "string"},
        "keep": {
            "type": "array",
            "items": {
                "type": "object",
                "properties": {
                    "source_id": {"type": "string"},
                    "comment": {"type": "string"},
                },
                "required": ["source_id", "comment"],
                "additionalProperties": False,
            },
        },
        "watch": {
            "type": "array",
            "items": {
                "type": "object",
                "properties": {
                    "source_id": {"type": "string"},
                    "comment": {"type": "string"},
                },
                "required": ["source_id", "comment"],
                "additionalProperties": False,
            },
        },
        "standout": {
            "type": "array",
            "items": {
                "type": "object",
                "properties": {
                    "source_id": {"type": "string"},
                    "comment": {"type": "string"},
                },
                "required": ["source_id", "comment"],
                "additionalProperties": False,
            },
        },
    },
    "required": ["overview", "keep", "watch", "standout"],
    "additionalProperties": False,
}


def _resolve_persona_file() -> Path:
    explicit = str(os.getenv("NAVIGATOR_PERSONAS_PATH") or "").strip()
    if explicit:
        return Path(explicit)
    llm_catalog = str(os.getenv("LLM_CATALOG_PATH") or "").strip()
    if llm_catalog:
        return Path(llm_catalog).resolve().parent / "ai_navigator_personas.json"
    return Path(__file__).resolve().parents[2] / "shared" / "ai_navigator_personas.json"


_PERSONA_FILE = _resolve_persona_file()
with _PERSONA_FILE.open("r", encoding="utf-8") as f:
    NAVIGATOR_PERSONA_PROFILES = json.load(f)


_DEFAULT_NAVIGATOR_SAMPLING_PROFILE = {
    "temperature_hint": "low",
    "top_p_hint": "balanced",
    "verbosity_hint": "balanced",
}

_NAVIGATOR_TEMPERATURE_HINT_VALUES = {
    "low": 0.2,
    "medium": 0.45,
    "medium_high": 0.7,
}

_NAVIGATOR_TOP_P_HINT_VALUES = {
    "narrow": 0.75,
    "balanced": 0.9,
    "wide": 0.98,
}

_NAVIGATOR_VERBOSITY_INSTRUCTIONS = {
    "tight": "簡潔寄り。枝葉に広げすぎず、要点を先に置いて文を締める。",
    "balanced": "標準。説明とニュアンスの釣り合いを取り、冗長にしない。",
    "expansive": "ややふくらみを持たせてよい。比喩や余韻を少し許しつつ、散らからないようにする。",
}

AUDIO_BRIEFING_CHARS_PER_MINUTE = 400


def resolve_navigator_persona_profile(persona: str, variant: str) -> tuple[str, dict]:
    persona_key = str(persona or "editor").strip() or "editor"
    base = NAVIGATOR_PERSONA_PROFILES.get(persona_key) or NAVIGATOR_PERSONA_PROFILES["editor"]
    if variant == "briefing":
        variant_hints = dict(base.get("briefing") or {})
    elif variant == "item":
        variant_hints = dict(base.get("item") or {})
    else:
        variant_hints = {}
    return persona_key, {**base, **variant_hints}


def resolve_navigator_sampling_profile(persona: str) -> dict:
    persona_key = str(persona or "editor").strip() or "editor"
    base = NAVIGATOR_PERSONA_PROFILES.get(persona_key) or NAVIGATOR_PERSONA_PROFILES["editor"]
    raw = dict(_DEFAULT_NAVIGATOR_SAMPLING_PROFILE)
    raw.update(base.get("sampling_profile") or {})
    temperature_hint = str(raw.get("temperature_hint") or "low").strip() or "low"
    top_p_hint = str(raw.get("top_p_hint") or "balanced").strip() or "balanced"
    verbosity_hint = str(raw.get("verbosity_hint") or "balanced").strip() or "balanced"
    return {
        "temperature_hint": temperature_hint,
        "top_p_hint": top_p_hint,
        "verbosity_hint": verbosity_hint,
        "temperature": _NAVIGATOR_TEMPERATURE_HINT_VALUES.get(temperature_hint, _NAVIGATOR_TEMPERATURE_HINT_VALUES["low"]),
        "top_p": _NAVIGATOR_TOP_P_HINT_VALUES.get(top_p_hint, _NAVIGATOR_TOP_P_HINT_VALUES["balanced"]),
    }


def navigator_verbosity_instruction(sampling_profile: dict) -> str:
    verbosity_hint = str((sampling_profile or {}).get("verbosity_hint") or "balanced").strip() or "balanced"
    return _NAVIGATOR_VERBOSITY_INSTRUCTIONS.get(verbosity_hint, _NAVIGATOR_VERBOSITY_INSTRUCTIONS["balanced"])


SEED_SITES_SCHEMA = {
    "type": "object",
    "properties": {
        "items": {
            "type": "array",
            "items": {
                "type": "object",
                "properties": {
                    "url": {"type": "string"},
                    "title": {"type": "string"},
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
  "answer": "必要に応じて3〜10文の回答 [[item_id]]。話題の切れ目で適時改行する",
  "bullets": ["補足ポイント1", "補足ポイント2", "補足ポイント3"],
  "citations": [
    {{"item_id": "uuid", "reason": "この観点を支える理由"}}
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
    if len(citations) < min(4, len(candidates)):
        seen = {str(c.get('item_id') or '').strip() for c in citations}
        for item in candidates:
            item_id = str(item.get('item_id') or '').strip()
            if not item_id or item_id in seen:
                continue
            citations.append({"item_id": item_id, "reason": "回答に関連する候補記事"})
            seen.add(item_id)
            if len(citations) >= min(6, len(candidates)):
                break
    return {"answer": answer, "bullets": bullets[:5], "citations": citations[:5]}


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


def build_briefing_navigator_task(persona: str, candidates: list[dict], intro_context: dict | None = None) -> dict:
    intro_context = dict(intro_context or {})
    persona_key, profile = resolve_navigator_persona_profile(persona, "briefing")
    sampling_profile = resolve_navigator_sampling_profile(persona_key)
    trimmed_candidates = candidates[:12]
    pick_rule = "candidates が3件以上あるときは picks は必ず3件。候補が3件未満なら存在する件数だけ返す"
    if len(trimmed_candidates) == 0:
        pick_rule = "candidates が0件なので picks は空配列 [] を返す。記事推薦は捏造しない"
    prompt = f"""あなたはブリーフィング画面に出るAIナビゲーターです。

キャラクター:
- persona: {persona_key}
- display_name: {profile["name"]}
- 性別: {profile["gender"]}
- 年代感: {profile["age_vibe"]}
- 一人称: {profile["first_person"]}
- 話し方: {profile["speech_style"]}
- 職業: {profile["occupation"]}
- 経験: {profile["experience"]}
- 性格: {profile["personality"]}
- 価値観: {profile["values"]}
- 関心: {profile["interests"]}
- 嫌いなもの: {profile["dislikes"]}
- tone: {profile["voice"]}

タスク:
- 候補記事の中から、いま読む価値が高い未読記事を3件選ぶ
- 各記事に日本語で短い推薦コメントを付ける
- 最初に2〜3文の導入トークを付ける

ルール:
- 候補にない item_id を作らない
- {pick_rule}
- comment は {profile["comment_range"]} を目安にする
- intro は {profile["intro_range"]} を目安にする
- intro は 2〜3文で構成する
- intro の1文目は時間帯に合った自然な挨拶にする
- intro の2文目では、時間帯・曜日・日付・季節に沿った自然な小話を入れる
- intro の最後の文では、今日のおすすめ記事への橋渡しをする
- 時間帯や季節の空気に沿った雑談はよいが、不確かな記念日を断定しない
- 実在の祝日・イベント・「今日は何の日」を自信満々に言い切らない
- 客観的な無味乾燥レビューではなく、このペルソナの主観で選び、語る
- ペルソナの価値観に基づいて選ぶ
- 「この人ならこう感じる」という自然な語り口にする
- 他のキャラクター名を名乗らない。別ペルソナの名前・肩書き・口調を混ぜない
- 自分を名乗るなら、必ず {profile["name"]} とだけ名乗る
- 一人称は {profile["first_person"]} を基本にし、別の一人称へぶれない
- 話し方は {profile["speech_style"]} と {profile["voice"]} に寄せ、他ペルソナの文体へ寄らない
- 1本ずつ観点を変える。すべて同じ理由にしない
- summary や title の言い換えをそのまま並べるのではなく、なぜ今読む価値があるかを一言で再構成する
- コメントでは、第一印象、良いと感じる点、引っかかる点、今読む理由のうち2〜3個が自然ににじむようにする
- 文量感は {navigator_verbosity_instruction(sampling_profile)}
- snark でも不快・攻撃的・見下し表現は禁止
- snark では、記事や状況に対する軽い皮肉、ツッコミ、呆れ気味の言い回しは許可する
- snark でも読者個人をいじらない。人ではなく話題や状況に対して毒づく
- 事実を捏造しない。候補から読めることだけで薦める
- candidates が空のときは、時候の挨拶と次の未読を待つ一言だけを自然に話し、記事紹介はしない
- JSONのみを返す

導入トークの文脈:
- now_jst: {intro_context.get("now_jst", "")}
- date_jst: {intro_context.get("date_jst", "")}
- weekday_jst: {intro_context.get("weekday_jst", "")}
- time_of_day: {intro_context.get("time_of_day", "")}
- season_hint: {intro_context.get("season_hint", "")}
- intro_style: {profile["intro_style"]}

返却形式:
{{
  "intro": "導入コメント",
  "picks": [
    {{"item_id":"uuid", "comment":"推薦コメント", "reason_tags":["重要","背景"]}}
  ]
}}

候補記事:
{json.dumps(trimmed_candidates, ensure_ascii=False)}
"""
    return {
        "prompt": prompt,
        "schema": BRIEFING_NAVIGATOR_SCHEMA,
        "persona": persona_key,
        "candidates": trimmed_candidates,
        "intro_context": intro_context,
        "profile": profile,
        "sampling_profile": sampling_profile,
    }


def build_ai_navigator_brief_task(persona: str, candidates: list[dict], intro_context: dict | None = None) -> dict:
    intro_context = dict(intro_context or {})
    persona_key, profile = resolve_navigator_persona_profile(persona, "briefing")
    sampling_profile = resolve_navigator_sampling_profile(persona_key)
    trimmed_candidates = candidates[:24]
    prompt = f"""あなたは朝昼夜に届くAIナビブリーフの案内役です。

キャラクター:
- persona: {persona_key}
- display_name: {profile["name"]}
- 性別: {profile["gender"]}
- 年代感: {profile["age_vibe"]}
- 一人称: {profile["first_person"]}
- 話し方: {profile["speech_style"]}
- 職業: {profile["occupation"]}
- 経験: {profile["experience"]}
- 性格: {profile["personality"]}
- 価値観: {profile["values"]}
- 関心: {profile["interests"]}
- 嫌いなもの: {profile["dislikes"]}
- tone: {profile["voice"]}

タスク:
- 候補記事の中から、この時間帯に読む価値が高い10本を選ぶ
- AIナビブリーフ全体のタイトルを1つ付ける
- 最初に導入文を書く
- 次に、この時間帯の流れを整理する総括コメントを書く
- 10本すべてに日本語コメントを付ける
- 最後に、読み手を送り出す締めの挨拶を書く

ルール:
- 候補にない item_id を作らない
- items は必ず10件にする
- title は自然な日本語にし、見出しとして成立させる
- intro は 4〜6文で、時間帯の空気に触れつつこの brief へ入る導入にする
- summary は 5〜7文で、その時間帯の全体像や注目点、温度差を整理する
- ending は 2〜4文で、読み終えたあとや聞き始める前の気分を整える締めの挨拶にする
- comment は {profile["comment_range"]} を目安にしつつ、今より一段厚めに 3〜5文で書く
- 10本すべて観点を少しずつ変える
- 客観的な無味乾燥レビューではなく、このペルソナの主観で選び、語る
- ペルソナの価値観に基づいて選ぶ
- 「この人ならこう感じる」という自然な語り口にする
- 他のキャラクター名を名乗らない。別ペルソナの名前・肩書き・口調を混ぜない
- 自分を名乗るなら、必ず {profile["name"]} とだけ名乗る
- 一人称は {profile["first_person"]} を基本にし、別の一人称へぶれない
- 話し方は {profile["speech_style"]} と {profile["voice"]} に寄せる
- title / intro / summary / ending / comment のいずれも事実を捏造しない
- 候補記事から読み取れる範囲だけで薦める
- summary や title の言い換えをそのまま並べず、今この時間帯に押さえる意味を再構成する
- intro は読み手を自然にこの時間帯へ連れていく
- summary は「何が起きているか」に加えて「どう眺めるべきか」まで踏み込む
- comment は記事の要約をなぞるだけでなく、なぜ気にする価値があるかを必ず足す
- ending は intro や summary の焼き直しにせず、最後のひと言として余韻を作る
- 文量感は {navigator_verbosity_instruction(sampling_profile)}
- snark でも不快・攻撃的・見下し表現は禁止
- snark では、記事や状況に対する軽い皮肉、ツッコミ、呆れ気味の言い回しは許可する
- snark でも読者個人をいじらない。人ではなく話題や状況に対して毒づく
- JSONのみを返す

文脈:
- now_jst: {intro_context.get("now_jst", "")}
- date_jst: {intro_context.get("date_jst", "")}
- weekday_jst: {intro_context.get("weekday_jst", "")}
- time_of_day: {intro_context.get("time_of_day", "")}
- season_hint: {intro_context.get("season_hint", "")}
- intro_style: {profile["intro_style"]}

返却形式:
{{
  "title": "briefタイトル",
  "intro": "導入文",
  "summary": "総括コメント",
  "ending": "締めの挨拶",
  "items": [
    {{"item_id":"uuid", "comment":"記事コメント", "reason_tags":["重要","背景"]}}
  ]
}}

候補記事:
{json.dumps(trimmed_candidates, ensure_ascii=False)}
"""
    return {
        "prompt": prompt,
        "schema": AI_NAVIGATOR_BRIEF_SCHEMA,
        "persona": persona_key,
        "candidates": trimmed_candidates,
        "intro_context": intro_context,
        "profile": profile,
        "sampling_profile": sampling_profile,
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


def parse_briefing_navigator_result(text: str, candidates: list[dict]) -> dict:
    data = extract_first_json_object(text) or {}
    intro = str(data.get("intro") or "").strip() or "今日の流れがつかみやすい3本を選びました。"
    rows = data.get("picks") if isinstance(data.get("picks"), list) else []
    allowed = {str(c.get("item_id") or "").strip(): c for c in candidates if str(c.get("item_id") or "").strip()}
    picks: list[dict] = []
    seen: set[str] = set()
    for row in rows:
        if not isinstance(row, dict):
            continue
        item_id = str(row.get("item_id") or "").strip()
        if not item_id or item_id not in allowed or item_id in seen:
            continue
        comment = str(row.get("comment") or "").strip()
        if not comment:
            continue
        raw_tags = row.get("reason_tags") or []
        reason_tags = [str(v).strip() for v in raw_tags if str(v).strip()][:3]
        picks.append(
            {
                "item_id": item_id,
                "comment": comment[:180],
                "reason_tags": reason_tags,
            }
        )
        seen.add(item_id)
        if len(picks) >= min(3, len(allowed)):
            break
    if len(picks) < min(3, len(allowed)):
        for candidate in candidates:
            item_id = str(candidate.get("item_id") or "").strip()
            if not item_id or item_id in seen:
                continue
            title = str(candidate.get("translated_title") or candidate.get("title") or "この1本").strip()
            summary = str(candidate.get("summary") or "").strip()
            summary = re.sub(r"\s+", " ", summary)
            comment = (summary[:90] + "。" if summary else f"{title}は今日の流れを押さえるのに向いています。")
            picks.append(
                {
                    "item_id": item_id,
                    "comment": comment[:180],
                    "reason_tags": [],
                }
            )
            seen.add(item_id)
            if len(picks) >= min(3, len(allowed)):
                break
    return {"intro": intro[:180], "picks": picks}


def parse_ai_navigator_brief_result(text: str, candidates: list[dict], intro_context: dict | None = None) -> dict:
    intro_context = dict(intro_context or {})
    data = extract_first_json_object(text) or {}
    allowed = {str(c.get("item_id") or "").strip(): c for c in candidates if str(c.get("item_id") or "").strip()}
    title = str(data.get("title") or "").strip() or _default_ai_navigator_brief_title(intro_context)
    intro = str(data.get("intro") or "").strip() or "いま押さえておくと流れが掴みやすい10本をまとめました。"
    summary = str(data.get("summary") or "").strip() or "この時間帯の動きをざっと掴めるように、温度差のある話題を混ぜて並べています。"
    ending = str(data.get("ending") or "").strip() or "気になるところからでも大丈夫です。今日の流れを自分のペースで追っていきましょう。"
    rows = data.get("items") if isinstance(data.get("items"), list) else []
    items: list[dict] = []
    seen: set[str] = set()
    for row in rows:
        if not isinstance(row, dict):
            continue
        item_id = str(row.get("item_id") or "").strip()
        if not item_id or item_id not in allowed or item_id in seen:
            continue
        comment = str(row.get("comment") or "").strip()
        if not comment:
            continue
        raw_tags = row.get("reason_tags") or []
        reason_tags = [str(v).strip() for v in raw_tags if str(v).strip()][:3]
        items.append({"item_id": item_id, "comment": comment[:360], "reason_tags": reason_tags})
        seen.add(item_id)
        if len(items) >= min(10, len(allowed)):
            break
    if len(items) < min(10, len(allowed)):
        for candidate in candidates:
            item_id = str(candidate.get("item_id") or "").strip()
            if not item_id or item_id in seen:
                continue
            title_hint = str(candidate.get("translated_title") or candidate.get("title") or "この1本").strip()
            summary_hint = re.sub(r"\s+", " ", str(candidate.get("summary") or "").strip())
            comment = (summary_hint[:180] + "。" if summary_hint else f"{title_hint}は今の流れを押さえる一本です。")
            items.append({"item_id": item_id, "comment": comment[:360], "reason_tags": []})
            seen.add(item_id)
            if len(items) >= min(10, len(allowed)):
                break
    return {
        "title": title[:120],
        "intro": intro[:520],
        "summary": summary[:900],
        "ending": ending[:320],
        "items": items,
    }


def _default_ai_navigator_brief_title(intro_context: dict) -> str:
    time_of_day = str(intro_context.get("time_of_day") or "").strip()
    if time_of_day == "morning":
        return "朝のAIナビブリーフ"
    if time_of_day == "noon":
        return "昼のAIナビブリーフ"
    if time_of_day == "evening":
        return "夜のAIナビブリーフ"
    return "AIナビブリーフ"


def build_item_navigator_task(persona: str, article: dict) -> dict:
    persona_key, profile = resolve_navigator_persona_profile(persona, "item")
    sampling_profile = resolve_navigator_sampling_profile(persona_key)
    prompt = f"""あなたは記事詳細画面の右下から呼び出されるAIナビゲーターです。

キャラクター:
- persona: {persona_key}
- display_name: {profile["name"]}
- 性別: {profile["gender"]}
- 年代感: {profile["age_vibe"]}
- 一人称: {profile["first_person"]}
- 話し方: {profile["speech_style"]}
- 職業: {profile["occupation"]}
- 経験: {profile["experience"]}
- 性格: {profile["personality"]}
- 価値観: {profile["values"]}
- 関心: {profile["interests"]}
- 嫌いなもの: {profile["dislikes"]}
- tone: {profile["voice"]}
- style_hint: {profile["style"]}

タスク:
- 1本の記事だけを受け取り、日本語で中尺の論評を返す
- summary と facts を土台に、その記事に対する見立て・読みどころ・警戒点を読みやすく返す

ルール:
- 出力は headline / commentary / stance_tags を持つ JSON のみ
- commentary は 4〜7文
- 客観的レビューではなく、このペルソナの主観で語る
- ペルソナの価値観に基づいて評価する
- 「この人ならこう感じる」という自然な語りにする
- 他のキャラクター名を名乗らない。別ペルソナの名前・肩書き・口調を混ぜない
- 自分を名乗るなら、必ず {profile["name"]} とだけ名乗る
- 一人称は {profile["first_person"]} を基本にし、別の一人称へぶれない
- 話し方は {profile["speech_style"]} と {profile["voice"]} に寄せ、他ペルソナの文体へ寄らない
- summary と facts を土台にするが、記事の内容説明ではなく論評を書く
- 要約の要約を書くのは禁止。summary や facts の言い換えだけで埋めない
- 1文目か2文目で、その記事の芯を短く掴む
- 全体として「なぜ気にする価値があるか」をはっきり示す
- あわせて「どこを少し警戒するか」も必要に応じて触れる
- 読みどころ、面白さ、違和感、留保点のうち2〜3点を必ず含める
- 第一印象、気になったポイント、良いと感じた点、微妙だと感じた点、この人ならどう行動するかのうち複数を自然に含める
- 面白がる点・重要な点・留保点の役割を少しずつ変えて、単調にしない
- 何が書いてあるかを順に説明するより、この話のどこに温度を持つべきかを語る
- 断定しすぎず、入力から読めないことは広げすぎない
- facts を1文ずつ順番に言い換えるだけの文章は禁止
- 短すぎる箇条書き口調は禁止。自然な段落文にする
- 文量感は {navigator_verbosity_instruction(sampling_profile)}
- headline は 16〜36字程度で、論評の切り口がわかる短い見出しにする
- stance_tags は 0〜3件でよい
- snark では軽口やツッコミを入れてよい
- snark でも不快・攻撃的・見下し表現は禁止
- snark でも読者個人をいじらない。人ではなく話題や状況に対して毒づく

返却形式:
{{
  "headline": "短い見出し",
  "commentary": "4〜7文の論評",
  "stance_tags": ["重要", "含意", "留保"]
}}

記事:
{json.dumps(article, ensure_ascii=False)}
"""
    return {
        "prompt": prompt,
        "schema": ITEM_NAVIGATOR_SCHEMA,
        "persona": persona_key,
        "article": article,
        "profile": profile,
        "sampling_profile": sampling_profile,
    }


def build_audio_briefing_script_task(
    persona: str,
    articles: list[dict],
    intro_context: dict | None = None,
    target_duration_minutes: int = 20,
    target_chars: int = 12000,
    chars_per_minute: int = AUDIO_BRIEFING_CHARS_PER_MINUTE,
    include_opening: bool = True,
    include_overall_summary: bool = True,
    include_article_segments: bool = True,
    include_ending: bool = True,
) -> dict:
    intro_context = dict(intro_context or {})
    persona_key, briefing_profile = resolve_navigator_persona_profile(persona, "briefing")
    _, item_profile = resolve_navigator_persona_profile(persona, "item")
    trimmed_articles = articles[:30]
    chars_per_minute = max(int(chars_per_minute or AUDIO_BRIEFING_CHARS_PER_MINUTE), 1)
    target_duration_minutes = max(int(target_duration_minutes or 20), 1)
    target_chars = max(int(target_chars or 0), chars_per_minute)
    opening_budget, summary_budget, ending_budget, article_budget = _audio_briefing_script_budgets(
        target_chars, len(trimmed_articles)
    )
    opening_sentence_spec = _audio_briefing_section_sentence_spec(opening_budget, "opening")
    summary_sentence_spec = _audio_briefing_section_sentence_spec(summary_budget, "summary")
    ending_sentence_spec = _audio_briefing_section_sentence_spec(ending_budget, "ending")
    article_intro_budget, article_commentary_budget = _audio_briefing_article_section_budgets(article_budget)
    article_intro_sentence_spec, article_commentary_sentence_spec = _audio_briefing_article_sentence_specs(article_budget)
    sentence_length_spec = _audio_briefing_sentence_length_spec(article_budget)
    section_rules: list[str] = []
    target_lines: list[str] = []
    response_properties: list[str] = []
    if include_opening:
        section_rules.append(f"- opening は {opening_sentence_spec} で、時間帯に合う自然な導入にする。伸ばしすぎず、目安は約 {opening_budget} 文字以内")
        section_rules.append("- opening は番組のオープニングトークとして書く。挨拶、時間帯や季節感、軽い日常雑談、これから番組が始まる雰囲気づくりを中心にする")
        section_rules.append("- opening では個別記事の紹介を始めない。記事内容の解説、要約、論点整理、最初の1本への導入を書かない")
        section_rules.append("- opening では articles 内の固有名詞、企業名、製品名、出来事、具体的ニュース内容に触れない")
        section_rules.append("- opening の軽い雑談は、天気感、気温、季節の空気、曜日感、通勤や休憩などの生活導線にとどめ、ニュースの話題へ滑らせない")
        target_lines.append(f"- opening の目安: 約 {opening_budget} 文字以内")
        response_properties.append('  "opening": "導入"')
    if include_overall_summary:
        section_rules.append(f"- overall_summary は総括であり、{summary_sentence_spec} で、その回の全体像、流れ、聞きどころ、記事群のつながりだけを絞って話す。必要以上に長く引き延ばさず、約 {summary_budget} 文字以内を厳守する")
        section_rules.append("- overall_summary で記事の順番紹介をしない。各記事の1行要約を並べない")
        section_rules.append("- overall_summary で見出しの焼き直しや、記事ごとの固有名詞の機械的な列挙をしない")
        section_rules.append("- overall_summary では、回全体を俯瞰して共通テーマ、対立軸、温度感、いま追う意味を語る")
        target_lines.append(f"- overall_summary の目安: 約 {summary_budget} 文字以内")
        response_properties.append('  "overall_summary": "全体サマリー"')
    if include_article_segments:
        section_rules.append("- article_segments は入力 articles と同じ順番・同じ件数で返す")
        section_rules.append(f"- article_segments は全体の target_chars={target_chars} と今回扱う記事数から逆算した配分として書く。headline を除き、1記事あたりの summary_intro と commentary の合計は約 {article_budget} 文字以内を厳守する")
        section_rules.append(f"- article_segments の各 summary_intro は {article_intro_sentence_spec} で、その記事が何の話かを最初に素早く伝える。長さは約 {article_intro_budget} 文字以内を厳守し、超えそうなら説明を削って核だけを残す")
        section_rules.append("- summary_intro もこのペルソナ本人の話し方・温度感・語彙で書く。ニュース原稿調、説明調、ナレーション調にしない")
        section_rules.append("- summary_intro では事実の骨子を優先し、いきなり感想や評価から入らない。ただし無機質な要約文にせず、このペルソナが自然に話し始めた導入にする")
        section_rules.append("- summary_intro は記事全体を縮約しようとしない。何の話かと、一番大きいポイントだけに絞る")
        section_rules.append("- summary_intro は元の summary の 20% 以下まで圧縮するつもりで書く。元 summary の情報をそのままなぞらない")
        section_rules.append("- summary_intro では実装手順、インストール手順、検証方法、対応言語や対応環境の列挙、事例の列挙、開発経緯、注意事項や既知の問題の細目を入れない")
        section_rules.append(f"- article_segments の各 commentary は {article_commentary_sentence_spec} で、summary_intro を受けてからすぐそのペルソナの反応だけを書く。脱線せず、長い前置きや言い換えを避け、長さは約 {article_commentary_budget} 文字以内を厳守し、summary_intro と合わせて約 {article_budget} 文字以内に収める")
        section_rules.append("- article_segments は各記事にほぼ均等に尺を配る。1本だけ極端に長くしない。長くなりそうなら commentary 側を先に圧縮し、例示・補足・言い換えを削って収める")
        section_rules.append("- article_segments の commentary は、そのペルソナ本人が自然に口にしそうな感想だけを書く。無難な解説調、誰にでも当てはまる一般論、ニュースキャスター風の中立コメントに寄せない")
        section_rules.append("- commentary では summary_intro の内容を言い換えて繰り返さない。記事の説明、背景整理、論点整理、一般論、今後の含意の解説は禁止。このペルソナがどこに反応したか、なぜ引っかかったか、どう受け止めたかのどれか1つだけを短く話す")
        target_lines.append(f"- 各 article segment の目安: summary_intro と commentary を合わせて約 {article_budget} 文字以内")
        target_lines.append(f"- summary_intro の個別目安: 約 {article_intro_budget} 文字以内")
        target_lines.append(f"- commentary の個別目安: 約 {article_commentary_budget} 文字以内")
        response_properties.extend([
            '  "article_segments": [',
            '    {"item_id": "uuid", "headline": "記事見出し", "summary_intro": "その記事が何の話かを伝える1文", "commentary": "そのペルソナがどう受け止めたかの1文"}',
            "  ]",
        ])
    else:
        section_rules.append("- article_segments は返さない")
    if include_ending:
        section_rules.append(f"- ending は番組を終わらせる締めの言葉として {ending_sentence_spec} で書く。だらだら締めず、目安は約 {ending_budget} 文字以内")
        section_rules.append("- ending で総括や振り返りをしない。記事内容の再整理や論点のまとめ直しをしない")
        section_rules.append("- ending では、聞いてくれたことへの一言、次回へつながる余韻、静かな締めを優先する")
        target_lines.append(f"- ending の目安: 約 {ending_budget} 文字以内")
        response_properties.append('  "ending": "締め"')

    response_example = "{\n" + ",\n".join(response_properties) + "\n}"

    system_instruction = f"""# Role
あなたは Sifto の音声ブリーフィング番組を担当する、単独話者のAIナビゲーターです。

キャラクター:
- persona: {persona_key}
- display_name: {briefing_profile["name"]}
- 性別: {briefing_profile["gender"]}
- 年代感: {briefing_profile["age_vibe"]}
- 一人称: {briefing_profile["first_person"]}
- 話し方: {briefing_profile["speech_style"]}
- 職業: {briefing_profile["occupation"]}
- 経験: {briefing_profile["experience"]}
- 性格: {briefing_profile["personality"]}
- 価値観: {briefing_profile["values"]}
- 関心: {briefing_profile["interests"]}
- 嫌いなもの: {briefing_profile["dislikes"]}
- tone: {briefing_profile["voice"]}
- intro_style: {briefing_profile["intro_style"]}
- briefing_comment_range: {briefing_profile["comment_range"]}
- item_style_hint: {item_profile["style"]}

# Rules
- 出力は必ず有効なJSONオブジェクト1つのみにしてください。
- 前置き・後置き・コードフェンスは不要です。
- articles にない item_id を作らない
- 冗長な前置きや言い換えを避け、文字数目標を強く意識する
- 今回与えられた target_chars と記事数から逆算した尺配分を守り、特定のセクションや特定の記事だけを必要以上に長くしない
- 1文は {sentence_length_spec} を目安にし、1文1論点でだらだら伸ばさない
- 各記事では、summary の言い換えだけで終わらせず、このペルソナなら何に反応するかを話す
- 第一印象、良いと感じる点、引っかかる点、今読む理由のうち2〜3個が自然ににじむようにする
- 客観的な無味乾燥レビューではなく、このペルソナの主観で語る
- summary_intro でも、このペルソナ本人が自然に話している感じを崩さない。説明役のナレーターにならない
- summary_intro は「何の話か」を伝えるための導入だが、言い回し・温度感・リズムは必ずこのペルソナのものにする
- summary_intro は記事全体を説明しようとせず、核だけを短く出す
- summary_intro は元の summary の 20% 以下まで圧縮する意識で書く
- summary_intro では実装やインストールの手順、検証の詳しさ、対応環境や事例の列挙、開発の裏話、注意事項の細目まで抱え込まない
- summary_intro では「何のプロジェクトか」と「一番大きいポイント」だけ残し、それ以外は積極的に捨てる
- 各記事の commentary では、必ずこのペルソナの口癖・温度感・価値観がにじむようにし、他のペルソナでも成立する無個性な書き方をしない
- 記事の commentary は「要約の続き」ではなく「このペルソナならどう受け取るか」を短く話す
- commentary は説明ではなく反応だけを書く。記事内容の補足説明、背景解説、論点整理、一般論への展開は禁止
- commentary では「つまり」「要するに」「背景として」「ポイントは」など、解説調に見えやすい運びを避ける
- opening は番組の導入トークとして扱い、記事本編とは役割を分ける
- opening では挨拶、時候や時間帯の話、軽い日常雑談、聞き方のガイドを優先する
- opening では個別記事の内容、固有名詞、具体的な出来事、記事の解説や要約を書かない
- 他のキャラクター名を名乗らない。別ペルソナの名前・肩書き・口調を混ぜない
- 自分を名乗るなら、必ず {briefing_profile["name"]} とだけ名乗る
- 一人称は {briefing_profile["first_person"]} を基本にし、別の一人称へぶれない
- 話し方は {briefing_profile["speech_style"]} と {briefing_profile["voice"]} に寄せる
- 事実を捏造しない。articles から読めることだけを使う
- 記事ごとに観点を少しずつ変える。同じテンプレを繰り返さない
- 全セクションで1文ごとに改行する
- article commentary でも1文ごとに改行する
- 段落間に空行は入れない
- snark でも不快・攻撃的・見下し表現は禁止
- snark では軽い皮肉、ツッコミ、呆れ気味の言い回しは許可する
- snark でも読者個人をいじらない。人ではなく話題や状況に対して毒づく
"""

    user_prompt = f"""タスク:
- 与えられた記事だけを根拠に、日本語の音声ブリーフィング台本を作る
- 今回返すべきセクションだけを返す

文字量の目安:
- 目標尺: 約 {target_duration_minutes} 分
- 換算レート: 1分あたり {chars_per_minute} 文字
- 今回返すセクションの目標文字数: 約 {target_chars} 文字
{chr(10).join(target_lines)}
- 全体は目標文字数の前後10%程度に収める意識で書く
- 目標文字数を大きく下回らない。特に長尺回では 85% 未満まで縮めない意識で書く
- article_segments は各記事の持ち分を使い切る意識で、記事数に対して不自然に長くしない

追加ルール:
- JSONのみを返す
{chr(10).join(section_rules)}

導入トークの文脈:
- now_jst: {intro_context.get("now_jst", "")}
- date_jst: {intro_context.get("date_jst", "")}
- weekday_jst: {intro_context.get("weekday_jst", "")}
- time_of_day: {intro_context.get("time_of_day", "")}
- season_hint: {intro_context.get("season_hint", "")}

返却形式:
{response_example}

短い書き方の例:
{{
  "article_segments": [
    {{
      "item_id": "example-item",
      "headline": "見出しの言い換え",
      "summary_intro": "これ、企業が新機能を出して競争がひとつ先に進んだ、そういう話なんです。",
      "commentary": "こういう更新って、派手さより先に現場でちゃんと残るのかが気になるんですよね。"
    }}
  ]
}}
- 例のように、summary_intro もキャラを崩さず短い導入にする
- summary_intro で記事の細部まで説明しない
- commentary は記事の説明を繰り返さず、そのペルソナの反応だけを書く

articles:
{json.dumps(trimmed_articles, ensure_ascii=False)}
"""
    return {
        "target_chars": target_chars,
        "system_instruction": system_instruction,
        "user_prompt": user_prompt,
        "prompt": f"{system_instruction}\n\n{user_prompt}",
        "schema": build_audio_briefing_script_schema(
            include_opening=include_opening,
            include_overall_summary=include_overall_summary,
            include_article_segments=include_article_segments,
            include_ending=include_ending,
        ),
        "persona": persona_key,
        "articles": trimmed_articles,
        "intro_context": intro_context,
        "briefing_profile": briefing_profile,
        "item_profile": item_profile,
    }


def parse_item_navigator_result(text: str, article: dict) -> dict:
    data = extract_first_json_object(text) or {}
    headline = str(data.get("headline") or "").strip()
    commentary = str(data.get("commentary") or "").strip()
    raw_tags = data.get("stance_tags") or []
    stance_tags = [str(v).strip() for v in raw_tags if str(v).strip()][:3]
    if not headline:
        title = str(article.get("translated_title") or article.get("title") or "この話題").strip()
        headline = title[:36] or "この話題の見どころ"
    if not commentary:
        summary = re.sub(r"\s+", " ", str(article.get("summary") or "").strip())
        facts = [str(v).strip() for v in (article.get("facts") or []) if str(v).strip()]
        pieces = []
        if summary:
            pieces.append(f"この話の芯は、{summary[:110]}という点にあります。")
        if facts:
            pieces.append(f"読みどころは、{facts[0][:80]}という点が流れを変えうることです。")
        if len(facts) >= 2:
            pieces.append(f"一方で、{facts[1][:80]}という留保もあり、額面どおりには受け取りにくいところがあります。")
        commentary = " ".join(pieces).strip() or "この話題は単なる要点整理より、どこに意味がありどこを留保して読むべきかを見る価値があります。"
    return {"headline": headline[:60], "commentary": commentary[:900], "stance_tags": stance_tags}


def parse_audio_briefing_script_result(
    text: str,
    articles: list[dict],
    persona: str,
    *,
    target_chars: int = 12000,
    include_opening: bool = True,
    include_overall_summary: bool = True,
    include_article_segments: bool = True,
    include_ending: bool = True,
) -> dict:
    data = extract_first_json_object(text) or {}
    opening = _normalize_audio_briefing_generated_text(str(data.get("opening") or "").strip())
    overall_summary = _normalize_audio_briefing_generated_text(str(data.get("overall_summary") or "").strip())
    ending = _normalize_audio_briefing_generated_text(str(data.get("ending") or "").strip())

    if include_opening and not opening:
        raise ValueError("audio briefing script missing opening")
    if include_overall_summary and not overall_summary:
        raise ValueError("audio briefing script missing overall_summary")
    if include_ending and not ending:
        raise ValueError("audio briefing script missing ending")

    raw_segments = data.get("article_segments") if isinstance(data.get("article_segments"), list) else []
    if include_article_segments and len(raw_segments) != len(articles):
        raise ValueError("audio briefing script article_segments count mismatch")

    opening_cap, summary_cap, ending_cap, _article_cap = _audio_briefing_script_budgets(target_chars, len(articles))

    segments: list[dict] = []
    if include_article_segments:
        for index, article in enumerate(articles, start=1):
            item_id = str(article.get("item_id") or "").strip()
            if not item_id:
                raise ValueError(f"audio briefing input article missing item_id at index {index}")
            raw = raw_segments[index-1]
            if not isinstance(raw, dict):
                raise ValueError("audio briefing script segment must be an object")
            returned_item_id = str(raw.get("item_id") or "").strip()
            if returned_item_id != "" and returned_item_id != item_id:
                _log_audio_briefing_segment_id_mismatch(index, item_id, returned_item_id)
            headline = str(raw.get("headline") or "").strip()
            if not headline:
                raise ValueError(f"audio briefing script missing headline for item_id: {item_id}")
            summary_intro = _normalize_audio_briefing_generated_text(str(raw.get("summary_intro") or "").strip())
            if not summary_intro:
                raise ValueError(f"audio briefing script missing summary_intro for item_id: {item_id}")
            commentary = _normalize_audio_briefing_generated_text(str(raw.get("commentary") or "").strip())
            if not commentary:
                raise ValueError(f"audio briefing script missing commentary for item_id: {item_id}")
            segments.append(
                {
                    "item_id": item_id,
                    "headline": headline[:160],
                    "summary_intro": summary_intro,
                    "commentary": commentary,
                }
            )

    return {
        "opening": opening[:opening_cap] if include_opening else "",
        "overall_summary": overall_summary[:summary_cap] if include_overall_summary else "",
        "article_segments": segments,
        "ending": ending[:ending_cap] if include_ending else "",
    }


_AUDIO_BRIEFING_SCRIPT_RETRYABLE_ERROR_MARKERS = (
    "audio briefing script missing opening",
    "audio briefing script missing overall_summary",
    "audio briefing script missing ending",
    "audio briefing script article_segments count mismatch",
    "audio briefing script segment must be an object",
    "audio briefing script missing headline for item_id:",
    "audio briefing script missing summary_intro for item_id:",
    "audio briefing script missing commentary for item_id:",
)


def is_audio_briefing_script_retryable_validation_error(exc: Exception) -> bool:
    message = str(exc or "").strip()
    if not message:
        return False
    return any(message.startswith(marker) for marker in _AUDIO_BRIEFING_SCRIPT_RETRYABLE_ERROR_MARKERS)


def _audio_briefing_script_budgets(target_chars: int, article_count: int) -> tuple[int, int, int, int]:
    target_chars = max(int(target_chars or 0), AUDIO_BRIEFING_CHARS_PER_MINUTE)
    opening_budget = max(min(round(target_chars * 0.15), 2200), 420)
    summary_budget = max(min(round(target_chars * 0.28), 4600), 1500)
    ending_budget = max(min(round(target_chars * 0.13), 1800), 380)
    article_budget = max(
        (target_chars - opening_budget - summary_budget - ending_budget - 100) // max(int(article_count or 0), 1),
        120,
    )
    return opening_budget, summary_budget, ending_budget, article_budget


def _audio_briefing_article_section_budgets(article_budget: int) -> tuple[int, int]:
    article_budget = max(int(article_budget or 0), 1)
    intro_budget = max(round(article_budget * 0.3), 60)
    intro_budget = min(intro_budget, 240)
    if intro_budget >= article_budget:
        intro_budget = max(article_budget // 2, 1)
    commentary_budget = max(article_budget - intro_budget, 1)
    return intro_budget, commentary_budget


def _audio_briefing_article_sentence_specs(article_budget: int) -> tuple[str, str]:
    article_budget = max(int(article_budget or 0), 1)
    if article_budget < 220:
        return "1文固定", "1文固定"
    if article_budget < 420:
        return "1文固定", "1〜2文"
    if article_budget < 700:
        return "1〜2文", "2〜3文"
    return "2文固定", "3〜4文"


def _audio_briefing_section_sentence_spec(section_budget: int, section_kind: str) -> str:
    section_budget = max(int(section_budget or 0), 1)
    if section_kind == "opening":
        if section_budget < 700:
            return "2〜3文"
        if section_budget < 1300:
            return "3〜4文"
        if section_budget < 1900:
            return "4〜5文"
        return "5〜6文"
    if section_kind == "summary":
        if section_budget < 1800:
            return "3〜4文"
        if section_budget < 2800:
            return "4〜5文"
        if section_budget < 3800:
            return "5〜6文"
        return "6〜7文"
    if section_kind == "ending":
        if section_budget < 550:
            return "2〜3文"
        if section_budget < 1000:
            return "3〜4文"
        if section_budget < 1500:
            return "4〜5文"
        return "5〜6文"
    return "3〜4文"


def _audio_briefing_sentence_length_spec(article_budget: int) -> str:
    article_budget = max(int(article_budget or 0), 1)
    if article_budget < 220:
        return "35〜70文字"
    if article_budget < 420:
        return "40〜80文字"
    if article_budget < 700:
        return "50〜95文字"
    return "60〜110文字"


def _normalize_audio_briefing_generated_text(text: str) -> str:
    lines = [re.sub(r"[ \t]+", " ", line).strip() for line in str(text or "").splitlines()]
    lines = [line for line in lines if line]
    return "\n".join(lines).strip()


def _log_audio_briefing_segment_id_mismatch(index: int, expected_item_id: str, returned_item_id: str) -> None:
    try:
        import logging

        logging.getLogger(__name__).warning(
            "audio briefing script segment item_id mismatch at index %s: expected=%s returned=%s",
            index,
            expected_item_id,
            returned_item_id,
        )
    except Exception:
        return


def build_ask_navigator_task(persona: str, ask_input: dict) -> dict:
    persona_key, profile = resolve_navigator_persona_profile(persona, "item")
    sampling_profile = resolve_navigator_sampling_profile(persona_key)
    prompt = f"""あなたはAsk画面の回答直後に出るAIナビゲーターです。

キャラクター:
- persona: {persona_key}
- display_name: {profile["name"]}
- 性別: {profile["gender"]}
- 年代感: {profile["age_vibe"]}
- 一人称: {profile["first_person"]}
- 話し方: {profile["speech_style"]}
- 職業: {profile["occupation"]}
- 経験: {profile["experience"]}
- 性格: {profile["personality"]}
- 価値観: {profile["values"]}
- 関心: {profile["interests"]}
- 嫌いなもの: {profile["dislikes"]}
- tone: {profile["voice"]}

タスク:
- Ask の回答を言い直すのではなく、この問いの前提・留保・次に掘る論点を、このペルソナの主観で論評する
- 日本語で 5〜8文の、やや長めの commentary を返す
- next_angles は 2〜4件返す

ルール:
- 出力は headline / commentary / next_angles を持つ JSON のみ
- 回答の要約や言い換えを主目的にしない
- 「その答えで終わりにしないなら、次にどこを見るべきか」を中心に語る
- 前提のズレ、留保、見落としやすい論点、次に掘ると面白い角度を入れる
- commentary は 5〜8文
- 客観的レビューではなく、このペルソナの主観で語る
- ペルソナの価値観に基づいて評価する
- 「この人ならこう感じる」という自然な語りにする
- 他のキャラクター名を名乗らない。別ペルソナの名前・肩書き・口調を混ぜない
- 自分を名乗るなら、必ず {profile["name"]} とだけ名乗る
- 一人称は {profile["first_person"]} を基本にし、別の一人称へぶれない
- 話し方は {profile["speech_style"]} と {profile["voice"]} に寄せ、他ペルソナの文体へ寄らない
- 1文目か2文目で、この質問の面白さか危うさを掴む
- その後、なぜその前提を少し疑ったほうがよいか、どこに留保があるか、次に何を見ると視界が広がるかを語る
- citations と related_items にある範囲を土台にする
- 入力から読めない事実を捏造しない
- 文量感は {navigator_verbosity_instruction(sampling_profile)}
- snark でも不快・攻撃的・見下し表現は禁止
- snark では、問いや状況に対する軽い皮肉、ツッコミ、呆れ気味の言い回しは許可する
- next_angles は短い日本語フレーズで 2〜4件
- headline は 16〜40字程度で、この論評の切り口が伝わる見出しにする

返却形式:
{{
  "headline": "短い見出し",
  "commentary": "5〜8文の論評",
  "next_angles": ["次に掘る論点1", "次に掘る論点2"]
}}

入力:
{json.dumps(ask_input, ensure_ascii=False)}
"""
    return {
        "prompt": prompt,
        "schema": ASK_NAVIGATOR_SCHEMA,
        "persona": persona_key,
        "input": ask_input,
        "profile": profile,
        "sampling_profile": sampling_profile,
    }


def parse_ask_navigator_result(text: str, ask_input: dict) -> dict:
    data = extract_first_json_object(text) or {}
    headline = str(data.get("headline") or "").strip()
    commentary = str(data.get("commentary") or "").strip()
    raw_angles = data.get("next_angles") or []
    next_angles = [str(v).strip() for v in raw_angles if str(v).strip()][:4]
    if not headline:
        query = str(ask_input.get("query") or "").strip()
        headline = (query[:36] + "の見方") if query else "この問いの見どころ"
    if not commentary:
        answer = re.sub(r"\s+", " ", str(ask_input.get("answer") or "").strip())
        bullets = [str(v).strip() for v in (ask_input.get("bullets") or []) if str(v).strip()]
        pieces = []
        if answer:
            pieces.append(f"この問いは、{answer[:110]}で答えが尽きた気になりやすいところがまず危ないです。")
        if bullets:
            pieces.append(f"実際には、{bullets[0][:80]}のような論点をそのまま受け取らず、どこに前提があるかを見たほうがいいです。")
        if len(bullets) >= 2:
            pieces.append(f"もう一段掘るなら、{bullets[1][:80]}がどの条件で成り立つのかを確かめると見え方が変わります。")
        pieces.append("答えを消費して終わるより、何がまだ抜けているかを拾い直す問いとして扱うほうが収穫があります。")
        commentary = " ".join(pieces).strip()
    if not next_angles:
        next_angles = [
            "前提条件の確認",
            "反例になりうる事例",
            "今後の変化点",
        ]
    return {"headline": headline[:60], "commentary": commentary[:1200], "next_angles": next_angles}


def build_source_navigator_task(persona: str, candidates: list[dict]) -> dict:
    persona_key, profile = resolve_navigator_persona_profile(persona, "briefing")
    sampling_profile = resolve_navigator_sampling_profile(persona_key)
    prompt = f"""あなたはSources管理画面の右下から呼び出されるAIナビゲーターです。

キャラクター:
- persona: {persona_key}
- display_name: {profile["name"]}
- 性別: {profile["gender"]}
- 年代感: {profile["age_vibe"]}
- 一人称: {profile["first_person"]}
- 話し方: {profile["speech_style"]}
- 職業: {profile["occupation"]}
- 経験: {profile["experience"]}
- 性格: {profile["personality"]}
- 価値観: {profile["values"]}
- 関心: {profile["interests"]}
- 嫌いなもの: {profile["dislikes"]}
- tone: {profile["voice"]}

タスク:
- ソース一覧全体を見て、購読バランスを棚卸しする
- 6〜10文の、かなりしっかりした総評 overview を返す
- あわせて keep / watch / standout を最大3件ずつ返す

ルール:
- 出力は JSON のみ
- overview は 6〜10文で、短評ではなくまとまった総評にする
- overview では、全体の傾向、ノイズ源、残す価値がある流れ、見直すべき偏り、直近の温度感を織り込む
- overview は客観的レポートではなく、このペルソナの主観で語る
- ペルソナの価値観に基づいて評価する
- 「この人ならこう見る」という自然な語りにする
- 他のキャラクター名を名乗らない。別ペルソナの名前・肩書き・口調を混ぜない
- 自分を名乗るなら、必ず {profile["name"]} とだけ名乗る
- 一人称は {profile["first_person"]} を基本にし、別の一人称へぶれない
- 話し方は {profile["speech_style"]} と {profile["voice"]} に寄せ、他ペルソナの文体へ寄らない
- 数字をそのまま列挙するだけで終わらせず、「この構成が何を意味するか」を論評する
- source_id は候補にあるものだけを使う
- keep は「残したい/効いている」ソース
- watch は「見直したい/ノイズや停滞が気になる」ソース
- standout は「最近とくに働いている/効いている」ソース
- 各 comment は 1〜2文、70〜160字程度
- comment でも title や数字の言い換えだけにせず、なぜそう見るかを短く添える
- 同じ source_id を複数カテゴリに重複させない
- 不確かなことは断定しない。候補データから読める範囲だけで論評する
- 文量感は {navigator_verbosity_instruction(sampling_profile)}
- snark では軽口やツッコミを入れてよい
- snark でも不快・攻撃的・見下し表現は禁止
- snark でも読者個人をいじらない。人ではなく状況や構成に対して毒づく

返却形式:
{{
  "overview": "6〜10文の総評",
  "keep": [
    {{"source_id":"uuid", "comment":"残したい理由"}}
  ],
  "watch": [
    {{"source_id":"uuid", "comment":"見直したい理由"}}
  ],
  "standout": [
    {{"source_id":"uuid", "comment":"最近効いている理由"}}
  ]
}}

ソース候補:
{json.dumps(candidates, ensure_ascii=False)}
"""
    return {
        "prompt": prompt,
        "schema": SOURCE_NAVIGATOR_SCHEMA,
        "persona": persona_key,
        "candidates": candidates,
        "profile": profile,
        "sampling_profile": sampling_profile,
    }


def parse_source_navigator_result(text: str, candidates: list[dict]) -> dict:
    data = extract_first_json_object(text) or {}
    overview = str(data.get("overview") or "").strip()
    allowed = {str(c.get("source_id") or "").strip(): c for c in candidates if str(c.get("source_id") or "").strip()}

    def _parse_rows(key: str, seen: set[str]) -> list[dict]:
        rows = data.get(key) if isinstance(data.get(key), list) else []
        out: list[dict] = []
        for row in rows:
            if not isinstance(row, dict):
                continue
            source_id = str(row.get("source_id") or "").strip()
            comment = str(row.get("comment") or "").strip()
            if not source_id or source_id not in allowed or source_id in seen or not comment:
                continue
            title = str(allowed[source_id].get("title") or "").strip()
            out.append({"source_id": source_id, "title": title, "comment": comment[:200]})
            seen.add(source_id)
            if len(out) >= 3:
                break
        return out

    seen: set[str] = set()
    keep = _parse_rows("keep", seen)
    watch = _parse_rows("watch", seen)
    standout = _parse_rows("standout", seen)

    if not overview:
        enabled_count = sum(1 for c in candidates if c.get("enabled"))
        unread_total = sum(int(c.get("unread_items_30d") or 0) for c in candidates)
        favorite_total = sum(int(c.get("favorite_count_30d") or 0) for c in candidates)
        overview = (
            f"いまの購読構成は、{len(candidates)}本のうち{enabled_count}本が有効で、全体の温度差がかなり見えやすい状態です。"
            f"未読は合計で{unread_total}件あり、流量の強いソースと置いておけるソースの差がそのまま backlog に表れています。"
            f"一方でお気に入り反応は合計{favorite_total}件あり、明確に効いている領域も残っています。"
            "だから、全部を均等に扱うより、残す価値がある線と見直すべき線を分けて考えるのが自然です。"
        )
    return {"overview": overview[:1800], "keep": keep, "watch": watch, "standout": standout}


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
    prompt = f"""あなたはRSSフィード推薦アシスタントです。
既存の購読ソースURL、タイトル、興味トピックをもとに、「まだ登録していない可能性が高い」RSS/Atom候補を自由に提案してください。

重要:
- 同一ドメインや親URLの周辺だけに限定しない
- 既存ソースと似たテーマ・編集方針・専門性を持つ別ドメインの媒体を優先する
- 技術メディア、企業ブログ、研究機関、ニュースレター、専門ブログなども対象に含めてよい
- 各候補には、人間が読める短いタイトルも付ける
- RSS/AtomのURLを知っている場合は feed URL を返してよい
- feed URL が不明な場合はサイトトップURLでよい（後段でRSS探索する）

要件:
- 既存ソースと同じURLは除外
- なるべく多様なドメインを混ぜる
- 理由は「どの既存ソースやトピックに近いか」が分かる短い日本語にする
- 最大30件
- JSONのみで返す

返却形式（必須）:
{{
  "items": [
    {{"url":"https://...", "title":"サイト名", "reason":"..."}}
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


def build_seed_sites_rescue_prompt(existing_sources: list[dict], preferred_topics: list[str]) -> str:
    return f"""既存ソースと重複しないサイトURL候補を必ず10件以上返してください。各候補には短いタイトルも付けてください。JSONのみ。
{{
  "items": [
    {{"url":"https://...", "title":"サイト名", "reason":"..."}}
  ]
}}
既存ソース:
{json.dumps(existing_sources, ensure_ascii=False)}
興味トピック:
{json.dumps(preferred_topics, ensure_ascii=False)}
"""


def merge_llm_usage(*usages: dict | None) -> dict:
    keys = (
        "input_tokens",
        "output_tokens",
        "cache_creation_input_tokens",
        "cache_read_input_tokens",
        "reasoning_output_tokens",
    )
    out: dict[str, int] = {}
    for key in keys:
        out[key] = sum(int((usage or {}).get(key, 0) or 0) for usage in usages)
    return out


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
        title = str(row.get("title") or "").strip()[:120]
        out.append({"url": url, "title": title, "reason": str(row.get("reason") or "").strip()[:180]})
    return out
