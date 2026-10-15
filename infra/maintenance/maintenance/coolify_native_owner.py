"""Dormant native admission primitives. Fixed root input is synthetic-only.

Request metadata is root authority. The controller never reads material bodies;
only the preparation-budgeted engine may hash and validate those bytes. The
concrete controller/engine execution bridge remains an N4b gate: public
entrypoint factories reject before request, claim, reservation or launch IO.
"""
import hashlib
import json
import os
from pathlib import Path
import re
import stat

from . import coolify_docker as d
from .coolify_rehearsal import FILES, MAX_INPUT, RootBinding

_REQUEST = Path('/var/lib/sifto-maintenance/coolify-native-request')
_STATE = Path('/var/lib/sifto-maintenance/coolify-native-state')
_RUNS = Path('/var/lib/sifto-maintenance/coolify-native-runs')
_RUNTIME = Path('/run/sifto-maintenance/coolify-native')
_HASH = re.compile('[a-f0-9]{64}\\Z')
_TOKEN = re.compile('[a-f0-9]{32}\\Z')
_CAUSES = frozenset(('request_invalid', 'request_replayed', 'claim_invalid',
    'ownership_unknown', 'source_closure', 'cleanup_incomplete', 'command_failed'))


class OwnerError(Exception):
    def __init__(self, cause='command_failed'):
        super().__init__(cause if type(cause) is str and cause in _CAUSES else 'command_failed')


def run_controller(installed_source_sha256):
    raise OwnerError('source_closure') from None


def run_engine(installed_source_sha256):
    raise OwnerError('source_closure') from None


def _require(condition, cause):
    if not condition:
        raise OwnerError(cause) from None


def _json(value):
    return json.dumps(value, sort_keys=True, separators=(',', ':'), allow_nan=False).encode('ascii')


def _decode(raw, cause):
    def pairs(items):
        result = {}
        for key, value in items:
            _require(key not in result, cause)
            result[key] = value
        return result
    def invalid(_):
        raise OwnerError(cause)
    try:
        return json.loads(raw, object_pairs_hook=pairs, parse_constant=invalid)
    except BaseException:
        raise OwnerError(cause) from None


def _identity(info):
    return (info.st_dev, info.st_ino, info.st_mode, info.st_uid, info.st_gid,
            info.st_nlink, info.st_size, info.st_mtime_ns, info.st_ctime_ns)


def _stable(info):
    return info.st_dev, info.st_ino, info.st_mode, info.st_uid, info.st_gid, info.st_nlink


def _close(fd, pin):
    try:
        if (os.fstat(fd).st_dev, os.fstat(fd).st_ino) == pin[:2]:
            os.close(fd)
    except OSError:
        pass


class _Directory:
    def __init__(self, path, cause):
        self.path, self.cause = path, cause
        self.fd = None
        try:
            self.fd, self.parents = d._protected_open(path, os.O_RDONLY | os.O_DIRECTORY,
                                                     category='context_policy')
            info = os.fstat(self.fd)
            _require(stat.S_ISDIR(info.st_mode) and info.st_uid == info.st_gid == 0
                     and stat.S_IMODE(info.st_mode) == 0o700, cause)
            self.pin = _stable(info)
        except BaseException:
            if self.fd is not None:
                os.close(self.fd)
            raise OwnerError(cause) from None

    def guard(self):
        try:
            _require(_stable(os.fstat(self.fd)) == self.pin == _stable(self.path.lstat()), self.cause)
            for path, pin in self.parents:
                _require(d._metadata(path.lstat(), True) == pin, self.cause)
        except OSError:
            raise OwnerError(self.cause) from None

    def close(self):
        if self.fd is not None:
            _close(self.fd, self.pin)
            self.fd = None


def _file(directory, name, flags, *, create=False):
    cause = directory.cause
    directory.guard()
    fd = os.open(name, flags | os.O_NOFOLLOW | os.O_CLOEXEC | os.O_NONBLOCK
                 | (os.O_CREAT if create else 0), 0o600, dir_fd=directory.fd)
    try:
        info = os.fstat(fd)
        _require(stat.S_ISREG(info.st_mode) and info.st_uid == info.st_gid == 0
                 and info.st_nlink == 1 and stat.S_IMODE(info.st_mode) == 0o600, cause)
        return fd, _identity(info)
    except BaseException:
        os.close(fd)
        raise


