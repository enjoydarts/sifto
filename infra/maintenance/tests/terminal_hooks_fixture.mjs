import fs from 'node:fs';
import vm from 'node:vm';
import { EventEmitter } from 'node:events';
import { execFileSync } from 'node:child_process';
import { instrumentTerminal } from '../startup-terminal/instrument-terminal.mjs';

export const VALID_COMMAND = "ssh -i /var/www/html/storage/app/ssh/keys/ssh_key@public -p 22 -o StrictHostKeyChecking=no -o UserKnownHostsFile=/dev/null -o PasswordAuthentication=no -o ConnectTimeout=10 -o ServerAliveInterval=10 -o RequestTTY=yes -o LogLevel=ERROR root@127.0.0.1 'sh' << \\EOF\necho public\nEOF";
export async function loadFixedSources(variant = 'old') {
  if (!['old', 'target'].includes(variant)) throw new TypeError('invalid public source variant');
  const archive = `/src/${variant}-extra-attempt-01.tar`;
  const read = member => execFileSync('tar', ['-xOf', archive, member], { maxBuffer: 65536 });
  const source = { server: read('terminal/terminal-server.js'), utils: read('terminal/terminal-utils.js') };
  instrumentTerminal(source);
  return source;
}

export function timerDriver() {
  let now = 0;
  let id = 0;
  let failure = false;
  let afterCallback = () => {};
  const entries = new Map();
  const errors = [];
  const flush = async () => { for (let i = 0; i < 40; i++) await Promise.resolve(); };
  function register(callback, ms, args, interval) {
    if (failure) { failure = false; throw new Error('public timer registration failure'); }
    const handle = { id: ++id, unreferenced: false, unref() { this.unreferenced = true; return this; } };
    entries.set(handle, { callback, at: now + ms, ms, args, interval });
    return handle;
  }
  const clear = handle => entries.delete(handle);
  return {
    entries, errors, flush,
    failNext() { failure = true; },
    afterNextCallback(callback) { afterCallback = callback; },
    setTimeout: (callback, ms, ...args) => register(callback, ms, args, false),
    clearTimeout: clear,
    setInterval: (callback, ms, ...args) => register(callback, ms, args, true),
    clearInterval: clear,
    async advance(ms) {
      const until = now + ms;
      let steps = 0;
      for (;;) {
        const due = [...entries].filter(([, e]) => e.at <= until)
          .sort((a, b) => a[1].at - b[1].at || a[0].id - b[0].id)[0];
        if (!due) break;
        if (++steps > 10000) throw new Error('public timer fixture exceeded finite step bound');
        const [handle, entry] = due;
        now = entry.at;
        if (entry.interval) entry.at += entry.ms;
        else entries.delete(handle);
        try { entry.callback(...entry.args); } catch (error) { errors.push(error); }
        const after = afterCallback; afterCallback = () => {}; after();
        await flush();
      }
      now = until;
      await flush();
    },
  };
}

