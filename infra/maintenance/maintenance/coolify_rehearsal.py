"""Pure bounded validation of private Coolify recovery material.

This module never runs images, scripts, shells, network requests or DB restores.
RootBinding must come from trusted root policy, not an exported proof or CLI.
Validation/extraction alone is not evidence that the control plane recovered.
Private bytes and exceptions have deliberately uninformative representations.
"""
import base64
import binascii
from dataclasses import dataclass, field
import hashlib
import json
import os
from pathlib import Path
import re
import stat
import tarfile
import time
import zlib

MAX_INPUT = 160 * 1024 * 1024
MAX_FILE = 64 * 1024 * 1024
MAX_EXPANDED = 256 * 1024 * 1024
MAX_MEMBERS = 10000
MAX_DIRECTORIES = 10000
ROOTS = frozenset(('source', 'ssh', 'ssl', 'proxy'))
FILES = frozenset(('coolify.dump', 'coolify-config.tar.gz', 'created_at'))
_HASH = re.compile(r'[a-f0-9]{64}\Z')
_PROOF_FIELDS = frozenset((
    'backup_id', 'host_id', 'verified_at', 'offhost_verified_at', 'restore_tested_at',
    'offhost_object', 'offhost_sha256', 'retrieved_file_sha256', 'image_id',
    'offhost_verified', 'test_location', 'coolify_config_operator_restore_required'))


class RehearsalError(Exception):
    """Static category only; no paths, JSON, credentials or OS error messages."""


def _require(condition, category):
    if not condition:
        raise RehearsalError(category)


@dataclass(frozen=True, repr=False)
class RootBinding:
    snapshot_id: str
    host_id: str
    file_sha256: tuple
    proof_sha256: str


@dataclass(frozen=True, repr=False)
class FilePolicy:
    root: Path
    owner_uid: int


@dataclass(frozen=True, repr=False)
class ValidatedArchive:
    entries: tuple = field(repr=False)
    recovery: bool


@dataclass(frozen=True, repr=False)
class ValidatedMaterial:
    snapshot_id: str
    files: tuple = field(repr=False)
    recovery: ValidatedArchive = field(repr=False)


def _metadata(value):
    # Reads can change atime without indicating an input change.
    return (value.st_dev, value.st_ino, value.st_uid, value.st_gid, value.st_size,
            value.st_mode, value.st_nlink, value.st_mtime_ns, value.st_ctime_ns)


def _protected_parent(path, policy):
    """Open all ancestors by directory descriptors; callers close every fd."""
    descriptors = []
    try:
        _require(type(policy) is FilePolicy and type(policy.owner_uid) is int
                 and policy.owner_uid >= 0, 'file_policy')
        root, path = Path(policy.root), Path(path)
        _require(root.is_absolute() and path.is_absolute()
                 and all(part not in ('.', '..') for part in path.parts + root.parts)
                 and path != root and path.is_relative_to(root), 'file_policy')
        root_parts = root.parts[1:]
        parts = path.parts[1:]
        fd = os.open('/', os.O_RDONLY | os.O_DIRECTORY | os.O_NOFOLLOW)
        descriptors.append((fd, _metadata(os.fstat(fd))))
        for index, name in enumerate(parts[:-1]):
            fd = os.open(name, os.O_RDONLY | os.O_DIRECTORY | os.O_NOFOLLOW,
                         dir_fd=fd)
            info = os.fstat(fd)
            descriptors.append((fd, _metadata(info)))
            # Before the protected root, root-owned sticky /tmp is acceptable.
            inside = index + 1 >= len(root_parts)
            _require(stat.S_ISDIR(info.st_mode) and info.st_uid ==
                     (policy.owner_uid if inside else 0), 'file_permissions')
            writable = stat.S_IMODE(info.st_mode) & 0o022
            _require(not writable or (not inside and info.st_uid == 0
                     and bool(info.st_mode & stat.S_ISVTX)), 'file_permissions')
        return fd, parts[-1], descriptors
    except RehearsalError:
        for selected, _ in reversed(descriptors):
            os.close(selected)
        raise
    except (OSError, ValueError, TypeError):
        for selected, _ in reversed(descriptors):
            os.close(selected)
        raise RehearsalError('file_invalid') from None


