#!/bin/bash
# Root-local timer only. Approval and full retrieved restore are checked in Python.
set -euo pipefail
umask 077
[[ $(id -u) == 0 && $# == 1 ]]
exec /usr/bin/python3 -I /opt/sifto-maintenance/backup/backup_readiness.py backup "$1"
