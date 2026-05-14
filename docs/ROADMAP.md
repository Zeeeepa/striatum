# Striatum Roadmap

**Purpose:** This is the operator kickoff document. If you are picking up
Striatum cold — or resuming after a context compaction — read this first.
It sequences the deferred and blocked work in `docs/TODO.md`, the proposed
RFCs under `docs/rfcs/`, the open GitHub issues, and the in-flight
dogfood follow-ups; everything else is the authoritative status source.

This roadmap is *opinionated about ordering*. Items here exist in TODO,
RFCs, GH, or DECISION_LOG already — the roadmap only adds sequencing,
dependency edges, and "what would I do next" framing. Update on every
`vX.Y.0` version bump; treat as stale on minor bumps.

---

## 1. State as of 2026-05-14 (v1.48.2)

- **Latest commit:** `f8afacd` on `main`. Tag `v1.48.2` pushed.
- **Latest substantive release:** v1.48.0 (RFC 0050 V2 + RFC 0051 draft).
- **CI:** in flight on `f8afacd` after 298 consecutive red runs since
  2026-05-08. Two root causes fixed in v1.48.2: Python typecheck
  (16 mypy errors across 7 files) and Go matrix version pin (1.22 →
  1.23 to match `go/go.mod`). Verify with `gh run list --workflow CI --limit 1`.
- **Last successful CI:** commit `2c7237d` (6 days ago).
- **Active dogfoods:** none (056 completed; v1.48.0/1/2 are mop-up commits).
- **Branches:** only `main` survives; orphan branches cleaned in this session.

## 2. Just shipped (this week)

| Version | Scope | Notes |
|---:|---|---|
| v1.46.0 | RFC 0050 V1 (dogfood-054 + 054b) | UI primitives + dashboard parity + 4-finding provenance fix-up. |
| v1.47.0 | RFC 0050 V1.5 (dogfood-055 + 055b) | Template extensions + 3-finding provenance fix-up. |
| v1.48.0 | RFC 0050 V2 (dogfood-056) + RFC 0051 (draft) | Recovery panel island + override modal + copy-on-click + graph-editor data binding. |
| v1.48.1 | claude/gemini wrapper auth fix | `claude --print --permission-mode acceptEdits --allowedTools "Bash"`; gemini `--approval-mode yolo`. Closes the 10+ instance lane-stall pattern at its root. |
| v1.48.2 | CI green | Python typecheck + Go version pin. |

## 3. Operator decision rules — read this before doing any work

These are the patterns you will hit in the first dogfood. They are NOT in
the SPEC; they are operational lore.

### 3.1 Operator-on-behalf publish path (RFC 0046 V1, mandatory)

When an agent lane stalls but the on-disk artifact is valid:

```
striatum ack --session-id <S> --message-id <M> --lease-id <L>
striatum publish-artifact --session-id <S> --job-id <J> --lease-id <L> \
    --kind <K> --logical-name <N> --path <P> \
    --allow-no-process-execution \
    --override-rationale "<one-line reason>"
striatum verdict --session-id <S> --job-id <J> --lease-id <L> \
    --verdict <V> --rationale "<one-line reason>"
```

**Never** publish-on-behalf without `--allow-no-process-execution --override-rationale "..."`.
The 055b implementation now refuses model bylines without the override marker.
Every override gets audit-chained — that's the contract; respect it.

### 3.2 Operator verdict override (RFC 0046 V1)

When a reviewer's `needs_revision` is a packet-design artifact (e.g., the
review packet didn't include the fix-up's HANDOFF, so the reviewer correctly
refused on missing evidence), override after the fix-up dogfood ratifies:

```
striatum override-verdict --session-id <S> --job-id <J> \
    --verdict accept_with_findings \
    --auto-fresh-session \
    --rationale "<cite the fix-up dogfood commits + accepting reviewers>"
```

### 3.3 Fix-up dogfood pattern (054b → 055b → ...)

When an adversarial reviewer finds **V1 non-negotiable** violations:

1. Honestly submit the `needs_revision` verdict — do NOT override pre-fix.
   The run goes to `waiting_human` with a blocker.
2. Scaffold a `<N>b` fix-up dogfood whose implementer's spec is the
   adversarial REVIEW.md itself.
3. After 3-way build review of the fix-up ratifies the fixes, override
   the parent run's `needs_revision` verdict citing the fix-up's commits
   + accepting reviewers.
