import json
import logging
import os
import random
import shlex
import shutil
import subprocess
import tempfile
import urllib.request
from pathlib import Path

from app.callback_client import post_callback
from app.r2_client import R2Client

logger = logging.getLogger(__name__)

_BGM_ALLOWED_SUFFIXES = {".mp3", ".m4a", ".aac", ".wav", ".ogg"}
_BGM_FADE_IN_SEC = 2
_BGM_FADE_OUT_SEC = 4
_LOUDNORM_TARGET_I = "-16"
_LOUDNORM_TARGET_TP = "-1.5"
_BGM_VOLUME_FILTER = "volume=0.06"
_OUTPUT_CHANNELS = "2"


def run_from_env() -> int:
    job_id = required_env("AUDIO_BRIEFING_JOB_ID")
    request_id = required_env("AUDIO_BRIEFING_REQUEST_ID")
    callback_url = required_env("AUDIO_BRIEFING_CALLBACK_URL")
    callback_token = required_env("AUDIO_BRIEFING_CALLBACK_TOKEN")
    output_object_key = required_env("AUDIO_BRIEFING_OUTPUT_OBJECT_KEY")
    audio_object_keys = json.loads(required_env("AUDIO_BRIEFING_AUDIO_OBJECT_KEYS_JSON"))
    bgm_enabled = env_bool("AUDIO_BRIEFING_BGM_ENABLED")
    bgm_r2_prefix = os.getenv("AUDIO_BRIEFING_BGM_R2_PREFIX", "").strip() or None
    provider_job_id = os.getenv("CLOUD_RUN_EXECUTION", "").strip() or None
    return run_job(
        job_id=job_id,
        request_id=request_id,
        callback_url=callback_url,
        callback_token=callback_token,
        output_object_key=output_object_key,
        audio_object_keys=audio_object_keys,
        provider_job_id=provider_job_id,
        bgm_enabled=bgm_enabled,
        bgm_r2_prefix=bgm_r2_prefix,
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
    bgm_enabled: bool = False,
    bgm_r2_prefix: str | None = None,
) -> int:
    try:
        with tempfile.TemporaryDirectory(prefix="audio-concat-") as tmp_dir:
            tmp_path = Path(tmp_dir)
            r2 = R2Client()
            segment_files = download_segments(tmp_path, r2, audio_object_keys)
            concat_path = tmp_path / "episode-concat.mp3"
            concat_audio(segment_files, concat_path)
            bgm_object_key = None
            try:
                if bgm_enabled and bgm_r2_prefix:
                    bgm_object_key, output_path = mix_bgm_with_normalize(concat_path, tmp_path, r2, bgm_r2_prefix)
                else:
                    output_path = normalize_audio(concat_path, tmp_path / "episode.mp3")
            except Exception:
                logger.exception("audio concat bgm mix failed", extra={"job_id": job_id, "bgm_r2_prefix": bgm_r2_prefix})
                bgm_object_key = None
                output_path = normalize_audio(concat_path, tmp_path / "episode.mp3")
            duration_sec = probe_duration_seconds(output_path)
            r2.upload_file(output_path, output_object_key)
        callback_error = try_post_callback(
            callback_url,
            callback_token,
            {
                "request_id": request_id,
                "provider_job_id": provider_job_id,
                "status": "published",
                "audio_object_key": output_object_key,
                "bgm_object_key": bgm_object_key,
                "audio_duration_sec": duration_sec,
            },
        )
        if callback_error is None:
            return 0
        try_post_callback(
            callback_url,
            callback_token,
            {
                "request_id": request_id,
                "provider_job_id": provider_job_id,
                "status": "failed",
                "error_code": "concat_callback_failed",
                "error_message": str(callback_error),
            },
        )
        return 1
    except Exception as exc:
        try_post_callback(
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


def try_post_callback(callback_url: str, callback_token: str, payload: dict) -> Exception | None:
    try:
        post_callback(callback_url, callback_token, payload)
        return None
    except Exception as exc:
        logger.exception("audio concat callback failed", extra={"callback_url": callback_url, "status": payload.get("status")})
        return exc


def required_env(name: str) -> str:
    value = os.getenv(name, "").strip()
    if not value:
        raise RuntimeError(f"missing env: {name}")
    return value


def env_bool(name: str) -> bool:
    return os.getenv(name, "").strip().lower() in {"1", "true", "yes", "on"}


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


def list_bgm_candidates(r2: R2Client, prefix: str) -> list[str]:
    prefix = prefix.strip()
    if not prefix:
        return []
    keys = r2.list_object_keys(prefix)
    return [key for key in keys if Path(key).suffix.lower() in _BGM_ALLOWED_SUFFIXES]


def mix_bgm_with_normalize(main_audio_path: Path, tmp_path: Path, r2: R2Client, bgm_r2_prefix: str) -> tuple[str, Path]:
    candidates = list_bgm_candidates(r2, bgm_r2_prefix)
    if not candidates:
        raise RuntimeError("bgm candidates are empty")
    bgm_object_key = random.choice(candidates)
    bgm_path = tmp_path / f"bgm{Path(bgm_object_key).suffix or '.mp3'}"
    r2.download_file(bgm_object_key, bgm_path)

    output_path = tmp_path / "episode.mp3"
    duration_sec = probe_duration_seconds(main_audio_path)
    fade_out_start = max(duration_sec - _BGM_FADE_OUT_SEC, 0)
    command = [
        "ffmpeg",
        "-y",
        "-i",
        str(main_audio_path),
        "-stream_loop",
        "-1",
        "-i",
        str(bgm_path),
        "-filter_complex",
        (
            f"[0:a]aformat=channel_layouts=stereo[voice];"
            f"[1:a]atrim=duration={duration_sec},aformat=channel_layouts=stereo,"
            f"afade=t=in:st=0:d={_BGM_FADE_IN_SEC},"
            f"afade=t=out:st={fade_out_start}:d={_BGM_FADE_OUT_SEC},{_BGM_VOLUME_FILTER}[bgm];"
            f"[voice][bgm]amix=inputs=2:duration=first:dropout_transition=0,"
            f"loudnorm=I={_LOUDNORM_TARGET_I}:TP={_LOUDNORM_TARGET_TP}:LRA=11[out]"
        ),
        "-map",
        "[out]",
        "-ac",
        _OUTPUT_CHANNELS,
        "-c:a",
        "libmp3lame",
        "-q:a",
        "2",
        str(output_path),
    ]
    if not run_command(command):
        raise RuntimeError("ffmpeg bgm mix failed")
    return bgm_object_key, output_path


def normalize_audio(input_path: Path, output_path: Path) -> Path:
    command = [
        "ffmpeg",
        "-y",
        "-i",
        str(input_path),
        "-af",
        f"loudnorm=I={_LOUDNORM_TARGET_I}:TP={_LOUDNORM_TARGET_TP}:LRA=11",
        "-ac",
        _OUTPUT_CHANNELS,
        "-c:a",
        "libmp3lame",
        "-q:a",
        "2",
        str(output_path),
    ]
    if not run_command(command):
        raise RuntimeError("ffmpeg loudnorm failed")
    return output_path


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
