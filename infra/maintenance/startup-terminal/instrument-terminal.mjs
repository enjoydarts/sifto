import { createHash } from 'node:crypto';

const pins = Object.freeze({
  server: '3196d818d6d084cd9364c561f0d305953d2f71fc9622e6d5084fb4f02bef24c7',
  utils: '2a605d59ae58e4832370adc33419127e6337a4cb1ca4a5257b9acb2f83946790',
});
function replaceOnce(source, before, after) {
  const at = source.indexOf(before);
  if (at < 0 || source.indexOf(before, at + before.length) >= 0) {
    throw new Error('terminal patch anchor mismatch');
  }
  return source.slice(0, at) + after + source.slice(at + before.length);
}
const boundary = `const boundaryError = error => {
    if (!hooks.isUnavailable(error)) queueMicrotask(() => { throw error; });
};`;

export function instrumentTerminal(source) {
  if (!source || Reflect.ownKeys(source).length !== 2 ||
      Object.keys(source).sort().join(',') !== 'server,utils') {
    throw new TypeError('invalid terminal source pair');
  }
  // Both original byte strings are admitted before either transformation.
  const admitted = {};
  for (const name of ['server', 'utils']) {
    const descriptor = Object.getOwnPropertyDescriptor(source, name);
    if (!descriptor || !Object.hasOwn(descriptor, 'value') || !Buffer.isBuffer(descriptor.value)) {
      throw new Error('unsupported terminal source');
    }
    admitted[name] = Buffer.from(descriptor.value);
  }
  for (const name of ['server', 'utils']) {
    if (createHash('sha256').update(admitted[name]).digest('hex') !== pins[name]) {
      throw new Error('unsupported terminal source');
    }
  }
  let server = admitted.server.toString('utf8');
  let utils = admitted.utils.toString('utf8');
  const s = (before, after) => { server = replaceOnce(server, before, after); };
  const u = (before, after) => { utils = replaceOnce(utils, before, after); };
  s('    rejectTerminalSocket,', '    rejectTerminalSocket as vendorRejectTerminalSocket,');
  s('async function postToCoolify(path, headers, body = null) {', `import { createTerminalHooks } from './terminal-hooks.mjs';

export function createTerminalServer(accounting) {
const hooks = createTerminalHooks(accounting);
const setTimeout = hooks.setTimeout;
const clearTimeout = hooks.clearTimeout;
const rejectTerminalSocket = (ws, code, reason, options = {}) =>
    vendorRejectTerminalSocket(ws, code, reason, { ...options, hooks });
${boundary}

async function postToCoolify(path, headers, body = null) {`);
  s("createTerminalUpgradeHandler({ wss, authenticate: verifyClient })", "createTerminalUpgradeHandler({ wss, authenticate: verifyClient, hooks })");
  s("wss.on('connection', async (ws, req) => {", `wss.on('connection', (ws, req) => {
    hooks.run('second_auth', () => onConnection(ws, req)).catch(boundaryError);
});
async function onConnection(ws, req) {`);
  s('        authReady: false,', '        authReady: false,\n        closed: false,');
  s('    const connectionContext = {', '    let unavailable = false;\n    try {\n    const connectionContext = {');
  s(`    ws.on('message', (message) => {
        if (userSession.authReady) {
            handleMessage(userSession, message);
        } else {
            userSession.pendingMessages.push(message);
        }
    });`, `    ws.on('message', (message) => {
        try {
            if (userSession.closed || ws.readyState !== 1) return;
            if (userSession.authReady) {
                handleMessage(userSession, message);
            } else {
                userSession.pendingMessages.push(hooks.buffer(message));
            }
        } catch (error) { boundaryError(error); }
    });`);
  s("ws.on('error', (err) => handleError(err, userId));", "ws.on('error', (err) => { handleError(err, userId).catch(boundaryError); });");
  s("    ws.on('close', (code, reason) => {", "    ws.on('close', (code, reason) => {\n        userSession.closed = true;");
  s('        handleClose(userId);', '        handleClose(userId).catch(boundaryError);');
  s(`    } catch (error) {
        logTerminal('error', 'Failed to fetch authorized terminal hosts.', {`, `    } catch (error) {
        if (hooks.isUnavailable(error)) throw error;
        logTerminal('error', 'Failed to fetch authorized terminal hosts.', {`);
  s('    userSessions.set(userId, userSession);', '    if (userSession.closed || ws.readyState !== 1) return;\n    userSessions.set(userId, userSession);');
  s(`        handleMessage(userSession, userSession.pendingMessages.shift());
    }
});`, `        hooks.consume(userSession.pendingMessages.shift(), message => handleMessage(userSession, message));
    }
    } catch (error) {
        unavailable = hooks.isUnavailable(error);
        throw error;
    } finally {
        if (!unavailable) {
            while (userSession.pendingMessages.length) hooks.discard(userSession.pendingMessages.shift());
        }
    }
}`);
  s('            killPtyProcess(session.userId);', '            killPtyProcess(session.userId).catch(boundaryError);');
  s('terminalToken: (session, token) => handleTerminalToken(session, token)', 'terminalToken: (session, token) => { handleTerminalToken(session, token).catch(boundaryError); }');
  s('async function handleCommand(ws, command, userId) {', `function liveSession(session, ws) {
    return !!session && !session.closed && session.authReady && session.ws === ws &&
        ws.readyState === 1 && userSessions.get(session.userId) === session;
}
function handleCommand(ws, command, userId, expectedSession) {
    return hooks.run('command', () => handleCommandBody(ws, command, userId, expectedSession));
}
async function handleCommandBody(ws, command, userId, expectedSession) {`);
  s(`    const userSession = userSessions.get(userId);
    if (userSession && userSession.isActive) {`, `    const userSession = userSessions.get(userId);
    if (userSession !== expectedSession || !liveSession(userSession, ws)) return;
    if (userSession && userSession.isActive) {`);
  s('        const result = await killPtyProcess(userId);', '        const result = await killPtyProcess(userId);\n        if (userSession !== expectedSession || !liveSession(userSession, ws)) return;');
  s("    const ptyProcess = pty.spawn('ssh', sanitizedSshArgs.concat([hereDocContent]), options);", `    if (userSession !== expectedSession || !liveSession(userSession, ws)) return;
    if (!isAuthorizedTargetHost(targetHost, userSession.authorizedIPs) ||
        !validateSshArgs(sshArgs, userSession.authorizedIPs)) return;
    hooks.assertHealthy();
    const ptyProcess = hooks.spawn(() =>
        pty.spawn('ssh', sanitizedSshArgs.concat([hereDocContent]), options));`);
  s('async function handleTerminalToken(userSession, token) {', `function handleTerminalToken(userSession, token) {
    return hooks.run('token_redemption', () => handleTerminalTokenBody(userSession, token));
}
async function handleTerminalTokenBody(userSession, token) {`);
  s('        await handleCommand(userSession.ws, [response.data.command], userSession.userId);', '        if (!liveSession(userSession, userSession.ws)) return;\n        await handleCommand(userSession.ws, [response.data.command], userSession.userId, userSession);');
  s(`    } catch (error) {
        logTerminal('error', 'Failed to redeem terminal token.', {`, `    } catch (error) {
        if (hooks.isUnavailable(error)) throw error;
        logTerminal('error', 'Failed to redeem terminal token.', {`);
  // Exact final listen block is the factory terminator, not a module side effect.
  s(`server.listen(6002, () => {
    logTerminal('log', 'Terminal debug logging is enabled.', {
        terminalDebugEnabled,
    });
});`, `server.listen(6002, () => {
    logTerminal('log', 'Terminal debug logging is enabled.', {
        terminalDebugEnabled,
    });
});
return { server, wss };
}`);
  u('terminateAfterMs = TERMINAL_REJECTED_SOCKET_TERMINATE_MS } = {}) {', 'terminateAfterMs = TERMINAL_REJECTED_SOCKET_TERMINATE_MS, hooks } = {}) {');
  u('const terminateTimer = setTimeout(() => ws.terminate(), terminateAfterMs);', 'const terminateTimer = hooks.setTimeout(() => ws.terminate(), terminateAfterMs);');
  u("ws.once('close', () => clearTimeout(terminateTimer));", "ws.once('close', () => hooks.clearTimeout(terminateTimer));");
  u('export function createTerminalUpgradeHandler({ wss, authenticate, onRejected = () => {}, terminateAfterMs = TERMINAL_REJECTED_SOCKET_TERMINATE_MS }) {', `export function createTerminalUpgradeHandler({ wss, authenticate, hooks, onRejected = () => {}, terminateAfterMs = TERMINAL_REJECTED_SOCKET_TERMINATE_MS }) {
    ${boundary}`);
  u('    return (req, socket, head) => {', `    return (req, socket, head) => {
        try {
        hooks.socket(socket);
        return hooks.run('first_auth', async () => {`);
  u('        Promise.resolve()\n            .then(() => authenticate(req))', '        return Promise.resolve()\n            .then(() => authenticate(req))');
  u(`            .catch((error) => ({
                authenticated: false,
                closeCode: TERMINAL_CLOSE_CODES.AUTH_UNAVAILABLE,
                reason: 'Internal Server Error',
                error,
            }))`, `            .catch((error) => {
                if (hooks.isUnavailable(error)) throw error;
                return {
                    authenticated: false,
                    closeCode: TERMINAL_CLOSE_CODES.AUTH_UNAVAILABLE,
                    reason: 'Internal Server Error',
                    error,
                };
            })`);
  u('rejectTerminalSocket(ws, result.closeCode, result.reason, { terminateAfterMs });', 'rejectTerminalSocket(ws, result.closeCode, result.reason, { terminateAfterMs, hooks });');
  u(`            });
    };
}

const DEFAULT_TERMINAL_PATH`, `            });
        }).catch(boundaryError);
        } catch (error) { boundaryError(error); }
    };
}

const DEFAULT_TERMINAL_PATH`);
  return Object.freeze({ server, utils });
}
