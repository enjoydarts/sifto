SHELL := /bin/sh

COMPOSE := docker compose
LOCAL_MIGRATE_DB ?= postgres://sifto:sifto@localhost:5432/sifto?sslmode=disable
GOFMT_FILES := $(shell find api -type f -name '*.go')

.PHONY: help
.PHONY: build build-api build-web build-worker build-audio-concat-local up up-core down restart ps logs logs-api logs-worker logs-web logs-db logs-audio-concat-local
.PHONY: web-build web-lint api-shell web-shell worker-shell psql
.PHONY: migrate-up migrate-down migrate-version
.PHONY: fmt-go fmt-go-check check-worker test-worker check-worker-full check check-fast check-web

help:
	@printf '%s\n' \
	  'Common targets:' \
	  '  make up            # Start core services (postgres, api, worker, inngest, web)' \
	  '  make down          # Stop all services' \
	  '  make build         # Build api/web/worker images' \
	  '  make build-audio-concat-local # Build local audio concat image' \
	  '  make ps            # Show compose status' \
	  '  make logs-api      # Tail API logs' \
	  '  make web-lint      # Run ESLint in web container' \
	  '  make web-build     # Run Next.js production build in web container' \
	  '  make check-worker  # Python syntax check for worker app' \
	  '  make test-worker   # Run pytest in worker container' \
	  '  make check-worker-full # Syntax check + pytest for worker' \
	  '  make check-fast    # Fast checks (gofmt + worker syntax)' \
	  '  make check-web     # Web lint + production build' \
	  '  make check         # PR前チェック一式' \
	  '  make migrate-up    # Apply DB migrations to local Postgres via golang-migrate' \
	  '  make fmt-go        # Run gofmt (api container preferred)' \
	  '  make fmt-go-check  # Check gofmt formatting'
	@printf '%s\n' \
	  '  make image-security IMAGE_COMPONENT=api # Final-image SBOM and CVEs (requires docker-container Buildx builder and 5 GiB free)'

build:
	$(COMPOSE) build api worker web audio-concat-local

build-api:
	$(COMPOSE) build api

build-worker:
	$(COMPOSE) build worker

build-audio-concat-local:
	$(COMPOSE) build audio-concat-local

build-web:
	$(COMPOSE) build web

up:
	$(COMPOSE) up -d postgres api worker inngest web audio-concat-local

up-core:
	$(COMPOSE) up -d postgres api worker inngest audio-concat-local

down:
	$(COMPOSE) down

restart:
	$(COMPOSE) up -d --force-recreate api worker web audio-concat-local

ps:
	$(COMPOSE) ps

logs:
	$(COMPOSE) logs -f --tail=100

logs-api:
	$(COMPOSE) logs -f --tail=100 api

logs-worker:
	$(COMPOSE) logs -f --tail=100 worker

logs-web:
	$(COMPOSE) logs -f --tail=100 web

logs-audio-concat-local:
	$(COMPOSE) logs -f --tail=100 audio-concat-local

logs-db:
	$(COMPOSE) logs -f --tail=100 postgres

web-lint:
	$(COMPOSE) exec -T web npm run lint

web-build:
	$(COMPOSE) exec -T web npm run build

api-shell:
	$(COMPOSE) exec api sh

web-shell:
	$(COMPOSE) exec web sh

worker-shell:
	$(COMPOSE) exec worker sh

psql:
	$(COMPOSE) exec postgres psql -U sifto -d sifto

migrate-up:
	migrate -path db/migrations -database "$(LOCAL_MIGRATE_DB)" up

migrate-down:
	migrate -path db/migrations -database "$(LOCAL_MIGRATE_DB)" down 1

migrate-version:
	migrate -path db/migrations -database "$(LOCAL_MIGRATE_DB)" version

check-worker:
	$(COMPOSE) exec -T worker sh -lc 'python -m py_compile $$(find /app/app -type f -name "*.py")'

check-fast: fmt-go-check check-worker

check-web: web-lint web-build

check: check-fast test-worker check-web

fmt-go:
	@if $(COMPOSE) ps api >/dev/null 2>&1; then \
		$(COMPOSE) exec -T api sh -lc '/usr/local/go/bin/gofmt -w $$(find /app -type f -name "*.go")'; \
	elif command -v gofmt >/dev/null 2>&1; then \
		gofmt -w $(GOFMT_FILES); \
	else \
		echo "gofmt not found. Start the api container (make up) or install Go locally."; \
		exit 1; \
	fi

fmt-go-check:
	@if $(COMPOSE) ps api >/dev/null 2>&1; then \
		out=$$($(COMPOSE) exec -T api sh -lc '/usr/local/go/bin/gofmt -l $$(find /app -type f -name "*.go")'); \
		if [ -n "$$out" ]; then \
			echo "The following files are not gofmt-formatted:"; \
			echo "$$out" | sed 's#^/app/#api/#'; \
			exit 1; \
		fi; \
	elif command -v gofmt >/dev/null 2>&1; then \
		out=$$(gofmt -l $(GOFMT_FILES)); \
		if [ -n "$$out" ]; then \
			echo "The following files are not gofmt-formatted:"; \
			echo "$$out"; \
			exit 1; \
		fi; \
	else \
		echo "gofmt not found. Start the api container (make up) or install Go locally."; \
		exit 1; \
	fi

