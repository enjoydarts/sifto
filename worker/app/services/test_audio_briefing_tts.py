import unittest
from unittest.mock import patch

import httpx

import app.services.audio_briefing_tts as audio_briefing_tts
from app.services.audio_briefing_tts import AivisRateLimiter, AivisRedisRateLimiter, AudioBriefingTTSService


class AivisRateLimiterTests(unittest.TestCase):
    def test_acquire_waits_to_keep_requests_under_seven_per_minute(self):
        limiter = AivisRateLimiter(min_interval_sec=8.6)

        with (
            patch("app.services.audio_briefing_tts.time.monotonic", side_effect=[100.0, 103.0]),
            patch("app.services.audio_briefing_tts.time.sleep") as sleep_mock,
        ):
            limiter.acquire("shared-key")
            limiter.acquire("shared-key")

        sleep_mock.assert_called_once()
        slept = sleep_mock.call_args.args[0]
        self.assertAlmostEqual(slept, 5.6, places=1)

    def test_redis_acquire_waits_until_shared_slot_is_available(self):
        class FakeRedis:
            def __init__(self):
                self.set_calls = 0

            def set(self, key, value, nx=False, px=None):
                self.set_calls += 1
                return self.set_calls != 2

            def pttl(self, key):
                return 5600

        limiter = AivisRedisRateLimiter(
            redis_client=FakeRedis(),
            min_interval_sec=8.6,
            fallback=AivisRateLimiter(min_interval_sec=8.6),
        )

        with patch("app.services.audio_briefing_tts.time.sleep") as sleep_mock:
            limiter.acquire("shared-key")
            limiter.acquire("shared-key")

        sleep_mock.assert_called_once()
        slept = sleep_mock.call_args.args[0]
        self.assertAlmostEqual(slept, 5.6, places=1)


class AudioBriefingTTSServiceTests(unittest.TestCase):
    def test_presign_audio_url_uses_r2_client(self):
        service = AudioBriefingTTSService()
        service.r2_bucket = "briefings"

        class FakeClient:
            def generate_presigned_url(self, method, Params, ExpiresIn):
                assert method == "get_object"
                assert Params == {"Bucket": "briefings", "Key": "audio-briefings/job/episode.mp3"}
                assert ExpiresIn == 900
                return "https://signed.example.com/audio.mp3"

        with patch.object(service, "r2_client", return_value=FakeClient()):
            got = service.presign_audio_url("audio-briefings/job/episode.mp3", expires_sec=900)

        self.assertEqual(got, "https://signed.example.com/audio.mp3")

    def test_synthesize_aivis_audio_uses_exponential_backoff_after_429(self):
        service = AudioBriefingTTSService()
        service.aivis_tts_endpoint = "https://example.test/v1/tts/synthesize"
        service.aivis_retry_attempts = 3
        service.aivis_retry_fallback_sec = 9.0

        class FakeResponse:
            def __init__(self, status_code: int, content: bytes = b"", headers: dict | None = None):
                self.status_code = status_code
                self.content = content
                self.headers = headers or {}
                self.text = "rate limited" if status_code == 429 else "ok"

            def raise_for_status(self):
                if self.status_code >= 400:
                    request = httpx.Request("POST", "https://example.test/v1/tts/synthesize")
                    response = httpx.Response(self.status_code, request=request, headers=self.headers, text=self.text)
                    raise httpx.HTTPStatusError("boom", request=request, response=response)

        class FakeClient:
            def __init__(self):
                self.calls = 0

            def __enter__(self):
                return self

            def __exit__(self, exc_type, exc, tb):
                return False

            def post(self, url, json, headers):
                self.calls += 1
                if self.calls < 3:
                    return FakeResponse(429)
                return FakeResponse(200, content=b"mp3-bytes")

        fake_client = FakeClient()

        with (
            patch("app.services.audio_briefing_tts.httpx.Client", return_value=fake_client),
            patch("app.services.audio_briefing_tts.probe_duration_seconds", return_value=12),
            patch.object(audio_briefing_tts.AIVIS_RATE_LIMITER, "acquire") as acquire_mock,
            patch("app.services.audio_briefing_tts.time.sleep") as sleep_mock,
        ):
            audio_bytes, content_type, suffix, duration_sec = service.synthesize_aivis_audio(
                voice_model="model-uuid",
                voice_style="speaker-uuid:0",
                text="hello",
                speech_rate=1.0,
                emotional_intensity=1.0,
                tempo_dynamics=1.0,
                line_break_silence_seconds=0.4,
                pitch=0.0,
                volume_gain=0.0,
                api_key_override="key",
            )

        self.assertEqual(audio_bytes, b"mp3-bytes")
        self.assertEqual(content_type, "audio/mpeg")
        self.assertEqual(suffix, ".mp3")
        self.assertEqual(duration_sec, 12)
        self.assertEqual(fake_client.calls, 3)
        self.assertEqual(acquire_mock.call_count, 3)
        self.assertEqual([call.args[0] for call in sleep_mock.call_args_list], [9.0, 18.0])


if __name__ == "__main__":
    unittest.main()
