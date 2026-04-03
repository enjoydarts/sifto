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
from app.services.prompt_template_defaults import get_default_prompt_template
from app.services.runtime_prompt_overrides import apply_prompt_override


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


def _join_non_empty_lines(lines: list[str]) -> str:
    return "\n".join(str(line) for line in lines if str(line).strip())


def build_audio_briefing_script_schema(
    *,
    conversation_mode: str = "single",
    include_opening: bool,
    include_overall_summary: bool,
    include_article_segments: bool,
    include_ending: bool,
    article_count: int = 0,
    article_turn_count: int = 5,
) -> dict:
    if str(conversation_mode).strip() == "duo":
        normalized_article_count = max(int(article_count or 0), 0)
        min_turns = 0
        article_turn_count = 3 if int(article_turn_count or 0) <= 3 else 5
        allowed_sections = []
        if include_opening:
            allowed_sections.append("opening")
            min_turns += 5
        if include_overall_summary:
            allowed_sections.append("overall_summary")
            min_turns += 5
        if include_article_segments:
            allowed_sections.append("article")
            min_turns += normalized_article_count * article_turn_count
        if include_ending:
            allowed_sections.append("ending")
            min_turns += 5
        base_properties = {
            "speaker": {"type": "string", "enum": ["host", "partner"]},
            "text": {"type": "string"},
        }
        section_item_schemas: list[dict] = []
        frame_sections = [section for section in allowed_sections if section != "article"]
        if frame_sections:
            section_item_schemas.append(
                {
                    "type": "object",
                    "properties": {
                        **base_properties,
                        "section": {"type": "string", "enum": frame_sections},
                    },
                    "required": ["speaker", "section", "text"],
                    "additionalProperties": False,
                }
            )
        if include_article_segments:
            section_item_schemas.append(
                {
                    "type": "object",
                    "properties": {
                        **base_properties,
                        "section": {"type": "string", "enum": ["article"]},
                        "item_id": {"type": "string"},
                    },
                    "required": ["speaker", "section", "item_id", "text"],
                    "additionalProperties": False,
                }
            )
        if min_turns <= 0:
            min_turns = 1
        return {
            "type": "object",
            "properties": {
                "turns": {
                    "type": "array",
                    "minItems": min_turns,
                    "items": section_item_schemas[0] if len(section_item_schemas) == 1 else {"anyOf": section_item_schemas},
                }
            },
            "required": ["turns"],
            "additionalProperties": False,
        }
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
    conversation_mode = _normalize_audio_briefing_conversation_mode(
        str(intro_context.get("audio_briefing_conversation_mode") or "single")
    )
    prompt_key = "audio_briefing_script.duo" if conversation_mode == "duo" else "audio_briefing_script.single"
    default_template = get_default_prompt_template(prompt_key)
    host_persona = str(intro_context.get("audio_briefing_host_persona") or persona)
    partner_persona = str(intro_context.get("audio_briefing_partner_persona") or "analyst")
    generation_mode = str(intro_context.get("audio_briefing_generation_mode") or "").strip()
    generation_section = str(intro_context.get("audio_briefing_generation_section") or "").strip()
    program_position = str(intro_context.get("audio_briefing_program_position") or "").strip()
    article_batch_start_index = int(intro_context.get("audio_briefing_article_batch_start_index") or 0)
    article_batch_end_index = int(intro_context.get("audio_briefing_article_batch_end_index") or 0)
    existing_section_text = _normalize_audio_briefing_generated_text(
        str(intro_context.get("audio_briefing_existing_section_text") or "").strip()
    )
    existing_article_segments = intro_context.get("audio_briefing_existing_article_segments") or []
    persona_key, briefing_profile = resolve_navigator_persona_profile(persona, "briefing")
    _, item_profile = resolve_navigator_persona_profile(persona, "item")
    host_persona_key, host_profile = resolve_navigator_persona_profile(host_persona, "briefing")
    partner_persona_key, partner_profile = resolve_navigator_persona_profile(partner_persona, "briefing")
    trimmed_articles = articles[:30]
    total_articles = int(intro_context.get("audio_briefing_total_articles") or len(trimmed_articles) or 0)
    chars_per_minute = max(int(chars_per_minute or AUDIO_BRIEFING_CHARS_PER_MINUTE), 1)
    target_duration_minutes = max(int(target_duration_minutes or 20), 1)
    target_chars = max(int(target_chars or 0), chars_per_minute)
    opening_budget, summary_budget, ending_budget, article_budget = _audio_briefing_script_budgets(
        target_chars,
        len(trimmed_articles),
        include_opening=include_opening,
        include_overall_summary=include_overall_summary,
        include_ending=include_ending,
    )
    opening_sentence_spec = _audio_briefing_section_sentence_spec(opening_budget, "opening")
    summary_sentence_spec = _audio_briefing_section_sentence_spec(summary_budget, "summary")
    ending_sentence_spec = _audio_briefing_section_sentence_spec(ending_budget, "ending")
    article_headline_budget, article_summary_intro_budget, article_commentary_budget = _audio_briefing_article_section_budgets(article_budget)
    article_headline_sentence_spec, article_summary_intro_sentence_spec, article_commentary_sentence_spec = _audio_briefing_article_sentence_specs(
        article_headline_budget, article_summary_intro_budget, article_commentary_budget
    )
    opening_min_sentences, opening_max_sentences = _audio_briefing_sentence_bounds(opening_sentence_spec)
    summary_min_sentences, summary_max_sentences = _audio_briefing_sentence_bounds(summary_sentence_spec)
    ending_min_sentences, ending_max_sentences = _audio_briefing_sentence_bounds(ending_sentence_spec)
    article_headline_min_sentences, article_headline_max_sentences = _audio_briefing_sentence_bounds(article_headline_sentence_spec)
    article_summary_intro_min_sentences, article_summary_intro_max_sentences = _audio_briefing_sentence_bounds(article_summary_intro_sentence_spec)
    article_commentary_min_sentences, article_commentary_max_sentences = _audio_briefing_sentence_bounds(article_commentary_sentence_spec)
    opening_min_chars, opening_max_chars = _audio_briefing_char_bounds(opening_budget)
    summary_min_chars, summary_max_chars = _audio_briefing_char_bounds(summary_budget)
    ending_min_chars, ending_max_chars = _audio_briefing_char_bounds(ending_budget)
    article_min_chars, article_max_chars = _audio_briefing_char_bounds(article_budget)
    article_headline_min_chars, article_headline_max_chars = _audio_briefing_char_bounds(article_headline_budget)
    article_summary_intro_min_chars, article_summary_intro_max_chars = _audio_briefing_char_bounds(article_summary_intro_budget)
    article_commentary_min_chars, article_commentary_max_chars = _audio_briefing_char_bounds(article_commentary_budget)
    duo_article_turn_count = _audio_briefing_duo_article_turn_count(article_budget)
    duo_article_turn_phrase = "3手" if duo_article_turn_count == 3 else "5手"
    duo_article_turn_sequence = (
        "`host -> partner -> host`"
        if duo_article_turn_count == 3
        else "`host -> partner -> host -> partner -> host`"
    )
    duo_article_turn_flow = (
        "`host(setup) -> partner(reaction/contrast) -> host(close)`"
        if duo_article_turn_count == 3
        else "`host(setup) -> partner(reaction) -> host(deepen) -> partner(contrast) -> host(close)`"
    )
    target_chars_min = max(1, round(target_chars * 0.9))
    target_chars_max = max(target_chars_min, round(target_chars * 1.1))
    sentence_length_spec = _audio_briefing_sentence_length_spec(article_budget)
    common_section_rules: list[str] = []
    single_article_section_rules: list[str] = []
    single_target_lines: list[str] = []
    response_properties: list[str] = []
    if include_opening:
        common_section_rules.append(f"- opening は {opening_min_sentences}文以上 {opening_max_sentences}文以下で、時間帯に合う自然な導入にする。必ず {opening_min_chars}文字以上 {opening_max_chars}文字以下で書く")
        common_section_rules.append("- opening は番組のオープニングトークとして書く。リスナーに向かって語りかけ、挨拶、時間帯や季節感、軽い日常雑談、これから番組が始まる雰囲気づくりを必ず入れる")
        common_section_rules.append("- opening では個別記事の紹介を始めない。記事内容の長い解説、細かな要約、最初の1本への導入を書かない")
        common_section_rules.append("- opening では articles 内の固有名詞、企業名、製品名、出来事、具体的ニュース内容に触れない")
        common_section_rules.append("- opening の軽い雑談では、天気感、気温、季節の空気、曜日感、通勤や休憩などの生活導線のうち少なくとも2つに必ず触れる。ニュースの話題へ滑らせない")
        common_section_rules.append("- opening では、今回の回のテーマ感、空気感、何を見ていく回かを必ず話す。ただの挨拶だけで終わらせない")
        single_target_lines.append(f"- opening の制約: {opening_min_chars}文字以上 {opening_max_chars}文字以下")
        response_properties.append('  "opening": "導入"')
    if include_overall_summary:
        common_section_rules.append(f"- overall_summary は総括であり、{summary_min_sentences}文以上 {summary_max_sentences}文以下で、その回の全体像、流れ、聞きどころ、記事群のつながりだけを絞って話す。必ず {summary_min_chars}文字以上 {summary_max_chars}文字以下で書く")
        common_section_rules.append("- overall_summary で記事の順番紹介をしない")
        common_section_rules.append("- overall_summary では、回全体を俯瞰して共通テーマ、対立軸、温度感、いま追う意味、記事間のつながりを広く語ってよい")
        common_section_rules.append("- overall_summary では、記事群の共通点、流れ、温度差、見方の違い、別の話に見える記事どうしのつながりまで具体的にしてよい")
        common_section_rules.append("- overall_summary では、なぜ今この回として追う価値があるのか、何が引っかかりとして残るのかまで話してよい")
        common_section_rules.append("- overall_summary では、前半から後半へどう流れが変わるか、どこで話題の重心が移るかまで具体的にしてよい")
        common_section_rules.append("- overall_summary では、別々の話に見える記事どうしがどの点でつながっているか、同じ根っこを持っているかをはっきり述べてよい")
        common_section_rules.append("- overall_summary では、表面上の出来事の列挙ではなく、この回で何が続いていて何が切り替わっているのかを具体的に語る")
        single_target_lines.append(f"- overall_summary の制約: {summary_min_chars}文字以上 {summary_max_chars}文字以下")
        response_properties.append('  "overall_summary": "全体サマリー"')
    if include_article_segments:
        single_article_section_rules.append("- article_segments は入力 articles と同じ順番・同じ件数で返す")
        single_article_section_rules.append(f"- article_segments は全体の target_chars={target_chars} と今回扱う記事数から逆算した配分として書く。1記事あたりの headline と summary_intro と commentary の合計は必ず {article_min_chars}文字以上 {article_max_chars}文字以下で収める")
        single_article_section_rules.append(f"- article_segments の各 headline は {article_headline_min_sentences}文以上 {article_headline_max_sentences}文以下で、これから扱う記事をリスナーに詳細に紹介する導入として書く。必ず {article_headline_min_chars}文字以上 {article_headline_max_chars}文字以下で、見出しの読み上げとして一息で入る長さにする")
        single_article_section_rules.append(f"- article_segments の各 summary_intro は {article_summary_intro_min_sentences}文以上 {article_summary_intro_max_sentences}文以下で、記事の中身や何が起きたかを置く役割として書く。必ず {article_summary_intro_min_chars}文字以上 {article_summary_intro_max_chars}文字以下で書く")
        single_article_section_rules.append(f"- article_segments の各 commentary は {article_commentary_min_sentences}文以上 {article_commentary_max_sentences}文以下で書く。summary_intro を受けて、そのペルソナの反応、理由、比較、引っかかりを広げる。必ず {article_commentary_min_chars}文字以上 {article_commentary_max_chars}文字以下で書く")
        single_article_section_rules.append("- article_segments は各記事にほぼ均等に尺を配る")
        single_article_section_rules.append("- headline では、その記事をリスナーに詳細に紹介するつもりで話す。何の記事で、何が起きていて、どこが気になるのかが自然に伝わるようにする")
        single_article_section_rules.append("- summary_intro では、記事の要点、何が起きたか、何が新しいか、どこがポイントか、なぜ今見る記事かをやや詳しく要約してよい。ただし記事全文の言い換えや細部の列挙にはしない")
        single_article_section_rules.append("- article_segments の commentary は、そのペルソナ本人が自然に口にしそうな感想だけを書く。無難な解説調、誰にでも当てはまる一般論、ニュースキャスター風の中立コメントに寄せない")
        single_article_section_rules.append("- commentary では summary_intro の続きを受けて、このペルソナがどこに反応したか、なぜそう感じたか、どう受け止めたかを話す")
        single_article_section_rules.append("- commentary では反応だけで終わらせず、その反応の理由、比較、背景、引っかかり、今後の見方、今追う意味のうち少なくとも3つを必ず入れる。軽い背景説明や自分ならどう見るかまで話してよい")
        single_article_section_rules.append("- commentary で headline や summary_intro の内容を長く言い換えて繰り返さない。要点を置いたら、反応、理由、比較に進む")
        single_target_lines.append(f"- 各 article segment の制約: headline と summary_intro と commentary を合わせて {article_min_chars}文字以上 {article_max_chars}文字以下")
        single_target_lines.append(f"- headline の個別制約: {article_headline_min_chars}文字以上 {article_headline_max_chars}文字以下")
        single_target_lines.append(f"- summary_intro の個別制約: {article_summary_intro_min_chars}文字以上 {article_summary_intro_max_chars}文字以下")
        single_target_lines.append(f"- commentary の個別制約: {article_commentary_min_chars}文字以上 {article_commentary_max_chars}文字以下")
        response_properties.extend([
            '  "article_segments": [',
            '    {"item_id": "uuid", "headline": "これから扱う記事をリスナーに詳細に紹介する導入", "summary_intro": "記事の中身や何が起きたかを置く", "commentary": "そのペルソナがどう受け止めたかを理由や比較も含めて話す"}',
            "  ]",
        ])
    else:
        single_article_section_rules.append("- article_segments は返さない")
    if include_ending:
        common_section_rules.append(f"- ending は番組を終わらせる締めの言葉として {ending_min_sentences}文以上 {ending_max_sentences}文以下で書く。だらだら締めず、必ず {ending_min_chars}文字以上 {ending_max_chars}文字以下で書く")
        common_section_rules.append("- ending で記事内容の再整理や論点のまとめ直しをしない。ただし今日の回で残った感触や温度感を1〜2点だけ軽く振り返るのはよい")
        common_section_rules.append("- ending では、最後に残った印象や引っかかりを必ず言葉にする")
        common_section_rules.append("- ending では、聞いてくれたことへの一言と、次の時間へ戻っていく感じを必ず入れる。短いお礼だけで終わらせない")
        single_target_lines.append(f"- ending の制約: {ending_min_chars}文字以上 {ending_max_chars}文字以下")
        response_properties.append('  "ending": "締め"')

    response_example = "{\n" + ",\n".join(response_properties) + "\n}"
    supplement_rules: list[str] = []
    if generation_mode == "supplement" and generation_section in {"opening", "overall_summary", "ending"} and existing_section_text:
        supplement_rules.append(f"- 今回は {generation_section} の不足分を補う追記モードです。既存の {generation_section} を繰り返さず、自然につながる差分だけを書く")
        supplement_rules.append("- すでに触れた論点や言い回しを言い直さない。まだ置けていない内容だけを足す")
        supplement_rules.append("- 追記であってもメモや箇条書きにせず、そのセクション単体で自然な話し言葉にする")
    if generation_mode == "supplement" and generation_section == "article_segments" and include_article_segments and existing_article_segments:
        supplement_rules.append("- 今回は article_segments の不足分を補う追記モードです。入力記事と同じ item_id・同じ順番で article_segments を返す")
        supplement_rules.append("- 既存の article_segments を短くしない。既存の流れを保ったまま、不足している厚みだけを足す")
        supplement_rules.append("- 長さが不足している場合は commentary を最優先で厚くし、次に summary_intro を厚くする。headline は明らかに短いときだけ補う")
        supplement_rules.append("- commentary では既存の反応をなぞるだけで終わらせず、新しい理由、比較、背景、今後の見方を追加して厚みを出す")

    if conversation_mode == "duo":
        duo_target_lines: list[str] = []
        duo_section_rules: list[str] = []
        section_names = []
        if include_opening:
            section_names.append("opening")
        if include_overall_summary:
            section_names.append("overall_summary")
        if include_article_segments:
            section_names.append("article")
        if include_ending:
            section_names.append("ending")
        allowed_sections = ", ".join(section_names)
        active_section = generation_section or ("article_segments" if include_article_segments else allowed_sections)
        section_scope_rules: list[str] = []
        program_flow_rules = [
            "- 番組全体の流れは opening -> overall_summary -> article -> ending の順番で一度だけ進む",
            "- 今回の call は番組全体の一部分だけを担当する。番組全体を最初からやり直さない",
        ]
        bridge_rules: list[str] = []
        if include_article_segments:
            duo_target_lines.append(f"- article の会話は各記事あたり {article_min_chars}文字以上 {article_max_chars}文字以下を目安に均等配分する")
            duo_target_lines.append(f"- article は 1記事あたり約 {article_budget}文字の予算で、今回は {duo_article_turn_phrase} に収める")
            duo_section_rules.extend(
                [
                    "- article の turns は入力 articles と同じ順番で進め、各記事を会話として扱う",
                    f"- article は各記事を {duo_article_turn_phrase} の中で完結させる。足りないからといって追加 turn を生やさない",
                    "- host の最初の article turn は、その記事が何の話かを自然に導入する",
                    "- partner は直前の host を受けて、理由・比較・違和感・今後のいずれかを足す",
                    "- host の最後の article turn は、その記事の着地点を作りつつ次へ渡せる温度で閉じる",
                ]
            )
            batch_position_rule = f"- 今回の batch は全{total_articles}本中の {article_batch_start_index}本目から{article_batch_end_index}本目までを担当する"
            if article_batch_start_index <= 0 or article_batch_end_index <= 0:
                batch_position_rule = "- 今回の batch は article パート途中の連続した数本だけを担当する"
            program_flow_rules.extend(
                [
                    "- opening と overall_summary はすでに終わっている前提で続きから入る",
                    "- ending はまだ先なので、この batch の中で締めに入らない",
                    batch_position_rule,
                ]
            )
            bridge_rules.extend(
                [
                    "- この batch の最初のやり取りは、すでに番組が進んでいる流れを軽く受けて始める",
                    "- 各記事の最後は完全に閉じすぎず、次の話題へ滑らかにつながる余地を残す",
                    "- batch の最後のやり取りは、ここで番組全体を締めず、次の article へ続けられる温度で終える",
                    "- 前の記事から次の記事へ移るときは、論点の差、温度差、共通点のどれかをひとこと挟んで自然につなぐ",
                ]
            )
            section_scope_rules.extend(
                [
                    "- 今回は番組の article パートだけを書く。opening、overall_summary、ending は書かない",
                    "- 各記事の会話だけを書く。番組の挨拶、仕切り直し、自己紹介、今日の全体テーマの再説明は禁止",
                    "- 「おはようございます」「こんにちは」「こんばんは」「ではここから」「改めて」など、番組を開き直す導入は禁止",
                    "- article パートは、すでに番組の途中に入っている前提で話す。各 batch の先頭でも opening や overall_summary の言い直しをしない",
                    "- 各記事の最初の host turn は、その記事そのものの導入から始める。番組全体の前置きに戻らない",
                ]
            )
        elif include_opening:
            program_flow_rules.extend(
                [
                    "- 今回は番組冒頭の opening を担当する。まだ overall_summary や article には入っていない",
                    "- opening はこのあとに overall_summary と記事パートが続く前提で空気を作る",
                ]
            )
            bridge_rules.extend(
                [
                    "- opening の最後は、overall_summary が自然に始められるように今日の回の空気や見どころへ橋をかける",
                    "- opening 単体で完結しすぎず、『これからこの回で何を見ていくか』へ軽く受け渡して終える",
                ]
            )
            section_scope_rules.extend(
                [
                    "- 今回は opening だけを書く。overall_summary、article、ending は書かない",
                    "- opening では番組の挨拶と導入だけを書く。個別記事の説明に入らない",
                ]
            )
        elif include_overall_summary:
            program_flow_rules.extend(
                [
                    "- opening はすでに終わっている。いまはその直後の overall_summary を担当する",
                    "- これから article パートに入る直前なので、回全体の流れと注目点だけを整える",
                ]
            )
            bridge_rules.extend(
                [
                    "- overall_summary の最初は opening の空気を軽く受けて始める",
                    "- overall_summary の最後は、最初の記事へ自然に入れるように視点や温度感を整えて渡す",
                ]
            )
            section_scope_rules.extend(
                [
                    "- 今回は overall_summary だけを書く。opening、article、ending は書かない",
                    "- overall_summary では挨拶や番組開始の言い直しをしない。すぐに回全体の流れへ入る",
                    "- 個別記事の導入トークや記事ごとの見出し読み上げは禁止",
                ]
            )
        elif include_ending:
            program_flow_rules.extend(
                [
                    "- opening、overall_summary、article はすでに終わっている。いまは最後の ending を担当する",
                    "- ending は番組全体の締めだけを担い、新しい話題や記事紹介に戻らない",
                ]
            )
            bridge_rules.extend(
                [
                    "- ending の最初は、最後の記事の余韻や番組全体に残った感触を軽く受けて始める",
                    "- ending は唐突に切らず、ここまでの流れを受けた自然な着地として閉じる",
                ]
            )
            section_scope_rules.extend(
                [
                    "- 今回は ending だけを書く。opening、overall_summary、article は書かない",
                    "- ending では新しい記事紹介や番組冒頭の挨拶をしない",
                ]
            )
        response_example = """{
  "turns": [
    {"speaker": "host", "section": "opening", "text": "host が導入する"},
    {"speaker": "partner", "section": "opening", "text": "partner が受けて視点を足す"},
    {"speaker": "host", "section": "opening", "text": "host が話を少し進める"},
    {"speaker": "partner", "section": "opening", "text": "partner がさらに空気を足す"},
    {"speaker": "host", "section": "opening", "text": "host がまとめて次へ進める"}
  ]
}"""
        call_scope_note = _join_non_empty_lines(program_flow_rules + bridge_rules + section_scope_rules + duo_section_rules)
        supplement_note = _join_non_empty_lines(supplement_rules) or "通常生成です。既存台本への追記ではありません。"
        existing_context = "なし"
        if generation_mode == "supplement" and generation_section in {"opening", "overall_summary", "ending"} and existing_section_text:
            existing_context = f"既存の {generation_section}:\n{existing_section_text}"
        elif generation_mode == "supplement" and generation_section == "article_segments" and include_article_segments and existing_article_segments:
            existing_context = f"既存の article_segments:\n{json.dumps(existing_article_segments, ensure_ascii=False)}"
        variables = {
            "host_persona_key": host_persona_key,
            "host_display_name": host_profile["name"],
            "host_first_person": host_profile["first_person"],
            "host_speech_style": host_profile["speech_style"],
            "host_personality": host_profile["personality"],
            "host_values": host_profile["values"],
            "host_voice": host_profile["voice"],
            "partner_persona_key": partner_persona_key,
            "partner_display_name": partner_profile["name"],
            "partner_first_person": partner_profile["first_person"],
            "partner_speech_style": partner_profile["speech_style"],
            "partner_personality": partner_profile["personality"],
            "partner_values": partner_profile["values"],
            "partner_voice": partner_profile["voice"],
            "include_opening": "true" if include_opening else "false",
            "include_overall_summary": "true" if include_overall_summary else "false",
            "include_article_segments": "true" if include_article_segments else "false",
            "include_ending": "true" if include_ending else "false",
            "active_section": "article" if active_section == "article_segments" else active_section,
            "allowed_sections": allowed_sections,
            "target_duration_minutes": target_duration_minutes,
            "chars_per_minute": chars_per_minute,
            "target_chars": target_chars,
            "target_chars_min": target_chars_min,
            "target_chars_max": target_chars_max,
            "article_char_range": f"{article_min_chars}〜{article_max_chars}文字",
            "duo_article_turn_phrase": duo_article_turn_phrase,
            "duo_article_turn_sequence": duo_article_turn_sequence.replace("`", ""),
            "duo_article_turn_flow": duo_article_turn_flow.replace("`", ""),
            "generation_mode": generation_mode or "full",
            "generation_section": generation_section or "all",
            "program_position": program_position or "unspecified",
            "program_name": intro_context.get("program_name", ""),
            "total_articles": total_articles,
            "article_batch_range": f'{article_batch_start_index or "-"} - {article_batch_end_index or "-"}',
            "call_scope_note": call_scope_note or "今回返すべきセクションだけを書き、他の section へ脱線しないでください。",
            "supplement_note": supplement_note,
            "now_jst": intro_context.get("now_jst", ""),
            "date_jst": intro_context.get("date_jst", ""),
            "weekday_jst": intro_context.get("weekday_jst", ""),
            "time_of_day": intro_context.get("time_of_day", ""),
            "season_hint": intro_context.get("season_hint", ""),
            "response_example": response_example,
            "articles_json": json.dumps(trimmed_articles, ensure_ascii=False),
            "existing_context": existing_context,
        }
        system_instruction, user_prompt = apply_prompt_override(
            prompt_key,
            str(default_template.get("system_instruction") or ""),
            str(default_template.get("prompt_text") or ""),
            variables,
        )
        effective_prompt = f"{system_instruction}\n\n{user_prompt}"
        return {
            "target_chars": target_chars,
            "system_instruction": system_instruction,
            "user_prompt": user_prompt,
            "prompt": effective_prompt,
            "schema": build_audio_briefing_script_schema(
                conversation_mode=conversation_mode,
                include_opening=include_opening,
                include_overall_summary=include_overall_summary,
                include_article_segments=include_article_segments,
                include_ending=include_ending,
                article_count=len(trimmed_articles),
                article_turn_count=duo_article_turn_count,
            ),
            "persona": persona_key,
            "articles": trimmed_articles,
            "intro_context": intro_context,
            "briefing_profile": briefing_profile,
            "item_profile": item_profile,
        }

    existing_context = "なし"
    if generation_mode == "supplement" and generation_section in {"opening", "overall_summary", "ending"} and existing_section_text:
        existing_context = f"既存の {generation_section}:\n{existing_section_text}"
    if generation_mode == "supplement" and generation_section == "article_segments" and include_article_segments and existing_article_segments:
        existing_context = f"既存の article_segments:\n{json.dumps(existing_article_segments, ensure_ascii=False)}"
    supplement_note = _join_non_empty_lines(supplement_rules) or "通常生成です。既存台本への追記ではありません。"
    variables = {
        "persona_key": persona_key,
        "display_name": briefing_profile["name"],
        "gender": briefing_profile["gender"],
        "age_vibe": briefing_profile["age_vibe"],
        "first_person": briefing_profile["first_person"],
        "speech_style": briefing_profile["speech_style"],
        "occupation": briefing_profile["occupation"],
        "experience": briefing_profile["experience"],
        "personality": briefing_profile["personality"],
        "values": briefing_profile["values"],
        "interests": briefing_profile["interests"],
        "dislikes": briefing_profile["dislikes"],
        "voice": briefing_profile["voice"],
        "intro_style": briefing_profile["intro_style"],
        "comment_range": briefing_profile["comment_range"],
        "item_style": item_profile["style"],
        "include_opening": "true" if include_opening else "false",
        "include_overall_summary": "true" if include_overall_summary else "false",
        "include_article_segments": "true" if include_article_segments else "false",
        "include_ending": "true" if include_ending else "false",
        "target_duration_minutes": target_duration_minutes,
        "chars_per_minute": chars_per_minute,
        "target_chars": target_chars,
        "target_chars_min": target_chars_min,
        "target_chars_max": target_chars_max,
        "sentence_length_spec": sentence_length_spec,
        "opening_sentence_range": f"{opening_min_sentences}〜{opening_max_sentences}文",
        "opening_char_range": f"{opening_min_chars}〜{opening_max_chars}文字",
        "summary_sentence_range": f"{summary_min_sentences}〜{summary_max_sentences}文",
        "summary_char_range": f"{summary_min_chars}〜{summary_max_chars}文字",
        "ending_sentence_range": f"{ending_min_sentences}〜{ending_max_sentences}文",
        "ending_char_range": f"{ending_min_chars}〜{ending_max_chars}文字",
        "article_char_range": f"{article_min_chars}〜{article_max_chars}文字",
        "headline_sentence_range": f"{article_headline_min_sentences}〜{article_headline_max_sentences}文",
        "headline_char_range": f"{article_headline_min_chars}〜{article_headline_max_chars}文字",
        "summary_intro_sentence_range": f"{article_summary_intro_min_sentences}〜{article_summary_intro_max_sentences}文",
        "summary_intro_char_range": f"{article_summary_intro_min_chars}〜{article_summary_intro_max_chars}文字",
        "commentary_sentence_range": f"{article_commentary_min_sentences}〜{article_commentary_max_sentences}文",
        "commentary_char_range": f"{article_commentary_min_chars}〜{article_commentary_max_chars}文字",
        "generation_mode": generation_mode or "full",
        "generation_section": generation_section or "all",
        "program_position": program_position or "full_episode",
        "program_name": intro_context.get("program_name", ""),
        "total_articles": total_articles,
        "article_batch_range": f'{article_batch_start_index or "-"} - {article_batch_end_index or "-"}',
        "supplement_note": supplement_note,
        "now_jst": intro_context.get("now_jst", ""),
        "date_jst": intro_context.get("date_jst", ""),
        "weekday_jst": intro_context.get("weekday_jst", ""),
        "time_of_day": intro_context.get("time_of_day", ""),
        "season_hint": intro_context.get("season_hint", ""),
        "response_example": f'{response_example}\n- headline でこれから扱う記事をリスナーに詳細に紹介し、commentary でそのペルソナの反応を書く',
        "articles_json": json.dumps(trimmed_articles, ensure_ascii=False),
        "existing_context": existing_context,
    }
    system_instruction, user_prompt = apply_prompt_override(
        prompt_key,
        str(default_template.get("system_instruction") or ""),
        str(default_template.get("prompt_text") or ""),
        variables,
    )
    effective_prompt = f"{system_instruction}\n\n{user_prompt}"
    return {
        "target_chars": target_chars,
        "system_instruction": system_instruction,
        "user_prompt": user_prompt,
        "prompt": effective_prompt,
        "schema": build_audio_briefing_script_schema(
            conversation_mode=conversation_mode,
            include_opening=include_opening,
            include_overall_summary=include_overall_summary,
            include_article_segments=include_article_segments,
            include_ending=include_ending,
            article_count=len(trimmed_articles),
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
    conversation_mode: str = "single",
    target_chars: int = 12000,
    include_opening: bool = True,
    include_overall_summary: bool = True,
    include_article_segments: bool = True,
    include_ending: bool = True,
) -> dict:
    normalized_conversation_mode = str(conversation_mode or "single").strip().lower()
    if normalized_conversation_mode == "duo":
        _raise_if_audio_briefing_turn_text_embeds_turns_payload(text)
    data = extract_first_json_object(text) or {}
    turns_value = data.get("turns") if isinstance(data, dict) else None
    raw_turns = turns_value if isinstance(turns_value, list) else []
    if normalized_conversation_mode == "duo" and raw_turns:
        turns: list[dict] = []
        for index, raw in enumerate(raw_turns, start=1):
            if not isinstance(raw, dict):
                raise ValueError("audio briefing script turn must be an object")
            speaker = _normalize_audio_briefing_turn_speaker(str(raw.get("speaker") or "").strip())
            if speaker == "":
                raise ValueError(f"audio briefing script missing speaker for turn index: {index}")
            section = _normalize_audio_briefing_turn_section(str(raw.get("section") or "").strip())
            if section == "":
                raise ValueError(f"audio briefing script missing section for turn index: {index}")
            item_id = str(raw.get("item_id") or "").strip() or None
            text_value = _normalize_audio_briefing_generated_text(str(raw.get("text") or "").strip())
            if not text_value:
                raise ValueError(f"audio briefing script missing text for turn index: {index}")
            embedded = extract_first_json_object(text_value)
            if isinstance(embedded, dict) and isinstance(embedded.get("turns"), list):
                raise ValueError(f"audio briefing script embedded turns payload for turn index: {index}")
            if section == "article":
                if not item_id:
                    raise ValueError(f"audio briefing script missing item_id for article turn index: {index}")
            else:
                item_id = None
            turns.append(
                {
                    "speaker": speaker,
                    "section": section,
                    "item_id": item_id,
                    "text": text_value,
                }
            )
        return {
            "opening": "",
            "overall_summary": "",
            "article_segments": [],
            "turns": turns,
            "ending": "",
        }
    if normalized_conversation_mode == "duo":
        top_level_keys = []
        if isinstance(data, dict):
            top_level_keys = sorted(str(key) for key in data.keys())
        turns_state = "missing"
        if isinstance(data, dict) and "turns" in data:
            if isinstance(turns_value, list):
                turns_state = f"list(len={len(turns_value)})"
            else:
                turns_state = f"type={type(turns_value).__name__}"
        snippet = strip_code_fence(text)
        snippet = re.sub(r"\s+", " ", snippet).strip()[:240]
        raise ValueError(
            f"audio briefing script missing turns state={turns_state} keys={top_level_keys} snippet={snippet}"
        )
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
                    "summary_intro": summary_intro[:700],
                    "commentary": commentary,
                }
            )

    return {
        "opening": opening if include_opening else "",
        "overall_summary": overall_summary if include_overall_summary else "",
        "article_segments": segments,
        "turns": [],
        "ending": ending if include_ending else "",
    }


