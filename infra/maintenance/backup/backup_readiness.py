#!/usr/bin/python3
"""Prove an exact encrypted offhost snapshot by retrieving and restoring it.

Activation requires an operator-approved root-only env, matching explicit root
backup_destination policy and initialized repository in the private backup bucket.
No operation enables database maintenance or guesses a destination.
"""
import argparse
from contextlib import contextmanager
import fcntl
import gzip
import hashlib
import json
import os
from pathlib import Path
import pwd
import re
import resource
import selectors
import signal
import shutil
import stat
import subprocess
import tarfile
import tempfile
import time

STATE = Path("/var/lib/sifto-maintenance")
CONFIG = Path("/etc/sifto-maintenance/config.json")
ENV_FILE = Path("/etc/sifto-maintenance/backup.env")
COMPOSE = Path("/opt/sifto-maintenance/backup/compose.yml")
DB_SOURCES = Path("/data/backups")
COOLIFY_SOURCE = Path("/data/coolify")
SOURCE_UID = None
SAFE_ENV = {"PATH": "/usr/sbin:/usr/bin:/sbin:/bin", "HOME": "/root"}
HEX = re.compile(r"[0-9a-f]{64}")
RESTORE_IMAGES = {
    "application": "docker.io/library/postgres:15.19-bookworm@sha256:d4a8e1f88f475ee3e0137fa89d21ebc59f6c6ab16bf369ee92907607cc3455ae",
    "database": "docker.io/library/postgres:16.13-bookworm@sha256:472efd9a66f2b2f1a5aeb18b28de74332e6ef88c2b93a1a5d812fb6db67a5f60",
}
REPOSITORY = re.compile(r"s3:https://[0-9a-f]{32}\.r2\.cloudflarestorage\.com/"
                        r"([a-z0-9][a-z0-9.-]{2,62})/([A-Za-z0-9][A-Za-z0-9_./-]{0,800})")


class BackupError(Exception):
    pass


def require(condition, message):
    if not condition:
        raise BackupError(message)


def protected(path, uid=0, private=False, directory=False):
    path = Path(path)
    require(path.is_absolute(), "absolute protected path required")
    for parent in reversed(path.parents):
        info = parent.lstat()
        require(stat.S_ISDIR(info.st_mode) and info.st_uid in (0, uid)
                and info.st_mode & 0o022 == 0, "unprotected path ancestor")
    info = path.lstat()
    require((stat.S_ISDIR(info.st_mode) if directory else stat.S_ISREG(info.st_mode))
            and info.st_uid == uid and info.st_mode & (0o077 if private else 0o022) == 0,
            "unprotected file or directory")
    return info


@contextmanager
def lock(path, uid=0):
    protected(Path(path).parent, uid=uid, directory=True)
    if uid != os.geteuid():
        require(Path(path).exists(), "producer must provision its own backup lock")
    fd = os.open(path, os.O_RDWR | os.O_NOFOLLOW | (os.O_CREAT if uid == os.geteuid() else 0), 0o600)
    try:
        info = os.fstat(fd)
        require(stat.S_ISREG(info.st_mode) and info.st_uid == uid and info.st_mode & 0o077 == 0,
                "unprotected lock")
        try:
            fcntl.flock(fd, fcntl.LOCK_EX | fcntl.LOCK_NB)
        except BlockingIOError as exc:
            raise BackupError("backup or maintenance lock already held") from exc
        yield
    finally:
        os.close(fd)


def parse_json(data):
    def pairs(items):
        result = {}
        for key, value in items:
            require(key not in result, "duplicate JSON field")
            result[key] = value
        return result
    try:
        return json.loads(data, object_pairs_hook=pairs)
    except (ValueError, UnicodeError) as exc:
        raise BackupError("invalid JSON") from exc


