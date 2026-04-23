from __future__ import annotations

from app.services.alibaba_service import _p as alibaba_provider
from app.services.claude_service import _call_with_model_fallback as anthropic_call_with_model_fallback
from app.services.claude_service import _llm_meta as anthropic_llm_meta
from app.services.deepseek_service import _p as deepseek_provider
from app.services.deepinfra_service import _p as deepinfra_provider
from app.services.fireworks_service import _p as fireworks_provider
from app.services.gemini_tts import resolve_audio_briefing_persona_prompts
from app.services.gemini_service import _generate_content as gemini_generate_content
from app.services.gemini_service import _llm_meta as gemini_llm_meta
from app.services.groq_service import _p as groq_provider
from app.services.llm_catalog import provider_for_model
from app.services.mistral_service import _p as mistral_provider
from app.services.minimax_service import _call_with_model_fallback as minimax_call_with_model_fallback
from app.services.minimax_service import _llm_meta as minimax_llm_meta
from app.services.moonshot_service import _p as moonshot_provider
from app.services.openai_service import _p as openai_provider
from app.services.openrouter_service import _p as openrouter_provider
from app.services.poe_service import _p as poe_provider
from app.services.prompt_template_defaults import get_default_prompt_template, render_prompt_template
from app.services.siliconflow_service import _p as siliconflow_provider
from app.services.anthropic_transport import message_text as anthropic_message_text
from app.services.task_transport_common import with_execution_failures
from app.services.xai_service import _p as xai_provider
from app.services.zai_service import _p as zai_provider

FISH_PREPROCESS_PURPOSE = "fish_preprocess"
GEMINI_TTS_PREPROCESS_PURPOSE = "gemini_tts_preprocess"
ELEVENLABS_TTS_PREPROCESS_PURPOSE = "elevenlabs_tts_preprocess"
XAI_TTS_PREPROCESS_PURPOSE = "xai_tts_preprocess"
AZURE_SPEECH_TTS_PREPROCESS_PURPOSE = "azure_speech_tts_preprocess"
DEFAULT_TTS_MARKUP_PREPROCESS_PROMPT_KEY = "fish.summary_preprocess"
_MAX_OUTPUT_TOKENS = 3200
_PURPOSE_BY_PROMPT_KEY = {
    "fish.summary_preprocess": FISH_PREPROCESS_PURPOSE,
    "fish.audio_briefing_single_preprocess": FISH_PREPROCESS_PURPOSE,
    "fish.audio_briefing_duo_preprocess": FISH_PREPROCESS_PURPOSE,
    "gemini.summary_preprocess": GEMINI_TTS_PREPROCESS_PURPOSE,
    "gemini.audio_briefing_single_preprocess": GEMINI_TTS_PREPROCESS_PURPOSE,
    "gemini.audio_briefing_duo_preprocess": GEMINI_TTS_PREPROCESS_PURPOSE,
    "elevenlabs.summary_preprocess": ELEVENLABS_TTS_PREPROCESS_PURPOSE,
    "elevenlabs.audio_briefing_single_preprocess": ELEVENLABS_TTS_PREPROCESS_PURPOSE,
    "elevenlabs.audio_briefing_duo_preprocess": ELEVENLABS_TTS_PREPROCESS_PURPOSE,
    "xai.summary_preprocess": XAI_TTS_PREPROCESS_PURPOSE,
    "xai.audio_briefing_single_preprocess": XAI_TTS_PREPROCESS_PURPOSE,
    "xai.audio_briefing_duo_preprocess": XAI_TTS_PREPROCESS_PURPOSE,
    "azure_speech.summary_preprocess": AZURE_SPEECH_TTS_PREPROCESS_PURPOSE,
    "azure_speech.audio_briefing_single_preprocess": AZURE_SPEECH_TTS_PREPROCESS_PURPOSE,
    "azure_speech.audio_briefing_duo_preprocess": AZURE_SPEECH_TTS_PREPROCESS_PURPOSE,
}

openai_chat_json = openai_provider._chat_json
openai_llm_meta = openai_provider._llm_meta
openrouter_chat_json = openrouter_provider._chat_json
openrouter_llm_meta = openrouter_provider._llm_meta
xai_chat_json = xai_provider._chat_json
xai_llm_meta = xai_provider._llm_meta


