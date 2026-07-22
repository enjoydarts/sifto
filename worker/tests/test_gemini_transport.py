import asyncio
import unittest
from unittest.mock import Mock, patch

from app.services.gemini_transport import generate_content, generate_content_async


class _FakeResponse:
    status_code = 200
    text = ""

    def json(self):
        return {
            "candidates": [{"content": {"parts": [{"text": "ok"}]}}],
            "usageMetadata": {
                "promptTokenCount": 1,
                "candidatesTokenCount": 1,
            },
        }


class _FakeClient:
    last_json = None

    def __init__(self, **_kwargs):
        pass

    def __enter__(self):
        return self

    def __exit__(self, *_args):
        return None

    def post(self, _url, *, json, params):
        type(self).last_json = json
        return _FakeResponse()


class _FakeAsyncClient:
    last_json = None

    def __init__(self, **_kwargs):
        pass

    async def __aenter__(self):
        return self

    async def __aexit__(self, *_args):
        return None

    async def post(self, _url, *, json, params):
        type(self).last_json = json
        return _FakeResponse()


def _normalize(model):
    return model.removeprefix("models/")


class GeminiTransportSamplingTests(unittest.TestCase):
    @patch("app.services.gemini_transport.httpx.Client", _FakeClient)
    def test_new_models_omit_sampling_parameters_sync(self):
        for model in (
            "gemini-3.6-flash",
            "gemini-3.5-flash-lite",
            "models/gemini-3.6-flash-20260721",
        ):
            with self.subTest(model=model):
                generate_content(
                    "prompt",
                    model,
                    "key",
                    normalize_model_name=_normalize,
                    logger=Mock(),
                    temperature=0.7,
                    top_p=0.8,
                )
                config = _FakeClient.last_json["generationConfig"]
                self.assertNotIn("temperature", config)
                self.assertNotIn("topP", config)
                self.assertEqual(config["maxOutputTokens"], 1024)

    @patch("app.services.gemini_transport.httpx.Client", _FakeClient)
    def test_legacy_model_keeps_sampling_parameters_sync(self):
        generate_content(
            "prompt",
            "gemini-2.5-flash",
            "key",
            normalize_model_name=_normalize,
            logger=Mock(),
            temperature=0.7,
            top_p=0.8,
        )
        config = _FakeClient.last_json["generationConfig"]
        self.assertEqual(config["temperature"], 0.7)
        self.assertEqual(config["topP"], 0.8)

    @patch("app.services.gemini_transport.httpx.AsyncClient", _FakeAsyncClient)
    def test_new_models_omit_sampling_parameters_async(self):
        for model in (
            "gemini-3.6-flash",
            "gemini-3.5-flash-lite",
            "models/gemini-3.5-flash-lite-20260721",
        ):
            with self.subTest(model=model):
                asyncio.run(
                    generate_content_async(
                        "prompt",
                        model,
                        "key",
                        normalize_model_name=_normalize,
                        logger=Mock(),
                        temperature=0.7,
                        top_p=0.8,
                    )
                )
                config = _FakeAsyncClient.last_json["generationConfig"]
                self.assertNotIn("temperature", config)
                self.assertNotIn("topP", config)
                self.assertEqual(config["maxOutputTokens"], 1024)


if __name__ == "__main__":
    unittest.main()
