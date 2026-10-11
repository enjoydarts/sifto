#!/bin/bash
# Verify an existing logical dump on the DB host, with no network or data egress.
# A successful check alone does NOT satisfy the offhost-backup requirement.
set -euo pipefail
umask 077
[[ $(id -u) == 0 && $# == 1 ]]
dump="$1"
[[ "$dump" =~ ^/data/backups/sifto-[0-9]{8}-[0-9]{4,6}\.dump\.gz$ && -f "$dump" && ! -L "$dump" ]]
[[ ! -L /data && ! -L /data/backups ]]
[[ ! -L /var/lib/sifto-maintenance ]]
install -d -o root -g root -m 0700 /var/lib/sifto-maintenance
exec 9>/var/lib/sifto-maintenance/restore-check.lock
flock -n 9 || { printf 'database restore check already running\n' >&2; exit 1; }
container=sifto-maintenance-restore-probe
if docker inspect "$container" >/dev/null 2>&1; then
  printf 'dedicated restore container already exists\n' >&2
  exit 1
fi
work=$(mktemp -d /data/sifto-restore-test.XXXXXXXX)
cleanup() {
  docker rm -f "$container" >/dev/null 2>&1 || true
  [[ "$work" == /data/sifto-restore-test.* && "$work" != /data/sifto-restore-test. ]] && rm -rf -- "$work"
}
trap cleanup EXIT
docker pull postgres:16.13-bookworm >/dev/null
image=$(docker image inspect postgres:16.13-bookworm --format '{{.Id}}')
docker run -d --name "$container" --network none --memory 768m --cpus 1 \
  -e POSTGRES_DB=sifto_restore -e POSTGRES_HOST_AUTH_METHOD=trust \
  --mount "type=bind,src=$work,dst=/var/lib/postgresql/data" "$image" >/dev/null
ready=false
for attempt in $(seq 1 60); do
  if docker exec "$container" psql -X -v ON_ERROR_STOP=1 -U postgres -d sifto_restore -Atc 'SELECT 1' >/dev/null 2>&1; then
    ready=true
    break
  fi
  sleep 1
done
[[ "$ready" == true ]]
printf 'isolated restore started\n'
gzip -dc "$dump" | docker exec -i "$container" pg_restore \
  --exit-on-error --no-owner --no-privileges --username=postgres --dbname=sifto_restore
docker exec "$container" psql -X -v ON_ERROR_STOP=1 -U postgres -d sifto_restore -Atc \
  "SELECT count(*) FROM information_schema.tables WHERE table_schema='public'; SELECT extversion FROM pg_extension WHERE extname='pgcrypto'; SELECT pg_database_size(current_database());"
digest=$(sha256sum "$dump" | cut -d ' ' -f 1)
backup_id=$(basename "$dump" .dump.gz)
install -d -m 0700 /var/lib/sifto-maintenance/evidence
evidence=$(mktemp /var/lib/sifto-maintenance/evidence/restore.XXXXXXXX)
printf '{"backup_id":"%s","sha256":"%s","restore_tested_at":%s,"test_location":"same-db-host-network-none","image_id":"%s","offhost_verified":false}\n' \
  "$backup_id" "$digest" "$(date +%s)" "$image" > "$evidence"
mv -- "$evidence" "/var/lib/sifto-maintenance/evidence/restore-$backup_id.json"
printf 'isolated restore verified; offhost backup still required\n'
