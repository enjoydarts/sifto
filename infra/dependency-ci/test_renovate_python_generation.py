"""Run the actual Renovate command/child env with the pinned real uv solver."""
import json
import os
from pathlib import Path
import shlex
import subprocess
import sys
import tempfile

from python_locks import COMMAND, check_directory
from dependency_updates.policy import UV_HEADER, python_lock
from dependency_updates.release_age import verify_python_release_age

payload = json.load(sys.stdin)
assert payload['version'] == '44.149.3'
assert tuple(shlex.split(payload['command'])) == COMMAND
assert payload['environment'] == {'UV_EXCLUDE_NEWER': '2026-10-05T00:00:00Z',
                                  'UV_CONFIG_FILE': '/sifto-python-generation.toml'}
with tempfile.TemporaryDirectory() as temporary:
    target = Path(temporary)
    (target / 'requirements.in').write_text('boto3==1.43.108\npydantic==2.13.5\n')
    (target / 'requirements.txt').write_text('botocore==1.43.111\npydantic-core==2.49.0\n')
    result = subprocess.run(shlex.split(payload['command']), cwd=target,
                            env={**os.environ, **payload['environment']},
                            capture_output=True, text=True, timeout=180)
    assert result.returncode == 0, result.stderr
    lock = (target / 'requirements.txt').read_text()
    pins = python_lock(lock)
    assert pins['botocore']['version'] == '1.43.108'
    assert pins['pydantic-core']['version'] == '2.46.5'
    assert all(pin['hashes'] for pin in pins.values())
    before = UV_HEADER + 'fixture==1.0.0 \\\n    --hash=sha256:' + 'a' * 64 + '\n    # via -r requirements.in\n'
    evidence = verify_python_release_age(before, lock, payload['environment']['UV_EXCLUDE_NEWER'])
    assert len(evidence) == len(pins)
    check_directory(target)
print('Renovate 44.149.3 -> uv 0.13.0: compatible mature transitives and every artifact hash independently verified; hash lock check passed')
