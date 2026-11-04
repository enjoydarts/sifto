import time
import http.client
from http.server import ThreadingHTTPServer
import threading
import unittest
import urllib.error
import io
import json
import hashlib
import hmac
import os
from unittest.mock import patch

import jwt
from cryptography.hazmat.primitives.asymmetric import rsa

from access_verifier import Verifier, make_handler, sync_app


class SyncUpstreamTest(unittest.TestCase):
    def setUp(self):
        self.url = 'https://api.sifto.net/api/inngest'
        self.key = 'a' * 64
        self.config = {'app_id': 'sifto-api', 'url': self.url, 'sdk_language': 'go',
                       'sdk_version': '0.16.5', 'functions': [{'id': 'sifto-api-fixture', 'steps': {
                           'step': {'runtime': {'type': 'http', 'url': self.url + '?fnId=fixture'}}}}]}

    def invoke(self, config=None, signature_valid=True, registration=None):
        raw = json.dumps(config or self.config).encode()
        timestamp = str(int(time.time()))
        digest = hmac.new(self.key.encode(), raw + timestamp.encode(), hashlib.sha256).hexdigest()
        response = io.BytesIO(raw + b'\n')
        response.headers = {'X-Inngest-Signature': 't=' + timestamp + '&s=' + (digest if signature_valid else '0' * 64)}
        registered = io.BytesIO(json.dumps(registration or {'ok': True}).encode())
        with patch.dict(os.environ, INNGEST_SIGNING_KEY=self.key), patch('access_verifier.urllib.request.OpenerDirector.open', side_effect=[response, registered]) as call:
            sync_app()
            sdk_request = call.call_args_list[0].args[0]
            self.assertEqual(sdk_request.full_url, self.url)
            self.assertEqual(sdk_request.method, 'PUT')
            self.assertIn('X-inngest-signature', sdk_request.headers)
            register_request = call.call_args_list[1].args[0]
            self.assertEqual(register_request.full_url, 'http://127.0.0.1:8288/fn/register')
            self.assertEqual(json.loads(register_request.data)['functions'], self.config['functions'])

    def test_sync_requires_functions(self):
        self.config['functions'] = []
        with self.assertRaises(ValueError):
            self.invoke()

    def test_sync_rejects_forged_sdk_response(self):
        with self.assertRaises(ValueError):
            self.invoke(signature_valid=False)

    def test_sync_rejects_wrong_app_or_url(self):
        for field, value in [('app_id', 'other'), ('url', 'https://other.example/')]:
            with self.subTest(field=field), self.assertRaises(ValueError):
                self.invoke(dict(self.config, **{field: value}))

    def test_sync_rejects_foreign_function_callback(self):
        self.config['functions'][0]['steps']['step']['runtime']['url'] = 'https://other.example/'
        with self.assertRaises(ValueError):
            self.invoke()

    def test_sync_checks_registration_result(self):
        with self.assertRaises(ValueError):
            self.invoke(registration={'ok': False})

    def test_sync_accepts_signed_config_and_registers_it(self):
        self.invoke()


class AccessVerifierTest(unittest.TestCase):
    @classmethod
    def setUpClass(cls):
        cls.key = rsa.generate_private_key(public_exponent=65537, key_size=2048)
        cls.other_key = rsa.generate_private_key(public_exponent=65537, key_size=2048)

    def setUp(self):
        self.issuer = 'https://example.cloudflareaccess.com'
        self.audience = 'a' * 64
        self.verifier = Verifier(self.issuer, self.audience, lambda _: self.key.public_key())

    def token(self, **changes):
        now = int(time.time())
        claims = {'iss': self.issuer, 'aud': [self.audience], 'exp': now + 300,
                  'iat': now - 1, 'common_name': 'synthetic-service.access'}
        claims.update(changes)
        return jwt.encode(claims, self.key, algorithm='RS256', headers={'kid': 'synthetic'})

    def test_accepts_current_signed_assertion_for_this_application(self):
        self.assertEqual(self.verifier.status(self.token()), 200)

    def test_rejects_signed_assertion_for_another_application(self):
        self.assertEqual(self.verifier.status(self.token(aud=['b' * 64])), 401)

    def test_rejects_signed_assertion_from_another_team(self):
        self.assertEqual(self.verifier.status(self.token(iss='https://other.cloudflareaccess.com')), 401)

    def test_rejects_expired_assertion(self):
        self.assertEqual(self.verifier.status(self.token(exp=int(time.time()) - 120)), 401)

    def test_rejects_assertion_with_future_issue_time(self):
        self.assertEqual(self.verifier.status(self.token(iat=int(time.time()) + 120)), 401)

    def test_rejects_missing_expiration(self):
        claims = jwt.decode(self.token(), options={'verify_signature': False})
        del claims['exp']
        self.assertEqual(self.verifier.status(jwt.encode(claims, self.key, algorithm='RS256')), 401)

    def test_rejects_forged_signature(self):
        claims = jwt.decode(self.token(), options={'verify_signature': False})
        forged = jwt.encode(claims, self.other_key, algorithm='RS256')
        self.assertEqual(self.verifier.status(forged), 401)

    def test_rejects_unsigned_and_hmac_assertions_without_key_lookup(self):
        def forbidden(_):
            self.fail('unsupported algorithms must not fetch a signing key')
        verifier = Verifier(self.issuer, self.audience, forbidden)
        claims = jwt.decode(self.token(), options={'verify_signature': False})
        for algorithm, key in [('none', None), ('HS256', 'synthetic-hmac-key')]:
            with self.subTest(algorithm=algorithm):
                self.assertEqual(verifier.status(jwt.encode(claims, key, algorithm=algorithm)), 401)

    def test_rejects_missing_malformed_and_oversized_assertions(self):
        for value in (None, '', 'not-a-jwt', 'a' * 20000):
            with self.subTest(value_length=len(value or '')):
                self.assertEqual(self.verifier.status(value), 401)

    def test_key_service_outage_does_not_authorize(self):
        def unavailable(_):
            raise urllib.error.URLError('synthetic JWKS outage')
        verifier = Verifier(self.issuer, self.audience, unavailable)
        self.assertEqual(verifier.status(self.token()), 503)

    def test_refuses_non_cloudflare_key_source(self):
        with self.assertRaises(ValueError):
            Verifier('https://attacker.example', self.audience)


