# Coordinator Role (Dogfood 057 — RFC 0048 Phase A)

You keep the operator-driven dogfood-057 moving. 10 jobs total, two parallel implementer tracks. The shape:

1. **3 designs** — codex, claude, gemini in parallel. Independent perspectives on RFC 0048 Phase A handler port (no track split inside design).
2. **1 synthesis** — codex picks one path; locks Track A + Track B scope, module layout, signature, delegation-swap pattern.
3. **1 design review** — claude `ergonomics_dx` gates implement.
4. **2 implementers** — Track A codex (9 workflow-loop handlers), Track B claude (7 recovery + evidence handlers). Sub-agents aggressively inside each track.
5. **3-way build review** — codex `threat_model`, claude `ergonomics_dx`, gemini adversarial `threat_model`, in `parallel_group: build_review`.

After build review, the operator runs consolidation manually (RFC index, TODO, CHANGELOG, SPEC, `docs/POSTGRES_TRANSITION.md`, `HOW_TO_*` updates) once the dogfood lands.

**Scope boundary**: this dogfood is RFC 0048 **Phase A only** — port the 16 single-repo handlers and swap `DaemonRpcRouter._route` delegation for them. Phase B (Go core parity) and Phase C (SQLite removal) are separate dogfoods. The Unix-socket accept-loop gap in `src/striatum/daemon.py` (`run_daemon_foreground` binds but does not accept) is **also out of scope** — note it in the operator report as a follow-on.

**Operating mode**: the daemon-required CLI path is non-functional on the current branch (RFC 0048's own subject matter). Operator runs CLI in legacy SQLite mode via `STRIATUM_DAEMON_REQUIRED=0 STRIATUM_TEST_HARNESS=1`. This is the documented test-harness escape. Record this as a break-glass in OPERATOR_REPORT.md.

**Why two tracks** (workflow-loop vs recovery+evidence): the methods naturally split. Workflow-loop handlers are tight on transaction shape + audit-chain anchoring; recovery handlers are tight on lease/process semantics + evidence export determinism. Splitting them avoids one implementer holding a huge cross-cutting context.
