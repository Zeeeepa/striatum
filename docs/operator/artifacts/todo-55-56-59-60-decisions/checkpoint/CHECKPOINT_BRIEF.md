---
schema_version: "striatum.synthesis.v1"
artifact_kind: "synthesis"
inputs: ["docs/operator/artifacts/todo-55-56-59-60-decisions/analysis/55/ANALYSIS.md", "docs/operator/artifacts/todo-55-56-59-60-decisions/analysis/56/ANALYSIS.md", "docs/operator/artifacts/todo-55-56-59-60-decisions/analysis/59/ANALYSIS.md", "docs/operator/artifacts/todo-55-56-59-60-decisions/analysis/60/ANALYSIS.md", "docs/SPEC.md", "docs/DECISION_LOG.md", "docs/TODO.md", "docs/ROADMAP.md", "docs/operator/BRIEF.md", "docs/rfcs/0051-auto-finalize-from-frontmatter.md", "docs/rfcs/0057-corpus-contract-v2.md", "docs/rfcs/0064-review-diversity-enforcement.md", "docs/rfcs/0066-replay-archive-corpus-v2-foundations.md", "docs/rfcs/0067-optional-git-pr-integration.md"]
---

author: synthesizer-claude-code-001

# Blocked-Decision Checkpoint Brief — TODOs 55, 56, 59, 60

This brief is for the human principal. It consolidates the four per-TODO
analysis artifacts into one decision package. It records **no product
decision**. Recommendations below are analytical reading of the evidence so
the human principal can accept, modify, or reject decision text inline.

The four TODOs share one tension: Striatum already has the local-first,
daemon-owned PostgreSQL boundary, the durable-artifact provenance model, and
the operator-on-behalf escape hatch. Each TODO asks where on those rails the
next slice of authority should sit. Decisions made in isolation risk
inventing parallel authorities (workflow-file metadata, plugin connectors,
multi-corpus identity) that re-litigate the boundary. Decisions made
together can keep the daemon as the single live authority and the durable
decision artifact as the single human-readable authority.

---

## TODO 55 — Where Does Accepted Lint Risk Live?

**Question.** Workflow risk lint (RFC 0064) has shipped advisory + strict +
`--override-rationale` + optional `--accepted-risk-decision-id`. The open
product question is: where does an *accepted* risk become durable, queryable
evidence? Not whether lint exists.

**Viable options (from analysis/55).**

- **A. Decision artifact linkage only.** Strict lint cites a decision id;
  no new daemon table. Smallest blast radius; weakest queryability.
- **B. Workflow metadata.** Accepted risks live inside `workflow.json`
  under e.g. `accepted_risks`. Travels with the workflow file; risks
  making a risky workflow look self-approved and confusing the
  authoring-vs-live-state boundary.
- **C. Run-preparation record.** Daemon records accepted risk against an
  immutable workflow snapshot at `run prepare` / `run start`, citing a
  decision artifact. Aligns with current daemon authority model.
- **D. Dedicated daemon accepted-risk table.** First-class PostgreSQL
  table keyed by repo + snapshot + rule + decision id. Most queryable,
  largest implementation surface, tempts `workflow lint` toward becoming
  a mutating command.
- **E. Daemon audit row only.** Strict override writes an audit row.
  Audit rows are metadata, not authority; viable only as corroboration.

**Tradeoffs and evidence.**

- `workflow lint` is currently a CLI-local non-mutating helper. Any
  option that requires lint to mutate daemon state changes a product
  boundary, not just an implementation.
- A run-preparation record (C) maps cleanly to the existing snapshot +
  audit chain and treats authoring-time lint as preview. It avoids
  drift problems where a workflow file is edited after risk acceptance.
- A workflow-metadata model (B) is attractive only for *reusable
  authoring guidance*; without (C) underneath, it does not produce a
  daemon-side queryable fact.
