import unittest

from fastapi import Request

from app.auto_dispatch import build_handler_map
from app.services.llm_dispatch import dispatch_by_model


def _request_with_headers(headers: dict[str, str]) -> Request:
    return Request(
        {
            "type": "http",
            "headers": [(str(k).lower().encode("latin-1"), str(v).encode("latin-1")) for k, v in headers.items()],
        }
    )


class FeatherlessDispatchTests(unittest.TestCase):
    def test_dispatch_routes_featherless_alias_and_reads_openai_compatible_header(self):
        request = _request_with_headers({"x-openai-api-key": "featherless-key"})
        result = dispatch_by_model(
            request,
            "featherless::Qwen/Qwen3.5-9B",
            handlers={
                "featherless": lambda api_key: {"provider": "featherless", "api_key": api_key},
                "anthropic": lambda api_key: {"provider": "anthropic", "api_key": api_key},
            },
        )

        self.assertEqual(result, {"provider": "featherless", "api_key": "featherless-key"})

    def test_auto_dispatch_can_build_handler_map_for_featherless_service(self):
        handlers = build_handler_map(
            "summarize",
            args_fn=lambda task_func, api_key: (task_func.__self__.config.provider_name, api_key),
            providers=["featherless"],
        )

        self.assertEqual(handlers["featherless"]("featherless-key"), ("featherless", "featherless-key"))


if __name__ == "__main__":
    unittest.main()
