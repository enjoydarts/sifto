import tempfile
import unittest
from pathlib import Path
from unittest.mock import patch

from app.concat_job import download_direct, run_job


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


if __name__ == "__main__":
    unittest.main()
