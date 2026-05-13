VENV ?= .venv
PYTHON ?= $(VENV)/bin/python

# HARNESS-002 fix: resolve the install path explicitly so
# ``make install`` invoked from any cwd installs *this* Makefile's
# directory in editable mode. The previous "pip install -e ." was
# cwd-dependent and silently pinned the install to the wrong tree
# when invoked from a Claude Code worktree (or any other cwd).
MAKEFILE_DIR := $(abspath $(dir $(lastword $(MAKEFILE_LIST))))

.PHONY: install lint typecheck pg-test test-multi-repo metadata-check package-smoke smoke check release-check ui-install ui-build ui-dev ui-test ui-bundle-hash ui-check-bundle daemon-go-build daemon-go-test daemon-go-lint

$(PYTHON):
	python3 -m venv $(VENV)

$(VENV)/.installed: pyproject.toml $(PYTHON)
	@echo "installing striatum (editable) from $(MAKEFILE_DIR)"
	$(PYTHON) -m pip install -e "$(MAKEFILE_DIR)[dev]"
	touch $(VENV)/.installed

install: $(VENV)/.installed

lint: $(VENV)/.installed
	$(PYTHON) -m ruff check .

typecheck: $(VENV)/.installed
	$(PYTHON) -m mypy

test: $(VENV)/.installed
	$(PYTHON) -m pytest

ui-install:
	npm install --prefix "$(MAKEFILE_DIR)/src/striatum/web/frontend"

ui-build:
	npm run build --prefix "$(MAKEFILE_DIR)/src/striatum/web/frontend"
	$(MAKE) ui-bundle-hash

ui-dev:
	npm run dev --prefix "$(MAKEFILE_DIR)/src/striatum/web/frontend"

ui-test:
	npm run test --prefix "$(MAKEFILE_DIR)/src/striatum/web/frontend"

ui-bundle-hash:
	python3 -c 'from pathlib import Path; import hashlib; root=Path("$(MAKEFILE_DIR)")/"src/striatum/web/static/build"; files=sorted(p for p in root.rglob("*") if p.is_file() and p.name not in {"__init__.py","manifest.sha256"}); (root/"manifest.sha256").write_text("".join(f"{hashlib.sha256(p.read_bytes()).hexdigest()}  {p.relative_to(root).as_posix()}\n" for p in files), encoding="utf-8")'

ui-check-bundle: ui-build
	git -C "$(MAKEFILE_DIR)" diff --exit-code -- src/striatum/web/static/build

daemon-go-build:
	$(MAKE) -C "$(MAKEFILE_DIR)/go" build

daemon-go-test:
	$(MAKE) -C "$(MAKEFILE_DIR)/go" test

daemon-go-lint:
	$(MAKE) -C "$(MAKEFILE_DIR)/go" lint

pg-test: $(VENV)/.installed
	$(PYTHON) -m pytest tests/test_daemon_pg.py -q

test-multi-repo: $(VENV)/.installed
	$(PYTHON) -m pytest -m multi_repo \
		tests/test_multi_repo_harness.py \
		tests/test_cross_repo_prepare_e2e.py \
		tests/test_cross_repo_lifecycle_e2e.py \
		tests/test_cross_repo_crash_recovery_e2e.py \
		tests/test_mcp_capability_scope_e2e.py \
		tests/test_per_repo_write_scope_e2e.py

metadata-check: $(VENV)/.installed
	$(PYTHON) scripts/release_metadata_check.py

package-smoke: $(VENV)/.installed
	PYTHON_FOR_BUILD=$(PYTHON) scripts/package_smoke.sh

smoke:
	scripts/fresh_clone_smoke.sh

check: lint typecheck test ui-check-bundle ui-test metadata-check package-smoke

release-check: check smoke

legacy-install:
	$(PYTHON) -m pip install -e "$(MAKEFILE_DIR)[dev]"
