import unittest
from unittest.mock import patch

import httpx

from app.services.azure_speech_tts import _build_single_voice_ssml
from app.services.azure_speech_tts import synthesize_azure_speech_duo_tts
from app.services.azure_speech_tts import synthesize_azure_speech_tts


class AzureSpeechTTSServiceTests(unittest.TestCase):
    def test_build_single_voice_ssml_strips_invalid_xml_chars_and_sets_voice_lang(self):
        ssml = _build_single_voice_ssml(
            voice_name="ja-JP-AoiNeural",
            text="冒頭\u0000です\n\n次の段落です",
            speech_rate=1.0,
            line_break_silence_seconds=0.4,
            pitch=0.0,
            volume_gain=0.0,
        )

        self.assertIn('<voice xml:lang="ja-JP" name="ja-JP-AoiNeural">', ssml)
        self.assertIn("冒頭です", ssml)
        self.assertNotIn("\u0000", ssml)

    def test_synthesize_azure_speech_tts_reports_request_id_when_body_is_empty(self):
        def fake_post(url, headers=None, content=None, timeout=None):
            request = httpx.Request("POST", url, headers=headers, content=content)
            return httpx.Response(
                400,
                request=request,
                headers={"X-RequestId": "req-123"},
                text="",
            )

        with patch("app.services.azure_speech_tts.httpx.post", side_effect=fake_post):
            with self.assertRaisesRegex(
                RuntimeError,
                r"azure speech request failed: status=400 body=<empty> request_id=req-123",
            ):
                synthesize_azure_speech_tts(
                    region="japaneast",
                    api_key="azure-key",
                    voice_name="ja-JP-AoiNeural",
                    text="テストです",
                    speech_rate=1.0,
                    line_break_silence_seconds=0.4,
                    pitch=0.0,
                    volume_gain=0.0,
                    timeout_sec=10.0,
                )

    def test_build_single_voice_ssml_omits_prosody_when_all_controls_are_neutral(self):
        ssml = _build_single_voice_ssml(
            voice_name="ja-JP-AoiNeural",
            text="テストです",
            speech_rate=1.0,
            line_break_silence_seconds=0.4,
            pitch=0.0,
            volume_gain=0.0,
        )

        self.assertNotIn("<prosody", ssml)
        self.assertNotIn('volume="0dB"', ssml)

    def test_synthesize_azure_speech_tts_accepts_valid_preprocessed_ssml(self):
        captured: dict[str, object] = {}

        def fake_post(url, headers=None, content=None, timeout=None):
            captured["content"] = content.decode("utf-8")
            request = httpx.Request("POST", url, headers=headers, content=content)
            return httpx.Response(200, request=request, content=b"audio")

        with patch("app.services.azure_speech_tts.httpx.post", side_effect=fake_post):
            synthesize_azure_speech_tts(
                region="japaneast",
                api_key="azure-key",
                voice_name="ja-JP-AoiNeural",
                text='<speak version="1.0" xml:lang="ja-JP" xmlns="http://www.w3.org/2001/10/synthesis"><voice xml:lang="ja-JP" name="ja-JP-AoiNeural"><break strength="medium"/>テストです</voice></speak>',
                speech_rate=1.0,
                line_break_silence_seconds=0.4,
                pitch=0.0,
                volume_gain=0.0,
                timeout_sec=10.0,
            )

        self.assertIn('name="ja-JP-AoiNeural"', str(captured["content"]))

    def test_synthesize_azure_speech_duo_tts_rejects_unapproved_voice_name(self):
        with self.assertRaisesRegex(RuntimeError, r"voice.name is not allowed"):
            synthesize_azure_speech_duo_tts(
                region="japaneast",
                api_key="azure-key",
                host_voice_name="ja-JP-AoiNeural",
                partner_voice_name="ja-JP-NanamiNeural",
                turns=[],
                preprocessed_text='<speak version="1.0" xml:lang="ja-JP" xmlns="http://www.w3.org/2001/10/synthesis"><voice xml:lang="ja-JP" name="ja-JP-KeitaNeural">テストです</voice></speak>',
                speech_rate=1.0,
                line_break_silence_seconds=0.4,
                pitch=0.0,
                volume_gain=0.0,
                timeout_sec=10.0,
            )
