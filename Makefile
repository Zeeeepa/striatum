VENV ?= .venv
PYTHON ?= $(VENV)/bin/python
CORE ?= go

# HARNESS-002 fix: resolve the install path explicitly so
# ``make install`` invoked from any cwd installs *this* Makefile's
# directory in editable mode. The previous "pip install -e ." was
# cwd-dependent and silently pinned the install to the wrong tree
# when invoked from a Claude Code worktree (or any other cwd).
MAKEFILE_DIR := $(abspath $(dir $(lastword $(MAKEFILE_LIST))))

.PHONY: install lint typecheck pg-test test-rfc0043 test-multi-repo metadata-check package-smoke package-wheel-size smoke check release-check ui-install ui-update-lock ui-audit ui-clean ui-build ui-dev ui-test ui-bundle-hash ui-bundle-size ui-check-bundle ui-verify-bundle daemon-go-build daemon-go-test daemon-go-lint daemon-go-helper-check daemon-go-helper-integration daemon-go-conformance daemon-go-install daemon-go-release

$(PYTHON):
	python3 -m venv $(VENV)

$(VENV)/.installed: pyproject.toml $(PYTHON)
	@echo "installing striatum (editable) from $(MAKEFILE_DIR)"
	$(PYTHON) -m pip install -e "$(MAKEFILE_DIR)[dev]"
	touch $(VENV)/.installed

$(VENV)/.installed-daemon-pg: pyproject.toml $(VENV)/.installed
	@echo "installing striatum daemon-pg extra into $(VENV)"
	$(PYTHON) -m pip install -e "$(MAKEFILE_DIR)[daemon-pg]"
	touch $(VENV)/.installed-daemon-pg

install: $(VENV)/.installed

lint: $(VENV)/.installed
	$(PYTHON) -m ruff check .

typecheck: $(VENV)/.installed
	$(PYTHON) -m mypy

test: $(VENV)/.installed
	$(PYTHON) -m pytest

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
	python3 -c 'from pathlib import Path; import hashlib; root=Path("$(MAKEFILE_DIR)")/"src/striatum/web/static/build"; files=sorted(p for p in root.rglob("*") if p.is_file() and p.name not in {"__init__.py","manifest.sha256"}); (root/"manifest.sha256").write_text("".join(f"{hashlib.sha256(p.read_bytes()).hexdigest()}  {p.relative_to(root).as_posix()}\n" for p in files), encoding="utf-8")'

# RFC 0038 V1.5 F1: assert real Vite output landed under src/striatum/web/static/build/.
# Rejects committed placeholder bundles (V1's `console.info(...)` stubs) and any
# stable island entry that is suspiciously small. Runs after `make ui-build` so
# build drift falls out as a git diff in `ui-check-bundle`.
ui-verify-bundle:
	python3 -c 'import sys; from pathlib import Path; root=Path("$(MAKEFILE_DIR)")/"src/striatum/web/static/build"; entries=["island-shared.js","island-tree-browser.js","island-code-viewer.js"]; sized_entries=[n for n in entries if n != "island-shared.js"]; bad=[]; SENTINEL="Striatum frontend island placeholder loaded"; MIN_BYTES=1024; \
	[bad.append(f"missing {n}") for n in entries if not (root/n).is_file()]; \
	[bad.append(f"{n} contains placeholder sentinel") for n in entries if (root/n).is_file() and SENTINEL in (root/n).read_text(encoding="utf-8", errors="ignore")]; \
	[bad.append(f"{p.name} contains placeholder sentinel") for p in root.glob("island-shared-*.js") if SENTINEL in p.read_text(encoding="utf-8", errors="ignore")]; \
	[bad.append(f"{n} below {MIN_BYTES} bytes ({(root/n).stat().st_size} B)") for n in sized_entries if (root/n).is_file() and (root/n).stat().st_size < MIN_BYTES]; \
	sys.exit("ui-verify-bundle: " + "; ".join(bad)) if bad else print("ui-verify-bundle: ok")'

ui-bundle-size:
	python3 "$(MAKEFILE_DIR)/scripts/check_ui_bundle_size.py" \
	  --root "$(MAKEFILE_DIR)/src/striatum/web/static/build"

ui-check-bundle: ui-build ui-verify-bundle ui-bundle-size
	git -C "$(MAKEFILE_DIR)" diff --exit-code -- src/striatum/web/static/build

daemon-go-build:
	$(MAKE) -C "$(MAKEFILE_DIR)/go" build

daemon-go-test:
	$(MAKE) -C "$(MAKEFILE_DIR)/go" test

daemon-go-lint:
	$(MAKE) -C "$(MAKEFILE_DIR)/go" lint

daemon-go-helper-check: $(VENV)/.installed
	$(MAKE) -C "$(MAKEFILE_DIR)/go" helper-check
	$(PYTHON) -m pytest tests/architecture/test_go_helper_boundary.py -q

daemon-go-helper-integration: $(VENV)/.installed-daemon-pg
	$(MAKE) -C "$(MAKEFILE_DIR)/go" build-helper
	STRIATUM_MULTI_REPO_REQUIRE_PG=1 \
	$(PYTHON) -m pytest -q tests/daemon_pg/handlers/test_supervision.py -k 'real_go_pty_helper'

