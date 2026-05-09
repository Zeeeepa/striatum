# Synthesis: lock V1.5

Pin:
1. **`--force` semantics.** When True, overwrite existing files
   (status: "overwritten" with `prior_sha256` recorded). When
   False (default), existing files report `skipped`.
2. **`--dry-run` semantics.** When True, no filesystem mutation;
   envelope reports what *would* happen. `dry_run` field in
   envelope reflects the flag.
3. **Per-file status vocabulary.** V1.5 adds `overwritten` and
   `would_create` / `would_skip` / `would_overwrite`.
4. **Non-file target handling.** `--force` does NOT clobber
   non-regular-file targets (directory/broken-symlink stays
   `error`).
5. **CLI flag wiring.** `--with-ddd-layout-force` and
   `--with-ddd-layout-dry-run` (verbose names) OR composed flag
   like `--ddd-layout-force`. Decide on the cleanest shape.
6. **Test plan.**

Deliverable: `DESIGN_SYNTHESIS.md`.
