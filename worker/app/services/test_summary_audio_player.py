import unittest
from unittest.mock import patch

import httpx

from app.services import summary_audio_player


class SummaryAudioPlayerTests(unittest.TestCase):
    def test_build_summary_audio_text_prefers_translated_title(self):
        text = summary_audio_player.build_summary_audio_text(
            translated_title="邦題タイトル",
            original_title="Original Title",
            summary="要約本文",
        )

        self.assertEqual(text, "邦題タイトル\n\n要約本文")

    def test_build_summary_audio_text_falls_back_to_original_title(self):
        text = summary_audio_player.build_summary_audio_text(
            translated_title="",
            original_title="Original Title",
            summary="要約本文",
        )

        self.assertEqual(text, "Original Title\n\n要約本文")

    def test_synthesize_passes_chunk_trailing_silence_to_aivis(self):
        captured: dict[str, object] = {}

        class FakeAivis:
            def synthesize(self, **kwargs):
                captured.update(kwargs)
                return (b"audio", "audio/mpeg", "resolved", 3)

        service = summary_audio_player.SummaryAudioPlayerService()
        service.aivis = FakeAivis()

        audio_base64, content_type, duration_sec, resolved_text = service.synthesize(
            provider="aivis",
            voice_model="model",
            voice_style="speaker:1",
            text="summary text",
            speech_rate=1.0,
            emotional_intensity=1.0,
            tempo_dynamics=1.0,
            line_break_silence_seconds=0.4,
            chunk_trailing_silence_seconds=1.25,
            pitch=0.0,
            volume_gain=0.0,
            user_dictionary_uuid=None,
            aivis_api_key=None,
        )

        self.assertEqual(content_type, "audio/mpeg")
        self.assertEqual(duration_sec, 3)
        self.assertEqual(resolved_text, "summary text")
        self.assertEqual(captured["chunk_trailing_silence_seconds"], 1.25)

    def test_synthesize_uses_xai_provider(self):
        service = summary_audio_player.SummaryAudioPlayerService()

        with patch("app.services.summary_audio_player.synthesize_xai_tts", return_value=(b"audio", "audio/mpeg", ".mp3", 5)) as synth:
            audio_base64, content_type, duration_sec, resolved_text = service.synthesize(
                provider="xai",
                voice_model="voice-1",
                voice_style="",
                text="summary text",
                speech_rate=1.0,
                emotional_intensity=1.0,
                tempo_dynamics=1.0,
                line_break_silence_seconds=0.4,
                chunk_trailing_silence_seconds=1.25,
                pitch=0.0,
                volume_gain=0.0,
                user_dictionary_uuid=None,
                aivis_api_key=None,
                xai_api_key="xai-key",
            )

        synth.assert_called_once()
        self.assertEqual(audio_base64, "YXVkaW8=")
        self.assertEqual(content_type, "audio/mpeg")
        self.assertEqual(duration_sec, 5)
        self.assertEqual(resolved_text, "summary text")

    def test_synthesize_xai_tts_uses_current_xai_payload_shape(self):
        captured: dict[str, object] = {}

        def fake_post(url, headers=None, json=None, timeout=None):
            captured["url"] = url
            captured["headers"] = headers
            captured["json"] = json
            captured["timeout"] = timeout
            request = httpx.Request("POST", url)
            return httpx.Response(200, content=b"audio", request=request)

        with patch("app.services.xai_tts.httpx.post", side_effect=fake_post):
            audio_bytes, content_type, suffix, duration_sec = summary_audio_player.synthesize_xai_tts(
                endpoint="https://api.x.ai",
                api_key="xai-key",
                voice_id="voice-1",
                text="summary text",
                speech_rate=1.0,
                timeout_sec=30.0,
            )

        self.assertEqual(audio_bytes, b"audio")
        self.assertEqual(content_type, "audio/mpeg")
        self.assertEqual(suffix, ".mp3")
        self.assertEqual(duration_sec, 1)
        self.assertEqual(captured["url"], "https://api.x.ai/v1/tts")
        self.assertEqual(captured["headers"], {"Authorization": "Bearer xai-key"})
        self.assertEqual(
            captured["json"],
            {
                "text": "summary text",
                "voice_id": "voice-1",
                "language": "ja",
                "output_format": {
                    "codec": "mp3",
                    "sample_rate": 44100,
                    "bit_rate": 192000,
                },
            },
        )
