import os
from pathlib import Path

import boto3


class R2Client:
    def __init__(self) -> None:
        self.bucket = os.environ["AUDIO_BRIEFING_R2_BUCKET"].strip()
        endpoint = os.environ["AUDIO_BRIEFING_R2_ENDPOINT"].strip()
        access_key = os.environ["AUDIO_BRIEFING_R2_ACCESS_KEY_ID"].strip()
        secret_key = os.environ["AUDIO_BRIEFING_R2_SECRET_ACCESS_KEY"].strip()
        region = os.getenv("AUDIO_BRIEFING_R2_REGION", "auto").strip() or "auto"
        self._client = boto3.client(
            "s3",
            endpoint_url=endpoint,
            aws_access_key_id=access_key,
            aws_secret_access_key=secret_key,
            region_name=region,
        )

    def download_file(self, object_key: str, destination: Path) -> None:
        destination.parent.mkdir(parents=True, exist_ok=True)
        self._client.download_file(self.bucket, object_key, str(destination))

    def upload_file(self, source: Path, object_key: str, content_type: str = "audio/mpeg") -> None:
        extra_args = {"ContentType": content_type}
        self._client.upload_file(str(source), self.bucket, object_key, ExtraArgs=extra_args)
