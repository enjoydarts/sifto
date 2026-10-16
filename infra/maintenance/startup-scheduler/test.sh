#!/bin/sh
# Actual native proof runs on the existing Linux ARM policy runner, never a
# privileged/host-PID container and never a macOS child-kernel substitute.
set -eu
scheduler_root=$(CDPATH= cd -- "$(dirname -- "$0")/../../.." && pwd)
scheduler_public=${SIFTO_STARTUP_SCHEDULER_PUBLIC:-/tmp/sifto-startup-scheduler-public}
test "$(uname -s)" = Linux || { printf 'native scheduler proof requires Linux Docker host; pending\n' >&2; exit 2; }
test "$(uname -m)" = aarch64 || { printf 'native scheduler proof requires ARM64 Docker host\n' >&2; exit 2; }
# Only a direct protected /tmp child is accepted for root-produced evidence.
# Never follow a caller-created symlink or adopt a foreign writable directory.
sudo -n /bin/sh -eu -c '
 p=$1
 case "$p" in /tmp/sifto-startup-scheduler-public*) ;; *) exit 2 ;; esac
 test "$(dirname "$p")" = /tmp
 case "${p#/tmp/}" in *[!a-zA-Z0-9_.-]*|"") exit 2 ;; esac
 if test -e "$p" || test -L "$p"; then
  test -d "$p" && test ! -L "$p" && test "$(stat -c %u:%a "$p")" = 0:700
 else
  umask 077; mkdir "$p"
 fi
 test "$(readlink -f "$p")" = "$p"
' sh "$scheduler_public"
scheduler_venv=$(sudo -n mktemp -d /tmp/sifto-startup-scheduler-venv.XXXXXXXX)
case "$scheduler_venv" in /tmp/sifto-startup-scheduler-venv.*) ;; *) exit 1 ;; esac
cleanup() {
 scheduler_status=$?; trap - EXIT
 if ! sudo -n rm -rf -- "$scheduler_venv"; then scheduler_status=1; fi
 printf '{"category":"native_outer_exit","exit_code":%s}\n' "$scheduler_status" \
  | sudo -n tee "$scheduler_public/native-outer-$$.json" >/dev/null
 exit "$scheduler_status"
}
trap cleanup EXIT
# Reuse the existing universal hash lock without installing host packages.
# The native observer imports only this exact crypto dependency closure.
sudo -n /usr/bin/python3 -m venv "$scheduler_venv"
awk '/^[a-zA-Z0-9_-]+==/ { keep = ($0 ~ /^(cryptography|cffi|pycparser)==/) } keep { print }' \
 "$scheduler_root/infra/maintenance/requirements.txt" \
 | sudo -n tee "$scheduler_venv/requirements.lock" >/dev/null
sudo -n "$scheduler_venv/bin/python3" -m pip --isolated install --require-hashes --only-binary=:all: \
 --no-deps --no-cache-dir --disable-pip-version-check -r "$scheduler_venv/requirements.lock"
# A short-lived source-owned scope bounds observer and Docker CLI children.
# Docker-created APP/PG/Redis retain their independent fixed caps.
set +e
sudo -n systemd-run --scope --quiet --collect \
 --property=MemoryMax=134217728 --property=MemorySwapMax=0 \
 --property=CPUQuota=25% --property=TasksMax=64 \
 "$scheduler_venv/bin/python3" -B "$scheduler_root/infra/maintenance/startup-scheduler/check.py" native "$scheduler_public"
scheduler_status=$?
set -e
exit "$scheduler_status"
