#!/bin/sh
# Test fixture ordering only. Never retries the product observer to green.
set -eu
export LC_ALL=C
runtime_expected=$(printf '%s\n' s6rc-oneshot-runner horizon nginx php-fpm reverb scheduler-worker \
 terminal-server db-migration init-script init-seeder fix-attrs legacy-cont-init legacy-services | sort)
runtime_deadline=$(( $(date +%s) + 30 ))
while test "$(date +%s)" -le "$runtime_deadline"; do
 runtime_active=$(/command/s6-rc -a list 2>/dev/null | sort) || runtime_active=
 runtime_fpm=$(/command/s6-svstat -o up,wantedup,ready,paused /run/service/php-fpm 2>/dev/null) || runtime_fpm=
 runtime_nginx=$(/command/s6-svstat -o up,wantedup,ready,paused /run/service/nginx 2>/dev/null) || runtime_nginx=
 if test "$runtime_active" = "$runtime_expected" && test "$runtime_fpm" = 'true true true false' \
   && test "$runtime_nginx" = 'true true true false'; then
  printf '{"ok":true,"category":"fixture_s6_settled"}\n'; exit 0
 fi
 sleep 1
done
printf '{"ok":false,"category":"fixture_s6_startup_timeout"}\n' >&2
exit 1
