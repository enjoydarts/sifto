// The standalone Docker runner is tested independently of Vercel hosting.
const assert = require('node:assert/strict');
const fs = require('node:fs');
const os = require('node:os');
const path = require('node:path');
const { spawn } = require('node:child_process');

(async () => {
  assert.equal(process.getuid(), 1000);
  assert.equal(process.getgid(), 1000);
  assert.equal(os.homedir(), '/home/node');
  fs.accessSync('/app/server.js', fs.constants.R_OK);
  assert.throws(() => fs.accessSync('/app/server.js', fs.constants.W_OK));
  const directory = fs.mkdtempSync('/app/.next/cache/runtime-ci-');
  const tempDirectory = fs.mkdtempSync(path.join(os.tmpdir(), 'runtime-ci-'));
  try {
    for (const root of [directory, tempDirectory]) {
      const probe = path.join(root, 'probe');
      fs.writeFileSync(probe, 'runtime-check');
      assert.equal(fs.readFileSync(probe, 'utf8'), 'runtime-check');
      assert.equal(fs.statSync(probe).uid, 1000);
    }
    const server = spawn('node', ['server.js'], {
      cwd: '/app', env: { ...process.env, HOSTNAME: '127.0.0.1', PORT: '3000' },
      stdio: 'ignore',
    });
    const exited = new Promise((resolve, reject) => {
      server.once('error', reject);
      server.once('exit', resolve);
    });
    try {
      let ready = false;
      for (let attempt = 0; attempt < 120; attempt++) {
        assert.equal(server.exitCode, null, 'Standalone server exited before becoming ready');
        try {
          const response = await fetch('http://127.0.0.1:3000/logo.png', { signal: AbortSignal.timeout(1000) });
          assert.equal(response.status, 200);
          assert.ok((await response.arrayBuffer()).byteLength > 0);
          ready = true;
          break;
        } catch (error) {
          if (error instanceof assert.AssertionError) throw error;
          await new Promise(resolve => setTimeout(resolve, 500));
        }
      }
      assert.ok(ready, 'Standalone server did not become ready');
      const status = fs.readFileSync(`/proc/${server.pid}/status`, 'utf8').split('\n');
      for (const field of ['Uid:', 'Gid:']) {
        assert.deepEqual(status.find(line => line.startsWith(field)).trim().split(/\s+/).slice(1), ['1000', '1000', '1000', '1000']);
      }
    } finally {
      server.kill('SIGTERM');
      const forced = setTimeout(() => server.kill('SIGKILL'), 10000);
      await exited;
      clearTimeout(forced);
    }
  } finally {
    fs.rmSync(directory, { recursive: true, force: true });
    fs.rmSync(tempDirectory, { recursive: true, force: true });
  }
  console.log('Web uid/gid, temp, Next.js cache, and standalone static serving verified');
})().catch(error => { console.error(error); process.exitCode = 1; });
