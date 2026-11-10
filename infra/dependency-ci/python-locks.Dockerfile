FROM ghcr.io/astral-sh/uv:0.13.0-python3.12-trixie-slim@sha256:17af15d103f32869a63825517f9e8867ce98598937e3e5f5ce1e24313c552406 AS uv
FROM python:3.15.2-slim-trixie@sha256:65a65fc7b30b06cb6df4930fd4650697cb23f4c2698ab81fc87e4b031bc36928
COPY --from=uv /usr/local/bin/uv /usr/local/bin/uvx /usr/local/bin/
