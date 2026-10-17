import assert from 'node:assert/strict';
import { test } from 'node:test';
import fs from 'node:fs';
import { randomUUID, createHmac } from 'node:crypto';

// PUBLIC synthetic fixture key; never an authority or production credential.
const key = Buffer.alloc(32, 0x41);
const context = { instance: '1'.repeat(64), run: '2'.repeat(64), generation: 1 };
const modulePath = '../startup-terminal/accounting.mjs';
const implementation = await import(modulePath).catch(error => {
  if (error.code === 'ERR_MODULE_NOT_FOUND') return {};
  throw error;
});
const make = (options = {}) => {
  assert.equal(typeof implementation.createAccounting, 'function', 'durable accounting constructor must exist');
  return implementation.createAccounting({ directory: `/tmp/ledger-${randomUUID()}`, key, ...context, ...options });
};
const read = directory => JSON.parse(fs.readFileSync(`${directory}/snapshot.json`, 'utf8'));

test('begin durably publishes before an asynchronous callback can be registered', async () => {
  const directory = `/tmp/ledger-${randomUUID()}`;
  const ledger = make({ directory });
  const op = ledger.begin('command');
  assert.equal(typeof op, 'object');
  assert.deepEqual(Object.keys(op), []);
  assert.equal(read(directory).payload.counts.command, 1);
  await new Promise(resolve => queueMicrotask(() => {
    assert.equal(ledger.inspect().pending, 1);
    ledger.end(op);
    resolve();
  }));
  assert.equal(ledger.inspect().pending, 0);
});

test('parents hold children and ownership transfer never publishes a zero gap', () => {
  const directory = `/tmp/ledger-${randomUUID()}`;
  const ledger = make({ directory });
  const command = ledger.begin('command');
  const timer = ledger.begin('kill_timer', command);
  assert.throws(() => ledger.end(command), /child/);
  const pty = ledger.transfer(timer, 'pty');
  assert.equal(read(directory).payload.ops.length, 2);
  assert.equal(ledger.inspect().counts.kill_timer, 0);
  assert.equal(ledger.inspect().counts.pty, 1);
  assert.throws(() => ledger.end(timer), /token/);
  ledger.end(pty);
  ledger.end(command);
  assert.equal(ledger.inspect().pending, 0);
});

test('all closed kinds, opaque tokens, malformed inputs and double settlement', () => {
  const ledger = make();
  for (const kind of ['first_auth', 'second_auth', 'buffered_token', 'token_redemption', 'command', 'kill_timer', 'ws', 'pty']) {
    const op = ledger.begin(kind);
    assert.throws(() => ledger.end({}), /token/);
    ledger.end(op);
    assert.throws(() => ledger.end(op), /token/);
  }
  for (const kind of ['', 'session', null, {}, 'command\n']) assert.throws(() => ledger.begin(kind), /kind/);
  assert.throws(() => ledger.begin('command', {}), /token/);
  const root = ledger.begin('ws');
  const child = ledger.begin('command', root);
  assert.throws(() => ledger.transfer(root, 'pty'), /child/);
  ledger.end(child);
  ledger.end(root);
  assert.equal(ledger.inspect().pending, 0);
});

test('256 records are admitted, overflow becomes sticky unknown', () => {
  const directory = `/tmp/ledger-${randomUUID()}`;
  const ledger = make({ directory });
  const ops = Array.from({ length: 256 }, () => ledger.begin('ws'));
  assert.equal(ledger.inspect().pending, 256);
  assert.equal(read(directory).payload.ops.length, 256);
  assert.ok(fs.statSync(`${directory}/snapshot.json`).size < 65536);
  assert.throws(() => ledger.begin('ws'), /unknown/);
  assert.deepEqual(ledger.inspect(), { state: 'unknown', pending: null, counts: null });
  assert.throws(() => ledger.end(ops[0]), /unknown/);
});

