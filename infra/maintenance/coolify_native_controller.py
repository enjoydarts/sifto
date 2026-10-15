#!/usr/bin/python3 -I
"""Fixed installed native entrypoint; all local code is verified before import."""
import sys
sys.dont_write_bytecode = True
sys.pycache_prefix = "/dev/null"

import hashlib
import json
import os
from pathlib import Path
import re
import stat

_SOURCES = ("host_agent.py", "coolify_native_controller.py", "coolify_native_engine.py",
    "maintenance/__init__.py", "maintenance/plan.py", "maintenance/host.py",
    "maintenance/coolify_host.py", "maintenance/coolify_plan.py",
    "maintenance/coolify_rehearsal.py", "maintenance/coolify_evidence.py",
    "maintenance/coolify_docker.py", "maintenance/coolify_rehearsal_engine.py",
    "maintenance/coolify_systemd.py", "maintenance/coolify_native_protocol.py",
    "maintenance/coolify_cleanup.py", "maintenance/coolify_native_owner.py",
    "maintenance/coolify-public-manifest.json", "maintenance/coolify_probe.php",
    "maintenance/coolify_startup.py", "maintenance/coolify_startup_init",
    "maintenance/coolify_startup_middleware.php", "maintenance/coolify_startup_nginx.conf",
    "maintenance/coolify_startup_site.conf",
    "systemd/sifto-coolify-native-controller.service")
_ROOT = Path("/opt/sifto-maintenance")
_MANIFEST = Path("/etc/sifto-maintenance/coolify-native-sources.json")


def _metadata(info):
    return (info.st_dev, info.st_ino, info.st_mode, info.st_uid, info.st_gid,
            info.st_nlink, info.st_size, info.st_mtime_ns, info.st_ctime_ns)


def _open_source(path, mode):
    for ancestor in path.parents:
        info = ancestor.lstat()
        if not stat.S_ISDIR(info.st_mode) or info.st_uid != 0 or info.st_mode & 0o022:
            raise ValueError()
    fd = os.open(path, os.O_RDONLY | os.O_NOFOLLOW | os.O_CLOEXEC | os.O_NONBLOCK)
    try:
        info = os.fstat(fd)
        if (not stat.S_ISREG(info.st_mode) or info.st_uid != 0 or info.st_gid != 0
                or stat.S_IMODE(info.st_mode) != mode or info.st_nlink != 1
                or not 0 < info.st_size <= 2 * 1024**2):
            raise ValueError()
        data = os.pread(fd, info.st_size + 1, 0)
        after = os.fstat(fd)
        if (len(data) != info.st_size or _metadata(info) != _metadata(after)
                or _metadata(path.lstat()) != _metadata(info)):
            raise ValueError()
        return data
    finally:
        os.close(fd)


def validate_installed_sources():
    if not sys.flags.isolated or os.geteuid() != 0 or len(sys.argv) != 1:
        raise ValueError()
    if Path(__file__).absolute() != _ROOT / _ENTRY:
        raise ValueError()
    def pairs(items):
        result = {}
        for key, value in items:
            if key in result:
                raise ValueError()
            result[key] = value
        return result
    manifest = json.loads(_open_source(_MANIFEST, 0o600), object_pairs_hook=pairs)
    if (type(manifest) is not dict or set(manifest) != {"version", "sources"}
            or type(manifest["version"]) is not int or manifest["version"] != 1
            or type(manifest["sources"]) is not dict or set(manifest["sources"]) != set(_SOURCES)):
        raise ValueError()
    closure = []
    for name in _SOURCES:
        digest = manifest["sources"][name]
        if type(digest) is not str or re.fullmatch("[a-f0-9]{64}", digest) is None:
            raise ValueError()
        path = _ROOT / name
        data = _open_source(path, 0o644)
        if hashlib.sha256(data).hexdigest() != digest:
            raise ValueError()
        closure.append((path, digest))
    return tuple(closure)


_ENTRY = "coolify_native_controller.py"
try:
    _closure = validate_installed_sources()
except BaseException:
    sys.stderr.write("native bootstrap rejected\n")
    raise SystemExit(1)

sys.path.insert(0, str(_ROOT))
try:
    from maintenance.coolify_native_owner import run_controller
    _result = run_controller(_closure)
except BaseException:
    sys.stderr.write("native request rejected\n")
    raise SystemExit(1)
raise SystemExit(0 if _result is True else 1)
