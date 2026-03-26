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
