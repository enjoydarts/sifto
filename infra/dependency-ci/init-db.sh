#!/bin/sh
set -eu

# The disposable database starts empty; apply only forward migrations in order.
for migration in /migrations/*.up.sql; do
  psql --username "$POSTGRES_USER" --dbname "$POSTGRES_DB" \
    --set ON_ERROR_STOP=1 --file "$migration"
done
