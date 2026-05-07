VENV ?= .venv
PYTHON ?= $(VENV)/bin/python

.PHONY: install test smoke

$(PYTHON):
	python3 -m venv $(VENV)

$(VENV)/.installed: pyproject.toml $(PYTHON)
	$(PYTHON) -m pip install -e ".[dev]"
	touch $(VENV)/.installed

install: $(VENV)/.installed

test: $(VENV)/.installed
	$(PYTHON) -m pytest

smoke:
	scripts/fresh_clone_smoke.sh

legacy-install:
	$(PYTHON) -m pip install -e ".[dev]"
