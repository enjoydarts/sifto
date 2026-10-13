"""Read-only production image evidence; no fixture executes artifact contents."""
import copy
import hashlib
import io
import json
import os
from pathlib import Path
import tempfile
import unittest
import zipfile
from unittest.mock import patch
import warnings

from dependency_updates import image_security as image
from dependency_updates.policy import Refused

HEAD, SOURCE = 'a' * 40, 'b' * 40
IMAGE = 'sha256:' + 'c' * 64


def encoded(value):
    return json.dumps(value).encode()


def fixture(component='api', findings=True):
    platform, dockerfile, os_package, library = image.EXPECTED[component]
    counts = dict.fromkeys(image.SEVERITIES, 0)
    counts['HIGH'] = int(findings)
    files = {
        'source-sha.txt': (HEAD + '\n').encode(), 'source-dirty.txt': b'false\n',
        'image-id.txt': (IMAGE + '\n').encode(), 'platform.txt': platform.encode(),
        'dockerfile.txt': dockerfile.encode(), 'summary.md': b'Inventory summary\n',
        'build-metadata.json': encoded({'containerimage.config.digest': IMAGE}),
        'sbom.cdx.json': encoded({'bomFormat': 'CycloneDX', 'metadata': {'component': {
            'properties': [{'name': 'aquasecurity:trivy:ImageID', 'value': IMAGE}]}},
            'components': [{'name': os_package}, {'name': library}]}),
        'vulnerabilities.json': encoded({'Metadata': {'ImageID': IMAGE}, 'Results': [
            {'Class': 'os-pkgs', 'Packages': [{'Name': os_package}], 'Vulnerabilities':
             [{'VulnerabilityID': 'CVE-fixture', 'Severity': 'HIGH'}] if findings else []},
            {'Class': 'lang-pkgs', 'Packages': [{'Name': library}]}]}),
        'scan-status.json': encoded({'sbom_exit_code': 0, 'vulnerability_exit_code': int(findings)}),
    }
    files['metadata.json'] = encoded({
        'component': component, 'source_commit': HEAD, 'source_tree_dirty': False,
        'platform': platform, 'dockerfile': dockerfile, 'stage': 'final',
        'image_id': IMAGE, 'build_image_id': IMAGE, 'archive_sha256': 'sha256:' + 'd' * 64,
        'trivy_version': '0.75.0', 'report_sha256': {name: 'sha256:' + hashlib.sha256(files[name]).hexdigest()
            for name in image.REPORTS}, 'package_counts': {'os': 1, 'library': 1},
        'vulnerability_counts': counts,
    })
    return files


def archive(files, extra=None):
    stream = io.BytesIO()
    with zipfile.ZipFile(stream, 'w', zipfile.ZIP_DEFLATED) as zipped:
        for name, raw in files.items():
            zipped.writestr(name, raw)
        if extra:
            with warnings.catch_warnings():
                warnings.simplefilter('ignore', UserWarning)
                zipped.writestr(*extra)
    return stream.getvalue()


