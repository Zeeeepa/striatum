# Implement Prompt

Implementation is blocked until `review_design_threat` returns an accepting verdict. Do not start implementation from RFC 0033 alone.

After the gate opens, implement only the accepted scope in `docs/dogfood/033/DESIGN_SYNTHESIS.md` and the resolved threat-model review findings. Stay inside the workflow write scope.

Expected behavior:

- introduce a daemon-owned Postgres connection layer in `src/striatum/` (concrete module name decided in the synthesis);
- add forward-only Postgres schema migrations alongside the existing repo-local SQLite migrations, with the daemon refusing to start on schema mismatch (exit code 9 parity);
- add audit row trigger / role-enforced append-only logic; map V1 audit segment manifests onto the new substrate; verify chain via `daemon doctor`;
- implement `striatum daemon migrate --from sqlite --to pg [--dry-run] [--keep-sqlite-readonly]` with byte-equivalent audit chain validation and explicit V1 read refusal after cutover;
- add the daemon-doctor onboarding hints (platform-specific Postgres install instructions when the URL is missing or unreachable);
- add the per-test Postgres harness in `tests/conftest.py` or equivalent; teardown leaves no zombie connections;
- add integration tests for: forward migration apply, V1 → V2 cutover byte-equivalent audit chain, capability check under serializable isolation, supervisor heartbeat concurrency, audit append concurrency;
- update `docs/SPEC.md`, `docs/MCP.md`, `docs/UBIQUITOUS_LANGUAGE.md`, `docs/CLI_REFERENCE.md`, `docs/HOW_TO_HUMAN.md`, the RFC 0033 status block, `docs/rfcs/README.md`, `README.md`, and `CHANGELOG.md` only as required by the accepted plan;
- describe daemon guarantees honestly per the RFC 0031 threat model: AI guardrail, not cryptographic proof, not defense against a malicious operator.

Do NOT:

- ship bundled / Dockerized Postgres (deferred);
- design or implement the Python → Go substrate port (D084; future RFC);
- modify repo-local `.striatum/state.sqlite3` semantics;
- add devils_advocate or security review jobs to this dogfood's workflow (those are post-implementation per operator decision);
- claim sealed-mode apply (RFC 0031) or RPC server (RFC 0030) features that depend on later RFCs.

## Maximize sub-agent usage where it helps

Per the harness profile, native sub-agent delegation is **encouraged**.
Use it aggressively for the parts of this implementation that are
independent enough to parallelize. The goal is to compress wall-clock
time without giving up coherence; sub-agents are a power tool when the
work is genuinely independent and a footgun when it is not.

Spawn sub-agents in parallel for work that meets all of these:

- the sub-task can be specified by a self-contained brief (file paths,
  expected behavior, test fixtures, ~1 page of context);
- it does not depend on the in-flight output of another sub-agent;
- you (the parent session) can independently verify its output.

Good candidates in this implementation:

- one sub-agent per major substrate module — connection layer, schema
  migrations, audit-chain trigger/role wiring, `daemon migrate` cutover,
  `daemon doctor` onboarding hints — running in parallel where the
  synthesis names disjoint write scopes;
- one sub-agent per new test file — migration-apply test, V1→V2
  byte-equivalent cutover test, serializable-isolation concurrency
  tests, doctor-hint tests — drafted in parallel while the code is
  being written, then reconciled with the actual API as it lands;
- one sub-agent per doc surface — SPEC.md section, MCP.md section,
  UBIQUITOUS_LANGUAGE.md entries, CLI_REFERENCE.md daemon-DB section,
  HOW_TO_HUMAN.md walkthrough — each with a focused brief about what
  the synthesis decided;
- exploratory sub-agents to read existing modules (`daemon.py`,
  `migrations.py`, `db.py`, `schema.py`) and produce one-page summaries
  of the current shape before you start editing.

Do NOT delegate (parent session owns these):

- the BUILD_HANDOFF.md authorship — it summarizes everything and binds
  the work to the run packet, so it stays in the parent;
- the integration step where the sub-agents' outputs are reconciled —
  the parent session verifies that the connection layer, migrations,
  audit triggers, and cutover all fit together;
- any `make lint`/`typecheck`/`test`/`smoke` invocation — the parent
  session is the verifier of record;
- final commit-shape and scope discipline — the parent session
  refuses sub-agent output that crosses the write scope or invents
  features outside the accepted synthesis.

When you delegate, give each sub-agent the relevant section of the
DESIGN_SYNTHESIS.md plus its concrete deliverable (file path + expected
contents/behavior + a test fixture). When a sub-agent returns, you read
its output; do not paste it in unchanged without verifying it matches
the synthesis and the test passes.

## Verification

Run `make install`, `make lint`, `make typecheck`, `make test`, `make smoke` after all changes are in place. Address every failure honestly; do not skip tests to make the bar look green.

## Handoff

Produce `docs/dogfood/033/BUILD_HANDOFF.md` summarizing changes, tests added/passing, deferred scope, follow-up RFC dependencies (RFC 0030 will key off the substrate version recorded here), and any human-decision items that the threat-model review did not pre-resolve. If sub-agents were used, briefly note which sub-tasks were delegated; the parent session remains the artifact author.

Do not call striatum CLI; the operator publishes on your behalf.
