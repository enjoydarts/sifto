import unittest

from app.services.anthropic_transport import message_text


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


if __name__ == "__main__":
    unittest.main()
