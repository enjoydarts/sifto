from __future__ import annotations

from app.services.alibaba_service import _chat_json as alibaba_chat_json
from app.services.alibaba_service import _llm_meta as alibaba_llm_meta
from app.services.claude_service import _call_with_model_fallback as anthropic_call_with_model_fallback
from app.services.claude_service import _llm_meta as anthropic_llm_meta
from app.services.deepseek_service import _chat_json as deepseek_chat_json
from app.services.deepseek_service import _llm_meta as deepseek_llm_meta
from app.services.fireworks_service import _chat_json as fireworks_chat_json
from app.services.fireworks_service import _llm_meta as fireworks_llm_meta
from app.services.gemini_service import _generate_content as gemini_generate_content
from app.services.gemini_service import _llm_meta as gemini_llm_meta
from app.services.groq_service import _chat_json as groq_chat_json
from app.services.groq_service import _llm_meta as groq_llm_meta
from app.services.llm_catalog import provider_for_model
from app.services.mistral_service import _chat_json as mistral_chat_json
from app.services.mistral_service import _llm_meta as mistral_llm_meta
from app.services.moonshot_service import _chat_json as moonshot_chat_json
from app.services.moonshot_service import _llm_meta as moonshot_llm_meta
from app.services.openai_service import _chat_json as openai_chat_json
from app.services.openai_service import _llm_meta as openai_llm_meta
from app.services.openrouter_service import _chat_json as openrouter_chat_json
from app.services.openrouter_service import _llm_meta as openrouter_llm_meta
from app.services.poe_service import _chat_json as poe_chat_json
from app.services.poe_service import _llm_meta as poe_llm_meta
from app.services.prompt_template_defaults import get_default_prompt_template, render_prompt_template
from app.services.siliconflow_service import _chat_json as siliconflow_chat_json
from app.services.siliconflow_service import _llm_meta as siliconflow_llm_meta
from app.services.task_transport_common import with_execution_failures
from app.services.xai_service import _chat_json as xai_chat_json
from app.services.xai_service import _llm_meta as xai_llm_meta
from app.services.zai_service import _chat_json as zai_chat_json
from app.services.zai_service import _llm_meta as zai_llm_meta

FISH_PREPROCESS_PURPOSE = "fish_preprocess"
DEFAULT_FISH_PREPROCESS_PROMPT_KEY = "fish.summary_preprocess"
_MAX_OUTPUT_TOKENS = 3200


