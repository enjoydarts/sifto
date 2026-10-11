#!/bin/bash
# Disposable, network-disabled container. Never mount a production directory.
set -euo pipefail
mkdir -p /data/backups
chown postgres:postgres /data/backups
real_dump=$(readlink -f /usr/bin/pg_dump)
rm /usr/bin/pg_dump
printf '#!/bin/sh\nexit 77\n' > /usr/bin/pg_dump
chmod 0755 /usr/bin/pg_dump
if runuser -u postgres -- /bin/bash /fixture/postgres-backup.sh >/tmp/probe.log 2>&1; then
  printf 'upstream failure was falsely accepted\n' >&2
  exit 1
fi
[[ -z $(find /data/backups -name '*.dump.gz' -o -name '*.partial') ]]
printf '#!/bin/sh\nprintf "invalid archive"\n' > /usr/bin/pg_dump
if runuser -u postgres -- /bin/bash /fixture/postgres-backup.sh >/tmp/probe.log 2>&1; then
  printf 'invalid archive was falsely accepted\n' >&2
  exit 1
fi
[[ -z $(find /data/backups -name '*.dump.gz' -o -name '*.partial') ]]
rm /usr/bin/pg_dump
ln -s "$real_dump" /usr/bin/pg_dump
install -d -o postgres -g postgres -m 0700 /tmp/database
runuser -u postgres -- initdb -D /tmp/database >/dev/null
runuser -u postgres -- pg_ctl -D /tmp/database -o '-k /tmp' -w start >/dev/null
trap 'runuser -u postgres -- pg_ctl -D /tmp/database -m fast -w stop >/dev/null' EXIT
export PGHOST=/tmp
createdb -U postgres sifto
psql -X -v ON_ERROR_STOP=1 -U postgres -d sifto -c 'CREATE TABLE backup_probe(id int); INSERT INTO backup_probe VALUES(1);' >/dev/null
# Large enough to expose early-reader SIGPIPE in archive TOC inspection.
psql -X -v ON_ERROR_STOP=1 -U postgres -d sifto >/dev/null <<'SQL'
CREATE TABLE backup_bulk AS
SELECT i, (SELECT string_agg(md5((i*4096+j)::text), '')
           FROM generate_series(1,4096) AS blocks(j)) AS payload
FROM generate_series(1,32) AS rows(i);
SQL
# Production sudo can retain an invoking user's inaccessible home directory.
# Exercise that case without exposing any production files to this fixture.
install -d -m 0700 /tmp/inaccessible-caller
cd /tmp/inaccessible-caller
runuser -u postgres -- /bin/bash /fixture/postgres-backup.sh
cd /
dump=$(find /data/backups -name '*.dump.gz')
[[ -f "$dump" && $(stat -c %a "$dump") == 600 ]]
createdb -U postgres restored
gzip -dc "$dump" | pg_restore --exit-on-error --no-owner --no-privileges -U postgres -d restored
[[ $(psql -X -U postgres -d restored -Atc 'SELECT count(*) FROM backup_probe') == 1 ]]
[[ $(psql -X -U postgres -d restored -Atc 'SELECT count(*) FROM backup_bulk') == 32 ]]
printf 'backup failure handling, permissions and isolated fixture restore verified\n'