class HTTPBoundaryTest(unittest.TestCase):
    @classmethod
    def setUpClass(cls):
        cls.key = rsa.generate_private_key(public_exponent=65537, key_size=2048)

    def setUp(self):
        issuer = 'https://example.cloudflareaccess.com'
        audience = 'a' * 64
        now = int(time.time())
        self.token = jwt.encode({'iss': issuer, 'aud': [audience], 'iat': now - 1,
                                 'exp': now + 300}, self.key, algorithm='RS256')
        verifier = Verifier(issuer, audience, lambda _: self.key.public_key())
        self.sync_calls = []
        def sync():
            self.sync_calls.append(True)
        self.server = ThreadingHTTPServer(('127.0.0.1', 0), make_handler(verifier, 'ci.access', sync))
        self.thread = threading.Thread(target=self.server.serve_forever, daemon=True)
        self.thread.start()

    def tearDown(self):
        self.server.shutdown()
        self.server.server_close()
        self.thread.join(timeout=2)

    def request(self, headers=(), path='/verify', method='GET'):
        client = http.client.HTTPConnection(*self.server.server_address, timeout=2)
        try:
            client.putrequest(method, path)
            for name, value in headers:
                client.putheader(name, value)
            client.endheaders()
            response = client.getresponse()
            return response.status, response.read()
        finally:
            client.close()

    def test_http_accepts_valid_access_assertion(self):
        status, _ = self.request([('Cf-Access-Jwt-Assertion', self.token)])
        self.assertEqual(status, 200)

    def test_http_rejects_missing_access_assertion(self):
        self.assertEqual(self.request()[0], 401)

    def test_http_does_not_treat_authorization_as_access_assertion(self):
        self.assertEqual(self.request([('Authorization', 'Bearer ' + self.token)])[0], 401)

    def test_http_rejects_ambiguous_duplicate_assertions(self):
        self.assertEqual(self.request([('Cf-Access-Jwt-Assertion', self.token)] * 2)[0], 401)

    def test_http_does_not_echo_rejected_token(self):
        status, body = self.request([('Cf-Access-Jwt-Assertion', 'sensitive-synthetic-value')])
        self.assertEqual(status, 401)
        self.assertNotIn(b'sensitive-synthetic-value', body)

    def test_http_refuses_other_paths(self):
        self.assertEqual(self.request(path='/other')[0], 404)

    def service_token(self, common_name):
        claims = jwt.decode(self.token, options={'verify_signature': False})
        claims['common_name'] = common_name
        return jwt.encode(claims, self.key, algorithm='RS256')

    def test_sync_accepts_only_signed_ci_identity(self):
        status, _ = self.request([('Cf-Access-Jwt-Assertion', self.service_token('ci.access'))], '/sync', 'POST')
        self.assertEqual(status, 200)
        self.assertEqual(self.sync_calls, [True])

    def test_sync_rejects_runtime_identity(self):
        self.assertEqual(self.request([('Cf-Access-Jwt-Assertion', self.service_token('runtime.access'))], '/sync', 'POST')[0], 403)
        self.assertEqual(self.sync_calls, [])

    def test_sync_rejects_unsigned_identity(self):
        self.assertEqual(self.request(path='/sync', method='POST')[0], 401)
        self.assertEqual(self.sync_calls, [])

    def test_sync_is_not_available_via_get(self):
        self.assertEqual(self.request(path='/sync')[0], 404)

    def test_sync_rejects_duplicate_assertions(self):
        headers = [('Cf-Access-Jwt-Assertion', self.service_token('ci.access'))] * 2
        self.assertEqual(self.request(headers, '/sync', 'POST')[0], 401)
        self.assertEqual(self.sync_calls, [])

    def test_sync_failure_is_not_reported_as_success(self):
        def fail():
            raise urllib.error.URLError('sensitive synthetic upstream error')
        verifier = Verifier('https://example.cloudflareaccess.com', 'a' * 64, lambda _: self.key.public_key())
        server = ThreadingHTTPServer(('127.0.0.1', 0), make_handler(verifier, 'ci.access', fail))
        thread = threading.Thread(target=server.serve_forever, daemon=True)
        thread.start()
        client = http.client.HTTPConnection(*server.server_address, timeout=2)
        try:
            client.request('POST', '/sync', headers={'Cf-Access-Jwt-Assertion': self.service_token('ci.access')})
            response = client.getresponse()
            self.assertEqual(response.status, 502)
            self.assertNotIn(b'sensitive', response.read())
        finally:
            client.close(); server.shutdown(); server.server_close(); thread.join(timeout=2)


if __name__ == '__main__':
    unittest.main()
