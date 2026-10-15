"""Dormant synthetic-only native rehearsal phase engine.

No command line, host dispatch, eligibility, receipt or production proof API.
Real Task4 registrations are deliberately absent, so native startup fails closed.
"""
from dataclasses import dataclass
from contextlib import contextmanager
import hmac
import json
import os
import secrets
import signal
import stat
import time
import threading

from .coolify_docker import (NativeContext, NativeProcess, OwnedDocker, Operation, Phase,
    BudgetPolicy, DockerError, _budget, _closed, _require, _metadata, _read, _hash)
from .coolify_rehearsal import (read_regular, validate_bundle, clone_env, parse_env,
                                RehearsalError, MAX_EXPANDED)
from .coolify_evidence import compare_evidence


@dataclass(repr=False)
class _EngineRun:
    context: NativeContext
    docker: OwnedDocker
    handle: object
    deadline: float
    master: dict
    original_sha: str
    inventory: tuple
    resources: dict
    services: dict
    held: dict
    original_files: dict
    cleanup_deadline: float | None = None
    signals: object | None = None


class _SignalScope:
    def __enter__(self):
        _require(threading.current_thread() is threading.main_thread(), 'context_policy')
        self.received = self.cleaning = False
        self.previous = signal.getsignal(signal.SIGTERM)
        signal.signal(signal.SIGTERM, self._term)
        return self

    def _term(self, *_):
        self.received = True
        if not self.cleaning:
            raise KeyboardInterrupt()

    def __exit__(self, *_):
        signal.signal(signal.SIGTERM, self.previous)


@contextmanager
def _cleaning(signals):
    if signals is None:
        yield
        return
    previous = signals.cleaning
    signals.cleaning = True
    try:
        yield
    finally:
        signals.cleaning = previous


def _check_signal(run):
    _require(run.signals is None or not run.signals.received, 'interrupted')


def require_synthetic_context(context, docker):
    _require(type(context) is NativeContext and isinstance(docker, OwnedDocker)
             and docker.context is context, 'context_policy')
    _budget(context)
    docker._guard_context()


def require_verified_startup(context, docker):
    startup = context.startup
    _require(startup is not None and startup.verified_scope == 'synthetic-task4',
             'startup_profile_unverified')
    # Registry lookup belongs to OwnedDocker. A caller-created record never grants
    # native capability. Only test-local subclasses can model a future registry.
    docker._require_verified_startup()


def _capacity(context, docker, *, staged=0):
    policy = _budget(context)
    _require(type(staged) is int and 0 <= staged <= MAX_EXPANDED, 'resource_capacity')
    observations = docker._capacity_observations()
    _require(type(observations) is tuple and len(observations) == 2
             and all(type(v) is int and v >= 0 for v in observations), 'resource_capacity')
    available, free = observations
    _require(available >= policy.aggregate_memory + policy.host_reserve
             and free >= policy.staging_free + staged, 'resource_capacity')


def _guard(run, *, staged=0):
    _require(time.monotonic() < run.deadline, 'command_timeout')
    owned = run.docker._guard_run(run.handle)
    _staged_usage(owned['fd'])
    require_verified_startup(run.context, run.docker)
    _capacity(run.context, run.docker, staged=staged)
    if isinstance(run.docker._adapter, NativeProcess):
        run.docker._adapter._cgroup.guard()
    record = run.docker._runs[run.handle]
    run.docker._guard_file(record['fd'], 'hmac.master', run.master, 'hmac_identity')
    _require(len(_read(run.master['fd'], 32)) == 32, 'hmac_identity')
    for name, info in run.original_files.items():
        run.docker._guard_hashed_file(record['fd'], name, info, 'material_binding')


