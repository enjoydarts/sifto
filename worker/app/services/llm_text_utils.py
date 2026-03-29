import json
import re
from urllib.parse import urlparse


def clamp01(v, default: float = 0.5) -> float:
    try:
        x = float(v)
    except Exception:
        return default
    if x < 0:
        return 0.0
    if x > 1:
        return 1.0
    return x


def clamp_int(v: int, lo: int, hi: int) -> int:
    return max(lo, min(hi, int(v)))


def target_summary_chars(source_text_chars: int | None, facts: list[str]) -> int:
    if isinstance(source_text_chars, int) and source_text_chars > 0:
        return clamp_int(round(source_text_chars * 0.16), 220, 1200)
    facts_chars = sum(len(str(f)) for f in (facts or []))
    if facts_chars > 0:
        return clamp_int(round(facts_chars * 0.9), 220, 900)
    return 300


def summary_max_tokens(target_chars: int) -> int:
    return clamp_int(round(target_chars * 1.2), 700, 2600)


def audio_briefing_script_max_tokens(target_chars: int) -> int:
    return clamp_int(round(target_chars * 2), 4800, 28000)


def summary_composite_score(breakdown: dict) -> float:
    weights = {
        "importance": 0.38,
        "novelty": 0.22,
        "actionability": 0.18,
        "reliability": 0.17,
        "relevance": 0.05,
    }
    total = 0.0
    for k, w in weights.items():
        total += clamp01(breakdown.get(k, 0.5), 0.5) * w
    return round(total, 4)


def parse_json_string_array(text: str) -> list[str]:
    start = text.find("[")
    end = text.rfind("]") + 1
    if start == -1 or end == 0:
        return []
    try:
        data = json.loads(text[start:end])
    except Exception:
        return []
    return [str(v) for v in data if isinstance(v, str)]


def strip_code_fence(text: str) -> str:
    s = (text or "").strip().lstrip("\ufeff")
    if s.startswith("```"):
        s = re.sub(r"^```[a-zA-Z0-9_-]*\n?", "", s)
        s = re.sub(r"\n?```$", "", s).strip()
    return s


def extract_first_json_object(text: str) -> dict | None:
    s = strip_code_fence(text)
    if not s:
        return None
    decoder = json.JSONDecoder()
    idx = s.find("{")
    while idx >= 0:
        try:
            obj, _ = decoder.raw_decode(s[idx:])
            if isinstance(obj, dict):
                return obj
        except Exception:
            pass
        idx = s.find("{", idx + 1)
    return None


def normalize_url_for_match(raw: str) -> str:
    s = (raw or "").strip()
    if not s:
        return ""
    try:
        u = urlparse(s)
    except Exception:
        return s.lower()
    scheme = (u.scheme or "https").lower()
    host = (u.netloc or "").lower()
    path = (u.path or "").rstrip("/")
    return f"{scheme}://{host}{path}"


def decode_json_string_fragment(raw: str) -> str:
    try:
        return json.loads(f'"{raw}"')
    except Exception:
        return raw.replace("\\n", "\n").replace('\\"', '"').replace("\\\\", "\\")


def extract_json_string_value_loose(text: str, field: str) -> str:
    s = strip_code_fence(text)
    key = f'"{field}"'
    i = s.find(key)
    if i < 0:
        return ""
    rest = s[i + len(key):]
    colon = rest.find(":")
    if colon < 0:
        return ""
    raw = rest[colon + 1 :].lstrip()
    if not raw.startswith('"'):
        return ""
    raw = raw[1:]
    out: list[str] = []
    escaped = False
    for ch in raw:
        if escaped:
            out.append(ch)
            escaped = False
            continue
        if ch == "\\":
            out.append(ch)
            escaped = True
            continue
        if ch == '"':
            break
        out.append(ch)
    return decode_json_string_fragment("".join(out)).strip()


def extract_compose_digest_fields(text: str) -> tuple[str, str]:
    data = extract_first_json_object(text) or {}
    subject = str(data.get("subject") or "").strip()
    body = str(data.get("body") or "").strip()
    if subject and body:
        return subject, body

    s = strip_code_fence(text)
    m_subject = re.search(r'"subject"\s*:\s*"((?:\\.|[^"\\])*)"', s, re.S)
    if not subject and m_subject:
        subject = decode_json_string_fragment(m_subject.group(1)).strip()

    m_body = re.search(r'"body"\s*:\s*"((?:\\.|[^"\\])*)"', s, re.S)
    if not body and m_body:
        body = decode_json_string_fragment(m_body.group(1)).strip()
    elif not body:
        key = '"body"'
        i = s.find(key)
        if i >= 0:
            rest = s[i + len(key):]
            colon = rest.find(":")
            if colon >= 0:
                raw = rest[colon + 1 :].strip()
                if raw.startswith('"'):
                    raw = raw[1:]
                marker_idx = raw.find('",\n  "sections"')
                if marker_idx < 0:
                    marker_idx = raw.find('", "sections"')
                if marker_idx > 0:
                    raw = raw[:marker_idx]
                raw = raw.strip().rstrip('"').strip()
                if raw:
                    body = raw.replace("\\n", "\n").replace('\\"', '"').strip()
    return subject, body


def contains_japanese(text: str) -> bool:
    s = (text or "").strip()
    if not s:
        return False
    return re.search(r"[\u3040-\u30ff\u3400-\u9fff]", s) is not None


def contains_japanese_kana(text: str) -> bool:
    s = (text or "").strip()
    if not s:
        return False
    return re.search(r"[\u3040-\u30ffー]", s) is not None


def facts_need_japanese_localization(facts: list[str]) -> bool:
    cleaned = [str(v).strip() for v in (facts or []) if str(v).strip()]
    if not cleaned:
        return False
    japanese_count = sum(1 for fact in cleaned if contains_japanese_kana(fact))
    return japanese_count * 2 < len(cleaned)


def needs_title_translation(title: str | None, translated_title: str) -> bool:
    src = (title or "").strip()
    if not src:
        return False
    if (translated_title or "").strip():
        return False
    if contains_japanese(src):
        return False
    return True
