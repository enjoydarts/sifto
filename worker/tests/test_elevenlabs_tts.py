import unittest
from unittest import mock

import httpx

from app.services.elevenlabs_tts import synthesize_elevenlabs_dialogue_tts, synthesize_elevenlabs_tts


class ElevenLabsTTSServiceTests(unittest.TestCase):
    def test_synthesize_elevenlabs_tts_uses_expected_payload_and_headers(self):
        captured: dict[str, object] = {}

        def fake_post(url, headers=None, json=None, timeout=None):
            captured["url"] = url
            captured["headers"] = headers
            captured["json"] = json
            captured["timeout"] = timeout
            request = httpx.Request("POST", url)
            return httpx.Response(200, content=b"audio", request=request)

        with mock.patch("app.services.elevenlabs_tts.httpx.post", side_effect=fake_post):
            audio_bytes, content_type, suffix, duration_sec = synthesize_elevenlabs_tts(
                endpoint="https://api.elevenlabs.io",
                api_key="eleven-key",
                model="eleven_multilingual_v2",
                voice_id="voice-1",
                text="summary text",
                timeout_sec=30.0,
            )

        self.assertEqual(audio_bytes, b"audio")
        self.assertEqual(content_type, "audio/mpeg")
        self.assertEqual(suffix, ".mp3")
        self.assertGreaterEqual(duration_sec, 1)
        self.assertEqual(captured["url"], "https://api.elevenlabs.io/v1/text-to-speech/voice-1")
        self.assertEqual(
            captured["headers"],
            {
                "xi-api-key": "eleven-key",
                "Content-Type": "application/json",
                "Accept": "audio/mpeg",
            },
        )
        self.assertEqual(
            captured["json"],
            {
                "text": "summary text",
                "model_id": "eleven_multilingual_v2",
                "output_format": "mp3_44100_128",
            },
        )

    def test_synthesize_elevenlabs_dialogue_tts_uses_expected_payload(self):
        captured: dict[str, object] = {}

        def fake_post(url, headers=None, json=None, timeout=None):
            captured["url"] = url
            captured["headers"] = headers
            captured["json"] = json
            request = httpx.Request("POST", url)
            return httpx.Response(200, content=b"audio", request=request)

        with mock.patch("app.services.elevenlabs_tts.httpx.post", side_effect=fake_post):
            audio_bytes, content_type, suffix, duration_sec = synthesize_elevenlabs_dialogue_tts(
                endpoint="https://api.elevenlabs.io",
                api_key="eleven-key",
                model="eleven_v3",
                turns=[
                    {"text": "最初の話題です。", "voice_id": "host-voice"},
                    {"text": "補足します。", "voice_id": "partner-voice"},
                ],
                timeout_sec=30.0,
            )

        self.assertEqual(audio_bytes, b"audio")
        self.assertEqual(content_type, "audio/mpeg")
        self.assertEqual(suffix, ".mp3")
        self.assertGreaterEqual(duration_sec, 1)
        self.assertEqual(captured["url"], "https://api.elevenlabs.io/v1/text-to-dialogue")
        self.assertEqual(
            captured["headers"],
            {
                "xi-api-key": "eleven-key",
                "Content-Type": "application/json",
                "Accept": "audio/mpeg",
            },
        )
        self.assertEqual(
            captured["json"],
            {
                "inputs": [
                    {"text": "最初の話題です。", "voice_id": "host-voice"},
                    {"text": "補足します。", "voice_id": "partner-voice"},
                ],
                "model_id": "eleven_v3",
                "output_format": "mp3_44100_128",
            },
        )

    def test_synthesize_elevenlabs_dialogue_tts_rejects_non_v3_models(self):
        with self.assertRaisesRegex(RuntimeError, "eleven_v3"):
            synthesize_elevenlabs_dialogue_tts(
                endpoint="https://api.elevenlabs.io",
                api_key="eleven-key",
                model="eleven_multilingual_v2",
                turns=[{"text": "最初の話題です。", "voice_id": "host-voice"}],
                timeout_sec=30.0,
            )