def _staged_usage(root_fd):
    """Bound owned mutable PG/storage data without reading file contents.

    Stream directory entries through owned descriptors; never follow a mutable
    application's symlink, FIFO or device, including sparse-file size growth.
    """
    total, count = 0, 0
    def walk(fd, depth):
        nonlocal total, count
        _require(depth <= 64, 'resource_capacity')
        with os.scandir(fd) as entries:
            for entry in entries:
                count += 1
                _require(count <= 50000, 'resource_capacity')
                info = os.stat(entry.name, dir_fd=fd, follow_symlinks=False)
                if stat.S_ISREG(info.st_mode):
                    total += info.st_size
                    _require(0 <= total <= 4 * 1024**3, 'resource_capacity')
                elif stat.S_ISDIR(info.st_mode):
                    child = os.open(entry.name, os.O_RDONLY | os.O_DIRECTORY | os.O_NOFOLLOW | os.O_CLOEXEC,
                                    dir_fd=fd)
                    try:
                        walk(child, depth + 1)
                    finally:
                        os.close(child)
                else:
                    raise DockerError('material_binding')
    try:
        walk(root_fd, 0)
    except DockerError:
        raise
    except OSError:
        raise DockerError('material_binding') from None
    return total


def _validated_original(context, docker):
    docker._guard_context()
    try:
        raw = read_regular(context.input_path, policy=context.input_policy)
        material = validate_bundle(raw, binding=context.binding)
    except RehearsalError:
        raise DockerError('material_binding') from None
    docker._guard_context()
    return raw, material


def _inventory(material):
    return tuple((name, directory, len(data), _hash(data))
                 for name, directory, data in material.recovery.entries)


def prepare_owned_run(context, docker, *, signals=None):
    """Validate raw trusted bytes before allocating any Docker/material capability."""
    require_synthetic_context(context, docker)
    require_verified_startup(context, docker)
    _capacity(context, docker)
    deadline = time.monotonic() + 2400
    handle = docker.allocate_run()
    run = None
    try:
        record = docker._guard_run(handle)
        if isinstance(docker._adapter, NativeProcess):
            docker._adapter._bind_run(handle)
        raw, material = _validated_original(context, docker)
        _capacity(context, docker, staged=sum(len(data) for _, _, data in material.recovery.entries))
        master = docker._exclusive_file(record['fd'], 'hmac.master', secrets.token_bytes(32))
        originals = {}
        # Private immutable original records, never mounts or executable inputs.
        for name, content in material.files:
            originals['original-' + name] = docker._exclusive_hashed_file(record['fd'], 'original-' + name, content)
        env = next(data for name, _, data in material.recovery.entries if name == 'source/.env')
        originals['original.env'] = docker._exclusive_hashed_file(record['fd'], 'original.env', env)
        originals['original.inventory'] = docker._exclusive_hashed_file(record['fd'], 'original.inventory',
            json.dumps(_inventory(material), separators=(',', ':')).encode())
        run = _EngineRun(context, docker, handle, deadline, master,
                         _hash(raw), _inventory(material), {}, {}, {}, originals)
        run.signals = signals
        del raw, material, env
        if isinstance(docker._adapter, NativeProcess):
            docker._adapter._steady()
        for operation in (Operation.IDENTITY_OLD, Operation.IDENTITY_TARGET,
                          Operation.IDENTITY_POSTGRES, Operation.IDENTITY_REDIS):
            _guard(run)
            container = docker.create(handle, operation=operation, phase=Phase.RUN, resources=None)
            docker.start(container.container_id)
            docker.read_identity(container.container_id)
            _remove_verified(docker, container.container_id)
        _require(set(record['identities']) == {'old', 'target', 'postgres', 'redis'}, 'image_identity')
        return run
    except BaseException as error:
        # Allocation failure still owns a journal and may own a late-created ID.
        with _cleaning(signals):
            complete = _cleanup_preparation(docker, handle)
        raise _failure(error, complete) from None


def _cleanup_preparation(docker, handle):
    if isinstance(docker._adapter, NativeProcess):
        docker._adapter._deadline = time.monotonic() + 120
    try:
        complete = OwnedDocker.cleanup(docker, handle) is True
    except BaseException:
        complete = False
    if isinstance(docker._adapter, NativeProcess) and docker._adapter._cgroup is not None:
        try:
            docker._adapter._finish()
        except BaseException:
            complete = False
    return complete and _transport_complete(docker)


