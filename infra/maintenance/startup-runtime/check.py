"""PUBLIC fixture only: actual clone_env generation and strict result checks."""
import json
from pathlib import Path
import sys

from maintenance.coolify_rehearsal import clone_env
from maintenance.coolify_startup import parse_runtime_result


if sys.argv[1:] == ['generate']:
    canonical = clone_env(Path('startup/public.env').read_bytes())
    Path('/fixture/canonical.env').write_bytes(canonical)
    workers = canonical.replace(b'HORIZON_ENABLED=false\n', b'HORIZON_ENABLED=true\n').replace(
        b'SCHEDULER_ENABLED=false\n', b'SCHEDULER_ENABLED=true\n')
    # One synthetic worker pool fits the unchanged 512MiB application ceiling.
    workers += (b"HORIZON_MIN_PROCESSES='1'\nHORIZON_MAX_PROCESSES='1'\n"
                b"HORIZON_QUEUES='public-runtime-observe'\n")
    Path('/fixture/workers.env').write_bytes(workers)
elif sys.argv[1:] == ['validate']:
    raw = sys.stdin.buffer.read(32769)
    assert len(raw) <= 32768
    lines = raw.splitlines()
    assert len(lines) == 4
    values = [parse_runtime_result(line, selected_version=version) for line, version in zip(
        lines, ('4.4.2', '4.4.6', '4.4.6', '4.4.6'))]
    for index, value in enumerate(values):
        assert value.horizon_enabled is (index == 3)
        assert value.scheduler_enabled is (index == 3)
        assert all(row.kind == 'oneshot' and row.state == 'active' for row in value.services[:6])
        assert all(row.readiness == 'notified' and row.state == 'up' for row in value.services[6:8])
        assert all(row.readiness == 'unsupported' for row in value.services[8:])
        assert [row.role for row in value.services[8:10]] == ['reverb', 'terminal-server']
        assert [row.role for row in value.services[10:]] == (
            ['horizon', 'scheduler-worker'] if index == 3 else ['disabled-sleep', 'disabled-sleep'])
    assert values[2].unclassified_count >= 1
    assert values[2].membership_state in ('unknown', 'changed')
    print(json.dumps(dict(ok=True, category='public_passive_runtime_proof',
                         startup_registered=False, production_eligible=False)))
else:
    raise SystemExit(1)