class ReportTests(unittest.TestCase):
    def test_expected_inventory_matches_actual_archive_report_contract(self):
        import importlib.util
        report_path = Path(__file__).parents[1] / 'image-security/report.py'
        spec = importlib.util.spec_from_file_location('actual_image_report', report_path)
        report = importlib.util.module_from_spec(spec)
        spec.loader.exec_module(report)
        self.assertEqual({name: (value[0], value[2], value[3]) for name, value in image.EXPECTED.items()},
                         report.EXPECTED)

    def test_all_production_platforms_accept_clean_and_coherent_cve_failure(self):
        for component in image.EXPECTED:
            for findings in (False, True):
                with self.subTest(component=component, findings=findings):
                    evidence = image.verify_archive(archive(fixture(component, findings)), component, HEAD)
                    self.assertEqual(evidence['image_id'], IMAGE)
                    self.assertEqual(evidence['vulnerability_counts']['HIGH'], int(findings))
                    self.assertNotIn('archive_verified', evidence)

    def test_changed_correlation_or_report_is_refused(self):
        mutations = [('source_commit', SOURCE), ('source_tree_dirty', True), ('platform', 'linux/amd64'),
                     ('dockerfile', 'other/Dockerfile'), ('stage', 'dev'), ('image_id', 'sha256:' + 'e' * 64),
                     ('build_image_id', 'sha256:' + 'e' * 64), ('archive_sha256', 'not-a-digest'),
                     ('package_counts', {'os': 2, 'library': 1}),
                     ('vulnerability_counts', dict.fromkeys(image.SEVERITIES, 0))]
        for key, value in mutations:
            files = fixture()
            metadata = json.loads(files['metadata.json'])
            metadata[key] = value
            files['metadata.json'] = encoded(metadata)
            with self.subTest(key=key), self.assertRaises(Refused):
                image.verify_archive(archive(files), 'api', HEAD)
        for name in image.REPORTS:
            files = fixture()
            files[name] += b' '
            with self.subTest(name=name), self.assertRaises(Refused):
                image.verify_archive(archive(files), 'api', HEAD)

    def test_scanner_or_inventory_failure_is_not_a_cve_failure(self):
        for name, value in [
            ('scan-status.json', {'sbom_exit_code': 1, 'vulnerability_exit_code': 1}),
            ('scan-status.json', {'sbom_exit_code': 0, 'vulnerability_exit_code': 2}),
            ('scan-status.json', {'sbom_exit_code': False, 'vulnerability_exit_code': True}),
            ('sbom.cdx.json', {'bomFormat': 'CycloneDX'}),
            ('vulnerabilities.json', {'Metadata': {'ImageID': IMAGE}, 'Results': []}),
        ]:
            files = fixture()
            files[name] = encoded(value)
            metadata = json.loads(files['metadata.json'])
            metadata['report_sha256'][name] = 'sha256:' + hashlib.sha256(files[name]).hexdigest()
            files['metadata.json'] = encoded(metadata)
            with self.subTest(name=name, value=value), self.assertRaises(Refused):
                image.verify_archive(archive(files), 'api', HEAD)

    def test_duplicate_keys_and_nonfinite_json_refused(self):
        for raw in (b'{"component":"api","component":"worker"}', b'{"component":NaN}'):
            files = fixture()
            files['metadata.json'] = raw
            with self.subTest(raw=raw), self.assertRaises(Refused):
                image.verify_archive(archive(files), 'api', HEAD)

    def test_zip_unknown_traversal_absolute_duplicate_and_symlink_refused(self):
        for name in ('../metadata.json', '/metadata.json', 'nested/metadata.json', 'metadata.json', 'image.tar'):
            with self.subTest(name=name), self.assertRaises(Refused):
                image.verify_archive(archive(fixture(), (name, b'x')), 'api', HEAD)
        link = zipfile.ZipInfo('link')
        link.create_system = 3
        link.external_attr = 0o120777 << 16
        with self.assertRaises(Refused):
            image.verify_archive(archive(fixture(), (link, b'metadata.json')), 'api', HEAD)

    def test_missing_and_bounded_zip_members_refused(self):
        files = fixture()
        del files['summary.md']
        with self.assertRaises(Refused): image.verify_archive(archive(files), 'api', HEAD)
        with patch.object(image, 'MAX_EXPANDED', 20), self.assertRaises(Refused):
            image.verify_archive(archive(fixture()), 'api', HEAD)
        with patch.object(image, 'MAX_ZIP', 20), self.assertRaises(Refused):
            image.verify_archive(archive(fixture()), 'api', HEAD)
        with patch.object(image, 'MAX_MEMBER', 20), self.assertRaises(Refused):
            image.verify_archive(archive(fixture()), 'api', HEAD)


