"""Real wrapper/image assertions with synthetic credentials and no network."""
import os
import errno
import socket
import stat
from pathlib import Path
from maintenance.post_apply import validate_source

validate_source(Path('/secrets/source.txt').read_bytes())
assert os.environ['MAINTENANCE_EXECUTOR_SHA'] != os.environ['GITHUB_SHA']
assert os.environ['GH_TOKEN'] == 'fixture-token-never-public'
assert not any(os.environ.get(name) for name in ('MAINTENANCE_SCAN_SSH_KEY', 'MAINTENANCE_OPS_WRITE_KEY',
    'MAINTENANCE_HOSTS_JSON', 'MAINTENANCE_KNOWN_HOSTS', 'MAINTENANCE_APPLY_SSH_KEY',
    'MAINTENANCE_APPROVAL_PRIVATE_KEY', 'MAINTENANCE_OPS_READ_KEY', 'MAINTENANCE_ANTHROPIC_API_KEY'))
assert {path.name for path in Path('/secrets').iterdir()} == {
    'source.txt', 'host_key', 'ops_key', 'hosts.json', 'known_hosts', 'inputs.json'}
for directory in ('/secrets', '/private'):
    assert stat.S_IMODE(Path(directory).stat().st_mode) == 0o700
for path in Path('/secrets').iterdir():
    assert stat.S_IMODE(path.stat().st_mode) == 0o600
assert Path('/secrets/host_key').read_text().strip() == 'fixture-scan-key-never-public'
# Docker Desktop may retain inactive tunnel interfaces in network-none. There
# must be no route or usable external transport in the actual network namespace.
assert len(Path('/proc/net/route').read_text().splitlines()) == 1
with socket.socket() as connection:
    connection.settimeout(0.2)
    assert connection.connect_ex(('192.0.2.1', 443)) == errno.ENETUNREACH
status = Path('/proc/self/status').read_text()
assert 'CapEff:\t0000000000000000' in status and 'NoNewPrivs:\t1' in status
try:
    Path('/secrets/scope-violation').write_text('fixture')
except OSError:
    pass
else:
    raise AssertionError('runtime secret mount must be read-only')
error = Path('/private/error.txt').read_text()
assert 'pipeline_' not in error and 'run_context' not in error
summary = Path('/private/summary.md').read_text()
assert 'Do not retry apply' in summary and 'daily scanner' in summary
assert 'fixture-' not in summary and 'Traceback' not in summary
print('Pipeline image, distinct executor, credential scope, network none, read-only secrets and private failure verified')
