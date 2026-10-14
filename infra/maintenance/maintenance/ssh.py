"""Dedicated forced-command transport. Remote commands never come from data."""
import ipaddress
import json
import os
import re
import subprocess
import threading
from pathlib import Path

from .plan import HOST, MaintenanceError, canonical_json, matches, require

MAX_OUTPUT = 16 * 1024 * 1024


def parse_hosts(raw):
    from .host import strict_json
    hosts = strict_json(raw)
    require(type(hosts) is list and 1 <= len(hosts) <= 16, "invalid host configuration")
    seen = set()
    for host in hosts:
        require(type(host) is dict and set(host) == {"host_id", "address", "port", "role"}, "invalid host fields")
        require(matches(HOST, host["host_id"]) and host["host_id"] not in seen, "invalid or duplicate host ID")
        seen.add(host["host_id"])
        require(host["role"] in ("application", "database") and type(host["port"]) is int
                and 1 <= host["port"] <= 65535, "invalid host role or port")
        address = host["address"]
        require(isinstance(address, str) and 1 <= len(address) <= 253, "invalid host address")
        try:
            ipaddress.ip_address(address)
        except ValueError:
            require(re.fullmatch(r"[A-Za-z0-9](?:[A-Za-z0-9.-]*[A-Za-z0-9])?", address)
                    and all(re.fullmatch(r"[A-Za-z0-9](?:[A-Za-z0-9-]{0,61}[A-Za-z0-9])?", label)
                            for label in address.split(".")), "invalid host address")
    return hosts


def capture(argv, *, data=None, timeout=120, env=None, limit=MAX_OUTPUT):
    """Drain both streams with a hard memory cap; never time out a mutation.

    Oversized output is discarded while draining, so apt is never killed by a
    diagnostic output limit. Only read-only/metadata commands have deadlines.
    """
    process = subprocess.Popen(argv, stdin=subprocess.PIPE, stdout=subprocess.PIPE,
                               stderr=subprocess.PIPE, env=env, shell=False)
    output = [bytearray(), bytearray()]
    oversized = [False, False]

    def drain(stream, index):
        while chunk := stream.read(65536):
            available = max(0, limit - len(output[index]))
            output[index].extend(chunk[:available])
            oversized[index] |= len(chunk) > available

    threads = [threading.Thread(target=drain, args=(process.stdout, 0)),
               threading.Thread(target=drain, args=(process.stderr, 1))]
    for thread in threads:
        thread.start()
    timed_out = False
    try:
        try:
            process.stdin.write(data or b"")
            process.stdin.close()
        except BrokenPipeError:
            pass
        process.wait(timeout=timeout)
    except subprocess.TimeoutExpired:
        timed_out = True
        process.kill()
        process.wait()
    finally:
        if not process.stdin.closed:
            try:
                process.stdin.close()
            except BrokenPipeError:
                pass
        for thread in threads:
            thread.join()
        process.stdout.close()
        process.stderr.close()
    return process.returncode, bytes(output[0]), bytes(output[1]), timed_out or any(oversized)


class Client:
    def __init__(self, workdir, key, known_hosts, role):
        require(role in ("scan", "apply"), "invalid SSH role")
        self.workdir, self.key, self.known_hosts, self.role = Path(workdir), Path(key), Path(known_hosts), role

    def argv(self, host, mode):
        parse_hosts(json.dumps([host]))
        require(mode in ({"scan", "refresh", "report", "coolify-report"} if self.role == "scan" else {"apply", "verify"}),
                "forbidden SSH command")
        return ["ssh", "-T", "-F", "/dev/null", "-i", str(self.key), "-p", str(host["port"]),
                "-o", "BatchMode=yes", "-o", "IdentitiesOnly=yes", "-o", "StrictHostKeyChecking=yes",
                "-o", "UserKnownHostsFile=" + str(self.known_hosts), "-o", "GlobalKnownHostsFile=/dev/null",
                "-o", "ForwardAgent=no", "-o", "ClearAllForwardings=yes", "-o", "ControlMaster=no",
                "-o", "ControlPath=none", "-o", "ConnectTimeout=15", "-o", "LogLevel=ERROR",
                "--", "sifto-maint-" + self.role + "@" + host["address"], mode]

    def call(self, host, mode, request=None):
        from .host import strict_json
        from .store import private_write
        result = capture(self.argv(host, mode), data=canonical_json(request or {}),
                         timeout=None if mode == "apply" else 180)
        code, stdout, stderr, capped = result
        import uuid
        directory = self.workdir / "transport" / str(uuid.uuid4())
        private_write(directory / "stdout.bin", stdout)
        private_write(directory / "stderr.bin", stderr)
        require(not capped and code == 0, "ssh_output_limit" if capped else "ssh_failed")
        try:
            return strict_json(stdout)
        except (MaintenanceError, UnicodeError):
            raise MaintenanceError("ssh_invalid_json") from None
