VENV ?= .venv
PYTHON ?= $(VENV)/bin/python

# HARNESS-002 fix: resolve the install path explicitly so
# ``make install`` invoked from any cwd installs *this* Makefile's
# directory in editable mode. The previous "pip install -e ." was
# cwd-dependent and silently pinned the install to the wrong tree
# when invoked from a Claude Code worktree (or any other cwd).
MAKEFILE_DIR := $(abspath $(dir $(lastword $(MAKEFILE_LIST))))

.PHONY: install lint typecheck pg-test test-multi-repo metadata-check package-smoke smoke check release-check ui-install ui-update-lock ui-audit ui-build ui-dev ui-test ui-bundle-hash ui-check-bundle ui-verify-bundle daemon-go-build daemon-go-test daemon-go-lint

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
	npm ci --prefix "$(MAKEFILE_DIR)/src/striatum/web/frontend"

ui-update-lock:
	npm install --prefix "$(MAKEFILE_DIR)/src/striatum/web/frontend"

ui-audit:
	npm audit --prefix "$(MAKEFILE_DIR)/src/striatum/web/frontend" --audit-level=high

ui-build:
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
	python3 -c 'import sys; from pathlib import Path; root=Path("$(MAKEFILE_DIR)")/"src/striatum/web/static/build"; entries=["island-shared.js","island-tree-browser.js","island-workflow-chooser.js","island-workflow-graph-editor.js","island-code-viewer.js"]; bad=[]; SENTINEL="Striatum frontend island placeholder loaded"; MIN_BYTES=1024; \
	[bad.append(f"missing {n}") for n in entries if not (root/n).is_file()]; \
	[bad.append(f"{n} contains placeholder sentinel") for n in entries if (root/n).is_file() and SENTINEL in (root/n).read_text(encoding="utf-8", errors="ignore")]; \
	[bad.append(f"{p.name} contains placeholder sentinel") for p in root.glob("island-shared-*.js") if SENTINEL in p.read_text(encoding="utf-8", errors="ignore")]; \
	[bad.append(f"{n} below {MIN_BYTES} bytes ({(root/n).stat().st_size} B)") for n in entries if (root/n).is_file() and (root/n).stat().st_size < MIN_BYTES and not any(p.stat().st_size >= MIN_BYTES for p in root.glob("island-shared-*.js"))]; \
	sys.exit("ui-verify-bundle: " + "; ".join(bad)) if bad else print("ui-verify-bundle: ok")'

ui-check-bundle: ui-build ui-verify-bundle
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
