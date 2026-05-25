---
schema_version: "striatum.synthesis.v1"
artifact_kind: "synthesis"
inputs: ["README.md", "docs/USING_STRIATUM.md", "docs/GETTING_STARTED.md", "docs/RELEASING.md", "docs/POSTGRES_TRANSITION.md", "docs/HOW_TO_HUMAN.md"]
---

# Docs Install Release Handoff
author: operator [self-declared: install-docs-owner-codex-gpt-5-001]

## Landed

- Rewrote README quick start and source install sections to use Go release
  archives or `make install`.
- Rewrote day-zero and getting-started prerequisites away from PyPI and
  Python.
- Replaced `docs/RELEASING.md` with Go archive and checksum release policy.
- Updated PostgreSQL transition prerequisites to require installed Striatum Go
  binaries instead of a Python install.
- Updated the human manual runner example to use `striatum` on `PATH`.

## Remaining Blockers

Some active docs still mention Python as historical daemon-transition context
or in old manual examples. The final docs/guardrail deletion workflow should
perform the broad active-doc zero-reference sweep after the Go CLI/web parity
gates decide the retained command surface.
