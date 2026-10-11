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
runuser -u postgres -- /bin/bash /fixture/postgres-backup.sh
dump=$(find /data/backups -name '*.dump.gz')
[[ -f "$dump" && $(stat -c %a "$dump") == 600 ]]
createdb -U postgres restored
gzip -dc "$dump" | pg_restore --exit-on-error --no-owner --no-privileges -U postgres -d restored
[[ $(psql -X -U postgres -d restored -Atc 'SELECT count(*) FROM backup_probe') == 1 ]]
printf 'backup failure handling, permissions and isolated fixture restore verified\n'
