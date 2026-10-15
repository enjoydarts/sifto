#!/bin/sh
# Fixed PUBLIC/SYNTHETIC real-image Task4A functional check. No native authority.
set -eu
fixture_root=$(CDPATH= cd -- "$(dirname -- "$0")/../../.." && pwd)
fixture_compose="$fixture_root/infra/maintenance/startup/compose.yml"
dc() {
  docker compose --env-file /dev/null --project-name sifto-controlled-startup-fixture -f "$fixture_compose" "$@"
}
empty() {
  fixture_present=$(docker ps -aq --filter label=com.docker.compose.project=sifto-controlled-startup-fixture) || return 1
  test -z "$fixture_present" || return 1
  fixture_present=$(docker volume ls -q --filter label=com.docker.compose.project=sifto-controlled-startup-fixture) || return 1
  test -z "$fixture_present" || return 1
  fixture_present=$(docker network ls -q --filter label=com.docker.compose.project=sifto-controlled-startup-fixture) || return 1
  test -z "$fixture_present" || return 1
}
empty || { printf 'startup fixture requires an empty owned namespace\n' >&2; exit 1; }
fixture_staging=$(mktemp -d "${TMPDIR:-/tmp}/sifto-controlled-startup.XXXXXXXX")
fixture_migration_log="$fixture_staging/migration.log"
SIFTO_STARTUP_PUBLIC_INIT="$fixture_staging/init"
export SIFTO_STARTUP_PUBLIC_INIT
cleanup() {
  fixture_status=$?
  trap - EXIT
  if ! dc down --volumes --timeout 10; then fixture_status=1; fi
  if ! empty; then fixture_status=1; fi
  rm -f -- "$fixture_migration_log" "$SIFTO_STARTUP_PUBLIC_INIT"
  rmdir -- "$fixture_staging"
  exit "$fixture_status"
}
trap cleanup EXIT
trap 'exit 130' INT
trap 'exit 143' TERM

# Match the installed-source/staged-executable distinction. Only this owned
# PUBLIC byte-for-byte copy is executable; repository/installer input is0644.
cp "$fixture_root/infra/maintenance/maintenance/coolify_startup_init" "$SIFTO_STARTUP_PUBLIC_INIT"
chmod 0755 "$SIFTO_STARTUP_PUBLIC_INIT"
cmp "$fixture_root/infra/maintenance/maintenance/coolify_startup_init" "$SIFTO_STARTUP_PUBLIC_INIT"

# Engine IDs are the configs of these exact immutable platform manifests.
image() {
  fixture_observed=$(docker image inspect --format '{{.Id}} {{.Os}}/{{.Architecture}}' "$1")
  test "$fixture_observed" = "$2 linux/arm64"
  fixture_digests=$(docker image inspect --format '{{json .RepoDigests}}' "$1")
  case "$fixture_digests" in *"@${1##*@}"*) ;; *) exit 1 ;; esac
}
image docker.io/coollabsio/coolify@sha256:a323ce1dfb54101b0c30d8bd5c92b0e4166c3b403a164316f8876be653204960 sha256:6145ffe0650d8dcdef48ab08442dc01b066d6450e1f6df9e3cd8efacadbb0064
image docker.io/coollabsio/coolify@sha256:f4ef1eb69daa265057ab700646485bd78bb43bcf96c691d65094947acb7f50a5 sha256:292e1d3e300d6eab9598e14a017bfa2e35bd89f78597916c179c24e099e40156
image docker.io/library/postgres@sha256:91a22c143801df42c2e4a8f9926eab5e48f39f40beb5220b7c29d4478263c341 sha256:00f747f11f25a40d61020089b8cafd867de9f8716fd054ea661a2f6a5b7c4f76
image docker.io/library/redis@sha256:1f09a89a207d794a8c61d9edfc26e7c58427de10ccef7c5d18d638df79a63b85 sha256:ec4f171e9ff071bb2a1c936057d3be00206069b028b6c0d61df2477e1dd02c42
dc run --rm -T --no-deps source-old source
dc run --rm -T --no-deps source-target source
for fixture_service in source-old source-target; do
  if fixture_negative=$(dc run --rm -T --no-deps "$fixture_service" maintenance-guard); then
    printf 'stock webhook exception negative unexpectedly passed\n' >&2; exit 1
  fi
  test "$fixture_negative" = '{"ok":false,"category":"startup_fixture_failed","stage":"stock_webhook_exception"}'
  printf '%s\n' "$fixture_negative"
done

dc up -d postgres redis
fixture_attempt=0
until dc exec -T postgres pg_isready -q -h 127.0.0.1 -U coolify -d coolify; do
  fixture_attempt=$((fixture_attempt + 1))
  test "$fixture_attempt" -lt 30
  sleep 1
done
dc run --rm -T --no-deps probe setup
dc run --rm -T --no-deps probe hold
dc run --rm -T --no-deps probe maintenance-guard

observe() {
  fixture_attempt=0
  until dc exec -T "$1" php /startup/fixture.php ready; do
    fixture_attempt=$((fixture_attempt + 1))
    test "$fixture_attempt" -lt 20
    sleep 1
  done
  # Retry transport readiness only. Security/data invariants never retry to green.
  dc exec -T "$1" php /startup/fixture.php observe
}
dc up -d --no-deps old
observe old
dc restart --timeout 10 old
observe old
dc stop --timeout 10 old
# Direct migration exit is necessary. The exact repository comparison above is
# also required, so a successful lock-skip or pending migration cannot pass.
if ! dc run --rm -T --no-deps --entrypoint php target artisan migrate --force >"$fixture_migration_log" 2>&1; then
  printf 'direct target migration failed (synthetic output withheld)\n' >&2; exit 1
fi
dc up -d --no-deps target
observe target
dc restart --timeout 10 target
observe target
printf '{"ok":true,"scope":"task4a-functional-only","startup_registered":false,"production_eligible":false}\n'
