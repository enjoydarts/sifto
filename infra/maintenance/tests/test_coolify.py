import json
import unittest
from datetime import datetime, timezone
from unittest.mock import Mock, patch

from maintenance import report
from test_controller import ready


class CoolifyObservationTests(unittest.TestCase):
    def setUp(self):
        self.assertTrue(hasattr(report, 'observe_coolify'), 'Coolify read-only release observation missing')
        self.inventory = ready()
        self.inventory['runtime']['containers'] = [{'image': 'ghcr.io/coollabsio/coolify:4.4.2', 'status': 'running'}]

    def response(self, version):
        response = Mock()
        response.__enter__ = Mock(return_value=response)
        response.__exit__ = Mock(return_value=False)
        response.geturl.return_value = 'https://api.github.com/repos/coollabsio/coolify/releases/latest'
        response.read.return_value = json.dumps({'tag_name': 'v' + version, 'prerelease': False, 'draft': False,
            'published_at': '1970-01-01T00:00:00Z',
            'html_url': 'https://github.com/coollabsio/coolify/releases/tag/v' + version}).encode()
        return response

    def test_observes_update_from_actual_image_and_official_latest_without_action(self):
        with patch.object(report.urllib.request, 'urlopen', return_value=self.response('4.5.0')) as request:
            result = report.observe_coolify(self.inventory, now=1000)
        self.assertEqual(result['status'], 'update_available')
        self.assertEqual(result['current_version'], '4.4.2')
        self.assertEqual(result['latest_version'], '4.5.0')
        self.assertEqual(result['fetched_at'], 1000)
        headers = request.call_args.args[0].headers
        self.assertNotIn('Authorization', headers)
        self.assertNotIn('action', result)
        rendered = report.render({'status': 'current', 'reason': 'no_updates', 'plan': None}, {'status': 'unavailable'}, result)
        self.assertIn('4.4.2', rendered)
        self.assertIn('4.5.0', rendered)
        self.assertIn('別途', rendered)

    def test_current_version_uses_container_tag_not_hardcoded_version(self):
        self.inventory['runtime']['containers'][0]['image'] = 'ghcr.io/coollabsio/coolify:9.1.0'
        with patch.object(report.urllib.request, 'urlopen', return_value=self.response('9.1.0')):
            self.assertEqual(report.observe_coolify(self.inventory)['status'], 'current')

    def test_fetch_failure_untrusted_release_prerelease_and_alias_are_unknown(self):
        with patch.object(report.urllib.request, 'urlopen', side_effect=OSError('private-details')):
            result = report.observe_coolify(self.inventory)
        self.assertEqual(result['status'], 'unknown')
        self.assertNotIn('private-details', json.dumps(result))
        for payload in [{'tag_name': 'v4.5.0', 'prerelease': True, 'draft': False, 'html_url': 'https://github.com/coollabsio/coolify/releases/tag/v4.5.0'},
                        {'tag_name': 'v4.5.0', 'prerelease': False, 'draft': False, 'html_url': 'https://attacker.invalid/release'},
                        {'tag_name': 'v4.5.0-beta', 'prerelease': False, 'draft': False, 'html_url': 'https://github.com/coollabsio/coolify/releases/tag/v4.5.0-beta'}]:
            response = self.response('4.5.0')
            response.read.return_value = json.dumps(payload).encode()
            with patch.object(report.urllib.request, 'urlopen', return_value=response):
                self.assertEqual(report.observe_coolify(self.inventory)['status'], 'unknown')
        self.inventory['runtime']['containers'][0]['image'] = 'ghcr.io/coollabsio/coolify:latest'
        with patch.object(report.urllib.request, 'urlopen') as fetch:
            self.assertEqual(report.observe_coolify(self.inventory)['status'], 'unknown')
        fetch.assert_not_called()

    def test_patch_candidate_waits_seven_days_and_never_becomes_executable(self):
        published = int(datetime(2026, 10, 9, 18, 42, 5, tzinfo=timezone.utc).timestamp())
        response = self.response('4.4.6')
        payload = json.loads(response.read.return_value)
        payload['published_at'] = '2026-10-09T18:42:05Z'
        response.read.return_value = json.dumps(payload).encode()
        for elapsed, review in [(0, 'waiting'), (7 * 86400 - 1, 'waiting'),
                                (7 * 86400, 'eligible_patch_review')]:
            with self.subTest(elapsed=elapsed), patch.object(report.urllib.request, 'urlopen', return_value=response):
                result = report.observe_coolify(self.inventory, now=published + elapsed)
            self.assertEqual(result['status'], 'update_available')
            self.assertEqual(result['release_published_at'], published)
            self.assertEqual(result['review_after'], published + 7 * 86400)
            self.assertEqual(result['update_scope'], 'patch')
            self.assertEqual(result['candidate_review'], review)
            self.assertNotIn('action', result)
            self.assertNotIn('plan', result)
        rendered = report.render({'status': 'current', 'reason': 'no_updates', 'plan': None},
                                 {'status': 'unavailable'}, result)
        self.assertIn('eligible_patch_review', rendered)
        self.assertIn(str(published + 7 * 86400), rendered)

    def test_minor_major_current_and_ahead_are_not_patch_candidates(self):
        for version, scope, review in [('4.5.0', 'minor', 'manual_review'),
                                      ('5.0.0', 'major', 'manual_review'),
                                      ('4.4.2', 'none', 'no_update'),
                                      ('4.4.1', 'downgrade', 'no_update')]:
            with self.subTest(version=version), patch.object(report.urllib.request, 'urlopen', return_value=self.response(version)):
                result = report.observe_coolify(self.inventory, now=8 * 86400)
            self.assertEqual(result['update_scope'], scope)
            self.assertEqual(result['candidate_review'], review)

    def test_missing_invalid_or_future_publication_cannot_pass_release_policy(self):
        for publication in [None, '', '2026-10-09', '2026-10-09T18:42:05+00:00',
                            '2026-99-09T18:42:05Z', '2026-10-09T18:42:05Z', 0, True]:
            response = self.response('4.4.6')
            payload = json.loads(response.read.return_value)
            if publication is None:
                payload.pop('published_at')
            else:
                payload['published_at'] = publication
            response.read.return_value = json.dumps(payload).encode()
            with self.subTest(publication=publication), patch.object(report.urllib.request, 'urlopen', return_value=response):
                result = report.observe_coolify(self.inventory, now=1000)
            self.assertEqual(result['status'], 'unknown')
            self.assertEqual(result['candidate_review'], 'unavailable')
            self.assertIsNone(result['review_after'])

    def test_duplicate_running_instance_even_same_version_is_unknown(self):
        self.inventory['runtime']['containers'].append(dict(self.inventory['runtime']['containers'][0]))
        with patch.object(report.urllib.request, 'urlopen') as fetch:
            result = report.observe_coolify(self.inventory)
        self.assertEqual(result['status'], 'unknown')
        fetch.assert_not_called()

    def test_response_size_redirect_and_ambiguous_versions_are_unknown(self):
        response = self.response('4.5.0')
        response.read.return_value = b'x' * (report.MAX_RELEASE + 1)
        with patch.object(report.urllib.request, 'urlopen', return_value=response):
            self.assertEqual(report.observe_coolify(self.inventory)['status'], 'unknown')
        response = self.response('4.5.0')
        response.geturl.return_value = 'https://attacker.invalid/releases/latest'
        with patch.object(report.urllib.request, 'urlopen', return_value=response):
            self.assertEqual(report.observe_coolify(self.inventory)['status'], 'unknown')
        self.inventory['runtime']['containers'].append({'image': 'ghcr.io/coollabsio/coolify:4.5.0', 'status': 'running'})
        with patch.object(report.urllib.request, 'urlopen') as fetch:
            self.assertEqual(report.observe_coolify(self.inventory)['status'], 'unknown')
        fetch.assert_not_called()


if __name__ == '__main__':
    unittest.main()
