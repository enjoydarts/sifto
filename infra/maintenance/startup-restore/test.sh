#!/bin/sh
set -eu
restore_root=$(CDPATH= cd -- "$(dirname -- "$0")/../../.." && pwd)
restore_public=${SIFTO_RESTORE_PUBLIC:?public artifact directory required}
dc() { docker compose --env-file /dev/null -p sifto-horizon-state-restoration-worker -f "$restore_root/infra/maintenance/startup-restore/compose.yml" "$@"; }
empty() {
 for restore_kind in container volume network; do
  case "$restore_kind" in
   container) restore_found=$(docker ps -aq --filter label=com.docker.compose.project=sifto-horizon-state-restoration-worker) || return 1 ;;
   volume) restore_found=$(docker volume ls -q --filter label=com.docker.compose.project=sifto-horizon-state-restoration-worker) || return 1 ;;
   network) restore_found=$(docker network ls -q --filter label=com.docker.compose.project=sifto-horizon-state-restoration-worker) || return 1 ;;
  esac
  test -z "$restore_found" || return 1
 done
}
empty
restore_staging=$(mktemp -d "$restore_public/fixture.XXXXXXXX")
SIFTO_INSPECT_ENV="$restore_staging/public.env"
SIFTO_STARTUP_PUBLIC_INIT="$restore_staging/init"
export SIFTO_INSPECT_ENV SIFTO_STARTUP_PUBLIC_INIT
cleanup() {
 restore_status=$?; trap - EXIT
 if test -n "${restore_running:-}"; then
  dc exec -T "$restore_running" sh -c 'cat /sys/fs/cgroup/memory.events /sys/fs/cgroup/memory.peak /sys/fs/cgroup/pids.current' || restore_status=1
  dc exec -T "$restore_running" php /startup/restore-fixture.php diagnostic || restore_status=1
  dc exec -T "$restore_running" php /startup/restore-fixture.php settle || restore_status=1
 fi
 dc down --volumes --timeout 20 || restore_status=1
 empty || restore_status=1
 exit "$restore_status"
}
trap cleanup EXIT
trap 'exit 130' INT
trap 'exit 143' TERM
cp "$restore_root/infra/maintenance/maintenance/coolify_startup_init" "$SIFTO_STARTUP_PUBLIC_INIT"
chmod 0755 "$SIFTO_STARTUP_PUBLIC_INIT"
sed 's/^HORIZON_ENABLED=false$/HORIZON_ENABLED=true/' "$restore_root/infra/maintenance/startup/public.env" > "$SIFTO_INSPECT_ENV"
printf '\nHORIZON_PREFIX=sifto_public_restore_horizon:\nHORIZON_BALANCE=simple\nHORIZON_MIN_PROCESSES=2\nHORIZON_MAX_PROCESSES=4\nHORIZON_QUEUES=public-drain,public-restore-paused\nHORIZON_MAX_TIME=35\n' >> "$SIFTO_INSPECT_ENV"
restore_services=${SIFTO_RESTORE_IMAGE:-'old target'}
restore_cases=${SIFTO_RESTORE_CASES:-'running mixed paused expired'}
for restore_service in $restore_services; do
 if test "${SIFTO_RESTORE_PROFILE:-component}" = component; then restore_service="$restore_service-component"; fi
 for restore_case in $restore_cases; do
 printf '%s\n' "PUBLIC case $restore_service $restore_case profile=${SIFTO_RESTORE_PROFILE:-component}"
 dc up -d postgres redis
 dc run --rm -T --no-deps --entrypoint php "$restore_service" /startup/inspect-fixture.php hold
 dc up -d --no-deps "$restore_service"
 restore_running=$restore_service
 if test "${SIFTO_RESTORE_PROFILE:-component}" = full-s6; then dc exec -T "$restore_service" sh /startup/runtime-wait-ready.sh; fi
 dc exec -T "$restore_service" php /startup/restore-fixture.php install
 dc exec -T "$restore_service" php /startup/restore-fixture.php start
 restore_intent=$restore_case
 if test "$restore_case" = expired; then restore_intent=mixed; fi
 case "$restore_case" in fault-*) restore_intent=running ;; esac
 dc exec -T "$restore_service" php /startup/restore-fixture.php "$restore_intent"
 dc exec -T "$restore_service" sh -c 'cat /sys/fs/cgroup/memory.events /sys/fs/cgroup/memory.peak /sys/fs/cgroup/pids.current'
 dc exec -T "$restore_service" php -d auto_prepend_file=/startup/inspect-diagnose.php /probe/startup.php horizon-drain-enter
 case "$restore_case" in
 fault-*) dc exec -T "$restore_service" php /startup/restore-fixture.php "$restore_case" ;;
 *)
 if test "$restore_case" = expired; then dc exec -T "$restore_service" php /startup/restore-fixture.php expire; fi
 dc exec -T "$restore_service" php -d auto_prepend_file=/startup/inspect-diagnose.php /probe/startup.php horizon-restore
 dc exec -T "$restore_service" php /startup/restore-fixture.php verify
 if test "$restore_case" = mixed || test "$restore_case" = expired; then dc exec -T "$restore_service" php /startup/restore-fixture.php native; fi
 ;;
 esac
 dc exec -T "$restore_service" sh -c 'cat /sys/fs/cgroup/memory.events /sys/fs/cgroup/memory.peak /sys/fs/cgroup/pids.current'
 dc down --volumes --timeout 20
 restore_running=
 empty
 done
done
