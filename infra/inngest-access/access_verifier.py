"""Validate Cloudflare Access assertions before forwarding to Inngest."""

import re
import os
import urllib.error
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer

import jwt


class Verifier:
    def __init__(self, issuer, audience, key_lookup=None):
        if not re.fullmatch(r'https://[a-zA-Z0-9-]+\.cloudflareaccess\.com', issuer):
            raise ValueError('a Cloudflare Access team issuer is required')
        if not isinstance(audience, str) or not audience:
            raise ValueError('an application audience is required')
        self.issuer = issuer
        self.audience = audience
        if key_lookup is None:
            client = jwt.PyJWKClient(issuer + '/cdn-cgi/access/certs',
                                     cache_keys=False, lifespan=300, timeout=5)
            key_lookup = lambda token: client.get_signing_key_from_jwt(token).key
        self.key_lookup = key_lookup

    def status(self, token):
        if not isinstance(token, str) or not token or len(token) > 16384:
            return 401
        try:
            if jwt.get_unverified_header(token).get('alg') != 'RS256':
                return 401
            key = self.key_lookup(token)
            jwt.decode(token, key, algorithms=['RS256'], issuer=self.issuer,
                       audience=self.audience, leeway=15,
                       options={'require': ['exp', 'iat', 'iss', 'aud']})
        except jwt.InvalidTokenError:
            return 401
        except (jwt.PyJWKClientError, urllib.error.URLError, OSError):
            return 503
        return 200


def make_handler(verifier):
    class Handler(BaseHTTPRequestHandler):
        def do_GET(self):
            assertions = self.headers.get_all('Cf-Access-Jwt-Assertion', [])
            status = 404 if self.path != '/verify' else (
                verifier.status(assertions[0]) if len(assertions) == 1 else 401)
            self.send_response(status)
            self.send_header('Cache-Control', 'no-store')
            self.send_header('Content-Length', '0')
            self.end_headers()

        def log_message(self, *_):
            pass
    return Handler


if __name__ == '__main__':
    verifier = Verifier(os.environ['CF_ACCESS_ISSUER'], os.environ['CF_ACCESS_AUDIENCE'])
    server = ThreadingHTTPServer(('127.0.0.1', 8290), make_handler(verifier))
    server.serve_forever()
