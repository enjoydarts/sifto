"""Validate Cloudflare Access assertions before forwarding to Inngest."""

import re
import os
import json
import threading
import urllib.error
import urllib.request
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

    def status(self, token, common_name=None):
        if not isinstance(token, str) or not token or len(token) > 16384:
            return 401
        try:
            if jwt.get_unverified_header(token).get('alg') != 'RS256':
                return 401
            key = self.key_lookup(token)
            claims = jwt.decode(token, key, algorithms=['RS256'], issuer=self.issuer,
                       audience=self.audience, leeway=15,
                       options={'require': ['exp', 'iat', 'iss', 'aud']})
        except jwt.InvalidTokenError:
            return 401
        except (jwt.PyJWKClientError, urllib.error.URLError, OSError):
            return 503
        if common_name is not None and (not common_name or claims.get('common_name') != common_name):
            return 403
        return 200


def sync_app():
    # The caller cannot choose a URL, query, or payload. Inngest signs the SDK ping.
    payload = {'query': 'mutation { createApp(input: {url: "https://api.sifto.net/api/inngest"}) { name error functionCount } }'}
    req = urllib.request.Request('http://127.0.0.1:8288/v0/gql',
                                 data=json.dumps(payload).encode(),
                                 headers={'Content-Type': 'application/json'})
    with urllib.request.urlopen(req, timeout=45) as response:
        result = json.load(response)
    app = (result.get('data') or {}).get('createApp') or {}
    if result.get('errors') or app.get('error') or app.get('name') != 'sifto-api' or not app.get('functionCount'):
        raise ValueError('app sync failed')


def make_handler(verifier, ci_client_id=None, sync=None):
    sync = sync or sync_app
    sync_lock = threading.Lock()
    class Handler(BaseHTTPRequestHandler):
        def reply(self, status):
            self.send_response(status)
            self.send_header('Cache-Control', 'no-store')
            self.send_header('Content-Length', '0')
            self.end_headers()

        def do_GET(self):
            assertions = self.headers.get_all('Cf-Access-Jwt-Assertion', [])
            status = 404 if self.path != '/verify' else (
                verifier.status(assertions[0]) if len(assertions) == 1 else 401)
            self.reply(status)

        def do_POST(self):
            if self.path != '/sync':
                self.reply(404)
                return
            assertions = self.headers.get_all('Cf-Access-Jwt-Assertion', [])
            status = verifier.status(assertions[0], ci_client_id or '') if len(assertions) == 1 else 401
            if status != 200:
                self.reply(status)
                return
            if not sync_lock.acquire(blocking=False):
                self.reply(409)
                return
            try:
                sync()
            except (urllib.error.URLError, OSError, ValueError):
                self.reply(502)
            else:
                self.reply(200)
            finally:
                sync_lock.release()

        def log_message(self, *_):
            pass
    return Handler


if __name__ == '__main__':
    verifier = Verifier(os.environ['CF_ACCESS_ISSUER'], os.environ['CF_ACCESS_AUDIENCE'])
    server = ThreadingHTTPServer(('127.0.0.1', 8290), make_handler(verifier, os.environ.get('CF_ACCESS_CI_CLIENT_ID')))
    server.serve_forever()