def _request_document(value, host):
    cause = 'request_invalid'
    _require(type(value) is dict and set(value) == {'version', 'scope', 'request_seq', 'nonce',
             'binding', 'material_sha256', 'material_size'}, cause)
    _require(type(value['version']) is int and value['version'] == 1
             and value['scope'] == 'synthetic-only'
             and type(value['request_seq']) is int and 1 <= value['request_seq'] < 2**63
             and type(value['nonce']) is str and _TOKEN.fullmatch(value['nonce'])
             and type(value['material_size']) is int and 0 < value['material_size'] <= MAX_INPUT
             and type(value['material_sha256']) is str and _HASH.fullmatch(value['material_sha256']), cause)
    binding = value['binding']
    _require(type(binding) is dict and set(binding) == {'snapshot_id', 'host_id', 'file_sha256', 'proof_sha256'}
             and type(binding['host_id']) is str and binding['host_id'] == host
             and re.fullmatch('[A-Za-z0-9_.-]{1,128}', host)
             and all(type(binding[k]) is str and _HASH.fullmatch(binding[k])
                     for k in ('snapshot_id', 'proof_sha256'))
             and type(binding['file_sha256']) is dict and set(binding['file_sha256']) == FILES
             and all(type(v) is str and _HASH.fullmatch(v) for v in binding['file_sha256'].values()), cause)
    return RootBinding(binding['snapshot_id'], host, tuple(sorted(binding['file_sha256'].items())),
                       binding['proof_sha256'])


class _RequestSlot:
    def __init__(self, directory, host):
        self.directory = _Directory(directory, 'request_invalid')
        self.owned = []
        try:
            self.request_fd, self.request_pin = _file(self.directory, 'request.json', os.O_RDONLY)
            self.owned.append((self.request_fd, self.request_pin))
            _require(0 < self.request_pin[6] <= 16384, 'request_invalid')
            raw = os.pread(self.request_fd, self.request_pin[6] + 1, 0)
            _require(len(raw) == self.request_pin[6], 'request_invalid')
            value = _decode(raw, 'request_invalid')
            self.binding = _request_document(value, host)
            self.digest = hashlib.sha256(raw).hexdigest()
            self.sequence, self.nonce = value['request_seq'], value['nonce']
            self.material_sha256, self.material_size = value['material_sha256'], value['material_size']
            self.material_fd, self.material_pin = _file(self.directory, 'material.tar', os.O_RDONLY)
            self.owned.append((self.material_fd, self.material_pin))
            _require(self.material_pin[6] == self.material_size and self.request_pin[:2] != self.material_pin[:2],
                     'request_invalid')
            self.guard()
        except BaseException:
            self.close()
            raise OwnerError('request_invalid') from None

    def guard(self):
        self.directory.guard()
        try:
            for name, fd, pin in (('request.json', self.request_fd, self.request_pin),
                                  ('material.tar', self.material_fd, self.material_pin)):
                _require(_identity(os.fstat(fd)) == pin == _identity(os.stat(name,
                         dir_fd=self.directory.fd, follow_symlinks=False)), 'request_invalid')
        except OSError:
            raise OwnerError('request_invalid') from None

    def close(self):
        for fd, pin in reversed(self.owned):
            _close(fd, pin)
        self.owned.clear()
        self.directory.close()


def _open_request(directory, host):
    return _RequestSlot(directory, host)


