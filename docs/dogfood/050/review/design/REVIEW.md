---
schema_version: "striatum.finding.v1"
artifact_kind: "finding"
verdict_intent: "accept_with_findings"
severity: "low"
tags: ["ergonomics_dx", "rfc-0043", "v1.5", "design"]
---

author: reviewer-unknown-model-001

# Design Review — RFC 0043 V1.5 Synthesis (ergonomics_dx)

Reviewed: `docs/dogfood/050/DESIGN_SYNTHESIS.md`.

## Verdict

`accept_with_findings`. The synthesis locks the four follow-up shapes (parser
wiring, default flip, exit-12 e2e, checkpointed crash-resume) with concrete
signatures, file paths, and rationale. The implementation order is named with a
non-trivial dependency justification (parser → flip → test → crash). Backward-
compat (no new SQL file, tombstone semantics preserved across all paths) is
spelled out. The escape-path audit is exhaustive ("none beyond this one
top-level gate" with six legacy modules enumerated).

The findings below are ergonomic polish items, not design blockers. They all
fall on the operator-facing surface — help text completeness, drafted error
envelopes, and the transition narrative — which is the explicit lens of the
posture.

## Findings

### F1 — Crash-recovery resume has no operator-visible message drafted (low)

Pinpoint: §4 "Harden crash recovery with checkpointed resume", the paragraph
describing `_resume_sqlite_finalization_after_checkpoint`.

The synthesis specifies the helper's internal behavior (SHA verification against
`checkpoint["source_state_db_sha256"]`, resume tombstone/delete, clear sentinel,
return `sqlite_finalization` result) but never names what the operator *sees*
on stderr when they rerun `striatum daemon migrate-repo-local` after a crash
and the resume path fires. The prompt explicitly asked for a drafted envelope
("Resuming partially-completed migration: Postgres state present, SQLite source
still writable, completing tombstone…").

Three resume sub-cases all need text:

1. Sentinel + source present, SHA matches → resume finalization.
2. Sentinel + source present, SHA mismatches → exit-8 refusal (synthesis names
   exit-8 but does not draft the message).
3. Sentinel only (orphan) → silent cleanup, or a single-line "stale sentinel
   cleared after prior successful finalization" notice.

Without drafted text, two implementers will produce two different operator
experiences and the regression test (`test_repo_local_migration_crash_resume.py`)
cannot assert on stderr.

Suggested remediation: append a short "Resume operator UX" subsection to §4
with one literal stderr line per sub-case, then add a `capsys.readouterr().err`
assertion to the existing regression test path.

### F2 — `migrate-repo-local --help` is partially actionable (low)

Pinpoint: §1 "Wire `daemon migrate-repo-local`", the `daemon_sub.add_parser`
code block.

The subparser top-level `help=` is `"migrate one repo-local
.striatum/state.sqlite3 into daemon PostgreSQL state"`. Only
`--keep-sqlite-readonly` and `--no-keep-sqlite-readonly` carry `help=` text.
`--from`, `--to`, `--repo`, `--postgres-url`, `--dry-run`, `--confirm-delete`,
and `--json` are bare. The prompt asked specifically whether the help text is
actionable in three respects:

- mentions exit-code 12 remediation context — NO (subparser top-line does not
  hint that this command resolves the exit-12 refusal).
- mentions `--confirm-delete` semantics — NO `help=` on the flag itself, and
  the pairing rule ("`--no-keep-sqlite-readonly` requires `--confirm-delete`")
  appears only in the `--no-keep-sqlite-readonly` text. A first-time operator
  running `--help` and reading top-to-bottom does not see the rule on the
  `--confirm-delete` line.
- mentions `--keep-sqlite-readonly` rename semantics — YES (the tombstone
  `0444` chmod is in the help text).

The smoke check at the end of §1 enumerates which flag *names* must appear in
`--help`, but does not enumerate which `help=` strings must be present. The
two are different ergonomic guarantees.

Suggested remediation: add `help=` to all eight flags, and either extend the
subparser top-line to "(resolves exit-code-12 'repo_not_migrated' refusal)" or
add a `description=` paragraph on `add_parser()` that mentions the exit-12
linkage. Lock the expected help-text fragments in the §1 smoke command list,
not just the flag names.

### F3 — Transition story stops at exit-12, glosses exit-11 (low)

Pinpoint: §3 first paragraph and the `dispatch_mod.main(...)` assertion;
implicit in the §2 default-flip section.

The exit-12 refusal text is locked: stderr contains `repo_not_migrated` and
`striatum daemon migrate-repo-local --from sqlite --to pg --repo`. That is
correct and operator-readable.

The transition narrative for a user upgrading striatum, however, depends on
both exit codes:

1. User upgrades striatum, daemon not yet running, runs any normal command.
2. Default flip makes the daemon-required gate fire.
3. They hit exit-code-11 (daemon-unreachable) BEFORE exit-code-12, because the
   exit-12 test itself "Monkeypatch[es] or bind[s] a temporary Unix socket so
   the test passes the exit-code-11 daemon-unreachable check and reaches
   `repo_is_migrated()`." That phrasing concedes exit-11 is the gate operators
   pass through first.

The synthesis does not state what the exit-11 stderr says, nor whether it
references `striatum daemon` start instructions. It also does not address the
quasi-paradox that `striatum daemon migrate-repo-local --help` "must not
require a daemon" (§1, last paragraph) but the operator only learns the
subcommand exists by *first* getting past exit-11 and then exit-12. If the
exit-11 message does not point at `daemon --help`, the discoverability chain
breaks for fresh installs.

Suggested remediation: either (a) confirm in §2 that the existing exit-11
refusal message already hints at the daemon lifecycle (cite the file/line), or
(b) draft a one-line addition to exit-11 stderr that names `striatum daemon
--help` as the discovery surface. The exit-11 path is outside V1.5's named
scope, but the synthesis should at least name it as "explicitly out of scope —
exit-11 wording is owned by V1" so a reader knows the gap is intentional.

### F4 — Exit-12 test is discoverable (no finding, confirmation)

Pinpoint: §3, second and third paragraphs.

The test lands in the existing file `tests/exit_codes/test_rfc0043_refusals.py`
("invert the existing default-off assertions and add the end-to-end unmigrated-
repo case there so all RFC 0043 refusal tests stay together"). The fixture is
a `tmp_path` repo with `.striatum/state.sqlite3` plus a temporary Unix socket;
no `STRIATUM_PG_TEST_URL` is required. The assertion calls
`dispatch_mod.main(["--repo", str(tmp_path), "status"])` and checks the return
code plus `capsys.readouterr().err`.

This satisfies `make test` discoverability without external substrate. No
change requested. Noted because the prompt explicitly raised it.

## Specific-check matrix

| Check | Status | Note |
|---|---|---|
| F-crash: shape named | ✓ | "checkpointed resume" with rationale against transactional rollback |
| F-crash: signature locked | ✓ | `_migrate_full(... sentinel_path: Path ...)` shown verbatim |
| F-crash: sentinel/lock primitive named | ✓ | `.striatum/state.sqlite3.migrated` JSON sentinel |
| F-crash: regression test path locked | ✓ | `tests/daemon_pg/test_repo_local_migration_crash_resume.py` |
| F-escape: default-flip locked | ✓ | env var opt-OUT (`=0`) retained, default flipped to enforced |
| F-escape: audit exhaustive | ✓ | "none beyond this one top-level gate"; six legacy modules enumerated |
| F-parser: exact subparser block | ✓ | full code block; smoke command names expected flag *names* (but see F2 for `help=` gap) |
| F-parser: exact dispatch arm | ✓ | `if args.daemon_command == "migrate-repo-local": ... return dispatch_daemon(args)` |
| F-parser: smoke command works | ✓ | `striatum daemon migrate-repo-local --help`, "must not require a daemon" |
| F-test: exact test file named | ✓ | `tests/exit_codes/test_rfc0043_refusals.py` |
| F-test: fixture shape locked | ✓ | `tmp_path` repo, `.striatum/state.sqlite3`, temporary Unix socket |
| F-test: assertion shape locked | ✓ | return-code + `capsys.readouterr().err` substring asserts |
| Implementation order locked + rationale | ✓ | parser → flip → test → crash, with dependency rationale in §0 |
| Backward-compat: SQL additive | ✓ (vacuous) | "No new SQL file is required" |
| Backward-compat: tombstone preserved | ✓ | "normal migration, resumed migration, dry-run, and already-migrated paths" |
| Operator UX envelopes (a) crash-resume | ✗ | not drafted — see F1 |
| Operator UX envelopes (b) exit-12 | partial | stderr substrings locked but full envelope shape not drafted |
| Operator UX envelopes (c) env-var-opt-out | ✗ | not drafted — opt-out is `STRIATUM_DAEMON_REQUIRED=0`; what does the operator see / not see? |

## Disposition

The four design questions are answered well enough to start implementing in
the named order without ambiguity in signatures, file paths, or test shape.
The findings above are scoped fixes to the synthesis text (or to follow-on
work-packet objectives), not redesigns. Accept with findings.