- A dedicated table (D) is overbuilt if it ships before C settles
  whether acceptance is per-source, per-snapshot, per-run, or
  per-repo-policy.

**Implementation impact.**

- A: docs + minor CLI tightening. No daemon mutation.
- B: workflow schema, generator/editor UI, lint-fingerprint stability,
  migration. Daemon optional.
- C: new `run prepare` flow (or new `risk accept` command), daemon
  table or snapshot-attached projection, events, dashboard/web
  projection. Bounded by existing run-preparation paths.
- D: full daemon method + Go handler + CLI route + MCP/UI parity +
  archive/corpus projection + authority guardrails. Largest surface.
- E: deceptively high if it forces lint through daemon RPC.

**Analyst recommendation (analysis/55).** A two-layer policy: durable
human-readable authority is **always a decision artifact**, and the daemon
records accepted risk at run preparation against the workflow snapshot,
citing that artifact. CLI-local lint overrides remain previews. Equivalent
to "Option C anchored by Option A," with the door open to (B) only as
reusable authoring guidance that must still cite a decision and be
re-confirmed at run preparation.

**Recommended option:** **C, anchored by A.**

**Decision text to accept or modify:**

> Accepted lint risks become durable only when recorded by daemon-backed
> run preparation against an immutable workflow snapshot. Every accepted
> risk record must cite a decision artifact (`decision` kind in the
> Striatum decision log or a durable artifact path pending promotion).
> CLI-local `workflow lint --strict --override-rationale` remains a
> non-mutating preview; the rationale string is not durable authority on
> its own. Workflow-file `accepted_risks` metadata is deferred unless and
> until a follow-up decision authorizes it as reusable authoring guidance
> whose entries each cite a decision artifact and are re-confirmed at run
> preparation. `workflow lint` is not promoted to a mutating command.

---

## TODO 56 — Default Auto-Finalize Policy and Dogfood Bar

**Question (two adjacent).**

1. Should default-on auto-finalize replace today's dry-run-default +
   per-workflow opt-in?
2. What evidence bar gates flipping that default?

The bounded slice (RFC 0051 V1) has shipped: `recovery.auto_finalize` is a
daemon method, sweep delegates to it before lazy lease expiry, lives behind
`workflow.recovery.auto_finalize.enabled=true`, never receives `--force`
from the sweep, projects dry-run previews through status/dashboard/web,
emits `artifact.auto_finalized` / `job.auto_finalized` events, and carries
`lane_finalization=auto_from_artifact`.

**Viable options — default policy (analysis/56 §1).**

- **A. Keep current posture indefinitely.** Dry-run default global,
  live opt-in per workflow. No regressions; per-workflow boilerplate
  remains the price of relief.
- **B. Default-on with explicit opt-out.** Workflows inherit
  `enabled=true`; authors set `enabled=false` to suppress. Removes
  boilerplate; changes the meaning of every existing snapshot that did
  not opt in.
- **C. Two-step rollout.** Default-on for an audited tier (e.g.
  `examples/`, internal dogfood fixtures); leave external/target-repo
  workflows on opt-in until follow-up. Introduces a "tier" concept.
- **D. Defer.** Keep dry-run-only globally; revisit on a defined run
  window.

**Viable options — evidence bar (analysis/56 §5).**

- **A. Ship default-on now** on existing pin + v1.48.1 wrapper auth
  evidence.
- **B. N live dogfood successes across ≥2 lane shapes with zero
  contested audit-chain events** (gemini-class stall *and* attested-CLI
  completion).
- **C. ≥30-day field test across the internal dogfood corpus.**
- **D. Coverage gate**: require regression coverage for the enumerated
  failure modes (malformed front matter, byline mismatch, partial
  expected_artifacts, expired lease, in-flight rewrite during grace,
  logical-name conflict with different content) before flipping.

**Tradeoffs and evidence.**

