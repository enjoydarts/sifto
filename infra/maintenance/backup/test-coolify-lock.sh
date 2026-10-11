#!/bin/bash
# Synthetic fixture: backup/restore must share the same exclusion boundary.
set -euo pipefail
umask 077
install -d -m 0700 /var/lib/sifto-maintenance/backups
printf 'synthetic archive placeholder\n' > /var/lib/sifto-maintenance/backups/coolify.dump
exec 9>/var/lib/sifto-maintenance/coolify-backup.lock
flock -n 9
for script in coolify-backup.sh coolify-restore-check.sh; do
  if bash "/fixture/$script" >/tmp/denied.out 2>&1; then
    printf 'concurrent recovery operation unexpectedly accepted\n' >&2; exit 1
  fi
  [[ $(cat /tmp/denied.out) == *'in use'* || $(cat /tmp/denied.out) == *'already running'* ]]
done
[[ ! -e /var/lib/sifto-maintenance/evidence/coolify-restore.json ]]
install -d -m 0700 /data/backups
printf 'synthetic archive placeholder\n' > /data/backups/sifto-20261011-0300.dump.gz
exec 8>/var/lib/sifto-maintenance/restore-check.lock
flock -n 8
if bash /fixture/restore-check.sh /data/backups/sifto-20261011-0300.dump.gz >/tmp/denied.out 2>&1; then
  printf 'concurrent DB restore unexpectedly accepted\n' >&2; exit 1
fi
[[ $(cat /tmp/denied.out) == *'already running'* ]]
printf 'Coolify backup and restore share one lock; no evidence created while busy\n'