export async function createFixture(sources, accounting, { start = true, MapConstructor = Map } = {}) {
  const generated = instrumentTerminal(sources);
  const driver = timerDriver();
  const requests = [];
  const spawns = [];
  const servers = [];
  const socketServers = [];
  let beforeSpawn = () => {};
  let failExitListener = false;
  const context = vm.createContext({ Buffer, console, URL, Promise, Map: MapConstructor,
    process: { env: { HOME: '/tmp' } },
    setTimeout: driver.setTimeout, clearTimeout: driver.clearTimeout,
    setInterval: driver.setInterval, clearInterval: driver.clearInterval,
    queueMicrotask: callback => queueMicrotask(() => {
      try { callback(); } catch (error) { driver.errors.push(error); }
    }),
  });
  const http = {
    createServer(handler) {
      const server = new EventEmitter();
      server.handler = handler;
      server.listen = (port, callback) => { server.port = port; callback(); };
      servers.push(server);
      return server;
    },
    request(options, callback) {
      const request = new EventEmitter();
      request.end = body => { requests.push({ ...options, body, callback, request, replied: false }); };
      return request;
    },
  };
  function closeRaw(raw, code = 1000, reason = '') {
    if (raw.destroyed) return;
    raw.destroyed = true;
    raw.emit('close');
    const ws = raw.ws;
    if (ws && ws.readyState !== 3) {
      ws.readyState = 3;
      ws.owner.clients.delete(ws);
      ws.emit('close', code, Buffer.from(reason));
    }
  }
  class WebSocketServer extends EventEmitter {
    constructor(options) { super(); this.options = options; this.clients = new Set(); socketServers.push(this); }
    shouldHandle(req) { return req.url.split('?')[0] === this.options.path; }
    handleUpgrade(req, raw, head, callback) {
      const ws = new EventEmitter();
      ws.owner = this;
      ws.readyState = 1;
      ws.sent = [];
      ws.closes = [];
      ws.terminations = 0;
      ws.send = value => { if (ws.sendError) throw ws.sendError; ws.sent.push(value); };
      ws.close = (code, reason) => { ws.closes.push({ code, reason }); ws.readyState = 2; };
      ws.terminate = () => { ws.terminations++; closeRaw(raw); };
      ws.ping = () => { ws.pings = (ws.pings || 0) + 1; ws.emit('pong'); };
      raw.ws = ws;
      this.clients.add(ws);
      callback(ws);
    }
  }
  const pty = {
    spawn(file, args, options) {
      beforeSpawn();
      const child = new EventEmitter();
      Object.assign(child, { file, args, options, writes: [], sizes: [], pauses: 0, resumes: 0 });
      child.write = value => child.writes.push(value);
      child.resize = (cols, rows) => child.sizes.push({ cols, rows });
      child.pause = () => child.pauses++;
      child.resume = () => child.resumes++;
      child.onData = callback => { child.on('data', callback); };
      child.onExit = callback => {
        if (failExitListener) { failExitListener = false; throw new Error('public added exit listener failure'); }
        child.on('exit', callback);
      };
      child.exit = event => child.emit('exit', event ?? { exitCode: 0, signal: 0 });
      spawns.push(child);
      return child;
    },
  };
  const synthetic = (name, exports) => new vm.SyntheticModule(Object.keys(exports), function () {
    for (const [key, value] of Object.entries(exports)) this.setExport(key, value);
  }, { context, identifier: name });
  const modules = new Map([
    ['http', synthetic('http', { default: http })],
    ['ws', synthetic('ws', { WebSocketServer })],
    ['node-pty', synthetic('node-pty', { default: pty })],
    ['cookie', synthetic('cookie', { parseCookie: value => Object.fromEntries(value.split(';').filter(Boolean).map(pair => {
      const at = pair.indexOf('='); return [pair.slice(0, at).trim(), pair.slice(at + 1).trim()];
    })) })],
    ['dotenv/config', synthetic('dotenv/config', {})],
    ['./terminal-hooks.mjs', new vm.SourceTextModule(fs.readFileSync(new URL('../startup-terminal/terminal-hooks.mjs', import.meta.url), 'utf8'), { context })],
    ['./terminal-utils.js', new vm.SourceTextModule(generated.utils, { context })],
  ]);
  const serverModule = new vm.SourceTextModule(generated.server, { context });
  const linker = specifier => {
    const module = modules.get(specifier);
    if (!module) throw new Error(`unexpected fixture import ${specifier}`);
    return module;
  };
  await serverModule.link(linker);
  await serverModule.evaluate();
  const create = owner => serverModule.namespace.createTerminalServer(owner);
  const result = start ? create(accounting) : {};
  return {
    ...result, create, servers, socketServers, accounting, spawns, requests, driver,
    errors: driver.errors, flush: driver.flush, advance: driver.advance,
    beforeSpawn(callback) { beforeSpawn = callback; },
    failNextExitListener() { failExitListener = true; },
    connect({ cookie = 'XSRF-TOKEN=public-xsrf; laravel_session=public-session', path = '/terminal/ws', failCloseListener = false } = {}) {
      const socket = new EventEmitter();
      socket.destroyed = false;
      socket.remoteAddress = '127.0.0.1';
      socket.destroy = () => closeRaw(socket);
      if (failCloseListener) {
        const once = socket.once;
        socket.once = function (event, callback) {
          if (event === 'close') throw new Error('public added close listener failure');
          return once.call(this, event, callback);
        };
      }
      const req = { url: path, headers: { cookie, origin: 'https://public.invalid' }, socket };
      result.server.emit('upgrade', req, socket, Buffer.alloc(0));
      return { socket, get ws() { return socket.ws; },
        send(value) { socket.ws.emit('message', typeof value === 'string' ? value : JSON.stringify(value)); },
        close() { closeRaw(socket); },
      };
    },
    closeTransport(connection) { closeRaw(connection.socket); },
    reply(path, status, data, { raw = false } = {}) {
      const request = requests.find(request => request.path === path && !request.replied);
      if (!request) throw new Error(`no outstanding PUBLIC request ${path}`);
      request.replied = true;
      const response = new EventEmitter();
      response.statusCode = status;
      response.headers = { 'content-type': 'application/json' };
      response.setEncoding = () => {};
      request.callback(response);
      if (data !== null) response.emit('data', raw ? data : JSON.stringify(data));
      response.emit('end');
      return request;
    },
    failRequest(path) {
      const request = requests.find(request => request.path === path && !request.replied);
      if (!request) throw new Error(`no outstanding PUBLIC request ${path}`);
      request.replied = true;
      request.request.emit('error', new Error('public HTTP failure'));
    },
  };
}