- Eligibility gates are already strict: required-artifact completeness,
  mtime grace, front-matter + byline strictness, lane evidence, lease
  liveness, logical-name idempotency, transcript exclusion. Most of
  these are de facto non-negotiable; the only real knobs are mtime
  grace (`DEFAULT_MTIME_GRACE_SECONDS=30`) and
  `allow_no_process_execution` exposure.
- Operator-on-behalf (RFC 0046 V1) and auto-finalize are now first-class
  peer paths. The v1.48.1 wrapper auth fix removed the urgency that
  drove RFC 0051's default-on push.
- Cross-repo workflows complicate (B): an external target-repo workflow
  author may not want its semantics to change because the Striatum
  daemon upgraded.
- Snapshot durability cuts both ways: a snapshot pinned to "policy at
  snapshot time" is more predictable; a snapshot re-evaluated under
  the new default is cheaper to ship but less honest about provenance.

**Implementation impact.**

- A: zero code. Documentation only.
- B: default flip in workflow validation + snapshot evaluation +
  generator templates + a written rollback plan + audit-chain
  storytelling so a flipped run is distinguishable from an opt-in run.
- C: tier concept added to workflow metadata + curated list +
  refresh discipline + docs.
- D: zero code; calendar discipline.

**Safety-condition split (analysis/56 §2).** Recommend invariants:
required-artifact completeness, front-matter + byline strictness,
run/lease/session/message liveness, logical-name idempotency, transcript
exclusion. Recommend knobs: mtime grace window, `allow_no_process_execution`
(today CLI-only; consider keeping it CLI-only).

**Visibility additions worth ratifying (analysis/56 §3).** Make
`lane_finalization=auto_from_artifact` a first-class column in dashboard
and web run-summary tables so agent-called, auto-finalized, and
operator-on-behalf publishes remain visually distinguishable without a
drill-down. Add a cause-class taxonomy to skipped-candidate reasons
(`mtime_grace`, `byline_mismatch`, `lane_evidence_missing`,
`frontmatter_invalid`, `lease_expired`, `logical_name_conflict`).

**Scheduler choices (analysis/56 §4).** Keep cadence aligned with the
lease-heartbeat tick; keep eligibility-before-expiry ordering as an
invariant; keep sweep-vs-CLI asymmetry (sweep policy-bounded, CLI
break-glass); add a configurable consecutive-failure circuit breaker to
pause the sweep after N failures.

**Recommended option:** **§1 = D (defer the default flip), §5 = B (N live
dogfoods).** This treats the v1.48.1 wrapper-auth fix as already having
relieved the original urgency, makes the flip an evidence gate rather than
a calendar gate, and keeps backwards-compat clean for external target
repos.

**Decision text to accept or modify:**

> The global default remains dry-run-projection; live auto-finalize remains
> opt-in via `workflow.recovery.auto_finalize.enabled=true`. The default
> flip is gated by **N=3 live dogfood successes across at least two lane
> shapes** (one gemini-class artifact-then-stall, one attested-CLI
> completion) with zero contested audit-chain events and zero
> operator-on-behalf publishes triggered as overrides for false-positive
> auto-finalize refusals. Safety conditions A/C/E/F/G in analysis/56 §2
> are invariants; B (mtime grace) and D (`allow_no_process_execution`)
> remain operator-tunable, with `allow_no_process_execution` CLI-only.
> Dashboard and web run-summary tables expose `lane_finalization` as a
> first-class column. Skipped-candidate refusals carry a cause-class
> taxonomy. The recovery sweep keeps lease-heartbeat cadence,
> eligibility-before-expiry ordering, sweep-vs-CLI asymmetry, and gains a
> consecutive-failure circuit breaker (default 5).

---

## TODO 59 — Replay, Archive, and Corpus Contract V2 Foundations

**Question.** RFC 0057 (Corpus Contract V2) and RFC 0066 (replay/archive
foundations) want explicit decisions on: multi-corpus identity, redaction
tiers, context-injection posture, archive content semantics, replay
semantics, and verification depth, all without crossing the local-first
boundary.

