"""Regenerate or verify locks with the pinned uv container; never run headers."""
from pathlib import Path
import os
import shutil
import subprocess
import sys
import tempfile

from dependency_updates.policy import python_lock, requirement_shape, Refused
from dependency_updates.release_age import generation_environment, release_cutoff

DIRECTORIES = ('worker', 'infra/maintenance', 'infra/audio-concat')
COMMAND = ('uv', 'pip', 'compile', '--python-version=3.12', '--universal', '--generate-hashes', '--output-file=requirements.txt', 'requirements.in')


class LockError(Exception):
    pass


def compile_lock(directory, cutoff=None, generate=True):
    environment = os.environ.copy()
    # Verification retains the locked version preferences and refreshes hashes;
    # it must never inherit generation's upgrade or publication cutoff settings.
    for key in ('UV_CONFIG_FILE', 'UV_EXCLUDE_NEWER'):
        environment.pop(key, None)
    if generate:
        environment.update(generation_environment(
            cutoff or release_cutoff(), str(Path(__file__).with_name('uv-renovate.toml'))))
    subprocess.run(COMMAND, cwd=directory, env=environment, check=True, stdout=subprocess.DEVNULL)


def check_directory(directory):
    """Keep locked versions as resolver preferences, recompute all artifact hashes."""
    source = directory / 'requirements.in'
    lock = directory / 'requirements.txt'
    try:
        requirement_shape(source.read_text())
        pins = python_lock(lock.read_text())
    except Refused as error:
        raise LockError(str(error)) from None
    with tempfile.TemporaryDirectory() as temporary:
        target = Path(temporary)
        shutil.copyfile(source, target / 'requirements.in')
        # uv reuses hashes from an existing output, including tampered hashes.
        # Retain only version preferences so all hashes are fetched afresh.
        preferences = ''.join(pin['requirement'] + '==' + pin['version'] + (' ; ' + pin['marker'] if pin['marker'] else '') + '\n' for pin in pins.values())
        (target / 'requirements.txt').write_text(preferences)
        compile_lock(target, generate=False)
        if lock.read_bytes() != (target / 'requirements.txt').read_bytes():
            raise LockError('Python lock is stale or has invalid artifact hashes: ' + str(directory))


def main():
    mode = sys.argv[1] if len(sys.argv) == 2 else ''
    if mode not in ('check', 'generate'):
        raise SystemExit('Expected check or generate')
    cutoff = release_cutoff() if mode == 'generate' else None
    for directory in DIRECTORIES:
        path = Path('/repository') / directory
        if mode == 'generate':
            compile_lock(path, cutoff=cutoff)
        else:
            check_directory(path)
        print(directory + ': ' + mode + ' succeeded')


if __name__ == '__main__':
    try:
        main()
    except (LockError, subprocess.CalledProcessError) as error:
        raise SystemExit(str(error)) from None
