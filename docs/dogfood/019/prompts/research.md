# Research: scaffold V1.5 force/dry-run shape

Map:
1. `src/striatum/scaffold/__init__.py` — confirm `force` and
   `dry_run` parameters are reserved with `del force, dry_run`
   and how the per-file decision currently works.
2. `src/striatum/cli/parser.py` — locate the `--with-ddd-layout`
   action; identify where `--force` / `--dry-run` flags would
   slot in.
3. `src/striatum/cli/dispatch.py` — the `init` branch's call
   to `scaffold_ddd_layout`; identify how to thread the flags.
4. Test precedent — `tests/test_scaffold_ddd_layout.py` patterns.

Deliverable: `research/SCAFFOLD_V15_SHAPE.md`.
