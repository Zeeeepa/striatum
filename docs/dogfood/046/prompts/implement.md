# Implement: RFC 0044 V1 Striatum-side (codex Python)

Blocked until `review_design` returns an accepting verdict.

Implement RFC 0044 V1 Striatum-side per `docs/dogfood/046/DESIGN_SYNTHESIS.md`. **You write Python only. Engram-side is OUT OF SCOPE — it lands separately under `~/git/engram/`.**

**Your scope (codex Python-side):**

- `src/striatum/cli/parser.py`, `src/striatum/cli/dispatch.py` — register and dispatch `striatum corpus export --since <ref> --out <path>`. Mirror the `run summary --json` / `recovery stale-leases` patterns.
- `src/striatum/corpus/` (NEW package) — enumerator, redaction, writer, manifest, bundle entry point. One module per concern per the synthesis layout.
- `src/striatum/` — only the minimal surface the synthesis names (e.g. a constants module). Do not touch unrelated subsystems.
- `tests/` — unit tests per corpus module + one integration test against a real run that verifies bundle replay-stability hashes.
- `docs/dogfood/046/build/HANDOFF.md` — handoff summarizing shipped scope, files touched, test results, deviations from the synthesis (if any) with one-line rationale.

**Use sub-agents aggressively** — one per concern, dispatched in parallel:

- Sub-agent CLI verb: `corpus export` registers in `parser.py`, dispatches in `dispatch.py`, JSON error envelope consistent with neighboring verbs.
- Sub-agent corpus enumerator: each `sub_kind` reader. Run summaries through `striatum run summary --json` (RFC 0044 §3), NOT free-text SQLite reads.
- Sub-agent redaction policy: denylist + per-field rules. Fail loudly on unknown fields.
- Sub-agent JSONL writer + manifest: deterministic ordering; manifest fields locked to RFC 0044 §3; replay-stable SHA-256s.
- Sub-agent tests: unit per module + the integration test against a real run.

Reconcile sub-agent outputs yourself before writing HANDOFF.

**Augmentation-not-dependency (non-negotiable)**: NO Engram client import anywhere under `src/striatum/`. NO `memory.*` capability on the daemon RPC registry. Striatum must run identically with Engram absent. Add the `rg -n "engram" src/striatum/cli` regression guard the synthesis names — call it out in HANDOFF.

**Do NOT write to**: anything outside `allowed_paths`. **No README / TODO / CHANGELOG / RFC index / SPEC / HOW_TO updates** — the operator handles those manually after the dogfood lands (no in-workflow consolidate job; dogfood-042 cascade lesson).

Verification: `make lint`, `make typecheck`, `make test` all pass. The integration test exports a real recent dogfood corpus and asserts manifest + JSONL hash stability on a second run.

One-shot supervised invocation. Do not ask follow-ups. If `striatum ack` is denied, write the HANDOFF and exit normally.
