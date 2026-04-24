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


class CerebrasDispatchTests(unittest.TestCase):
    def test_dispatch_routes_gpt_oss_120b_and_reads_cerebras_header(self):
        request = _request_with_headers({"x-cerebras-api-key": "cerebras-key"})
        result = dispatch_by_model(
            request,
            "gpt-oss-120b",
            handlers={
                "cerebras": lambda api_key: {"provider": "cerebras", "api_key": api_key},
                "anthropic": lambda api_key: {"provider": "anthropic", "api_key": api_key},
            },
        )

        self.assertEqual(result, {"provider": "cerebras", "api_key": "cerebras-key"})

    def test_dispatch_routes_cerebras_alias_and_reads_cerebras_header(self):
        request = _request_with_headers({"x-cerebras-api-key": "cerebras-key"})
        result = dispatch_by_model(
            request,
            "cerebras::gpt-oss-120b",
            handlers={
                "cerebras": lambda api_key: {"provider": "cerebras", "api_key": api_key},
                "anthropic": lambda api_key: {"provider": "anthropic", "api_key": api_key},
            },
        )

        self.assertEqual(result, {"provider": "cerebras", "api_key": "cerebras-key"})

    def test_auto_dispatch_can_build_handler_map_for_cerebras_service(self):
        handlers = build_handler_map(
            "summarize",
            args_fn=lambda task_func, api_key: (task_func.__self__.config.provider_name, api_key),
            providers=["cerebras"],
        )

        self.assertEqual(handlers["cerebras"]("cerebras-key"), ("cerebras", "cerebras-key"))


if __name__ == "__main__":
    unittest.main()