def _close(descriptors):
    for fd, _ in reversed(descriptors):
        os.close(fd)


def _unchanged(descriptors):
    _require(all(_metadata(os.fstat(fd)) == old for fd, old in descriptors), 'file_changed')


def read_regular(path, *, policy, maximum=MAX_INPUT):
    """Read a private owned file once, with a full stable metadata closure."""
    _require(type(maximum) is int and 0 < maximum <= MAX_INPUT, 'file_size')
    parent, name, descriptors = _protected_parent(path, policy)
    fd = None
    try:
        fd = os.open(name, os.O_RDONLY | os.O_NOFOLLOW | os.O_NONBLOCK, dir_fd=parent)
        before = os.fstat(fd)
        _require(stat.S_ISREG(before.st_mode) and before.st_nlink == 1
                 and before.st_uid == policy.owner_uid
                 and stat.S_IMODE(before.st_mode) == 0o600, 'file_permissions')
        _require(0 < before.st_size <= maximum, 'file_size')
        parts, remaining = [], before.st_size + 1
        while remaining:
            data = os.read(fd, min(remaining, 1024 * 1024))
            if not data:
                break
            parts.append(data)
            remaining -= len(data)
        data = b''.join(parts)
        _require(len(data) == before.st_size and _metadata(before) == _metadata(os.fstat(fd))
                 and _metadata(before) == _metadata(os.stat(name, dir_fd=parent,
                                                            follow_symlinks=False)), 'file_changed')
        _unchanged(descriptors)
        return data
    except RehearsalError:
        raise
    except (OSError, ValueError):
        raise RehearsalError('file_invalid') from None
    finally:
        if fd is not None:
            os.close(fd)
        _close(descriptors)


def _tar_bytes(raw):
    _require(type(raw) is bytes and 0 < len(raw) <= MAX_INPUT, 'archive_size')
    limit = MAX_EXPANDED + MAX_MEMBERS * 1024 + 10240
    if not raw.startswith(b'\x1f\x8b'):
        _require(len(raw) <= limit, 'archive_size')
        return raw
    try:
        decoder = zlib.decompressobj(16 + zlib.MAX_WBITS)
        pieces, length = [], 0
        for index in range(0, len(raw), 65536):
            piece = decoder.decompress(raw[index:index + 65536], limit - length + 1)
            pieces.append(piece)
            length += len(piece)
            _require(length <= limit and not decoder.unconsumed_tail, 'archive_size')
        _require(decoder.eof and not decoder.unused_data, 'archive_gzip')
        return b''.join(pieces)
    except zlib.error:
        raise RehearsalError('archive_gzip') from None