def _raise_if_audio_briefing_turn_text_embeds_turns_payload(text: str) -> None:
    snippet = strip_code_fence(text)
    if not snippet:
        return
    text_field_pattern = re.compile(r'"text"\s*:\s*"', re.S)
    embedded_turns_head_pattern = re.compile(r'\{\s*(?:\\)?"turns(?:\\)?"\s*:', re.S)
    for index, match in enumerate(text_field_pattern.finditer(snippet), start=1):
        tail = snippet[match.end() : match.end() + 240]
        if embedded_turns_head_pattern.match(tail):
            raise ValueError(f"audio briefing script embedded turns payload for turn index: {index}")


_AUDIO_BRIEFING_SCRIPT_RETRYABLE_ERROR_MARKERS = (
    "audio briefing script missing opening",
    "audio briefing script missing overall_summary",
    "audio briefing script missing ending",
    "audio briefing script article_segments count mismatch",
    "audio briefing script segment must be an object",
    "audio briefing script turn must be an object",
    "audio briefing script missing turns",
    "audio briefing script missing speaker for turn index:",
    "audio briefing script missing section for turn index:",
    "audio briefing script missing text for turn index:",
    "audio briefing script embedded turns payload for turn index:",
    "audio briefing script missing item_id for article turn index:",
    "audio briefing script missing headline for item_id:",
    "audio briefing script missing summary_intro for item_id:",
    "audio briefing script missing commentary for item_id:",
)


