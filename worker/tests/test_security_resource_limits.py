import subprocess
from unittest.mock import patch, Mock, MagicMock

import pytest

from app.services.audio_briefing_tts import AudioBriefingTTSService
from app.services.audio_briefing_tts import AudioBriefingHeartbeatLoop
from app.services import pdf_service, youtube_extract_service, claude_service


def test_r2_bucket_override_is_limited_to_configured_audio_buckets():
    service = AudioBriefingTTSService.__new__(AudioBriefingTTSService)
    service.r2_bucket = "standard"
    service.r2_standard_bucket = "standard"
    service.r2_ia_bucket = "archive"
    assert service.resolve_bucket() == "standard"
    assert service.resolve_bucket("archive") == "archive"
    with pytest.raises(ValueError):
        service.resolve_bucket("unrelated-private-bucket")


def test_pdf_extraction_rejects_too_many_pages_before_parsing_text():
    document = MagicMock()
    document.__enter__ = Mock(return_value=document)
    document.__exit__ = Mock(return_value=False)
    document.page_count = 1001
    with patch("fitz.open", return_value=document), pytest.raises(ValueError):
        pdf_service._extract_pdf_body_in_process(b"%PDF-test", "https://example.com/test.pdf")
    document.__iter__.assert_not_called()


def test_ytdlp_process_is_bounded_and_disables_playlists():
    def run(cmd, **kwargs):
        assert kwargs.get("timeout", 9999) <= 45
        assert "--no-playlist" in cmd
        assert "--ignore-config" in cmd
        return subprocess.CompletedProcess(cmd, 0, stdout='{"title": "Public video"}')
    with patch.object(youtube_extract_service.subprocess, "run", side_effect=run):
        assert youtube_extract_service._load_video_metadata("https://youtube.com/watch?v=123")["title"] == "Public video"


def test_anthropic_dated_catalog_models_keep_catalog_pricing():
    with patch.object(claude_service, "model_pricing", side_effect=lambda name: {"input_per_mtok_usd": 3} if name == "claude-sonnet-5" else None):
        assert claude_service._normalize_model_family("claude-sonnet-5-20260701") == "claude-sonnet-5"

def test_heartbeat_rejects_arbitrary_hosts_and_paths(monkeypatch):
    monkeypatch.setenv("AUDIO_BRIEFING_HEARTBEAT_BASE_URL", "https://api.example.com")
    for url in ("http://169.254.169.254/latest", "https://attacker.example/", "https://api.example.com/admin", "https://user:secret@api.example.com/api/internal/audio-briefings/chunks/00000000-0000-4000-8000-000000000001/heartbeat"):
        with pytest.raises(ValueError):
            AudioBriefingHeartbeatLoop(url, "token", 10, 5)
