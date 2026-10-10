import os
import asyncio
import unittest
from unittest.mock import patch

from app.services import alibaba_service
from app.services.llm_catalog import load_llm_catalog, model_pricing, model_supports, provider_for_model


class AlibabaEndpointTests(unittest.TestCase):
    def test_request_workspace_overrides_shared_endpoint_and_is_reset(self):
        from app.services.alibaba_service import alibaba_workspace_context
        with patch.dict(os.environ, {"ALIBABA_API_BASE_URL": "https://dashscope-us.aliyuncs.com/compatible-mode/v1"}):
            with alibaba_workspace_context("ws-first"):
                self.assertEqual(alibaba_service._p._get_chat_url(), "https://ws-first.ap-northeast-1.maas.aliyuncs.com/compatible-mode/v1/chat/completions")
                with alibaba_workspace_context("ws-second"):
                    self.assertIn("ws-second.ap-northeast-1", alibaba_service._p._get_chat_url())
                self.assertIn("ws-first.ap-northeast-1", alibaba_service._p._get_chat_url())
            with alibaba_workspace_context(""):
                with self.assertRaisesRegex(RuntimeError, "workspace"):
                    alibaba_service._p._get_chat_url()

    def test_invalid_workspace_cannot_change_host(self):
        from app.services.alibaba_service import alibaba_workspace_context
        for workspace in ("https://example.com", "ws-a.evil", "ws-a/", "ws-", "ws-A", "ws-" + "a" * 61):
            with self.subTest(workspace=workspace), alibaba_workspace_context(workspace):
                with self.assertRaisesRegex(RuntimeError, "workspace"):
                    alibaba_service._p._get_chat_url()

    def test_workspace_base_url_is_normalized_for_chat(self):
        base = "https://llm-test.ap-northeast-1.maas.aliyuncs.com/compatible-mode/v1"
        for configured in (base, base + "/", base + "/chat/completions", " " + base + "/chat/completions/ "):
            with self.subTest(configured=configured), patch.dict(os.environ, {"ALIBABA_API_BASE_URL": configured}):
                self.assertEqual(alibaba_service._p._get_chat_url(), base + "/chat/completions")

    def test_missing_workspace_endpoint_does_not_fall_back_to_virginia(self):
        with patch.dict(os.environ, {}, clear=True):
            with self.assertRaisesRegex(RuntimeError, "ALIBABA_API_BASE_URL"):
                alibaba_service._p._get_chat_url()
        for configured in ("", " "):
            with self.subTest(configured=configured), patch.dict(os.environ, {"ALIBABA_API_BASE_URL": configured}):
                with self.assertRaisesRegex(RuntimeError, "ALIBABA_API_BASE_URL"):
                    alibaba_service._p._get_chat_url()


class AlibabaRequestIsolationTests(unittest.IsolatedAsyncioTestCase):
    async def test_concurrent_requests_and_threaded_calls_keep_their_workspace(self):
        from fastapi import FastAPI
        import httpx
        from app import main

        app = FastAPI()
        app.middleware("http")(main.require_internal_worker_secret)

        @app.get("/chat-test")
        async def endpoint():
            await asyncio.sleep(0)
            return {"url": await asyncio.to_thread(alibaba_service._p._get_chat_url)}

        with patch.object(main, "_INTERNAL_WORKER_SECRET", "test-secret"):
            async with httpx.AsyncClient(transport=httpx.ASGITransport(app), base_url="http://worker") as client:
                async def call(workspace):
                    return await client.get("/chat-test", headers={"X-Internal-Worker-Secret": "test-secret", "X-Alibaba-Workspace-Id": workspace})
                first, second = await asyncio.gather(call("ws-first"), call("ws-second"))
                self.assertEqual(first.json()["url"], "https://ws-first.ap-northeast-1.maas.aliyuncs.com/compatible-mode/v1/chat/completions")
                self.assertEqual(second.json()["url"], "https://ws-second.ap-northeast-1.maas.aliyuncs.com/compatible-mode/v1/chat/completions")
                with patch.dict(os.environ, {"ALIBABA_API_BASE_URL": "https://dashscope-us.aliyuncs.com/compatible-mode/v1"}):
                    with self.assertRaisesRegex(RuntimeError, "workspace"):
                        await call("")


