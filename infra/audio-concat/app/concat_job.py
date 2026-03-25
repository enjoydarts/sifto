import json
import os
import shlex
import shutil
import subprocess
import tempfile
import urllib.request
from pathlib import Path

from app.callback_client import post_callback
from app.r2_client import R2Client


def run_from_env() -> int:
    job_id = required_env("AUDIO_BRIEFING_JOB_ID")
    request_id = required_env("AUDIO_BRIEFING_REQUEST_ID")
    callback_url = required_env("AUDIO_BRIEFING_CALLBACK_URL")
    callback_token = required_env("AUDIO_BRIEFING_CALLBACK_TOKEN")
    output_object_key = required_env("AUDIO_BRIEFING_OUTPUT_OBJECT_KEY")
    audio_object_keys = json.loads(required_env("AUDIO_BRIEFING_AUDIO_OBJECT_KEYS_JSON"))
    provider_job_id = os.getenv("CLOUD_RUN_EXECUTION", "").strip() or None
    return run_job(
        job_id=job_id,
        request_id=request_id,
        callback_url=callback_url,
        callback_token=callback_token,
        output_object_key=output_object_key,
        audio_object_keys=audio_object_keys,
        provider_job_id=provider_job_id,
    )


def run_job(
    *,
    job_id: str,
    request_id: str,
    callback_url: str,
    callback_token: str,
    output_object_key: str,
    audio_object_keys: list[str],
    provider_job_id: str | None = None,
) -> int:
    try:
        with tempfile.TemporaryDirectory(prefix="audio-concat-") as tmp_dir:
            tmp_path = Path(tmp_dir)
            r2 = R2Client()
            segment_files = download_segments(tmp_path, r2, audio_object_keys)
            output_path = tmp_path / "episode.mp3"
            concat_audio(segment_files, output_path)
            duration_sec = probe_duration_seconds(output_path)
            r2.upload_file(output_path, output_object_key)
        post_callback(
            callback_url,
            callback_token,
            {
                "request_id": request_id,
                "provider_job_id": provider_job_id,
                "status": "published",
                "audio_object_key": output_object_key,
                "audio_duration_sec": duration_sec,
            },
        )
        return 0
    except Exception as exc:
        post_callback(
            callback_url,
            callback_token,
            {
                "request_id": request_id,
                "provider_job_id": provider_job_id,
                "status": "failed",
                "error_code": "concat_runtime_failed",
                "error_message": str(exc),
            },
        )
        return 1


def required_env(name: str) -> str:
    value = os.getenv(name, "").strip()
    if not value:
        raise RuntimeError(f"missing env: {name}")
    return value


def download_segments(tmp_path: Path, r2: R2Client, audio_object_keys: list[str]) -> list[Path]:
    if not audio_object_keys:
        raise RuntimeError("audio object keys are empty")
    downloaded: list[Path] = []
    for index, object_key in enumerate(audio_object_keys, start=1):
        suffix = Path(object_key).suffix or ".mp3"
        destination = tmp_path / f"segment-{index:03d}{suffix}"
        if object_key.startswith("https://") or object_key.startswith("http://"):
            download_direct(object_key, destination)
        else:
            r2.download_file(object_key, destination)
        downloaded.append(destination)
    return downloaded


def download_direct(url: str, destination: Path) -> None:
    destination.parent.mkdir(parents=True, exist_ok=True)
    with urllib.request.urlopen(url, timeout=60) as response:
        with destination.open("wb") as stream:
            shutil.copyfileobj(response, stream)


def concat_audio(segment_files: list[Path], output_path: Path) -> None:
    list_path = output_path.parent / "concat.txt"
    list_path.write_text("".join(f"file {shlex.quote(str(path))}\n" for path in segment_files), encoding="utf-8")
    first_try = [
        "ffmpeg",
        "-y",
        "-f",
        "concat",
        "-safe",
        "0",
        "-i",
        str(list_path),
        "-c",
        "copy",
        str(output_path),
    ]
    if run_command(first_try):
        return
    second_try = [
        "ffmpeg",
        "-y",
        "-f",
        "concat",
        "-safe",
        "0",
        "-i",
        str(list_path),
        "-c:a",
        "libmp3lame",
        "-q:a",
        "2",
        str(output_path),
    ]
    if run_command(second_try):
        return
    raise RuntimeError("ffmpeg concat failed")


def run_command(command: list[str]) -> bool:
    completed = subprocess.run(command, capture_output=True, text=True, check=False)
    return completed.returncode == 0


def probe_duration_seconds(output_path: Path) -> int:
    command = [
        "ffprobe",
        "-v",
        "error",
        "-show_entries",
        "format=duration",
        "-of",
        "default=noprint_wrappers=1:nokey=1",
        str(output_path),
    ]
    completed = subprocess.run(command, capture_output=True, text=True, check=False)
    if completed.returncode != 0:
        raise RuntimeError("ffprobe failed")
    raw = completed.stdout.strip()
    duration = float(raw)
    return max(0, int(round(duration)))
