---
schema_version: striatum.finding.v1
artifact_kind: finding
verdict_intent: accept_with_findings
severity: medium
---

# Doctor Signal Review
author: doctor-auditor-codex-via-live-striatum-mcp-001

## Verdict on doctor trustworthiness

`striatum doctor` is currently trustworthy as the operator's red/green stop signal, with findings. The recent D204/D205 split is the right direction: `ok=false` is reserved for actionable integrity loss, while preserved, terminal, legacy, superseded, or sha-bound acknowledged historical residue remains visible as warnings.

The caveat is warning volume. A green doctor with hundreds of warnings can still teach operators to stop reading the diagnostic surface. The current implementation keeps safety load-bearing because uncatalogued loss and sha-mismatched acknowledged-loss entries still red the run, but the UI/summary layer needs to keep warning categories legible.

## Current doctor result summary

The direct CLI form `striatum doctor --verbose --json` was not usable from this lane because the CLI returned `daemon RPC authorization failed`. The equivalent daemon MCP `doctor` read succeeded.

Current MCP doctor summary:

- `ok=true`, `schema_version=28`
- `problems=0`, `problem_records=0`
- `needs_operator=0`, `waiting_human=0`, `stale_leases=0`
- `pg_write_boundary=full`, `pg_read_scope=partial_projection_gated`
- `artifact_anchor_integrity.checked=true`, `problem_count=0`, `warning_count=212`, `acknowledged_loss_status=loaded`
- artifact population: 331 total, 160 git-anchor, 171 blob-exhaust
- top-level verbose warning records: 217 total

Warning classes in the live result:

- `artifact_legacy_unverifiable`: 143
- `artifact_debris_terminal_run`: 41
- `artifact_acknowledged_loss`: 16
- `artifact_superseded_on_default_branch`: 12
- `worktree_unanchored_on_default_branch`: 4
- `worktree_debris_terminal_run`: 1

The live doctor also reported four supervisors as `tmux_ok`. The Codex block still reports stale local config relative to the live MCP endpoint, but the runtime token is present and the lane is using the injected live endpoint.

## Reclassification analysis

D204 correctly reclassified non-actionable historical findings:

- content preserved on the default branch is not a durability loss
- canceled or failed run debris should be visible but not red
- pre-blob-storage legacy artifacts should warn when no blob key exists but the content is otherwise preserved

D205 completed the practical cleanup by adding three more preservation signals:

- default-branch history awareness for content merged and later edited or deleted
- `artifact_superseded_on_default_branch` when the path remains live but the recorded draft sha is no longer the default-tip content
- `artifact_acknowledged_loss` only when a tracked baseline entry matches the artifact id and exact `content_sha256`

The load-bearing part is the negative path. These must remain red: uncatalogued artifact loss, acknowledged-loss id matches with sha mismatches, anchor hash mismatches, missing blob metadata when neither legacy nor preserved, worktree HEADs unreachable from the run branch or `refs/striatum/`, completed jobs without anchors, active work without active leases, and degraded mandatory substrate checks such as the PostgreSQL write boundary or lane sandbox enforcement.

D206 addresses the divergent-ideation fan-in class by integrating non-fast-forward fan-in siblings into the run branch and adding `fanin_sibling_unintegrated` as a running-run warning. That is suitable as a live regression signal, but it is not a substitute for the deferred join barrier or join manifest: doctor can now observe reachability/integration problems, not prove that a downstream synthesis consumed every sibling's content.

## Warning-noise risks

The current green result carries 217 warning records. That is defensible only if the operator can tell at a glance whether the warnings are old acknowledged residue or new live risk.

The largest category, `artifact_legacy_unverifiable`, is expected historical debt, but 143 entries makes the warning channel visually expensive. The 16 acknowledged losses are safer because they are sha-bound and curated, but they still normalize "loss as warning" unless the UI keeps the baseline status and count prominent. The 12 superseded warnings are also easy to misread: they mean the deliverable path is live, not that the recorded artifact content is recoverable.

The fan-in warning added by D206 is intentionally non-red for running runs. That is acceptable while the deferred join barrier is open, but it needs to remain conspicuous because a warning-only fan-in regression can still produce a misleading downstream artifact if consumers read a run branch missing a sibling.

## Missing doctor checks

Doctor does not yet appear to enforce a warning budget or warning-delta signal. A green result with 217 warnings is very different from a green result with 5 warnings, and a new warning class should be difficult to miss.

Doctor does not yet prove divergent-ideation downstream consumption. The supported fixture proves double fan-out/join recovery, and D206 improves run-branch integration, but the deferred join manifest would let doctor distinguish "all siblings are reachable" from "the join consumed the exact live sibling set."

Doctor does not yet surface the #308 class as an immediately actionable recoverable condition when a final job has reconstructable artifacts but remains unsealed after `agent_exited_unsealed`. The current result is clean because the operator finalized affected runs through daemon recovery, but the recurring pattern deserves a specific diagnostic once the recovery behavior lands.

Doctor should keep highlighting stale lane-client configuration separately from integrity. The current Codex stale-config signal did not affect `ok`, which is correct, but it should remain easy to separate from historical artifact warnings because it can cause future lanes to miss MCP injection.

## Tests that would keep doctor honest

Keep the D204/D205 negative tests as release-blocking: uncatalogued real loss must red `ok`, acknowledged-loss sha mismatch must red `ok`, and missing or unparseable acknowledged-loss baseline must safe-degrade without hiding loss.

Add a warning-regression test that seeds the current known warning classes and asserts the summary carries per-class counts, not just a flat warning list. This protects operator legibility without turning expected historical debt red.

Add a fan-in doctor fixture for D206: while a run is still running, a completed fan-in sibling reachable only by pin should produce `fanin_sibling_unintegrated`; after integration it should disappear; overlap should remain a loud completion failure, not a warning.

When the deferred join manifest lands, add a divergent-ideation fixture where all sibling commits are reachable but the join manifest omits one live attempt. That should become a doctor-visible problem or a high-signal warning, depending on the final decision.

Add a recovery diagnostic test for the #308 class: a reconstructable `agent_exited_unsealed` final job should either auto-finalize through sweep or appear in doctor/status with an exact recovery action, not sit as generic `needs_operator` noise.
