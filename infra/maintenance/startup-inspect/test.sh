#!/bin/sh
# PUBLIC only. One APP at a time; no signal/control/drain operations.
set -eu
inspect_root=$(CDPATH= cd -- "$(dirname -- "$0")/../../.." && pwd)
inspect_public=${SIFTO_INSPECT_PUBLIC:?public artifact directory required}
inspect_compose="$inspect_root/infra/maintenance/startup-inspect/compose.yml"
dc() { docker compose --env-file /dev/null -p sifto-horizon-inspection-worker -f "$inspect_compose" "$@"; }
empty() {
 for inspect_kind in container volume network; do
  case "$inspect_kind" in
   container) inspect_found=$(docker ps -aq --filter label=com.docker.compose.project=sifto-horizon-inspection-worker) || return 1 ;;
   volume) inspect_found=$(docker volume ls -q --filter label=com.docker.compose.project=sifto-horizon-inspection-worker) || return 1 ;;
   network) inspect_found=$(docker network ls -q --filter label=com.docker.compose.project=sifto-horizon-inspection-worker) || return 1 ;;
  esac
  test -z "$inspect_found" || return 1
 done
}
empty || exit 1
inspect_staging=$(mktemp -d "$inspect_public/fixture.XXXXXXXX")
SIFTO_INSPECT_ENV="$inspect_staging/public.env"
SIFTO_STARTUP_PUBLIC_INIT="$inspect_staging/init"
export SIFTO_INSPECT_ENV SIFTO_STARTUP_PUBLIC_INIT
cleanup() {
 inspect_status=$?; trap - EXIT
 dc down --volumes --timeout 10 || inspect_status=1
 empty || inspect_status=1
 exit "$inspect_status"
}
trap cleanup EXIT
trap 'exit 130' INT
trap 'exit 143' TERM
cp "$inspect_root/infra/maintenance/maintenance/coolify_startup_init" "$SIFTO_STARTUP_PUBLIC_INIT"
chmod 0755 "$SIFTO_STARTUP_PUBLIC_INIT"
image() {
 inspect_identity=$(docker image inspect --format '{{.Id}} {{.Os}}/{{.Architecture}}' "$1")
 test "$inspect_identity" = "$2 linux/arm64"
 inspect_digests=$(docker image inspect --format '{{json .RepoDigests}}' "$1")
 case "$inspect_digests" in *"@${1##*@}"*) ;; *) exit 1 ;; esac
}
image docker.io/coollabsio/coolify@sha256:a323ce1dfb54101b0c30d8bd5c92b0e4166c3b403a164316f8876be653204960 sha256:6145ffe0650d8dcdef48ab08442dc01b066d6450e1f6df9e3cd8efacadbb0064
image docker.io/coollabsio/coolify@sha256:f4ef1eb69daa265057ab700646485bd78bb43bcf96c691d65094947acb7f50a5 sha256:292e1d3e300d6eab9598e14a017bfa2e35bd89f78597916c179c24e099e40156
image docker.io/library/postgres@sha256:91a22c143801df42c2e4a8f9926eab5e48f39f40beb5220b7c29d4478263c341 sha256:00f747f11f25a40d61020089b8cafd867de9f8716fd054ea661a2f6a5b7c4f76
image docker.io/library/redis@sha256:1f09a89a207d794a8c61d9edfc26e7c58427de10ccef7c5d18d638df79a63b85 sha256:ec4f171e9ff071bb2a1c936057d3be00206069b028b6c0d61df2477e1dd02c42
sed 's/^HORIZON_ENABLED=false$/HORIZON_ENABLED=true/' "$inspect_root/infra/maintenance/startup/public.env" > "$SIFTO_INSPECT_ENV"
printf '\nHORIZON_MIN_PROCESSES=1\nHORIZON_MAX_PROCESSES=1\nHORIZON_QUEUES=public-inspection-empty\n' >> "$SIFTO_INSPECT_ENV"
for inspect_service in old target; do
 dc up -d postgres redis
 dc run --rm -T --no-deps --entrypoint php "$inspect_service" /startup/inspect-fixture.php hold
 dc up -d --no-deps "$inspect_service"
 dc exec -T "$inspect_service" sh /startup/runtime-wait-ready.sh
 dc exec -T "$inspect_service" php /startup/inspect-fixture.php install
 dc exec -T "$inspect_service" php -d auto_prepend_file=/startup/inspect-diagnose.php /probe/startup.php horizon-inspect-request
 dc exec -T "$inspect_service" php /startup/inspect-fixture.php wait-receipt
 dc exec -T "$inspect_service" php /probe/startup.php horizon-inspect-observe > "$inspect_staging/$inspect_service-receipt.json"
 cat "$inspect_staging/$inspect_service-receipt.json"
 dc exec -T "$inspect_service" php /startup/inspect-fixture.php verify-positive
 if inspect_rejected=$(dc exec -T "$inspect_service" php /probe/startup.php horizon-inspect-request); then exit 1; fi
 test "$inspect_rejected" = '{"version":1,"operation":"horizon-inspect-request","category":"horizon_inspection_held"}'
 printf '%s\n' "$inspect_rejected"
 dc exec -T "$inspect_service" php /startup/inspect-fixture.php verify-positive
 dc exec -T "$inspect_service" php /startup/inspect-fixture.php negative-producer-expired
 for inspect_case in stale expired foreign reordered duplicate config no-ack; do
  dc exec -T "$inspect_service" php /startup/inspect-fixture.php "negative-$inspect_case"
  if inspect_rejected=$(dc exec -T "$inspect_service" php /probe/startup.php horizon-inspect-observe); then exit 1; fi
  test "$inspect_rejected" = '{"version":1,"operation":"horizon-inspect-observe","category":"horizon_inspection_held"}'
  printf '%s\n' "$inspect_rejected"
 done
 dc down --volumes --timeout 10
 empty
done
for inspect_service in old target; do
 case "$inspect_service" in old) inspect_version=4.4.2 ;; target) inspect_version=4.4.6 ;; esac
 dc run --rm -T --no-deps unit python -B -c 'import sys; from maintenance.coolify_startup import parse_horizon_inspection; value = parse_horizon_inspection(sys.stdin.buffer.read(8193), selected_version=sys.argv[1]); assert value.live_worker_count == 1 and value.worker_object_count == 1 and value.maintenance == "held" and value.force is False; print("PUBLIC typed actual-supervisor receipt accepted without drain authority")' "$inspect_version" < "$inspect_staging/$inspect_service-receipt.json"
done