def inspect_archive(raw, *, recovery):
    """Strict USTAR/GNU ordinary entries, including terminator and gzip CRC.

    Extended metadata, links, sparse files, and executable ownership restoration
    are unsupported. Every explicit and implied directory is counted first.
    """
    _require(type(recovery) is bool, 'archive_scope')
    raw = _tar_bytes(raw)
    _require(len(raw) >= 1024 and len(raw) % 512 == 0, 'archive_truncated')
    offset, expanded = 0, 0
    nodes, explicit, entries = {}, set(), []
    while offset < len(raw):
        block = raw[offset:offset + 512]
        if block == bytes(512):
            _require(len(raw) - offset >= 1024 and not any(raw[offset:]), 'archive_trailer')
            break
        _require(len(entries) < MAX_MEMBERS, 'archive_members')
        try:
            member = tarfile.TarInfo.frombuf(block, 'utf-8', 'strict')
        except (tarfile.TarError, ValueError, UnicodeError):
            raise RehearsalError('archive_header') from None
        # TarInfo.frombuf silently strips *all* directory suffix slashes. Use
        # the original name field so malformed paths cannot be normalized away.
        try:
            name = block[:100].split(b'\x00', 1)[0].decode('utf-8')
            prefix = block[345:500].split(b'\x00', 1)[0].decode('utf-8')
            if block[257:265] != tarfile.GNU_MAGIC and prefix:
                name = prefix + '/' + name
        except UnicodeError:
            raise RehearsalError('archive_path') from None
        if member.isdir() and name.endswith('/'):
            name = name[:-1]
        parts = name.split('/')
        _require(name and len(name.encode()) <= 1024 and len(parts) <= 64
                 and not name.startswith('/') and '\\' not in name and '\x00' not in name
                 and all(part not in ('', '.', '..') for part in parts), 'archive_path')
        _require(member.type in (tarfile.REGTYPE, tarfile.AREGTYPE, tarfile.DIRTYPE)
                 and not member.issparse() and not member.linkname
                 and not member.pax_headers, 'archive_type')
        directory = member.isdir()
        _require(type(member.size) is int and (member.size == 0 if directory
                 else 0 <= member.size <= MAX_FILE), 'archive_size')
        _require(name not in explicit, 'archive_duplicate')
        _require(parts[0] in ROOTS if recovery else len(parts) == 1
                 and name in FILES | {'proof.json'} and not directory, 'archive_scope')
        for length in range(1, len(parts)):
            ancestor = '/'.join(parts[:length])
            _require(nodes.get(ancestor, True), 'archive_collision')
            nodes[ancestor] = True
        _require(name not in nodes or nodes[name] == directory, 'archive_collision')
        nodes[name] = directory
        _require(sum(nodes.values()) <= MAX_DIRECTORIES, 'archive_directories')
        explicit.add(name)
        padded = (member.size + 511) // 512 * 512
        end = offset + 512 + padded
        _require(end <= len(raw), 'archive_truncated')
        data = raw[offset + 512:offset + 512 + member.size]
        _require(not any(raw[offset + 512 + member.size:end]), 'archive_padding')
        expanded += member.size
        _require(expanded <= MAX_EXPANDED, 'archive_size')
        entries.append((name, directory, data))
        offset = end
    else:
        raise RehearsalError('archive_trailer')
    if recovery:
        _require(ROOTS <= explicit and all(nodes[root] for root in ROOTS), 'archive_roots')
        _require(any(name == 'source/.env' and not directory and data
                     for name, directory, data in entries), 'archive_env')
    else:
        _require(explicit == FILES | {'proof.json'}, 'archive_scope')
    return ValidatedArchive(tuple(entries), recovery)


def strict_proof(raw):
    _require(type(raw) is bytes and 0 < len(raw) <= 16384, 'proof_size')
    def pairs(items):
        result = {}
        for key, value in items:
            _require(key not in result, 'proof_duplicate')
            result[key] = value
        return result
    def constant(_):
        raise RehearsalError('proof_invalid')
    try:
        value = json.loads(raw, object_pairs_hook=pairs, parse_constant=constant)
        pending, count = [(value, 0)], 0
        while pending:
            item, depth = pending.pop()
            count += 1
            _require(depth <= 4 and count <= 128, 'proof_bounds')
            if type(item) is dict:
                _require(len(item) <= 32 and all(type(key) is str and len(key) <= 128
                                               for key in item), 'proof_bounds')
                pending.extend((nested, depth + 1) for nested in item.values())
            else:
                _require(type(item) in (str, int, bool), 'proof_invalid')
                _require(type(item) is not str or len(item) <= 4096, 'proof_bounds')
                _require(type(item) is not int or 0 <= item <= 2**63 - 1, 'proof_bounds')
        _require(type(value) is dict and set(value) == _PROOF_FIELDS, 'proof_fields')
        return value
    except RehearsalError:
        raise
    except (ValueError, UnicodeError, RecursionError, TypeError):
        raise RehearsalError('proof_invalid') from None


