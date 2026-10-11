#!/bin/bash
# Existing postgres crontab can call this path unchanged. Run as postgres.
set -euo pipefail
umask 077
backup_dir=/data/backups
[[ $(id -un) == postgres && -d "$backup_dir" && ! -L "$backup_dir" ]]
exec 9>"$backup_dir/.sifto-backup.lock"
flock -n 9 || { printf 'another backup is running\n' >&2; exit 1; }
stamp=$(date +%Y%m%d-%H%M%S)
partial=$(mktemp "$backup_dir/.sifto-backup.XXXXXXXX.partial")
trap 'rm -f -- "$partial"' EXIT
/usr/bin/pg_dump --username=postgres --format=custom --dbname=sifto | gzip > "$partial"
gzip -t "$partial"
# --list reads only the archive header/TOC. Drain the remaining stream so a
# valid large dump cannot fail pipefail when gzip receives an early SIGPIPE.
gzip -dc "$partial" | { /usr/bin/pg_restore --list >/dev/null; cat >/dev/null; }
mv -- "$partial" "$backup_dir/sifto-$stamp.dump.gz"
find "$backup_dir" -maxdepth 1 -type f -name 'sifto-*.dump.gz' -mtime +14 -delete
printf 'logical backup completed; restore and offhost verification are separate\n'