class TTSMarkupPreprocessService:
    def preprocess(
        self,
        *,
        text: str,
        model: str,
        api_key: str | None,
        prompt_key: str = DEFAULT_TTS_MARKUP_PREPROCESS_PROMPT_KEY,
        variables: dict[str, str] | None = None,
    ) -> dict:
        rendered_text = str(text or "").strip()
        if not rendered_text:
            raise RuntimeError("text is required")
        model_name = str(model or "").strip()
        if not model_name:
            raise RuntimeError("model is required")
        provider = provider_for_model(model_name)
        if not provider:
            raise RuntimeError(f"unsupported tts markup preprocess model provider: {model_name}")
        purpose = self._purpose_for_prompt_key(prompt_key)

        template = get_default_prompt_template(prompt_key)
        system_instruction = str(template.get("system_instruction") or "")
        prompt = self._build_prompt(str(template.get("prompt_text") or ""), rendered_text, prompt_key, variables)

        handlers = {
            "anthropic": lambda key: self._preprocess_anthropic(model_name, key, system_instruction, prompt, purpose),
            "google": lambda key: self._preprocess_gemini(model_name, key, system_instruction, prompt, purpose),
            "groq": lambda key: self._preprocess_openai_compat(groq_provider._chat_json, groq_provider._llm_meta, model_name, key, system_instruction, prompt, purpose),
            "deepseek": lambda key: self._preprocess_openai_compat(deepseek_provider._chat_json, deepseek_provider._llm_meta, model_name, key, system_instruction, prompt, purpose),
            "alibaba": lambda key: self._preprocess_openai_compat(alibaba_provider._chat_json, alibaba_provider._llm_meta, model_name, key, system_instruction, prompt, purpose),
            "mistral": lambda key: self._preprocess_openai_compat(mistral_provider._chat_json, mistral_provider._llm_meta, model_name, key, system_instruction, prompt, purpose),
            "moonshot": lambda key: self._preprocess_openai_compat(moonshot_provider._chat_json, moonshot_provider._llm_meta, model_name, key, system_instruction, prompt, purpose),
            "minimax": lambda key: self._preprocess_minimax(model_name, key, system_instruction, prompt, purpose),
            "xai": lambda key: self._preprocess_openai_compat(xai_chat_json, xai_llm_meta, model_name, key, system_instruction, prompt, purpose),
            "zai": lambda key: self._preprocess_openai_compat(zai_provider._chat_json, zai_provider._llm_meta, model_name, key, system_instruction, prompt, purpose),
            "fireworks": lambda key: self._preprocess_openai_compat(fireworks_provider._chat_json, fireworks_provider._llm_meta, model_name, key, system_instruction, prompt, purpose),
            "openai": lambda key: self._preprocess_openai_compat(openai_chat_json, openai_llm_meta, model_name, key, system_instruction, prompt, purpose),
            "openrouter": lambda key: self._preprocess_openai_compat(openrouter_chat_json, openrouter_llm_meta, model_name, key, system_instruction, prompt, purpose),
            "poe": lambda key: self._preprocess_openai_compat(poe_provider._chat_json, poe_provider._llm_meta, model_name, key, system_instruction, prompt, purpose),
            "siliconflow": lambda key: self._preprocess_openai_compat(siliconflow_provider._chat_json, siliconflow_provider._llm_meta, model_name, key, system_instruction, prompt, purpose),
            "deepinfra": lambda key: self._preprocess_openai_compat(deepinfra_provider._chat_json, deepinfra_provider._llm_meta, model_name, key, system_instruction, prompt, purpose),
        }
        handler = handlers.get(provider)
        if handler is None:
            raise RuntimeError(f"unsupported tts markup preprocess provider: {provider}")
        return handler((api_key or "").strip())

    def _build_prompt(self, prompt_text: str, rendered_text: str, prompt_key: str, variables: dict[str, str] | None) -> str:
        prompt_variables = self._enrich_variables(prompt_key, variables)
        prompt_variables["text"] = rendered_text
        rendered = render_prompt_template(prompt_text, prompt_variables)
        if rendered == prompt_text:
            return f"{rendered}\n\n{rendered_text}"
        return rendered

    def _enrich_variables(self, prompt_key: str, variables: dict[str, str] | None) -> dict[str, str]:
        enriched = {str(key): str(value) for key, value in (variables or {}).items()}
        if prompt_key in {
            "fish.audio_briefing_single_preprocess",
            "gemini.audio_briefing_single_preprocess",
            "elevenlabs.audio_briefing_single_preprocess",
            "xai.audio_briefing_single_preprocess",
            "azure_speech.audio_briefing_single_preprocess",
        }:
            persona_name = str(enriched.get("persona_name") or "").strip()
            if persona_name and not str(enriched.get("tone_prompt") or "").strip():
                tone_prompt, _speaking_style_prompt, _duo_conversation_prompt = resolve_audio_briefing_persona_prompts(persona_name)
                if tone_prompt:
                    enriched["tone_prompt"] = tone_prompt
        elif prompt_key in {
            "fish.audio_briefing_duo_preprocess",
            "gemini.audio_briefing_duo_preprocess",
            "elevenlabs.audio_briefing_duo_preprocess",
            "xai.audio_briefing_duo_preprocess",
            "azure_speech.audio_briefing_duo_preprocess",
        }:
            host_persona_name = str(enriched.get("host_persona_name") or "").strip()
            partner_persona_name = str(enriched.get("partner_persona_name") or "").strip()
            if host_persona_name and not str(enriched.get("host_tone_prompt") or "").strip():
                host_tone_prompt, _host_speaking_style_prompt, _host_duo_conversation_prompt = resolve_audio_briefing_persona_prompts(host_persona_name)
                if host_tone_prompt:
                    enriched["host_tone_prompt"] = host_tone_prompt
            if partner_persona_name and not str(enriched.get("partner_tone_prompt") or "").strip():
                partner_tone_prompt, _partner_speaking_style_prompt, _partner_duo_conversation_prompt = resolve_audio_briefing_persona_prompts(partner_persona_name)
                if partner_tone_prompt:
                    enriched["partner_tone_prompt"] = partner_tone_prompt
        return enriched

    def _purpose_for_prompt_key(self, prompt_key: str) -> str:
        normalized = str(prompt_key or "").strip()
        purpose = _PURPOSE_BY_PROMPT_KEY.get(normalized)
        if purpose is None:
            raise RuntimeError(f"unsupported tts markup preprocess prompt key: {normalized}")
        return purpose

    def _preprocess_openai_compat(self, chat_json, llm_meta, model: str, api_key: str, system_instruction: str, prompt: str, purpose: str) -> dict:
        text, usage = chat_json(
            prompt,
            model,
            api_key,
            system_instruction=system_instruction,
            max_output_tokens=_MAX_OUTPUT_TOKENS,
        )
        return {
            "text": str(text or "").strip(),
            "llm": llm_meta(model, purpose, usage),
        }

    def _preprocess_gemini(self, model: str, api_key: str, system_instruction: str, prompt: str, purpose: str) -> dict:
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
            "llm": gemini_llm_meta(model, purpose, usage),
        }

    def _preprocess_anthropic(self, model: str, api_key: str, system_instruction: str, prompt: str, purpose: str) -> dict:
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
            raise RuntimeError(f"anthropic tts markup preprocess failed{detail}")
        text = anthropic_message_text(message)
        return {
            "text": text,
            "llm": with_execution_failures(
                anthropic_llm_meta(message, purpose, used_model or model),
                execution_failures,
            ),
        }

    def _preprocess_minimax(self, model: str, api_key: str, system_instruction: str, prompt: str, purpose: str) -> dict:
        system_prompt = str(system_instruction or "").strip() or None
        combined_prompt = f"{system_prompt}\n\n{prompt}" if system_prompt else prompt
        message, used_model, execution_failures = minimax_call_with_model_fallback(
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
            raise RuntimeError(f"minimax tts markup preprocess failed{detail}")
        text = anthropic_message_text(message)
        return {
            "text": text,
            "llm": with_execution_failures(
                minimax_llm_meta(message, purpose, used_model or model),
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
