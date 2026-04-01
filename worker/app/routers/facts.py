from fastapi import APIRouter, HTTPException, Request
from pydantic import BaseModel
from app.services.claude_service import extract_facts
from app.services.alibaba_service import extract_facts as extract_facts_alibaba
from app.services.deepseek_service import extract_facts as extract_facts_deepseek
from app.services.fireworks_service import extract_facts as extract_facts_fireworks
from app.services.gemini_service import extract_facts as extract_facts_gemini
from app.services.groq_service import extract_facts as extract_facts_groq
from app.services.llm_dispatch import dispatch_by_model
from app.services.mistral_service import extract_facts as extract_facts_mistral
from app.services.openai_service import extract_facts as extract_facts_openai
from app.services.openrouter_service import extract_facts as extract_facts_openrouter
from app.services.poe_service import extract_facts as extract_facts_poe
from app.services.runtime_prompt_overrides import bind_prompt_override
from app.services.siliconflow_service import extract_facts as extract_facts_siliconflow
from app.services.moonshot_service import extract_facts as extract_facts_moonshot
from app.services.xai_service import extract_facts as extract_facts_xai
from app.services.zai_service import extract_facts as extract_facts_zai
from app.services.router_observe import llm_usage_summary, run_observed_request

router = APIRouter()


class FactsRequest(BaseModel):
    title: str | None
    content: str
    model: str | None = None
    prompt: dict | None = None


class FactsResponse(BaseModel):
    facts: list[str]
    llm: dict | None = None
    facts_localization_llm: dict | None = None


@router.post("/extract-facts", response_model=FactsResponse)
def extract_facts_endpoint(req: FactsRequest, request: Request):
    try:
        with bind_prompt_override((req.prompt or {}).get("prompt_key"), (req.prompt or {}).get("prompt_text"), (req.prompt or {}).get("system_instruction")):
            result = run_observed_request(
                request,
                metadata={"model": req.model or "", "title_present": bool(req.title), "content_chars": len(req.content or "")},
                input_payload={"title": req.title, "content_chars": len(req.content or ""), "model": req.model},
                call=lambda: dispatch_by_model(
                    request,
                    req.model,
                    handlers={
                    "anthropic": lambda api_key: extract_facts(req.title, req.content, api_key=api_key, model=req.model),
                    "google": lambda api_key: extract_facts_gemini(req.title, req.content, model=str(req.model), api_key=api_key or ""),
                    "fireworks": lambda api_key: extract_facts_fireworks(req.title, req.content, model=str(req.model), api_key=api_key or ""),
                    "groq": lambda api_key: extract_facts_groq(req.title, req.content, model=str(req.model), api_key=api_key or ""),
                    "deepseek": lambda api_key: extract_facts_deepseek(req.title, req.content, model=str(req.model), api_key=api_key or ""),
                    "alibaba": lambda api_key: extract_facts_alibaba(req.title, req.content, model=str(req.model), api_key=api_key or ""),
                    "mistral": lambda api_key: extract_facts_mistral(req.title, req.content, model=str(req.model), api_key=api_key or ""),
                    "moonshot": lambda api_key: extract_facts_moonshot(req.title, req.content, model=str(req.model), api_key=api_key or ""),
                    "xai": lambda api_key: extract_facts_xai(req.title, req.content, model=str(req.model), api_key=api_key or ""),
                    "zai": lambda api_key: extract_facts_zai(req.title, req.content, model=str(req.model), api_key=api_key or ""),
                    "openrouter": lambda api_key: extract_facts_openrouter(req.title, req.content, model=str(req.model), api_key=api_key or ""),
                    "poe": lambda api_key: extract_facts_poe(req.title, req.content, model=str(req.model), api_key=api_key or ""),
                    "siliconflow": lambda api_key: extract_facts_siliconflow(req.title, req.content, model=str(req.model), api_key=api_key or ""),
                    "openai": lambda api_key: extract_facts_openai(req.title, req.content, model=str(req.model), api_key=api_key or ""),
                    },
                ),
                output_builder=lambda result: {"facts_count": len(result.get("facts") or []), **llm_usage_summary(result)},
            )
        return FactsResponse(**result)
    except Exception as e:
        raise HTTPException(status_code=500, detail=f"extract_facts failed: {e}")
