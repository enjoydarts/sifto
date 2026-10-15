#!/bin/sh
# Mounted only by the PUBLIC own-tool fault fixture, never by the app fixture.
set -eu
[ "$#" -eq 2 ] && [ "$1" = -a ] && [ "$2" = list ] || exit 2
cat "/proc/$$/stat" > /tmp/observer-tool-stat
case "$(cat /tmp/observer-tool-case)" in
  deadline) ;;
  overflow) printf '%05000d' 0 ;;
  *) exit 2 ;;
esac
exec /bin/sleep 6