def _write_leaf(parent, name, content, uid, gid):
    fd = os.open(name, os.O_WRONLY | os.O_CREAT | os.O_EXCL | os.O_NOFOLLOW | os.O_CLOEXEC,
                 0o600, dir_fd=parent)
    try:
        os.fchown(fd, uid, gid)
        os.fchmod(fd, 0o600)
        OwnedDocker._write_all(fd, content)
        os.fsync(fd)
    finally:
        os.close(fd)


def _directory(parent, name, uid, gid):
    os.mkdir(name, 0o700, dir_fd=parent)
    fd = os.open(name, os.O_RDONLY | os.O_DIRECTORY | os.O_NOFOLLOW | os.O_CLOEXEC, dir_fd=parent)
    os.fchown(fd, uid, gid)
    os.fchmod(fd, 0o700)
    return fd


def _copy_inventory(parent, root, entries, uid, gid):
    """Copy inert SSL/proxy inventory through held stage descriptors only."""
    descriptors = {'': os.dup(parent)}
    try:
        selected = [(name[len(root) + 1:], directory, data) for name, directory, data in entries
                    if name.startswith(root + '/')]
        directories = set()
        for name, directory, _ in selected:
            parts = name.split('/')
            directories.update('/'.join(parts[:n]) for n in range(1, len(parts)))
            if directory:
                directories.add(name)
        for name in sorted(directories, key=lambda v: (v.count('/'), v)):
            ancestor, _, leaf = name.rpartition('/')
            descriptors[name] = _directory(descriptors[ancestor], leaf, uid, gid)
        for name, directory, data in selected:
            if not directory:
                ancestor, _, leaf = name.rpartition('/')
                _write_leaf(descriptors[ancestor], leaf, data, uid, gid)
        for fd in descriptors.values():
            os.fsync(fd)
    finally:
        for fd in reversed(list(descriptors.values())):
            os.close(fd)


def prepare_phase(run, phase):
    _guard(run)
    _require(type(phase) is Phase and phase in (Phase.OLD, Phase.FRESH), 'stage_identity')
    if isinstance(run.docker._adapter, NativeProcess):
        run.docker._adapter._preparation()
    raw, material = _validated_original(run.context, run.docker)
    _require(_hash(raw) == run.original_sha and _inventory(material) == run.inventory, 'material_binding')
    expanded = sum(len(data) for _, _, data in material.recovery.entries)
    _guard(run, staged=expanded)
    source_env = next(data for name, _, data in material.recovery.entries if name == 'source/.env')
    env = clone_env(source_env)
    original_values, cloned_values = parse_env(source_env), parse_env(env)
    _require(all(cloned_values.get(key) == value for key, value in original_values.items()
                 if key in ('APP_KEY', 'APP_ID', 'APP_NAME', 'PUSHER_APP_ID', 'PUSHER_APP_KEY', 'PUSHER_APP_SECRET')),
             'env_policy')
    docker = run.docker
    record = docker._guard_run(run.handle, phase)
    fd = docker.preparation_fd(run.handle, phase)
    try:
        contents = {'runtime.env': env, 'hmac.key': _read(run.master['fd'], 32),
                    'coolify.dump': dict(material.files)['coolify.dump'],
                    'probe.php': _read(docker._sources['maintenance/coolify_probe.php'][1]),
                    'public-manifest.json': _read(docker._sources['maintenance/coolify-public-manifest.json'][1])}
        for name, content in contents.items():
            uid, gid = (0, 0) if name == 'coolify.dump' else record['identities']['old'][:2]
            _write_leaf(fd, name, content, uid, gid)
        for name in ('storage', 'bootstrap-cache', 'runtime-ssh', 'runtime-ssl', 'runtime-proxy', 'pg', 'redis'):
            role = 'postgres' if name == 'pg' else 'redis' if name == 'redis' else 'old'
            uid, gid = record['identities'][role][:2]
            child = _directory(fd, name, uid, gid)
            try:
                if name in ('runtime-ssl', 'runtime-proxy'):
                    _copy_inventory(child, name[8:], material.recovery.entries, uid, gid)
            finally:
                os.close(child)
        # Derive registration from actual validated bytes and the authenticated
        # master. Caller-created ValidatedMaterial never reaches this function.
        observed = os.open('hmac.key', os.O_RDONLY | os.O_NOFOLLOW | os.O_CLOEXEC, dir_fd=fd)
        try:
            info = os.fstat(observed)
            _require(stat.S_ISREG(info.st_mode) and info.st_nlink == 1
                     and (info.st_uid, info.st_gid) == (9999, 9999)
                     and stat.S_IMODE(info.st_mode) == 0o600
                     and hmac.compare_digest(_read(observed, 32), _read(run.master['fd'], 32)), 'hmac_identity')
        finally:
            os.close(observed)
        record['material'][phase] = {name: _hash(content) for name, content in contents.items()}
        os.fsync(fd)
    finally:
        os.close(fd)
    # Release expanded archive/env copies before lowering the native cap.
    del material, raw, env, contents, source_env, original_values, cloned_values
    resources = docker.seal_phase(run.handle, phase)
    run.resources[phase] = resources
    if isinstance(docker._adapter, NativeProcess):
        docker._adapter._steady()
    _guard(run)
    return resources


