import unittest
from unittest import mock

import httpx

from app.services.fish_tts import build_fish_duo_text, synthesize_fish_multi_speaker_tts, synthesize_fish_tts


class FishTTSServiceTests(unittest.TestCase):
    def test_synthesize_fish_tts_uses_expected_payload_and_headers(self):
        captured: dict[str, object] = {}

        def fake_post(url, headers=None, json=None, timeout=None):
            captured["url"] = url
            captured["headers"] = headers
            captured["json"] = json
            captured["timeout"] = timeout
            request = httpx.Request("POST", url)
            return httpx.Response(200, content=b"audio", request=request)

        with mock.patch("app.services.fish_tts.httpx.post", side_effect=fake_post):
            audio_bytes, content_type, suffix, duration_sec = synthesize_fish_tts(
                model="s2-pro",
                voice_name="fish-voice-1",
                text="summary text",
                speech_rate=1.2,
                volume_gain=0.5,
                api_key="fish-key",
                timeout_sec=30.0,
            )

        self.assertEqual(audio_bytes, b"audio")
        self.assertEqual(content_type, "audio/mpeg")
        self.assertEqual(suffix, ".mp3")
        self.assertEqual(duration_sec, 1)
        self.assertEqual(captured["url"], "https://api.fish.audio/v1/tts")
        self.assertEqual(
            captured["headers"],
            {
                "Authorization": "Bearer fish-key",
                "Content-Type": "application/json",
                "model": "s2-pro",
            },
        )
        self.assertEqual(
            captured["json"],
            {
                "text": "summary text",
                "reference_id": "fish-voice-1",
                "prosody": {"speed": 1.2, "volume": 0.5},
                "chunk_length": 300,
                "normalize": True,
                "format": "mp3",
                "sample_rate": 44100,
                "mp3_bitrate": 192,
                "latency": "balanced",
                "temperature": 0.7,
                "top_p": 0.7,
            },
        )

    def test_build_fish_duo_text_uses_speaker_tags(self):
        text = build_fish_duo_text(
            [
                {"speaker": "host", "text": "最初の話題です。"},
                {"speaker": "partner", "text": "補足します。"},
            ]
        )

        self.assertEqual(text, "<|speaker:0|>最初の話題です。<|speaker:1|>補足します。")

    def test_synthesize_fish_multi_speaker_tts_uses_expected_payload(self):
        captured: dict[str, object] = {}

        def fake_post(url, headers=None, json=None, timeout=None):
            captured["url"] = url
            captured["headers"] = headers
            captured["json"] = json
            captured["timeout"] = timeout
            request = httpx.Request("POST", url)
            return httpx.Response(200, content=b"audio", request=request)

        with mock.patch("app.services.fish_tts.httpx.post", side_effect=fake_post):
            audio_bytes, content_type, suffix, duration_sec = synthesize_fish_multi_speaker_tts(
                model="s2-pro",
                host_voice_name="host-voice",
                partner_voice_name="partner-voice",
                turns=[
                    {"speaker": "host", "text": "最初の話題です。"},
                    {"speaker": "partner", "text": "補足します。"},
                ],
                api_key="fish-key",
                timeout_sec=30.0,
            )

        self.assertEqual(audio_bytes, b"audio")
        self.assertEqual(content_type, "audio/mpeg")
        self.assertEqual(suffix, ".mp3")
        self.assertEqual(duration_sec, 1)
        self.assertEqual(captured["headers"]["Authorization"], "Bearer fish-key")
        self.assertEqual(captured["headers"]["model"], "s2-pro")
        self.assertEqual(
            captured["json"],
            {
                "text": "<|speaker:0|>最初の話題です。<|speaker:1|>補足します。",
                "reference_id": ["host-voice", "partner-voice"],
                "prosody": {"speed": 1.0, "volume": 0.0},
                "chunk_length": 300,
                "normalize": True,
                "format": "mp3",
                "sample_rate": 44100,
                "mp3_bitrate": 192,
                "latency": "balanced",
                "temperature": 0.7,
                "top_p": 0.7,
            },
        )
