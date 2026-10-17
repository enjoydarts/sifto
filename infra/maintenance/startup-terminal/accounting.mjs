import fs from 'node:fs';
import { createHmac, randomBytes, timingSafeEqual } from 'node:crypto';

// Prototype component only: no request authority, process controls, hooks or recovery.
// The original caller owns every token and must end it only after ALL future work
// represented by it is finished. An exception never authorizes starting work.
const KINDS = Object.freeze(['first_auth', 'second_auth', 'buffered_token',
  'token_redemption', 'command', 'kill_timer', 'ws', 'pty']);
const MAX_RECORDS = 256;
const MAX_BYTES = 65536;
const DIRECTORY_FLAGS = fs.constants.O_RDONLY | fs.constants.O_DIRECTORY | fs.constants.O_NOFOLLOW;
const at = (fd, name) => `/proc/self/fd/${fd}/${name}`;
const countsFor = ops => Object.fromEntries(KINDS.map(kind => [kind,
  ops.filter(op => op.kind === kind).length]));
const unknown = () => ({ state: 'unknown', pending: null, counts: null });
const fail = () => { throw new Error('terminal accounting unknown'); };

// Linux descriptor-relative traversal: each actual filesystem component is opened
// O_NOFOLLOW. The proc fd indirection refers only to descriptors owned here.
function openDirectory(parts) {
  let fd = fs.openSync('/', DIRECTORY_FLAGS);
  try {
    for (const part of parts) {
      const next = fs.openSync(at(fd, part), DIRECTORY_FLAGS);
      fs.closeSync(fd);
      fd = next;
    }
    return fd;
  } catch (error) {
    fs.closeSync(fd);
    throw error;
  }
}

function privateDirectory(fd, uid) {
  const stat = fs.fstatSync(fd);
  if (!stat.isDirectory() || stat.uid !== uid || (stat.mode & 0o7777) !== 0o700) fail();
  return stat;
}

function privateFile(fd, uid) {
  const stat = fs.fstatSync(fd);
  if (!stat.isFile() || stat.uid !== uid || stat.nlink !== 1 ||
      (stat.mode & 0o7777) !== 0o600 || stat.size > MAX_BYTES) fail();
  return stat;
}