class FishSpeechPreprocessService:
    def preprocess(
        self,
        *,
        text: str,
        model: str,
        api_key: str | None,
        prompt_key: str = DEFAULT_FISH_PREPROCESS_PROMPT_KEY,
    ) -> dict:
        rendered_text = str(text or "").strip()
        if not rendered_text:
            raise RuntimeError("text is required")
        model_name = str(model or "").strip()
        if not model_name:
            raise RuntimeError("model is required")
        provider = provider_for_model(model_name)
        if not provider:
            raise RuntimeError(f"unsupported fish preprocess model provider: {model_name}")

        template = get_default_prompt_template(prompt_key)
        system_instruction = str(template.get("system_instruction") or "")
        prompt = self._build_prompt(str(template.get("prompt_text") or ""), rendered_text)

        handlers = {
            "anthropic": lambda key: self._preprocess_anthropic(model_name, key, system_instruction, prompt),
            "google": lambda key: self._preprocess_gemini(model_name, key, system_instruction, prompt),
            "groq": lambda key: self._preprocess_openai_compat(groq_chat_json, groq_llm_meta, model_name, key, system_instruction, prompt),
            "deepseek": lambda key: self._preprocess_openai_compat(deepseek_chat_json, deepseek_llm_meta, model_name, key, system_instruction, prompt),
            "alibaba": lambda key: self._preprocess_openai_compat(alibaba_chat_json, alibaba_llm_meta, model_name, key, system_instruction, prompt),
            "mistral": lambda key: self._preprocess_openai_compat(mistral_chat_json, mistral_llm_meta, model_name, key, system_instruction, prompt),
            "moonshot": lambda key: self._preprocess_openai_compat(moonshot_chat_json, moonshot_llm_meta, model_name, key, system_instruction, prompt),
            "xai": lambda key: self._preprocess_openai_compat(xai_chat_json, xai_llm_meta, model_name, key, system_instruction, prompt),
            "zai": lambda key: self._preprocess_openai_compat(zai_chat_json, zai_llm_meta, model_name, key, system_instruction, prompt),
            "fireworks": lambda key: self._preprocess_openai_compat(fireworks_chat_json, fireworks_llm_meta, model_name, key, system_instruction, prompt),
            "openai": lambda key: self._preprocess_openai_compat(openai_chat_json, openai_llm_meta, model_name, key, system_instruction, prompt),
            "openrouter": lambda key: self._preprocess_openai_compat(openrouter_chat_json, openrouter_llm_meta, model_name, key, system_instruction, prompt),
            "poe": lambda key: self._preprocess_openai_compat(poe_chat_json, poe_llm_meta, model_name, key, system_instruction, prompt),
            "siliconflow": lambda key: self._preprocess_openai_compat(siliconflow_chat_json, siliconflow_llm_meta, model_name, key, system_instruction, prompt),
        }
        handler = handlers.get(provider)
        if handler is None:
            raise RuntimeError(f"unsupported fish preprocess provider: {provider}")
        return handler((api_key or "").strip())

    def _build_prompt(self, prompt_text: str, rendered_text: str) -> str:
        rendered = render_prompt_template(prompt_text, {"text": rendered_text})
        if rendered == prompt_text:
            return f"{rendered}\n\n{rendered_text}"
        return rendered

    def _preprocess_openai_compat(self, chat_json, llm_meta, model: str, api_key: str, system_instruction: str, prompt: str) -> dict:
        text, usage = chat_json(
            prompt,
            model,
            api_key,
            system_instruction=system_instruction,
            max_output_tokens=_MAX_OUTPUT_TOKENS,
        )
        return {
            "text": str(text or "").strip(),
            "llm": llm_meta(model, FISH_PREPROCESS_PURPOSE, usage),
        }

    def _preprocess_gemini(self, model: str, api_key: str, system_instruction: str, prompt: str) -> dict:
        text, usage = gemini_generate_content(
            prompt,
            model=model,
            api_key=api_key,
            max_output_tokens=_MAX_OUTPUT_TOKENS,
            system_instruction=system_instruction,
            response_mime_type="text/plain",
        )
        return {
            "text": str(text or "").strip(),
            "llm": gemini_llm_meta(model, FISH_PREPROCESS_PURPOSE, usage),
        }

    def _preprocess_anthropic(self, model: str, api_key: str, system_instruction: str, prompt: str) -> dict:
        system_prompt = str(system_instruction or "").strip() or None
        combined_prompt = f"{system_prompt}\n\n{prompt}" if system_prompt else prompt
        message, used_model, execution_failures = anthropic_call_with_model_fallback(
            combined_prompt,
            model,
            None,
            max_tokens=_MAX_OUTPUT_TOKENS,
            api_key=api_key,
            system_prompt=system_prompt,
            user_prompt=prompt,
        )
        if message is None:
            detail = self._format_execution_failures(execution_failures)
            raise RuntimeError(f"anthropic fish preprocess failed{detail}")
        text = str(message.content[0].text or "").strip()
        return {
            "text": text,
            "llm": with_execution_failures(
                anthropic_llm_meta(message, FISH_PREPROCESS_PURPOSE, used_model or model),
                execution_failures,
            ),
        }

    def _format_execution_failures(self, execution_failures) -> str:
        failures = execution_failures or []
        parts: list[str] = []
        for failure in failures:
            if not isinstance(failure, dict):
                continue
            reason = str(failure.get("reason") or "").strip()
            model = str(failure.get("model") or "").strip()
            if reason and model:
                parts.append(f"{model}: {reason}")
            elif reason:
                parts.append(reason)
        if not parts:
            return ""
        return f": {' | '.join(parts)}"
