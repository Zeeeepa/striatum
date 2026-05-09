# Design review: RFC 0018 step 3 (V1.5)

author: reviewer-claude-opus-001
date: 2026-05-09
verdict: accept_with_findings

Devil's-advocate review of `DESIGN_SYNTHESIS.md`.

## Verdict

**accept_with_findings** — V1.5 is implementable. Three findings
folded into the implementation (one acceptance-blocking; two
notes).

## Sweep

### 1. Migration safety

`ALTER TABLE` on SQLite preserves indices and triggers attached
to the existing columns. The new `posture` column is independent;
no rebuild needed. The synthesis's PRAGMA `table_info` idempotency
check is correct. **Accept.**

### 2. Backfill rule — "neutral" SQL value matches "implicit neutral"

Both pre-migration rows (backfilled to `'neutral'`) and post-
migration verdicts on posture-omitting jobs get the literal
string `'neutral'`. Queries grouping by `posture` see them as a
single bucket. **Accept.**

### 3. Snapshot lookup — workflow snapshot is the source

The synthesis correctly reads from `workflow_snapshots.workflow_json`
rather than the live `workflow.json` file on disk. This matches
the immutable-snapshot pattern the rest of the codebase uses.
**Accept.**

### 4. Per-surface zero-regression — *one regression admitted*

The synthesis explicitly flags one intentional regression:
`evidence export` adds a `Posture: <value>` line to every
verdict block (always, not just for non-neutral postures). This
is a *format* change to a downstream-parsable Markdown surface.

**Counterargument:** evidence-export Markdown is consumed by
external auditors and CI scripts that may regex for specific
patterns. A new line in every verdict block could break a
parser that expects a fixed line count.

**Survives?** Partially. The line is *additive*; parsers that
extract by key name (`Verdict: ...`, `Posture: ...`) tolerate
it. Parsers that extract by line number would break. The
RFC explicitly states this surface is part of the V1.5 contract;
the CHANGELOG note is the right communication channel.

**Finding 1 (note):** the CHANGELOG note must explicitly call
out the format change as a breaking-for-line-counters
regression. Acceptable.

### 5. Web UI rendering — `posture-chip` CSS class

The synthesis adds a new CSS class but does not specify the
visual rendering rule (color? size? truncation for long
custom postures?).

**Finding 2 (acceptance-blocking):** pin the visual rule.
Recommendation: gray background, same height as verdict badges,
`max-width: 12em` with `text-overflow: ellipsis` for long
`custom:<long-name>` strings. Tooltip on hover shows the full
posture name.

### 6. Dashboard truncation rule

The synthesis specifies "top-3 by count with `+N more` overflow"
which matches RFC 0018's "≤ 4 postures" guidance. **Accept.**
But: what if two postures tie at the same count? Sort by name
secondarily for determinism (test snapshots will fail if order
is undefined).

**Finding 3 (note):** add a stable tie-break in the dashboard
sort: by descending count, then by ascending posture name.

### 7. Test plan completeness

Each surface has a test. The "zero-regression byte-identical"
assertions for posture-omitting runs are the right shape for
catching surface-creep. **Accept.**

## Findings summary

| # | Severity | Action |
| --- | --- | --- |
| 1 | note | CHANGELOG explicitly notes evidence-export format change |
| 2 | acceptance-blocking | Pin web UI chip CSS rule (color, width, ellipsis, tooltip) |
| 3 | note | Dashboard sort: count desc, then posture name asc (tie-break) |

## Decision

Accept V1.5 with Finding 2 implemented and Findings 1 + 3 noted
in BUILD_HANDOFF.
