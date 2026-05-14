# Striatum UI Rework — Implementation Handoff

Status: design handoff for Codex implementers
Date: 2026-05-13
Scope: redesign of the local web UI (`striatum serve --web`) and the
       terminal dashboard (`striatum dashboard`) for operator-driven runs.
Source of truth: `docs/SPEC.md`, `docs/UBIQUITOUS_LANGUAGE.md`,
       `docs/DECISION_LOG.md`, RFCs 0013/0022/0023/0024/0026/0029/0037/0038/0040,
       the live schema (`src/striatum/schema.py`), the live templates under
       `src/striatum/web/templates/`, and the V1.41 / V1.42 burn-down recorded
       in `CHANGELOG.md`.

This document is a design handoff, not a published artifact. It carries no
`author:` line — if and when it ships through the runner the publisher will
compute the canonical byline.

---

## 1. Design Intent

The redesigned operator surface is an **operations console** for a local
orchestration runner. The operator's day is: prepare a run, watch lanes
claim → ack → publish → complete, triage stale leases and process-adapter
blockers, override the occasional verdict, and audit the byline/attestation
chain when something looks wrong. The UI exists to make that loop fast and
honest. It is not a marketing surface, not a "dashboard sea" of vanity
metrics, and not a remote control plane.

The redesign keeps three rules from RFC 0022 / RFC 0037 / RFC 0038 /
D073 / D092 in place — server-rendered Jinja2 with React islands where
component complexity justifies them, owner-only loopback (D058 / D083),
and the CSP / mutation-gate boundary in `service.py`. It adds dense
information layout, a recovery-first run-detail page, and parity with
`striatum dashboard --once` so the same vocabulary works in either surface.

### Claims the UI must NOT make

- **No model-author forgery.** A session that is not lane-attested must
  never render as `author: <role>-<model>-<ordinal>`. Unattested
  publishes show `author: operator` or `author: operator [self-declared:
  <label>]` (RFC 0026; D080). Pin via regression test.
- **No calibrated lane provenance for unattested sessions.** Chips, hover
  cards, and badges must not imply a calibrated binding to a model
  process when `sessions.lane_attestation = 'unattested'`. Always render
  the reason sub-text (`no_attached_supervisor`, `pid_identity_mismatch`,
  `pid_gone`, `lane_command_mismatch`, `session_mismatch`, `run_mismatch`,
  `session_missing`).
- **No transcript capture.** D028 is enforced by construction. The UI
  must not propose a "live terminal output" panel, must not mirror a
  supervised child's stdout/stderr, must not stream a `--verbose-log`
  side channel by default. The process-adapter diagnostic envelope
  (`blockers.payload_json`) is the only legitimate post-exit detail.
- **No externalized live state.** `.striatum/state.sqlite3` is the live
  authority (D006/D007/D009). Templates and islands read the existing
  HTTP API; they do not synthesize their own state machines, do not
  cache run state across navigations, and do not write SQLite directly.
- **No verdict laundering.** An override verdict (`verdicts.source =
  'operator_override'`) must always render the operator's rationale
  beside the verdict pill; it must never visually substitute for the
  original natural verdict. The original row stays visible.
- **No silent recovery promises.** Buttons that would call CLI verbs
  whose preconditions are not met (e.g. `recovery resume` against a
  terminal job without `--force`) must be disabled and tell the
  operator why, not fail at the server with a generic error.

### Two corner cases worth pinning up front

1. **Drifted attestation mid-flight.** A session can attest at
   `claim-next` and lose attestation before `publish-artifact` (the
   supervisor dies). The UI must read attestation at the time the
   artifact row was recorded (`artifacts.author_line` is the literal
   line read from disk per HARNESS-003), not recompute from the
   session's current row. See GH #2 investigation in CHANGELOG v1.42.0.
2. **Operator-on-behalf publish via `recovery auto-publish`.** When the
   runner publishes for a dead session, the audit event is
   `recovery.auto_published` with `summary="auto-published on stale
   lease (V1.41 harness friction burn-down)"`. The artifact row's
   author byline is whatever was read from disk. The UI must show that
   provenance explicitly — both the original session id (now closed)
   and the operator-on-behalf trail.

---

## 2. Primary Operator Flows

Each flow lists the deterministic CLI sequence, the screens that participate,
and the specific affordances the redesign must surface. Mutation gates
(`serve --allow-mutations`) and operator-confirm gestures are called out.

### Flow A — Start a run

CLI sequence (driven mostly from a terminal):

```
striatum init
striatum workflow validate <workflow.json>
striatum run prepare --workflow <workflow.json>
striatum branch confirm --run-id <id> --branch <name> [--create | --use-current | --strict]
striatum run start --run-id <id>
```

UI scope:

- `/workflows/<rel_path>` — “Run this workflow now” button (already wired
  in `workflow_detail.html`) hits `POST /workflows/run/<rel_path>` which
  internally chains `run prepare` and returns `{run_id, status,
  suggested_branch_name}`. The redesign keeps this; the response status
  must visibly drive the next-action banner on `/run/<id>`.
- `/run/<id>` when `runs.state = 'needs_branch_confirmation'` — the
  existing inline `branch-confirm-panel` becomes the single primary
  affordance on the page. Cancel and Pause are hidden in this state.
  Default to **records-only** confirmation unless the operator ticks
  Create-if-missing or Use-current-git-branch; the radio for `--strict`
  is the safe default for CI / scripted runs.
- The Workflows index already exposes "New workflow" → `/workflows/new`
  (RFC 0038 5c). No changes; the chooser wizard's six steps stay.

`striatum dashboard --once` parity: a `needs_branch_confirmation` run
must render `branch confirm <run_id> --branch <name>` as the single
top-of-next-actions entry.

### Flow B — Claim-next and publish-artifact loop

Driven from a supervised or operator-driven CLI inside an active run
(HOW_TO_AGENT.md). The UI's job is **observability**, not control:

- `/run/<id>` shows lanes claiming, jobs flipping
  `queued → claimed → running → completed`. The job-list rail is the
  primary navigation; clicking jumps to `/run/<id>/job/<wfjob>`.
- `/run/<id>/job/<wfjob>` shows the latest verdict (review jobs), the
  artifacts list, blockers, and process-adapter execution evidence.
- A new `ExpectedArtifactsTable` (see §5) renders the work-packet's
  declared `expected_artifacts` next to actual published rows so the
  operator sees the gap immediately when a packet writes one path and
  forgets another. Each row exposes a copyable
  `striatum publish-artifact --path <p>` recipe with `--kind` and
  `--logical-name` defaulted from `expected_artifacts[]` (V1.41 A3 —
  `_resolve_publish_defaults`).

### Flow C — Verdict and override-verdict (V1.41 ergonomics)

Natural verdicts arrive via `submit-review` or `verdict`. The UI's job is
to surface the verdict pill with its **provenance**:

- Natural: `accept` / `accept_with_findings` / `needs_revision` /
  `reject` chip, provenance dot = filled.
- Operator-override: same chip, provenance dot = hollow, with the
  override rationale inlined immediately below (never collapsed by
  default). The original row stays visible above the override.

For the override flow itself:

- `/run/<id>/job/<wfjob>` exposes an **Override verdict** button when
  the latest verdict on a completed-or-`waiting_human` review job is
  non-accepting. Modal collects `--verdict {accept,
  accept_with_findings}`, optional `--findings-artifact-id`, and a
  required `--rationale`. The modal also exposes the V1.41
  `--auto-fresh-session` checkbox: when checked, the modal first
  registers a fresh operator reviewer session on the same lane and
  uses it; otherwise the operator must supply a different session id
  manually (the pre-V1.41 dance).
- `dispatch.py::_resolve_override_session` is the runtime gate; the UI
  surfaces it as an inline form-validation rule so an over-eager
  operator sees "this session already issued a verdict for this job;
  enable auto-fresh-session or pick a different session" before
  submitting.
- The "Next actions" banner promotes
  `override-verdict --auto-fresh-session ...` when an open
  non-accepting verdict has no downstream cycle to absorb it (the
  B2-class next-action surface).

### Flow D — Recovery triage

Recovery splits into four sub-flows, all reachable from `/run/<id>`'s
new Recovery panel and from `striatum dashboard --once`:

1. **Stale-lease recovery** — open blockers panel groups by reason.
   Review-only stale leases get an inline "Requeue review work"
   button mapped to `striatum recovery requeue-stale --run-id <r>
   --job-id <j>`. Repo-write stale leases display a non-actionable
   warning chip and a copy-on-click recipe for
   `striatum recovery cancel-job --run-id <r> --job-id <j> --reason
   "..." [--cascade]` (D036's hand-inspection requirement is
   preserved by construction — the UI does not auto-recover repo-write
   leases).
2. **Process-adapter blocker** — for every blocker whose
   `blocker_kind` starts with `process_`, a new `ProcessExecutionEvidence`
   block renders the diagnostic envelope verbatim
   (`exit_code`, `duration_seconds`, `timeout_seconds`,
   `missing_artifact_paths`, `review_verdict_missing`,
   `recovery_commands`). Each `recovery_commands[]` entry becomes a
   one-click "Copy recipe" affordance.
3. **Human-checkpoint blocker** — distinct severity chip and an
   inline `striatum checkpoint resolve --blocker-id <id> --action
   {continue, cancel}` (existing mutation buttons; the V1.42
   terminal-blocker dismiss path becomes an explicit
   "Dismiss legacy blocker on terminal job" disabled-by-default
   button — see §4 `BlockerTriagePanel`).
4. **Auto-publish on stale lease (V1.41 A1)** — open blockers whose
   shape matches the auto-publish gate (stale lease + on-disk file at
   declared `expected_artifacts[].path` + on-disk byline canonicalises
   exactly to `expected_author_line`) render a green
   `recovery auto-publish --run-id <r> [--dry-run]` recipe at the top
   of the panel. Always start with `--dry-run`; the redesign hard-codes
   that.

### Flow E — Cross-run audit

