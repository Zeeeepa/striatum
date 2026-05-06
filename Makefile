VENV ?= .venv
PYTHON ?= $(VENV)/bin/python

.PHONY: install test

$(PYTHON):
	python3 -m venv $(VENV)

$(VENV)/.installed: pyproject.toml $(PYTHON)
	$(PYTHON) -m pip install -e ".[dev]"
	touch $(VENV)/.installed

install: $(VENV)/.installed

test: $(VENV)/.installed
	$(PYTHON) -m pytest

legacy-install:
	$(PYTHON) -m pip install -e ".[dev]"
