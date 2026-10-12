"""Synthetic children only: no APT, database or service commands are executed."""

import contextlib
import os
from pathlib import Path
import subprocess
import sys
import tempfile
import tracemalloc
import unittest
from unittest.mock import patch

from maintenance.host import SubprocessRunner
from maintenance.plan import MaintenanceError


class SubprocessRunnerTests(unittest.TestCase):
    def setUp(self):
        self.runner = SubprocessRunner()
        self.tmp = tempfile.TemporaryDirectory()
        self.addCleanup(self.tmp.cleanup)
        self.marker = Path(self.tmp.name) / "completed"

    def child(self, source, **kwargs):
        return self.runner([sys.executable, "-I", "-c", source], **kwargs)

    def completion(self):
        return f"\nfrom pathlib import Path\nPath({str(self.marker)!r}).write_text('completed')\n"

    def small_limits(self):
        stack = contextlib.ExitStack()
        for key in ("MAX_STDOUT_BYTES", "MAX_STDERR_BYTES"):
            stack.enter_context(patch.object(SubprocessRunner, key, 128 * 1024, create=True))
        return stack

    def test_stdout_and_stderr_overflow_drain_until_child_completion(self):
        sources = {
            "stdout": "import os\nfor _ in range(64): os.write(1, b'x' * 8192)\nos.write(2, b'finished')",
            "stderr": "import os\nfor _ in range(64): os.write(2, b'x' * 8192)\nos.write(1, b'finished')",
            "interleaved": "import os\nfor _ in range(64):\n os.write(1, b'x' * 8192)\n os.write(2, b'x' * 8192)",
        }
        for stream, source in sources.items():
            with self.subTest(stream=stream), self.small_limits():
                self.marker.unlink(missing_ok=True)
                with self.assertRaisesRegex(MaintenanceError, "^command output exceeds limit$"):
                    self.child(source + self.completion(), timeout=None)
                self.assertEqual(self.marker.read_text(), "completed")

    def test_default_stdout_limit_bounds_memory_while_draining_large_output(self):
        # A 48 MiB producer must not become a 48 MiB captured output. The child
        # creates only a fixed chunk, so tracemalloc measures the runner alone.
        source = "import os\nfor _ in range(6144): os.write(1, b'x' * 8192)"
        tracemalloc.start()
        try:
            with self.assertRaisesRegex(MaintenanceError, "^command output exceeds limit$"):
                self.child(source + self.completion(), timeout=None)
            _, peak = tracemalloc.get_traced_memory()
        finally:
            tracemalloc.stop()
        self.assertEqual(self.marker.read_text(), "completed")
        self.assertLess(peak, 24 * 1024 * 1024)

    def test_default_stderr_limit_discards_body_and_bounds_memory(self):
        source = "import os\nfor _ in range(6144): os.write(2, b'x' * 8192)"
        tracemalloc.start()
        try:
            with self.assertRaisesRegex(MaintenanceError, "^command output exceeds limit$"):
                self.child(source + self.completion(), timeout=None)
            _, peak = tracemalloc.get_traced_memory()
        finally:
            tracemalloc.stop()
        self.assertEqual(self.marker.read_text(), "completed")
        self.assertLess(peak, 2 * 1024 * 1024)

    def test_exact_limits_and_newline_translation_preserve_stdout(self):
        with self.small_limits():
            self.assertEqual(self.child("import os\nos.write(1, b'x' * 131072)\nos.write(2, b'y' * 131072)"),
                             "x" * 131072)
        self.assertEqual(self.child("import os\nos.write(1, b'a\\r\\nb\\rc\\n')"), "a\nb\nc\n")

    def test_utf8_codepoint_split_across_reads_is_valid(self):
        source = ("import os, time\nos.write(1, b'\\xe3')\nos.write(2, b'\\xf0\\x9f')\n"
                  "time.sleep(0.05)\nos.write(1, b'\\x81\\x82')\nos.write(2, b'\\x98\\x80')")
        self.assertEqual(self.child(source), "あ")

    def test_invalid_utf8_in_either_stream_drains_without_disclosing_body(self):
        for descriptor, invalid in [(1, "b'private-body\\xff'"), (2, "b'private-body\\xff'"),
                                    (1, "b'private-body\\xe3'"), (2, "b'private-body\\xe3'")]:
            with self.subTest(descriptor=descriptor, invalid=invalid), self.small_limits():
                self.marker.unlink(missing_ok=True)
                source = (f"import os\nos.write({descriptor}, {invalid})\n"
                          "for _ in range(64):\n os.write(1, b'x' * 8192)\n os.write(2, b'x' * 8192)")
                with self.assertRaisesRegex(MaintenanceError, "^command execution failed or timed out$") as error:
                    self.child(source + self.completion(), timeout=None)
                self.assertEqual(self.marker.read_text(), "completed")
                self.assertTrue(error.exception.__cause__ is None, "output bytes must not be retained in an error cause")
                self.assertNotIn("private-body", str(error.exception))

    def test_incomplete_utf8_at_eof_is_rejected_after_child_completion(self):
        for descriptor in (1, 2):
            with self.subTest(descriptor=descriptor):
                self.marker.unlink(missing_ok=True)
                source = f"import os\nos.write({descriptor}, b'\\xe3')"
                with self.assertRaisesRegex(MaintenanceError, "^command execution failed or timed out$"):
                    self.child(source + self.completion(), timeout=None)
                self.assertEqual(self.marker.read_text(), "completed")

    def test_closed_output_pipes_still_wait_for_child_completion_without_timeout(self):
        source = "import os, time\nos.close(1)\nos.close(2)\ntime.sleep(0.05)"
        self.assertEqual(self.child(source + self.completion(), timeout=None), "")
        self.assertEqual(self.marker.read_text(), "completed")

    def test_output_validation_does_not_disable_an_explicit_finite_timeout(self):
        source = "import os, time\nfor _ in range(64): os.write(1, b'x' * 8192)\ntime.sleep(10)"
        with self.small_limits(), self.assertRaisesRegex(MaintenanceError, "^command execution failed or timed out$"):
            self.child(source + self.completion(), timeout=0.1)
        self.assertFalse(self.marker.exists())

    def test_nonzero_exit_check_false_returns_stdout(self):
        source = "import os, sys\nos.write(1, b'result')\nos.write(2, b'private-error')\nsys.exit(7)"
        self.assertEqual(self.child(source, check=False), "result")
        with self.assertRaises(MaintenanceError) as error:
            self.child(source)
        self.assertEqual(str(error.exception), "command failed: " + sys.executable)
        self.assertNotIn("private-error", str(error.exception))

    def test_finite_timeout_still_kills_and_reaps_synthetic_child(self):
        for source in ("import time\ntime.sleep(10)",
                       "import os, time\nos.close(1)\nos.close(2)\ntime.sleep(10)"):
            with self.subTest(source=source):
                children = []
                popen = subprocess.Popen

                def record(*args, **kwargs):
                    child = popen(*args, **kwargs)
                    children.append(child)
                    return child

                with patch("maintenance.host.subprocess.Popen", side_effect=record):
                    with self.assertRaisesRegex(MaintenanceError, "^command execution failed or timed out$"):
                        self.child(source + self.completion(), timeout=0.1)
                self.assertLess(children[0].returncode, 0)
                self.assertFalse(self.marker.exists())

    def test_clean_environment_devnull_stdin_and_explicit_descriptor(self):
        with patch.dict(os.environ, {"SIFTO_TEST_CALLER_SECRET": "private-value"}):
            source = ("import json, os, sys\nassert 'SIFTO_TEST_CALLER_SECRET' not in os.environ\n"
                      "assert sys.stdin.buffer.read() == b''\nprint(os.environ['NEEDRESTART_MODE'])")
            self.assertEqual(self.child(source), "l\n")
        with self.marker.open("w+b") as descriptor:
            descriptor.write(b"synthetic-fd-data")
            descriptor.flush()
            descriptor.seek(0)
            environment = {**SubprocessRunner.ENV, "PGPASSFILE": "/synthetic/health.pgpass"}
            source = ("import os\nassert os.environ['PGPASSFILE'] == '/synthetic/health.pgpass'\n"
                      f"assert os.read({descriptor.fileno()}, 100) == b'synthetic-fd-data'\nprint('ok')")
            self.assertEqual(self.child(source, env=environment, pass_fds=(descriptor.fileno(),)), "ok\n")
            self.assertEqual(environment["PGPASSFILE"], "/synthetic/health.pgpass")


if __name__ == "__main__":
    unittest.main()