test('transfer at the bound publishes exactly once without a zero gap', t => {
  const directory = `/tmp/ledger-${randomUUID()}`;
  const ledger = make({ directory });
  const tokens = Array.from({ length: 256 }, () => ledger.begin('command'));
  const before = read(directory).payload.sequence;
  const published = [];
  const rename = fs.renameSync;
  t.mock.method(fs, 'renameSync', (from, to) => {
    published.push(JSON.parse(fs.readFileSync(from, 'utf8')).payload);
    return rename(from, to);
  });
  const replacement = ledger.transfer(tokens[0], 'pty');
  assert.equal(published.length, 1);
  assert.equal(published[0].sequence, before + 1);
  assert.equal(published[0].ops.length, 256);
  assert.equal(published[0].counts.pty, 1);
  assert.equal(ledger.inspect().pending, 256);
  assert.throws(() => ledger.end(tokens[0]), /token/);
  ledger.end(replacement);
});

test('begin return follows file fsync, rename and final directory fsync', t => {
  const ledger = make();
  const events = [];
  const sync = fs.fsyncSync;
  const rename = fs.renameSync;
  t.mock.method(fs, 'fsyncSync', fd => {
    const result = sync(fd);
    events.push(fs.fstatSync(fd).isDirectory() ? 'dir-sync' : 'file-sync');
    return result;
  });
  t.mock.method(fs, 'renameSync', (...args) => {
    const result = rename(...args);
    events.push('rename');
    return result;
  });
  const token = ledger.begin('command');
  events.push('admitted');
  assert.deepEqual(events, ['dir-sync', 'file-sync', 'rename', 'dir-sync', 'admitted']);
  t.mock.restoreAll();
  ledger.end(token);
});

test('snapshot open is nonblocking and no-follow before file-type validation', t => {
  const ledger = make();
  const open = fs.openSync;
  let snapshotFlags;
  t.mock.method(fs, 'openSync', (path, flags, ...args) => {
    if (String(path).endsWith('/snapshot.json')) snapshotFlags = flags;
    return open(path, flags, ...args);
  });
  assert.equal(ledger.inspect().state, 'healthy');
  assert.ok(snapshotFlags & fs.constants.O_NOFOLLOW);
  assert.ok(snapshotFlags & fs.constants.O_NONBLOCK,
    'unexpected FIFO must not block before regular-file validation');
});

test('durable bytes have bounded canonical schema, private identity and valid HMAC', () => {
  const directory = `/tmp/ledger-${randomUUID()}`;
  const ledger = make({ directory });
  const op = ledger.begin('first_auth');
  const envelope = read(directory);
  const body = JSON.stringify(envelope.payload);
  assert.deepEqual(Object.keys(envelope), ['payload', 'hmac']);
  assert.deepEqual(Object.keys(envelope.payload), ['schema', 'sequence', 'instance', 'run', 'generation', 'ops', 'counts']);
  assert.equal(envelope.hmac, createHmac('sha256', key).update(body).digest('hex'));
  assert.equal(envelope.payload.schema, 1);
  assert.equal(envelope.payload.sequence, 2);
  assert.equal(envelope.payload.instance, context.instance);
  assert.equal(envelope.payload.run, context.run);
  assert.equal(envelope.payload.generation, 1);
  assert.equal(fs.statSync(directory).mode & 0o777, 0o700);
  assert.equal(fs.statSync(`${directory}/snapshot.json`).mode & 0o777, 0o600);
  assert.equal(fs.statSync(`${directory}/snapshot.json`).nlink, 1);
  assert.ok(Buffer.byteLength(JSON.stringify(envelope)) < 65536);
  ledger.end(op);
});

for (const damage of ['temp', 'truncated', 'tamper', 'stale', 'identity', 'shape', 'missing', 'hardlink', 'symlink', 'mode']) {
  test(`${damage} state is rejected and unknown stays sticky`, () => {
    const directory = `/tmp/ledger-${randomUUID()}`;
    const ledger = make({ directory });
    const previous = fs.readFileSync(`${directory}/snapshot.json`);
    ledger.begin('command');
    const file = `${directory}/snapshot.json`;
    if (damage === 'temp') fs.writeFileSync(`${directory}/.pending-interrupted`, 'partial', { mode: 0o600 });
    if (damage === 'truncated') fs.writeFileSync(file, '{');
    if (damage === 'tamper') fs.writeFileSync(file, fs.readFileSync(file, 'utf8').replace('command', 'changed'));
    if (damage === 'stale') fs.writeFileSync(file, previous);
    if (damage === 'missing') fs.unlinkSync(file);
    if (damage === 'hardlink') fs.linkSync(file, `${directory}/extra`);
    if (damage === 'symlink') { fs.renameSync(file, `${directory}/real`); fs.symlinkSync(`${directory}/real`, file); }
    if (damage === 'mode') fs.chmodSync(file, 0o644);
    if (damage === 'identity' || damage === 'shape') {
      const envelope = read(directory);
      if (damage === 'identity') envelope.payload.run = '3'.repeat(64);
      else envelope.payload.extra = true;
      envelope.hmac = createHmac('sha256', key).update(JSON.stringify(envelope.payload)).digest('hex');
      fs.writeFileSync(file, JSON.stringify(envelope));
    }
    assert.deepEqual(ledger.inspect(), { state: 'unknown', pending: null, counts: null });
    assert.throws(() => ledger.begin('pty'), /unknown/);
    fs.writeFileSync(file, previous);
    assert.equal(ledger.inspect().state, 'unknown');
  });
}