class _ClaimStore:
    """Durable monotonic claims; caller retains the original admission flock.

    Append ambiguity consumes the sequence or blocks all future admission. It
    never rolls back or replays a claimed request after a controller crash.
    """
    def __init__(self, directory):
        self.directory = _Directory(directory, 'claim_invalid')
        self.fd = None
        self.active = None
        try:
            self.fd, self.pin = _file(self.directory, 'claims', os.O_RDWR, create=True)
            _require(self.pin[6] <= 1024**2, 'claim_invalid')
            self.raw = os.pread(self.fd, self.pin[6] + 1, 0)
            _require(len(self.raw) == self.pin[6] and (not self.raw or self.raw.endswith(b'\n')), 'claim_invalid')
            self.highwater = 0
            previous = None
            for raw in self.raw.splitlines():
                row = _decode(raw, 'claim_invalid')
                self._validate(row, previous)
                self.highwater, previous = row['highwater'], row
            os.fsync(self.fd)
            os.fsync(self.directory.fd)
        except BaseException:
            self.close()
            raise OwnerError('claim_invalid') from None

    def _validate(self, row, previous):
        keys = {'version', 'state', 'highwater', 'nonce', 'request_sha256', 'request_identity', 'material_identity', 'run'}
        _require(type(row) is dict and set(row) == keys and type(row['version']) is int and row['version'] == 1
                 and type(row['highwater']) is int and 0 < row['highwater'] < 2**63
                 and type(row['nonce']) is str and _TOKEN.fullmatch(row['nonce'])
                 and type(row['request_sha256']) is str and _HASH.fullmatch(row['request_sha256'])
                 and all(type(row[k]) is list and len(row[k]) == 9
                         and all(type(v) is int and v >= 0 for v in row[k])
                         for k in ('request_identity', 'material_identity')), 'claim_invalid')
        for name, maximum in (('request_identity', 16384), ('material_identity', MAX_INPUT)):
            pin = row[name]
            _require(pin[0] > 0 and pin[1] > 0 and pin[2] == stat.S_IFREG | 0o600
                     and pin[3:6] == [0, 0, 1] and 0 < pin[6] <= maximum, 'claim_invalid')
        _require(row['request_identity'][:2] != row['material_identity'][:2], 'claim_invalid')
        state = row['state']
        if state == 'claimed':
            _require(row['run'] is None and row['highwater'] > (previous['highwater'] if previous else 0), 'claim_invalid')
        else:
            _require(previous is not None and state in ('bound-to-run', 'finished')
                     and previous['state'] == ('claimed' if state == 'bound-to-run' else 'bound-to-run')
                     and all(row[k] == previous[k] for k in keys - {'run', 'state'})
                     and type(row['run']) is str and _TOKEN.fullmatch(row['run'])
                     and (state != 'finished' or row['run'] == previous['run']), 'claim_invalid')

    def guard(self):
        self.directory.guard()
        try:
            _require(_identity(os.fstat(self.fd)) == self.pin == _identity(os.stat('claims',
                     dir_fd=self.directory.fd, follow_symlinks=False))
                     and os.pread(self.fd, len(self.raw) + 1, 0) == self.raw, 'claim_invalid')
        except OSError:
            raise OwnerError('claim_invalid') from None

    def _append(self, row):
        self.guard()
        data = _json(row) + b'\n'
        _require(len(self.raw) + len(data) <= 1024**2, 'claim_invalid')
        os.lseek(self.fd, 0, os.SEEK_END)
        view = memoryview(data)
        while view:
            written = os.write(self.fd, view)
            _require(written > 0, 'claim_invalid')
            view = view[written:]
        os.fsync(self.fd)
        os.fsync(self.directory.fd)
        self.pin, self.raw = _identity(os.fstat(self.fd)), self.raw + data
        self.highwater, self.active = row['highwater'], row

    def claim(self, request):
        _require(type(request) is _RequestSlot, 'ownership_unknown')
        request.guard()
        self.guard()
        _require(request.sequence > self.highwater, 'request_replayed')
        row = {'version': 1, 'state': 'claimed', 'highwater': request.sequence,
               'nonce': request.nonce, 'request_sha256': request.digest,
               'request_identity': list(request.request_pin),
               'material_identity': list(request.material_pin), 'run': None}
        self._append(row)

    def bind(self, run):
        _require(self.active is not None and self.active['state'] == 'claimed'
                 and type(run) is str and _TOKEN.fullmatch(run), 'claim_invalid')
        self._append(dict(self.active, state='bound-to-run', run=run))

    def finish(self):
        _require(self.active is not None and self.active['state'] == 'bound-to-run', 'claim_invalid')
        self._append(dict(self.active, state='finished'))

    def close(self):
        if self.fd is not None:
            _close(self.fd, self.pin)
            self.fd = None
        self.directory.close()