class ControllerTests(unittest.TestCase):
    def setUp(self):
        from dependency_updates.github import GitHub
        self.repo = 'owner/repo'
        self.repo_id = 42
        self.bot = {'id': 41898282, 'login': 'github-actions[bot]', 'type': 'Bot'}
        self.repository = {'id': self.repo_id, 'full_name': self.repo}
        self.plan = {'pr': 1, 'head': HEAD, 'base': SOURCE, 'source': SOURCE,
                     'repo': self.repo, 'branch': 'renovate-ci/pip-minor-patch'}
        self.environment = {'GITHUB_RUN_ID': '10', 'GITHUB_RUN_ATTEMPT': '1',
            'GITHUB_SHA': SOURCE, 'GITHUB_WORKFLOW_SHA': SOURCE, 'GITHUB_REPOSITORY': self.repo,
            'GITHUB_REPOSITORY_ID': str(self.repo_id), 'GITHUB_REF': 'refs/heads/main',
            'GITHUB_EVENT_NAME': 'workflow_dispatch',
            'GITHUB_WORKFLOW_REF': self.repo + '/.github/workflows/dependency-update.yml@refs/heads/main',
            'GITHUB_ACTOR': self.bot['login'], 'GITHUB_ACTOR_ID': str(self.bot['id']),
            'GITHUB_TRIGGERING_ACTOR': self.bot['login']}
        self.parent = self.make_run(10, 1, SOURCE, 'main', 'Dependency PR 1 ' + HEAD + ' ' + SOURCE,
            '.github/workflows/dependency-update.yml', 'in_progress', None)
        self.scans, self.jobs, self.artifacts, self.downloads, self.mutations = [], [], [], {}, []
        self.main = SOURCE
        self.post_error = None
        self.api = GitHub(self.repo, transport=self.transport)
        self.env_patch = patch.dict(os.environ, self.environment)
        self.env_patch.start()
        self.addCleanup(self.env_patch.stop)
        self.download_patch = patch.object(image, 'download_zip', side_effect=lambda repo, identifier: self.downloads[identifier])
        self.download_patch.start()
        self.addCleanup(self.download_patch.stop)

    def make_run(self, identifier, workflow, head, branch, title, path, status='completed', conclusion='failure'):
        return {'id': identifier, 'workflow_id': workflow, 'head_sha': head, 'head_branch': branch,
                'display_title': title, 'path': path, 'status': status, 'conclusion': conclusion,
                'event': 'workflow_dispatch', 'run_attempt': 1, 'repository': self.repository,
                'head_repository': self.repository, 'actor': self.bot, 'triggering_actor': self.bot}

    def transport(self, method, path, body):
        if method != 'GET':
            self.mutations.append((method, path, body))
            if self.post_error: raise self.post_error
            return None
        endpoint = path.split('?')[0]
        if endpoint == '': return self.repository
        if endpoint == '/branches/main': return {'name': 'main', 'protected': True, 'commit': {'sha': self.main}}
        if endpoint == '/git/ref/heads/main': return {'object': {'sha': self.main}}
        if endpoint == '/actions/workflows/dependency-update.yml':
            return {'id': 1, 'name': image.PARENT_NAME, 'path': '.github/workflows/dependency-update.yml', 'state': 'active'}
        if endpoint == '/actions/workflows/dependency-security.yml':
            return {'id': 2, 'name': image.NAME, 'path': '.github/workflows/dependency-security.yml', 'state': 'active'}
        if endpoint == '/actions/runs/10': return self.parent
        if endpoint == '/actions/workflows/dependency-security.yml/runs':
            return {'workflow_runs': self.scans, 'total_count': len(self.scans)}
        if endpoint == '/actions/runs/20': return self.scans[0]
        if endpoint.endswith('/jobs'): return {'jobs': self.jobs, 'total_count': len(self.jobs)}
        if endpoint.endswith('/artifacts'): return {'artifacts': self.artifacts, 'total_count': len(self.artifacts)}
        raise AssertionError('Unexpected evidence API request: ' + path)

    def complete(self, ticket, findings=True):
        head, ref = ticket['head'], ticket['ref']
        self.scans = [self.make_run(20, 2, head, ref, ticket['marker'], '.github/workflows/dependency-security.yml',
            conclusion='failure' if findings else 'success')]
        for index, component in enumerate(image.EXPECTED):
            files = fixture(component, findings)
            if head != HEAD:
                files['source-sha.txt'] = head.encode()
                metadata = json.loads(files['metadata.json'])
                metadata['source_commit'] = head
                files['metadata.json'] = encoded(metadata)
            raw = archive(files)
            identifier = 100 + index
            self.downloads[identifier] = raw
            self.artifacts.append({'id': identifier, 'name': 'image-security-' + component + '-' + head + '-1',
                'expired': False, 'expires_at': '2099-01-01T00:00:00Z', 'size_in_bytes': len(raw),
                'digest': image.sha256(raw), 'workflow_run': {'id': 20, 'head_sha': head, 'head_branch': ref,
                    'repository_id': 42, 'head_repository_id': 42}})
            self.jobs.append({'id': 200 + index, 'run_id': 20, 'head_sha': head, 'run_attempt': 1,
                'name': 'images (' + component + ')', 'status': 'completed',
                'conclusion': 'failure' if findings else 'success', 'steps': [
                    {'name': name, 'status': 'completed', 'conclusion':
                     'failure' if name == image.SCAN and findings else 'success'} for name in image.IMAGE_STEPS]})
        if ref == 'main':
            self.jobs.append({'id': 205, 'run_id': 20, 'head_sha': head, 'run_attempt': 1,
                'name': 'inventory', 'status': 'completed', 'conclusion': 'failure' if findings else 'success',
                'steps': [{'name': name, 'status': 'completed', 'conclusion':
                    'failure' if name == image.INVENTORY_SCAN and findings else 'success'} for name in image.INVENTORY_STEPS]})

    def test_candidate_dispatch_and_complete_failure_evidence_then_exact_revalidation(self):
        ticket = image.start(self.api, self.plan, 'candidate', HEAD, self.plan['branch'])
        self.assertEqual(len(self.mutations), 1)
        self.assertEqual(self.mutations[0][2]['inputs'], {'expected_head': HEAD, 'parent_run': '10', 'phase': 'candidate'})
        self.complete(ticket)
        evidence = image.wait(self.api, self.plan, ticket)
        self.assertEqual(evidence['conclusion'], 'failure')
        self.assertEqual(len(evidence['images']), 5)
        self.assertEqual(image.revalidate(self.api, self.plan, evidence), evidence)
        self.assertEqual(len(self.mutations), 1)

    def test_completed_clean_scan_and_merged_main_with_separate_inventory(self):
        self.main = 'e' * 40
        ticket = image.start(self.api, self.plan, 'merged', self.main, 'main')
        self.complete(ticket, False)
        evidence = image.wait(self.api, self.plan, ticket)
        self.assertEqual(evidence['conclusion'], 'success')
        self.assertEqual(evidence['inventory']['conclusion'], 'success')

    def test_foreign_parent_rerun_actor_source_repo_and_main_refused_before_post(self):
        for key, value in [('run_attempt', 2), ('head_sha', HEAD), ('workflow_id', 999),
                           ('actor', {'login': 'github-actions[bot]', 'id': 1, 'type': 'Bot'}),
                           ('triggering_actor', {'login': 'owner', 'id': 2, 'type': 'User'}),
                           ('repository', {'full_name': self.repo, 'id': 999})]:
            parent = copy.deepcopy(self.parent)
            self.parent[key] = value
            with self.subTest(key=key), self.assertRaises(Refused):
                image.start(self.api, self.plan, 'candidate', HEAD, self.plan['branch'])
            self.parent = parent
            self.assertEqual(self.mutations, [])
        self.main = HEAD
        with self.assertRaises(Refused): image.start(self.api, self.plan, 'candidate', HEAD, self.plan['branch'])
        self.assertEqual(self.mutations, [])

    def test_unknown_dispatch_is_never_replayed(self):
        self.post_error = TimeoutError()
        with self.assertRaisesRegex(Refused, 'unknown|audit'):
            image.start(self.api, self.plan, 'candidate', HEAD, self.plan['branch'])
        self.assertEqual(len(self.mutations), 1)

    def test_existing_marker_prevents_a_second_dispatch(self):
        ticket = image.start(self.api, self.plan, 'candidate', HEAD, self.plan['branch'])
        self.complete(ticket)
        with self.assertRaises(Refused): image.start(self.api, self.plan, 'candidate', HEAD, self.plan['branch'])
        self.assertEqual(len(self.mutations), 1)

    def test_any_previous_source_consumes_the_parent_phase_before_post(self):
        for source in ('f' * 40, 'unknown', '', 'rejected-sha'):
            self.scans = [self.make_run(20, 2, 'f' * 40, self.plan['branch'],
                ('Dependency image scan 10 candidate ' + source).rstrip(),
                '.github/workflows/dependency-security.yml')]
            with self.subTest(source=source), self.assertRaises(Refused):
                image.start(self.api, self.plan, 'candidate', HEAD, self.plan['branch'])
            self.assertEqual(self.mutations, [])

    def test_other_phase_or_parent_does_not_consume_this_dispatch(self):
        for title in ('Dependency image scan 10 merged ' + 'f' * 40,
                      'Dependency image scan 11 candidate ' + 'f' * 40):
            self.scans = [self.make_run(20, 2, 'f' * 40, 'main', title,
                '.github/workflows/dependency-security.yml')]
            image.start(self.api, self.plan, 'candidate', HEAD, self.plan['branch'])
        self.assertEqual(len(self.mutations), 2)

    def test_other_source_for_same_parent_phase_invalidates_selected_run(self):
        ticket = image.start(self.api, self.plan, 'candidate', HEAD, self.plan['branch'])
        self.complete(ticket)
        evidence = image.wait(self.api, self.plan, ticket)
        self.scans.append(self.make_run(21, 2, 'f' * 40, self.plan['branch'],
            'Dependency image scan 10 candidate ' + 'f' * 40,
            '.github/workflows/dependency-security.yml'))
        with self.assertRaises(Refused): image.revalidate(self.api, self.plan, evidence)
        self.assertEqual(len(self.mutations), 1)

    def test_child_rerun_foreign_identity_head_actor_marker_ambiguity_refused(self):
        ticket = image.start(self.api, self.plan, 'candidate', HEAD, self.plan['branch'])
        self.complete(ticket)
        original = copy.deepcopy(self.scans)
        for key, value in [('run_attempt', 2), ('head_sha', SOURCE), ('head_branch', 'main'),
                           ('workflow_id', 99), ('event', 'push'), ('conclusion', 'cancelled'),
                           ('actor', {'id': 3, 'login': 'owner', 'type': 'User'})]:
            self.scans[0][key] = value
            with self.subTest(key=key), self.assertRaises(Refused):
                image.wait(self.api, self.plan, ticket)
            self.scans = copy.deepcopy(original)
        self.scans.append(dict(self.scans[0], id=21))
        with self.assertRaises(Refused): image.wait(self.api, self.plan, ticket)

    def test_actual_rest_run_must_have_the_exact_history_run_id(self):
        ticket = image.start(self.api, self.plan, 'candidate', HEAD, self.plan['branch'])
        self.complete(ticket)
        transport = self.api.transport
        def wrong(method, path, body):
            result = transport(method, path, body)
            return dict(result, id=21) if path == '/actions/runs/20' else result
        self.api.transport = wrong
        with self.assertRaises(Refused): image.wait(self.api, self.plan, ticket)

    def test_history_cannot_become_nonterminal_during_final_evidence_readback(self):
        ticket = image.start(self.api, self.plan, 'candidate', HEAD, self.plan['branch'])
        self.complete(ticket)
        transport = self.api.transport
        history_reads = []
        def changed(method, path, body):
            result = transport(method, path, body)
            if path.startswith('/actions/workflows/dependency-security.yml/runs'):
                history_reads.append(path)
                if len(history_reads) >= 3:
                    result = copy.deepcopy(result)
                    result['workflow_runs'][0].update(status='in_progress', conclusion=None)
            return result
        self.api.transport = changed
        with self.assertRaises(Refused): image.wait(self.api, self.plan, ticket)

    def test_dispatch_marker_never_appears_reaches_bounded_deadline_without_replay(self):
        ticket = image.start(self.api, self.plan, 'candidate', HEAD, self.plan['branch'])
        ticket['deadline'] = 2
        elapsed = [0]
        def sleep(seconds): elapsed[0] += seconds
        with patch.object(image.time, 'monotonic', side_effect=lambda: elapsed[0]), \
            patch.object(image.time, 'sleep', side_effect=sleep), self.assertRaisesRegex(Refused, 'Timed out'):
            image.wait(self.api, self.plan, ticket)
        self.assertEqual(elapsed[0], 2)
        self.assertEqual(len(self.mutations), 1)

    def test_long_wait_reserves_shared_quota_and_final_evidence_with_twenty_history_pages(self):
        from urllib.error import HTTPError
        from urllib.parse import parse_qs, urlsplit
        elapsed, hourly = [0], {}
        original = self.api.transport
        older = [{'id': identifier, 'display_title': 'Ordinary image advisory %d' % identifier}
                 for identifier in range(1, 1901)]
        def charge():
            hour = int(elapsed[0] // 3600)
            hourly[hour] = hourly.get(hour, 0) + 1
            # 600 requests/hour are reserved for other workflows. A wait must
            # also leave room for its own full evidence and artifact readback.
            if hourly[hour] + 600 > 1000:
                raise HTTPError('https://api.github.com', 403, 'fixture quota exhausted', {}, None)
        def transport(method, path, body):
            charge()
            if self.scans:
                self.scans[0].update(status='completed' if elapsed[0] >= 70 * 60 else 'in_progress',
                                     conclusion='failure' if elapsed[0] >= 70 * 60 else None)
            if path.startswith('/actions/workflows/dependency-security.yml/runs'):
                rows = older + self.scans
                page = int(parse_qs(urlsplit(path).query)['page'][0])
                return {'workflow_runs': rows[(page - 1) * 100:page * 100], 'total_count': len(rows)}
            if path == '/actions/runs/5000': return self.scans[0]
            return original(method, path, body)
        self.api.transport = transport
        def download(repo, identifier):
            charge()  # Official gh artifact ZIP GET consumes the same quota.
            return self.downloads[identifier]
        with patch.object(image.time, 'monotonic', side_effect=lambda: elapsed[0]), \
            patch.object(image, 'download_zip', side_effect=download), \
            patch.object(image.time, 'sleep', side_effect=lambda seconds: elapsed.__setitem__(0, elapsed[0] + seconds)):
            ticket = image.start(self.api, self.plan, 'candidate', HEAD, self.plan['branch'])
            self.complete(ticket)
            self.scans[0]['id'] = 5000
            for job in self.jobs: job['run_id'] = 5000
            for artifact in self.artifacts: artifact['workflow_run']['id'] = 5000
            evidence = image.wait(self.api, self.plan, ticket)
        self.assertEqual(evidence['run_id'], 5000)
        self.assertGreaterEqual(elapsed[0], 70 * 60)
        self.assertLess(elapsed[0], 80 * 60)
        self.assertLessEqual(max(hourly.values()), 400)
        self.assertEqual(len(self.mutations), 1)

    def test_cached_wait_authority_cannot_accept_completed_child_after_parent_change(self):
        elapsed = [0]
        original = self.api.transport
        def transport(method, path, body):
            if path == '/actions/runs/20':
                self.scans[0].update(status='completed' if elapsed[0] >= 60 else 'in_progress',
                                     conclusion='failure' if elapsed[0] >= 60 else None)
                if elapsed[0] >= 60:
                    self.parent['triggering_actor'] = {'id': 9, 'login': 'owner', 'type': 'User'}
            return original(method, path, body)
        self.api.transport = transport
        with patch.object(image.time, 'monotonic', side_effect=lambda: elapsed[0]), \
            patch.object(image.time, 'sleep', side_effect=lambda seconds: elapsed.__setitem__(0, elapsed[0] + seconds)):
            ticket = image.start(self.api, self.plan, 'candidate', HEAD, self.plan['branch'])
            self.complete(ticket)
            self.scans[0].update(status='in_progress', conclusion=None)
            with self.assertRaises(Refused): image.wait(self.api, self.plan, ticket)
        self.assertEqual(elapsed[0], 60)
        self.assertEqual(len(self.mutations), 1)

    def test_missing_scan_marker_twenty_page_history_times_out_without_quota_exhaustion(self):
        from urllib.parse import parse_qs, urlsplit
        elapsed, requests = [0], [0]
        original = self.api.transport
        rows = [{'id': identifier, 'display_title': 'Ordinary advisory %d' % identifier}
                for identifier in range(1, 1901)]
        def transport(method, path, body):
            requests[0] += 1
            if path.startswith('/actions/workflows/dependency-security.yml/runs'):
                page = int(parse_qs(urlsplit(path).query)['page'][0])
                return {'workflow_runs': rows[(page - 1) * 100:page * 100], 'total_count': len(rows)}
            return original(method, path, body)
        self.api.transport = transport
        with patch.object(image.time, 'monotonic', side_effect=lambda: elapsed[0]), \
            patch.object(image.time, 'sleep', side_effect=lambda seconds: elapsed.__setitem__(0, elapsed[0] + seconds)):
            ticket = image.start(self.api, self.plan, 'candidate', HEAD, self.plan['branch'])
            with self.assertRaisesRegex(Refused, 'Timed out'):
                image.wait(self.api, self.plan, ticket)
        self.assertEqual(elapsed[0], 80 * 60)
        self.assertLess(requests[0], 500)
        self.assertEqual(len(self.mutations), 1)

    def test_missing_duplicate_unknown_failed_build_upload_or_job_attempt_refused(self):
        ticket = image.start(self.api, self.plan, 'candidate', HEAD, self.plan['branch'])
        self.complete(ticket)
        original = copy.deepcopy(self.jobs)
        for mutation in ('missing', 'duplicate', 'unknown', 'build', 'upload', 'attempt', 'source', 'scan-success'):
            self.jobs = copy.deepcopy(original)
            if mutation == 'missing': self.jobs.pop()
            if mutation == 'duplicate': self.jobs.append(dict(self.jobs[0], id=999))
            if mutation == 'unknown': self.jobs.append(dict(self.jobs[0], id=999, name='unknown'))
            if mutation == 'attempt': self.jobs[0]['run_attempt'] = 2
            if mutation == 'source': self.jobs[0]['head_sha'] = SOURCE
            if mutation in ('build', 'upload', 'scan-success'):
                index = {'build': 6, 'upload': 9, 'scan-success': 7}[mutation]
                self.jobs[0]['steps'][index]['conclusion'] = 'success' if mutation == 'scan-success' else 'failure'
            with self.subTest(mutation=mutation), self.assertRaises(Refused): image.wait(self.api, self.plan, ticket)

    def test_artifact_missing_duplicate_foreign_run_repo_expiry_digest_size_refused(self):
        ticket = image.start(self.api, self.plan, 'candidate', HEAD, self.plan['branch'])
        self.complete(ticket)
        original = copy.deepcopy(self.artifacts)
        for mutation in ('missing', 'duplicate', 'expired', 'expires_at', 'digest', 'size', 'repo', 'run', 'source'):
            self.artifacts = copy.deepcopy(original)
            if mutation == 'missing': self.artifacts.pop()
            if mutation == 'duplicate': self.artifacts.append(dict(self.artifacts[0], id=999))
            if mutation == 'expired': self.artifacts[0]['expired'] = True
            if mutation == 'expires_at': self.artifacts[0]['expires_at'] = '2000-01-01T00:00:00Z'
            if mutation == 'digest': self.artifacts[0]['digest'] = 'sha256:' + 'f' * 64
            if mutation == 'size': self.artifacts[0]['size_in_bytes'] += 1
            if mutation == 'repo': self.artifacts[0]['workflow_run']['repository_id'] = 99
            if mutation == 'run': self.artifacts[0]['workflow_run']['id'] = 99
            if mutation == 'source': self.artifacts[0]['workflow_run']['head_sha'] = SOURCE
            with self.subTest(mutation=mutation), self.assertRaises(Refused): image.wait(self.api, self.plan, ticket)

    def test_prepared_evidence_changed_refused_and_no_old_unbound_evidence(self):
        ticket = image.start(self.api, self.plan, 'candidate', HEAD, self.plan['branch'])
        self.complete(ticket)
        evidence = image.wait(self.api, self.plan, ticket)
        evidence['images'][0]['image_id'] = 'sha256:' + 'e' * 64
        with self.assertRaises(Refused): image.revalidate(self.api, self.plan, evidence)
        with self.assertRaises(Refused): image.revalidate(self.api, self.plan, {})

    def test_complete_pagination_is_required_for_scan_jobs_and_artifacts(self):
        ticket = image.start(self.api, self.plan, 'candidate', HEAD, self.plan['branch'])
        self.complete(ticket)
        transport = self.api.transport
        for endpoint in ('/runs', '/jobs', '/artifacts'):
            def broken(method, path, body):
                result = transport(method, path, body)
                if path.split('?')[0].endswith(endpoint):
                    result = dict(result, total_count=result['total_count'] + 1)
                return result
            self.api.transport = broken
            with self.subTest(endpoint=endpoint), self.assertRaises(Refused): image.wait(self.api, self.plan, ticket)
        self.api.transport = transport


class ReportShapeTests(unittest.TestCase):

    def test_link_under_allowed_filename_is_refused(self):
        files = fixture()
        del files['metadata.json']
        link = zipfile.ZipInfo('metadata.json')
        link.create_system = 3
        link.external_attr = 0o120777 << 16
        with self.assertRaises(Refused):
            image.verify_archive(archive(files, (link, b'outside')), 'api', HEAD)

    def test_malformed_nested_types_and_unknown_severity_refused(self):
        for name, value in [
            ('sbom.cdx.json', {'bomFormat': 'CycloneDX', 'metadata': {'component': []}}),
            ('vulnerabilities.json', {'Metadata': [], 'Results': []}),
            ('vulnerabilities.json', {'Metadata': {'ImageID': IMAGE}, 'Results': [
                {'Class': 'os-pkgs', 'Packages': [{'Name': 'ca-certificates'}],
                 'Vulnerabilities': [{'VulnerabilityID': 'CVE-fixture', 'Severity': 'SUPPRESSED'}]}]}),
            ('build-metadata.json', {'containerimage.config.digest': 'sha256:' + 'e' * 64}),
        ]:
            files = fixture()
            files[name] = encoded(value)
            metadata = json.loads(files['metadata.json'])
            if name in image.REPORTS:
                metadata['report_sha256'][name] = image.sha256(files[name])
                files['metadata.json'] = encoded(metadata)
            with self.subTest(name=name, value=value), self.assertRaises(Refused):
                image.verify_archive(archive(files), 'api', HEAD)


class DownloadTests(unittest.TestCase):
    def download(self, chunks, returncode=0, elapsed=False):
        from unittest.mock import Mock
        process = Mock()
        process.wait.return_value = returncode
        process.poll.return_value = None
        process.stdout.fileno.return_value = 123
        selector = Mock()
        selector.__enter__ = Mock(return_value=selector)
        selector.__exit__ = Mock(return_value=None)
        selector.select.return_value = [('ready', 1)]
        clock = iter((0, 121)) if elapsed else None
        with patch.dict(os.environ, {'GH_TOKEN': 'fixture-token', 'GH_DEBUG': 'api', 'GITHUB_TOKEN': 'other-token'}), \
            patch.object(image.subprocess, 'Popen', return_value=process) as launch, \
            patch.object(image.selectors, 'DefaultSelector', return_value=selector), \
            patch.object(image.os, 'read', side_effect=chunks), \
            patch.object(image.time, 'monotonic', side_effect=(lambda: next(clock)) if clock else None, return_value=0):
            try:
                result = image.download_zip('owner/repo', 100)
            finally:
                self.launch, self.process = launch, process
            return result

    def test_official_cli_download_keeps_token_out_of_arguments_and_stderr(self):
        self.assertEqual(self.download([b'zip-bytes', b'']), b'zip-bytes')
        args, kwargs = self.launch.call_args
        self.assertEqual(args[0], ['gh', 'api', '--hostname', 'github.com', 'repos/owner/repo/actions/artifacts/100/zip'])
        self.assertNotIn('fixture-token', str(args))
        self.assertEqual(kwargs['env']['GH_TOKEN'], 'fixture-token')
        self.assertNotIn('GH_DEBUG', kwargs['env'])
        self.assertNotIn('GITHUB_TOKEN', kwargs['env'])
        self.assertEqual(kwargs['stderr'], image.subprocess.DEVNULL)

    def test_cli_stdout_limit_error_and_timeout_fail_closed_without_retry(self):
        with patch.object(image, 'MAX_ZIP', 3), self.assertRaises(Refused):
            self.download([b'too-big'])
        self.assertEqual(self.launch.call_count, 1)
        self.process.kill.assert_called_once()
        with self.assertRaises(Refused): self.download([b''], returncode=1)
        self.assertEqual(self.launch.call_count, 1)
        with self.assertRaisesRegex(Refused, 'timed out'): self.download([], elapsed=True)
        self.assertEqual(self.launch.call_count, 1)


if __name__ == '__main__':
    unittest.main()
