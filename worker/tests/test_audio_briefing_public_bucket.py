from unittest.mock import Mock

import pytest
from fastapi import FastAPI
from fastapi.testclient import TestClient

from app.routers import audio_briefing_tts
from app.services.audio_briefing_tts import AudioBriefingTTSService


@pytest.fixture
def public_bucket_service(monkeypatch):
    monkeypatch.setenv("AUDIO_BRIEFING_R2_STANDARD_BUCKET", "briefings-standard")
    monkeypatch.setenv("AUDIO_BRIEFING_R2_IA_BUCKET", "briefings-ia")
    monkeypatch.setenv("AUDIO_BRIEFING_PUBLIC_BUCKET", " briefings-public ")
    service = AudioBriefingTTSService()
    client = Mock()
    monkeypatch.setattr(service, "r2_client", lambda: client)
    return service, client


def test_stat_object_route_reads_configured_podcast_public_bucket(monkeypatch, public_bucket_service):
    service, r2 = public_bucket_service
    r2.head_object.return_value = {"ContentLength": 12345}
    monkeypatch.setattr(audio_briefing_tts, "_service", service)
    app = FastAPI()
    app.include_router(audio_briefing_tts.router)

    with TestClient(app) as client:
        response = client.post(
            "/audio-briefing/stat-object",
            json={"bucket": "briefings-public", "object_key": "podcast/episode.mp3"},
        )

    assert response.status_code == 200
    assert response.json() == {"size_bytes": 12345}
    r2.head_object.assert_called_once_with(Bucket="briefings-public", Key="podcast/episode.mp3")


def test_podcast_publication_copies_into_configured_public_bucket(public_bucket_service):
    service, r2 = public_bucket_service

    assert service.copy_objects("briefings-standard", "briefings-public", ["podcast/episode.mp3"]) == 1

    r2.copy_object.assert_called_once_with(
        Bucket="briefings-public",
        Key="podcast/episode.mp3",
        CopySource={"Bucket": "briefings-standard", "Key": "podcast/episode.mp3"},
    )


def test_podcast_cleanup_deletes_from_configured_public_bucket(public_bucket_service):
    service, r2 = public_bucket_service
    r2.delete_objects.return_value = {"Deleted": [{"Key": "podcast/episode.mp3"}]}

    assert service.delete_objects(["podcast/episode.mp3"], bucket_override="briefings-public") == 1

    r2.delete_objects.assert_called_once_with(
        Bucket="briefings-public", Delete={"Objects": [{"Key": "podcast/episode.mp3"}], "Quiet": True}
    )


def test_podcast_artwork_upload_uses_configured_public_bucket(public_bucket_service):
    service, r2 = public_bucket_service

    service.upload_bytes("podcast/artwork.png", b"image", "image/png", bucket_override="briefings-public")

    r2.put_object.assert_called_once_with(
        Bucket="briefings-public", Key="podcast/artwork.png", Body=b"image", ContentType="image/png"
    )


@pytest.mark.parametrize("bucket", ["unrelated-private-bucket", "briefings-public-extra"])
def test_stat_object_rejects_unconfigured_buckets_without_r2_access(public_bucket_service, bucket):
    service, r2 = public_bucket_service

    with pytest.raises(ValueError, match="audio briefing R2 bucket is not allowed"):
        service.stat_object("podcast/episode.mp3", bucket_override=bucket)

    r2.head_object.assert_not_called()


def test_public_bucket_requires_explicit_configuration(monkeypatch):
    monkeypatch.setenv("AUDIO_BRIEFING_R2_STANDARD_BUCKET", "briefings-standard")
    monkeypatch.setenv("AUDIO_BRIEFING_R2_IA_BUCKET", "briefings-ia")
    monkeypatch.delenv("AUDIO_BRIEFING_PUBLIC_BUCKET", raising=False)
    service = AudioBriefingTTSService()

    assert service.resolve_bucket() == "briefings-standard"
    assert service.resolve_bucket("briefings-ia") == "briefings-ia"
    with pytest.raises(ValueError, match="audio briefing R2 bucket is not allowed"):
        service.resolve_bucket("briefings-public")
