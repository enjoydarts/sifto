#!/bin/bash
# Local root-only recovery material. No upload, package changes or service stop.
set -euo pipefail
umask 077
[[ $(id -u) == 0 ]]
backup_dir=/var/lib/sifto-maintenance/backups
[[ ! -L /var/lib/sifto-maintenance && ! -L "$backup_dir" ]]
install -d -o root -g root -m 0700 /var/lib/sifto-maintenance "$backup_dir"
exec 9>/var/lib/sifto-maintenance/coolify-backup.lock
flock -n 9 || { printf 'Coolify backup already running\n' >&2; exit 1; }
dump=$(mktemp "$backup_dir/coolify.dump.XXXXXXXX")
config=$(mktemp "$backup_dir/coolify-config.tar.gz.XXXXXXXX")
trap 'rm -f -- "$dump" "$config"' EXIT
docker exec coolify-db pg_dump --username=coolify --format=custom --dbname=coolify > "$dump"
docker exec -i coolify-db pg_restore --list < "$dump" >/dev/null
tar --directory /data/coolify --create --gzip --file "$config" source ssh ssl proxy
gzip -t "$config"
mv -f -- "$dump" "$backup_dir/coolify.dump"
mv -f -- "$config" "$backup_dir/coolify-config.tar.gz"
date +%s > "$backup_dir/created_at"
printf 'Coolify local recovery backup created; isolated restore and offhost copy are separate\n'