def validate_bundle(raw, *, binding, now=None):
    """Bind exact bytes to a trusted policy; proof metadata cannot supply it."""
    _require(type(binding) is RootBinding and type(binding.file_sha256) is tuple
             and len(binding.file_sha256) == 3, 'binding_invalid')
    try:
        expected = dict(binding.file_sha256)
    except (ValueError, TypeError):
        raise RehearsalError('binding_invalid') from None
    _require(set(expected) == FILES and all(type(value) is str and _HASH.fullmatch(value)
             for value in expected.values()) and type(binding.snapshot_id) is str
             and _HASH.fullmatch(binding.snapshot_id) and type(binding.proof_sha256) is str
             and _HASH.fullmatch(binding.proof_sha256) and type(binding.host_id) is str
             and re.fullmatch(r'[A-Za-z0-9_.-]{1,128}', binding.host_id), 'binding_invalid')
    now = int(time.time()) if now is None else now
    _require(type(now) is int and 0 <= now <= 2**63 - 1, 'clock_invalid')
    outer = inspect_archive(raw, recovery=False)
    files = {name: data for name, _, data in outer.entries}
    _require(all(hashlib.sha256(files[name]).hexdigest() == value
                 for name, value in expected.items()), 'binding_files')
    _require(hashlib.sha256(files['proof.json']).hexdigest() == binding.proof_sha256, 'binding_proof')
    proof = strict_proof(files['proof.json'])
    _require(proof['backup_id'] == binding.snapshot_id and proof['host_id'] == binding.host_id,
             'proof_identity')
    _require(type(proof['retrieved_file_sha256']) is dict
             and proof['retrieved_file_sha256'] == expected, 'proof_hashes')
    _require(proof['offhost_verified'] is True
             and proof['coolify_config_operator_restore_required'] is True
             and proof['test_location'] == 'retrieved-snapshot-network-none'
             and proof['offhost_sha256'] == expected['coolify.dump'], 'proof_scope')
    _require(type(proof['offhost_object']) is str and 0 < len(proof['offhost_object']) <= 1024
             and all(ord(char) >= 32 and ord(char) != 127 for char in proof['offhost_object'])
             and type(proof['image_id']) is str
             and re.fullmatch(r'sha256:[a-f0-9]{64}', proof['image_id']), 'proof_metadata')
    for field_name in ('verified_at', 'offhost_verified_at', 'restore_tested_at'):
        timestamp = proof[field_name]
        _require(type(timestamp) is int and 0 <= now - timestamp < 86400, 'proof_age')
    created = files['created_at']
    _require(re.fullmatch(rb'[0-9]{1,20}\n?', created) is not None, 'created_invalid')
    created = int(created)
    _require(0 <= now - created < 86400 and created <= min(proof[field_name] for field_name in
             ('verified_at', 'offhost_verified_at', 'restore_tested_at')), 'created_age')
    selected = inspect_archive(files['coolify-config.tar.gz'], recovery=True)
    # Environment must be safe before any extraction creates a destination.
    clone_env(next(data for name, _, data in selected.entries if name == 'source/.env'))
    return ValidatedMaterial(binding.snapshot_id, tuple((name, files[name]) for name in sorted(FILES)),
                             selected)


