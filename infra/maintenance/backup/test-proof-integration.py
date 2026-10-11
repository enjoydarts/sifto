"""Real encrypted local Restic repository and PostgreSQL; no remote transport."""
import gzip
import importlib.util
import io
import json
import os
from pathlib import Path
import shutil
import subprocess
import tarfile
import time
from unittest.mock import patch

spec = importlib.util.spec_from_file_location("proof", "/fixture/backup_readiness.py")
proof = importlib.util.module_from_spec(spec)
spec.loader.exec_module(proof)
state = Path("/var/lib/sifto-maintenance")
state.mkdir(mode=0o700, parents=True, exist_ok=True)
os.chmod(state, 0o700)
proof.STATE = state
proof.CONFIG = state / "config.json"
proof.ENV_FILE = state / "approved.env"
# Preserve production's root-only deployment checks on Linux UID1001 checkout
# mounts too. External transport below uses the real encrypted local repository.
proof.COMPOSE = state / "compose.yml"
proof.COMPOSE.write_bytes(Path("/fixture/compose.yml").read_bytes())
proof.COMPOSE.chmod(0o600)
proof.DB_SOURCES = state / "native-dumps"
proof.DB_SOURCES.mkdir(mode=0o700)
proof.SOURCE_UID = 0
proof.COOLIFY_SOURCE = state / "synthetic-coolify-source"
proof.COOLIFY_SOURCE.mkdir(mode=0o700)
for name in ("source", "ssh", "ssl", "proxy"):
    (proof.COOLIFY_SOURCE / name).mkdir(mode=0o700)
proof.CONFIG.write_text(json.dumps({"host_id": "synthetic", "host_role": "database", "database_enabled": False,
                                    "backup_destination": {"bucket": "sifto-backups-prod", "prefix": "database"},
                                    "allowed_approvers": [], "operator_field": "preserved"}))
proof.CONFIG.chmod(0o600)
proof.ENV_FILE.write_text("RESTIC_REPOSITORY=s3:https://" + "a" * 32 + ".r2.cloudflarestorage.com/sifto-backups-prod/database\n"
                          "RESTIC_PASSWORD=synthetic-independent-password\nAWS_ACCESS_KEY_ID=synthetic\nAWS_SECRET_ACCESS_KEY=synthetic\n"
                          "AWS_DEFAULT_REGION=auto\nBACKUP_DESTINATION_APPROVED=database\nBACKUP_HOST_ID=synthetic\n")
proof.ENV_FILE.chmod(0o600)
local_env = {**proof.SAFE_ENV, "RESTIC_REPOSITORY": "/tmp/encrypted-repo", "RESTIC_PASSWORD": "synthetic-independent-password",
             "RESTIC_CACHE_DIR": "/tmp/restic-cache"}
sql_env = {**proof.SAFE_ENV, "PGHOST": "/tmp/pg-socket", "PGUSER": "postgres"}
pg = "/usr/lib/postgresql/" + os.environ["PG_MAJOR"] + "/bin/"


def sql(database, query):
    return subprocess.check_output([pg + "psql", "-X", "-v", "ON_ERROR_STOP=1", "-d", database, "-Atc", query], env=sql_env).decode().strip()


sql("postgres", "CREATE DATABASE synthetic_source")
sql("synthetic_source", "CREATE TABLE recovery_probe(id int primary key, payload text); INSERT INTO recovery_probe SELECT n, repeat(md5(n::text), 100) FROM generate_series(1, 10000) n")
archive = subprocess.check_output([pg + "pg_dump", "--format=custom", "--dbname=synthetic_source"], env=sql_env)
dump = proof.DB_SOURCES / "sifto-20261011-030000.dump.gz"
dump.write_bytes(gzip.compress(archive))
dump.chmod(0o600)
real_run = proof.run
real_run(["/usr/bin/restic", "init"], env=local_env)
retrieved_hashes = []


def local_transport(command, **kwargs):
    # Substitute only external Docker/storage transport. Restic itself is real.
    if command[0] == "/bin/bash":
        return ""  # Synthetic completed Coolify material is prepared below.
    service_index = command.index("--no-deps") + 1
    service = command[service_index]
    args = command[service_index + 1:]
    if service == "backup":
        for path in Path("/source").iterdir():
            path.unlink()
        shutil.copytree(kwargs["env"]["MAINTENANCE_BACKUP_SOURCE"], "/source", dirs_exist_ok=True)
    if service == "retrieve":
        args[args.index("--target") + 1] = kwargs["env"]["MAINTENANCE_RESTORE_TARGET"]
    return real_run(["/usr/bin/restic", *args], env=local_env)


