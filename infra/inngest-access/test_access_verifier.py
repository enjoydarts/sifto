import time
import http.client
from http.server import ThreadingHTTPServer
import threading
import unittest
import urllib.error

import jwt
from cryptography.hazmat.primitives.asymmetric import rsa

from access_verifier import Verifier, make_handler


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
        self.server = ThreadingHTTPServer(('127.0.0.1', 0), make_handler(verifier))
        self.thread = threading.Thread(target=self.server.serve_forever, daemon=True)
        self.thread.start()

    def tearDown(self):
        self.server.shutdown()
        self.server.server_close()
        self.thread.join(timeout=2)

    def request(self, headers=(), path='/verify'):
        client = http.client.HTTPConnection(*self.server.server_address, timeout=2)
        try:
            client.putrequest('GET', path)
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


if __name__ == '__main__':
    unittest.main()
