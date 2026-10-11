#!/bin/bash
set -euo pipefail
umask 077
install -d -o postgres -g postgres -m 0700 /tmp/pg-data /tmp/pg-socket
runuser -u postgres -- /usr/lib/postgresql/"$PG_MAJOR"/bin/initdb -D /tmp/pg-data >/tmp/init.log
runuser -u postgres -- /usr/lib/postgresql/"$PG_MAJOR"/bin/pg_ctl -D /tmp/pg-data -o '-h "" -k /tmp/pg-socket' -w start >/tmp/start.log
python3 -B /fixture/test-proof-integration.py