def load_config(role):
    protected(CONFIG, private=True)
    require(CONFIG.stat().st_size <= 1024 * 1024, "configuration too large")
    config = parse_json(CONFIG.read_bytes())
    require(type(config) is dict and config.get("host_role") == role
            and isinstance(config.get("host_id"), str)
            and re.fullmatch(r"[A-Za-z0-9_.-]{1,128}", config.get("host_id", ""))
            and type(config.get("database_enabled")) is bool, "host configuration mismatch")
    return config


def settings(role):
    require(os.geteuid() == 0 and role in ("database", "application"), "root and fixed host role required")
    protected(STATE, private=True, directory=True)
    protected(COMPOSE)
    config = load_config(role)
    destination = approved_destination(config, role)
    protected(ENV_FILE, private=True)
    require(ENV_FILE.stat().st_mode & 0o777 == 0o600 and ENV_FILE.stat().st_size <= 16384,
            "backup env must be root 0600 and bounded")
    raw = ENV_FILE.read_text()
    env = {}
    allowed = {"RESTIC_REPOSITORY", "RESTIC_PASSWORD", "AWS_ACCESS_KEY_ID", "AWS_SECRET_ACCESS_KEY",
               "AWS_DEFAULT_REGION", "BACKUP_DESTINATION_APPROVED", "BACKUP_HOST_ID"}
    for line in raw.splitlines():
        if not line or line.startswith("#"):
            continue
        key, sep, value = line.partition("=")
        require(sep and key in allowed and key not in env and value
                and re.fullmatch(r"[A-Za-z0-9_./:+@%=,-]+", value)
                and not re.search(r"REPLACE|PLACEHOLDER|CHANGEME", value, re.I), "invalid backup env")
        env[key] = value
    require(set(env) == allowed and env["BACKUP_DESTINATION_APPROVED"] == role
            and env["BACKUP_HOST_ID"] == config["host_id"] and env["AWS_DEFAULT_REGION"] == "auto",
            "destination has no matching explicit approval")
    match = REPOSITORY.fullmatch(env["RESTIC_REPOSITORY"])
    require(match and all(p not in ("", ".", "..") for p in match[2].split("/")), "invalid approved R2 destination")
    require("s3://" + match[1] + "/" + match[2] == destination, "R2 repository does not match root-approved backup destination")
    require(len(env["RESTIC_PASSWORD"]) >= 20, "independent backup password required")
    return config, raw, destination


def approved_destination(config, role):
    destination = config.get("backup_destination")
    # Podcast objects have separate public delivery/retention policies. Even a
    # root-approved env must never repurpose that shared bucket for recovery.
    require(type(destination) is dict and set(destination) == {"bucket", "prefix"}
            and destination == {"bucket": "sifto-backups-prod", "prefix": role}
            and role in ("application", "database"), "explicit dedicated backup destination approval required")
    return "s3://" + destination["bucket"] + "/" + destination["prefix"]


def run(command, *, env=None, timeout=6 * 3600, limit=2 * 1024 * 1024):
    # RLIMIT_FSIZE also caps producer dump/archive files. Bound only the output
    # pipe, while retaining the fixed timeout and cleaning up this child group.
    process = None
    deadline = time.monotonic() + timeout
    try:
        process = subprocess.Popen(command, env=env or SAFE_ENV, stdin=subprocess.DEVNULL,
                                   stdout=subprocess.PIPE, stderr=subprocess.STDOUT, start_new_session=True)
        output = bytearray()
        os.set_blocking(process.stdout.fileno(), False)
        with selectors.DefaultSelector() as reader:
            reader.register(process.stdout, selectors.EVENT_READ)
            while True:
                remaining = deadline - time.monotonic()
                require(remaining > 0 and reader.select(remaining), "backup subprocess timed out")
                block = os.read(process.stdout.fileno(), min(65536, limit - len(output) + 1))
                if not block:
                    break
                output.extend(block)
                require(len(output) <= limit, "backup subprocess output limit exceeded")
        require(process.wait(timeout=max(0, deadline - time.monotonic())) == 0, "backup subprocess failed")
        return output.decode("utf-8", errors="strict")
    except (subprocess.TimeoutExpired, OSError, UnicodeError):
        # Do not attach captured output or inherited credentials to errors.
        raise BackupError("backup subprocess failed or timed out") from None
    finally:
        if process is not None:
            # Session isolation limits termination to our own command family.
            # Kill surviving descendants even when their parent already exited.
            try:
                os.killpg(process.pid, signal.SIGKILL)
            except ProcessLookupError:
                pass
            process.stdout.close()
            try:
                process.wait(timeout=5)
            except subprocess.TimeoutExpired:
                raise BackupError("backup subprocess cleanup timed out") from None


