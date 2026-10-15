#!/bin/sh
# Finite public S1 only. No native dispatch, queue/session inspection or drain.
set -eu
runtime_root=$(CDPATH= cd -- "$(dirname -- "$0")/../../.." && pwd)
runtime_compose="$runtime_root/infra/maintenance/startup-runtime/compose.yml"
runtime_public=${SIFTO_STARTUP_RUNTIME_PUBLIC:?public artifact directory required}
dc() { docker compose --env-file /dev/null -p sifto-startup-runtime-observe-worker -f "$runtime_compose" "$@"; }
empty() {
 for runtime_kind in container volume network; do
  case "$runtime_kind" in
   container) runtime_found=$(docker ps -aq --filter label=com.docker.compose.project=sifto-startup-runtime-observe-worker) || return 1 ;;
   volume) runtime_found=$(docker volume ls -q --filter label=com.docker.compose.project=sifto-startup-runtime-observe-worker) || return 1 ;;
   network) runtime_found=$(docker network ls -q --filter label=com.docker.compose.project=sifto-startup-runtime-observe-worker) || return 1 ;;
  esac
  test -z "$runtime_found" || return 1
 done
}
empty || { printf 'runtime fixture requires empty owned namespace\n' >&2; exit 1; }
SIFTO_STARTUP_RUNTIME_STAGING=$(mktemp -d "$runtime_public/fixture.XXXXXXXX")
SIFTO_STARTUP_RUNTIME_ENV="$SIFTO_STARTUP_RUNTIME_STAGING/canonical.env"
SIFTO_STARTUP_PUBLIC_INIT="$SIFTO_STARTUP_RUNTIME_STAGING/init"
SIFTO_STARTUP_RUNTIME_TOOL_FAULT="$SIFTO_STARTUP_RUNTIME_STAGING/tool-fault"
export SIFTO_STARTUP_RUNTIME_STAGING SIFTO_STARTUP_RUNTIME_ENV SIFTO_STARTUP_PUBLIC_INIT SIFTO_STARTUP_RUNTIME_TOOL_FAULT
runtime_extra_pid=
cleanup() {
 runtime_status=$?; trap - EXIT
 if test -n "$runtime_extra_pid"; then wait "$runtime_extra_pid" || runtime_status=1; fi
 if ! dc down --volumes --timeout 10; then runtime_status=1; fi
 if ! empty; then runtime_status=1; fi
 # Keep bounded PUBLIC observations and generated canonical inputs for review.
 exit "$runtime_status"
}
trap cleanup EXIT
trap 'exit 130' INT
trap 'exit 143' TERM
cp "$runtime_root/infra/maintenance/maintenance/coolify_startup_init" "$SIFTO_STARTUP_PUBLIC_INIT"
chmod 0755 "$SIFTO_STARTUP_PUBLIC_INIT"
cmp "$runtime_root/infra/maintenance/maintenance/coolify_startup_init" "$SIFTO_STARTUP_PUBLIC_INIT"
cp "$runtime_root/infra/maintenance/startup-runtime/tool-fault.sh" "$SIFTO_STARTUP_RUNTIME_TOOL_FAULT"
chmod 0755 "$SIFTO_STARTUP_RUNTIME_TOOL_FAULT"
cmp "$runtime_root/infra/maintenance/startup-runtime/tool-fault.sh" "$SIFTO_STARTUP_RUNTIME_TOOL_FAULT"
image() {
 runtime_observed=$(docker image inspect --format '{{.Id}} {{.Os}}/{{.Architecture}}' "$1")
 test "$runtime_observed" = "$2 linux/arm64"
 runtime_digests=$(docker image inspect --format '{{json .RepoDigests}}' "$1")
 case "$runtime_digests" in *"@${1##*@}"*) ;; *) exit 1 ;; esac
}
image docker.io/coollabsio/coolify@sha256:a323ce1dfb54101b0c30d8bd5c92b0e4166c3b403a164316f8876be653204960 sha256:6145ffe0650d8dcdef48ab08442dc01b066d6450e1f6df9e3cd8efacadbb0064
image docker.io/coollabsio/coolify@sha256:f4ef1eb69daa265057ab700646485bd78bb43bcf96c691d65094947acb7f50a5 sha256:292e1d3e300d6eab9598e14a017bfa2e35bd89f78597916c179c24e099e40156
image docker.io/library/postgres@sha256:91a22c143801df42c2e4a8f9926eab5e48f39f40beb5220b7c29d4478263c341 sha256:00f747f11f25a40d61020089b8cafd867de9f8716fd054ea661a2f6a5b7c4f76
image docker.io/library/redis@sha256:1f09a89a207d794a8c61d9edfc26e7c58427de10ccef7c5d18d638df79a63b85 sha256:ec4f171e9ff071bb2a1c936057d3be00206069b028b6c0d61df2477e1dd02c42
# Compose must resolve the env_file before the isolated generator starts.
cp "$runtime_root/infra/maintenance/startup/public.env" "$SIFTO_STARTUP_RUNTIME_ENV"
dc run --rm -T --no-deps controller python -B startup-runtime/check.py generate
# Source-only real-child faults run before any application/database resource.
for runtime_tool in tool-old tool-target; do
 for runtime_fault in deadline overflow; do
  dc run --rm -T --no-deps "$runtime_tool" "$runtime_fault"
 done
