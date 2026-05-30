SHELL := /usr/bin/env bash

MAKEFILE_DIR := $(abspath $(dir $(lastword $(MAKEFILE_LIST))))
GO_DIR := $(MAKEFILE_DIR)/go
VERSION := $(shell tr -d '[:space:]' < "$(MAKEFILE_DIR)/VERSION")
PREFIX ?= $(HOME)/.local
DIST_DIR ?= $(MAKEFILE_DIR)/dist

.PHONY: install uninstall build lint typecheck test smoke check release-check \
	go-build go-test go-vet go-release release-archives check-release-archives package-smoke

install: go-build
	mkdir -p "$(PREFIX)/bin"
	install -m 0755 "$(GO_DIR)/bin/striatum" "$(PREFIX)/bin/striatum"
	install -m 0755 "$(GO_DIR)/bin/striatumd" "$(PREFIX)/bin/striatumd"
	install -m 0755 "$(GO_DIR)/bin/striatum-supervisor-helper" "$(PREFIX)/bin/striatum-supervisor-helper"
	"$(PREFIX)/bin/striatum" daemon install --no-start
	"$(PREFIX)/bin/striatum" skills install
	@echo "==> attempting daemon start + health check (best effort)"
	@echo "    (a Postgres DSN must be configured before the daemon can bind;"
	@echo "     see ~/.config/striatum/daemon.toml — set postgres_url)"
	-"$(PREFIX)/bin/striatum" daemon install
	-"$(PREFIX)/bin/striatum" daemon status
	@echo "==> if 'doctor' is not ok, set postgres_url in ~/.config/striatum/daemon.toml"
	@echo "    then run: striatum daemon install && striatum doctor"

uninstall:
	-"$(PREFIX)/bin/striatum" daemon uninstall
	rm -f "$(PREFIX)/bin/striatum" "$(PREFIX)/bin/striatumd" "$(PREFIX)/bin/striatum-supervisor-helper"
	@echo "Removed binaries and systemd user unit. Left ~/.config/striatum/daemon.toml and data intact."

build: go-build

lint: go-vet

typecheck: go-test

test: go-test

smoke:
	"$(MAKEFILE_DIR)/scripts/go_fresh_clone_smoke.sh"

check: lint test

release-check: check release-archives check-release-archives package-smoke smoke

go-build:
	$(MAKE) -C "$(GO_DIR)" build

go-test:
	$(MAKE) -C "$(GO_DIR)" test

go-vet:
	$(MAKE) -C "$(GO_DIR)" lint

go-release:
	$(MAKE) -C "$(GO_DIR)" release

release-archives:
	"$(MAKEFILE_DIR)/scripts/build_go_release_archives.sh" --dist "$(DIST_DIR)"

check-release-archives:
	"$(MAKEFILE_DIR)/scripts/check_go_release_archives.sh" "$(DIST_DIR)"

package-smoke:
	"$(MAKEFILE_DIR)/scripts/go_package_smoke.sh"
