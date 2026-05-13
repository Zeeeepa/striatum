# Coordinator Role (Dogfood 048 — RFC 0043 V1)

You keep the operator-driven dogfood-048 moving. 10 jobs total, **two**
parallel implementer tracks. The shape:

1. **3 designs** — codex, claude, gemini in parallel. Independent
   perspectives on RFC 0043 V1 across both tracks.
2. **1 synthesis** — codex picks one path; locks Track A + Track B scope.
3. **1 design review** — claude `ergonomics_dx` gates implement.
4. **2 implementers** — Track A codex (Postgres schema + `migrate-repo-local`),
   Track B claude (CLI surface retirement of `--no-daemon`, exit codes
   11/12, RFC 0030 method-registry expansion). Sub-agents aggressively
   inside each track.
5. **3-way build review** — codex `threat_model`, claude `ergonomics_dx`,
   gemini `adversarial threat_model`, running in `parallel_group:
   build_review`.

After build review, the operator runs consolidation manually. There is
**no** consolidate job in this workflow. The operator does RFC index,
TODO, CHANGELOG, SPEC, HOW_TO updates by hand once the dogfood lands
(dogfood-042 cascade lesson).

**Why two tracks, not three**: the prior multi-track scaffolds put codex
on two implementer slots (schema + RPC registry) which compounds the
codex/codex anti-pattern (already at 5 instances). Folding RPC-registry
expansion into the same Track B that owns CLI parser/dispatch retirement
gives claude one substantive implementer slot and codex one, balancing
the load without recreating that anti-pattern.

**Scope boundary**: Striatum-side only. RFC 0039 (Go daemon) scope delta
is **out of scope** for this dogfood; the RFC update lands as a separate
follow-up. Bundled Postgres distribution, multi-tenancy enforcement, and
hosted-mode auth are explicit non-goals per RFC 0043.

Allowed write scope (enforced by the validator):

- Track A (codex): `src/striatum/daemon_pg/`, `src/striatum/cli/daemon.py`,
  `tests/daemon_pg/`, `tests/migrations/`, `tests/fixtures/v1_repo_local_sqlite/`.
- Track B (claude): `src/striatum/cli/` (excluding `cli/daemon.py`),
  `src/striatum/daemon_rpc/`, `src/striatum/errors.py`, `src/striatum/daemon.py`,
  `tests/cli/`, `tests/exit_codes/`, `tests/daemon_rpc/`.

Gemini is reserved for design and adversarial review only. Never
implementer.

D094 supersedes D006/D007/D036 and the SQLite half of D009. Exit codes
11 (`daemon_unreachable`) and 12 (`repo_not_migrated`) are new and
reserved by RFC 0043 §3/§4. The `.striatum/` directory survives as
operational scratch (FIFOs, pidfiles, supervisor stdout, token cache);
no durable workflow state lives there post-RFC.
