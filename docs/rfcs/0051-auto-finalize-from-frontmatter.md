# RFC 0051 — Auto-finalize jobs from artifact frontmatter

**Status:** accepted (bounded daemon slice landed)
**Scope:** V1 (single-version)
**Driven by:** the operator-on-behalf burden observed across
dogfood-054b / 055 / 055b / 056 (8 on-behalf publishes in one session,
all RFC 0046 V1 audit-chained but cumulatively expensive).

## Background

The supervised-lane pattern (RFC 0009 / RFC 0010 V2) is: the wrapper
process receives a packet, the agent inside spawns to do work, the
agent writes the expected artifact to disk, and the agent then calls
the closing CLI verbs from the packet's `commands` block
(`publish-artifact`, `verdict` / `submit-review`, `complete`).

A recurring failure mode — observed in **every** dogfood from 048
onward — is *gemini-class lane stall*: the agent does the substantive
work and writes a well-formed artifact, but then exits or stalls
before invoking the closing CLI verbs. The job stays in
`claimed`/`running` state for the entire lease window (typically 30
minutes) and the operator must:

1. Read the on-disk artifact.
2. Run `striatum ack`, `striatum publish-artifact
   --allow-no-process-execution --override-rationale "<text>"`,
   `striatum verdict --verdict <V> --rationale "<text>"` on-behalf.
3. Record the override in the audit chain.

This is RFC 0046 V1 working as designed — the override path exists
and is honest about itself — but the *frequency* of operator override
weakens "operator-on-behalf is the exception" toward "operator-on-behalf
is the rule." Across dogfood-056 alone, three of four reviewer
verdicts were operator-on-behalf publishes; only codex completed
naturally.

The artifact already carries the information needed to finalize.
The `finding.v1` frontmatter schema requires `verdict_intent` (one of
`accept`, `accept_with_findings`, `needs_revision`, `reject`). The
`handoff.v1` schema implicitly indicates completion. The byline is
required to match `expected_author_line`. The artifact path is
declared in the work packet's `expected_artifacts`. Everything the
runner needs to advance state is in the file the agent wrote.

## Goals

- When an expected artifact appears on disk under a healthy lease
  and validates against its schema, the runner auto-publishes,
  auto-records the verdict (for review jobs), and auto-completes
  the job — no operator intervention required.
- Auto-finalization records a `lane_finalization=auto_from_artifact`
  marker on the artifact and verdict events, so audits can
  distinguish "agent called CLI" from "runner finalized from disk"
  from "operator override".
- Operator-on-behalf remains available for cases where the on-disk
  artifact is missing or fails schema validation (the original
  RFC 0046 V1 path stays intact).

## Non-goals (V1)

- Watching the filesystem with inotify. V1 polls on the runner's
  existing lease-heartbeat tick — adequate and avoids new platform
  dependencies.
- Auto-finalizing when no `expected_author_line` match is achieved.
  RFC 0046 V1 byline rules still apply; auto-finalization composes
  with them, it does not bypass them.
- Removing `submit-review` / `verdict` / `complete` CLI verbs. They
  remain the canonical agent path and the operator-override path.
- Cross-job auto-finalization (only the lane's own expected artifact
  is in scope).

## Design

### Trigger

The runner already runs a periodic reconciliation tick that visits
each active lease and emits `lease.heartbeat` events. On that same
tick, for each session whose state is `claimed` or `running` and
whose lease is healthy:

1. For each declared `expected_artifacts[]` entry, check if the
   path exists on disk.
2. If yes, attempt to read + parse the artifact.
3. If parse succeeds and frontmatter validates against the declared
   `kind`'s schema, *and* the artifact's `author:` line equals
   `expected_author_line` exactly, proceed to auto-finalize.
4. Otherwise, fall through to normal lease-tick behavior (no
   change).

### Auto-finalize sequence

The runner executes the same internal operations the CLI verbs
would, recording two new event types:

```
artifact.auto_finalized
  payload: {
    "logical_name": "<name>",
    "path": "<path>",
    "sha256": "<hex>",
    "trigger": "lease_tick",
    "frontmatter_kind": "finding" | "handoff" | ...,
  }

job.auto_finalized
  payload: {
    "job_id": "<id>",
    "verdict": "<v>" | null,   # null for non-review jobs
    "verdict_source": "frontmatter_intent",
    "trigger": "lease_tick",
  }
```

For review jobs, the verdict comes from `verdict_intent` in the
frontmatter. For build/synthesis jobs, the runner just publishes
+ completes (no verdict step).

### Distinct from operator-on-behalf

