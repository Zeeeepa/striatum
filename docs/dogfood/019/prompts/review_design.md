# Design review (devils_advocate)

Sweep:
1. Is `--force` too destructive? Should it require a prior-SHA
   check or only overwrite generated files (those with the
   RFC 0021 comment)?
2. `--dry-run` envelope shape — does it match `would_*` status
   vocabulary or reuse `created` + a top-level `dry_run: true`?
3. Composability: `--force` AND `--dry-run` together — what
   happens?
4. Zero regression: plain `--with-ddd-layout` (neither flag)
   produces v1.9.0-identical output.
5. Test plan completeness.

Verdict ∈ {accept, accept_with_findings, needs_revision, reject}.