def _normalize_audio_briefing_conversation_mode(value: str) -> str:
    if str(value).strip() == "duo":
        return "duo"
    return "single"


def _normalize_audio_briefing_turn_speaker(value: str) -> str:
    normalized = str(value or "").strip().lower()
    if normalized in {"host", "partner"}:
        return normalized
    return ""


def _normalize_audio_briefing_turn_section(value: str) -> str:
    normalized = str(value or "").strip().lower()
    if normalized in {"opening", "overall_summary", "article", "ending"}:
        return normalized
    return ""


def is_audio_briefing_script_retryable_validation_error(exc: Exception) -> bool:
    message = str(exc or "").strip()
    if not message:
        return False
    return any(message.startswith(marker) for marker in _AUDIO_BRIEFING_SCRIPT_RETRYABLE_ERROR_MARKERS)


def _audio_briefing_script_budgets(
    target_chars: int,
    article_count: int,
    *,
    include_opening: bool = True,
    include_overall_summary: bool = True,
    include_ending: bool = True,
) -> tuple[int, int, int, int]:
    target_chars = max(int(target_chars or 0), AUDIO_BRIEFING_CHARS_PER_MINUTE)
    opening_budget = max(min(round(target_chars * 0.05), 1000), 180) if include_opening else 0
    summary_budget = max(min(round(target_chars * 0.07), 1600), 300) if include_overall_summary else 0
    ending_budget = max(min(round(target_chars * 0.05), 1000), 180) if include_ending else 0
    article_budget = max(
        (target_chars - opening_budget - summary_budget - ending_budget - 100) // max(int(article_count or 0), 1),
        120,
    )
    return opening_budget, summary_budget, ending_budget, article_budget


