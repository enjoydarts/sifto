import asyncio
import unittest
from unittest.mock import AsyncMock, Mock, patch

from app.services.anthropic_transport import message_text, messages_create, messages_create_async


class AnthropicTransportTests(unittest.TestCase):
    def test_message_text_skips_thinking_blocks(self):
        message = type(
            "Message",
            (),
            {
                "content": [
                    type("ThinkingBlock", (), {"type": "thinking", "thinking": "..."})(),
                    type("TextBlock", (), {"type": "text", "text": "本文"})(),
                ]
            },
        )()

        self.assertEqual(message_text(message), "本文")

    def test_message_text_joins_multiple_text_blocks(self):
        message = type(
            "Message",
            (),
            {
                "content": [
                    type("TextBlock", (), {"type": "text", "text": "前半"})(),
                    type("TextBlock", (), {"type": "text", "text": "後半"})(),
                ]
            },
        )()

        self.assertEqual(message_text(message), "前半\n後半")

    @patch("app.services.anthropic_transport.client_for_api_key")
    def test_messages_create_omits_sampling_parameters_for_opus_5(self, client_for_api_key):
        client = type("Client", (), {})()
        client.messages = type("Messages", (), {})()
        client.messages.create = Mock(return_value=object())
        client_for_api_key.return_value = client

        messages_create(
            "prompt",
            "claude-opus-5",
            api_key="anthropic-key",
            temperature=0.2,
            top_p=0.8,
        )

        kwargs = client.messages.create.call_args.kwargs
        self.assertNotIn("temperature", kwargs)
        self.assertNotIn("top_p", kwargs)

    @patch("app.services.anthropic_transport.async_client_for_api_key")
    def test_messages_create_async_omits_sampling_parameters_for_opus_5(self, async_client_for_api_key):
        client = type("Client", (), {})()
        client.messages = type("Messages", (), {})()
        client.messages.create = AsyncMock(return_value=object())
        async_client_for_api_key.return_value = client

        asyncio.run(
            messages_create_async(
                "prompt",
                "claude-opus-5",
                api_key="anthropic-key",
                temperature=0.2,
                top_p=0.8,
            )
        )

        kwargs = client.messages.create.call_args.kwargs
        self.assertNotIn("temperature", kwargs)
        self.assertNotIn("top_p", kwargs)


if __name__ == "__main__":
    unittest.main()
