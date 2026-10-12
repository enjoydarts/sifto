"""Bounded GitHub REST transport; mutations are never replayed."""
import json
import os
import re
import time
import urllib.error
import urllib.request
from .policy import Refused, require


class Ambiguous(Refused):
    pass


class NoRedirect(urllib.request.HTTPRedirectHandler):
    def redirect_request(self, *_):
        raise Refused('API redirect refused')


class GitHub:
    def __init__(self, repo, transport=None):
        require(re.fullmatch(r'[a-zA-Z0-9_.-]+/[a-zA-Z0-9_.-]+', repo), 'Invalid repository')
        self.repo = repo
        self.transport = transport or self.http

    def http(self, method, path, body):
        require(path.startswith('/') and not path.startswith('//') and '\\' not in path, 'Invalid API path')
        request = urllib.request.Request('https://api.github.com/repos/' + self.repo + path, method=method, data=None if body is None else json.dumps(body).encode(), headers={'Authorization': 'Bearer ' + os.environ['GH_TOKEN'], 'Accept': 'application/vnd.github+json', 'X-GitHub-Api-Version': '2022-11-28', 'Content-Type': 'application/json'})
        with urllib.request.build_opener(NoRedirect).open(request, timeout=30) as response:
            raw = response.read(5_000_001)
            require(len(raw) <= 5_000_000, 'API response too large')
            return json.loads(raw) if raw else None

    def request(self, method, path, body=None):
        for attempt in range(3 if method == 'GET' else 1):
            try:
                return self.transport(method, path, body)
            except urllib.error.HTTPError as exc:
                if method != 'GET' or exc.code not in (429, 502, 503, 504):
                    raise Refused('GitHub API request rejected (HTTP %d)' % exc.code) from None
            except (TimeoutError, OSError, ValueError):
                if method != 'GET':
                    raise Ambiguous('Mutation outcome unknown; no retry permitted') from None
            if attempt == 2:
                raise Refused('GitHub read unavailable') from None
            time.sleep(2)

    def get(self, path):
        return self.request('GET', path)

    def pages(self, path, key=None, max_pages=20, require_total=False):
        result = []
        expected_total = None
        for page in range(1, max_pages + 1):
            data = self.get(path + ('&' if '?' in path else '?') + 'per_page=100&page=' + str(page))
            rows = data[key] if key else data
            require(isinstance(rows, list), 'Malformed API pagination')
            if require_total:
                require(isinstance(data.get('total_count'), int) and data['total_count'] >= 0, 'API inventory total is missing')
                if expected_total is None:
                    expected_total = data['total_count']
                require(data['total_count'] == expected_total, 'API inventory changed during pagination')
            result.extend(rows)
            if len(rows) < 100:
                if key and 'total_count' in data:
                    require(data['total_count'] == len(result), 'Incomplete API pagination')
                return result
        raise Refused('Pagination limit reached; refusing incomplete inventory')