def _audio_briefing_article_section_budgets(article_budget: int) -> tuple[int, int, int]:
    article_budget = max(int(article_budget or 0), 1)
    headline_budget = max(round(article_budget * 0.12), 40)
    headline_budget = min(headline_budget, 160)
    summary_intro_budget = max(round(article_budget * 0.43), 130)
    summary_intro_budget = min(summary_intro_budget, 460)
    used = headline_budget + summary_intro_budget
    if used >= article_budget:
        headline_budget = max(round(article_budget * 0.12), 1)
        summary_intro_budget = max(round(article_budget * 0.43), 1)
        used = headline_budget + summary_intro_budget
    commentary_budget = max(article_budget - used, 1)
    return headline_budget, summary_intro_budget, commentary_budget


def _audio_briefing_sentence_spec_from_budget(
    budget: int,
    *,
    chars_per_sentence: int,
    min_sentences: int,
    max_sentences: int,
    spread: int = 1,
) -> str:
    budget = max(int(budget or 0), 1)
    chars_per_sentence = max(int(chars_per_sentence or 0), 1)
    count = round(budget / chars_per_sentence)
    count = max(min(count, max_sentences), min_sentences)
    low = max(min_sentences, count - spread)
    high = min(max_sentences, count + spread)
    if low >= high:
        return f"{count}文固定"
    return f"{low}〜{high}文"


