#!/bin/bash
# Check existing: ROLE. Create explicitly after storage approval: ROLE --create.
set -euo pipefail
umask 077
[[ $(id -u) == 0 && ( $# == 1 || ( $# == 2 && $2 == --create ) ) ]]
exec /usr/bin/python3 -I /opt/sifto-maintenance/backup/backup_readiness.py init "$@"