def compose(service, arguments):
    return ["/usr/bin/docker", "compose", "--env-file", "/dev/null", "--project-name",
            "sifto-maintenance-backup", "--file", str(COMPOSE), "run", "--rm", "-T",
            "--no-deps", service, *arguments]


def private_env(work, raw):
    path = work / "backup.env"
    path.write_text(raw)
    path.chmod(0o600)
    return {**SAFE_ENV, "MAINTENANCE_BACKUP_ENV": str(path)}


def initialize(role, create=False):
    _, raw, _ = settings(role)
    with lock(STATE / "backup.lock"), tempfile.TemporaryDirectory(prefix="offhost-init.", dir=STATE) as directory:
        env = private_env(Path(directory), raw)
        try:
            run(compose("repository", ["snapshots", "--json"]), env=env)
        except BackupError:
            require(create, "repository check failed; explicit --create required for a new repository")
            # Explicit --create only; restic refuses an existing repository. Never unlock.
            run(compose("repository", ["init"]), env=env)


def digest(path):
    value = hashlib.sha256()
    with Path(path).open("rb") as source:
        for block in iter(lambda: source.read(1024 * 1024), b""):
            value.update(block)
    return value.hexdigest()


def fresh(path, uid):
    info = protected(path, uid=uid)
    require(info.st_size > 0 and 0 <= time.time() - info.st_mtime < 86400, "backup source expired or empty")


def source_identity(info):
    # Reads can update atime. All attributes identifying content/ownership must
    # stay stable, including nanosecond modification and metadata-change times.
    return (info.st_dev, info.st_ino, info.st_mode, info.st_uid, info.st_gid,
            info.st_nlink, info.st_size, info.st_mtime_ns, info.st_ctime_ns)


def stage(role, source):
    if role == "database":
        uid = SOURCE_UID if SOURCE_UID is not None else pwd.getpwnam("postgres").pw_uid
        protected(DB_SOURCES, uid=uid, directory=True)
        paths = [p for p in DB_SOURCES.iterdir() if re.fullmatch(r"sifto-[0-9]{8}-[0-9]{4,6}\.dump\.gz", p.name)]
        require(paths, "no completed database backup")
        paths.sort(key=lambda p: p.lstat().st_mtime, reverse=True)
        paths = paths[:1]
    else:
        uid = 0
        directory = STATE / "backups"
        protected(directory, private=True, directory=True)
        paths = [directory / "coolify.dump", directory / "coolify-config.tar.gz", directory / "created_at"]
        created = directory / "created_at"
        fresh(created, 0)
        require(re.fullmatch(r"[0-9]{10}\n?", created.read_text()), "invalid completed Coolify backup")
        require(0 <= time.time() - int(created.read_text()) < 86400, "Coolify backup expired")
    hashes = {}
    for path in paths:
        fresh(path, uid)
        before = path.stat()
        target = source / path.name
        shutil.copy2(path, target)
        target.chmod(0o600)
        require(digest(path) == digest(target) and source_identity(path.stat()) == source_identity(before),
                "backup source changed during staging")
        hashes[path.name] = digest(target)
    return hashes


def snapshot_id(output):
    summaries = []
    for line in output.splitlines():
        if not line.startswith("{"):
            continue
        message = parse_json(line)
        require(type(message) is dict, "invalid backup JSON message")
        if message.get("message_type") == "summary":
            summaries.append(message.get("snapshot_id"))
    require(len(summaries) == 1 and isinstance(summaries[0], str) and HEX.fullmatch(summaries[0]),
            "successful backup has no exact snapshot ID")
    return summaries[0]


