import tempfile
import unittest
from pathlib import Path
from unittest.mock import patch

from app.concat_job import download_direct


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


if __name__ == "__main__":
    unittest.main()
