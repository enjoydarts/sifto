// Exercise the shipped Renovate implementation; only GitHub transport is mocked.
// Compose disables networking and mounts no tokens or credentials.
import assert from 'node:assert/strict';
import { execFileSync } from 'node:child_process';
import { mkdtempSync, readFileSync, rmSync } from 'node:fs';
import { tmpdir } from 'node:os';
import { join } from 'node:path';
import { parse } from '/usr/local/renovate/node_modules/yaml/dist/index.js';
import { GlobalConfig } from '/usr/local/renovate/dist/config/global.js';
import { setPlatformApi } from '/usr/local/renovate/dist/modules/platform/index.js';
import { initRepo } from '/usr/local/renovate/dist/modules/platform/github/index.js';
import { githubApi } from '/usr/local/renovate/dist/modules/platform/github/common.js';
import { setStability } from '/usr/local/renovate/dist/workers/repository/update/branch/status-checks.js';
import { REPOSITORY_CHANGED } from '/usr/local/renovate/dist/constants/error-messages.js';
import { init as initLogger } from '/usr/local/renovate/dist/logger/index.js';

const workflow = parse(readFileSync('/fixtures/renovate.yml', 'utf8'));
const action = workflow.jobs.renovate.steps.find((step) => step.uses?.startsWith('renovatebot/'));
const installedVersion = JSON.parse(readFileSync('/usr/local/renovate/package.json', 'utf8')).version;
assert.equal(installedVersion, action.with['renovate-version']);

const directory = mkdtempSync(join(tmpdir(), 'renovate-status-'));
const branchName = 'renovate-ci/status-fixture';
const repository = 'fixture/dependency-ci';
const context = 'renovate/minimum-release-age';
const calls = [];
let currentStatuses = [];
let canWriteStatuses = false;

try {
  await initLogger();
  execFileSync('git', ['init', '--initial-branch=main', directory], { stdio: 'pipe' });
  execFileSync('git', ['-C', directory, '-c', 'user.name=Dependency CI', '-c', 'user.email=dependency-ci@example.invalid', 'commit', '--allow-empty', '-m', 'Synthetic status fixture'], { stdio: 'pipe' });
  execFileSync('git', ['-C', directory, 'branch', branchName], { stdio: 'pipe' });
  const commit = execFileSync('git', ['-C', directory, 'rev-parse', 'HEAD'], { encoding: 'utf8' }).trim();
  GlobalConfig.set({ localDir: directory, dryRun: false });
  setPlatformApi('github');

  githubApi.requestGraphql = async () => ({ data: { repository: {
    nameWithOwner: repository,
    defaultBranchRef: { name: 'main' },
    // The official initRepo performs git ls-remote against this local fixture.
    sshUrl: directory,
    issues: { nodes: [] },
    hasIssuesEnabled: false,
    autoMergeAllowed: false,
  } } });
  githubApi.getJsonUnchecked = async (url) => {
    if (url === `repos/${repository}/commits/${commit}/statuses`) return { body: currentStatuses };
    if (url === `repos/${repository}/commits/${branchName}/status`) return { body: { state: currentStatuses[0]?.state ?? 'pending', statuses: currentStatuses } };
    throw new Error(`Unexpected GitHub read: ${url}`);
  };
  githubApi.postJson = async (url, { body }) => {
    assert.equal(url, `repos/${repository}/statuses/${commit}`);
    if (!canWriteStatuses) throw Object.assign(new Error('Resource not accessible by integration'), { statusCode: 403 });
    calls.push({ url, body });
    currentStatuses = [body];
    return { body };
  };
  await initRepo({ repository, gitUrl: 'ssh', renovateUsername: 'github-actions[bot]' });
  const config = { branchName, statusCheckNames: { minimumReleaseAge: context } };

  // Missing status write access aborts both waiting and satisfied age checks.
  for (const stabilityStatus of ['yellow', 'green']) {
    await assert.rejects(setStability({ ...config, stabilityStatus }), { message: REPOSITORY_CHANGED });
  }
  assert.equal(calls.length, 0);

  canWriteStatuses = workflow.jobs.renovate.permissions.statuses === 'write';
  await setStability({ ...config, stabilityStatus: 'yellow' });
  await setStability({ ...config, stabilityStatus: 'green' });
  assert.deepEqual(calls.map(({ body }) => body.state), ['pending', 'success']);
  assert.ok(calls.every(({ body }) => body.context === context));
  assert.match(calls[0].body.description, /have not met minimum release age/);
  assert.match(calls[1].body.description, /have met minimum release age/);
  // The satisfied status is idempotent and does not need another POST.
  await setStability({ ...config, stabilityStatus: 'green' });
  assert.equal(calls.length, 2);
  console.log(`Renovate ${installedVersion}: denied status writes abort; workflow permissions publish pending then success; repeated success is idempotent.`);
} finally {
  rmSync(directory, { recursive: true, force: true });
}
