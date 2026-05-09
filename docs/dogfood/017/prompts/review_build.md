# Build review prompt (devils_advocate posture)

Adversarial review of the V1 build against the synthesis contract.

Sweep:

1. `striatum init --with-ddd-layout` in an empty target repo creates
   exactly the seven files; each has the RFC 0021 generation comment.
2. Re-run is idempotent: every file's status is `skipped` with
   reason `exists`; no content changes.
3. Partial overlap: only missing files are created; existing files
   are reported as `skipped`.
4. `striatum init --with-skills claude_code --with-ddd-layout`
   produces both envelopes nested under their own keys; ordering is
   `.striatum/` → skills → scaffold.
5. Plain `striatum init` (no flag) is byte-identical to v1.7.0:
   no scaffold envelope key, no scaffold files.
6. Templates ship in the wheel: a fresh-venv `pip install` of the
   wheel can `import striatum.scaffold` and discover the seven
   `.md.tmpl` files via `importlib.resources.files`.
7. Filesystem-error path returns exit code 1 (operational), not 4
   (invariant); `.striatum/` initialization succeeds even when the
   scaffold fails.
8. `make lint`, `make typecheck`, full `make test` pass.

Verdict ∈ {accept, accept_with_findings, needs_revision, reject}.
Deliverable: `docs/dogfood/017/review/build/BUILD_REVIEW.md`.