def _create_start(run, operation, phase):
    _guard(run)
    created = run.docker.create(run.handle, operation=operation, phase=phase,
                                resources=run.resources[phase])
    run.docker.start(created.container_id)
    return created.container_id


def _one_shot(run, operation, phase, reader):
    cid = _create_start(run, operation, phase)
    value = getattr(run.docker, reader)(cid)
    _guard(run)
    _remove_verified(run.docker, cid)
    return value


def _wait_postgres(run, cid):
    end = min(run.deadline, time.monotonic() + 900)
    while True:
        _guard(run)
        if run.docker.postgres_ready(cid):
            return
        _require(time.monotonic() + 0.1 < end, 'command_timeout')
        time.sleep(0.1)


def _held(run, operation, phase):
    cid = _create_start(run, operation, phase)
    end = min(run.deadline, time.monotonic() + 900)
    while not run.docker.held_ready(cid):
        _guard(run)
        _require(time.monotonic() + 0.1 < end, 'command_timeout')
        time.sleep(0.1)
    run.held[phase] = cid
    return cid


def restore_and_hold_original(run, phase):
    prepare_phase(run, phase)
    pg = _create_start(run, Operation.POSTGRES, phase)
    _wait_postgres(run, pg)
    redis = _create_start(run, Operation.REDIS, phase)
    run.services[phase] = (pg, redis)
    run.docker.restore_original(pg, run.resources[phase])
    raw = _one_shot(run, Operation.BASELINE_OLD, phase, 'read_probe')
    _one_shot(run, Operation.NETWORK_OLD, phase, 'read_probe')
    _one_shot(run, Operation.MIGRATE_OLD, phase, 'read_migration')
    _one_shot(run, Operation.SSH_OLD, phase, 'read_ssh_reconstruction')
    cid = _held(run, Operation.HELD_OLD, phase)
    old = _one_shot(run, Operation.VERIFY_OLD, phase, 'read_probe')
    _require(run.docker.held_ready(cid) is True, 'startup_state')
    return raw, old


def replace_app_and_migrate(run):
    _guard(run)
    phase = Phase.OLD
    services = run.services[phase]
    _remove_verified(run.docker, run.held.pop(phase))
    _guard(run)
    _one_shot(run, Operation.MIGRATE_TARGET, phase, 'read_migration')
    _one_shot(run, Operation.SSH_TARGET, phase, 'read_ssh_reconstruction')
    held = _held(run, Operation.HELD_TARGET, phase)
    _one_shot(run, Operation.NETWORK_TARGET, phase, 'read_probe')
    target = _one_shot(run, Operation.VERIFY_TARGET, phase, 'read_probe')
    _require(run.docker.held_ready(held) is True and run.services[phase] == services, 'startup_state')
    for cid in services:
        run.docker._authenticate_container(run.docker._record_for_id(cid))
    _guard(run)
    return target


