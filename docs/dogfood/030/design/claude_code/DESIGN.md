# Claude Code Design: RFC 0026 and RFC 0027 — Provenance That Doesn't Lie

author: designer-claude-opus-001
date: 2026-05-11
status: design (fresh-lane round 1)
target: docs/rfcs/0026-lane-attestation-and-operator-byline-honesty.md, docs/rfcs/0027-sealed-patch-provenance-mode.md

## 0. Frame

These two RFCs share a root cause: today the operator who drives the
`striatum` CLI is structurally the same entity that can write source
bytes, mutate `.striatum/state.sqlite3`, and assert whatever lane id it
likes on `register-session`. Every layer of the runner accepts that
self-attestation as fact. The observed failure mode in dogfood is
**operator-surrogate inline edits published under a lane byline that was
never running** — `author: reviewer-codex-gpt-5.5-001` on review
artifacts whose bytes the surrogate typed itself.

RFC 0026 stops the byline forgery at the runner's CLI boundary. RFC 0027
stops the byte forgery at the filesystem boundary. They are deliberately
two RFCs because they have very different blast radii: 0026 is a Python
patch and a workflow flag; 0027 is an OS-level authority redesign that
only makes sense for users who can pay the containment cost.

This design treats them as a single program with five phases. The phases
are ordered so that each is independently shippable, defensible, and
honest about what it does and does not prove.

## 1. What Striatum Cannot Prove (and Will Stop Pretending To)

Before the design proper, write down the negative space. Every
acceptance test in this design exists to keep one of these honest:

1. **Model-token authorship.** The bytes in a published artifact came
   from *some* process; the runner cannot prove they came from the
   model lane's token stream. D028 forbids broad transcript capture, so
   the bytes-to-tokens link does not exist. RFC 0026 attestation means
   "a process from this lane's command is alive on this pid right now",
   not "this artifact was authored by that process".
2. **Independent human decision provenance.** A `decision record`
   carries `owner: human`. The runner has no privileged channel to tell
   a human typing from an LLM operator surrogate. Operator bylines are
   role-typed (`author: operator`), never identity-typed.
3. **Adversarial-local-root resistance.** An operator who can write
   `.striatum/state.sqlite3`, patch `src/striatum/identity.py`, or
   install a malicious `striatum` binary can still forge anything they
   like. RFC 0027 sealed mode raises the bar by moving the signing key
   and the protected tree outside the operator's writable authority,
   but **it does not defend against root**. The runner must say this in
   doctor output, in evidence export, and in receipt verification
   error messages — not bury it in an RFC.
4. **No prompt-dictation defense.** Even an attested lane can be told
   what to write. RFC 0027 protects the *path from reviewed patch to
   protected tree*, not the path from human intent to reviewed patch.

Every external surface that touches provenance — receipt JSON, evidence
export, `doctor`, `status`, `run summary`, web UI — gets an explicit
"this is what mode X does NOT prove" sentence. See §10.

## 2. Provenance Mode as a First-Class Workflow Property

The first user-visible change is naming. Add to the workflow schema:

```json
{
  "provenance_mode": "advisory" | "attested_bylines" | "sealed_patch"
}
```

- `advisory` — today's behavior, the default for every existing
  workflow. The runner records what the operator asserts. No new
  guarantee.
- `attested_bylines` — RFC 0026 mode. Lane-typed bylines require a live
  supervised lane binding; unattested writes are labeled `author:
  operator`. No filesystem containment.
- `sealed_patch` — RFC 0027 mode. Adds capture, hash-bound review,
  apply gate, and signed receipts on top of `attested_bylines`
  semantics. Requires a containment boundary (§5) that the operator
  cannot dismantle from a normal shell.

### Storage and snapshotting

`provenance_mode` is added to the workflow JSON schema (validator
update in `src/striatum/workflow.py`). It is snapshotted into the
existing `workflow_snapshots.workflow_json` blob; no new column on
`runs`. A small helper `run_provenance_mode(conn, run_id)` reads the
snapshot once and caches per call.

The validator enforces:

- value is one of the closed set
- `sealed_patch` requires `protected_paths` and `operator_writable_paths`
  (§5) to be present and non-overlapping
- `sealed_patch` requires `require_attested_lane: true` on every
  repo-write job (you cannot seal a patch and disclaim the lane that
  produced it)

When the field is absent the run defaults to `advisory` and emits a
`workflow.provenance_mode_default` lint warning under
`workflow validate --json` so authors know what they got.

### Surfacing

`provenance_mode` is added as a top-level field on:

