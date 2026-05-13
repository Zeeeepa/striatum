# Build review: RFC 0021 V1.5

author: reviewer-claude-opus-002
date: 2026-05-09
verdict: accept

Devil's-advocate review of the V1.5 build.

## Verdict

**accept** — V1.5 acceptance gate satisfied; 379/379 full suite
pass. Implementation matches the synthesis exactly.

## Sweep matrix

| Acceptance gate | How V1.5 satisfies it | Verified |
| --- | --- | --- |
| `--force` overwrites | `_read_template_body` writes the template; status becomes `overwritten`; `prior_sha256` records the bytes that were replaced. | `test_scaffold_force_overwrites_existing_file` |
| `--force` skips non-regular-file targets | `if not target.is_file()` branch fires before the force branch; status `error`/`would_error`; non-file target untouched. | `test_scaffold_force_does_not_clobber_non_regular_file` |
| `--dry-run` writes no files | The dry-run branches early-return before any `mkdir`/`write_text`. | `test_scaffold_dry_run_writes_no_files` |
| Envelope `dry_run` flag reflects | `return {..., "dry_run": dry_run}` (was hardcoded `False` in V1). | `test_scaffold_dry_run_envelope_reflects_flag` |
| `would_*` vocabulary | Five status values total: `would_create` (missing target), `would_skip` (existing file no force), `would_overwrite` (existing file + force), `would_error` (non-file target). The vocabulary is unambiguous and exhaustively partitioned from the non-`would_*` set. | `test_scaffold_dry_run_status_vocabulary_*` |
| Force + dry-run together | Existing file + both flags → `would_overwrite` with `prior_sha256` recorded; on-disk content unchanged. | `test_scaffold_force_and_dry_run_together_no_writes` |
| CLI flag wiring | `parser.py` adds both flags; `dispatch.py` reads via `getattr` and threads as kwargs. | `test_init_cli_flags_thread_through_to_envelope` |
| V1.5 flags no-op without layout flag | The dispatch path's outer guard (`if getattr(args, "with_ddd_layout", False)`) means the scaffold call doesn't fire; envelope has no `ddd_layout` key. | `test_init_v15_flags_noop_without_with_ddd_layout` |
| Zero regression for plain V1 invocation | All 13 V1 tests still pass. Envelope shape preserved when both flags are False. | Full suite. |
| Public API stability | `scaffold_ddd_layout(repo, *, force=False, dry_run=False)` signature unchanged; V1 callers get V1 behavior. | Source review. |
| Suite health | lint clean; mypy clean (62 source files); 22/22 scaffold tests; 379/379 full. | Direct run output. |

## Counterargument sweep

### "Reading prior_bytes for the audit field doubles I/O on force"

The hash is a 32-byte sha256 over a small Markdown file. The
read-then-write is at most a few KB; this is not a hot path.
The audit value is the synthesis's stated trade-off. **Accept.**

### "What if read_bytes() succeeds but write_text() fails after?"

The implementation catches OSError on the write path and
records status `error` in that case. The `prior_sha256` is
already in the envelope entry; an operator can recover the
prior content from the still-on-disk file (since the write
failed, the file was unchanged). **Accept.**

### "The `would_error` status is hairsplitting"

V1 dry-run could just emit `error` for non-file targets, since
the envelope already has `dry_run: true` to signal preview mode.
But emitting `would_error` keeps the per-row vocabulary
self-describing — a tooling consumer scanning row statuses
without checking the top-level flag still understands "this is
a preview." **Accept.**

### "The synthesis said 7 test cases but the build has 9"

The implementer split two of the synthesis's bullets into
multiple cases (status vocabulary across empty + partial-overlap
repos; CLI wiring + the no-op guard test). Tighter coverage,
not scope creep. **Accept.**

## Decision

Accept V1.5 clean. RFC 0021 transitions to
`accepted (V1+V1.5)`. Tag v1.10.0.

This dogfood also exercises RFC 0018 V1+step 3 yet again: both
reviews carry `review_posture: "devils_advocate"`, the build
declares `required_review_postures: ["devils_advocate"]`, and
the verdicts.posture column will record `devils_advocate` for
both verdicts. The dashboard's posture summary will show
`Postures: devils_advocate=2` once this run terminates.
