import json
import os
import threading
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer

from app.concat_job import run_job


class LocalConcatHandler(BaseHTTPRequestHandler):
    server_version = "audio-concat-local/1.0"

    def do_GET(self) -> None:
        if self.path.rstrip("/") == "/health":
            self._send_json(200, {"status": "ok"})
            return
        self._send_json(404, {"error": "not_found"})

    def do_POST(self) -> None:
        if self.path.rstrip("/") != "/run":
            self._send_json(404, {"error": "not_found"})
            return
        try:
            length = int(self.headers.get("Content-Length", "0"))
            raw = self.rfile.read(length)
            payload = json.loads(raw or b"{}")
            job_id = required_field(payload, "job_id")
            request_id = required_field(payload, "request_id")
            callback_url = required_field(payload, "callback_url")
            callback_token = required_field(payload, "callback_token")
            output_object_key = required_field(payload, "output_object_key")
            audio_object_keys = payload.get("audio_object_keys")
            if not isinstance(audio_object_keys, list) or not audio_object_keys:
                raise ValueError("audio_object_keys must be a non-empty array")
            execution_name = f"local-{request_id}"
            thread = threading.Thread(
                target=run_job,
                kwargs={
                    "job_id": job_id,
                    "request_id": request_id,
                    "callback_url": callback_url,
                    "callback_token": callback_token,
                    "output_object_key": output_object_key,
                    "audio_object_keys": [str(value).strip() for value in audio_object_keys],
                    "provider_job_id": execution_name,
                },
                daemon=True,
            )
            thread.start()
            self._send_json(202, {"execution_name": execution_name})
        except Exception as exc:
            self._send_json(400, {"error": str(exc)})

    def log_message(self, format: str, *args) -> None:
        return

    def _send_json(self, status: int, payload: dict) -> None:
        raw = json.dumps(payload).encode("utf-8")
        self.send_response(status)
        self.send_header("Content-Type", "application/json")
        self.send_header("Content-Length", str(len(raw)))
        self.end_headers()
        self.wfile.write(raw)


def required_field(payload: dict, key: str) -> str:
    value = str(payload.get(key) or "").strip()
    if not value:
        raise ValueError(f"missing field: {key}")
    return value


def main() -> int:
    port = int(os.getenv("PORT", "8080"))
    server = ThreadingHTTPServer(("0.0.0.0", port), LocalConcatHandler)
    try:
        server.serve_forever()
    except KeyboardInterrupt:
        pass
    finally:
        server.server_close()
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
