FROM postgres:16.13-bookworm
COPY db/migrations/ /migrations/
COPY infra/dependency-ci/init-db.sh /docker-entrypoint-initdb.d/10-migrations.sh
