"""Source-owned PUBLIC actual S6/Laravel native-host proof. No production entry.

The Linux runner's own proc/cgroup view is required. Unsupported pidfd, mapping,
driver, metadata access, scope budgets or restoration is a failing proof.
"""
from dataclasses import asdict
import hashlib
import json
import os
from pathlib import Path
import platform
import secrets
import stat
import subprocess
import sys
import tempfile
import time
import cryptography

PACKAGE = Path(__file__).resolve().parents[1]
sys.path.insert(0, str(PACKAGE))
from maintenance.coolify_docker import (_SchedulerHostProcessControl, _SchedulerDrain, DockerError, strict_json,
    _SCHEDULER_IMAGE_PINS, _scheduler_image_format, _scheduler_image_identity)
from maintenance.coolify_rehearsal import clone_env

PROJECT = 'sifto-startup-scheduler-public'


def check(condition, message):
    if not condition:
        raise RuntimeError(message)


class Fixture:
    def __init__(self, public):
        check(public.is_absolute() and public.resolve() == public and not public.is_symlink()
              and public.parent == Path('/tmp').resolve() and public.lstat().st_uid == 0
              and stat.S_IMODE(public.lstat().st_mode) == 0o700, 'protected native public directory required')
        self.public = Path(tempfile.mkdtemp(prefix='native-evidence-', dir=public)).resolve()
        self.root = self.public / 'fixture'
        self.root.mkdir(mode=0o700)
        os.chmod(self.root, 0o700)
        self.run = secrets.token_hex(16)
        self.state = self.root / 'state'
        self.state.mkdir(mode=0o700)
        os.chown(self.state, 9999, 9999)
        (self.root / 'canonical.env').write_bytes(clone_env((PACKAGE / 'startup/public.env').read_bytes()).replace(
            b'SCHEDULER_ENABLED=false\n', b'SCHEDULER_ENABLED=true\n'))
        (self.root / 'init').write_bytes((PACKAGE / 'maintenance/coolify_startup_init').read_bytes())
        (self.root / 'init').chmod(0o755)
        self.request()
        self.env = {'PATH': '/usr/bin:/bin', 'HOME': '/nonexistent', 'LC_ALL': 'C',
            'SIFTO_SCHEDULER_ENV': str(self.root / 'canonical.env'),
            'SIFTO_STARTUP_PUBLIC_INIT': str(self.root / 'init'),
            'SIFTO_SCHEDULER_STATE': str(self.state),
            'SIFTO_SCHEDULER_REQUEST': str(self.root / 'request.json'),
            'SIFTO_SCHEDULER_RUN': self.run}
        self.controls = []
        self.drains = []
        self.started = False

    def record(self, name, value):
        raw = json.dumps(value, separators=(',', ':'), sort_keys=True).encode() + b'\n'
        check(len(raw) <= 8192, 'public output bound')
        with (self.public / 'native-observations.jsonl').open('ab') as stream:
            stream.write(raw)
        print(name + ': ' + raw.decode().strip(), flush=True)

    def command(self, args, *, timeout=30, limit=65536):
        result = subprocess.run(['/usr/bin/docker', *args], stdin=subprocess.DEVNULL,
            stdout=subprocess.PIPE, stderr=subprocess.PIPE, timeout=timeout,
            env=self.env, check=False)
        check(len(result.stdout) <= limit and len(result.stderr) <= 65536, 'fixture output bound')
        check(result.returncode == 0, 'fixed fixture command failed: ' + ' '.join(args[:2]))
        return result.stdout

    def compose(self, args, **kwargs):
        return self.command(['compose', '--env-file', '/dev/null', '-p', PROJECT, '-f',
            str(PACKAGE / 'startup-scheduler/compose.yml'), *args], **kwargs)

    def empty(self):
        for kind, args in (('container', ['container', 'ls', '-aq']),
                           ('volume', ['volume', 'ls', '-q']), ('network', ['network', 'ls', '-q'])):
            check(not self.command([*args, '--filter=label=com.docker.compose.project=' + PROJECT]).strip(),
                  'fixture requires empty owned namespace: ' + kind)
        for name in (PROJECT + '_storage', PROJECT + '_bootstrap-cache'):
            check(not self.command(['volume', 'ls', '-q', '--filter=name=' + name]).strip(),
                  'fixture refuses preexisting intended volume')

    def request(self):
        # Root is the source-owned fixture phase owner. The application sees a
        # RO mount and cannot replace this bound request with its own JSON.
        request = {'version': 1, 'phase': 'scheduler-proof-held', 'generation': secrets.token_hex(16),
                   'deadline_ns': time.monotonic_ns() + 120 * 1000000000}
        path = self.root / 'request.json'
        path.write_text(json.dumps(request, separators=(',', ':')))
        path.chmod(0o444)

    def app(self, service, mode):
        check(service in ('old', 'target') and mode in ('prepare', 'hold', 'unhold', 'source', 'topology'),
              'fixed fixture operation')
        return self.compose(['exec', '-T', service, 'php', '/scheduler/fixture.php', mode], timeout=15)

    def root_path(self, service):
        raw = self.compose(['ps', '-q', service]).strip()
        check(len(raw) == 64 and all(part in b'abcdef0123456789' for part in raw), 'owned fixture CID')
        value = self.command(['container', 'inspect', '--format={{.State.Pid}}', raw.decode()]).strip()
        check(value.isdigit() and int(value) > 0, 'owned init PID')
        return Path('/proc/' + value.decode() + '/root')

    def caps(self, service):
        raw = self.compose(['ps', '-q', service]).strip().decode('ascii')
        value = strict_json(self.command(['container', 'inspect', '--format',
            '{"memory":{{json .HostConfig.Memory}},"memory_swap":{{json .HostConfig.MemorySwap}},"cpu":{{json .HostConfig.NanoCpus}},"pids":{{json .HostConfig.PidsLimit}},"running":{{json .State.Running}},"oom":{{json .State.OOMKilled}}}', raw]))
        expected = {'postgres': (256 * 1024 * 1024, 250000000, 128),
                    'redis': (128 * 1024 * 1024, 125000000, 64),
                    'old': (512 * 1024 * 1024, 1000000000, 256),
                    'target': (512 * 1024 * 1024, 1000000000, 256)}[service]
        check((value['memory'], value['cpu'], value['pids']) == expected
              and value['memory_swap'] == value['memory'] and value['running'] is True
              and value['oom'] is False, 'fixed actual resource caps/OOM')
        self.record('caps', dict(service=service, **value))

    def control(self, service, name):
        self.request()
        projection = self.root_path(service) / 'probe/scheduler.context.json'
        if projection.exists():
            info = projection.lstat()
            check(stat.S_ISREG(info.st_mode) and info.st_uid == 9999
                  and stat.S_IMODE(info.st_mode) == 0o600 and info.st_nlink == 1, 'owned completed context')
            projection.unlink()
        private = self.root / ('private-' + service + '-' + name)
        private.mkdir(mode=0o700)
        started = time.monotonic()
        try:
            control = _SchedulerHostProcessControl._for_public_fixture(
                service=service, run=self.run, root=self.root, private=private)
        except DockerError as failure:
            checks = getattr(failure, '_scheduler_placement_checks', None)
            if checks is not None:
                check(type(checks) is dict and len(checks) == 15
                      and all(type(value) is bool for value in checks.values()), 'fixed mapping diagnostic')
                self.record('mapping-rejected', dict(category='native_mapping_rejected', checks=checks,
                            startup_registered=False, production_eligible=False))
            checks = getattr(failure, '_scheduler_identity_checks', None)
            if checks is not None:
                check(str(failure) == 'startup_state' and type(checks) is dict
                      and set(checks) == {'scope', 'role', 'task_tokens', 'boot', 'process', 'parent', 'init',
                                          'members', 'executable_sha256'}
                      and all(type(value) is bool for value in checks.values()), 'fixed identity diagnostic')
                self.record('identity-rejected', dict(case=name, image_version=service, category='startup_state',
                            checks=checks, startup_registered=False, production_eligible=False))
            raise
        self.controls.append(control)
        self.record('mapped', dict(case=name, image_version=service, category='owned_native_mapping',
            elapsed_ms=int((time.monotonic() - started) * 1000),
            process_count=len(control._selected['members']), driver=control._driver,
            production_eligible=False, startup_registered=False))
        return control

    def drain(self, control):
        drain = _SchedulerDrain(control)
        self.drains.append(drain)
        return drain

    def marker(self, mode, name):
        return self.state / mode / name

    def wait(self, mode, name, seconds):
        limit = time.monotonic() + seconds
        while not self.marker(mode, name).is_file():
            check(time.monotonic() < limit, 'actual scheduler marker timed out: ' + mode + '/' + name)
            time.sleep(.02)

    def release(self, mode, name='release'):
        self.marker(mode, name).write_bytes(b'PUBLIC')
        os.chown(self.marker(mode, name), 9999, 9999)

    def arm(self, service, mode):
        directory = self.state / mode
        directory.mkdir(mode=0o700)
        os.chown(directory, 9999, 9999)
        (self.state / 'case').write_text(mode)
        os.chown(self.state / 'case', 9999, 9999)
        self.app(service, 'unhold')
        self.wait(mode, 'ready', 70)
        self.app(service, 'hold')

    def observe_case(self, service, name, result):
        self.record('case', dict(case=name, image_version=service, **asdict(result),
            production_eligible=False, startup_registered=False))

    def restore(self, service, name, control):
        started = time.monotonic()
        restored = control.restore()
        self.record('restore', dict(image_version=service, case=name, **asdict(restored),
            elapsed_ms=int((time.monotonic() - started) * 1000)))
        check(restored.category in ('restored', 'unchanged'), 'exact native launcher restore failed')

    def matrix(self, service):
        self.app(service, 'hold')
        control = self.control(service, 'idle')
        result = self.drain(control).run()
        self.observe_case(service, 'idle', result)
        check(result.category == 'quiescent', 'actual initial empty scheduler not proven')
        self.restore(service, 'idle', control)

        import signal
        # Harness-owned deliberate original-stop injection, never a production
        # method accepting a signal or public PID. Exact retained pidfd only.
        control = self.control(service, 'original-stopped')
        signal.pidfd_send_signal(control._pidfd, signal.SIGSTOP)
        limit = time.monotonic() + 1
        while not all(state == 'T' for _, state in control._tasks()):
            check(time.monotonic() < limit, 'owned original-stop latch')
        result = self.drain(control).run()
        self.observe_case(service, 'original-stopped', result)
        check(result.category == 'quiescent' and not result.restore_required, 'original stopped preservation')
        self.restore(service, 'original-stopped', control)
        check(all(state == 'T' for _, state in control._tasks()), 'original stopped was continued')
        # Finish the harness's independent injected stop before the next case.
        signal.pidfd_send_signal(control._pidfd, signal.SIGCONT)

        self.arm(service, 'normal')
        control = self.control(service, 'normal-later-selected')
        drain = self.drain(control)
        check(control.freeze().category == 'frozen', 'actual scheduler freeze')
        self.release('normal')
        self.wait('normal', 'late', 5)
        result = drain.observe()
        self.observe_case(service, 'later-event-selected-before-barrier', result)
        check(result.category == 'held' and result.live_count > 0, 'later selected event was missed')
        self.release('normal', 'release-late')
        self.wait('normal', 'done', 5)
        limit = time.monotonic() + 4
        while True:
            result = drain.observe()
            if result.category == 'quiescent' or result.unknown_count or time.monotonic() >= limit:
                break
            time.sleep(.01)
        self.observe_case(service, 'normal-zero', result)
        check(result.category == 'quiescent' and result.finished_count > 0, 'positive actual known-child normal exit missing')
        self.restore(service, 'normal-zero', control)

        self.arm(service, 'detached')
        control = self.control(service, 'detached')
        drain = self.drain(control)
        check(control.freeze().category == 'frozen', 'actual detached launcher freeze')
        self.release('detached')
        self.wait('detached', 'done', 5)
        # Allow S6/PID1 to naturally reap detached work. No disappearance is
        # promoted to a success receipt; the exact frozen owner is preserved.
        time.sleep(.2)
        result = drain.observe()
        self.observe_case(service, 'detached-reaped', result)
        check(result.category == 'held' and result.unknown_count > 0, 'detached disappearance was declared successful')
        self.restore(service, 'detached', control)

        self.arm(service, 'pipefull')
        control = self.control(service, 'pipefull')
        drain = self.drain(control)
        check(control.freeze().category == 'frozen', 'actual pipefull launcher freeze')
        self.release('pipefull')
        time.sleep(.2)
        result = drain.observe()
        self.observe_case(service, 'pipe-backpressure', result)
        check(result.category == 'held' and result.live_count > 0
              and not self.marker('pipefull', 'first-finished').exists(), 'pipe backpressure work disappeared')
        control._deadline = time.monotonic() - 1
        self.restore(service, 'pipe-backpressure-timeout', control)
        self.wait('pipefull', 'first-finished', 10)

        (self.state / 'case').write_text('idle')
        self.app(service, 'hold')
        control = self.control(service, 'request-drift')
        self.request()  # authenticated generation changed BEFORE any STOP
        result = self.drain(control).run()
        self.observe_case(service, 'request-identity-drift', result)
        check(result.category == 'rejected', 'changed native phase request did not reject')
        check(all(state != 'T' for _, state in control._tasks()), 'request drift stopped original launcher')

    def run_all(self):
        self.empty()
        group = Path('/proc/self/cgroup').read_text().strip().split('::')
        check(len(group) == 2 and group[0] == '0' and group[1].startswith('/'), 'observer v2 scope')
        scope = Path('/sys/fs/cgroup' + group[1])
        check((scope / 'memory.max').read_text() == '134217728\n'
              and (scope / 'memory.swap.max').read_text() == '0\n'
              and (scope / 'pids.max').read_text() == '64\n', 'observer scope bounds')
        quota = (scope / 'cpu.max').read_text().split()
        check(len(quota) == 2 and quota[0].isdigit() and quota[1].isdigit()
              and 4 * int(quota[0]) == int(quota[1]) > 0, 'observer CPU scope bound')
        for name, reference, expected, _ in _SCHEDULER_IMAGE_PINS:
            result = subprocess.run(['/usr/bin/docker', 'image', 'inspect', '--format={{.Id}}', reference],
                stdout=subprocess.PIPE, stderr=subprocess.PIPE, env=self.env, check=False)
            if result.returncode:
                self.command(['pull', '--platform=linux/arm64', reference], timeout=300)
            actual = strict_json(self.command(['image', 'inspect', '--format=' + _scheduler_image_format(), reference]))
            # Safe PUBLIC image fields are recorded before validation so a
            # failing store shape remains distinguishable from its diagnosis.
            self.record('image-' + name, dict(category='image_observation', reference=reference,
                        expected_config=expected, **actual))
            _scheduler_image_identity(reference, actual)
        info = strict_json(self.command(['info', '--format', '{"driver":{{json .CgroupDriver}},"version":{{json .CgroupVersion}},"os":{{json .OSType}},"arch":{{json .Architecture}}}']))
        self.record('preflight', dict(category='native_preflight', kernel=platform.release(), **info,
            docker_version=self.command(['version', '--format={{.Server.Version}}'], limit=128).decode().strip(),
            pidfd=hasattr(os, 'pidfd_open'), python=platform.python_version(), cryptography=cryptography.__version__,
            startup_registered=False, production_eligible=False))
        check(info['version'] == '2' and info['driver'] in ('systemd', 'cgroupfs')
              and info['arch'] in ('aarch64', 'arm64') and hasattr(os, 'pidfd_open'), 'native preflight unsupported')
        self.started = True
        self.compose(['up', '-d', 'postgres', 'redis'])
        self.caps('postgres')
        self.caps('redis')
        for service in ('old', 'target'):
            self.compose(['up', '-d', '--no-deps', service])
            self.compose(['exec', '-T', service, 'sh', '/scheduler/wait-ready.sh'], timeout=35)
            self.app(service, 'prepare')
            source = self.app(service, 'source')
            (self.public / (service + '-source.txt')).write_bytes(source)
            self.matrix(service)
            self.caps(service)
            # No active fixture work remains. This is owned teardown, never
            # counted as natural scheduled-work completion.
            self.compose(['stop', '--timeout', '10', service])
            for mode in ('normal', 'detached', 'pipefull'):
                for file in (self.state / mode).iterdir():
                    file.unlink()
                (self.state / mode).rmdir()

    def cleanup(self):
        # Release only these PUBLIC synthetic fixture gates. Draining product
        # code never terminates work or drops the launcher's Process objects.
        for mode in ('normal', 'detached', 'pipefull'):
            if (self.state / mode).is_dir():
                self.release(mode)
                if mode == 'normal':
                    self.release(mode, 'release-late')
        for drain in self.drains:
            drain.close()
        for control in self.controls:
            # Any live acknowledged STOP obligation is compensated before
            # teardown; errors never become a green cleanup/completion result.
            if control._intent and control._intent['restore_required']:
                restored = control.restore()
                check(restored.category == 'restored', 'owned exact cleanup restore failed')
            control.close()
        if self.started:
            for mode, marker in (('normal', 'done'), ('detached', 'done'), ('pipefull', 'first-finished')):
                if self.marker(mode, 'ready').exists():
                    self.wait(mode, marker, 10)
            self.compose(['down', '--volumes', '--timeout', '10'])
            self.empty()
            self.record('cleanup', dict(category='owned_cleanup_verified', startup_registered=False,
                                        production_eligible=False))


def main():
    if sys.argv[1:] == ['generate']:
        sys.stdout.buffer.write(clone_env((PACKAGE / 'startup/public.env').read_bytes()).replace(
            b'SCHEDULER_ENABLED=false\n', b'SCHEDULER_ENABLED=true\n'))
    elif len(sys.argv) == 3 and sys.argv[1] == 'native':
        check(platform.system() == 'Linux' and os.geteuid() == 0, 'native Linux root host observer required')
        fixture = Fixture(Path(sys.argv[2]).resolve())
        status = 0
        try:
            fixture.run_all()
        except BaseException as failure:
            status = 1
            fixture.record('failure', dict(category='native_fixture_failed', failure_type=type(failure).__name__,
                startup_registered=False, production_eligible=False))
            raise
        finally:
            fixture.cleanup()
        sys.exit(status)
    else:
        raise SystemExit('fixed scheduler fixture operation required')


if __name__ == '__main__':
    main()
