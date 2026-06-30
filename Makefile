SHELL := /bin/bash

DOCKER_COMPOSE := $(shell if command -v docker-compose >/dev/null 2>&1; then printf 'docker-compose'; else printf 'docker compose'; fi)
TOOLS := $(DOCKER_COMPOSE) run --rm tools

.DEFAULT_GOAL := help

.PHONY: help deps format test test-e2e vet check run temporal shell clean
help:
	@printf "\nPaveBank Fees API\n"
	@printf "  make deps       Install Go module dependencies\n"
	@printf "  make format     Format Go files\n"
	@printf "  make test       Run unit and workflow tests\n"
	@printf "  make test-e2e   Run Testcontainers end-to-end tests\n"
	@printf "  make vet        Run go vet\n"
	@printf "  make check      Run the full verification suite\n"
	@printf "  make temporal   Start local Temporal dev server\n"
	@printf "  make run        Run Encore API locally\n"
	@printf "  make shell      Open the Dockerized toolbox\n\n"

deps:
	go mod tidy

format:
	gofmt -w $$(find . -name '*.go' -not -path './.git/*')

test:
	go test ./...

test-e2e:
	go test -tags=e2e ./...

vet:
	go vet ./...

check: test test-e2e vet

run:
	$(TOOLS) encore run --listen=0.0.0.0:4000

temporal:
	$(DOCKER_COMPOSE) up temporal

shell:
	$(TOOLS) bash

clean:
	$(DOCKER_COMPOSE) down --remove-orphans

