# Build review (devils_advocate)

Verify:
1. `--force` overwrites; new `overwritten` status correctly
   reported; original content replaced with template body.
2. `--dry-run` does NOT touch the filesystem; envelope's
   `dry_run: true`; statuses reflect what would happen.
3. Both flags together: dry-run wins (no writes), but envelope
   shows `would_overwrite`.
4. Plain flag (neither) byte-identical to v1.9.0.
5. Lint / typecheck / full test pass.

Verdict ∈ {accept, accept_with_findings, needs_revision, reject}.