The audit surface answers: "who wrote this, with what attestation, and
did it ever get overridden?" Four read-only inspections:

- **Byline integrity** — `/run/<id>/artifact/<artifact_id>` shows the
  literal `artifacts.author_line` (or `<missing>` when the file omitted
  it) next to the work-packet `expected_author_line` at publish time.
  When they disagree, the row is flagged amber and the difference is
  pre-expanded.
- **Verdict provenance** — `/run/<id>/posture/<posture>` (existing
  `run_posture_verdicts.html`) gains a `provenance` column showing
  `natural` / `operator-override (rationale)` / `cycle-revised`.
- **Override reasoning** — same page; the override rationale is its
  own column, never the hover tooltip. Long rationales get
  show-more/show-less, never display:none.
- **Attestation state** — every artifact and verdict row carries a
  `LaneAttestationChip` derived from the row's session at the row's
  time, not the session's current state. Hover reveals the reason
  vocabulary and an optional `supervisor_id` link to
  `/run/<id>/supervisor/<sid>` when present.

### Flow F — `striatum doctor` problem groups + per-record disposition

`/doctor` already groups by `check` and supports the "hide problems on
terminal runs" toggle (RFC 0037 step 4). The redesign:

- Adds **per-record disposition** affordances inline. For each known
  `check`, the row exposes the exact recovery CLI verb that resolves
  it (e.g. `supervisor_lost_with_held_lease` → recipe for
  `striatum recovery cancel-job --reason "..."` + `striatum supervise
  stop --session-id <s>`; `editable_install_outside_repo` → no
  affordance, just docs link).
- Pins V1.41 burn-down checks. New (or extended) check names the UI
  must render explicitly:
  - `byline_drift_on_disk` (when `artifacts.author_line` and
    `expected_author_line` disagree)
  - `auto_publish_candidate` (stale lease + on-disk artifact ready
    for `recovery auto-publish`)
- Adds a "Recently dismissed" section for V1.42-style
  `recovery.blocker_dismissed_terminal` events so the operator can
  audit what was forgiven without re-opening triage.

---

## 3. Information Architecture

### Top-level navigation

`base.html`'s header keeps four primary links: **Runs · Workflows ·
Chat · Doctor**. The header also keeps the UTC/Local toggle, the
keyboard-shortcut help (`?`), and gains one secondary link: **Repo**
(→ `/view/`). Five entries is the cap.

```
┌─────────────────────────────────────────────────────────────────────────┐
│ STRIATUM   Runs  Workflows  Repo  Chat  Doctor              UTC  ?      │
└─────────────────────────────────────────────────────────────────────────┘
```

The brand wordmark is plain typography, not a logo lockup. No marketing
hero, no animated wordmark, no theme switcher (the dark/light follows
`prefers-color-scheme` per D073).

### First-viewport contents per page (1440 wide, 900 tall)

`/` (run list):

```
Runs                                              [filter pills][search]
─────────────────────────────────────────────────────────────────────────
Run ID         State    Branch       Workflow ID   Created    Duration
run_aabbcc...  running  striatum/df  rfc-0038      2m ago     2m running
run_001122...  failed   striatum/df  rfc-0029      3h ago     1h 14m
…
```

Above the fold: filter row + first 20 rows of the table. No banners,
no "you have 3 unresolved runs" cards. Empty state stays the rich
empty already shipped by RFC 0037.

`/run/<id>`:

```
Run run_aabbcc…                                  [Cancel] [Pause]
[status pill: running] on striatum/df-053

NEXT ACTIONS  (only when run is non-terminal AND actions exist)
- recovery auto-publish --run-id run_aabbcc…  (suggested)
- claim available work for role=reviewer lane=codex
─────────────────────────────────────────────────────────────────────────
                       │   Status      Created   Started   Completed
   Jobs (rail)         │   …
   draft        ✓      │
   review_a    ●       │   Recovery (open blockers)
   build      ▢        │   process_outputs_missing  ▶ envelope + recipes
   …                   │
                       │   Verdicts by posture
                       │   security 2  threat_model 1  ergonomics_dx 1
                       │
                       │   [Graph SVG]
```

Above the fold: header + pill + next-actions banner + the jobs rail
+ the top half of the status + recovery panel + graph. The
verdicts-by-posture list moves into the right column under the graph,
not above it — graph wins at first viewport because it answers
"what is the run doing right now?" fastest.

`/run/<id>/job/<wfjob>`:

```
← run_aabbcc…
Job draft_codex                                  [Cancel job] [Retry job]
[status pill: completed] designer · codex
[verdict pill: accept_with_findings · operator-override]  (open inline)
   rationale: operator accepted findings: see <decision_id> …

EXPECTED ARTIFACTS                ACTUAL
striatum/053/design/codex/DRAFT…  ✓ published  sha:abc…  author:operator
striatum/053/design/codex/PLAN.md  ✗ missing   recipe: striatum publish-artifact --path …

Process execution evidence
   exit_code: 0   duration: 1247s   timeout: 1800s
   missing_artifact_paths: [striatum/053/design/codex/PLAN.md]
   recovery_commands: [...]

Artifacts published
   …
```

Above the fold: header + pill + verdict + expected-vs-actual table
+ process evidence. The artifacts list is below — it's reference, not
the decision-driving payload.

`/workflows/` and `/doctor` stay close to their current shapes; the
filter toolbar from RFC 0037 stays as-is. Empty states retain the
rich-empty SVG from RFC 0037 / Step 9.

### Narrow screens (375, 768)

Mobile is **inspectable, not fast**. Above 768 the page keeps the rail +
center + graph three-column layout; below 768 it collapses to a single
column in this order:

1. Page header + status pill + primary CTA (Cancel / Pause / Confirm
   branch).
2. Next-actions banner (full-width).
3. Recovery panel — open blockers first.
4. Jobs rail becomes a horizontally-scrolling chip strip
   (`overflow-x: auto`, no wrap), each chip is role+lane+state.
5. Status kv-grid.
6. Verdicts by posture.
7. Graph (panel collapsed by default below 768 — tap to expand).

Claim-next, publish-artifact, override-verdict, and recovery
auto-publish are **desktop-first**. The mobile design must not pretend
they fit a 375 viewport. Mutation buttons drop to a single column of
full-width buttons; modals become full-screen overlays with a single
sticky CTA at the bottom.

### V1.41 burn-down verbs in the recovery panel

`/run/<id>` recovery panel surfaces all three V1.41 verbs as
copy-on-click recipes:

- `striatum byline --session-id <s> --job-id <j>` — inline button on
  any artifact row whose byline column is amber-flagged.
- `striatum inbox --session-id <s>` — button on any active session row
  in the supervisors strip.
- `striatum recovery auto-publish --run-id <r> --dry-run` — banner at
  the top of Recovery when at least one open blocker matches the
  auto-publish gate. Pressing it does NOT execute the recipe; it
  copies the literal argv string. Mutation requires the operator's CLI.

`striatum dashboard --once` surfaces the same three verbs in its
**Next actions** column when the introspection layer produces them. The
two renderers consume the same `status_payload["next_actions"]` so the
single string list is the parity contract.

---

## 4. Screen Specifications

For each major template: purpose, visible data, controls, empty state,
loading state, error state, disabled state. All exact strings are
quoted.

### 4.1 `run_list.html` — `/`

**Purpose.** Operator triage entry point. Answer "what's running, what
finished, what failed."

**Visible data.** One row per run: `run_id` (mono), state pill, branch
(mono), workflow id (mono), created_at (`<time>`-tagged), duration
(`Xm Ys` for terminal, `Xm running` for non-terminal). Filter toolbar
above the table from RFC 0037.

**Controls.**
- Search input — substring match on `run_id`, `branch`, `workflow_id`.
- State filter pills: `all` / `running` / `completed` / `canceled` /
  `failed` / `paused`. The pills mirror the schema-visible run states
  + the derived `paused` (from `paused_at IS NOT NULL`). No `compromised`
  pill yet — see §10 OQ-1.
- Date filter: `all` / `24h` / `7d` / `30d`.
- Each row's `run_id` cell is a click target to `/run/<id>`. The
  whole `<tr>` carries the data attributes the filter JS uses so
  clicks don't have to retarget.

**Empty state.** RFC 0037-shipped rich-empty SVG, copy:
"No runs yet. Run `striatum run prepare --workflow <workflow.json>` to
create your first run; see docs/HOW_TO_HUMAN.md."

**Loading state.** Server-rendered; no skeletons. If the page first-paints
without a `<script id="run-list-data">` payload, the filter row hides.

**Error state.** `service.py`'s 500 templates already render plain
text; the run list keeps that. No client-side error overlay.

**Disabled state.** Filter pills behave as toggles. The state filter
pill `paused` should be disabled when zero runs are paused, with
`title="No paused runs"`.

### 4.2 `run_detail.html` — `/run/<run_id>`

**Purpose.** The operator's home for one run. Triage decisions made
here.

**Visible data (above the fold).**
- Header: `Run <code>run_<hex></code>`, status pill, branch (mono),
  paused pill if applicable.
- Action buttons (top right): **Cancel run**, **Pause/Resume run**,
  **Branch confirm** (only when state == `needs_branch_confirmation`).
  Order is consistent — destructive last.
- **Next actions banner** (top, full width) for non-terminal runs
  with at least one action. Each action renders as a list item with
  a "Copy recipe" affordance on hover. Banner is in document flow,
  not sticky (RFC 0037 OQ resolution).
- **Recovery panel** (right column, top) — open blockers, grouped by
  severity (`human_checkpoint` first, then `blocked`). Each blocker
  row uses `BlockerTriagePanel` (§5).

**Visible data (below the fold).**
- Status kv-grid (already in template): State / Created / Started /
  Completed.
- Verdicts by posture list with chip-style links to
  `/run/<id>/posture/<posture>`.
- Graph SVG (existing `graph_svg` Jinja injection).
- Sessions strip — one chip per registered session with
  `LaneAttestationChip` + `BylineLine` (§5). New.
