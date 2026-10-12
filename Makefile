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

.PHONY: maintenance-backup-test
maintenance-backup-test: maintenance-backup-unit-test
	docker compose --env-file /dev/null --project-name sifto-maintenance-backup-fixtures -f infra/maintenance/backup/compose.yml --profile test run --build --rm -T proof-test
	PROOF_PG_VERSION=15.19 docker compose --env-file /dev/null --project-name sifto-maintenance-backup-fixtures-pg15 -f infra/maintenance/backup/compose.yml --profile test run --build --rm -T proof-test
	docker compose --env-file /dev/null --project-name sifto-maintenance-backup-fixtures -f infra/maintenance/backup/compose.yml --profile test run --rm -T script-test
	docker compose --env-file /dev/null --project-name sifto-maintenance-backup-fixtures -f infra/maintenance/backup/compose.yml --profile test run --rm -T lock-test

# Public PR checks use an isolated project with no .env, host ports, or secrets.
DEPENDENCY_COMPOSE := docker compose --env-file /dev/null -f infra/dependency-ci/compose.yml
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
dependency-api:
	$(DEPENDENCY_COMPOSE) run --build --rm -T api

dependency-worker:
	$(DEPENDENCY_COMPOSE) run --build --rm -T --no-deps worker

dependency-web:
	$(DEPENDENCY_COMPOSE) run --build --rm -T --no-deps web

dependency-contract: dependency-python-locks-mounts-test
	$(DEPENDENCY_COMPOSE) run --build --rm -T --no-deps contract
	$(DEPENDENCY_COMPOSE) run --rm -T --no-deps contract python -B /repository/infra/dependency-ci/test_updates.py
	sh -n infra/dependency-ci/init-db.sh

dependency-renovate-validate:
	$(DEPENDENCY_COMPOSE) run --rm -T --no-deps renovate-validator
	$(DEPENDENCY_COMPOSE) run --rm -T --no-deps renovate-status-test

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
