import test from 'node:test';
import assert from 'node:assert/strict';
import fs from 'node:fs';
import vm from 'node:vm';
import { EventEmitter } from 'node:events';
import { createHmac } from 'node:crypto';
import { createAccounting } from '../startup-terminal/accounting.mjs';
import { instrumentTerminal } from '../startup-terminal/instrument-terminal.mjs';
import { loadFixedSources, createFixture, timerDriver, VALID_COMMAND } from './terminal_hooks_fixture.mjs';

const TOKEN = 'a'.repeat(64);
const roots = new WeakMap();
function realAccounting(t) {
  const root = fs.mkdtempSync('/tmp/terminal-hooks-');
  t.after(() => fs.rmSync(root, { recursive: true, force: true }));
  const accounting = createAccounting({ directory: `${root}/ledger`, key: Buffer.alloc(32, 1),
    instance: '1'.repeat(64), run: '2'.repeat(64), generation: 1 });
  roots.set(accounting, root);
  return accounting;
}
const fault = accounting => fs.writeFileSync(`${roots.get(accounting)}/ledger/.pending-public`, 'public');
const corrupt = accounting => fs.appendFileSync(`${roots.get(accounting)}/ledger/snapshot.json`, '\n');
const counts = f => f.accounting.inspect().counts;
const positive = (f, ...kinds) => { for (const kind of kinds) assert.ok(counts(f)[kind] > 0, `${kind} must remain pending`); };
async function fixture(t, variant = 'old') { return createFixture(await loadFixedSources(variant), realAccounting(t)); }
async function first(f, options) {
  const c = f.connect(options);
  positive(f, 'ws', 'first_auth');
  await f.flush();
  f.reply('/terminal/auth', 200, null);
  await f.flush();
  return c;
}
async function authorized(f, options) {
  const c = await first(f, options);
  f.reply('/terminal/auth/ips', 200, { ipAddresses: ['127.0.0.1'] });
  await f.flush();
  return c;
}
async function redeem(f, c, command = VALID_COMMAND) {
  c.send({ terminalToken: TOKEN });
  await f.flush();
  positive(f, 'token_redemption', 'ws');
  f.reply('/terminal/session', 200, { command });
  await f.flush();
}
function fakeOwner() {
  const records = new Map();
  const history = [];
  let broken = false;
  const inspect = () => ({ state: broken ? 'unknown' : 'healthy', pending: records.size,
    counts: Object.fromEntries(['first_auth','second_auth','buffered_token','token_redemption','command','kill_timer','ws','pty'].map(kind => [kind, [...records.values()].filter(value => value === kind).length])) });
  const owner = { inspect, history, records,
    break() { broken = true; },
    begin(kind, parent) { assert.equal(parent, undefined); if (broken || records.size >= 256) { broken = true; throw new Error('owner fault'); }
      const token = {}; records.set(token, kind); history.push(['begin', kind, inspect()]); return token; },
    end(token) { if (broken) throw new Error('owner fault'); assert.ok(records.has(token));
      const kind = records.get(token); records.delete(token); history.push(['end', kind, inspect()]); },
    transfer() { assert.fail('independent roots must not transfer'); },
  };
  return owner;
}
function observedMaps() {
  const maps = [];
  class ObservedMap extends Map {
    constructor(...args) { super(...args); maps.push(this); }
  }
  // The adapter constructs its timer map, then its private held map.
  return { MapConstructor: ObservedMap, held: () => maps[1] };
}
async function adapter(owner = fakeOwner(), MapConstructor = Map) {
  const driver = timerDriver();
  const context = vm.createContext({ Promise, WeakSet, WeakMap, Map: MapConstructor,
    setTimeout: driver.setTimeout, clearTimeout: driver.clearTimeout,
    queueMicrotask: callback => queueMicrotask(() => { try { callback(); } catch (error) { driver.errors.push(error); } }),
  });
  const module = new vm.SourceTextModule(fs.readFileSync(new URL('../startup-terminal/terminal-hooks.mjs', import.meta.url), 'utf8'), { context });
  await module.link(() => assert.fail('adapter must have no external authority'));
  await module.evaluate();
  return { owner, driver, hooks: module.namespace.createTerminalHooks(owner) };
}

test('fixed source instrumentation interface exists', async () => {
  const implementation = await import('../startup-terminal/instrument-terminal.mjs').catch(() => null);
  assert.equal(typeof implementation?.instrumentTerminal, 'function');
});

