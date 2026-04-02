import unittest
import subprocess
import os
from unittest.mock import Mock, patch

from app.services.youtube_extract_service import YouTubeTranscriptUnavailableError, extract_body, is_youtube_url


class YoutubeExtractServiceTests(unittest.TestCase):
    def test_is_youtube_url(self):
        self.assertTrue(is_youtube_url("https://www.youtube.com/watch?v=abc123"))
        self.assertTrue(is_youtube_url("https://youtu.be/abc123"))
        self.assertTrue(is_youtube_url("https://www.youtube.com/shorts/abc123"))
        self.assertFalse(is_youtube_url("https://example.com/watch?v=abc123"))

    def test_extract_body_prefers_manual_japanese_subtitles(self):
        metadata = {
            "title": "動画タイトル",
            "upload_date": "20260402",
            "thumbnail": "https://img.example/thumb.jpg",
            "subtitles": {
                "ja": [{"ext": "json3", "url": "https://subs.example/ja.json3"}],
                "en": [{"ext": "json3", "url": "https://subs.example/en.json3"}],
            },
            "automatic_captions": {},
        }
        proc = Mock(stdout='{"ignored": true}')
        response = Mock()
        response.raise_for_status.return_value = None
        response.text = '{"events":[{"segs":[{"utf8":"日本語字幕です。"}]}]}'

        with patch("app.services.youtube_extract_service.subprocess.run", return_value=proc), patch(
            "app.services.youtube_extract_service.json.loads", side_effect=[metadata, {"events": [{"segs": [{"utf8": "日本語字幕です。"}]}]}]
        ), patch("app.services.youtube_extract_service.httpx.get", return_value=response) as mocked_get:
            result = extract_body("https://www.youtube.com/watch?v=abc123")

        self.assertEqual(result["title"], "動画タイトル")
        self.assertEqual(result["content"], "日本語字幕です。")
        self.assertEqual(result["published_at"], "2026-04-02")
        mocked_get.assert_called_once_with("https://subs.example/ja.json3", timeout=30.0, follow_redirects=True)

    def test_extract_body_falls_back_to_english_auto_captions(self):
        metadata = {
            "title": "Video Title",
            "subtitles": {},
            "automatic_captions": {
                "en-US": [{"ext": "vtt", "url": "https://subs.example/en.vtt"}],
            },
        }
        proc = Mock(stdout='{"ignored": true}')
        response = Mock()
        response.raise_for_status.return_value = None
        response.text = "WEBVTT\n\n00:00:00.000 --> 00:00:01.000\nEnglish line"

        with patch("app.services.youtube_extract_service.subprocess.run", return_value=proc), patch(
            "app.services.youtube_extract_service.json.loads", return_value=metadata
        ), patch("app.services.youtube_extract_service.httpx.get", return_value=response):
            result = extract_body("https://youtu.be/abc123")

        self.assertEqual(result["content"], "English line")

    def test_extract_body_parses_srv3_auto_captions(self):
        metadata = {
            "title": "Video Title",
            "subtitles": {},
            "automatic_captions": {
                "ja": [{"ext": "srv3", "url": "https://subs.example/ja.srv3"}],
            },
        }
        proc = Mock(stdout='{"ignored": true}')
        response = Mock()
        response.raise_for_status.return_value = None
        response.text = """<?xml version="1.0" encoding="utf-8" ?><timedtext><body><p t="0" d="1000"><s>字幕</s><s>あります</s></p></body></timedtext>"""

        with patch("app.services.youtube_extract_service.subprocess.run", return_value=proc), patch(
            "app.services.youtube_extract_service.json.loads", return_value=metadata
        ), patch("app.services.youtube_extract_service.httpx.get", return_value=response):
            result = extract_body("https://youtu.be/abc123")

        self.assertEqual(result["content"], "字幕あります")

    def test_extract_body_raises_when_transcript_unavailable(self):
        metadata = {
            "title": "Video Title",
            "subtitles": {"fr": [{"ext": "vtt", "url": "https://subs.example/fr.vtt"}]},
            "automatic_captions": {"en-US": [{"ext": "srv3", "url": "https://subs.example/en.srv3"}]},
        }
        proc = Mock(stdout='{"ignored": true}')
        response = Mock()
        response.raise_for_status.return_value = None
        response.text = ""

        with patch("app.services.youtube_extract_service.subprocess.run", return_value=proc), patch(
            "app.services.youtube_extract_service.json.loads", return_value=metadata
        ), patch("app.services.youtube_extract_service.httpx.get", return_value=response):
            with self.assertRaisesRegex(
                YouTubeTranscriptUnavailableError,
                r"youtube transcript unavailable: .*manual_langs=\['fr'\].*auto_langs=\['en-US'\].*auto_exts=\['srv3'\]",
            ) as ctx:
                extract_body("https://www.youtube.com/watch?v=abc123")
        self.assertEqual(ctx.exception.title, "Video Title")

    def test_extract_body_includes_ytdlp_stderr_on_metadata_failure(self):
        err = subprocess.CalledProcessError(
            1,
            ["yt-dlp", "--dump-single-json"],
            stderr="ERROR: Sign in to confirm you’re not a bot. This helps protect our community.",
        )

        with patch("app.services.youtube_extract_service.subprocess.run", side_effect=err):
            with self.assertRaisesRegex(
                RuntimeError,
                "yt-dlp metadata fetch failed: cookies_present=False extractor_args_present=False ERROR: Sign in to confirm you’re not a bot",
            ):
                extract_body("https://www.youtube.com/watch?v=abc123")

    def test_extract_body_passes_temp_cookie_file_when_env_is_set(self):
        metadata = {
            "title": "Video Title",
            "subtitles": {},
            "automatic_captions": {"en": [{"ext": "vtt", "url": "https://subs.example/en.vtt"}]},
        }
        response = Mock()
        response.raise_for_status.return_value = None
        response.text = "WEBVTT\n\n00:00:00.000 --> 00:00:01.000\nEnglish line"
        captured = {}

        def fake_run(cmd, **kwargs):
            captured["cmd"] = cmd
            cookies_path = cmd[cmd.index("--cookies") + 1]
            captured["cookies_path"] = cookies_path
            with open(cookies_path, "r", encoding="utf-8") as f:
                captured["cookies_content"] = f.read()
            return Mock(stdout='{"ignored": true}')

        with patch.dict(
            os.environ,
            {"YTDLP_COOKIES_B64": "I0hUVFAgQ29va2llIEZpbGUKLnlvdXR1YmUuY29tCVRSVUUJLwlGQUxTRQkwCVNJRAl2YWx1ZQo="},
            clear=False,
        ), patch("app.services.youtube_extract_service.subprocess.run", side_effect=fake_run), patch(
            "app.services.youtube_extract_service.json.loads", return_value=metadata
        ), patch("app.services.youtube_extract_service.httpx.get", return_value=response):
            result = extract_body("https://youtu.be/abc123")

        self.assertEqual(result["content"], "English line")
        self.assertIn("--cookies", captured["cmd"])
        self.assertIn("--ignore-no-formats-error", captured["cmd"])
        self.assertIn("#HTTP Cookie File", captured["cookies_content"])
        self.assertFalse(os.path.exists(captured["cookies_path"]))

    def test_extract_body_passes_extractor_args_when_env_is_set(self):
        metadata = {
            "title": "Video Title",
            "subtitles": {},
            "automatic_captions": {"en": [{"ext": "vtt", "url": "https://subs.example/en.vtt"}]},
        }
        response = Mock()
        response.raise_for_status.return_value = None
        response.text = "WEBVTT\n\n00:00:00.000 --> 00:00:01.000\nEnglish line"
        captured = {}

        def fake_run(cmd, **kwargs):
            captured["cmd"] = cmd
            return Mock(stdout='{"ignored": true}')

        with patch.dict(
            os.environ,
            {"YTDLP_EXTRACTOR_ARGS": "youtube:player_client=mweb,android"},
            clear=False,
        ), patch("app.services.youtube_extract_service.subprocess.run", side_effect=fake_run), patch(
            "app.services.youtube_extract_service.json.loads", return_value=metadata
        ), patch("app.services.youtube_extract_service.httpx.get", return_value=response):
            result = extract_body("https://youtu.be/abc123")

        self.assertEqual(result["content"], "English line")
        self.assertIn("--extractor-args", captured["cmd"])
        self.assertIn("youtube:player_client=mweb,android", captured["cmd"])
