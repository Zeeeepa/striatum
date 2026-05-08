VENV ?= .venv
PYTHON ?= $(VENV)/bin/python

# HARNESS-002 fix: resolve the install path explicitly so
# ``make install`` invoked from any cwd installs *this* Makefile's
# directory in editable mode. The previous "pip install -e ." was
# cwd-dependent and silently pinned the install to the wrong tree
# when invoked from a Claude Code worktree (or any other cwd).
MAKEFILE_DIR := $(abspath $(dir $(lastword $(MAKEFILE_LIST))))

.PHONY: install lint typecheck test metadata-check package-smoke smoke check release-check

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