**Viable options (analysis/59).**

- **Corpus identity (§1.1).** A: human slug; B: structural hash; C:
  composite slug + hash.
- **Redaction (§1.2).** A: binary public/private; B: graduated tiers
  (`tier_1_public`, `tier_2_operator_prose`, `tier_3_restricted`); C:
  rule-by-kind projection.
- **Context-injection posture (§1.3).** A: passive (Striatum only
  exports; consumer fetches); B: explicit workflow opt-in
  (`augmentation` block produces a reference in the packet; agent
  fetches); C: runner-side fetch (Striatum embeds retrieved bytes).
- **Archive content (§2.1).** A: metadata-only; B: self-contained
  bundle (bytes + transcripts); C: hybrid/configurable per kind.
- **Replay semantics (§2.2).** A: verification replay (re-verify
  hashes + event chain); B: semantic inspection (read-only run state
  in TUI/web); C: comparative replay (re-run the job).
- **Verification depth (§3.1).** A: manifest verification; B: deep
  chain verification with `previous_hash` linking; C: audit-chain
  cross-check against the daemon's `audit_chain_entry` rows.

**Tradeoffs and evidence.**

- Striatum's product-boundary commitments (no transcript capture, no
  external persistence, augmentation-not-dependency, local-first
  always-functional) rule out (§1.3 C) runner-side fetch and rule out
  any archive variant that *requires* network. Striatum must verify
  archives offline.
- Composite identity (§1.1 C) is the only option that survives renames
  *and* remains greppable. Hash-only is opaque for humans; slug-only
  collides.
- Graduated redaction tiers (§1.2 B) are the only model that handles
  the real cases: an operator wants to share decision text but not
  operator prose, or share rule-based fingerprints but not artifact
  bodies. Rule-by-kind (C) is an implementation detail under the
  tier model.
- Self-contained archives (§2.1 B) match the "portable offline-first"
  goal but can be very large for runs with many findings/syntheses.
  Hybrid (C) preserves choice; metadata-only (A) is the cheapest
  default but ties verification to the original repo or blob store.
- Verification replay (§2.2 A) is already implemented and is the
  load-bearing semantics. Semantic inspection (B) is an additive UI
  surface; comparative replay (C) is non-deterministic and out of
  scope until LLMs become deterministic enough to validate the
  comparison.
- Deep chain + audit-chain cross-check (§3.1 B and C) are both
  desirable but C requires daemon access. The defensible default is
  B always-on, C optionally when a daemon is reachable.

**Implementation impact.**

- §1.1 C (composite id): contract V2 surface in `corpus export` / `corpus
  verify`, archive bundling, and the migration story from V1 corpora.
- §1.2 B (graduated tiers): redaction vocabulary, archive/corpus
  projection rules, schema validation, docs.
- §1.3 B (workflow opt-in): new `augmentation` block in workflow JSON,
  packet shape (a reference, not bytes), tests proving Striatum stays
  100% functional when the augmentation source is unreachable.