4. Both runs reach terminal `completed`. Ship as the parent run's version.

This pattern is in `docs/dogfood/054b/OPERATOR_REPORT.md` and `055b/OPERATOR_REPORT.md`.

### 3.4 Wrapper auth contract (v1.48.1)

`.striatum/bin/{claude,codex,gemini}-supervised-wrapper.sh` all enable
shell tool use without an interactive permission prompt:

- claude: `--permission-mode acceptEdits --allowedTools "Bash"`
- codex: `--dangerously-bypass-approvals-and-sandbox -c approval_policy=never`
- gemini: `--approval-mode yolo`

Filesystem boundaries are enforced by the packet's `write_scope`, not by
the CLI's permission system. **If you regenerate or reinstall wrappers,
verify these flags survive** — `striatum skills install --profile all` is
known to sometimes regenerate the wrappers.

### 3.5 Anti-patterns to recognize early

- **claude-no-publish** (was 10+ instances; mitigated by v1.48.1): claude
  wrapper alive, no `claude --print` subprocess produces work, no on-disk
  artifact. Check `$STRIATUM_SCRATCH_DIR/claude-logs/packet-NNNN.log`
  for the agent's last words — usually a permission prompt.
- **gemini-no-frontmatter** (3+ instances): gemini writes a valid review
  but the frontmatter lacks `verdict_intent` / `severity`. Operator must
  edit the file before publish-on-behalf succeeds. Don't fabricate a
  verdict the agent didn't intend — re-read the conclusion text.
- **codex/codex co-blindness** (5+ instances, D095-D098, D100):
  implementer and a reviewer are both codex; reviewer findings cluster
  around the implementer's blind spots, producing `needs_revision`
  verdicts that 2-of-3 cross-lane majority overrides. TODO item 26
  proposes a validator rule.
- **packet-design gap** (observed dogfood-055b/056): fix-up review packets
  inherit the parent's `context.docs` and don't include the fix-up's
  HANDOFF + source diff. The reviewer correctly refuses on missing
  evidence. Either include the fix-up artifacts in the next workflow's
  `context_docs` or expect to override the codex verdict.

### 3.6 Cycle-exhaustion override

When a `needs_revision` verdict has no matching workflow cycle (workflow
declares 0 retries or the retry was already consumed), the runner opens
`blocker_kind: revision_routing` and `human_checkpoint`. Operator decides:

- **Real findings** → spawn a fix-up dogfood (§3.3).
- **Anti-pattern overrides** → record a D-decision (D095, D096, D097,
  D098, D099, D100, D101, D102 are precedents) and override via §3.2.
  Always document the anti-pattern variant in the decision record so
  future operators can recognize.

---

## 4. Active runway (this week, next 1-3 dogfoods)

### 4.1 ← NEXT — Dogfood-057: v1.48.x security hardening (RFC 0050 V2 surface)