test('new owners never adopt existing state, symlink ancestors or traversal', () => {
  const directory = `/tmp/ledger-${randomUUID()}`;
  make({ directory });
  const original = fs.readFileSync(`${directory}/snapshot.json`);
  assert.equal(make({ directory }).inspect().state, 'unknown');
  assert.deepEqual(fs.readFileSync(`${directory}/snapshot.json`), original);
  const link = `/tmp/link-${randomUUID()}`;
  fs.symlinkSync(directory, link);
  assert.equal(make({ directory: `${link}/nested` }).inspect().state, 'unknown');
  assert.equal(make({ directory: `${directory}/../escape` }).inspect().state, 'unknown');
});

test('cross-instance tokens cannot settle or become parents', () => {
  const first = make();
  const second = make();
  const token = first.begin('command');
  assert.throws(() => second.end(token), /token/);
  assert.throws(() => second.begin('kill_timer', token), /token/);
  assert.throws(() => second.transfer(token, 'pty'), /token/);
  assert.equal(first.inspect().pending, 1);
  first.end(token);
});

test('invalid private identity/key configuration cannot publish healthy zero', () => {
  for (const options of [{ key: Buffer.alloc(31) }, { instance: 'name' }, { run: '' }, { generation: 0 }, { generation: 1.5 }]) {
    assert.throws(() => make(options), /configuration/);
  }
});

for (const operation of ['openSync', 'writeFileSync', 'fsyncSync', 'renameSync']) {
  test(`publication ${operation} failure admits no work and never reports healthy zero`, t => {
    const ledger = make();
    let admitted = false;
    t.mock.method(fs, operation, () => { throw new Error('synthetic publication IO failure'); });
    assert.throws(() => { ledger.begin('command'); admitted = true; }, /unknown/);
    assert.equal(admitted, false);
    assert.deepEqual(ledger.inspect(), { state: 'unknown', pending: null, counts: null });
    t.mock.restoreAll();
    assert.throws(() => ledger.begin('pty'), /unknown/);
  });
}

for (const failureAt of [1, 2, 3]) {
  test(`fsync stage ${failureAt} never admits work or clears sticky unknown`, t => {
    const directory = `/tmp/ledger-${randomUUID()}`;
    const ledger = make({ directory });
    const sync = fs.fsyncSync;
    let calls = 0;
    t.mock.method(fs, 'fsyncSync', fd => {
      if (++calls === failureAt) throw new Error('synthetic staged fsync failure');
      return sync(fd);
    });
    let admitted = false;
    assert.throws(() => { ledger.begin('command'); admitted = true; }, /unknown/);
    assert.equal(admitted, false);
    t.mock.restoreAll();
    assert.equal(ledger.inspect().state, 'unknown');
    assert.equal(make({ directory }).inspect().state, 'unknown');
  });
}

test('failed final completion never becomes successful healthy zero', t => {
  const ledger = make();
  const command = ledger.begin('command');
  const rename = fs.renameSync;
  t.mock.method(fs, 'renameSync', (...args) => {
    rename(...args);
    throw new Error('synthetic interrupted publication after rename');
  });
  assert.throws(() => ledger.end(command), /unknown/);
  t.mock.restoreAll();
  assert.deepEqual(ledger.inspect(), { state: 'unknown', pending: null, counts: null });
  assert.throws(() => ledger.end(command), /unknown/);
});
