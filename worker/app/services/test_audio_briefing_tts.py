import unittest
from unittest.mock import patch

import httpx

import app.services.aivis_speech as aivis_speech
import app.services.audio_briefing_tts as audio_briefing_tts
from app.services.audio_briefing_tts import (
    AivisRateLimiter,
    AivisRedisRateLimiter,
    AudioBriefingHeartbeatLoop,
    AudioBriefingTTSService,
)


class AivisRateLimiterTests(unittest.TestCase):
    def test_acquire_waits_to_keep_requests_under_seven_per_minute(self):
        limiter = AivisRateLimiter(min_interval_sec=8.6)

        with (
            patch("app.services.aivis_speech.time.monotonic", side_effect=[100.0, 103.0]),
            patch("app.services.aivis_speech.time.sleep") as sleep_mock,
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

        with patch("app.services.aivis_speech.time.sleep") as sleep_mock:
            limiter.acquire("shared-key")
            limiter.acquire("shared-key")

        sleep_mock.assert_called_once()
        slept = sleep_mock.call_args.args[0]
        self.assertAlmostEqual(slept, 5.6, places=1)

    def test_redis_acquire_falls_back_after_max_wait(self):
        class FakeRedis:
            def set(self, key, value, nx=False, px=None):
                return False

            def pttl(self, key):
                return 200000

        fallback = AivisRateLimiter(min_interval_sec=8.6)
        limiter = AivisRedisRateLimiter(
            redis_client=FakeRedis(),
            min_interval_sec=8.6,
            fallback=fallback,
            max_wait_sec=5.0,
        )

        with (
            patch("app.services.aivis_speech.time.monotonic", side_effect=[100.0, 100.0, 106.0]),
            patch("app.services.aivis_speech.time.sleep") as sleep_mock,
            patch.object(fallback, "acquire") as fallback_acquire_mock,
        ):
            limiter.acquire("shared-key")

        self.assertGreaterEqual(sleep_mock.call_count, 1)
        fallback_acquire_mock.assert_called_once_with("shared-key")

    def test_redis_execution_gate_waits_until_shared_request_finishes(self):
        class FakeRedis:
            def __init__(self):
                self.set_calls = 0
                self.values = {}

            def set(self, key, value, nx=False, px=None):
                self.set_calls += 1
                if self.set_calls == 1:
                    self.values[key] = value
                    return True
                if self.set_calls == 2:
                    return False
                self.values[key] = value
                return True

            def pttl(self, key):
                return 700

            def get(self, key):
                return self.values.get(key)

            def delete(self, key):
                self.values.pop(key, None)

        fallback = aivis_speech.AivisProcessExecutionGate()
        gate = aivis_speech.AivisRedisExecutionGate(
            redis_client=FakeRedis(),
            lease_sec=30.0,
            fallback=fallback,
            poll_sec=0.1,
        )

        with patch("app.services.aivis_speech.time.sleep") as sleep_mock:
            first = gate.acquire("shared-key")
            gate.release("shared-key", first)
            second = gate.acquire("shared-key")

        self.assertIsNotNone(first)
        self.assertIsNotNone(second)
        sleep_mock.assert_called_once()
        self.assertAlmostEqual(sleep_mock.call_args.args[0], 0.7, places=1)


class AudioBriefingTTSServiceTests(unittest.TestCase):
    def test_heartbeat_loop_posts_bearer_token(self):
        class FakeResponse:
            def raise_for_status(self):
                return None

        class FakeClient:
            def __init__(self):
                self.calls = []

            def __enter__(self):
                return self

            def __exit__(self, exc_type, exc, tb):
                return False

            def post(self, url, headers):
                self.calls.append((url, headers))
                return FakeResponse()

        fake_client = FakeClient()
        loop = AudioBriefingHeartbeatLoop(
            heartbeat_url="https://api.example.com/api/internal/audio-briefings/chunks/chunk-1/heartbeat",
            heartbeat_token="heartbeat-token",
            interval_sec=20.0,
            timeout_sec=10.0,
        )

        with patch("app.services.audio_briefing_tts.httpx.Client", return_value=fake_client):
            loop._send_once()

        self.assertEqual(
            fake_client.calls,
            [
                (
                    "https://api.example.com/api/internal/audio-briefings/chunks/chunk-1/heartbeat",
                    {"Authorization": "Bearer heartbeat-token"},
                )
            ],
        )

    def test_build_aivis_payload_includes_user_dictionary_uuid_and_trailing_silence(self):
        payload = audio_briefing_tts.build_aivis_payload(
            voice_model="model-uuid",
            voice_style="speaker-uuid:1",
            text="hello",
            speech_rate=1.0,
            emotional_intensity=1.0,
            tempo_dynamics=1.0,
            line_break_silence_seconds=0.4,
            chunk_trailing_silence_seconds=1.0,
            pitch=0.0,
            volume_gain=0.0,
            user_dictionary_uuid="5b6f7aa3-2c34-4ad7-aad0-4e1d683d7861",
        )

        self.assertEqual(payload["user_dictionary_uuid"], "5b6f7aa3-2c34-4ad7-aad0-4e1d683d7861")
        self.assertEqual(payload["trailing_silence_seconds"], 1.0)

    def test_build_aivis_payload_wraps_and_escapes_plain_text_for_ssml(self):
        payload = audio_briefing_tts.build_aivis_payload(
            voice_model="model-uuid",
            voice_style="speaker-uuid:1",
            text="A & B < C > D",
            speech_rate=1.0,
            emotional_intensity=1.0,
            tempo_dynamics=1.0,
            line_break_silence_seconds=0.4,
            chunk_trailing_silence_seconds=1.0,
            pitch=0.0,
            volume_gain=0.0,
        )

        self.assertEqual(payload["text"], "<speak>A &amp; B &lt; C &gt; D</speak>")
        self.assertTrue(payload["use_ssml"])

    def test_init_reads_aivis_timeout_from_env(self):
        with patch.dict("os.environ", {"AIVIS_TTS_TIMEOUT_SEC": "420"}, clear=False):
            service = AudioBriefingTTSService()

        self.assertEqual(service.aivis_timeout_sec, 420.0)

    def test_resolve_bucket_prefers_explicit_bucket_override(self):
        service = AudioBriefingTTSService()
        service.r2_bucket = "briefings-standard"

        self.assertEqual(service.resolve_bucket("briefings-ia"), "briefings-ia")

    def test_presign_audio_url_uses_bucket_override(self):
        service = AudioBriefingTTSService()
        service.r2_bucket = "briefings-standard"

        class FakeClient:
            def generate_presigned_url(self, method, Params, ExpiresIn):
                assert method == "get_object"
                assert Params == {"Bucket": "briefings-ia", "Key": "audio-briefings/job/episode.mp3"}
                assert ExpiresIn == 900
                return "https://signed.example.com/audio.mp3"

        with patch.object(service, "r2_client", return_value=FakeClient()):
            got = service.presign_audio_url("audio-briefings/job/episode.mp3", expires_sec=900, bucket_override="briefings-ia")

        self.assertEqual(got, "https://signed.example.com/audio.mp3")

    def test_delete_objects_uses_bucket_override(self):
        service = AudioBriefingTTSService()
        service.r2_bucket = "briefings-standard"

        class FakeClient:
            def __init__(self):
                self.calls = []

            def delete_objects(self, Bucket, Delete):
                self.calls.append((Bucket, Delete))
                return {"Deleted": Delete["Objects"]}

        fake_client = FakeClient()

        with patch.object(service, "r2_client", return_value=fake_client):
            deleted = service.delete_objects(["one.mp3", "two.mp3"], bucket_override="briefings-ia")

        self.assertEqual(deleted, 2)
        self.assertEqual(
            fake_client.calls,
            [("briefings-ia", {"Objects": [{"Key": "one.mp3"}, {"Key": "two.mp3"}], "Quiet": True})],
        )

    def test_delete_objects_batches_non_empty_keys(self):
        service = AudioBriefingTTSService()
        service.r2_bucket = "briefings"

        class FakeClient:
            def __init__(self):
                self.calls = []

            def delete_objects(self, Bucket, Delete):
                self.calls.append((Bucket, Delete))
                return {"Deleted": Delete["Objects"]}

        fake_client = FakeClient()

        with patch.object(service, "r2_client", return_value=fake_client):
            deleted = service.delete_objects(["one.mp3", "", "two.mp3", "one.mp3"])

        self.assertEqual(deleted, 2)
        self.assertEqual(
            fake_client.calls,
            [("briefings", {"Objects": [{"Key": "one.mp3"}, {"Key": "two.mp3"}], "Quiet": True})],
        )

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

    def test_copy_objects_copies_each_key_between_buckets(self):
        service = AudioBriefingTTSService()

        class FakeClient:
            def __init__(self):
                self.calls = []

            def copy_object(self, Bucket, Key, CopySource):
                self.calls.append((Bucket, Key, CopySource))

        fake_client = FakeClient()

        with patch.object(service, "r2_client", return_value=fake_client):
            copied = service.copy_objects(
                source_bucket="briefings-standard",
                target_bucket="briefings-ia",
                object_keys=["one.mp3", "", "two.mp3", "one.mp3"],
            )

        self.assertEqual(copied, 2)
        self.assertEqual(
            fake_client.calls,
            [
                ("briefings-ia", "one.mp3", {"Bucket": "briefings-standard", "Key": "one.mp3"}),
                ("briefings-ia", "two.mp3", {"Bucket": "briefings-standard", "Key": "two.mp3"}),
            ],
        )

    def test_synthesize_and_upload_uses_xai_provider(self):
        service = AudioBriefingTTSService()

        with patch.object(service, "synthesize_xai_audio", return_value=(b"mp3", "audio/mpeg", ".mp3", 12)) as synth:
            with patch.object(service, "upload_bytes") as upload:
                object_key, duration_sec = service.synthesize_and_upload(
                    provider="xai",
                    voice_model="voice-1",
                    voice_style="",
                    tts_model="",
                    text="hello",
                    speech_rate=1.0,
                    emotional_intensity=1.0,
                    tempo_dynamics=1.0,
                    line_break_silence_seconds=0.0,
                    chunk_trailing_silence_seconds=0.0,
                    pitch=0.0,
                    volume_gain=0.0,
                    output_object_key="audio/test",
                )

        synth.assert_called_once()
        upload.assert_called_once()
        self.assertEqual(duration_sec, 12)
        self.assertTrue(object_key.endswith(".mp3"))

    def test_synthesize_and_upload_uses_openai_provider(self):
        service = AudioBriefingTTSService()

        with patch.object(service, "synthesize_openai_audio", return_value=(b"mp3", "audio/mpeg", ".mp3", 12)) as synth:
            with patch.object(service, "upload_bytes") as upload:
                object_key, duration_sec = service.synthesize_and_upload(
                    provider="openai",
                    voice_model="alloy",
                    voice_style="",
                    tts_model="gpt-4o-mini-tts",
                    text="hello",
                    speech_rate=1.0,
                    emotional_intensity=1.0,
                    tempo_dynamics=1.0,
                    line_break_silence_seconds=0.0,
                    chunk_trailing_silence_seconds=0.0,
                    pitch=0.0,
                    volume_gain=0.0,
                    output_object_key="audio/test",
                    openai_api_key="openai-key",
                )

        synth.assert_called_once()
        upload.assert_called_once()
        self.assertEqual(duration_sec, 12)
        self.assertTrue(object_key.endswith(".mp3"))

    def test_synthesize_openai_audio_uses_current_openai_payload_shape(self):
        captured: dict[str, object] = {}
        service = AudioBriefingTTSService()

        def fake_post(url, headers=None, json=None, timeout=None):
            captured["url"] = url
            captured["headers"] = headers
            captured["json"] = json
            captured["timeout"] = timeout
            request = httpx.Request("POST", url)
            return httpx.Response(200, content=b"audio", request=request)

        with patch("app.services.openai_tts.httpx.post", side_effect=fake_post):
            audio_bytes, content_type, suffix, duration_sec = service.synthesize_openai_audio(
                voice_id="alloy",
                tts_model="gpt-4o-mini-tts",
                text="summary text",
                speech_rate=1.0,
                api_key_override="openai-key",
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
            patch("app.services.aivis_speech.httpx.Client", return_value=fake_client),
            patch("app.services.aivis_speech.probe_duration_seconds", return_value=12),
            patch.object(aivis_speech.AIVIS_RATE_LIMITER, "acquire") as acquire_mock,
            patch.object(aivis_speech.AIVIS_SYNTHESIS_GATE, "acquire", side_effect=["lock-1", "lock-2", "lock-3"]) as gate_acquire_mock,
            patch.object(aivis_speech.AIVIS_SYNTHESIS_GATE, "release") as gate_release_mock,
            patch("app.services.aivis_speech.time.sleep") as sleep_mock,
        ):
            audio_bytes, content_type, suffix, duration_sec = service.synthesize_aivis_audio(
                voice_model="model-uuid",
                voice_style="speaker-uuid:0",
                text="hello",
                speech_rate=1.0,
                emotional_intensity=1.0,
                tempo_dynamics=1.0,
                line_break_silence_seconds=0.4,
                chunk_trailing_silence_seconds=1.0,
                pitch=0.0,
                volume_gain=0.0,
                user_dictionary_uuid=None,
                api_key_override="key",
            )

        self.assertEqual(audio_bytes, b"mp3-bytes")
        self.assertEqual(content_type, "audio/mpeg")
        self.assertEqual(suffix, ".mp3")
        self.assertEqual(duration_sec, 12)
        self.assertEqual(fake_client.calls, 3)
        self.assertEqual(acquire_mock.call_count, 3)
        self.assertEqual(gate_acquire_mock.call_count, 3)
        self.assertEqual(gate_release_mock.call_args_list[0].args, ("key", "lock-1"))
        self.assertEqual(gate_release_mock.call_args_list[1].args, ("key", "lock-2"))
        self.assertEqual(gate_release_mock.call_args_list[2].args, ("key", "lock-3"))
        self.assertEqual([call.args[0] for call in sleep_mock.call_args_list], [9.0, 18.0])

    def test_synthesize_aivis_audio_retries_after_504(self):
        service = AudioBriefingTTSService()
        service.aivis_tts_endpoint = "https://example.test/v1/tts/synthesize"
        service.aivis_retry_attempts = 2
        service.aivis_retry_fallback_sec = 9.0

        class FakeResponse:
            def __init__(self, status_code: int, content: bytes = b"", headers: dict | None = None):
                self.status_code = status_code
                self.content = content
                self.headers = headers or {}
                self.text = "gateway timeout" if status_code == 504 else "ok"

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
                if self.calls == 1:
                    return FakeResponse(504)
                return FakeResponse(200, content=b"mp3-bytes")

        fake_client = FakeClient()

        with (
            patch("app.services.aivis_speech.httpx.Client", return_value=fake_client),
            patch("app.services.aivis_speech.probe_duration_seconds", return_value=12),
            patch.object(aivis_speech.AIVIS_RATE_LIMITER, "acquire"),
            patch.object(aivis_speech.AIVIS_SYNTHESIS_GATE, "acquire", side_effect=["lock-1", "lock-2"]),
            patch.object(aivis_speech.AIVIS_SYNTHESIS_GATE, "release"),
            patch("app.services.aivis_speech.time.sleep") as sleep_mock,
        ):
            audio_bytes, content_type, suffix, duration_sec = service.synthesize_aivis_audio(
                voice_model="model-uuid",
                voice_style="speaker-uuid:0",
                text="hello",
                speech_rate=1.0,
                emotional_intensity=1.0,
                tempo_dynamics=1.0,
                line_break_silence_seconds=0.4,
                chunk_trailing_silence_seconds=1.0,
                pitch=0.0,
                volume_gain=0.0,
                user_dictionary_uuid=None,
                api_key_override="key",
            )

        self.assertEqual(audio_bytes, b"mp3-bytes")
        self.assertEqual(content_type, "audio/mpeg")
        self.assertEqual(suffix, ".mp3")
        self.assertEqual(duration_sec, 12)
        self.assertEqual(fake_client.calls, 2)
        self.assertEqual([call.args[0] for call in sleep_mock.call_args_list], [9.0])


if __name__ == "__main__":
    unittest.main()