export function createAccounting({ directory, key, instance, run, generation }) {
  if (!Buffer.isBuffer(key) || key.length !== 32 ||
      typeof instance !== 'string' || !/^[a-f0-9]{64}$/.test(instance) ||
      typeof run !== 'string' || !/^[a-f0-9]{64}$/.test(run) ||
      !Number.isSafeInteger(generation) || generation < 1) {
    throw new TypeError('invalid accounting configuration');
  }
  const secret = Buffer.from(key);
  const uid = process.getuid();
  let broken = false;
  let identity;
  let expected;
  let sequence = 0;
  let records = new Map();
  let parts;
  const mac = payload => createHmac('sha256', secret).update(JSON.stringify(payload)).digest('hex');

  function guard(action) {
    if (broken) fail();
    try { return action(); }
    catch {
      broken = true;
      // This object cannot reset/adopt state. Reconciliation is deliberately
      // absent from T1; callers must retain and resolve their original work.
      fail();
    }
  }

  function withDirectory(action) {
    const fd = openDirectory(parts);
    try {
      const stat = privateDirectory(fd, uid);
      if (stat.dev !== identity.dev || stat.ino !== identity.ino) fail();
      return action(fd);
    } finally { fs.closeSync(fd); }
  }

  function verify(fd) {
    const entries = fs.readdirSync(at(fd, '.'));
    if (entries.length !== 1 || entries[0] !== 'snapshot.json') fail();
    const file = fs.openSync(at(fd, 'snapshot.json'), fs.constants.O_RDONLY |
      fs.constants.O_NOFOLLOW | fs.constants.O_NONBLOCK);
    try {
      privateFile(file, uid);
      const buffer = Buffer.alloc(MAX_BYTES + 1);
      let length = 0;
      for (;;) {
        const size = fs.readSync(file, buffer, length, buffer.length - length, null);
        length += size;
        if (length > MAX_BYTES) fail();
        if (size === 0) break;
      }
      privateFile(file, uid);
      const bytes = buffer.subarray(0, length).toString('utf8');
      // Exact owner-issued canonical bytes also enforce schema, sequence,
      // identity, ops, counts and no extra fields. Never adopt a parsed file.
      if (bytes !== expected) fail();
      const envelope = JSON.parse(bytes);
      if (!timingSafeEqual(Buffer.from(envelope.hmac, 'hex'), Buffer.from(mac(envelope.payload), 'hex'))) fail();
    } finally { fs.closeSync(file); }
  }

  function publish(fd, next) {
    if (!Number.isSafeInteger(sequence + 1)) fail();
    const ops = [...next.values()].map(({ id, kind, parent }) => ({ id, kind, parent }));
    const payload = { schema: 1, sequence: sequence + 1, instance, run, generation,
      ops, counts: countsFor(ops) };
    const bytes = JSON.stringify({ payload, hmac: mac(payload) });
    if (Buffer.byteLength(bytes) > MAX_BYTES) fail();
    const temporary = at(fd, `.pending-${randomBytes(16).toString('hex')}`);
    // An interrupted update leaves a visible pending file; no cleanup or retry
    // is allowed to conceal an incomplete publication.
    const file = fs.openSync(temporary, fs.constants.O_WRONLY | fs.constants.O_CREAT |
      fs.constants.O_EXCL | fs.constants.O_NOFOLLOW, 0o600);
    try {
      fs.fsyncSync(fd);
      privateFile(file, uid);
      fs.writeFileSync(file, bytes, { encoding: 'utf8' });
      fs.fsyncSync(file);
      privateFile(file, uid);
    } finally { fs.closeSync(file); }
    fs.renameSync(temporary, at(fd, 'snapshot.json'));
    fs.fsyncSync(fd);
    expected = bytes;
    sequence += 1;
    records = next;
  }

  // Directory creation is exclusive. Even authenticated existing state belongs
  // to a previous owner and must stay unknown rather than be silently adopted.
  try {
    if (typeof directory !== 'string' || !directory.startsWith('/') || directory.includes('\0')) fail();
    parts = directory.slice(1).split('/');
    if (!parts.length || parts.some(part => !part || part === '.' || part === '..')) fail();
    const parent = openDirectory(parts.slice(0, -1));
    try {
      fs.mkdirSync(at(parent, parts.at(-1)), { mode: 0o700 });
      fs.fsyncSync(parent);
    } finally { fs.closeSync(parent); }
    const fd = openDirectory(parts);
    try {
      identity = privateDirectory(fd, uid);
      if (fs.readdirSync(at(fd, '.')).length !== 0) fail();
      publish(fd, records);
    } finally { fs.closeSync(fd); }
  } catch { broken = true; }

  function kindCheck(kind) {
    if (!KINDS.includes(kind)) throw new TypeError('invalid operation kind');
  }
  function record(token) {
    if (!records.has(token)) throw new TypeError('invalid operation token');
    return records.get(token);
  }
  function childCheck(op) {
    if ([...records.values()].some(child => child.parent === op.id)) {
      throw new Error('operation has pending child');
    }
  }
  function tokenRecord(kind, parent) {
    return { id: randomBytes(16).toString('hex'), kind, parent };
  }

  return Object.freeze({
    begin(kind, parent) {
      if (broken) fail();
      kindCheck(kind);
      const parentId = parent === undefined ? null : record(parent).id;
      return guard(() => withDirectory(fd => {
        verify(fd);
        if (records.size >= MAX_RECORDS) fail();
        const token = Object.freeze(Object.create(null));
        const next = new Map(records);
        next.set(token, tokenRecord(kind, parentId));
        publish(fd, next);
        return token;
      }));
    },
    end(token) {
      if (broken) fail();
      const op = record(token);
      childCheck(op);
      guard(() => withDirectory(fd => {
        verify(fd);
        const next = new Map(records);
        next.delete(token);
        publish(fd, next);
      }));
    },
    // Replace one lifetime with its continuation in ONE durable publication.
    // Keep its parent; replacing a parent with live children is rejected.
    transfer(token, kind) {
      if (broken) fail();
      kindCheck(kind);
      const op = record(token);
      childCheck(op);
      return guard(() => withDirectory(fd => {
        verify(fd);
        const replacement = Object.freeze(Object.create(null));
        const next = new Map(records);
        next.delete(token);
        next.set(replacement, tokenRecord(kind, op.parent));
        publish(fd, next);
        return replacement;
      }));
    },
    inspect() {
      if (broken) return unknown();
      try {
        guard(() => withDirectory(verify));
        return { state: 'healthy', pending: records.size, counts: countsFor([...records.values()]) };
      } catch { return unknown(); }
    },
  });
}
