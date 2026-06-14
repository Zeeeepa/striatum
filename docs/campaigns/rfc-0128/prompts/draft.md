# Draft — RFC 0128 P0: fail-fast cross-repo lint

Implement **Phase P0 of RFC 0128 (cross-repo run boundary, accepted D196)**.
Read `docs/rfcs/0128-cross-repo-run-boundary.md` (P0 only) and D196 first.

## The P0 slice (and ONLY this slice)

The validate-time cross-repo guardrail (closes the legible half of #280):
`run validate` / `workflow lint` scans declared `write_scope.allowed_paths`
across all lanes and **FAILS with a structured error (exit 7)** if any path
resolves outside the run's registered repo root; it additionally **WARNS** when
a free-text prompt field carries a path token or `org/repo` slug that does not
match the registered target. Never silently narrow — surface the cross-repo
intent before a lane spawns.

Do NOT implement P1–P3 (dispatch-time `scope_violation` terminal state,
read-only artifact federation, decomposition ergonomics) or the deferred
first-class multi-repo manifest. This slice is the validate-time lint only.

## Gotchas
- This is a **product-boundary guardrail**: single-repo run is the invariant
  (D196). Refuse cross-repo reach; do not add cross-repo write capability.
- Exit code 7 is the contract for the validate failure — match the existing
  validate error-code conventions in the CLI.
- A guard test must assert no `secondary_repos` manifest is honored (the deferred
  path has no code surface).
- New validate surface ⇒ update `docs/reference/command-authority-matrix.md` if a
  new RPC/param is introduced.

## Deliverable
Write `docs/campaigns/rfc-0128/artifacts/DRAFT.md` (design note + edit sites +
the P0 tests: a workflow whose lane write-scope reaches outside the repo root
fails validate with exit 7 and the offending path; a foreign prompt slug warns).
Write the code into the worktree (feature branch, repo-write scope).
`make -C go build` + targeted tests before completing; report results. Do not
merge to main.
