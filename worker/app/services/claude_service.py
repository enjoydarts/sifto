import json
import os
import anthropic

_client = anthropic.Anthropic(api_key=os.environ["ANTHROPIC_API_KEY"])
_model = "claude-3-5-haiku-latest"


def extract_facts(title: str | None, content: str) -> list[str]:
    prompt = f"""以下の記事から重要な事実を箇条書きで抽出してください。
事実は客観的かつ具体的に記述し、5〜10個程度にまとめてください。
JSON配列として返してください。例: ["事実1", "事実2"]

タイトル: {title or "（不明）"}

本文:
{content[:4000]}
"""
    message = _client.messages.create(
        model=_model,
        max_tokens=1024,
        messages=[{"role": "user", "content": prompt}],
    )
    text = message.content[0].text.strip()
    # Extract JSON array from response
    start = text.find("[")
    end = text.rfind("]") + 1
    if start == -1 or end == 0:
        return []
    return json.loads(text[start:end])


def summarize(title: str | None, facts: list[str]) -> dict:
    facts_text = "\n".join(f"- {f}" for f in facts)
    prompt = f"""以下の事実リストをもとに、記事の要約を作成してください。
以下のJSON形式で返してください:
{{
  "summary": "200字程度の要約",
  "topics": ["トピック1", "トピック2"],
  "score": 0.0〜1.0の関連度スコア（一般的な読者にとっての重要度）
}}

タイトル: {title or "（不明）"}

事実:
{facts_text}
"""
    message = _client.messages.create(
        model=_model,
        max_tokens=1024,
        messages=[{"role": "user", "content": prompt}],
    )
    text = message.content[0].text.strip()
    start = text.find("{")
    end = text.rfind("}") + 1
    data = json.loads(text[start:end])
    return {
        "summary": data.get("summary", ""),
        "topics": data.get("topics", []),
        "score": float(data.get("score", 0.5)),
    }
