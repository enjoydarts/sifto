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


class DeepInfraDispatchTests(unittest.TestCase):
    def test_dispatch_routes_deepinfra_alias_and_reads_openai_compatible_header(self):
        request = _request_with_headers({"x-openai-api-key": "deepinfra-key"})
        result = dispatch_by_model(
            request,
            "deepinfra::meta-llama/Meta-Llama-3.3-70B-Instruct-Turbo",
            handlers={
                "deepinfra": lambda api_key: {"provider": "deepinfra", "api_key": api_key},
                "anthropic": lambda api_key: {"provider": "anthropic", "api_key": api_key},
            },
        )

        self.assertEqual(result, {"provider": "deepinfra", "api_key": "deepinfra-key"})

    def test_auto_dispatch_can_build_handler_map_for_deepinfra_service(self):
        handlers = build_handler_map(
            "summarize",
            args_fn=lambda task_func, api_key: (task_func.__self__.config.provider_name, api_key),
            providers=["deepinfra"],
        )

        self.assertEqual(handlers["deepinfra"]("deepinfra-key"), ("deepinfra", "deepinfra-key"))


if __name__ == "__main__":
    unittest.main()
