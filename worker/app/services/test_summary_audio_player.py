import unittest
from unittest.mock import patch

import httpx

from app.services import summary_audio_player
from app.services.openai_tts import synthesize_openai_tts


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
            persona="editor",
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

        with patch("app.services.summary_audio_player.synthesize_catalog_tts", return_value=(b"audio", "audio/mpeg", ".mp3", 5)) as synth:
            audio_base64, content_type, duration_sec, resolved_text = service.synthesize(
                provider="xai",
                voice_model="voice-1",
                voice_style="",
                tts_model="",
                text="summary text",
                persona="editor",
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

    def test_synthesize_uses_openai_provider(self):
        service = summary_audio_player.SummaryAudioPlayerService()

        with patch("app.services.summary_audio_player.synthesize_catalog_tts", return_value=(b"audio", "audio/mpeg", ".mp3", 5)) as synth:
            audio_base64, content_type, duration_sec, resolved_text = service.synthesize(
                provider="openai",
                voice_model="alloy",
                voice_style="",
                tts_model="gpt-4o-mini-tts",
                text="summary text",
                persona="editor",
                speech_rate=1.0,
                emotional_intensity=1.0,
                tempo_dynamics=1.0,
                line_break_silence_seconds=0.4,
                chunk_trailing_silence_seconds=1.25,
                pitch=0.0,
                volume_gain=0.0,
                user_dictionary_uuid=None,
                aivis_api_key=None,
                xai_api_key=None,
                openai_api_key="openai-key",
            )

        synth.assert_called_once()
        self.assertEqual(audio_base64, "YXVkaW8=")
        self.assertEqual(content_type, "audio/mpeg")
        self.assertEqual(duration_sec, 5)
        self.assertEqual(resolved_text, "summary text")

    def test_synthesize_uses_gemini_provider(self):
        service = summary_audio_player.SummaryAudioPlayerService()

        with patch("app.services.summary_audio_player.synthesize_gemini_tts", return_value=(b"audio", "audio/mpeg", ".mp3", 5)) as synth:
            audio_base64, content_type, duration_sec, resolved_text = service.synthesize(
                provider="gemini_tts",
                voice_model="Kore",
                voice_style="",
                tts_model="gemini-2.5-flash-tts",
                text="summary text",
                persona="snark",
                speech_rate=1.0,
                emotional_intensity=1.0,
                tempo_dynamics=1.0,
                line_break_silence_seconds=0.4,
                chunk_trailing_silence_seconds=1.25,
                pitch=0.0,
                volume_gain=0.0,
                user_dictionary_uuid=None,
                aivis_api_key=None,
                google_api_key=None,
                xai_api_key=None,
                openai_api_key=None,
            )

        synth.assert_called_once_with(
            model="gemini-2.5-flash-tts",
            voice_name="Kore",
            persona="snark",
            text="summary text",
            speech_rate=1.0,
        )
        self.assertEqual(audio_base64, "YXVkaW8=")
        self.assertEqual(content_type, "audio/mpeg")
        self.assertEqual(duration_sec, 5)
        self.assertEqual(resolved_text, "summary text")

    def test_synthesize_openai_tts_uses_current_openai_payload_shape(self):
        captured: dict[str, object] = {}

        def fake_post(url, headers=None, json=None, timeout=None):
            captured["url"] = url
            captured["headers"] = headers
            captured["json"] = json
            captured["timeout"] = timeout
            request = httpx.Request("POST", url)
            return httpx.Response(200, content=b"audio", request=request)

        with patch("app.services.openai_tts.httpx.post", side_effect=fake_post):
            audio_bytes, content_type, suffix, duration_sec = synthesize_openai_tts(
                endpoint="https://api.openai.com",
                api_key="openai-key",
                model="gpt-4o-mini-tts",
                voice_id="alloy",
                text="summary text",
                speech_rate=1.0,
                timeout_sec=30.0,
            )

        self.assertEqual(audio_bytes, b"audio")
        self.assertEqual(content_type, "audio/mpeg")
        self.assertEqual(suffix, ".mp3")
        self.assertEqual(duration_sec, 1)
        self.assertEqual(captured["url"], "https://api.openai.com/v1/audio/speech")
        self.assertEqual(captured["headers"], {"Authorization": "Bearer openai-key"})
        self.assertEqual(
            captured["json"],
            {
                "model": "gpt-4o-mini-tts",
                "voice": "alloy",
                "input": "summary text",
                "language": "ja",
                "response_format": "mp3",
            },
        )
