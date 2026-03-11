import json

from app.services.llm_text_utils import extract_compose_digest_fields, extract_first_json_object, extract_json_string_value_loose
from app.services.langfuse_client import get_prompt_text


DIGEST_SYSTEM_INSTRUCTION = """# Role
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


DIGEST_SCHEMA = {
    "type": "object",
    "properties": {
        "subject": {"type": "string"},
        "body": {"type": "string"},
        "sections": {
            "type": "object",
            "properties": {
                "overall_summary": {"type": "string"},
                "highlights": {"type": "array", "items": {"type": "string"}},
                "other_points": {"type": "array", "items": {"type": "string"}},
                "follow_up": {"type": "string"},
                "closing": {"type": "string"},
            },
            "required": ["overall_summary", "highlights", "other_points", "follow_up", "closing"],
            "additionalProperties": False,
        },
    },
    "required": ["subject", "body", "sections"],
    "additionalProperties": False,
}


CLUSTER_DRAFT_SYSTEM_INSTRUCTION = """# Role
あなたはニュースダイジェストの下書き編集者です。

# Task
同じ話題に属する複数記事の要点メモから、重複をまとめたクラスタ下書きを作成してください。

# Rules
- 与えられた内容のみ使う
- 重複をまとめる
- 重要な相違点があれば残す
- 出力は必ず自然な日本語にする
- 原文が英語でも日本語で要約する
- プレーンテキストの箇条書き 3〜8 行程度
- draft_summary 以外のキーを出さない
- JSONのみで返す"""


CLUSTER_DRAFT_SCHEMA = {
    "type": "object",
    "properties": {"draft_summary": {"type": "string"}},
    "required": ["draft_summary"],
    "additionalProperties": False,
}


def build_simple_digest_input(items: list[dict]) -> str:
    lines = []
    for idx, item in enumerate(items, start=1):
        lines.append(
            f"- item={idx} rank={item.get('rank')} | title={item.get('title') or '（タイトルなし）'} | "
            f"topics={', '.join(item.get('topics') or [])} | score={item.get('score')} | "
            f"summary={str(item.get('summary') or '')[:260]}"
        )
    return "\n".join(lines)


def build_digest_task(digest_date: str, items_count: int, digest_input: str, *, input_mode: str = "items") -> dict:
    prompt_fallback = f"""# Output
{{
  "subject": "件名（40字程度）",
  "body": "メール本文（プレーンテキスト。改行を含めてよい）",
  "sections": {{
    "overall_summary": "1〜3段落",
    "highlights": ["注目ポイント1（1〜2段落）", "注目ポイント2（1〜2段落）"],
    "other_points": ["補足1", "補足2"],
    "follow_up": "明日以降のフォローポイント（1段落）",
    "closing": "締めの1文"
  }}
}}

# Input
digest_date: {digest_date}
items_count: {items_count}
input_mode: {input_mode}
items:
{digest_input}
"""
    prompt = get_prompt_text(
        "digest.primary",
        prompt_fallback,
        variables={
            "digest_date": digest_date,
            "items_count": items_count,
            "input_mode": input_mode,
            "digest_input": digest_input,
        },
    )
    return {
        "system_instruction": DIGEST_SYSTEM_INSTRUCTION,
        "prompt": prompt,
        "schema": DIGEST_SCHEMA,
    }


def parse_digest_result(text: str, *, error_prefix: str) -> tuple[str, str]:
    subject, body = extract_compose_digest_fields(text)
    if not subject:
        subject = extract_json_string_value_loose(text, "subject")
    if not body:
        body = extract_json_string_value_loose(text, "body")
    subject = str(subject or "").strip()
    body = str(body or "").strip()
    if not subject or not body:
        raise RuntimeError(f"{error_prefix}: response_snippet={text[:500]}")
    return subject, body


def build_cluster_draft_task(cluster_label: str, item_count: int, topics: list[str], source_lines: list[str]) -> dict:
    topics = [str(t).strip() for t in topics if str(t).strip()][:8]
    source_lines = [str(x).strip()[:500] for x in source_lines if str(x).strip()][:16]
    prompt_fallback = f"""# Output
{{
  "draft_summary": "- ...\\n- ..."
}}

# Input
cluster_label: {cluster_label}
item_count: {item_count}
topics: {json.dumps(topics, ensure_ascii=False)}
source_lines:
{json.dumps(source_lines, ensure_ascii=False)}
"""
    fallback_prompt_fallback = f"""次の要点メモだけを使って、重複をまとめたクラスタ下書きを作成してください。

要件:
- 推測しない
- 出力は必ず自然な日本語にする
- 原文が英語でも日本語で要約する
- 箇条書き 3〜8 行
- JSONのみで返す
- キーは draft_summary のみ

返却形式:
{{"draft_summary":"- ...\\n- ..."}}

cluster_label: {cluster_label}
item_count: {item_count}
topics: {json.dumps(topics, ensure_ascii=False)}
source_lines:
{json.dumps(source_lines[:10], ensure_ascii=False)}
"""
    prompt = get_prompt_text(
        "digest_cluster_draft.primary",
        prompt_fallback,
        variables={
            "cluster_label": cluster_label,
            "item_count": item_count,
            "topics": json.dumps(topics, ensure_ascii=False),
            "source_lines": json.dumps(source_lines, ensure_ascii=False),
        },
    )
    fallback_prompt = get_prompt_text(
        "digest_cluster_draft.fallback",
        fallback_prompt_fallback,
        variables={
            "cluster_label": cluster_label,
            "item_count": item_count,
            "topics": json.dumps(topics, ensure_ascii=False),
            "source_lines": json.dumps(source_lines[:10], ensure_ascii=False),
        },
    )
    return {
        "system_instruction": CLUSTER_DRAFT_SYSTEM_INSTRUCTION,
        "prompt": prompt,
        "schema": CLUSTER_DRAFT_SCHEMA,
        "fallback_prompt": fallback_prompt,
        "source_lines": source_lines,
    }


def parse_cluster_draft_result(text: str, source_lines: list[str]) -> str:
    data = extract_first_json_object(text) or {}
    draft = str(data.get("draft_summary") or "").strip()
    if not draft:
        draft = extract_json_string_value_loose(text, "draft_summary")
    draft = str(draft or "").strip()
    if not draft:
        draft = "\n".join(source_lines[:5])
    return draft
