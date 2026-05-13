# Track A Implement: Python Glue + Harness Extension + Docs (claude_code)

Blocked until `review_go_design` returns an accepting verdict.

Implement Python-side glue for Steps 1+2 from `docs/dogfood/042/track_a/DESIGN_SYNTHESIS.md`. **You write Python + docs; codex writes Go code.** Do NOT cross into Go scope.

**Your scope (claude_code Python + docs side):**

- `tests/_harness/multi_repo.py` — extend `MultiRepoHarness` with `daemon_core` parameter (`"python"` | `"go"`).
- `tests/_harness/daemon.py` — extend to launch Go daemon binary when `daemon_core="go"`.
- `tests/_harness/{pg,repos,mcp,scope,tokens,audit}.py` — minor adjustments if needed.
- `docs/HOW_TO_HUMAN.md` — add section on Go daemon (`--core go` flag will land in Phase 2; mention Phase 1 as the binary-only / read-only stage).
- `docs/SPEC.md` — daemon section noting Python vs Go cores per RFC 0039.
- `docs/UBIQUITOUS_LANGUAGE.md` — add "daemon core" entry.
- `docs/rfcs/0039-go-daemon-core.md` — status block update reflecting Steps 1+2 landed (still proposed for Phase 1 overall).
- `docs/dogfood/042/track_a/build/glue/HANDOFF.md` summarizing your shipped scope.

Do NOT update `docs/rfcs/README.md`, `docs/TODO.md`, or `CHANGELOG.md` — the `consolidate_phase_1` job handles those.

Use sub-agents aggressively for parallelizable work (one sub-agent per harness helper + per doc surface).

This is a one-shot supervised invocation. If `striatum ack` is denied, write the HANDOFF and exit normally; the operator publishes on your behalf.
