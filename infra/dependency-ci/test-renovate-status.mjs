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
import { getExecOptions } from '/usr/local/renovate/dist/modules/manager/pip-compile/common.js';
import { setCustomEnv } from '/usr/local/renovate/dist/util/env.js';
import { getChildEnv } from '/usr/local/renovate/dist/util/exec/utils.js';
import { generateUpdate } from '/usr/local/renovate/dist/workers/repository/process/lookup/generate.js';
import { get as getVersioning } from '/usr/local/renovate/dist/modules/versioning/index.js';
import { getRangeStrategy } from '/usr/local/renovate/dist/modules/manager/index.js';
import { extractPackageFile as extractGoMod } from '/usr/local/renovate/dist/modules/manager/gomod/extract.js';
import { getConfig } from '/usr/local/renovate/dist/config/defaults.js';
import { mergeChildConfig } from '/usr/local/renovate/dist/config/utils.js';
import { resolveConfigPresets } from '/usr/local/renovate/dist/config/presets/index.js';
import { branchifyUpgrades } from '/usr/local/renovate/dist/workers/repository/updates/branchify.js';
import { prAlreadyExisted } from '/usr/local/renovate/dist/workers/repository/update/branch/check-existing.js';

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
const closedPrs = [];

try {
  await initLogger();
  const repositoryConfig = JSON.parse(readFileSync('/fixtures/renovate.json', 'utf8'));
  // Plain UV_EXCLUDE_NEWER is filtered by Renovate. Trusted customEnvVariables
  // must reach the real pip-compile execution path without changing the header.
  assert.equal(action.env.RENOVATE_CUSTOM_ENV_VARIABLES, '${{ steps.python-age.outputs.custom_env }}');
  const cutoffStep = workflow.jobs.renovate.steps.find((step) => step.id === 'python-age');
  assert.ok(cutoffStep && cutoffStep.run.includes('dependency_updates.release_age'));
  assert.equal(action.with['docker-volumes'], '${{ github.workspace }}/infra/dependency-ci/uv-renovate.toml:/sifto-python-generation.toml:ro');
  const cutoff = '2026-10-05T00:00:00Z';
  const option = getOptions().find((entry) => entry.name === 'customEnvVariables');
  assert.ok(option && option.globalOnly);
  const uvConfig = '/sifto-python-generation.toml';
  setCustomEnv({ UV_EXCLUDE_NEWER: cutoff, UV_CONFIG_FILE: uvConfig });
  GlobalConfig.set({ localDir: directory, cacheDir: directory, exposeAllEnv: false });
  const execOptions = await getExecOptions(repositoryConfig, 'uv', directory, {}, '3.12');
  assert.equal(getChildEnv(execOptions).UV_EXCLUDE_NEWER, cutoff);
  assert.equal(getChildEnv(execOptions).UV_CONFIG_FILE, uvConfig);
  assert.match(readFileSync('/fixtures/uv-renovate.toml', 'utf8'), /\[pip\]\s+upgrade = true/);
  setCustomEnv({});
  GlobalConfig.set({ localDir: directory });
  // Exercise the real Go parser's disabled indirect dependencies, then apply
  // repository rules. Mirror extract/manager-files normalizeDepNames, which
  // fills packageName before production applies package-name rules.
  // No Go command, module download or datasource lookup runs.
  const goMod = `module example.invalid/dependency-fixture

go 1.26.0

require github.com/inngest/inngestgo v0.15.1

require (
  google.golang.org/grpc v1.82.1 // indirect
  golang.org/x/net v0.58.0 // indirect
  golang.org/x/text v0.42.0 // indirect
  golang.org/x/sys v0.48.0 // indirect
  google.golang.org/protobuf v1.36.10 // indirect
  golang.org/x/net/neighbor v1.0.0 // indirect
)
`;
  const enabledGoIndirect = new Set(['google.golang.org/grpc', 'golang.org/x/net']);
  const indirectNames = [...enabledGoIndirect, 'golang.org/x/text', 'golang.org/x/sys', 'google.golang.org/protobuf', 'golang.org/x/net/neighbor'];
  for (const packageFile of ['api/go.mod', 'go.mod', 'api/extra/go.mod', 'infra/maintenance/go.mod']) {
    const goFile = extractGoMod(goMod);
    assert.ok(goFile);
    const indirect = goFile.deps.filter((dep) => dep.depType === 'indirect');
    assert.deepEqual(indirect.map((dep) => dep.depName).sort(), [...indirectNames].sort());
    for (const dep of indirect) {
      assert.equal(dep.enabled, false, `parser default for ${dep.depName}`);
      assert.equal(dep.datasource, 'go');
      for (const updateType of ['minor', 'patch']) {
        const config = await applyPackageRules({ ...repositoryConfig, ...dep, packageName: dep.packageName ?? dep.depName, packageFile, manager: 'gomod', updateType });
        assert.equal(config.enabled, packageFile === 'api/go.mod' && enabledGoIndirect.has(dep.depName), `${packageFile}: ${dep.depName}`);
        assert.equal(config.minimumReleaseAge, '7 days');
        assert.equal(config.automerge, false);
        assert.equal(config.platformAutomerge, false);
        assert.equal(config.branchPrefix, 'renovate-ci/');
        assert.equal(config.groupName, packageFile.startsWith('infra/maintenance/') ? undefined : '{{manager}} minor and patch dependencies');
      }
    }
    const direct = goFile.deps.find((dep) => dep.depName === 'github.com/inngest/inngestgo');
    assert.ok(direct);
    assert.notEqual(direct.depType, 'indirect');
    const directConfig = await applyPackageRules({ ...repositoryConfig, ...direct, packageName: direct.packageName ?? direct.depName, packageFile, manager: 'gomod', updateType: 'minor' });
    assert.equal(directConfig.enabled, direct.enabled);
    assert.notEqual(directConfig.enabled, false);
  }
  const directGoFile = extractGoMod(goMod.replaceAll(' // indirect', ''));
  assert.ok(directGoFile);
  for (const depName of enabledGoIndirect) {
    const dep = directGoFile.deps.find((entry) => entry.depName === depName);
    assert.ok(dep);
    assert.notEqual(dep.depType, 'indirect');
    const config = await applyPackageRules({ ...repositoryConfig, ...dep, packageName: dep.packageName ?? dep.depName, packageFile: 'api/go.mod', manager: 'gomod', updateType: 'minor' });
    assert.equal(config.enabled, dep.enabled, `direct default for ${depName}`);
  }
  const targetDep = extractGoMod(goMod).deps.find((dep) => dep.depName === 'google.golang.org/grpc');
  for (const manager of ['npm', 'pip-compile']) {
    const config = await applyPackageRules({ ...repositoryConfig, ...targetDep, packageName: targetDep.packageName ?? targetDep.depName, packageFile: 'api/go.mod', manager, updateType: 'patch' });
    assert.equal(config.enabled, false, `other manager ${manager}`);
  }
  console.log(`Renovate ${installedVersion}: only api/go.mod indirect grpc and x/net are enabled; other paths, managers and indirect dependencies retain defaults and approval controls.`);
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
    if (url === `repos/${repository}/pulls?per_page=100&state=all&sort=updated&direction=desc&page=1`) return { body: closedPrs, headers: {} };
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

  // Resolve the shipped recommended presets and update-type/group defaults,
  // then run actual branch generation and closed-PR lookup. This catches a
  // rule that matches JSON but loses precedence during pin/group normalization.
  const resolved = mergeChildConfig(getConfig(), (await resolveConfigPresets(repositoryConfig)).config);
  const legacy = { ...resolved, packageRules: resolved.packageRules.filter((rule) =>
    !(rule.recreateWhen === 'never' && rule.matchUpdateTypes?.includes('pin'))) };
  const fixtures = [];
  for (const updateType of ['pinDigest', 'pin', 'digest', 'minor', 'patch', 'major']) {
    for (const maintenance of updateType === 'pin' ? [false] : [false, true]) {
      const docker = ['pinDigest', 'digest'].includes(updateType);
      const manager = docker ? 'dockerfile' : maintenance ? 'pip-compile' : 'npm';
      const packageFile = docker ? maintenance ? 'infra/maintenance/Dockerfile' : 'api/Dockerfile'
        : maintenance ? 'infra/maintenance/requirements.in' : 'web/package.json';
      const deps = ['fixture-a', 'fixture-b'].map((depName, index) => ({
        depName, packageName: depName, datasource: docker ? 'docker' : maintenance ? 'pypi' : 'npm',
        currentValue: `${updateType === 'pin' ? '^' : ''}${index + 1}.0.0`, currentVersion: `${index + 1}.0.0`,
        ...(updateType === 'digest' ? { currentDigest: `sha256:${'a'.repeat(64)}` } : {}),
        updates: [{ updateType,
          newMajor: index + (updateType === 'major' ? 2 : 1),
          newVersion: `${index + (updateType === 'major' ? 2 : 1)}.${updateType === 'minor' ? 1 : 0}.${updateType === 'patch' ? 1 : 0}`,
          newValue: `${index + (updateType === 'major' ? 2 : 1)}.${updateType === 'minor' ? 1 : 0}.${updateType === 'patch' ? 1 : 0}`,
          ...(updateType.endsWith('Digest') || updateType === 'digest' ? { newDigest: `sha256:${String(index + 1).repeat(64)}` } : {}),
        }],
      }));
      const generate = async (config, dependencyFixture = deps) => (await branchifyUpgrades({
        ...config, semanticCommits: 'disabled', baseBranch: 'main', repoIsOnboarded: true, errors: [], warnings: [],
      }, { [manager]: [{ packageFile, deps: structuredClone(dependencyFixture) }] })).branches;
      const before = await generate(legacy);
      const after = await generate(resolved);
      assert.ok(after.length > 0, `${updateType}: updates must remain enabled`);
      assert.equal(after.length, before.length);
      for (const branch of after) {
        const previous = before.find((entry) => entry.branchName === branch.branchName);
        assert.ok(previous, `${updateType}: branch identity must remain unchanged`);
        assert.equal(branch.prTitle, previous.prTitle);
        assert.equal(branch.minimumReleaseAge, '7 days');
        assert.equal(branch.automerge, false);
        assert.equal(branch.platformAutomerge, false);
        assert.equal(branch.pinDigests, true);
        assert.equal(branch.upgrades.length, previous.upgrades.length);
        if (maintenance) assert.equal(branch.groupName, activationGroup);
        if (['pin', 'pinDigest'].includes(updateType)) {
          assert.equal(previous.recreateClosed, true, `${updateType}: default immortal pin group reproduces the bug`);
          assert.equal(branch.recreateClosed, false, `${updateType}: closed pin proposal must be respected`);
          assert.ok(branch.upgrades.every((upgrade) => upgrade.recreateWhen === 'never'));
        } else {
          assert.equal(branch.recreateWhen, previous.recreateWhen, `${updateType}: retain version/digest policy`);
          assert.equal(branch.recreateClosed, previous.recreateClosed);
        }
        fixtures.push({ branch, previous, updateType });
      }
      if (!['pin', 'pinDigest'].includes(updateType)) {
        const future = deps.map((dep, index) => ({ ...dep, updates: dep.updates.map((update) => ({
          ...update,
          ...(!docker ? { newMajor: index + 10, newVersion: `${index + 10}.1.1`, newValue: `${index + 10}.1.1` } : {}),
          ...(docker ? { newDigest: `sha256:${String(index + 3).repeat(64)}` } : {}),
        })) }));
        const futureBranches = await generate(resolved, future);
        assert.ok(futureBranches.length > 0, `${updateType}: future updates must remain enabled`);
        for (const branch of futureBranches) fixtures.push({ branch, updateType, future: true });
      }
    }
  }
  for (const [index, { branch, future }] of fixtures.entries()) {
    if (future) continue;
    closedPrs.push({ number: index + 1, title: branch.prTitle, state: 'closed',
      user: { login: 'github-actions[bot]' }, updated_at: '2026-10-01T00:00:00Z',
      head: { ref: branch.branchName, repo: { full_name: repository } },
      base: { ref: 'main' },
    });
  }
  for (const { branch, previous, updateType, future } of fixtures) {
    const existing = await prAlreadyExisted(branch);
    if (future) {
      assert.equal(existing, null, `${updateType}: future version/digest proposals must remain eligible`);
      continue;
    }
    if (['pin', 'pinDigest'].includes(updateType)) {
      assert.equal(await prAlreadyExisted(previous), null, `${updateType}: legacy behavior ignores closed PR`);
      assert.equal(existing?.state, 'closed');
      assert.equal(existing?.title, branch.prTitle);
      assert.equal(existing?.sourceBranch, branch.branchName);
    } else {
      assert.deepEqual(existing, await prAlreadyExisted(previous));
    }
  }
  assert.equal(calls.length, 0, 'Closed-PR policy must only read GitHub');
  console.log(`Renovate ${installedVersion}: closed pin/pinDigest groups are not recreated; future version/digest updates, seven-day age and maintenance isolation retain their behavior.`);
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
