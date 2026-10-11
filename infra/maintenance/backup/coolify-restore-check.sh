#!/bin/bash
# Keep production configuration and data on this host; discard only test data.
set -euo pipefail
umask 077
[[ $(id -u) == 0 ]]
exec 9>/var/lib/sifto-maintenance/coolify-backup.lock
flock -n 9 || { printf 'Coolify recovery material is in use\n' >&2; exit 1; }
dump=/var/lib/sifto-maintenance/backups/coolify.dump
[[ -f "$dump" && ! -L "$dump" ]]
container=sifto-coolify-restore-probe
if docker inspect "$container" >/dev/null 2>&1; then
  printf 'dedicated restore container already exists\n' >&2
  exit 1
fi
work=$(mktemp -d /var/lib/sifto-maintenance/coolify-restore.XXXXXXXX)
cleanup() {
  docker rm -f "$container" >/dev/null 2>&1 || true
  [[ "$work" == /var/lib/sifto-maintenance/coolify-restore.* ]] && rm -rf -- "$work"
}
trap cleanup EXIT
docker pull postgres:15.19-bookworm >/dev/null
image=$(docker image inspect postgres:15.19-bookworm --format '{{.Id}}')
docker run -d --name "$container" --network none --memory 256m --cpus 1 \
  -e POSTGRES_DB=coolify_restore -e POSTGRES_HOST_AUTH_METHOD=trust \
  --mount "type=bind,src=$work,dst=/var/lib/postgresql/data" "$image" >/dev/null
ready=false
for attempt in $(seq 1 60); do
  if docker exec "$container" psql -X -v ON_ERROR_STOP=1 -U postgres -d coolify_restore -Atc 'SELECT 1' >/dev/null 2>&1; then
    ready=true
    break
  fi
  sleep 1
done
[[ "$ready" == true ]]
docker exec -i "$container" pg_restore --exit-on-error --no-owner --no-privileges \
  --username=postgres --dbname=coolify_restore < "$dump"
docker exec "$container" psql -X -v ON_ERROR_STOP=1 -U postgres -d coolify_restore -Atc \
  "SELECT count(*) FROM information_schema.tables WHERE table_schema='public';"
digest=$(sha256sum "$dump" | cut -d ' ' -f 1)
install -d -m 0700 /var/lib/sifto-maintenance/evidence
printf '{"sha256":"%s","restore_tested_at":%s,"test_location":"same-app-host-network-none","image_id":"%s","offhost_verified":false}\n' \
  "$digest" "$(date +%s)" "$image" > /var/lib/sifto-maintenance/evidence/coolify-restore.json
printf 'Coolify isolated DB restore verified; configuration restoration and offhost copy still separate\n'
