SHELL := /usr/bin/env bash

MAKEFILE_DIR := $(abspath $(dir $(lastword $(MAKEFILE_LIST))))
GO_DIR := $(MAKEFILE_DIR)/go
VERSION := $(shell tr -d '[:space:]' < "$(MAKEFILE_DIR)/VERSION")
PREFIX ?= $(HOME)/.local
DIST_DIR ?= $(MAKEFILE_DIR)/dist

.PHONY: install uninstall build lint typecheck test smoke check release-check \
	go-build go-test go-vet go-release release-archives check-release-archives package-smoke \
	ui-install ui-update-lock ui-audit ui-clean ui-build ui-dev ui-test ui-bundle-hash \
	ui-verify-bundle ui-bundle-size ui-check-bundle \
	python-trace-report python-trace-guardrail

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

python-trace-report:
	"$(MAKEFILE_DIR)/scripts/python_trace_guardrail.sh" --report

python-trace-guardrail:
	"$(MAKEFILE_DIR)/scripts/python_trace_guardrail.sh" --strict

ui-install:
	npm ci --prefix "$(MAKEFILE_DIR)/src/striatum/web/frontend"

ui-update-lock:
	npm install --prefix "$(MAKEFILE_DIR)/src/striatum/web/frontend"

ui-audit:
	npm audit --prefix "$(MAKEFILE_DIR)/src/striatum/web/frontend" --audit-level=high

ui-clean:
	rm -rf "$(MAKEFILE_DIR)/src/striatum/web/static/build"
	mkdir -p "$(MAKEFILE_DIR)/src/striatum/web/static/build"

ui-build: ui-clean
	npm run build --prefix "$(MAKEFILE_DIR)/src/striatum/web/frontend"
	$(MAKE) ui-bundle-hash

ui-dev:
	npm run dev --prefix "$(MAKEFILE_DIR)/src/striatum/web/frontend"

ui-test:
	npm run test --prefix "$(MAKEFILE_DIR)/src/striatum/web/frontend"

ui-bundle-hash:
	root="$(MAKEFILE_DIR)/src/striatum/web/static/build"; \
	tmp="$$root/manifest.sha256.tmp"; \
	: > "$$tmp"; \
	while IFS= read -r file; do \
	  rel="$${file#$$root/}"; \
	  if command -v shasum >/dev/null 2>&1; then \
	    hash="$$(shasum -a 256 "$$file" | awk '{print $$1}')"; \
	  else \
	    hash="$$(sha256sum "$$file" | awk '{print $$1}')"; \
	  fi; \
	  printf '%s  %s\n' "$$hash" "$$rel" >> "$$tmp"; \
	done < <(find "$$root" -type f ! -name manifest.sha256 ! -name manifest.sha256.tmp | sort); \
	mv "$$tmp" "$$root/manifest.sha256"

ui-verify-bundle:
	root="$(MAKEFILE_DIR)/src/striatum/web/static/build"; \
	sentinel="Striatum frontend island placeholder loaded"; \
	min_bytes=1024; \
	for entry in island-shared.js island-tree-browser.js island-code-viewer.js; do \
	  test -f "$$root/$$entry" || { echo "ui-verify-bundle: missing $$entry" >&2; exit 1; }; \
	  ! grep -q "$$sentinel" "$$root/$$entry" || { echo "ui-verify-bundle: $$entry contains placeholder sentinel" >&2; exit 1; }; \
	done; \
	for entry in island-tree-browser.js island-code-viewer.js; do \
	  size="$$(wc -c < "$$root/$$entry")"; \
	  test "$$size" -ge "$$min_bytes" || { echo "ui-verify-bundle: $$entry below $$min_bytes bytes ($$size B)" >&2; exit 1; }; \
	done; \
	echo "ui-verify-bundle: ok"

ui-bundle-size:
	STRIATUM_UI_BUNDLE_ROOT="$(MAKEFILE_DIR)/src/striatum/web/static/build" \
	node -e 'const fs=require("fs"),p=require("path"); const root=process.env.STRIATUM_UI_BUNDLE_ROOT; const maxBytes=Number(process.env.STRIATUM_UI_BUNDLE_MAX_BYTES||12000000); const maxFiles=Number(process.env.STRIATUM_UI_BUNDLE_MAX_FILES||32); const maxShared=Number(process.env.STRIATUM_UI_BUNDLE_MAX_SHARED_CHUNKS||4); const ignore=new Set(["manifest.sha256"]); if(!fs.existsSync(root)||!fs.statSync(root).isDirectory()){console.error(`ui-bundle-size: missing build directory: $${root}`);process.exit(1);} const walk=(d)=>fs.readdirSync(d,{withFileTypes:true}).flatMap(e=>{const f=p.join(d,e.name); return e.isDirectory()?walk(f):[f];}); const files=walk(root).filter(f=>!ignore.has(p.basename(f))); const total=files.reduce((s,f)=>s+fs.statSync(f).size,0); const shared=files.filter(f=>/^island-shared-.*\.js$$/.test(p.basename(f))); if(files.length>maxFiles){console.error(`ui-bundle-size: $${files.length} files exceeds limit $${maxFiles}; check for stale generated chunks before raising STRIATUM_UI_BUNDLE_MAX_FILES`);process.exit(1);} if(shared.length>maxShared){console.error(`ui-bundle-size: $${shared.length} island-shared chunks exceeds limit $${maxShared}; group dynamic imports before raising STRIATUM_UI_BUNDLE_MAX_SHARED_CHUNKS`);process.exit(1);} if(total>maxBytes){console.error(`ui-bundle-size: $${total} bytes exceeds limit $${maxBytes}; raise STRIATUM_UI_BUNDLE_MAX_BYTES only with an explicit review note`);process.exit(1);} console.log(`ui-bundle-size: $${total} bytes <= $${maxBytes}; $${files.length} files <= $${maxFiles}; $${shared.length} shared chunks <= $${maxShared}`);'

ui-check-bundle: ui-build ui-verify-bundle ui-bundle-size
	git -C "$(MAKEFILE_DIR)" diff --exit-code -- src/striatum/web/static/build
