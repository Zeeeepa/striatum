# RFC 0091: Lane health — one deep module for attestation, liveness, and delivery

Status: accepted
Date: 2026-05-29
Context: RFC 0088, RFC 0089, D026, D080, D141, D149, D152, D153; ubiquitous-language: lane-bound, lane health
Author: proposer-claude-opus-4.8-001

## Problem

The composite question an operator, a read view, and three mutation paths all
ask — *"is this supervised lane bound, alive, attested, and deliverable right
now?"* — has no module. It is recomputed in at least four places, and the
attestation rules are duplicated across the mutation path and the read path:

- `go/pkg/mutations/mutations.go:648` `sessionLaneAttestation` — the 3-table
  join (`process_supervisors` + `process_supervisor_pointers` +
  `daemon_supervisors`), six sequential row checks, the tmux-metadata integrity
  guard, then `supervisor.ProbeLaneLiveness` with a `start_token_unverified`
  downgrade. Returns `map{attested,state,supervisor_id,pid,reason,liveness}`.
- `go/pkg/reads/supervision.go:789` `applySupervisorLaneAttestation` +
  `:813` `tmuxStartTokenUnverified` — the read path **re-derives** the same
  attestation. The `start_token_unverified` ⇒ unattested rule is written
  verbatim in two packages (`mutations/mutations.go:709` and
  `reads/supervision.go:813`).
- `go/pkg/mutations/supervision_control.go:826`
  `reconcileSupervisorForDelivery` + `:896` `supervisorDeliveryDegraded` —
  combine delivery-bridge degradation with the probe; `:859` reaches past the
  `supervisor.PointerStore` interface with raw pgx.
- `go/pkg/reads/supervision.go:188` `sessionProtocolLiveness` →
  `sessionliveness.Classify` — the pure stall/lease classification.

Two further frictions compound this:

- The `metadata["tmux"]` shape is implicitly defined across five files
  (`supervisor/liveness.go`, `supervisor/tmux_liveness.go`,
  `mutations/supervision_control.go`, `mutations/supervision.go`,
  `reads/supervision.go`) — read and written without a single typed definition.
- The pure attestation logic is only testable through real Postgres and a real
  tmux binary, so the existing supervision tests global-mock
  `supervisionTmuxRunner`.

The duplication is the bug surface: the two copies of the attestation rule can
drift, and an operator/maintainer (or an AI agent) must bounce across four
packages to answer one question.

## Goals

- One deep module, `go/pkg/lanehealth`, that computes composite lane health
  once, behind a small interface.
- A pure classifier (`Classify`) reachable with no DB and no tmux — the single
  test surface for every reason, including `start_token_unverified` precedence.
- Preserve today's wire semantics exactly: the `attested` value, the reason
  strings, and the `sessionLaneAttestation` map shape.
- Make the `metadata["tmux"]` shape a single typed codec.
- Improve AI-navigability: one seam answers the composite question; the glossary
  gains `lane-bound` and `lane health`.

## Non-Goals

- No new liveness semantics. `Attested` stays `Bound && Alive &&
  start-token-verified`.
- No change to the daemon RPC contract, the wire JSON, or any migration.
- No batch/dashboard optimization (a later `AssessMany` is the escape hatch;
  out of scope here).
- Not normalizing the `pid_start_time` redundancy across the three supervisor
  tables — that is a separate data-model decision.
- lanehealth does **not** own transactional locking;
  `reconcileSupervisorForDelivery` keeps its in-transaction `FOR UPDATE` lock.

## Proposal

### Module shape — pure `Classify` behind a thin `Checker`

```go
package lanehealth

type Health struct {
    Bound, Alive, Attested, Deliverable bool
    Stall         sessionliveness.Result
    Reason        LaneReason     // .String() == today's wire strings
    LivenessClass string
    SupervisorID  string
    PID           int
}

func (h Health) LiveTarget() bool { return h.Attested && h.Alive }   // interrogation
func (h Health) CanDeliver() bool { return h.Alive && h.Deliverable } // delivery pre-check
func (h Health) IsStalled() bool  { return h.Stall.StallClass != "" }

// Checker is wired once at cmd/striatumd/main.go. Check takes the db.Runner
// per call because handlers already hold one. Check = Load → Classify.
type Checker struct{ Probe Probe }
func (c Checker) Check(ctx context.Context, runner db.Runner, repositoryID, sessionID string) (Health, error)

// Probe is the one injected port: prod wraps supervisor.ProbeLaneLiveness,
// tests return a canned supervisor.LaneLiveness.
type Probe interface {
    ProbeLane(ctx context.Context, meta supervisor.TmuxMeta, pid int, startToken string) supervisor.LaneLiveness
}

// Classify is PURE — no DB, no tmux. The whole test surface.
// now is a parameter so stall classification is deterministic in tests.
func Classify(f Facts, now time.Time) Health

type Facts struct { /* 3-table join rows + Liveness + Activity + Policy */ }

// LegacyMap reproduces today's sessionLaneAttestation map shape for the
// compat wrapper. Deprecated: new callers read Health fields/predicates.
func LegacyMap(h Health) map[string]any
```

### Axes and precedence

- `Bound` — all row checks pass (rows only, pure).
- `Alive` — the process/pane probe says alive.
- `Attested` — `Bound && Alive && start-token-verified` (equals today).
- `Deliverable` — the delivery bridge is not degraded (independent of `Attested`).
- `Stall` — `sessionliveness.Result`, composed unchanged.

