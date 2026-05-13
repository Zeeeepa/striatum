# Implementer Role (Dogfood 046 — codex Python)

Single implementer, codex Python only. The workflow validator enforces
the write scope — stay strictly inside your job's
`write_scope.allowed_paths`.

Owns:

- `src/striatum/cli/parser.py`, `src/striatum/cli/dispatch.py` —
  register and dispatch `striatum corpus export --since <ref> --out
  <path>`. Mirror `run summary --json` / `recovery stale-leases`.
- `src/striatum/corpus/` (NEW package) — enumerator, redaction,
  writer, manifest, bundle entry. One module per concern per the
  synthesis layout.
- `src/striatum/` — only the minimal surface the synthesis names.
- `tests/` — unit tests per corpus module + one integration test
  against a real recent dogfood that verifies bundle replay-stability
  hashes.

Use sub-agents aggressively. Dispatch one per concern in parallel
(CLI verb, enumerator, redaction, writer+manifest, tests). Reconcile
sub-agent outputs yourself before writing HANDOFF.

**Scope boundary (critical)**: Striatum-side ONLY. Do NOT touch
`~/git/engram/`. Do NOT add `engram ingest-striatum`, the MCP stdio
server, retrieval tools, `striatum operator memory check`, or the
`striatum-engram` skill — those land in a separate Engram-side
dogfood.

**Augmentation-not-dependency is non-negotiable**: NO Engram client
import anywhere under `src/striatum/`. NO `memory.*` capability on
the daemon RPC registry. The `rg -n "engram" src/striatum/cli`
regression guard the synthesis names MUST be wired and pinned by a
test. Call it out in HANDOFF.

**Do NOT write to**: anything outside `allowed_paths`. **Neither this
implementer nor any sub-agent updates `docs/rfcs/README.md`,
`docs/TODO.md`, `CHANGELOG.md`, `docs/SPEC.md`, `docs/HOW_TO_AGENT.md`,
`docs/HOW_TO_HUMAN.md`, or `docs/UBIQUITOUS_LANGUAGE.md`** — the
operator handles those manually after the dogfood lands (no in-
workflow consolidate job; dogfood-042 cascade lesson).

Operational notes:

- Lease can expire if `make test` exceeds ~30 minutes. Prefer focused
  pytest before wider verification.
- This is a one-shot supervised invocation. Do not ask the operator
  follow-up questions. If `striatum ack` is denied, write the artifact
  and exit normally; the operator publishes on your behalf.
- Per D089/D091, OPERATOR_REPORT.md is the operator's responsibility,
  written incrementally — not yours.
