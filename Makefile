VENV ?= .venv
PYTHON ?= $(VENV)/bin/python

.PHONY: install lint typecheck test metadata-check package-smoke smoke check release-check

$(PYTHON):
	python3 -m venv $(VENV)

$(VENV)/.installed: pyproject.toml $(PYTHON)
	$(PYTHON) -m pip install -e ".[dev]"
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
	$(PYTHON) -m pip install -e ".[dev]"