def verify_retrieval(retrieved, hashes):
    require({p.name for p in retrieved.iterdir()} == {"source"}, "unexpected restored root")
    directory = retrieved / "source"
    protected(directory, directory=True)
    require({p.name for p in directory.iterdir()} == set(hashes), "retrieved snapshot contents differ")
    for name, expected in hashes.items():
        path = directory / name
        protected(path)
        require(digest(path) == expected, "retrieved snapshot hash mismatch")
    return directory


def verify_config_archive(path):
    # Inspect names/types without extracting or executing retrieved configuration.
    try:
        with tarfile.open(path, "r:gz") as archive:
            count = 0
            roots = set()
            for member in archive:
                count += 1
                parts = member.name.rstrip("/").split("/")
                require(count <= 100000 and parts[0] in ("source", "ssh", "ssl", "proxy")
                        and all(p not in ("", ".", "..") for p in parts)
                        and (member.isfile() or member.isdir()), "unsafe Coolify configuration archive")
                if len(parts) == 1 and member.isdir():
                    roots.add(parts[0])
            require(roots == {"source", "ssh", "ssl", "proxy"}, "incomplete Coolify recovery archive")
        with gzip.open(path, "rb") as stream:
            while stream.read(1024 * 1024):
                pass
    except (tarfile.TarError, OSError, EOFError) as exc:
        raise BackupError("invalid Coolify configuration archive") from exc


def restore_archive(role, dump, command, *, env=None):
    """Fully validate/decompress retrieved bytes and require full pg_restore success."""
    with tempfile.TemporaryFile(dir=STATE) as archive:
        if role == "database":
            try:
                with gzip.open(dump, "rb") as compressed:
                    shutil.copyfileobj(compressed, archive, 1024 * 1024)
            except (OSError, EOFError) as exc:
                raise BackupError("invalid retrieved compressed dump") from exc
        else:
            with Path(dump).open("rb") as source:
                shutil.copyfileobj(source, archive, 1024 * 1024)
        archive.seek(0)
        with tempfile.TemporaryFile(dir=STATE) as output:
            result = subprocess.run(command, env=env or SAFE_ENV, stdin=archive, stdout=output, stderr=output,
                                    timeout=6 * 3600, preexec_fn=lambda: resource.setrlimit(resource.RLIMIT_FSIZE,
                                                                                       (2 * 1024 * 1024,) * 2))
            require(result.returncode == 0, "full retrieved database restore failed")


def ensure_restore_image(role):
    require(role in RESTORE_IMAGES, "fixed restore host role required")
    reference = RESTORE_IMAGES[role]
    inspect = ["/usr/bin/docker", "image", "inspect", reference, "--format", "{{.Id}}"]
    try:
        image = run(inspect, timeout=60).strip()
    except BackupError:
        # Recover a pruned cache once, using only the previously verified index.
        run(["/usr/bin/docker", "pull", reference], timeout=600, limit=2 * 1024 * 1024)
        image = run(inspect, timeout=60).strip()
    # Docker's opaque ID can be a config or index digest depending on its store.
    require(re.fullmatch(r"sha256:[0-9a-f]{64}", image), "invalid fixed PostgreSQL image ID")
    return reference, image


