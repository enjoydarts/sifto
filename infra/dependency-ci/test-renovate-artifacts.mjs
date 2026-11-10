// Exercise tool installation and updateArtifacts, not just command construction.
import assert from 'node:assert/strict';
import { execFileSync } from 'node:child_process';
import { mkdtempSync, mkdirSync, readFileSync, writeFileSync } from 'node:fs';
import { join } from 'node:path';
import { GlobalConfig } from '/usr/local/renovate/dist/config/global.js';
import { init as initLogger } from '/usr/local/renovate/dist/logger/index.js';
import { applyPackageRules } from '/usr/local/renovate/dist/util/package-rules/index.js';
import { updateArtifacts } from '/usr/local/renovate/dist/modules/manager/pip-compile/artifacts.js';
import { setCustomEnv } from '/usr/local/renovate/dist/util/env.js';
import { initRepo, getRepoStatus } from '/usr/local/renovate/dist/util/git/index.js';

await initLogger();
const localDir = mkdtempSync('/tmp/renovate-artifacts-');
GlobalConfig.set({ localDir, cacheDir: join(localDir, 'cache'), binarySource: 'install',
  allowCustomCrateRegistries: false, exposeAllEnv: false });
setCustomEnv({ UV_CONFIG_FILE: '/fixtures/uv-renovate.toml',
  UV_EXCLUDE_NEWER: new Date(Date.now() - 7 * 86400000).toISOString() });
const repositoryConfig = JSON.parse(readFileSync('/fixtures/renovate.json', 'utf8'));
execFileSync('git', ['init', '--initial-branch=main', localDir]);
execFileSync('git', ['-C', localDir, '-c', 'user.name=Fixture', '-c', 'user.email=fixture@example.invalid', 'commit', '--allow-empty', '-m', 'Fixture']);
execFileSync('git', ['-C', localDir, 'remote', 'add', 'origin', localDir]);
await initRepo({ url: localDir, defaultBranch: 'main' });
await getRepoStatus();
for (const directory of ['worker', 'infra/maintenance', 'infra/audio-concat']) {
  const packageFileName = `${directory}/requirements.in`;
  const lock = `${directory}/requirements.txt`;
  mkdirSync(join(localDir, directory), { recursive: true });
  const header = readFileSync(`/fixtures/${lock}`, 'utf8').split('\n').slice(0, 2).join('\n') + '\n';
  writeFileSync(join(localDir, packageFileName), 'six==1.16.0\n');
  writeFileSync(join(localDir, lock), header + 'six==1.16.0\n');
  execFileSync('git', ['-C', localDir, 'add', '.']);
  execFileSync('git', ['-C', localDir, '-c', 'user.name=Fixture', '-c', 'user.email=fixture@example.invalid', 'commit', '-m', directory]);
  const config = await applyPackageRules({ ...repositoryConfig, packageFile: packageFileName,
    manager: 'pip-compile', datasource: 'pypi', depName: 'six', packageName: 'six',
    lockFiles: [lock] });
  const result = await updateArtifacts({ packageFileName, newPackageFileContent: 'six==1.17.0\n',
    updatedDeps: [{ depName: 'six', newVersion: '1.17.0' }], config });
  assert.ok(result?.some((item) => item.file?.path === lock), JSON.stringify(result));
  assert.ok(!result.some((item) => item.artifactError), JSON.stringify(result));
  const generated = result.find((item) => item.file)?.file.contents;
  assert.match(generated, /six==1\.17\.0/);
  assert.match(generated, /--hash=sha256:[0-9a-f]{64}/);
  console.log(`${directory}: actual Renovate tool installation and hashed artifact update passed`);
}