`Classify` must encode today's precedence exactly:
`no_attached_supervisor → pid_missing → daemon_supervisor_missing →
pointer_state_mismatch → daemon_state_mismatch → pointer_pid_mismatch →
tmux_metadata_corrupt →` (probe) `!Alive ⇒ pid_gone/class →
start_token_unverified → attested`. `LaneReason` is a closed enum whose
`String()` returns the existing wire strings.

### Seams and dependencies

- **Probe — the one port.** Two adapters justify it: production wraps
  `supervisor.ProbeLaneLiveness`; tests return canned `supervisor.LaneLiveness`
  to cover every liveness branch without a tmux binary.
- **`db.Runner` — unported.** It is already an interface, and this project
  tests against real Postgres via `go/pkg/pgtest`. The hard logic lives in the
  pure `Classify` (DB-free); the thin load glue is covered by `pgtest`.
  A `RowStore`-style DB port is **explicitly rejected** — its only second
  adapter would be an in-memory fake that contradicts the `pgtest` convention
  and tests glue `pgtest` already covers (single-adapter indirection).
- **`sessionliveness` — composed, unported.** Pure in-process dependency;
  `Classify` calls `sessionliveness.Classify(f.Activity, policy, now)` directly.
- **`supervisor.TmuxMeta` — the single metadata codec.** It lives in package
  `supervisor` (beside `TmuxIdentityFromMetadata` / `CaptureTmuxIdentity` /
  `TmuxLivenessPayload`) to avoid the cycle `supervisor → lanehealth →
  supervisor`. The loader unmarshals pointer `metadata_json` into it.

### Implementation sequence

**Phase 1 — introduce the module (no caller changes).**
Add `go/pkg/lanehealth` (`Facts`, `Classify`, `Health`, `LaneReason`, `Probe`,
`Checker`, `LegacyMap`) and `supervisor.TmuxMeta` (read side: unmarshal). Land
the pure table-tests. Nothing else changes yet.

**Phase 2 — migrate callers and delete the duplication.**
- `sessionLaneAttestation` → thin wrapper over `Check` + `LegacyMap`
  (wire-identical map).
- Delete `applySupervisorLaneAttestation` and `tmuxStartTokenUnverified`; the
  read view maps `Health` fields.
- `reconcileSupervisorForDelivery` reads `.Alive`/`.Deliverable`; the
  `supervision_control.go:859` raw-pgx duplicate read folds into the loader,
  **but the in-transaction `FOR UPDATE` consistency check stays in the
  mutation**.
- `interrogation.requireLiveTarget` → `Check(...).LiveTarget()`, preserving the
  RFC 0084 `awaiting_interrogation` fallback.
- Delete the now-dead read-side attestation tests and the
  `supervisionTmuxRunner` attestation global-mock.

**Phase 3 — fast-follow (non-gating).**
Migrate the three `TmuxMeta` writers (`supervisor/liveness.go`,
`supervision_control.go::tmuxMetadataFromHelperEvents`,
`mutations/supervision.go`) onto `supervisor.TmuxMeta.Marshal`, so the metadata
shape has one definition on both the read and write sides.

## Required Tests

- **Pure** — `TestClassify`: a table over every `LaneReason` including the
  `start_token_unverified` precedence and the `Bound`/`Alive`/`Attested`/
  `Deliverable` decomposition. No DB, no tmux.
- **Integration** — `TestLoad` (`pgtest` + fake probe): the attested happy path
  plus representative unattested rows (pointer/daemon mismatch, pid gone).
- **Compat** — `TestLegacyMapMatchesWire`: `Health → LegacyMap` equals today's
  `sessionLaneAttestation` map for attested and unattested cases (golden).
- **Regression preserved** — `requireLiveTarget` accept/reject (incl. the
  `awaiting_interrogation` path) and the delivery-degraded refusal.

## Migration / Compatibility

No migration, no wire change. `LegacyMap` preserves the map shape and
`LaneReason.String()` preserves the reason strings, so CLI/MCP/web responses
are byte-identical. Phases 1 and 2 may land together or in sequence; Phase 3
is independent and non-gating.

## Risks

- `Facts` is a wide struct the read view constructs by hand → a field-mapping
  mistake is silent. Mitigate with a shared loader helper and the `pgtest`
  integration test.
- `db.TxRunner` vs `db.Runner`: the delivery caller runs in a transaction while
  `Check` takes `db.Runner`. Resolution: keep the `FOR UPDATE` check in the
  mutation; `Check` is the non-transactional pre-check and derivation.
- `Probe` returning `supervisor.LaneLiveness` couples `lanehealth` to
  `supervisor` types. Acceptable — `supervisor` is a leaf and already owns the
  tmux probe vocabulary.

## Alternatives Considered (design-it-twice)

- **DB behind a `RowStore` port (ports-&-adapters).** Rejected: a single
  production adapter plus an in-memory fake that contradicts the `pgtest`
  testing convention; the pure `Classify` already delivers DB-free testing of
  the rules, which is the hard part. (Its disciplined probe-port and its
  removal of the `:859` bypass are kept.)
- **Flexible loader with batch / skip-options / cache.** Rejected as YAGNI:
  the dashboard's N-probe cost is preexisting and not worsened. Escape hatch:
  a later `AssessMany` that leaves `Check`/`Classify` untouched.
- **Bare minimal struct without predicates.** Superseded by adding
  `Health.LiveTarget()` / `CanDeliver()` so callers read intent rather than
  recomposing axes at four sites.

## Revisit Triggers

- A dashboard probe-cost profile that justifies a batch `AssessMany`.
- The agent-loop earning first-class byline attestation (would add or change an
  axis / `Attested`'s inputs).
- Moving delivery's transactional consistency check behind the module.
