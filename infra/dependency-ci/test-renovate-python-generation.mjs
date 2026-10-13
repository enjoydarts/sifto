// Export the real pinned Renovate pip-compile command and filtered child env.
// No credentials, GitHub requests, or candidate repository code run here.
import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import { parse } from '/usr/local/renovate/node_modules/yaml/dist/index.js';
import { GlobalConfig } from '/usr/local/renovate/dist/config/global.js';
import { getExecOptions, extractHeaderCommand } from '/usr/local/renovate/dist/modules/manager/pip-compile/common.js';
import { constructPipCompileCmd } from '/usr/local/renovate/dist/modules/manager/pip-compile/artifacts.js';
import { setCustomEnv } from '/usr/local/renovate/dist/util/env.js';
import { getChildEnv } from '/usr/local/renovate/dist/util/exec/utils.js';
import { init as initLogger } from '/usr/local/renovate/dist/logger/index.js';

const workflow = parse(readFileSync('/fixtures/renovate.yml', 'utf8'));
const action = workflow.jobs.renovate.steps.find((step) => step.uses?.startsWith('renovatebot/'));
const version = JSON.parse(readFileSync('/usr/local/renovate/package.json', 'utf8')).version;
assert.equal(version, action.with['renovate-version']);
assert.equal(action.env.RENOVATE_CUSTOM_ENV_VARIABLES, '${{ steps.python-age.outputs.custom_env }}');
const config = JSON.parse(readFileSync('/fixtures/renovate.json', 'utf8'));
const cutoff = '2026-10-05T00:00:00Z';
await initLogger();
setCustomEnv({ UV_EXCLUDE_NEWER: cutoff, UV_CONFIG_FILE: '/sifto-python-generation.toml' });
GlobalConfig.set({ localDir: '/tmp', cacheDir: '/tmp', exposeAllEnv: false });
const options = await getExecOptions(config, 'uv', '/tmp', {}, '3.12');
const child = getChildEnv(options);
const command = constructPipCompileCmd(extractHeaderCommand(
  readFileSync('/fixtures/worker/requirements.txt', 'utf8'), 'worker/requirements.txt'), []);
process.stdout.write(JSON.stringify({ version, command,
  environment: { UV_EXCLUDE_NEWER: child.UV_EXCLUDE_NEWER, UV_CONFIG_FILE: child.UV_CONFIG_FILE } }));