- Events list (recent N) — append-only, oldest first.

**Controls.**
- Cancel/Pause/Resume/Branch-confirm are unchanged from current
  template; their existing `static/run_*.js` files load via
  `{% block scripts %}`.
- Override-verdict and recovery-auto-publish buttons appear inside the
  Recovery panel and the Next-actions banner, not at the run-header
  level.

**Empty state.** Zero jobs (impossible on a started run, but reachable
on `prepared`/`needs_branch_confirmation`): jobs rail shows "No jobs
yet — confirm the branch and `run start`." with a button to focus the
branch-confirm form.

**Loading state.** Server-rendered. No skeletons. SSE (`GET
/v1/runs/<id>/events`) optionally pushes incremental updates to the
next-actions banner and the events list; failure to connect must not
break the page (the SSE handler logs to console only).

**Error state.** A run id that returns 404 renders `service.py`'s
existing 404. A run row that exists but whose
`workflow_snapshot_id` is missing renders an inline `<div
class="banner error">` with the diagnostic and a copyable
`striatum doctor --run-id <id>` recipe.

**Disabled state.** Cancel is disabled (and gets `title="Run already
terminal"`) when `runs.state IN ('completed','failed','canceled')`.
Pause is disabled when state is not in `('ready','running')`. Resume
appears only when `paused_at IS NOT NULL`.

### 4.3 `job_detail.html` — `/run/<run_id>/job/<workflow_job_id>`

**Purpose.** Inspect one workflow job. Decide override / retry /
cancel.

**Visible data.**
- Breadcrumb to run.
- Header: `Job <code>workflow_job_id</code>`, state pill, role+lane,
  latest verdict pill with `VerdictChip` + `provenance` slot, posture
  chip (only when non-neutral).
- **Expected artifacts table** (§5 `ExpectedArtifactsTable`) — the
  V1.41 publish-defaults source of truth. Always rendered for jobs
  that declared `expected_artifacts`; empty rows show the
  missing-artifact recipe.
- **Process execution evidence** block (§5
  `ProcessExecutionEvidence`) when the job has a row in
  `process_executions`. Shows the privacy-safe diagnostic envelope
  literally.
- Job details kv-grid: Job ID, Type, Attempt, Created, Started,
  Completed, Lease state, Owning session id (mono).
- Artifacts list (all rows from `artifacts` for this job_id) —
  `BylineLine` per row.
- Verdicts list (all rows from `verdicts` for this job_id), newest
  first, each with `VerdictChip` + provenance + rationale (always
  inline, never collapsed by default).
- Blockers list (all `blockers` for this job_id whose state = 'open'
  or 'resolved' in the last 24h) — see `BlockerTriagePanel`.

**Controls.**
- Cancel job — enabled when `jobs.state IN ('queued','running',
  'blocked','ready','claimed')`. Already wired (`static/job_actions.js`).
- Retry job — enabled when `jobs.state IN ('failed','canceled',
  'blocked')`.
- **Override verdict** (new) — enabled only when the latest verdict
  is `needs_revision` or `reject` AND the job state is in
  `('completed','waiting_human')`. Opens a modal with the V1.41
  `--auto-fresh-session` flow.
- **Publish artifact recipe** (new, copy-only) — under each missing
  expected artifact, the literal
  `striatum publish-artifact --session-id <s> --job-id <j> --lease-id
  <l> --path <p>` argv with `--kind` and `--logical-name` defaulted
  from `expected_artifacts[]` (V1.41 A3). Copy-on-click only; the UI
  does not POST publish-artifact.

**Empty state.** A job with zero artifacts and zero verdicts (queued
or blocked job): expected-artifacts table renders with all rows in
the "not yet" state; process-evidence block hides; artifacts list
renders empty.

**Loading state.** Server-rendered. Static.

**Error state.** Unknown job_id → 404 from `service.py`. A job whose
`run_id` doesn't match the URL run_id → 400 (treat as malformed URL,
not data inconsistency).

**Disabled state.** Override-verdict is disabled when the latest
verdict is already accepting; tooltip: "Override is only available
when the latest verdict is `needs_revision` or `reject`." Cancel and
Retry follow the state matrix above.

### 4.4 `run_posture_verdicts.html` — `/run/<run_id>/posture/<posture>`

**Purpose.** Audit verdicts grouped by adversarial posture (RFC 0018).

**Visible data.** Existing table. The redesign adds two columns:

- `Provenance` — `natural` / `operator-override` / `cycle-revised`
  (derived from `verdicts.source` plus revision-cycle bookkeeping).
  Chip-style, monospace.
- `Attestation` — `LaneAttestationChip` for the issuing session at
  the verdict's creation time, not the session's current state.

The existing rationale row collapses to a `<details>` when longer than
240 characters; otherwise it renders inline. Override rationales are
never collapsed.

**Controls.** None (read-only page). The breadcrumb back to
`/run/<id>` stays.

**Empty state.** Existing: "No verdicts recorded with posture <code>X
</code> for this run." Keep verbatim.

**Loading / error state.** Server-rendered, static. 404 on unknown
posture (existing).

### 4.5 `workflows_index.html` — `/workflows`

**Purpose.** Find a workflow to inspect, edit, or run.

**Visible data.** Path (mono), workflow_id (mono), status pill,
last-modified (`<time>`-tagged), jobs / lanes / roles counts. RFC 0037
filter row + status pills + search.

**Controls.**
- `+ New workflow` → `/workflows/new` (RFC 0038 chooser wizard).
- Each row click → `/workflows/<rel_path>`.

**Empty state.** RFC 0037 rich-empty + copy: "No workflow.json files
found. Run `striatum workflow generate <path> --shape minimal
--lane-set local --artifact-root striatum/<name>` to create one; see
docs/HOW_TO_HUMAN.md."

**Loading state.** Server-rendered. Static.

**Error state.** Per-row: invalid workflows render with the
`workflow_error` / `parse_error` status pill and a `title="<message>"`
tooltip on the pill. The redesign promotes this to an inline error
strip under the pill on hover/focus so screen-reader users see the
detail.

### 4.6 `workflow_detail.html` — `/workflows/<rel_path>`

**Purpose.** Inspect one workflow file.

**Visible data.** Existing: status pill, version, jobs / lanes /
roles / edges / cycles tables, graph SVG. Validation error block
when status != "valid".

**Controls.**
- **Edit** — primary button (RFC 0038 5a promotion already shipped).
  Icon + label, next to "Run this workflow now".
- **Run this workflow now** — primary button. POST to
  `/workflows/run/<rel_path>`. On success redirects to `/run/<id>`;
  on `needs_branch_confirmation` redirects to `/run/<id>` with the
  branch-confirm panel pre-focused.

**Empty / loading / error states.** Existing behavior. Validation
errors render in a `<pre class="code-pre">` block.

**Disabled state.** "Run this workflow now" is hidden when
`workflow.status != "valid"`. Edit is always enabled.

### 4.7 `workflow_new.html` — `/workflows/new`

**Purpose.** Scaffold a new workflow via the RFC 0034 / RFC 0038
chooser wizard.

**Visible data.** Six-step wizard (RFC 0038 5c) inside the
`workflow-chooser` island: shape → lane-set → modifiers → required
fields → preview → confirm + save.

**Controls.** Wizard-internal. The save step uses the existing
operator-confirmation gate (`POST /workflows/generate` with
`confirm_write: true`).

**Disabled state.** When `allow_mutations = False`, the wizard's
final Save step is replaced by a banner: "Mutations are gated.
Restart `striatum serve` with `--allow-mutations` to write a
generated workflow. Preview remains available."

### 4.8 `workflow_edit.html` — `/workflows/edit/<rel_path>`

**Purpose.** Edit one workflow's JSON via the visual graph editor +
structured form fallback.

**Visible data.** Two stacked panels:

- `workflow-graph-editor` island (RFC 0038 5d) — react-flow graph
  with node palette, per-node inspector, validation badges.
- Form-fallback sections (existing `workflow_edit.js`): header,
  roles, lanes, jobs, edges, cycles. Stays in the page so a
  bundle-loading failure doesn't strand the operator.

**Controls.** Save / Cancel at the bottom. Save POSTs
`/workflows/edit/<rel_path>` with `If-Match: <workflow_sha256>` so
out-of-band edits collide deterministically.

**Empty state.** New file (`is_new = True`): the graph editor opens
with one draft job seeded from a minimal template.

**Error state.** `edit-error-banner` (existing div) renders:
- `If-Match` mismatch → "Workflow changed on disk since you opened
  the editor. Reload to merge."
- Validation error → render `WorkflowError` message verbatim.

**Disabled state.** Save is disabled (and labeled "Validating…")
while client-side schema-shape checks haven't returned.

### 4.9 `doctor.html` — `/doctor`

**Purpose.** Triage cross-run runner health.

**Visible data.** Status pill (ok / issues found) + count. Grouped
problem list per RFC 0037 step 4. Each `<details>` is a `check` name
with a problem count and a docs anchor link.

**Controls.**
- **Hide terminal-run problems** toggle (RFC 0037).
- New: **Recovery recipes per check** — each problem record exposes
  an inline "Copy recipe" affordance with the exact CLI verb that
  resolves it. Closed-vocabulary mapping (see §5
  `BlockerTriagePanel.next_actions`):

  | `check` | recipe |
  | --- | --- |
  | `supervisor_lost_with_held_lease` | `striatum recovery cancel-job --run-id <r> --job-id <j> --reason "supervisor exited"` |
  | `process_running_but_pid_gone` | `striatum recovery process-reconcile --run-id <r>` |
  | `process_running_with_expired_lease` | `striatum recovery process-reconcile --run-id <r>` |
  | `worktree_path_missing_on_disk` | `striatum worktree release --worktree-id <w>` |
  | `skills_missing` / `skills_outdated` | `striatum skills install --profile <p>` |
  | `active_session_on_terminal_run` | `striatum session close --session-id <s> --reason "run terminal"` |
  | `reviewer_independence_unverified` | docs link only (no mechanical recovery) |
  | `byline_drift_on_disk` (new) | `striatum byline --session-id <s> --job-id <j>` |
  | `auto_publish_candidate` (new) | `striatum recovery auto-publish --run-id <r> --dry-run` |

