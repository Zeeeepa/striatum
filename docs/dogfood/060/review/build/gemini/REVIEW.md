---
author: reviewer-unknown-model-001
kind: finding
logical_name: build_review_gemini
---

# Threat-Modeling Review: RFC 0048 Read Handlers (Adversarial)

**Status:** needs_revision
**Verdict:** reject (7 findings)

This review evaluates the ported native PostgreSQL read handlers for RFC 0048 Phase C. While the core repository scoping is generally correct, significant gaps exist in evidence redaction, pagination consistency, and test robustness.

## Trust Boundaries & Attack Surfaces

- **RPC Dispatcher:** Trusted boundary between external capability tokens and internal DB access.
- **Redaction Policy:** Trusted boundary between internal prose-heavy state and committed/exported artifacts.
- **Repository Isolation:** Trusted boundary between multiple repositories sharing a single Postgres instance.

## Findings

### 1. Hardcoded "OK" Doctor Status in Evidence Export (Stale Data)
**Severity:** HIGH
**Path:** `src/striatum/daemon_pg/handlers/reads/evidence_export.py`

The `evidence.export` handler hardcodes the `doctor` output to a successful state instead of calling the actual `doctor` handler logic:
```python
doctor = redact_evidence_payload({"ok": True, "schema_version": 5, "problems": []})
```
This causes evidence exports to silently omit any repository health problems (e.g., missing expected artifacts), providing a false sense of integrity.

### 2. Redacted Prose Leak in Corpus Export
**Severity:** HIGH
**Path:** `src/striatum/corpus/redaction.py`

`redact_run_summary_payload` allows the `artifacts` and `sessions` keys through its top-level allowlist but fails to recursively redact them. Because `run_summary_payload` includes the full `author_line` for artifacts and `close_reason` for sessions (both of which are operator-supplied prose), these fields are leaked unredacted into the `corpus.export` output.

### 3. Inconsistent Sorting in Status vs. List.Runs
**Severity:** MEDIUM
**Path:** `src/striatum/daemon_pg/handlers/reads/_read_model.py`

The `status_payload` helper sorts runs by `created_at` without a deterministic tie-breaker:
```python
ORDER BY r.created_at
```
In contrast, `list.runs` uses `ORDER BY r.created_at, r.run_id`. This inconsistency can lead to "jumping" or missing items when an operator switches between summary and list views, especially for runs created in the same millisecond.

### 4. Non-Existent Pagination in List.Jobs
**Severity:** MEDIUM
**Path:** `src/striatum/daemon_pg/handlers/reads/list_jobs.py`

Unlike `list.runs` and `list.workflows`, the `list.jobs` handler lacks a `LIMIT` clause and does not support any form of pagination. For large runs with thousands of jobs (e.g., exhaustive search or batch refactoring), this method will return the entire result set in a single RPC response, creating a denial-of-service vector for the daemon's memory and transport.

### 5. Parity Tests Pass on Empty Connections (False Positives)
**Severity:** MEDIUM
**Path:** `tests/daemon_pg/handlers/reads/conftest.py`

The `empty_ctx` fixture uses an `EmptyConnection` that always returns empty lists for `fetchall()`. Any test comparing the PG handler against a legacy path using this connection will "pass" simply because both return empty results for different reasons (one because the DB is empty, the other because the cursor is mocked to be empty). The test suite lacks actual byte-equivalence parity tests that populate a real PG instance.

### 6. Missing Repository Filter in Substrate Pointers (Adversarial Escape)
**Severity:** MEDIUM
**Path:** `go/pkg/db/supervisor_pointers.go` (Inferred)

While Python-side read handlers generally filter by `repository_id`, the supervision logic relies on `process_supervisors` which joins with global `daemon_supervisors`. If the Go-side upsert logic for pointers or the Python-side read join forgets the `repository_id` constraint, a session in Repo A could theoretically observe or claim a supervisor process belonging to Repo B if their `supervisor_id` slugs collide.

### 7. Redacted JSON Leaked in Dashboard Events
**Severity:** LOW
**Path:** `src/striatum/daemon_pg/handlers/reads/_read_model.py`

The `events_for` helper returns raw `payload_json` for event logs. When surfaced through the `dashboard` RPC (which does not apply `redact_evidence_payload`), this leaks the full structured data of events, including rationale for review overrides and blocker descriptions, which are intended to be redacted in exported surfaces.

## Mitigation Roadmap

1.  **Fix `evidence_export.py`** to call the real `doctor.handle` logic.
2.  **Update `redaction.py`** to recursively redact `artifacts` and `sessions` in `redact_run_summary_payload`.
3.  **Standardize sorting** across all `list.*` and `status` handlers to use `(timestamp, id)` deterministic pairs.
4.  **Add `LIMIT` and `OFFSET`** to `list.jobs`.
5.  **Implement real PG parity tests** that use `tests/_harness/pg.py` to populate data rather than mocking empty cursors.
