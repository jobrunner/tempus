# deploy/dev/dev.mk — per-feature isolated dev environments.
# Included at the bottom of the root Makefile.
# Usage: make dev-up | make dev NAME=<slug> | make dev-destroy NAME=<slug>

DEV_SVC        := tempus
DEV_NET        := tempus-dev
DEV_INFRA_FILE := deploy/dev/docker-compose.infra.yaml
DEV_FILE       := deploy/dev/docker-compose.dev.yaml

_dev-name:
	$(eval NAME ?= feat-$(shell date +%s))
	$(eval COMPOSE_PROJECT_NAME := $(DEV_SVC)-dev-$(NAME))

.PHONY: dev-up dev-obs dev-dns-setup dev dev-new dev-list dev-attach \
        dev-logs dev-destroy dev-doctor

dev-up: ## One-time: create shared network, start Traefik + Dozzle
	docker network create $(DEV_NET) 2>/dev/null || true
	docker volume create tempus-go-mod-cache 2>/dev/null || true
	docker compose -f $(DEV_INFRA_FILE) up -d

dev-obs: ## Start optional shared observability stack
	@echo "Observability stack not configured — add deploy/dev/docker-compose.obs.yaml to enable."

dev-dns-setup: ## One-time: dnsmasq so *.tempus.local resolves (macOS)
	@echo "127.0.0.1 *.tempus.local — add to /etc/hosts or configure dnsmasq."

dev: _dev-name ## Create/activate a feature env (NAME=<slug>)
	docker volume create tempus-go-build-cache-$(NAME) 2>/dev/null || true
	NAME=$(NAME) COMPOSE_PROJECT_NAME=$(COMPOSE_PROJECT_NAME) \
	  docker compose -f $(DEV_FILE) up -d --build

dev-new: _dev-name dev ## Alias for dev

dev-list: ## Show running feature envs
	docker ps --filter "label=com.docker.compose.project.working_dir" \
	          --filter "name=tempus-dev-" \
	          --format "table {{.Names}}\t{{.Status}}"

dev-attach: _dev-name ## Interactive shell in the feature container (NAME=<slug>)
	NAME=$(NAME) COMPOSE_PROJECT_NAME=$(COMPOSE_PROJECT_NAME) \
	  docker compose -f $(DEV_FILE) exec tempus sh

dev-logs: _dev-name ## Tail service logs (NAME=<slug>)
	NAME=$(NAME) COMPOSE_PROJECT_NAME=$(COMPOSE_PROJECT_NAME) \
	  docker compose -f $(DEV_FILE) logs -f tempus

dev-destroy: _dev-name ## Tear down feature container + volumes (NAME=<slug>)
	NAME=$(NAME) COMPOSE_PROJECT_NAME=$(COMPOSE_PROJECT_NAME) \
	  docker compose -f $(DEV_FILE) down -v
	docker volume rm tempus-go-build-cache-$(NAME) 2>/dev/null || true

dev-doctor: ## Health-check DNS/network/Traefik
	@docker network inspect $(DEV_NET) >/dev/null 2>&1 && echo "network $(DEV_NET): OK" || echo "network $(DEV_NET): MISSING — run make dev-up"
	@docker ps --filter "name=tempus-dev-infra-traefik" --format "{{.Names}}" | grep -q traefik && echo "traefik: OK" || echo "traefik: NOT RUNNING"
