#!/bin/sh
# Finite PUBLIC synthetic proofs. No native engine, registration or final release.
set -eu
barrier_root=$(CDPATH= cd -- "$(dirname -- "$0")/../../.." && pwd)
barrier_compose="$barrier_root/infra/maintenance/startup-barrier/compose.yml"
barrier_scope=${1-all}
case "$barrier_scope" in all|matrix|http) ;; *) exit 1 ;; esac
dc() { docker compose --env-file /dev/null --project-name sifto-startup-barrier-fixture -f "$barrier_compose" "$@"; }
empty() {
 for barrier_kind in container volume network; do
  case "$barrier_kind" in
   container) barrier_found=$(docker ps -aq --filter label=com.docker.compose.project=sifto-startup-barrier-fixture) || return 1 ;;
   volume) barrier_found=$(docker volume ls -q --filter label=com.docker.compose.project=sifto-startup-barrier-fixture) || return 1 ;;
   network) barrier_found=$(docker network ls -q --filter label=com.docker.compose.project=sifto-startup-barrier-fixture) || return 1 ;;
  esac
  test -z "$barrier_found" || return 1
 done
}
empty || { printf 'barrier fixture requires empty owned namespace\n' >&2; exit 1; }
barrier_staging=$(mktemp -d "${TMPDIR:-/tmp}/sifto-startup-barrier.XXXXXXXX")
SIFTO_STARTUP_PUBLIC_INIT="$barrier_staging/init"; export SIFTO_STARTUP_PUBLIC_INIT
cleanup() {
 barrier_status=$?; trap - EXIT
 if ! dc down --volumes --timeout 10; then barrier_status=1; fi
 if ! empty; then barrier_status=1; fi
 rm -f -- "$SIFTO_STARTUP_PUBLIC_INIT" "$barrier_staging/migration.log"
 rmdir -- "$barrier_staging"; exit "$barrier_status"
}
trap cleanup EXIT
trap 'exit 130' INT
trap 'exit 143' TERM
cp "$barrier_root/infra/maintenance/maintenance/coolify_startup_init" "$SIFTO_STARTUP_PUBLIC_INIT"
chmod 0755 "$SIFTO_STARTUP_PUBLIC_INIT"
cmp "$barrier_root/infra/maintenance/maintenance/coolify_startup_init" "$SIFTO_STARTUP_PUBLIC_INIT"
dc up -d postgres redis
barrier_attempt=0
until dc exec -T postgres pg_isready -q -h 127.0.0.1 -U coolify -d coolify; do
 barrier_attempt=$((barrier_attempt + 1)); test "$barrier_attempt" -lt 30; sleep 1
done
if test "$barrier_scope" != http; then
 for barrier_image in probe-old probe-target; do
  dc run --rm -T --no-deps --entrypoint php "$barrier_image" -l /probe/startup.php
  dc run --rm -T --no-deps "$barrier_image" raw-profile
  dc run --rm -T --no-deps "$barrier_image" alternate-prefix
  dc run --rm -T --no-deps "$barrier_image" oversized-cas
  dc run --rm -T --no-deps "$barrier_image" expiry-bounds
  dc run --rm -T --no-deps "$barrier_image" matrix
 done
fi
ready() {
 barrier_attempt=0
 until dc exec -T "$1" php /startup/barrier-fixture.php ready; do
  barrier_attempt=$((barrier_attempt + 1)); test "$barrier_attempt" -lt 20; sleep 1
 done
 # Readiness transport only is retried; invariant checks below run once.
}
if test "$barrier_scope" != matrix; then
 dc run --rm -T --no-deps --entrypoint php probe-old /startup/fixture.php setup
 dc run --rm -T --no-deps probe-old prepare-cookie
 dc up -d --no-deps old
 ready old
 dc exec -T old php /startup/barrier-fixture.php cookie-before
 dc exec -T old php /startup/barrier-fixture.php cookie-enter
 dc restart --timeout 10 old
 ready old
 dc exec -T old php /startup/barrier-fixture.php cookie-held
 dc stop --timeout 10 old
 # PRETARGET exact restoration, while no target application has been started.
 dc run --rm -T --no-deps probe-old cookie-restore
 # New synthetic fixture run; cleanup is fixture-owned, never engine authority.
 dc run --rm -T --no-deps probe-old prepare-cookie
 dc up -d --no-deps old
 ready old
 dc exec -T old php /startup/barrier-fixture.php cookie-before
 dc exec -T old php /startup/barrier-fixture.php cookie-enter
 dc stop --timeout 10 old
 if ! dc run --rm -T --no-deps --entrypoint php probe-target artisan migrate --force >"$barrier_staging/migration.log" 2>&1; then
  printf 'synthetic target migration failed; output withheld\n' >&2; exit 1
 fi
 dc up -d --no-deps target
 ready target
 dc exec -T target php /startup/barrier-fixture.php cookie-held
 dc restart --timeout 10 target
 ready target
 dc exec -T target php /startup/barrier-fixture.php cookie-held
 dc stop --timeout 10 target
 dc run --rm -T --no-deps probe-target target-restore-denied
 # Independent target-image cookie proof; no post-target success release.
 dc run --rm -T --no-deps probe-target prepare-cookie
 dc up -d --no-deps target
 ready target
 dc exec -T target php /startup/barrier-fixture.php cookie-before
 dc exec -T target php /startup/barrier-fixture.php cookie-enter
 dc stop --timeout 10 target
 # Separate deliberate identity-reset experiment after preservation proofs.
 dc restart --timeout 10 redis
 dc run --rm -T --no-deps probe-target identity-denied
fi
printf '{"ok":true,"category":"public_barrier_proof_complete","startup_registered":false,"production_eligible":false,"native_final_release_available":false}\n'
