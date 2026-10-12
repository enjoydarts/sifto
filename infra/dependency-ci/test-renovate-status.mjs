// Exercise the shipped Renovate implementation; only GitHub transport is mocked.
// Compose disables networking and mounts no tokens or credentials.
import assert from 'node:assert/strict';
import { execFileSync } from 'node:child_process';
import { copyFileSync, mkdirSync, mkdtempSync, readFileSync, rmSync } from 'node:fs';
import { tmpdir } from 'node:os';
import { join } from 'node:path';
import { parse } from '/usr/local/renovate/node_modules/yaml/dist/index.js';
import { GlobalConfig } from '/usr/local/renovate/dist/config/global.js';
import { getOptions } from '/usr/local/renovate/dist/config/options/index.js';
import { setPlatformApi } from '/usr/local/renovate/dist/modules/platform/index.js';
import { initRepo } from '/usr/local/renovate/dist/modules/platform/github/index.js';
import { githubApi } from '/usr/local/renovate/dist/modules/platform/github/common.js';
import { setStability } from '/usr/local/renovate/dist/workers/repository/update/branch/status-checks.js';
import { REPOSITORY_CHANGED } from '/usr/local/renovate/dist/constants/error-messages.js';
import { init as initLogger } from '/usr/local/renovate/dist/logger/index.js';
import { applyPackageRules } from '/usr/local/renovate/dist/util/package-rules/index.js';
import { extractAllPackageFiles } from '/usr/local/renovate/dist/modules/manager/pip-compile/extract.js';
import { extractHeaderCommand } from '/usr/local/renovate/dist/modules/manager/pip-compile/common.js';
import { constructPipCompileCmd } from '/usr/local/renovate/dist/modules/manager/pip-compile/artifacts.js';
import { generateUpdate } from '/usr/local/renovate/dist/workers/repository/process/lookup/generate.js';
import { get as getVersioning } from '/usr/local/renovate/dist/modules/versioning/index.js';
import { getRangeStrategy } from '/usr/local/renovate/dist/modules/manager/index.js';

const workflow = parse(readFileSync('/fixtures/renovate.yml', 'utf8'));
const action = workflow.jobs.renovate.steps.find((step) => step.uses?.startsWith('renovatebot/'));
const installedVersion = JSON.parse(readFileSync('/usr/local/renovate/package.json', 'utf8')).version;
assert.equal(installedVersion, action.with['renovate-version']);

const directory = mkdtempSync(join(tmpdir(), 'renovate-status-'));
const branchName = 'renovate-ci/status-fixture';
const repository = 'fixture/dependency-ci';
const context = getOptions().find((option) => option.name === 'statusCheckNames').default.minimumReleaseAge;
assert.equal(context, 'renovate/stability-days');
const calls = [];
let currentStatuses = [];
let canWriteStatuses = false;