daemon-go-conformance: $(VENV)/.installed-daemon-pg daemon-go-build daemon-go-test
	$(MAKE) test-multi-repo CORE=go

# RFC 0039 Phase 2 Step 6: copy the host-platform Go binary into the in-tree
# wheel package-data path so a local `pip install -e .` install picks it up
# via striatum._daemongo.find_binary().
daemon-go-install: daemon-go-build
	mkdir -p "$(MAKEFILE_DIR)/src/striatum/_daemongo/binaries"
	cp "$(MAKEFILE_DIR)/go/bin/striatumd" \
	   "$(MAKEFILE_DIR)/src/striatum/_daemongo/binaries/striatumd-$$(python3 -c 'import sys, platform; print(f"{sys.platform}-{platform.machine()}")')"

# RFC 0039 Phase 2 Step 6: cross-compile all four target platforms and stage
# them under the wheel package-data tree. The release pipeline runs this
# before `python -m build`.
daemon-go-release:
	$(MAKE) -C "$(MAKEFILE_DIR)/go" release
	mkdir -p "$(MAKEFILE_DIR)/src/striatum/_daemongo/binaries"
	cp "$(MAKEFILE_DIR)/go/bin/striatumd-linux-amd64"  "$(MAKEFILE_DIR)/src/striatum/_daemongo/binaries/striatumd-linux-x86_64"
	cp "$(MAKEFILE_DIR)/go/bin/striatumd-linux-arm64"  "$(MAKEFILE_DIR)/src/striatum/_daemongo/binaries/striatumd-linux-aarch64"
	cp "$(MAKEFILE_DIR)/go/bin/striatumd-darwin-amd64" "$(MAKEFILE_DIR)/src/striatum/_daemongo/binaries/striatumd-darwin-x86_64"
	cp "$(MAKEFILE_DIR)/go/bin/striatumd-darwin-arm64" "$(MAKEFILE_DIR)/src/striatum/_daemongo/binaries/striatumd-darwin-arm64"

pg-test: $(VENV)/.installed-daemon-pg
	$(PYTHON) -m pytest tests/test_daemon_pg.py -q

test-rfc0043: $(VENV)/.installed-daemon-pg
	STRIATUM_MULTI_REPO_DAEMON_CORE=$(CORE) \
	STRIATUM_MULTI_REPO_REQUIRE_PG=1 \
	$(PYTHON) -m pytest -q \
		tests/cli/test_daemon_doctor_without_daemon.py \
		tests/cli/test_no_daemon_retired.py \
		tests/cli/test_parser_help.py \
		tests/daemon_pg/test_repo_local_migration.py \
		tests/daemon_pg/test_repo_local_migration_crash_resume.py \
		tests/daemon_pg/test_repo_local_migration_locking.py \
		tests/daemon_rpc/test_registry_rfc0043_coverage.py \
		tests/exit_codes/test_rfc0043_refusals.py \
		tests/exit_codes/test_rfc0043_split_brain.py \
		tests/architecture/test_authority_guardrails.py::test_production_daemon_required_commands_refuse_before_sqlite_connect

test-multi-repo: $(VENV)/.installed-daemon-pg
	STRIATUM_MULTI_REPO_DAEMON_CORE=$(CORE) \
	STRIATUM_MULTI_REPO_REQUIRE_PG=1 \
	$(PYTHON) -m pytest -m multi_repo \
		tests/test_multi_repo_harness.py \
		tests/test_cross_repo_prepare_e2e.py \
		tests/test_cross_repo_lifecycle_e2e.py \
		tests/test_cross_repo_crash_recovery_e2e.py \
		tests/test_mcp_capability_scope_e2e.py \
		tests/test_per_repo_write_scope_e2e.py \
		tests/test_daemon_go_smoke.py \
		tests/test_daemon_go_audit.py \
		tests/test_daemon_go_mutations.py \
		tests/test_daemon_go_supervisor.py

metadata-check: $(VENV)/.installed
	$(PYTHON) scripts/release_metadata_check.py

package-smoke: $(VENV)/.installed
	PYTHON_FOR_BUILD=$(PYTHON) scripts/package_smoke.sh

package-wheel-size: $(VENV)/.installed
	tmp="$$(mktemp -d)"; \
	mkdir -p "$$tmp/source"; \
	trap 'rm -rf "$$tmp"' EXIT; \
	tar --exclude .git --exclude .striatum --exclude .venv --exclude .pytest_cache --exclude .mypy_cache --exclude .ruff_cache --exclude __pycache__ --exclude build --exclude dist -C "$(MAKEFILE_DIR)" -cf - . | tar -C "$$tmp/source" -xf -; \
	$(PYTHON) -m build --wheel --outdir "$$tmp/dist" "$$tmp/source" >/dev/null; \
	$(PYTHON) "$(MAKEFILE_DIR)/scripts/check_wheel_size.py" --wheel "$$tmp/dist"

smoke:
	scripts/fresh_clone_smoke.sh

check: lint typecheck test ui-check-bundle ui-test metadata-check package-wheel-size package-smoke

release-check: check smoke

legacy-install:
	$(PYTHON) -m pip install -e "$(MAKEFILE_DIR)[dev]"