- §2.1 C (hybrid): config knobs for which kinds embed bytes; sensible
  default (probably "embed decisional + finding + synthesis; reference
  transcripts and large blobs").
- §2.2 A + B: A is already there; B layers a read-only TUI/web mode on
  archived rows.
- §3.1 B always-on + C optional: B is a thorough recompute pass; C is a
  daemon-backed verification that compares archived rows to live
  audit-chain rows.

**Recommended options.**

- Identity: **§1.1 C** (composite `slug:sha256`).
- Redaction: **§1.2 B** (graduated tiers).
- Context injection: **§1.3 B** (workflow opt-in producing a
  reference; agent fetches; Striatum must stay 100% functional when
  the source is unreachable).
- Archive content: **§2.1 C** (hybrid/configurable; default embeds
  decisional/finding/synthesis bytes, references transcripts and
  large blobs).
- Replay: **§2.2 A + B** (verification replay default, semantic
  inspection as additive read-only mode; no comparative replay).
- Verification: **§3.1 B always-on, §3.1 C when a daemon is
  reachable.**

**Decision text to accept or modify:**

> Corpus Contract V2 adopts a composite `corpus_id` of the shape
> `slug:sha256` where `slug` is human-readable and `sha256` is a stable
> hash of the repository identity. Redaction is expressed as graduated
> tiers (`tier_1_public`, `tier_2_operator_prose`, `tier_3_restricted`);
> rule-by-kind projections are an implementation detail under those
> tiers. Context injection is explicit per workflow: workflows may
> declare an `augmentation` block that produces a reference in the work
> packet, and the agent performs the fetch; Striatum must remain 100%
> functional when the augmentation source is unreachable. Striatum
> never embeds fetched bytes runner-side. Archive bundles are
> hybrid/configurable; the default `archive create` embeds bytes for
> `decision`, `finding`, `findings_ledger`, `synthesis`,
> `support_ledger`, `action_item_ledger`,
> `harness_improvement_proposal`, `escalation`, `operator_brief`,
> `work_plan`, `progress_note`, and `operator_report` kinds, and stores
> references for transcripts and S3-backed blobs. `archive verify` and
> `corpus verify` are deep-chain verifications by default and never
> require network access; an optional `--audit-chain-cross-check` flag
> compares archived rows against the daemon's `audit_chain_entry`
> rows when a daemon is reachable. Replay is verification-only by
> default; a future "semantic inspection" mode is additive and
> read-only. Comparative replay is out of scope.

---

## TODO 60 — Optional Git/PR Integration Boundary

**Question.** Should Striatum grow Git/PR integration beyond RFC 0067's
read-only local snapshot slice? If so, on what authority, with what
credentials, and against which providers?

**Decision is five separable axes (analysis/60 §"Decision Package").**

1. **Commit authority.** A: never; handoff only. B: explicit-confirm
   local commits from a durable commit-request artifact. C: workflow-opt-in
   autonomous local commits.
2. **Hosted boundary.** A: never. B: optional plugin/connector
   outside core. C: first-class core integration.
3. **Confirmation model.** AI operator, human principal, or
   workflow-declared (with human-only for hosted).
4. **Credential boundary.** External-only credentials; no durable
   persistence; explicit answer on whether redaction-safe provider
   URLs/IDs may be recorded.
5. **Implementation sequencing.** Read-only snapshots → commit
   request artifacts → optional local commit apply → optional
   provider plugin.

**Tradeoffs and evidence.**

- D017 already records that the coordinator starts/selects a branch and
  requests a commit from the human; any commit-creating slice changes
  that decision and must say so.
- The current product boundary explicitly excludes hosted services,
  cloud APIs, telemetry, transcript capture, and external persistence.
  First-class hosted PR creation (axis-2 C) is incompatible with that
  boundary without a deliberate product expansion.
- The "least-disruptive" path identified by the analyst is read-only
  local snapshots (already RFC 0067) plus durable commit-request and
  PR-request artifacts as durable provenance, with local commit apply
  requiring explicit confirmation and hosted actions remaining manual
  or behind an optional plugin.
- Credentials must remain outside durable state: no provider tokens in
  workflow JSON, artifacts, archives, corpus exports, audit payloads,
  or `.striatum/` scratch.

**Implementation impact.**

- Axis-1 A: zero new mutation; commit-request artifact surface only.
- Axis-1 B: durable commit-request artifact schema, daemon-backed
  commit-apply method, write-scope re-validation at apply time, hook
  policy decision, refusal on dirty trees outside scope, audit events.
- Axis-1 C: everything in B plus workflow opt-in policy, identity
  policy, rollback semantics, post-commit provenance.
- Axis-2 A: zero new code.
- Axis-2 B: plugin contract, credential adapter pattern, redaction
  policy for provider URLs/IDs, docs, tests.
- Axis-2 C: would require committing core Striatum to network-aware
  behavior; rejected here unless the product owner expands scope.

**Recommended options.**

- Axis 1 (commit authority): **B** for the durable-artifact + apply
  step, sequenced after **A** ships first as a working handoff.
- Axis 2 (hosted boundary): **A** for core; **B** as future optional
  plugin. **Reject C.**
- Axis 3 (confirmation): **AI operator confirmation for local
  commits**, **human-principal confirmation for hosted-provider
  actions**, with workflow-declared escalation to human for local
  commits permitted but not the default.
- Axis 4 (credentials): **external-only with no durable persistence;
  redaction-safe provider URLs/IDs are recordable only when the
  hosted-plugin option is later accepted.**
- Axis 5 (sequencing): **read-only snapshots → commit request
  artifacts → optional local commit apply → optional provider plugin.**

**Decision text to accept or modify:**

> Striatum core does not create commits autonomously, does not push, does
> not call hosted providers, and does not import provider SDKs. RFC
> 0067's read-only local Git snapshotting is the first slice. Beyond it,
> Striatum may grow (a) durable commit-request and PR-request artifacts
> as provenance and (b) an optional daemon-backed `git commit-apply` step
> that consumes a reviewed commit-request artifact and creates a local
> commit only after explicit operator confirmation. Hosted-provider
> actions are out of core; if ever accepted, they live behind an
> optional plugin/connector with credentials sourced from outside
> durable runner state. Local commit apply may be confirmed by the AI
> operator when workflow gates are satisfied; hosted-provider actions
> require human-principal confirmation; workflows may escalate local
> commit confirmation to human-principal but not the reverse. Provider
> tokens are never persisted in workflow JSON, artifacts, archive
> bundles, corpus exports, audit payloads, or `.striatum/` scratch.
> Local commit SHAs are durable provenance; hosted URLs and remote
> branch names are durable only under the optional-plugin product
> decision, redacted per the corpus contract. Implementation sequencing
> is: (1) RFC 0067 read-only local snapshots, (2) commit-request and
> PR-request artifact contracts, (3) optional `git commit-apply` behind
> explicit confirmation, (4) optional provider plugin if and when its
> own product decision lands.

---

## Cross-Cutting Notes For The Human Principal

- **Single live authority.** TODO 55 (C+A), TODO 56 (defer+evidence
  gate), TODO 59 (composite id + tiered redaction + opt-in
  augmentation + hybrid archive + deep-chain verify), and TODO 60
  (no autonomous commits; no hosted core) all read as "preserve the
  daemon as the single live authority; preserve the durable decision
  artifact as the single human-readable authority; do not invent
  parallel authority surfaces in workflow files or external
  providers." That coherence is a useful sanity check on each
  individual decision.
- **Operator-on-behalf is the always-available escape hatch.** Three
  of the four TODOs assume it. Do not erode it inadvertently when
  ratifying TODO 56.
- **Decision-log discipline.** Each ratified option above should land
  as its own `decision` artifact, update `docs/DECISION_LOG.md`, and
  bump TODO 55/56/59/60 status in `docs/TODO.md` and
  `docs/ROADMAP.md` §4.x. None of those updates happens in this
  synthesizer job by design.
- **Sequencing.** TODO 55's "C anchored by A" needs the daemon
  run-prepare hook; TODO 56's evidence gate needs N live dogfoods;
  TODO 59's V2 contract needs new schema; TODO 60's slice 2 (commit
  request artifacts) is a precondition for slice 3 (commit apply).
  None of these block each other, but all four lean on the same
  daemon-PG + artifact-contract bones.

This brief does not record a decision. Human principal: accept, modify,
or reject the decision text in each section. The downstream
`apply_decisions` job in this workflow will turn accepted text into a
`decision` artifact and update the decision log.