class AlibabaCatalogTests(unittest.TestCase):
    def test_tokyo_selection_uses_models_with_confirmed_regional_prices(self):
        catalog = load_llm_catalog()
        selectable = {entry["id"] for entry in catalog["chat_models"] if entry["provider"] == "alibaba" and entry["available_purposes"]}
        self.assertEqual(selectable, {"qwen3.8-max", "qwen3.8-flash", "qwen-plus-character", "qwen3.7-max", "qwen3.7-plus", "qwen3.6-plus", "qwen3.6-flash"})
        provider = next(entry for entry in catalog["providers"] if entry["id"] == "alibaba")
        self.assertEqual(provider["default_models"]["summary"], "qwen3.7-plus")
        self.assertEqual(provider["default_models"]["ask"], "qwen3.7-plus")
        for entry in catalog["chat_models"]:
            if entry["id"] in selectable:
                self.assertEqual(entry["pricing"]["pricing_source"], "alibaba_modelstudio_tokyo_global_2026_10_10")

    def test_tokyo_plus_and_flash_include_long_context_rates(self):
        for model, rates in (("qwen3.7-plus", (0.826, 3.301)), ("qwen3.6-plus", (1.101, 6.602)), ("qwen3.6-flash", (0.66, 3.961))):
            with self.subTest(model=model):
                pricing = model_pricing(model)["long_context"]
                self.assertEqual(pricing["input_token_threshold"], 256000)
                self.assertEqual((pricing["input_per_mtok_usd"], pricing["output_per_mtok_usd"]), rates)

    def test_tokyo_implicit_cache_is_charged_at_the_published_rate(self):
        usage = {"input_tokens": 1000000, "output_tokens": 0, "cache_read_input_tokens": 1000000}
        for model, expected in (("qwen3.8-max", 0.206), ("qwen3.8-flash", 0.014), ("qwen3.7-plus", 0.166)):
            with self.subTest(model=model):
                self.assertEqual(alibaba_service._p._estimate_cost_usd(model, "summary", usage), expected)

    def test_qwen_plus_character_is_available(self):
        pricing = model_pricing("qwen-plus-character")

        self.assertEqual(provider_for_model("qwen-plus-character"), "alibaba")
        self.assertIsNotNone(pricing)
        self.assertEqual(pricing["input_per_mtok_usd"], 0.115)
        self.assertEqual(pricing["output_per_mtok_usd"], 0.287)
        self.assertTrue(model_supports("qwen-plus-character", "supports_structured_output"))
        self.assertFalse(model_supports("qwen-plus-character", "supports_reasoning"))
        self.assertFalse(model_supports("qwen-plus-character", "supports_tool_calling"))

    def test_qwen_flash_character_is_available(self):
        pricing = model_pricing("qwen-flash-character")

        self.assertEqual(provider_for_model("qwen-flash-character"), "alibaba")
        self.assertIsNotNone(pricing)
        self.assertEqual(pricing["input_per_mtok_usd"], 0.034)
        self.assertEqual(pricing["output_per_mtok_usd"], 0.203)
        self.assertTrue(model_supports("qwen-flash-character", "supports_structured_output"))
        self.assertFalse(model_supports("qwen-flash-character", "supports_reasoning"))
        self.assertFalse(model_supports("qwen-flash-character", "supports_tool_calling"))

    def test_qwen38_flash_is_available(self):
        pricing = model_pricing("qwen3.8-flash")

        self.assertEqual(provider_for_model("qwen3.8-flash"), "alibaba")
        self.assertIsNotNone(pricing)
        self.assertEqual(pricing["input_per_mtok_usd"], 0.113)
        self.assertEqual(pricing["output_per_mtok_usd"], 0.382)
        self.assertNotIn("cache_write_per_mtok_usd", pricing)
        self.assertEqual(pricing["cache_read_per_mtok_usd"], 0.014)
        self.assertTrue(model_supports("qwen3.8-flash", "supports_structured_output"))
        self.assertTrue(model_supports("qwen3.8-flash", "supports_reasoning"))
        self.assertTrue(model_supports("qwen3.8-flash", "supports_tool_calling"))
        self.assertFalse(model_supports("qwen3.8-flash", "supports_strict_json_schema"))
        self.assertFalse(model_supports("qwen3.8-flash", "supports_cache_write_pricing"))
        self.assertTrue(model_supports("qwen3.8-flash", "supports_cache_read_pricing"))

    def test_qwen38_max_is_available(self):
        pricing = model_pricing("qwen3.8-max")

        self.assertEqual(provider_for_model("qwen3.8-max"), "alibaba")
        self.assertIsNotNone(pricing)
        self.assertEqual(pricing["input_per_mtok_usd"], 1.65)
        self.assertEqual(pricing["output_per_mtok_usd"], 4.951)
        self.assertNotIn("cache_write_per_mtok_usd", pricing)
        self.assertEqual(pricing["cache_read_per_mtok_usd"], 0.206)
        self.assertTrue(model_supports("qwen3.8-max", "supports_structured_output"))
        self.assertTrue(model_supports("qwen3.8-max", "supports_reasoning"))
        self.assertTrue(model_supports("qwen3.8-max", "supports_tool_calling"))
        self.assertFalse(model_supports("qwen3.8-max", "supports_strict_json_schema"))
        self.assertFalse(model_supports("qwen3.8-max", "supports_cache_write_pricing"))
        self.assertTrue(model_supports("qwen3.8-max", "supports_cache_read_pricing"))

    def test_qwen37_max_is_available(self):
        pricing = model_pricing("qwen3.7-max")

        self.assertEqual(provider_for_model("qwen3.7-max"), "alibaba")
        self.assertIsNotNone(pricing)
        self.assertEqual(pricing["input_per_mtok_usd"], 1.65)
        self.assertEqual(pricing["output_per_mtok_usd"], 4.951)
        self.assertNotIn("cache_write_per_mtok_usd", pricing)
        self.assertNotIn("cache_read_per_mtok_usd", pricing)
        self.assertTrue(model_supports("qwen3.7-max", "supports_structured_output"))
        self.assertTrue(model_supports("qwen3.7-max", "supports_reasoning"))
        self.assertFalse(model_supports("qwen3.7-max", "supports_cache_write_pricing"))
        self.assertFalse(model_supports("qwen3.7-max", "supports_cache_read_pricing"))

    def test_qwen37_plus_is_available(self):
        pricing = model_pricing("qwen3.7-plus")

        self.assertEqual(provider_for_model("qwen3.7-plus"), "alibaba")
        self.assertIsNotNone(pricing)
        self.assertEqual(pricing["input_per_mtok_usd"], 0.276)
        self.assertEqual(pricing["output_per_mtok_usd"], 1.101)
        self.assertNotIn("cache_write_per_mtok_usd", pricing)
        self.assertEqual(pricing["cache_read_per_mtok_usd"], 0.056)
        self.assertTrue(model_supports("qwen3.7-plus", "supports_structured_output"))
        self.assertTrue(model_supports("qwen3.7-plus", "supports_reasoning"))
        self.assertFalse(model_supports("qwen3.7-plus", "supports_cache_write_pricing"))
        self.assertTrue(model_supports("qwen3.7-plus", "supports_cache_read_pricing"))


if __name__ == "__main__":
    unittest.main()