**Empty state.** RFC 0037 rich-empty SVG + "0 problems found. Nothing
to triage."

**Loading / error state.** Server-rendered. Static.

**Disabled state.** Recipes are always copy-only; never disabled.
The hide-terminal toggle is disabled (and labeled "no terminal
problems") when there are no terminal-run problems.

### 4.10 `chat.html` / `chat_index.html` — `/chat`, `/chat/<session_id>`

**Purpose.** Interact with the local chat provider (RFC 0023). The
redesign keeps the page shape; two adjustments:

- The "Operator confirmation required" tool-confirmation block must
  render the `tool_input` JSON inside a syntax-highlighted block
  (reuse the `code-viewer` island with `language=json` and
  `collapsible=false`). The current `<pre class="tool-result">` is
  fine for the operator-confirm panel of dogfood-lifecycle tools
  (RFC 0040 V1) where the argv is short.
- Add an inline session-state strip in the chat header:
  `LaneAttestationChip` (always `operator`-flavored because chat
  drives the operator-on-behalf surface, never a lane), the active
  capability set (e.g. `read`, `write`, `claim`, etc.), and the
  mutation-gate status.

**Empty / loading / error / disabled states.** Existing behavior
preserved. The four chat-provider env vars and the loopback-only
non-HTTPS rule stay as-is.

### 4.11 `artifact_view.html` — `/run/<id>/artifact/<artifact_id>`

**Purpose.** Inspect a published artifact.

**Visible data.** Existing metadata (artifact_id, sha256, size,
created, author_line). Two new rows:

- **Byline integrity** — when `artifacts.author_line` ≠
  publish-time `expected_author_line`, render an amber chip and
  inline the diff. Otherwise render a quiet "matches expected"
  micro-line.
- **Provenance evidence (future / GH #5 backlog)** —
  `ProcessExecutionEvidence` summary chip and a link to the matching
  `process_executions` row. Until GH #5 ships, render the chip in
  the `provenance evidence: not yet correlated` muted state — never
  render a green "attested provenance" claim for unattested
  operator-on-behalf publishes.

**Controls.** Raw-bytes link (existing). Copy-to-clipboard on
sha256, artifact_id, repo_path.

**Empty / loading / error / disabled states.** Existing. Markdown
rendering stays server-side via `markdown.py`.

### 4.12 `view_file.html` — `/view/<path>`

**Purpose.** Inspect any repo file (RFC 0023 V1.5 / RFC 0038 5e).

**Visible data.** Header with rel-path and size. Body:
- For `.md` files: server-rendered Markdown via `markdown.py`.
- For everything else: the `code-viewer` island (RFC 0038 5e).

The redesign adds a path-context strip at the top: when the file
belongs to a known kind (e.g. `striatum/0040-rfc-0040.../EVIDENCE.md`),
render a breadcrumb back to the relevant run (best-effort regex match
against `runs.branch_name`).

**Controls.** Copy-to-clipboard on path; raw-bytes link.

**Empty / loading / error / disabled states.** Existing. Files > 5MB
render a "too large to inline" message + raw-bytes link.

### 4.13 `view_tree.html` — `/view/`

**Purpose.** Browse the target repository as a file tree (RFC 0038
5b).

**Visible data.** The `tree-browser` island over `GET /v1/repo/tree`.
Existing. The redesign keeps the tree island as the only content of
the page.

**Controls.** Click directories to expand; click files to navigate
to `/view/<path>`.

**Empty state.** "Repository is empty." (would never happen in
practice; cover the case anyway.)

**Loading state.** The island renders a skeleton until the first
`GET /v1/repo/tree` returns.

**Error state.** Per-node load failure → polite live-region
announcement (existing island behavior); the tree node stays in
"failed to expand — retry" state.

**Disabled state.** Path traversal is rejected server-side (`..`,
leading `/`, null bytes → HTTP 400). The island never builds
traversal-shaped URLs.

---

## 5. Component Inventory

These are the reusable components Codex should refactor toward. Each
lives at the path called out, supports both the web surface (HTML +
CSS + optional TS island where interactivity is needed) and a
text-mode equivalent for `striatum dashboard --once`.

Every web component must:

- Accept its data as props (TypeScript `interface` in
  `src/striatum/web/frontend/src/shared/types.ts` when the component
  is island-mounted; Jinja2 macro params when server-rendered).
- Apply CSS via semantic tokens (§7); no literal hex codes in
  component CSS.
- Carry a stable `data-component="..."` attribute for snapshot tests.
- Be keyboard-reachable and screen-reader-labeled (every chip with
  state has an `aria-label` that expands its short text).

### 5.1 `RunStatePill`

**File.** `src/striatum/web/frontend/src/shared/components/RunStatePill.tsx`
(React); Jinja2 mirror as `{% macro run_state_pill(state) %}` in
`templates/_components.html`.

**States.** Closed enum, schema-aligned:

```
needs_branch_confirmation
ready
running
blocked
completed
failed
canceled
```

Plus two derived/backlog states:

- `paused` — derived in the view layer from `runs.paused_at IS NOT NULL`
  combined with `state IN ('ready','running')`. The pill swaps to
  `paused`; the underlying state stays in the schema.
- `compromised` — **future-backlog** per GH #3 / V1.7. Pill class
  exists in CSS (`--status-compromised`); template emits the pill
  only when the column is added (§10 OQ-1).

Note: the prompt's list mentions `prepared`. The live schema does not
have `prepared` — `run prepare` produces a row in
`needs_branch_confirmation` directly (D026 / SPEC §Branches and
Commits). The pill follows the schema; `prepared` is not a render
state. If a future RFC restores it, the pill extends.

**Props.** `{ state: RunState; pausedAt?: string | null; classModifier?:
string }`.

**Terminal renderer.** Single ASCII chip in
`dashboard.py::_render_left_column` and the header line, using the
state literal (no abbreviation).

### 5.2 `JobStatePill`

**File.** Same module as `RunStatePill`.

**States.** Closed enum from the schema:

```
blocked queued ready claimed running completed
failed canceled skipped stale_lease waiting_human
```

**Props.** `{ state: JobState; classModifier?: string }`.

**Terminal renderer.** Letters reuse the existing dashboard graph
panel mapping
(`Q`/`R`/`C`/`B`/`H`/`F`/`P`/`X`/`S`) per RFC 0016. In the table
list view, render the full word.

### 5.3 `VerdictChip`

**File.** `…/shared/components/VerdictChip.tsx`.

**Variants.** `accept` / `accept_with_findings` / `needs_revision` /
`reject`.

**Provenance slot.** Required prop. One of:

- `natural` — solid dot under the chip; no extra text.
- `operator-override` — hollow dot + small text
  "override · <short rationale>". Hover/tap expands a card with the
  full `verdicts.rationale`. The rationale is **never substituted for
  the original verdict** — `job_detail` renders the original verdict
  row above the override.
- `cycle-revised` — striped dot; the chip notes the cycle ordinal
  (e.g. "cycle 2 of 3").

**Props.** `{ verdict: VerdictKind; provenance: VerdictProvenance;
rationale?: string; cycleIndex?: number; cycleLimit?: number }`.

**Terminal renderer.** `accept` / `accept_with_findings` /
`needs_revision` / `reject` plus `(override)` suffix when applicable.
Dashboard renders only the count by variant; per-verdict
provenance is reserved for `striatum status --run-id <id>
--json` consumers.

### 5.4 `LaneAttestationChip`

**File.** `…/shared/components/LaneAttestationChip.tsx`.

**States.**
- `attested` — green chip.
- `unattested` — amber chip with required `reason` sub-text. Reason
  vocabulary (closed set; matches RFC 0026 + `session_lane_attestation`):
  - `no_attached_supervisor`
  - `pid_gone`
  - `pid_identity_mismatch`
  - `lane_command_mismatch`
  - `session_missing`
  - `session_mismatch`
  - `run_mismatch`

**Props.** `{ attested: boolean; reason?: AttestationReason;
supervisorId?: string | null; operatorLabel?: string | null }`.

**Optional supervisor_id link.** When present, the chip's hover card
includes a link to `/run/<id>/supervisor/<supervisor_id>` (page is
future-backlog; see §10 OQ-3). Until that page lands, the link is
rendered as a copy-on-click value rather than an `<a>`.

**Terminal renderer.** `attested` or `unattested:<reason>` as a single
ASCII chip. Dashboard truncates long reasons after the first
underscore segment if width is constrained.

### 5.5 `PostureChip`

**File.** `…/shared/components/PostureChip.tsx`.

**Variants.** Closed set from RFC 0018:

```
neutral devils_advocate security threat_model
latency_performance ergonomics_dx accessibility
compliance_license supply_chain
```

Plus `custom:<name>` rendered as `custom · <name>` chip with no
auto-appended docs sentence.

**Props.** `{ posture: PostureKind | string }`.

**Terminal renderer.** `posture=count` strings on the
"Postures:" row of the dashboard's verdicts column (existing
behavior in `dashboard.py::_render_right_column`).

### 5.6 `BylineLine`

**File.** `…/shared/components/BylineLine.tsx`.

**Purpose.** Render the canonical `author:` line truthfully. Refuses
to be substituted for a free-text author label.

**Variants.**
- Attested lane: `author: <role>-<model>-<ordinal>` (monospace).
- Operator (no label): `author: operator`.
- Operator self-declared:
  `author: operator [self-declared: <label>]`.
- Missing on disk: `author: <missing>` (italic muted).

**Props.** `{ authorLine: string | null; expectedAuthorLine?: string
| null; attested?: boolean }`. Component compares
`canonicalize(authorLine)` against `canonicalize(expectedAuthorLine)`;
on mismatch, renders an amber dot and inlines the diff. Canonicalization
mirrors `_canonical_byline_form` in `src/striatum/artifacts.py`.

**Anti-affordance.** The component must reject (TypeScript-level) any
prop named `displayAuthor` or similar — there is no override prop.
The render is governed entirely by the live row data.

**Terminal renderer.** Exact string from the corresponding evidence
field, no truncation.

### 5.7 `BlockerTriagePanel`

**File.** `…/shared/components/BlockerTriagePanel.tsx`.

**Purpose.** List open blockers for a run or job and surface
deterministic next actions.

**Visible data.** Per blocker:
- `blocker_kind` (mono, e.g. `process_outputs_missing`).
- Severity chip — `human_checkpoint` (red) or `blocked` (amber).
- Created-at timestamp.
- Owning job + lane chips.
- Short blocker description (redacted per the evidence-export rule
  when the page is itself an evidence-style surface).
- **Next actions** (closed-vocabulary recipes):

  | Blocker kind | Severity | Recipes (in order) |
  | --- | --- | --- |
  | `process_outputs_missing` | `blocked` | `recovery auto-publish --run-id <r> --dry-run` (when gate matches), `recovery resume --blocker-id <b> --complete --session-id <s>`, `recovery cancel-job --run-id <r> --job-id <j> --reason "..."` |
  | `process_review_verdict_missing` | `blocked` | `verdict --session-id <s> --job-id <j> --lease-id <l> --verdict accept_with_findings --rationale "..."`, `recovery resume --blocker-id <b>` |
  | `process_exit_nonzero` | `blocked` | `recovery resume --blocker-id <b> --force` (then `--complete` if the operator chooses), `recovery cancel-job --cascade` |
  | `process_timeout_exceeded` | `blocked` | `recovery resume --blocker-id <b> --force --extend-seconds <n>`, `recovery cancel-job` |
  | `process_lost_with_outputs_missing` | `blocked` | `recovery process-reconcile --run-id <r>` (then `recovery resume`) |
  | `human_checkpoint` | `human_checkpoint` | `checkpoint resolve --blocker-id <b> --action {continue, cancel}` |
  | `stale_lease` (review-only) | `blocked` | `recovery requeue-stale --run-id <r> --job-id <j>` |
  | `stale_lease` (repo-write) | `blocked` | `recovery cancel-job --run-id <r> --job-id <j> --reason "..." [--cascade]` (no auto-requeue per D036) |
  | terminal-job legacy blocker (V1.42) | `blocked` | `recovery resume --blocker-id <b> --force` (renders as "Dismiss legacy blocker") |

The component never invents an unsupported recovery path. If the
blocker doesn't map to a known recipe, the panel renders the
diagnostic envelope and the literal next-action list from the runner
(`status_payload.next_actions`).

**Props.** `{ blockers: BlockerSummary[]; allowMutations: boolean }`.

**Mutation behavior.** Buttons that POST through `/v1/invoke` are
disabled and labeled "mutations disabled" when `allowMutations` is
false. Copy-on-click recipes are always available.

**Terminal renderer.** Each open blocker renders as
`severity blocker_kind job_role/lane next: <first recipe>` on a
single line. The dashboard's "Blockers (open):" panel keeps its
existing per-severity count row; the table fills the gap below
when `--graph-only` is not in force and width permits.

### 5.8 `ExpectedArtifactsTable`

**File.** `…/shared/components/ExpectedArtifactsTable.tsx`.

**Purpose.** Render the work-packet's declared
`expected_artifacts` next to actually published rows; the V1.41 A3
publish-defaults source of truth.

**Visible data.** Per row (sorted by `expected_artifacts[]` order):
- `logical_name` (mono).
- `kind` chip (one of `prompt`, `finding`, `synthesis`, `decision`,
  `marker`, `handoff`, `support_ledger`, `action_item_ledger`,
  `findings_ledger`, `harness_improvement_proposal`, `patch_summary`,
  `test_report`, `other`).
- `path` (mono).
- `required` flag.
- `expected_author_line` (BylineLine variant: read-only).
- **Status column**:
  - `✓ published` (green) with sha256 + actual byline.
  - `▼ missing — required` (amber) with the copy-on-click
    `striatum publish-artifact --path <p>` recipe (defaults
    `--kind` and `--logical-name` from the row).
  - `· missing — optional` (muted).
  - `! byline drift` (amber) when published but
    `actual_author_line ≠ expected_author_line`.

**Props.** `{ expectedArtifacts: ExpectedArtifact[]; actualArtifacts:
PublishedArtifact[]; session: SessionSummary | null; lease: LeaseSummary
| null; }`.

**Anti-affordance.** The table never offers a one-click
publish-artifact POST. The runner's `publish-artifact` requires file
existence and content-hash validation; the UI must not pretend it can
publish what it cannot inspect.

**Terminal renderer.** Single-line per row in
`dashboard.py` when the dashboard is wide enough (>=120 cols).
Otherwise omitted; the count still surfaces in the "missing
artifacts" next-action.

### 5.9 `ProcessExecutionEvidence`

**File.** `…/shared/components/ProcessExecutionEvidence.tsx`.

**Status.** Pairs with GH #5 (`process_executions` lookup) and the
RFC 0014 diagnostic envelope. Renderable today against
`blockers.payload_json`; full RFC 0026/RFC 0027 V1.7 work for
matching artifact rows is **future-backlog**.

**Purpose.** Render the privacy-safe diagnostic envelope from
`blockers.payload_json` and (when GH #5 ships) the matching
`process_executions` row(s) for an artifact.

**Visible data.** Render the envelope literally:

```
envelope_version: striatum.process_adapter.envelope.v1
process_id:      <id>
command:         <argv array, mono>
exit_code:       <int>
duration:        <s>
timeout:         <s or "—">
missing_artifact_paths:
  - <path>
  - <path>
review_verdict_missing: <bool>
recovery_commands:
  - <argv string, copy-on-click>
  - …
```

Never render child stdout/stderr or model output — the envelope
carries none, by D028.

**Future field.** When GH #5 V1.7 lands, add a `provenance evidence`
sub-block:
- `lane_evidence_present` when a matching `process_executions` row
  exists.
- `lane_evidence_missing_operator_override` when the artifact was
  published via `recovery auto-publish` against a dead session
  (operator-on-behalf). Chip color stays amber; the operator
  rationale renders inline.

Until GH #5 ships, render the sub-block in a `not_yet_correlated`
state — never green. See §10 OQ-2.

**Props.** `{ envelope: ProcessAdapterEnvelope; provenanceEvidence?:
ProvenanceEvidence | "not_yet_correlated" }`.

**Terminal renderer.** Dashboard `--once` reproduces the envelope as
a small text block under the blocker row, indented two spaces. The
recovery_commands list is preserved verbatim.

### 5.10 Shared Jinja2 macros

The two surfaces (Jinja2 server-render and React island) share a
prop contract through TypeScript in
`src/striatum/web/frontend/src/shared/types.ts`. For pages where an
island would be overkill (status pills, byline lines, etc.), define
Jinja2 macros in a new `templates/_components.html`:

- `run_state_pill(state, paused_at=None)`
- `job_state_pill(state)`
- `verdict_chip(verdict, provenance, rationale=None, cycle_index=None,
  cycle_limit=None)`
- `lane_attestation_chip(attested, reason=None, supervisor_id=None,
  operator_label=None)`
- `posture_chip(posture)`
- `byline_line(author_line, expected_author_line=None, attested=None)`

Templates currently inline these as classes (`status-pill
status-{{state}}`, `posture-chip`, etc.); the refactor consolidates
the inline markup into macros so the React component and the Jinja2
macro stay symmetric.

---

## 6. Truthfulness And Claim-State Rules

The UI must never overclaim. The table below is the binding rule for
what each status renders, in both web (chip + hover/tap detail card)
and `striatum dashboard --once` (single ASCII chip). Strings in
quotes are exact.

| Domain | Status | Web rendering | Dashboard rendering |
| --- | --- | --- | --- |
| Byline | `attested` | `BylineLine` green "author: \<role-model-ord\>" | `author: <role-model-ord>` |
| Byline | `unattested (no_attached_supervisor)` | `BylineLine` amber "author: operator" + `LaneAttestationChip(reason="no_attached_supervisor")` | `author: operator (unattested:no_attached_supervisor)` |
| Byline | `unattested (pid_gone)` | "author: operator" + `LaneAttestationChip(reason="pid_gone")` | `author: operator (unattested:pid_gone)` |
| Byline | `unattested (pid_identity_mismatch)` | "author: operator" + `LaneAttestationChip(reason="pid_identity_mismatch")` | `author: operator (unattested:pid_identity_mismatch)` |
| Byline | `unattested (lane_command_mismatch)` | "author: operator" + chip with reason | `author: operator (unattested:lane_command_mismatch)` |
| Byline | `unattested (session_mismatch)` | same | `author: operator (unattested:session_mismatch)` |
| Byline | `unattested (run_mismatch)` | same | `author: operator (unattested:run_mismatch)` |
| Byline | `unattested (session_missing)` | same | `author: operator (unattested:session_missing)` |
| Byline | `operator (no label)` | green-muted "author: operator" — never an upgrade, always honest | `author: operator` |
| Byline | `operator (self-declared)` | "author: operator [self-declared: \<label\>]" — italic on the label | `author: operator[self-declared:<label>]` |
| Byline | missing on disk | italic muted "author: \<missing\>" | `author: <missing>` |
| Verdict provenance | `natural` | filled dot under `VerdictChip` | suffix omitted |
| Verdict provenance | `operator-override` | hollow dot + inlined rationale (never collapsed) | `(override)` suffix; full rationale only in `why <verdict_id>` |
| Verdict provenance | `cycle-revised` | striped dot + `cycle N of M` text | `(cycle N/M)` suffix |
| Blocker severity | `human_checkpoint` | red severity chip + ⚠ icon (icon decorative, not a state signal) | `H` letter + leading `*` |
| Blocker severity | `blocked` | amber severity chip | `B` letter |
| Process-adapter outcome | `process_outputs_missing` | amber, "missing required artifacts" | `process_outputs_missing` |
| Process-adapter outcome | `process_review_verdict_missing` | amber, "review without verdict" | `process_review_verdict_missing` |
| Process-adapter outcome | `process_exit_nonzero` | red, "exit code \<n\>" — destructive recovery needs `--force` | `process_exit_nonzero(exit=<n>)` |
| Process-adapter outcome | `process_timeout_exceeded` | red, "timeout \<s\>s" | `process_timeout_exceeded(<s>s)` |
| Process-adapter outcome | `process_lost_with_outputs_missing` | red, "PID gone; outputs missing" | `process_lost_with_outputs_missing` |
| Provenance evidence (V1.7 backlog) | `lane_evidence_present` | reserved green chip — render only when GH #5 lands | reserved |
| Provenance evidence (V1.7 backlog) | `lane_evidence_missing_operator_override` | reserved amber chip with rationale + operator-on-behalf trail | reserved |
| Provenance evidence | `not_yet_correlated` (today) | muted "provenance evidence: not yet correlated" — never green | omitted |

### Two regressions to pin

These two are non-negotiable and must each have a dedicated test in
`tests/test_web_*.py`:

1. **No model byline for an unattested session.** The byline
   computation (`expected_author_line`, `artifacts.author_line`)
   must never emit `author: <role>-<model>-<ord>` when
   `session_lane_attestation(...).attested = False`. The UI must
   render whatever the runner produced; the regression test asserts
   no template emits the model-author shape for an unattested
   session. See §9 acceptance check `regression_unattested_no_model_byline`.
2. **Override rationale always rendered.** Every page that renders
   an `operator-override` verdict must render the override
   rationale prominently and must never silently substitute it for
   the original verdict. Override flows on `job_detail.html` and
   `run_posture_verdicts.html` are the two surfaces. The regression
   test loads each template against a seed fixture with an override
   row and asserts the rationale string is present in the DOM and
   not inside a default-collapsed `<details>` element.

---

## 7. Visual System

The redesign borrows the RFC 0022 V1 / D073 palette + spacing
conventions and tightens them. No new color, no new fonts, no
gradients beyond a single subtle 1px border treatment for elevated
chips.

### 7.1 Typography

- **System font stack** for body text — unchanged from RFC 0022.
  `font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto,
  "Helvetica Neue", Arial, sans-serif;`. No web font load.
- **Monospace stack** for identifiers, byline lines, paths, and
  CLI recipes. `font-family: ui-monospace, SFMono-Regular, "SF Mono",
  Menlo, Consolas, monospace;`.
- **Scale.** Single modular scale, base 14px on the page, 13px in
  rails and tables, 12px in muted captions. Headings: `h1` 22px,
  `h2` 17px, `h3` 15px. Line-height 1.5 across body; 1.4 in tables.
- **Identifiers.** `run_<hex>`, `job_<hex>`, `art_<hex>`,
  `sess_<hex>`, `proc_<hex>`, `super_<hex>`, `lease_<hex>` always
  render in `code` with the monospace stack. The web layer never
  wraps mid-token and never ellipsizes without surfacing a copy-on-
  click affordance.

### 7.2 Spacing and density

- 4px grid scale: `--space-1: 4px`, `--space-2: 8px`, `--space-3:
  12px`, `--space-4: 16px`, `--space-5: 20px`, `--space-6: 24px`,
  `--space-8: 32px`.
- Table row padding: `8px 12px`. Compact mode (`--density-compact`)
  drops to `6px 8px` for the recovery panel and the
  expected-artifacts table.
- `gap` is the spacing primitive for flex/grid rows; never use
  per-element margins.

### 7.3 Color tokens

Existing tokens in `base.css` stay (`--bg`, `--bg-elevated`, `--fg`,
`--fg-muted`). The redesign adds **semantic status tokens**:

```css
:root {
  /* Run / job state */
  --status-prepared:                   #6e7681;
  --status-needs-branch-confirmation:  #d29922;
  --status-ready:                      #8b949e;
  --status-queued:                     #6e7681;
  --status-claimed:                    #2f81f7;
  --status-running:                    #2f81f7;
  --status-blocked:                    #d29922;
  --status-stale-lease:                #d29922;
  --status-waiting-human:              #db6d28;
  --status-completed:                  #3fb950;
  --status-failed:                     #f85149;
  --status-canceled:                   #8b949e;
  --status-paused:                     #a371f7;
  --status-compromised:                #ff7b72; /* V1.7 backlog only */

  /* Attestation */
  --attestation-attested:              #3fb950;
  --attestation-unattested:            #d29922;
  --attestation-warn:                  #db6d28;

  /* Verdict provenance */
  --override-marker:                   #a371f7;
  --override-marker-bg:                #2d2545;

  /* Posture (reuse current chip palette) */
  --posture-neutral:                   #8b949e;
  --posture-security:                  #f85149;
  --posture-threat-model:              #db6d28;
  --posture-ergonomics-dx:             #2f81f7;
  --posture-accessibility:             #3fb950;
  /* …rest reuse the RFC 0018 palette */
}
```

Light-mode tokens are defined by `prefers-color-scheme` overrides in
`base.css` per RFC 0022 V1.

Style choices the redesign forbids:

- No purple-dominant theme. `--override-marker` is the only purple
  family; it's reserved for the verdict-override semantic so it
  doesn't compete.
- No beige/tan, brown/orange dashboard chrome. The neutral surface
  is GitHub-style dark gray (`#0d1117` family) or matching light.
- No dominant slate/blue gradient sweep on hero blocks. The only
  blue is `--status-running` / `--status-claimed`.

### 7.4 Tables and lists

Single table style — `.data-table` — used everywhere a tabular
entity is rendered (runs, jobs, verdicts, blockers, expected
artifacts, workflows, doctor problems). No card grids for the same
entity type. The existing `.data-table` class in `app.css` is the
canonical implementation; extend it with the `--density-compact`
modifier instead of forking a second table style.

List style — `.kv-grid` — used for status / detail key-value pairs.
No mixing of `dl` and `table` for the same entity.

### 7.5 Icons

The redesign keeps icons sparse and decorative. Allowed:

- Edit pencil on the workflow-detail Edit button (RFC 0038 5a).
- Skip-link arrow.
- Empty-state SVGs (RFC 0037).
- Severity ⚠ on the human-checkpoint blocker chip (decorative —
  state is still encoded in the chip's color + label).

Forbidden: status icons in place of words (no green checkmarks
substituting for "completed"; the pill is the source of truth).

### 7.6 Plot treatment

The only plot in the UI is the SVG dependency graph on
`run_detail.html` and `workflow_detail.html`. The redesign keeps the
layered top-down layout, the state-colored nodes (using the same
status tokens), and the click-to-navigate behavior.

Hover tooltip (RFC 0037 step 5) renders job name, role, state,
duration. The redesign expands the tooltip with `LaneAttestationChip`
+ `VerdictChip` (when present) for the node's session/verdict so the
graph is read-only in the same vocabulary as the detail pages.

### 7.7 Mono-identifier handling

Every identifier rendered in `<code>` carries a click handler that
copies the text to the clipboard, with a 1.2-second toast confirming.
Implementation: a single `static/copy_on_click.js` hooked from
`base.html` to every `<code>` matching `^(run|job|sess|art|proc|super|
lease)_[0-9a-f]+$`. No per-template wiring.

---

## 8. Implementation Map

Each section names the proposed UI change, the files it touches, and
whether it is safe frontend-only work or requires backend / model
support.

### 8.1 `RunStatePill` / `JobStatePill` / `VerdictChip` /
`LaneAttestationChip` / `PostureChip` / `BylineLine` consolidation

**Files.**
- New: `src/striatum/web/templates/_components.html` (Jinja2 macros).
- New: `src/striatum/web/frontend/src/shared/components/*.tsx` (React).
- New: `src/striatum/web/frontend/src/shared/types.ts` — extend with
  `RunState`, `JobState`, `VerdictProvenance`, `AttestationReason`,
  `BlockerKind`, `ProcessAdapterEnvelope` exports.
- Existing templates that use inline `status-pill`/`posture-chip`
  classes: `run_list.html`, `run_detail.html`, `job_detail.html`,
  `workflow_detail.html`, `workflows_index.html`,
  `run_posture_verdicts.html`, `doctor.html`, `artifact_view.html`,
  `chat_index.html` — refactored to call macros.

**Backend / model support required.** None. Frontend-only.

**Dashboard parity.** `src/striatum/dashboard.py::render_frame` reuses
the same status / attestation / posture / byline / verdict literals.
The two pill renderers (`_render_left_column`, `_render_right_column`,
header) share a small set of constants in
`dashboard.py::JOB_STATE_ORDER`, `VERDICT_ORDER`,
`BLOCKER_SEVERITY_ORDER`; the redesign adds `RUN_STATE_ORDER` and
`ATTESTATION_REASON_ORDER` for the same purpose.

### 8.2 Next-actions banner promotion on `run_detail.html`

**Files.**
- `src/striatum/web/templates/run_detail.html` — move the
  `.next-actions-banner` to the top of the run-grid (RFC 0037 step 6).
- `src/striatum/web/static/run_detail.js` — listen for SSE
  `next_actions_changed` events when the SSE stream gets that event
  kind (backlog; today the banner is server-rendered only).
- `src/striatum/web/static/app.css` — add `.next-actions-banner` /
  `.next-actions` styles using semantic tokens.

**Backend / model support required.** None for V1; SSE incremental
update is future-backlog.

**Dashboard parity.** Existing — both surfaces consume
`status_payload["next_actions"]` (`introspect.py::next_actions`).

### 8.3 Recovery panel + `BlockerTriagePanel`

**Files.**
- `src/striatum/web/templates/run_detail.html` — new right-column
  panel inserted between Status and Verdicts-by-posture (or as the
  first section above the rail; pick by viewport — see IA above).
- New Jinja2 partial `templates/_recovery_panel.html` rendering the
  blocker triage list.
- New island: `src/striatum/web/frontend/src/islands/recovery-panel/`
  for the auto-publish dry-run preview and the copy-on-click
  recipes. The island is optional — the page renders correctly
  without JS (recipes are pre-rendered as `<code>` blocks with the
  click handler from §7.7).
- `src/striatum/service.py::_render_run_page` (or wherever the
  payload is shaped) — add the precomputed
  `recovery_panel_payload` field with grouped blockers,
  per-blocker `next_actions`, and the auto-publish-gate boolean.

**Backend / model support required.** Three pieces of work on the
runner side; all already exist in V1.41:

- `striatum recovery auto-publish --run-id <r> --dry-run` returns
  the per-row gate (byline + path match). The UI calls the same
  shape via `POST /v1/invoke` only when the operator clicks the
  copy-recipe; otherwise it reads from the precomputed payload.
- `striatum recovery resume --blocker-id <b>` (existing) +
  `--force` (existing) + `--complete` (existing).
- The doctor checks the panel surfaces (§4.9).

The V1.42 terminal-blocker dismiss path
(`recovery.blocker_dismissed_terminal` event) is already in
`recovery.py:439`. The panel renders the dismiss button with the
existing argv.

### 8.4 `ExpectedArtifactsTable` on `job_detail.html`

**Files.**
- `src/striatum/web/templates/job_detail.html` — inject the new
  table between header and Details.
- `src/striatum/service.py` — shape the job-detail response with
  the work-packet's `expected_artifacts_json` (already on the
  `jobs` row) and the matching published `artifacts` rows.
- New Jinja2 partial `templates/_expected_artifacts_table.html`.

**Backend / model support required.** None. The V1.41 A3 publish
defaults are computed by `dispatch.py::_resolve_publish_defaults`;
the UI reuses the per-row mapping by reading the workflow's
declared artifacts.

### 8.5 `ProcessExecutionEvidence` on `job_detail.html` and
`artifact_view.html`

**Files.**
- `src/striatum/web/templates/job_detail.html` — render the envelope
  block under blockers.
- `src/striatum/web/templates/artifact_view.html` — render a stub
  block tagged `provenance evidence: not yet correlated` until GH #5
  ships.
- `src/striatum/service.py` — surface `blockers.payload_json` as a
  parsed envelope in the page payload.

**Backend / model support required.** Today the envelope is
available on `blockers.payload_json` (RFC 0014 V1). The
`provenance evidence` sub-block is future-backlog; ship it as a
muted "not yet correlated" chip until GH #5 V1.7 lands the
`process_executions` lookup.

### 8.6 Override-verdict modal on `job_detail.html`

**Files.**
- `src/striatum/web/templates/job_detail.html` — add the modal
  shell with `<dialog>` semantics.
- New: `src/striatum/web/static/override_verdict.js` — collects
  inputs, posts to `/v1/invoke` with the literal argv:
  `override-verdict --session-id ... --job-id ...
  [--auto-fresh-session] --verdict ... --rationale ...
  [--findings-artifact-id ...] --json`.
- `src/striatum/service.py` — the existing
  `POST /v1/invoke` whitelist already routes `override-verdict`
  through `striatum.api.invoke` when `--allow-mutations` is on.
  No changes required.

**Backend / model support required.** None. V1.41 B1
(`override-verdict --auto-fresh-session`) is shipped.

### 8.7 V1.41 burn-down surfaces — `byline` / `inbox` /
`recovery auto-publish`

**Files.**
- `src/striatum/web/templates/run_detail.html` recovery panel —
  copy-on-click recipes for `striatum byline`, `striatum inbox`,
  `striatum recovery auto-publish`.
- `src/striatum/web/templates/_session_chip.html` (new partial) —
  rendered in `run_detail.html`'s sessions strip and on
  `job_detail.html` under the artifacts list. The chip exposes
  `striatum byline --session-id <s> --job-id <j>` and `striatum
  inbox --session-id <s>` recipes inline.
- `src/striatum/dashboard.py::_render_next_actions` — when the
  status payload's `next_actions` contains an auto-publish or
  byline/inbox verb, render it verbatim in the dashboard's Next
  actions column. The runner already emits these via
  `cli/introspect.py::next_actions`.

**Backend / model support required.** None new for the UI surface.
The CLI verbs (`_cli_byline`, `_cli_inbox`,
`auto_publish_stale_artifacts`) already exist. The work is to wire
their CLI invocations into the `status_payload["next_actions"]`
list when the conditions match — that's a follow-up to
`cli/introspect.py::next_actions` and is shared by both surfaces.
Mark as **backend-side follow-up** and track under §10 OQ-4.

### 8.8 Workflow editor — graph-editor island parity

**Files.**
- `src/striatum/web/frontend/src/islands/workflow-graph-editor/` —
  existing island; no schema changes. The redesign asks the
  per-node inspector to expose **lane attestation requirements**
  as a structured field (`require_attested_lane: bool` for review
  jobs; reject for non-review per SPEC §Reviewer Independence).
- `src/striatum/web/templates/workflow_edit.html` — no change.

**Backend / model support required.** None.

### 8.9 Dashboard parity additions in `src/striatum/dashboard.py`

**Files.** `src/striatum/dashboard.py` — extend `_render_*` helpers to
include:

- `RUN_STATE_ORDER` constant for run header.
- `BlockerTriagePanel` text mode (single line per blocker).
- `ExpectedArtifactsTable` text mode (single line per row, gated on
  width).
- `ProcessExecutionEvidence` text mode (the envelope block).
- `LaneAttestationChip` text mode (single ASCII chip beside the
  session line — needs a sessions panel addition to
  `dashboard.py`).

**Backend / model support required.** None; the dashboard reads
the same SQLite views the CLI does.

### 8.10 Backlog items requiring an RFC

These cannot be implemented in this redesign without an explicit
design pass. Each should land as a future RFC. Propose **RFC 0046**
(next available after RFC 0045) for the operator-recovery-UI bundle:

1. `runs.state = 'compromised'` enum value + propagation (GH #3 /
   V1.7). Schema migration, byline-rewrite path, `striatum status`
   and `striatum why` changes, the `--status-compromised` token in
   the UI. **Requires backend.**
2. `process_executions` ↔ artifact correlation for the
   `lane_evidence_present` chip (GH #5 / V1.7). New query +
   migration index. **Requires backend.**
3. `/run/<id>/supervisor/<supervisor_id>` page — a focused view of
   one supervisor row + its lifecycle events. Today this is
   accessible only through `striatum why <supervisor_id>`. **UI +
   small service.py route work.**
4. Operator-side incremental UI for `striatum byline` /
   `striatum inbox` next-action surfacing — the runner-side hook
   into `cli/introspect.py::next_actions`. **Backend follow-up;
   UI follows.**

RFC 0046's working title: **"Operator recovery UI and provenance
honesty"**. Reference RFC 0026 (lane attestation), RFC 0027 (sealed
patch), RFC 0029 (operator recovery for process-adapter blockers),
RFC 0037 (web UI ergonomics), RFC 0038 (web UI feature additions),
and RFC 0040 (MCP-driven dogfood harness) as predecessors.

---

## 9. Acceptance Checks For Codex

Concrete checklist; each item maps to a test under
`tests/test_web_*.py`, `tests/test_dashboard.py`, or a Vitest spec
under `src/striatum/web/frontend/src/__tests__/`. The committed
bundle is the authority; CI must rebuild and refuse drift.

### 9.1 Browser smoke

`tests/test_web_browser_smoke.py` — every template renders against a
seed fixture and contains the expected chips. One assertion per
template:

- `run_list.html` against a fixture with three runs (running,
  completed, paused) — assert one `RunStatePill` per row, the
  duration column rendered, no empty `next-actions-banner`.
- `run_detail.html` against a fixture with one running run, one
  open process blocker, one accept verdict, one unattested
  session — assert `RunStatePill`, `next-actions-banner`,
  `BlockerTriagePanel`, `VerdictChip(provenance="natural")`,
  `LaneAttestationChip(attested=false, reason="no_attached_supervisor")`,
  and the graph SVG.
- `job_detail.html` against a job with one missing required
  artifact and one process-adapter blocker — assert
  `ExpectedArtifactsTable` rows, `ProcessExecutionEvidence`,
  and an Override-verdict button only when verdict is
  non-accepting.
- `run_posture_verdicts.html` against fixtures for each posture in
  RFC 0018 — assert provenance column present.
- `workflows_index.html`, `workflow_detail.html`, `workflow_new.html`,
  `workflow_edit.html` — render against seed fixtures, assert filter
  toolbar / Edit primary button / chooser island mount / graph editor
  island mount.
- `doctor.html` against a fixture with five problem kinds — assert
  grouped `<details>`, hide-terminal-run toggle, per-record recipe
  buttons.
- `chat.html`, `chat_index.html`, `artifact_view.html`,
  `view_file.html`, `view_tree.html` — render against seed fixtures
  and assert the island mount or server-rendered body block.

### 9.2 Responsive screenshots

`tests/test_web_responsive.py` — Playwright screenshots at 1440,
1024, 768, 375 widths for:

- `/run/<id>` with the recovery panel populated.
- `/run/<id>/job/<wfjob>` with the expected-artifacts table
  populated.

Compare against committed reference PNGs under
`tests/responsive_refs/`. The CI bundle-hash check (RFC 0038 step 1)
already refuses drift in `static/build/manifest.sha256`; extend
the same hash discipline to the responsive refs by committing
SHA256s alongside the PNGs.

### 9.3 Byline regression (truthfulness rule 1)

`tests/test_web_byline_truthfulness.py` —

- Seed fixture: one session whose
  `session_lane_attestation(...).attested = False` and whose
  artifact byline is therefore `author: operator`.
- Load `run_detail.html`, `job_detail.html`,
  `run_posture_verdicts.html`, `artifact_view.html` against the
  fixture.
- Assert each rendered DOM contains zero strings matching
  the regex `^author:\s+[a-z]+-[a-z][a-z0-9_]+-[0-9]+$` from a
  session row whose attestation is False.
- Assert each rendered DOM contains
  `<lane-attestation reason="no_attached_supervisor">` (or the
  Jinja2 macro-emitted equivalent).

### 9.4 Override-verdict rendering (truthfulness rule 2)

`tests/test_web_override_verdict_visible.py` —

- Seed fixture: a review job whose latest verdict is
  `accept_with_findings` via `verdicts.source = 'operator_override'`
  with a 240-character rationale.
- Load `job_detail.html` and `run_posture_verdicts.html`.
- Assert the rationale string is present in the rendered HTML.
- Assert the rationale is **not** inside a default-collapsed
  `<details>` (no ancestor `<details>:not([open])` for the
  rationale node).
- Assert the original (`needs_revision`) verdict row is present
  above the override on `job_detail.html`.

### 9.5 No-blocker-promises regression

`tests/test_web_blocker_recipes_truthful.py` —

- Seed fixture: one job in `failed` state with an open
  `process_exit_nonzero` blocker.
- Load `job_detail.html`.
- Assert the `BlockerTriagePanel` recipe for that blocker includes
  `--force` (the runner refuses `recovery resume` on nonzero-exit
  without `--force`; the UI must not pretend otherwise).
- Repeat for `process_timeout_exceeded`.

### 9.6 Keyboard / accessibility

`tests/test_web_a11y.py` (Playwright + axe-core):

- Tab order through the next-actions banner is linear top-to-bottom.
- Every status chip has an `aria-label` that matches the chip's
  short text plus the long form (e.g. "unattested:
  no_attached_supervisor").
- Visible focus ring on all interactive elements (existing focus
  styles in `base.css` are the source; the test confirms
  `:focus-visible` is in the matched ruleset).
- Keyboard shortcuts (RFC 0037 step 6) `g r` / `g w` / `g c` / `g d`
  trigger navigation when focus is on the body.

### 9.7 Unsupported-claim regressions

`tests/test_web_truthful_claims.py`:

- `assert no template emits "author: <role>-<lane>" text for a
  session whose lane_attestation.attested is False` — covered by
  9.3 but also pinned as a single-template-grep test.
- `assert no verdict-override page omits the override rationale` —
  covered by 9.4.
- `assert no blocker view promises recovery via a path that does
  not exist` — covered by 9.5; extend with
  `process_running_with_expired_lease` (no auto-recovery; doctor
  link only) and stale-lease repo-write (no auto-requeue per D036).

### 9.8 `make ui-verify-bundle` refusal preserved

CI continues to run `make ui-build` and compare against
`src/striatum/web/static/build/manifest.sha256` (RFC 0038 V1.5 /
dogfood-045 D099). The redesign must not silently rebuild without
committing — the same `STRIATUM_MULTI_REPO_REQUIRE_PG`-style sentinel
pattern guards the bundle path.

### 9.9 Dashboard parity

`tests/test_dashboard.py::test_dashboard_once_chip_parity` —

- Seed fixture with a run that has at least one unattested session,
  one open process_outputs_missing blocker, one accept_with_findings
  verdict (natural), one operator-override verdict.
- Capture `striatum dashboard --once --run-id <id>` output.
- Capture `/run/<id>` rendered HTML, extract the chip strings.
- Assert the dashboard frame contains:
  - `unattested:no_attached_supervisor`
  - `process_outputs_missing`
  - `accept_with_findings`
  - `accept_with_findings (override)`
- Assert the same strings are present in the HTML (modulo HTML
  entity encoding). The two surfaces must agree on the literal
  status vocabulary.

### 9.10 Browser / CLI parity for V1.41 next actions

`tests/test_web_next_actions_v141.py` —

- Seed fixture matching the auto-publish gate (stale lease + on-disk
  conformant artifact).
- Assert `status_payload["next_actions"]` contains
  `recovery auto-publish --run-id <r> --dry-run` (this is the
  backend-side follow-up in §8.7 OQ-4 — write the test as a marker
  for the work).
- Assert `run_detail.html` renders the same recipe in the
  next-actions banner.
- Assert `dashboard --once` renders the same recipe in the Next
  actions column.

---

## 10. Open Questions

These are the only questions that block implementation. Everything
else is decided above with an explicit assumption.

### OQ-1. Ship `runs.state = 'compromised'` now or defer to V1.7?

**Assumption (safe).** Defer to V1.7 / GH #3.

**Rationale.** The redesign reserves the `--status-compromised`
semantic token and `RunStatePill` future variant. The CSS class
exists so a future schema migration only needs to add the state
value; no template churn at the V1.7 cutover. Pull-up cost today is
not worth the migration + propagation paths.

**Block.** None — the redesign ships without it.

### OQ-2. Render the `lane_evidence_present` chip as a stub today, or wait until the publish-time guard ships?

**Assumption (safe).** Stub today as
`provenance evidence: not yet correlated` (muted). Never render the
green `lane_evidence_present` chip until GH #5 V1.7 lands the
`process_executions` lookup.

**Rationale.** A green chip with no backing data would be the exact
overclaim the prompt forbids. A muted "not yet correlated" chip
honestly states the state of the world.

**Block.** None.

### OQ-3. `/run/<id>/supervisor/<supervisor_id>` — ship in this redesign or future RFC?

**Assumption (safe).** Future RFC (RFC 0046 candidate above).

**Rationale.** The `LaneAttestationChip` hover card references a
supervisor_id and would link to the page; the redesign renders the
value as copy-on-click until the page exists. No template breaks.

**Block.** None.

### OQ-4. Should the runner's `next_actions` emit the V1.41 burn-down verbs (`byline`, `inbox`, `auto-publish`)?

**Assumption (safe-ish).** Yes — the runner's
`cli/introspect.py::next_actions` should emit these verbs when the
status conditions match. This is a small follow-up to V1.41; the UI
work depends on it for the dashboard / web parity.

**Recommendation.** Add a follow-up implementation item against
`cli/introspect.py::next_actions`. Without it, the UI surfaces are
the only places the verbs appear — that violates the parity rule in
§3. Treat as a **blocking-for-acceptance backend change**, but
small enough to land in the same release as the redesign.

**Block.** Yes — the dashboard parity test in §9.9/9.10 will fail
until this lands. Sequence as: backend hook → dashboard renderer →
web surfaces.

### OQ-5. Inline vs collapsed override rationale on the run-list page?

**Assumption (safe).** The run-list page does not render override
rationales at all — too dense. The override marker is the
`VerdictChip` provenance dot only. Full rationale is on
`job_detail.html` and `run_posture_verdicts.html`.

**Block.** None — but flag for design review if a reader expects
rationale on the list.

### OQ-6. Tree browser breadcrumb back to a run?

**Assumption (safe).** Best-effort regex against
`runs.branch_name` matches like `striatum/dogfood-<NNN>-*`. When
the rel-path under `/view/<path>` contains the run's
artifact-root, render a breadcrumb chip linking to the run. When
the heuristic doesn't match, render nothing — never a wrong link.

**Block.** None.

---

## Appendix — File map summary

```
src/striatum/web/
├── templates/
│   ├── _components.html              (NEW — Jinja2 macros for pills/chips)
│   ├── _expected_artifacts_table.html (NEW — used by job_detail)
│   ├── _recovery_panel.html          (NEW — used by run_detail)
│   ├── _session_chip.html            (NEW — used by run_detail, job_detail)
│   ├── artifact_view.html            (extend: byline integrity + provenance stub)
│   ├── base.html                     (no change to nav order; minor)
│   ├── chat.html                     (extend: session-state strip)
│   ├── chat_index.html               (no change)
│   ├── doctor.html                   (extend: per-record recipes)
│   ├── job_detail.html               (extend: expected artifacts + process evidence + override modal)
│   ├── run_detail.html               (restructure: next-actions banner + recovery panel)
│   ├── run_list.html                 (no change; already RFC 0037 shape)
│   ├── run_posture_verdicts.html     (extend: provenance + attestation columns)
│   ├── view_file.html                (extend: path-context breadcrumb)
│   ├── view_tree.html                (no change)
│   ├── workflow_detail.html          (no change beyond RFC 0038 5a)
│   ├── workflow_edit.html            (no change)
│   ├── workflow_new.html             (no change)
│   └── workflows_index.html          (no change)
├── frontend/src/
│   ├── shared/
│   │   ├── components/               (NEW — RunStatePill, JobStatePill, …)
│   │   ├── types.ts                  (extend: closed enums for state, attestation, posture, …)
│   │   └── …
│   ├── islands/
│   │   ├── recovery-panel/           (NEW — optional, copy-on-click + dry-run preview)
│   │   ├── code-viewer/              (no change)
│   │   ├── tree-browser/             (no change)
│   │   ├── workflow-chooser/         (no change)
│   │   └── workflow-graph-editor/    (extend: require_attested_lane per-node field)
│   └── …
├── static/
│   ├── app.css                       (extend: semantic status tokens, .data-table compact mode)
│   ├── base.css                      (extend: status-* tokens, --override-marker, --attestation-*)
│   ├── base.js                       (extend: copy-on-click for identifiers; existing UTC/Local toggle stays)
│   ├── copy_on_click.js              (NEW)
│   ├── override_verdict.js           (NEW — modal + POST /v1/invoke)
│   ├── run_detail.js                 (extend: next-actions live region SSE listener — V1.5)
│   └── …
src/striatum/service.py               (extend: page-payload shapers for recovery panel + expected-artifacts + process-evidence)
src/striatum/dashboard.py             (extend: RUN_STATE_ORDER, sessions panel, blocker triage text mode, evidence text mode)
tests/test_web_*.py                   (NEW + extend per §9)
tests/test_dashboard.py               (extend per §9.9)
docs/design/UI_REWORK.md              (THIS FILE)
docs/rfcs/0046-*.md                   (FUTURE — see §8.10)
```