test-worker:
	$(COMPOSE) exec -T worker sh -lc 'python -m pytest tests/ -v'

check-worker-full: check-worker test-worker

# Isolated final-image checks: no .env, application secrets, or external runtime network.
.PHONY: app-runtime-api app-runtime-worker app-runtime-web
app-runtime-api:
	sh infra/runtime-ci/check.sh api

app-runtime-worker:
	sh infra/runtime-ci/check.sh worker

app-runtime-web:
	sh infra/runtime-ci/check.sh web

.PHONY: maintenance-test maintenance-syntax maintenance-backup-unit-test
maintenance-test: maintenance-backup-unit-test
	docker compose -f infra/maintenance/compose.yml --profile test run --build --rm -T test

maintenance-backup-unit-test:
	docker compose --env-file /dev/null --project-name sifto-maintenance-backup-unit -f infra/maintenance/compose.yml --profile test run --build --rm -T test python -B backup/test_backup_readiness.py

maintenance-syntax:
	docker compose -f infra/maintenance/compose.yml --profile test run --build --rm -T -e ANSIBLE_LOCAL_TEMP=/tmp/ansible -e ANSIBLE_HOME=/tmp/ansible-home test ansible-playbook -i localhost, --syntax-check playbooks/scan.yml playbooks/apply.yml
	docker compose -f infra/maintenance/compose.yml --profile test run --rm -T test python -B -c 'import ast, pathlib; [ast.parse(p.read_text()) for p in pathlib.Path("maintenance").glob("*.py")]'
	sh -n infra/maintenance/run-controller.sh
	sh infra/maintenance/test-controller-local.sh

.PHONY: maintenance-bootstrap-test
maintenance-bootstrap-test:
	docker compose --env-file /dev/null --project-name sifto-maintenance-bootstrap-fixture -f infra/maintenance/bootstrap/compose.yml run --build --rm -T --no-deps test

.PHONY: maintenance-backup-test
maintenance-backup-test: maintenance-backup-unit-test
	docker compose --env-file /dev/null --project-name sifto-maintenance-backup-fixtures -f infra/maintenance/backup/compose.yml --profile test run --build --rm -T proof-test
	PROOF_PG_VERSION=15.19 docker compose --env-file /dev/null --project-name sifto-maintenance-backup-fixtures-pg15 -f infra/maintenance/backup/compose.yml --profile test run --build --rm -T proof-test
	docker compose --env-file /dev/null --project-name sifto-maintenance-backup-fixtures -f infra/maintenance/backup/compose.yml --profile test run --rm -T script-test
	docker compose --env-file /dev/null --project-name sifto-maintenance-backup-fixtures -f infra/maintenance/backup/compose.yml --profile test run --rm -T lock-test

# Verify the fixed production /data/postgres policy with Ubuntu 24's real loader.
# This loads an offline fixture; it never installs a drop-in or starts PostgreSQL.
.PHONY: maintenance-postgres-data-mount-test
maintenance-postgres-data-mount-test:
	sh infra/maintenance/bootstrap/test-postgres-data-mount-context.sh
	docker compose --env-file /dev/null --project-name sifto-postgres-data-mount-fixture -f infra/maintenance/bootstrap/compose.postgres-data-mount.yml run --build --rm -T --no-deps test

# Public PR checks use an isolated project with no .env, host ports, or secrets.
DEPENDENCY_COMPOSE := docker compose --env-file /dev/null -f infra/dependency-ci/compose.yml
export PYTHON_LOCK_UID ?= $(shell id -u)
export PYTHON_LOCK_GID ?= $(shell id -g)
PYTHON_LOCK_COMPOSE := docker compose --env-file /dev/null -f infra/dependency-ci/compose.python-locks.yml
.PHONY: dependency-python-locks dependency-python-locks-check dependency-python-locks-test dependency-python-locks-mounts-test dependency-audio
dependency-python-locks:
	$(PYTHON_LOCK_COMPOSE) run --rm -T generate

dependency-python-locks-check:
	$(PYTHON_LOCK_COMPOSE) run --rm -T locks check

dependency-python-locks-test:
	$(PYTHON_LOCK_COMPOSE) run --rm -T --entrypoint python locks -B /repository/infra/dependency-ci/test_python_locks.py

dependency-python-locks-mounts-test:
	$(PYTHON_LOCK_COMPOSE) --profile generate config --format json | $(PYTHON_LOCK_COMPOSE) run --rm -T --entrypoint python generate -B /repository/infra/dependency-ci/test_python_lock_mounts.py

dependency-audio:
	$(DEPENDENCY_COMPOSE) run --build --rm -T --no-deps audio

