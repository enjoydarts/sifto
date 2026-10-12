#!/bin/sh
set -eu
component=${1:?usage: build.sh COMPONENT}
case "$component" in
  api) platform=linux/arm64; context=.; dockerfile=api/Dockerfile ;;
  worker) platform=linux/arm64; context=.; dockerfile=worker/Dockerfile ;;
  audio-concat) platform=linux/amd64; context=.; dockerfile=infra/audio-concat/Dockerfile ;;
  maintenance) platform=linux/amd64; context=infra/maintenance; dockerfile=infra/maintenance/Dockerfile ;;
  web) platform=linux/amd64; context=web; dockerfile=web/Dockerfile ;;
  *) echo 'Unknown image component' >&2; exit 2 ;;
esac
# Archives and BuildKit layers can exhaust a shared local Docker host. Fail
# before starting the build; CI runners build one matrix image per machine.
available_kib=$(df -Pk . | awk 'NR == 2 {print $4}')
if [ "$available_kib" -lt 5242880 ]; then
  echo 'Image security build requires at least 5 GiB free disk space; no build started.' >&2
  exit 2
fi
IMAGE_SECURITY_DIR="$(pwd)/.image-security/$component"
IMAGE_SECURITY_CACHE_DIR="$(pwd)/.image-security/cache"
IMAGE_SECURITY_UID=$(id -u)
IMAGE_SECURITY_GID=$(id -g)
IMAGE_COMPONENT=$component
IMAGE_SOURCE_SHA=$(git rev-parse HEAD)
IMAGE_SOURCE_DIRTY=false
if [ -n "$(git status --porcelain --untracked-files=normal)" ]; then IMAGE_SOURCE_DIRTY=true; fi
export IMAGE_SECURITY_DIR IMAGE_SECURITY_CACHE_DIR IMAGE_SECURITY_UID IMAGE_SECURITY_GID
export IMAGE_COMPONENT IMAGE_SOURCE_SHA IMAGE_SOURCE_DIRTY
mkdir -p "$IMAGE_SECURITY_DIR" "$IMAGE_SECURITY_CACHE_DIR"
# Prevent reusing reports from an earlier image when a build/scan fails.
rm -f "$IMAGE_SECURITY_DIR/image.tar" "$IMAGE_SECURITY_DIR/image-id.txt" \
  "$IMAGE_SECURITY_DIR/build-metadata.json" "$IMAGE_SECURITY_DIR/metadata.json" \
  "$IMAGE_SECURITY_DIR/sbom.cdx.json" "$IMAGE_SECURITY_DIR/vulnerabilities.json" \
  "$IMAGE_SECURITY_DIR/summary.md" "$IMAGE_SECURITY_DIR/scan-status.json"
printf '%s\n' "$platform" > "$IMAGE_SECURITY_DIR/platform.txt"
printf '%s\n' "$dockerfile" > "$IMAGE_SECURITY_DIR/dockerfile.txt"
printf '%s\n' "$IMAGE_SOURCE_SHA" > "$IMAGE_SECURITY_DIR/source-sha.txt"
printf '%s\n' "$IMAGE_SOURCE_DIRTY" > "$IMAGE_SECURITY_DIR/source-dirty.txt"
# No --target: scan the final production stage, never the dev stage.
# No registry login, push, production environment or secret arguments.
set -- --build-arg "APP_COMMIT_SHA=$IMAGE_SOURCE_SHA"
if [ "$component" = maintenance ]; then
  # Match run-controller.sh, including its caller passwd/group entries.
  set -- "$@" --build-arg "MAINTENANCE_CONTROLLER_UID=$(id -u)" \
    --build-arg "MAINTENANCE_CONTROLLER_GID=$(id -g)"
fi
docker buildx build --platform "$platform" --file "$dockerfile" \
  "$@" \
  --label "org.opencontainers.image.revision=$IMAGE_SOURCE_SHA" \
  --tag "sifto-security/$component:$IMAGE_SOURCE_SHA" \
  --iidfile "$IMAGE_SECURITY_DIR/image-id.txt" \
  --metadata-file "$IMAGE_SECURITY_DIR/build-metadata.json" \
  --output "type=docker,dest=$IMAGE_SECURITY_DIR/image.tar" "$context"
docker compose --env-file /dev/null -f infra/image-security/compose.yml \
  run --rm -T --no-deps verifier python -B /tool/report.py prepare
