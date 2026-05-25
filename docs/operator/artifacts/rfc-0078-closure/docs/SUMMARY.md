---
schema_version: "striatum.synthesis.v1"
artifact_kind: "synthesis"
---

# RFC 0078 Documentation Transition Summary

author: implementer-gemini-003
date: 2026-05-25
status: complete
kind: synthesis

## Overview

This artifact summarizes the work performed to drive `active_python_runtime_guidance` to 0 across the Striatum documentation as part of RFC 0078 (Go-only runtime and Python removal). All current-guidance documents have been updated to reflect the removal of the Python runtime, daemon, and MCP wrapper, and the project's transition to a Go-only production environment.

## Updated Files

The following files were updated to remove Python references and transition to Go-only guidance:

1.  **README.md**: Updated project status table and web UI description to reflect RFC 0078 completion and Go-only status.
2.  **AGENTS.md**: Updated Development and Change Discipline sections. Stated that the project is Go-only and Python has been removed. Removed Python-specific artifacts (`.venv/`, `egg-info`) from the 'Do not commit' list.
3.  **docs/ADOPTER_READING_PATH.md**: Replaced `pip install` instructions with Go binary installation guidance.
4.  **docs/UBIQUITOUS_LANGUAGE.md**: Updated definitions for `binary`, `frontend island`, `frontend toolchain`, and `daemon core` to remove Python wheel, pip, and Python-client references.
5.  **docs/TODO.md**: Updated item 68 (RFC 0078) to reflect that the documentation rewrite is complete and the project is nearing final closure.
6.  **docs/SPEC.md**: Performed a comprehensive review and update of the specification. Removed or updated numerous references to the Python daemon, MCP wrapper, and local API. Stated that Python source and tests are being retired/removed.
7.  **docs/CLI_REFERENCE.md**: Verified and updated as needed (mostly already aligned, but checked for consistency).

## Key Changes

- **Guidance Cutover**: All instructions for installation and use now lead with Go release archives and the `striatum` Go binary.
- **Python Retirement**: Mentions of Python components (daemon, MCP, API) have been changed from "active legacy" or "port in progress" to "retired" or "removed".
- **Target-Repo Preservation**: Guidance correctly distinguishes between the Striatum runtime (Go-only) and the ability to orchestrate target repositories which may use any language (including Python).
- **Web UI**: Updated descriptions to reflect that the UI is Go-served and that the route-parity work is complete.

## Verification

- Grep-search confirms that active production guidance no longer mentions `pip install striatum-orchestrator` or requires a Python virtual environment for Striatum itself.
- All updated documents maintain internal consistency and link to the correct Go-based prerequisites.
- The root `AGENTS.md` now correctly instructs contributors on the Go-only development workflow.

The "docs rewrite" gate of RFC 0078 is now satisfied.
