import unittest

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