def cleanup_phase(run, phase):
    with _cleaning(run.signals):
        _require(_begin_cleanup(run, phase=True), 'cleanup_incomplete')
        _guard(run)
        record = run.docker._guard_run(run.handle, phase)
        for container in list(record['intents'].values()):
            if container['phase'] is phase and container['state'] != 'removed':
                _require(time.monotonic() < run.cleanup_deadline, 'command_timeout')
                _require(container['id'] is not None, 'cleanup_incomplete')
                _remove_verified(run.docker, container['id'])
        _require(all(c['state'] == 'removed' for c in record['intents'].values()
                     if c['phase'] is phase) and time.monotonic() < run.cleanup_deadline,
                 'cleanup_incomplete')
        _check_signal(run)
        # A failed phase keeps this deadline for outer failure cleanup.
        run.cleanup_deadline = None
        if isinstance(run.docker._adapter, NativeProcess):
            run.docker._adapter._deadline = run.deadline


def cleanup_owned_success(run):
    with _cleaning(run.signals):
        _require(_begin_cleanup(run), 'cleanup_incomplete')
        _guard(run)
        _require(OwnedDocker.cleanup(run.docker, run.handle) is True, 'cleanup_incomplete')
        if isinstance(run.docker._adapter, NativeProcess):
            run.docker._adapter._finish()
        _require(_transport_complete(run.docker) and time.monotonic() < run.cleanup_deadline, 'cleanup_incomplete')
        _check_signal(run)


def cleanup_owned_failure(run):
    # Cleanup has its own bounded deadline; an expired whole deadline must not
    # block stopping exact journal-owned containers or permit any new phase.
    with _cleaning(run.signals):
        if not _begin_cleanup(run):
            return False
        try:
            complete = OwnedDocker.cleanup(run.docker, run.handle) is True
        except BaseException:
            complete = False
        if isinstance(run.docker._adapter, NativeProcess):
            try:
                run.docker._adapter._finish()
            except BaseException:
                complete = False
        return complete and _transport_complete(run.docker) and time.monotonic() < run.cleanup_deadline


def _begin_cleanup(run, *, phase=False):
    if run.cleanup_deadline is None:
        run.cleanup_deadline = time.monotonic() + 120
        if phase:
            run.cleanup_deadline = min(run.cleanup_deadline, run.deadline)
    if isinstance(run.docker._adapter, NativeProcess):
        run.docker._adapter._deadline = run.cleanup_deadline
    return time.monotonic() < run.cleanup_deadline


def _transport_complete(docker):
    return not isinstance(docker._adapter, NativeProcess) or docker._adapter._transport_clean is True


@_closed
def run_synthetic(context, *, docker):
    with _SignalScope() as signals:
        return _run_with_signals(context, docker, signals)


def _run_with_signals(context, docker, signals):
    require_synthetic_context(context, docker)
    require_verified_startup(context, docker)
    run = prepare_owned_run(context, docker, signals=signals)
    try:
        raw, old = restore_and_hold_original(run, Phase.OLD)
        _compare(raw, old, startup=True)
        target = replace_app_and_migrate(run)
        _compare(old, target, forward=True)
        old_ids = set(run.services[Phase.OLD])
        cleanup_phase(run, Phase.OLD)
        _, fresh = restore_and_hold_original(run, Phase.FRESH)
        _require(not old_ids.intersection(run.services[Phase.FRESH]), 'container_identity')
        _compare(old, fresh)
        cleanup_owned_success(run)
        return {'ok': True, 'scope': 'synthetic-only', 'category': 'synthetic_verified'}
    except BaseException as error:
        complete = cleanup_owned_failure(run)
        raise _failure(error, complete) from None


def _failure(error, complete):
    category = str(error) if isinstance(error, DockerError) else (
        'interrupted' if isinstance(error, (KeyboardInterrupt, SystemExit)) else 'command_failed')
    safe = DockerError(category)
    # Always overwrite incoming flags with our own complete owned cleanup result.
    safe.cleanup_complete = type(complete) is bool and complete
    return safe


def _remove_verified(docker, cid):
    try:
        docker.remove(cid)
    except DockerError:
        raise
    except (KeyboardInterrupt, SystemExit):
        raise DockerError('interrupted') from None
    except BaseException:
        raise DockerError('command_failed') from None


def _compare(before, after, **flags):
    try:
        compare_evidence(before, after, **flags)
    except RehearsalError:
        raise DockerError('evidence_difference') from None
