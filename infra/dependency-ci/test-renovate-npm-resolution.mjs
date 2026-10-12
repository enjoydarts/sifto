// Public-registry fixture only: no GitHub requests, credentials, or lifecycle scripts.
import assert from 'node:assert/strict';
import { createHash } from 'node:crypto';
import { execFileSync, spawnSync } from 'node:child_process';
import { existsSync, mkdirSync, mkdtempSync, readFileSync, rmSync, writeFileSync } from 'node:fs';
import { tmpdir } from 'node:os';
import { join } from 'node:path';

const config = JSON.parse(readFileSync(process.argv[2] ?? '/fixtures/renovate.json', 'utf8'));
const version = '12.2.0';
const registry = 'https://registry.npmjs.org/';
const npmrc = 'registry=https://registry.npmjs.org/\nallow-remote=none\nallow-git=none\nallow-file=none\nallow-directory=none\nignore-scripts=true\nreplace-registry-host=never\n';
const root = mkdtempSync(join(tmpdir(), 'renovate-npm-resolution-'));
try {
  const metadataResponse = await fetch(`${registry}npm/${version}`, { signal: AbortSignal.timeout(30000) });
  assert.equal(metadataResponse.status, 200);
  const metadata = await metadataResponse.json();
  assert.equal(metadata.version, version);
  assert.equal(metadata.dist.tarball, `${registry}npm/-/npm-${version}.tgz`);
  assert.match(metadata.dist.integrity, /^sha512-[A-Za-z0-9+/]+={0,2}$/);
  const archiveResponse = await fetch(metadata.dist.tarball, { signal: AbortSignal.timeout(30000) });
  assert.equal(archiveResponse.status, 200);
  const archive = Buffer.from(await archiveResponse.arrayBuffer());
  assert.equal(`sha512-${createHash('sha512').update(archive).digest('base64')}`, metadata.dist.integrity);
  const tool = join(root, 'tool');
  mkdirSync(tool);
  writeFileSync(join(root, 'npm.tgz'), archive);
  execFileSync('tar', ['-xzf', join(root, 'npm.tgz'), '-C', tool, '--strip-components=1']);
  const cli = join(tool, 'bin/npm-cli.js');
  const home = join(root, 'home');
  mkdirSync(home);
  writeFileSync(join(home, 'npmrc'), '');
  writeFileSync(join(home, 'global-npmrc'), '');
  const env = { PATH: '/usr/local/bin:/usr/bin:/bin', HOME: home,
    NPM_CONFIG_USERCONFIG: join(home, 'npmrc'), NPM_CONFIG_GLOBALCONFIG: join(home, 'global-npmrc'),
    NPM_CONFIG_CACHE: join(root, 'cache'), TMPDIR: root };
  assert.equal(execFileSync(process.execPath, [cli, '--version'], { env, encoding: 'utf8' }).trim(), version);
  const cutoff = new Date(Date.now() - 7 * 86400000).toISOString();
  function install(name, dependencies, lockOnly, settings = npmrc, optionalBundle = false, seededLock) {
    const directory = join(root, name);
    mkdirSync(directory);
    writeFileSync(join(directory, '.npmrc'), settings);
    writeFileSync(join(directory, 'package.json'), JSON.stringify({ name: 'public-registry-fixture', version: '1.0.0',
      [optionalBundle ? 'optionalDependencies' : 'dependencies']: dependencies,
      scripts: { preinstall: 'node -e "require(\'fs\').writeFileSync(\'scripts-ran\',\'bad\')"' } }));
    if (seededLock) writeFileSync(join(directory, 'package-lock.json'), JSON.stringify(seededLock));
    const args = [cli, 'install', '--no-audit', '--no-fund', '--ignore-scripts', '--cpu=wasm32', `--before=${cutoff}`];
    if (lockOnly) args.push('--package-lock-only');
    const result = spawnSync(process.execPath, args, { cwd: directory, env, encoding: 'utf8', timeout: 180000 });
    assert.ifError(result.error);
    assert.equal(existsSync(join(directory, 'scripts-ran')), false);
    return { directory, result };
  }
  const dependency = { '@tailwindcss/oxide-wasm32-wasi': '4.3.3' };
  const red = install('lock-only', dependency, true, npmrc, true);
  assert.notEqual(red.result.status, 0, 'The pinned lock-only bundle regression must reproduce');
  assert.match(red.result.stderr, /EALLOWREMOTE/);
  assert.match(red.result.stderr, /registry\.npmjs\.org\/@tailwindcss\/oxide-wasm32-wasi/);
  console.log('RED reproduced: npm lock-only rejects the official bundled oxide tarball');

  assert.equal(config.npm?.skipInstalls, false, 'Renovate npm must use normal registry reification');
  assert.equal(config.npm.ignoreScripts, true);
  assert.equal(config.npm.npmrcMerge, false);
  assert.equal(config.npm.npmrc, npmrc);
  assert.equal(config.npm.constraints.npm, version);
  assert.equal(config.npm.constraints.node, '22.23.3');
  assert.equal(config.minimumReleaseAge, '7 days');
  assert.equal(config.automerge, false);
  const green = install('normal-install', dependency, config.npm.skipInstalls !== false, config.npm.npmrc, true);
  assert.equal(green.result.status, 0, green.result.stderr);
  const lock = JSON.parse(readFileSync(join(green.directory, 'package-lock.json'), 'utf8'));
  assert.equal(lock.lockfileVersion, 3);
  assert.equal(lock.packages['node_modules/@tailwindcss/oxide-wasm32-wasi'].version, '4.3.3');
  const installed = join(green.directory, 'node_modules/@tailwindcss/oxide-wasm32-wasi');
  const manifest = JSON.parse(readFileSync(join(installed, 'package.json'), 'utf8'));
  const bundled = manifest.bundleDependencies ?? manifest.bundledDependencies;
  assert.ok(Array.isArray(bundled) && bundled.length > 0);
  for (const name of bundled) assert.ok(existsSync(join(installed, 'node_modules', name, 'package.json')), `Missing extracted bundle ${name}`);
  for (const entry of Object.values(lock.packages)) {
    if (entry.resolved) assert.match(entry.resolved, /^https:\/\/registry\.npmjs\.org\/[a-zA-Z0-9@%/_.+-]+\.tgz$/);
  }
  console.log('GREEN: normal npm install regenerates lock v3 and extracts the registry bundle with scripts disabled');
  for (const [name, spec, code] of [
    ['remote', 'https://example.invalid/package.tgz', 'EALLOWREMOTE'],
    ['direct-registry-url', `${registry}is-number/-/is-number-7.0.0.tgz`, 'EALLOWREMOTE'],
    ['git', 'git+https://example.invalid/package.git', 'EALLOWGIT'],
    ['local-file', 'file:/tmp/not-a-package.tgz', 'EALLOWFILE'],
    ['local-directory', 'file:/tmp/not-a-directory', 'EALLOWDIRECTORY'],
  ]) {
    const denied = install(name, { denied: spec }, false, config.npm.npmrc);
    assert.notEqual(denied.result.status, 0);
    assert.ok(denied.result.stderr.includes(code), denied.result.stderr);
  }
  const redirected = install('registry-metadata-other-origin', { 'is-number': '7.0.0' }, false, config.npm.npmrc, false, {
    name: 'public-registry-fixture', version: '1.0.0', lockfileVersion: 3,
    packages: {
      '': { name: 'public-registry-fixture', version: '1.0.0', dependencies: { 'is-number': '7.0.0' } },
      'node_modules/is-number': { version: '7.0.0', resolved: 'https://example.invalid/is-number-7.0.0.tgz' },
    },
  });
  assert.notEqual(redirected.result.status, 0);
  assert.match(redirected.result.stderr, /EALLOWREMOTE/);
  console.log('Non-registry remote/git/local transports remain denied; owner gate and release age remain configured');
} finally {
  rmSync(root, { recursive: true, force: true });
}