def extract_validated(archive, destination, *, policy):
    """Exclusive extraction. A failure retains only our private partial stage.

    Existing paths are never overwritten, followed, adopted or cleaned up.
    Archived permission/owner/executable bits are intentionally not restored.
    """
    _require(type(archive) is ValidatedArchive and archive.recovery is True, 'extract_invalid')
    # Revalidate structural bounds even if a caller manufactured the dataclass.
    directories, names, total = set(), set(), 0
    _require(type(archive.entries) is tuple and len(archive.entries) <= MAX_MEMBERS,
             'extract_bounds')
    for entry in archive.entries:
        _require(type(entry) is tuple and len(entry) == 3, 'extract_invalid')
        name, directory, data = entry
        _require(type(name) is str and type(directory) is bool and type(data) is bytes,
                 'extract_invalid')
        try:
            name_bytes = name.encode('utf-8', 'strict')
        except UnicodeError:
            raise RehearsalError('extract_invalid') from None
        parts = name.split('/')
        _require(name not in names and 1 <= len(parts) <= 64 and parts[0] in ROOTS
                 and all(part not in ('', '.', '..') and '\\' not in part and '\x00' not in part
                         for part in parts) and len(name_bytes) <= 1024, 'extract_invalid')
        names.add(name)
        _require(len(data) == 0 if directory else len(data) <= MAX_FILE, 'extract_bounds')
        total += len(data)
        _require(total <= MAX_EXPANDED, 'extract_bounds')
        directories.update('/'.join(parts[:length]) for length in range(1, len(parts)))
        if directory:
            directories.add(name)
        _require(len(directories) <= MAX_DIRECTORIES, 'extract_bounds')
    explicit_roots = {name for name, directory, _ in archive.entries if directory and name in ROOTS}
    _require(ROOTS == explicit_roots and not any(name in directories and not directory
             for name, directory, _ in archive.entries), 'extract_invalid')
    _require(any(name == 'source/.env' and not directory and data
                 for name, directory, data in archive.entries), 'extract_invalid')
    clone_env(next(data for name, _, data in archive.entries if name == 'source/.env'))
    parent, name, descriptors = _protected_parent(destination, policy)
    stage_fd = None
    try:
        _unchanged(descriptors)
        os.mkdir(name, 0o700, dir_fd=parent)
        stage_fd = os.open(name, os.O_RDONLY | os.O_DIRECTORY | os.O_NOFOLLOW, dir_fd=parent)
        _require(os.fstat(stage_fd).st_uid == policy.owner_uid, 'extract_owner')
        os.fchmod(stage_fd, 0o700)
        directory_fds = {'': stage_fd}
        try:
            for dirname in sorted(directories, key=lambda item: (item.count('/'), item)):
                ancestor, _, basename = dirname.rpartition('/')
                selected = directory_fds[ancestor]
                os.mkdir(basename, 0o700, dir_fd=selected)
                directory_fds[dirname] = os.open(basename, os.O_RDONLY | os.O_DIRECTORY | os.O_NOFOLLOW,
                                                dir_fd=selected)
                os.fchmod(directory_fds[dirname], 0o700)
            for filename, directory, data in archive.entries:
                if directory:
                    continue
                ancestor, _, basename = filename.rpartition('/')
                fd = os.open(basename, os.O_WRONLY | os.O_CREAT | os.O_EXCL | os.O_NOFOLLOW,
                             0o600, dir_fd=directory_fds[ancestor])
                try:
                    os.fchmod(fd, 0o600)
                    written = 0
                    while written < len(data):
                        count = os.write(fd, data[written:])
                        _require(count > 0, 'extract_write')
                        written += count
                    os.fsync(fd)
                finally:
                    os.close(fd)
            for selected in reversed(list(directory_fds.values())):
                os.fsync(selected)
            os.fsync(parent)
        finally:
            for dirname, selected in reversed(list(directory_fds.items())):
                if dirname:
                    os.close(selected)
    except RehearsalError:
        raise
    except (OSError, ValueError, KeyError):
        raise RehearsalError('extract_failed') from None
    finally:
        if stage_fd is not None:
            os.close(stage_fd)
        _close(descriptors)


