import tempfile
import unittest
from pathlib import Path
from unittest.mock import patch

from app.concat_job import concat_audio, download_direct, run_job


class DownloadDirectTests(unittest.TestCase):
    def test_download_direct_streams_response_to_file(self):
        expected = b"hello streamed world"

        class FakeResponse:
            def __init__(self, payload: bytes):
                self._payload = payload
                self._offset = 0

            def read(self, size=-1):
                if size is None or size < 0:
                    raise AssertionError("download_direct should not call read() without a chunk size")
                if self._offset >= len(self._payload):
                    return b""
                chunk = self._payload[self._offset : self._offset + size]
                self._offset += len(chunk)
                return chunk

            def __enter__(self):
                return self

            def __exit__(self, exc_type, exc, tb):
                return False

        with tempfile.TemporaryDirectory(prefix="concat-job-test-") as tmp_dir:
            destination = Path(tmp_dir) / "segment.mp3"
            with patch("app.concat_job.urllib.request.urlopen", return_value=FakeResponse(expected)):
                download_direct("https://example.com/segment.mp3", destination)

            self.assertEqual(destination.read_bytes(), expected)

    def test_run_job_returns_failure_when_success_callback_and_failure_callback_both_fail(self):
        class FakeR2Client:
            def upload_file(self, path: Path, object_key: str) -> None:
                return None

        with (
            patch("app.concat_job.R2Client", return_value=FakeR2Client()),
            patch("app.concat_job.download_segments", return_value=[Path("/tmp/segment-001.mp3")]),
            patch("app.concat_job.concat_audio"),
            patch("app.concat_job.normalize_audio", return_value=Path("/tmp/final.mp3")),
            patch("app.concat_job.probe_duration_seconds", return_value=42),
            patch(
                "app.concat_job.post_callback",
                side_effect=[RuntimeError("publish callback failed"), RuntimeError("failed callback failed")],
            ) as post_callback_mock,
        ):
            exit_code = run_job(
                job_id="job-1",
                request_id="request-1",
                callback_url="https://api.example.com/callback",
                callback_token="token",
                output_object_key="audio-briefings/user/job/episode.mp3",
                audio_object_keys=["audio-briefings/user/job/chunk-1.mp3"],
                provider_job_id="execution-1",
            )

        self.assertEqual(exit_code, 1)
        self.assertEqual(post_callback_mock.call_count, 2)

    def test_run_job_mixes_random_bgm_and_reports_selected_key(self):
        class FakeR2Client:
            def upload_file(self, path: Path, object_key: str) -> None:
                return None

        with (
            patch("app.concat_job.R2Client", return_value=FakeR2Client()),
            patch("app.concat_job.download_segments", return_value=[Path("/tmp/segment-001.mp3")]),
            patch("app.concat_job.concat_audio"),
            patch("app.concat_job.list_bgm_candidates", return_value=["bgm/track-1.mp3", "bgm/track-2.mp3"]),
            patch("app.concat_job.random.choice", return_value="bgm/track-2.mp3"),
            patch("app.concat_job.mix_bgm_with_normalize", return_value=("bgm/track-2.mp3", Path("/tmp/final.mp3"))),
            patch("app.concat_job.probe_duration_seconds", return_value=42),
            patch("app.concat_job.post_callback") as post_callback_mock,
        ):
            exit_code = run_job(
                job_id="job-1",
                request_id="request-1",
                callback_url="https://api.example.com/callback",
                callback_token="token",
                output_object_key="audio-briefings/user/job/episode.mp3",
                audio_object_keys=["audio-briefings/user/job/chunk-1.mp3"],
                provider_job_id="execution-1",
                bgm_enabled=True,
                bgm_r2_prefix="bgm/",
            )

        self.assertEqual(exit_code, 0)
        payload = post_callback_mock.call_args.args[2]
        self.assertEqual(payload["bgm_object_key"], "bgm/track-2.mp3")

    def test_run_job_falls_back_to_normalized_main_audio_when_bgm_mix_fails(self):
        class FakeR2Client:
            def upload_file(self, path: Path, object_key: str) -> None:
                return None

        with (
            patch("app.concat_job.R2Client", return_value=FakeR2Client()),
            patch("app.concat_job.download_segments", return_value=[Path("/tmp/segment-001.mp3")]),
            patch("app.concat_job.concat_audio"),
            patch("app.concat_job.list_bgm_candidates", return_value=["bgm/track-1.mp3"]),
            patch("app.concat_job.random.choice", return_value="bgm/track-1.mp3"),
            patch("app.concat_job.mix_bgm_with_normalize", side_effect=RuntimeError("mix failed")),
            patch("app.concat_job.normalize_audio", return_value=Path("/tmp/final.mp3")) as normalize_mock,
            patch("app.concat_job.probe_duration_seconds", return_value=42),
            patch("app.concat_job.post_callback") as post_callback_mock,
        ):
            exit_code = run_job(
                job_id="job-1",
                request_id="request-1",
                callback_url="https://api.example.com/callback",
                callback_token="token",
                output_object_key="audio-briefings/user/job/episode.mp3",
                audio_object_keys=["audio-briefings/user/job/chunk-1.mp3"],
                provider_job_id="execution-1",
                bgm_enabled=True,
                bgm_r2_prefix="bgm/",
            )

        self.assertEqual(exit_code, 0)
        self.assertEqual(normalize_mock.call_count, 1)
        payload = post_callback_mock.call_args.args[2]
        self.assertIsNone(payload.get("bgm_object_key"))


class TestConcatAudio(unittest.TestCase):
    def test_concat_audio_normalizes_inputs_and_inserts_one_second_gap_between_chunks(self):
        captured = {}

        def fake_run_command(command):
            captured["command"] = command
            return True

        with tempfile.TemporaryDirectory(prefix="concat-audio-test-") as tmp_dir:
            tmp_path = Path(tmp_dir)
            segment_files = [
                tmp_path / "segment-001.mp3",
                tmp_path / "segment-002.wav",
            ]
            output_path = tmp_path / "episode-concat.mp3"

            with patch("app.concat_job.run_command", side_effect=fake_run_command):
                concat_audio(segment_files, output_path)

        command = captured["command"]
        self.assertEqual(command[:2], ["ffmpeg", "-y"])
        self.assertIn("-filter_complex", command)
        filter_index = command.index("-filter_complex") + 1
        filter_graph = command[filter_index]
        self.assertIn("aresample=48000", filter_graph)
        self.assertIn("aformat=sample_fmts=fltp:channel_layouts=stereo", filter_graph)
        self.assertIn("apad=pad_dur=1", filter_graph)
        self.assertIn("concat=n=2:v=0:a=1", filter_graph)
        self.assertIn("-ar", command)
        self.assertIn("48000", command)
        self.assertIn("-ac", command)
        self.assertIn("2", command)
        self.assertIn("-q:a", command)
        self.assertIn("2", command)


if __name__ == "__main__":
    unittest.main()
