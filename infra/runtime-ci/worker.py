"""Exercise the final image without provider credentials or external access."""
import json
import os
import pathlib
import shutil
import stat
import subprocess
import tempfile
import time
import urllib.error
import urllib.request

from yt_dlp import YoutubeDL

assert shutil.which("curl") is None, "Worker must not depend on curl"
healthcheck = json.loads(os.environ["WORKER_HEALTHCHECK"])
assert healthcheck[0] == "CMD", "Worker must define an exec-form image health check"
healthcheck = healthcheck[1:]

assert os.getuid() == os.getgid() == 10001
assert not os.getenv("INTERNAL_WORKER_SECRET")
assert pathlib.Path.home() == pathlib.Path("/home/sifto")
assert pathlib.Path(os.environ["XDG_CACHE_HOME"]).is_relative_to(pathlib.Path.home())
assert not os.access("/app/app/main.py", os.W_OK)
json.loads(pathlib.Path("/app/shared/llm_catalog.json").read_text())
with tempfile.TemporaryDirectory() as directory:
    probe = pathlib.Path(directory) / "download"
    probe.write_bytes(b"runtime-check")
    assert probe.read_bytes() == b"runtime-check"
    credentials = pathlib.Path(directory) / "public-fixture.json"
    subprocess.run(
        ["/usr/local/bin/docker-entrypoint.sh", "true"], check=True,
        env={**os.environ, "GOOGLE_SERVICE_ACCOUNT_JSON": '{"runtime_ci":true}',
             "GOOGLE_APPLICATION_CREDENTIALS": str(credentials)},
    )
    assert json.loads(credentials.read_text()) == {"runtime_ci": True}
    assert credentials.stat().st_uid == 10001
    assert stat.S_IMODE(credentials.stat().st_mode) == 0o600
with YoutubeDL({"quiet": True}) as downloader:
    downloader.cache.store("runtime-ci", "probe", {"ok": True})
    assert downloader.cache.load("runtime-ci", "probe") == {"ok": True}
    cache = pathlib.Path(downloader.cache._get_root_dir())
    assert cache.is_relative_to(pathlib.Path.home())
    assert cache.stat().st_uid == 10001

# Start the actual production entrypoint and server; keep all traffic loopback.
with tempfile.TemporaryFile() as output:
    server = subprocess.Popen(
        ["/usr/local/bin/docker-entrypoint.sh", "uvicorn", "app.main:app",
         "--host", "127.0.0.1", "--port", "8000"],
        stdout=output, stderr=output,
    )
    try:
        for _ in range(120):
            if server.poll() is not None:
                raise AssertionError("Worker exited before health became ready")
            try:
                with urllib.request.urlopen("http://127.0.0.1:8000/health", timeout=1) as response:
                    assert response.status == 200
                    assert json.load(response)["status"] == "ok"
                break
            except (urllib.error.URLError, TimeoutError):
                time.sleep(0.5)
        else:
            raise AssertionError("Worker health did not become ready")
        # Execute the image's actual probe with a live server and hostile proxy settings.
        subprocess.run(healthcheck, check=True, timeout=5,
                       env={**os.environ, "HTTP_PROXY": "http://127.0.0.1:1",
                            "ALL_PROXY": "http://127.0.0.1:1", "NO_PROXY": ""})
        status = pathlib.Path(f"/proc/{server.pid}/status").read_text().splitlines()
        for field in ("Uid:", "Gid:"):
            assert next(line for line in status if line.startswith(field)).split()[1:] == ["10001"] * 4
        try:
            urllib.request.urlopen("http://127.0.0.1:8000/extract", timeout=2)
        except urllib.error.HTTPError as error:
            assert error.code == 503, "Worker must fail closed without its internal secret"
        else:
            raise AssertionError("Worker accepted an unauthenticated protected request")
    finally:
        server.terminate()
        try:
            server.wait(timeout=10)
        except subprocess.TimeoutExpired:
            server.kill()
            server.wait()
    assert subprocess.run(healthcheck, timeout=5, stdout=subprocess.DEVNULL,
                          stderr=subprocess.DEVNULL).returncode != 0, "Stopped Worker must fail its health check"
print("Worker uid/gid, temp, yt-dlp cache, health, and missing-secret denial verified")