def _audio_briefing_min_sentences(sentence_spec: str) -> int:
    match = re.match(r"^\s*(\d+)", str(sentence_spec or ""))
    if not match:
        return 1
    return max(int(match.group(1)), 1)


def _audio_briefing_sentence_bounds(sentence_spec: str) -> tuple[int, int]:
    text = str(sentence_spec or "").strip()
    fixed_match = re.match(r"^\s*(\d+)\s*文固定\s*$", text)
    if fixed_match:
        count = max(int(fixed_match.group(1)), 1)
        return count, count
    range_match = re.match(r"^\s*(\d+)\s*〜\s*(\d+)\s*文\s*$", text)
    if range_match:
        low = max(int(range_match.group(1)), 1)
        high = max(int(range_match.group(2)), low)
        return low, high
    count = _audio_briefing_min_sentences(text)
    return count, count


def _audio_briefing_char_bounds(
    budget: int,
    *,
    min_ratio: float = 0.9,
    max_ratio: float = 1.15,
    min_slack: int = 20,
) -> tuple[int, int]:
    budget = max(int(budget or 0), 1)
    lower = max(1, round(budget * min_ratio))
    upper = max(lower, round(budget * max_ratio))
    if upper-lower < min_slack:
        upper = lower + min_slack
    return lower, upper


