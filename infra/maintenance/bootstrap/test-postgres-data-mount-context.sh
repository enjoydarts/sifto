#!/bin/sh
# Export only a private synthetic context; never copy a host repository or secrets.
set -eu
umask 077
scratch=$(mktemp -d)
trap 'rm -rf "$scratch"' EXIT
trap 'exit 1' HUP INT TERM
mkdir -p "$scratch/context/bootstrap/private" "$scratch/export"
fixture=infra/maintenance/bootstrap
for file in Dockerfile.postgres-data-mount Dockerfile.postgres-data-mount.dockerignore \
            52-sifto-data-mount.conf test-postgres-data-mount-systemd.py; do
    cp "$fixture/$file" "$scratch/context/bootstrap/$file"
done
printf '%s\n' synthetic-operator-canary > "$scratch/context/bootstrap/postgres_data_mount.py"
printf '%s\n' synthetic-nested-secret-canary > "$scratch/context/bootstrap/private/secret.env"
printf '%s\n' synthetic-root-secret-canary > "$scratch/context/private.pem"
docker build --network none --target context-test \
    --output "type=local,dest=$scratch/export" \
    -f "$scratch/context/bootstrap/Dockerfile.postgres-data-mount" "$scratch/context"
actual=$(cd "$scratch/export" && find . ! -type d -print | LC_ALL=C sort)
expected=$(printf '%s\n' ./bootstrap/52-sifto-data-mount.conf ./bootstrap/test-postgres-data-mount-systemd.py)
test "$actual" = "$expected"
for file in 52-sifto-data-mount.conf test-postgres-data-mount-systemd.py; do
    test ! -L "$scratch/export/bootstrap/$file"
    cmp "$fixture/$file" "$scratch/export/bootstrap/$file"
done
test ! -e "$scratch/export/bootstrap/postgres_data_mount.py"
test ! -e "$scratch/export/bootstrap/private"
test ! -e "$scratch/export/private.pem"
printf '%s\n' 'Docker context export contains exactly the two public fixture files; operator and secret canaries excluded'
