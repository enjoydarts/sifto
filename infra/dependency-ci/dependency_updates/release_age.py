"""Public PyPI artifact evidence for the fixed seven-day Python update policy."""
from datetime import datetime, timedelta, timezone
import os
import re
import urllib.request

from .github import NoRedirect
from .policy import Refused, canonical, load_json, python_lock, require

MAX_RESPONSE = 4_000_000
MAX_PACKAGES = 150
MAX_ARTIFACTS = 5000


def timestamp(value):
    require(type(value) is str and re.fullmatch(
        r'[0-9]{4}-[0-9]{2}-[0-9]{2}T[0-9]{2}:[0-9]{2}:[0-9]{2}(?:\.[0-9]{1,6})?(?:Z|[+-](?:[01][0-9]|2[0-3]):[0-5][0-9])', value),
        'Missing or invalid timezone-aware PyPI publication timestamp')
    try:
        return datetime.fromisoformat(value.replace('Z', '+00:00')).astimezone(timezone.utc)
    except ValueError:
        raise Refused('Invalid PyPI publication timestamp') from None


def utc_text(value):
    return value.astimezone(timezone.utc).isoformat().replace('+00:00', 'Z')


def release_cutoff(now=None):
    now = now if now is not None else datetime.now(timezone.utc)
    require(isinstance(now, datetime) and now.tzinfo is not None and now.utcoffset() is not None,
            'Release-age clock must be timezone aware')
    return utc_text(now.astimezone(timezone.utc) - timedelta(days=7))


class PyPI:
    def __init__(self, transport=None):
        self.transport = transport or self.http

    def http(self, url):
        request = urllib.request.Request(url, headers={'Accept': 'application/json'})
        with urllib.request.build_opener(NoRedirect).open(request, timeout=30) as response:
            require(response.status == 200, 'PyPI release metadata request rejected')
            raw = response.read(MAX_RESPONSE + 1)
            require(len(raw) <= MAX_RESPONSE, 'PyPI release metadata exceeds response limit')
            return load_json(raw)

    def release(self, name, version):
        require(type(name) is str and re.fullmatch(r'[a-z0-9](?:[a-z0-9-]{0,198}[a-z0-9])?', name)
                and type(version) is str and re.fullmatch(r'[0-9][a-zA-Z0-9_.+-]{0,199}', version),
                'Invalid public PyPI release target')
        try:
            return self.transport('https://pypi.org/pypi/' + name + '/' + version + '/json')
        except (OSError, ValueError):
            raise Refused('PyPI release metadata unavailable; seven-day policy cannot be verified') from None


def verify_python_release_age(before, after, cutoff, fetch=None):
    boundary = timestamp(cutoff)
    old, new = python_lock(before), python_lock(after)
    changed = []
    for name, pin in sorted(new.items()):
        previous = old.get(name)
        hashes = set(pin['hashes'])
        if previous and previous['version'] == pin['version']:
            hashes -= set(previous['hashes'])
        if hashes:
            changed.append((name, pin['version'], hashes))
    require(len(changed) <= MAX_PACKAGES and sum(len(hashes) for _, _, hashes in changed) <= MAX_ARTIFACTS,
            'Changed Python artifact inventory exceeds policy limit; split the PR')
    fetch = fetch or PyPI().release
    evidence = []
    for name, version, hashes in changed:
        metadata = fetch(name, version)
        require(type(metadata) is dict and type(metadata.get('info')) is dict,
                'Missing PyPI release identity')
        info = metadata['info']
        require(type(info.get('name')) is str and re.sub(r'[-_.]+', '-', info['name']).lower() == name
                and info.get('version') == version, 'PyPI release identity does not match candidate')
        urls = metadata.get('urls')
        require(type(urls) is list and 0 < len(urls) <= MAX_ARTIFACTS, 'Missing or excessive PyPI artifact inventory')
        inventory = {}
        for entry in urls:
            require(type(entry) is dict and type(entry.get('digests')) is dict,
                    'Invalid PyPI artifact digest metadata')
            digest = entry['digests'].get('sha256')
            require(type(digest) is str and re.fullmatch(r'[0-9a-f]{64}', digest)
                    and digest not in inventory and type(entry.get('yanked')) is bool,
                    'Missing or ambiguous PyPI artifact metadata')
            inventory[digest] = entry
        artifacts = []
        for digest in sorted(hashes):
            require(digest in inventory, 'Candidate hash is missing from official PyPI release metadata')
            entry = inventory[digest]
            require(entry['yanked'] is False, 'Yanked Python candidate artifact refused')
            uploaded = timestamp(entry.get('upload_time_iso_8601'))
            require(uploaded <= boundary, 'Python artifact has not satisfied the seven-day publication policy')
            artifacts.append({'sha256': digest, 'uploaded': utc_text(uploaded)})
        evidence.append({'name': name, 'version': version, 'artifacts': artifacts})
    return evidence


def generation_environment(cutoff, config='/sifto-python-generation.toml'):
    timestamp(cutoff)
    return {'UV_EXCLUDE_NEWER': cutoff, 'UV_CONFIG_FILE': config}


if __name__ == '__main__':
    # One absolute cutoff is shared by every Renovate solver invocation in a run.
    with open(os.environ['GITHUB_OUTPUT'], 'a') as output:
        output.write('custom_env=' + canonical(generation_environment(release_cutoff())) + '\n')
