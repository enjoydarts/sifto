#!/bin/sh
set -eu
drain_root=$(CDPATH= cd -- "$(dirname -- "$0")/../../.." && pwd)
drain_public=${SIFTO_DRAIN_PUBLIC:?public artifact directory required}
dc() { docker compose --env-file /dev/null -p sifto-horizon-graceful-drain-worker -f "$drain_root/infra/maintenance/startup-drain/compose.yml" "$@"; }
empty() {
 for drain_kind in container volume network; do
  case "$drain_kind" in
   container) drain_found=$(docker ps -aq --filter label=com.docker.compose.project=sifto-horizon-graceful-drain-worker) || return 1 ;;
   volume) drain_found=$(docker volume ls -q --filter label=com.docker.compose.project=sifto-horizon-graceful-drain-worker) || return 1 ;;
   network) drain_found=$(docker network ls -q --filter label=com.docker.compose.project=sifto-horizon-graceful-drain-worker) || return 1 ;;
  esac
  test -z "$drain_found" || return 1
 done
}
empty
drain_staging=$(mktemp -d "$drain_public/fixture.XXXXXXXX")
SIFTO_INSPECT_ENV="$drain_staging/public.env"
SIFTO_STARTUP_PUBLIC_INIT="$drain_staging/init"
export SIFTO_INSPECT_ENV SIFTO_STARTUP_PUBLIC_INIT
cleanup() {
 drain_status=$?; trap - EXIT
 if test -n "${drain_running:-}"; then
  dc exec -T "$drain_running" php /startup/drain-fixture.php settle || drain_status=1
 fi
 dc down --volumes --timeout 20 || drain_status=1
 empty || drain_status=1
 exit "$drain_status"
}
trap cleanup EXIT
trap 'exit 130' INT
trap 'exit 143' TERM
cp "$drain_root/infra/maintenance/maintenance/coolify_startup_init" "$SIFTO_STARTUP_PUBLIC_INIT"
chmod 0755 "$SIFTO_STARTUP_PUBLIC_INIT"
image() {
 drain_identity=$(docker image inspect --format '{{.Id}} {{.Os}}/{{.Architecture}}' "$1")
 test "$drain_identity" = "$2 linux/arm64"
 drain_digests=$(docker image inspect --format '{{json .RepoDigests}}' "$1")
 case "$drain_digests" in *"@${1##*@}"*) ;; *) exit 1 ;; esac
}
image docker.io/coollabsio/coolify@sha256:a323ce1dfb54101b0c30d8bd5c92b0e4166c3b403a164316f8876be653204960 sha256:6145ffe0650d8dcdef48ab08442dc01b066d6450e1f6df9e3cd8efacadbb0064
image docker.io/coollabsio/coolify@sha256:f4ef1eb69daa265057ab700646485bd78bb43bcf96c691d65094947acb7f50a5 sha256:292e1d3e300d6eab9598e14a017bfa2e35bd89f78597916c179c24e099e40156
image docker.io/library/postgres@sha256:91a22c143801df42c2e4a8f9926eab5e48f39f40beb5220b7c29d4478263c341 sha256:00f747f11f25a40d61020089b8cafd867de9f8716fd054ea661a2f6a5b7c4f76
image docker.io/library/redis@sha256:1f09a89a207d794a8c61d9edfc26e7c58427de10ccef7c5d18d638df79a63b85 sha256:ec4f171e9ff071bb2a1c936057d3be00206069b028b6c0d61df2477e1dd02c42
sed 's/^HORIZON_ENABLED=false$/HORIZON_ENABLED=true/' "$drain_root/infra/maintenance/startup/public.env" > "$SIFTO_INSPECT_ENV"
printf '\nHORIZON_MIN_PROCESSES=1\nHORIZON_MAX_PROCESSES=1\nHORIZON_QUEUES=public-drain\n' >> "$SIFTO_INSPECT_ENV"
drain_cases='natural timeout faults'
if test "${SIFTO_DRAIN_ONLY:-all}" = faults; then drain_cases=faults; fi
if test "${SIFTO_DRAIN_ONLY:-all}" = post; then drain_cases=natural; fi
if test "${SIFTO_DRAIN_ONLY:-all}" = expiry; then drain_cases='natural timeout'; fi
drain_services='old target'
if test "${SIFTO_DRAIN_IMAGE:-both}" = target; then drain_services=target; fi
for drain_service in $drain_services; do
 for drain_case in $drain_cases; do
  dc up -d postgres redis
  dc run --rm -T --no-deps --entrypoint php "$drain_service" /startup/inspect-fixture.php hold
  dc up -d --no-deps "$drain_service"
  drain_running=$drain_service
  dc exec -T "$drain_service" sh /startup/runtime-wait-ready.sh
  dc exec -T "$drain_service" php /startup/inspect-fixture.php install
  case "$drain_case" in
   natural)
    dc exec -T "$drain_service" php /startup/drain-fixture.php start
    dc exec -T "$drain_service" php -d auto_prepend_file=/startup/inspect-diagnose.php /probe/startup.php horizon-drain-enter > "$drain_staging/$drain_service-receipt.json"
    cat "$drain_staging/$drain_service-receipt.json"
    dc exec -T "$drain_service" php -d auto_prepend_file=/startup/inspect-diagnose.php /probe/startup.php horizon-drain-observe
    if drain_rejected=$(dc exec -T "$drain_service" php /probe/startup.php horizon-drain-enter); then exit 1; fi
    test "$drain_rejected" = '{"version":1,"operation":"horizon-drain-enter","category":"horizon_inspection_held"}'
    printf '%s\n' "$drain_rejected"
    dc exec -T "$drain_service" php /startup/drain-fixture.php verify
    dc exec -T "$drain_service" php /startup/drain-fixture.php post-foreign ;;
   timeout)
    dc exec -T "$drain_service" php /startup/drain-fixture.php start-long
    dc exec -T "$drain_service" php /startup/drain-fixture.php bounded-enter
    dc exec -T "$drain_service" php /startup/drain-fixture.php verify-long ;;
   faults)
    dc exec -T "$drain_service" php /startup/drain-faults.php ;;
  esac
  dc down --volumes --timeout 20
  drain_running=
  empty
 done
done