try {
  await initLogger();
  const repositoryConfig = JSON.parse(readFileSync('/fixtures/renovate.json', 'utf8'));
  GlobalConfig.set({ localDir: directory });
  const lockPaths = ['worker', 'infra/maintenance', 'infra/audio-concat'].map((path) => `${path}/requirements.txt`);
  for (const lockPath of lockPaths) {
    mkdirSync(join(directory, lockPath.replace('/requirements.txt', '')), { recursive: true });
    for (const path of [lockPath, lockPath.replace('.txt', '.in')]) copyFileSync(`/fixtures/${path}`, join(directory, path));
    const header = extractHeaderCommand(readFileSync(`/fixtures/${lockPath}`, 'utf8'), lockPath);
    assert.equal(header.commandType, 'uv');
    assert.equal(header.pythonVersion, '3.12');
    assert.deepEqual(header.sourceFiles, ['requirements.in']);
  }
  const extracted = await extractAllPackageFiles(repositoryConfig, lockPaths);
  assert.equal(extracted.length, 3);
  for (const file of extracted) {
    assert.ok(file.packageFile.endsWith('/requirements.in'));
    assert.equal(file.lockFiles.length, 1);
    assert.ok(file.deps.some((dep) => dep.depType === 'indirect' && dep.lockedVersion));
    assert.ok(file.deps.some((dep) => dep.currentValue && dep.lockedVersion));
    assert.ok(file.deps.every((dep) => dep.datasource === 'pypi'));
  }
  console.log(`Renovate ${installedVersion}: uv Python 3.12 headers extract all three direct inputs and hashed transitive dependencies.`);
  const activationGroup = 'Maintenance executor activation dependencies';
  for (const updateType of ['minor', 'patch']) {
    for (const packageFile of ['worker/requirements.in', 'worker/requirements.txt', 'infra/audio-concat/requirements.in', 'infra/audio-concat/requirements.txt', 'infra/maintenance/requirements.in', 'infra/maintenance/requirements.txt']) {
      const config = await applyPackageRules({ ...repositoryConfig, packageFile, manager: 'pip-compile', updateType });
      assert.equal(config.groupName, packageFile.startsWith('infra/maintenance/') ? activationGroup : '{{manager}} minor and patch dependencies');
      assert.equal(config.automerge, false);
    }
  }
  for (const updateType of ['digest', 'pinDigest']) {
    for (const packageFile of ['api/Dockerfile', 'infra/dependency-ci/compose.python-locks.yml', 'infra/maintenance/Dockerfile']) {
      const config = await applyPackageRules({ ...repositoryConfig, packageFile, manager: 'dockerfile', updateType });
      assert.equal(config.groupName, packageFile === 'infra/maintenance/Dockerfile' ? activationGroup : 'Docker image digests');
      assert.equal(config.automerge, false);
    }
  }
  for (const packageFile of ['worker/requirements.txt', 'infra/audio-concat/requirements.txt', 'infra/maintenance/requirements.txt']) {
    const config = await applyPackageRules({ ...repositoryConfig, packageFile, manager: 'pip-compile', depType: 'indirect', updateType: 'patch', enabled: false });
    assert.equal(config.enabled, false);
    assert.equal(config.minimumReleaseAge, '7 days');
    assert.equal(config.groupName, packageFile.startsWith('infra/maintenance/') ? activationGroup : '{{manager}} minor and patch dependencies');
  }
  const worker = extracted.find((file) => file.packageFile === 'worker/requirements.in');
  const direct = worker.deps.find((dep) => dep.depName === 'pydantic');
  const indirect = worker.deps.find((dep) => dep.depName === 'pydantic-core');
  assert.ok(direct.currentValue && direct.lockedVersion);
  assert.equal(indirect.depType, 'indirect');
  const directConfig = await applyPackageRules({ rangeStrategy: getOptions().find((option) => option.name === 'rangeStrategy').default, ...repositoryConfig, ...direct, packageFile: worker.packageFile, manager: 'pip-compile', updateType: 'minor' });
  const indirectConfig = await applyPackageRules({ ...repositoryConfig, ...indirect, packageFile: worker.packageFile, manager: 'pip-compile', updateType: 'minor' });
  assert.notEqual(directConfig.enabled, false);
  assert.equal(indirectConfig.enabled, false);
  assert.equal(directConfig.minimumReleaseAge, '7 days');
  assert.equal(directConfig.groupName, '{{manager}} minor and patch dependencies');
  const rangeStrategy = getRangeStrategy(directConfig);
  assert.equal(rangeStrategy, 'replace');
  const update = await generateUpdate(directConfig, direct.currentValue, getVersioning('pep440'), rangeStrategy, direct.lockedVersion, 'minor', { version: '2.13.5' }, new Set([direct.lockedVersion, '2.13.5']));
  assert.equal(update.newValue, '==2.13.5');
  assert.notEqual(update.isLockfileUpdate, true);
  const header = extractHeaderCommand(readFileSync('/fixtures/worker/requirements.txt', 'utf8'), 'worker/requirements.txt');
  const command = constructPipCompileCmd(header, [update].filter((dep) => dep.isLockfileUpdate));
  assert.equal(command, 'uv pip compile --python-version=3.12 --universal --generate-hashes --output-file=requirements.txt requirements.in');
  console.log(`Renovate ${installedVersion}: direct Python updates regenerate complete locks without independently forced indirect versions.`);
  console.log(`Renovate ${installedVersion}: live executor dependencies are separate from Worker/Audio minor-patch and ordinary Docker digest groups.`);
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
