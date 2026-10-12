#!/bin/sh
set -eu

service=${1:?Usage: check.sh api|worker|web}
case "$service" in
    api|worker) platform=linux/arm64; expected_user=10001:10001; context=. ;;
    web) platform=linux/amd64; expected_user=1000:1000; context=web ;;
    *) echo 'Unknown runtime service' >&2; exit 2 ;;
esac
image="sifto-runtime-ci-$service"

# Web uses the configured production publishable key, never backend credentials.
if [ "$service" = web ]; then
    case "${NEXT_PUBLIC_CLERK_PUBLISHABLE_KEY:-}" in
        pk_live_?*) ;;
        *) echo 'Web runtime CI requires a production NEXT_PUBLIC_CLERK_PUBLISHABLE_KEY' >&2; exit 1 ;;
    esac
    docker build --platform "$platform" --target runner -t "$image" \
        --build-arg "NEXT_PUBLIC_CLERK_PUBLISHABLE_KEY=$NEXT_PUBLIC_CLERK_PUBLISHABLE_KEY" \
        -f web/Dockerfile "$context"
else
    docker build --platform "$platform" --target runner -t "$image" \
        -f "$service/Dockerfile" "$context"
fi
test "$(docker image inspect --format '{{.Config.User}}' "$image")" = "$expected_user"
test "$(docker image inspect --format '{{.Os}}/{{.Architecture}}' "$image")" = "$platform"

# No env file, forwarded runtime environment, mounts, ports, or external network.
# Do not override --user: the image's configured runtime user is under test.
docker run --rm -i --network none --cap-drop ALL --security-opt no-new-privileges \
    --entrypoint sh "$image" -eu -s -- "$expected_user" <<'SH'
test "$(id -u):$(id -g)" = "$1"
test -d "$HOME"
test -w "$HOME"
test ! -w /app
test ! -w /usr/local/bin
test ! -w /etc/passwd
temp_dir=$(mktemp -d)
trap 'rm -rf "$temp_dir"' EXIT
printf runtime-check > "$temp_dir/probe"
test "$(cat "$temp_dir/probe")" = runtime-check
printf 'Runtime identity: uid=%s gid=%s\n' "$(id -u)" "$(id -g)"
SH

case "$service" in
    api)
        docker run --rm -i --network none --cap-drop ALL --security-opt no-new-privileges \
            --entrypoint sh "$image" -eu -s <<'SH'
test -x /app/server
test -r /app/shared/llm_catalog.json
test ! -w /app/server
test ! -w /app/shared/llm_catalog.json
output=$(mktemp)
trap 'rm -f "$output"' EXIT
if /app/server > "$output" 2>&1; then
    echo 'API unexpectedly started without its database configuration' >&2
    exit 1
fi
grep -q 'DATABASE_URL is not set' "$output"
echo 'API binary runs as the image user; dependency-free startup fails closed'
SH
        ;;
    worker)
        docker run --rm -i --network none --cap-drop ALL --security-opt no-new-privileges \
            --entrypoint python "$image" -B - < infra/runtime-ci/worker.py
        ;;
    web)
        docker run --rm -i --network none --cap-drop ALL --security-opt no-new-privileges \
            --entrypoint node "$image" - < infra/runtime-ci/web.cjs
        ;;
esac