.PHONY: dependency-check dependency-api dependency-worker dependency-web dependency-contract dependency-renovate-validate dependency-security dependency-inventory dependency-clean
dependency-api: sentry-privacy-compat-test
	$(DEPENDENCY_COMPOSE) run --build --rm -T api

.PHONY: sentry-privacy-compat-test
sentry-privacy-compat-test:
	docker compose --env-file /dev/null -f infra/dependency-ci/compose.sentry-privacy.yml run --rm -T --no-deps test

dependency-worker:
	$(DEPENDENCY_COMPOSE) run --build --rm -T --no-deps worker

dependency-web:
	$(DEPENDENCY_COMPOSE) run --build --rm -T --no-deps web

dependency-contract: dependency-python-locks-mounts-test
	$(DEPENDENCY_COMPOSE) run --build --rm -T --no-deps contract
	$(DEPENDENCY_COMPOSE) run --rm -T --no-deps contract python -B /repository/infra/dependency-ci/test_updates.py
	$(DEPENDENCY_COMPOSE) run --rm -T --no-deps contract python -B /repository/infra/dependency-ci/test_pr_ci.py
	$(DEPENDENCY_COMPOSE) run --rm -T --no-deps contract python -B /repository/infra/dependency-ci/test_approval_probe.py
	$(DEPENDENCY_COMPOSE) run --rm -T --no-deps contract python -B /repository/infra/dependency-ci/test_followup.py
	sh -n infra/dependency-ci/init-db.sh

dependency-renovate-validate:
	$(DEPENDENCY_COMPOSE) run --rm -T --no-deps renovate-validator
	$(DEPENDENCY_COMPOSE) run --rm -T --no-deps renovate-status-test
	$(MAKE) dependency-renovate-python-test
	$(MAKE) dependency-renovate-npm-test

.PHONY: dependency-renovate-python-test
dependency-renovate-python-test:
	$(DEPENDENCY_COMPOSE) run --rm -T --no-deps renovate-python-command | $(PYTHON_LOCK_COMPOSE) run --rm -T --volume "$(CURDIR)/infra/dependency-ci/uv-renovate.toml:/sifto-python-generation.toml:ro" --entrypoint python locks -B /repository/infra/dependency-ci/test_renovate_python_generation.py

.PHONY: dependency-renovate-npm-test
dependency-renovate-npm-test:
	docker compose --env-file /dev/null -f infra/dependency-ci/compose.renovate-npm.yml run --rm -T --no-deps test

dependency-security:
	$(DEPENDENCY_COMPOSE) run --rm -T --no-deps trivy

dependency-inventory:
	mkdir -p .dependency-inventory/api .dependency-inventory/worker .dependency-inventory/web .dependency-inventory/maintenance .dependency-inventory/audio-concat
	cp api/go.mod api/go.sum .dependency-inventory/api/
	cp worker/requirements.txt .dependency-inventory/worker/
	cp web/package-lock.json .dependency-inventory/web/
	cp infra/maintenance/requirements.txt .dependency-inventory/maintenance/
	cp infra/audio-concat/requirements.txt .dependency-inventory/audio-concat/

dependency-check: dependency-python-locks-check dependency-python-locks-test dependency-contract dependency-renovate-validate dependency-api dependency-worker dependency-web dependency-audio maintenance-test maintenance-syntax

dependency-clean:
	$(DEPENDENCY_COMPOSE) down --volumes --remove-orphans

# Final production Dockerfile stages, exported locally; never push/deploy.
IMAGE_COMPONENT ?= api
IMAGE_SECURITY_COMPOSE := docker compose --env-file /dev/null -f infra/image-security/compose.yml
.PHONY: image-security-build image-security-scan image-security image-security-test
image-security-build:
	sh infra/image-security/build.sh "$(IMAGE_COMPONENT)"

image-security-scan:
	sh infra/image-security/scan.sh "$(IMAGE_COMPONENT)"

image-security: image-security-build
	$(MAKE) image-security-scan IMAGE_COMPONENT="$(IMAGE_COMPONENT)"

image-security-test:
	@mkdir -p .image-security/test .image-security/cache
	@set -eu; export IMAGE_SECURITY_UID=$$(id -u) IMAGE_SECURITY_GID=$$(id -g); \
	  $(IMAGE_SECURITY_COMPOSE) config --format json > .image-security/test/compose.json; \
	  $(IMAGE_SECURITY_COMPOSE) run --rm -T --no-deps verifier sh -ec 'python -B /tool/test_report.py && python -B /tool/test_compose.py && python -B /tool/test_build.py'
	docker run --rm --network none --read-only --security-opt no-new-privileges:true --tmpfs /dev/shm:rw,nosuid,nodev,noexec,size=16m --mount type=bind,source="$(CURDIR)/infra/image-security",target=/tool,readonly python:3.12.15-slim python -B /tool/test_dac.py
	sh -n infra/image-security/build.sh infra/image-security/scan.sh
