// Finite source-owned lifetimes. There is deliberately no recovery or adoption.
const owners = new WeakSet();

export function createTerminalHooks(accounting) {
  const unavailable = Object.freeze(new Error('terminal accounting unavailable'));
  let broken = false;
  const leases = new WeakMap();
  const timers = new Map();
  const held = new Map();
  const never = new Promise(() => {});
  const isUnavailable = error => error === unavailable;
  const latch = () => { broken = true; return unavailable; };
  function assertHealthy() {
    if (broken) throw unavailable;
    try {
      if (accounting.inspect().state !== 'healthy') throw unavailable;
    } catch { throw latch(); }
  }
  // One synchronous owner claim precedes every listener, server, or timer.
  try {
    if (!accounting || owners.has(accounting) ||
        accounting.inspect().state !== 'healthy' || accounting.inspect().pending !== 0) {
      throw unavailable;
    }
    owners.add(accounting);
  } catch { throw latch(); }

  function hold(lease, continuation) {
    // Only successfully admitted roots enter this map (T1 bounds these at 256).
    const entry = leases.get(lease);
    if (!entry) return never;
    let slot = held.get(lease);
    if (!slot) {
      if (held.size >= 256) return never;
      slot = { entry, continuation: undefined };
      held.set(lease, slot);
    }
    // A failed release first retains bookkeeping. Its owning route can then
    // attach its one original continuation; repeated releases cannot replace it.
    if (continuation !== entry && slot.continuation === undefined) slot.continuation = continuation;
    return never;
  }
  function reserve(kind) {
    assertHealthy();
    let token;
    try { token = accounting.begin(kind); }
    catch { throw latch(); }
    const lease = Object.freeze(Object.create(null));
    leases.set(lease, { token, released: false });
    return lease;
  }
  function release(lease) {
    const entry = leases.get(lease);
    if (!entry) throw new TypeError('invalid terminal lease');
    if (entry.released) return true;
    try {
      assertHealthy();
      accounting.end(entry.token);
      entry.released = true;
      return true;
    } catch {
      latch();
      hold(lease, entry);
      return false;
    }
  }
  async function run(kind, action) {
    const lease = reserve(kind);
    try {
      const result = await action();
      if (!release(lease)) return hold(lease, action);
      return result;
    } catch (error) {
      if (isUnavailable(error) || !release(lease)) return hold(lease, action);
      throw error;
    }
  }
  function buffer(message) { return Object.freeze({ message, lease: reserve('buffered_token') }); }
  function consume(buffered, action) {
    try {
      const result = action(buffered.message);
      if (!release(buffered.lease)) throw unavailable;
      return result;
    } catch (error) {
      if (isUnavailable(error)) hold(buffered.lease, buffered);
      else if (!release(buffered.lease)) {
        hold(buffered.lease, buffered);
        throw unavailable;
      }
      throw error;
    }
  }
  function discard(buffered) {
    if (!release(buffered.lease)) hold(buffered.lease, buffered);
  }
  function socket(raw) {
    const lease = reserve('ws');
    try { raw.once('close', () => queueMicrotask(() => release(lease))); }
    catch { hold(lease, raw); throw latch(); }
    return lease;
  }
  function setTimeout(callback, ms, ...args) {
    const lease = reserve('kill_timer');
    const entry = { lease, fired: false, finished: false };
    let handle;
    function finish() {
      if (entry.finished) return;
      entry.finished = true;
      timers.delete(handle);
      if (!release(lease)) hold(lease, callback);
    }
    try {
      handle = globalThis.setTimeout(() => {
        entry.fired = true;
        let result;
        try { result = callback(...args); }
        catch (error) {
          if (isUnavailable(error)) { hold(lease, callback); return; }
          finish();
          throw error;
        }
        Promise.resolve(result).then(finish, error => {
          if (isUnavailable(error)) { hold(lease, callback); return; }
          finish();
          queueMicrotask(() => { throw error; });
        });
      }, ms);
    } catch (error) { hold(lease, callback); throw error; }
    timers.set(handle, entry);
    return handle;
  }
  function clearTimeout(handle) {
    globalThis.clearTimeout(handle);
    const entry = timers.get(handle);
    if (!entry || entry.fired || entry.finished) return;
    entry.finished = true;
    timers.delete(handle);
    if (!release(entry.lease)) hold(entry.lease, entry);
  }
  function spawn(action) {
    const lease = reserve('pty');
    let process;
    try { process = action(); }
    catch (error) { hold(lease, action); throw error; }
    try {
      process.onExit(() => queueMicrotask(() => release(lease)));
    } catch { hold(lease, process); throw latch(); }
    return process;
  }
  return Object.freeze({ run, reserve, release, socket, buffer, consume, discard,
    setTimeout, clearTimeout, spawn, isUnavailable, assertHealthy });
}
