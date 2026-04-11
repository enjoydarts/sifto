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

        with patch("app.services.summary_audio_player.synthesize_single_speaker_tts", return_value=(b"audio", "audio/mpeg", ".mp3", 5)) as synth:
            audio_base64, content_type, duration_sec, resolved_text = service.synthesize(
                provider="xai",
                voice_model="voice-1",
                voice_style="",
                tts_model="",
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

    def test_synthesize_uses_openai_provider(self):
        service = summary_audio_player.SummaryAudioPlayerService()

        with patch("app.services.summary_audio_player.synthesize_single_speaker_tts", return_value=(b"audio", "audio/mpeg", ".mp3", 5)) as synth:
            audio_base64, content_type, duration_sec, resolved_text = service.synthesize(
                provider="openai",
                voice_model="alloy",
                voice_style="",
                tts_model="gpt-4o-mini-tts",
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
                xai_api_key=None,
                openai_api_key="openai-key",
            )

        synth.assert_called_once()
        self.assertEqual(audio_base64, "YXVkaW8=")
        self.assertEqual(content_type, "audio/mpeg")
        self.assertEqual(duration_sec, 5)
        self.assertEqual(resolved_text, "summary text")

    def test_synthesize_uses_fish_provider(self):
        service = summary_audio_player.SummaryAudioPlayerService()

        with patch("app.services.summary_audio_player.synthesize_single_speaker_tts", return_value=(b"audio", "audio/mpeg", ".mp3", 5)) as synth:
            audio_base64, content_type, duration_sec, resolved_text = service.synthesize(
                provider="fish",
                voice_model="fish-model-1",
                voice_style="",
                tts_model="s2-pro",
                text="summary text",
                speech_rate=1.1,
                emotional_intensity=1.0,
                tempo_dynamics=1.0,
                line_break_silence_seconds=0.4,
                chunk_trailing_silence_seconds=1.25,
                pitch=0.0,
                volume_gain=0.4,
                user_dictionary_uuid=None,
                aivis_api_key=None,
                google_api_key=None,
                xai_api_key=None,
                openai_api_key=None,
                fish_api_key="fish-key",
            )

        synth.assert_called_once_with(
            "fish",
            endpoint="",
            api_key="fish-key",
            voice_id="fish-model-1",
            tts_model="s2-pro",
            text="summary text",
            speech_rate=1.1,
            timeout_sec=service.fish_timeout_sec,
            volume_gain=0.4,
        )
        self.assertEqual(audio_base64, "YXVkaW8=")
        self.assertEqual(content_type, "audio/mpeg")
        self.assertEqual(duration_sec, 5)
        self.assertEqual(resolved_text, "summary text")

    def test_synthesize_uses_gemini_provider(self):
        service = summary_audio_player.SummaryAudioPlayerService()

        with patch("app.services.summary_audio_player.synthesize_single_speaker_tts", return_value=(b"audio", "audio/mpeg", ".mp3", 5)) as synth:
            audio_base64, content_type, duration_sec, resolved_text = service.synthesize(
                provider="gemini_tts",
                voice_model="Kore",
                voice_style="",
                tts_model="gemini-2.5-flash-tts",
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
                google_api_key=None,
                xai_api_key=None,
                openai_api_key=None,
            )

        synth.assert_called_once_with(
            "gemini_tts",
            endpoint=service.gemini_tts_endpoint,
            api_key="",
            voice_id="Kore",
            tts_model="gemini-2.5-flash-tts",
            text="summary text",
            speech_rate=1.0,
            timeout_sec=service.gemini_timeout_sec,
            volume_gain=0.0,
        )
        self.assertEqual(audio_base64, "YXVkaW8=")
        self.assertEqual(content_type, "audio/mpeg")
        self.assertEqual(duration_sec, 5)
        self.assertEqual(resolved_text, "summary text")

    def test_synthesize_uses_elevenlabs_provider(self):
        service = summary_audio_player.SummaryAudioPlayerService()

        with patch("app.services.summary_audio_player.synthesize_single_speaker_tts", return_value=(b"audio", "audio/mpeg", ".mp3", 6)) as synth:
            audio_base64, content_type, duration_sec, resolved_text = service.synthesize(
                provider="elevenlabs",
                voice_model="voice-1",
                voice_style="",
                tts_model="eleven_multilingual_v2",
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
                google_api_key=None,
                xai_api_key=None,
                openai_api_key=None,
                elevenlabs_api_key="eleven-key",
            )

        synth.assert_called_once_with(
            "elevenlabs",
            endpoint=service.elevenlabs_tts_endpoint,
            api_key="eleven-key",
            voice_id="voice-1",
            tts_model="eleven_multilingual_v2",
            text="summary text",
            speech_rate=1.0,
            timeout_sec=service.elevenlabs_timeout_sec,
            volume_gain=0.0,
        )
        self.assertEqual(audio_base64, "YXVkaW8=")
        self.assertEqual(content_type, "audio/mpeg")
        self.assertEqual(duration_sec, 6)
        self.assertEqual(resolved_text, "summary text")

    def test_synthesize_dispatches_by_provider_key(self):
        service = summary_audio_player.SummaryAudioPlayerService()
        cases = [
            {
                "provider": "xai",
                "voice_model": "voice-1",
                "tts_model": "",
                "patch_target": "app.services.summary_audio_player.synthesize_single_speaker_tts",
                "patch_args": ("xai",),
                "call_kwargs": {"xai_api_key": "xai-key"},
            },
            {
                "provider": "openai",
                "voice_model": "alloy",
                "tts_model": "gpt-4o-mini-tts",
                "patch_target": "app.services.summary_audio_player.synthesize_single_speaker_tts",
                "patch_args": ("openai",),
                "call_kwargs": {"openai_api_key": "openai-key"},
            },
            {
                "provider": "gemini_tts",
                "voice_model": "Kore",
                "tts_model": "gemini-2.5-flash-preview-tts",
                "patch_target": "app.services.summary_audio_player.synthesize_single_speaker_tts",
                "patch_args": ("gemini_tts",),
                "call_kwargs": {"google_api_key": "google-key"},
            },
            {
                "provider": "fish",
                "voice_model": "fish-model-1",
                "tts_model": "s2-pro",
                "patch_target": "app.services.summary_audio_player.synthesize_single_speaker_tts",
                "patch_args": ("fish",),
                "call_kwargs": {"fish_api_key": "fish-key"},
            },
            {
                "provider": "elevenlabs",
                "voice_model": "voice-1",
                "tts_model": "eleven_multilingual_v2",
                "patch_target": "app.services.summary_audio_player.synthesize_single_speaker_tts",
                "patch_args": ("elevenlabs",),
                "call_kwargs": {"elevenlabs_api_key": "eleven-key"},
            },
        ]

        for case in cases:
            with self.subTest(provider=case["provider"]):
                with patch(case["patch_target"], return_value=(b"audio", "audio/mpeg", ".mp3", 5)) as synth:
                    kwargs = {
                        "provider": case["provider"],
                        "voice_model": case["voice_model"],
                        "voice_style": "",
                        "tts_model": case["tts_model"],
                        "text": "summary text",
                        "speech_rate": 1.0,
                        "emotional_intensity": 1.0,
                        "tempo_dynamics": 1.0,
                        "line_break_silence_seconds": 0.4,
                        "chunk_trailing_silence_seconds": 1.25,
                        "pitch": 0.0,
                        "volume_gain": 0.0,
                        "user_dictionary_uuid": None,
                        "aivis_api_key": None,
                        "google_api_key": None,
                        "xai_api_key": None,
                        "openai_api_key": None,
                        "fish_api_key": None,
                        "elevenlabs_api_key": None,
                    }
                    kwargs.update(case["call_kwargs"])
                    audio_base64, content_type, duration_sec, resolved_text = service.synthesize(**kwargs)

                synth.assert_called_once()
                if case["patch_args"]:
                    self.assertEqual(synth.call_args.args[0], case["patch_args"][0])
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
