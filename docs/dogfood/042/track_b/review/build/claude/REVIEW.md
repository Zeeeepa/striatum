---
schema_version: "striatum.finding.v1"
artifact_kind: "finding"
verdict_intent: "accept"
severity: "low"
tags: ["ergonomics_dx", "rfc-0044", "engram", "build", "track_b"]
---

author: reviewer-claude-opus-004

# Track B Build Review — RFC 0044 (ergonomics_dx, claude lane)

## Scope and posture

This is a fresh-context, developer-ergonomics review of
`docs/rfcs/0044-engram-phase-1-implementation-spec.md`. The claude-lane angle
is operator-side UX: can a first-time operator discover and use the Engram
retrieval surface, is the augmentation-not-replacement boundary legible at
the surface, and does the spec describe graceful degradation when Engram is
unavailable.

## Required-checks summary

| Check | Result | Notes |
|---|---|---|
| Acceptance criteria concrete enough for a future dogfood to implement | Pass | Section "Acceptance Criteria" enumerates testable predicates per surface; smoke set is explicit; a few thresholds are deferred to Phase 1 implementation (see findings). |
| Augmentation-not-replacement boundary preserved | Pass | Section 8 + Acceptance "Augmentation Boundary" + Domain Modeling all explicitly forbid redesigning Engram's `sources/segments/claims/beliefs` and add `source_kind='striatum'` + `corpus_id` additively. |
| Striatum-must-run-without-Engram fallback explicit | Pass | Section 8 lists timeouts (2 s search, 5 s fetch), the "Engram off" dogfood-shaped acceptance test, `striatum operator memory check` exits `0` regardless, no Engram import in `src/striatum/cli`. |
| Capability vocabulary documented | Pass | Section 6 capability table is clear; Engram-local scope explicitly separated from RFC 0030 daemon-RPC capabilities; defaults are stated per capability. |
| Open questions for future RFCs (Phase 2/3/4) clearly named | Partial | The five "Open Questions" are all Phase-1 implementation details. Phase 2/3/4 follow-on questions are described in RFC 0041's roadmap but not re-surfaced here as an explicit "deferred to Phase N" list. Low-severity gap; see finding F5. |

## Ergonomics_dx findings

Findings are ordered by operator-impact, not by severity (all are low).

### F1. First-time operator setup surface is described but not located

The RFC documents MCP client configuration as a JSON snippet (Section 7) and
says it is "documented, not automated." It does not name the canonical doc
the snippet lands in. The acceptance criteria list five Striatum docs the
implementation must update (`docs/SPEC.md`, `docs/HOW_TO_AGENT.md`,
`docs/HOW_TO_HUMAN.md`, `docs/UBIQUITOUS_LANGUAGE.md`,
`docs/DECISION_LOG.md`) but does not say which one carries the first-time
operator's "install + wire Engram MCP" walkthrough. From a first-time
operator's perspective, this is the discovery entry point, so it deserves to
be named.

Recommendation (non-blocking): name `docs/HOW_TO_HUMAN.md` as the canonical
operator setup home for the Engram wiring section, and `docs/HOW_TO_AGENT.md`
for the runtime retrieval-conventions section.

### F2. `striatum operator memory check` UX shape is underspecified

The acceptance criterion is that the verb "exits `0` regardless of Engram
availability," which correctly protects the augmentation boundary. From an
ergonomics standpoint, the operator also needs the verb to be **useful** —
exit `0` plus silent output would meet the criterion but defeat its purpose.
The RFC says the verb "prints status," but does not specify the four cases an
operator will actually encounter:

- Engram installed, MCP healthy, Striatum corpus present.
- Engram installed, MCP healthy, Striatum corpus missing (no
  `engram ingest-striatum` has run).
- Engram installed, MCP unreachable (Postgres down, console script not on
  PATH).
- Engram absent.

Each case has different operator next steps (run export+ingest, start
Postgres, install Engram). The current spec leaves these implicit.

Recommendation (non-blocking): the V1 implementation dogfood should make
each of these four cases produce a distinct one-line status with a
single actionable next step.

### F3. The retrieval-result `privacy_tier` field is surfaced but not defined

`engram.search` results carry `privacy_tier` (Section 5), and Open Question 3
mentions "Tier 1 for commit-safe public-repo-style rows; stricter effective
treatment for operator-report and audit-chain free text." The enumerated
tier set is not stated, and the RFC does not say how an operator should
interpret a tier in a search result.