test('OLD and TARGET produce identical frozen ESM output', async () => {
  const old = instrumentTerminal(await loadFixedSources());
  const target = instrumentTerminal(await loadFixedSources('target'));
  assert.deepEqual(old, target);
  assert.ok(Object.isFrozen(old));
  assert.deepEqual(Object.keys(old), ['server', 'utils']);
  assert.match(old.server, /export function createTerminalServer\(accounting\)/);
  assert.match(old.server, /hooks\.spawn\(\(\) =>/);
  assert.match(old.utils, /return Promise\.resolve\(\)/);
  assert.ok(old.server.includes(`    if (userSession !== expectedSession || !liveSession(userSession, ws)) return;
    if (!isAuthorizedTargetHost(targetHost, userSession.authorizedIPs) ||
        !validateSshArgs(sshArgs, userSession.authorizedIPs)) return;
    hooks.assertHealthy();
    const ptyProcess = hooks.spawn(() =>
        pty.spawn('ssh', sanitizedSshArgs.concat([hereDocContent]), options));`));
});
for (const invalid of ['byte', 'newline', 'truncate', 'swap', 'string', 'missing', 'extra', 'symbol', 'reinstrument']) {
  test(`source admission rejects ${invalid}`, async () => {
    const source = await loadFixedSources();
    if (invalid === 'byte') { source.server = Buffer.from(source.server); source.server[0] ^= 1; }
    if (invalid === 'newline') source.utils = Buffer.concat([source.utils, Buffer.from('\n')]);
    if (invalid === 'truncate') source.server = source.server.subarray(0, -1);
    if (invalid === 'swap') [source.server, source.utils] = [source.utils, source.server];
    if (invalid === 'string') source.utils = source.utils.toString();
    if (invalid === 'missing') delete source.server;
    if (invalid === 'extra') source.pins = {};
    if (invalid === 'symbol') source[Symbol('override')] = {};
    if (invalid === 'reinstrument') { const output = instrumentTerminal(source); source.server = Buffer.from(output.server); source.utils = Buffer.from(output.utils); }
    assert.throws(() => instrumentTerminal(source), /terminal source/);
  });
}
test('source pair rejects accessors before executing untrusted property reads', async () => {
  const source = await loadFixedSources(); let reads = 0;
  Object.defineProperty(source, 'server', { enumerable: true, get() { reads++; return Buffer.from('untrusted'); } });
  assert.throws(() => instrumentTerminal(source), /terminal source/);
  assert.equal(reads, 0);
});
test('factory imports create no listener and rejects unknown, busy and reused owners', async (t) => {
  const f = await createFixture(await loadFixedSources(), realAccounting(t), { start: false });
  assert.equal(f.servers.length, 0);
  assert.equal(f.socketServers.length, 0);
  assert.throws(() => f.create({ inspect: () => ({ state: 'unknown' }) }), /unavailable/);
  const busy = realAccounting(t); busy.begin('command');
  assert.throws(() => f.create(busy), /unavailable/);
  assert.equal(f.servers.length, 0);
  const owner = realAccounting(t);
  assert.equal(f.create(owner).server.port, 6002);
  assert.throws(() => f.create(owner), /unavailable/);
  assert.equal(f.servers.length, 1);
});
test('adapter begins synchronously, awaits completion and uses idempotent opaque leases', async () => {
  const { hooks, owner, driver } = await adapter();
  let complete;
  const p = hooks.run('command', () => { assert.equal(owner.inspect().counts.command, 1); return new Promise(resolve => { complete = resolve; }); });
  assert.equal(owner.inspect().pending, 1);
  complete('public'); assert.equal(await p, 'public');
  const lease = hooks.reserve('ws');
  assert.deepEqual(Object.keys(lease), []);
  assert.equal(hooks.release(lease), true); assert.equal(hooks.release(lease), true);
  assert.equal(owner.history.filter(event => event[0] === 'end' && event[1] === 'ws').length, 1);
  await driver.flush(); assert.equal(owner.inspect().pending, 0);
});
test('buffer handoff admits independent redemption before buffer completion', async () => {
  const { hooks, owner } = await adapter();
  const buffered = hooks.buffer('public');
  let complete;
  const p = hooks.consume(buffered, message => hooks.run('token_redemption', () => {
    assert.equal(message, 'public'); assert.equal(owner.inspect().counts.buffered_token, 1);
    return new Promise(resolve => { complete = resolve; });
  }));
  assert.equal(owner.inspect().counts.buffered_token, 0);
  assert.equal(owner.inspect().counts.token_redemption, 1);
  complete(); await p;
  assert.equal(owner.inspect().pending, 0);
});
test('raw errors do not complete sockets and close cleanup admits before release microtask', async () => {
  const { hooks, owner, driver } = await adapter();
  const raw = new EventEmitter(); raw.on('error', () => {});
  hooks.socket(raw); raw.emit('error', new Error('public'));
  assert.equal(owner.inspect().counts.ws, 1);
  let lease;
  raw.on('close', () => { lease = hooks.reserve('command'); });
  raw.emit('close'); assert.equal(owner.inspect().counts.ws, 1);
  await driver.flush(); assert.equal(owner.inspect().counts.command, 1);
  hooks.release(lease); assert.equal(owner.inspect().pending, 0);
});
test('timer identity, unref, cancel before fire and duplicate cancellation', async () => {
  const { hooks, owner, driver } = await adapter();
  let calls = 0;
  const handle = hooks.setTimeout(() => calls++, 2000); handle.unref();
  assert.equal(handle.unreferenced, true); assert.ok(driver.entries.has(handle));
  hooks.clearTimeout(handle); hooks.clearTimeout(handle);
  await driver.advance(2000); assert.equal(calls, 0); assert.equal(owner.inspect().pending, 0);
});
test('cancel during async timer callback retains lease until awaited completion', async () => {
  const { hooks, owner, driver } = await adapter(); let complete;
  const handle = hooks.setTimeout(() => new Promise(resolve => { complete = resolve; }), 1);
  await driver.advance(1); hooks.clearTimeout(handle);
  assert.equal(owner.inspect().counts.kill_timer, 1);
  complete(); await driver.flush(); assert.equal(owner.inspect().pending, 0);
});
test('recursive timers admit successor before predecessor completion', async () => {
  const { hooks, owner, driver } = await adapter();
  hooks.setTimeout(() => hooks.setTimeout(() => {}, 500), 500);
  await driver.advance(500);
  assert.equal(owner.inspect().counts.kill_timer, 1);
  assert.ok(owner.history.some(([event, kind, state]) => event === 'begin' && kind === 'kill_timer' && state.counts.kill_timer === 2));
  await driver.advance(500); assert.equal(owner.inspect().pending, 0);
});
test('ordinary timer exceptions remain reported while admitted registration failures retain work', async () => {
  const { hooks, owner, driver } = await adapter();
  hooks.setTimeout(() => { throw new Error('public callback'); }, 1);
  await driver.advance(1); assert.equal(driver.errors[0].message, 'public callback');
  assert.equal(owner.inspect().pending, 0);
  driver.failNext(); assert.throws(() => hooks.setTimeout(() => {}, 1), /registration failure/);
  assert.equal(owner.inspect().counts.kill_timer, 1);
});
test('physical PTY leases end only on each own exit and listener precedes vendor delivery', async () => {
  const { hooks, owner, driver } = await adapter();
  const children = [new EventEmitter(), new EventEmitter()];
  for (const child of children) {
    child.onExit = callback => child.on('exit', callback);
    const result = hooks.spawn(() => { assert.ok(owner.inspect().counts.pty); return child; });
    assert.equal(result, child); assert.equal(child.listenerCount('exit'), 1);
    child.on('exit', () => { throw new Error('public vendor send'); });
  }
  assert.throws(() => children[0].emit('exit'), /vendor send/); await driver.flush();
  assert.equal(owner.inspect().counts.pty, 1);
  assert.throws(() => children[1].emit('exit'), /vendor send/); await driver.flush();
  assert.equal(owner.inspect().pending, 0);
});
for (const where of ['spawn', 'listener']) test(`uncertain ${where} exception retains physical PTY possibility`, async () => {
  const { hooks, owner } = await adapter();
  assert.throws(() => hooks.spawn(() => {
    if (where === 'spawn') throw new Error('public spawn');
    return { onExit() { throw new Error('public listener'); } };
  }), error => where === 'listener' ? hooks.isUnavailable(error) : /public spawn/.test(error.message));
  assert.equal(owner.inspect().counts.pty, 1);
});
test('latched faults reject new admission, retain admitted scope unresolved and never retry end', async () => {
  const { hooks, owner, driver } = await adapter(); let complete; let settled = false;
  const pending = hooks.run('command', () => new Promise(resolve => { complete = resolve; }));
  pending.then(() => { settled = true; });
  owner.break(); complete(); await driver.flush();
  assert.equal(settled, false); assert.equal(owner.records.size, 1);
  let calls = 0;
  for (let i = 0; i < 1000; i++) await hooks.run('command', () => calls++).catch(error => assert.ok(hooks.isUnavailable(error)));
  assert.equal(calls, 0); assert.equal(owner.records.size, 1);
});
for (const kind of ['run', 'buffer', 'timer']) test(`retention slot preserves actual ${kind} continuation after release bookkeeping fault`, async () => {
  const observer = observedMaps();
  const { hooks, owner, driver } = await adapter(fakeOwner(), observer.MapConstructor);
  let continuation;
  if (kind === 'run') {
    let complete;
    continuation = () => new Promise(resolve => { complete = resolve; });
    hooks.run('command', continuation);
    owner.break(); complete(); await driver.flush();
  } else if (kind === 'buffer') {
    continuation = hooks.buffer({ public: 'already admitted payload' });
    assert.throws(() => hooks.consume(continuation, () => { owner.break(); }), error => hooks.isUnavailable(error));
  } else {
    continuation = () => { owner.break(); };
    hooks.setTimeout(continuation, 1); await driver.advance(1);
  }
  const held = observer.held(); assert.equal(held.size, 1);
  const [lease, slot] = [...held][0];
  assert.equal(slot.continuation, continuation);
  assert.ok(owner.records.has(slot.entry.token));
  for (let i = 0; i < 1000; i++) {
    assert.equal(hooks.release(lease), false);
    await hooks.run('command', () => assert.fail('unadmitted action')).catch(error => assert.ok(hooks.isUnavailable(error)));
  }
  assert.equal(held.size, 1); assert.equal(held.get(lease), slot);
  assert.equal(slot.continuation, continuation);
  assert.deepEqual(Object.keys(slot).sort(), ['continuation', 'entry']);
});
test('actual buffered auth fault retains the shifted message envelope without an unbounded request queue', async (t) => {
  const observer = observedMaps();
  const f = await createFixture(await loadFixedSources(), realAccounting(t), { MapConstructor: observer.MapConstructor });
  const c = await first(f);
  const message = Buffer.from(JSON.stringify({ terminalToken: TOKEN }));
  c.ws.emit('message', message); positive(f, 'buffered_token', 'second_auth');
  fault(f.accounting);
  f.reply('/terminal/auth/ips', 200, { ipAddresses: ['127.0.0.1'] }); await f.flush();
  const held = observer.held();
  const retained = [...held.values()].find(slot => slot.continuation?.message === message);
  assert.ok(retained, 'shifted admitted envelope must remain directly retained');
  assert.equal(held.size, 2);
  const slots = [...held.values()];
  for (let i = 0; i < 1000; i++) c.send({ terminalToken: TOKEN });
  await f.flush();
  assert.equal(held.size, 2); assert.deepEqual([...held.values()], slots);
  assert.equal(retained.continuation.message, message);
  assert.equal(f.requests.filter(request => request.path === '/terminal/session').length, 0);
  assert.equal(f.spawns.length, 0); assert.equal(c.ws.closes.length, 0);
  assert.equal(f.accounting.inspect().state, 'unknown'); assert.equal(f.errors.length, 0);
});
test('ordinary consume error plus release fault retains the original Buffer envelope', async () => {
  const observer = observedMaps();
  const { hooks, owner } = await adapter(fakeOwner(), observer.MapConstructor);
  const message = Buffer.from('public admitted message');
  const envelope = hooks.buffer(message);
  assert.throws(() => hooks.consume(envelope, () => {
    owner.break(); throw new Error('public ordinary action error');
  }), error => hooks.isUnavailable(error));
  const held = observer.held(); assert.equal(held.size, 1);
  const slot = [...held.values()][0];
  assert.equal(slot.continuation, envelope);
  assert.equal(slot.continuation.message, message);
});
test('actual buffered ping send error plus T1 fault retains its shifted Buffer envelope', async (t) => {
  const observer = observedMaps();
  const f = await createFixture(await loadFixedSources(), realAccounting(t), { MapConstructor: observer.MapConstructor });
  const c = await first(f);
  const message = Buffer.from(JSON.stringify({ ping: true }));
  c.ws.emit('message', message);
  c.ws.sendError = new Error('public ordinary buffered ping send error');
  fault(f.accounting);
  f.reply('/terminal/auth/ips', 200, { ipAddresses: ['127.0.0.1'] }); await f.flush();
  const held = observer.held();
  const retained = [...held.values()].find(slot => slot.continuation?.message === message);
  assert.ok(retained, 'ordinary buffered send error must retain the exact shifted Buffer');
  assert.equal(held.size, 2); assert.equal(retained.continuation.message, message);
  assert.equal(f.accounting.inspect().state, 'unknown'); assert.equal(f.errors.length, 0);
  assert.equal(c.ws.closes.length, 0); assert.equal(c.ws.terminations, 0);
});
test('T1 snapshot and canonical HMAC cover actual adapter roots', async (t) => {
  const accounting = realAccounting(t); const { hooks } = await adapter(accounting);
  const lease = hooks.reserve('ws');
  const envelope = JSON.parse(fs.readFileSync(`${roots.get(accounting)}/ledger/snapshot.json`));
  assert.equal(envelope.payload.counts.ws, 1);
  assert.equal(envelope.hmac, createHmac('sha256', Buffer.alloc(32, 1)).update(JSON.stringify(envelope.payload)).digest('hex'));
  assert.equal(hooks.release(lease), true); assert.equal(accounting.inspect().pending, 0);
});
test('ordinary rejected async timer callbacks are reported and accounting sentinel callbacks stay unresolved', async () => {
  const { hooks, owner, driver } = await adapter();
  hooks.setTimeout(async () => { throw new Error('public async callback'); }, 1);
  await driver.advance(1); assert.equal(driver.errors[0].message, 'public async callback');
  assert.equal(owner.inspect().pending, 0);
  hooks.setTimeout(async () => { owner.break(); hooks.assertHealthy(); }, 1);
  await driver.advance(1); assert.equal(driver.errors.length, 1);
  assert.equal(owner.records.size, 1);
});
test('socket and PTY release failures cannot crash synchronous event dispatch', async () => {
  const { hooks, owner, driver } = await adapter();
  const raw = new EventEmitter(); hooks.socket(raw);
  const child = new EventEmitter(); child.onExit = callback => child.on('exit', callback);
  hooks.spawn(() => child); owner.break();
  assert.doesNotThrow(() => { raw.emit('close'); child.emit('exit'); });
  await driver.flush(); assert.equal(driver.errors.length, 0); assert.equal(owner.records.size, 2);
});
test('T1 saturation retains at most 256 admitted roots and rejects unadmitted payload work', async (t) => {
  const accounting = realAccounting(t); const observer = observedMaps();
  const { hooks, driver } = await adapter(accounting, observer.MapConstructor);
  const leases = Array.from({ length: 256 }, () => hooks.reserve('buffered_token'));
  assert.equal(accounting.inspect().pending, 256);
  let actions = 0;
  await hooks.run('command', () => actions++).catch(error => assert.ok(hooks.isUnavailable(error)));
  assert.equal(accounting.inspect().state, 'unknown');
  for (const lease of leases) assert.equal(hooks.release(lease), false);
  assert.equal(observer.held().size, 256);
  for (let i = 0; i < 1000; i++) await hooks.run('command', () => actions++).catch(error => assert.ok(hooks.isUnavailable(error)));
  await driver.flush(); assert.equal(actions, 0); assert.equal(observer.held().size, 256);
  assert.deepEqual(Object.keys(hooks).sort(), ['run','reserve','release','socket','buffer','consume','discard','setTimeout','clearTimeout','spawn','isUnavailable','assertHealthy'].sort());
});
test('delayed first authentication remains pending after actual raw close', async (t) => {
  const f = await fixture(t); const c = f.connect(); positive(f, 'first_auth', 'ws');
  await f.flush(); c.close(); await f.flush(); positive(f, 'first_auth');
  assert.equal(counts(f).ws, 0); f.reply('/terminal/auth', 200, null); await f.flush();
  assert.equal(f.accounting.inspect().pending, 0); assert.equal(c.ws, undefined);
});
test('delayed second authentication discards closed buffered tokens without redemption', async (t) => {
  const f = await fixture(t); const c = await first(f);
  c.send({ terminalToken: TOKEN }); positive(f, 'ws', 'second_auth', 'buffered_token');
  c.close(); await f.flush(); positive(f, 'second_auth', 'buffered_token');
  f.reply('/terminal/auth/ips', 200, { ipAddresses: ['127.0.0.1'] }); await f.flush();
  assert.equal(f.requests.filter(request => request.path === '/terminal/session').length, 0);
  assert.equal(f.accounting.inspect().pending, 0);
});
for (const variant of ['old', 'target']) test(`${variant}: buffered handoff executes actual authorization, token and SSH parser routes`, async (t) => {
  const f = await fixture(t, variant); const c = await first(f);
  c.send({ ping: true, terminalToken: TOKEN }); positive(f, 'buffered_token', 'second_auth');
  f.reply('/terminal/auth/ips', 200, { ipAddresses: ['127.0.0.1'] }); await f.flush();
  positive(f, 'token_redemption', 'ws'); assert.equal(counts(f).buffered_token, 0);
  assert.equal(c.ws.sent[0], 'pong');
  const request = f.reply('/terminal/session', 200, { command: VALID_COMMAND }); await f.flush();
  assert.deepEqual(JSON.parse(request.body), { token: TOKEN });
  assert.equal(request.headers['X-XSRF-TOKEN'], 'public-xsrf');
  assert.equal(f.spawns.length, 1); assert.equal(f.spawns[0].file, 'ssh');
  assert.ok(f.spawns[0].args.includes('root@127.0.0.1'));
  assert.equal(c.ws.closes.length, 0); positive(f, 'ws', 'pty', 'kill_timer');
  assert.equal(counts(f).command, 0); assert.equal(counts(f).token_redemption, 0);
});
test('actual transport close during token redemption prevents spawn after HTTP reply', async (t) => {
  const f = await fixture(t); const c = await authorized(f);
  c.send({ terminalToken: TOKEN }); await f.flush(); c.close(); await f.flush();
  positive(f, 'token_redemption'); f.reply('/terminal/session', 200, { command: VALID_COMMAND }); await f.flush();
  assert.equal(f.spawns.length, 0); assert.equal(f.accounting.inspect().pending, 0);
});
for (const close of [true, false]) test(`vendor 500ms continuation ${close ? 'cancels closed session' : 'preserves live authorized replacement'}`, async (t) => {
  const f = await fixture(t); const c = await authorized(f); await redeem(f, c);
  await redeem(f, c); positive(f, 'command', 'token_redemption', 'kill_timer', 'pty', 'ws');
  assert.equal(f.spawns.length, 1); assert.equal(f.spawns[0].writes.length, 1);
  if (close) c.close();
  f.spawns[0].exit(); await f.flush();
  positive(f, 'command', 'kill_timer');
  await f.advance(500);
  assert.equal(f.spawns.length, close ? 1 : 2);
  if (!close) {
    assert.equal(c.ws.readyState, 1); assert.equal(c.ws.closes.length, 0);
    assert.equal(f.spawns[0].writes.length, 1); positive(f, 'ws', 'pty');
  } else assert.equal(f.accounting.inspect().pending, 0);
});
test('five unsuccessful kill attempts preserve original unprocessable result and physical PTY', async (t) => {
  const f = await fixture(t); const c = await authorized(f); await redeem(f, c); await redeem(f, c);
  for (let i = 0; i < 5; i++) { positive(f, 'command', 'kill_timer', 'pty', 'ws'); await f.advance(500); }
  assert.equal(f.spawns.length, 1); assert.equal(f.spawns[0].writes.length, 5);
  assert.ok(c.ws.sent.includes('unprocessable')); positive(f, 'pty', 'ws');
  assert.equal(counts(f).command, 0);
});
test('old delayed command cannot replace a newly authenticated connection', async (t) => {
  const f = await fixture(t); const a = await authorized(f); await redeem(f, a); await redeem(f, a);
  a.close(); const b = await authorized(f); await redeem(f, b);
  f.spawns[0].exit(); await f.advance(500);
  assert.equal(f.spawns.length, 2); assert.equal(b.ws.readyState, 1); positive(f, 'ws', 'pty');
});
for (const [name, command, expected] of [
  ['unauthorized host', VALID_COMMAND.replace('root@127.0.0.1', 'root@192.0.2.1'), 'Unauthorized: Target host'],
  ['disallowed SSH option', VALID_COMMAND.replace('LogLevel=ERROR', 'LogLevel=DEBUG'), 'Invalid SSH command: Unsupported'],
  ['missing target', 'echo public', 'Invalid SSH command: No target'],
]) test(`actual vendor parser preserves rejection of ${name}`, async (t) => {
  const f = await fixture(t); const c = await authorized(f); await redeem(f, c, command);
  assert.equal(f.spawns.length, 0); assert.ok(c.ws.sent.some(value => String(value).startsWith(expected)));
  positive(f, 'ws'); assert.equal(counts(f).command, 0);
});
for (const [name, options, status, raw, expected] of [
  ['missing cookies', { cookie: '' }, null, null, 4401],
  ['rejected credentials', {}, 403, null, 4401],
  ['HTTP failure', {}, 'failure', null, 1011],
  ['malformed auth JSON', {}, 200, '{', 1011],
]) test(`first auth ${name} preserves close code and owned 2s rejection timer`, async (t) => {
  const f = await fixture(t); const c = f.connect(options); await f.flush();
  if (status === 'failure') f.failRequest('/terminal/auth');
  else if (status !== null) f.reply('/terminal/auth', status, raw, { raw: true });
  await f.flush(); assert.equal(c.ws.closes[0].code, expected);
  positive(f, 'ws', 'kill_timer');
  const rejection = [...f.driver.entries].find(([, entry]) => entry.ms === 2000);
  assert.equal(rejection[0].unreferenced, true);
  c.close(); await f.flush(); assert.equal(f.accounting.inspect().pending, 0);
  await f.advance(2000); assert.equal(c.ws.terminations, 0);
});
test('unknown path owns raw socket until original termination', async (t) => {
  const f = await fixture(t); const c = f.connect({ path: '/unknown' }); await f.flush();
  assert.equal(c.ws.terminations, 1); assert.equal(f.accounting.inspect().pending, 0);
});
test('invalid token and malformed buffered message preserve dispatch order and close code', async (t) => {
  const f = await fixture(t); const c = await first(f); c.send('{'); c.send({ ping: true, terminalToken: 'invalid' });
  assert.equal(counts(f).buffered_token, 2);
  f.reply('/terminal/auth/ips', 200, { ipAddresses: ['127.0.0.1'] }); await f.flush();
  assert.equal(c.ws.sent[0], 'pong'); assert.equal(c.ws.closes[0].code, 4403);
  assert.equal(counts(f).buffered_token, 0); positive(f, 'ws', 'kill_timer');
  await f.advance(2000); assert.equal(c.ws.terminations, 1); assert.equal(f.accounting.inspect().pending, 0);
});
test('failed second auth discards pending buffers with original 1011 close', async (t) => {
  const f = await fixture(t); const c = await first(f); c.send({ terminalToken: TOKEN });
  f.reply('/terminal/auth/ips', 403, null); await f.flush();
  assert.equal(c.ws.closes[0].code, 1011); assert.equal(counts(f).buffered_token, 0);
  assert.equal(f.spawns.length, 0); positive(f, 'ws');
});
test('empty heartbeat is idle and live heartbeat keeps physical ownership', async (t) => {
  const f = await fixture(t); assert.equal(f.accounting.inspect().pending, 0);
  await f.advance(30000); assert.equal(f.accounting.inspect().pending, 0);
  const c = await authorized(f); await f.advance(30000);
  assert.ok(c.ws.pings); positive(f, 'ws');
});
test('two concurrent PTYs retain distinct physical lifetimes after latest session pointer changes', async (t) => {
  const f = await fixture(t); const c = await authorized(f);
  await redeem(f, c);
  c.close(); await f.advance(2500);
  // Vendor deletes the old session after five attempts even if its PTY lives.
  assert.equal(counts(f).ws, 0); assert.equal(counts(f).pty, 1);
  const b = await authorized(f); await redeem(f, b);
  assert.equal(f.spawns.length, 2); assert.equal(counts(f).pty, 2);
  c.ws.sendError = new Error('public send failure');
  assert.throws(() => f.spawns[0].exit(), /public send failure/);
  await f.flush(); assert.equal(counts(f).pty, 1);
  assert.equal(b.ws.readyState, 1);
  f.spawns[1].exit(); await f.flush(); assert.equal(counts(f).pty, 0);
});
test('concurrent same-session redemptions independently cover serialized retry continuations', async (t) => {
  const f = await fixture(t); const c = await authorized(f); await redeem(f, c);
  c.send({ terminalToken: TOKEN }); c.send({ terminalToken: TOKEN }); await f.flush();
  f.reply('/terminal/session', 200, { command: VALID_COMMAND });
  f.reply('/terminal/session', 200, { command: VALID_COMMAND }); await f.flush();
  assert.equal(counts(f).command, 2); assert.equal(counts(f).token_redemption, 2);
  f.spawns[0].exit(); await f.advance(500);
  assert.equal(f.spawns.length, 2); assert.equal(counts(f).command, 1);
  positive(f, 'pty', 'ws', 'kill_timer');
  await f.advance(2000);
  assert.equal(f.spawns.length, 2); assert.equal(counts(f).command, 0);
  assert.ok(c.ws.sent.includes('unprocessable'));
});
test('8-hour timer remains owned during awaited vendor kill and cancellation after firing', async (t) => {
  const f = await fixture(t); const c = await authorized(f); await redeem(f, c);
  await f.advance(8 * 60 * 60 * 1000); assert.equal(counts(f).kill_timer, 2);
  assert.equal(f.spawns[0].writes.length, 1);
  f.spawns[0].exit(); await f.flush(); assert.equal(counts(f).kill_timer, 2);
  await f.advance(500); assert.equal(counts(f).kill_timer, 0); positive(f, 'ws');
});
test('real owner publication observations never show zero between buffered auth and physical PTY handoffs', async (t) => {
  const real = realAccounting(t); const observations = [];
  const accounting = Object.freeze({ inspect: () => real.inspect(),
    begin(kind) { const token = real.begin(kind); observations.push(real.inspect()); return token; },
    end(token) { real.end(token); observations.push(real.inspect()); },
  });
  const f = await createFixture(await loadFixedSources(), accounting); const c = await first(f);
  c.send({ terminalToken: TOKEN });
  f.reply('/terminal/auth/ips', 200, { ipAddresses: ['127.0.0.1'] }); await f.flush();
  f.reply('/terminal/session', 200, { command: VALID_COMMAND }); await f.flush();
  await redeem(f, c); f.spawns[0].exit(); await f.advance(500);
  assert.equal(f.spawns.length, 2); assert.ok(observations.length > 20);
  assert.ok(observations.some(state => state.counts.first_auth && state.counts.second_auth));
  assert.ok(observations.some(state => state.counts.buffered_token && state.counts.token_redemption));
  assert.ok(observations.some(state => state.counts.token_redemption && state.counts.command));
  assert.ok(observations.some(state => state.counts.command && state.counts.pty));
  for (const observation of observations) {
    assert.equal(observation.state, 'healthy'); assert.ok(observation.pending > 0);
  }
});
test('snapshot corruption before admission starts no upgrade, HTTP, PTY or close work', async (t) => {
  const f = await fixture(t); corrupt(f.accounting); const c = f.connect(); await f.flush();
  assert.equal(c.ws, undefined); assert.equal(f.requests.length, 0); assert.equal(c.socket.destroyed, false);
  assert.equal(f.spawns.length, 0); assert.equal(f.errors.length, 0); assert.equal(f.accounting.inspect().state, 'unknown');
});
test('snapshot corruption preserves already live PTY data, resize, pause, resume and ping', async (t) => {
  const f = await fixture(t); const c = await authorized(f); await redeem(f, c);
  corrupt(f.accounting); const requests = f.requests.length;
  c.send({ terminalToken: TOKEN }); await f.flush();
  c.send({ message: 'public input', resize: { cols: 101, rows: 41 }, pause: true, resume: true, ping: true });
  f.spawns[0].emit('data', 'public output'); await f.flush();
  assert.deepEqual(f.spawns[0].writes, ['public input']);
  assert.deepEqual(f.spawns[0].sizes, [{ cols: 101, rows: 41 }]);
  assert.equal(c.ws.sent.at(-1), 'public output'); assert.equal(f.requests.length, requests);
  assert.equal(c.ws.closes.length, 0); assert.equal(c.ws.terminations, 0);
  assert.equal(f.accounting.inspect().state, 'unknown'); assert.equal(f.errors.length, 0);
});
test('added raw close listener failure latches sentinel without closing existing live WS or PTY', async (t) => {
  const f = await fixture(t); const c = await authorized(f); await redeem(f, c);
  const requests = f.requests.length;
  const failed = f.connect({ failCloseListener: true }); await f.flush();
  assert.equal(failed.ws, undefined); assert.equal(failed.socket.destroyed, false);
  assert.equal(f.errors.length, 0); assert.equal(f.requests.length, requests);
  c.send({ message: 'public existing input', ping: true });
  c.send({ terminalToken: TOKEN }); await f.flush();
  assert.deepEqual(f.spawns[0].writes, ['public existing input']);
  assert.equal(c.ws.closes.length, 0); assert.equal(c.ws.terminations, 0);
  assert.equal(f.requests.length, requests); positive(f, 'ws', 'pty');
});
test('added PTY exit listener failure latches sentinel without rejecting valid sessions or killing PTYs', async (t) => {
  const f = await fixture(t); const a = await authorized(f); await redeem(f, a);
  const b = await authorized(f); f.failNextExitListener(); await redeem(f, b);
  assert.equal(f.spawns.length, 2); assert.equal(counts(f).pty, 2);
  assert.equal(b.ws.closes.length, 0); assert.equal(b.ws.terminations, 0);
  assert.equal(b.ws.sent.includes('pty-ready'), false);
  assert.equal(f.errors.length, 0); positive(f, 'command', 'token_redemption');
  a.send({ message: 'public existing input', ping: true });
  assert.deepEqual(f.spawns[0].writes, ['public existing input']);
  assert.deepEqual(f.spawns[1].writes, []);
  const requests = f.requests.length; b.send({ terminalToken: TOKEN }); await f.flush();
  assert.equal(f.requests.length, requests); assert.equal(a.ws.closes.length, 0);
});
test('fault after successful command admission is caught by final pre-spawn health guard', async (t) => {
  const real = realAccounting(t); let commandAdmitted = false;
  const accounting = Object.freeze({ inspect: () => real.inspect(), end: token => real.end(token),
    begin(kind) { const token = real.begin(kind); if (kind === 'command') { commandAdmitted = true; fault(real); } return token; },
  });
  const f = await createFixture(await loadFixedSources(), accounting); const c = await authorized(f);
  await redeem(f, c); assert.equal(commandAdmitted, true); assert.equal(f.spawns.length, 0);
  assert.equal(c.ws.closes.length, 0); assert.equal(c.ws.terminations, 0); assert.equal(real.inspect().state, 'unknown');
  assert.equal(f.errors.length, 0);
});
test('real token endpoint errors keep original rejection messages and accounting terminate deadline', async (t) => {
  const f = await fixture(t); const c = await authorized(f);
  c.send({ terminalToken: TOKEN }); await f.flush(); f.failRequest('/terminal/session'); await f.flush();
  assert.equal(c.ws.closes[0].code, 4403);
  assert.ok(c.ws.sent.includes('Unauthorized: Terminal token was rejected'));
  positive(f, 'ws', 'kill_timer'); assert.equal(counts(f).token_redemption, 0);
  await f.advance(2000); assert.equal(f.accounting.inspect().pending, 0);
});
for (const stage of ['first auth', 'second auth', 'buffer handoff', 'redemption', 'before spawn', 'timer registration', 'release']) {
  test(`real T1 pending-file fault at ${stage} prevents failed admission and accounting cleanup`, async (t) => {
    const f = await fixture(t); let c;
    if (stage === 'first auth') {
      c = f.connect(); await f.flush(); fault(f.accounting);
      f.reply('/terminal/auth', 200, null);
    } else if (stage === 'second auth' || stage === 'buffer handoff') {
      c = await first(f); if (stage === 'buffer handoff') c.send({ terminalToken: TOKEN });
      fault(f.accounting); f.reply('/terminal/auth/ips', 200, { ipAddresses: ['127.0.0.1'] });
    } else {
      c = await authorized(f);
      if (stage === 'before spawn') {
        // T1 is faulted when the command begin checks its snapshot, before spawn.
        c.send({ terminalToken: TOKEN }); await f.flush(); fault(f.accounting);
        f.reply('/terminal/session', 200, { command: VALID_COMMAND });
      } else if (stage === 'timer registration') {
        f.beforeSpawn(() => fault(f.accounting)); await redeem(f, c);
      } else if (stage === 'release') {
        await redeem(f, c); await redeem(f, c);
        await f.advance(4 * 500);
        // After the fifth real vendor retry callback returns, fail its release.
        // The original physical PTY is still active and its ordinary I/O works.
        f.driver.afterNextCallback(() => fault(f.accounting));
        await f.advance(500);
      } else {
        fault(f.accounting); c.send({ terminalToken: TOKEN });
      }
    }
    await f.flush(); assert.equal(f.accounting.inspect().state, 'unknown');
    if (c.ws) {
      assert.equal(c.ws.closes.length, 0); assert.equal(c.ws.terminations, 0);
      if (stage !== 'timer registration' && stage !== 'release') assert.equal(f.spawns.length, 0);
      if (stage === 'timer registration' || stage === 'release') {
        const writes = f.spawns[0].writes.length;
        c.send({ message: 'public input', resize: { cols: 100, rows: 40 }, pause: true, resume: true, ping: true });
        assert.equal(f.spawns[0].writes.length, writes + 1);
        assert.equal(f.spawns[0].pauses, 1); assert.equal(f.spawns[0].resumes, 1);
        assert.equal(c.ws.sent.at(-1), 'pong');
      }
      for (let i = 0; i < 300; i++) c.send({ terminalToken: TOKEN });
      await f.flush(); assert.equal(c.ws.closes.length, 0);
    }
    assert.equal(f.errors.length, 0);
  });
}
