import unittest

from app.services.llm_catalog import provider_api_key_header, provider_for_model, resolve_model_id


class DeepInfraCatalogTests(unittest.TestCase):
    def test_provider_for_model_detects_deepinfra_alias(self):
        self.assertEqual(provider_for_model("deepinfra::meta-llama/Meta-Llama-3.3-70B-Instruct-Turbo"), "deepinfra")

    def test_resolve_model_id_strips_deepinfra_alias(self):
        self.assertEqual(resolve_model_id("deepinfra::meta-llama/Meta-Llama-3.3-70B-Instruct-Turbo"), "meta-llama/Meta-Llama-3.3-70B-Instruct-Turbo")

    def test_provider_api_key_header_uses_openai_compatible_internal_header(self):
        self.assertEqual(provider_api_key_header("deepinfra"), "x-openai-api-key")


class DeepInfraServiceTests(unittest.TestCase):
    def test_chat_completions_url_normalizes_base(self):
        from app.services.deepinfra_service import _chat_completions_url

        self.assertEqual(_chat_completions_url("https://api.deepinfra.com/v1/openai"), "https://api.deepinfra.com/v1/openai/chat/completions")
        self.assertEqual(
            _chat_completions_url("https://api.deepinfra.com/v1/openai/chat/completions"),
            "https://api.deepinfra.com/v1/openai/chat/completions",
        )


if __name__ == "__main__":
    unittest.main()
