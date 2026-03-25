import json
import urllib.request


def post_callback(callback_url: str, callback_token: str, payload: dict) -> None:
    request = urllib.request.Request(
        callback_url,
        data=json.dumps(payload).encode("utf-8"),
        method="POST",
        headers={
            "Content-Type": "application/json",
            "Authorization": f"Bearer {callback_token}",
        },
    )
    with urllib.request.urlopen(request, timeout=30) as response:
        if response.status < 200 or response.status >= 300:
            raise RuntimeError(f"callback failed with status {response.status}")
