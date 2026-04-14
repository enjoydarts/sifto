import asyncio
import unittest

from app.services.claude_service import summarize, summarize_async


class ClaudeServiceTests(unittest.TestCase):
    def test_summarize_requires_api_key(self):
        with self.assertRaisesRegex(RuntimeError, "anthropic api key is required for summary"):
            summarize(
                title="title",
                facts=["fact"],
                api_key=None,
            )

    def test_summarize_async_requires_api_key(self):
        with self.assertRaisesRegex(RuntimeError, "anthropic api key is required for summary"):
            asyncio.run(
                summarize_async(
                    title="title",
                    facts=["fact"],
                    api_key=None,
                )
            )


if __name__ == "__main__":
    unittest.main()
