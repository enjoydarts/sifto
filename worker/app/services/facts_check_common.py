import re
from app.services.facts_task_common import is_placeholder_fact
from app.services.check_result_common import CHECK_RESULT_SCHEMA, extract_first_json_object, normalize_check_result, parse_check_line, require_check_comment
from app.services.langfuse_client import get_prompt_text


FACTS_CHECK_SCHEMA = CHECK_RESULT_SCHEMA


def facts_check_system_instruction() -> str:
    fallback = """# Role
あなたはニュース記事から抽出された facts の妥当性を検査するレビュアーです。

# Task
facts が元記事本文に忠実かを判定してください。

# Rules
- facts の内容が article で裏付けられているかだけを見る
- 事実数が記事内容に対して明らかに少ない場合は、重要情報の取りこぼしとみなし warn か fail へ寄せる
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
    expected_min_facts = _required_min_facts(len((content or "")))
    coverage_hint = _coverage_hint_text(content, facts)
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
- pass: 主要 facts が article に忠実で、明確な unsupported fact がない。  
  補助基準: `facts` 数が記事サイズの最低目安 (`{expected_min_facts}` 件) を満たし、主要テーマの要点に漏れがないこと。
- warn: 主要 facts は大半妥当だが、補助情報や一部テーマで取りこぼしがある。  
  `facts` 数が最低目安未満、または coverage の説明が弱い場合。
- fail: article にない断定、矛盾、placeholder、または主要テーマで明らかな欠落がある。  
  `facts` 数が極端に少なく、記事の大枠が検証不能な場合。

# 注意
- まず verdict を決め、その理由を short_comment に 1 文で書く
- short_comment を空にしない
- JSON 以外は出力しない

# 補助判定
- こちらは補助情報: {coverage_hint}

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
        variables={
            "title": title or "（不明）",
            "content": content,
            "facts_text": facts_text,
            "expected_min_facts": expected_min_facts,
        },
    )


def facts_check_retry_prompt(title: str | None, content: str, facts: list[str]) -> str:
    facts_text = "\n".join(f"- {f}" for f in facts)
    expected_min_facts = _required_min_facts(len((content or "")))
    coverage_hint = _coverage_hint_text(content, facts)
    fallback = f"""JSON オブジェクト 1 つのみで返してください。
形式:
{{
  "verdict": "pass",
  "short_comment": "本文で裏付けられた事実抽出です。"
}}

条件:
- verdict は pass / warn / fail のいずれか
- short_comment は日本語 1 文、80 文字以内
- short_comment を空にしない
- 前置き、後置き、コードフェンス禁止
- JSON 以外は出力しない
- `facts` が空、または記事サイズに対して `facts` が少なすぎる場合は fail に近い扱いを優先する
- 想定最低数は記事内容目安 `{expected_min_facts}` 件
- 補助判定:
  {coverage_hint}

タイトル: {title or "（不明）"}
article:
{content}

facts:
{facts_text}
"""
    return get_prompt_text(
        "facts_check.retry",
        fallback,
        variables={
            "title": title or "（不明）",
            "content": content,
            "facts_text": facts_text,
            "expected_min_facts": expected_min_facts,
        },
    )


def _coverage_hint_text(content: str, facts: list[str]) -> str:
    issue = detect_facts_check_coverage_issue(content, facts)
    if issue is None:
        return "不足判定は見当たりません。LLM 判定を通常どおり実施してください。"
    return (
        f"coverage_issue: verdict={issue.get('verdict')} / "
        f"{issue.get('short_comment') or '補助情報あり'}。"
        "LLM 判定では厳しめの扱いも許容します。"
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


def _cleaned_facts(facts: list[str]) -> list[str]:
    out: list[str] = []
    seen: set[str] = set()
    for fact in facts:
        text = str(fact or "").strip()
        if not text:
            continue
        key = " ".join(text.lower().split())
        if key in seen:
            continue
        seen.add(key)
        out.append(text)
    return out


def _required_min_facts(content_chars: int) -> int:
    if content_chars < 450:
        return 1
    if content_chars < 1200:
        return 2
    if content_chars < 2200:
        return 3
    if content_chars < 3600:
        return 4
    if content_chars < 5000:
        return 5
    return 6


def detect_facts_check_coverage_issue(content: str, facts: list[str]) -> dict | None:
    clean_facts = _cleaned_facts(facts)
    if not clean_facts:
        return {
            "verdict": "fail",
            "short_comment": "事実抽出が空のため、本文との照合ができません。",
        }

    content_chars = len((content or "").strip())
    if content_chars <= 0:
        return None

    min_facts = _required_min_facts(content_chars)
    if len(clean_facts) < min_facts:
        if content_chars >= 3600 and len(clean_facts) <= 2:
            return {
                "verdict": "fail",
                "short_comment": "長文記事で抽出 fact が極端に少なく、主要情報の検証が困難です。",
            }
        if len(clean_facts) <= max(1, min_facts // 2):
            return {
                "verdict": "fail",
                "short_comment": "主要な事実の抽出が少なすぎるため、記事根拠の確認が難しいです。",
            }
        return {
            "verdict": "warn",
            "short_comment": "記事量に対して fact が少なく、取りこぼしが疑われます。",
        }

    non_placeholder = [f for f in clean_facts if not is_placeholder_fact(f)]
    if not non_placeholder:
        return {
            "verdict": "fail",
            "short_comment": "抽出された facts が形式的で、記事根拠の確認ができません。",
        }
    if len(non_placeholder) <= max(1, len(clean_facts) // 2):
        if len(clean_facts) <= 3:
            return {
                "verdict": "fail",
                "short_comment": "抽出された facts が形式的で、記事根拠の確認が難しいです。",
            }
        return {
            "verdict": "warn",
            "short_comment": "抽出 facts の一部が形式的で、重要情報の取りこぼしが疑われます。",
        }

    short_fact_count = sum(1 for fact in clean_facts if len(fact) < 12)
    if short_fact_count >= max(1, len(clean_facts) - 1) and content_chars >= 600:
        return {
            "verdict": "warn",
            "short_comment": "抽出 facts が短すぎて本文検証が十分でない可能性があります。",
        }

    if content_chars >= 1500:
        if len(clean_facts) <= 2:
            return {
                "verdict": "warn",
                "short_comment": "長い記事に対して facts が少なく、主要情報の取りこぼしが疑われます。",
            }
    if content_chars >= 2500 and len(clean_facts) <= 3:
        return {
            "verdict": "fail",
            "short_comment": "長文記事に対し facts が著しく少なく、検査結果の信頼性が低下します。",
        }
    return None


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
