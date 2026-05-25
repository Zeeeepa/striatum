# Pattern spec — Iterated Panel Review with Interrogation (ground truth)

Status: operator spec for RFC 0083 + the reusable example + the live run.
Date: 2026-05-25. Author: operator. This file is the single source of truth that
every sub-agent and the live run consume. If something here conflicts with an
older doc, this wins for this effort only.

## 1. What the pattern is

A reusable workflow shape with **two structurally identical loops** — a **design
loop** and a **build loop** — chained design → build. Each loop is:

```
fan-out (3 independent lanes)  →  synthesis  →  interrogating panel review
        ^                                              |
        |___________ revision cycle (needs_revision) __|
```

- **Fan-out (3 lanes):** three independent agents produce independent artifacts
  for the same objective, no cross-talk (`parallel_group`, disjoint write
  scopes). Design loop → three design proposals; build loop → the implementation
  is single-author but reviewed by a 3-wide panel (build fan-out is on the
  *review* side, not duplicate builds, to avoid three conflicting diffs).
- **Synthesis:** one synthesizer reconciles the three design proposals into one
  buildable synthesis (design loop). In the build loop the "synthesis" node is
  the implementer producing the actual change + a HANDOFF.
- **Interrogating panel review:** a `parallel_group` of **3 reviewers** with
  distinct postures (`threat_model`, `ergonomics_dx`, `devils_advocate`). The
  reviewed (synthesizer/implementer) session is **`interrogable: true`** and
  stays live (`awaiting_interrogation`) after it completes, so each panel
  reviewer can **interrogate its preserved context** before rendering a verdict.

## 2. The two bounded iteration concepts (do not conflate)

1. **Interrogation rounds — ≤ 3, early exit on resolved findings.**
   Within a single review, each panel reviewer runs an interrogation thread
   against the live reviewed session: `interrogation.open` → up to **3**
   `ask`/`answer` rounds → `close`. The reviewer **stops early** and closes the
   moment its open findings are resolved by the answers. The cap and the
   early-exit are enforced by the **reviewer role prompt** (the engine does not
   bound ask/answer); the reviewer must state in its finding how many rounds it
   used and why it stopped.

2. **Revision cycle — bounded re-work.**
   If the panel's aggregate verdict is `needs_revision`, the loop returns to the
   synthesis/implement node. Encoded as a `cycle` with
   `on_verdict: needs_revision`, `max_iterations: 2`. Early exit is automatic:
   if no reviewer returns `needs_revision`, the cycle does not fire.

## 3. Execution substrate — agent-loop-first

All lanes run via the **MCP agent-loop** (`striatumd -agent-loop <cmd>`), NOT the
`--print` supervised wrapper. Rationale: interrogation requires **preserved
context** so the reviewed agent can answer from its own working memory; the
`--print` wrapper spawns a fresh process per packet (no memory) and cannot be
interrogated truthfully. RFC 0083 proposes deprecating `--print` for new
workflows *conditional on* the agent-loop functioning for each lane adapter
(claude / codex / gemini) — validated empirically in Phase A.

Lane note: if a given adapter's agent-loop does not function, that lane falls
back to `--print` for *fan-out/review authoring only* and MUST NOT be the
interrogation target. The interrogation target is always an agent-loop lane
(claude is the known-good baseline).

## 4. Schema mapping (striatum.workflow.v1 — no engine changes needed)

- 3-wide fan-out / panel → `parallel_group` + `parallelism.max_active_jobs: 3` +
  `require_disjoint_write_scopes: true`.
- reviewed node interrogable → job `interrogable: true`, `expected_artifacts`
  may be `[]` so `complete` needs no published artifact on the reviewed node
  when its artifact is the synthesis/handoff itself.
- panel reviewers → `type: review`, `reviewer_context_policy: fresh`,
  `reviewer_access_scope: document_only` (NOT `artifact_only` — invalid),
  distinct `review_posture`, each holding the `interrogate` capability at
  register-session time (`--capability interrogate`).
- revision → `cycles: [{from: <review>, to: <synth/impl>, on_verdict:
  needs_revision, max_iterations: 2, allow_same_lane: true}]`.
- verdict vocabulary (artifactcontracts): `accept`, `accept_with_findings`,
  `needs_revision`, `reject`.

## 5. The object task this pattern will be RUN on

Design + build **the interrogation-log UI feature**: render a run's interrogation
Q&A thread as a **chat-style transcript** in the workflow-history web UI. This
depends on closing the F36 gap (the Go web service must actually serve the
React bundle; today `go/pkg/webassets` embeds only app.js/base.css/page.html).
Data source: persisted interrogation turns (`message_kind` =
`interrogation_question` / answer) already exportable via
`striatum trajectory export --profile dialogue`.

The design loop produces the feature RFC; the build loop implements it; the
payoff is viewing *this run's own* interrogation thread as chat in the new UI.

## 6. Naming / paths

- Reusable pattern: `examples/iterated-interrogating-panel/`
- RFC: `docs/rfcs/0083-iterated-panel-review-with-interrogation.md`
- Live run dir: `docs/operator/workflows/interrogating-panel-2026-05-25/`
- Feature artifact root (run output): under the live run dir, design/ + build/.
</content>
</invoke>