This is in the operator's primary read path — every search result has the
field — so an undefined value harms discoverability. Recommendation
(non-blocking): the implementation dogfood should fix the V1 tier enum
(e.g. `public`, `internal`, `sensitive`) in code and surface the same
strings in `engram.describe_corpus` output so an operator can look up
meaning at the same surface they encounter the value.

### F4. Failure-mode visibility at the retrieval call site

The RFC budgets retrieval at 2 s search / 5 s fetch and says Engram
unavailability "degrades to the pre-Engram operator path: read the
repository docs and explicit work packet context directly." It is silent on
what the MCP tool returns to the operator on timeout, empty result vs.
unavailable, and whether `engram.health` should be auto-invoked when search
returns zero.

A first-time operator who sees an empty result from `engram.search` cannot
tell the difference between "no hits" and "Engram is down." Both are
legitimate outcomes; both should be distinguishable. Recommendation
(non-blocking): the implementation dogfood should specify the empty-vs-
degraded result shape (e.g. `engram.search` returns an `engram_status`
field on every call, or a typed empty result includes a `reason` value).

### F5. Future-phase open questions are not re-surfaced

The Open Questions section is well-shaped for Phase 1 implementation. It
does not include the future-phase questions RFC 0041 deferred — for
example, when (if ever) does the operator's session brief auto-invoke
retrieval (RFC 0041 Phase 2)? When does write-side ingestion run on
`run.completed` (RFC 0041 Phase 3)? How is cross-corpus retrieval gated in
practice (RFC 0041 Phase 4)?

Phase-2/3/4 questions live in RFC 0041's roadmap, so this is not a
correctness gap. From an ergonomics perspective, a future operator reading
RFC 0044 alone may not see the deferred decisions. Recommendation
(non-blocking): a one-paragraph "Deferred to Phase 2-4" pointer back to
RFC 0041's roadmap closes the loop.

## Strengths worth keeping

- The four-tool surface (`engram.search`, `engram.fetch_reference`,
  `engram.describe_corpus`, `engram.health`) is minimal and matches a
  first-time operator's mental model: search, drill, describe, ping.
- The capability table in Section 6 is the single best ergonomic affordance
  in the RFC: defaults are explicit, the personal/cross-corpus rails are
  off by default, and the Engram-local boundary is named.
- `striatum-engram` skill bundled into RFC 0015 profiles is the right
  discovery surface — short, harmless when Engram is offline, and lives
  next to other skills an operator already loads.
- The "Engram off" dogfood-shaped acceptance test is the strongest possible
  evidence that the augmentation-not-dependency invariant survives the V1
  implementation pass.
- Result rows carry `reference_id`, `corpus_id`, `source_kind`, `sub_kind`,
  `external_id`, `content`, `score`, `privacy_tier`, and provenance — a
  generous schema that lets the operator construct follow-up `fetch`
  calls without leaving the result.
- `striatum corpus export` produces deterministic JSONL with idempotent
  hashes; re-running with unchanged inputs is a no-op for the ingester.
  Operators can run the export defensively before each session without
  cost — this is the right ergonomic shape.

## Verdict

**verdict_intent: accept**

The RFC body is implementation-ready under the ergonomics_dx posture. The
operator-facing surface is small, discoverable, and consistent; the
augmentation-not-dependency boundary is enforced at multiple layers
(timeouts, capability scoping, no-import grep check, dogfood-shaped "Engram
off" test); the capability vocabulary is documented and separated from
RFC 0030; graceful degradation to the pre-Engram operator path is named.

The findings above are all low-severity polish items appropriate for the
Phase 1 **implementation** dogfood to resolve, not blockers to accepting
the spec. F2 (memory-check status cases) and F3 (privacy tier enum) are
the ones most likely to bite a first-time operator and should be picked up
early in the implementation pass.

## Cross-cuts (informational)

- Numbering: the RFC notes that RFC 0041's roadmap tentatively assigned
  RFC 0044 to Phase 3 write-side; this RFC reassigns 0044 to Phase 1
  implementation. The note is correct and worth keeping. Phases 2/3/4
  will need to renumber when written.
- The RFC stays inside Striatum's product boundary in `docs/SPEC.md`:
  no hosted service, no telemetry, single-machine local-only, repository
  artifacts remain authoritative provenance, `.striatum/state.sqlite3`
  remains the live workflow state, Engram is read-only and optional.