def _audio_briefing_article_sentence_specs(headline_budget: int, summary_intro_budget: int, commentary_budget: int) -> tuple[str, str, str]:
    return (
        _audio_briefing_sentence_spec_from_budget(
            headline_budget,
            chars_per_sentence=42,
            min_sentences=2,
            max_sentences=6,
        ),
        _audio_briefing_sentence_spec_from_budget(
            summary_intro_budget,
            chars_per_sentence=45,
            min_sentences=4,
            max_sentences=10,
        ),
        _audio_briefing_sentence_spec_from_budget(
            commentary_budget,
            chars_per_sentence=40,
            min_sentences=5,
            max_sentences=14,
        ),
    )


def _audio_briefing_section_sentence_spec(section_budget: int, section_kind: str) -> str:
    section_budget = max(int(section_budget or 0), 1)
    if section_kind == "opening":
        return _audio_briefing_sentence_spec_from_budget(
            section_budget,
            chars_per_sentence=65,
            min_sentences=2,
            max_sentences=12,
        )
    if section_kind == "summary":
        return _audio_briefing_sentence_spec_from_budget(
            section_budget,
            chars_per_sentence=70,
            min_sentences=2,
            max_sentences=14,
        )
    if section_kind == "ending":
        return _audio_briefing_sentence_spec_from_budget(
            section_budget,
            chars_per_sentence=65,
            min_sentences=2,
            max_sentences=12,
        )
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


def _audio_briefing_duo_article_turn_count(article_budget: int) -> int:
    article_budget = max(int(article_budget or 0), 1)
    if article_budget < 420:
        return 3
    return 5


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