- `status --json` (run-level)
- `why <run_id>` and `why <job_id>`
- `evidence export` (in the run header block)
- `run summary` (`## Provenance Mode` section with the literal "what
  this proves / does not prove" sentences from §10)
- the dashboard top bar
- `striatum doctor` (a `mode_unsupported_by_environment` problem record
  when `sealed_patch` is declared but containment is not actually in
  place)
- the web UI run-detail page (a colored chip per mode)

## 3. Phase A — RFC 0026: Lane Attestation and Byline Honesty

This is the first shippable unit. No new tables. No filesystem
permission changes. The change is entirely inside Python plus one new
nullable column on `sessions`.

### 3.1 Attestation as a derived property

In `src/striatum/identity.py`:

```python
def session_lane_attested(conn, *, session_id) -> AttestationResult: ...

class AttestationResult(TypedDict):
    attested: bool
    pid: int | None
    state: str | None
    reason: str | None  # "no_supervisor", "pid_gone", "supervisor_lost"
```

The helper consults `process_supervisors` for the session, filters to
states `('starting','attached')`, runs the existing `os.kill(pid, 0)`
liveness probe, and returns the structured result. Returning the
*reason* lets callers render distinct hints
(`recover_orphan_supervisor` vs. `attach_supervisor`) without
re-querying. The check is read-only; it does not mutate
`process_supervisors` rows. A separate lazy-transition path (already
present in `expire_leases` and `supervise status`) handles
pid-gone-to-`lost` transitions.

### 3.2 Honest byline derivation

`artifact_author_identity` in `src/striatum/identity.py` gains an
`attested: bool` parameter. Default is `False` so any caller that
forgets to wire it gets the honest answer.

```python
def artifact_author_identity(workflow, *, role_id, lane_id,
                             workflow_job_id, ordinal=None,
                             attested=False,
                             operator_label=None):
    if attested:
        # existing path: reviewer-codex-gpt-5.5-001
        line = f"author: {role}-{model}-{ordinal:03d}"
    elif operator_label:
        line = f"author: operator [self-declared: {label}]"
    else:
        line = "author: operator"
    ...
```

`expected_author_line` in `src/striatum/artifacts.py:564` becomes the
single chokepoint. It now:

1. resolves the session row (already does)
2. calls `session_lane_attested(conn, session_id=...)` once
3. reads `sessions.operator_label` (new nullable column)
4. passes both into `artifact_author_identity`

Because `validate_optional_markdown_author_line` already compares
front-matter against `expected_author_line`, the existing exit-code-6
chokepoint now *enforces* the honest byline. An operator who registers
`--lane codex` without `supervise start` and tries to publish
`author: reviewer-codex-gpt-5.5-001` is refused. The diff that ships
is one line in `identity.py` and one helper call in `artifacts.py` —
the publisher path itself does not change.

### 3.3 Verdicts and run summary

Verdict bylines are reconstructed at *read* time, not stored. Surfaces
that need to render a verdict author identity (`why`, `status`,
`evidence export`, `run summary`, the web UI verdict block) all flow
through `artifact_author_identity`. Wiring `attested` through the read
path is mechanical: the helper accepts `session_id` and looks up
attestation itself, so callers do not need to remember.

Important subtlety: attestation is **evaluated at read time, not at
record time**. A verdict recorded while a supervisor was alive is
displayed differently after the supervisor dies. This is the right
semantics because *the runner does not know after the fact whether
the bytes really came from the supervised process*; reflecting current
attestation state is the most honest read. The `verdict.recorded`
event payload continues to carry the operator-asserted `lane_id` (the
operator did claim it). The byline is purely a presentation concern.

Open question for review: should `verdict.recorded` payload also
snapshot `attested_at_record_time: true|false` so historical audits
can distinguish "was attested when written" from "is attested now"?
My take: yes, for one direction only — record the snapshot, but never
display "was attested" as if that meant something. It is an audit
field, not a display field. See open question Q1.

### 3.4 `require_attested_lane` opt-in

Add an optional field on jobs (preferred over a lane-level field — the
relevant unit is the work, not the configured lane):

```json
{"id": "review_x", "type": "review", "require_attested_lane": true}
```

When set, `record_artifact` and `record_verdict` (in
`src/striatum/cli/mutations.py`) compute attestation and refuse with
`InvalidTransitionError` if false. The error includes the recovery
command literal: `striatum supervise start --session-id <sid>`.

Workflow validation rejects `require_attested_lane: true` on a job
whose lane uses an adapter that cannot support supervision (e.g. an
adapter family that has no `supervise start` command). This stops the
"set the flag, never spawn a supervisor, every claim refused" foot-gun
at workflow-validate time.

When `provenance_mode = "sealed_patch"`, the validator promotes
`require_attested_lane` to required-on-every-repo-write-job (§2).

### 3.5 `operator_label` self-labelling

Add a nullable `operator_label TEXT` column to `sessions` (migration
version N — see §9). Surface in `register-session`:

```text
striatum register-session --run-id <r> --role reviewer --lane codex \
    --operator-label "claude-opus-driver"
```

The label is rendered as `author: operator [self-declared:
claude-opus-driver]`. The bracketed framing is intentionally ugly so
it cannot be mistaken for an attested byline at a skim. The label is
never silently sanitized; the validator restricts it to `[a-z0-9-]{1,32}`
to prevent operator prose leakage into committed bylines.

The label has zero effect on attestation gates. It is purely a
human-readable signal.

### 3.6 `register-session` JSON output

The response gains:

```json
{
  "session_id": "...",
  "slug": "...",
  "lane_attestation": "unattested",
  "operator_label": "claude-opus-driver"
}
```

Plus a stderr hint when `unattested`:

```text
session sess_abc registered (lane: codex, attestation: unattested).
attach a supervisor: striatum supervise start --session-id sess_abc
```

The hint is one line, printed to stderr so it does not corrupt
`--json` stdout. It points at the *exact* recovery verb so an operator
surrogate reading the line cannot misread what to do next.

### 3.7 Phase A migrations and compat

- Migration vN: `ALTER TABLE sessions ADD COLUMN operator_label TEXT`.
  Forward-only. Nullable. No backfill.
- No `process_supervisors` change.
- Existing committed artifacts and event payloads are not rewritten.
  Historical incidents stay as-is; retraction is the issue #3 RFC
  scope.
- Existing workflows with no `provenance_mode` continue to operate as
  `advisory`. Their unattested publishes start emitting `author:
  operator` *only* in the rendered byline at the publish-validate
  chokepoint. Workflows that today register sessions without
  `supervise start` and rely on the old byline will see Markdown
  publishes refused with exit 6 until they either (a) add
  `supervise start`, (b) update their author lines to `author:
  operator`, or (c) declare a workflow opt-out
  (`legacy_unattested_bylines: true` — see open question Q2).

This is a behavior break in the most defensible direction: the change
is exactly the one the RFC says we want. The dogfood `examples/`
workflows that depend on the old byline need updates as part of
Phase A delivery (already in RFC 0026 acceptance).

### 3.8 Phase A tests

New test file `tests/test_lane_attestation.py`. Coverage:

- `session_lane_attested` returns `attested=False` with `reason=
  "no_supervisor"` for a session that never had one.
- Returns `attested=True` after `supervise start` puts a row in
  `attached` with an alive pid.
- Returns `attested=False` after the supervisor pid is killed (the
  next mutation that hits `process_supervisors` transitions to
  `lost`).
- `register-session` emits the `unattested` hint and the JSON field.
- Publishing an artifact whose front matter says `author:
  reviewer-codex-gpt-5.5-001` from an unattested session is refused
  with exit 6.
- Publishing the same artifact from an attested session succeeds.
- Verdict byline rendering in `evidence export` reflects current
  attestation, not record-time.
- `require_attested_lane: true` on a review job refuses
  `verdict record` from an unattested session with the recovery hint
  in the error message.
- `operator_label` round-trips through `register-session` →
  `publish-artifact` → `evidence export`.
- The `--operator-label` value is validated against the allowed
  character set; rejection cases for over-long, non-ascii, and
  injection-bait labels.

Existing tests in `tests/test_artifacts.py` and
`tests/test_identity.py` are updated to pass `attested=True`
explicitly so they continue to test the attested code path. A handful
of dogfood fixtures (`examples/code-change-flow`,
`examples/docs-review-flow`) get either `require_attested_lane: true`
on review jobs *or* a workflow comment acknowledging they will render
operator bylines.

## 4. Phase B — Honest Mode Surfacing (RFC 0027 Step 1)

This is the second shippable unit and the first piece of RFC 0027.

Phase B does **not** add containment, signing, capture, or apply gates.
It adds the *honest naming* across every surface that today implies
provenance:

- `provenance_mode` validation (§2)
- `status`, `why`, `evidence export`, `run summary`, dashboard, web UI
  display the mode prominently
- `doctor` problem record `provenance_mode_unsupported_by_environment`
  when `sealed_patch` is declared but the workflow lacks
  `protected_paths` / `operator_writable_paths`, OR when the runner
  cannot detect the OS containment guarantees (§5) the mode requires
- `run start` refuses a `sealed_patch` run when the containment probe
  fails. Refusal exit code: 9. Message names the missing capability:
  `protected source writable by operator` or `lane scratch not
  isolated`.
- `evidence export` includes a `## Provenance Mode` section with the
  literal "this is what mode X proves / does not prove" sentences. The
  same sentences are embedded in `striatum doctor` output.

Phase B ships a behavior change that is purely additive: existing
workflows default to `advisory` and the new mode surface is read-only.
It is delivered before patch capture so that downstream RFCs can be
designed against the actual mode plumbing rather than speculating.

### Phase B touch points

- `src/striatum/workflow.py` — validator for `provenance_mode`,
  `protected_paths`, `operator_writable_paths`.
- `src/striatum/cli/runs.py` — `run start` refusal hook.
- `src/striatum/cli/status.py` — top-level field plus problem record.
- `src/striatum/evidence.py` — header section, redaction-safe.
- `src/striatum/web/templates/run_detail.html` — chip.
- `src/striatum/dashboard.py` — top bar.

### Phase B tests

- Validator rejects unknown mode values, conflicting paths, and
  `sealed_patch` without `require_attested_lane: true` on a
  repo-write job.
- `run start` refuses `sealed_patch` when the containment probe in
  doctor fails (probe can be stubbed in tests).
- Evidence export, run summary, and dashboard render the mode and the
  "what this proves" sentences for each value.

## 5. Phase C — Containment Boundary

This is where RFC 0027 stops being a Python patch and starts being an
OS-level decision. It must be designed honestly: either we mechanically
prevent the operator from writing protected source paths, or we admit
we do not and stay in `attested_bylines`.

### 5.1 Three local authorities

Per RFC 0027:

1. **Operator control workspace** — writes workflow JSON, RFCs, docs,
   driving the runner. Cannot write protected source paths or lane
   scratch.
2. **Lane scratch workspace** — writable by the attested lane process,
   not by the operator. One per repo-write job.
3. **Protected repository** — the canonical target tree. Writable only
   by the apply mutation. Readable by the operator through
   `striatum source read`/`grep` (added in Phase D) and through any
   read-only mount mechanism the platform offers.

### 5.2 Implementation order

Phase C lands the **first** containment mechanism. The synthesis (see
§13) chooses between Linux options; I propose Linux `bwrap` as the
first because it is the lowest-deployment-friction option that lets us
honestly say "operator cannot write protected paths" without forcing
the user to manage separate Unix accounts. bwrap is widely available
and gives us namespace-level mount isolation per process.

For Phase C V1:

- Linux supported via `bwrap` if installed, else clearly unsupported.
- macOS and Windows are explicitly marked **unsupported for
  `sealed_patch`**. `run start` refuses with a hint pointing at
  `attested_bylines`. Silent degradation is forbidden (RFC 0027 Goals).
- The runner ships a doctor probe `sealed_patch_environment` that
  checks: bwrap available, the protected paths are not writable by the
  current uid in a normal shell, and the scratch directory permissions
  are tight.

### 5.3 What containment actually does

When the run is in `sealed_patch` mode:

- `run start` re-mounts `protected_paths` read-only for the operator's
  shell view if a control-workspace bind mount is being used. (The
  exact mechanism is implementation-defined; the contract is "operator
  shell cannot write protected paths.")
- `supervise start` for a repo-write job:
  - Allocates a per-job lane scratch directory at
    `.striatum/sealed/scratch/<job_id>/<work_session>/` and bwrap-mounts
    it as the lane's writable cwd. The bind covers
    `protected_paths` read-only and `lane scratch` read-write; the
    operator's normal shell sees them with the operator's permissions
    (read-only for protected).
  - Sets the lane's working directory to the scratch path. The lane's
    edits never touch the canonical tree.
- `apply reviewed-patch` is the only command that writes
  `protected_paths`. It performs the write inside the runner process,
  not by running a lane command.

### 5.4 Operator read paths

Sealed mode cannot kill the operator's ability to *read* source. Adding
`striatum source read --path <p> [--lines a:b]` and `striatum source
grep --pattern <re>` is the path-of-least-surprise: same vocabulary,
read-only, runs in the runner's authority. Both are mediated through
the existing API so the local web UI and chat surfaces inherit them
free. They are not new authority — anyone who can run any `striatum`
verb can already see the bytes — but they are a friction-free
substitute for the operator's normal `cat` and `rg`.

### 5.5 Honest unsupported messaging

`striatum doctor` and `run start` emit, verbatim:

> "sealed_patch is currently unsupported on this platform/configuration.
> The runner cannot mechanically prevent operator writes to declared
> protected paths. Use provenance_mode: attested_bylines for evidence
> honesty without containment, or install bwrap (Linux) to enable
> sealed_patch."

That message will get pasted by every operator trying to debug their
setup. It must say exactly what is missing and exactly what the
fallback is.

### 5.6 Phase C tests

The hard ones, all under `tests/test_sealed_patch_containment.py`
(skipped on non-Linux):

- Operator shell `echo` > `src/foo.py` fails with permission denied
  when the run is sealed.
- Operator shell cannot write to `.striatum/sealed/scratch/<job>/`.
- Lane scratch path written from inside a supervised process succeeds.
- `apply reviewed-patch` writes `src/foo.py` through the runner.
- Doctor probe reports the expected status under each condition.

These tests will be platform-gated and may run only in Linux CI. The
acceptance criterion remains "tests pass under the supported
configuration" — not "tests skip silently when unsupported".

## 6. Phase D — Patch Artifacts and Hash-Bound Reviews

This phase adds the durable patch object and the digest-bound verdict
without yet adding the apply gate. It can ship under `advisory` mode
too, giving workflows the exact-review-object property even when
containment is off — useful for evidence quality and a small first
step toward the apply-gate semantics.

### 6.1 Schema

New migration vN+1 adds:

```sql
CREATE TABLE patch_artifacts (
  artifact_id TEXT PRIMARY KEY REFERENCES artifacts(artifact_id),
  run_id TEXT NOT NULL REFERENCES runs(run_id),
  producer_job_id TEXT NOT NULL REFERENCES jobs(job_id),
  producer_session_id TEXT NOT NULL REFERENCES sessions(session_id),
  producer_supervisor_id TEXT REFERENCES process_supervisors(supervisor_id),
  base_tree TEXT NOT NULL,
  result_tree TEXT NOT NULL,
  patch_sha256 TEXT NOT NULL,
  paths_json TEXT NOT NULL,
  blob_hashes_json TEXT NOT NULL,
  hunk_hashes_json TEXT NOT NULL,
  write_scope_validated INTEGER NOT NULL,
  captured_at TEXT NOT NULL
);
CREATE INDEX idx_patch_artifacts_run ON patch_artifacts(run_id);
CREATE UNIQUE INDEX uq_patch_artifacts_digest
  ON patch_artifacts(run_id, patch_sha256);
```

`artifacts.artifact_kind` gains `patch` (allowed-kinds set in
`src/striatum/artifacts.py:ALLOWED_ARTIFACT_KINDS`). Patch artifacts
do not carry an `author:` line (they are byte objects, not authored
prose), but they do carry the same producer identity the byline path
uses for verdict rendering.

Verdicts gain optional columns for the bound digest:

```sql
ALTER TABLE verdicts ADD COLUMN reviewed_artifact_id TEXT
    REFERENCES patch_artifacts(artifact_id);
ALTER TABLE verdicts ADD COLUMN reviewed_digest TEXT;
ALTER TABLE verdicts ADD COLUMN reviewed_base_tree TEXT;
ALTER TABLE verdicts ADD COLUMN reviewed_result_tree TEXT;
```

All nullable. Existing verdicts continue to validate.

### 6.2 New mutations

- `striatum patch capture --session-id <s> --job-id <j> --lease-id <l>
  --base-tree <sha>` walks the lane scratch workspace, computes the
  delta against the recorded `base_tree`, validates write scope and
  forbidden paths, refuses empty patches (unless the job declares
  `allow_empty_patch: true`), computes blob and hunk hashes, and
  writes one row to `patch_artifacts`. Returns the artifact id and
  digest.
- `striatum verdict record [...] --reviewed-artifact-id <a>` resolves
  the patch artifact, copies its digest/trees into the verdict columns,
  and records as today.
- `striatum submit-review` is updated to accept the same flag.

### 6.3 Capture refusals

`patch capture` refuses, with exit code 6:

- empty diff against `base_tree` unless `allow_empty_patch`
- any path outside the job's `write_scope.allowed_paths`
- any path in `forbidden_paths` (which includes `.striatum/` by
  construction)
- workspace not allocated to that job (sealed mode only — the bind
  mount label drives this check)
- supervised lane is `lost` when the job's `require_attested_lane:
  true` is set (Phase A interlock)

Captured patches are immutable. If a lane revises after a `needs_
revision` verdict, the new capture is a new artifact with a new id and
new digest. The old verdict no longer binds to it.

### 6.4 Phase D tests

- Capture against a clean workspace produces a deterministic digest.
- Capture refuses out-of-scope paths.
- Capture refuses forbidden paths.
- Capture refuses empty diff unless the workflow opts in.
- A verdict over digest A does not satisfy any future apply check for
  digest B (the check itself ships in Phase E, but the verdict's
  copied columns are testable now).
- Re-running capture after a scratch mutation produces a different
  digest.
- Patch artifact appears in `evidence export` with its digest and
  recorded producer identity.

## 7. Phase E — Apply Gate and Local Signed Receipt

This is the final shippable unit. It adds the mutation `apply
reviewed-patch`, the per-run signing key, the receipt format, and
`provenance verify`.

### 7.1 Apply preconditions

`apply reviewed-patch --run-id <r> --artifact-id <a>` refuses unless
**all** hold:

1. run `provenance_mode = "sealed_patch"`
2. patch artifact exists and matches digest
3. accepting verdicts present for every job edge whose
   `required_review_postures` (or workflow-declared apply gate) names
   it
4. each accepting verdict has `reviewed_digest = patch.patch_sha256`
   and `reviewed_base_tree = patch.base_tree`
5. patch touches only allowed paths
6. protected tree is currently at `patch.base_tree` (recompute; do not
   trust a snapshot — drift detection)
7. required `verification` jobs (RFC 0027 Open Question, see §11)
   passed for the candidate tree
8. no open blocker, paused run, canceled run, or unsatisfied
   dependency on the producer job
9. `require_attested_lane` satisfied for the producer session at
   capture time

Refusals exit code 5 and include a one-line specific reason. There is
no `--force` flag on `apply reviewed-patch`. The flag does not exist
in this design.

### 7.2 Signing

Run-level signing key, Ed25519, generated by `striatum keys init` at
init time and stored at `.striatum/keys/runner-ed25519.pem` (private)
plus `.striatum/keys/runner-ed25519.pub` (public). **Critical
containment requirement:** in `sealed_patch` mode, the private key
file must be in a location the operator cannot write. Phase C
containment is the only mechanism that makes this true. In `advisory`
mode the runner will sign receipts for evidence quality, but doctor
emits `signing_key_in_operator_authority` and the receipt verification
output names it.

This is the single most important honesty constraint in the whole
design: **in advisory and attested_bylines modes, signed receipts do
not prove anything an attacker with shell access cannot forge**. Phase
E ships with that exact sentence in receipt verification output.

### 7.3 Receipt format

```json
{
  "receipt_version": "striatum.receipt.v1",
  "run_id": "...",
  "workflow_hash": "...",
  "striatum_version": "1.20.1",
  "schema_version": 8,
  "base_tree": "...",
  "result_tree": "...",
  "applied_at": "2026-05-11T12:00:00Z",
  "patch_artifact_ids": ["..."],
  "patch_digests": ["..."],
  "producer_jobs": ["..."],
  "producer_sessions": ["..."],
  "producer_supervisors": ["..."],
  "review_verdict_ids": ["..."],
  "verification_job_ids": [],
  "commit_hash": "...",
  "provenance_mode": "sealed_patch",
  "signature": {"alg": "ed25519", "key_id": "...", "sig": "..."}
}
```

Receipts are durable. Storage choice (see open question Q3): I
recommend a dedicated `.striatum/receipts/<run_id>/<sequence>.json`
file plus an `artifacts` row of kind `receipt` so the file flows
through the existing artifact path naturally. Git notes and commit
trailers are too convention-heavy and split the data across two
sources of truth.

### 7.4 Sealed-mode local commit

`apply reviewed-patch` in `sealed_patch` mode performs `git add` of
the touched paths plus a signed git commit using the runner's
signing key (separate Ed25519 key, registered with git via
`gpg.format = ssh` and an allowed signers file the operator cannot
modify — same containment story as the receipt key). The commit
message names the run, patch artifact id, and receipt path. The
`commit_hash` field of the receipt records the resulting sha.

Striatum **never** pushes, merges, rebases, or rewrites history. This
is non-negotiable per RFC 0027 Goals.

### 7.5 `striatum provenance verify`

`striatum provenance verify --run-id <r>` (and the equivalent
`--receipt-file <p>` for verifying from an exported bundle) walks the
receipt:

- recompute patch digests from the patch artifact bytes
- compare receipt signature against the runner public key
- compare `base_tree` against the current `HEAD~1` (when a commit was
  made) or recorded prior tree
- compare `result_tree` against current `HEAD`
- assert the recorded review verdict ids exist and have matching
  `reviewed_digest` and `reviewed_base_tree`

Verification fails after direct SQLite tamper (signature does not
verify because the receipt was computed over the canonical patch
artifact bytes, not the row contents), after patch substitution
(digest mismatch), and after protected-tree drift (tree mismatch).
Verification cannot detect prompt-dictation or model-token authorship
forgery — see §10.

`striatum provenance status --run-id <r>` reports the full chain
state: latest receipt, current tree match, signature key id, mode.

### 7.6 Phase E tests

- Apply refuses every individual precondition with the specific exit
  code and message.
- Apply with all preconditions met writes the file and produces a
  receipt.
- Receipt round-trips through `provenance verify`.
- SQLite tamper (`UPDATE verdicts SET reviewed_digest=... WHERE
  ...`) fails verification.
- Patch artifact byte substitution fails verification.
- Forced tree edit by the operator (in advisory mode where it is
  possible) fails verification.
- Evidence export contains the receipt JSON and a verify command.
- Receipts can be verified from an exported bundle without the
  SQLite database (this is the actual portability test).
- Sealed-mode commit lands locally and never pushes; a git push
  hook smoke test confirms (just runs `git remote -v` and `git log
  origin/<branch>` to assert no push).

## 8. CLI Surface Summary

New verbs:

- `striatum patch capture` (Phase D)
- `striatum apply reviewed-patch` (Phase E)
- `striatum provenance verify` (Phase E)
- `striatum provenance status` (Phase E)
- `striatum keys init` / `keys rotate` / `keys export-public`
  (Phase E)
- `striatum receipt show` (Phase E; thin wrapper over artifact read)
- `striatum source read` / `source grep` (Phase C)

Modified verbs:

- `register-session` gains `--operator-label`, returns
  `lane_attestation` (Phase A)
- `publish-artifact` gains attestation lookup transparently (Phase A)
- `verdict record` / `submit-review` gain `--reviewed-artifact-id`
  (Phase D)
- `status` / `why` / `evidence export` / `run summary` / `doctor` /
  dashboard / web UI gain `provenance_mode` and attestation rendering
  (Phases A/B)

Removed verbs: none. Every change is additive.

## 9. Migration Strategy

Schema migrations needed, in version order:

- **vN**: `sessions.operator_label TEXT` nullable. (Phase A)
- **vN+1**: `patch_artifacts` table + `verdicts.reviewed_*` columns +
  add `patch` and `receipt` to allowed artifact kinds. (Phase D)
- **vN+2**: No schema change for Phase E — receipts are artifact rows
  plus filesystem JSON. Phase E does add an index on
  `artifacts(artifact_kind, run_id)` to keep receipt lookup fast as
  artifact counts grow.

Compatibility risks and mitigations:

- **Byline break (Phase A)**: existing dogfood workflows lose their
  attested bylines until `supervise start` lands or the workflow opts
  in. Mitigation: ship a one-shot `striatum doctor --check
  byline_breakage` that scans the workflow for review jobs with no
  supervise-aware lane and prints the exact remediation. Add a
  `legacy_unattested_bylines: true` workflow opt-out for one minor
  release (deprecation cycle), then remove. See open question Q2.
- **Schema version bump**: each migration follows the standard RFC
  0006 path (`PRAGMA user_version`, forward-only). Existing dbs apply
  pending migrations on next connect. Older `striatum` versions
  refuse newer dbs with exit code 9 — that is the contract.
- **Path validation**: `provenance_mode = "sealed_patch"` introduces
  `protected_paths` and `operator_writable_paths` validators that
  reject `..`, absolute paths, and `.striatum/` overlaps. Same
  rejection rules as existing `write_scope` checks; reuse the helper.
- **Adapter compatibility**: only `process` adapter with supervision
  is compatible with `require_attested_lane: true`. The validator
  rejects the combination otherwise. Existing `tool_family` profiles
  (`codex`, `claude_code`, `gemini_cli`, `generic`) already declare
  supervision compatibility; the validator reads that.

Rollback plan per phase:

- Phase A is rollback-safe via `legacy_unattested_bylines: true`
  workflow opt-out. After the deprecation window, rollback requires a
  release downgrade — but the bylines themselves are not destructive.
- Phase B is purely additive; rolling back leaves the field accepted
  but ignored.
- Phase C is opt-in per workflow (`provenance_mode: sealed_patch`).
  Workflows that did not opt in are unaffected.
- Phase D adds tables; rollback requires the standard migration
  downgrade story (manual `ALTER TABLE DROP` in a forward migration).
- Phase E adds receipts as artifacts plus a per-run signing key.
  Disabling apply on rollback leaves receipts as historical
  artifacts.

## 10. Honest-Naming Block

Every mode-rendering surface (status, why, evidence export, run
summary, dashboard chip hover, doctor) includes the literal sentences
below. They are not paraphrased anywhere; they ship as a string
constant in `src/striatum/provenance.py:MODE_DOCSTRINGS`.

**advisory:** "The runner records workflow state for work submitted
through its commands. It does not verify that lane bylines correspond
to live supervised processes. It does not contain an operator who can
write source files directly. Use this mode for fast local workflows
where honesty of provenance is not a workflow requirement."

**attested_bylines:** "Lane-typed bylines (e.g.
`reviewer-codex-gpt-5.5-001`) are minted only by sessions with a live
supervised lane binding. Unattested sessions publish as `author:
operator`. The runner still does not contain an operator who can write
source files directly. Direct edits to `.striatum/state.sqlite3` can
still forge any byline. Use this mode for good-faith operator flows
where a forged byline should require a deliberate act."

**sealed_patch:** "Protected source writes are mediated by Striatum.
Lanes write to scratch workspaces, never to canonical source. Patch
artifacts are immutable and digest-bound. Review verdicts bind to the
exact patch digest. The apply gate writes the protected tree and
emits a signed receipt. This mode does NOT prove that a specific
model token stream authored a patch (transcripts are not captured).
It does NOT prove independent human decision provenance. It does NOT
defend against local root, a compromised Striatum binary, or a
compromised signing key. It proves that accepted protected source
bytes entered through a reviewed Striatum patch object."

The string constants are exported and rendered verbatim. Tests assert
the strings appear in evidence export and run summary, by exact
substring match, so they cannot be silently rewritten.

## 11. Open Questions for the Synthesis

These are the genuine forks where a different reviewer might reasonably
disagree. Naming them here so synthesis can resolve them with full
context:

**Q1. Snapshot attestation on `verdict.recorded`?** I propose recording
`attested_at_record_time: true|false` in the event payload but never
displaying it as if it meant authorship. Audit-only. Alternative: do
not record it at all; rely on the supervisor row's history. My
preference: snapshot, for forensic value.

**Q2. Deprecation window for legacy unattested bylines?** Phase A
breaks existing workflows that publish bylined artifacts without
supervisors. Options: (a) one minor release with
`legacy_unattested_bylines: true` opt-out, then remove; (b) ship the
break in a major release with no opt-out; (c) ship the break in a
minor release with no opt-out and accept that existing dogfood
fixtures need a same-release update. My preference: (a). The dogfood
runs are the obvious validation that the opt-out works.

**Q3. Receipt storage shape?** Options: (a) dedicated `.striatum/
receipts/<run_id>/<sequence>.json` plus an `artifact` row of kind
`receipt`; (b) Git notes on the signed commit; (c) commit trailers; (d)
all of the above. My preference: (a) only. Receipts are not git
metadata; mixing them in confuses the boundary. Evidence export bundles
the JSON. Verification reads the JSON directly.

**Q4. First containment mechanism platform?** RFC 0027 leaves this
open. My recommendation: Linux `bwrap` first because (1) widely
available on developer machines and CI runners, (2) namespace-level
guarantees per process, (3) does not require separate Unix accounts,
(4) compatible with our process adapter shape. Separate users and
ACLs are more deployment-friction. macOS sandboxing is interesting
but the API surface is fiddly and the failure modes silent. Defer
macOS/Windows to a follow-up RFC.

**Q5. `verification` jobs as a hard apply precondition?** RFC 0027
lists "required verification jobs passed for the candidate tree" as an
apply precondition. We do not have a `verification` job type today.
Either (a) require a build-job edge with `required_review_postures`
including some verification posture and read the existing verdict, or
(b) add a new `verification` job type. My preference: (a) for V1, (b)
in a follow-up if the workflow vocabulary needs it.

**Q6. `decision record` and `apply reviewed-patch`?** A `decision`
artifact represents owner acceptance. Should apply require a
co-occurring decision record? RFC 0027 says "the human, or a future
operator quorum design, remains the intent authority". My take: V1
does not gate apply on decision record; it gates on accepting review
verdicts. Decisions remain the acceptance-of-outcome surface (D047
language).

**Q7. Should sealed-mode commit author be configurable?** The local
signed commit defaults to a runner identity (`Striatum Apply
<runner@local>`). Some workflows may want the producer session lane
in the commit message body. My take: ship a fixed format in V1; add
configurability in a follow-up.

## 12. Test Plan Summary

Total new test files (rough scope):

- `tests/test_lane_attestation.py` (Phase A) — ~25 cases
- `tests/test_provenance_mode_surfacing.py` (Phase B) — ~15 cases
- `tests/test_sealed_patch_containment.py` (Phase C, Linux-only) —
  ~10 cases
- `tests/test_patch_artifacts.py` (Phase D) — ~20 cases
- `tests/test_apply_gate.py` (Phase E) — ~25 cases
- `tests/test_receipts.py` (Phase E) — ~15 cases
- `tests/test_provenance_verify.py` (Phase E) — ~10 cases

Each phase's test file is mergeable independently and lands together
with its source. Existing tests under `tests/test_artifacts.py`,
`tests/test_identity.py`, `tests/test_workflow.py`, and
`tests/test_evidence.py` need updates for the byline change and the
new validator.

Coverage commits with each phase: `make test` and `make smoke` pass.
A new `make smoke-sealed` (Linux-only) runs the sealed-mode fixture
end-to-end.

## 13. Staged Delivery

Recommended order (each phase is an independent PR/release):

1. **Phase A** — RFC 0026 V1. Ship `lane_attestation`,
   `author: operator`, `--operator-label`, `require_attested_lane`,
   doctor/status surfacing, fixture updates, `legacy_unattested_
   bylines` opt-out. Release: v1.21.0.
2. **Phase B** — RFC 0027 Step 1 (honest mode surfacing). Ship
   `provenance_mode` validator, doctor probe, status/why/evidence/
   summary/dashboard/web UI rendering, honest-naming block. Release:
   v1.22.0.
3. **Phase C** — RFC 0027 Step 5 first (containment). Ship Linux
   bwrap-based sealed-mode authority model, doctor environment probe,
   `source read`/`grep` operator helpers, `run start` refusal. Release:
   v1.23.0. macOS/Windows clearly marked unsupported.
4. **Phase D** — RFC 0027 Step 3 (patch capture + hash-bound reviews).
   Ship `patch_artifacts` table, `verdicts.reviewed_*`, `patch capture`,
   `--reviewed-artifact-id` on verdicts. Works under any
   `provenance_mode` — sealed mode is not yet required. Release:
   v1.24.0.
5. **Phase E** — RFC 0027 Steps 4 + 6 (apply gate + signed local
   commit). Ship `keys init/rotate`, `apply reviewed-patch`,
   `provenance verify`/`status`, receipt format, sealed-mode local
   signed commit. Release: v1.25.0.

This is the order I would ship even if all five phases were already
implemented, because each phase is independently defensible and
shippable, and each phase's failures (Phase C's containment probe,
Phase E's signature mismatch) are easier to diagnose when they ship
in isolation.

## 14. What Doesn't Land In These RFCs

Explicitly out of scope (in addition to the negative space in §1):

- Issue #3 retraction primitive — separate recovery RFC.
- Cross-machine signing or transparency logs — future RFC.
- macOS/Windows containment — follow-up RFC after Linux V1 lands.
- Adversarial operator quorum (multi-operator decision authority) —
  future RFC.
- Model-token attestation — requires either transcript capture (D028
  refuses) or model-side cryptographic attestation (does not exist
  in the providers we use).
- A `compromised` run state — Issue #3 territory.

Naming these here so the synthesis does not accidentally try to fold
them into RFC 0026 + 0027's V1 scope.