**Closes:** [#9](https://github.com/halbritt/striatum/issues/9) (HIGH CSRF on `/v1/invoke`), [#10](https://github.com/halbritt/striatum/issues/10), [#11](https://github.com/halbritt/striatum/issues/11).

**Why next:** #9 is HIGH-severity attack surface introduced when V2
landed. Until it's closed, any operator running `striatum serve` is one
malicious website visit away from arbitrary CLI execution.

**Scope:**
1. **Content-Type validation in `_read_json_body`** (`src/striatum/service.py`)
   — refuse non-`application/json` POST bodies. This alone defeats simple-
   request CSRF.
2. **Origin / Referer enforcement** for non-GET requests when `web_enabled` is true.
3. **Client-side run_id/job_id context validation** in `override_verdict.js`
   — refuse posts whose `data-job-id` doesn't belong to the current URL's run.
4. **`recovery auto-publish --dry-run` strict read-only audit** —
   pin a regression test asserting no `events` row, no lease, no artifact.

**Deliverables:**
- Source changes in `service.py` + `override_verdict.js` + recovery
  module.
- New tests: `test_invoke_csrf_refused.py`, `test_origin_enforcement.py`,
  `test_override_modal_context_validation.py`, `test_recovery_dry_run_no_side_effects.py`.
- `docs/dogfood/057/` scaffold (workflow.json + prompts) per the 6-job
  pattern (synth → review_design → implement → 3-way build review).

**Empirical verification of v1.48.1 wrapper fix:** This dogfood doubles
as the first opportunity to confirm v1.48.1 closed the claude/gemini
stall pattern. Expect **zero operator-on-behalf publishes** if the fix
held. If any lane stalls, capture the packet log and reopen analysis.

**Suggested implementer:** claude or codex. Avoid codex/codex
implementer↔reviewer pairing.

**Verification:** `gh issue close 9 10 11` after the dogfood ships.

### 4.2 Dogfood-058: RFC 0051 V1 implementation (auto-finalize from frontmatter)

**Closes:** RFC 0051 (this is its V1 landing).

**Why next:** RFC 0051 is the operational complement to v1.48.1. The wrapper
fix prevents lane stalls at the *cause*; auto-finalize is the safety net
for genuinely-crashed agents that still wrote a valid artifact. Both
mechanisms together collapse the operator-on-behalf burden by ~80%.

**Scope:**
- New event types `artifact.auto_finalized` and `job.auto_finalized`.
- Lease-tick reconciliation: check declared `expected_artifacts[].path`
  exists, parse, validate frontmatter, byline matches `expected_author_line`,
  file mtime older than 10 seconds (anti-race).
- Auto-finalize sequence: publish-artifact → record verdict (from
  `verdict_intent` for findings) → complete job. Atomic per session.
- Refusal cases preserved: malformed frontmatter, byline mismatch,
  missing required artifact — all fall through to existing lane-stall
  behavior with operator-override available.
- Feature flag `STRIATUM_AUTO_FINALIZE_ENABLE=1` for V1; default-on in V1.1.

**Deliverables:**
- Source changes in supervisor reconciliation tick.
- 4 regression tests (see RFC 0051 §Acceptance).
- One end-to-end dogfood with **zero** operator-on-behalf publishes on
  jobs whose agents wrote valid artifacts (the success criterion).

### 4.3 Dogfood-059: TODO #30 / RFC 0039 V1.6 — Go core CI hardening

**Closes:** [TODO item 30](TODO.md#L527).

**Why now:** v1.48.2 only fixed the Go version pin; the deeper Go-matrix
hygiene from dogfood-047's codex review is still open:

- (F1) `(cd go && go mod tidy)` + commit `go.sum` — already present
  locally; verify CI receives it cleanly.
- (F2) Remove the unauthenticated/no-audit production fallback in
  `go/cmd/striatumd/main.go:49` — serving without `--postgres-url` must
  refuse to bind a socket, not install `AllowAllAuthorizer{}`.
- (F3) `make test-multi-repo CORE=go` must hard-fail when Postgres is
  unavailable (not skip). Add a sentinel assertion that the Go-core
  smoke + audit tests actually executed.
- (F4) Extend `tests/test_daemon_go_smoke.py` to assert unauthenticated
  `daemon.describe` is denied + audit row present.
- (F5) Make `go/pkg/db/audit_race_test.go` + `tests/test_daemon_go_audit.py`
  actually run in CI (CI must provision Postgres for the `CORE=go` matrix).

**Suggested implementer:** claude (Go + Python harness). Deliberately
avoid codex (D101 precedent).

---

## 5. Near-term queue (after the active runway)

Order is **dependency-driven, not preference-driven**. Promote items up
when their blocker clears.

### 5.1 RFC 0050 V2 ergonomics polish

**Closes:** [#12](https://github.com/halbritt/striatum/issues/12) (clipboard hijack), [#13](https://github.com/halbritt/striatum/issues/13) (ghost field).

Low-severity, bundled together. Single dogfood:
- Restrict `copy_on_click.js` `[data-copy]` matching to allowlist container
  classes (`.recipe-list`, `.code-recipe`, `.copyable-token`).
- Purge `require_attested_lane` from `WorkflowGraphEditor.tsx` state on
  job type change.
- Add the 4 ergonomic refinements from dogfood-056's review (recovery
  panel error-state CLI recipe fallback, override modal submit cue, etc.)

### 5.2 RFC 0039 Phase 2 (TODO item 25) — Go core CLI integration

**Now unblocked** since RFC 0043 V1 landed (D094 in dogfood-048). Scope:
- `striatum daemon start --core go` CLI integration.
- Mutating workflow verbs on the Go core (currently delegated to Python).
- Supervised processes implemented in Go (PTY + creack).
- Release artifacts + macOS/Linux CI matrix across `daemon_core={python,go}`.
- `make` wiring for end users.

This is a multi-week phase. May want to split into V2.0 / V2.0.5 / V2.0.6
fragments if the surface grows.

### 5.3 RFC 0048 Daemon-side substrate migration (V2.0 phase)

The daemon's RPC router still delegates single-repo verbs to SQLite-backed
CLI dispatch even after `migrate-repo-local`. Three phases:

- **(A)** Port each `cli/mutations.py` handler to PG-backed daemon-internal logic.
- **(B)** Implement the same handlers in `go/pkg/rpc/` so `--core go` actually
  services single-repo verbs.
- **(C)** Remove the `STRIATUM_DAEMON_REQUIRED=0 + STRIATUM_TEST_HARNESS=1`
  test-harness escape entirely.

Multi-week phase, paired with RFC 0039 Phase 2.

### 5.4 TODO item 26 — Codex/codex pairing validator rule

5 documented instances (D095, D096, D097, D098, D100) of the implementer-
↔-reviewer co-blindness anti-pattern. Soft warning landed; full
refuse-by-default with `--allow-same-model-pairing` override knob is
still open.

**Suggested implementer:** any lane. Small validator extension to
`src/striatum/workflow.py::_validate_lane_constraints`.

### 5.5 RFC 0049 (experimental) — Interactive claude lane via MCP

Spike required to verify Anthropic's billing semantics for PTY-supervised
interactive sessions + Claude Code's interactive-loop stability under
bootstrap-prompt-only headless operation. Motivated by Anthropic's
2026-06-15 plan-credit policy.

**Decision needed:** spike or shelve? V1.48.1's wrapper fix bought us
time; RFC 0049 is now a *capability* RFC (~100× token-per-dollar
improvement on Max 20x) rather than a *blocker*.

### 5.6 RFC 0047 — Decision-record propagation

Closes the GH #3 design surface (now-closed issue had no implementation
beyond an event row). Proposes `compromised` run state + supersession
columns on `verdicts` + propagation logic. V1.8 scope.

### 5.7 Engram memory integration — Striatum Corpus Contract V2

**Driven by:** `~/git/engram/STRIATUM_MEMORY_ROADMAP.md` (Engram-side
roadmap dated 2026-05-14). Engram is positioning to become the local
memory layer for Striatum operators and workflow agents — retrieval-
backed working memory over operator/agent logs, RFCs, designs, reviews,
operator reports, changelogs, git history, issues, blockers, and
generated artifacts.

**What already shipped on our side:**
- `striatum corpus export --since <ref> --out <dir>` (RFC 0044 V1,
  dogfood-046, v1.35.0) — nine JSONL files + `manifest.json`, redacted,
  with replay-stable hashes, under `tenant_id='striatum'` and
  `corpus_id='striatum'`.
- The augmentation-not-dependency boundary regression test in
  `tests/test_cli_corpus_export.py::test_no_engram_imports_or_memory_capabilities_in_striatum`
  pinning that no `import engram` / no `from engram` / no `memory.*`
  capabilities exist in Striatum source.

**What Engram is asking for (Striatum-side asks):**

1. **Corpus Contract V2 RFC** — new Striatum RFC (next free number; would
   be 0052 since 0051 just shipped). Define bundle manifest shape, source
   kinds, required + optional metadata, stable item IDs, content hashes,
   instance and repository identity, privacy/redaction metadata,
   incremental-export watermarks, validation rules, and backward
   compatibility. This is the dependency for everything Engram does
   downstream (their projections, retrieval, context injection); their
   roadmap is gated on it.

2. **Multi-corpus support in the exporter** — emit
   `corpus_id = striatum:<repo-or-instance-id>` rather than the V1
   single-corpus `striatum`. Lets one machine host multiple local
   application memories without mixing separate Striatum projects.

3. **Reciprocal augmentation-boundary record** — extend the V1
   regression test to cover any new Engram-integration entry points so
   the "Striatum runs without Engram" property survives the integration
   phases.

4. **Context-injection policy** (RFC-level decision, not implementation
   yet) — when/how Striatum pulls from Engram, per-packet memory budget
   defaults, which workflows opt into augmentation. Engram lists
   operator-startup summaries, workflow scaffolding, agent-packet prep,
   review-cycle prep, blocker/recovery investigation, and UI/CLI memory
   search as the candidate consumers.

**Open decisions to make before implementation** (from Engram's roadmap
§Open Decisions, applicable to our side):

- Striatum instance identity representation.
- `corpus_id` naming — human-readable, UUID-based, or both.
- Which log streams are mandatory vs. optional.
- How much git diff content to export by default.
- Redaction tier guarantees Striatum commits to before export.
- Incremental-export watermark storage location.
- How to record Engram availability without creating a runtime dependency.
- Default per-packet memory injection budget.

**Suggested implementer:** any lane. Phase 1 is a design RFC + the
contract test; no end-user surface changes yet. Subsequent Striatum-side
phases (multi-corpus exporter, then context-injection integration) are
separate dogfoods.

**Blocked on:** nothing on our side. Engram's Phase 1 (their RFC 0045)
is gated on this Striatum-side contract, so this unblocks them.

**Forward link:** §11 lists the Engram-side roadmap for context;
Engram's full backlog is at `~/git/engram/STRIATUM_MEMORY_ROADMAP.md`.

---

## 6. RFC follow-ups (cycle-exhaustion deltas)

These are codex `needs_revision` findings deferred via D095-D102 overrides.
Each is a list of file:line corrections that should land in a future
dogfood. Order them by impact, not by RFC number.

| TODO | RFC | Origin | Decision | Scope |
|---:|---|---|---|---|
| [27](TODO.md) | RFC 0045 V1.5 | dogfood-043 | D097 | Cycle phase-jump validator gap; strict phase-skip; `phase_id` strict-on-v1; drag-drop dropdown bypass; malformed v1.1 tolerance. |
| [28](TODO.md) | RFC 0040 V1.6 | dogfood-044 | D098 | Codex findings from dogfood-044 build review. |
| [29](TODO.md) | RFC 0038 V1.6 | dogfood-045 | D099 | Real-bundle commit (`make ui-update-lock` + `make ui-build`) + supply-chain polish. **First `reject critical` override.** |
| [30](TODO.md) | RFC 0039 V1.6 | dogfood-047 | D101 | F1-F5 (`go.sum`, auth fallback, hard-fail CI, audit denial test, race test in CI). **Already in active runway as 4.3.** |
| [31](TODO.md) | RFC 0043 V1.5 | dogfood-048 | D102 | Crash-recovery tombstone two-phase; daemon-required default flip; `daemon migrate-repo-local` subparser wiring; e2e tests. **Distinct from D095-D101 — both reviewers had real findings, not co-blindness.** |
| (NEW) | RFC 0050 follow-up | dogfood-056 | (no override) | 5 reviewer findings filed as GH #9-13; 1 ergonomic from claude review. Already in active runway as 4.1 + 5.1. |

---

## 7. Blocked / waiting

| Item | Blocker | Unblock criterion |
|---|---|---|
| 5.2 (RFC 0039 Phase 2) | Was: RFC 0043 V1. | **Now unblocked.** |
| RFC 0049 spike | Anthropic billing semantics for interactive PTY sessions on Max 20x | Spike + measurement. |
| RFC 0048 Phase A | Operator capacity (multi-week) | None — schedulable. |
| Item 32 (Engram-side RFC 0044 Phase 1) | External repo (`~/git/engram/`) | Engram-side work; **not Striatum's TODO**. |
| Item F1 (historical bootstrap fixture) | No active operator demand | Tmux harness retirement; cleanup task. |
| Item 16 (generic language sweep) | No active operator demand | Cleanup task. |

---

## 8. Open GitHub issues

### 8.1 RFC 0050 V2 surface (filed 2026-05-14 by operator from gemini's dogfood-056 review)

Bundle #9 + #10 + #11 into the security hardening dogfood (§4.1); leave
#12 + #13 for ergonomics polish (§5.1).

| # | Sev | Title | Bundle |
|---|---|---|---|
| [9](https://github.com/halbritt/striatum/issues/9) | HIGH | CSRF on `/v1/invoke` — no Content-Type validation | §4.1 dogfood-057 |
| [10](https://github.com/halbritt/striatum/issues/10) | MEDIUM | Override modal trusts DOM `data-*` for job/session IDs | §4.1 dogfood-057 |
| [11](https://github.com/halbritt/striatum/issues/11) | MEDIUM | Recovery panel dry-run relies on CLI-side read-only guarantee | §4.1 dogfood-057 |
| [12](https://github.com/halbritt/striatum/issues/12) | LOW | `copy-on-click` works on any `data-copy` — clipboard poisoning | §5.1 polish |
| [13](https://github.com/halbritt/striatum/issues/13) | LOW | Workflow editor — `require_attested_lane` not purged on type change | §5.1 polish |

### 8.2 Operator-reported (pre-existing or filed during this session)

Each gets a `docs/issues/<N>/` workflow when scheduled (per the type
shipped with gh-16).

| # | Kind | Title | Suggested workflow |
|---|---|---|---|
| [14](https://github.com/halbritt/striatum/issues/14) | bug | Recovery cannot clear terminal-run `process_exit_nonzero` blocker without lease | `docs/issues/14/` (gh-issue-driven 3-job). Triage must decide whether the fix is (a) `recovery checkpoint resolve` accepts the blocker on terminal runs, (b) a new `recovery dismiss-blocker --blocker-id <id>` verb, or (c) the process adapter's post-completion blocker insertion is gated by job state. Real product bug with concrete repro from Engram-side. |
| [15](https://github.com/halbritt/striatum/issues/15) | docs | Clarify PostgreSQL transition guidance (`README.md`, `docs/SPEC.md`, `docs/GETTING_STARTED.md`, `docs/HOW_TO_HUMAN.md`) | `docs/issues/15/` OR fold into RFC 0043 V1.5 follow-up dogfood (item 31). Lighter-weight option: docs-only sweep before item 31 lands. |
| [17](https://github.com/halbritt/striatum/issues/17) | docs | Striatum doc consistency for Engram memory integration | `docs/issues/17/` paired with the Corpus Contract V2 RFC scaffold (§5.7). Engram side is gated on Striatum's contract decision; this issue cleans up the docs that lag the integration direction. |

### 8.3 Resolved this session

| # | Title | Closed by |
|---|---|---|
| [16](https://github.com/halbritt/striatum/issues/16) | Add complete operator initialization prompt | `b9add6f` via `docs/issues/16/` workflow. **First production use of the new GH-issue workflow type.** Verify verdict `accept` severity `info`. End-to-end 21 minutes wall-clock, zero operator-on-behalf publishes — empirically validated v1.48.1's wrapper auth fix. |

---

## 9. Cross-cutting operator concerns

### 9.1 CI health (now)

v1.48.2 fixed the Python typecheck + Go matrix pin. **First CI run post-
fix is in flight at the time of writing.** When it goes green:

- Verify the Multi-repo harness step doesn't fail (env-dependent on PG;
  currently expected to skip).
- Verify the UI build hash check + UI tests pass (caught a placeholder-
  bundle regression historically in dogfood-045 / item 29).
- The `release-check` and `package-smoke` should already be passing per
  v1.45.0+ tagging activity, but confirm.

### 9.2 Test failures not yet fixed

The full `make test` (`pytest`) reports 11 failures locally. Most are
env-dependent (no Postgres / no daemon). Two are pre-existing and worth
filing if not already tracked:

- `test_static_assets_no_external_urls` — bundle contains W3C namespace
  URIs and reactflow.dev help URLs; need whitelist.
- `test_decision_log_rows_under_word_budget` — D094 over budget (439 words;
  budget is lower). Either trim D094 prose or raise budget.

### 9.3 Wrappers regenerate sometimes

`striatum skills install --profile all` (which every supervisor invocation
runs as its `lane.command` prefix) appears to occasionally regenerate the
wrapper scripts under `.striatum/bin/`. After v1.48.1, this is no longer
an active hazard for permission flags (they are committed to git and survive
regeneration), but verify after any future wrapper-template change that
`grep "claude --print" .striatum/bin/claude-supervised-wrapper.sh` shows
the `--permission-mode acceptEdits --allowedTools "Bash"` flags.

### 9.4 Memory items (operator-side)

Read these before driving a multi-step run:

- `/home/halbritt/.claude/projects/-home-halbritt-git-striatum/memory/MEMORY.md`
  — operator lessons learned (dogfood-driven over free-form, autonomous
  run decisions, finalize-without-asking, OPERATOR_REPORT incrementality,
  claude-stall recovery, lane attestation gap, CI poll discipline).

---

## 10. How to kick off a new dogfood

For a fresh operator context. Assumes the target dogfood number is `<N>`
and the scope is one RFC phase or one self-contained fix.

```bash
# 0. Pre-flight
cd /home/halbritt/git/striatum
git status                                 # main, clean
gh issue list --state open --label rfc-XXXX  # know what you're closing
cat docs/ROADMAP.md                        # this doc

# 1. Scaffold
mkdir -p docs/dogfood/<N>/{prompts,roles}
# Copy workflow.json from a recent similar dogfood (056 is the latest V1 + 3-way reviewer pattern)
cp docs/dogfood/056/workflow.json docs/dogfood/<N>/workflow.json
$EDITOR docs/dogfood/<N>/workflow.json     # update workflow_id, context_docs, objective, allowed_paths
# Write per-job prompts pointing at the concrete spec (RFC or REVIEW.md)
$EDITOR docs/dogfood/<N>/prompts/synth.md docs/dogfood/<N>/prompts/implement.md docs/dogfood/<N>/prompts/review_build.md
# Initial OPERATOR_REPORT.md scaffold
$EDITOR docs/dogfood/<N>/OPERATOR_REPORT.md

# 2. Validate + prepare + start
striatum workflow validate docs/dogfood/<N>/workflow.json --json
striatum run prepare --workflow docs/dogfood/<N>/workflow.json --json   # remember run_id
striatum run start --run-id <run_id> --json

# 3. Drive each job (per workflow job in dependency order)
striatum register-session --run-id <run_id> --role <R> --lane <L> --fresh --json
striatum supervise start --session-id <S> --json
striatum claim-next --session-id <S> --json    # may auto-fire under supervisor

# 4. Monitor
striatum why <run_id>     # tail events, see state, see blockers
striatum dashboard --run-id <run_id> --once     # compact frame

# 5. Per-job recovery if a lane stalls
#    First check the wrapper log for the agent's last words:
ls .striatum/scratch/sup_*/{claude,codex,gemini}-logs/packet-*.log
#    Then operator-on-behalf per §3.1, or compose a review per §3.3 if claude.

# 6. Override needs_revision verdicts only after the fix-up ratifies (§3.2)

# 7. Ship
$EDITOR docs/dogfood/<N>/OPERATOR_REPORT.md           # final outcome + decisions
$EDITOR pyproject.toml CHANGELOG.md                   # bump minor or patch
git add -A docs/dogfood/<N>/ pyproject.toml CHANGELOG.md src/ tests/
git commit -m "vX.Y.Z: ..."
git tag vX.Y.Z
git push origin main
git push origin vX.Y.Z
git branch -d striatum/dogfood-<N>-...  || true       # if a branch was used
git push origin --delete striatum/dogfood-<N>-... 2>/dev/null || true

# 8. Update this doc
$EDITOR docs/ROADMAP.md                                # promote what's done, advance the queue
```

---

## 11. Where to look next

| If you want... | Read |
|---|---|
| Authoritative status of any item | `docs/TODO.md` |
| Architectural rationale for a decision | `docs/DECISION_LOG.md` (latest D102) |
| RFC design + acceptance criteria | `docs/rfcs/<NNNN>-*.md` and `docs/rfcs/README.md` index |
| Per-dogfood outcomes + interventions | `docs/dogfood/<N>/OPERATOR_REPORT.md` |
| Operator-facing CLI verbs + skills | `docs/HOW_TO_AGENT.md`, `docs/SPEC.md` |
| Patterns that aren't in SPEC | §3 above, MEMORY.md |
| What's actively broken | §1, §9.1, §9.2 |
| What to do today | §4 (active runway) |
| Engram memory integration (external dependency) | `~/git/engram/STRIATUM_MEMORY_ROADMAP.md` and §5.7 above |

---

## 12. Promotion checklist (update this doc per release)

On every `vX.Y.0`:

- [ ] Move items from §4 to §2 if they shipped.
- [ ] Promote items from §5 to §4 if their blocker cleared.
- [ ] Recompute §7 (blocked) — what's still gated and on what.
- [ ] Add new GH issues to §8.
- [ ] Note any new anti-pattern instances in §3.5.
- [ ] Move §1 forward to the new commit/version/CI state.
