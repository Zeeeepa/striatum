# Implement Prompt

Implementation is blocked until `review_design_threat` returns an accepting verdict. Do not start implementation from RFC 0030 or RFC 0031 alone.

After the gate opens, implement only the accepted scope in `docs/dogfood/034/DESIGN_SYNTHESIS.md` and the resolved threat-model review findings. Stay inside the workflow write scope.

Expected behavior (both RFCs land together):

**RFC 0030 (daemon RPC server):**
- introduce `src/striatum/daemon_rpc/` (or whatever module path the synthesis settles on) with envelope codec, request router, capability authorizer, version handshake, method registry, and audit-append integration with the existing `src/striatum/daemon_pg/audit.py` helpers;
- Unix-domain socket transport with owner-only permissions; loopback HTTP opt-in; MCP over socket sub-path;
- `daemon.hello` / `daemon.welcome` handshake including the methods-etag cache;
- CLI client mode: read verbs (`status`, `doctor`, `why`, `dashboard`) route through daemon by default; mutating workflow verbs default to daemon during the transition with `--no-daemon` documented as a sunset path; admin verbs are daemon-only;
- audit + request log rows appended to the RFC 0033 substrate for every RPC call;
- new exit code 10 for version-incompatible handshakes.

**RFC 0031 (daemon-owned supervision + sealed apply):**
- daemon DB `process_supervisors` table + repo-local `process_supervisor_pointers` table with a forward migration;
- daemon-mediated `supervise.start / send / stop / status / list` RPC methods with capability checks;
- supervisor reattach across daemon restart using pid + pid_start_time verification;
- `apply.reviewed_patch` RPC method: load patch artifact, verify digest, load reviewer verdict, verify `patch_digest_hash` match, verify base-tree hash, apply to daemon-owned worktree under `${XDG_STATE_HOME}/striatum/daemon/worktrees/`, record receipt + Markdown evidence artifact;
- signing key bootstrap on first sealed-mode `daemon start`: Ed25519 keypair, OS keyring via `keyring` library with `0600` runtime fallback at `~/.local/state/striatum/daemon/signing_key`, daemon refuses sealed-mode without a loadable key;
- `daemon.key.rotate` admin RPC; old key revocation recorded in audit chain;
- workflow schema: `require_daemon: true`, `apply_gate: true`, `sealed_patch_provider: refuse` (debug aid);
- sealed-mode `run start` gate: refuses without daemon + signing key + `apply` token.

**Cross-cutting:**
- update `docs/SPEC.md` (RPC + sealed apply sections), `docs/MCP.md` (capability-gated tools), `docs/UBIQUITOUS_LANGUAGE.md` (new terms: daemon RPC envelope, method registry, capability scope, request log, daemon-owned supervisor, supervisor pointer, sealed-apply authority, apply receipt, daemon signing key), `docs/CLI_REFERENCE.md` (new daemon verbs + exit code 10), `docs/HOW_TO_HUMAN.md` (operator setup for daemon RPC + token + signing key);
- update RFC 0030 + RFC 0031 status blocks and `docs/rfcs/README.md`;
- update `README.md` daemon section and `CHANGELOG.md` Unreleased + version bump.

Do NOT:

- ship cross-repository workflow mutation or MCP mutation capability expansion (RFC 0032; future);
- ship a Go core (D084; future);
- ship bundled / Dockerized Postgres (RFC 0033 follow-up);
- retire `--no-daemon` for direct repo-local CLI mode (separate future RFC);
- claim cryptographic non-repudiation, model-token authorship proof, or malicious-local-root resistance (RFC 0031 threat model is the AI-guardrail framing);
- add devils_advocate or security review jobs to this dogfood's workflow (post-implementation per operator decision).

## Maximize sub-agent usage where it helps

Per the harness profile, native sub-agent delegation is **encouraged**. Use it aggressively for the parts of this implementation that are independent enough to parallelize. RFC 0030 + RFC 0031 paired is large; sub-agent parallelism materially compresses wall-clock time without giving up coherence.

Spawn sub-agents in parallel for work that meets all of these:

- the sub-task can be specified by a self-contained brief (file paths, expected behavior, test fixtures, ~1 page of context);
- it does not depend on the in-flight output of another sub-agent;
- you (the parent session) can independently verify its output.

Good candidates in this implementation:

- one sub-agent per major module — RPC envelope codec, request router, capability authorizer, version handshake, supervisor migration + reattach, sealed-apply gate, signing key custody, audit-append wiring on the substrate, MCP route filter — running in parallel where the synthesis names disjoint write scopes;
- one sub-agent per new test file — handshake refuse/downgrade, capability denial, audit append per RPC call, supervisor reattach with pid_start_time mismatch, sealed-apply refuse paths (digest mismatch, base-tree drift, wrong verdict, missing key), version skew, two-daemons-against-one-DB collision;
- one sub-agent per doc surface — SPEC (RPC + sealed sections), MCP, UBIQUITOUS_LANGUAGE, CLI_REFERENCE, HOW_TO_HUMAN walkthrough, RFC 0030 status block, RFC 0031 status block, CHANGELOG entry;
- exploratory sub-agents to read existing modules (`daemon.py`, `daemon_pg/*`, `supervisor.py`, `process_adapter.py`, `mcp.py`, `service.py`, `cli/dispatch.py`, `cli/parser.py`) and produce one-page summaries of the current shape before you start editing.

Do NOT delegate (parent session owns these):

- the BUILD_HANDOFF.md authorship — it summarizes everything and binds the work to the run packet, so it stays in the parent;
- the integration step where the sub-agents' outputs are reconciled — the parent session verifies that envelope + router + authorizer + audit + supervisor migration + sealed-apply all fit together;
- any `make lint`/`typecheck`/`test`/`smoke` invocation — the parent session is the verifier of record;
- final commit-shape and scope discipline — the parent session refuses sub-agent output that crosses the write scope or invents features outside the accepted synthesis.

When you delegate, give each sub-agent the relevant section of the DESIGN_SYNTHESIS.md plus its concrete deliverable (file path + expected contents/behavior + a test fixture). When a sub-agent returns, you read its output; do not paste it in unchanged without verifying it matches the synthesis and the test passes.

## Verification

Run `make install`, `make lint`, `make typecheck`, `make test`, `make smoke` after all changes are in place. Address every failure honestly; do not skip tests to make the bar look green.

## Handoff

Produce `docs/dogfood/034/BUILD_HANDOFF.md` summarizing changes, new modules, schema migrations on the substrate, tests added/passing, deferred scope, follow-up RFC dependencies (RFC 0032 will key off the RPC envelope here), and any human-decision items the threat-model review did not pre-resolve. If sub-agents were used, briefly note which sub-tasks were delegated; the parent session remains the artifact author.

The byline must be `author: implementer-codex-gpt-5.5-001` (or whatever the work packet supplies) — plain Markdown line, lowercase `author:`, no decoration.

Do not call striatum CLI unless your harness profile permits it; the operator publishes on your behalf otherwise.
