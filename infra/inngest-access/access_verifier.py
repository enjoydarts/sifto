"""Validate Cloudflare Access assertions before forwarding to Inngest."""

import re
import os
import json
import threading
import time
import hashlib
import hmac
import urllib.error
import urllib.request
import urllib.parse
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
    # Go SDK 0.16 returns in-band config; OSS Ping does not register that response.
    # Perform the signed exchange explicitly. Caller input is never forwarded.
    sdk_url = 'https://api.sifto.net/api/inngest'
    signing_key = os.environ.get('INNGEST_SIGNING_KEY', '')
    key = re.sub(r'^signkey-\w+-', '', signing_key)
    if not re.fullmatch(r'[0-9a-fA-F]{64}', key):
        raise ValueError('missing signing key')
    body = json.dumps({'url': sdk_url}, separators=(',', ':')).encode()
    timestamp = str(int(time.time()))
    digest = hmac.new(key.encode(), body + timestamp.encode(), hashlib.sha256).hexdigest()
    req = urllib.request.Request(sdk_url, data=body, method='PUT', headers={
        'Content-Type': 'application/json', 'X-Inngest-Signature': 't=' + timestamp + '&s=' + digest})
    # Redirects would send a signed request to an unintended endpoint.
    class NoRedirect(urllib.request.HTTPRedirectHandler):
        def redirect_request(self, *_):
            return None
    opener = urllib.request.build_opener(NoRedirect)
    with opener.open(req, timeout=20) as response:
        raw = response.read(2 * 1024 * 1024 + 1)
        signature = response.headers.get('X-Inngest-Signature', '')
    if len(raw) > 2 * 1024 * 1024:
        raise ValueError('oversized SDK response')
    parts = urllib.parse.parse_qs(signature, strict_parsing=True)
    if set(parts) != {'t', 's'} or any(len(v) != 1 for v in parts.values()):
        raise ValueError('invalid SDK signature')
    if abs(int(time.time()) - int(parts['t'][0])) > 300:
        raise ValueError('expired SDK signature')
    expected = hmac.new(key.encode(), raw.removesuffix(b'\n') + parts['t'][0].encode(), hashlib.sha256).hexdigest()
    if not hmac.compare_digest(expected, parts['s'][0]):
        raise ValueError('invalid SDK signature')
    config = json.loads(raw)
    if config.get('app_id') != 'sifto-api' or config.get('url') != sdk_url or not config.get('functions'):
        raise ValueError('invalid SDK config')
    for function in config['functions']:
        if not function.get('steps'):
            raise ValueError('missing steps')
        for step in function['steps'].values():
            url = urllib.parse.urlsplit(step.get('runtime', {}).get('url', ''))
            if url.scheme != 'https' or url.netloc != 'api.sifto.net' or url.path != '/api/inngest' or url.fragment:
                raise ValueError('unexpected callback')
    registration = {'v': '1', 'url': sdk_url, 'deployType': 'ping', 'appName': 'sifto-api',
                    'sdk': config['sdk_language'] + ':' + config['sdk_version'],
                    'functions': config['functions'], 'headers': {},
                    'capabilities': {'in_band_sync': 'v1'}}
    prefix = signing_key.removesuffix(key)
    bearer = prefix + hashlib.sha256(bytes.fromhex(key)).hexdigest()
    req = urllib.request.Request('http://127.0.0.1:8288/fn/register',
                                 data=json.dumps(registration).encode(), headers={
                                     'Content-Type': 'application/json', 'Authorization': 'Bearer ' + bearer})
    with opener.open(req, timeout=20) as response:
        result = json.loads(response.read(1024 * 1024))
    if result.get('ok') is not True:
        raise ValueError('app registration failed')


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