done
dc up -d postgres redis
for runtime_service in old target; do
 dc up -d --no-deps "$runtime_service"
 dc exec -T "$runtime_service" php /startup/runtime-fixture.php prepare-hmac
 dc exec -T "$runtime_service" php /startup/runtime-fixture.php source-runtime
 dc exec -T "$runtime_service" sh /startup/runtime-wait-ready.sh
 # Producer is a direct exec. No parent fixture PHP is whitelisted by product.
 dc exec -T "$runtime_service" php /probe/startup.php runtime-observe > "$SIFTO_STARTUP_RUNTIME_STAGING/$runtime_service.json"
 if test "$runtime_service" = old; then dc stop --timeout 10 old; fi
done
for runtime_mode in ssh-reconstruct barrier-observe barrier-enter barrier-restore; do
 if runtime_negative=$(dc exec -T -e HORIZON_ENABLED=true -e SCHEDULER_ENABLED=true target php /probe/startup.php "$runtime_mode"); then exit 1; fi
 test "$runtime_negative" = "{\"version\":1,\"operation\":\"$runtime_mode\",\"category\":\"ssh_environment_invalid\"}"
 printf '%s\n' "$runtime_negative"
done
if runtime_negative=$(dc exec -T -e HORIZON_ENABLED=true target php /probe/startup.php runtime-observe); then exit 1; fi
test "$runtime_negative" = '{"version":1,"operation":"runtime-observe","category":"runtime_environment_invalid"}'
printf '%s\n' "$runtime_negative"
if runtime_negative=$(dc exec -T -e MIGRATION_ENABLED=true target php /probe/startup.php runtime-observe); then exit 1; fi
test "$runtime_negative" = '{"version":1,"operation":"runtime-observe","category":"ssh_environment_invalid"}'
printf '%s\n' "$runtime_negative"
dc exec -T target php /startup/runtime-fixture.php extra-process > "$SIFTO_STARTUP_RUNTIME_STAGING/extra.txt" &
runtime_extra_pid=$!
runtime_attempt=0
until test "$(cat "$SIFTO_STARTUP_RUNTIME_STAGING/extra.txt")" = 'PUBLIC EXTRA PROCESS STARTED'; do
 runtime_attempt=$((runtime_attempt + 1)); test "$runtime_attempt" -lt 15; sleep 1
done
dc exec -T target php /probe/startup.php runtime-observe > "$SIFTO_STARTUP_RUNTIME_STAGING/unknown.json"
wait "$runtime_extra_pid"; runtime_extra_pid=
dc stop --timeout 10 target
SIFTO_STARTUP_RUNTIME_ENV="$SIFTO_STARTUP_RUNTIME_STAGING/workers.env"; export SIFTO_STARTUP_RUNTIME_ENV
# Hold synthetic maintenance before enabling real workers; no jobs are seeded.
dc run --rm -T --no-deps --entrypoint php target /startup/fixture.php hold
dc up -d --no-deps target
dc exec -T target php /startup/runtime-fixture.php prepare-hmac
dc exec -T target sh /startup/runtime-wait-ready.sh
dc exec -T target php /probe/startup.php runtime-observe > "$SIFTO_STARTUP_RUNTIME_STAGING/workers.json"
cat "$SIFTO_STARTUP_RUNTIME_STAGING/old.json" "$SIFTO_STARTUP_RUNTIME_STAGING/target.json" \
 "$SIFTO_STARTUP_RUNTIME_STAGING/unknown.json" "$SIFTO_STARTUP_RUNTIME_STAGING/workers.json" \
 | dc run --rm -T --no-deps controller python -B startup-runtime/check.py validate
