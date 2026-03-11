import re
from app.services.check_result_common import CHECK_RESULT_SCHEMA, extract_first_json_object, normalize_check_result, parse_check_line, require_check_comment
from app.services.langfuse_client import get_prompt_text


FACTS_CHECK_SCHEMA = CHECK_RESULT_SCHEMA


def facts_check_system_instruction() -> str:
    fallback = """# Role
あなたはニュース記事から抽出された facts の妥当性を検査するレビュアーです。

# Task
facts が元記事本文に忠実かを判定してください。

# Rules
- facts に書かれた内容が article で裏付けられているかだけを見る
- 推測、誇張、因果の飛躍、本文にない断定があれば厳しく評価する
- 軽微な言い換えや自然な圧縮は許容する
- placeholder や雛形のような抽象的箇条書きは厳しく評価する
- 出力は必ず JSON オブジェクト 1 つのみ
- verdict は pass / warn / fail のいずれか
- short_comment は日本語 1 文、80 文字以内
- short_comment は verdict の理由を短く直接述べる
- 長い説明や言い訳は不要
- 応答に迷ったら short_comment を省略せず warn を返す"""
    return get_prompt_text("facts_check.system", fallback)


def facts_check_prompt(title: str | None, content: str, facts: list[str]) -> str:
    facts_text = "\n".join(f"- {f}" for f in facts)
    fallback = f"""# Output
{{
  "verdict": "pass",
  "short_comment": "本文で裏付けられた事実抽出です。"
}}

{{
  "verdict": "warn",
  "short_comment": "一部に本文根拠が弱い記述があります。"
}}

{{
  "verdict": "fail",
  "short_comment": "本文にない断定または重大な欠落があります。"
}}

# 判定基準
- pass: 主要 facts が article に忠実で、明確な unsupported fact がない
- warn: おおむね妥当だが、抽象的な箇所や軽い取りこぼしがある
- fail: article にない断定、矛盾、placeholder、重大な欠落がある

# 注意
- まず verdict を決め、その理由を short_comment に 1 文で書く
- short_comment を空にしない
- JSON 以外は出力しない

# Input
タイトル: {title or "（不明）"}

article:
{content}

facts:
{facts_text}
"""
    return get_prompt_text(
        "facts_check.primary",
        fallback,
        variables={"title": title or "（不明）", "content": content, "facts_text": facts_text},
    )


def facts_check_retry_prompt(title: str | None, content: str, facts: list[str]) -> str:
    facts_text = "\n".join(f"- {f}" for f in facts)
    fallback = f"""1行のみで返してください。
形式は verdict のみです。

条件:
- verdict は pass / warn / fail のいずれか
- 前置き、後置き、コードフェンス禁止
- 例: pass

タイトル: {title or "（不明）"}
article:
{content}

facts:
{facts_text}
"""
    return get_prompt_text(
        "facts_check.retry",
        fallback,
        variables={"title": title or "（不明）", "content": content, "facts_text": facts_text},
    )


def facts_check_comment_for_verdict(verdict: str) -> str:
    mapping = {
        "pass": "本文で裏付けられた事実抽出です。",
        "warn": "一部に本文根拠が弱い記述があります。",
        "fail": "本文にない断定または重大な欠落があります。",
    }
    return mapping.get(str(verdict or "").strip().lower(), "事実抽出の判定結果を確認してください。")


def _is_valid_short_comment(text: str) -> bool:
    s = str(text or "").strip()
    if len(s) < 8:
        return False
    if re.search(r"[\u3040-\u30ff\u3400-\u9fff]", s) is None:
        return False
    if re.search(r"(は|が|を|に|で|と|や|の|も|へ|から|より|について|として)$", s):
        return False
    if s.endswith(("記事の内容は", "本文では", "事実として", "一部で", "ただし", "なお")):
        return False
    return True


def parse_facts_check_line(text: str) -> dict | None:
    return parse_check_line(text, comment_for_verdict=facts_check_comment_for_verdict)


def normalize_facts_check_result(data: dict | None) -> dict:
    return normalize_check_result(data, max_comment_len=240)


def require_facts_check_comment(result: dict, raw_text: str) -> dict:
    return require_check_comment(
        result,
        raw_text,
        error_prefix="facts check",
        validator=_is_valid_short_comment,
    )
