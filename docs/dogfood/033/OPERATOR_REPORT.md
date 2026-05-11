# Dogfood 033 Operator Report

author: operator
date: 2026-05-11
status: complete

## Run

- Run ID: `run_95475e5eff0247908c0bd6d23c5c6200`
- Workflow: `dogfood-033-rfc-0033-storage-substrate`
- Branch: `striatum/dogfood-033-rfc-0033-substrate`
- Final state: `completed`
- Final job tally: 6 jobs completed, 0 canceled, 0 open blockers, 0 human checkpoints.

## Scope

Implement the RFC 0033 V1 acceptance-criteria slice: optional system-PostgreSQL
substrate for daemon-owned state (registry, capability tokens, audit chain
with role-enforced append-only, audit segment manifests, scheduler cursors,
RPC request log and client-session scaffolding for RFC 0030), forward-only
daemon DB migration runner, `daemon doctor` Postgres section, and
`striatum daemon migrate --from sqlite --to pg [--dry-run]
[--keep-sqlite-readonly]` for V1 daemon-registry SQLite → V2 Postgres
cutover with byte-equivalent audit chain validation. Deferred per the
accepted synthesis: bundled / Dockerized PG distribution, daemon RPC server
(RFC 0030), daemon-owned supervision and sealed apply (RFC 0031), cross-repo
workflows and MCP mutation (RFC 0032), repo-local SQLite changes.

## Control-Plane Outcome

The streamlined workflow shape (3-lane fresh design → synthesize → single
threat_model review → implement; no devils_advocate / security review in the
gate; no build review) compressed the run dramatically vs dogfood-031:

- 3 fresh designs landed in ~7 min total
- Synthesis in ~6 min
- Single threat-model review (`accept_with_findings` first try, severity
  medium) in ~3 min
- Implementation + sub-agent delegation + full make-targets + handoff in
  ~15 min

Total wall-clock: ~33 minutes vs dogfood-031's ~3 hours.

## Notable Wins vs Dogfood-031

1. **Codex drove its own claim loop end-to-end.** With the new
   `.striatum/bin/codex-supervised-wrapper.sh` and
   `--dangerously-bypass-approvals-and-sandbox`, codex sessions called
   `striatum ack`, `striatum publish-artifact`, and `striatum complete`
   directly. No operator publishes required for codex jobs. (Compare:
   dogfood-031 needed manual `codex exec` invocations + operator publishes
   for every codex job; bylines downgraded to `author: operator`.)

2. **Sub-agent delegation worked as designed.** The implementer's
   `BUILD_HANDOFF.md` records that codex spawned "one explorer mapped the
   current daemon/CLI/migration/audit insertion points; one explorer mapped
   the existing daemon and migration tests and recommended focused RFC 0033
   coverage; one worker drafted the documentation deltas." Parent session
   integrated and verified.

3. **Single review posture compressed cycles.** Threat-model returned
   `accept_with_findings` on the first try with 4 explicit conditions for
   implementation; the implementer addressed them in code. No revision
   cycles needed. (Compare: dogfood-031 design review went 3 rounds; build
   review exhausted to a cycle-exhaustion checkpoint.)

4. **No cascade-collision UNIQUE errors.** The dogfood-031 follow-up fix
   (commit 4bd359f) made parallel `needs_revision` verdicts share a single
   cycle target idempotently. Not exercised in dogfood-033 (only one review
   posture; only one accepting verdict), but the fix is now in place.

## Operator Interventions

Two minor mechanical fixes only:

1. **Gemini design byline format**: gemini wrote `**Author:**` (bolded)
   instead of the lowercase `author:` the publisher expects. The operator
   was unable to patch the byline before publish (Edit tool ordering); the
   publisher accepted the artifact regardless, but the run-summary records
   the gemini design with `author: <missing>` rather than the attested
   byline. Recommendation: add this to gemini's prompt as a hard constraint
   in dogfood-034.

2. **Gemini threat-review front matter**: gemini wrote the review body but
   omitted the `striatum.finding.v1` front matter block. The operator
   prepended the front matter (`schema_version`, `artifact_kind`,
   `verdict_intent: accept_with_findings`, `severity: medium`) before
   submit-review. The verdict and content remain entirely gemini-authored;
   the operator added required machine-readable metadata only.

Neither intervention crossed the operator boundary into role work. Both are
documented harness-improvement candidates: future RFCs should consider
whether the publisher's byline regex should accept Markdown bold (`**`)
forms, and whether the runner should auto-attach a default front matter
block when a known-kind artifact is published without one.

## Verification Artifacts

- `docs/dogfood/033/RUN_SUMMARY.md`
- `docs/dogfood/033/EVIDENCE.md`

Implementation verification (from the BUILD_HANDOFF):

- `make install`: passed
- `make lint`: passed
- `make typecheck`: passed
- `make test`: 582 passed (+7 from baseline of 575; new
  `tests/test_daemon_pg.py`)
- `make smoke`: passed (with only existing deprecated-`needs` warnings)

## Recorded Risks and Follow-ups

Documented in `docs/dogfood/033/review/design/threat/REVIEW.md` and
acknowledged in the BUILD_HANDOFF:

- Live-Postgres CI harness for migration apply, imported-audit equivalence,
  privilege enforcement, scheduler cursor concurrency, and capability
  revocation races against a real server is still required. The
  unit-testable substrate scaffolding ships now; live integration is the
  next hardening pass.
- `daemon doctor` superuser refusal is documented as a hardening item; the
  V1 scaffold does not yet enforce it.
- Subtle Postgres version drift behavior (collation, JSONB) is documented
  as a recommendation rather than a hard floor; PG 14+ remains the
  recommended minimum.
- `STRIATUM_DAEMON_DB_URL` inheritance to lane child processes is
  documented as a requirement to address; not yet enforced.

## Deliberately Left Out

The operator did not author design, synthesis, review, or implementation
content. The two mechanical interventions (gemini byline format + missing
front matter) are documented above. Devil's-advocate and security reviews
were deliberately deferred to post-implementation per the operator decision
recorded in commit 9d95487; they were not run as part of this dogfood.
