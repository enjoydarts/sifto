#!/bin/sh
set -eu
ssh_root=$(CDPATH= cd -- "$(dirname -- "$0")/../../.." && pwd)
ssh_compose="$ssh_root/infra/maintenance/startup-ssh/compose.yml"
dc() { docker compose --env-file /dev/null --project-name sifto-startup-runtime-fixture -f "$ssh_compose" "$@"; }
empty() {
 for ssh_kind in container volume network; do
  case "$ssh_kind" in
   container) ssh_found=$(docker ps -aq --filter label=com.docker.compose.project=sifto-startup-runtime-fixture) || return 1 ;;
   volume) ssh_found=$(docker volume ls -q --filter label=com.docker.compose.project=sifto-startup-runtime-fixture) || return 1 ;;
   network) ssh_found=$(docker network ls -q --filter label=com.docker.compose.project=sifto-startup-runtime-fixture) || return 1 ;;
  esac
  test -z "$ssh_found" || return 1
 done
}
empty || { printf 'SSH fixture requires empty owned namespace\n' >&2; exit 1; }
ssh_staging=$(mktemp -d "${TMPDIR:-/tmp}/sifto-startup-ssh.XXXXXXXX")
SIFTO_STARTUP_PUBLIC_INIT="$ssh_staging/init"; export SIFTO_STARTUP_PUBLIC_INIT
cleanup() {
 ssh_status=$?; trap - EXIT
 if ! dc down --volumes --timeout 10; then ssh_status=1; fi
 if ! empty; then ssh_status=1; fi
 rm -f -- "$SIFTO_STARTUP_PUBLIC_INIT"; rmdir -- "$ssh_staging"; exit "$ssh_status"
}
trap cleanup EXIT
trap 'exit 130' INT
trap 'exit 143' TERM
cp "$ssh_root/infra/maintenance/maintenance/coolify_startup_init" "$SIFTO_STARTUP_PUBLIC_INIT"
chmod 0755 "$SIFTO_STARTUP_PUBLIC_INIT"
dc up -d postgres redis
ssh_attempt=0
until dc exec -T postgres pg_isready -q -h 127.0.0.1 -U coolify -d coolify; do
 ssh_attempt=$((ssh_attempt + 1)); test "$ssh_attempt" -lt 30; sleep 1
done
dc run --rm -T --no-deps old setup
for ssh_image in old target; do
 dc run --rm -T --no-deps --entrypoint php "$ssh_image" -l /probe/startup.php
 dc run --rm -T --no-deps "$ssh_image" matrix
done
