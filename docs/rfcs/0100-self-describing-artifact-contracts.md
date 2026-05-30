# RFC 0100: Self-Describing Artifact Contracts — packet + error ergonomics

Status: proposed
Date: 2026-05-30
author: proposer-claude-opus-4-8-001

Context:
- Four issues from two dogfoods hit the **same wall**: an agent finishes the
  substantive work, tries to `artifact.publish`, and the front-matter contract
  rejects it with an opaque error — forcing the agent to read Striatum Go source
  mid-lane to reverse-engineer the allowed keys/enums:
  - [#74](https://github.com/halbritt/striatum/issues/74) — the `synthesis`
    contract is opaque and rejects common markdown metadata
    (`author`/`workflow`/`phase`/`lane`/`date`/`visibility`); the agent had to
    inspect source to find `schema_version`/`artifact_kind`.
  - [#79](https://github.com/halbritt/striatum/issues/79) — the
    `collaboration_ledger` contract rejects natural ledger front matter
    (`entries`) with no statement of what *is* allowed.
  - [#96](https://github.com/halbritt/striatum/issues/96) — `submit-review`
    silently defaults `logical_name` to `review`; the `finding` contract rejects
    `run_id`/`job_id`/`scope_ref`/`severity: none` without naming the allowed set.
  - [#88](https://github.com/halbritt/striatum/issues/88) — the prompt said
    `verdict: clear` but the contract only accepts
    `accept|accept_with_findings|needs_revision|reject`; the invalid value was
    not surfaced clearly in the lane's visible output.
- [RFC 0098](0098-adjudicated-constraint-extraction-loop.md) slice 1 already
  did this **for one shape** (`collaboration_ledger.v1.1`): the productive-refusal
  gate names allowed verdicts/dispositions and additively accepts richer
  metadata. RFC 0100 **generalizes that across all artifact kinds**.
- [`docs/decisions/decision-log.md`](../decisions/decision-log.md) — **D016**
  (build outputs are durable, idempotent, inspectable artifacts), **D028** (rich
  data belongs in front matter / body, not transcripts). An artifact the agent
  cannot author without reading Go source is not inspectable by its own author.

## Problem

Artifact contracts live in `go/pkg/artifactcontracts` as Go validators. Their
required + allowed front-matter shape is **not surfaced to the agent at the point
of need**:

1. **The work packet** advertises `expected_artifacts[].kind`/`logical_name`/
   `path` but **not** the front-matter schema for that kind — so the agent
   guesses, then fails at publish.
2. **Validation errors** name one bad field at a time
   (`field "entries" is invalid`) without enumerating the **allowed** keys or
   the **required** ones, so fixing is trial-and-error against an unknown set.
3. **Common workflow metadata** (`author`/`workflow`/`phase`/`lane`/`date`/
   `visibility`) — present in every lane's artifact template — is rejected by
   several kinds with no statement of whether it belongs in the body instead.

The work is done; the agent then burns context (and sometimes reaches for the
daemon DB, cf. #87) debugging a contract it cannot see.

## Goals

- The work packet **surfaces the required + allowed front-matter schema** for
  each `expected_artifacts` entry's kind.
- Validation/publish errors **enumerate allowed keys and enum values** and mark
  which are required — not one opaque field at a time. (The `verdict` error at
  `contracts.go:315` already does this for one field; generalize the pattern.)
- A standard set of **optional workflow metadata keys** is accepted across kinds
  (or the error explicitly says "put this in the body"), so a single lane
  artifact template publishes against every kind.
- An introspection surface (`striatum artifact describe <kind>` / a `--explain`
  flag) so the schema is discoverable **without** reading Go source.

## Non-Goals

- Loosening validation correctness — required fields stay required; bad enums
  still fail. This is about *legibility*, not permissiveness.
- Auto-repairing or auto-rewriting an agent's artifact.
- A new artifact storage model (that is RFC 0072).

## Proposal

### 1. Machine-readable contract descriptors (single source)
Derive a descriptor per artifact kind from the existing `artifactcontracts`
definitions — for each field: name, required?, allowed values/enum, "body vs
front matter" placement. One source of truth (the Go validators), exported as a
descriptor rather than re-specified.

### 2. Packet-embedded schema
Embed the descriptor (or a stable ref to it) in each `expected_artifacts` entry
in the work packet, so an agent authoring the artifact sees the exact contract it
must satisfy before writing.

### 3. Enriched validation errors
On rejection, list the **allowed keys + required keys + enum values** for the
kind, and for an invalid enum echo the **submitted** value alongside the allowed
set (extend the `contracts.go:315` pattern to every field). For `submit-review`,
echo the submitted `(logical_name, kind, path)` tuple next to the expected one
(#96), and infer `logical_name`/`kind` from the sole expected artifact when the
path matches.

### 4. Standard optional-metadata allowlist
Define a small set of optional workflow metadata keys
(`author`/`workflow`/`phase`/`lane`/`date`/`visibility`) accepted by every
front-matter–carrying kind, so the common lane template is portable. Keys outside
the union of "kind-specific" + "standard optional" produce the enumerated error,
not a bare reject.

### 5. Introspection verb
`striatum artifact describe <kind>` prints the descriptor; `artifact.publish`
gains an `--explain`/dry-run that validates and reports the full contract without
publishing.

## Acceptance Criteria

- The work packet for a job with `expected_artifacts` carries the front-matter
  descriptor for each artifact kind.
- A publish/validate failure enumerates allowed + required keys and enum values
  for the kind; `submit-review` echoes submitted vs expected identity and infers
  identity from a sole expected artifact.
- A lane artifact template carrying the standard optional metadata keys publishes
  against `synthesis`, `finding`, and `collaboration_ledger` without per-kind
  reshaping.
- `striatum artifact describe <kind>` and `artifact.publish --explain` exist and
  match the live validators (one source).
- Regression tests cover the enriched error text and the inference path.

## Phased plan

- **Phase 1 (no schema):** enriched validation errors (enumerate allowed/required
  keys + enums; echo submitted-vs-expected) + the standard optional-metadata
  allowlist + `submit-review` inference (#96). Closes the acute pain in
  #74/#79/#96/#88 without new descriptors. (#88 prompt-wording and #96 inference
  also land as the inline fixes referenced from those issues.)
- **Phase 2:** exported descriptors + `artifact describe`/`--explain`
  introspection.
- **Phase 3:** packet-embedded descriptors in `expected_artifacts`.

## Open Questions

1. **Descriptor source.** Reflectively derive from the Go validator structs, or
   author a sidecar schema kept in sync by a test? (Leaning: derive, so it cannot
   drift — same discipline as RFC 0060's single method-contract source.)
2. **Metadata placement.** Accept standard optional keys silently, or accept-and-
   ignore with a one-line note that they are non-semantic? (Leaning: accept;
   silence is fine if documented in the descriptor.)
3. **Scope creep vs RFC 0098.** Keep RFC 0098's collaboration_ledger gate as the
   one *semantic* contract; RFC 0100 is purely *legibility*. Confirm no overlap
   where 0100 would relax a 0098 gate.
