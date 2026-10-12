"""Exercise the real pinned uv solver against stale and corrupted lock copies."""
from pathlib import Path
import json
import re
import shutil
import tempfile
import unittest
import subprocess
import sys

from python_locks import COMMAND, check_directory, LockError
from dependency_updates.policy import python_lock

ROOT = Path('/repository')


class PythonLocksTests(unittest.TestCase):
    def test_indirect_updates_do_not_force_incompatible_solver_versions(self):
        config = json.loads((ROOT / 'renovate.json').read_text())
        rules = [rule for rule in config['packageRules']
                 if 'pip-compile' in rule.get('matchManagers', [])
                 and 'indirect' in rule.get('matchDepTypes', [])]
        self.assertEqual(len(rules), 1)
        self.assertIs(rules[0]['enabled'], False)
        self.assertEqual(config['minimumReleaseAge'], '7 days')
        self.assertIs(config['pip-compile']['lockFileMaintenance']['enabled'], False)

    def test_current_proposal_resolves_without_independent_transitive_pins(self):
        # Public PR 34 inputs at d8f8a9afa44a36c7c4c4c8636d3a1b08d5cf9c1d.
        # These reproduce Renovate run 38227994961 without modifying a Bot PR.
        inputs = '''fastapi==0.142.2
uvicorn[standard]==0.54.0
trafilatura==2.3.0
anthropic==0.125.0
httpx==0.28.1
httpcore==1.0.9
pydantic==2.13.5
redis==5.3.1
sentry-sdk[fastapi]==2.71.0
langfuse==3.15.0
json-repair==0.64.0
PyMuPDF==1.28.2
boto3==1.43.108
mutagen==1.48.1
google-auth==2.59.1
pytest==8.4.2
yt-dlp==2026.8.19
bgutil-ytdlp-pot-provider==1.3.2
'''
        forced = ['botocore==1.43.108', 'pydantic-core==2.49.0',
                  's3transfer==0.19.2', 'starlette==0.52.1']
        with tempfile.TemporaryDirectory() as temporary:
            target = Path(temporary)
            (target / 'requirements.in').write_text(inputs)
            shutil.copyfile(ROOT / 'worker/requirements.txt', target / 'requirements.txt')
            result = subprocess.run([*COMMAND, *('--upgrade-package=' + pin for pin in forced)],
                                    cwd=target, capture_output=True, text=True, timeout=180)
            self.assertNotEqual(result.returncode, 0)
            self.assertIn('pydantic-core==2.49.0', result.stderr)
            self.assertIn('pydantic-core==2.46.5', result.stderr)
            result = subprocess.run(COMMAND, cwd=target, capture_output=True, text=True, timeout=180)
            self.assertEqual(result.returncode, 0, result.stdout + result.stderr)
            pins = python_lock((target / 'requirements.txt').read_text())
            self.assertEqual(pins['pydantic']['version'], '2.13.5')
            self.assertEqual(pins['pydantic-core']['version'], '2.46.5')
            self.assertTrue(all(pin['hashes'] for pin in pins.values()))
            check_directory(target)

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