def actual_restore(role, retrieved_dump, work):
    sql("postgres", "DROP DATABASE IF EXISTS offhost_restore")
    sql("postgres", "CREATE DATABASE offhost_restore")
    retrieved_hashes.append(proof.digest(retrieved_dump))
    # The same production stream/exit-on-error implementation, targeting this
    # disposable Compose container's PostgreSQL without mounting a Docker socket.
    proof.restore_archive(role, retrieved_dump,
                          [pg + "pg_restore", "--exit-on-error", "--no-owner", "--no-privileges", "--dbname=offhost_restore"],
                          env=sql_env)
    assert sql("offhost_restore", "SELECT count(*) FROM recovery_probe") == "10000"
    assert sql("offhost_restore", "SELECT payload FROM recovery_probe WHERE id=9999") == sql("synthetic_source", "SELECT payload FROM recovery_probe WHERE id=9999")
    return "sha256:" + "c" * 64


# Prove real retrieval orchestration and native compressed DB archive restoration.
with patch.object(proof, "run", local_transport), patch.object(proof, "restore_database", actual_restore):
    proof.backup("database")
updated = json.loads(proof.CONFIG.read_text())
assert updated["database_enabled"] is False and updated["allowed_approvers"] == []
assert updated["backup_readiness"]["offhost_sha256"] == retrieved_hashes[0] == proof.digest(dump)
assert updated["operator_field"] == "preserved"
assert updated["backup_destination"] == {"bucket": "sifto-backups-prod", "prefix": "database"}
snapshot = updated["backup_readiness"]["backup_id"]
assert len(snapshot) == 64

# Prove the application flow from real retrieved dump + configuration archive.
proof.CONFIG.write_text(json.dumps({"host_id": "synthetic", "host_role": "application", "database_enabled": False,
                                    "backup_destination": {"bucket": "sifto-backups-prod", "prefix": "application"},
                                    "allowed_approvers": [], "operator_field": "preserved"}))
proof.ENV_FILE.write_text(proof.ENV_FILE.read_text().replace("/database", "/application").replace("APPROVED=database", "APPROVED=application"))
backups = state / "backups"
backups.mkdir(mode=0o700)
(backups / "coolify.dump").write_bytes(archive)
(backups / "created_at").write_text(str(int(time.time())) + "\n")
with tarfile.open(backups / "coolify-config.tar.gz", "w:gz") as tar:
    for name in ("source", "ssh", "ssl", "proxy"):
        member = tarfile.TarInfo(name)
        member.type = tarfile.DIRTYPE
        tar.addfile(member)
    member = tarfile.TarInfo("source/synthetic.env")
    content = b"SYNTHETIC_ONLY=true\n"
    member.size = len(content)
    tar.addfile(member, io.BytesIO(content))
for path in backups.iterdir():
    path.chmod(0o600)
with patch.object(proof, "run", local_transport), patch.object(proof, "restore_database", actual_restore):
    proof.backup("application")
app = json.loads(proof.CONFIG.read_text())
assert app["database_enabled"] is False and app["allowed_approvers"] == []
assert app["backup_destination"] == {"bucket": "sifto-backups-prod", "prefix": "application"}
assert set(app["backup_readiness"]["retrieved_file_sha256"]) == {"coolify.dump", "coolify-config.tar.gz", "created_at"}
assert app["backup_readiness"]["coolify_config_operator_restore_required"] is True

# Coolify uses a raw custom archive. Exercise real raw-archive restore as well.
raw = state / "coolify.dump"
raw.write_bytes(archive)
sql("postgres", "CREATE DATABASE coolify_restore")
proof.restore_archive("application", raw,
                      [pg + "pg_restore", "--exit-on-error", "--no-owner", "--no-privileges", "--dbname=coolify_restore"], env=sql_env)
assert sql("coolify_restore", "SELECT count(*) FROM recovery_probe") == "10000"
broken = state / "broken.dump"
broken.write_bytes(b"this is not a PostgreSQL archive")
try:
    proof.restore_archive("application", broken,
                          [pg + "pg_restore", "--exit-on-error", "--no-owner", "--no-privileges", "--dbname=coolify_restore"], env=sql_env)
except proof.BackupError:
    pass
else:
    raise AssertionError("invalid archive accepted")
print("real encrypted exact-snapshot retrieval + full compressed/raw PG restore + invalid archive rejection verified; PG " + os.environ["PG_MAJOR"])
