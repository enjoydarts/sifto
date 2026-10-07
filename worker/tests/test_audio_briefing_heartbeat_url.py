import pytest

from app.services.audio_briefing_tts import AudioBriefingHeartbeatLoop


HEARTBEAT_PATH = "/api/internal/audio-briefings/chunks/00000000-0000-4000-8000-000000000001/heartbeat"
BASE_ENV = {
    "AUDIO_BRIEFING_HEARTBEAT_BASE_URL": "",
    "AUDIO_BRIEFING_CONCAT_MODE": "",
    "AUDIO_BRIEFING_LOCAL_CALLBACK_BASE_URL": "http://api:8080",
    "APP_BASE_URL": "https://api.example.com",
    "NEXT_PUBLIC_APP_URL": "",
}


@pytest.mark.parametrize(
    "overrides,base",
    [
        ({}, "https://api.example.com"),
        ({"AUDIO_BRIEFING_CONCAT_MODE": "cloud_run"}, "https://api.example.com"),
        ({"AUDIO_BRIEFING_CONCAT_MODE": "local"}, "http://api:8080"),
        ({"AUDIO_BRIEFING_CONCAT_MODE": "local", "AUDIO_BRIEFING_LOCAL_CALLBACK_BASE_URL": ""}, "http://api:8080"),
        ({"AUDIO_BRIEFING_CONCAT_MODE": " LOCAL ", "AUDIO_BRIEFING_LOCAL_CALLBACK_BASE_URL": "https://internal.example.com/base/"}, "https://internal.example.com/base"),
        ({"APP_BASE_URL": "", "NEXT_PUBLIC_APP_URL": "https://fallback.example.com"}, "https://fallback.example.com"),
        ({"AUDIO_BRIEFING_HEARTBEAT_BASE_URL": " https://heartbeat.example.com/base/ "}, "https://heartbeat.example.com/base"),
        ({"AUDIO_BRIEFING_HEARTBEAT_BASE_URL": "https://heartbeat.example.com", "AUDIO_BRIEFING_CONCAT_MODE": "local"}, "https://heartbeat.example.com"),
    ],
)
def test_heartbeat_accepts_api_callback_base(monkeypatch, overrides, base):
    for name, value in (BASE_ENV | overrides).items():
        monkeypatch.setenv(name, value)
    loop = AudioBriefingHeartbeatLoop(base + HEARTBEAT_PATH, "token", 20, 10)
    assert loop.enabled()


@pytest.mark.parametrize(
    "url",
    [
        "http://api:8080" + HEARTBEAT_PATH,
        "http://169.254.169.254" + HEARTBEAT_PATH,
        "https://attacker.example.com" + HEARTBEAT_PATH,
        "https://api.example.com/admin",
        "https://api.example.com" + HEARTBEAT_PATH + "?redirect=1",
        "https://api.example.com" + HEARTBEAT_PATH + "#fragment",
        "https://user:password@api.example.com" + HEARTBEAT_PATH,
    ],
)
def test_cloud_heartbeat_rejects_untrusted_targets(monkeypatch, url):
    for name, value in BASE_ENV.items():
        monkeypatch.setenv(name, value)
    with pytest.raises(ValueError, match="invalid heartbeat callback URL"):
        AudioBriefingHeartbeatLoop(url, "token", 20, 10)
