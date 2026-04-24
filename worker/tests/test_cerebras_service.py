import unittest

from app.services.llm_catalog import provider_api_key_header, provider_for_model, resolve_model_id
from app.services.openai_compat_transport import _apply_openai_compat_request_overrides


class CerebrasCatalogTests(unittest.TestCase):
    def test_provider_for_model_detects_cerebras_alias(self):
        self.assertEqual(provider_for_model("cerebras::gpt-oss-120b"), "cerebras")

    def test_provider_for_model_detects_gpt_oss_120b(self):
        self.assertEqual(provider_for_model("gpt-oss-120b"), "cerebras")

    def test_resolve_model_id_strips_cerebras_alias(self):
        self.assertEqual(resolve_model_id("cerebras::gpt-oss-120b"), "gpt-oss-120b")

    def test_provider_api_key_header_uses_cerebras_header(self):
        self.assertEqual(provider_api_key_header("cerebras"), "x-cerebras-api-key")


class CerebrasServiceTests(unittest.TestCase):
    def test_provider_config(self):
        from app.services.cerebras_service import _p

        self.assertEqual(_p.config.provider_name, "cerebras")
        self.assertEqual(_p.config.env_prefix, "CEREBRAS")
        self.assertEqual(_p.config.pricing_source_version, "cerebras_pricing_2026_04")
        self.assertEqual(_p.config.api_base_url, "https://api.cerebras.ai/v1/chat/completions")
        self.assertEqual(_p.config.default_model, "gpt-oss-120b")
        self.assertEqual(_p.config.default_translate_model, "llama3.1-8b")

    def test_gpt_oss_uses_low_reasoning_effort_for_structured_tasks(self):
        body = {"model": "gpt-oss-120b", "response_format": {"type": "json_object"}}

        _apply_openai_compat_request_overrides("cerebras", "gpt-oss-120b", body)

        self.assertEqual(body["reasoning_effort"], "low")

    def test_glm_disables_reasoning_for_structured_tasks(self):
        body = {"model": "zai-glm-4.7", "response_format": {"type": "json_object"}}

        _apply_openai_compat_request_overrides("cerebras", "zai-glm-4.7", body)

        self.assertEqual(body["reasoning_effort"], "none")


if __name__ == "__main__":
    unittest.main()
