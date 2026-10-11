#!/bin/bash
# Root-local timer only. Root-only backup.env supplies R2 credentials and the
# password. This script never enables database maintenance or claims a restore.
set -euo pipefail
umask 077
[[ $(id -u) == 0 && $# == 1 ]]
role="$1"
[[ "$role" == database || "$role" == application ]]
env_file=/etc/sifto-maintenance/backup.env
[[ -f "$env_file" && ! -L "$env_file" && $(stat -c %u "$env_file") == 0 ]]
[[ $(stat -c %a "$env_file") == 600 ]]
compose=/opt/sifto-maintenance/backup/compose.yml
[[ -f "$compose" && ! -L "$compose" && $(stat -c %u "$compose") == 0 ]]
exec 9>/var/lib/sifto-maintenance/backup.lock
flock -n 9 || { printf 'offhost backup already running\n' >&2; exit 1; }
if [[ "$role" == database ]]; then
  candidate=$(find /data/backups -maxdepth 1 -type f -name 'sifto-*.dump.gz' -printf '%T@ %p\n' | sort -nr | head -1 | cut -d ' ' -f2-)
  [[ "$candidate" =~ ^/data/backups/sifto-[0-9]{8}-[0-9]{4,6}\.dump\.gz$ && ! -L "$candidate" ]]
  now=$(date +%s)
  stamp=$(stat -c %Y "$candidate")
  [[ $((now-stamp)) -ge 0 && $((now-stamp)) -lt 86400 ]]
  gzip -t "$candidate"
  export MAINTENANCE_BACKUP_SOURCE="$candidate"
else
  # Refresh the local recovery material every day before uploading it.
  /bin/bash /opt/sifto-maintenance/backup/coolify-backup.sh
  exec 8>/var/lib/sifto-maintenance/coolify-backup.lock
  flock -n 8 || { printf 'Coolify recovery material is in use\n' >&2; exit 1; }
  export MAINTENANCE_BACKUP_SOURCE=/var/lib/sifto-maintenance/backups
  [[ -d "$MAINTENANCE_BACKUP_SOURCE" && ! -L "$MAINTENANCE_BACKUP_SOURCE" ]]
  [[ -f "$MAINTENANCE_BACKUP_SOURCE/coolify.dump" ]]
  [[ -f "$MAINTENANCE_BACKUP_SOURCE/coolify-config.tar.gz" ]]
fi
docker compose --project-name sifto-maintenance-backup --file "$compose" \
  run --rm --no-deps backup backup /source --tag "sifto-$role" --json \
  > /var/lib/sifto-maintenance/last-backup.json
docker compose --project-name sifto-maintenance-backup --file "$compose" \
  run --rm --no-deps backup check --read-data \
  > /var/lib/sifto-maintenance/last-backup-check.txt
printf 'encrypted offhost repository checked; restore evidence and enablement still separate\n'