_IDENTITY = ('APP_KEY', 'APP_ID', 'APP_NAME', 'PUSHER_APP_ID', 'PUSHER_APP_KEY', 'PUSHER_APP_SECRET')
_OVERRIDES = {
    'APP_ENV': 'production', 'APP_DEBUG': 'false', 'APP_URL': 'http://127.0.0.1:8080',
    'ASSET_URL': '', 'SELF_HOSTED': 'true', 'AUTOUPDATE': 'false',
    'MIGRATION_ENABLED': 'false', 'SEEDER_ENABLED': 'false', 'HORIZON_ENABLED': 'false',
    'SCHEDULER_ENABLED': 'false', 'QUEUE_CONNECTION': 'redis', 'QUEUE_FAILED_DRIVER': 'database-uuids',
    'DB_CONNECTION': 'pgsql', 'DATABASE_URL': '', 'DB_HOST': '127.0.0.1', 'DB_PORT': '5432',
    'DB_DATABASE': 'coolify', 'DB_USERNAME': 'coolify', 'DB_PASSWORD': 'rehearsal-only-password',
    'REDIS_HOST': '127.0.0.1', 'REDIS_PORT': '6379', 'REDIS_PASSWORD': 'rehearsal-only-password',
    'REDIS_USERNAME': '', 'REDIS_URL': '', 'REDIS_DB': '0', 'REDIS_CACHE_DB': '1',
    'CACHE_DRIVER': 'redis', 'SESSION_DRIVER': 'redis', 'SESSION_SECURE_COOKIE': 'false',
    'SESSION_DOMAIN': '', 'SENTRY_DSN': '', 'PUSHER_BACKEND_HOST': '127.0.0.1',
    'PUSHER_BACKEND_PORT': '6001', 'SSH_COMMAND_TIMEOUT': '5', 'SSH_MAX_RETRIES': '1',
    'PHP_MEMORY_LIMIT': '512M', 'ROOT_USER_EMAIL': '', 'ROOT_USER_PASSWORD': '',
    'DISABLE_DEFAULT_CONFIG': 'true', 'AUTORUN_ENABLED': 'false', 'SSL_MODE': 'off',
    'APP_MAINTENANCE_DRIVER': 'cache', 'APP_MAINTENANCE_STORE': 'redis',
    **{f'DB_{side}_{column}': '' for side in ('READ', 'WRITE')
       for column in ('HOST', 'PORT', 'USERNAME', 'PASSWORD')},
}


def parse_env(raw):
    _require(type(raw) is bytes and 0 < len(raw) <= 65536, 'env_size')
    try:
        source = raw.decode('utf-8')
    except UnicodeError:
        raise RehearsalError('env_invalid') from None
    _require(not any(char in source for char in ('\x00', '\r', '\\', '$', '`')), 'env_invalid')
    values = {}
    lines = source.split('\n')
    _require(len(lines) <= 4096, 'env_size')
    for line in lines:
        if not line.strip() or line.lstrip().startswith('#'):
            continue
        match = re.fullmatch(r'([A-Z][A-Z0-9_]*)=(.*)', line)
        _require(match is not None, 'env_assignment')
        key, value = match.groups()
        _require(key not in values, 'env_duplicate')
        if value.startswith(('"', "'")):
            _require(len(value) >= 2 and value[-1] == value[0], 'env_quotes')
            value = value[1:-1]
        _require(not any(char in value for char in ('"', "'")) and all(ord(char) >= 32
                 and ord(char) != 127 for char in value), 'env_value')
        values[key] = value
    return values


def clone_env(raw):
    """Curated clone only; original bytes remain caller-owned and unchanged."""
    values = parse_env(raw)
    _require(values.get('APP_NAME') and values.get('APP_KEY', '').startswith('base64:'), 'env_identity')
    try:
        key = base64.b64decode(values['APP_KEY'][7:], validate=True)
    except (ValueError, binascii.Error):
        raise RehearsalError('env_key') from None
    _require(len(key) == 32 and base64.b64encode(key).decode() == values['APP_KEY'][7:], 'env_key')
    clone = {key: values[key] for key in _IDENTITY if key in values}
    for key in ('PUSHER_APP_ID', 'PUSHER_APP_KEY', 'PUSHER_APP_SECRET'):
        clone[key] = clone.get(key) or 'rehearsal-only-' + key.lower()
    clone.update(_OVERRIDES)
    _require(clone['APP_KEY'] == values['APP_KEY'], 'env_key')
    return ''.join(key + '=' + (value if key in ('HORIZON_ENABLED', 'SCHEDULER_ENABLED')
                               else "'" + value + "'") + '\n'
                   for key, value in sorted(clone.items())).encode('utf-8')
