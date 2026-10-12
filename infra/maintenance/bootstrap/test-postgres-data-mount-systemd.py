"""Ubuntu 24/systemd 255 offline loader fixture; never starts any host service."""
from pathlib import Path
import re
import subprocess
import tempfile

UNIT = 'postgresql@16-main.service'


def verify(root):
    result = subprocess.run(['/usr/bin/systemd-analyze', 'verify', '--root=' + str(root), UNIT],
                            env={'PATH': '/usr/bin:/bin', 'LC_ALL': 'C', 'SYSTEMD_LOG_LEVEL': 'debug'},
                            stdin=subprocess.DEVNULL, capture_output=True, text=True, timeout=30)
    if result.returncode:
        raise RuntimeError('offline systemd syntax verification failed: ' + result.stderr[-3000:])
    # The real systemd loader prints each unit's effective ordering/dependency DAG.
    return result.stdout + result.stderr


def unit_dump(output):
    marker = '-> Unit ' + UNIT + ':'
    start = output.index(marker)
    following = output.find('-> Unit ', start + len(marker))
    return output[start:following if following != -1 else None]


def main():
    version = subprocess.check_output(['/usr/bin/systemd-analyze', '--version'], text=True).splitlines()[0]
    assert re.match(r'systemd 255\b', version), version
    with tempfile.TemporaryDirectory(prefix='sifto-postgres-mount.') as temporary:
        root = Path(temporary)
        units = root / 'etc/systemd/system'
        units.mkdir(parents=True)
        (root / 'data').mkdir()
        (root / 'bin').mkdir()
        (root / 'bin/true').write_bytes(Path('/bin/true').read_bytes())
        (root / 'bin/true').chmod(0o755)
        (units / UNIT).write_text('[Unit]\nDefaultDependencies=no\n[Service]\nType=forking\nExecStart=/bin/true\nPIDFile=/run/postgresql/16-main.pid\n')
        (units / 'data.mount').write_text('[Unit]\nDefaultDependencies=no\n[Mount]\nWhat=/dev/fixture-database-volume\nWhere=/data\nType=ext4\nOptions=noauto\n')
        baseline = unit_dump(verify(root))
        assert not re.search(r'\b(?:After|Requires): data\.mount\b', baseline), baseline
        dropin = units / (UNIT + '.d')
        dropin.mkdir()
        policy = Path('/fixture/52-sifto-data-mount.conf').read_bytes()
        assert policy == b'[Unit]\nRequiresMountsFor=/data/postgres\n'
        (dropin / '52-sifto-data-mount.conf').write_bytes(policy)
        corrected = unit_dump(verify(root))
        assert re.search(r'\bRequiresMountsFor: /data/postgres\b', corrected), corrected
        assert re.search(r'\bAfter: data\.mount\b', corrected), corrected
        assert re.search(r'\bRequires: data\.mount\b', corrected), corrected
    print(version + ': baseline missing mount edge reproduced; fixed RequiresMountsFor adds Requires+After even for noauto; syntax verified; no services started')


if __name__ == '__main__':
    main()
