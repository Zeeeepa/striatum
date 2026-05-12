VENV ?= .venv
PYTHON ?= $(VENV)/bin/python

# HARNESS-002 fix: resolve the install path explicitly so
# ``make install`` invoked from any cwd installs *this* Makefile's
# directory in editable mode. The previous "pip install -e ." was
# cwd-dependent and silently pinned the install to the wrong tree
# when invoked from a Claude Code worktree (or any other cwd).
MAKEFILE_DIR := $(abspath $(dir $(lastword $(MAKEFILE_LIST))))

.PHONY: install lint typecheck pg-test test-multi-repo metadata-check package-smoke smoke check release-check

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

check: lint typecheck test metadata-check package-smoke

release-check: check smoke

legacy-install:
	$(PYTHON) -m pip install -e "$(MAKEFILE_DIR)[dev]"
