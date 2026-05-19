---
schema_version: striatum.finding.v1
artifact_kind: finding
severity: high
verdict_intent: needs_revision
---

# GH #25 Verification Review

author: reviewer-unknown-model-001

Final verdict: `needs_revision`

## Scope

This was a fresh-context compliance/license review. The work packet's review
policy restricted the review to the provided documents and explicitly forbade
consulting repository contents beyond those inputs. The verify prompt, however,
requires reading the implementer handoff, grepping changed files, running
before/after JSON comparisons, exercising daemon-unreachable behavior, and
checking source/test changes. Those implementation inputs were not available in
the allowed document set.

Within the allowed documents, I found no unresolved license, attribution, hosted
service, telemetry, transcript-capture, or external-persistence issue. The
project boundary forbids hosted services, external persistence, telemetry, and
transcript capture in `docs/SPEC.md:12-18`, and GH #25 is scoped to local CLI
registry/listing behavior.

## Acceptance Verification

1. `striatum repo list` without `--json` prints a human-readable table and
   highlights or sorts the cwd repository first.

   Not verified. The requirement is defined in `docs/issues/25/SPEC.md:36`,
   but the allowed evidence set does not include the implementation handoff,
   changed files, or command output needed to confirm table rendering.

2. The SQLite-presence pre-flight check is removed from the `list` path.

   Not verified. The requirement is defined in `docs/issues/25/SPEC.md:37`.
   The broader product contract says daemon-owned PostgreSQL is authoritative
   and `.striatum/` is operational scratch in `docs/SPEC.md:29-39`, while the
   transition runbook says mapped CLI routes fail closed instead of falling back
   to SQLite when the daemon is unreachable in `docs/POSTGRES_TRANSITION.md:31-39`.
   Those documents support the intended direction, but they do not prove the
   `repo list` read path no longer checks `state.sqlite3`.

3. `striatum repo list --json` semantics are unchanged byte-for-byte.

   Not verified. The requirement is defined in `docs/issues/25/SPEC.md:38`.
   The allowed document set contains no before/after JSON fixtures or command
   output for a byte-for-byte comparison.

4. `repo list` never emits `repo_not_migrated`; daemon-unreachable behavior is
   reported as `daemon_unreachable`.

   Not verified. The requirement is defined in `docs/issues/25/SPEC.md:39`.
   The general contract says unreachable daemons refuse with exit code 11
   `daemon_unreachable`, while unregistered repositories refuse with exit code
   12 `repo_not_migrated` in `docs/SPEC.md:118-125`. That supports the desired
   distinction but does not prove the `repo list` path implements it.

5. Tests cover registered-repo table output and daemon-unreachable behavior.

   Not verified. The requirement is defined in `docs/issues/25/SPEC.md:40`.
   The allowed document set does not include test files, test output, or the
   implementer handoff.

## Required Adversarial Probes

- No SQLite check anywhere on the `list` path: not performed because changed
  files/source reads were outside the allowed evidence set.
- `adopt` and `repo add --init` still refuse an unmigrated
  state-DB-present setup: not performed because source/test reads and command
  execution for this behavior were outside the allowed evidence set.
- `--json` round-trip byte-for-byte unchanged: not performed because the
  required before/after command outputs were not available in the allowed
  documents.
- Daemon-unreachable path returns `daemon_unreachable`: not performed against
  the implementation for the same reason.
- Table format readability: not performed because no rendered table output was
  included in the allowed documents.

## Finding

Severity: high

The review packet does not include the implementation handoff, changed files,
or verification outputs required by the verify prompt, so GH #25 cannot be
accepted as closed from the allowed evidence. Remediation is to rerun this
verification with `docs/issues/25/build/HANDOFF.md`, any issue-local scope file,
the changed source/test files named by the handoff, and command/test output for
the five acceptance bullets. With those inputs, the reviewer can complete the
specific SQLite-path grep, mutation-side regression check, JSON round-trip diff,
daemon-unreachable probe, table-format inspection, and test assessment.