| Path | Trigger | Audit marker |
| --- | --- | --- |
| Agent called CLI | Wrapper-driven | `lane_attestation=attested`, normal `artifact.published` |
| Runner auto-finalized | Lease tick + valid artifact on disk | `artifact.auto_finalized` |
| Operator on-behalf | Operator CLI call | `provenance.publish_without_process_execution` (RFC 0046 V1) |

All three paths remain visible and distinguishable in audits.

### Lane attestation interaction

If the lane is attested at finalization time and the artifact's
byline matches the lane's expected author line, the auto-finalized
artifact is recorded as attested — this is the *correct* behavior
because the supervised process *did* produce the file; it just
didn't call the closing CLI. The runner is recording what actually
happened.

If the lane is not attested (no `process_executions` row covering
the artifact path), RFC 0046 V1 still refuses, and the auto-finalize
attempt fails the same way an in-band `publish-artifact` would.
Operator override remains the explicit path for that case.

### Frontmatter schema strictness

The runner's existing `front_matter_validate(kind=...)` is the gate.
This RFC does not loosen it. Specifically:

- `finding.v1` requires `verdict_intent` — auto-finalize uses it
  verbatim. If absent, schema validation fails and auto-finalize
  refuses, falling through to lane stall behavior.
- `handoff.v1` has no verdict — auto-finalize only publishes +
  completes.
- Custom schemas (synthesis, support_ledger, etc.) are eligible if
  they validate.

### Failure modes

- **Artifact present but malformed:** auto-finalize refuses; lease
  continues; operator override still available.
- **Artifact present, valid, byline mismatch:** auto-finalize
  refuses; lease continues; operator can either rewrite the file or
  override.
- **Multiple expected artifacts, only one on disk:** auto-finalize
  refuses (`required: true` artifacts must all be present); lease
  continues.
- **Lease expired before tick:** normal stale-lease recovery applies;
  auto-finalize never fires on an expired lease.

## Open questions

- **OQ-1:** Should auto-finalize run on *every* lease-heartbeat tick
  or only after the agent has been quiet for N seconds? Argument for
  immediate: agent already wrote the file, no reason to wait.
  Argument for delay: agent may still be editing the artifact;
  finalize-mid-write would record a partial file. **Proposed
  resolution:** require a stat-stable window (e.g., file mtime older
  than 10 seconds) before finalizing. Cheap and avoids the race.

- **OQ-2:** Should the operator be able to disable auto-finalize
  per-workflow or per-job? Argument for: some workflows want strict
  agent-only finalization. Argument against: lane-stall pattern is
  universal; opt-out would re-introduce the original burden.
  **Implemented resolution:** V1 initially shipped dry-run-visible with live
  workflow opt-in. D125 later satisfied the default-on evidence gate, and D133
  flips live allowance on by default. Workflows that require strict
  agent-only finalization opt out with `recovery.auto_finalize.enabled=false`.

- **OQ-3:** What happens if the agent calls `publish-artifact`
  *after* auto-finalize fires? Argument for race: idempotent —
  publishing the same SHA256 is a no-op; verdict for the same job
  already exists → 409. Already the normal contention path. **Proposed
  resolution:** no special handling; existing idempotency wins.

## Acceptance

- Pin a regression test that demonstrates: agent writes valid
  artifact, wrapper exits without calling closing verbs, lease-tick
  fires, runner auto-publishes + auto-verdicts + auto-completes.
- Pin a regression test for the "still attested" path: auto-finalized
  artifact from attested lane records `lane_attestation=attested`
  on the verdict.
- Pin a regression test for malformed-frontmatter refusal: agent
  writes artifact with missing `verdict_intent`, lease-tick fires,
  no auto-finalize, lease continues.
- Pin a regression test for the new event types
  `artifact.auto_finalized` + `job.auto_finalized`.
- One dogfood run end-to-end with **zero** operator-on-behalf
  publishes on jobs whose agents wrote valid artifacts.

## Out-of-scope (RFC 0049 territory)

The claude-class lane stall (B in the conversation that prompted this
RFC) is a *different* failure mode — claude wrapper never produces an
artifact at all. RFC 0049 (interactive claude lane via MCP control
plane) is the longer-term fix. This RFC does not address it; it makes
the gemini-class stall (agent-wrote-artifact-but-stalled-on-CLI)
self-resolving.

## Migration

- New event types — no existing event renamed or removed.
- New audit marker `lane_finalization=auto_from_artifact` — additive.
- Live auto-finalize changes lease-tick semantics. D133 makes it live by
  default after the D125 evidence gate; workflows may opt out with
  `recovery.auto_finalize.enabled=false`, and dry-run projections remain
  read-only.
