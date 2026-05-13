---
schema_version: "striatum.finding.v1"
artifact_kind: "finding"
verdict_intent: "accept_with_findings"
severity: "medium"
tags: ["ergonomics_dx", "rfc-0043", "v1", "design"]
---

author: reviewer-unknown-model-001

# Design Review — RFC 0043 V1 Synthesis (ergonomics_dx)

Scope: `docs/dogfood/048/DESIGN_SYNTHESIS.md`. Posture: developer
ergonomics. The reviewer treated the synthesis as the only target
artifact and consulted `src/striatum/cli/mutations.py`,
`src/striatum/cli/parser.py`, and `src/striatum/daemon_rpc/registry.py`
only for the cross-checks named in the prompt
("Method-registry expansion list complete vs
`src/striatum/cli/mutations.py`", parser line citation, etc.).

The design is substantive: it picks the schema-namespacing choice,
names the SERIALIZABLE single-tx semantics, defines the audit-chain
byte-equivalence algorithm, locks the read-capability vocabulary,
and gives implementation-precise filenames and function signatures.
The findings below are DX gaps that an implementer and a first-time
operator will hit before they reach behavioral problems; they should
be tightened before Track A/B implementation lands.

## F1 — Exit-code-12 stderr template prescribes a command that will
##      fail when copy-pasted (medium)

Synthesis §"Track B" (CLI Surface), exit code 12 stderr template:

```text
repo_not_migrated: {repo_path} has not been migrated to daemon PostgreSQL state
Run: striatum daemon migrate-repo-local --from sqlite --to pg --repo {repo_path}
```

But the synthesis defines `migrate-repo-local`'s argparse surface
earlier in the same section as: "required `--repo`, `--postgres-url`,
`--dry-run`, `--keep-sqlite-readonly` ... `--confirm-delete`, and
`--json`." `--postgres-url` is required. A first-time user who
copy-pastes the suggested command gets a second argparse error:
`required: --postgres-url`. The exit-11 template handles this
ergonomically ("`striatum daemon doctor --postgres-url <url>` or set
`STRIATUM_DAEMON_DB_URL`") but exit-12 does not.

Fix one of:
- Include the placeholder in the template:
  `Run: striatum daemon migrate-repo-local --from sqlite --to pg --repo {repo_path} --postgres-url <url>`
  (and mention `STRIATUM_DAEMON_DB_URL` as the alternative, matching
  the exit-11 hint).
- Or make `--postgres-url` optional in the migrate-repo-local
  argparse surface when `STRIATUM_DAEMON_DB_URL` is set, and document
  that explicitly. The current "required" wording rules this out.

## F2 — D094 framing is not cited (medium)

The review prompt's last specific check: "D094 framing cited with RFC
0043 reference." The synthesis cites RFC 0043 in eight places
(intro, Track A schema decision, Track A `repo_migrations`, Track B
exit-code paragraphs, Track B registry expansion, Track B SQL
migration ordering, Integration Order) and is consistent with that
RFC. It never names D094. A reader returning to this synthesis later
cannot link the design choices back to the operator decision that
authorized them.

Fix: add a single-sentence framing line (likely in the introductory
paragraph or the first Track A paragraph) that says, in effect,
"This design implements RFC 0043 V1 per decision D094." If D094
includes any constraint not in RFC 0043 (e.g., release boundary
atomicity), repeat that constraint in the synthesis with the D094
citation.

## F3 — "Every mutation in `src/striatum/cli/mutations.py` has a
##      registered method" mis-cites the source of truth (low/medium)

Synthesis §"Track B" (RPC Registry): "Expand
`src/striatum/daemon_rpc/registry.py::_ENTRIES` to … cover the full
RFC 0043 surface."

The accompanying expansion list does name `work.complete/write` and
`artifact.publish/write`, so the *surface* is covered. But neither
`complete_job` nor `publish_artifact` lives in
`src/striatum/cli/mutations.py`:

```
src/striatum/db.py:1446:def complete_job(
src/striatum/artifacts.py:433:def publish_artifact(
```

`mutations.py` defines `branch_confirm`, `run_start`,
`register_session`, `close_session`, `ack_work`, `heartbeat`,
`release_work`, `send_message`, `block_work`, `verdict_work`,
`submit_review`, `decision_record`, `checkpoint_resolve`. The
reviewer also notes `dogfood.publish_on_behalf` lives at
`src/striatum/dogfood/operator_tools.py:66` and `apply.*` lives
outside `mutations.py` too.

A DX-friendly synthesis names the actual file set (`mutations.py`,
`db.py::complete_job`, `artifacts.py::publish_artifact`,
`dogfood/operator_tools.py`, `apply/*`) once, then says "the
registry must cover every entry in this set." Otherwise the next
reviewer will assume `mutations.py` is exhaustive and miss
`work.complete` and `artifact.publish` if they ever drift.

## F4 — `workflow.generate.preview` is silently dropped from the
##      expansion list (medium)

Today `_ENTRIES` in `src/striatum/daemon_rpc/registry.py:48`
contains:

```
MethodEntry("workflow.generate.preview", "read", True),
MethodEntry("workflow.generate", "write", True),
```

The synthesis §"Track B" preservation list reads "Keep existing
`apply.*`, `cross_repo.*`, `daemon.token.*`, `daemon.key.rotate`,
`daemon.shutdown`, `dogfood.publish_on_behalf`, and
`dogfood.surgical_recovery`." `workflow.generate.preview` is not in
that preservation list, not in the repo-local CLI surface list,
and not in the Daemon-global additions list. A reasonable
implementer reads this as "delete it." If that is the intent, name
it. If not, add `workflow.generate.preview/read` (and confirm
whether the new read-capability "lock" — "Read-capability method
names are locked as `status`, `why`, `doctor`, `dashboard`,
`dashboard.all`, `run.summary`, and `run.graph`" — is meant as a
prohibition on other read methods or only on aliases).

## F5 — Exit code 8 (used inside Track A) is undefined for the
##      reader (low)

Synthesis §"Track A" migrate-repo-local algorithm: "If
`--no-keep-sqlite-readonly` is used, deletion requires
`--confirm-delete`; otherwise refuse with exit code 8."

Exit codes 11 and 12 each get a named token (`daemon_unreachable`,
`repo_not_migrated`), a stderr template, and a JSON envelope. Exit
code 8 here gets a numeric mention only. A first-time operator who
hits "exit 8" will not know what error class it represents or
whether the message they see is correct. The prompt explicitly
calls out exit codes 11 and 12; 8 is a new code introduced by this
design.

Fix: give exit code 8 a token (`destructive_delete_requires_confirm`
or reuse an existing class) and either show the stderr template or
state explicitly that it reuses an existing exit-code-8 contract.
Same shape as 11/12 if possible — one paragraph each.

## F6 — `repo.init/admin` vs `repo.add/admin` distinction is
##      undefined (low)

Synthesis §"Track B" repo-local capability list opens with
`repo.init/admin` and later lists "Daemon-global additions" as
`repo.add/admin`, `repo.remove/admin`, `repo.list/read`. Today's
parser surface (`src/striatum/cli/parser.py:170` onward) has
`repo add`, `repo list`, `repo remove` but no `repo init`. The
existing `_ENTRIES` has no `repo.init` either. The reader cannot
tell whether `repo.init` is a renamed `repo.add`, an additional new
verb, or a stale entry.

Fix: name what `repo.init` is (e.g., "called by
`migrate-repo-local` to implicitly register repos" — the synthesis
hints at this with `_ensure_registered` but does not connect it).
If `repo.init` is internal-only, drop it from the public registry
surface and route through `_ensure_registered` directly. Either
choice is fine; the synthesis must pick one.

## F7 — Exit-11 "Postgres install hints" gap (low)

Prompt-specific check: "Exit code 11 stderr template names socket
path + platform remediation (Linux systemd, macOS launchctl,
foreground hint, Postgres install hints)."

The synthesis template covers socket path, Linux systemd, macOS
launchd, foreground, and Postgres *connection* guidance
(`STRIATUM_DAEMON_DB_URL`, `striatum daemon doctor`). It does not
include OS install hints (`apt install postgresql`,
`brew install postgresql`, container hint). A first-time user
without Postgres installed will run `striatum daemon doctor` and
hit the same connection failure.

Verdict-affecting only if the prompt's "Postgres install hints"
was literal. Recommended fix is a single appended line, e.g.
"Need Postgres? `brew install postgresql@16` (macOS) /
`apt install postgresql` (Debian/Ubuntu) / Docker image
`postgres:16`." Severity low because the connection-time hints
already cover the most common case (Postgres present, env not set).

## F8 — CLI-verb → RPC-method rename mapping not surfaced (low)

The synthesis rewrites the registry namespace: `ack` →
`work.ack`, `publish_artifact` → `artifact.publish`,
`submit_review` → `review.submit`, `verdict` → `review.verdict`,
`complete` → `work.complete`, `claim_next` → `work.claim_next`,
`release` → `work.release`, `block` → `work.block`, `send` →
`work.send_message`, `register-session` → `session.register`,
`session close` → `session.close`, `branch confirm` →
`branch.confirm`, `decision record` → `decision.record`,
`checkpoint resolve` → `checkpoint.resolve`.

User-facing CLI verbs (e.g. `striatum ack`) are unchanged — this
is internal RPC vocabulary — but the synthesis never says so
explicitly. A first-time DX reader scanning the list above will
think `striatum ack` is being renamed to `striatum work ack`.

Fix: add one sentence under the registry expansion list, e.g.
"User-facing CLI verbs are unchanged; only the daemon RPC method
namespace is reorganized. CLI dispatch translates verbs to
namespaced methods inside `daemon_required.py`." If CLI verbs
*are* being renamed, that is a much larger change and the
synthesis should call it out prominently with a deprecation
strategy.

## What the synthesis got right

- The schema-namespacing decision is taken (single shared
  `striatumd.*` schema) and justified ("per-repo schemas would
  make cross-repo queries and registry joins harder without adding
  useful local isolation"). DX-clear.
- The expansion from 15 → 17 tables is explained at the point of
  the deviation: "The prompt names 15 tables, but
  `workflow_snapshots` and `job_dependencies` are required
  structural tables in `src/striatum/schema.py`." Excellent DX
  practice — the reader learns *why* the table count differs from
  the brief, in the same sentence.
- Index strategy is enumerated table-by-table with
  `(repository_id, …)` prefix consistent across every entry. A
  reader can audit the migration SQL against this list line by
  line.
- `--keep-sqlite-readonly` defaulting true through
  `argparse.BooleanOptionalAction` is the right choice for a
  destructive flag pair.
- The byte-equivalence algorithm in `compute_repo_local_reanchor`
  is named ("canonical JSON arrays of source rows ordered by
  stable primary key … projected to source-column names and
  compact UTF-8 JSON … requires SHA-256 equality") and is
  reproducible.
- Append-only grants on `events` and `artifacts` are preserved
  via trigger functions + revoked `UPDATE`/`DELETE` per
  RFC 0033 §3.
- `--no-daemon` retirement cites
  `src/striatum/cli/parser.py:26` through `:30` exactly, and the
  reviewer verified the lines are correct. The argparse failure
  mode (exit 2, `unrecognized arguments`) is named, not
  hand-waved.
- The release-boundary rule ("no user-facing build should ship
  with the schema present but a partial mutation registry or any
  SQLite fallback") is the right framing for an atomic-cutover
  RFC.

## Verdict

`accept_with_findings`. The design is implementable as written;
findings F1, F2, F3, F4 each surface a place where an implementer
or first-time operator will be slowed down or misled. Tightening
these before scaffold-land keeps the DX surface honest.