def restore_database(role, dump, work):
    reference, image = ensure_restore_image(role)
    memory = "768m" if role == "database" else "256m"
    name = "sifto-offhost-restore-" + role
    # Never remove a pre-existing container; only successful creation owns cleanup.
    run(["/usr/bin/docker", "run", "--pull=never", "-d", "--name", name, "--network", "none", "--memory", memory,
         "--cpus", "1", "-e", "POSTGRES_DB=offhost_restore", "-e", "POSTGRES_HOST_AUTH_METHOD=trust",
         "--mount", "type=bind,src=" + str(work) + ",dst=/var/lib/postgresql/data", reference], timeout=60)
    try:
        sql = ["/usr/bin/docker", "exec", name, "psql", "-X", "-v", "ON_ERROR_STOP=1", "-U", "postgres",
               "-d", "offhost_restore", "-Atc", "SELECT 1"]
        for attempt in range(60):
            try:
                require(run(sql, timeout=30).strip() == "1", "restore database is not ready")
                break
            except BackupError:
                require(attempt < 59, "isolated database readiness timed out")
                time.sleep(1)
        restore_archive(role, dump, ["/usr/bin/docker", "exec", "-i", name, "pg_restore", "--exit-on-error",
                                     "--no-owner", "--no-privileges", "--username=postgres", "--dbname=offhost_restore"])
        require(run(sql, timeout=30).strip() == "1", "restored database query failed")
        return image
    finally:
        run(["/usr/bin/docker", "rm", "-f", name], timeout=60)


def atomic_json(path, value, *, rollback_on_commit_failure=False):
    if path.exists() or path.is_symlink():
        protected(path, private=True)
    protected(path.parent, private=True, directory=True)
    temporary = rollback = None
    retain_rollback = False
    parent_fd = os.open(path.parent, os.O_RDONLY | os.O_DIRECTORY | os.O_NOFOLLOW)
    try:
        if rollback_on_commit_failure:
            # Publication holds maintenance.lock. Durably prepare the previous
            # complete config before replacement, so recovery needs no allocation.
            old_fd, rollback = tempfile.mkstemp(prefix=".rollback.", dir=path.parent)
            with os.fdopen(old_fd, "wb") as stream:
                stream.write(path.read_bytes())
                stream.flush()
                os.fsync(stream.fileno())
            os.fsync(parent_fd)
        fd, temporary = tempfile.mkstemp(prefix=".proof.", dir=path.parent)
        with os.fdopen(fd, "w") as stream:
            json.dump(value, stream, sort_keys=True, separators=(",", ":"))
            stream.write("\n")
            stream.flush()
            os.fsync(stream.fileno())
        os.replace(temporary, path)
        temporary = None
        try:
            os.fsync(parent_fd)
        except OSError as exc:
            if rollback is None:
                raise
            try:
                os.replace(rollback, path)
                rollback = None
            except OSError:
                # An allocation-free rename should normally restore the old
                # config. If the directory cannot be changed, invalidate the
                # existing config in place: native load_config then denies all
                # maintenance. Retain the private recovery file for an operator.
                retain_rollback = True
                try:
                    disabled_fd = os.open(path, os.O_WRONLY | os.O_TRUNC | os.O_NOFOLLOW)
                    try:
                        os.fsync(disabled_fd)
                    finally:
                        os.close(disabled_fd)
                except OSError:
                    raise BackupError("configuration recovery could not be confirmed; private rollback file retained; operator recovery required") from exc
                raise BackupError("configuration rollback failed; readiness disabled; private rollback file retained for recovery") from exc
            try:
                os.fsync(parent_fd)
            except OSError:
                raise BackupError("previous configuration restored; directory durability unconfirmed; operator recovery required") from exc
            raise BackupError("configuration commit failed; previous readiness restored") from exc
    finally:
        os.close(parent_fd)
        # Housekeeping cannot turn an already durable successful commit into a
        # reported failure with fresh readiness. All leftovers are root 0600.
        for leftover in (temporary, None if retain_rollback else rollback):
            if leftover is not None:
                try:
                    os.unlink(leftover)
                except OSError:
                    pass


def publish(role, host_id, proof):
    with lock(STATE / "maintenance.lock"):
        config = load_config(role)
        require(config["host_id"] == host_id, "host changed during proof generation")
        require(proof["offhost_object"] == approved_destination(config, role) + "/snapshots/" + proof["backup_id"],
                "backup destination approval changed during proof generation")
        evidence = STATE / "evidence"
        if not evidence.exists():
            evidence.mkdir(mode=0o700)
        protected(evidence, private=True, directory=True)
        atomic_json(evidence / ("offhost-" + proof["backup_id"] + ".json"), proof)
        config["backup_readiness"] = proof
        atomic_json(CONFIG, config, rollback_on_commit_failure=True)


