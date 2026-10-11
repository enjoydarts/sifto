"""Private Git object storage: immutable reads, no checkout of downloaded files."""
import os
import re
import shlex
import uuid
from pathlib import Path

from .plan import SHA, MaintenanceError, canonical_json, matches, require
from .ssh import capture


def private_write(path, value):
    path = Path(path)
    path.parent.mkdir(parents=True, exist_ok=True, mode=0o700)
    os.chmod(path.parent, 0o700)
    require(not path.is_symlink(), "private path cannot be a symlink")
    descriptor = os.open(path, os.O_CREAT | os.O_TRUNC | os.O_WRONLY | os.O_NOFOLLOW, 0o600)
    with os.fdopen(descriptor, "wb") as stream:
        os.fchmod(stream.fileno(), 0o600)
        stream.write(value if isinstance(value, bytes) else canonical_json(value))


def repo_url(repo):
    require(isinstance(repo, str) and re.fullmatch(r"[A-Za-z0-9][A-Za-z0-9_-]{0,99}/[A-Za-z0-9][A-Za-z0-9_.-]{0,99}", repo),
            "invalid private repository")
    require(repo.split("/")[1] not in (".", ".."), "invalid private repository")
    return "ssh://git@github.com/" + repo + ".git"


def validate_ref(commit, path):
    require(matches(SHA, commit), "immutable commit required")
    require(isinstance(path, str) and len(path) <= 512
            and re.fullmatch(r"[A-Za-z0-9][A-Za-z0-9_./-]*", path)
            and all(re.fullmatch(r"[A-Za-z0-9][A-Za-z0-9_.-]*", part)
                    and part not in (".", "..") for part in path.split("/")), "invalid private object path")


class GitStore:
    def __init__(self, root, repo, key, known_hosts, *, writer=False):
        self.root, self.repo, self.writer = Path(root), repo, writer
        self.url = repo_url(repo)
        self.root.mkdir(mode=0o700, parents=True, exist_ok=True)
        os.chmod(self.root, 0o700)
        self.env = {"PATH": os.environ["PATH"], "HOME": str(self.root), "GIT_TERMINAL_PROMPT": "0",
                    "GIT_CONFIG_NOSYSTEM": "1", "GIT_CONFIG_GLOBAL": "/dev/null",
                    "GIT_AUTHOR_NAME": "Maintenance automation", "GIT_COMMITTER_NAME": "Maintenance automation",
                    "GIT_AUTHOR_EMAIL": "maintenance@users.noreply.github.com",
                    "GIT_COMMITTER_EMAIL": "maintenance@users.noreply.github.com",
                    "GIT_SSH_COMMAND": "ssh -T -F /dev/null -i " + shlex.quote(str(key)) +
                    " -o BatchMode=yes -o IdentitiesOnly=yes -o StrictHostKeyChecking=yes"
                    " -o GlobalKnownHostsFile=/dev/null -o ForwardAgent=no -o ClearAllForwardings=yes"
                    " -o ControlMaster=no -o ControlPath=none -o ConnectTimeout=15 -o UserKnownHostsFile=" +
                    shlex.quote(str(known_hosts))}
        self.git("init", "--bare", str(self.root / "objects.git"))
        self.git("config", "core.hooksPath", "/dev/null")
        self.git("config", "protocol.file.allow", "never")

    def git(self, *arguments, data=None, allow_missing=False):
        argv = ["git", "--git-dir=" + str(self.root / "objects.git"), "-c", "core.hooksPath=/dev/null", *arguments]
        code, stdout, stderr, capped = capture(argv, data=data, env=self.env, timeout=180)
        private_write(self.root / "git-stdout.bin", stdout)
        private_write(self.root / "git-stderr.bin", stderr)
        require(not capped and (code == 0 or allow_missing), "private_git_failed")
        return stdout

    def fetch(self, commit, path, *, digest=None):
        validate_ref(commit, path)
        self.git("fetch", "--depth=1", "--no-tags", self.url, commit)
        raw = self.git("show", commit + ":" + path)
        if digest is not None:
            import hashlib
            require(re.fullmatch(r"[0-9a-f]{64}", digest) is not None and hashlib.sha256(raw).hexdigest() == digest,
                    "private object hash mismatch")
        return raw

    def head(self):
        require(self.writer, "read-only private store")
        self.git("fetch", "--depth=1", "--no-tags", self.url, "refs/heads/main")
        commit = self.git("rev-parse", "FETCH_HEAD").decode().strip()
        require(matches(SHA, commit), "invalid private head")
        return commit

    def read_at(self, commit, path, *, optional=False):
        validate_ref(commit, path)
        raw = self.git("show", commit + ":" + path, allow_missing=optional)
        return raw or None

    def write(self, parent, files):
        require(self.writer and matches(SHA, parent), "private writer required")
        self.env["GIT_INDEX_FILE"] = str(self.root / (str(uuid.uuid4()) + ".index"))
        self.git("read-tree", parent)
        for path, value in sorted(files.items()):
            validate_ref(parent, path)
            raw = value if isinstance(value, bytes) else canonical_json(value)
            blob = self.git("hash-object", "-w", "--stdin", data=raw).decode().strip()
            require(matches(SHA, blob), "invalid private blob")
            self.git("update-index", "--add", "--cacheinfo", "100644," + blob + "," + path)
        tree = self.git("write-tree").decode().strip()
        require(matches(SHA, tree), "invalid private tree")
        commit = self.git("commit-tree", tree, "-p", parent, data=b"Private maintenance record\n").decode().strip()
        require(matches(SHA, commit), "invalid private commit")
        # A conflicting push fails; never force-push or silently rewrite evidence.
        self.git("push", self.url, commit + ":refs/heads/main")
        return commit
