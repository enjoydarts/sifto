"""Exercise the real pinned uv solver against stale and corrupted lock copies."""
from pathlib import Path
import re
import shutil
import tempfile
import unittest
import subprocess
import sys

from python_locks import check_directory, LockError

ROOT = Path('/repository')


class PythonLocksTests(unittest.TestCase):
    def test_all_three_existing_locks_are_reproducible(self):
        for directory in ['worker', 'infra/maintenance', 'infra/audio-concat']:
            with self.subTest(directory=directory):
                check_directory(ROOT / directory)

    def test_stale_input_hash_and_missing_transitive_are_refused(self):
        for failure in ['direct', 'hash', 'transitive']:
            with self.subTest(failure=failure), tempfile.TemporaryDirectory() as temporary:
                target = Path(temporary)
                for file in ['requirements.in', 'requirements.txt']:
                    shutil.copyfile(ROOT / 'infra/audio-concat' / file, target / file)
                if failure == 'direct':
                    source = target / 'requirements.in'
                    source.write_text(source.read_text().replace('boto3==1.39.13', 'boto3==1.39.14'))
                else:
                    lock = target / 'requirements.txt'
                    text = lock.read_text()
                    if failure == 'hash':
                        text = re.sub(r'sha256:[0-9a-f]{64}', 'sha256:' + '0' * 64, text, count=1)
                    else:
                        start = text.index('six==')
                        end = text.find('\n', text.index('    # via', start))
                        text = text[:start] + text[end + 1:]
                    lock.write_text(text)
                with self.assertRaises(LockError):
                    check_directory(target)

    def test_pip_require_hashes_rejects_a_tampered_artifact(self):
        lock = (ROOT / 'infra/audio-concat/requirements.txt').read_text()
        pin = re.search(r'(?m)^six==[^\n]+\n(?:    --hash[^\n]+\n)+', lock).group()
        with tempfile.TemporaryDirectory() as temporary:
            path = Path(temporary) / 'requirements.txt'
            command = [sys.executable, '-m', 'pip', 'install', '--disable-pip-version-check', '--no-deps', '--no-cache-dir', '--require-hashes', '--index-url', 'https://pypi.org/simple', '--target', temporary + '/packages', '-r', str(path)]
            path.write_text(pin)
            result = subprocess.run(command, capture_output=True, text=True)
            self.assertEqual(result.returncode, 0, result.stdout + result.stderr)
            shutil.rmtree(temporary + '/packages')
            hashes = iter(['0' * 64, '1' * 64])
            path.write_text(re.sub(r'sha256:[0-9a-f]{64}', lambda _: 'sha256:' + next(hashes), pin))
            result = subprocess.run(command, capture_output=True, text=True)
            self.assertNotEqual(result.returncode, 0)
            self.assertIn('DO NOT MATCH THE HASHES', result.stderr)


if __name__ == '__main__':
    unittest.main()
