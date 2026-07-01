SHELL := /bin/bash

DOCKER_COMPOSE := $(shell if command -v docker-compose >/dev/null 2>&1; then printf 'docker-compose'; else printf 'docker compose'; fi)

.DEFAULT_GOAL := help

.PHONY: help deps format format-all lint test test-e2e vet check build-tools run temporal shell clean
help:
	@printf "\nGocanto Bank Test\n"
	@printf "  make deps       Install Go module dependencies\n"
	@printf "  make format     Format Go files with go-fmt\n"
	@printf "  make format-all Format all Go files with go-fmt\n"
	@printf "  make lint       Check Go formatting with go-fmt\n"
	@printf "  make test       Run unit and workflow tests\n"
	@printf "  make test-e2e   Run Testcontainers end-to-end tests\n"
	@printf "  make vet        Run go vet\n"
	@printf "  make check      Run the full verification suite\n"
	@printf "  make build-tools Build the Dockerized toolbox\n"
	@printf "  make temporal   Start local Temporal dev server\n"
	@printf "  make run        Run Encore API locally\n"
	@printf "  make shell      Open the Dockerized toolbox\n\n"

deps:
	go mod tidy

format:
	./infra/scripts/go-fmt.sh format .

format-all:
	./infra/scripts/go-fmt.sh format .

lint:
	./infra/scripts/go-fmt.sh check .

test:
	go test ./...

test-e2e:
	go test -tags=e2e -run 'E2E' ./fees ./fees/workflows

vet:
	go vet ./...

check: test test-e2e vet

build-tools:
	GO_VERSION="$$(./infra/scripts/go-version.sh)" $(DOCKER_COMPOSE) build tools

run:
	GO_VERSION="$$(./infra/scripts/go-version.sh)" $(DOCKER_COMPOSE) run --rm tools encore run --listen=0.0.0.0:4000

temporal:
	$(DOCKER_COMPOSE) up temporal

shell:
	GO_VERSION="$$(./infra/scripts/go-version.sh)" $(DOCKER_COMPOSE) run --rm tools bash

clean:
	$(DOCKER_COMPOSE) down --remove-orphans