def backup(role):
    config, raw, object_prefix = settings(role)
    with lock(STATE / "backup.lock"):
        if role == "application":
            protected(COOLIFY_SOURCE.parent, directory=True)
            uid = COOLIFY_SOURCE.lstat().st_uid
            require(uid in (0, 9999), "unexpected Coolify source owner")
            protected(COOLIFY_SOURCE, uid=uid, directory=True)
            for name in ("source", "ssh", "ssl", "proxy"):
                protected(COOLIFY_SOURCE / name, uid=uid, directory=True)
            run(["/bin/bash", "/opt/sifto-maintenance/backup/coolify-backup.sh"])
        uid = SOURCE_UID if SOURCE_UID is not None else (pwd.getpwnam("postgres").pw_uid if role == "database" else 0)
        source_lock = DB_SOURCES / ".sifto-backup.lock" if role == "database" else STATE / "coolify-backup.lock"
        with lock(source_lock, uid=uid), tempfile.TemporaryDirectory(prefix="offhost-proof.", dir=STATE) as directory:
            work = Path(directory)
            source, retrieved, pgdata = [work / name for name in ("source", "retrieved", "pgdata")]
            for path in (source, retrieved, pgdata):
                path.mkdir(mode=0o700)
            hashes = stage(role, source)
            if role == "application":
                verify_config_archive(source / "coolify-config.tar.gz")
            env = {**private_env(work, raw), "MAINTENANCE_BACKUP_SOURCE": str(source),
                   "MAINTENANCE_RESTORE_TARGET": str(retrieved)}
            identity = "sifto-" + role
            snapshot = snapshot_id(run(compose("backup", ["backup", "/source", "--host", identity,
                                                          "--tag", identity, "--json"]), env=env))
            run(compose("repository", ["check", "--read-data"]), env=env)
            run(compose("retrieve", ["restore", snapshot, "--target", "/restore"]), env=env)
            restored = verify_retrieval(retrieved, hashes)
            if role == "application":
                verify_config_archive(restored / "coolify-config.tar.gz")
            dump = restored / (next(iter(hashes)) if role == "database" else "coolify.dump")
            image = restore_database(role, dump, pgdata)
            require(all(digest(restored / name) == value for name, value in hashes.items()), "retrieved bytes changed during restore")
            for name in hashes:
                fresh(source / name, 0)
            now = int(time.time())
            proof = {"backup_id": snapshot, "host_id": config["host_id"], "verified_at": now,
                     "restore_tested_at": now, "offhost_object": object_prefix + "/snapshots/" + snapshot,
                     "offhost_sha256": hashes[dump.name], "offhost_verified_at": now,
                     "retrieved_file_sha256": hashes, "image_id": image, "offhost_verified": True,
                     "test_location": "retrieved-snapshot-network-none",
                     "coolify_config_operator_restore_required": role == "application"}
            run(compose("repository", ["forget", "--host", identity, "--tag", identity, "--group-by", "host,tags",
                                       "--keep-within", "14d", "--keep-daily", "14", "--prune"]), env=env)
            publish(role, config["host_id"], proof)


def main():
    os.umask(0o077)
    parser = argparse.ArgumentParser()
    parser.add_argument("operation", choices=("backup", "init"))
    parser.add_argument("role", choices=("database", "application"))
    parser.add_argument("--create", action="store_true")
    args = parser.parse_args()
    try:
        require(not args.create or args.operation == "init", "create applies only to init")
        initialize(args.role, args.create) if args.operation == "init" else backup(args.role)
        print("backup operation verified; maintenance enablement is unchanged")
    except (BackupError, OSError, ValueError, subprocess.SubprocessError) as exc:
        print("backup proof failed: " + (str(exc) if isinstance(exc, BackupError) else type(exc).__name__),
              file=__import__("sys").stderr)
        raise SystemExit(1)


if __name__ == "__main__":
    main()
