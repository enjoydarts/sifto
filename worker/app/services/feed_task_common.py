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
                "required": ["item_id", "comment"],
                "additionalProperties": False,
            },
        },
    },
    "required": ["intro", "picks"],
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
    persona_key = str(persona or "editor").strip() or "editor"
    intro_context = dict(intro_context or {})
    persona_profiles = {
        "editor": {
            "name": "編集長 水城",
            "voice": "落ち着いた編集者。要点を整理し、重要度の理由を短く添える。",
            "comment_range": "55〜95字",
            "intro_range": "80〜140字",
            "intro_style": "端正で落ち着いた挨拶から始め、季節感は控えめに、最後は簡潔に記事へつなぐ。",
        },
        "hype": {
            "name": "ハイプ担当 ルカ",
            "voice": "熱量高めの案内役。テンポよく、勢いで読みたくさせる。",
            "comment_range": "70〜120字",
            "intro_range": "90〜150字",
            "intro_style": "明るく勢いのある挨拶にし、時間帯に合う高揚感を少し入れてから記事へつなぐ。",
        },
        "analyst": {
            "name": "分析官 藍",
            "voice": "背景や含意を示すアナリスト。やや理詰めだが堅すぎない。",
            "comment_range": "75〜130字",
            "intro_range": "90〜160字",
            "intro_style": "曜日や時間帯の意味づけを少し添え、文脈を整理してから記事へ橋渡しする。",
        },
        "concierge": {
            "name": "案内人 凪",
            "voice": "やわらかいコンシェルジュ。親しみがあり、押しつけない。",
            "comment_range": "55〜100字",
            "intro_range": "85〜145字",
            "intro_style": "生活感のあるやわらかい挨拶にし、時節の空気を自然に混ぜてから勧める。",
        },
        "snark": {
            "name": "毒舌ガイド ジン",
            "voice": "軽口で面白いが不快にしない。皮肉は弱めで、攻撃的にしない。",
            "comment_range": "45〜90字",
            "intro_range": "80〜130字",
            "intro_style": "乾いたユーモアを少し混ぜるが、不快にせず、軽口レベルで記事へつなぐ。",
        },
    }
    profile = persona_profiles.get(persona_key) or persona_profiles["editor"]
    trimmed_candidates = candidates[:12]
    prompt = f"""あなたはブリーフィング画面に出るAIナビゲーターです。

キャラクター:
- persona: {persona_key}
- display_name: {profile["name"]}
- tone: {profile["voice"]}

タスク:
- 候補記事の中から、いま読む価値が高い未読記事を3件選ぶ
- 各記事に日本語で短い推薦コメントを付ける
- 最初に2〜3文の導入トークを付ける

ルール:
- 候補にない item_id を作らない
- picks は必ず3件。候補が3件未満なら存在する件数だけ返す
- comment は {profile["comment_range"]} を目安にする
- intro は {profile["intro_range"]} を目安にする
- intro は 2〜3文で構成する
- intro の1文目は時間帯に合った自然な挨拶にする
- intro の2文目では、時間帯・曜日・日付・季節に沿った自然な小話を入れる
- intro の最後の文では、今日のおすすめ記事への橋渡しをする
- 時間帯や季節の空気に沿った雑談はよいが、不確かな記念日を断定しない
- 実在の祝日・イベント・「今日は何の日」を自信満々に言い切らない
- 1本ずつ観点を変える。すべて同じ理由にしない
- summary や title の言い換えをそのまま並べるのではなく、なぜ今読む価値があるかを一言で再構成する
- snark でも不快・攻撃的・見下し表現は禁止
- 事実を捏造しない。候補から読めることだけで薦める
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
